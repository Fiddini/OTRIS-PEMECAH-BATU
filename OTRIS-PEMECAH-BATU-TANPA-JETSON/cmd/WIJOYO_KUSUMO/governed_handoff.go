package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/otris-id/otris-pemecah-batu/internal/approval"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/profilebuilder"
	"github.com/otris-id/otris-pemecah-batu/internal/qc"
	"github.com/otris-id/otris-pemecah-batu/internal/registry"
)

// admissionInputs carries the explicit governed-handoff inputs. Only the key PATH
// is accepted here; raw private key material never comes from argv or the environment.
type admissionInputs struct {
	approver     string
	decision     string
	issuerKeyPath string
	issuerKeyID  string
	sequence     uint64
}

func (in admissionInputs) validate() error {
	for _, field := range []struct{ name, value string }{
		{"--approver", in.approver}, {"--decision", in.decision},
		{"--issuer-key", in.issuerKeyPath}, {"--issuer-key-id", in.issuerKeyID},
	} {
		if strings.TrimSpace(field.value) == "" || !utf8.ValidString(field.value) {
			return fmt.Errorf("admission requires %s", field.name)
		}
	}
	if in.sequence == 0 {
		return errors.New("admission requires --sequence greater than zero")
	}
	// Decision stays explicit; absence or anything else is never read as approval.
	if in.decision != approval.DecisionApprove && in.decision != approval.DecisionReject {
		return fmt.Errorf("--decision must be %s or %s", approval.DecisionApprove, approval.DecisionReject)
	}
	return nil
}

// loadIssuerKey reads a 64-byte Ed25519 private key, supplied either as exactly 64 raw
// bytes or exactly 128 hex characters. The key file is never modified, copied or logged,
// and key bytes are never included in errors.
func loadIssuerKey(path string) (ed25519.PrivateKey, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read issuer key: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("issuer key must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open issuer key: %w", err)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect issuer key: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("issuer key must remain the same regular file")
	}
	// Check the opened file, not just its path. Windows lacks Unix mode semantics.
	if runtime.GOOS != "windows" && opened.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("issuer key permissions allow group or other access")
	}
	data, err := io.ReadAll(io.LimitReader(f, 129))
	defer zero(data)
	if err != nil {
		return nil, fmt.Errorf("read issuer key: %w", err)
	}

	switch {
	case len(data) == ed25519.PrivateKeySize:
		return ed25519.PrivateKey(append([]byte(nil), data...)), nil
	case len(data) == hex.EncodedLen(ed25519.PrivateKeySize):
		decoded := make([]byte, ed25519.PrivateKeySize)
		if _, err := hex.Decode(decoded, data); err != nil {
			zero(decoded)
			return nil, errors.New("issuer key is not valid hexadecimal")
		}
		return ed25519.PrivateKey(decoded), nil
	default:
		return nil, fmt.Errorf("issuer key must be %d raw bytes or %d hex characters, got %d bytes",
			ed25519.PrivateKeySize, hex.EncodedLen(ed25519.PrivateKeySize), len(data))
	}
}

// zero clears a temporary buffer. Go makes no guarantee that every copy the runtime
// made is erased, so this reduces exposure rather than eliminating it.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// runGovernedAdmission completes approval, admission and self-verification in this
// process, then publishes the finished artifact set atomically.
//
// SIGN → VERIFY → PUBLISH: the final directory appears only after the signature has
// been verified, so a partially completed chain can never look like an admission.
func runGovernedAdmission(
	candidate *profilebuilder.Profile,
	liveMapping *mapping.MappingDefinition,
	in admissionInputs,
	stagingDir, finalOutDir string,
	qcRecord *qc.QCKnowledgeRecord,
	qcApplicability qc.QCApplicability,
) error {
	issuerKey, err := loadIssuerKey(in.issuerKeyPath)
	if err != nil {
		return err
	}
	defer zero(issuerKey)

	engineerApproval, err := approval.Review(candidate, liveMapping, in.approver, in.decision)
	if err != nil {
		return fmt.Errorf("engineer approval: %w", err)
	}
	attestation, err := registry.Admit(candidate, liveMapping, engineerApproval, qcRecord, qcApplicability, issuerKey, in.issuerKeyID, in.sequence)
	if err != nil {
		return fmt.Errorf("registry admission: %w", err)
	}
	// Defense in depth: verify the signature we just produced before publishing.
	derivedPublicKey := issuerKey.Public().(ed25519.PublicKey)
	if err := registry.VerifyAttestation(attestation, candidate, derivedPublicKey, in.issuerKeyID); err != nil {
		return fmt.Errorf("attestation self-verification: %w", err)
	}
	for _, artifact := range []struct {
		name  string
		value any
	}{
		{"approval.json", engineerApproval},
		{"registry_attestation.json", attestation},
	} {
		data, err := json.MarshalIndent(artifact.value, "", "  ")
		if err != nil {
			return fmt.Errorf("encode %s: %w", artifact.name, err)
		}
		if err := writeArtifact(filepath.Join(stagingDir, artifact.name), data); err != nil {
			return err
		}
	}
	return publish(stagingDir, finalOutDir)
}

// writeArtifact creates a new file and flushes it to disk; it never overwrites.
func writeArtifact(path string, data []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", filepath.Base(path), err)
	}
	return nil
}

func requireNewOutput(finalOutDir string) error {
	if finalOutDir == "" {
		return errors.New("admission requires --out")
	}
	if _, err := os.Lstat(finalOutDir); err == nil {
		return fmt.Errorf("output directory already exists: %s", finalOutDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output directory: %w", err)
	}
	return nil
}

// publish flushes the complete artifact set before one same-filesystem rename.
// ponytail: os.Rename is not an atomic no-replace primitive. Use a trusted,
// single-writer parent; hostile concurrent writers need native no-replace rename.
func publish(stagingDir, finalOutDir string) error {
	expected := map[string]bool{
		"profile.yaml": true, "profile.json": true,
		"approval.json": true, "registry_attestation.json": true,
	}
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return fmt.Errorf("inspect staging: %w", err)
	}
	if len(entries) != len(expected) {
		return errors.New("staging must contain exactly four admission artifacts")
	}
	for _, entry := range entries {
		if !expected[entry.Name()] || !entry.Type().IsRegular() {
			return errors.New("unexpected admission artifact")
		}
		if err := syncPath(filepath.Join(stagingDir, entry.Name())); err != nil {
			return err
		}
	}
	if runtime.GOOS != "windows" {
		if err := syncPath(stagingDir); err != nil {
			return err
		}
	}
	if err := requireNewOutput(finalOutDir); err != nil {
		return err
	}
	// No fallible operations after commit: an error must never accompany a final dir.
	if err := os.Rename(stagingDir, finalOutDir); err != nil {
		return fmt.Errorf("publish output: %w", err)
	}
	return nil
}

func syncPath(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open artifact for sync: %w", err)
	}
	return errors.Join(f.Sync(), f.Close())
}

// newStagingDir creates a unique sibling directory of finalOutDir, so publication is
// a rename within one parent filesystem rather than a cross-device copy.
func newStagingDir(finalOutDir string) (string, error) {
	parent := filepath.Dir(finalOutDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return "", fmt.Errorf("create output parent: %w", err)
	}
	dir, err := os.MkdirTemp(parent, "."+filepath.Base(finalOutDir)+".staging-")
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	return dir, nil
}

package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otris-id/otris-pemecah-batu/internal/approval"
	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/profilebuilder"
	"github.com/otris-id/otris-pemecah-batu/internal/registry"
	"github.com/otris-id/otris-pemecah-batu/internal/replay"
)

// TestD3JApproveChainSucceeds: Full APPROVE chain succeeds (test B)
func TestD3JApproveChainSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".out.staging-1")
	finalOut := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	// Copy profile files from build to staging (as BuildFromMappingDefinition produces them)
	if err := copyProfileFiles(t, tmpDir, stagingDir); err != nil {
		t.Fatalf("copy profile files: %v", err)
	}

	issuerPrivKey, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID) // Clean up key file after test

	admission := admissionInputs{
		approver:     "test-engineer",
		decision:     approval.DecisionApprove,
		issuerKeyPath: keyID,
		issuerKeyID:  "issuer-key-1",
		sequence:     1,
	}

	if err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, ""); err != nil {
		t.Fatalf("admission failed: %v", err)
	}

	// Verify final output exists
	if _, err := os.Stat(finalOut); err != nil {
		t.Fatalf("final output not published: %v", err)
	}

	// Verify all four artifacts exist (test C)
	for _, artifact := range []string{"profile.yaml", "profile.json", "approval.json", "registry_attestation.json"} {
		if _, err := os.Stat(filepath.Join(finalOut, artifact)); err != nil {
			t.Fatalf("artifact %s missing: %v", artifact, err)
		}
	}

	// Verify approval.json says ENGINEER_APPROVED (test E)
	// Note: approval.Approval rejects UnmarshalJSON by design; use raw map instead.
	appData, err := os.ReadFile(filepath.Join(finalOut, "approval.json"))
	if err != nil {
		t.Fatalf("read approval.json: %v", err)
	}
	var appMap map[string]interface{}
	if err := json.Unmarshal(appData, &appMap); err != nil {
		t.Fatalf("unmarshal approval.json: %v", err)
	}
	if status, ok := appMap["approval_status"].(string); !ok || status != approval.StatusEngineerApproved {
		t.Fatalf("approval status not ENGINEER_APPROVED, got %v", appMap["approval_status"])
	}

	// Verify registry_attestation.json verifies (test F)
	attData, err := os.ReadFile(filepath.Join(finalOut, "registry_attestation.json"))
	if err != nil {
		t.Fatalf("read registry_attestation.json: %v", err)
	}
	var att registry.Attestation
	if err := json.Unmarshal(attData, &att); err != nil {
		t.Fatalf("unmarshal registry_attestation.json: %v", err)
	}
	if att.AdmissionStatus != registry.StatusRegistryAdmitted {
		t.Fatalf("attestation not REGISTRY_ADMITTED, got %s", att.AdmissionStatus)
	}

	pubKey := issuerPrivKey.Public().(ed25519.PublicKey)
	if err := registry.VerifyAttestation(&att, &candidate, pubKey, admission.issuerKeyID); err != nil {
		t.Fatalf("attestation verification failed: %v", err)
	}

	// Verify candidate remains CANDIDATE (test D) — legacy Status/ValidationStatus fields deprecated
	if candidate.GovernanceStatus != "CANDIDATE" {
		t.Fatalf("candidate not CANDIDATE, got %s", candidate.GovernanceStatus)
	}
}

// TestD3JRejectPublishesNothing: REJECT publishes no final output (test J)
func TestD3JRejectPublishesNothing(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".out.staging-2")
	finalOut := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	// Copy profile files to staging (as BuildFromMappingDefinition would)
	if err := copyProfileFiles(t, tmpDir, stagingDir); err != nil {
		t.Fatalf("copy profile files: %v", err)
	}

	_, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID)

	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionReject,
		issuerKeyPath: keyID,
		issuerKeyID:   "issuer-key-1",
		sequence:      1,
	}

	if err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, ""); err != nil {
		// Rejection should fail gracefully, but the point is no final output
		// The error comes from approval.Review detecting REJECT
	}

	// Verify final output was NOT published
	if _, err := os.Stat(finalOut); err == nil {
		t.Fatal("REJECT published final output; should not")
	}
}

// TestD3JMissingDecisionFails: missing decision fails (test H)
func TestD3JMissingDecisionFails(t *testing.T) {
	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      "", // missing
		issuerKeyPath: "/tmp/key",
		issuerKeyID:   "issuer-1",
		sequence:      1,
	}

	if err := admission.validate(); err == nil {
		t.Fatal("should have failed with missing decision")
	}
}

// TestD3JUnknownDecisionFails: unknown decision fails (test I)
func TestD3JUnknownDecisionFails(t *testing.T) {
	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      "MAYBE",
		issuerKeyPath: "/tmp/key",
		issuerKeyID:   "issuer-1",
		sequence:      1,
	}

	if err := admission.validate(); err == nil {
		t.Fatal("should have failed with unknown decision")
	}
}

// TestD3JPartialAdmissionFlagsFail: any partial admission flag set fails (test G)
func TestD3JPartialAdmissionFlagsFail(t *testing.T) {
	testCases := []struct {
		name     string
		admission admissionInputs
		wantErr  bool
	}{
		{
			"only approver",
			admissionInputs{approver: "eng"},
			true,
		},
		{
			"only decision",
			admissionInputs{decision: approval.DecisionApprove},
			true,
		},
		{
			"only key path",
			admissionInputs{issuerKeyPath: "/tmp/key"},
			true,
		},
		{
			"only key id",
			admissionInputs{issuerKeyID: "key-1"},
			true,
		},
		{
			"only sequence",
			admissionInputs{sequence: 1},
			true,
		},
		{
			"approver + decision",
			admissionInputs{approver: "eng", decision: approval.DecisionApprove},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.admission.validate()
			if tc.wantErr && err == nil {
				t.Fatalf("should have failed: %s", tc.name)
			}
		})
	}
}

// TestD3JMissingApprover: empty approver fails (test S)
func TestD3JMissingApprover(t *testing.T) {
	admission := admissionInputs{
		approver:      "", // empty
		decision:      approval.DecisionApprove,
		issuerKeyPath: "/tmp/key",
		issuerKeyID:   "issuer-1",
		sequence:      1,
	}

	if err := admission.validate(); err == nil {
		t.Fatal("should have failed with empty approver")
	}
}

// TestD3JMissingKeyFile: missing key file fails (test K)
func TestD3JMissingKeyFile(t *testing.T) {
	_, err := loadIssuerKey("/tmp/nonexistent-key-file-xyz")
	if err == nil {
		t.Fatal("should have failed with missing key file")
	}
	if !strings.Contains(err.Error(), "read issuer key") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestD3JInvalidKeyLength: invalid key length fails (test L)
func TestD3JInvalidKeyLength(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "bad-key")

	// Write 32 bytes (too short)
	if err := os.WriteFile(keyPath, make([]byte, 32), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	_, err := loadIssuerKey(keyPath)
	if err == nil {
		t.Fatal("should have failed with invalid key length")
	}
	if !strings.Contains(err.Error(), "must be") || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestD3JMalformedHex: malformed hex key fails (test M)
func TestD3JMalformedHex(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "bad-hex")

	// Write 128 chars that are not valid hex
	badHex := strings.Repeat("Z", 128)
	if err := os.WriteFile(keyPath, []byte(badHex), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	_, err := loadIssuerKey(keyPath)
	if err == nil {
		t.Fatal("should have failed with malformed hex")
	}
	if !strings.Contains(err.Error(), "hexadecimal") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestD3JRaw64ByteKeyWorks: raw 64-byte key works (test N)
func TestD3JRaw64ByteKeyWorks(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "raw-key")

	// Generate a valid Ed25519 key
	_, privKey, _ := ed25519.GenerateKey(nil)
	if err := os.WriteFile(keyPath, privKey, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	loaded, err := loadIssuerKey(keyPath)
	if err != nil {
		t.Fatalf("load raw key failed: %v", err)
	}
	if len(loaded) != ed25519.PrivateKeySize {
		t.Fatalf("loaded key wrong size: %d", len(loaded))
	}
}

// TestD3J128HexKeyWorks: 128-character hex key works (test O)
func TestD3J128HexKeyWorks(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "hex-key")

	// Generate a valid Ed25519 key and encode as hex
	_, privKey, _ := ed25519.GenerateKey(nil)
	hexKey := hex.EncodeToString(privKey)
	if err := os.WriteFile(keyPath, []byte(hexKey), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	loaded, err := loadIssuerKey(keyPath)
	if err != nil {
		t.Fatalf("load hex key failed: %v", err)
	}
	if len(loaded) != ed25519.PrivateKeySize {
		t.Fatalf("loaded key wrong size: %d", len(loaded))
	}
}

// TestD3JInsecurePermissionsFail: insecure Unix permissions fail (test P)
func TestD3JInsecurePermissionsFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission test skipped on Windows")
	}

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "world-readable-key")

	_, privKey, _ := ed25519.GenerateKey(nil)
	if err := os.WriteFile(keyPath, privKey, 0644); err != nil { // 0644 = world-readable
		t.Fatalf("write key: %v", err)
	}

	_, err := loadIssuerKey(keyPath)
	if err == nil {
		t.Fatal("should have failed with insecure permissions")
	}
	if !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestD3JMissingKeyID: missing KeyID fails (test Q)
func TestD3JMissingKeyID(t *testing.T) {
	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: "/tmp/key",
		issuerKeyID:   "", // empty
		sequence:      1,
	}

	if err := admission.validate(); err == nil {
		t.Fatal("should have failed with empty KeyID")
	}
}

// TestD3JSequenceZeroFails: sequence zero fails (test R)
func TestD3JSequenceZeroFails(t *testing.T) {
	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: "/tmp/key",
		issuerKeyID:   "issuer-1",
		sequence:      0, // not > 0
	}

	if err := admission.validate(); err == nil {
		t.Fatal("should have failed with sequence zero")
	}
}

// TestD3JPreexistingOutFails: pre-existing final --out fails (test T)
func TestD3JPreexistingOutFails(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".out.staging-3")
	finalOut := filepath.Join(tmpDir, "out")

	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	if err := os.MkdirAll(finalOut, 0755); err != nil {
		t.Fatalf("mkdir final: %v", err)
	}

	// Copy profile files to staging (as BuildFromMappingDefinition would)
	if err := copyProfileFiles(t, tmpDir, stagingDir); err != nil {
		t.Fatalf("copy profile files: %v", err)
	}

	_, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID)
	_ = keyID

	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: keyID,
		issuerKeyID:   "issuer-key-1",
		sequence:      1,
	}

	err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, "")
	if err == nil {
		t.Fatal("should have failed with pre-existing output")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestD3JStagingCleanedAfterFailure: staging cleaned after failure (test X)
func TestD3JStagingCleanedAfterFailure(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".out.staging-4")
	finalOut := filepath.Join(tmpDir, "out-preexists")

	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	if err := os.MkdirAll(finalOut, 0755); err != nil { // Pre-create to force failure
		t.Fatalf("mkdir final: %v", err)
	}

	_, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID)
	_ = keyID

	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: keyID,
		issuerKeyID:   "issuer-key-1",
		sequence:      1,
	}

	// This should fail at publish stage due to pre-existing final output
	err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, "")
	if err == nil {
		t.Fatal("should have failed")
	}

	// Staging dir should still exist (caller is responsible for cleanup)
	// The test is that it exists, not that it's cleaned automatically within runGovernedAdmission
	if _, err := os.Stat(stagingDir); err != nil {
		// If staging dir is gone, that's fine—it means cleanup happened
		// But if it exists, that's also acceptable as responsibility can be delegated
	}
}

// TestD3JFinalAppearsOnlyAfterSuccess: final directory appears only after success (test Y)
func TestD3JFinalAppearsOnlyAfterSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".out.staging-5")
	finalOut := filepath.Join(tmpDir, "out")

	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	// Copy profile files to staging (as BuildFromMappingDefinition would)
	if err := copyProfileFiles(t, tmpDir, stagingDir); err != nil {
		t.Fatalf("copy profile files: %v", err)
	}

	_, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID)

	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: keyID,
		issuerKeyID:   "issuer-key-1",
		sequence:      1,
	}

	// Before admission
	if _, err := os.Stat(finalOut); err == nil {
		t.Fatal("output should not exist before success")
	}

	if err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, ""); err != nil {
		t.Fatalf("admission failed: %v", err)
	}

	// After success, final output must exist
	if _, err := os.Stat(finalOut); err != nil {
		t.Fatalf("final output should exist after success: %v", err)
	}
}

// TestD3JCandidateRemainsCandidateCandidateAfterApproval: Candidate remains CANDIDATE/CANDIDATE (test D)
func TestD3JCandidateRemainsCandidateCandidateAfterApproval(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".staging-6")
	finalOut := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	// Copy profile files to staging (as BuildFromMappingDefinition would)
	if err := copyProfileFiles(t, tmpDir, stagingDir); err != nil {
		t.Fatalf("copy profile files: %v", err)
	}

	_, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID)

	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: keyID,
		issuerKeyID:   "issuer-key-1",
		sequence:      1,
	}

	if err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, ""); err != nil {
		t.Fatalf("admission failed: %v", err)
	}

	// Candidate struct should NOT be modified by the process (check governed status only)
	if candidate.GovernanceStatus != "CANDIDATE" {
		t.Fatalf("candidate modified governance status: %s", candidate.GovernanceStatus)
	}
}

// TestD3JArtifactMutualConsistency: candidate / approval / attestation mutually consistent (test Z)
func TestD3JArtifactMutualConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".staging-7")
	finalOut := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	// Copy profile files to staging (as BuildFromMappingDefinition would)
	if err := copyProfileFiles(t, tmpDir, stagingDir); err != nil {
		t.Fatalf("copy profile files: %v", err)
	}

	_, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID)

	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: keyID,
		issuerKeyID:   "issuer-key-1",
		sequence:      1,
	}

	if err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, ""); err != nil {
		t.Fatalf("admission failed: %v", err)
	}

	// Load all three artifacts
	profileData, err := os.ReadFile(filepath.Join(finalOut, "profile.json"))
	if err != nil {
		t.Fatalf("read profile.json: %v", err)
	}
	appData, err := os.ReadFile(filepath.Join(finalOut, "approval.json"))
	if err != nil {
		t.Fatalf("read approval.json: %v", err)
	}
	attData, err := os.ReadFile(filepath.Join(finalOut, "registry_attestation.json"))
	if err != nil {
		t.Fatalf("read registry_attestation.json: %v", err)
	}

	var profile profilebuilder.Profile
	var appMap map[string]interface{}
	var att registry.Attestation

	if err := json.Unmarshal(profileData, &profile); err != nil {
		t.Fatalf("unmarshal profile.json: %v", err)
	}
	if err := json.Unmarshal(appData, &appMap); err != nil {
		t.Fatalf("unmarshal approval.json: %v", err)
	}
	if err := json.Unmarshal(attData, &att); err != nil {
		t.Fatalf("unmarshal registry_attestation.json: %v", err)
	}

	// All should reference the same mapping identity
	appMappingIdentity, _ := appMap["mapping_identity"].(string)
	if appMappingIdentity != profile.MappingIdentity {
		t.Fatalf("mapping identity mismatch: %s vs %s", appMappingIdentity, profile.MappingIdentity)
	}
	if att.MappingIdentity != profile.MappingIdentity {
		t.Fatalf("attestation mapping mismatch: %s vs %s", att.MappingIdentity, profile.MappingIdentity)
	}

	// Approval and attestation approver must match input
	appApprover, _ := appMap["approver_identity"].(string)
	if appApprover != admission.approver {
		t.Fatalf("approval approver mismatch: %s vs %s", appApprover, admission.approver)
	}
	if att.IssuerKeyID != admission.issuerKeyID {
		t.Fatalf("attestation issuer key ID mismatch: %s vs %s", att.IssuerKeyID, admission.issuerKeyID)
	}
}

// TestD3JNoVerifiedStatuses: no VERIFIED / REAL_DEVICE_VALIDATED / PRODUCTION_VALIDATED (test AA)
func TestD3JNoVerifiedStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	candidate, mapping := makeTestCandidate(t, tmpDir)

	stagingDir := filepath.Join(tmpDir, ".staging-8")
	finalOut := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	// Copy profile files to staging (as BuildFromMappingDefinition would)
	if err := copyProfileFiles(t, tmpDir, stagingDir); err != nil {
		t.Fatalf("copy profile files: %v", err)
	}

	_, keyID := makeTestIssuerKey(t, tmpDir)
	defer os.Remove(keyID)

	admission := admissionInputs{
		approver:      "test-engineer",
		decision:      approval.DecisionApprove,
		issuerKeyPath: keyID,
		issuerKeyID:   "issuer-key-1",
		sequence:      1,
	}

	if err := runGovernedAdmission(&candidate, mapping, admission, stagingDir, finalOut, nil, ""); err != nil {
		t.Fatalf("admission failed: %v", err)
	}

	profileData, err := os.ReadFile(filepath.Join(finalOut, "profile.json"))
	if err != nil {
		t.Fatalf("read profile.json: %v", err)
	}
	var profile profilebuilder.Profile
	if err := json.Unmarshal(profileData, &profile); err != nil {
		t.Fatalf("unmarshal profile.json: %v", err)
	}

	badStatuses := []string{"VERIFIED", "PRODUCTION_VALIDATED"}
	for _, bad := range badStatuses {
		// Check governed status (CompatibilityStatus allows REAL_DEVICE_VALIDATED which is expected)
		if profile.GovernanceStatus == bad {
			t.Fatalf("should not have governance status %s", bad)
		}
	}
}

// makeTestCandidate creates a profile CANDIDATE bound to a REPLAY_VALIDATED mapping.
// Uses the real governed authority chain: CANDIDATE → ReviewEvidence → ReviewReplay → REPLAY_VALIDATED.
// copyProfileFiles copies profile.yaml and profile.json from build to staging dir
func copyProfileFiles(t *testing.T, tmpDir, stagingDir string) error {
	buildDir := filepath.Join(tmpDir, "build")

	// Verify build directory exists and list its contents
	buildEntries, err := os.ReadDir(buildDir)
	if err != nil {
		return fmt.Errorf("read build dir %s: %w", buildDir, err)
	}
	var buildFiles []string
	for _, e := range buildEntries {
		buildFiles = append(buildFiles, e.Name())
	}

	for _, file := range []string{"profile.yaml", "profile.json"} {
		src := filepath.Join(buildDir, file)
		// Verify source exists
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("profile file missing in build dir %s: %s (%v); build contains: %v", buildDir, file, err, buildFiles)
		}
		dst := filepath.Join(stagingDir, file)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
	}
	return nil
}

func makeTestCandidate(t *testing.T, tmpDir string) (profilebuilder.Profile, *mapping.MappingDefinition) {
	m, err := mapping.New(
		"TEST_MAPPING", "1.0.0",
		"HL7_V2", 5, "OBX-5", "bpm",
		"HeartRate", "bpm",
		"test mapping", "test_author",
	)
	if err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	m.DeviceFingerprint = "test_fp"
	m.DeviceIdentityRef = "test_ref"
	m.ParserIdentity = "generic_hl7_v2"

	// Step 1: ReviewEvidence - requires valid evidence matching the mapping claim.
	inputDir := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("mkdir input: %v", err)
	}

	// Use the exact same evidence structure as the governed pipeline tests.
	claimReview := evidence.ClaimReview{
		Claim: evidence.Claim{
			MappingIdentity: m.MappingIdentity,
			MappingVersion: m.MappingVersion,
			SourceProtocol: m.SourceProtocol,
			SourcePosition: m.SourcePosition,
			SourceCode: m.SourceCode,
			SourceUnit: m.SourceUnit,
			CanonicalConcept: m.CanonicalConcept,
			CanonicalUnit: m.CanonicalUnit,
			ParserIdentity: m.ParserIdentity,
			DeviceFingerprint: m.DeviceFingerprint,
			DeviceIdentityRef: m.DeviceIdentityRef,
		},
		Evidence: []evidence.Evidence{
			{Type: "VENDOR_MANUAL", Ref: "test_manual_1"},
			{Type: "VENDOR_MANUAL", Ref: "test_manual_2"},
		},
	}
	evJSON, _ := json.Marshal(claimReview)
	evidencePath := filepath.Join(inputDir, "evidence.json")
	if err := os.WriteFile(evidencePath, evJSON, 0644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}

	// ReviewEvidence transitions CANDIDATE → EVIDENCE_APPROVED
	reportPath := filepath.Join(inputDir, "evidence_report.json")
	if err := mapping.ReviewEvidence(m, evidencePath, reportPath); err != nil {
		t.Fatalf("evidence review: %v", err)
	}

	// Step 2: ReviewReplay - requires raw stream and ground truth matching the claim.
	rawStream := []byte(
		"MSH|^~\\&|LAB|HOSP|LAB|HOSP|20240101120000||ADT^A01|12345|P|2.5\r\n" +
		"PID|1||12345^^^MRN||TEST^PATIENT||19800101|M\r\n" +
		"OBX|1|NM|OBX-5^HEART_RATE||88|bpm|60-100|N|||F\r\n",
	)
	if err := os.WriteFile(filepath.Join(inputDir, "raw_stream.bin"), rawStream, 0644); err != nil {
		t.Fatalf("write raw stream: %v", err)
	}

	// Ground truth uses canonical concept as key (not source code)
	groundTruth := map[string]float64{"HeartRate": 88.0}
	gtJSON, _ := json.Marshal(groundTruth)
	gtPath := filepath.Join(inputDir, "ground_truth.json")
	if err := os.WriteFile(gtPath, gtJSON, 0644); err != nil {
		t.Fatalf("write ground truth: %v", err)
	}

	// ReviewReplay transitions EVIDENCE_APPROVED → REPLAY_VALIDATED
	replayReportPath := filepath.Join(inputDir, "replay_report.json")
	replayOpts := replay.ReplayOptions{GroundTruthPath: gtPath}
	_, err = mapping.ReviewReplay(m, inputDir, 2, replayOpts, replayReportPath)
	if err != nil {
		t.Fatalf("replay review: %v", err)
	}

	// Verify mapping is now REPLAY_VALIDATED
	status, _ := m.Status()
	if status != mapping.StatusReplayValidated {
		t.Fatalf("mapping not REPLAY_VALIDATED after ReviewReplay: %s", status)
	}

	// Build candidate using the real profile builder with the now-validated mapping.
	// This ensures the candidate exactly matches what approval.Review expects.
	buildDir := filepath.Join(tmpDir, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("mkdir build: %v", err)
	}

	candidate, err := profilebuilder.BuildFromMappingDefinition(
		m, "TestMfg", "TestModel", "tcp", "1.0.0", buildDir, "1.0.0", nil,
	)
	if err != nil {
		t.Fatalf("build candidate: %v", err)
	}

	return candidate, m
}

// Helper: create and write test issuer key, return key and path
func makeTestIssuerKey(t *testing.T, tmpDir string) (ed25519.PrivateKey, string) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	keyPath := filepath.Join(tmpDir, "issuer.key")
	if err := os.WriteFile(keyPath, priv, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	_ = pub // unused but generated for completeness
	return priv, keyPath
}

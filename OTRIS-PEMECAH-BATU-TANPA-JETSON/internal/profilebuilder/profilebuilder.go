package profilebuilder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
)

type Profile struct {
	ProfileID              string         `yaml:"profile_id" json:"profile_id"`
	Version                string         `yaml:"version" json:"version"`
	Status                 string         `yaml:"status" json:"status"`
	Manufacturer           string         `yaml:"manufacturer" json:"manufacturer"`
	Model                  string         `yaml:"model" json:"model"`
	FirmwareTested         string         `yaml:"firmware_tested" json:"firmware_tested"`
	Transport              string         `yaml:"transport" json:"transport"`
	Framing                string         `yaml:"framing" json:"framing"`
	ProtocolFamily         string         `yaml:"protocol_family" json:"protocol_family"`
	ParserIdentity         string         `yaml:"parser_identity" json:"parser_identity"`
	DeviceFingerprint      string         `yaml:"device_fingerprint" json:"device_fingerprint"`
	DeviceIdentityRef      string         `yaml:"device_identity_ref" json:"device_identity_ref"`
	Mapping                []FieldMapping `yaml:"mapping_profile" json:"mapping_profile"`
	MappingIdentity        string         `yaml:"mapping_identity" json:"mapping_identity"`
	MappingVersion         string         `yaml:"mapping_version" json:"mapping_version"`
	MappingProvenance      string         `yaml:"mapping_provenance" json:"mapping_provenance"`
	EvidenceReference      string         `yaml:"evidence_reference" json:"evidence_reference"`
	ReplayReference        string         `yaml:"replay_reference" json:"replay_reference"`
	ValidationStatus       string         `yaml:"validation_status" json:"validation_status"`
	ValidatedAt            string         `yaml:"validated_at" json:"validated_at"`
	FalseDataRate          float64        `yaml:"false_data_rate" json:"false_data_rate"`
	KnownLimitations       []string       `yaml:"known_limitations" json:"known_limitations"`
	AuthorityChain         []string       `yaml:"authority_chain" json:"authority_chain"`
	NormalizationAuthority bool           `yaml:"normalization_authority" json:"normalization_authority"`
	NormalizerIdentity     string         `yaml:"normalizer_identity" json:"normalizer_identity,omitempty"`
}

type FieldMapping struct {
	SourcePos    int      `yaml:"source_pos" json:"source_pos"`
	SourceCode   string   `yaml:"source_code" json:"source_code"`
	SourceUnit   string   `yaml:"source_unit" json:"source_unit"`
	Canonical    string   `yaml:"canonical" json:"canonical"`
	Unit         string   `yaml:"unit" json:"unit"`
	Confidence   float64  `yaml:"confidence" json:"confidence"`
	EvidenceRefs []string `yaml:"evidence_refs" json:"evidence_refs"`
}

// BuildOptions specifies required inputs for profile generation
type BuildOptions struct {
	Manufacturer       string
	Model              string
	FirmwareTested     string
	Transport          string
	EvidenceReportPath string
	ReplayReportPath   string
	MappingIdentity    string
	MappingVersion     string
	MappingProvenance  string
}

// GOVERNED authority path: requires REPLAY_VALIDATED MappingDefinition.
// Only this function produces an Integration Profile Candidate with established authority chain.
func BuildFromMappingDefinition(m *mapping.MappingDefinition, manufacturer, model, transport, firmwareTested, outDir, version string) (Profile, error) {
	emptyProfile := Profile{}
	if m == nil {
		return emptyProfile, errors.New("MappingDefinition required")
	}
	status, err := m.Status()
	if err != nil || status != mapping.StatusReplayValidated {
		return emptyProfile, fmt.Errorf("MappingDefinition must be REPLAY_VALIDATED, got %s: %v", status, err)
	}
	// Validate integrity: claim unchanged, authority binding intact.
	if err := m.Validate(); err != nil {
		return emptyProfile, fmt.Errorf("MappingDefinition authority invalid: %w", err)
	}

	// Device/parser context required for integration profile.
	claim := m.Claim()
	if strings.TrimSpace(claim.ParserIdentity) == "" || strings.TrimSpace(claim.DeviceFingerprint) == "" || strings.TrimSpace(claim.DeviceIdentityRef) == "" {
		return emptyProfile, errors.New("ParserIdentity, DeviceFingerprint, and DeviceIdentityRef required")
	}

	// Device identity supplied and required.
	for _, field := range []struct{ name, value string }{
		{"manufacturer", manufacturer}, {"model", model}, {"transport", transport}, {"firmware_tested", firmwareTested},
	} {
		if strings.TrimSpace(field.value) == "" || !utf8.ValidString(field.value) {
			return emptyProfile, fmt.Errorf("missing or invalid %s", field.name)
		}
	}

	if version == "" {
		version = "1.0.0"
	}

	// Derive all mapping semantics from authenticated claim; no hardcode.
	// Normalization authority requires explicit governed fields.
	normalizationAuthority := strings.TrimSpace(m.NormalizerIdentity) != ""
	if normalizationAuthority {
		if strings.TrimSpace(m.NormalizerIdentity) == "" || !utf8.ValidString(m.NormalizerIdentity) {
			return emptyProfile, errors.New("NormalizerIdentity is invalid or missing")
		}
	}

	// Convert evidence references to strings for per-entry evidence tracking.
	evidenceRefStrings := make([]string, 0, len(m.EvidenceRefs))
	for _, ref := range m.EvidenceRefs {
		if strings.TrimSpace(ref.Ref) != "" {
			evidenceRefStrings = append(evidenceRefStrings, ref.Ref)
		}
	}

	// If normalization_authority=true, require evidence_refs.
	if normalizationAuthority && len(evidenceRefStrings) == 0 {
		return emptyProfile, errors.New("normalization authority requires at least one evidence reference")
	}

	profile := Profile{
		ProfileID:         fmt.Sprintf("%s_%s_%s_v%s", manufacturer, model, firmwareTested, version),
		Version:           version,
		Status:            "CANDIDATE",
		Manufacturer:      manufacturer,
		Model:             model,
		FirmwareTested:    firmwareTested,
		Transport:         transport,
		Framing:           "", // MappingDefinition supplies no framing authority.
		ProtocolFamily:    claim.SourceProtocol,
		ParserIdentity:    claim.ParserIdentity,
		DeviceFingerprint: claim.DeviceFingerprint,
		DeviceIdentityRef: claim.DeviceIdentityRef,
		Mapping: []FieldMapping{{
			SourcePos:    claim.SourcePosition,
			SourceCode:   claim.SourceCode,
			SourceUnit:   claim.SourceUnit,
			Canonical:    claim.CanonicalConcept,
			Unit:         claim.CanonicalUnit,
			EvidenceRefs: evidenceRefStrings,
		}},
		MappingIdentity:        claim.MappingIdentity,
		MappingVersion:         claim.MappingVersion,
		MappingProvenance:      m.Provenance,
		EvidenceReference:      m.EvidenceGateReportRef,
		ReplayReference:        "", // No report path exposed by MappingDefinition; do not invent one.
		ValidationStatus:       "CANDIDATE",
		ValidatedAt:            m.ReplayValidatedAt.Format("2006-01-02"),
		FalseDataRate:          0.0, // REPLAY_VALIDATED guarantees false_data==0.
		KnownLimitations:       []string{},
		NormalizationAuthority: normalizationAuthority,
		NormalizerIdentity:     m.NormalizerIdentity,
		AuthorityChain: []string{
			fmt.Sprintf("Evidence approved: %s", m.EvidenceGateReportRef),
			fmt.Sprintf("Replay validated: false_data=0.0 at %s", m.ReplayValidatedAt.Format(time.RFC3339Nano)),
			fmt.Sprintf("Mapping: %s v%s by %s", claim.MappingIdentity, claim.MappingVersion, m.ProvenanceAuthor),
			fmt.Sprintf("Parser: %s", claim.ParserIdentity),
		},
	}

	jsonData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return emptyProfile, fmt.Errorf("encode profile: %w", err)
	}

	var yamlData strings.Builder
	for _, field := range []struct{ name, value string }{
		{"profile_id", profile.ProfileID},
		{"version", profile.Version},
		{"status", profile.Status},
		{"manufacturer", profile.Manufacturer},
		{"model", profile.Model},
		{"firmware_tested", profile.FirmwareTested},
		{"transport", profile.Transport},
		{"framing", profile.Framing},
		{"protocol_family", profile.ProtocolFamily},
		{"parser_identity", profile.ParserIdentity},
		{"device_fingerprint", profile.DeviceFingerprint},
		{"device_identity_ref", profile.DeviceIdentityRef},
		{"mapping_identity", profile.MappingIdentity},
		{"mapping_version", profile.MappingVersion},
		{"mapping_provenance", profile.MappingProvenance},
		{"evidence_reference", profile.EvidenceReference},
		{"validation_status", profile.ValidationStatus},
		{"validated_at", profile.ValidatedAt},
		{"normalizer_identity", profile.NormalizerIdentity},
	} {
		value, err := json.Marshal(field.value)
		if err != nil {
			return emptyProfile, fmt.Errorf("encode %s: %w", field.name, err)
		}
		fmt.Fprintf(&yamlData, "%s: %s\n", field.name, value)
	}
	fmt.Fprintf(&yamlData, "normalization_authority: %v\nfalse_data_rate: %.1f\nauthority_chain:\n", profile.NormalizationAuthority, profile.FalseDataRate)
	for _, authority := range profile.AuthorityChain {
		value, err := json.Marshal(authority)
		if err != nil {
			return emptyProfile, fmt.Errorf("encode authority chain: %w", err)
		}
		fmt.Fprintf(&yamlData, "  - %s\n", value)
	}
	// JSON flow values are valid YAML; preserve the existing mapping_profile.fields shape.
	fieldsJSON, err := json.Marshal(profile.Mapping)
	if err != nil {
		return emptyProfile, fmt.Errorf("encode field mappings: %w", err)
	}
	fmt.Fprintf(&yamlData, "mapping_profile:\n  fields: %s\n", fieldsJSON)

	if err := writeProfiles(outDir, []byte(yamlData.String()), jsonData); err != nil {
		return emptyProfile, err
	}

	fmt.Printf("[PB-13] GOVERNED PROFILE BUILDER succeeded: %s\n", filepath.Join(outDir, "profile.yaml"))
	fmt.Printf("  Authority: MappingDefinition %s/%s REPLAY_VALIDATED\n", claim.MappingIdentity, claim.MappingVersion)
	fmt.Printf("  Status: CANDIDATE (requires engineer approval)\n")
	return profile, nil
}

// LEGACY non-authoritative path: preserved for backward compatibility only.
// This path does NOT produce a governed Integration Profile Candidate.
// Callers must use BuildFromMappingDefinition for authoritative profiles.
func Build(inDir string, outDir string, version string) (Profile, error) {
	return BuildWithEvidence(inDir, outDir, version, BuildOptions{})
}

// BuildWithEvidence is legacy report compatibility, not an authority path.
// Loose reports can produce only LEGACY_UNTRUSTED output, never a governed candidate.
func BuildWithEvidence(inDir string, outDir string, version string, opts BuildOptions) (Profile, error) {
	emptyProfile := Profile{}

	// Supplied metadata establishes provenance, not proof of mapping semantics.
	for _, field := range []struct{ name, value string }{
		{"mapping identity", opts.MappingIdentity},
		{"mapping version", opts.MappingVersion},
		{"mapping provenance", opts.MappingProvenance},
	} {
		if strings.TrimSpace(field.value) == "" || !utf8.ValidString(field.value) {
			return emptyProfile, fmt.Errorf("missing or invalid %s", field.name)
		}
	}

	if version == "" {
		version = "1.0.0"
	}

	// FAIL CLOSED: Evidence Gate result required
	evidenceReportPath := opts.EvidenceReportPath
	if evidenceReportPath == "" {
		evidenceReportPath = filepath.Join(inDir, "evidence_gate_report.json")
	}
	evidenceReport := readJSON(evidenceReportPath)
	if evidenceReport == nil {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Evidence Gate report not found at %s\n", evidenceReportPath)
		return emptyProfile, fmt.Errorf("missing evidence gate report")
	}

	// FAIL CLOSED: Evidence Gate verdict must be APPROVED_FOR_MAPPING
	verdict, verdictExists := evidenceReport["verdict"].(string)
	if !verdictExists || verdict != "APPROVED_FOR_MAPPING" {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Evidence Gate verdict not APPROVED_FOR_MAPPING (got: %v)\n", verdict)
		return emptyProfile, fmt.Errorf("evidence gate not approved for mapping")
	}

	// FAIL CLOSED: Replay result required
	replayReportPath := opts.ReplayReportPath
	if replayReportPath == "" {
		replayReportPath = filepath.Join(inDir, "replay_report_1000.json")
		if _, err := os.Stat(replayReportPath); err != nil {
			// Try glob if default path not found
			files, _ := filepath.Glob(filepath.Join(inDir, "replay_report_*.json"))
			if len(files) > 0 {
				replayReportPath = files[0]
			}
		}
	}
	replayReport := readJSON(replayReportPath)
	if replayReport == nil {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Replay report not found at %s\n", replayReportPath)
		return emptyProfile, fmt.Errorf("missing replay report")
	}

	// FAIL CLOSED: Replay verdict must be READY_FOR_PROFILE_BUILDER
	replayVerdict, replayVerdictExists := replayReport["verdict"].(string)
	if !replayVerdictExists || replayVerdict != "READY_FOR_PROFILE_BUILDER" {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Replay verdict not READY_FOR_PROFILE_BUILDER (got: %v)\n", replayVerdict)
		return emptyProfile, fmt.Errorf("replay not ready for profile builder")
	}

	// FAIL CLOSED: Replay false_data_rate must be 0
	falseDataRate, falseDataExists := replayReport["false_data_rate"].(float64)
	if !falseDataExists || falseDataRate != 0.0 {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Replay false_data_rate not 0 (got: %v)\n", falseDataRate)
		return emptyProfile, fmt.Errorf("replay detected false data")
	}

	// FAIL CLOSED: Required device identity must be supplied
	if opts.Manufacturer == "" {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Manufacturer identity not supplied\n")
		return emptyProfile, fmt.Errorf("missing manufacturer identity")
	}
	if opts.Model == "" {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Model identity not supplied\n")
		return emptyProfile, fmt.Errorf("missing model identity")
	}
	if opts.Transport == "" {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Transport not supplied\n")
		return emptyProfile, fmt.Errorf("missing transport identity")
	}
	if opts.FirmwareTested == "" {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Firmware version not supplied\n")
		return emptyProfile, fmt.Errorf("missing firmware version")
	}

	// Read protocol classification from evidence
	classification := readJSON(filepath.Join(inDir, "protocol_classification.json"))
	protoFamily := "UNKNOWN"
	if classification != nil {
		if v, ok := classification["protocol_family"]; ok {
			protoFamily = fmt.Sprintf("%v", v)
		}
	}

	// Read framing from evidence
	framing := readJSON(filepath.Join(inDir, "framing_candidates.json"))
	framingType := "UNKNOWN"
	if framing != nil {
		if v, ok := framing["selected"]; ok {
			framingType = fmt.Sprintf("%v", v)
		}
	}

	// Extract parser identity from replay report
	parserIdentity := "UNKNOWN"
	if parser, ok := replayReport["parser"].(string); ok {
		parserIdentity = parser
	}

	// FAIL CLOSED: Parser identity required
	if parserIdentity == "UNKNOWN" {
		fmt.Printf("[PB-13] PROFILE BUILD FAILED: Parser identity not found in replay report\n")
		return emptyProfile, fmt.Errorf("missing parser identity")
	}

	// Preserve legacy metadata without asserting evidence or replay authority.
	profile := Profile{
		ProfileID:         fmt.Sprintf("%s_%s_%s_v%s", opts.Manufacturer, opts.Model, opts.FirmwareTested, version),
		Version:           version,
		Status:            "LEGACY_UNTRUSTED",
		Manufacturer:      opts.Manufacturer,
		Model:             opts.Model,
		FirmwareTested:    opts.FirmwareTested,
		Transport:         opts.Transport,
		Framing:           framingType,
		ProtocolFamily:    protoFamily,
		ParserIdentity:    parserIdentity,
		Mapping:           []FieldMapping{}, // Metadata does not supply or validate field mappings.
		MappingIdentity:   opts.MappingIdentity,
		MappingVersion:    opts.MappingVersion,
		MappingProvenance: opts.MappingProvenance,
		EvidenceReference: evidenceReportPath,
		ReplayReference:   replayReportPath,
		ValidationStatus:  "LEGACY_UNTRUSTED",
		ValidatedAt:       "",
		FalseDataRate:     falseDataRate, // Untrusted report value, not measured proof.
		KnownLimitations:  []string{"Loose reports are not authority; use BuildFromMappingDefinition."},
		AuthorityChain:    []string{},
	}

	jsonData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return emptyProfile, fmt.Errorf("encode profile: %w", err)
	}

	// JSON-quoted strings are valid YAML scalars and preserve supplied metadata verbatim.
	var yamlData strings.Builder
	for _, field := range []struct{ name, value string }{
		{"profile_id", profile.ProfileID},
		{"version", profile.Version},
		{"status", profile.Status},
		{"manufacturer", profile.Manufacturer},
		{"model", profile.Model},
		{"firmware_tested", profile.FirmwareTested},
		{"transport", profile.Transport},
		{"framing", profile.Framing},
		{"protocol_family", profile.ProtocolFamily},
		{"parser_identity", profile.ParserIdentity},
		{"device_fingerprint", profile.DeviceFingerprint},
		{"device_identity_ref", profile.DeviceIdentityRef},
		{"mapping_identity", profile.MappingIdentity},
		{"mapping_version", profile.MappingVersion},
		{"mapping_provenance", profile.MappingProvenance},
		{"evidence_reference", profile.EvidenceReference},
		{"replay_reference", profile.ReplayReference},
		{"validation_status", profile.ValidationStatus},
		{"validated_at", profile.ValidatedAt},
	} {
		value, err := json.Marshal(field.value)
		if err != nil {
			return emptyProfile, fmt.Errorf("encode %s: %w", field.name, err)
		}
		fmt.Fprintf(&yamlData, "%s: %s\n", field.name, value)
	}
	fmt.Fprintf(&yamlData, "false_data_rate: %.4f\nauthority_chain: []\n", profile.FalseDataRate)
	yamlData.WriteString("mapping_profile:\n  fields: []\n")

	if err := writeProfiles(outDir, []byte(yamlData.String()), jsonData); err != nil {
		return emptyProfile, err
	}

	fmt.Printf("[PB-13] LEGACY_UNTRUSTED output: %s (no governed authority)\n", filepath.Join(outDir, "profile.yaml"))
	return profile, nil
}

// writeProfiles never overwrites existing artifacts; failed writes remove only newly created files.
func writeProfiles(outDir string, yamlData, jsonData []byte) (err error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}
	var created []string
	defer func() {
		if err != nil {
			for _, path := range created {
				err = errors.Join(err, os.Remove(path))
			}
		}
	}()
	for _, output := range []struct {
		name string
		data []byte
	}{
		{"profile.yaml", yamlData},
		{"profile.json", jsonData},
	} {
		path := filepath.Join(outDir, output.name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return fmt.Errorf("create %s: %w", output.name, err)
		}
		created = append(created, path)
		_, writeErr := f.Write(output.data)
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return fmt.Errorf("write %s: %w", output.name, err)
		}
	}
	return nil
}

func readJSON(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return m
}

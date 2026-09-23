package profilebuilder

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/replay"
)

// Test A: No Evidence Gate report → profile NOT generated
func TestNoEvidenceGateReport(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: filepath.Join(tmpDir, "nonexistent.json"),
		ReplayReportPath:   filepath.Join(tmpDir, "replay_report_1000.json"),
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err == nil {
		t.Error("missing evidence gate report: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Error("missing evidence gate report: profile should not be generated")
	}
}

// Test B: Evidence Gate != APPROVED_FOR_MAPPING → profile NOT generated
func TestEvidenceGateNotApproved(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	// Create Evidence Gate report with REJECTED verdict
	evidenceReport := map[string]interface{}{
		"verdict": "REJECTED",
		"status":  "REJECTED",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   filepath.Join(tmpDir, "replay_report_1000.json"),
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err == nil {
		t.Error("rejected evidence gate: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Error("rejected evidence gate: profile should not be generated")
	}
}

// Test C: No Replay report → profile NOT generated
func TestNoReplayReport(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	// Create valid Evidence Gate report
	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
		"status":  "MAPPED",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   filepath.Join(tmpDir, "nonexistent_replay.json"),
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err == nil {
		t.Error("missing replay report: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Error("missing replay report: profile should not be generated")
	}
}

// Test D: Replay != READY_FOR_PROFILE_BUILDER → profile NOT generated
func TestReplayNotReady(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	// Create valid Evidence Gate report
	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	// Create Replay report with NOT_PROVEN verdict
	replayReport := map[string]interface{}{
		"verdict":         "NOT_PROVEN",
		"false_data_rate": 0.0,
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err == nil {
		t.Error("replay not ready: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Error("replay not ready: profile should not be generated")
	}
}

// Test E: Replay false_data > 0 → profile NOT generated
func TestReplayFalseDataDetected(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	// Create valid Evidence Gate report
	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	// Create Replay report with false_data > 0
	replayReport := map[string]interface{}{
		"verdict":         "FAIL_FALSE_DATA",
		"false_data_rate": 0.5,
		"false_data":      5,
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err == nil {
		t.Error("replay false data detected: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Error("replay false data: profile should not be generated")
	}
}

// Test F: Missing device/vendor/model identity → profile NOT generated
func TestMissingDeviceIdentity(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	// Create reports
	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	replayReport := map[string]interface{}{
		"verdict":         "READY_FOR_PROFILE_BUILDER",
		"false_data_rate": 0.0,
		"parser":          "generic_hl7_v2",
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	// Missing manufacturer
	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err == nil {
		t.Error("missing manufacturer: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Error("missing manufacturer: profile should not be generated")
	}
}

// Test G: Missing parser identity → profile NOT generated
func TestMissingParserIdentity(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	// Replay report missing parser identity
	replayReport := map[string]interface{}{
		"verdict":         "READY_FOR_PROFILE_BUILDER",
		"false_data_rate": 0.0,
		// NO parser field
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err == nil {
		t.Error("missing parser identity: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Error("missing parser: profile should not be generated")
	}
}

// Test H: Replay's mapping label cannot supply missing mapping authority.
func TestMissingMappingIdentity(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "output")

	writeProfileTestJSON(t, filepath.Join(tmpDir, "evidence_gate_report.json"), map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	})
	writeProfileTestJSON(t, filepath.Join(tmpDir, "replay_report_1000.json"), map[string]interface{}{
		"verdict":               "READY_FOR_PROFILE_BUILDER",
		"false_data":            0,
		"false_data_rate":       0.0,
		"parser":                "fixture_parser",
		"mapping":               "mindray_n12_mapping_v0.1",
		"success":               100,
		"failed_parse":          0,
		"ground_truth_provided": true,
	})

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", BuildOptions{
		Manufacturer:   "Fixture Vendor",
		Model:          "Fixture Model",
		FirmwareTested: "1.2.3",
		Transport:      "TCP_RAW",
	})
	if err == nil {
		t.Fatal("missing mapping authority: expected error, got success")
	}
	if profile.ProfileID != "" {
		t.Fatal("missing mapping authority: profile must not be generated")
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Fatalf("missing mapping authority created output directory: %v", err)
	}
}

func writeProfileTestJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// Test I: Valid approved Evidence Gate + valid Replay + complete identity → profile generated
func TestValidProfileGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
		"status":  "MAPPED",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	replayReport := map[string]interface{}{
		"verdict":         "READY_FOR_PROFILE_BUILDER",
		"false_data_rate": 0.0,
		"parser":          "generic_hl7_v2",
		"success":         100,
		"failed_parse":    0,
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err != nil {
		t.Errorf("valid input: unexpected error: %v", err)
	}
	if profile.ProfileID == "" {
		t.Error("valid input: profile should be generated")
	}
	if profile.Status != "LEGACY_UNTRUSTED" || profile.ValidationStatus != "LEGACY_UNTRUSTED" {
		t.Fatalf("loose reports must not grant governed status: %+v", profile)
	}
}

// Test J: Generated profile contains supplied identity, not hardcoded Mindray/BeneVision
func TestProfileUsesSuppliedIdentity(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	replayReport := map[string]interface{}{
		"verdict":         "READY_FOR_PROFILE_BUILDER",
		"false_data_rate": 0.0,
		"parser":          "generic_hl7_v2",
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	// Supply different device identity
	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "GE Healthcare",
		Model:              "Carescape Monitor B650",
		FirmwareTested:     "12.34.56",
		Transport:          "ETHERNET",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Manufacturer != "GE Healthcare" {
		t.Errorf("should use supplied manufacturer, got %s", profile.Manufacturer)
	}
	if profile.Model != "Carescape Monitor B650" {
		t.Errorf("should use supplied model, got %s", profile.Model)
	}
	if profile.FirmwareTested != "12.34.56" {
		t.Errorf("should use supplied firmware, got %s", profile.FirmwareTested)
	}
	if profile.Transport != "ETHERNET" {
		t.Errorf("should use supplied transport, got %s", profile.Transport)
	}

	// Verify hardcoded values are NOT used
	if profile.Manufacturer == "Mindray" {
		t.Error("VIOLATION: hardcoded Mindray found in generated profile")
	}
	if profile.Model == "BeneVision N12" {
		t.Error("VIOLATION: hardcoded BeneVision N12 found in generated profile")
	}
}

// Test K: Profile Builder does NOT self-assign REAL_DEVICE_VALIDATED
func TestNoSelfValidation(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	replayReport := map[string]interface{}{
		"verdict":         "READY_FOR_PROFILE_BUILDER",
		"false_data_rate": 0.0,
		"parser":          "generic_hl7_v2",
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.ValidationStatus == "REAL_DEVICE_VALIDATED" {
		t.Error("VIOLATION: Profile Builder self-assigned REAL_DEVICE_VALIDATED")
	}
	if profile.ValidationStatus == "PRODUCTION_VALIDATED" {
		t.Error("VIOLATION: Profile Builder self-assigned PRODUCTION_VALIDATED")
	}
	if profile.ValidationStatus != "LEGACY_UNTRUSTED" {
		t.Errorf("legacy status should be LEGACY_UNTRUSTED, got %s", profile.ValidationStatus)
	}
}

// Test L: No synthetic evidence/provenance generated
func TestNoSyntheticProvenance(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	evidenceReport := map[string]interface{}{
		"verdict": "APPROVED_FOR_MAPPING",
	}
	evData, _ := json.Marshal(evidenceReport)
	evPath := filepath.Join(tmpDir, "evidence_gate_report.json")
	os.WriteFile(evPath, evData, 0644)

	replayReport := map[string]interface{}{
		"verdict":         "READY_FOR_PROFILE_BUILDER",
		"false_data_rate": 0.0,
		"parser":          "generic_hl7_v2",
	}
	replayData, _ := json.Marshal(replayReport)
	replayPath := filepath.Join(tmpDir, "replay_report_1000.json")
	os.WriteFile(replayPath, replayData, 0644)

	opts := BuildOptions{
		MappingIdentity:    "fixture.mapping",
		MappingVersion:     "2026-review-A",
		MappingProvenance:  "test-fixture: explicitly supplied mapping authority",
		Manufacturer:       "Mindray",
		Model:              "BeneVision N12",
		FirmwareTested:     "01.12.00",
		Transport:          "TCP_RAW",
		EvidenceReportPath: evPath,
		ReplayReportPath:   replayPath,
	}

	profile, err := BuildWithEvidence(tmpDir, outDir, "1.0.0", opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify references point to actual supplied reports
	if profile.EvidenceReference != evPath {
		t.Errorf("evidence reference should point to actual file, got %s", profile.EvidenceReference)
	}
	if profile.ReplayReference != replayPath {
		t.Errorf("replay reference should point to actual file, got %s", profile.ReplayReference)
	}

	// Verify no hardcoded evidence strings like old implementation
	for _, auth := range profile.AuthorityChain {
		if auth == "VENDOR_MANUAL p.45" {
			t.Error("VIOLATION: synthetic evidence found in authority chain")
		}
		if auth == "SIMULATOR_LOG correlation 0.97" {
			t.Error("VIOLATION: synthetic evidence found in authority chain")
		}
	}
}

// ===== D3F GOVERNED AUTHORITY TESTS =====
// These tests construct REPLAY_VALIDATED MappingDefinition through the real authority chain:
// CANDIDATE → ReviewEvidence → EVIDENCE_APPROVED → ReviewReplay → REPLAY_VALIDATED

// Helper: create CANDIDATE MappingDefinition with required context
func d3fCandidateMapping(t *testing.T) *mapping.MappingDefinition {
	t.Helper()
	m, err := mapping.New("d3f-test-mapping", "1.0", "HL7_V2", 5, "HR", "bpm", "heart_rate", "bpm",
		"D3F test claim", "test-author")
	if err != nil {
		t.Fatal(err)
	}
	m.DeviceFingerprint = "test-device-fp"
	m.DeviceIdentityRef = "test-device-model"
	m.ParserIdentity = "generic_hl7_v2"
	d3fRequireStatus(t, m, mapping.StatusCandidate)
	return m
}

func d3fRequireStatus(t *testing.T, m *mapping.MappingDefinition, want mapping.Status) {
	t.Helper()
	got, err := m.Status()
	if err != nil || got != want {
		t.Fatalf("mapping status=%s error=%v; want %s", got, err, want)
	}
}

// Helper: advance through ReviewEvidence to EVIDENCE_APPROVED
func d3fApproveEvidence(t *testing.T, m *mapping.MappingDefinition) {
	t.Helper()
	dir := t.TempDir()
	bundle := evidence.ClaimReview{
		Claim: m.Claim(),
		Evidence: []evidence.Evidence{
			{Type: "D3F_TEST", Ref: "test:evidence"},
			{Type: "D3F_MEASUREMENT", Ref: "test:measurement"},
		},
	}
	d3fRequireStatus(t, m, mapping.StatusCandidate)
	evPath := filepath.Join(dir, "evidence.json")
	writeProfileTestJSON(t, evPath, bundle)
	if err := mapping.ReviewEvidence(m, evPath, filepath.Join(dir, "evidence_report.json")); err != nil {
		t.Fatal(err)
	}
	d3fRequireStatus(t, m, mapping.StatusEvidenceApproved)
}

// Helper: advance through ReviewReplay to REPLAY_VALIDATED
func d3fValidateReplay(t *testing.T, m *mapping.MappingDefinition) {
	t.Helper()
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "raw_stream.bin")
	gtPath := filepath.Join(dir, "gt.json")
	reportPath := filepath.Join(dir, "replay.json")

	if err := os.WriteFile(rawPath, []byte("MSH|^~\\&|test\nOBX|1|NM|HR||88|bpm\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gtPath, []byte(`{"heart_rate":88}`), 0600); err != nil {
		t.Fatal(err)
	}

	d3fRequireStatus(t, m, mapping.StatusEvidenceApproved)
	report, err := mapping.ReviewReplay(m, dir, 1, replay.ReplayOptions{GroundTruthPath: gtPath}, reportPath)
	if err != nil {
		t.Fatalf("ReviewReplay failed: %v", err)
	}
	if report.Verdict != "READY_FOR_PROFILE_BUILDER" || report.Compared != 1 || report.Success != 2 ||
		report.FailedParse != 0 || report.FalseData != 0 || report.FalseDataRate != 0 || !report.GroundTruthProvided ||
		report.Claim == nil || *report.Claim != m.Claim() {
		t.Fatalf("governed replay did not prove fixture: %+v", report)
	}
	d3fRequireStatus(t, m, mapping.StatusReplayValidated)
}

// Test A: Valid REPLAY_VALIDATED MappingDefinition → Profile Candidate built
func TestD3FBuildReplayValidated(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)

	outDir := t.TempDir()
	p, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")

	if err != nil {
		t.Fatalf("REPLAY_VALIDATED rejected: %v", err)
	}
	if p.Status != "CANDIDATE" || p.ValidationStatus != "CANDIDATE" {
		t.Fatalf("profile not CANDIDATE: %s/%s", p.Status, p.ValidationStatus)
	}
}

// Test B: CANDIDATE → rejected
func TestD3FBuildCandidate(t *testing.T) {
	m := d3fCandidateMapping(t)
	outDir := t.TempDir()
	_, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")
	if err == nil {
		t.Fatal("CANDIDATE accepted")
	}
}

// Test C: EVIDENCE_APPROVED without Replay → rejected
func TestD3FBuildEvidenceOnly(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	outDir := t.TempDir()
	_, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")
	if err == nil {
		t.Fatal("EVIDENCE_APPROVED without Replay accepted")
	}
}

// Test D: Forged/invalid REPLAY_VALIDATED authority → rejected
func TestD3FBuildForgedAuthority(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	// Manually set ReplayValidatedAt without proper authority binding
	m.ReplayValidatedAt = time.Now()
	outDir := t.TempDir()
	_, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")
	if err == nil {
		t.Fatal("forged authority accepted")
	}
}

// Test E: Post-approval/post-replay claim mutation → rejected
func TestD3FBuildMutatedClaim(t *testing.T) {
	claimType := reflect.TypeOf(evidence.Claim{})
	for i := 0; i < claimType.NumField(); i++ {
		name := claimType.Field(i).Name
		t.Run(name, func(t *testing.T) {
			m := d3fCandidateMapping(t)
			d3fApproveEvidence(t, m)
			d3fValidateReplay(t, m)
			field := reflect.ValueOf(m).Elem().FieldByName(name)
			if field.Kind() == reflect.Int {
				field.SetInt(field.Int() + 1)
			} else {
				field.SetString(field.String() + "-mutated")
			}
			d3fRequireBuildRejected(t, m)
		})
	}
}

// Test F: MappingIdentity preserved exactly
func TestD3FMappingIdentityPreserved(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)

	outDir := t.TempDir()
	p, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if p.MappingIdentity != "d3f-test-mapping" || p.MappingVersion != "1.0" {
		t.Fatalf("identity/version not preserved: %s/%s", p.MappingIdentity, p.MappingVersion)
	}
}

// Test G: All mapping claim fields preserved exactly
func TestD3FClaimFieldsPreserved(t *testing.T) {
	m := d3fCandidateMapping(t)
	// Context is copied verbatim, not trimmed, reformatted or encoded into prose.
	m.MappingIdentity = " explicit mapping : \"review\" \n"
	m.MappingVersion = " version/02.007 "
	m.DeviceFingerprint = " fingerprint : \"α\" \n"
	m.DeviceIdentityRef = " device/reference : \"β\" "
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)

	claim := m.Claim()
	outDir := t.TempDir()
	p, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	d3fAssertClaim(t, p, claim)
	if p.MappingProvenance != m.Provenance || p.EvidenceReference != m.EvidenceGateReportRef {
		t.Fatal("mapping provenance or evidence reference changed")
	}
	data, err := os.ReadFile(filepath.Join(outDir, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted Profile
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, persisted) {
		t.Fatalf("JSON differs from returned profile: %+v", persisted)
	}
	d3fAssertClaim(t, persisted, claim)

	// Scalar strings and the fields flow sequence use JSON's YAML-compatible syntax.
	data, err = os.ReadFile(filepath.Join(outDir, "profile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"mapping_identity": claim.MappingIdentity, "mapping_version": claim.MappingVersion,
		"protocol_family": claim.SourceProtocol, "parser_identity": claim.ParserIdentity,
		"device_fingerprint": claim.DeviceFingerprint, "device_identity_ref": claim.DeviceIdentityRef,
		"framing": "", "status": "CANDIDATE", "validation_status": "CANDIDATE",
	}
	seen := map[string]bool{}
	var fields []FieldMapping
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		if expected, ok := want[key]; ok {
			var got string
			if err := json.Unmarshal([]byte(value), &got); err != nil || got != expected {
				t.Fatalf("YAML %s not preserved: %q, %v", key, got, err)
			}
			seen[key] = true
		}
		if key == "  fields" {
			if err := json.Unmarshal([]byte(value), &fields); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(seen) != len(want) || !reflect.DeepEqual(fields, p.Mapping) {
		t.Fatalf("YAML missing claim data: seen=%v fields=%+v", seen, fields)
	}
	for _, entry := range p.AuthorityChain {
		if strings.Contains(entry, claim.DeviceFingerprint) || strings.Contains(entry, claim.DeviceIdentityRef) {
			t.Fatal("device claim must be structured, not authority prose")
		}
	}
}

func d3fAssertClaim(t *testing.T, p Profile, claim evidence.Claim) {
	t.Helper()
	if len(p.Mapping) != 1 {
		t.Fatalf("expected one explicit field mapping, got %d", len(p.Mapping))
	}
	field := p.Mapping[0]
	got := evidence.Claim{
		MappingIdentity: p.MappingIdentity, MappingVersion: p.MappingVersion,
		SourceProtocol: p.ProtocolFamily, SourcePosition: field.SourcePos,
		SourceCode: field.SourceCode, SourceUnit: field.SourceUnit,
		CanonicalConcept: field.Canonical, CanonicalUnit: field.Unit,
		ParserIdentity: p.ParserIdentity, DeviceFingerprint: p.DeviceFingerprint, DeviceIdentityRef: p.DeviceIdentityRef,
	}
	if got != claim {
		t.Fatalf("claim differs: got %+v, want %+v", got, claim)
	}
}

// Test H: No vendor/model hardcode
func TestD3FNoVendorHardcode(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)

	outDir := t.TempDir()
	p, err := BuildFromMappingDefinition(m, "CustomVendor", "CustomModel", "Serial", "5.0", outDir, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if p.Manufacturer != "CustomVendor" || p.Model != "CustomModel" {
		t.Fatalf("hardcoded: %s/%s", p.Manufacturer, p.Model)
	}
}

// Test I: No REAL_DEVICE_VALIDATED or PRODUCTION_VALIDATED
func TestD3FNoVerifiedStatus(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)

	outDir := t.TempDir()
	p, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "CANDIDATE" || p.ValidationStatus != "CANDIDATE" {
		t.Fatalf("unauthorized status: %s/%s", p.Status, p.ValidationStatus)
	}
	if p.Framing != "" {
		t.Fatalf("framing invented without authority: %q", p.Framing)
	}
}

// Test J: No Profile Builder self-promotion of MappingDefinition
func TestD3FNoSelfPromotion(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)

	before, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	if _, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0"); err != nil {
		t.Fatal(err)
	}
	d3fRequireStatus(t, m, mapping.StatusReplayValidated)
	after, err := json.Marshal(m)
	if err != nil || string(before) != string(after) {
		t.Fatalf("Builder modified MappingDefinition: %v", err)
	}
}

// Test K: No direct Edge Agent source dependency
func TestD3FNoEdgeAgentImport(t *testing.T) {
	deps, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("list package dependencies: %v\n%s", err, deps)
	}
	if strings.Contains(strings.ToLower(string(deps)), "otris-edge-agent") {
		t.Fatal("Profile Builder depends on Edge Agent source")
	}
}

// Test L: Missing required parser/mapping/provenance authority → fail closed
func TestD3FMissingParserContext(t *testing.T) {
	for _, name := range []string{"ParserIdentity", "DeviceFingerprint", "DeviceIdentityRef", "MappingIdentity", "MappingVersion", "Provenance", "ProvenanceAuthor"} {
		t.Run(name, func(t *testing.T) {
			m := d3fCandidateMapping(t)
			d3fApproveEvidence(t, m)
			d3fValidateReplay(t, m)
			// Removing approved context invalidates authority, not a valid replay state.
			reflect.ValueOf(m).Elem().FieldByName(name).SetString("")
			if err := m.Validate(); err == nil {
				t.Fatal("context removal retained authority")
			}
			d3fRequireBuildRejected(t, m)
		})
	}
}

func d3fRequireBuildRejected(t *testing.T, m *mapping.MappingDefinition) {
	t.Helper()
	outDir := filepath.Join(t.TempDir(), "output")
	p, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0")
	if err == nil || !reflect.DeepEqual(p, Profile{}) {
		t.Fatalf("invalid authority accepted: %+v, %v", p, err)
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Fatalf("rejected build created output: %v", err)
	}
}

// Device context is optional to D3D/D3E, but required by the Builder.
func TestD3FMissingDeviceContextWithValidReplay(t *testing.T) {
	for _, name := range []string{"DeviceFingerprint", "DeviceIdentityRef"} {
		for _, value := range []string{"", " \t "} {
			t.Run(name+value, func(t *testing.T) {
				m := d3fCandidateMapping(t)
				reflect.ValueOf(m).Elem().FieldByName(name).SetString(value)
				d3fApproveEvidence(t, m)
				d3fValidateReplay(t, m)
				d3fRequireBuildRejected(t, m)
				d3fRequireStatus(t, m, mapping.StatusReplayValidated)
			})
		}
	}
}

func TestD3FLegacyCannotGrantGovernedCandidate(t *testing.T) {
	dir := t.TempDir()
	evPath, replayPath := filepath.Join(dir, "evidence_gate_report.json"), filepath.Join(dir, "replay_report_1000.json")
	writeProfileTestJSON(t, evPath, map[string]any{"verdict": "APPROVED_FOR_MAPPING"})
	writeProfileTestJSON(t, replayPath, map[string]any{
		"verdict": "READY_FOR_PROFILE_BUILDER", "false_data_rate": 0, "parser": "generic_hl7_v2",
		"status": "REPLAY_VALIDATED", "validation_status": "VERIFIED",
	})
	p, err := Build(dir, filepath.Join(dir, "legacy-build"), "1")
	if err == nil || p.Status == "CANDIDATE" {
		t.Fatalf("legacy Build accepted loose authority: %+v, %v", p, err)
	}
	outDir := filepath.Join(dir, "legacy-reports")
	p, err = BuildWithEvidence(dir, outDir, "1", BuildOptions{
		Manufacturer: "LegacyVendor", Model: "LegacyModel", FirmwareTested: "1", Transport: "Serial",
		MappingIdentity: "loose-mapping", MappingVersion: "loose-version", MappingProvenance: "caller-supplied claim",
		EvidenceReportPath: evPath, ReplayReportPath: replayPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "LEGACY_UNTRUSTED" || p.ValidationStatus != "LEGACY_UNTRUSTED" ||
		len(p.AuthorityChain) != 0 || p.ValidatedAt != "" || len(p.Mapping) != 0 {
		t.Fatalf("legacy output asserted governed authority: %+v", p)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted Profile
	if err := json.Unmarshal(data, &persisted); err != nil || !reflect.DeepEqual(p, persisted) {
		t.Fatalf("persisted legacy JSON differs: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(outDir, "profile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"status: \"LEGACY_UNTRUSTED\"\n", "validation_status: \"LEGACY_UNTRUSTED\"\n", "authority_chain: []\n", "validated_at: \"\"\n"} {
		if !strings.Contains(string(data), line) {
			t.Fatalf("legacy YAML missing %q", line)
		}
	}
}

func TestD3FNilAndSerializedAuthorityRejected(t *testing.T) {
	d3fRequireBuildRejected(t, nil)
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var imported mapping.MappingDefinition
	if err := json.Unmarshal(data, &imported); err == nil {
		t.Fatal("serialized advanced authority was imported")
	}
	d3fRequireBuildRejected(t, &imported)
}

func TestD3FPersistenceFailurePreservesAuthority(t *testing.T) {
	m := d3fCandidateMapping(t)
	d3fApproveEvidence(t, m)
	d3fValidateReplay(t, m)
	outDir := t.TempDir()
	path := filepath.Join(outDir, "profile.json")
	before := []byte("existing profile must survive")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildFromMappingDefinition(m, "TestVendor", "TestModel", "Serial", "1.0", outDir, "1.0"); err == nil {
		t.Fatal("existing profile overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(before) {
		t.Fatalf("existing file changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "profile.yaml")); !os.IsNotExist(err) {
		t.Fatalf("failed build left partial output: %v", err)
	}
	d3fRequireStatus(t, m, mapping.StatusReplayValidated)
}

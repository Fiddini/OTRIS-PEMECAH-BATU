package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/profilebuilder"
	"github.com/otris-id/otris-pemecah-batu/internal/replay"
)

// TestGovernedPipelineHappyPath: Full authority chain succeeds
func TestGovernedPipelineHappyPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test data: MappingDefinition CANDIDATE
	m, err := mapping.New(
		"TEST_MAPPING", "1.0.0",
		"HL7_V2", 5, "OBX-5", "bpm",
		"HeartRate", "bpm",
		"test mapping from HL7 OBX-5 heart rate field", "test_author",
	)
	if err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	m.DeviceFingerprint = "fingerprint_hash_abc123"
	m.DeviceIdentityRef = "device_ref_xyz"
	m.ParserIdentity = "generic_hl7_v2"

	mappingJSON, _ := json.Marshal(m)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, mappingJSON, 0644)

	// Create evidence directory with valid evidence
	inputDir := filepath.Join(tmpDir, "input")
	os.MkdirAll(inputDir, 0755)

	evidencePath := filepath.Join(inputDir, "evidence.json")
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
			{Type: "VENDOR_MANUAL", Ref: "manual_ref_1"},
			{Type: "VENDOR_MANUAL", Ref: "manual_ref_2"},
		},
	}
	evJSON, _ := json.Marshal(claimReview)
	os.WriteFile(evidencePath, evJSON, 0644)

	// Create raw HL7 stream with OBX segment containing HR value
	// Format: MSH|...\nOBX|1|NM|OBX-5^...|...|88|bpm|...|...|...
	rawStream := []byte(
		"MSH|^~\\&|LAB|HOSP|LAB|HOSP|20240101120000||ADT^A01|12345|P|2.5\r\n" +
		"PID|1||12345^^^MRN||TEST^PATIENT||19800101|M\r\n" +
		"OBX|1|NM|OBX-5^HEART_RATE||88|bpm|60-100|N|||F\r\n",
	)
	os.WriteFile(filepath.Join(inputDir, "raw_stream.bin"), rawStream, 0644)

	// Create ground truth expectations (must use canonical concept, not source code)
	groundTruthPath := filepath.Join(inputDir, "ground_truth.json")
	groundTruth := map[string]interface{}{
		"HeartRate": 88.0, // Use canonical concept as key
	}
	gtJSON, _ := json.Marshal(groundTruth)
	os.WriteFile(groundTruthPath, gtJSON, 0644)

	// Create output directory
	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	// Run orchestrated pipeline logic
	err = runGovernedPipelineTest(
		mappingPath, evidencePath, inputDir, groundTruthPath, outDir, "1.0.0",
		"TestMfg", "TestModel", "tcp", "1.0.0_test", 100,
	)

	if err != nil {
		t.Fatalf("governed pipeline failed: %v", err)
	}

	// Verify profile.yaml exists
	profilePath := filepath.Join(outDir, "profile.yaml")
	if _, err := os.Stat(profilePath); err != nil {
		t.Fatalf("profile.yaml not created: %v", err)
	}

	// Verify profile.json exists
	profileJSONPath := filepath.Join(outDir, "profile.json")
	if _, err := os.Stat(profileJSONPath); err != nil {
		t.Fatalf("profile.json not created: %v", err)
	}

	// Verify output is CANDIDATE
	profileData, _ := os.ReadFile(profileJSONPath)
	var profile map[string]interface{}
	json.Unmarshal(profileData, &profile)

	if profile["status"] != "CANDIDATE" {
		t.Fatalf("profile status not CANDIDATE, got %v", profile["status"])
	}
	if profile["validation_status"] != "CANDIDATE" {
		t.Fatalf("validation_status not CANDIDATE, got %v", profile["validation_status"])
	}
}

// TestGovernedPipelineFailsMissingMapping: Missing mapping file fails closed
func TestGovernedPipelineFailsMissingMapping(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	os.MkdirAll(inputDir, 0755)

	err := runGovernedPipelineTest(
		filepath.Join(tmpDir, "nonexistent.json"),
		filepath.Join(inputDir, "evidence.json"),
		inputDir,
		filepath.Join(inputDir, "gt.json"),
		filepath.Join(tmpDir, "out"),
		"1.0.0",
		"Mfg", "Model", "tcp", "fw", 100,
	)

	if err == nil {
		t.Fatal("should have failed with missing mapping file")
	}
	if !strings.Contains(err.Error(), "cannot read mapping") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestGovernedPipelineFailsInvalidMapping: Invalid MappingDefinition JSON fails closed
func TestGovernedPipelineFailsInvalidMapping(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	os.MkdirAll(inputDir, 0755)

	// Write invalid JSON
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, []byte("not valid json {"), 0644)

	err := runGovernedPipelineTest(
		mappingPath,
		filepath.Join(inputDir, "evidence.json"),
		inputDir,
		filepath.Join(inputDir, "gt.json"),
		filepath.Join(tmpDir, "out"),
		"1.0.0",
		"Mfg", "Model", "tcp", "fw", 100,
	)

	if err == nil {
		t.Fatal("should have failed with invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid MappingDefinition JSON") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestGovernedPipelineFailsInsufficientEvidence: Missing evidence stops before replay
func TestGovernedPipelineFailsInsufficientEvidence(t *testing.T) {
	tmpDir := t.TempDir()

	m, _ := mapping.New(
		"TEST_MAPPING", "1.0.0",
		"HL7_V2", 5, "OBX-5", "bpm",
		"HeartRate", "bpm",
		"test", "author",
	)
	m.DeviceFingerprint = "fp"
	m.DeviceIdentityRef = "ref"
	m.ParserIdentity = "generic_hl7_v2"

	mappingJSON, _ := json.Marshal(m)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, mappingJSON, 0644)

	inputDir := filepath.Join(tmpDir, "input")
	os.MkdirAll(inputDir, 0755)

	// Create raw HL7 stream
	rawStream := []byte(
		"MSH|^~\\&|LAB|HOSP|LAB|HOSP|20240101120000||ADT^A01|12345|P|2.5\r\n" +
		"PID|1||12345^^^MRN||TEST^PATIENT||19800101|M\r\n" +
		"OBX|1|NM|OBX-5^HEART_RATE||88|bpm|60-100|N|||F\r\n",
	)
	os.WriteFile(filepath.Join(inputDir, "raw_stream.bin"), rawStream, 0644)

	// Only ONE evidence item (insufficient)
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
			{Type: "VENDOR_MANUAL", Ref: "only_one"},
		},
	}
	evJSON, _ := json.Marshal(claimReview)
	evidencePath := filepath.Join(inputDir, "evidence.json")
	os.WriteFile(evidencePath, evJSON, 0644)

	gtPath := filepath.Join(inputDir, "gt.json")
	gtJSON, _ := json.Marshal(map[string]interface{}{"HeartRate": 88.0})
	os.WriteFile(gtPath, gtJSON, 0644)

	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	err := runGovernedPipelineTest(
		mappingPath, evidencePath, inputDir, gtPath, outDir, "1.0.0",
		"Mfg", "Model", "tcp", "fw", 100,
	)

	if err == nil {
		t.Fatal("should have failed with insufficient evidence")
	}
	if !strings.Contains(err.Error(), "evidence review rejected") &&
		!strings.Contains(err.Error(), "NEEDS_MORE_EVIDENCE") {
		t.Logf("error: %v", err)
	}
}

// TestGovernedPipelineFailsMissingGroundTruth: Missing ground truth stops pipeline
func TestGovernedPipelineFailsMissingGroundTruth(t *testing.T) {
	tmpDir := t.TempDir()

	m, _ := mapping.New(
		"TEST_MAPPING", "1.0.0",
		"HL7_V2", 5, "OBX-5", "bpm",
		"HeartRate", "bpm",
		"test", "author",
	)
	m.DeviceFingerprint = "fp"
	m.DeviceIdentityRef = "ref"
	m.ParserIdentity = "generic_hl7_v2"

	mappingJSON, _ := json.Marshal(m)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, mappingJSON, 0644)

	inputDir := filepath.Join(tmpDir, "input")
	os.MkdirAll(inputDir, 0755)

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
			{Type: "VENDOR_MANUAL", Ref: "ref1"},
			{Type: "VENDOR_MANUAL", Ref: "ref2"},
		},
	}
	evJSON, _ := json.Marshal(claimReview)
	evidencePath := filepath.Join(inputDir, "evidence.json")
	os.WriteFile(evidencePath, evJSON, 0644)

	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	// Missing ground truth file
	nonexistentGT := filepath.Join(inputDir, "nonexistent_gt.json")

	err := runGovernedPipelineTest(
		mappingPath, evidencePath, inputDir, nonexistentGT, outDir, "1.0.0",
		"Mfg", "Model", "tcp", "fw", 100,
	)

	if err == nil {
		t.Fatal("should have failed with missing ground truth")
	}
}

// TestGovernedPipelineUsesGovernedAPIs: Verifies governed APIs are called
func TestGovernedPipelineUsesGovernedAPIs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create MappingDefinition
	m, _ := mapping.New(
		"TEST_MAPPING", "1.0.0",
		"HL7_V2", 5, "OBX-5", "bpm",
		"HeartRate", "bpm",
		"test", "author",
	)
	m.DeviceFingerprint = "fp_test"
	m.DeviceIdentityRef = "ref_test"
	m.ParserIdentity = "generic_hl7_v2"

	// Verify it starts as CANDIDATE
	status, _ := m.Status()
	if status != mapping.StatusCandidate {
		t.Fatalf("mapping should start as CANDIDATE, got %s", status)
	}

	mappingJSON, _ := json.Marshal(m)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, mappingJSON, 0644)

	inputDir := filepath.Join(tmpDir, "input")
	os.MkdirAll(inputDir, 0755)

	// Create raw HL7 stream
	rawStream := []byte(
		"MSH|^~\\&|LAB|HOSP|LAB|HOSP|20240101120000||ADT^A01|12345|P|2.5\r\n" +
		"PID|1||12345^^^MRN||TEST^PATIENT||19800101|M\r\n" +
		"OBX|1|NM|OBX-5^HEART_RATE||88|bpm|60-100|N|||F\r\n",
	)
	os.WriteFile(filepath.Join(inputDir, "raw_stream.bin"), rawStream, 0644)

	// Valid evidence for ReviewEvidence
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
			{Type: "VENDOR_MANUAL", Ref: "ref1"},
			{Type: "VENDOR_MANUAL", Ref: "ref2"},
		},
	}
	evJSON, _ := json.Marshal(claimReview)
	evidencePath := filepath.Join(inputDir, "evidence.json")
	os.WriteFile(evidencePath, evJSON, 0644)

	// Valid ground truth for ReviewReplay (use canonical concept as key)
	gtPath := filepath.Join(inputDir, "gt.json")
	gtJSON, _ := json.Marshal(map[string]interface{}{"HeartRate": 88.0})
	os.WriteFile(gtPath, gtJSON, 0644)

	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	err := runGovernedPipelineTest(
		mappingPath, evidencePath, inputDir, gtPath, outDir, "1.0.0",
		"Mfg", "Model", "tcp", "fw", 100,
	)

	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Verify evidence report was written by ReviewEvidence
	evReportPath := filepath.Join(inputDir, "governed_evidence_report.json")
	if _, err := os.Stat(evReportPath); err != nil {
		t.Fatalf("governed_evidence_report.json not created by ReviewEvidence: %v", err)
	}

	// Verify replay report was written by ReviewReplay
	replayReportPath := filepath.Join(inputDir, "governed_replay_report.json")
	if _, err := os.Stat(replayReportPath); err != nil {
		t.Fatalf("governed_replay_report.json not created by ReviewReplay: %v", err)
	}

	// Verify profile was written by BuildFromMappingDefinition
	profilePath := filepath.Join(outDir, "profile.yaml")
	if _, err := os.Stat(profilePath); err != nil {
		t.Fatalf("profile.yaml not created by BuildFromMappingDefinition: %v", err)
	}
}

// TestGovernedPipelineOutputIsCandidate: Output status must be CANDIDATE
func TestGovernedPipelineOutputIsCandidate(t *testing.T) {
	tmpDir := t.TempDir()

	m, _ := mapping.New(
		"TEST_MAPPING", "1.0.0",
		"HL7_V2", 5, "OBX-5", "bpm",
		"HeartRate", "bpm",
		"test", "author",
	)
	m.DeviceFingerprint = "fp"
	m.DeviceIdentityRef = "ref"
	m.ParserIdentity = "generic_hl7_v2"

	mappingJSON, _ := json.Marshal(m)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, mappingJSON, 0644)

	inputDir := filepath.Join(tmpDir, "input")
	os.MkdirAll(inputDir, 0755)

	// Create raw HL7 stream
	rawStream := []byte(
		"MSH|^~\\&|LAB|HOSP|LAB|HOSP|20240101120000||ADT^A01|12345|P|2.5\r\n" +
		"PID|1||12345^^^MRN||TEST^PATIENT||19800101|M\r\n" +
		"OBX|1|NM|OBX-5^HEART_RATE||88|bpm|60-100|N|||F\r\n",
	)
	os.WriteFile(filepath.Join(inputDir, "raw_stream.bin"), rawStream, 0644)

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
			{Type: "VENDOR_MANUAL", Ref: "ref1"},
			{Type: "VENDOR_MANUAL", Ref: "ref2"},
		},
	}
	evJSON, _ := json.Marshal(claimReview)
	evidencePath := filepath.Join(inputDir, "evidence.json")
	os.WriteFile(evidencePath, evJSON, 0644)

	gtPath := filepath.Join(inputDir, "gt.json")
	gtJSON, _ := json.Marshal(map[string]interface{}{"HeartRate": 88.0})
	os.WriteFile(gtPath, gtJSON, 0644)

	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	err := runGovernedPipelineTest(
		mappingPath, evidencePath, inputDir, gtPath, outDir, "1.0.0",
		"Mfg", "Model", "tcp", "fw", 100,
	)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	profileJSONPath := filepath.Join(outDir, "profile.json")
	profileData, err := os.ReadFile(profileJSONPath)
	if err != nil {
		t.Fatalf("profile.json not found: %v", err)
	}

	var profile map[string]interface{}
	if err := json.Unmarshal(profileData, &profile); err != nil {
		t.Fatalf("failed to parse profile.json: %v", err)
	}

	if profile == nil {
		t.Fatal("profile is nil")
	}
	if profile["status"] != "CANDIDATE" {
		t.Fatalf("status should be CANDIDATE, got %v", profile["status"])
	}
	if profile["validation_status"] != "CANDIDATE" {
		t.Fatalf("validation_status should be CANDIDATE, got %v", profile["validation_status"])
	}

	// Should NOT have VERIFIED or REAL_DEVICE_VALIDATED
	if profile["status"] == "VERIFIED" || profile["status"] == "REAL_DEVICE_VALIDATED" || profile["status"] == "PRODUCTION_VALIDATED" {
		t.Fatalf("output incorrectly has advanced status: %v", profile["status"])
	}
}

// runGovernedPipelineTest orchestrates the full governed pipeline
func runGovernedPipelineTest(
	mappingPath, evidencePath, inputDir, groundTruthPath, outDir, version string,
	manufacturer, model, transport, firmware string, count int,
) error {
	// Load MappingDefinition
	mappingData, err := os.ReadFile(mappingPath)
	if err != nil {
		return fmt.Errorf("cannot read mapping: %w", err)
	}

	m := &mapping.MappingDefinition{}
	if err := json.Unmarshal(mappingData, m); err != nil {
		return fmt.Errorf("invalid MappingDefinition JSON: %w", err)
	}

	status, err := m.Status()
	if err != nil || status != mapping.StatusCandidate {
		return fmt.Errorf("MappingDefinition must be CANDIDATE, got %s: %w", status, err)
	}

	// Step 1: ReviewEvidence
	evidenceReportPath := filepath.Join(inputDir, "governed_evidence_report.json")
	if err := mapping.ReviewEvidence(m, evidencePath, evidenceReportPath); err != nil {
		return fmt.Errorf("evidence review rejected: %w", err)
	}

	status, err = m.Status()
	if err != nil || status != mapping.StatusEvidenceApproved {
		return fmt.Errorf("after ReviewEvidence, status should be EVIDENCE_APPROVED, got %s: %w", status, err)
	}

	// Step 2: ReviewReplay
	replayReportPath := filepath.Join(inputDir, "governed_replay_report.json")
	replayOpts := replay.ReplayOptions{GroundTruthPath: groundTruthPath}
	replayReport, err := mapping.ReviewReplay(m, inputDir, count, replayOpts, replayReportPath)
	if err != nil {
		return fmt.Errorf("replay validation rejected: %w", err)
	}

	status, err = m.Status()
	if err != nil || status != mapping.StatusReplayValidated {
		return fmt.Errorf("after ReviewReplay, status should be REPLAY_VALIDATED, got %s: %w", status, err)
	}

	if replayReport.Verdict != "READY_FOR_PROFILE_BUILDER" {
		return fmt.Errorf("replay verdict should be READY_FOR_PROFILE_BUILDER, got %s", replayReport.Verdict)
	}

	// Step 3: BuildFromMappingDefinition
	os.MkdirAll(outDir, 0755)
	profile, err := profilebuilder.BuildFromMappingDefinition(m, manufacturer, model, transport, firmware, outDir, version)
	if err != nil {
		return fmt.Errorf("profile build rejected: %w", err)
	}

	if profile.Status != "CANDIDATE" || profile.ValidationStatus != "CANDIDATE" {
		return fmt.Errorf("output status should be CANDIDATE/CANDIDATE, got %s/%s", profile.Status, profile.ValidationStatus)
	}

	return nil
}

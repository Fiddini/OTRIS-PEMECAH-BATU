package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test A: missing evidence → NOT APPROVED
func TestMissingEvidenceFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentPath := filepath.Join(tmpDir, "nonexistent.json")

	report, err := Run(tmpDir, nonexistentPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "REJECTED" {
		t.Errorf("missing evidence: expected REJECTED, got %s", report.Verdict)
	}
	if report.Status != "REJECTED" {
		t.Errorf("missing evidence: expected REJECTED status, got %s", report.Status)
	}
	if len(report.Evidence) != 0 {
		t.Errorf("missing evidence: expected 0 evidence items, got %d (no synthetic data)", len(report.Evidence))
	}
}

// Test B: malformed evidence → NOT APPROVED
func TestMalformedEvidenceFile(t *testing.T) {
	tmpDir := t.TempDir()
	malformedPath := filepath.Join(tmpDir, "malformed.json")

	// Write malformed JSON
	err := os.WriteFile(malformedPath, []byte("{invalid json"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := Run(tmpDir, malformedPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "REJECTED" {
		t.Errorf("malformed evidence: expected REJECTED, got %s", report.Verdict)
	}
	if report.Status != "REJECTED" {
		t.Errorf("malformed evidence: expected REJECTED status, got %s", report.Status)
	}
	if len(report.Evidence) != 0 {
		t.Errorf("malformed evidence: expected 0 evidence items, got %d", len(report.Evidence))
	}
}

// Test C: fewer than required evidence items → NOT APPROVED
func TestInsufficientEvidenceItems(t *testing.T) {
	tmpDir := t.TempDir()
	insufficientPath := filepath.Join(tmpDir, "insufficient.json")

	// Single evidence item
	singleEvidence := []Evidence{
		{Type: "VENDOR_MANUAL", Ref: "Manual p.45", Quote: "Field 2"},
	}
	data, _ := json.Marshal(singleEvidence)
	err := os.WriteFile(insufficientPath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := Run(tmpDir, insufficientPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "NEEDS_MORE_EVIDENCE" {
		t.Errorf("one evidence item: expected NEEDS_MORE_EVIDENCE, got %s", report.Verdict)
	}
	if report.Status != "NEEDS_EVIDENCE" {
		t.Errorf("one evidence item: expected NEEDS_EVIDENCE status, got %s", report.Status)
	}
	if len(report.Evidence) != 1 {
		t.Errorf("one evidence item: expected 1 evidence item, got %d", len(report.Evidence))
	}
}

// Test D: valid sufficient evidence → APPROVED_FOR_MAPPING
func TestValidSufficientEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "valid.json")

	// Two evidence items (minimum required)
	validEvidence := []Evidence{
		{Type: "VENDOR_MANUAL", Ref: "Mindray BeneVision N12 Service Manual p.45", Quote: "Field 2 = Heart Rate"},
		{Type: "SIMULATOR_LOG", Ref: "captures/session_hr70/sim_log.json"},
	}
	data, _ := json.Marshal(validEvidence)
	err := os.WriteFile(validPath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := Run(tmpDir, validPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "APPROVED_FOR_MAPPING" {
		t.Errorf("valid two items: expected APPROVED_FOR_MAPPING, got %s", report.Verdict)
	}
	if report.Status != "MAPPED" {
		t.Errorf("valid two items: expected MAPPED status, got %s", report.Status)
	}
	if len(report.Evidence) != 2 {
		t.Errorf("valid two items: expected 2 evidence items, got %d", len(report.Evidence))
	}
	// Verify no synthetic data - exact source from file
	if report.Evidence[0].Ref != "Mindray BeneVision N12 Service Manual p.45" {
		t.Errorf("evidence should be from file, got: %s", report.Evidence[0].Ref)
	}
}

// Test E: no synthetic evidence is generated
func TestNoSyntheticEvidenceOnMissing(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentPath := filepath.Join(tmpDir, "missing.json")

	report, err := Run(tmpDir, nonexistentPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The hardcoded synthetic evidence from old code would be:
	// "Mindray BeneVision N12 Service Manual p.45"
	// Verify it's NOT present
	for _, ev := range report.Evidence {
		if ev.Ref == "Mindray BeneVision N12 Service Manual p.45" {
			t.Error("SYNTHETIC EVIDENCE FOUND: old code's hardcoded evidence should not be generated")
		}
		if ev.Ref == "captures/session_hr70/sim_log.json" {
			t.Error("SYNTHETIC EVIDENCE FOUND: old code's hardcoded simulator log should not be generated")
		}
	}

	if len(report.Evidence) > 0 {
		t.Errorf("no synthetic evidence should be created; got %d items", len(report.Evidence))
	}
}

// Additional test: empty JSON array → NOT APPROVED
func TestEmptyEvidenceArray(t *testing.T) {
	tmpDir := t.TempDir()
	emptyPath := filepath.Join(tmpDir, "empty.json")

	// Empty array
	data := []byte("[]")
	err := os.WriteFile(emptyPath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := Run(tmpDir, emptyPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "REJECTED" {
		t.Errorf("empty array: expected REJECTED, got %s", report.Verdict)
	}
	if len(report.Evidence) != 0 {
		t.Errorf("empty array: expected 0 evidence items, got %d", len(report.Evidence))
	}
}

// Additional test: single object (not array) with valid Type → should work
func TestSingleObjectEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	singleObjPath := filepath.Join(tmpDir, "single.json")

	// Single object (not array)
	singleEvidence := Evidence{
		Type:  "VENDOR_MANUAL",
		Ref:   "Manual section 5.2",
		Quote: "Device identification",
	}
	data, _ := json.Marshal(singleEvidence)
	err := os.WriteFile(singleObjPath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := Run(tmpDir, singleObjPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "NEEDS_MORE_EVIDENCE" {
		t.Errorf("single object: expected NEEDS_MORE_EVIDENCE, got %s", report.Verdict)
	}
	if len(report.Evidence) != 1 {
		t.Errorf("single object: expected 1 evidence item, got %d", len(report.Evidence))
	}
}

// Additional test: no evidence path provided → NOT APPROVED (fail closed)
func TestNoEvidencePathProvided(t *testing.T) {
	tmpDir := t.TempDir()

	report, err := Run(tmpDir, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "NEEDS_EVIDENCE" {
		t.Errorf("no path: expected NEEDS_EVIDENCE, got %s", report.Verdict)
	}
	if report.Status != "CANDIDATE" {
		t.Errorf("no path: expected CANDIDATE status, got %s", report.Status)
	}
	if len(report.Evidence) != 0 {
		t.Errorf("no path: expected 0 evidence items, got %d", len(report.Evidence))
	}
}

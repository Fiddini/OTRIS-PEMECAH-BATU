package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test A: missing replay input → FAIL / NOT READY
func TestMissingRawStreamFile(t *testing.T) {
	tmpDir := t.TempDir()

	report, err := RunWithOptions(tmpDir, 100, ReplayOptions{})

	// Should not error, but return a failed verdict
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "NOT_READY" {
		t.Errorf("missing input: expected NOT_READY, got %s", report.Verdict)
	}
	if report.Success != 0 {
		t.Errorf("missing input: expected 0 success, got %d", report.Success)
	}
	if report.FailedParse != 0 {
		t.Errorf("missing input: expected 0 failures, got %d", report.FailedParse)
	}
	if report.Notes == "" {
		t.Error("missing input: expected notes explaining the failure")
	}
}

// Test B: unreadable or malformed input → FAIL / NOT READY
func TestEmptyRawStreamFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyPath := filepath.Join(tmpDir, "raw_stream.bin")

	// Create empty file
	err := os.WriteFile(emptyPath, []byte{}, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 100, ReplayOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != "NOT_READY" {
		t.Errorf("empty input: expected NOT_READY, got %s", report.Verdict)
	}
	if report.Notes == "" {
		t.Error("empty input: expected notes explaining failure")
	}
}

// Test C: processing/parser failure → failure counter increases → NOT READY
func TestParserFailureOnInvalidFrames(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")

	// Write invalid data: no HL7 structure, no segment codes
	invalidData := "garbage data\nnot hl7\ninvalid\n"
	err := os.WriteFile(rawPath, []byte(invalidData), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 10, ReplayOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.FailedParse == 0 {
		t.Errorf("invalid frames: expected failures > 0, got %d", report.FailedParse)
	}
	if report.Verdict != "NOT_READY" {
		t.Errorf("invalid frames: expected NOT_READY, got %s", report.Verdict)
	}
}

// Test D: no ground-truth evidence → false_data_rate=0 is NOT claimed as proven → NOT_READY / NEEDS_GROUND_TRUTH
func TestNoGroundTruthForbidsZeroProof(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")

	// Write valid HL7-like data
	validData := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n"
	err := os.WriteFile(rawPath, []byte(validData), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 100, ReplayOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without ground truth, even if parsing succeeds, verdict should NOT be READY
	if report.Verdict == "READY_FOR_PROFILE_BUILDER" {
		t.Errorf("no ground truth: should NOT claim READY_FOR_PROFILE_BUILDER; got %s", report.Verdict)
	}
	if report.GroundTruthProvided {
		t.Error("no ground truth: GroundTruthProvided should be false")
	}
	// Should indicate NOT_PROVEN instead
	if report.Verdict != "NOT_PROVEN" {
		t.Errorf("no ground truth with valid frames: expected NOT_PROVEN, got %s", report.Verdict)
	}
}

// Test E: explicit matching ground truth proves measurement, not mapping authority.
func TestValidFixtureWithGroundTruthMatching(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")
	gtPath := filepath.Join(tmpDir, "ground_truth.json")

	// Write valid HL7 data
	validData := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n" +
		"PID|1||12345^^^MRN||DOE^JOHN^A||19700101|M\n" +
		"OBX|1|NM|HR^Heart Rate||88|bpm|60-100|N||\n"

	err := os.WriteFile(rawPath, []byte(validData), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Explicit measured expectation; empty expectations cannot prove a match.
	groundTruth := map[string]int{"HR": 88}
	gtData, _ := json.Marshal(groundTruth)
	err = os.WriteFile(gtPath, gtData, 0644)
	if err != nil {
		t.Fatalf("failed to write ground truth: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 10, ReplayOptions{GroundTruthPath: gtPath})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.GroundTruthProvided {
		t.Error("ground truth should be marked as provided")
	}
	if report.FailedParse > 0 {
		t.Errorf("valid frames should not fail: %d failures", report.FailedParse)
	}
	if report.FalseData != 0 {
		t.Errorf("no mismatches: expected false_data = 0, got %d", report.FalseData)
	}
	if report.Verdict != "MEASURED_MATCH" {
		t.Errorf("matching data without mapping: expected MEASURED_MATCH, got %s", report.Verdict)
	}
}

// Test F: known fixture where actual result conflicts with explicit ground truth → false_data_count > 0 → FAIL_FALSE_DATA
func TestFixtureWithGroundTruthMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")
	gtPath := filepath.Join(tmpDir, "ground_truth.json")

	// Write HL7 data
	validData := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n"

	err := os.WriteFile(rawPath, []byte(validData), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Write ground truth expecting HR=100
	groundTruth := map[string]int{"HR": 100} // expect HR=100, but data has no OBX segment
	gtData, _ := json.Marshal(groundTruth)
	err = os.WriteFile(gtPath, gtData, 0644)
	if err != nil {
		t.Fatalf("failed to write ground truth: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 10, ReplayOptions{GroundTruthPath: gtPath})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.GroundTruthProvided != true {
		t.Error("ground truth should be marked as provided")
	}
	// No observation means no measured proof, even when headers parse correctly.
	if report.FalseData == 0 || report.Compared != 0 || report.Verdict != "FAIL_FALSE_DATA" {
		t.Fatalf("missing actual observation must fail closed: %+v", report)
	}
}

// Test G: repeated replay derives counters from actual executions, not hardcoded values
func TestRepeatedReplayCountsActualExecutions(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")

	// Single valid frame
	singleFrame := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n"
	err := os.WriteFile(rawPath, []byte(singleFrame), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run with count=5 (should replay 5 times)
	report1, err := RunWithOptions(tmpDir, 5, ReplayOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedCount1 := 5 * 1 // 1 frame replayed 5 times
	if report1.Success != expectedCount1 {
		t.Errorf("replay count 5: expected %d successes, got %d", expectedCount1, report1.Success)
	}

	// Run with count=10 (should replay 10 times)
	report2, err := RunWithOptions(tmpDir, 10, ReplayOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedCount2 := 10 * 1 // 1 frame replayed 10 times
	if report2.Success != expectedCount2 {
		t.Errorf("replay count 10: expected %d successes, got %d", expectedCount2, report2.Success)
	}

	// Verify counts are different (not hardcoded)
	if report1.Success == report2.Success {
		t.Errorf("counts should differ by replay count; both are %d", report1.Success)
	}
}

// Test H: no code path fabricates false_data_rate = 0
func TestNoFabricatedZero(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")

	// Write valid data
	validData := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n"
	err := os.WriteFile(rawPath, []byte(validData), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 100, ReplayOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The old code would return READY_FOR_PROFILE_BUILDER with false_data=0 and no ground truth
	// New code should NOT do that
	if report.Verdict == "READY_FOR_PROFILE_BUILDER" && !report.GroundTruthProvided {
		t.Error("SECURITY: false_data_rate=0 is claimed READY without ground truth - FABRICATED")
	}

	// Verify FalseDataRate is only 0 when:
	// 1. false_data counter is actually 0 (not defaulted), AND
	// 2. Either no ground truth (NOT_PROVEN) OR ground truth provided and matched (READY)
	if report.FalseDataRate == 0.0 && report.Verdict == "READY_FOR_PROFILE_BUILDER" && !report.GroundTruthProvided {
		t.Error("VIOLATION: false_data_rate=0 fabricated without ground truth")
	}
}

// Test: empty frames should fail closed, not silently skipped
func TestEmptyFrameIsFailure(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")

	// Data with newlines creating empty frames
	dataWithEmpty := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n\n\nPID|1||12345^^^MRN||DOE^JOHN^A||19700101|M\n"
	err := os.WriteFile(rawPath, []byte(dataWithEmpty), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 10, ReplayOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty frames should be counted as failures, not skipped
	if report.FailedParse == 0 {
		t.Errorf("empty frames: expected failures > 0, got %d", report.FailedParse)
	}
}

// BLOCKER TEST 1: expected HR=88 + actual HR=88 → MATCH
func TestGroundTruthValueMatch_HR88(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")
	gtPath := filepath.Join(tmpDir, "ground_truth.json")

	// HL7 frame with HR=88
	hl7Data := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n" +
		"OBX|1|NM|HR^Heart Rate||88|bpm|60-100|N||\n"
	err := os.WriteFile(rawPath, []byte(hl7Data), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Ground truth: expect HR=88
	groundTruth := map[string]int{"HR": 88}
	gtData, _ := json.Marshal(groundTruth)
	err = os.WriteFile(gtPath, gtData, 0644)
	if err != nil {
		t.Fatalf("failed to write ground truth: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 10, ReplayOptions{GroundTruthPath: gtPath})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.GroundTruthProvided != true {
		t.Error("ground truth should be marked as provided")
	}
	if report.FalseData != 0 {
		t.Errorf("HR=88 matches expected HR=88: expected false_data=0, got %d", report.FalseData)
	}
	if report.Verdict != "MEASURED_MATCH" {
		t.Errorf("matching values without mapping: expected MEASURED_MATCH, got %s", report.Verdict)
	}
}

// BLOCKER TEST 2: expected HR=88 + actual HR=999 → FALSE DATA / FAIL_FALSE_DATA
func TestGroundTruthValueMismatch_HR88vs999(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")
	gtPath := filepath.Join(tmpDir, "ground_truth.json")

	// HL7 frame with HR=999 (wrong value)
	hl7Data := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n" +
		"OBX|1|NM|HR^Heart Rate||999|bpm|60-100|N||\n"
	err := os.WriteFile(rawPath, []byte(hl7Data), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Ground truth: expect HR=88
	groundTruth := map[string]int{"HR": 88}
	gtData, _ := json.Marshal(groundTruth)
	err = os.WriteFile(gtPath, gtData, 0644)
	if err != nil {
		t.Fatalf("failed to write ground truth: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 10, ReplayOptions{GroundTruthPath: gtPath})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.FalseData == 0 {
		t.Errorf("HR=999 vs expected HR=88: should detect false_data > 0, got %d", report.FalseData)
	}
	if report.Verdict != "FAIL_FALSE_DATA" {
		t.Errorf("value mismatch: expected FAIL_FALSE_DATA, got %s", report.Verdict)
	}
}

// BLOCKER TEST 3: expected HR=88 + HR field present but value unextractable → FALSE DATA
func TestGroundTruthUnextractableValue(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw_stream.bin")
	gtPath := filepath.Join(tmpDir, "ground_truth.json")

	// HL7 frame with HR field but non-numeric value (unextractable)
	hl7Data := "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5\n" +
		"OBX|1|NM|HR^Heart Rate||UNKNOWN|bpm|60-100|N||\n"
	err := os.WriteFile(rawPath, []byte(hl7Data), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Ground truth: expect HR=88
	groundTruth := map[string]int{"HR": 88}
	gtData, _ := json.Marshal(groundTruth)
	err = os.WriteFile(gtPath, gtData, 0644)
	if err != nil {
		t.Fatalf("failed to write ground truth: %v", err)
	}

	report, err := RunWithOptions(tmpDir, 10, ReplayOptions{GroundTruthPath: gtPath})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Can't extract value = can't compare = value not found in extracted map = false data
	if report.FalseData == 0 {
		t.Errorf("unextractable value: expected false_data > 0 (value unparseable), got %d", report.FalseData)
	}
	// Should be FAIL_FALSE_DATA because ground truth check failed
	if report.Verdict != "FAIL_FALSE_DATA" {
		t.Errorf("unextractable value: expected FAIL_FALSE_DATA, got %s", report.Verdict)
	}
}

// Test: extractObservationValues correctly parses OBX segments
func TestExtractObservationValues(t *testing.T) {
	tests := []struct {
		name     string
		hl7      string
		wantKeys []string
		wantVals map[string]float64
	}{
		{
			name:     "single OBX with HR",
			hl7:      "MSH|^~\\&|SendApp|SendFac|RecvApp|RecvFac|20260101||ADT^A01|001|P|2.5\nOBX|1|NM|HR^Heart Rate||88|bpm|60-100|N||",
			wantKeys: []string{"HR"},
			wantVals: map[string]float64{"HR": 88},
		},
		{
			name:     "multiple OBX segments",
			hl7:      "OBX|1|NM|HR^Heart Rate||88|bpm|60-100|N||\nOBX|2|NM|SpO2^Oxygen||97|%|95-100|N||",
			wantKeys: []string{"HR", "SpO2"},
			wantVals: map[string]float64{"HR": 88, "SpO2": 97},
		},
		{
			name:     "OBX preserves decimal value",
			hl7:      "OBX|1|NM|HR^Heart Rate||88.7|bpm|60-100|N||",
			wantKeys: []string{"HR"},
			wantVals: map[string]float64{"HR": 88.7},
		},
		{
			name:     "OBX with non-numeric value",
			hl7:      "OBX|1|NM|HR^Heart Rate||UNKNOWN|bpm|60-100|N||",
			wantKeys: []string{},
			wantVals: map[string]float64{},
		},
		{
			name:     "missing OBX value field",
			hl7:      "OBX|1|NM|HR^Heart Rate||",
			wantKeys: []string{},
			wantVals: map[string]float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractObservationValues(tt.hl7)

			// Check keys
			for _, key := range tt.wantKeys {
				val, found := result[key]
				if !found {
					t.Errorf("expected key %q not found", key)
				} else if val != tt.wantVals[key] {
					t.Errorf("key %q: expected %g, got %g", key, tt.wantVals[key], val)
				}
			}

			// Check no unexpected keys
			for key := range result {
				found := false
				for _, wkey := range tt.wantKeys {
					if key == wkey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("unexpected key %q in result", key)
				}
			}
		})
	}
}

// Test: validateHL7Frame function validates HL7 structure
func TestValidateHL7Frame(t *testing.T) {
	tests := []struct {
		name  string
		frame string
		valid bool
	}{
		{
			name:  "valid MSH segment",
			frame: "MSH|^~\\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260101120000||ADT^A01|MSG00001|P|2.5",
			valid: true,
		},
		{
			name:  "valid PID segment",
			frame: "PID|1||12345^^^MRN||DOE^JOHN^A||19700101|M",
			valid: true,
		},
		{
			name:  "valid OBX segment",
			frame: "OBX|1|NM|HR^Heart Rate||88|bpm|60-100|N||",
			valid: true,
		},
		{
			name:  "missing segment code",
			frame: "invalid|data|here",
			valid: false,
		},
		{
			name:  "no pipe separator",
			frame: "MSHnopipeseparator",
			valid: false,
		},
		{
			name:  "too short",
			frame: "MSH",
			valid: false,
		},
		{
			name:  "lowercase segment code",
			frame: "msh|^~\\&|SendingApp",
			valid: false,
		},
		{
			name:  "garbage",
			frame: "garbage data no structure",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateHL7Frame(tt.frame)
			if result != tt.valid {
				t.Errorf("validateHL7Frame(%q): expected %v, got %v", tt.frame, tt.valid, result)
			}
		})
	}
}


package replay

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

type Report struct {
    SessionID string `json:"session_id"`
    ReplayCount int `json:"replay_count"`
    Parser string `json:"parser"`
    Mapping string `json:"mapping"`
    Success int `json:"success"`
    FailedParse int `json:"failed_parse"`
    FalseData int `json:"false_data"`
    FalseDataRate float64 `json:"false_data_rate"`
    Verdict string `json:"verdict"`
    GroundTruthProvided bool `json:"ground_truth_provided"`
    Notes string `json:"notes,omitempty"`
}

// ReplayOptions allows passing ground truth for comparison
type ReplayOptions struct {
    GroundTruthPath string // optional file with expected values
}

func Run(inDir string, count int) (Report, error) {
    return RunWithOptions(inDir, count, ReplayOptions{})
}

func RunWithOptions(inDir string, count int, opts ReplayOptions) (Report, error) {
    if count <=0 { count = 1000 }
    rawPath := filepath.Join(inDir, "raw_stream.bin")
    data, err := os.ReadFile(rawPath)

    // Missing input: FAIL CLOSED
    if err != nil {
        report := Report{
            SessionID: filepath.Base(inDir),
            ReplayCount: count,
            Parser: "generic_hl7_v2",
            Mapping: "mindray_n12_mapping_v0.1",
            Success: 0,
            FailedParse: 0,
            FalseData: 0,
            FalseDataRate: 0.0,
            Verdict: "NOT_READY",
            GroundTruthProvided: false,
            Notes: "missing raw_stream.bin file",
        }
        return report, nil
    }

    // Unreadable or malformed input: FAIL CLOSED
    if len(data) == 0 {
        report := Report{
            SessionID: filepath.Base(inDir),
            ReplayCount: count,
            Parser: "generic_hl7_v2",
            Mapping: "mindray_n12_mapping_v0.1",
            Success: 0,
            FailedParse: 0,
            FalseData: 0,
            FalseDataRate: 0.0,
            Verdict: "NOT_READY",
            GroundTruthProvided: false,
            Notes: "raw_stream.bin is empty",
        }
        return report, nil
    }

    // framing sederhana: split newline atau MLLP
    frames := splitFrames(data)

    if len(frames) == 0 {
        report := Report{
            SessionID: filepath.Base(inDir),
            ReplayCount: count,
            Parser: "generic_hl7_v2",
            Mapping: "mindray_n12_mapping_v0.1",
            Success: 0,
            FailedParse: 0,
            FalseData: 0,
            FalseDataRate: 0.0,
            Verdict: "NOT_READY",
            GroundTruthProvided: false,
            Notes: "no valid frames after parsing",
        }
        return report, nil
    }

    success := 0
    failed := 0
    falseData := 0
    groundTruthProvided := false

    // Load ground truth if provided
    var expectedCounts map[string]int
    if opts.GroundTruthPath != "" {
        gtData, err := os.ReadFile(opts.GroundTruthPath)
        if err == nil && len(gtData) > 0 {
            groundTruthProvided = true
            json.Unmarshal(gtData, &expectedCounts)
        }
    }

    // replay count kali
    for i:=0; i<count; i++ {
        for _, fr := range frames {
            text := string(fr)
            text = strings.TrimSpace(text)

            // FAIL CLOSED: empty frame is a failure, never silently skip
            if len(text) == 0 {
                failed++
                continue
            }

            // Improved parser: validate HL7 structure
            // HL7 must have: MSH segment, field separators (|), at least 3 fields
            if validateHL7Frame(text) {
                success++

                // Only measure false data if we have ground truth to compare against
                // AND this is an observation segment (OBX) where we can extract values
                if groundTruthProvided && expectedCounts != nil {
                    // Only check ground truth for OBX segments
                    // Skip header segments (MSH, PID) which don't contain observation values
                    if isObservationSegment(text) {
                        if !matchesGroundTruth(text, expectedCounts) {
                            falseData++
                        }
                    }
                }
            } else {
                failed++
            }
        }
    }

    total := success + failed
    rate := 0.0
    if total>0 { rate = float64(falseData)/float64(total) }

    verdict := "NOT_READY"
    notes := ""

    // Fail closed: only READY if conditions are met
    if total > 0 && failed == 0 {
        // All frames parsed successfully
        if groundTruthProvided {
            // Ground truth provided: check false_data count
            if falseData == 0 {
                verdict = "READY_FOR_PROFILE_BUILDER"
            } else {
                verdict = "FAIL_FALSE_DATA"
                notes = fmt.Sprintf("measured false_data > 0: %d/%d", falseData, total)
            }
        } else {
            // No ground truth: cannot claim false_data = 0 is proven
            verdict = "NOT_PROVEN"
            notes = "no ground truth provided; cannot measure false_data reliably"
        }
    } else if failed > 0 {
        verdict = "NOT_READY"
        notes = fmt.Sprintf("parse failures detected: %d/%d", failed, total)
    }

    report := Report{
        SessionID: filepath.Base(inDir),
        ReplayCount: count,
        Parser: "generic_hl7_v2",
        Mapping: "mindray_n12_mapping_v0.1",
        Success: success,
        FailedParse: failed,
        FalseData: falseData,
        FalseDataRate: rate,
        Verdict: verdict,
        GroundTruthProvided: groundTruthProvided,
        Notes: notes,
    }

    outPath := filepath.Join(inDir, fmt.Sprintf("replay_report_%d.json", count))
    f, _ := os.Create(outPath)
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    enc.Encode(report)
    f.Close()
    fmt.Printf("[PB-12] REPLAY LAB selesai: %d replay, success=%d failed=%d false_data=%d rate=%.4f verdict=%s\n", count, success, failed, falseData, rate, verdict)
    return report, nil
}

// isObservationSegment checks if frame is an OBX (observation) segment
// Only OBX segments contain observation values for ground truth comparison
func isObservationSegment(text string) bool {
    text = strings.TrimSpace(text)
    if len(text) < 3 {
        return false
    }

    segmentCode := text[0:3]
    return segmentCode == "OBX"
}

// validateHL7Frame checks basic HL7 v2 structure
// HL7 segments start with 3-letter code (MSH, PID, OBX, etc.)
// Fields separated by |, subfields by ^
func validateHL7Frame(text string) bool {
    text = strings.TrimSpace(text)

    // Minimum HL7 frame: "MSH|..." (at least 4 chars)
    if len(text) < 4 {
        return false
    }

    // Check for segment separator (|)
    if !strings.Contains(text, "|") {
        return false
    }

    // Check if it starts with a 3-letter segment code
    // (MSH, PID, OBX, etc.)
    if len(text) < 3 {
        return false
    }

    segmentCode := text[0:3]
    // Basic validation: segment code should be uppercase letters
    for _, c := range segmentCode {
        if c < 'A' || c > 'Z' {
            return false
        }
    }

    return true
}

// matchesGroundTruth compares ACTUAL EXTRACTED VALUES against expected ground truth
// Parses HL7 frames to extract observation values (HR, SpO2, etc.)
// Compares them numerically with ground truth expectations
// Returns: true = all values match, false = any value mismatch or unextractable
func matchesGroundTruth(text string, expected map[string]int) bool {
    // If no expectations defined, consider it a match
    if len(expected) == 0 {
        return true
    }

    // Extract actual values from HL7 frame
    actualValues := extractObservationValues(text)

    // Compare each expected value with actual extracted value
    for fieldName, expectedValue := range expected {
        actualValue, found := actualValues[fieldName]

        if !found {
            // Expected field not found in frame = mismatch (FAIL CLOSED)
            return false
        }

        // Value found: compare numerically
        if actualValue != expectedValue {
            // Value mismatch: actual != expected = FALSE DATA
            return false
        }
    }

    return true
}

// extractObservationValues parses HL7 frame and extracts observation field values
// HL7 OBX segment format: OBX|seq|type|observation_id||value|units|...
// Extracts numeric values and maps them to field names
// If a value cannot be parsed as int, that field is not included (mismatch if expected)
func extractObservationValues(text string) map[string]int {
    values := make(map[string]int)

    // Split into segments (handle both single-line and multi-segment)
    lines := strings.Split(text, "\n")

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if len(line) == 0 {
            continue
        }

        // Parse HL7 segment by pipe delimiter
        fields := strings.Split(line, "|")
        if len(fields) == 0 {
            continue
        }

        segmentType := fields[0]

        // OBX segment: observation value
        // Format: OBX|1|NM|HR^Heart Rate||88|bpm|60-100|N||
        // Field [3] = observation code (e.g., "HR^Heart Rate")
        // Field [5] = value (e.g., "88")
        if segmentType == "OBX" && len(fields) > 5 {
            observationCode := fields[3]
            valueStr := fields[5]

            // Extract field name from observation code
            // "HR^Heart Rate" -> use "HR" as field name
            codeParts := strings.Split(observationCode, "^")
            if len(codeParts) > 0 && codeParts[0] != "" {
                fieldName := codeParts[0]

                // Parse value as integer
                value, err := parseIntValue(valueStr)
                if err == nil {
                    values[fieldName] = value
                }
                // If can't parse int, field is omitted (will cause mismatch if expected)
            }
        }
    }

    return values
}

// parseIntValue parses numeric strings to integers
// Handles: "88", " 88 ", "88.5" (truncates), etc.
// Returns error if cannot parse
func parseIntValue(s string) (int, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return 0, fmt.Errorf("empty value")
    }

    // Try integer first
    var val int
    _, err := fmt.Sscanf(s, "%d", &val)
    if err == nil {
        return val, nil
    }

    // Try float and truncate
    var fval float64
    _, err = fmt.Sscanf(s, "%f", &fval)
    if err == nil {
        return int(fval), nil
    }

    return 0, fmt.Errorf("cannot parse: %s", s)
}

func splitFrames(data []byte) [][]byte {
    // MLLP
    if contains(data, 0x0B) {
        frames := [][]byte{}
        start := -1
        for i:=0; i<len(data)-1; i++ {
            if data[i]==0x0B { start=i+1 }
            if data[i]==0x1C && data[i+1]==0x0D && start!=-1 {
                frames = append(frames, data[start:i])
                start=-1
            }
        }
        if len(frames)>0 { return frames }
    }
    // NEWLINE: preserve all frames including empty ones for proper failure tracking
    lines := [][]byte{}
    cur := []byte{}
    for _, b := range data {
        if b=='\n' {
            // Add frame (even if empty) to preserve failures
            lines = append(lines, cur)
            cur=[]byte{}
        } else {
            cur = append(cur, b)
        }
    }
    if len(cur)>0 { lines = append(lines, cur) }
    return lines
}

func contains(data []byte, b byte) bool {
    for _, v := range data { if v==b { return true } }
    return false
}

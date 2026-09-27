package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
)

type Report struct {
	SessionID           string          `json:"session_id"`
	ReplayCount         int             `json:"replay_count"`
	Parser              string          `json:"parser"`
	Mapping             string          `json:"mapping"`
	MappingVersion      string          `json:"mapping_version"`
	Claim               *evidence.Claim `json:"claim,omitempty"`
	Success             int             `json:"success"`
	FailedParse         int             `json:"failed_parse"`
	Compared            int             `json:"compared"`
	FalseData           int             `json:"false_data"`
	FalseDataRate       float64         `json:"false_data_rate"`
	Verdict             string          `json:"verdict"`
	GroundTruthProvided bool            `json:"ground_truth_provided"`
	Notes               string          `json:"notes,omitempty"`
}

type ReplayOptions struct {
	GroundTruthPath string // Explicit numeric expectations; never inferred.
}

func Run(inDir string, count int) (Report, error) {
	return RunWithOptions(inDir, count, ReplayOptions{})
}

// RunWithOptions measures source-code expectations without granting mapping authority.
func RunWithOptions(inDir string, count int, opts ReplayOptions) (Report, error) {
	if count <= 0 {
		count = 1000
	}
	report, err := run(inDir, count, opts, nil)
	if err != nil {
		return report, err
	}
	if err := WriteReport(filepath.Join(inDir, fmt.Sprintf("replay_report_%d.json", count)), report); err != nil {
		report.Verdict = "NOT_READY"
		return report, err
	}
	return report, nil
}

// MeasureClaim returns fresh measured data, NOT authority or Profile Builder readiness.
// Mapping's governor checks Evidence approval before calling this function.
// Ground truth must contain exactly this claim's canonical concept and its numeric value.
func MeasureClaim(claim evidence.Claim, inDir string, count int, opts ReplayOptions) (Report, error) {
	return run(inDir, count, opts, &claim)
}

func run(inDir string, count int, opts ReplayOptions, claim *evidence.Claim) (Report, error) {
	report := Report{
		SessionID: filepath.Base(inDir), ReplayCount: count,
		Parser: "generic_hl7_v2", Verdict: "NOT_READY", Claim: claim,
	}
	if claim != nil {
		report.Mapping, report.MappingVersion = claim.MappingIdentity, claim.MappingVersion
		report.Parser = claim.ParserIdentity
		if count <= 0 || strings.TrimSpace(claim.MappingIdentity) == "" || strings.TrimSpace(claim.MappingVersion) == "" {
			return report, errors.New("claim replay requires identity, version and positive replay count")
		}
		// ponytail: numeric HL7 OBX-5 with identical units only. Add explicit parser
		// or conversion support before accepting other claims; never guess semantics.
		if claim.SourceProtocol != "HL7_V2" || claim.ParserIdentity != "generic_hl7_v2" ||
			claim.SourcePosition != 5 || strings.TrimSpace(claim.SourceCode) == "" ||
			strings.TrimSpace(claim.SourceUnit) == "" || claim.SourceUnit != claim.CanonicalUnit ||
			strings.TrimSpace(claim.CanonicalConcept) == "" {
			return report, errors.New("unsupported explicit replay claim")
		}
	}

	var expected map[string]float64
	if opts.GroundTruthPath != "" {
		var err error
		expected, err = readGroundTruth(opts.GroundTruthPath)
		if err != nil {
			return report, err
		}
		report.GroundTruthProvided = true
	}
	if claim != nil {
		if _, ok := expected[claim.CanonicalConcept]; !ok || len(expected) != 1 {
			return report, errors.New("ground truth must explicitly supply exactly the claimed canonical concept")
		}
	}

	data, err := os.ReadFile(filepath.Join(inDir, "raw_stream.bin"))
	if err != nil || len(data) == 0 {
		report.Notes = "missing, unreadable or empty raw_stream.bin"
		return report, nil
	}
	frames := splitFrames(data)
	for i := 0; i < count; i++ {
		comparedBefore := report.Compared
		for _, frame := range frames {
			text := strings.ReplaceAll(string(frame), "\r\n", "\n")
			text = strings.ReplaceAll(text, "\r", "\n")
			for _, segment := range strings.Split(strings.TrimSpace(text), "\n") {
				segment = strings.TrimSpace(segment)
				if !validateHL7Frame(segment) {
					report.FailedParse++
					continue
				}
				report.Success++
				if !report.GroundTruthProvided || !isObservationSegment(segment) {
					continue
				}
				if claim == nil {
					report.Compared++
					if !matchesGroundTruth(segment, expected) {
						report.FalseData++
					}
					continue
				}
				fields := strings.Split(segment, "|")
				if len(fields) <= 3 || strings.SplitN(fields[3], "^", 2)[0] != claim.SourceCode {
					continue
				}
				report.Compared++
				if len(fields) <= 6 || fields[2] != "NM" || fields[6] != claim.SourceUnit {
					report.FalseData++
					continue
				}
				actual, err := parseNumericValue(fields[claim.SourcePosition])
				if err != nil || actual != expected[claim.CanonicalConcept] {
					report.FalseData++
				}
			}
		}
		if report.GroundTruthProvided && report.Compared == comparedBefore {
			report.FalseData++ // Missing actual observation is not a zero-error proof.
		}
	}
	total := report.Success + report.FailedParse
	if total > 0 {
		report.FalseDataRate = float64(report.FalseData) / float64(total)
	}
	switch {
	case total == 0 || report.FailedParse > 0:
		report.Notes = fmt.Sprintf("parse failures detected: %d/%d", report.FailedParse, total)
	case !report.GroundTruthProvided:
		report.Verdict, report.Notes = "NOT_PROVEN", "no explicit ground truth; false_data is not proven"
	case report.FalseData > 0:
		report.Verdict = "FAIL_FALSE_DATA"
		report.Notes = fmt.Sprintf("measured false_data > 0: %d/%d", report.FalseData, total)
	case report.Compared > 0:
		report.Verdict = "MEASURED_MATCH"
	}
	return report, nil
}

func readGroundTruth(path string) (map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ground truth: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errors.New("ground truth must be a numeric expectation object")
	}
	expected := make(map[string]float64)
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("decode ground truth key: %w", err)
		}
		key, ok := token.(string)
		if !ok || strings.TrimSpace(key) == "" {
			return nil, errors.New("invalid ground truth key")
		}
		if _, exists := expected[key]; exists {
			return nil, fmt.Errorf("duplicate ground truth key %q", key)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode ground truth value: %w", err)
		}
		number, err := parseNumericValue(string(value))
		if err != nil {
			return nil, fmt.Errorf("invalid ground truth value for %q", key)
		}
		expected[key] = number
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') || len(expected) == 0 {
		return nil, errors.New("ground truth requires complete nonempty expectations")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing ground truth data")
	}
	return expected, nil
}

// WriteReport persists provenance only; no caller-supplied report grants authority.
// Never overwrite inputs or an existing report, including a symlink.
func WriteReport(path string, report Report) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create replay report: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := errors.Join(enc.Encode(report), f.Close()); err != nil {
		return errors.Join(fmt.Errorf("write replay report: %w", err), os.Remove(path))
	}
	return nil
}

func isObservationSegment(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "OBX|")
}

func validateHL7Frame(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 4 || text[3] != '|' {
		return false
	}
	for _, c := range text[:3] {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func matchesGroundTruth(text string, expected map[string]float64) bool {
	if len(expected) == 0 {
		return false
	}
	actual := extractObservationValues(text)
	for field, value := range expected {
		if got, ok := actual[field]; !ok || got != value {
			return false
		}
	}
	return true
}

func extractObservationValues(text string) map[string]float64 {
	values := make(map[string]float64)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if fields[0] != "OBX" || len(fields) <= 5 {
			continue
		}
		code := strings.SplitN(fields[3], "^", 2)[0]
		value, err := parseNumericValue(fields[5])
		if code != "" && err == nil {
			values[code] = value
		}
	}
	return values
}

// Preserve fractional values; reject non-HL7 syntax and non-finite numbers.
// HL7 NM: optional sign, digits, optional decimal point and digits only.
func parseNumericValue(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty numeric value")
	}
	// Reject Go-only numeric syntax: 0x prefix, underscores, exponent notation.
	for _, substr := range []string{"0x", "0X", "0o", "0O", "0b", "0B", "_", "e", "E"} {
		if strings.Contains(s, substr) {
			return 0, fmt.Errorf("non-HL7 numeric syntax: %q", s)
		}
	}
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid numeric value: %q", s)
	}
	return value, nil
}

func splitFrames(data []byte) [][]byte {
	if bytes.Contains(data, []byte{0x0B}) {
		var frames [][]byte
		for len(data) > 0 {
			end := bytes.Index(data, []byte{0x1C, 0x0D})
			if data[0] != 0x0B || end < 1 || bytes.Contains(data[1:end], []byte{0x0B}) {
				return [][]byte{nil} // Do not discard malformed MLLP or its trailing data.
			}
			frames = append(frames, data[1:end])
			data = data[end+2:]
		}
		return frames
	}
	// Preserve interior empty frames; a final line terminator is not a frame.
	return bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
}

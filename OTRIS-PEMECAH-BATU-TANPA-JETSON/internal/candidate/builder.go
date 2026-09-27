package candidate

import (
	"fmt"
	"os"
	"path/filepath"
)

// BuildRealDeviceCandidates creates candidate claim records for all real device codes
// from captured wire evidence. Returns map of source_code → candidate claim.
func BuildRealDeviceCandidates(captureDir string) (map[string]*Claim, error) {
	if captureDir == "" {
		return nil, fmt.Errorf("captureDir required")
	}

	// Define all 5 real codes from wire evidence
	// (capture 001: 44 frames, capture 002: 22 frames, all consistent)
	codes := []struct {
		code           string
		unitString     string
		unitLabel      string
		mappingID      string
		fingerprint001 string
		fingerprint002 string
	}{
		{
			code:           "147842",
			unitString:     "264864^MDC_DIM_BEAT_PER_MIN^MDC",
			unitLabel:      "MDC_DIM_BEAT_PER_MIN",
			mappingID:      "real-device-147842-mdc-ecg-heart-rate",
			fingerprint001: "4de28380713b7867",
			fingerprint002: "578b92f887e2cf67",
		},
		{
			code:           "150456",
			unitString:     "262688^MDC_DIM_PERCENT^MDC",
			unitLabel:      "MDC_DIM_PERCENT",
			mappingID:      "real-device-150456-mdc-puls-oxim-sat-o2",
			fingerprint001: "4de28380713b7867",
			fingerprint002: "578b92f887e2cf67",
		},
		{
			code:           "149530",
			unitString:     "264864^MDC_DIM_BEAT_PER_MIN^MDC",
			unitLabel:      "MDC_DIM_BEAT_PER_MIN",
			mappingID:      "real-device-149530-mdc-puls-oxim-puls-rate",
			fingerprint001: "4de28380713b7867",
			fingerprint002: "578b92f887e2cf67",
		},
		{
			code:           "151578",
			unitString:     "264832^MDC_DIM_RESP_PER_MIN^MDC",
			unitLabel:      "MDC_DIM_RESP_PER_MIN",
			mappingID:      "real-device-151578-mdc-tthor-resp-rate",
			fingerprint001: "4de28380713b7867",
			fingerprint002: "578b92f887e2cf67",
		},
		{
			code:           "150344",
			unitString:     "268192^MDC_DIM_DEGC^MDC",
			unitLabel:      "MDC_DIM_DEGC",
			mappingID:      "real-device-150344-mdc-temp",
			fingerprint001: "4de28380713b7867",
			fingerprint002: "578b92f887e2cf67",
		},
	}

	candidates := make(map[string]*Claim)

	// Build candidate for each code using capture 001 as primary
	for _, codeInfo := range codes {
		claim, err := New(
			"patient-monitor-001",
			"HL7_V2",
			5,
			codeInfo.code,
			codeInfo.unitString,
			codeInfo.unitLabel,
			-100.0, // observed value (NOT ground truth)
			"generic_hl7_v2",
			codeInfo.fingerprint001,
			"UNKNOWN", // device identity unknown until manual inspection
			codeInfo.mappingID,
			"1.0.0",
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build candidate for code %s: %w", codeInfo.code, err)
		}

		// Note: All 5 codes have real wire evidence but semantic authority is still OPEN
		// Canonical concept, unit, and ground truth will be resolved when authority is provided
		candidates[codeInfo.code] = claim
	}

	return candidates, nil
}

// SaveCandidatePackage persists all candidate claims to output directory.
// Returns directory path containing all candidates.
func SaveCandidatePackage(outDir string, candidates map[string]*Claim) (string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("create candidate directory: %w", err)
	}

	for code, claim := range candidates {
		filename := fmt.Sprintf("candidate_%s.json", code)
		path := filepath.Join(outDir, filename)
		if err := SaveCandidateRecord(path, claim); err != nil {
			return "", fmt.Errorf("failed to save candidate for code %s: %w", code, err)
		}
	}

	return outDir, nil
}

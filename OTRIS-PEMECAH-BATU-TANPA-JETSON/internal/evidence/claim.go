package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const ApprovedForMapping = "APPROVED_FOR_MAPPING"

// Claim is a comparable value binding evidence to one explicit mapping claim.
// ParserIdentity, DeviceFingerprint, and DeviceIdentityRef are optional context.
type Claim struct {
	MappingIdentity   string `json:"mapping_identity"`
	MappingVersion    string `json:"mapping_version"`
	SourceProtocol    string `json:"source_protocol"`
	SourcePosition    int    `json:"source_position"`
	SourceCode        string `json:"source_code"`
	SourceUnit        string `json:"source_unit"`
	CanonicalConcept  string `json:"canonical_concept"`
	CanonicalUnit     string `json:"canonical_unit"`
	ParserIdentity    string `json:"parser_identity"`
	DeviceFingerprint string `json:"device_fingerprint"`
	DeviceIdentityRef string `json:"device_identity_ref"`
}

// ClaimReview is input only; it cannot supply a verdict.
type ClaimReview struct {
	Claim    Claim      `json:"claim"`
	Evidence []Evidence `json:"evidence"`
}

// ClaimReport is report data, not an authorization credential.
type ClaimReport struct {
	Claim    Claim      `json:"claim"`
	Evidence []Evidence `json:"evidence"`
	Verdict  string     `json:"verdict"`
}

// ReviewClaim reads fresh evidence and persists its bound result before success.
func ReviewClaim(claim Claim, evidencePath, reportPath string) (ClaimReport, error) {
	report := ClaimReport{Claim: claim, Evidence: []Evidence{}, Verdict: "REJECTED"}
	if reportPath == "" {
		return report, errors.New("claim report path is required")
	}
	if err := validateClaim(claim); err != nil {
		return report, err
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		return report, fmt.Errorf("read claim evidence: %w", err)
	}
	if !utf8.Valid(data) {
		return report, errors.New("claim evidence is not valid UTF-8")
	}
	var bundle ClaimReview
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bundle); err != nil {
		return report, fmt.Errorf("decode claim evidence: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return report, errors.New("claim evidence has trailing data")
	}
	if bundle.Claim != claim {
		return report, errors.New("evidence claim does not exactly match requested claim")
	}
	if bundle.Evidence == nil {
		return report, errors.New("claim evidence must contain an explicit evidence array")
	}
	for i, ref := range bundle.Evidence {
		if strings.TrimSpace(ref.Type) == "" || strings.TrimSpace(ref.Ref) == "" ||
			!utf8.ValidString(ref.Type) || !utf8.ValidString(ref.Ref) || !utf8.ValidString(ref.Quote) {
			return report, fmt.Errorf("invalid evidence reference at index %d", i)
		}
	}
	report.Evidence = append([]Evidence{}, bundle.Evidence...)
	result := report
	// SEMANTIC VERIFICATION BOUNDARY (PB-14-semantic)
	// Count check first: 0 items → REJECTED, 1 item → NEEDS_MORE_EVIDENCE
	// Only for >=2 items, apply semantic verification: raw counts don't grant approval.
	switch len(result.Evidence) {
	case 0:
		result.Verdict = "REJECTED"
	case 1:
		result.Verdict = "NEEDS_MORE_EVIDENCE"
	default:
		// >=2 items: verify semantic authority requirement
		// At least one item must be semantic evidence (VENDOR_MANUAL) to establish clinical meaning.
		// Raw captures/simulator logs prove field existence, not semantics.
		result.Verdict = VerifySemanticEvidence(claim, result.Evidence)
	}
	f, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return report, fmt.Errorf("create claim report: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := errors.Join(enc.Encode(result), f.Close()); err != nil {
		return report, errors.Join(fmt.Errorf("write claim report: %w", err), os.Remove(reportPath))
	}
	return result, nil
}

func validateClaim(claim Claim) error {
	if claim.SourcePosition <= 0 {
		return errors.New("claim source_position must be positive")
	}
	for _, field := range []struct {
		name     string
		value    string
		required bool
	}{
		{"mapping_identity", claim.MappingIdentity, true},
		{"mapping_version", claim.MappingVersion, true},
		{"source_protocol", claim.SourceProtocol, true},
		{"source_code", claim.SourceCode, true},
		{"source_unit", claim.SourceUnit, false}, // MAY be empty for unitless observations
		{"canonical_concept", claim.CanonicalConcept, true},
		{"canonical_unit", claim.CanonicalUnit, true},
		{"parser_identity", claim.ParserIdentity, false},
		{"device_fingerprint", claim.DeviceFingerprint, false},
		{"device_identity_ref", claim.DeviceIdentityRef, false},
	} {
		if !utf8.ValidString(field.value) || (field.required && strings.TrimSpace(field.value) == "") {
			return fmt.Errorf("claim %s is missing or invalid UTF-8", field.name)
		}
	}
	return nil
}

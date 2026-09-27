package candidate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateClaimNew_ValidProvenance(t *testing.T) {
	claim, err := New(
		"patient-monitor-001",
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if claim == nil {
		t.Fatal("claim should not be nil")
	}
	if claim.SourceCode != "147842" {
		t.Errorf("expected source_code 147842, got %s", claim.SourceCode)
	}
	if claim.CanonicalConceptAuth != ResolutionOpen {
		t.Errorf("expected canonical_concept_auth OPEN, got %s", claim.CanonicalConceptAuth)
	}
	if claim.IsReady() {
		t.Error("IsReady() should return false (unresolved dependencies present)")
	}
}

func TestCandidateClaimNew_MissingProvenance(t *testing.T) {
	_, err := New(
		"", // empty capture ID
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	if err == nil {
		t.Error("expected error for missing captureID, got nil")
	}
}

func TestCandidateClaimNew_InvalidSourcePosition(t *testing.T) {
	_, err := New(
		"patient-monitor-001",
		"HL7_V2",
		0, // invalid: must be positive
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	if err == nil {
		t.Error("expected error for invalid sourcePosition, got nil")
	}
}

func TestCandidateClaimSetCanonicalConcept_Resolves(t *testing.T) {
	claim, _ := New(
		"patient-monitor-001",
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	if claim.CanonicalConceptAuth != ResolutionOpen {
		t.Error("initially canonical_concept_auth should be OPEN")
	}

	err := claim.SetCanonicalConcept("HR_BPM", "IEEE 11073-10406:2022 Section 6.3")
	if err != nil {
		t.Fatalf("SetCanonicalConcept failed: %v", err)
	}

	if claim.CanonicalConcept != "HR_BPM" {
		t.Errorf("expected canonical_concept HR_BPM, got %s", claim.CanonicalConcept)
	}
	if claim.CanonicalConceptAuth != ResolutionResolved {
		t.Errorf("expected canonical_concept_auth RESOLVED, got %s", claim.CanonicalConceptAuth)
	}
	if claim.CanonicalConceptRef != "IEEE 11073-10406:2022 Section 6.3" {
		t.Errorf("expected reference preserved, got %s", claim.CanonicalConceptRef)
	}
}

func TestCandidateClaimAwaitingAuthority_UpdatesOnResolve(t *testing.T) {
	claim, _ := New(
		"patient-monitor-001",
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	// Initially expecting 3 unresolved
	if len(claim.AwaitingAuthority) != 3 {
		t.Errorf("expected 3 awaiting authorities, got %d", len(claim.AwaitingAuthority))
	}

	// Resolve one
	claim.SetCanonicalConcept("HR_BPM", "IEEE 11073")

	if len(claim.AwaitingAuthority) != 2 {
		t.Errorf("after resolving concept, expected 2 awaiting, got %d", len(claim.AwaitingAuthority))
	}

	// Resolve another
	claim.SetCanonicalUnit("beats/min", "IEEE 11073")

	if len(claim.AwaitingAuthority) != 1 {
		t.Errorf("after resolving unit, expected 1 awaiting, got %d", len(claim.AwaitingAuthority))
	}

	// Resolve last one
	claim.SetGroundTruth(72.0, "reference_device", "Philips MP30 simultaneous measurement")

	if len(claim.AwaitingAuthority) != 0 {
		t.Errorf("after resolving all, expected 0 awaiting, got %d: %v", len(claim.AwaitingAuthority), claim.AwaitingAuthority)
	}

	if !claim.IsReady() {
		t.Error("IsReady() should return true when all fields resolved")
	}
}

func TestCandidateClaimPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "candidate_147842.json")

	// Create and save
	original, _ := New(
		"patient-monitor-001",
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)
	original.SetCanonicalConcept("HR_BPM", "IEEE 11073")

	err := SaveCandidateRecord(path, original)
	if err != nil {
		t.Fatalf("SaveCandidateRecord failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadCandidateRecord(path)
	if err != nil {
		t.Fatalf("LoadCandidateRecord failed: %v", err)
	}

	if loaded.SourceCode != "147842" {
		t.Errorf("loaded source_code mismatch: expected 147842, got %s", loaded.SourceCode)
	}
	if loaded.CanonicalConcept != "HR_BPM" {
		t.Errorf("loaded canonical_concept mismatch: expected HR_BPM, got %s", loaded.CanonicalConcept)
	}
	if loaded.CanonicalConceptAuth != ResolutionResolved {
		t.Errorf("loaded concept_auth mismatch: expected RESOLVED, got %s", loaded.CanonicalConceptAuth)
	}
}

func TestCandidateClaimCannotHaveEmptyCanonicalConcept(t *testing.T) {
	claim, _ := New(
		"patient-monitor-001",
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	err := claim.SetCanonicalConcept("", "dummy")
	if err == nil {
		t.Error("SetCanonicalConcept should reject empty concept")
	}
}

func TestCandidateClaimNormalizerIdentityLocked(t *testing.T) {
	claim, _ := New(
		"patient-monitor-001",
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0,
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	expected := "otris.normalizer.exact-evidence-mapping.v1"
	if claim.NormalizerIdentity != expected {
		t.Errorf("NormalizerIdentity not locked correctly: expected %s, got %s", expected, claim.NormalizerIdentity)
	}
}

func TestCandidateClaimObservedValueNotGroundTruth(t *testing.T) {
	claim, _ := New(
		"patient-monitor-001",
		"HL7_V2",
		5,
		"147842",
		"264864^MDC_DIM_BEAT_PER_MIN^MDC",
		"MDC_DIM_BEAT_PER_MIN",
		-100.0, // observed value (NOT ground truth)
		"generic_hl7_v2",
		"4de28380713b7867",
		"UNKNOWN",
		"siemens-infinity-147842-001",
		"1.0.0",
	)

	// Verify observed value is recorded separately
	if claim.ObservedValue != -100.0 {
		t.Errorf("ObservedValue should preserve -100, got %f", claim.ObservedValue)
	}

	// When ground truth is resolved, it's independent
	claim.SetGroundTruth(72.0, "reference_device", "test")
	if claim.GroundTruthValue != 72.0 {
		t.Errorf("GroundTruthValue should be 72, got %f", claim.GroundTruthValue)
	}

	// They should be different
	if claim.ObservedValue == claim.GroundTruthValue {
		t.Error("ObservedValue and GroundTruthValue should be different (observed may be placeholder)")
	}
}

func TestBuildRealDeviceCandidates_FiveCodesPresent(t *testing.T) {
	// Test that all 5 real codes can be built as candidates
	candidates, err := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")
	if err != nil {
		t.Fatalf("BuildRealDeviceCandidates failed: %v", err)
	}

	expectedCodes := []string{"147842", "150456", "149530", "151578", "150344"}
	if len(candidates) != 5 {
		t.Errorf("expected 5 candidates, got %d", len(candidates))
	}

	for _, code := range expectedCodes {
		if _, ok := candidates[code]; !ok {
			t.Errorf("missing candidate for code %s", code)
		}
	}
}

func TestBuildRealDeviceCandidates_WireProvenancePreserved(t *testing.T) {
	// Verify real wire provenance is preserved
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")

	c147842 := candidates["147842"]
	if c147842.SourceCode != "147842" {
		t.Errorf("source_code not preserved: %s", c147842.SourceCode)
	}
	if c147842.SourceUnit != "264864^MDC_DIM_BEAT_PER_MIN^MDC" {
		t.Errorf("source_unit not preserved: %s", c147842.SourceUnit)
	}
	if c147842.SourceUnitLabel != "MDC_DIM_BEAT_PER_MIN" {
		t.Errorf("source_unit_label not preserved: %s", c147842.SourceUnitLabel)
	}
	if c147842.ParserIdentity != "generic_hl7_v2" {
		t.Errorf("parser_identity not preserved: %s", c147842.ParserIdentity)
	}
	if c147842.SourcePosition != 5 {
		t.Errorf("source_position not preserved: %d", c147842.SourcePosition)
	}
}

func TestBuildRealDeviceCandidates_SemanticAuthorityOpen(t *testing.T) {
	// Verify that semantic authority starts as OPEN (not guessed)
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")

	for code, claim := range candidates {
		if claim.CanonicalConceptAuth != ResolutionOpen {
			t.Errorf("code %s canonical_concept_auth should be OPEN, got %s", code, claim.CanonicalConceptAuth)
		}
		if claim.CanonicalConcept != "" {
			t.Errorf("code %s canonical_concept should be empty (OPEN), got %s", code, claim.CanonicalConcept)
		}
		if !claim.isFieldOpen("canonical_concept") {
			t.Errorf("code %s canonical_concept not marked as open", code)
		}
	}
}

func TestBuildRealDeviceCandidates_GroundTruthOpen(t *testing.T) {
	// Verify ground truth starts as OPEN
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")

	for code, claim := range candidates {
		if claim.GroundTruthAuth != ResolutionOpen {
			t.Errorf("code %s ground_truth_auth should be OPEN, got %s", code, claim.GroundTruthAuth)
		}
		if claim.GroundTruthValue != 0.0 {
			t.Errorf("code %s ground_truth_value should be 0.0 (unset), got %f", code, claim.GroundTruthValue)
		}
		if !claim.isFieldOpen("ground_truth") {
			t.Errorf("code %s ground_truth not marked as open", code)
		}
	}
}

func TestBuildRealDeviceCandidates_NormalizerIdentityLocked(t *testing.T) {
	// Verify normalizer identity is locked (not derived from code/protocol/vendor)
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")

	expected := "otris.normalizer.exact-evidence-mapping.v1"
	for code, claim := range candidates {
		if claim.NormalizerIdentity != expected {
			t.Errorf("code %s normalizer_identity not locked: expected %s, got %s", code, expected, claim.NormalizerIdentity)
		}
	}
}

// Helper to check if a field is marked as open
func (c *Claim) isFieldOpen(fieldName string) bool {
	for _, field := range c.AwaitingAuthority {
		if field == fieldName {
			return true
		}
	}
	return false
}

func TestPromotionBoundary_RealCandidateRejected(t *testing.T) {
	// Real patient-monitor candidates must NOT cross boundary
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")

	c147842 := candidates["147842"]
	err := c147842.PromoteToEvidence()
	if err == nil {
		t.Error("PromoteToEvidence should fail for unresolved real candidate, got nil")
	}
	if !strings.Contains(err.Error(), "awaiting") {
		t.Errorf("expected 'awaiting' in error message, got: %v", err)
	}
}

func TestPromotionBoundary_UnresolvedConceptRejected(t *testing.T) {
	claim, _ := New(
		"test-capture",
		"HL7_V2",
		5,
		"TEST001",
		"test_unit",
		"test_unit_label",
		-100.0,
		"generic_hl7_v2",
		"abc123",
		"UNKNOWN",
		"test-mapping",
		"1.0.0",
	)

	// Resolve unit and GT, but leave concept OPEN
	claim.SetCanonicalUnit("test_unit", "authority")
	claim.SetGroundTruth(50.0, "test_source", "test_ref")

	err := claim.PromoteToEvidence()
	if err == nil {
		t.Error("PromoteToEvidence should fail when concept unresolved")
	}
	if !strings.Contains(err.Error(), "canonical_concept") {
		t.Errorf("expected canonical_concept error, got: %v", err)
	}
}

func TestPromotionBoundary_UnresolvedUnitRejected(t *testing.T) {
	claim, _ := New(
		"test-capture",
		"HL7_V2",
		5,
		"TEST001",
		"test_unit",
		"test_unit_label",
		-100.0,
		"generic_hl7_v2",
		"abc123",
		"UNKNOWN",
		"test-mapping",
		"1.0.0",
	)

	// Resolve concept and GT, but leave unit OPEN
	claim.SetCanonicalConcept("test_concept", "authority")
	claim.SetGroundTruth(50.0, "test_source", "test_ref")

	err := claim.PromoteToEvidence()
	if err == nil {
		t.Error("PromoteToEvidence should fail when unit unresolved")
	}
	if !strings.Contains(err.Error(), "canonical_unit") {
		t.Errorf("expected canonical_unit error, got: %v", err)
	}
}

func TestPromotionBoundary_UnresolvedGroundTruthRejected(t *testing.T) {
	claim, _ := New(
		"test-capture",
		"HL7_V2",
		5,
		"TEST001",
		"test_unit",
		"test_unit_label",
		-100.0,
		"generic_hl7_v2",
		"abc123",
		"UNKNOWN",
		"test-mapping",
		"1.0.0",
	)

	// Resolve concept and unit, but leave GT OPEN
	claim.SetCanonicalConcept("test_concept", "authority")
	claim.SetCanonicalUnit("test_unit", "authority")

	err := claim.PromoteToEvidence()
	if err == nil {
		t.Error("PromoteToEvidence should fail when ground_truth unresolved")
	}
	if !strings.Contains(err.Error(), "ground_truth") {
		t.Errorf("expected ground_truth error, got: %v", err)
	}
}

func TestPromotionBoundary_AllResolvedAccepted(t *testing.T) {
	claim, _ := New(
		"test-capture",
		"HL7_V2",
		5,
		"TEST001",
		"test_unit",
		"test_unit_label",
		-100.0,
		"generic_hl7_v2",
		"abc123def456",
		"test-device-ref",
		"test-mapping",
		"1.0.0",
	)

	// Resolve all three
	claim.SetCanonicalConcept("test_concept", "IEEE standard section X.Y.Z")
	claim.SetCanonicalUnit("test_unit", "IEEE standard section A.B.C")
	claim.SetGroundTruth(50.0, "reference_device", "Philips MP30 simultaneous measurement 2024-02-12")

	err := claim.PromoteToEvidence()
	if err != nil {
		t.Errorf("PromoteToEvidence should succeed when all resolved, got: %v", err)
	}
}

func TestPromotionBoundary_MissingAuthorityRefRejected(t *testing.T) {
	claim, _ := New(
		"test-capture",
		"HL7_V2",
		5,
		"TEST001",
		"test_unit",
		"test_unit_label",
		-100.0,
		"generic_hl7_v2",
		"abc123",
		"UNKNOWN",
		"test-mapping",
		"1.0.0",
	)

	// Resolve but with empty refs
	claim.CanonicalConcept = "test_concept"
	claim.CanonicalConceptAuth = ResolutionResolved
	claim.CanonicalConceptRef = "" // Empty ref!
	claim.CanonicalUnit = "test_unit"
	claim.CanonicalUnitAuth = ResolutionResolved
	claim.CanonicalUnitRef = "authority"
	claim.GroundTruthValue = 50.0
	claim.GroundTruthAuth = ResolutionResolved
	claim.GroundTruthRef = "test"
	claim.AwaitingAuthority = []string{}

	err := claim.PromoteToEvidence()
	if err == nil {
		t.Error("PromoteToEvidence should fail when authority ref is empty")
	}
	if !strings.Contains(err.Error(), "concept_ref") {
		t.Errorf("expected concept_ref error, got: %v", err)
	}
}

func TestPromotionBoundary_InvalidNormalizerIdentityRejected(t *testing.T) {
	claim, _ := New(
		"test-capture",
		"HL7_V2",
		5,
		"TEST001",
		"test_unit",
		"test_unit_label",
		-100.0,
		"generic_hl7_v2",
		"abc123",
		"UNKNOWN",
		"test-mapping",
		"1.0.0",
	)

	claim.SetCanonicalConcept("test_concept", "authority")
	claim.SetCanonicalUnit("test_unit", "authority")
	claim.SetGroundTruth(50.0, "test_source", "test_ref")

	// Corrupt normalizer identity
	claim.NormalizerIdentity = "wrong.normalizer.identity"

	err := claim.PromoteToEvidence()
	if err == nil {
		t.Error("PromoteToEvidence should fail when normalizer identity invalid")
	}
	if !strings.Contains(err.Error(), "normalizer") {
		t.Errorf("expected normalizer error, got: %v", err)
	}
}

func TestPromotionBoundary_SourceProvenanceRequired(t *testing.T) {
	claim, _ := New(
		"test-capture",
		"HL7_V2",
		5,
		"", // Empty source code!
		"test_unit",
		"test_unit_label",
		-100.0,
		"generic_hl7_v2",
		"abc123",
		"UNKNOWN",
		"test-mapping",
		"1.0.0",
	)

	// Even if we try to bypass New()...
	if claim != nil {
		t.Error("New() should have failed with empty source_code")
	}
}

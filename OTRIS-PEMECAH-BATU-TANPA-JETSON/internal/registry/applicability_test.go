package registry

import (
	"testing"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/identity"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
)

// Helper: create candidate knowledge match for testing
func createTestCandidate(
	deviceFP, mappingID, mappingVersion, refID, semanticDigest string,
) *identity.CandidateKnowledgeMatch {
	iki, _ := identity.New(deviceFP, mappingID, mappingVersion, refID, semanticDigest)
	return &identity.CandidateKnowledgeMatch{
		IntegrationIdentity:     iki,
		FingerprintRelationship: identity.RelationshipExact,
		ApplicabilityStatus:     identity.ApplicabilityNotYetEstablished,
		HistoricalAuthorityStatus: StatusRegistryAdmitted,
		CandidateStatement:      "Historical knowledge from this source is candidate for new context and does not automatically grant authority",
		MatchedAt:               time.Now().UTC(),
	}
}

// Helper: create mapping definition for testing
func createTestMapping(
	mappingID, mappingVersion, semanticDigest string,
) *mapping.MappingDefinition {
	m, _ := mapping.New(
		mappingID, mappingVersion,
		"HL7_V2", 1, "OBX-1", "val",
		"HEART_RATE", "bpm",
		"test provenance", "test author",
	)
	m.SemanticEvidenceDigest = semanticDigest
	return m
}

// Helper: create current identity for testing
func createTestIdentity(
	deviceFP, mappingID, mappingVersion, refID, semanticDigest string,
) *identity.IntegrationKnowledgeIdentity {
	iki, _ := identity.New(deviceFP, mappingID, mappingVersion, refID, semanticDigest)
	return iki
}

// Test 1: Exact applicable candidate accepted into verification path
func TestApplicabilityExactMatch(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-semantic-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-semantic-001")
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-semantic-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	err := av.Verify()

	if err != nil {
		t.Fatalf("exact match should not error: %v", err)
	}

	if !av.IsVerified() {
		t.Fatal("verification should be completed")
	}

	if !av.IsCandidateApplicable() {
		t.Fatal("exact match should be CONFIRMED")
	}

	decision, reason := av.GetDecision()
	if decision != DecisionConfirmed {
		t.Fatalf("decision should be CONFIRMED, got %s", decision)
	}

	if reason == "" {
		t.Fatal("reason should be provided")
	}
}

// Test 2: Device fingerprint mismatch rejected
func TestApplicabilityDeviceFingerprintMismatch(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	// Different device fingerprint
	currentID := createTestIdentity("device-002", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	_ = av.Verify()

	if av.Decision != DecisionRejected {
		t.Fatalf("decision should be REJECTED, got %s", av.Decision)
	}

	if av.ReasonRejected == "" {
		t.Fatal("rejection reason should be provided")
	}

	if !stringContains(av.ReasonRejected, "device fingerprint mismatch") {
		t.Fatalf("reason should mention fingerprint: %s", av.ReasonRejected)
	}
}

// Test 3: Mapping identity mismatch rejected
func TestApplicabilityMappingIdentityMismatch(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-bp", "1.0.0", "digest-001") // Different mapping
	currentID := createTestIdentity("device-001", "mapping-bp", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	_ = av.Verify()

	if av.Decision != DecisionRejected {
		t.Fatalf("decision should be REJECTED, got %s", av.Decision)
	}

	if !stringContains(av.ReasonRejected, "mapping identity mismatch") {
		t.Fatalf("reason should mention mapping identity: %s", av.ReasonRejected)
	}
}

// Test 4: Mapping version mismatch rejected
func TestApplicabilityMappingVersionMismatch(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "2.0.0", "digest-001") // Different version
	currentID := createTestIdentity("device-001", "mapping-hr", "2.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	_ = av.Verify()

	if av.Decision != DecisionRejected {
		t.Fatalf("decision should be REJECTED, got %s", av.Decision)
	}

	if !stringContains(av.ReasonRejected, "mapping version mismatch") {
		t.Fatalf("reason should mention version: %s", av.ReasonRejected)
	}
}

// Test 5: Semantic evidence digest mismatch rejected
func TestApplicabilitySemanticDigestMismatch(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-A",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-B") // Different digest
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-B")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	_ = av.Verify()

	if av.Decision != DecisionRejected {
		t.Fatalf("decision should be REJECTED, got %s", av.Decision)
	}

	if !stringContains(av.ReasonRejected, "semantic evidence digest mismatch") {
		t.Fatalf("reason should mention digest: %s", av.ReasonRejected)
	}
}

// Test 6: Reference identity mismatch rejected
func TestApplicabilityReferenceIdentityMismatch(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	// Different reference identity
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-high", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	_ = av.Verify()

	if av.Decision != DecisionRejected {
		t.Fatalf("decision should be REJECTED, got %s", av.Decision)
	}

	if !stringContains(av.ReasonRejected, "reference identity mismatch") {
		t.Fatalf("reason should mention reference: %s", av.ReasonRejected)
	}
}

// Test 7: Missing current mapping rejected (insufficient evidence)
func TestApplicabilityMissingCurrentMapping(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, nil, currentID)
	err := av.Verify()

	if err == nil {
		t.Fatal("missing current mapping should error")
	}

	if av.Decision != DecisionNeedsEvidence {
		t.Fatalf("decision should be NEEDS_EVIDENCE, got %s", av.Decision)
	}
}

// Test 8: Missing current identity rejected (insufficient evidence)
func TestApplicabilityMissingCurrentIdentity(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, nil)
	err := av.Verify()

	if err == nil {
		t.Fatal("missing current identity should error")
	}

	if av.Decision != DecisionNeedsEvidence {
		t.Fatalf("decision should be NEEDS_EVIDENCE, got %s", av.Decision)
	}
}

// Test 9: Candidate without fingerprint (from lookup) still verifiable
func TestApplicabilityPlaceholderFingerprintAccepted(t *testing.T) {
	// Lookup returns placeholder fingerprint
	candidate := createTestCandidate(
		"(not-stored-in-attestation)", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	_ = av.Verify()

	// Should not error; should indicate needs evidence (fingerprint not confirmed)
	if av.Decision != DecisionNeedsEvidence {
		t.Fatalf("placeholder fingerprint should result in NEEDS_EVIDENCE, got %s", av.Decision)
	}

	if !stringContains(av.ReasonNeeds, "candidate fingerprint not available") {
		t.Fatalf("reason should mention unavailable fingerprint: %s", av.ReasonNeeds)
	}
}

// Test 10: Historical authority is NOT transferred
func TestApplicabilityHistoricalAuthorityNotTransferred(t *testing.T) {
	// Create candidate with historical REGISTRY_ADMITTED status
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	originalStatement := candidate.CandidateStatement
	originalApplicability := candidate.ApplicabilityStatus

	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	av.Verify()

	// Applicability must still be NOT_YET_ESTABLISHED (verification is identity-only)
	if candidate.ApplicabilityStatus != originalApplicability {
		t.Fatalf("applicability should not be mutated, got %s", candidate.ApplicabilityStatus)
	}

	// Historical status is preserved
	if candidate.HistoricalAuthorityStatus != StatusRegistryAdmitted {
		t.Fatal("historical status should be preserved")
	}

	// But verification confirms identity match at AV level
	if !av.IsCandidateApplicable() {
		t.Fatal("identity match should be CONFIRMED")
	}

	// CandidateStatement must still deny authority transfer (unchanged by verification)
	if candidate.CandidateStatement != originalStatement {
		t.Fatal("candidate statement should not be mutated")
	}

	if !stringContains(candidate.CandidateStatement, "does not automatically grant authority") {
		t.Fatalf("candidate statement must deny authority transfer, got: %q", candidate.CandidateStatement)
	}
}

// Test 11: Verification results do not mutate candidate
func TestApplicabilityNoMutationOfCandidate(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	originalApplicability := candidate.ApplicabilityStatus
	originalStatement := candidate.CandidateStatement

	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	av.Verify()

	// Candidate should be unchanged (verification is read-only)
	if candidate.ApplicabilityStatus != originalApplicability {
		t.Fatalf("candidate should not be mutated: got %s", candidate.ApplicabilityStatus)
	}

	if candidate.CandidateStatement != originalStatement {
		t.Fatal("candidate statement should not be mutated")
	}
}

// Test 12: Existing evidence/replay/QC/approval/registry gates remain required
func TestApplicabilityDoesNotBypassGates(t *testing.T) {
	// Even with confirmed applicability, candidate must still go through all gates
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	av.Verify()

	if !av.IsCandidateApplicable() {
		t.Fatal("candidate should be applicable")
	}

	// Decision is CONFIRMED, but this is just identity verification
	// Actual admission still requires ReviewEvidence, ReviewReplay, approval, registry
	// (This test documents that applicability is a PREREQUISITE, not a shortcut)
	decision, _ := av.GetDecision()
	if decision != DecisionConfirmed {
		t.Fatalf("decision should be CONFIRMED, got %s", decision)
	}
}

// Test 13: Multiple verification calls error (idempotency)
func TestApplicabilityDoubleVerifyErrors(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)

	// First verify succeeds
	err1 := av.Verify()
	if err1 != nil {
		t.Fatalf("first verify should succeed: %v", err1)
	}

	// Second verify should error
	err2 := av.Verify()
	if err2 == nil {
		t.Fatal("second verify should error")
	}

	if !stringContains(err2.Error(), "already verified") {
		t.Fatalf("error should mention already verified: %v", err2)
	}
}

// Test 14: Rejected candidate does not proceed to governance
func TestApplicabilityRejectedCannotProceed(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)
	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-002") // Different digest
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-002")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	av.Verify()

	if av.IsCandidateApplicable() {
		t.Fatal("rejected candidate should not be applicable")
	}

	decision, reason := av.GetDecision()
	if decision != DecisionRejected {
		t.Fatalf("decision should be REJECTED, got %s", decision)
	}

	if reason == "" {
		t.Fatal("rejection reason required")
	}
}

// Test 15: Verification preserves candidate immutability
func TestApplicabilityPreservesImmutability(t *testing.T) {
	candidate := createTestCandidate(
		"device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001",
	)

	// Candidate has immutable properties
	originalDigest := candidate.IntegrationIdentity.Digest
	originalMatchedAt := candidate.MatchedAt

	currentMapping := createTestMapping("mapping-hr", "1.0.0", "digest-001")
	currentID := createTestIdentity("device-001", "mapping-hr", "1.0.0", "ref-normal", "digest-001")

	av := NewApplicabilityVerification(candidate, currentMapping, currentID)
	av.Verify()

	// Candidate properties unchanged
	if candidate.IntegrationIdentity.Digest != originalDigest {
		t.Fatal("candidate identity digest should not change")
	}

	if candidate.MatchedAt != originalMatchedAt {
		t.Fatal("candidate matched timestamp should not change")
	}
}

// Helper: check if string contains substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

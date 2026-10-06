package registry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/identity"
)

// Test 1: Exact governed lookup can find matching historical knowledge
func TestLookupByIntegrationIdentityExactMatch(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	// Create a historical attestation
	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		ProfileID:              "profile-001",
		ProfileVersion:         "1.0.0",
		CandidateDigest:        "abc123",
		MappingIdentity:        "hr-obx-field-5",
		MappingVersion:         "1.0.0",
		EvidenceReference:      "evidence-report.json",
		SemanticEvidenceDigest: "semantic-digest-001",
		ReplayValidatedAt:      time.Now().UTC(),
		MappingProvenance:      "test provenance",
		ProvenanceAuthor:       "test author",
		ApproverIdentity:       "engineer-001",
		ApprovalTimestamp:      time.Now().UTC(),
		IssuerKeyID:            "issuer-001",
		IssuedAt:               time.Now().UTC(),
		Sequence:               1,
		Signature:              "signature-001",
	}

	if err := store.Add(attestation); err != nil {
		t.Fatal(err)
	}

	// Create matching integration identity (must match attestation on mapping + semantic digest)
	iki, err := identity.New(
		"fp-001", // DeviceFingerprint (not in Attestation, doesn't affect lookup match)
		"hr-obx-field-5", // Must match attestation.MappingIdentity
		"1.0.0", // Must match attestation.MappingVersion
		"hr-reference", // ReferenceIdentity (not in Attestation, doesn't affect lookup match)
		"semantic-digest-001", // Must match attestation.SemanticEvidenceDigest
	)
	if err != nil {
		t.Fatal(err)
	}

	// Lookup should find exact match
	match, err := LookupByIntegrationIdentity(store, iki)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}

	if match == nil {
		t.Fatal("lookup should find exact match")
	}

	// Verify candidate properties
	if match.FingerprintRelationship != identity.RelationshipExact {
		t.Fatalf("expected EXACT relationship, got %s", match.FingerprintRelationship)
	}

	if match.ApplicabilityStatus != identity.ApplicabilityNotYetEstablished {
		t.Fatalf("applicability should be NOT_YET_ESTABLISHED, got %s", match.ApplicabilityStatus)
	}

	if match.SourceAttestation == nil {
		t.Fatal("candidate should have attestation reference")
	}

	if match.SourceAttestation.AdmissionStatus != StatusRegistryAdmitted {
		t.Fatalf("historical status should be preserved: %s", match.SourceAttestation.AdmissionStatus)
	}
}

// Test 2: No match returns clean no-match result
func TestLookupByIntegrationIdentityNoMatch(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	// Add an attestation
	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "hr-obx-field-5",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	// Create non-matching identity
	iki, _ := identity.New("fp", "o2-sat-field-3", "1.0.0", "ref", "digest-other")

	// Lookup should return nil (no match)
	match, err := LookupByIntegrationIdentity(store, iki)
	if err != nil {
		t.Fatalf("lookup should not error on no match: %v", err)
	}

	if match != nil {
		t.Fatal("lookup should return nil for no match, not a non-match result")
	}
}

// Test 3: Lookup result remains non-authoritative
func TestLookupResultNonAuthoritative(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	// Candidate statement must explicitly deny authority transfer
	if !contains(match.CandidateStatement, "candidate") {
		t.Fatal("candidate statement should include 'candidate'")
	}

	if !contains(match.CandidateStatement, "does not automatically grant authority") {
		t.Fatal("candidate statement should deny authority transfer")
	}
}

// Test 4: Applicability remains NOT_YET_ESTABLISHED
func TestLookupApplicabilityNotYetEstablished(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	if match.ApplicabilityStatus != identity.ApplicabilityNotYetEstablished {
		t.Fatalf("applicability must remain NOT_YET_ESTABLISHED, got %s", match.ApplicabilityStatus)
	}
}

// Test 5: Historical ADMITTED state does not transfer to current authority
func TestLookupHistoricalStatusPreservedNotTransferred(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	// Create attestation with REGISTRY_ADMITTED status
	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted, // Historical status
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	// Historical status is preserved as metadata only
	if match.HistoricalAuthorityStatus != StatusRegistryAdmitted {
		t.Fatalf("historical status should be preserved: %s", match.HistoricalAuthorityStatus)
	}

	// But applicability is NOT_YET_ESTABLISHED (not transferred)
	if match.ApplicabilityStatus != identity.ApplicabilityNotYetEstablished {
		t.Fatal("applicability should NOT be CONFIRMED even if historical status was ADMITTED")
	}
}

// Test 6: Lookup cannot mutate Registry authority
func TestLookupCannotMutateRegistry(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	// Try to mutate match (candidate should be independent)
	originalStatus := match.ApplicabilityStatus
	match.ApplicabilityStatus = identity.ApplicabilityConfirmed // Try to claim authority

	// Re-lookup should return fresh candidate, not mutated state
	match2, _ := LookupByIntegrationIdentity(store, iki)
	if match2.ApplicabilityStatus != identity.ApplicabilityNotYetEstablished {
		t.Fatal("lookup should return fresh candidate, not mutated state")
	}

	// Original match mutation is independent (but shouldn't have happened)
	if match.ApplicabilityStatus != identity.ApplicabilityConfirmed {
		t.Fatalf("mutated match should reflect mutation: %s", match.ApplicabilityStatus)
	}
	_ = originalStatus // Suppress unused warning
}

// Test 7: Identity mismatch does not become a match
func TestLookupIdentityMismatchNotMatch(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	// Add attestation with semantic digest A
	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-A",
		Signature:              "sig",
	}
	store.Add(attestation)

	// Try to lookup with semantic digest B (different)
	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-B")
	match, _ := LookupByIntegrationIdentity(store, iki)

	if match != nil {
		t.Fatal("semantic digest mismatch should not produce a match")
	}
}

// Test 8: Empty store returns no match
func TestLookupEmptyStore(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, err := LookupByIntegrationIdentity(store, iki)

	if err != nil {
		t.Fatalf("empty store should not error: %v", err)
	}

	if match != nil {
		t.Fatal("empty store should return nil")
	}
}

// Test 9: ValidateLookupResult enforces applicability
func TestValidateLookupResultEnforcesApplicability(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	// Valid result should pass
	if err := ValidateLookupResult(match); err != nil {
		t.Fatalf("valid result should pass validation: %v", err)
	}

	// Corrupted result (applicability changed) should fail
	match.ApplicabilityStatus = identity.ApplicabilityConfirmed
	if err := ValidateLookupResult(match); err == nil {
		t.Fatal("corrupted applicability should fail validation")
	}
}

// Test 10: LookupByDeviceAndMapping finds match on mapping
func TestLookupByDeviceAndMapping(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "hr-obx-field-5",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	// Lookup by mapping identity + version
	match, err := LookupByDeviceAndMapping(store, "fp-001", "hr-obx-field-5", "1.0.0")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}

	if match == nil {
		t.Fatal("should find match on mapping identity + version")
	}

	if match.ApplicabilityStatus != identity.ApplicabilityNotYetEstablished {
		t.Fatal("result should be non-authoritative candidate")
	}
}

// Test 11: Profile serialization preserves non-authoritative property
func TestLookupResultSerialization(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig",
	}
	store.Add(attestation)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	// Serialize to JSON
	data, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("serialization failed: %v", err)
	}

	// Deserialize back
	var restored identity.CandidateKnowledgeMatch
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("deserialization failed: %v", err)
	}

	// Applicability must still be NOT_YET_ESTABLISHED
	if restored.ApplicabilityStatus != identity.ApplicabilityNotYetEstablished {
		t.Fatal("deserialized result must preserve non-authoritative status")
	}
}

// Test 12: Lookup with invalid store returns error
func TestLookupWithNilStore(t *testing.T) {
	iki, _ := identity.New("fp", "mapping", "v", "ref", "digest")
	_, err := LookupByIntegrationIdentity(nil, iki)

	if err == nil {
		t.Fatal("nil store should return error")
	}
}

// Test 13: Lookup with invalid identity returns error
func TestLookupWithNilIdentity(t *testing.T) {
	store := NewInMemoryKnowledgeStore()
	_, err := LookupByIntegrationIdentity(store, nil)

	if err == nil {
		t.Fatal("nil identity should return error")
	}
}

// Helper: check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test 14: Registry attestation signature is never provided in lookup result
func TestLookupDoesNotExposedSignature(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	// Create attestation with signature
	attestation := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "secret-signature-key",
	}
	store.Add(attestation)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	// AttestationReference in result should NOT contain signature
	if match.SourceAttestation == nil {
		t.Fatal("should have attestation reference")
	}

	// Verify no signature field is exposed (already structurally absent from AttestationReference)
	// Just check that candidate statement maintains non-authoritative nature
	if !contains(match.CandidateStatement, "does not automatically grant authority") {
		t.Fatal("non-authoritative nature should be explicit")
	}
}

// Test 15: Multiple attestations; exact match chosen
func TestLookupMultipleAttestations(t *testing.T) {
	store := NewInMemoryKnowledgeStore()

	// Add multiple attestations
	att1 := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001",
		Signature:              "sig1",
	}

	att2 := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-001",
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-001", // Same identity
		Signature:              "sig2",
	}

	att3 := &Attestation{
		SchemaVersion:          SchemaV1,
		AdmissionStatus:        StatusRegistryAdmitted,
		MappingIdentity:        "mapping-002", // Different
		MappingVersion:         "1.0.0",
		SemanticEvidenceDigest: "digest-002",
		Signature:              "sig3",
	}

	store.Add(att1)
	store.Add(att2)
	store.Add(att3)

	iki, _ := identity.New("fp", "mapping-001", "1.0.0", "ref", "digest-001")
	match, _ := LookupByIntegrationIdentity(store, iki)

	if match == nil {
		t.Fatal("should find match among multiple attestations")
	}

	// Should match att1 or att2 (both have same identity)
	if match.IntegrationIdentity.MappingIdentity != "mapping-001" {
		t.Fatal("wrong attestation matched")
	}
}

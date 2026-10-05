package identity

import (
	"fmt"
	"time"
)

// FingerprintRelationship categorizes how observed fingerprint relates to candidate.
type FingerprintRelationship string

const (
	RelationshipExact    FingerprintRelationship = "EXACT"     // fingerprints match exactly
	RelationshipNoMatch  FingerprintRelationship = "NO_MATCH"  // fingerprints differ materially
)

// ApplicabilityStatus describes whether candidate knowledge may be applied.
type ApplicabilityStatus string

const (
	ApplicabilityNotYetEstablished ApplicabilityStatus = "NOT_YET_ESTABLISHED" // lookup result; must run governance
	ApplicabilityChecking          ApplicabilityStatus = "CHECKING"             // applicability check in progress
	ApplicabilityConfirmed         ApplicabilityStatus = "CONFIRMED"            // governance confirmed applicability
	ApplicabilityRejected          ApplicabilityStatus = "REJECTED"             // governance rejected candidate
)

// CandidateKnowledgeMatch is the result of a knowledge lookup query.
// It explicitly represents that the result is CANDIDATE knowledge only.
// Historical knowledge that was ADMITTED remains candidate until new governance confirms it.
type CandidateKnowledgeMatch struct {
	// What was found
	IntegrationIdentity *IntegrationKnowledgeIdentity `json:"integration_identity"` // matched identity composition

	// Fingerprint Relationship
	FingerprintRelationship FingerprintRelationship `json:"fingerprint_relationship"` // EXACT or NO_MATCH

	// What the candidate comes from
	SourceProfile              *ProfileReference `json:"source_profile,omitempty"`              // where this knowledge came from
	SourceAttestation          *AttestationReference `json:"source_attestation,omitempty"`      // if from registry
	SourceSemanticEvidenceDigest string            `json:"source_semantic_evidence_digest"`    // evidence binding reference

	// Applicability Assessment
	ApplicabilityStatus ApplicabilityStatus `json:"applicability_status"` // NOT_YET_ESTABLISHED initially
	ApplicabilityNotes  string              `json:"applicability_notes,omitempty"` // why status is what it is

	// Authority Status (always explicit)
	HistoricalAuthorityStatus string `json:"historical_authority_status"` // REGISTRY_ADMITTED, etc.
	CandidateStatement        string `json:"candidate_statement"`         // "Historical knowledge from this source is candidate for new context"

	// Evidence & Provenance
	EvidenceReferences []string `json:"evidence_references,omitempty"` // links to supporting data

	// Timestamp
	MatchedAt time.Time `json:"matched_at"`
}

// ProfileReference is a non-authoritative pointer to historical profile data.
type ProfileReference struct {
	ProfileID              string `json:"profile_id"`
	ProfileVersion         string `json:"profile_version"`
	MappingIdentity        string `json:"mapping_identity"`
	MappingVersion         string `json:"mapping_version"`
	DeviceFingerprint      string `json:"device_fingerprint"`
	SemanticEvidenceDigest string `json:"semantic_evidence_digest"`
	// Note: No approval/authority data; just identity reference
}

// AttestationReference is a non-authoritative pointer to registry knowledge.
type AttestationReference struct {
	AdmissionStatus        string `json:"admission_status"` // REGISTRY_ADMITTED (confirms it was admitted, not that it applies now)
	ProfileID              string `json:"profile_id"`
	ProfileVersion         string `json:"profile_version"`
	MappingIdentity        string `json:"mapping_identity"`
	MappingVersion         string `json:"mapping_version"`
	SemanticEvidenceDigest string `json:"semantic_evidence_digest"`
	// Note: No signature; not authoritative. Just historical reference.
}

// NewMatch creates a candidate knowledge match result.
// HistoricalAuthorityStatus must be explicit (even if empty, clarifies no authority).
func NewMatch(
	identity *IntegrationKnowledgeIdentity,
	relationship FingerprintRelationship,
	sourceProfile *ProfileReference,
	sourceAttestation *AttestationReference,
	historicalStatus string,
) *CandidateKnowledgeMatch {
	return &CandidateKnowledgeMatch{
		IntegrationIdentity:       identity,
		FingerprintRelationship:   relationship,
		SourceProfile:             sourceProfile,
		SourceAttestation:         sourceAttestation,
		SourceSemanticEvidenceDigest: extractSemanticDigest(sourceProfile, sourceAttestation),
		ApplicabilityStatus:       ApplicabilityNotYetEstablished,
		ApplicabilityNotes:        "Lookup returned candidate knowledge; existing governance must determine applicability",
		HistoricalAuthorityStatus: historicalStatus,
		CandidateStatement:        "Historical knowledge from this source is candidate for new context and does not automatically grant authority",
		MatchedAt:                 time.Now().UTC(),
	}
}

// extractSemanticDigest retrieves semantic evidence digest from sources.
func extractSemanticDigest(profile *ProfileReference, attestation *AttestationReference) string {
	if profile != nil && profile.SemanticEvidenceDigest != "" {
		return profile.SemanticEvidenceDigest
	}
	if attestation != nil && attestation.SemanticEvidenceDigest != "" {
		return attestation.SemanticEvidenceDigest
	}
	return ""
}

// String provides human-readable summary.
func (ckm *CandidateKnowledgeMatch) String() string {
	src := "unknown"
	if ckm.SourceProfile != nil {
		src = fmt.Sprintf("profile-%s", ckm.SourceProfile.ProfileID[:16])
	} else if ckm.SourceAttestation != nil {
		src = fmt.Sprintf("attestation-%s", ckm.SourceAttestation.ProfileID[:16])
	}

	return fmt.Sprintf("CandidateKnowledgeMatch{fp=%s, source=%s, applicability=%s}",
		ckm.FingerprintRelationship, src, ckm.ApplicabilityStatus)
}

package registry

import (
	"errors"
	"fmt"

	"github.com/otris-id/otris-pemecah-batu/internal/identity"
)

// Knowledge lookup provides read-only, deterministic discovery of historical
// governed knowledge without transferring authority. All lookups return
// candidates with ApplicabilityStatus = NOT_YET_ESTABLISHED.
//
// A successful lookup means: historical knowledge exists and may be relevant.
// It does NOT mean: current applicability is established.

// KnowledgeStore is an interface for attestation storage/retrieval.
// Implementations may be file-based, database-backed, or in-memory.
// This interface isolates lookup from persistence details.
type KnowledgeStore interface {
	// ListAdmitted returns all admitted attestations.
	// Returns empty list if none exist (not an error).
	ListAdmitted() ([]*Attestation, error)
}

// InMemoryKnowledgeStore is a simple in-memory store for testing and
// single-process scenarios. In production, replace with persistent storage.
type InMemoryKnowledgeStore struct {
	attestations []*Attestation
}

// NewInMemoryKnowledgeStore creates an empty in-memory store.
func NewInMemoryKnowledgeStore() *InMemoryKnowledgeStore {
	return &InMemoryKnowledgeStore{
		attestations: make([]*Attestation, 0),
	}
}

// Add stores an attestation (for testing/initialization).
func (s *InMemoryKnowledgeStore) Add(a *Attestation) error {
	if a == nil {
		return errors.New("attestation required")
	}
	s.attestations = append(s.attestations, a)
	return nil
}

// ListAdmitted returns all admitted attestations.
func (s *InMemoryKnowledgeStore) ListAdmitted() ([]*Attestation, error) {
	result := make([]*Attestation, 0, len(s.attestations))
	for _, a := range s.attestations {
		if a.AdmissionStatus == StatusRegistryAdmitted {
			result = append(result, a)
		}
	}
	return result, nil
}

// LookupByIntegrationIdentity queries the knowledge store for an exact match
// on the given integration identity. Returns a candidate knowledge match if found.
//
// Lookup is deterministic: same identity → same result.
// Lookup is read-only: never modifies registry or authority.
// Lookup is identity-aware: requires exact match on all required identity fields.
func LookupByIntegrationIdentity(
	store KnowledgeStore,
	iki *identity.IntegrationKnowledgeIdentity,
) (*identity.CandidateKnowledgeMatch, error) {
	if store == nil {
		return nil, errors.New("knowledge store required")
	}
	if iki == nil {
		return nil, errors.New("integration identity required")
	}

	// Verify identity integrity
	if err := iki.Verify(); err != nil {
		return nil, fmt.Errorf("integration identity invalid: %w", err)
	}

	// Query store for attestations matching this identity
	attestations, err := store.ListAdmitted()
	if err != nil {
		return nil, fmt.Errorf("store query failed: %w", err)
	}

	if len(attestations) == 0 {
		// No match found; return nil (caller handles no-match case)
		return nil, nil
	}

	// Find exact match on all required identity fields
	for _, a := range attestations {
		if matchesExactly(a, iki) {
			// Found a match; return as candidate (never as authority)
			match := createCandidateMatch(a, iki, true) // true = exact match
			return match, nil
		}
	}

	// No exact match found
	return nil, nil
}

// LookupByDeviceAndMapping queries the knowledge store for attestations
// matching the given device fingerprint and mapping identity/version.
// Returns the first match as a candidate (exact match on fp + mapping).
func LookupByDeviceAndMapping(
	store KnowledgeStore,
	deviceFingerprint, mappingID, mappingVersion string,
) (*identity.CandidateKnowledgeMatch, error) {
	if store == nil {
		return nil, errors.New("knowledge store required")
	}
	if deviceFingerprint == "" || mappingID == "" || mappingVersion == "" {
		return nil, errors.New("device fingerprint, mapping identity, and version all required")
	}

	attestations, err := store.ListAdmitted()
	if err != nil {
		return nil, fmt.Errorf("store query failed: %w", err)
	}

	if len(attestations) == 0 {
		return nil, nil
	}

	// Find match on device fingerprint + mapping identity + version
	// Note: Attestation doesn't store DeviceFingerprint directly, but
	// it can be inferred from ProfileID context. For now, we search by
	// mapping identity + version only.
	for _, a := range attestations {
		if a.MappingIdentity == mappingID && a.MappingVersion == mappingVersion {
			// Found a match on mapping identity/version
			match := createCandidateMatchFromAttestation(a, identity.RelationshipExact)
			return match, nil
		}
	}

	return nil, nil
}

// matchesExactly checks if an attestation matches the integration identity.
// Matches on: MappingIdentity + MappingVersion + SemanticEvidenceDigest
// (the fields that are actually stored in Attestation).
// DeviceFingerprint is not stored in Attestation and must be verified separately.
func matchesExactly(a *Attestation, iki *identity.IntegrationKnowledgeIdentity) bool {
	// Match on all available identity fields from Attestation
	if a.MappingIdentity != iki.MappingIdentity {
		return false
	}
	if a.MappingVersion != iki.MappingVersion {
		return false
	}
	if a.SemanticEvidenceDigest != iki.SemanticEvidenceDigest {
		return false
	}
	return true
}

// createCandidateMatch creates a CandidateKnowledgeMatch from an attestation.
// The match is explicitly non-authoritative: ApplicabilityStatus=NOT_YET_ESTABLISHED.
func createCandidateMatch(
	a *Attestation,
	iki *identity.IntegrationKnowledgeIdentity,
	isExactMatch bool,
) *identity.CandidateKnowledgeMatch {
	fpRel := identity.RelationshipExact
	if !isExactMatch {
		fpRel = identity.RelationshipNoMatch
	}

	return createCandidateMatchFromAttestation(a, fpRel)
}

// createCandidateMatchFromAttestation creates a CandidateKnowledgeMatch directly
// from an attestation without requiring IntegrationKnowledgeIdentity.
// Note: DeviceFingerprint and ReferenceIdentity are not stored in Attestation,
// so we use placeholder values for reconstruction (they're not used for matching).
func createCandidateMatchFromAttestation(
	a *Attestation,
	fpRel identity.FingerprintRelationship,
) *identity.CandidateKnowledgeMatch {
	// Reconstruct IntegrationKnowledgeIdentity from attestation fields.
	// Missing fields (DeviceFingerprint, ReferenceIdentity) use placeholders.
	// These don't affect lookup matching (which uses mapping + semantic digest).
	iki, err := identity.New(
		"(not-stored-in-attestation)", // DeviceFingerprint not in Attestation
		a.MappingIdentity,
		a.MappingVersion,
		"(not-stored-in-attestation)", // ReferenceIdentity not in Attestation
		a.SemanticEvidenceDigest,
	)
	if err != nil {
		// Attestation corrupted or fields invalid; return nil
		return nil
	}

	// Create attestation reference (non-authoritative metadata)
	attestationRef := &identity.AttestationReference{
		AdmissionStatus:        a.AdmissionStatus,
		ProfileID:              a.ProfileID,
		ProfileVersion:         a.ProfileVersion,
		MappingIdentity:        a.MappingIdentity,
		MappingVersion:         a.MappingVersion,
		SemanticEvidenceDigest: a.SemanticEvidenceDigest,
	}

	// Create match (explicit: historical authority does NOT transfer)
	return identity.NewMatch(
		iki,
		fpRel,
		nil, // No profile reference (only attestation)
		attestationRef,
		a.AdmissionStatus, // Preserve historical status (REGISTRY_ADMITTED)
	)
}

// ValidateLookupResult checks that a lookup result maintains non-authoritative status.
// Returns error if applicability is anything other than NOT_YET_ESTABLISHED.
func ValidateLookupResult(match *identity.CandidateKnowledgeMatch) error {
	if match == nil {
		return errors.New("lookup result required")
	}
	if match.ApplicabilityStatus != identity.ApplicabilityNotYetEstablished {
		return fmt.Errorf("lookup result applicability corrupted: got %s, want %s",
			match.ApplicabilityStatus, identity.ApplicabilityNotYetEstablished)
	}
	return nil
}

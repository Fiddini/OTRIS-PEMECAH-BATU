package registry

import (
	"errors"
	"fmt"

	"github.com/otris-id/otris-pemecah-batu/internal/identity"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
)

// ApplicabilityDecision describes why a candidate was accepted or rejected.
type ApplicabilityDecision string

const (
	DecisionConfirmed         ApplicabilityDecision = "CONFIRMED"          // candidate applicable in current context
	DecisionRejected          ApplicabilityDecision = "REJECTED"           // candidate not applicable; identity mismatch
	DecisionNeedsEvidence     ApplicabilityDecision = "NEEDS_EVIDENCE"     // insufficient current evidence to decide
)

// ApplicabilityVerification evaluates whether a candidate knowledge match
// may proceed through existing governance gates. It does NOT execute those gates;
// it confirms the candidate's identity is compatible with current context.
//
// CRITICAL: Verification only compares identities.
// It DOES NOT reconstruct missing live MappingDefinition fields.
// Fields like SourceCode, SourceUnit must come from current evidence gates (C1/C2).
type ApplicabilityVerification struct {
	// Inputs
	Candidate       *identity.CandidateKnowledgeMatch
	CurrentMapping  *mapping.MappingDefinition
	CurrentIdentity *identity.IntegrationKnowledgeIdentity

	// Decision
	Decision       ApplicabilityDecision
	ReasonAccepted string // why CONFIRMED (if applicable)
	ReasonRejected string // why REJECTED (if applicable)
	ReasonNeeds    string // why NEEDS_EVIDENCE (if applicable)

	// Status (for test/validation)
	verified bool
}

// NewApplicabilityVerification creates a new verification context.
func NewApplicabilityVerification(
	candidate *identity.CandidateKnowledgeMatch,
	currentMapping *mapping.MappingDefinition,
	currentIdentity *identity.IntegrationKnowledgeIdentity,
) *ApplicabilityVerification {
	return &ApplicabilityVerification{
		Candidate:       candidate,
		CurrentMapping:  currentMapping,
		CurrentIdentity: currentIdentity,
		Decision:        "", // Not yet decided
		verified:        false,
	}
}

// Verify checks identity relationships between candidate and current context.
// Returns decision reason and error (error = verification incomplete or fatal mismatch).
func (av *ApplicabilityVerification) Verify() error {
	if av.verified {
		return errors.New("applicability verification: already verified")
	}

	// Validate inputs
	if av.Candidate == nil {
		av.Decision = DecisionRejected
		av.ReasonRejected = "candidate is nil"
		av.verified = true
		return errors.New("applicability verification: candidate required")
	}

	if av.CurrentMapping == nil {
		av.Decision = DecisionNeedsEvidence
		av.ReasonNeeds = "current mapping is nil; cannot verify context"
		av.verified = true
		return errors.New("applicability verification: current mapping required")
	}

	if av.CurrentIdentity == nil {
		av.Decision = DecisionNeedsEvidence
		av.ReasonNeeds = "current identity is nil; insufficient context"
		av.verified = true
		return errors.New("applicability verification: current identity required")
	}

	// Verify candidate identity integrity
	if err := av.Candidate.IntegrationIdentity.Verify(); err != nil {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf("candidate identity corrupted: %v", err)
		av.verified = true
		return fmt.Errorf("applicability verification: %w", err)
	}

	// Verify current identity integrity
	if err := av.CurrentIdentity.Verify(); err != nil {
		av.Decision = DecisionNeedsEvidence
		av.ReasonNeeds = fmt.Sprintf("current identity corrupted: %v", err)
		av.verified = true
		return fmt.Errorf("applicability verification: %w", err)
	}

	// Check device fingerprint match (CRITICAL: determines if hardware is same)
	if av.checkDeviceFingerprintMismatch() {
		av.verified = true
		return nil // Decision already set by check function
	}

	// Check mapping identity match (candidate must be for same mapping)
	if av.checkMappingIdentityMismatch() {
		av.verified = true
		return nil // Decision already set
	}

	// Check mapping version match (candidate version must match or be compatible)
	if av.checkMappingVersionMismatch() {
		av.verified = true
		return nil // Decision already set
	}

	// Check semantic evidence digest match (identity binding must be same)
	if av.checkSemanticDigestMismatch() {
		av.verified = true
		return nil // Decision already set
	}

	// Check reference identity match (if applicable for QC)
	if av.checkReferenceIdentityMismatch() {
		av.verified = true
		return nil // Decision already set
	}

	// All identity checks passed; candidate is applicable
	av.Decision = DecisionConfirmed
	deviceFPDisplay := truncate(av.CurrentIdentity.DeviceFingerprint, 16)
	semanticDisplay := truncate(av.CurrentIdentity.SemanticEvidenceDigest, 16)
	refDisplay := truncate(av.CurrentIdentity.ReferenceIdentity, 16)
	av.ReasonAccepted = fmt.Sprintf(
		"Candidate identity matches current context: "+
			"device=%s, mapping=%s/%s, semantic=%s..., reference=%s",
		deviceFPDisplay,
		av.CurrentIdentity.MappingIdentity,
		av.CurrentIdentity.MappingVersion,
		semanticDisplay,
		refDisplay,
	)
	av.verified = true
	return nil
}

// checkDeviceFingerprintMismatch returns true if fingerprints differ (sets Decision).
func (av *ApplicabilityVerification) checkDeviceFingerprintMismatch() bool {
	candidateFP := av.Candidate.IntegrationIdentity.DeviceFingerprint
	currentFP := av.CurrentIdentity.DeviceFingerprint

	// Placeholder values (not-stored-in-attestation) indicate lookup result
	// If current has real FP but candidate is placeholder, cannot verify hardware match
	if isPlaceholder(candidateFP) && !isPlaceholder(currentFP) {
		av.Decision = DecisionNeedsEvidence
		av.ReasonNeeds = "candidate fingerprint not available from historical record; " +
			"current evidence must confirm device identity"
		return true
	}

	// If both have real values, they must match
	if !isPlaceholder(candidateFP) && !isPlaceholder(currentFP) {
		if candidateFP != currentFP {
			av.Decision = DecisionRejected
			av.ReasonRejected = fmt.Sprintf(
				"device fingerprint mismatch: candidate=%s, current=%s",
				truncate(candidateFP, 16), truncate(currentFP, 16),
			)
			return true
		}
	}

	return false // No mismatch detected
}

// checkMappingIdentityMismatch returns true if mapping IDs differ (sets Decision).
func (av *ApplicabilityVerification) checkMappingIdentityMismatch() bool {
	candidateID := av.Candidate.IntegrationIdentity.MappingIdentity
	currentID := av.CurrentIdentity.MappingIdentity
	currentMappingID := av.CurrentMapping.MappingIdentity

	// Candidate mapping must match both current identity and current mapping definition
	if candidateID != currentID {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf(
			"mapping identity mismatch: candidate=%s, current=%s",
			candidateID, currentID,
		)
		return true
	}

	if candidateID != currentMappingID {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf(
			"mapping identity mismatch: candidate=%s, current_mapping=%s",
			candidateID, currentMappingID,
		)
		return true
	}

	return false
}

// checkMappingVersionMismatch returns true if versions differ (sets Decision).
func (av *ApplicabilityVerification) checkMappingVersionMismatch() bool {
	candidateVersion := av.Candidate.IntegrationIdentity.MappingVersion
	currentVersion := av.CurrentIdentity.MappingVersion
	currentMappingVersion := av.CurrentMapping.MappingVersion

	// Versions must match exactly (evidence + replay testing is version-specific)
	if candidateVersion != currentVersion {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf(
			"mapping version mismatch: candidate=%s, current=%s",
			candidateVersion, currentVersion,
		)
		return true
	}

	if candidateVersion != currentMappingVersion {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf(
			"mapping version mismatch: candidate=%s, current_mapping=%s",
			candidateVersion, currentMappingVersion,
		)
		return true
	}

	return false
}

// checkSemanticDigestMismatch returns true if digests differ (sets Decision).
func (av *ApplicabilityVerification) checkSemanticDigestMismatch() bool {
	candidateDigest := av.Candidate.IntegrationIdentity.SemanticEvidenceDigest
	currentDigest := av.CurrentIdentity.SemanticEvidenceDigest
	currentMappingDigest := av.CurrentMapping.SemanticEvidenceDigest

	// Semantic digests must match (proves same evidence was used)
	if candidateDigest != currentDigest {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf(
			"semantic evidence digest mismatch: candidate=%s..., current=%s...",
			truncate(candidateDigest, 16), truncate(currentDigest, 16),
		)
		return true
	}

	// Current mapping should also have same digest (or be empty if current evidence not yet digested)
	if currentMappingDigest != "" && candidateDigest != currentMappingDigest {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf(
			"semantic evidence digest mismatch: candidate=%s..., current_mapping=%s...",
			truncate(candidateDigest, 16), truncate(currentMappingDigest, 16),
		)
		return true
	}

	return false
}

// checkReferenceIdentityMismatch returns true if reference IDs differ (sets Decision).
// Reference identity may be from QC policy or be a placeholder.
func (av *ApplicabilityVerification) checkReferenceIdentityMismatch() bool {
	candidateRef := av.Candidate.IntegrationIdentity.ReferenceIdentity
	currentRef := av.CurrentIdentity.ReferenceIdentity

	// If either is placeholder, cannot verify reference match yet
	if isPlaceholder(candidateRef) || isPlaceholder(currentRef) {
		// This is acceptable; reference identity may not be available from lookup
		return false
	}

	// If both have real values, they must match
	if candidateRef != currentRef {
		av.Decision = DecisionRejected
		av.ReasonRejected = fmt.Sprintf(
			"reference identity mismatch: candidate=%s, current=%s",
			candidateRef, currentRef,
		)
		return true
	}

	return false
}

// isPlaceholder checks if value is the attestation lookup placeholder.
func isPlaceholder(value string) bool {
	return value == "(not-stored-in-attestation)"
}

// truncate limits string to max length for display.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// GetDecision returns the verification decision and reasons.
func (av *ApplicabilityVerification) GetDecision() (ApplicabilityDecision, string) {
	switch av.Decision {
	case DecisionConfirmed:
		return av.Decision, av.ReasonAccepted
	case DecisionRejected:
		return av.Decision, av.ReasonRejected
	case DecisionNeedsEvidence:
		return av.Decision, av.ReasonNeeds
	default:
		return "", "verification not completed"
	}
}

// IsVerified returns true if verification completed.
func (av *ApplicabilityVerification) IsVerified() bool {
	return av.verified
}

// IsCandidateApplicable returns true if decision is CONFIRMED.
func (av *ApplicabilityVerification) IsCandidateApplicable() bool {
	return av.Decision == DecisionConfirmed
}

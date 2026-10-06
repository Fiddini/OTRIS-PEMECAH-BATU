package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// ReverificationStatus describes whether observed identity change requires re-verification.
type ReverificationStatus string

const (
	// NoAction: observation does not require re-verification.
	// Applies to INITIAL observations or pending investigations.
	ReverificationNoAction ReverificationStatus = "NO_ACTION"

	// ReverifyRequired: identity change confirmed; re-verification required.
	// Applies to engineer-confirmed UPDATE or POSSIBLE_CORRUPTION observations.
	ReverificationRequired ReverificationStatus = "REVERIFY_REQUIRED"

	// InsufficientEvidence: identity change recorded but insufficient data to decide.
	// Applies to UNKNOWN observations or pending investigation completion.
	ReverificationInsufficientEvidence ReverificationStatus = "INSUFFICIENT_EVIDENCE"
)

// ReverificationRequirement is an immutable decision record linking an identity
// observation to a re-verification requirement. It does NOT modify authority.
// It does NOT grant or revoke any privileges.
type ReverificationRequirement struct {
	// Decision Identity
	RequirementID string `json:"requirement_id"` // unique identifier

	// The Observation
	ObservationID string `json:"observation_id"` // which observation triggered this

	// The Decision
	Status ReverificationStatus `json:"status"` // NO_ACTION, REVERIFY_REQUIRED, or INSUFFICIENT_EVIDENCE
	Reason string               `json:"reason"` // why this decision

	// Context (immutable reference, no modification)
	DeviceFingerprint  string `json:"device_fingerprint,omitempty"`   // what device is affected
	ChangeType         string `json:"change_type,omitempty"`          // what type of change
	InvestigationState string `json:"investigation_state,omitempty"`  // CLOSED, UNDER_INVESTIGATION, etc.

	// Evidence References (where to find supporting documentation)
	ObservationReference string `json:"observation_reference,omitempty"` // path to FingerprintIdentityObservation
	DecisionJustification string `json:"decision_justification,omitempty"` // engineer/system reasoning

	// Governance (never transparent)
	CannotModifyAuthority bool `json:"cannot_modify_authority"` // always true; decision is separate from authority
	CannotRevokeAttestation bool `json:"cannot_revoke_attestation"` // true; does not revoke existing admission

	// Metadata
	DecidedAt time.Time `json:"decided_at"` // when decision was made
	DecidedBy string    `json:"decided_by"` // system logic or engineer decision

	// Integrity
	Digest  string `json:"digest"`  // SHA256 immutable hash
	Version string `json:"version"` // schema version

	sealed bool
}

// NewReverificationRequirement creates a new re-verification decision record.
// All fields are required and validated before sealing.
func NewReverificationRequirement(
	requirementID, observationID string,
	status ReverificationStatus,
	reason string,
	deviceFingerprint, changeType, investigationState string,
) (*ReverificationRequirement, error) {
	// Validate required fields
	if err := validateRequiredField("requirement_id", requirementID); err != nil {
		return nil, fmt.Errorf("reverification requirement: %w", err)
	}
	if err := validateRequiredField("observation_id", observationID); err != nil {
		return nil, fmt.Errorf("reverification requirement: %w", err)
	}
	if err := validateRequiredField("reason", reason); err != nil {
		return nil, fmt.Errorf("reverification requirement: %w", err)
	}

	// Validate status
	switch status {
	case ReverificationNoAction, ReverificationRequired, ReverificationInsufficientEvidence:
		// OK
	default:
		return nil, fmt.Errorf("reverification requirement: unknown status %q", status)
	}

	// Validate device fingerprint if provided
	if deviceFingerprint != "" && !utf8.ValidString(deviceFingerprint) {
		return nil, errors.New("reverification requirement: device_fingerprint invalid UTF-8")
	}

	// Validate change type if provided
	if changeType != "" && !utf8.ValidString(changeType) {
		return nil, errors.New("reverification requirement: change_type invalid UTF-8")
	}

	rrq := &ReverificationRequirement{
		RequirementID:         requirementID,
		ObservationID:         observationID,
		Status:                status,
		Reason:                reason,
		DeviceFingerprint:     deviceFingerprint,
		ChangeType:            changeType,
		InvestigationState:    investigationState,
		CannotModifyAuthority: true,  // always true
		CannotRevokeAttestation: true, // never revokes
		DecidedAt:             time.Now().UTC(),
		DecidedBy:             "system", // "system" for automatic decision, engineer name for manual
		Version:               "1.0.0",
	}

	rrq.Digest = rrq.computeDigest()
	rrq.sealed = true

	return rrq, nil
}

// SetObservationReference sets the reference to the originating observation.
// Returns error if already sealed (after creation, cannot modify).
func (rrq *ReverificationRequirement) SetObservationReference(ref string) error {
	if rrq.sealed {
		return errors.New("reverification requirement: cannot modify sealed requirement")
	}
	if ref != "" && !utf8.ValidString(ref) {
		return errors.New("reverification requirement: observation_reference invalid UTF-8")
	}
	rrq.ObservationReference = ref
	rrq.Digest = rrq.computeDigest()
	return nil
}

// SetDecisionJustification sets the reasoning for the decision.
// Returns error if already sealed.
func (rrq *ReverificationRequirement) SetDecisionJustification(justification string) error {
	if rrq.sealed {
		return errors.New("reverification requirement: cannot modify sealed requirement")
	}
	if justification != "" && !utf8.ValidString(justification) {
		return errors.New("reverification requirement: decision_justification invalid UTF-8")
	}
	rrq.DecisionJustification = justification
	rrq.Digest = rrq.computeDigest()
	return nil
}

// SetDecidedBy records who made the decision.
// Returns error if already sealed.
func (rrq *ReverificationRequirement) SetDecidedBy(decider string) error {
	if rrq.sealed {
		return errors.New("reverification requirement: cannot modify sealed requirement")
	}
	if err := validateRequiredField("decided_by", decider); err != nil {
		return fmt.Errorf("reverification requirement: %w", err)
	}
	rrq.DecidedBy = decider
	rrq.Digest = rrq.computeDigest()
	return nil
}

// computeDigest returns immutable integrity hash.
func (rrq *ReverificationRequirement) computeDigest() string {
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%d",
		rrq.RequirementID,
		rrq.ObservationID,
		rrq.Status,
		rrq.Reason,
		rrq.DeviceFingerprint,
		rrq.ChangeType,
		rrq.InvestigationState,
		rrq.DecidedBy,
		rrq.DecidedAt.Unix(),
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// Verify checks integrity of this requirement.
func (rrq *ReverificationRequirement) Verify() error {
	if !rrq.sealed {
		return errors.New("reverification requirement: not sealed")
	}
	expected := rrq.computeDigest()
	if rrq.Digest != expected {
		return fmt.Errorf("reverification requirement: digest mismatch (expected %s, got %s)", expected, rrq.Digest)
	}
	if !rrq.CannotModifyAuthority {
		return errors.New("reverification requirement: authority modification barrier broken")
	}
	if !rrq.CannotRevokeAttestation {
		return errors.New("reverification requirement: attestation revocation barrier broken")
	}
	return nil
}

// IsNoAction returns true if decision is NO_ACTION.
func (rrq *ReverificationRequirement) IsNoAction() bool {
	return rrq.Status == ReverificationNoAction
}

// IsRequired returns true if re-verification is required.
func (rrq *ReverificationRequirement) IsRequired() bool {
	return rrq.Status == ReverificationRequired
}

// IsInsufficientEvidence returns true if decision is INSUFFICIENT_EVIDENCE.
func (rrq *ReverificationRequirement) IsInsufficientEvidence() bool {
	return rrq.Status == ReverificationInsufficientEvidence
}

// String provides human-readable summary.
func (rrq *ReverificationRequirement) String() string {
	return fmt.Sprintf("ReverificationRequirement{id=%s, obs=%s, status=%s, reason=%q}",
		rrq.RequirementID, rrq.ObservationID, rrq.Status, rrq.Reason)
}

// DecideFromObservation evaluates a fingerprint observation and determines whether
// re-verification is required. This is a deterministic decision function that does
// NOT modify the observation and does NOT affect authority.
func DecideFromObservation(obs *FingerprintIdentityObservation) ReverificationStatus {
	if obs == nil {
		return ReverificationNoAction // no observation, no decision
	}

	// INITIAL observations do not trigger re-verification
	if obs.ChangeType == ChangeTypeInitial {
		return ReverificationNoAction
	}

	// UNREVIEWED or UNDER_INVESTIGATION: no decision yet
	if obs.InvestigationStatus != "CLOSED" {
		// If investigation shows high confidence but is still open, return INSUFFICIENT_EVIDENCE
		// Otherwise, return NO_ACTION (decision pending)
		if obs.Confidence > 0.7 && (obs.InvestigationStatus == "UNDER_INVESTIGATION" || obs.InvestigationStatus == "UNREVIEWED") {
			return ReverificationInsufficientEvidence
		}
		return ReverificationNoAction
	}

	// Investigation is CLOSED; engineer has made a decision via Review()

	// Engineer-confirmed UPDATE: legitimate change requires re-verification
	// Note: IsConfirmedChange() returns true ONLY if ChangeType==UPDATE AND engineer confirmed in Review()
	if obs.IsConfirmedChange() {
		return ReverificationRequired
	}

	// POSSIBLE_CORRUPTION: suspicious change always requires re-verification
	if obs.ChangeType == ChangeTypePossibleCorruption {
		return ReverificationRequired
	}

	// UNKNOWN: insufficient evidence to decide
	if obs.ChangeType == ChangeTypeUnknown {
		return ReverificationInsufficientEvidence
	}

	// Default: no action
	return ReverificationNoAction
}

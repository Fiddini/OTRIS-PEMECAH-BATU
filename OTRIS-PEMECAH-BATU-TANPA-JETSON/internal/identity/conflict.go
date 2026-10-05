package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// ConflictType categorizes what identities are in disagreement.
type ConflictType string

const (
	ConflictTypeFingerprintMismatch    ConflictType = "FINGERPRINT_MISMATCH"    // observed vs expected fingerprint differ
	ConflictTypeSemanticDigestMismatch ConflictType = "SEMANTIC_DIGEST_MISMATCH" // semantic evidence binding changed
	ConflictTypeMappingIdMismatch      ConflictType = "MAPPING_ID_MISMATCH"      // mapping identity differs
	ConflictTypeReferenceIdMismatch    ConflictType = "REFERENCE_ID_MISMATCH"    // QC reference changed
	ConflictTypeQCPolicyMismatch       ConflictType = "QC_POLICY_MISMATCH"       // QC policy version conflict
	ConflictTypeProfileBindingMismatch ConflictType = "PROFILE_BINDING_MISMATCH" // profile/history disagree
	ConflictTypeUnknown                ConflictType = "UNKNOWN"                  // cannot determine cause
)

// IdentityConflict is an immutable record of identity disagreement between contexts.
// It does NOT resolve automatically. All conflicting identities are preserved.
// No majority vote. No automatic winner.
type IdentityConflict struct {
	// Conflict Identity
	ConflictID string      `json:"conflict_id"` // unique identifier
	ConflictType ConflictType `json:"conflict_type"` // what identities conflict

	// The Values (both preserved, neither deleted)
	CurrentIdentity   string `json:"current_identity"`    // what we have now
	PreviousIdentity  string `json:"previous_identity"`   // what we had before (or expected)
	IdentityDimension string `json:"identity_dimension"`  // which field is in conflict (e.g., "device_fingerprint", "semantic_digest")

	// Context
	AffectedMappingID string `json:"affected_mapping_id,omitempty"`  // which mapping involved
	AffectedDeviceRef string `json:"affected_device_ref,omitempty"`  // which device involved

	// Evidence References (where to find supporting data)
	CurrentEvidenceRef  string `json:"current_evidence_ref,omitempty"`  // reference to current state artifact
	PreviousEvidenceRef string `json:"previous_evidence_ref,omitempty"` // reference to prior state artifact

	// Resolution (initially unresolved)
	Status            string    `json:"status"`             // UNRESOLVED, UNDER_INVESTIGATION, RESOLVED_EXPLICIT
	BlocksAutomaticReuse bool  `json:"blocks_automatic_reuse"` // cannot use candidate reuse without resolution
	BlocksApproval    bool    `json:"blocks_approval,omitempty"` // can engineer still approve? (false = engineer can override)
	RequiresExplicitReview bool `json:"requires_explicit_review"` // true: must be reviewed before proceeding

	// Resolution Details (filled when Status becomes RESOLVED_EXPLICIT)
	ResolvedAt      time.Time `json:"resolved_at,omitempty"`
	ResolvedBy      string    `json:"resolved_by,omitempty"`
	ResolutionReason string   `json:"resolution_reason,omitempty"` // human explanation

	// Governance (conflict is never transparent)
	CannotModifyAuthority bool `json:"cannot_modify_authority"` // always true

	// Metadata
	DetectedAt time.Time `json:"detected_at"`
	Provenance string    `json:"provenance"` // how conflict was detected

	// Integrity
	Digest  string `json:"digest"`  // SHA256 immutable
	Version string `json:"version"` // schema version

	sealed bool
}

// NewConflict creates a new identity conflict.
// Both conflicting identities must be preserved; neither is automatically chosen.
func NewConflict(
	conflictID string,
	conflictType ConflictType,
	currentIdentity, previousIdentity, dimension string,
	provenance string,
) (*IdentityConflict, error) {
	// Validate required fields
	if err := validateRequiredField("conflict_id", conflictID); err != nil {
		return nil, fmt.Errorf("identity conflict: %w", err)
	}

	// Validate conflict type
	switch conflictType {
	case ConflictTypeFingerprintMismatch, ConflictTypeSemanticDigestMismatch,
		ConflictTypeMappingIdMismatch, ConflictTypeReferenceIdMismatch,
		ConflictTypeQCPolicyMismatch, ConflictTypeProfileBindingMismatch,
		ConflictTypeUnknown:
		// OK
	default:
		return nil, fmt.Errorf("identity conflict: unknown conflict type %q", conflictType)
	}

	if err := validateRequiredField("current_identity", currentIdentity); err != nil {
		return nil, fmt.Errorf("identity conflict: %w", err)
	}
	if err := validateRequiredField("previous_identity", previousIdentity); err != nil {
		return nil, fmt.Errorf("identity conflict: %w", err)
	}
	if err := validateRequiredField("dimension", dimension); err != nil {
		return nil, fmt.Errorf("identity conflict: %w", err)
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("identity conflict: %w", err)
	}

	ic := &IdentityConflict{
		ConflictID:             conflictID,
		ConflictType:           conflictType,
		CurrentIdentity:        currentIdentity,
		PreviousIdentity:       previousIdentity,
		IdentityDimension:      dimension,
		Status:                 "UNRESOLVED",
		BlocksAutomaticReuse:   true,
		BlocksApproval:         false, // engineer can still decide to approve
		RequiresExplicitReview: true,
		CannotModifyAuthority:  true, // always true, never changes
		DetectedAt:             time.Now().UTC(),
		Provenance:             provenance,
		Version:                "1.0.0",
	}

	ic.Digest = ic.computeDigest()
	ic.sealed = true

	return ic, nil
}

// SetContext sets optional context (mapping/device reference).
// Returns error if already sealed.
func (ic *IdentityConflict) SetContext(mappingID, deviceRef string) error {
	if ic.sealed {
		return errors.New("identity conflict: cannot modify sealed conflict")
	}
	if mappingID != "" && !utf8.ValidString(mappingID) {
		return errors.New("identity conflict: mapping_id invalid UTF-8")
	}
	if deviceRef != "" && !utf8.ValidString(deviceRef) {
		return errors.New("identity conflict: device_ref invalid UTF-8")
	}
	ic.AffectedMappingID = mappingID
	ic.AffectedDeviceRef = deviceRef
	ic.Digest = ic.computeDigest()
	return nil
}

// SetEvidenceReferences sets references to supporting data.
// Returns error if already sealed.
func (ic *IdentityConflict) SetEvidenceReferences(currentRef, previousRef string) error {
	if ic.sealed {
		return errors.New("identity conflict: cannot modify sealed conflict")
	}
	if currentRef != "" && !utf8.ValidString(currentRef) {
		return errors.New("identity conflict: current_evidence_ref invalid UTF-8")
	}
	if previousRef != "" && !utf8.ValidString(previousRef) {
		return errors.New("identity conflict: previous_evidence_ref invalid UTF-8")
	}
	ic.CurrentEvidenceRef = currentRef
	ic.PreviousEvidenceRef = previousRef
	ic.Digest = ic.computeDigest()
	return nil
}

// Resolve records explicit human decision.
// Returns error if already resolved or conflict is not sealed.
func (ic *IdentityConflict) Resolve(
	resolvedBy, reason string,
) error {
	if !ic.sealed {
		return errors.New("identity conflict: cannot resolve unsealed conflict")
	}
	if ic.Status == "RESOLVED_EXPLICIT" {
		return errors.New("identity conflict: already resolved")
	}
	if err := validateRequiredField("resolved_by", resolvedBy); err != nil {
		return fmt.Errorf("identity conflict: %w", err)
	}

	ic.ResolvedAt = time.Now().UTC()
	ic.ResolvedBy = resolvedBy
	ic.ResolutionReason = reason
	ic.Status = "RESOLVED_EXPLICIT"
	// Note: both original identities preserved in CurrentIdentity/PreviousIdentity

	return nil
}

// computeDigest returns immutable integrity hash.
func (ic *IdentityConflict) computeDigest() string {
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%d",
		ic.ConflictID,
		ic.ConflictType,
		ic.CurrentIdentity,
		ic.PreviousIdentity,
		ic.IdentityDimension,
		ic.AffectedMappingID,
		ic.AffectedDeviceRef,
		ic.Provenance,
		ic.DetectedAt.Unix(),
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// Verify checks integrity of this conflict.
func (ic *IdentityConflict) Verify() error {
	if !ic.sealed {
		return errors.New("identity conflict: not sealed")
	}
	expected := ic.computeDigest()
	if ic.Digest != expected {
		return fmt.Errorf("identity conflict: digest mismatch (expected %s, got %s)", expected, ic.Digest)
	}
	if !ic.CannotModifyAuthority {
		return errors.New("identity conflict: authority modification barrier broken")
	}
	return nil
}

// IsResolved returns true if conflict has explicit resolution.
func (ic *IdentityConflict) IsResolved() bool {
	return ic.Status == "RESOLVED_EXPLICIT"
}

// String provides human-readable summary.
func (ic *IdentityConflict) String() string {
	return fmt.Sprintf("IdentityConflict{id=%s, type=%s, dimension=%s, status=%s, blocks_reuse=%v}",
		ic.ConflictID, ic.ConflictType, ic.IdentityDimension, ic.Status, ic.BlocksAutomaticReuse)
}

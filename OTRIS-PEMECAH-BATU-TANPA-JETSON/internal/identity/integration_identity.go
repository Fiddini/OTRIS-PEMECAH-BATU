package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// IntegrationKnowledgeIdentity is the smallest non-authoritative composition
// of existing governed identities required to locate and reuse candidate knowledge.
// It does NOT grant authority; it enables discovery only.
type IntegrationKnowledgeIdentity struct {
	// Existing Governed Identities (deterministic composition)
	DeviceFingerprint      string `json:"device_fingerprint"`       // structural device identity
	MappingIdentity        string `json:"mapping_identity"`         // field mapping claim
	MappingVersion         string `json:"mapping_version"`          // evidence + replay version
	ReferenceIdentity      string `json:"reference_identity"`       // QC reference levels
	SemanticEvidenceDigest string `json:"semantic_evidence_digest"` // VENDOR_MANUAL binding

	// Optional Identity Dimensions (where applicable)
	QCPolicyIdentity string `json:"qc_policy_identity,omitempty"`    // policy version ID (if available)
	TransferFunctionID string `json:"transfer_function_id,omitempty"` // factor algorithm (if applicable)
	KnowledgeEpochID   string `json:"knowledge_epoch_id,omitempty"`   // temporal boundary (if applicable)

	// Integrity (immutable)
	Digest  string `json:"digest"`  // SHA256 of deterministic composition
	Version string `json:"version"` // semantic version of identity schema

	sealed bool
}

// New creates a new integration knowledge identity from governed inputs.
// All required fields must be non-empty. Optional fields may be empty (omitted).
// Digest is computed deterministically.
func New(
	deviceFingerprint, mappingIdentity, mappingVersion,
	referenceIdentity, semanticEvidenceDigest string,
) (*IntegrationKnowledgeIdentity, error) {
	// Validate required fields
	if err := validateRequiredField("device_fingerprint", deviceFingerprint); err != nil {
		return nil, fmt.Errorf("integration identity: %w", err)
	}
	if err := validateRequiredField("mapping_identity", mappingIdentity); err != nil {
		return nil, fmt.Errorf("integration identity: %w", err)
	}
	if err := validateRequiredField("mapping_version", mappingVersion); err != nil {
		return nil, fmt.Errorf("integration identity: %w", err)
	}
	if err := validateRequiredField("reference_identity", referenceIdentity); err != nil {
		return nil, fmt.Errorf("integration identity: %w", err)
	}
	// SemanticEvidenceDigest is required (proves semantic authority binding)
	if err := validateRequiredField("semantic_evidence_digest", semanticEvidenceDigest); err != nil {
		return nil, fmt.Errorf("integration identity: %w", err)
	}

	iki := &IntegrationKnowledgeIdentity{
		DeviceFingerprint:      deviceFingerprint,
		MappingIdentity:        mappingIdentity,
		MappingVersion:         mappingVersion,
		ReferenceIdentity:      referenceIdentity,
		SemanticEvidenceDigest: semanticEvidenceDigest,
		Version:                "1.0.0",
	}

	iki.Digest = iki.computeDigest()
	iki.sealed = true

	return iki, nil
}

// WithOptionals sets optional identity dimensions.
// Does NOT unseal; must be called before creation completes.
func (iki *IntegrationKnowledgeIdentity) WithOptionals(
	qcPolicyID, transferFunctionID, epochID string,
) error {
	if iki.sealed {
		return errors.New("integration identity: cannot modify sealed identity")
	}

	// Validate optional fields if provided (must be valid UTF-8, non-empty if present)
	if qcPolicyID != "" {
		if !utf8.ValidString(qcPolicyID) {
			return errors.New("integration identity: qc_policy_id invalid UTF-8")
		}
		iki.QCPolicyIdentity = qcPolicyID
	}

	if transferFunctionID != "" {
		if !utf8.ValidString(transferFunctionID) {
			return errors.New("integration identity: transfer_function_id invalid UTF-8")
		}
		iki.TransferFunctionID = transferFunctionID
	}

	if epochID != "" {
		if !utf8.ValidString(epochID) {
			return errors.New("integration identity: knowledge_epoch_id invalid UTF-8")
		}
		iki.KnowledgeEpochID = epochID
	}

	// Recompute digest after optional fields added
	iki.Digest = iki.computeDigest()
	return nil
}

// computeDigest returns deterministic SHA256 of identity composition.
// Canonical order: required fields first (always present), optional fields in defined order.
func (iki *IntegrationKnowledgeIdentity) computeDigest() string {
	// Canonical representation (deterministic, order matters)
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%s",
		iki.DeviceFingerprint,
		iki.MappingIdentity,
		iki.MappingVersion,
		iki.ReferenceIdentity,
		iki.SemanticEvidenceDigest,
		iki.QCPolicyIdentity,     // empty string if not set
		iki.TransferFunctionID,   // empty string if not set
		iki.KnowledgeEpochID,     // empty string if not set
		iki.Version,              // schema version
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// Verify checks integrity of this identity.
func (iki *IntegrationKnowledgeIdentity) Verify() error {
	if !iki.sealed {
		return errors.New("integration identity: not sealed")
	}
	expected := iki.computeDigest()
	if iki.Digest != expected {
		return fmt.Errorf("integration identity: digest mismatch (expected %s, got %s)", expected, iki.Digest)
	}
	return nil
}

// Matches checks if another identity is the same (exact composition match).
func (iki *IntegrationKnowledgeIdentity) Matches(other *IntegrationKnowledgeIdentity) bool {
	if other == nil {
		return false
	}
	return iki.Digest == other.Digest
}

// String provides human-readable summary.
func (iki *IntegrationKnowledgeIdentity) String() string {
	return fmt.Sprintf("IntegrationKnowledgeIdentity{mapping=%s/%s, device=%s, semantic=%s..., digest=%s}",
		iki.MappingIdentity, iki.MappingVersion, iki.DeviceFingerprint, iki.SemanticEvidenceDigest[:16], iki.Digest[:16])
}

// validateRequiredField checks that a field is non-empty and valid UTF-8.
func validateRequiredField(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", name)
	}
	return nil
}

package candidate

import (
	"fmt"

	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
)

// ToMappingDefinition converts a fully resolved candidate claim to a MappingDefinition.
// FAIL-CLOSED: Candidate must pass PromoteToEvidence() check first.
// Returns error if any required field is unresolved or invalid.
func (c *Claim) ToMappingDefinition() (*mapping.MappingDefinition, error) {
	// Enforce promotion boundary: fail-closed if unresolved
	if err := c.PromoteToEvidence(); err != nil {
		return nil, fmt.Errorf("candidate not eligible for promotion: %w", err)
	}

	// Create MappingDefinition with all promoted fields
	m, err := mapping.New(
		c.MappingIdentity,
		c.MappingVersion,
		c.SourceProtocol,
		c.SourcePosition,
		c.SourceCode,
		c.SourceUnit,
		c.CanonicalConcept,
		c.CanonicalUnit,
		c.Provenance,
		c.ProvenanceAuthor,
	)
	if err != nil {
		return nil, fmt.Errorf("create MappingDefinition from candidate: %w", err)
	}

	// Set governed normalizer identity (cannot be inferred)
	m.NormalizerIdentity = c.NormalizerIdentity

	// Set optional device context
	m.DeviceFingerprint = c.DeviceFingerprint
	m.DeviceIdentityRef = c.DeviceIdentityRef
	m.ParserIdentity = c.ParserIdentity

	return m, nil
}

// PromoteAndBindEvidence performs the complete candidate promotion and evidence binding.
// This is the test fixture path that ALLOWS test promotion while REJECTING real candidates.
// CRITICAL: This must preserve the fail-closed contract for production candidates.
func (c *Claim) PromoteAndBindEvidence(evidencePath, reportPath string, evidenceItems []mapping.Evidence) (*mapping.MappingDefinition, error) {
	// Step 1: Convert to MappingDefinition (fails if unresolved)
	m, err := c.ToMappingDefinition()
	if err != nil {
		return nil, fmt.Errorf("promotion failed: %w", err)
	}

	// Step 2: Set evidence context
	if len(evidenceItems) > 0 {
		if err := m.SetEvidenceContext(reportPath, evidenceItems, "APPROVED_FOR_MAPPING"); err != nil {
			return nil, fmt.Errorf("bind evidence: %w", err)
		}
	}

	return m, nil
}

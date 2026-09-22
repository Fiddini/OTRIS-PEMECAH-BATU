package mapping

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Evidence represents a single evidence item supporting a mapping claim.
// Reused from evidence package conceptually but defined here to avoid circular imports.
type Evidence struct {
	Type  string `json:"type"`              // VENDOR_MANUAL, SIMULATOR_LOG, SCREEN_PHOTO
	Ref   string `json:"ref"`               // reference path or citation
	Quote string `json:"quote,omitempty"`   // supporting quote if applicable
}

// MappingDefinition represents an explicit source→canonical field mapping claim
// with full provenance trail. Atomic: one definition = one field mapping.
type MappingDefinition struct {
	// Artifact Identity
	MappingIdentity string `json:"mapping_identity"`
	MappingVersion  string `json:"mapping_version"`

	// Explicit Mapping Claim (must be supplied, not inferred)
	SourceProtocol  string `json:"source_protocol"`   // e.g., HL7_V2, ASTM_H
	SourcePosition  int    `json:"source_position"`   // field/column position
	SourceCode      string `json:"source_code"`       // field code/mnemonic
	SourceUnit      string `json:"source_unit"`       // unit as specified in protocol
	CanonicalConcept string `json:"canonical_concept"` // canonical clinical concept
	CanonicalUnit   string `json:"canonical_unit"`    // normalized unit

	// Device & Parser Context (reference only, not authority)
	DeviceFingerprint  string `json:"device_fingerprint,omitempty"`  // structural hash from PB-10
	DeviceIdentityRef  string `json:"device_identity_ref,omitempty"` // device model/vendor reference
	ParserIdentity     string `json:"parser_identity,omitempty"`     // parser that extracts this field

	// Evidence & Provenance References
	EvidenceGateReportRef  string     `json:"evidence_gate_report_ref"`            // absolute path to evidence_gate_report.json
	DifferentialReportRef  string     `json:"differential_report_ref,omitempty"`   // optional: path to differential_report.json
	EvidenceRefs           []Evidence `json:"evidence_refs"`                       // evidence items that support this claim
	EvidenceGateVerdict    string     `json:"evidence_gate_verdict,omitempty"`     // should be APPROVED_FOR_MAPPING when approved

	// Provenance Chain
	Provenance       string `json:"provenance"`        // narrative explanation of how claim was determined
	ProvenanceAuthor string `json:"provenance_author"` // role/person/system responsible for the claim

	// Status & Timestamps
	Status               Status    `json:"status"`                 // CANDIDATE, EVIDENCE_APPROVED, REPLAY_VALIDATED
	CreatedAt            time.Time `json:"created_at"`             // when MappingDefinition was created
	EvidenceApprovedAt   time.Time `json:"evidence_approved_at,omitempty"`   // when Evidence Gate approved
	ReplayValidatedAt    time.Time `json:"replay_validated_at,omitempty"`    // when Replay validated
}

// New creates a new MappingDefinition in CANDIDATE status.
// It requires all explicit claim fields to be non-empty and valid.
// Returns error if any required field is missing or invalid.
func New(
	mappingIdentity, mappingVersion string,
	sourceProtocol string, sourcePosition int,
	sourceCode, sourceUnit string,
	canonicalConcept, canonicalUnit string,
	provenance, provenanceAuthor string,
) (*MappingDefinition, error) {
	// Validate all required fields for explicit claim
	if err := validateRequiredField("mapping_identity", mappingIdentity); err != nil {
		return nil, err
	}
	if err := validateRequiredField("mapping_version", mappingVersion); err != nil {
		return nil, err
	}
	if err := validateRequiredField("source_protocol", sourceProtocol); err != nil {
		return nil, err
	}
	if sourcePosition <= 0 {
		return nil, fmt.Errorf("source_position must be positive integer, got %d", sourcePosition)
	}
	if err := validateRequiredField("source_code", sourceCode); err != nil {
		return nil, err
	}
	if err := validateRequiredField("source_unit", sourceUnit); err != nil {
		return nil, err
	}
	if err := validateRequiredField("canonical_concept", canonicalConcept); err != nil {
		return nil, err
	}
	if err := validateRequiredField("canonical_unit", canonicalUnit); err != nil {
		return nil, err
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, err
	}
	if err := validateRequiredField("provenance_author", provenanceAuthor); err != nil {
		return nil, err
	}

	return &MappingDefinition{
		MappingIdentity:    mappingIdentity,
		MappingVersion:     mappingVersion,
		SourceProtocol:     sourceProtocol,
		SourcePosition:     sourcePosition,
		SourceCode:         sourceCode,
		SourceUnit:         sourceUnit,
		CanonicalConcept:   canonicalConcept,
		CanonicalUnit:      canonicalUnit,
		Provenance:         provenance,
		ProvenanceAuthor:   provenanceAuthor,
		Status:             StatusCandidate,
		CreatedAt:          time.Now(),
		EvidenceRefs:       []Evidence{},
	}, nil
}

// SetEvidenceContext associates evidence gate report and evidence items with this definition.
// Does not change status; that is done by TransitionToEvidenceApproved.
func (m *MappingDefinition) SetEvidenceContext(reportRef string, evidenceRefs []Evidence, verdict string) error {
	if err := validateRequiredField("evidence_gate_report_ref", reportRef); err != nil {
		return err
	}
	if len(evidenceRefs) == 0 {
		return fmt.Errorf("evidence_refs cannot be empty")
	}
	m.EvidenceGateReportRef = reportRef
	m.EvidenceRefs = evidenceRefs
	m.EvidenceGateVerdict = verdict
	return nil
}

// SetDifferentialContext associates a differential report (optional).
func (m *MappingDefinition) SetDifferentialContext(reportRef string) error {
	if err := validateRequiredField("differential_report_ref", reportRef); err != nil {
		return err
	}
	m.DifferentialReportRef = reportRef
	return nil
}

// CanTransitionTo returns true if a transition from current status to target is legal.
// This is a query method; it does NOT perform the transition.
// State mutation is the responsibility of external authority (Evidence Gate, Replay).
func (m *MappingDefinition) CanTransitionTo(target Status) bool {
	return m.Status.CanTransitionTo(target)
}

// ValidateTransitionTo checks if transition to target status is legal and returns error if not.
// This is for validation only; it does NOT perform the transition.
// State mutation must be performed by authorized external authority (D3D/D3E integration).
func (m *MappingDefinition) ValidateTransitionTo(target Status) error {
	if !m.CanTransitionTo(target) {
		return fmt.Errorf("illegal status transition from %s to %s", m.Status, target)
	}
	return nil
}

// Validate checks that this MappingDefinition has all required fields valid.
// Used for comprehensive validation before JSON marshaling/storage.
func (m *MappingDefinition) Validate() error {
	// Artifact identity
	if err := validateRequiredField("mapping_identity", m.MappingIdentity); err != nil {
		return err
	}
	if err := validateRequiredField("mapping_version", m.MappingVersion); err != nil {
		return err
	}

	// Explicit claim
	if err := validateRequiredField("source_protocol", m.SourceProtocol); err != nil {
		return err
	}
	if m.SourcePosition <= 0 {
		return fmt.Errorf("source_position must be positive integer, got %d", m.SourcePosition)
	}
	if err := validateRequiredField("source_code", m.SourceCode); err != nil {
		return err
	}
	if err := validateRequiredField("source_unit", m.SourceUnit); err != nil {
		return err
	}
	if err := validateRequiredField("canonical_concept", m.CanonicalConcept); err != nil {
		return err
	}
	if err := validateRequiredField("canonical_unit", m.CanonicalUnit); err != nil {
		return err
	}

	// Provenance
	if err := validateRequiredField("provenance", m.Provenance); err != nil {
		return err
	}
	if err := validateRequiredField("provenance_author", m.ProvenanceAuthor); err != nil {
		return err
	}

	// Status
	if !m.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", m.Status)
	}

	// CreatedAt
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("created_at must be set")
	}

	// If EVIDENCE_APPROVED, must have evidence context
	if m.Status == StatusEvidenceApproved || m.Status == StatusReplayValidated {
		if err := validateRequiredField("evidence_gate_report_ref", m.EvidenceGateReportRef); err != nil {
			return fmt.Errorf("evidence_gate_report_ref required for %s status: %w", m.Status, err)
		}
		if len(m.EvidenceRefs) == 0 {
			return fmt.Errorf("evidence_refs required for %s status", m.Status)
		}
		if m.EvidenceApprovedAt.IsZero() {
			return fmt.Errorf("evidence_approved_at must be set for %s status", m.Status)
		}
	}

	// If REPLAY_VALIDATED, must have replay timestamp
	if m.Status == StatusReplayValidated {
		if m.ReplayValidatedAt.IsZero() {
			return fmt.Errorf("replay_validated_at must be set for REPLAY_VALIDATED status")
		}
	}

	return nil
}

// MarshalJSON implements json.Marshaler.
func (m *MappingDefinition) MarshalJSON() ([]byte, error) {
	type Alias MappingDefinition
	return json.Marshal(&struct {
		*Alias
		CreatedAt         string `json:"created_at"`
		EvidenceApprovedAt string `json:"evidence_approved_at,omitempty"`
		ReplayValidatedAt  string `json:"replay_validated_at,omitempty"`
	}{
		Alias:             (*Alias)(m),
		CreatedAt:         m.CreatedAt.Format(time.RFC3339),
		EvidenceApprovedAt: optionalTimeFormat(m.EvidenceApprovedAt),
		ReplayValidatedAt:  optionalTimeFormat(m.ReplayValidatedAt),
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *MappingDefinition) UnmarshalJSON(data []byte) error {
	type Alias MappingDefinition
	aux := &struct {
		CreatedAt         string `json:"created_at"`
		EvidenceApprovedAt string `json:"evidence_approved_at"`
		ReplayValidatedAt  string `json:"replay_validated_at"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse timestamps
	createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid created_at: %w", err)
	}
	m.CreatedAt = createdAt

	if aux.EvidenceApprovedAt != "" {
		approvedAt, err := time.Parse(time.RFC3339, aux.EvidenceApprovedAt)
		if err != nil {
			return fmt.Errorf("invalid evidence_approved_at: %w", err)
		}
		m.EvidenceApprovedAt = approvedAt
	}

	if aux.ReplayValidatedAt != "" {
		validatedAt, err := time.Parse(time.RFC3339, aux.ReplayValidatedAt)
		if err != nil {
			return fmt.Errorf("invalid replay_validated_at: %w", err)
		}
		m.ReplayValidatedAt = validatedAt
	}

	return nil
}

// Helper functions

func validateRequiredField(fieldName, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("missing or empty %s", fieldName)
	}
	if !utf8.ValidString(trimmed) {
		return fmt.Errorf("invalid UTF-8 in %s", fieldName)
	}
	return nil
}

func optionalTimeFormat(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

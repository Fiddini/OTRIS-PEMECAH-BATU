package candidate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// ResolutionStatus indicates whether a field has been verified or is still open.
type ResolutionStatus string

const (
	ResolutionOpen    ResolutionStatus = "OPEN"      // Requires external authority
	ResolutionPending ResolutionStatus = "PENDING"   // Awaiting validation
	ResolutionResolved ResolutionStatus = "RESOLVED" // Authority verified
)

// Claim represents a pre-MappingDefinition candidate claim with real provenance
// and explicit OPEN dependencies. This allows building candidate packages
// before all semantic authority is available.
type Claim struct {
	// Provenance (all required from real evidence)
	CaptureID           string `json:"capture_id"`           // patient-monitor-001 or 002
	SourceProtocol      string `json:"source_protocol"`      // HL7_V2
	SourcePosition      int    `json:"source_position"`      // OBX field position (5 for value)
	SourceCode          string `json:"source_code"`          // 147842 (from OBX-3)
	SourceUnit          string `json:"source_unit"`          // 264864^MDC_DIM_BEAT_PER_MIN^MDC (from OBX-6)
	SourceUnitLabel     string `json:"source_unit_label"`    // MDC_DIM_BEAT_PER_MIN (parsed from OBX-6)
	ObservedValue       float64 `json:"observed_value"`      // -100 (from OBX-5, OBSERVED ONLY, NOT GT)
	ParserIdentity      string `json:"parser_identity"`      // generic_hl7_v2
	DeviceFingerprint   string `json:"device_fingerprint"`   // 4de28380713b7867 or 578b92f887e2cf67
	DeviceIdentityRef   string `json:"device_identity_ref"`  // UNKNOWN (pending device manual)

	// Semantic Authority (requires external source)
	CanonicalConcept     string            `json:"canonical_concept"`     // HR_BPM, O2_SAT, etc. — OPEN until authority provided
	CanonicalConceptAuth ResolutionStatus  `json:"canonical_concept_auth"` // OPEN|RESOLVED
	CanonicalConceptRef  string            `json:"canonical_concept_ref"`  // Citation if resolved
	CanonicalUnit        string            `json:"canonical_unit"`        // OPEN until authority provided
	CanonicalUnitAuth    ResolutionStatus  `json:"canonical_unit_auth"`    // OPEN|RESOLVED
	CanonicalUnitRef     string            `json:"canonical_unit_ref"`     // Citation if resolved

	// Ground Truth (requires independent measurement)
	GroundTruthValue    float64           `json:"ground_truth_value"`    // Placeholder: 0.0 until provided
	GroundTruthAuth     ResolutionStatus  `json:"ground_truth_auth"`     // OPEN|RESOLVED
	GroundTruthSource   string            `json:"ground_truth_source"`   // "reference_device", "manual_measurement", "device_spec", etc.
	GroundTruthRef      string            `json:"ground_truth_ref"`      // Citation/document path if resolved

	// Artifact metadata
	MappingIdentity  string `json:"mapping_identity"`  // e.g., "siemens-infinity-147842-001"
	MappingVersion   string `json:"mapping_version"`   // e.g., "1.0.0"
	Provenance       string `json:"provenance"`        // Narrative: "Real device capture 001, OBX code 147842"
	ProvenanceAuthor string `json:"provenance_author"` // "PB-13 candidate builder"
	CreatedAt        string `json:"created_at"`        // ISO 8601 timestamp
	NormalizerIdentity string `json:"normalizer_identity"` // otris.normalizer.exact-evidence-mapping.v1

	// Resolution path
	AwaitingAuthority []string `json:"awaiting_authority"` // List of ["canonical_concept", "ground_truth", "canonical_unit"] that are OPEN
}

// New creates a candidate claim from real device evidence.
// Returns error if required provenance fields are missing.
func New(
	captureID string,
	sourceProtocol string,
	sourcePosition int,
	sourceCode, sourceUnit, sourceUnitLabel string,
	observedValue float64,
	parserIdentity, deviceFingerprint, deviceIdentityRef string,
	mappingIdentity, mappingVersion string,
) (*Claim, error) {
	if captureID == "" || sourceProtocol == "" || sourceCode == "" || sourceUnit == "" ||
		parserIdentity == "" || deviceFingerprint == "" || mappingIdentity == "" {
		return nil, errors.New("required provenance fields missing")
	}
	if sourcePosition <= 0 {
		return nil, errors.New("source_position must be positive")
	}
	if !utf8.ValidString(sourceCode) || !utf8.ValidString(sourceUnit) {
		return nil, errors.New("source_code and source_unit must be valid UTF-8")
	}

	c := &Claim{
		CaptureID:          captureID,
		SourceProtocol:     sourceProtocol,
		SourcePosition:     sourcePosition,
		SourceCode:         sourceCode,
		SourceUnit:         sourceUnit,
		SourceUnitLabel:    sourceUnitLabel,
		ObservedValue:      observedValue,
		ParserIdentity:     parserIdentity,
		DeviceFingerprint:  deviceFingerprint,
		DeviceIdentityRef:  deviceIdentityRef,
		MappingIdentity:    mappingIdentity,
		MappingVersion:     mappingVersion,
		CanonicalConceptAuth: ResolutionOpen,
		CanonicalUnitAuth:    ResolutionOpen,
		GroundTruthAuth:      ResolutionOpen,
		CreatedAt:           time.Now().Format(time.RFC3339),
		NormalizerIdentity:  "otris.normalizer.exact-evidence-mapping.v1",
		Provenance: fmt.Sprintf("Real device capture %s, protocol %s, OBX code %s", captureID, sourceProtocol, sourceCode),
		ProvenanceAuthor: "PB-13-candidate",
		AwaitingAuthority: []string{"canonical_concept", "canonical_unit", "ground_truth"},
	}

	return c, nil
}

// SetCanonicalConcept marks the canonical concept as resolved with authority reference.
func (c *Claim) SetCanonicalConcept(concept, reference string) error {
	if strings.TrimSpace(concept) == "" {
		return errors.New("canonical_concept cannot be empty")
	}
	if !utf8.ValidString(concept) {
		return errors.New("canonical_concept must be valid UTF-8")
	}
	c.CanonicalConcept = concept
	c.CanonicalConceptAuth = ResolutionResolved
	c.CanonicalConceptRef = reference
	c.updateAwaitingAuthority()
	return nil
}

// SetCanonicalUnit marks the canonical unit as resolved.
func (c *Claim) SetCanonicalUnit(unit, reference string) error {
	if strings.TrimSpace(unit) == "" {
		return errors.New("canonical_unit cannot be empty")
	}
	if !utf8.ValidString(unit) {
		return errors.New("canonical_unit must be valid UTF-8")
	}
	c.CanonicalUnit = unit
	c.CanonicalUnitAuth = ResolutionResolved
	c.CanonicalUnitRef = reference
	c.updateAwaitingAuthority()
	return nil
}

// SetGroundTruth marks the ground truth as resolved.
func (c *Claim) SetGroundTruth(value float64, source, reference string) error {
	if strings.TrimSpace(source) == "" {
		return errors.New("ground_truth source cannot be empty")
	}
	c.GroundTruthValue = value
	c.GroundTruthSource = source
	c.GroundTruthAuth = ResolutionResolved
	c.GroundTruthRef = reference
	c.updateAwaitingAuthority()
	return nil
}

// updateAwaitingAuthority recalculates the list of unresolved dependencies.
func (c *Claim) updateAwaitingAuthority() {
	var awaiting []string
	if c.CanonicalConceptAuth != ResolutionResolved {
		awaiting = append(awaiting, "canonical_concept")
	}
	if c.CanonicalUnitAuth != ResolutionResolved {
		awaiting = append(awaiting, "canonical_unit")
	}
	if c.GroundTruthAuth != ResolutionResolved {
		awaiting = append(awaiting, "ground_truth")
	}
	c.AwaitingAuthority = awaiting
}

// IsReady returns true if all required fields are resolved.
func (c *Claim) IsReady() bool {
	return len(c.AwaitingAuthority) == 0
}

// SaveCandidateRecord persists the candidate claim to JSON.
func SaveCandidateRecord(path string, claim *Claim) error {
	data, err := json.MarshalIndent(claim, "", "  ")
	if err != nil {
		return fmt.Errorf("encode candidate: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write candidate record: %w", err)
	}
	return nil
}

// LoadCandidateRecord loads a candidate claim from JSON.
func LoadCandidateRecord(path string) (*Claim, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read candidate record: %w", err)
	}
	var c Claim
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("decode candidate: %w", err)
	}
	return &c, nil
}

// PromoteToEvidence converts a resolved candidate claim to evidence.Claim for MappingDefinition.
// FAIL-CLOSED: all required authority must be resolved.
func (c *Claim) PromoteToEvidence() error {
	if len(c.AwaitingAuthority) > 0 {
		return fmt.Errorf("cannot promote unresolved candidate: awaiting %v", c.AwaitingAuthority)
	}
	if c.CanonicalConceptAuth != ResolutionResolved {
		return fmt.Errorf("canonical_concept not resolved")
	}
	if c.CanonicalUnitAuth != ResolutionResolved {
		return fmt.Errorf("canonical_unit not resolved")
	}
	if c.GroundTruthAuth != ResolutionResolved {
		return fmt.Errorf("ground_truth not resolved")
	}
	if strings.TrimSpace(c.CanonicalConcept) == "" {
		return fmt.Errorf("canonical_concept empty after resolution")
	}
	if strings.TrimSpace(c.CanonicalUnit) == "" {
		return fmt.Errorf("canonical_unit empty after resolution")
	}
	if strings.TrimSpace(c.CanonicalConceptRef) == "" {
		return fmt.Errorf("canonical_concept_ref missing (authority citation required)")
	}
	if strings.TrimSpace(c.CanonicalUnitRef) == "" {
		return fmt.Errorf("canonical_unit_ref missing (authority citation required)")
	}
	if strings.TrimSpace(c.GroundTruthRef) == "" {
		return fmt.Errorf("ground_truth_ref missing (measurement provenance required)")
	}
	if strings.TrimSpace(c.SourceCode) == "" || strings.TrimSpace(c.SourceUnit) == "" {
		return fmt.Errorf("source provenance incomplete")
	}
	if strings.TrimSpace(c.NormalizerIdentity) != "otris.normalizer.exact-evidence-mapping.v1" {
		return fmt.Errorf("normalizer identity invalid or derived: %s", c.NormalizerIdentity)
	}
	if c.SourcePosition <= 0 {
		return fmt.Errorf("source_position invalid")
	}
	return nil
}

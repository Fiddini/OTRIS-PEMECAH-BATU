package mapping

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Builder creates MappingDefinition CANDIDATE from explicit semantic claims.
// All semantic fields (concept, unit) must be provided explicitly; no inference.
// Fail-closed: UNKNOWN values, missing provenance, or test fixtures are rejected.
type Builder struct {
	// Explicit semantic claim (required, non-UNKNOWN)
	concept string
	unit    string

	// Provenance (required, non-empty)
	provenance string
	author     string

	// Device & parser context (optional)
	fingerprint   string
	deviceIdentity string
	parserId      string

	// Source protocol metadata (required)
	mappingIdentity string
	mappingVersion  string
	sourceProtocol  string
	sourcePosition  int
	sourceCode      string
	sourceUnit      string
}

// NewBuilder creates a new builder with explicit semantic fields.
// Returns error if concept, unit, provenance, or author are empty or UNKNOWN.
// All fields are validated for UTF-8 and non-whitespace.
func NewBuilder(
	mappingIdentity, mappingVersion string,
	sourceProtocol string, sourcePosition int,
	sourceCode, sourceUnit string,
	concept, unit string,
	provenance, author string,
) (*Builder, error) {
	// Validate semantic fields: no UNKNOWN, no empty
	if err := validateSemanticField("concept", concept); err != nil {
		return nil, err
	}
	if err := validateSemanticField("unit", unit); err != nil {
		return nil, err
	}

	// Validate provenance fields: required, non-empty
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, err
	}
	if err := validateRequiredField("author", author); err != nil {
		return nil, err
	}

	// Validate source protocol metadata
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
	// SourceUnit MAY be empty (unitless observations are valid).
	if sourceUnit != "" && !utf8.ValidString(sourceUnit) {
		return nil, fmt.Errorf("source_unit is invalid UTF-8")
	}

	return &Builder{
		concept:         concept,
		unit:            unit,
		provenance:      provenance,
		author:          author,
		mappingIdentity: mappingIdentity,
		mappingVersion:  mappingVersion,
		sourceProtocol:  sourceProtocol,
		sourcePosition:  sourcePosition,
		sourceCode:      sourceCode,
		sourceUnit:      sourceUnit,
	}, nil
}

// WithDeviceFingerprint sets the device fingerprint context (optional).
func (b *Builder) WithDeviceFingerprint(fingerprint string) *Builder {
	b.fingerprint = fingerprint
	return b
}

// WithDeviceIdentity sets the device identity reference (optional).
func (b *Builder) WithDeviceIdentity(identity string) *Builder {
	b.deviceIdentity = identity
	return b
}

// WithParserIdentity sets the parser identity (optional).
func (b *Builder) WithParserIdentity(parserId string) *Builder {
	b.parserId = parserId
	return b
}

// Build creates a MappingDefinition CANDIDATE with all explicit claim fields.
// Returns error if any required field is invalid.
// Does not perform status transitions; output is always CANDIDATE.
func (b *Builder) Build() (*MappingDefinition, error) {
	// Call mapping.New() with all validated fields
	m, err := New(
		b.mappingIdentity,
		b.mappingVersion,
		b.sourceProtocol,
		b.sourcePosition,
		b.sourceCode,
		b.sourceUnit,
		b.concept,
		b.unit,
		b.provenance,
		b.author,
	)
	if err != nil {
		return nil, fmt.Errorf("create MappingDefinition: %w", err)
	}

	// Set optional device context (never inferred)
	if b.fingerprint != "" {
		m.DeviceFingerprint = b.fingerprint
	}
	if b.deviceIdentity != "" {
		m.DeviceIdentityRef = b.deviceIdentity
	}
	if b.parserId != "" {
		m.ParserIdentity = b.parserId
	}

	return m, nil
}

// validateSemanticField checks that a semantic field (concept, unit) is:
// - not empty (after trimming)
// - not "UNKNOWN"
// - valid UTF-8
func validateSemanticField(fieldName, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("missing or empty %s", fieldName)
	}
	if !utf8.ValidString(trimmed) {
		return fmt.Errorf("invalid UTF-8 in %s", fieldName)
	}
	if strings.ToUpper(trimmed) == "UNKNOWN" {
		return fmt.Errorf("%s cannot be UNKNOWN (must be explicit concept/unit)", fieldName)
	}
	return nil
}

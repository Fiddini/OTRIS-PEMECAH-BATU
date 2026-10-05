package qc

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ReferenceLevel represents a single point on the reference scale.
// Levels are explicit, never inferred. Support LOW/NORMAL/HIGH default model.
// Extensible: any domain can define custom levels (CRITICAL, NORMAL, DEGRADED, etc.)
type ReferenceLevel struct {
	// Identity
	Level string `json:"level"` // "LOW", "NORMAL", "HIGH" or domain-specific label

	// Reference Value (optional: may not exist for qualitative observations)
	ExpectedValue string `json:"expected_value,omitempty"` // numeric or categorical, stored as string (no interpretation)
	Unit          string `json:"unit,omitempty"`           // unit label if applicable

	// Authority
	EvidenceRef string `json:"evidence_ref"` // citation: manual page, standard section, measurement method
	Author      string `json:"author"`       // who defined this reference level

	// Immutability
	Version string `json:"version"` // reference set version (for tracking changes)
}

// ReferenceDefinition binds reference levels to a specific mapping.
// Never auto-detect reference values. Always explicit.
type ReferenceDefinition struct {
	// Binding
	MappingIdentity string `json:"mapping_identity"` // exact match required
	MappingVersion  string `json:"mapping_version"`  // exact match required
	DeviceRef       string `json:"device_ref"`       // optional: device model/fingerprint

	// Reference Levels (explicit enumeration)
	Levels []ReferenceLevel `json:"levels"` // at least one level required

	// Metadata
	Provenance string `json:"provenance"` // how/why these reference levels were chosen
	Author     string `json:"author"`     // authority establishing these levels
	Version    string `json:"version"`    // reference definition version
}

// Validate enforces fail-closed requirements.
func (rd *ReferenceDefinition) Validate() error {
	if err := validateRequiredField("mapping_identity", rd.MappingIdentity); err != nil {
		return err
	}
	if err := validateRequiredField("mapping_version", rd.MappingVersion); err != nil {
		return err
	}
	if err := validateRequiredField("provenance", rd.Provenance); err != nil {
		return fmt.Errorf("reference definition: %w", err)
	}
	if err := validateRequiredField("author", rd.Author); err != nil {
		return fmt.Errorf("reference definition: %w", err)
	}

	// Explicit levels required (no auto-generation)
	if len(rd.Levels) == 0 {
		return errors.New("reference definition: at least one reference level required (not auto-generated)")
	}

	// Validate each level
	seenLevels := make(map[string]bool)
	for i, level := range rd.Levels {
		if strings.TrimSpace(level.Level) == "" {
			return fmt.Errorf("reference level %d: level name is empty", i)
		}
		if !utf8.ValidString(level.Level) {
			return fmt.Errorf("reference level %d: invalid UTF-8", i)
		}
		if seenLevels[level.Level] {
			return fmt.Errorf("reference level %d: duplicate level %q", i, level.Level)
		}
		seenLevels[level.Level] = true

		// Optional expected value/unit must be valid UTF-8 if present
		if level.ExpectedValue != "" && !utf8.ValidString(level.ExpectedValue) {
			return fmt.Errorf("reference level %d: expected_value invalid UTF-8", i)
		}
		if level.Unit != "" && !utf8.ValidString(level.Unit) {
			return fmt.Errorf("reference level %d: unit invalid UTF-8", i)
		}

		// Evidence reference required (no inference)
		if err := validateRequiredField(fmt.Sprintf("level_%s.evidence_ref", level.Level), level.EvidenceRef); err != nil {
			return err
		}
		// Author required for each level
		if err := validateRequiredField(fmt.Sprintf("level_%s.author", level.Level), level.Author); err != nil {
			return err
		}
	}

	return nil
}

// validateRequiredField checks non-empty, valid UTF-8, no whitespace-only
func validateRequiredField(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("required field %s is empty or whitespace-only", name)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("required field %s is invalid UTF-8", name)
	}
	return nil
}

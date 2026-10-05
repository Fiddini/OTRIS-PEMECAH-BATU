package qc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ReferenceChallenge records a device measurement against a known reference level.
// Raw observed value is preserved exactly — no interpretation, no inference.
// Immutable: hash integrity enforced.
type ReferenceChallenge struct {
	// Identity (must match ReferenceDefinition exactly)
	MappingIdentity string `json:"mapping_identity"`
	MappingVersion  string `json:"mapping_version"`
	RefLevel        string `json:"ref_level"` // must exist in corresponding ReferenceDefinition.Levels

	// Device Context
	DeviceFingerprint string `json:"device_fingerprint"` // structural hash from PB-10 (if applicable)

	// Raw Observation (immutable, exact preservation)
	RawValue string `json:"raw_value"` // preserved exactly as observed (no interpretation)

	// Authority
	Provenance string    `json:"provenance"` // how measurement was obtained
	Operator   string    `json:"operator"`   // who/what recorded the measurement
	Timestamp  time.Time `json:"timestamp"`  // when measurement was taken

	// Integrity (computed at creation, immutable)
	Digest string `json:"digest"` // SHA256 hash of all fields above

	// immutable markers (not JSON serialized)
	sealed bool
}

// NewChallenge creates a new reference challenge with integrity hash.
// Fail-closed: all fields required, validated before hash computation.
func NewChallenge(
	mappingID, mappingVersion, refLevel, deviceFingerprint, rawValue,
	provenance, operator string,
	timestamp time.Time,
) (*ReferenceChallenge, error) {
	// Validate all required fields before sealing
	if err := validateRequiredField("mapping_identity", mappingID); err != nil {
		return nil, fmt.Errorf("reference challenge: %w", err)
	}
	if err := validateRequiredField("mapping_version", mappingVersion); err != nil {
		return nil, fmt.Errorf("reference challenge: %w", err)
	}
	if err := validateRequiredField("ref_level", refLevel); err != nil {
		return nil, fmt.Errorf("reference challenge: %w", err)
	}
	if err := validateRequiredField("device_fingerprint", deviceFingerprint); err != nil {
		return nil, fmt.Errorf("reference challenge: %w", err)
	}
	// RawValue may be empty (e.g., disconnected sensor) but must be valid UTF-8 if present
	if rawValue != "" && !utf8.ValidString(rawValue) {
		return nil, errors.New("reference challenge: raw_value invalid UTF-8")
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("reference challenge: %w", err)
	}
	if err := validateRequiredField("operator", operator); err != nil {
		return nil, fmt.Errorf("reference challenge: %w", err)
	}
	if timestamp.IsZero() {
		return nil, errors.New("reference challenge: timestamp required")
	}

	ch := &ReferenceChallenge{
		MappingIdentity:   mappingID,
		MappingVersion:    mappingVersion,
		RefLevel:          refLevel,
		DeviceFingerprint: deviceFingerprint,
		RawValue:          rawValue,
		Provenance:        provenance,
		Operator:          operator,
		Timestamp:         timestamp.UTC(),
	}

	// Compute digest for integrity
	ch.Digest = ch.computeDigest()
	ch.sealed = true

	return ch, nil
}

// computeDigest returns SHA256 of canonical representation.
// Must be deterministic for same inputs.
func (ch *ReferenceChallenge) computeDigest() string {
	// Canonical order (not JSON order) to ensure reproducibility
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%d",
		ch.MappingIdentity,
		ch.MappingVersion,
		ch.RefLevel,
		ch.DeviceFingerprint,
		ch.RawValue,
		ch.Provenance,
		ch.Operator,
		ch.Timestamp.UnixNano(),
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// Verify checks integrity: digest matches current state.
// Returns error if digest mismatch (data corruption or unauthorized modification).
func (ch *ReferenceChallenge) Verify() error {
	if !ch.sealed {
		return errors.New("reference challenge: not sealed (should not happen)")
	}
	expected := ch.computeDigest()
	if ch.Digest != expected {
		return fmt.Errorf("reference challenge: digest mismatch (expected %s, got %s)", expected, ch.Digest)
	}
	return nil
}

// BindsTo validates that this challenge matches the reference definition.
// Returns error if binding constraints violated.
func (ch *ReferenceChallenge) BindsTo(rd *ReferenceDefinition) error {
	if err := ch.Verify(); err != nil {
		return fmt.Errorf("challenge integrity failed: %w", err)
	}

	if ch.MappingIdentity != rd.MappingIdentity {
		return fmt.Errorf(
			"reference challenge mapping mismatch: challenge has %q, definition has %q",
			ch.MappingIdentity, rd.MappingIdentity,
		)
	}
	if ch.MappingVersion != rd.MappingVersion {
		return fmt.Errorf(
			"reference challenge version mismatch: challenge has %q, definition has %q",
			ch.MappingVersion, rd.MappingVersion,
		)
	}

	// Verify device fingerprint matches (if definition specifies one)
	if rd.DeviceRef != "" && ch.DeviceFingerprint != rd.DeviceRef {
		return fmt.Errorf(
			"reference challenge device mismatch: challenge has %q, definition has %q",
			ch.DeviceFingerprint, rd.DeviceRef,
		)
	}

	// Verify reference level exists in definition
	levelFound := false
	for _, level := range rd.Levels {
		if level.Level == ch.RefLevel {
			levelFound = true
			break
		}
	}
	if !levelFound {
		return fmt.Errorf(
			"reference challenge level %q not found in definition (available: %s)",
			ch.RefLevel, availableLevels(rd),
		)
	}

	return nil
}

func availableLevels(rd *ReferenceDefinition) string {
	var levels []string
	for _, level := range rd.Levels {
		levels = append(levels, level.Level)
	}
	return strings.Join(levels, ", ")
}

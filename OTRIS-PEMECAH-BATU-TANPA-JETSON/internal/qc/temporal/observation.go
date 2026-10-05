package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// TimeOrderedObservationSet is an immutable artifact preserving temporal ordering
// of observations within a run, day, or longitudinal scope.
// References immutable ReferenceChallenge records by digest, does not copy raw evidence.
type TimeOrderedObservationSet struct {
	// Identity
	SetID              string `json:"set_id"`              // unique identifier for this observation set
	DeviceFingerprint  string `json:"device_fingerprint"` // from MappingDefinition context
	MappingID          string `json:"mapping_identity"`
	ReferenceID        string `json:"reference_id"`       // which reference definition?
	ReferenceLevel     string `json:"reference_level"`    // LOW, NORMAL, HIGH, etc.

	// Temporal Scope (explicit, never inferred)
	Scope              string `json:"scope"`              // WITHIN_RUN, WITHIN_DAY, LONGITUDINAL
	RunID              string `json:"run_id"`             // explicit run identity (required)
	DayID              string `json:"day_id,omitempty"`   // explicit day identity (for WITHIN_DAY, LONGITUDINAL)
	KnowledgeEpochID   string `json:"knowledge_epoch_id,omitempty"` // explicit epoch (for LONGITUDINAL)

	// Ordered Observation References (immutable, immutable digests of challenges)
	Observations       []ObservationReference `json:"observations"` // time-ordered references to challenges
	ObservationCount   int                    `json:"observation_count"`

	// Authority
	Provenance         string    `json:"provenance"` // how this set was created
	CreatedAt          time.Time `json:"created_at"`
	CreatedBy          string    `json:"created_by"`

	// Integrity (immutable)
	Digest             string `json:"digest"` // SHA256 of canonical representation
	Version            string `json:"version"` // semantic version

	// sealed marker (internal only, not JSON serialized)
	sealed             bool
}

// ObservationReference points to an immutable ReferenceChallenge by digest.
// Does NOT copy the challenge; preserves temporal order via explicit Sequence.
type ObservationReference struct {
	ChallengeDigest    string    `json:"challenge_digest"`    // SHA256 of ReferenceChallenge
	Timestamp          time.Time `json:"timestamp"`           // when this observation was recorded
	Sequence           int64     `json:"sequence"`            // order within run (0-based, must be unique or explicitly handled)
	RawValue           string    `json:"raw_value"`           // preserved as observed (no interpretation)
	IsMissing          bool      `json:"is_missing"`          // true if UNKNOWN/absent, false if present
	IsOutlier          bool      `json:"is_outlier,omitempty"` // marked by policy, but NOT deleted

	// Preservation
	MappingID          string    `json:"mapping_id"`
	ReferenceLevel     string    `json:"reference_level"`
	DeviceFingerprint  string    `json:"device_fingerprint"`
}

// New creates a new TimeOrderedObservationSet with integrity digest.
// All fields required; immutable after creation.
func New(
	setID, deviceFP, mappingID, refID, refLevel string,
	scope, runID, dayID, epochID string,
	observations []ObservationReference,
	provenance, createdBy, version string,
) (*TimeOrderedObservationSet, error) {
	// Validate required fields
	if err := validateRequiredField("set_id", setID); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}
	if err := validateRequiredField("device_fingerprint", deviceFP); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}
	if err := validateRequiredField("mapping_id", mappingID); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}
	if err := validateRequiredField("reference_id", refID); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}
	if err := validateRequiredField("reference_level", refLevel); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}

	// Validate scope
	if scope != "WITHIN_RUN" && scope != "WITHIN_DAY" && scope != "LONGITUDINAL" {
		return nil, fmt.Errorf("time-ordered observation set: invalid scope %q", scope)
	}

	// Validate temporal identity
	if err := validateRequiredField("run_id", runID); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}

	if scope == "WITHIN_DAY" || scope == "LONGITUDINAL" {
		if err := validateRequiredField("day_id", dayID); err != nil {
			return nil, fmt.Errorf("time-ordered observation set: %w (required for %s scope)", err, scope)
		}
	}

	if scope == "LONGITUDINAL" {
		if err := validateRequiredField("knowledge_epoch_id", epochID); err != nil {
			return nil, fmt.Errorf("time-ordered observation set: %w (required for LONGITUDINAL)", err)
		}
	}

	// Validate observations
	if len(observations) == 0 {
		return nil, errors.New("time-ordered observation set: at least one observation required")
	}

	// Validate observation sequences (must be unique or explicitly duplicated, never silent merge)
	seenSequences := make(map[int64]int)
	for i, obs := range observations {
		if obs.Sequence < 0 {
			return nil, fmt.Errorf("time-ordered observation set: observation %d has negative sequence %d", i, obs.Sequence)
		}
		seenSequences[obs.Sequence]++
	}

	// Check for duplicate sequences (allowed only with explicit governance)
	for seq, count := range seenSequences {
		if count > 1 {
			return nil, fmt.Errorf("time-ordered observation set: duplicate sequence %d found %d times (must be explicitly governed)", seq, count)
		}
	}

	// Validate observation timestamps are ordered
	for i := 1; i < len(observations); i++ {
		if observations[i].Timestamp.Before(observations[i-1].Timestamp) {
			return nil, fmt.Errorf("time-ordered observation set: observation %d timestamp before previous (not time-ordered)", i)
		}
	}

	// Validate provenance
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}

	// Validate created_by
	if err := validateRequiredField("created_by", createdBy); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}

	// Validate version
	if err := validateRequiredField("version", version); err != nil {
		return nil, fmt.Errorf("time-ordered observation set: %w", err)
	}

	set := &TimeOrderedObservationSet{
		SetID:              setID,
		DeviceFingerprint:  deviceFP,
		MappingID:          mappingID,
		ReferenceID:        refID,
		ReferenceLevel:     refLevel,
		Scope:              scope,
		RunID:              runID,
		DayID:              dayID,
		KnowledgeEpochID:   epochID,
		Observations:       observations,
		ObservationCount:   len(observations),
		Provenance:         provenance,
		CreatedAt:          time.Now().UTC(),
		CreatedBy:          createdBy,
		Version:            version,
	}

	// Compute and seal digest
	set.Digest = set.computeDigest()
	set.sealed = true

	return set, nil
}

// computeDigest returns immutable integrity hash.
func (tos *TimeOrderedObservationSet) computeDigest() string {
	// Canonical representation: deterministic ordering
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%s|%d|",
		tos.SetID,
		tos.DeviceFingerprint,
		tos.MappingID,
		tos.ReferenceID,
		tos.ReferenceLevel,
		tos.Scope,
		tos.RunID,
		tos.DayID,
		tos.KnowledgeEpochID,
		len(tos.Observations),
	)

	// Add each observation's digest in order
	for _, obs := range tos.Observations {
		canonical += fmt.Sprintf("%s|%d|%d|", obs.ChallengeDigest, obs.Sequence, obs.Timestamp.UnixNano())
	}

	canonical += fmt.Sprintf("%s|%s", tos.Provenance, tos.Version)

	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// Verify checks integrity of this artifact.
func (tos *TimeOrderedObservationSet) Verify() error {
	if !tos.sealed {
		return errors.New("time-ordered observation set: not sealed (should not happen)")
	}

	expected := tos.computeDigest()
	if tos.Digest != expected {
		return fmt.Errorf(
			"time-ordered observation set: digest mismatch (expected %s, got %s)",
			expected, tos.Digest,
		)
	}

	return nil
}

// validateRequiredField checks non-empty, valid UTF-8, no whitespace-only
func validateRequiredField(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s required, got empty", name)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s invalid UTF-8", name)
	}
	return nil
}

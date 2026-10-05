package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// PatternCandidateType identifies the kind of pattern detected (diagnostic only).
type PatternCandidateType string

const (
	// TrendCandidate: linear trend detected (slope > threshold).
	TrendCandidate PatternCandidateType = "TREND_CANDIDATE"
	// ShiftCandidate: step change in mean detected (before/after comparison).
	ShiftCandidate PatternCandidateType = "SHIFT_CANDIDATE"
	// RecurrenceCandidate: periodic/cyclic pattern detected (spectral analysis).
	RecurrenceCandidate PatternCandidateType = "RECURRENCE_CANDIDATE"
)

// PatternCandidate is diagnostic evidence only. It CANNOT:
// - Modify mapping definitions
// - Modify reference definitions
// - Create semantic authority
// - Modify profile status
// - Modify registry admission
// - Modify any governed decision
//
// PatternCandidate IS:
// - Observable diagnostic trend/shift/recurrence marker
// - Human-reviewable summary for trend analysis
// - Optional input to manual investigation
// - Explicitly marked "cannot_trigger" authority modification
type PatternCandidate struct {
	// Identity
	CandidateID            string                  `json:"candidate_id"`            // unique identifier
	PatternType            PatternCandidateType    `json:"pattern_type"`            // TREND/SHIFT/RECURRENCE
	KnowledgeSourceID      string                  `json:"knowledge_source_id"`     // references WithinRunTemporalKnowledge/WithinDayKnowledge/LongitudinalQCKnowledge by digest
	ReferenceID            string                  `json:"reference_id"`
	DeviceFingerprint      string                  `json:"device_fingerprint"`
	MappingID              string                  `json:"mapping_id"`

	// Detection Details
	ConfidenceScore        float64                 `json:"confidence_score"`        // 0.0 to 1.0 (how confident in the pattern)
	ConfidenceThreshold    float64                 `json:"confidence_threshold"`    // policy-defined threshold
	DetectionMethod        string                  `json:"detection_method"`        // e.g., "linear_regression", "step_detection", "autocorrelation"

	// Pattern-Specific Data (polymorphic)
	TrendSlope             *float64                `json:"trend_slope,omitempty"`   // for TREND_CANDIDATE
	TrendIntercept         *float64                `json:"trend_intercept,omitempty"` // for TREND_CANDIDATE
	TrendRSquared          *float64                `json:"trend_r_squared,omitempty"` // for TREND_CANDIDATE
	ShiftMagnitude         *float64                `json:"shift_magnitude,omitempty"` // for SHIFT_CANDIDATE (delta mean)
	ShiftTimepoint         *int64                  `json:"shift_timepoint,omitempty"` // for SHIFT_CANDIDATE (sequence index)
	RecurrencePeriod       *int64                  `json:"recurrence_period,omitempty"` // for RECURRENCE_CANDIDATE (observations)
	RecurrenceFundamental  *float64                `json:"recurrence_fundamental,omitempty"` // for RECURRENCE_CANDIDATE (Hz)

	// Governance Authority Barrier
	CannotTrigger          CannotTriggerAuthority  `json:"cannot_trigger"`          // explicit marker: cannot modify any authority
	NeedsHumanReview       bool                    `json:"needs_human_review"`      // flagged for manual investigation
	InvestigationNotes     string                  `json:"investigation_notes"`     // optional human-added context

	// Provenance
	Provenance             string                  `json:"provenance"`              // how detected
	CreatedAt              time.Time               `json:"created_at"`
	CreatedBy              string                  `json:"created_by"`

	// Integrity
	Digest                 string                  `json:"digest"`
	Version                string                  `json:"version"`

	sealed                 bool
}

// CannotTriggerAuthority is an explicit marker that PatternCandidate cannot modify authority.
type CannotTriggerAuthority struct {
	CannotModifyMappingDefinition     bool   `json:"cannot_modify_mapping_definition"`
	CannotModifyReferenceDefinition   bool   `json:"cannot_modify_reference_definition"`
	CannotModifyProfile               bool   `json:"cannot_modify_profile"`
	CannotModifyRegistry              bool   `json:"cannot_modify_registry"`
	CannotModifyFactor                bool   `json:"cannot_modify_factor"`
	CannotModifySemanticAuthority     bool   `json:"cannot_modify_semantic_authority"`
	ExplicitAuthorityBarrier          string `json:"explicit_authority_barrier"` // human-readable reason
}

// NewPatternCandidate creates a diagnostic pattern candidate.
func NewPatternCandidate(
	candidateID string,
	patternType PatternCandidateType,
	knowledgeSourceID string,
	referenceID, deviceFP, mappingID string,
	confidenceScore, confidenceThreshold float64,
	detectionMethod string,
	provenance, createdBy, version string,
) (*PatternCandidate, error) {
	if err := validateRequiredField("candidate_id", candidateID); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if patternType != TrendCandidate && patternType != ShiftCandidate && patternType != RecurrenceCandidate {
		return nil, fmt.Errorf("pattern candidate: invalid pattern type %q", patternType)
	}

	if err := validateRequiredField("knowledge_source_id", knowledgeSourceID); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if err := validateRequiredField("reference_id", referenceID); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if err := validateRequiredField("device_fingerprint", deviceFP); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if err := validateRequiredField("mapping_id", mappingID); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if confidenceScore < 0.0 || confidenceScore > 1.0 {
		return nil, fmt.Errorf("pattern candidate: confidence score must be 0.0-1.0, got %f", confidenceScore)
	}

	if confidenceThreshold < 0.0 || confidenceThreshold > 1.0 {
		return nil, fmt.Errorf("pattern candidate: confidence threshold must be 0.0-1.0, got %f", confidenceThreshold)
	}

	if err := validateRequiredField("detection_method", detectionMethod); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if err := validateRequiredField("created_by", createdBy); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	if err := validateRequiredField("version", version); err != nil {
		return nil, fmt.Errorf("pattern candidate: %w", err)
	}

	pc := &PatternCandidate{
		CandidateID:           candidateID,
		PatternType:           patternType,
		KnowledgeSourceID:     knowledgeSourceID,
		ReferenceID:           referenceID,
		DeviceFingerprint:     deviceFP,
		MappingID:             mappingID,
		ConfidenceScore:       confidenceScore,
		ConfidenceThreshold:   confidenceThreshold,
		DetectionMethod:       detectionMethod,
		CannotTrigger: CannotTriggerAuthority{
			CannotModifyMappingDefinition:   true,
			CannotModifyReferenceDefinition: true,
			CannotModifyProfile:             true,
			CannotModifyRegistry:            true,
			CannotModifyFactor:              true,
			CannotModifySemanticAuthority:   true,
			ExplicitAuthorityBarrier:        "PatternCandidate is diagnostic evidence only; all authority gates remain unchanged",
		},
		NeedsHumanReview:  confidenceScore >= confidenceThreshold, // auto-flag if confident
		Provenance:        provenance,
		CreatedAt:         time.Now().UTC(),
		CreatedBy:         createdBy,
		Version:           version,
	}

	pc.Digest = pc.computeDigest()
	pc.sealed = true

	return pc, nil
}

// SetTrendData sets trend-specific fields (validates TREND_CANDIDATE type).
func (pc *PatternCandidate) SetTrendData(slope, intercept, rSquared float64) error {
	if !pc.sealed {
		return errors.New("pattern candidate: cannot modify sealed candidate")
	}
	if pc.PatternType != TrendCandidate {
		return fmt.Errorf("pattern candidate: SetTrendData only valid for TREND_CANDIDATE, got %q", pc.PatternType)
	}
	pc.TrendSlope = &slope
	pc.TrendIntercept = &intercept
	pc.TrendRSquared = &rSquared
	// Recompute digest after modification (breaks seal, but intentional for data setting)
	pc.sealed = false
	pc.Digest = pc.computeDigest()
	pc.sealed = true
	return nil
}

// SetShiftData sets shift-specific fields (validates SHIFT_CANDIDATE type).
func (pc *PatternCandidate) SetShiftData(magnitude float64, timepoint int64) error {
	if !pc.sealed {
		return errors.New("pattern candidate: cannot modify sealed candidate")
	}
	if pc.PatternType != ShiftCandidate {
		return fmt.Errorf("pattern candidate: SetShiftData only valid for SHIFT_CANDIDATE, got %q", pc.PatternType)
	}
	if timepoint < 0 {
		return fmt.Errorf("pattern candidate: shift timepoint must be >= 0, got %d", timepoint)
	}
	pc.ShiftMagnitude = &magnitude
	pc.ShiftTimepoint = &timepoint
	pc.sealed = false
	pc.Digest = pc.computeDigest()
	pc.sealed = true
	return nil
}

// SetRecurrenceData sets recurrence-specific fields (validates RECURRENCE_CANDIDATE type).
func (pc *PatternCandidate) SetRecurrenceData(period int64, fundamental float64) error {
	if !pc.sealed {
		return errors.New("pattern candidate: cannot modify sealed candidate")
	}
	if pc.PatternType != RecurrenceCandidate {
		return fmt.Errorf("pattern candidate: SetRecurrenceData only valid for RECURRENCE_CANDIDATE, got %q", pc.PatternType)
	}
	if period < 1 {
		return fmt.Errorf("pattern candidate: recurrence period must be >= 1, got %d", period)
	}
	if fundamental < 0.0 {
		return fmt.Errorf("pattern candidate: recurrence fundamental must be >= 0.0, got %f", fundamental)
	}
	pc.RecurrencePeriod = &period
	pc.RecurrenceFundamental = &fundamental
	pc.sealed = false
	pc.Digest = pc.computeDigest()
	pc.sealed = true
	return nil
}

// Verify checks integrity of this candidate.
func (pc *PatternCandidate) Verify() error {
	if !pc.sealed {
		return errors.New("pattern candidate: not sealed")
	}
	expected := pc.computeDigest()
	if pc.Digest != expected {
		return fmt.Errorf("pattern candidate: digest mismatch (expected %s, got %s)", expected, pc.Digest)
	}
	return nil
}

// CanAuthorityModify returns false always (diagnostic only).
func (pc *PatternCandidate) CanAuthorityModify() bool {
	return false // diagnostic evidence only, never triggers authority modification
}

// computeDigest returns immutable integrity hash (includes pattern-specific data).
func (pc *PatternCandidate) computeDigest() string {
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%f|%s|",
		pc.CandidateID,
		pc.PatternType,
		pc.KnowledgeSourceID,
		pc.ReferenceID,
		pc.DeviceFingerprint,
		pc.MappingID,
		pc.ConfidenceScore,
		pc.DetectionMethod,
	)

	// Include pattern-specific data
	if pc.TrendSlope != nil {
		canonical += fmt.Sprintf("TREND|%f|%f|%f|", *pc.TrendSlope, *pc.TrendIntercept, *pc.TrendRSquared)
	} else if pc.ShiftMagnitude != nil {
		canonical += fmt.Sprintf("SHIFT|%f|%d|", *pc.ShiftMagnitude, *pc.ShiftTimepoint)
	} else if pc.RecurrencePeriod != nil {
		canonical += fmt.Sprintf("RECURRENCE|%d|%f|", *pc.RecurrencePeriod, *pc.RecurrenceFundamental)
	}

	canonical += fmt.Sprintf("%s|%s", pc.Provenance, pc.Version)

	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

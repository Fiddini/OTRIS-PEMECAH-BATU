package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// WithinRunTemporalKnowledge captures QC observations within an explicit run boundary.
// RunID is an explicit governed identity, not inferred from timestamps.
type WithinRunTemporalKnowledge struct {
	RunID                  string                      `json:"run_id"`                // explicit run identity
	ObservationSet         *TimeOrderedObservationSet  `json:"observation_set"`      // time-ordered references
	StatisticalSummary     *RunStatisticalSummary      `json:"statistical_summary"`  // aggregated stats
	Provenance             string                      `json:"provenance"`           // how this knowledge was created
	CreatedAt              time.Time                   `json:"created_at"`
	CreatedBy              string                      `json:"created_by"`
	Digest                 string                      `json:"digest"`
	Version                string                      `json:"version"`
	sealed                 bool
}

// RunStatisticalSummary preserves statistics without losing observation order.
type RunStatisticalSummary struct {
	ObservationCount       int64                       `json:"observation_count"`
	MissingCount           int64                       `json:"missing_count"`
	MeanValue              float64                     `json:"mean_value"`
	StdDev                 float64                     `json:"std_dev"`
	CoefficientOfVariation float64                     `json:"coefficient_of_variation"` // can be NaN
	MinValue               float64                     `json:"min_value"`
	MaxValue               float64                     `json:"max_value"`
	MedianValue            float64                     `json:"median_value"`
}

// WithinDayKnowledge captures QC observations within an explicit day boundary.
// DayID is an explicit governed identity (policy-defined), not inferred from timestamps sharing a calendar date.
type WithinDayKnowledge struct {
	RunID                  string                      `json:"run_id"`
	DayID                  string                      `json:"day_id"`                // explicit day identity
	ObservationSet         *TimeOrderedObservationSet  `json:"observation_set"`      // observations within this day
	StatisticalSummary     *DayStatisticalSummary      `json:"statistical_summary"`
	RunCountThisDay        int64                       `json:"run_count_this_day"`   // how many distinct runs in this day?
	Provenance             string                      `json:"provenance"`
	CreatedAt              time.Time                   `json:"created_at"`
	CreatedBy              string                      `json:"created_by"`
	Digest                 string                      `json:"digest"`
	Version                string                      `json:"version"`
	sealed                 bool
}

// DayStatisticalSummary aggregates within-day observations.
type DayStatisticalSummary struct {
	ObservationCount       int64                       `json:"observation_count"`
	MissingCount           int64                       `json:"missing_count"`
	MeanValue              float64                     `json:"mean_value"`
	StdDev                 float64                     `json:"std_dev"`
	CoefficientOfVariation float64                     `json:"coefficient_of_variation"`
	MinValue               float64                     `json:"min_value"`
	MaxValue               float64                     `json:"max_value"`
	MedianValue            float64                     `json:"median_value"`
}

// LongitudinalQCKnowledge captures observations across multiple days/epochs.
// KnowledgeEpochID is an explicit governed epoch (policy-defined), not universal temporal rules.
type LongitudinalQCKnowledge struct {
	KnowledgeEpochID       string                      `json:"knowledge_epoch_id"`   // explicit epoch identity
	ObservationSet         *TimeOrderedObservationSet  `json:"observation_set"`      // all observations in epoch
	StatisticalSummary     *LongitudinalStatisticalSummary `json:"statistical_summary"`
	DayCountInEpoch        int64                       `json:"day_count_in_epoch"`
	RunCountInEpoch        int64                       `json:"run_count_in_epoch"`
	Provenance             string                      `json:"provenance"`
	CreatedAt              time.Time                   `json:"created_at"`
	CreatedBy              string                      `json:"created_by"`
	Digest                 string                      `json:"digest"`
	Version                string                      `json:"version"`
	sealed                 bool
}

// LongitudinalStatisticalSummary aggregates across multiple days.
type LongitudinalStatisticalSummary struct {
	ObservationCount       int64                       `json:"observation_count"`
	MissingCount           int64                       `json:"missing_count"`
	MeanValue              float64                     `json:"mean_value"`
	StdDev                 float64                     `json:"std_dev"`
	CoefficientOfVariation float64                     `json:"coefficient_of_variation"`
	MinValue               float64                     `json:"min_value"`
	MaxValue               float64                     `json:"max_value"`
	MedianValue            float64                     `json:"median_value"`
	TrendSlope             float64                     `json:"trend_slope,omitempty"` // linear regression slope (optional)
}

// NewWithinRunTemporalKnowledge creates WITHIN_RUN temporal knowledge.
func NewWithinRunTemporalKnowledge(
	runID string,
	obsSet *TimeOrderedObservationSet,
	summary *RunStatisticalSummary,
	provenance, createdBy, version string,
) (*WithinRunTemporalKnowledge, error) {
	if err := validateRequiredField("run_id", runID); err != nil {
		return nil, fmt.Errorf("within-run temporal knowledge: %w", err)
	}
	if obsSet == nil {
		return nil, errors.New("within-run temporal knowledge: observation set required")
	}
	if obsSet.Scope != "WITHIN_RUN" {
		return nil, fmt.Errorf("within-run temporal knowledge: observation set scope must be WITHIN_RUN, got %q", obsSet.Scope)
	}
	if summary == nil {
		return nil, errors.New("within-run temporal knowledge: statistical summary required")
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("within-run temporal knowledge: %w", err)
	}
	if err := validateRequiredField("created_by", createdBy); err != nil {
		return nil, fmt.Errorf("within-run temporal knowledge: %w", err)
	}
	if err := validateRequiredField("version", version); err != nil {
		return nil, fmt.Errorf("within-run temporal knowledge: %w", err)
	}

	k := &WithinRunTemporalKnowledge{
		RunID:              runID,
		ObservationSet:     obsSet,
		StatisticalSummary: summary,
		Provenance:         provenance,
		CreatedAt:          time.Now().UTC(),
		CreatedBy:          createdBy,
		Version:            version,
	}

	k.Digest = k.computeDigest()
	k.sealed = true

	return k, nil
}

// NewWithinDayKnowledge creates WITHIN_DAY temporal knowledge.
func NewWithinDayKnowledge(
	runID, dayID string,
	obsSet *TimeOrderedObservationSet,
	summary *DayStatisticalSummary,
	runCount int64,
	provenance, createdBy, version string,
) (*WithinDayKnowledge, error) {
	if err := validateRequiredField("run_id", runID); err != nil {
		return nil, fmt.Errorf("within-day temporal knowledge: %w", err)
	}
	if err := validateRequiredField("day_id", dayID); err != nil {
		return nil, fmt.Errorf("within-day temporal knowledge: %w", err)
	}
	if obsSet == nil {
		return nil, errors.New("within-day temporal knowledge: observation set required")
	}
	if obsSet.Scope != "WITHIN_DAY" {
		return nil, fmt.Errorf("within-day temporal knowledge: observation set scope must be WITHIN_DAY, got %q", obsSet.Scope)
	}
	if summary == nil {
		return nil, errors.New("within-day temporal knowledge: statistical summary required")
	}
	if runCount < 1 {
		return nil, fmt.Errorf("within-day temporal knowledge: run count must be >= 1, got %d", runCount)
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("within-day temporal knowledge: %w", err)
	}
	if err := validateRequiredField("created_by", createdBy); err != nil {
		return nil, fmt.Errorf("within-day temporal knowledge: %w", err)
	}
	if err := validateRequiredField("version", version); err != nil {
		return nil, fmt.Errorf("within-day temporal knowledge: %w", err)
	}

	k := &WithinDayKnowledge{
		RunID:           runID,
		DayID:           dayID,
		ObservationSet:  obsSet,
		StatisticalSummary: summary,
		RunCountThisDay: runCount,
		Provenance:      provenance,
		CreatedAt:       time.Now().UTC(),
		CreatedBy:       createdBy,
		Version:         version,
	}

	k.Digest = k.computeDigest()
	k.sealed = true

	return k, nil
}

// NewLongitudinalQCKnowledge creates LONGITUDINAL temporal knowledge.
func NewLongitudinalQCKnowledge(
	epochID string,
	obsSet *TimeOrderedObservationSet,
	summary *LongitudinalStatisticalSummary,
	dayCount, runCount int64,
	provenance, createdBy, version string,
) (*LongitudinalQCKnowledge, error) {
	if err := validateRequiredField("knowledge_epoch_id", epochID); err != nil {
		return nil, fmt.Errorf("longitudinal qc knowledge: %w", err)
	}
	if obsSet == nil {
		return nil, errors.New("longitudinal qc knowledge: observation set required")
	}
	if obsSet.Scope != "LONGITUDINAL" {
		return nil, fmt.Errorf("longitudinal qc knowledge: observation set scope must be LONGITUDINAL, got %q", obsSet.Scope)
	}
	if summary == nil {
		return nil, errors.New("longitudinal qc knowledge: statistical summary required")
	}
	if dayCount < 1 {
		return nil, fmt.Errorf("longitudinal qc knowledge: day count must be >= 1, got %d", dayCount)
	}
	if runCount < 1 {
		return nil, fmt.Errorf("longitudinal qc knowledge: run count must be >= 1, got %d", runCount)
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("longitudinal qc knowledge: %w", err)
	}
	if err := validateRequiredField("created_by", createdBy); err != nil {
		return nil, fmt.Errorf("longitudinal qc knowledge: %w", err)
	}
	if err := validateRequiredField("version", version); err != nil {
		return nil, fmt.Errorf("longitudinal qc knowledge: %w", err)
	}

	k := &LongitudinalQCKnowledge{
		KnowledgeEpochID:    epochID,
		ObservationSet:      obsSet,
		StatisticalSummary:  summary,
		DayCountInEpoch:     dayCount,
		RunCountInEpoch:     runCount,
		Provenance:          provenance,
		CreatedAt:           time.Now().UTC(),
		CreatedBy:           createdBy,
		Version:             version,
	}

	k.Digest = k.computeDigest()
	k.sealed = true

	return k, nil
}

// Verify checks WITHIN_RUN integrity.
func (k *WithinRunTemporalKnowledge) Verify() error {
	if !k.sealed {
		return errors.New("within-run temporal knowledge: not sealed")
	}
	expected := k.computeDigest()
	if k.Digest != expected {
		return fmt.Errorf("within-run temporal knowledge: digest mismatch (expected %s, got %s)", expected, k.Digest)
	}
	return nil
}

// Verify checks WITHIN_DAY integrity.
func (k *WithinDayKnowledge) Verify() error {
	if !k.sealed {
		return errors.New("within-day temporal knowledge: not sealed")
	}
	expected := k.computeDigest()
	if k.Digest != expected {
		return fmt.Errorf("within-day temporal knowledge: digest mismatch (expected %s, got %s)", expected, k.Digest)
	}
	return nil
}

// Verify checks LONGITUDINAL integrity.
func (k *LongitudinalQCKnowledge) Verify() error {
	if !k.sealed {
		return errors.New("longitudinal qc knowledge: not sealed")
	}
	expected := k.computeDigest()
	if k.Digest != expected {
		return fmt.Errorf("longitudinal qc knowledge: digest mismatch (expected %s, got %s)", expected, k.Digest)
	}
	return nil
}

// computeDigest for WITHIN_RUN.
func (k *WithinRunTemporalKnowledge) computeDigest() string {
	canonical := fmt.Sprintf(
		"%s|%s|%d|%s|%s",
		k.RunID,
		k.ObservationSet.Digest,
		k.StatisticalSummary.ObservationCount,
		k.Provenance,
		k.Version,
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// computeDigest for WITHIN_DAY.
func (k *WithinDayKnowledge) computeDigest() string {
	canonical := fmt.Sprintf(
		"%s|%s|%s|%d|%s|%s",
		k.RunID,
		k.DayID,
		k.ObservationSet.Digest,
		k.RunCountThisDay,
		k.Provenance,
		k.Version,
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// computeDigest for LONGITUDINAL.
func (k *LongitudinalQCKnowledge) computeDigest() string {
	canonical := fmt.Sprintf(
		"%s|%s|%d|%d|%s|%s",
		k.KnowledgeEpochID,
		k.ObservationSet.Digest,
		k.DayCountInEpoch,
		k.RunCountInEpoch,
		k.Provenance,
		k.Version,
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

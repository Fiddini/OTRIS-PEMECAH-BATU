package qc

import (
	"errors"
	"fmt"
	"strings"
)

// QCApplicability defines whether QC is required, optional, or not applicable.
// Always explicit, never inferred. NOT_APPLICABLE requires justification.
type QCApplicability string

const (
	ApplicabilityRequired      QCApplicability = "REQUIRED"       // QC results mandatory
	ApplicabilityOptional      QCApplicability = "OPTIONAL"       // QC helpful but not mandatory
	ApplicabilityNotApplicable QCApplicability = "NOT_APPLICABLE" // QC not suitable for this mapping
)

// AcceptanceCriteria defines policy for QC evaluation.
// All thresholds explicit, never hard-coded. NO PASS without explicit criteria.
type AcceptanceCriteria struct {
	// Policy Identity
	Applicability QCApplicability `json:"applicability"` // REQUIRED, OPTIONAL, NOT_APPLICABLE

	// Thresholds (all optional, but if absent → fail-closed)
	MaxCV           *float64 `json:"max_cv_percent,omitempty"`      // max coefficient of variation %
	MaxAbsDeviation *float64 `json:"max_abs_deviation,omitempty"`   // max absolute deviation from reference
	MaxRelDeviation *float64 `json:"max_rel_deviation_percent,omitempty"` // max relative deviation %
	MinObservations *int     `json:"min_observations,omitempty"`    // minimum required samples

	// Required Reference Levels (if specified)
	RequiredLevels []string `json:"required_levels,omitempty"` // must have measurements at these levels

	// Justification (mandatory for NOT_APPLICABLE)
	Justification string `json:"justification"` // why these criteria (or why NOT_APPLICABLE)
	Authority     string `json:"authority"`     // who set the criteria
}

// Validate enforces policy requirements.
func (ac *AcceptanceCriteria) Validate() error {
	switch ac.Applicability {
	case ApplicabilityRequired:
		// REQUIRED: at least one criterion must be specified
		if ac.MaxCV == nil && ac.MaxAbsDeviation == nil && ac.MaxRelDeviation == nil &&
			ac.MinObservations == nil && len(ac.RequiredLevels) == 0 {
			return errors.New("acceptance criteria: REQUIRED applicability but no thresholds specified")
		}

	case ApplicabilityOptional:
		// OPTIONAL: zero or more criteria; absence means no PASS/FAIL decision
		// Always valid

	case ApplicabilityNotApplicable:
		// NOT_APPLICABLE: must have explicit justification (never default/inferred)
		if strings.TrimSpace(ac.Justification) == "" {
			return errors.New("acceptance criteria: NOT_APPLICABLE requires explicit justification")
		}

	default:
		return fmt.Errorf("acceptance criteria: unknown applicability %q", ac.Applicability)
	}

	// Validate threshold values (if present, must be sensible)
	if ac.MaxCV != nil && *ac.MaxCV < 0 {
		return fmt.Errorf("acceptance criteria: max_cv_percent must be >= 0, got %.2f", *ac.MaxCV)
	}
	if ac.MaxAbsDeviation != nil && *ac.MaxAbsDeviation < 0 {
		return fmt.Errorf("acceptance criteria: max_abs_deviation must be >= 0, got %.2f", *ac.MaxAbsDeviation)
	}
	if ac.MaxRelDeviation != nil && *ac.MaxRelDeviation < 0 {
		return fmt.Errorf("acceptance criteria: max_rel_deviation must be >= 0, got %.2f", *ac.MaxRelDeviation)
	}
	if ac.MinObservations != nil && *ac.MinObservations < 0 {
		return fmt.Errorf("acceptance criteria: min_observations must be >= 0, got %d", *ac.MinObservations)
	}

	return nil
}

// QCVerdict is the outcome of QC evaluation.
type QCVerdict string

const (
	VerdictPass                 QCVerdict = "PASS"
	VerdictSuspect              QCVerdict = "SUSPECT"
	VerdictFail                 QCVerdict = "FAIL"
	VerdictInsufficientEvidence QCVerdict = "INSUFFICIENT_EVIDENCE"
	VerdictNotApplicable        QCVerdict = "NOT_APPLICABLE"
)

// Evaluate applies criteria to statistics and returns verdict.
// Fail-closed: missing criteria → INSUFFICIENT_EVIDENCE or FAIL.
func Evaluate(criteria *AcceptanceCriteria, stats *QCStatistics) (QCVerdict, string) {
	if criteria == nil {
		return VerdictFail, "no acceptance criteria provided"
	}

	switch criteria.Applicability {
	case ApplicabilityNotApplicable:
		// QC not applicable; preserve justification
		return VerdictNotApplicable, criteria.Justification

	case ApplicabilityOptional:
		// Optional: evaluate if criteria provided, otherwise INSUFFICIENT_EVIDENCE
		if criteria.MaxCV == nil && criteria.MaxAbsDeviation == nil &&
			criteria.MaxRelDeviation == nil && criteria.MinObservations == nil &&
			len(criteria.RequiredLevels) == 0 {
			return VerdictInsufficientEvidence, "optional QC with no explicit criteria"
		}
		// Fall through to evaluate

	case ApplicabilityRequired:
		// Required: fail-closed if criteria missing or not met
	}

	// Evaluate against thresholds
	if criteria.MinObservations != nil && stats.N < *criteria.MinObservations {
		return VerdictFail, fmt.Sprintf("insufficient observations: %d < %d required", stats.N, *criteria.MinObservations)
	}

	if criteria.MaxCV != nil && !stats.IsZeroMean() && stats.CV > *criteria.MaxCV {
		return VerdictSuspect, fmt.Sprintf("CV exceeds threshold: %.2f%% > %.2f%%", stats.CV, *criteria.MaxCV)
	}

	if criteria.MaxAbsDeviation != nil && stats.AbsDeviation > *criteria.MaxAbsDeviation {
		return VerdictSuspect, fmt.Sprintf("absolute deviation exceeds threshold: %.3f > %.3f", stats.AbsDeviation, *criteria.MaxAbsDeviation)
	}

	if criteria.MaxRelDeviation != nil && stats.RelDeviation > *criteria.MaxRelDeviation {
		return VerdictSuspect, fmt.Sprintf("relative deviation exceeds threshold: %.2f%% > %.2f%%", stats.RelDeviation, *criteria.MaxRelDeviation)
	}

	// All checks passed
	return VerdictPass, "all QC criteria met"
}

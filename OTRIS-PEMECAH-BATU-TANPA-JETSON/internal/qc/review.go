package qc

import (
	"errors"
	"fmt"
)

// ReviewResult captures the outcome of ReviewQC.
type ReviewResult struct {
	Record *QCKnowledgeRecord // QC result (always non-nil, even on error)
	Err    error              // nil if review succeeded
}

// ReviewQC governs QC measurement and produces immutable knowledge record.
// Follows the same governance pattern as ReviewEvidence and ReviewReplay:
// 1. Validate precondition
// 2. Measure/validate QC data
// 3. Create immutable record
// 4. Return verdict without modifying input definitions
func ReviewQC(
	mappingID, mappingVersion, deviceRef, provenance, reviewer, version string,
	challenges []*ReferenceChallenge,
	refDef *ReferenceDefinition,
	criteria *AcceptanceCriteria,
) (*QCKnowledgeRecord, error) {
	// Validate inputs
	if mappingID == "" || mappingVersion == "" || deviceRef == "" {
		return nil, errors.New("ReviewQC: mapping identity and device reference required")
	}
	if len(challenges) == 0 {
		return nil, errors.New("ReviewQC: at least one reference challenge required")
	}
	if refDef == nil {
		return nil, errors.New("ReviewQC: reference definition required")
	}
	if criteria == nil {
		return nil, errors.New("ReviewQC: acceptance criteria required")
	}

	// Validate all challenges bind to reference definition
	for i, ch := range challenges {
		if err := ch.BindsTo(refDef); err != nil {
			return nil, fmt.Errorf("ReviewQC: challenge %d binding failed: %w", i, err)
		}
	}

	// Validate reference definition matches mapping
	if refDef.MappingIdentity != mappingID || refDef.MappingVersion != mappingVersion {
		return nil, fmt.Errorf(
			"ReviewQC: reference definition mismatch (mapping %s/%s vs definition %s/%s)",
			mappingID, mappingVersion, refDef.MappingIdentity, refDef.MappingVersion,
		)
	}

	// Group challenges by reference level and compute statistics
	levelGroups := make(map[string][]float64)
	allStats := []QCStatistics{}

	for _, ch := range challenges {
		// Extract numeric value from raw observation (best-effort)
		// For now: treat as string; real implementation would parse
		levelGroups[ch.RefLevel] = append(levelGroups[ch.RefLevel], 0.0) // placeholder
	}

	// Calculate statistics per level (isolated populations)
	for level, values := range levelGroups {
		if len(values) > 0 {
			stats, err := Calculate("WITHIN_RUN", level, values, nil)
			if err != nil {
				return nil, fmt.Errorf("ReviewQC: statistics calculation failed for level %s: %w", level, err)
			}
			allStats = append(allStats, stats)
		}
	}

	// Evaluate against criteria
	var verdict QCVerdict
	var verdictNote string

	if len(allStats) > 0 {
		// Use first stat for verdict (in production: aggregate or per-level)
		verdict, verdictNote = Evaluate(criteria, &allStats[0])
	} else {
		verdict = VerdictInsufficientEvidence
		verdictNote = "no valid statistics computed"
	}

	// Create immutable knowledge record
	record, err := NewQCRecord(
		mappingID, mappingVersion, deviceRef,
		provenance, reviewer, version,
		allStats,
		*criteria,
		verdict,
		verdictNote,
		len(challenges),
	)
	if err != nil {
		return nil, fmt.Errorf("ReviewQC: failed to create QC record: %w", err)
	}

	// Verify integrity before returning
	if err := record.Verify(); err != nil {
		return nil, fmt.Errorf("ReviewQC: integrity check failed: %w", err)
	}

	return record, nil
}

package temporal

import (
	"math"
	"testing"
	"time"
)

// Test 1: TimeOrderedObservationSet creation with valid data
func TestTimeOrderedObservationSetCreation(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest:   "abc123",
			Timestamp:         time.Now().UTC(),
			Sequence:          0,
			RawValue:          "42.5",
			IsMissing:         false,
			MappingID:         "map1",
			ReferenceLevel:    "NORMAL",
			DeviceFingerprint: "dev1",
		},
	}

	set, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set == nil {
		t.Fatal("set should not be nil")
	}
	if set.Digest == "" {
		t.Fatal("digest should not be empty")
	}
	if set.ObservationCount != 1 {
		t.Fatalf("expected 1 observation, got %d", set.ObservationCount)
	}
}

// Test 2: TimeOrderedObservationSet requires set_id
func TestTimeOrderedObservationSetMissingSetID(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "42.5",
		},
	}

	_, err := New(
		"", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error for missing set_id")
	}
}

// Test 3: Scope validation (rejects invalid scope)
func TestTimeOrderedObservationSetInvalidScope(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "42.5",
		},
	}

	_, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"INVALID_SCOPE", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}

// Test 4: WITHIN_DAY requires day_id
func TestTimeOrderedObservationSetWithinDayNeedsDayID(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "42.5",
		},
	}

	_, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_DAY", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: WITHIN_DAY requires day_id")
	}
}

// Test 5: LONGITUDINAL requires day_id and epoch_id
func TestTimeOrderedObservationSetLongitudinalNeedsEpochID(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "42.5",
		},
	}

	_, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"LONGITUDINAL", "run1", "day1", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: LONGITUDINAL requires knowledge_epoch_id")
	}
}

// Test 6: Requires at least one observation
func TestTimeOrderedObservationSetEmptyObservations(t *testing.T) {
	_, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		[]ObservationReference{},
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: at least one observation required")
	}
}

// Test 7: Rejects negative sequence numbers
func TestTimeOrderedObservationSetNegativeSequence(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        -1,
			RawValue:        "42.5",
		},
	}

	_, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: negative sequence")
	}
}

// Test 8: Rejects duplicate sequence numbers
func TestTimeOrderedObservationSetDuplicateSequence(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "42.5",
		},
		{
			ChallengeDigest: "def456",
			Timestamp:       time.Now().UTC().Add(1 * time.Second),
			Sequence:        0,
			RawValue:        "43.5",
		},
	}

	_, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: duplicate sequence")
	}
}

// Test 9: Enforces time ordering (rejects out-of-order timestamps)
func TestTimeOrderedObservationSetOutOfOrderTimestamps(t *testing.T) {
	later := time.Now().UTC()
	earlier := later.Add(-1 * time.Second)

	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       later,
			Sequence:        0,
			RawValue:        "42.5",
		},
		{
			ChallengeDigest: "def456",
			Timestamp:       earlier,
			Sequence:        1,
			RawValue:        "43.5",
		},
	}

	_, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: out-of-order timestamps")
	}
}

// Test 10: Digest verification catches tampering
func TestTimeOrderedObservationSetDigestVerification(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "42.5",
		},
	}

	set, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tamper with digest
	originalDigest := set.Digest
	set.Digest = "tampered_digest_value"

	err = set.Verify()
	if err == nil {
		t.Fatal("expected error: digest verification should catch tampering")
	}

	// Restore and verify OK
	set.Digest = originalDigest
	err = set.Verify()
	if err != nil {
		t.Fatalf("digest verification failed after restore: %v", err)
	}
}

// Test 11: Preserves UNKNOWN observations (is_missing=true, no interpretation)
func TestTimeOrderedObservationSetPreservesUnknown(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "UNKNOWN",
			IsMissing:       true,
		},
	}

	set, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !set.Observations[0].IsMissing {
		t.Fatal("UNKNOWN observation should be marked as missing")
	}
	if set.Observations[0].RawValue != "UNKNOWN" {
		t.Fatal("raw value should preserve UNKNOWN exactly")
	}
}

// Test 12: WithinRunTemporalKnowledge creation
func TestWithinRunTemporalKnowledgeCreation(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &RunStatisticalSummary{
		ObservationCount:       1,
		MissingCount:           0,
		MeanValue:              42.5,
		StdDev:                 0.0,
		CoefficientOfVariation: math.NaN(),
		MinValue:               42.5,
		MaxValue:               42.5,
		MedianValue:            42.5,
	}

	k, err := NewWithinRunTemporalKnowledge(
		"run1", set, summary,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k == nil {
		t.Fatal("knowledge should not be nil")
	}
	if k.Digest == "" {
		t.Fatal("digest should not be empty")
	}
}

// Test 13: WithinRunTemporalKnowledge requires correct scope
func TestWithinRunTemporalKnowledgeWrongScope(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_DAY", "run1", "day1", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &RunStatisticalSummary{
		ObservationCount: 1,
	}

	_, err := NewWithinRunTemporalKnowledge(
		"run1", set, summary,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: scope mismatch")
	}
}

// Test 14: WithinDayKnowledge creation
func TestWithinDayTemporalKnowledgeCreation(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_DAY", "run1", "day1", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &DayStatisticalSummary{
		ObservationCount: 1,
	}

	k, err := NewWithinDayKnowledge(
		"run1", "day1", set, summary, 1,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k == nil {
		t.Fatal("knowledge should not be nil")
	}
}

// Test 15: WithinDayKnowledge requires run_count >= 1
func TestWithinDayTemporalKnowledgeInvalidRunCount(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_DAY", "run1", "day1", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &DayStatisticalSummary{
		ObservationCount: 1,
	}

	_, err := NewWithinDayKnowledge(
		"run1", "day1", set, summary, 0,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: run_count must be >= 1")
	}
}

// Test 16: LongitudinalQCKnowledge creation
func TestLongitudinalQCKnowledgeCreation(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"LONGITUDINAL", "run1", "day1", "epoch1",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &LongitudinalStatisticalSummary{
		ObservationCount: 1,
	}

	k, err := NewLongitudinalQCKnowledge(
		"epoch1", set, summary, 1, 1,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k == nil {
		t.Fatal("knowledge should not be nil")
	}
}

// Test 17: LongitudinalQCKnowledge requires day_count >= 1
func TestLongitudinalQCKnowledgeInvalidDayCount(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"LONGITUDINAL", "run1", "day1", "epoch1",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &LongitudinalStatisticalSummary{
		ObservationCount: 1,
	}

	_, err := NewLongitudinalQCKnowledge(
		"epoch1", set, summary, 0, 1,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: day_count must be >= 1")
	}
}

// Test 18: PatternCandidate creation (diagnostic only)
func TestPatternCandidateCreation(t *testing.T) {
	pc, err := NewPatternCandidate(
		"pc1",
		TrendCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pc == nil {
		t.Fatal("pattern candidate should not be nil")
	}
	if !pc.CannotTrigger.CannotModifyMappingDefinition {
		t.Fatal("must mark cannot_modify_mapping_definition=true")
	}
	if !pc.CannotTrigger.CannotModifyRegistry {
		t.Fatal("must mark cannot_modify_registry=true")
	}
	if !pc.CannotTrigger.CannotModifySemanticAuthority {
		t.Fatal("must mark cannot_modify_semantic_authority=true")
	}
}

// Test 19: PatternCandidate requires valid confidence range
func TestPatternCandidateInvalidConfidence(t *testing.T) {
	_, err := NewPatternCandidate(
		"pc1",
		TrendCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		1.5, 0.75, // confidence > 1.0
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: confidence_score > 1.0")
	}
}

// Test 20: PatternCandidate.SetTrendData enforces type match
func TestPatternCandidateSetTrendDataWrongType(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		ShiftCandidate, // wrong type
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"step_detection",
		"temporal analysis", "operator1", "v1",
	)

	err := pc.SetTrendData(0.5, 10.0, 0.95)
	if err == nil {
		t.Fatal("expected error: SetTrendData on non-TREND_CANDIDATE")
	}
}

// Test 21: PatternCandidate.SetShiftData enforces type match
func TestPatternCandidateSetShiftDataWrongType(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		TrendCandidate, // wrong type
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	err := pc.SetShiftData(5.0, 10)
	if err == nil {
		t.Fatal("expected error: SetShiftData on non-SHIFT_CANDIDATE")
	}
}

// Test 22: PatternCandidate.SetRecurrenceData enforces period >= 1
func TestPatternCandidateSetRecurrenceDataInvalidPeriod(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		RecurrenceCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"autocorrelation",
		"temporal analysis", "operator1", "v1",
	)

	err := pc.SetRecurrenceData(0, 0.1) // period < 1
	if err == nil {
		t.Fatal("expected error: period < 1")
	}
}

// Test 23: PatternCandidate.CanAuthorityModify always returns false
func TestPatternCandidateCannotTriggerAuthority(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		TrendCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	if pc.CanAuthorityModify() {
		t.Fatal("PatternCandidate.CanAuthorityModify() must always return false")
	}
}

// Test 24: PatternCandidate digest verification
func TestPatternCandidateDigestVerification(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		TrendCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	originalDigest := pc.Digest
	pc.Digest = "tampered"

	err := pc.Verify()
	if err == nil {
		t.Fatal("expected error: digest verification should fail on tampering")
	}

	pc.Digest = originalDigest
	err = pc.Verify()
	if err != nil {
		t.Fatalf("digest verification failed: %v", err)
	}
}

// Test 25: CV correctly reports NaN for zero-mean data (not fabricated)
func TestCoefficientOfVariationZeroMean(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "0.0",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &RunStatisticalSummary{
		ObservationCount:       1,
		MissingCount:           0,
		MeanValue:              0.0,
		StdDev:                 0.0,
		CoefficientOfVariation: math.NaN(), // explicitly undefined, not fabricated
		MinValue:               0.0,
		MaxValue:               0.0,
		MedianValue:            0.0,
	}

	k, _ := NewWithinRunTemporalKnowledge(
		"run1", set, summary,
		"ReviewQC", "operator1", "v1",
	)

	if !math.IsNaN(k.StatisticalSummary.CoefficientOfVariation) {
		t.Fatal("CV for zero-mean data should be NaN, not fabricated")
	}
}

// Test 26: Multiple observations preserve order
func TestTimeOrderedObservationSetMultipleInOrder(t *testing.T) {
	now := time.Now().UTC()
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc",
			Timestamp:       now,
			Sequence:        0,
			RawValue:        "40.0",
		},
		{
			ChallengeDigest: "def",
			Timestamp:       now.Add(1 * time.Second),
			Sequence:        1,
			RawValue:        "41.0",
		},
		{
			ChallengeDigest: "ghi",
			Timestamp:       now.Add(2 * time.Second),
			Sequence:        2,
			RawValue:        "42.0",
		},
	}

	set, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.ObservationCount != 3 {
		t.Fatalf("expected 3 observations, got %d", set.ObservationCount)
	}
	if set.Observations[0].Sequence != 0 || set.Observations[1].Sequence != 1 || set.Observations[2].Sequence != 2 {
		t.Fatal("observation order not preserved")
	}
}

// Test 27: Outlier observations preserved (not deleted)
func TestTimeOrderedObservationSetPreservesOutliers(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc123",
			Timestamp:       time.Now().UTC(),
			Sequence:        0,
			RawValue:        "1000.0", // outlier
			IsOutlier:       true,
		},
	}

	set, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !set.Observations[0].IsOutlier {
		t.Fatal("outlier flag not preserved")
	}
	if set.Observations[0].RawValue != "1000.0" {
		t.Fatal("outlier value not preserved")
	}
}

// Test 28: NaN handling in CV (preserved, not modified)
func TestCoefficientOfVariationNaN(t *testing.T) {
	summary := &RunStatisticalSummary{
		CoefficientOfVariation: math.NaN(),
	}

	// Verify NaN is preserved (special comparison needed)
	if !math.IsNaN(summary.CoefficientOfVariation) {
		t.Fatal("NaN not preserved in summary")
	}
}

// Test 29: WithinRunTemporalKnowledge digest includes observation set digest
func TestWithinRunTemporalKnowledgeDigestInclusion(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &RunStatisticalSummary{
		ObservationCount: 1,
	}

	k, _ := NewWithinRunTemporalKnowledge(
		"run1", set, summary,
		"ReviewQC", "operator1", "v1",
	)

	// Digest should include the observation set's digest
	if k.Digest == "" {
		t.Fatal("digest should not be empty")
	}
	if k.Digest == set.Digest {
		t.Fatal("temporal knowledge digest should differ from observation set digest")
	}
}

// Test 30: PatternCandidate auto-flags NeedsHumanReview when confident
func TestPatternCandidateAutoFlagReview(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		TrendCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75, // confidence (0.85) >= threshold (0.75)
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	if !pc.NeedsHumanReview {
		t.Fatal("should auto-flag NeedsHumanReview when confidence >= threshold")
	}
}

// Test 31: PatternCandidate does not flag review when not confident
func TestPatternCandidateNoAutoFlagReview(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		TrendCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		0.60, 0.75, // confidence (0.60) < threshold (0.75)
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	if pc.NeedsHumanReview {
		t.Fatal("should not auto-flag NeedsHumanReview when confidence < threshold")
	}
}

// Test 32: WithinDayKnowledge Verify method
func TestWithinDayTemporalKnowledgeVerify(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_DAY", "run1", "day1", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &DayStatisticalSummary{
		ObservationCount: 1,
	}

	k, _ := NewWithinDayKnowledge(
		"run1", "day1", set, summary, 1,
		"ReviewQC", "operator1", "v1",
	)

	// Verify should pass
	err := k.Verify()
	if err != nil {
		t.Fatalf("verify should pass: %v", err)
	}

	// Tamper and verify should fail
	k.Digest = "tampered"
	err = k.Verify()
	if err == nil {
		t.Fatal("verify should fail on tampering")
	}
}

// Test 33: LongitudinalQCKnowledge Verify method
func TestLongitudinalQCKnowledgeVerify(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"LONGITUDINAL", "run1", "day1", "epoch1",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &LongitudinalStatisticalSummary{
		ObservationCount: 1,
	}

	k, _ := NewLongitudinalQCKnowledge(
		"epoch1", set, summary, 1, 1,
		"ReviewQC", "operator1", "v1",
	)

	err := k.Verify()
	if err != nil {
		t.Fatalf("verify should pass: %v", err)
	}
}

// Test 34: WithinRunTemporalKnowledge Verify method
func TestWithinRunTemporalKnowledgeVerify(t *testing.T) {
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	summary := &RunStatisticalSummary{
		ObservationCount: 1,
	}

	k, _ := NewWithinRunTemporalKnowledge(
		"run1", set, summary,
		"ReviewQC", "operator1", "v1",
	)

	err := k.Verify()
	if err != nil {
		t.Fatalf("verify should pass: %v", err)
	}
}

// Test 35: PatternCandidate Verify method
func TestPatternCandidateVerify(t *testing.T) {
	pc, _ := NewPatternCandidate(
		"pc1",
		TrendCandidate,
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	err := pc.Verify()
	if err != nil {
		t.Fatalf("verify should pass: %v", err)
	}
}

// Test 36: Temporal identity boundaries (WITHIN_RUN, WITHIN_DAY, LONGITUDINAL) not mixed
func TestTemporalScopesNotMixed(t *testing.T) {
	// Create WITHIN_RUN observation set
	obsRef := ObservationReference{
		ChallengeDigest: "abc123",
		Timestamp:       time.Now().UTC(),
		Sequence:        0,
		RawValue:        "42.5",
	}

	withinRunSet, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		[]ObservationReference{obsRef},
		"ReviewQC", "operator1", "v1",
	)

	// Trying to use WITHIN_RUN set in WITHIN_DAY knowledge should fail
	daySum := &DayStatisticalSummary{ObservationCount: 1}
	_, err := NewWithinDayKnowledge(
		"run1", "day1", withinRunSet, daySum, 1,
		"ReviewQC", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: scope mismatch between set and knowledge")
	}
}

// Test 37: Observation count preserved
func TestObservationCountPreserved(t *testing.T) {
	obs := []ObservationReference{
		{ChallengeDigest: "a", Timestamp: time.Now().UTC(), Sequence: 0, RawValue: "1"},
		{ChallengeDigest: "b", Timestamp: time.Now().UTC().Add(1 * time.Second), Sequence: 1, RawValue: "2"},
		{ChallengeDigest: "c", Timestamp: time.Now().UTC().Add(2 * time.Second), Sequence: 2, RawValue: "3"},
		{ChallengeDigest: "d", Timestamp: time.Now().UTC().Add(3 * time.Second), Sequence: 3, RawValue: "4"},
		{ChallengeDigest: "e", Timestamp: time.Now().UTC().Add(4 * time.Second), Sequence: 4, RawValue: "5"},
	}

	set, err := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.ObservationCount != 5 {
		t.Fatalf("expected 5 observations, got %d", set.ObservationCount)
	}
}

// Test 38: Device fingerprint immutability (bound at observation)
func TestObservationDeviceFingerprintBinding(t *testing.T) {
	obs := []ObservationReference{
		{
			ChallengeDigest:   "abc123",
			Timestamp:         time.Now().UTC(),
			Sequence:          0,
			RawValue:          "42.5",
			DeviceFingerprint: "dev-original",
		},
	}

	set, _ := New(
		"set1", "dev-original", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	// Observation preserves its device fingerprint
	if set.Observations[0].DeviceFingerprint != "dev-original" {
		t.Fatal("device fingerprint not preserved in observation")
	}
}

// Test 39: Missing observations indicated (IsMissing flag, not deleted)
func TestMissingObservationPreserved(t *testing.T) {
	now := time.Now().UTC()
	obs := []ObservationReference{
		{
			ChallengeDigest: "abc",
			Timestamp:       now,
			Sequence:        0,
			RawValue:        "40.0",
			IsMissing:       false,
		},
		{
			ChallengeDigest: "missing_digest",
			Timestamp:       now.Add(1 * time.Second),
			Sequence:        1,
			RawValue:        "UNKNOWN",
			IsMissing:       true,
		},
		{
			ChallengeDigest: "def",
			Timestamp:       now.Add(2 * time.Second),
			Sequence:        2,
			RawValue:        "42.0",
			IsMissing:       false,
		},
	}

	set, _ := New(
		"set1", "dev1", "map1", "ref1", "NORMAL",
		"WITHIN_RUN", "run1", "", "",
		obs,
		"ReviewQC", "operator1", "v1",
	)

	// Missing observation preserved, not deleted
	if set.ObservationCount != 3 {
		t.Fatalf("expected 3 observations including missing, got %d", set.ObservationCount)
	}
	if !set.Observations[1].IsMissing {
		t.Fatal("missing observation flag not preserved")
	}
}

// Test 40: PatternCandidate invalid type rejected
func TestPatternCandidateInvalidType(t *testing.T) {
	_, err := NewPatternCandidate(
		"pc1",
		PatternCandidateType("INVALID_TYPE"),
		"knowledge1",
		"ref1", "dev1", "map1",
		0.85, 0.75,
		"linear_regression",
		"temporal analysis", "operator1", "v1",
	)

	if err == nil {
		t.Fatal("expected error: invalid pattern type")
	}
}

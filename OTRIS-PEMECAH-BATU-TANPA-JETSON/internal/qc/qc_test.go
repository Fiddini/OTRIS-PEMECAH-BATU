package qc

import (
	"math"
	"testing"
	"time"
)

// TEST 1: ReferenceDefinition validation — no inferred levels
func TestReferenceDefinitionRequiresExplicitLevels(t *testing.T) {
	rd := &ReferenceDefinition{
		MappingIdentity: "test_mapping",
		MappingVersion:  "1.0.0",
		Levels:          []ReferenceLevel{}, // empty — should fail
		Provenance:      "manual definition",
		Author:          "test_author",
		Version:         "1.0.0",
	}
	if err := rd.Validate(); err == nil {
		t.Error("ReferenceDefinition validation should reject empty levels (no inference)")
	}
}

// TEST 2: ReferenceChallenge binding — fingerprint mismatch rejected
func TestReferenceChallengeBindingFingerprintMismatch(t *testing.T) {
	rd := &ReferenceDefinition{
		MappingIdentity: "test_mapping",
		MappingVersion:  "1.0.0",
		Levels: []ReferenceLevel{
			{Level: "NORMAL", EvidenceRef: "manual:p45", Author: "test"},
		},
		Provenance: "manual",
		Author:     "test_author",
		Version:    "1.0.0",
	}

	ch, _ := NewChallenge(
		"test_mapping", "1.0.0", "NORMAL",
		"fingerprint_A", // Device A
		"72.5",
		"method: reading", "operator", time.Now(),
	)

	rd.DeviceRef = "fingerprint_B" // Different device
	if err := ch.BindsTo(rd); err == nil {
		t.Error("BindsTo should reject fingerprint mismatch (fail-closed)")
	}
}

// TEST 3: QCStatistics isolation — LOW/NORMAL/HIGH not mixed
func TestQCStatisticsIsolation(t *testing.T) {
	stats := []QCStatistics{
		{Scope: "WITHIN_RUN", Level: "LOW", N: 10, Mean: 50, StdDev: 2.5},
		{Scope: "WITHIN_RUN", Level: "NORMAL", N: 10, Mean: 75, StdDev: 3.0},
		{Scope: "WITHIN_RUN", Level: "HIGH", N: 10, Mean: 95, StdDev: 2.0},
	}

	if err := ValidateIsolation(stats); err != nil {
		t.Errorf("ValidateIsolation failed for proper level isolation: %v", err)
	}

	// Now test scope mixing (should fail)
	statsMixed := []QCStatistics{
		{Scope: "WITHIN_RUN", Level: "NORMAL"},
		{Scope: "WITHIN_DAY", Level: "NORMAL"}, // Different scope
	}
	if err := ValidateIsolation(statsMixed); err == nil {
		t.Error("ValidateIsolation should reject scope mixing (fail-closed)")
	}
}

// TEST 4: CV calculation — zero-mean CV is undefined, not fabricated
func TestCVZeroMeanUndefined(t *testing.T) {
	observations := []float64{-50, -25, 0, 25, 50} // mean = 0

	stats, _ := Calculate("WITHIN_RUN", "NORMAL", observations, nil)

	if !math.IsNaN(stats.CV) {
		t.Errorf("CV should be NaN for zero-mean data, got %.2f", stats.CV)
	}
	if stats.IsZeroMean() == false {
		t.Error("IsZeroMean should return true")
	}
}

// TEST 5: TransferFunction — fingerprint change requires re-verification
func TestTransferFunctionFingerprintReverification(t *testing.T) {
	tf, _ := NewTransferFunction(
		"scalar_factor", "scalar_multiply", "method:empirical",
		"provenance", "author", "1.0.0",
		"fingerprint_A", "mapping_001", "1.0.0",
		map[string]string{"factor": "2.5"},
	)

	// Try to reuse with different fingerprint
	if err := tf.CanReuseWith("fingerprint_B", "mapping_001", "1.0.0"); err == nil {
		t.Error("CanReuseWith should reject fingerprint change (requires re-verification)")
	}
}

// TEST 6: AcceptanceCriteria — NOT_APPLICABLE requires explicit justification
func TestAcceptanceCriteriaNOTAPPLICABLEJustification(t *testing.T) {
	ac := &AcceptanceCriteria{
		Applicability: ApplicabilityNotApplicable,
		Justification: "", // Empty justification
		Authority:     "test",
	}

	if err := ac.Validate(); err == nil {
		t.Error("Validate should require justification for NOT_APPLICABLE (fail-closed)")
	}

	// With justification should pass
	ac.Justification = "QC not applicable for categorical observations"
	if err := ac.Validate(); err != nil {
		t.Errorf("Validate failed with proper justification: %v", err)
	}
}

// TEST 7: AcceptanceCriteria — REQUIRED without thresholds fails
func TestAcceptanceCriteriaREQUIREDNeedsThresholds(t *testing.T) {
	ac := &AcceptanceCriteria{
		Applicability: ApplicabilityRequired,
		Justification: "Quality critical",
		Authority:     "test",
		// No thresholds specified
	}

	if err := ac.Validate(); err == nil {
		t.Error("Validate should require thresholds for REQUIRED applicability (fail-closed)")
	}
}

// TEST 8: QCVerdict — missing policy results in INSUFFICIENT_EVIDENCE
func TestQCVerdictMissingPolicy(t *testing.T) {
	stats := &QCStatistics{
		Level: "NORMAL", N: 10, Mean: 75, StdDev: 3.0, CV: 4.0,
	}

	ac := &AcceptanceCriteria{
		Applicability: ApplicabilityOptional,
		// No criteria specified
	}

	verdict, note := Evaluate(ac, stats)
	if verdict != VerdictInsufficientEvidence {
		t.Errorf("Expected INSUFFICIENT_EVIDENCE without criteria, got %s: %s", verdict, note)
	}
}

// TEST 9: QCKnowledgeRecord — immutability verification
func TestQCKnowledgeRecordImmutability(t *testing.T) {
	record, _ := NewQCRecord(
		"test_mapping", "1.0.0", "device_A",
		"provenance", "reviewer", "1.0.0",
		[]QCStatistics{},
		AcceptanceCriteria{
			Applicability: ApplicabilityNotApplicable,
			Justification: "test",
		},
		VerdictPass,
		"test",
		5,
	)

	// Verify integrity
	if err := record.Verify(); err != nil {
		t.Errorf("Integrity check failed: %v", err)
	}

	// Simulate tampering (change verdict)
	record.Verdict = VerdictFail
	if err := record.Verify(); err == nil {
		t.Error("Verify should detect tampering (integrity mismatch)")
	}
}

// TEST 10: QCKnowledgeRecord — test fixture rejection (fail-closed)
func TestQCKnowledgeRecordTestFixtureRejection(t *testing.T) {
	record, _ := NewQCRecord(
		"test_mapping", "1.0.0", "device_A",
		"provenance", "reviewer", "1.0.0",
		[]QCStatistics{},
		AcceptanceCriteria{
			Applicability: ApplicabilityNotApplicable,
			Justification: "test",
		},
		VerdictPass,
		"test",
		5,
	)

	// Mark as test fixture
	record.IsTestFixture = true
	if record.IsProductionReady() {
		t.Error("IsProductionReady should reject test fixtures (fail-closed)")
	}
}

// TEST 11: UNKNOWN preservation — raw values not inferred
func TestUNKNOWNPreservation(t *testing.T) {
	// Create challenge with empty raw value
	ch, _ := NewChallenge(
		"test_mapping", "1.0.0", "NORMAL",
		"device_A", "", // Empty raw value (UNKNOWN)
		"method: reading", "operator", time.Now(),
	)

	// RawValue should remain empty (no inference)
	if ch.RawValue != "" {
		t.Errorf("RawValue should remain empty (UNKNOWN), got %q", ch.RawValue)
	}

	// Verify should pass (empty is valid UTF-8)
	if err := ch.Verify(); err != nil {
		t.Errorf("Verify should accept empty raw value: %v", err)
	}
}

// TEST 12: QC cannot admit profile — verdict doesn't change MappingDefinition status
func TestQCCannotAdmitProfile(t *testing.T) {
	// QCKnowledgeRecord is separate from MappingDefinition
	// QC verdict (PASS, FAIL, etc.) does NOT transition MappingDefinition status
	// This test documents the architectural boundary:
	// MappingDefinition.Status remains REPLAY_VALIDATED even after QC PASS

	record, _ := NewQCRecord(
		"test_mapping", "1.0.0", "device_A",
		"provenance", "reviewer", "1.0.0",
		[]QCStatistics{},
		AcceptanceCriteria{
			Applicability: ApplicabilityNotApplicable,
			Justification: "test",
		},
		VerdictPass, // QC PASS
		"test",
		0,
	)

	// Even with PASS, QC cannot change MappingDefinition governance
	// (This is enforced at the middleware level, not in QC itself)
	if record.Status != "CANDIDATE" {
		t.Errorf("QC record should remain CANDIDATE status, got %s", record.Status)
	}
}

// TEST 13: -100 value preservation
func TestMinusHundredPreservation(t *testing.T) {
	ch, _ := NewChallenge(
		"test_mapping", "1.0.0", "NORMAL",
		"device_A", "-100", // Sentinel/error value
		"method: reading", "operator", time.Now(),
	)

	if ch.RawValue != "-100" {
		t.Errorf("RawValue should preserve -100 exactly, got %q", ch.RawValue)
	}
}

// TEST 14: QC applicability NOT_APPLICABLE without justification fails
func TestQCApplicabilityNOTAPPLICABLENoJustification(t *testing.T) {
	ac := &AcceptanceCriteria{
		Applicability: ApplicabilityNotApplicable,
		Justification: "", // Missing
		Authority:     "test",
	}

	if err := ac.Validate(); err == nil {
		t.Error("NOT_APPLICABLE without justification should fail validation")
	}
}

// TEST 15: ReferenceChallenge digest immutability
func TestReferenceChallengeDigestImmutability(t *testing.T) {
	original, _ := NewChallenge(
		"test_mapping", "1.0.0", "NORMAL",
		"device_A", "72.5",
		"method: manual", "operator", time.Now(),
	)

	// Store original digest
	originalDigest := original.Digest

	// Clone and verify same digest (deterministic)
	clone, _ := NewChallenge(
		original.MappingIdentity, original.MappingVersion, original.RefLevel,
		original.DeviceFingerprint, original.RawValue,
		original.Provenance, original.Operator, original.Timestamp,
	)

	if clone.Digest != originalDigest {
		t.Error("Same input should produce same digest (deterministic)")
	}

	// Modify timestamp and verify digest changes
	modified := *original
	modified.Timestamp = time.Now().Add(1 * time.Hour)
	if modified.computeDigest() == originalDigest {
		t.Error("Modified challenge should produce different digest")
	}
}

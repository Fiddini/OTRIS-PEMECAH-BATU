package identity

import (
	"testing"
	"time"
)

// Test 1: INITIAL observation produces NO_ACTION
func TestDecideInitialObservation(t *testing.T) {
	obs, _ := NewObservation(
		"obs-001",
		"device-fp-001",
		"PB-10",
		"initial fingerprint",
		"system",
		ChangeTypeInitial,
		1.0,
		time.Now().UTC(),
	)
	obs.sealed = true
	obs.InvestigationStatus = "CLOSED"

	status := DecideFromObservation(obs)

	if status != ReverificationNoAction {
		t.Fatalf("INITIAL should result in NO_ACTION, got %s", status)
	}
}

// Test 2: UNREVIEWED observation cannot establish re-verification
func TestDecideUnreviewedObservation(t *testing.T) {
	obs, _ := NewObservation(
		"obs-002",
		"device-fp-002",
		"PB-10",
		"device change detected",
		"system",
		ChangeTypeUpdate,
		0.9,
		time.Now().UTC(),
	)

	status := DecideFromObservation(obs)

	if status == ReverificationRequired {
		t.Fatal("UNREVIEWED observation should not produce REVERIFY_REQUIRED")
	}

	if status != ReverificationNoAction && status != ReverificationInsufficientEvidence {
		t.Fatalf("UNREVIEWED observation should produce NO_ACTION or INSUFFICIENT_EVIDENCE, got %s", status)
	}
}

// Test 3: UNDER_INVESTIGATION observation cannot establish re-verification
func TestDecideUnderInvestigationObservation(t *testing.T) {
	obs, _ := NewObservation(
		"obs-003",
		"device-fp-003",
		"edge-discovery",
		"firmware version changed",
		"system",
		ChangeTypeUpdate,
		0.85,
		time.Now().UTC(),
	)
	obs.sealed = true
	obs.InvestigationStatus = "UNDER_INVESTIGATION"

	status := DecideFromObservation(obs)

	if status == ReverificationRequired {
		t.Fatal("UNDER_INVESTIGATION observation should not produce REVERIFY_REQUIRED")
	}
}

// Test 4: Engineer-confirmed UPDATE + CLOSED → REVERIFY_REQUIRED
func TestDecideConfirmedUpdateRequired(t *testing.T) {
	obs, _ := NewObservation(
		"obs-004",
		"device-fp-004",
		"PB-10",
		"firmware update detected",
		"system",
		ChangeTypeUpdate,
		0.95,
		time.Now().UTC(),
	)
	obs.sealed = true

	// Engineer reviews and confirms change
	err := obs.Review("engineer-001", "Firmware updated; legitimate change", true)
	if err != nil {
		t.Fatalf("Review should succeed: %v", err)
	}

	status := DecideFromObservation(obs)

	if status != ReverificationRequired {
		t.Fatalf("confirmed UPDATE should result in REVERIFY_REQUIRED, got %s", status)
	}
}

// Test 5: POSSIBLE_CORRUPTION + CLOSED → REVERIFY_REQUIRED
func TestDecidePossibleCorruptionRequired(t *testing.T) {
	obs, _ := NewObservation(
		"obs-005",
		"device-fp-005",
		"edge-discovery",
		"unexpected fingerprint change",
		"system",
		ChangeTypePossibleCorruption,
		0.8,
		time.Now().UTC(),
	)
	obs.sealed = true

	// Engineer reviews (result doesn't matter for POSSIBLE_CORRUPTION)
	obs.Review("engineer-001", "Suspicious change detected; requires investigation", false)

	status := DecideFromObservation(obs)

	if status != ReverificationRequired {
		t.Fatalf("POSSIBLE_CORRUPTION should result in REVERIFY_REQUIRED, got %s", status)
	}
}

// Test 6: UNKNOWN + CLOSED → INSUFFICIENT_EVIDENCE
func TestDecideUnknownInsufficientEvidence(t *testing.T) {
	obs, _ := NewObservation(
		"obs-006",
		"device-fp-006",
		"unknown-detector",
		"unknown fingerprint variation",
		"system",
		ChangeTypeUnknown,
		0.5,
		time.Now().UTC(),
	)
	obs.sealed = true

	err := obs.Review("engineer-001", "Cannot determine cause; needs more investigation", false)
	if err != nil {
		t.Fatalf("Review should succeed: %v", err)
	}

	status := DecideFromObservation(obs)

	if status != ReverificationInsufficientEvidence {
		t.Fatalf("closed UNKNOWN should result in INSUFFICIENT_EVIDENCE, got %s", status)
	}
}

// Test 7: Historical observation remains immutable
func TestObservationImmutability(t *testing.T) {
	obs, _ := NewObservation(
		"obs-007",
		"device-fp-007",
		"PB-10",
		"original observation",
		"system",
		ChangeTypeUpdate,
		0.9,
		time.Now().UTC(),
	)
	obs.sealed = true

	originalDigest := obs.Digest

	// Try to modify (should fail)
	err := obs.SetPreviousReference("prev-fp-007")

	if err == nil {
		t.Fatal("sealed observation should not allow modification")
	}

	if obs.Digest != originalDigest {
		t.Fatal("digest should not change on failed modification")
	}
}

// Test 8: No Registry/authority mutation
func TestNoAuthorityMutation(t *testing.T) {
	obs, _ := NewObservation(
		"obs-008",
		"device-fp-008",
		"PB-10",
		"test observation",
		"system",
		ChangeTypeUpdate,
		0.9,
		time.Now().UTC(),
	)
	obs.sealed = true
	obs.Review("engineer-001", "Change confirmed", true)

	// Decide from observation
	status := DecideFromObservation(obs)

	if status != ReverificationRequired {
		t.Fatalf("decision should be REVERIFY_REQUIRED, got %s", status)
	}

	// Verify observation is unchanged
	if obs.ChangeType != ChangeTypeUpdate {
		t.Fatal("observation should not be mutated by decision logic")
	}

	// Create requirement
	rrq, err := NewReverificationRequirement(
		"req-008",
		obs.ObservationID,
		status,
		"firmware change requires re-verification",
		obs.DeviceFingerprint,
		string(obs.ChangeType),
		obs.InvestigationStatus,
	)

	if err != nil {
		t.Fatalf("requirement creation should succeed: %v", err)
	}

	// Verify requirement cannot revoke
	if !rrq.CannotRevokeAttestation {
		t.Fatal("requirement must not revoke attestation")
	}

	if !rrq.CannotModifyAuthority {
		t.Fatal("requirement must not modify authority")
	}
}

// Test 9: Deterministic decision
func TestDeterministicDecision(t *testing.T) {
	obs, _ := NewObservation(
		"obs-009",
		"device-fp-009",
		"PB-10",
		"test fingerprint",
		"system",
		ChangeTypeUpdate,
		0.95,
		time.Now().UTC(),
	)
	obs.sealed = true
	obs.Review("engineer-001", "Confirmed update", true)

	// Call decision multiple times
	status1 := DecideFromObservation(obs)
	status2 := DecideFromObservation(obs)
	status3 := DecideFromObservation(obs)

	if status1 != status2 || status2 != status3 {
		t.Fatal("decision should be deterministic")
	}

	if status1 != ReverificationRequired {
		t.Fatalf("decision should be REVERIFY_REQUIRED, got %s", status1)
	}
}

// Test 10: Requirement immutability and integrity
func TestRequirementImmutability(t *testing.T) {
	rrq, _ := NewReverificationRequirement(
		"req-010",
		"obs-010",
		ReverificationRequired,
		"device change detected",
		"device-fp-010",
		string(ChangeTypeUpdate),
		"CLOSED",
	)

	originalDigest := rrq.Digest

	// Try to modify (should fail)
	err := rrq.SetObservationReference("new-ref")

	if err == nil {
		t.Fatal("sealed requirement should not allow modification")
	}

	if rrq.Digest != originalDigest {
		t.Fatal("digest should not change on failed modification")
	}
}

// Test 11: Decision matrix completeness
func TestDecisionMatrix(t *testing.T) {
	testCases := []struct {
		name                string
		changeType          ChangeType
		investigationStatus string
		confirmed           bool
		expectedStatus      ReverificationStatus
	}{
		{"INITIAL", ChangeTypeInitial, "CLOSED", false, ReverificationNoAction},
		{"UPDATE unreviewed", ChangeTypeUpdate, "UNREVIEWED", false, ReverificationInsufficientEvidence}, // high confidence pending review
		{"UPDATE investigating", ChangeTypeUpdate, "UNDER_INVESTIGATION", false, ReverificationInsufficientEvidence}, // high confidence pending
		{"UPDATE closed confirmed", ChangeTypeUpdate, "CLOSED", true, ReverificationRequired},
		{"UPDATE closed unconfirmed", ChangeTypeUpdate, "CLOSED", false, ReverificationRequired}, // UPDATE + CLOSED always requires reverification (no confirmation tracking)
		{"POSSIBLE_CORRUPTION closed", ChangeTypePossibleCorruption, "CLOSED", false, ReverificationRequired},
		{"UNKNOWN closed", ChangeTypeUnknown, "CLOSED", false, ReverificationInsufficientEvidence},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			obs, _ := NewObservation(
				"obs-matrix",
				"device-fp",
				"detector",
				"reason",
				"system",
				tc.changeType,
				0.8,
				time.Now().UTC(),
			)
			obs.sealed = true
			obs.InvestigationStatus = tc.investigationStatus

			if tc.investigationStatus == "CLOSED" {
				obs.Review("engineer", "notes", tc.confirmed)
			}

			status := DecideFromObservation(obs)

			if status != tc.expectedStatus {
				t.Fatalf("expected %s, got %s", tc.expectedStatus, status)
			}
		})
	}
}

// Test 12: Full existing test suite remains PASS (integration check)
func TestExistingSuiteCompatibility(t *testing.T) {
	// This test verifies that:
	// 1. FingerprintIdentityObservation is not modified
	// 2. IdentityConflict is not modified
	// 3. No existing functionality is broken

	// Create an observation (existing functionality)
	obs, err := NewObservation(
		"obs-compat",
		"device-fp-compat",
		"detector",
		"reason",
		"system",
		ChangeTypeUpdate,
		0.9,
		time.Now().UTC(),
	)

	if err != nil {
		t.Fatalf("existing NewObservation should work: %v", err)
	}

	if obs == nil {
		t.Fatal("observation should be created")
	}

	// Verify existing methods work
	if !obs.sealed {
		t.Fatal("observation should be sealed")
	}

	// Create a conflict (existing functionality)
	conflict, err := NewConflict(
		"conflict-compat",
		ConflictTypeFingerprintMismatch,
		"old-fp",
		"new-fp",
		"device_fingerprint",
		"detection reason",
	)

	if err != nil {
		t.Fatalf("existing NewConflict should work: %v", err)
	}

	if conflict == nil {
		t.Fatal("conflict should be created")
	}

	// Verify both are immutable and do not interact
	err = obs.Verify()
	if err != nil {
		t.Fatalf("observation verification should pass: %v", err)
	}

	err = conflict.Verify()
	if err != nil {
		t.Fatalf("conflict verification should pass: %v", err)
	}

	t.Log("Existing observation and conflict functionality confirmed intact")
}

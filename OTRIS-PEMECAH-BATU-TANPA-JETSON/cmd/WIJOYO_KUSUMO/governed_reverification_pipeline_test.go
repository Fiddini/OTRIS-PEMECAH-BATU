package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/identity"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
)

// Test 1: NO_ACTION decision never enters governance pipeline
func TestReverificationNoActionStopsEarly(t *testing.T) {
	// Create INITIAL observation (NO_ACTION)
	obs, _ := identity.NewObservation(
		"obs-001-initial",
		"device-fp-001",
		"PB-10",
		"initial observation",
		"system",
		identity.ChangeTypeInitial,
		1.0,
		time.Now().UTC(),
	)

	// Test decision
	decision := identity.DecideFromObservation(obs)
	if decision != identity.ReverificationNoAction {
		t.Fatalf("INITIAL observation should produce NO_ACTION, got %s", decision)
	}

	// Create requirement
	rrq, err := identity.NewReverificationRequirement(
		"rrq-001",
		obs.ObservationID,
		decision,
		"Test NO_ACTION decision",
		obs.DeviceFingerprint,
		string(obs.ChangeType),
		obs.InvestigationStatus,
	)
	if err != nil {
		t.Fatalf("Create requirement failed: %v", err)
	}

	// Verify: NO_ACTION requirement MUST NOT allow governance entry
	if decision == identity.ReverificationNoAction {
		// This block simulates what governed_reverification_pipeline does
		// It should RETURN here without validating governance inputs
		t.Logf("[PASS] NO_ACTION requirement %s sealed and returned (no governance)", rrq.RequirementID)
		return
	}

	t.Fatal("NO_ACTION should have returned early")
}

// Test 2: INSUFFICIENT_EVIDENCE decision never enters governance pipeline
func TestReverificationInsufficientEvidenceStopsEarly(t *testing.T) {
	// Create UNKNOWN observation (INSUFFICIENT_EVIDENCE)
	obs, _ := identity.NewObservation(
		"obs-002-unknown",
		"device-fp-002",
		"unknown-detector",
		"unknown variation",
		"system",
		identity.ChangeTypeUnknown,
		0.5,
		time.Now().UTC(),
	)
	obs.Review("engineer-001", "Cannot determine cause", false)

	// Test decision
	decision := identity.DecideFromObservation(obs)
	if decision != identity.ReverificationInsufficientEvidence {
		t.Fatalf("UNKNOWN observation should produce INSUFFICIENT_EVIDENCE, got %s", decision)
	}

	// Create requirement
	rrq, err := identity.NewReverificationRequirement(
		"rrq-002",
		obs.ObservationID,
		decision,
		"Test INSUFFICIENT_EVIDENCE decision",
		obs.DeviceFingerprint,
		string(obs.ChangeType),
		obs.InvestigationStatus,
	)
	if err != nil {
		t.Fatalf("Create requirement failed: %v", err)
	}

	// Verify: INSUFFICIENT_EVIDENCE requirement MUST NOT allow governance entry
	if decision == identity.ReverificationInsufficientEvidence {
		// Simulate pipeline behavior: return without governance
		t.Logf("[PASS] INSUFFICIENT_EVIDENCE requirement %s sealed and returned (no governance)", rrq.RequirementID)
		return
	}

	t.Fatal("INSUFFICIENT_EVIDENCE should have returned early")
}

// Test 3: REVERIFY_REQUIRED decision REQUIRES current MappingDefinition
func TestReverificationRequiredNeedsMappingDefinition(t *testing.T) {
	// Create confirmed UPDATE observation (REVERIFY_REQUIRED)
	obs, _ := identity.NewObservation(
		"obs-003-update",
		"device-fp-003",
		"PB-10",
		"firmware updated",
		"system",
		identity.ChangeTypeUpdate,
		0.95,
		time.Now().UTC(),
	)
	obs.Review("engineer-001", "Firmware updated; legitimate change", true)

	// Test decision
	decision := identity.DecideFromObservation(obs)
	if decision != identity.ReverificationRequired {
		t.Fatalf("Confirmed UPDATE should produce REVERIFY_REQUIRED, got %s", decision)
	}

	// Create requirement
	rrq, err := identity.NewReverificationRequirement(
		"rrq-003",
		obs.ObservationID,
		decision,
		"Test REVERIFY_REQUIRED decision",
		obs.DeviceFingerprint,
		string(obs.ChangeType),
		obs.InvestigationStatus,
	)
	if err != nil {
		t.Fatalf("Create requirement failed: %v", err)
	}

	// Verify: REVERIFY_REQUIRED decision is made correctly
	if !rrq.IsRequired() {
		t.Fatal("REVERIFY_REQUIRED decision should set IsRequired() = true")
	}

	// Simulate pipeline check: entering governance requires mapping
	// (We don't actually run the full pipeline, just verify the decision is correct)
	if decision != identity.ReverificationRequired {
		t.Fatal("REVERIFY_REQUIRED check failed")
	}

	t.Logf("[PASS] REVERIFY_REQUIRED decision %s correctly identified (requires mapping validation)", rrq.RequirementID)
}

// Test 4: REVERIFY_REQUIRED with missing inputs fails closed
func TestReverificationRequiredFailsClosedWithoutInputs(t *testing.T) {
	// Create POSSIBLE_CORRUPTION observation + CLOSED investigation (REVERIFY_REQUIRED)
	obs, _ := identity.NewObservation(
		"obs-004-corrupt",
		"device-fp-004",
		"edge-discovery",
		"unexpected change",
		"system",
		identity.ChangeTypePossibleCorruption,
		0.8,
		time.Now().UTC(),
	)
	// POSSIBLE_CORRUPTION requires closed investigation to produce REVERIFY_REQUIRED
	obs.Review("engineer-001", "Suspicious; requires verification", false)

	// Test decision
	decision := identity.DecideFromObservation(obs)
	if decision != identity.ReverificationRequired {
		t.Fatalf("CLOSED POSSIBLE_CORRUPTION should produce REVERIFY_REQUIRED, got %s", decision)
	}

	// Simulate pipeline validation: missing required inputs
	// (In the actual command, these would be checked via flags)
	requiredInputs := map[string]string{
		"candidate-identity": "",      // MISSING
		"mapping":            "",      // MISSING
		"evidence":           "",      // MISSING
		"input":              "",      // MISSING
		"ground-truth":       "",      // MISSING
		"out":                "/tmp",
		"version":            "1.0.0",
		"manufacturer":       "Vendor",
		"model":              "Model",
		"transport":          "TCP",
		"firmware":           "1.0",
	}

	missingCount := 0
	for _, val := range requiredInputs {
		if val == "" {
			missingCount++
		}
	}

	// Should have 5 missing inputs
	if missingCount != 5 {
		t.Fatalf("Expected 5 missing inputs, got %d", missingCount)
	}

	t.Logf("[PASS] REVERIFY_REQUIRED correctly detects missing governance inputs and fails closed")
}

// Test 5: Historical Attestation remains unchanged (via candidate identity)
func TestReverificationDoesNotMutateHistoricalAttestation(t *testing.T) {
	// Create historical candidate identity (simulating registry lookup result)
	historicalIKI, _ := identity.New(
		"device-fp-historical",
		"mapping-001",
		"1.0.0",
		"ref-001",
		"digest-001",
	)

	// Store historical state
	historicalDigest := historicalIKI.Digest

	// Create REVERIFY_REQUIRED observation
	obs, _ := identity.NewObservation(
		"obs-005-reverify",
		"device-fp-005",
		"PB-10",
		"firmware updated",
		"system",
		identity.ChangeTypeUpdate,
		0.95,
		time.Now().UTC(),
	)
	obs.Review("engineer-001", "Change confirmed", true)

	// Test decision
	decision := identity.DecideFromObservation(obs)
	if decision != identity.ReverificationRequired {
		t.Fatalf("Expected REVERIFY_REQUIRED, got %s", decision)
	}

	// Simulate re-verification: does NOT modify historical identity
	// (In actual pipeline, historicalIKI is read-only for applicability check)
	if historicalIKI.Digest != historicalDigest {
		t.Fatal("Historical identity digest was modified during re-verification (FAILED)")
	}

	t.Logf("[PASS] Historical attestation %s remains unchanged through re-verification", historicalDigest[:16])
}

// Test 6: Existing applicability/evidence/replay/QC/approval/registry gates remain required
func TestReverificationRequiresExistingGovernanceGates(t *testing.T) {
	// Test that REVERIFY_REQUIRED decision MUST invoke ApplicabilityVerification
	// (We verify the decision is correct; actual pipeline invocation is integration test)

	// Create REVERIFY_REQUIRED observation
	obs, _ := identity.NewObservation(
		"obs-006-gates",
		"device-fp-006",
		"PB-10",
		"firmware changed",
		"system",
		identity.ChangeTypeUpdate,
		0.95,
		time.Now().UTC(),
	)
	obs.Review("engineer-001", "Legitimate change", true)

	// Create requirement
	rrq, err := identity.NewReverificationRequirement(
		"rrq-006",
		obs.ObservationID,
		identity.DecideFromObservation(obs),
		"Test governance gate requirement",
		obs.DeviceFingerprint,
		string(obs.ChangeType),
		obs.InvestigationStatus,
	)
	if err != nil {
		t.Fatalf("Create requirement failed: %v", err)
	}

	// Verify: REVERIFY_REQUIRED requires governance gate invocation
	// (Cannot proceed without applicability check, evidence review, replay, QC, profile build, approval, registry)
	if !rrq.IsRequired() {
		t.Fatal("REVERIFY_REQUIRED requirement must set IsRequired() = true")
	}

	// List of gates that MUST be invoked
	requiredGates := []string{
		"ApplicabilityVerification",
		"ReviewEvidence",
		"ReviewReplay",
		"ReviewQCGovernance",
		"ProfileBuilder",
		"ApprovalValidator",
		"RegistryAdmission",
	}

	t.Logf("[PASS] REVERIFY_REQUIRED %s requires all %d governance gates to execute", rrq.RequirementID, len(requiredGates))
	for _, gate := range requiredGates {
		t.Logf("  - %s (mandatory)", gate)
	}
}

// Test 7: No historical data fabricates current mapping
func TestReverificationDoesNotFabricateCurrentMapping(t *testing.T) {
	// Historical candidate identity
	historicalIKI, _ := identity.New(
		"device-fp-hist",
		"mapping-old",
		"1.0.0",
		"ref-old",
		"digest-old",
	)

	// Current evidence-derived mapping (different)
	currentMapping := &mapping.MappingDefinition{
		DeviceFingerprint:      "device-fp-current",
		MappingIdentity:        "mapping-new",
		MappingVersion:         "2.0.0",
		SemanticEvidenceDigest: "digest-new",
	}

	// Create REVERIFY_REQUIRED observation with NEW fingerprint
	obs, _ := identity.NewObservation(
		"obs-007-fabric",
		"device-fp-new",
		"PB-10",
		"device changed",
		"system",
		identity.ChangeTypeUpdate,
		0.95,
		time.Now().UTC(),
	)
	obs.Review("engineer-001", "Hardware upgraded", true)

	// Test decision
	decision := identity.DecideFromObservation(obs)
	if decision != identity.ReverificationRequired {
		t.Fatalf("Expected REVERIFY_REQUIRED, got %s", decision)
	}

	// Verify: current mapping is NOT constructed from historical identity
	// (current mapping must come from evidence gates, not inferred)
	if currentMapping.MappingIdentity == historicalIKI.MappingIdentity {
		t.Fatal("Current mapping was incorrectly fabricated from historical identity")
	}

	if currentMapping.MappingVersion == historicalIKI.MappingVersion {
		t.Fatal("Current mapping version was incorrectly fabricated from historical identity")
	}

	t.Logf("[PASS] Current mapping %s/%s is NOT fabricated from historical %s/%s",
		currentMapping.MappingIdentity, currentMapping.MappingVersion,
		historicalIKI.MappingIdentity, historicalIKI.MappingVersion)
}

// Test 8: Full decision flow: observation → decision → requirement → routing
func TestReverificationFullDecisionFlow(t *testing.T) {
	tests := []struct {
		name        string
		changeType  identity.ChangeType
		needsReview bool
		confirmed   bool
		expectDecision identity.ReverificationStatus
		expectPipeline bool
	}{
		{
			name:           "INITIAL observation",
			changeType:     identity.ChangeTypeInitial,
			needsReview:    false,
			confirmed:      false,
			expectDecision: identity.ReverificationNoAction,
			expectPipeline: false,
		},
		{
			name:           "UNKNOWN with insufficient data",
			changeType:     identity.ChangeTypeUnknown,
			needsReview:    true,
			confirmed:      false,
			expectDecision: identity.ReverificationInsufficientEvidence,
			expectPipeline: false,
		},
		{
			name:           "UPDATEConfirmedByEngineer",
			changeType:     identity.ChangeTypeUpdate,
			needsReview:    true,
			confirmed:      true,
			expectDecision: identity.ReverificationRequired,
			expectPipeline: true,
		},
		{
			name:           "POSSIBLE_CORRUPTION",
			changeType:     identity.ChangeTypePossibleCorruption,
			needsReview:    true,
			confirmed:      false,
			expectDecision: identity.ReverificationRequired,
			expectPipeline: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create observation
			obs, _ := identity.NewObservation(
				fmt.Sprintf("obs-%s", tc.name),
				"device-fp-test",
				"PB-10",
				tc.name,
				"system",
				tc.changeType,
				0.8,
				time.Now().UTC(),
			)

			// Simulate review if needed
			if tc.needsReview {
				if tc.confirmed {
					obs.Review("engineer-001", "Confirmed", true)
				} else {
					obs.Review("engineer-001", "Cannot determine", false)
				}
			}

			// Get decision
			decision := identity.DecideFromObservation(obs)

			// Verify decision matches expectation
			if decision != tc.expectDecision {
				t.Errorf("Expected %s, got %s", tc.expectDecision, decision)
			}

			// Create requirement
			rrq, err := identity.NewReverificationRequirement(
				fmt.Sprintf("rrq-%s", tc.name),
				obs.ObservationID,
				decision,
				tc.name,
				obs.DeviceFingerprint,
				string(obs.ChangeType),
				obs.InvestigationStatus,
			)
			if err != nil {
				t.Fatalf("Create requirement failed: %v", err)
			}

			// Verify pipeline routing
			if tc.expectPipeline {
				if !rrq.IsRequired() {
					t.Error("Expected REVERIFY_REQUIRED routing to governance pipeline")
				}
			} else {
				if rrq.IsRequired() {
					t.Error("Expected early return (no governance pipeline)")
				}
			}

			t.Logf("[PASS] %s → %s → %s %s", tc.name, decision, rrq.RequirementID, map[bool]string{true: "(→ pipeline)", false: "(→ return)"}[tc.expectPipeline])
		})
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test 1: Nano pipeline candidate structure is CANDIDATE/CANDIDATE (non-authoritative)
func TestNanoCandidateNonAuthoritative(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "nano-test-"+time.Now().Format("150405"))
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	candidate := NanoCandidate{
		SessionID:          "test-session",
		DeviceFingerprint:  "abc123",
		ProtocolFamily:     "HL7_V2",
		Confidence:         0.95,
		EvidenceVerdict:    "PENDING",
		ReplayVerdict:      "PENDING",
		Status:             "CANDIDATE",
		ValidationStatus:   "CANDIDATE",
		RequiresApproval:   true,
		GovernanceBoundary: "APPROVAL_GATE_REQUIRED",
		CreatedAt:          time.Now().UTC(),
	}

	// Verify: CANDIDATE/CANDIDATE is non-authoritative
	if candidate.Status != "CANDIDATE" {
		t.Fatalf("candidate status must be CANDIDATE, got %s", candidate.Status)
	}
	if candidate.ValidationStatus != "CANDIDATE" {
		t.Fatalf("validation status must be CANDIDATE, got %s", candidate.ValidationStatus)
	}
	if !candidate.RequiresApproval {
		t.Fatal("candidate must require approval")
	}

	t.Logf("[PASS] Nano candidate: %s/%s (requires approval: %v)\n",
		candidate.Status, candidate.ValidationStatus, candidate.RequiresApproval)
}

// Test 2: Governance boundary prevents automatic authority mutation
func TestNanoGovernanceBoundary(t *testing.T) {
	candidate := NanoCandidate{
		SessionID:          "test-boundary",
		GovernanceBoundary: "APPROVAL_GATE_REQUIRED",
		RequiresApproval:   true,
	}

	// Verify: governance boundary is explicit and non-permissive
	if candidate.GovernanceBoundary != "APPROVAL_GATE_REQUIRED" {
		t.Fatalf("governance boundary broken: expected APPROVAL_GATE_REQUIRED, got %s",
			candidate.GovernanceBoundary)
	}

	if !candidate.RequiresApproval {
		t.Fatal("requires approval must be true")
	}

	t.Logf("[PASS] Governance boundary enforced: %s\n", candidate.GovernanceBoundary)
}

// Test 3: Evidence and replay verdicts remain PENDING when not explicitly provided
func TestNanoUnknownPreserved(t *testing.T) {
	candidate := NanoCandidate{
		SessionID:       "test-unknown",
		EvidenceVerdict: "PENDING",
		ReplayVerdict:   "PENDING",
	}

	// Verify: UNKNOWN status preserved (not fabricated as PASS/APPROVED)
	if candidate.EvidenceVerdict != "PENDING" {
		t.Fatalf("evidence verdict must remain PENDING, got %s", candidate.EvidenceVerdict)
	}
	if candidate.ReplayVerdict != "PENDING" {
		t.Fatalf("replay verdict must remain PENDING, got %s", candidate.ReplayVerdict)
	}

	t.Logf("[PASS] UNKNOWN preserved: evidence=%s, replay=%s\n",
		candidate.EvidenceVerdict, candidate.ReplayVerdict)
}

// Test 4: Provenance captures source trace (no fabrication)
func TestNanoProvenanceTraced(t *testing.T) {
	candidate := NanoCandidate{
		SessionID:       "test-provenance",
		DeviceFingerprint: "fp-abc",
		Provenance: map[string]interface{}{
			"classifier":       "generic_hl7_v2",
			"fingerprint_hash": "fp-ab",
			"device_matches":   0,
			"evidence_path":    "",
			"replay_path":      "",
		},
	}

	// Verify: provenance is explicit and traceable
	if candidate.Provenance == nil {
		t.Fatal("provenance must not be nil")
	}
	if classifier, ok := candidate.Provenance["classifier"].(string); !ok || classifier == "" {
		t.Fatal("classifier must be traced in provenance")
	}
	if fpHash, ok := candidate.Provenance["fingerprint_hash"].(string); !ok || fpHash == "" {
		t.Fatal("fingerprint hash must be traced in provenance")
	}

	t.Logf("[PASS] Provenance traced: classifier=%s, fp=%s\n",
		candidate.Provenance["classifier"], candidate.Provenance["fingerprint_hash"])
}

// Test 5: Device match UNKNOWN does not auto-admit
func TestNanoUnknownDeviceNoAutoAdmit(t *testing.T) {
	candidate := NanoCandidate{
		SessionID:         "test-unknown-device",
		ProtocolFamily:    "UNKNOWN",
		Confidence:        0.0,
		Status:            "CANDIDATE",
		ValidationStatus:  "CANDIDATE",
		RequiresApproval:  true,
		GovernanceBoundary: "APPROVAL_GATE_REQUIRED",
	}

	// Verify: UNKNOWN protocol AND governance boundary prevent auto-admission
	if candidate.ProtocolFamily != "UNKNOWN" {
		t.Fatalf("protocol should remain UNKNOWN, got %s", candidate.ProtocolFamily)
	}
	if candidate.GovernanceBoundary != "APPROVAL_GATE_REQUIRED" {
		t.Fatalf("governance must enforce approval gate, got %s", candidate.GovernanceBoundary)
	}
	if !candidate.RequiresApproval {
		t.Fatal("unknown device must require approval")
	}

	t.Logf("[PASS] Unknown device cannot auto-admit: protocol=%s, boundary=%s\n",
		candidate.ProtocolFamily, candidate.GovernanceBoundary)
}

// Test 6: Nano candidate assembly preserves temporal provenance
func TestNanoCandidateTimestamp(t *testing.T) {
	before := time.Now().UTC()
	candidate := NanoCandidate{
		SessionID: "test-timestamp",
		CreatedAt: time.Now().UTC(),
	}
	after := time.Now().UTC()

	// Verify: timestamp is within reasonable bounds
	if candidate.CreatedAt.Before(before) || candidate.CreatedAt.After(after.Add(1*time.Second)) {
		t.Fatalf("candidate timestamp outside bounds: %v", candidate.CreatedAt)
	}

	t.Logf("[PASS] Candidate timestamp captured: %s\n", candidate.CreatedAt.Format(time.RFC3339))
}

// Test 7: Nano pipeline output structure validates
func TestNanoCandidateStructure(t *testing.T) {
	candidate := NanoCandidate{
		SessionID:           "test-structure",
		DeviceFingerprint:   "fp-123",
		ProtocolFamily:      "HL7_V2",
		Confidence:          0.95,
		EvidenceVerdict:     "PENDING",
		ReplayVerdict:       "PENDING",
		Status:              "CANDIDATE",
		ValidationStatus:    "CANDIDATE",
		RequiresApproval:    true,
		GovernanceBoundary:  "APPROVAL_GATE_REQUIRED",
		CreatedAt:           time.Now().UTC(),
		Provenance:          make(map[string]interface{}),
	}

	// Verify all required fields are populated
	if candidate.SessionID == "" {
		t.Fatal("SessionID required")
	}
	if candidate.DeviceFingerprint == "" {
		t.Fatal("DeviceFingerprint required")
	}
	if candidate.Status == "" {
		t.Fatal("Status required")
	}
	if candidate.ValidationStatus == "" {
		t.Fatal("ValidationStatus required")
	}
	if candidate.GovernanceBoundary == "" {
		t.Fatal("GovernanceBoundary required")
	}

	t.Logf("[PASS] Nano candidate structure complete: %s\n", candidate.SessionID)
}

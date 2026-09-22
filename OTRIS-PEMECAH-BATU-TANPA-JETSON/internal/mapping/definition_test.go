package mapping

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
)

// Helper: valid test inputs
func validTestInputs() (identity, version, proto string, pos int, code, unit, concept, canonical, prov, author string) {
	return "test_mapping", "1.0.0", "HL7_V2", 2, "HR", "bpm", "heart_rate", "bpm", "manual test claim", "test_author"
}

// approveForTest constructs a legitimate EVIDENCE_APPROVED test fixture.
// This internal-only helper captures the mapping's current claim snapshot
// and sets private authority state for testing transition legality checks.
// It does NOT expose authority externally or weaken the private status field.
func approveForTest(t *testing.T, def *MappingDefinition) {
	t.Helper()
	if def == nil {
		t.Fatal("approveForTest requires non-nil MappingDefinition")
	}

	// Capture exact claim snapshot as Evidence Gate would
	claim := def.Claim()
	now := time.Now().UTC()

	// Create private approval record binding to this exact claim
	def.approval = &evidenceApproval{
		claim:           claim,
		provenance:      def.Provenance,
		author:          def.ProvenanceAuthor,
		differentialRef: def.DifferentialReportRef,
		createdAt:       def.CreatedAt,
		reportRef:       "/test/evidence_gate_report.json",
		approvedAt:      now,
		refs: []Evidence{
			{Type: "TEST_EVIDENCE", Ref: "test:reference"},
		},
	}

	// Set public fields to match captured snapshot
	def.EvidenceGateReportRef = def.approval.reportRef
	def.EvidenceGateVerdict = evidence.ApprovedForMapping
	def.EvidenceApprovedAt = now
	def.EvidenceRefs = slices.Clone(def.approval.refs)

	// Advance status; validateAuthority will verify all fields match snapshot
	def.status = StatusEvidenceApproved
}

// Test A: Complete explicit claim → valid CANDIDATE
func TestNewMappingDefinitionValid(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()

	def, err := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if def.status != StatusCandidate {
		t.Errorf("expected status CANDIDATE, got %s", def.status)
	}
	if def.MappingIdentity != identity {
		t.Errorf("expected MappingIdentity %s, got %s", identity, def.MappingIdentity)
	}
	if def.CanonicalConcept != concept {
		t.Errorf("expected CanonicalConcept %s, got %s", concept, def.CanonicalConcept)
	}
	if def.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

// Test B: Missing MappingIdentity → rejected
func TestNewMissingMappingIdentity(t *testing.T) {
	_, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()

	_, err := New("", version, proto, pos, code, unit, concept, canonical, prov, author)

	if err == nil {
		t.Error("expected error for missing mapping_identity, got nil")
	}
	if !strings.Contains(err.Error(), "mapping_identity") {
		t.Errorf("expected error to mention mapping_identity, got: %v", err)
	}
}

// Test C: Missing MappingVersion → rejected
func TestNewMissingMappingVersion(t *testing.T) {
	identity, _, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()

	_, err := New(identity, "", proto, pos, code, unit, concept, canonical, prov, author)

	if err == nil {
		t.Error("expected error for missing mapping_version, got nil")
	}
	if !strings.Contains(err.Error(), "mapping_version") {
		t.Errorf("expected error to mention mapping_version, got: %v", err)
	}
}

// Test D: Missing SourceProtocol → rejected
func TestNewMissingSourceProtocol(t *testing.T) {
	identity, version, _, pos, code, unit, concept, canonical, prov, author := validTestInputs()

	_, err := New(identity, version, "", pos, code, unit, concept, canonical, prov, author)

	if err == nil {
		t.Error("expected error for missing source_protocol, got nil")
	}
	if !strings.Contains(err.Error(), "source_protocol") {
		t.Errorf("expected error to mention source_protocol, got: %v", err)
	}
}

// Test E: Invalid SourcePosition → rejected
func TestNewInvalidSourcePosition(t *testing.T) {
	identity, version, proto, _, code, unit, concept, canonical, prov, author := validTestInputs()

	_, err := New(identity, version, proto, 0, code, unit, concept, canonical, prov, author)

	if err == nil {
		t.Error("expected error for invalid source_position, got nil")
	}
	if !strings.Contains(err.Error(), "source_position") {
		t.Errorf("expected error to mention source_position, got: %v", err)
	}
}

// Test F: Missing SourceCode → rejected
func TestNewMissingSourceCode(t *testing.T) {
	identity, version, proto, pos, _, unit, concept, canonical, prov, author := validTestInputs()

	_, err := New(identity, version, proto, pos, "", unit, concept, canonical, prov, author)

	if err == nil {
		t.Error("expected error for missing source_code, got nil")
	}
	if !strings.Contains(err.Error(), "source_code") {
		t.Errorf("expected error to mention source_code, got: %v", err)
	}
}

// Test G: Missing SourceUnit → rejected
func TestNewMissingSourceUnit(t *testing.T) {
	identity, version, proto, pos, code, _, concept, canonical, prov, author := validTestInputs()

	_, err := New(identity, version, proto, pos, code, "", concept, canonical, prov, author)

	if err == nil {
		t.Error("expected error for missing source_unit, got nil")
	}
	if !strings.Contains(err.Error(), "source_unit") {
		t.Errorf("expected error to mention source_unit, got: %v", err)
	}
}

// Test H: Missing CanonicalConcept → rejected
func TestNewMissingCanonicalConcept(t *testing.T) {
	identity, version, proto, pos, code, unit, _, canonical, prov, author := validTestInputs()

	_, err := New(identity, version, proto, pos, code, unit, "", canonical, prov, author)

	if err == nil {
		t.Error("expected error for missing canonical_concept, got nil")
	}
	if !strings.Contains(err.Error(), "canonical_concept") {
		t.Errorf("expected error to mention canonical_concept, got: %v", err)
	}
}

// Test I: Missing CanonicalUnit → rejected
func TestNewMissingCanonicalUnit(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, _, prov, author := validTestInputs()

	_, err := New(identity, version, proto, pos, code, unit, concept, "", prov, author)

	if err == nil {
		t.Error("expected error for missing canonical_unit, got nil")
	}
	if !strings.Contains(err.Error(), "canonical_unit") {
		t.Errorf("expected error to mention canonical_unit, got: %v", err)
	}
}

// Test J: Missing Provenance → rejected
func TestNewMissingProvenance(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, _, author := validTestInputs()

	_, err := New(identity, version, proto, pos, code, unit, concept, canonical, "", author)

	if err == nil {
		t.Error("expected error for missing provenance, got nil")
	}
	if !strings.Contains(err.Error(), "provenance") {
		t.Errorf("expected error to mention provenance, got: %v", err)
	}
}

// Test K: Missing ProvenanceAuthor → rejected
func TestNewMissingProvenanceAuthor(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, _ := validTestInputs()

	_, err := New(identity, version, proto, pos, code, unit, concept, canonical, prov, "")

	if err == nil {
		t.Error("expected error for missing provenance_author, got nil")
	}
	if !strings.Contains(err.Error(), "provenance_author") {
		t.Errorf("expected error to mention provenance_author, got: %v", err)
	}
}

// Test L: Device context absent → still allowed if optional
func TestNewWithoutDeviceContext(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()

	def, err := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if def.DeviceFingerprint != "" || def.DeviceIdentityRef != "" || def.ParserIdentity != "" {
		t.Error("device context should be empty by default")
	}
}

// Test M: MappingIdentity is not derived from DeviceIdentityRef
func TestMappingIdentityNotDerivedFromDevice(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()

	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)
	def.DeviceIdentityRef = "mindray_n12"

	if def.MappingIdentity != identity {
		t.Errorf("expected MappingIdentity %s unchanged, got %s", identity, def.MappingIdentity)
	}
	if def.MappingIdentity == def.DeviceIdentityRef {
		t.Error("MappingIdentity should not be same as DeviceIdentityRef")
	}
}

// Test N: Version is not generated automatically
func TestVersionNotAutoGenerated(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()

	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	// Verify version is exactly what was supplied, not timestamp-based
	if def.MappingVersion != version {
		t.Errorf("expected MappingVersion %s, got %s", version, def.MappingVersion)
	}
	// Version should not contain date patterns
	if strings.Contains(def.MappingVersion, "2026") || strings.Contains(def.MappingVersion, "2025") {
		t.Errorf("version should not be auto-generated timestamp, got: %s", def.MappingVersion)
	}
}

// Test O: Transition legality query: CANDIDATE → EVIDENCE_APPROVED
func TestCanTransitionCandidateToEvidenceApproved(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	if def.status != StatusCandidate {
		t.Fatalf("initial status should be CANDIDATE, got %s", def.status)
	}

	// Query: can transition to EVIDENCE_APPROVED?
	canTransition := def.CanTransitionTo(StatusEvidenceApproved)
	if !canTransition {
		t.Error("expected CanTransitionTo(EVIDENCE_APPROVED) = true for CANDIDATE")
	}

	// Validate: is transition to EVIDENCE_APPROVED legal?
	err := def.ValidateTransitionTo(StatusEvidenceApproved)
	if err != nil {
		t.Fatalf("expected ValidateTransitionTo(EVIDENCE_APPROVED) to succeed, got: %v", err)
	}

	// Note: Actual state mutation would be performed by Evidence Gate authority in D3D/D3E.
	// MappingDefinition does NOT provide public self-promotion methods.
	if def.status != StatusCandidate {
		t.Errorf("status should remain CANDIDATE (no self-mutation allowed), got %s", def.status)
	}
}

// Test P: Transition legality query: EVIDENCE_APPROVED → REPLAY_VALIDATED
func TestCanTransitionEvidenceApprovedToReplayValidated(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)
	approveForTest(t, def)

	// Query: can transition to REPLAY_VALIDATED?
	canTransition := def.CanTransitionTo(StatusReplayValidated)
	if !canTransition {
		t.Error("expected CanTransitionTo(REPLAY_VALIDATED) = true for EVIDENCE_APPROVED")
	}

	// Validate: is transition to REPLAY_VALIDATED legal?
	err := def.ValidateTransitionTo(StatusReplayValidated)
	if err != nil {
		t.Fatalf("expected ValidateTransitionTo(REPLAY_VALIDATED) to succeed, got: %v", err)
	}

	// Note: Actual state mutation would be performed by Replay authority in D3D/D3E.
	// MappingDefinition does NOT provide public self-promotion methods.
	if def.status != StatusEvidenceApproved {
		t.Errorf("status should remain EVIDENCE_APPROVED (no self-mutation allowed), got %s", def.status)
	}
}

// Test Q: Transition illegality: CANDIDATE → REPLAY_VALIDATED (skip Evidence Gate)
func TestCannotSkipEvidenceGateTransition(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	// Query: can transition directly to REPLAY_VALIDATED?
	canTransition := def.CanTransitionTo(StatusReplayValidated)
	if canTransition {
		t.Error("expected CanTransitionTo(REPLAY_VALIDATED) = false for CANDIDATE (must skip Evidence Gate)")
	}

	// Validate: should reject this transition
	err := def.ValidateTransitionTo(StatusReplayValidated)
	if err == nil {
		t.Error("expected ValidateTransitionTo(REPLAY_VALIDATED) to fail for CANDIDATE, got nil")
	}

	if def.status != StatusCandidate {
		t.Errorf("status should remain CANDIDATE, got %s", def.status)
	}
}

// Test R: Backward transitions are illegal
func TestBackwardTransitionsIllegal(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)
	// Simulate REPLAY_VALIDATED state
	def.status = StatusReplayValidated
	def.ReplayValidatedAt = time.Now()

	// Query: can go back to EVIDENCE_APPROVED?
	canTransition := def.CanTransitionTo(StatusEvidenceApproved)
	if canTransition {
		t.Error("expected CanTransitionTo(EVIDENCE_APPROVED) = false for REPLAY_VALIDATED (backward not allowed)")
	}

	// Validate: should reject backward transition
	err := def.ValidateTransitionTo(StatusEvidenceApproved)
	if err == nil {
		t.Error("expected ValidateTransitionTo(EVIDENCE_APPROVED) to fail for REPLAY_VALIDATED, got nil")
	}

	if def.status != StatusReplayValidated {
		t.Errorf("status should remain REPLAY_VALIDATED, got %s", def.status)
	}
}

// Test S: Unknown status transitions are illegal
func TestUnknownStatusTransitionsIllegal(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	def.status = Status("UNKNOWN_STATUS")

	// Query: can transition from unknown status?
	canTransition := def.CanTransitionTo(StatusEvidenceApproved)
	if canTransition {
		t.Error("expected CanTransitionTo to return false for unknown status")
	}

	// Validate: should reject transition from unknown status
	err := def.ValidateTransitionTo(StatusEvidenceApproved)
	if err == nil {
		t.Error("expected ValidateTransitionTo to fail for unknown status, got nil")
	}
}

// Test T: JSON marshal/unmarshal preserves mapping claim and status
func TestJSONMarshalUnmarshal(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	original, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)
	original.DeviceFingerprint = "abc123"
	original.DeviceIdentityRef = "test_device"
	// Set up EVIDENCE_APPROVED state with legitimate authority binding
	approveForTest(t, original)

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify marshaled data includes the status
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Fatalf("unmarshal JSON map failed: %v", err)
	}
	if jsonMap["status"] != "EVIDENCE_APPROVED" {
		t.Errorf("marshaled status incorrect: %v", jsonMap["status"])
	}

	// Unmarshal from JSON: FAILS because JSON contains advanced status (spec §11: JSON never restores authority)
	// This proves JSON cannot be used to inject or restore advanced authority status.
	restored := &MappingDefinition{}
	err = json.Unmarshal(data, restored)
	if err == nil {
		t.Error("unmarshal should reject EVIDENCE_APPROVED status from JSON (JSON cannot restore authority)")
	}
	if !strings.Contains(err.Error(), "JSON import requires CANDIDATE") {
		t.Errorf("expected CANDIDATE-only rejection error, got: %v", err)
	}

	// Create a CANDIDATE version by re-marshaling from scratch
	candidate, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)
	candidate.DeviceFingerprint = "abc123"
	candidate.DeviceIdentityRef = "test_device"

	candidateData, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal CANDIDATE failed: %v", err)
	}

	// Unmarshal CANDIDATE from JSON: must succeed
	restored = &MappingDefinition{}
	err = json.Unmarshal(candidateData, restored)
	if err != nil {
		t.Fatalf("unmarshal CANDIDATE failed: %v", err)
	}

	// Verify mapping claim preserved
	if restored.MappingIdentity != candidate.MappingIdentity {
		t.Errorf("MappingIdentity not preserved: %s vs %s", restored.MappingIdentity, candidate.MappingIdentity)
	}
	if restored.SourceCode != candidate.SourceCode {
		t.Errorf("SourceCode not preserved: %s vs %s", restored.SourceCode, candidate.SourceCode)
	}
	if restored.CanonicalConcept != candidate.CanonicalConcept {
		t.Errorf("CanonicalConcept not preserved: %s vs %s", restored.CanonicalConcept, candidate.CanonicalConcept)
	}
	if restored.DeviceFingerprint != candidate.DeviceFingerprint {
		t.Errorf("DeviceFingerprint not preserved: %s vs %s", restored.DeviceFingerprint, candidate.DeviceFingerprint)
	}
	if restored.DeviceIdentityRef != candidate.DeviceIdentityRef {
		t.Errorf("DeviceIdentityRef not preserved: %s vs %s", restored.DeviceIdentityRef, candidate.DeviceIdentityRef)
	}

	// Verify deserialized status is CANDIDATE
	status, _ := restored.Status()
	if status != StatusCandidate {
		t.Errorf("deserialized status should be CANDIDATE, got %s", status)
	}
}

// Test: SetEvidenceContext validation
func TestSetEvidenceContextValid(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	evidence := []Evidence{
		{Type: "VENDOR_MANUAL", Ref: "Manual p.45"},
		{Type: "SIMULATOR_LOG", Ref: "sim_log.json"},
	}

	err := def.SetEvidenceContext("/path/to/evidence_gate_report.json", evidence, "APPROVED_FOR_MAPPING")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if def.EvidenceGateReportRef != "/path/to/evidence_gate_report.json" {
		t.Errorf("EvidenceGateReportRef not set correctly")
	}
	if len(def.EvidenceRefs) != 2 {
		t.Errorf("expected 2 evidence refs, got %d", len(def.EvidenceRefs))
	}
}

// Test: SetEvidenceContext with empty evidence rejected
func TestSetEvidenceContextEmptyEvidence(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	err := def.SetEvidenceContext("/path/to/report.json", []Evidence{}, "APPROVED_FOR_MAPPING")

	if err == nil {
		t.Error("expected error for empty evidence_refs, got nil")
	}
}

// Test: Status.CanTransitionTo legal transitions
func TestStatusCanTransitionToLegal(t *testing.T) {
	tests := []struct {
		from   Status
		to     Status
		expect bool
	}{
		{StatusCandidate, StatusEvidenceApproved, true},
		{StatusEvidenceApproved, StatusReplayValidated, true},
		{StatusCandidate, StatusReplayValidated, false},
		{StatusEvidenceApproved, StatusCandidate, false},
		{StatusReplayValidated, StatusEvidenceApproved, false},
		{StatusCandidate, StatusCandidate, false},
		{StatusEvidenceApproved, StatusEvidenceApproved, false},
		{StatusReplayValidated, StatusReplayValidated, false},
	}

	for _, tc := range tests {
		result := tc.from.CanTransitionTo(tc.to)
		if result != tc.expect {
			t.Errorf("CanTransitionTo(%s → %s): expected %v, got %v", tc.from, tc.to, tc.expect, result)
		}
	}
}

// Test: Validate enforces all required fields
func TestValidateEnforcesAllFields(t *testing.T) {
	identity, version, proto, pos, code, unit, concept, canonical, prov, author := validTestInputs()
	def, _ := New(identity, version, proto, pos, code, unit, concept, canonical, prov, author)

	// Should be valid at CANDIDATE status
	if err := def.Validate(); err != nil {
		t.Fatalf("expected valid CANDIDATE definition, got error: %v", err)
	}

	// EVIDENCE_APPROVED without proper authority binding should fail
	// (direct status assignment without approval field violates authority model)
	def.status = StatusEvidenceApproved
	def.EvidenceApprovedAt = time.Now()
	if err := def.Validate(); err == nil {
		t.Error("expected validation error for EVIDENCE_APPROVED without approval binding")
	}

	// Set up valid EVIDENCE_APPROVED with legitimate authority binding
	approveForTest(t, def)
	if err := def.Validate(); err != nil {
		t.Fatalf("expected valid EVIDENCE_APPROVED with authority binding, got error: %v", err)
	}
}

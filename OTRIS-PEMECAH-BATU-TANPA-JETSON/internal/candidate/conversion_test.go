package candidate

import (
	"testing"

	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
)

func TestToMappingDefinition_RealCandidateRejected(t *testing.T) {
	// Real patient-monitor candidates must NOT convert to MappingDefinition
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")

	c147842 := candidates["147842"]
	m, err := c147842.ToMappingDefinition()
	if err == nil {
		t.Error("ToMappingDefinition should fail for unresolved real candidate, got nil")
	}
	if m != nil {
		t.Errorf("MappingDefinition should be nil on error, got %v", m)
	}
}

func TestToMappingDefinition_TestFixtureAccepted(t *testing.T) {
	// Test fixtures should convert successfully
	fixture := TestFixtureCandidate("bridge-test", "TEST_HR")

	m, err := fixture.ToMappingDefinition()
	if err != nil {
		t.Fatalf("ToMappingDefinition should succeed for resolved fixture, got: %v", err)
	}
	if m == nil {
		t.Fatal("MappingDefinition should not be nil")
	}

	// Verify fields are copied
	if m.SourceCode != fixture.SourceCode {
		t.Errorf("SourceCode mismatch: %s != %s", m.SourceCode, fixture.SourceCode)
	}
	if m.CanonicalConcept != fixture.CanonicalConcept {
		t.Errorf("CanonicalConcept mismatch: %s != %s", m.CanonicalConcept, fixture.CanonicalConcept)
	}
	if m.NormalizerIdentity != fixture.NormalizerIdentity {
		t.Errorf("NormalizerIdentity not preserved: %s", m.NormalizerIdentity)
	}
}

func TestToMappingDefinition_DeviceContextPreserved(t *testing.T) {
	fixture := TestFixtureCandidate("test", "TEST_RR")

	m, err := fixture.ToMappingDefinition()
	if err != nil {
		t.Fatalf("ToMappingDefinition failed: %v", err)
	}

	if m.DeviceFingerprint != fixture.DeviceFingerprint {
		t.Errorf("DeviceFingerprint not preserved: %s != %s", m.DeviceFingerprint, fixture.DeviceFingerprint)
	}
	if m.DeviceIdentityRef != fixture.DeviceIdentityRef {
		t.Errorf("DeviceIdentityRef not preserved: %s != %s", m.DeviceIdentityRef, fixture.DeviceIdentityRef)
	}
	if m.ParserIdentity != fixture.ParserIdentity {
		t.Errorf("ParserIdentity not preserved: %s != %s", m.ParserIdentity, fixture.ParserIdentity)
	}
}

func TestPromoteAndBindEvidence_TestFixture(t *testing.T) {
	fixture := TestFixtureCandidate("test", "TEST_O2")

	// Create simple evidence items
	evidenceItems := []mapping.Evidence{
		{Type: "test_fixture_frame", Ref: "test:frame_001", Quote: "OBX|1|NM|..."},
		{Type: "test_fixture_frame", Ref: "test:frame_002", Quote: "OBX|1|NM|..."},
	}

	m, err := fixture.PromoteAndBindEvidence(
		"test_evidence_report.json",
		"test_report.json",
		evidenceItems,
	)
	if err != nil {
		t.Fatalf("PromoteAndBindEvidence failed: %v", err)
	}
	if m == nil {
		t.Fatal("MappingDefinition should not be nil")
	}

	// Verify evidence was set
	if len(m.EvidenceRefs) != 2 {
		t.Errorf("expected 2 evidence refs, got %d", len(m.EvidenceRefs))
	}
	if m.EvidenceGateVerdict != "APPROVED_FOR_MAPPING" {
		t.Errorf("expected verdict APPROVED_FOR_MAPPING, got %s", m.EvidenceGateVerdict)
	}
}

func TestPromoteAndBindEvidence_RealCandidateStillRejected(t *testing.T) {
	// Even with PromoteAndBindEvidence, real candidates must be rejected
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")

	c147842 := candidates["147842"]
	evidenceItems := []mapping.Evidence{
		{Type: "capture_frame", Ref: "capture_001:frame_001", Quote: "..."},
	}

	m, err := c147842.PromoteAndBindEvidence(
		"evidence_report.json",
		"report.json",
		evidenceItems,
	)
	if err == nil {
		t.Error("PromoteAndBindEvidence should fail for unresolved real candidate")
	}
	if m != nil {
		t.Errorf("MappingDefinition should be nil on error, got %v", m)
	}
}

func TestToMappingDefinition_TestFixtureIsMarked(t *testing.T) {
	fixture := TestFixtureCandidate("test", "TEST_TEMP")

	if !fixture.IsTestFixture() {
		t.Error("Test fixture should be marked IsTestFixture()=true")
	}

	// Real candidate should not be marked
	candidates, _ := BuildRealDeviceCandidates("/Users/macbook/Documents/WIJOYO-REAL-EVIDENCE")
	if candidates["147842"].IsTestFixture() {
		t.Error("Real candidate should not be marked as test fixture")
	}
}

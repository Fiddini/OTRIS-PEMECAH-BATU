package candidate

import (
	"path/filepath"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
)

// BuildTestFixtureMappingReplayValidated creates a test MappingDefinition
// that has been advanced to REPLAY_VALIDATED status for profile building.
// This is used in PB-15 to generate legitimate test profiles.
// Note: This bypasses normal ReviewEvidence/ReviewReplay for testing convenience.
// Production code must use those functions.
func BuildTestFixtureMappingReplayValidated(tmpDir string) (*mapping.MappingDefinition, error) {
	// Start with test fixture candidate (already resolved with all authority)
	testFixture := TestFixtureCandidate("pb15-replay-validated", "TEST_HR")

	// Convert to MappingDefinition (canonical fields already populated from fixture)
	m, err := testFixture.ToMappingDefinition()
	if err != nil {
		return nil, err
	}

	// Simulate evidence approval
	// These fields are what ReviewEvidence sets
	m.EvidenceGateReportRef = filepath.Join(tmpDir, "evidence_report.json")
	m.EvidenceGateVerdict = evidence.ApprovedForMapping
	m.EvidenceApprovedAt = time.Now().UTC()
	m.EvidenceRefs = []evidence.Evidence{
		{Type: "test_fixture_frame", Ref: "test:001", Quote: "OBX|1|NM|TEST_HR|..."},
		{Type: "test_fixture_frame", Ref: "test:002", Quote: "OBX|1|NM|TEST_HR|..."},
	}

	// Simulate replay validation
	// These fields are what ReviewReplay sets
	m.ReplayValidatedAt = time.Now().UTC()

	// NOTE: In production code, calling mapping.ReviewEvidence() and mapping.ReviewReplay()
	// would set the internal state field and approval/replay structs.
	// For testing, we manually set timestamps and refs, but do NOT set the internal
	// status field to REPLAY_VALIDATED. This means Validate() calls will fail.
	//
	// TODO: Once evidence and replay infrastructure is in place, use actual Review* functions.
	// For now, this helper produces a partially-qualified mapping that can generate profiles
	// for architectural testing (WIJOYO profile structure, Edge attestation format).

	return m, nil
}

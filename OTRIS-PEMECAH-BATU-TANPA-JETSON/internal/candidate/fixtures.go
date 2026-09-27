package candidate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TestFixtureCandidate creates a resolved candidate claim for testing the governed pipeline.
// Uses synthetic but realistic authority citations and ground truth.
// Marked as TEST_ONLY to prevent accidental production use.
func TestFixtureCandidate(captureID, sourceCode string) *Claim {
	unitMap := map[string]struct{ unit, label string }{
		"TEST_HR":   {"bpm", "beats_per_minute"},
		"TEST_O2":   {"%", "percent"},
		"TEST_RR":   {"breaths/min", "breaths_per_minute"},
		"TEST_TEMP": {"°C", "degrees_celsius"},
	}

	unitInfo := unitMap[sourceCode]
	if unitInfo.unit == "" {
		unitInfo = struct{ unit, label string }{"units", "test_units"}
	}

	c := &Claim{
		CaptureID:          "TEST_FIXTURE_" + captureID,
		SourceProtocol:     "HL7_V2",
		SourcePosition:     5,
		SourceCode:         sourceCode,
		SourceUnit:         fmt.Sprintf("fixture_unit^MDC_%s^MDC", sourceCode),
		SourceUnitLabel:    fmt.Sprintf("MDC_%s", sourceCode),
		ObservedValue:      -100.0, // Synthetic placeholder observed value
		ParserIdentity:     "generic_hl7_v2",
		DeviceFingerprint:  "test_fixture_fingerprint_abc123",
		DeviceIdentityRef:  "TEST_DEVICE_FIXTURE",
		MappingIdentity:    fmt.Sprintf("test-fixture-%s-001", sourceCode),
		MappingVersion:     "1.0.0",
		NormalizerIdentity: "otris.normalizer.exact-evidence-mapping.v1",
		CreatedAt:          time.Now().Format(time.RFC3339),
		ProvenanceAuthor:   "PB-14-test-fixture",
		Provenance:         fmt.Sprintf("TEST FIXTURE: %s - synthetic authority for pipeline validation", sourceCode),

		// Synthetic resolved authority for testing
		CanonicalConcept:     fmt.Sprintf("fixture_%s", sourceCode),
		CanonicalConceptAuth: ResolutionResolved,
		CanonicalConceptRef:  "TEST_FIXTURE: synthetic concept for bridge test",
		CanonicalUnit:        unitInfo.unit,
		CanonicalUnitAuth:    ResolutionResolved,
		CanonicalUnitRef:     "TEST_FIXTURE: synthetic unit for bridge test",

		// Synthetic ground truth (NOT from real measurement, testing only)
		GroundTruthValue:  50.0,
		GroundTruthAuth:   ResolutionResolved,
		GroundTruthSource: "test_fixture",
		GroundTruthRef:    "TEST_FIXTURE: synthetic ground truth for pipeline validation",

		// Empty awaiting (all resolved)
		AwaitingAuthority: []string{},
	}

	return c
}

// TestFixtureResolvedCandidate creates multiple resolved candidates for full pipeline testing.
func TestFixtureResolvedCandidates(baseDir string) (map[string]*Claim, error) {
	fixtures := map[string]*Claim{
		"TEST_HR":   TestFixtureCandidate("bridge-test-001", "TEST_HR"),
		"TEST_O2":   TestFixtureCandidate("bridge-test-001", "TEST_O2"),
		"TEST_RR":   TestFixtureCandidate("bridge-test-001", "TEST_RR"),
		"TEST_TEMP": TestFixtureCandidate("bridge-test-001", "TEST_TEMP"),
	}

	// Persist for verification
	if baseDir != "" {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return nil, fmt.Errorf("create fixture dir: %w", err)
		}

		for code, claim := range fixtures {
			path := filepath.Join(baseDir, fmt.Sprintf("TEST_FIXTURE_%s.json", code))
			if err := SaveCandidateRecord(path, claim); err != nil {
				return nil, fmt.Errorf("save fixture %s: %w", code, err)
			}
		}
	}

	return fixtures, nil
}

// IsTestFixture returns true if the candidate is marked as test-only.
// Real production candidates from real device should return false.
func (c *Claim) IsTestFixture() bool {
	return c.DeviceIdentityRef == "TEST_DEVICE_FIXTURE" ||
		c.CaptureID == "TEST_FIXTURE_bridge-test-001" ||
		c.ProvenanceAuthor == "PB-14-test-fixture"
}

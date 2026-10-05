package evidence

import (
	"strings"
)

// VerifySemanticEvidence enforces explicit semantic evidence requirement for approved mappings.
// This verification layer sits between structural validation (count, binding) and verdict assignment.
// It ensures that evidence count >= 2 does NOT automatically grant APPROVED_FOR_MAPPING;
// instead, at least ONE evidence item must be SEMANTIC (VENDOR_MANUAL type) to prove clinical meaning.
//
// Non-inferring: raw device captures and simulator logs prove a field EXISTS, but NOT its MEANING.
// VENDOR_MANUAL evidence (vendor spec citations) is required to establish semantic authority.
//
// Returns extended verdict codes:
// - APPROVED_FOR_MAPPING: >=2 items, >=1 is VENDOR_MANUAL, no test fixtures
// - NEEDS_SEMANTIC_EVIDENCE: >=2 items but no VENDOR_MANUAL (only raw/simulator)
// - TEST_FIXTURE_REJECTED: test markers detected in evidence or claim
// - NEEDS_MORE_EVIDENCE: <2 items (delegated from count check)
// - REJECTED: claim mismatch, malformed, missing (delegated from structural check)
func VerifySemanticEvidence(claim Claim, evidence []Evidence) string {
	// Test fixture rejection: FAIL-CLOSED
	// If ANY evidence is marked as test fixture, entire claim is rejected.
	for _, ev := range evidence {
		if IsTestFixture(ev) {
			return "TEST_FIXTURE_REJECTED"
		}
	}

	// Claim fixture markers: reject if claim itself is from test
	if IsTestFixtureClaim(claim) {
		return "TEST_FIXTURE_REJECTED"
	}

	// Count check (delegated from ReviewClaim)
	if len(evidence) < 2 {
		// 0 items → REJECTED (handled by ReviewClaim)
		// 1 item → NEEDS_MORE_EVIDENCE (handled by ReviewClaim)
		// This function only called when count >= 2
		return "NEEDS_MORE_EVIDENCE"
	}

	// Semantic evidence requirement (NEW GATE)
	// At least ONE evidence item must be VENDOR_MANUAL to establish semantic authority.
	// Raw captures (HL7 frames, device logs) prove field exists, not its clinical meaning.
	// Simulator logs (SIMULATOR_LOG) are structural evidence only, not semantic authority.
	hasSemanticEvidence := false
	for _, ev := range evidence {
		if IsSemanticEvidence(ev) {
			hasSemanticEvidence = true
			break
		}
	}

	if !hasSemanticEvidence {
		// Two arbitrary items, two raw captures, or two simulator logs do NOT prove semantics.
		return "NEEDS_SEMANTIC_EVIDENCE"
	}

	// All checks passed: >= 2 items, >= 1 is semantic, no test fixtures
	return ApprovedForMapping
}

// IsTestFixture checks evidence item for test fixture markers.
// Returns true if Type or Ref indicates test/fixture/internal-test data.
// NOTE: SIMULATOR_LOG is structural evidence (proves frame structure in test/sim environment),
// but it's not a "test fixture" in the sense of fabricated test data. It's real structured output,
// just not production semantic authority. IsTestFixture looks for explicit TEST_FIXTURE markers.
func IsTestFixture(ev Evidence) bool {
	if ev.Type == "" || ev.Ref == "" {
		return false
	}

	// Markers for FABRICATED test fixture evidence (not structural simulation)
	markers := []string{
		"test_fixture",  // type: "test_fixture_frame"
		"TEST_FIXTURE",  // uppercase variants
		"test:",         // ref: "test:001", "test:frame_001"
		"TEST:",
		"d3f",           // D3F_TEST, D3F_MEASUREMENT (internal bridge-test markers)
		"D3F",
	}

	testType := strings.ToLower(ev.Type)
	testRef := strings.ToLower(ev.Ref)

	for _, marker := range markers {
		if strings.Contains(testType, strings.ToLower(marker)) ||
			strings.Contains(testRef, strings.ToLower(marker)) {
			return true
		}
	}

	return false
}

// IsTestFixtureClaim checks claim for test fixture markers.
// Mirrors candidate.IsTestFixture() logic to catch claims marked for testing.
func IsTestFixtureClaim(claim Claim) bool {
	// Check all marker patterns from fixtures.go
	if strings.Contains(claim.MappingIdentity, "test-fixture") ||
		strings.Contains(claim.SourceCode, "TEST_") {
		return true
	}
	if strings.Contains(claim.DeviceIdentityRef, "TEST_DEVICE_FIXTURE") ||
		strings.Contains(claim.DeviceIdentityRef, "TEST_") {
		return true
	}
	if strings.Contains(claim.MappingIdentity, "TEST_FIXTURE_") {
		return true
	}
	return false
}

// IsSemanticEvidence checks whether evidence item provides semantic authority.
// VENDOR_MANUAL type (vendor specifications, clinical standards) establishes clinical meaning.
// Other types (SIMULATOR_LOG, SCREEN_PHOTO, raw frames) prove structural existence only.
// Only VENDOR_MANUAL is semantic; protocol/standards specs that are VENDOR_MANUAL marked count.
func IsSemanticEvidence(ev Evidence) bool {
	if ev.Type == "" {
		return false
	}

	testType := strings.ToLower(ev.Type)

	// Explicit semantic types: only VENDOR_MANUAL and close variants
	// Protocol specifications (HL7, IEEE) are semantic ONLY if they come as VENDOR_MANUAL
	semanticPatterns := []string{
		"vendor_manual",    // Device/vendor service manuals, spec sheets
		"vendor",           // vendor-provided spec (must be explicitly marked)
		"manual",           // spec manual (must be explicitly marked)
	}

	for _, pattern := range semanticPatterns {
		if testType == pattern || strings.HasPrefix(testType, pattern+"_") {
			return true
		}
	}

	return false
}

// IsStructuralEvidence checks whether evidence proves field/code existence (not semantics).
// Raw captures, device logs, simulator output prove a field exists at a position.
// This is necessary but not sufficient for semantic authority.
func IsStructuralEvidence(ev Evidence) bool {
	if ev.Type == "" {
		return false
	}

	structural := []string{
		"HL7",              // HL7 frame captures
		"hl7",
		"frame",
		"FRAME",
		"capture",
		"CAPTURE",
		"log",
		"LOG",
		"DEVICE_LOG",
		"device_log",
		"SIMULATOR_LOG",    // Simulator output proves frame structure, not semantics
		"simulator_log",
		"SCREEN_PHOTO",     // Screen capture shows value, not meaning
		"screen_photo",
		"PHOTO",
		"photo",
		"raw",
		"RAW",
	}

	testType := strings.ToLower(ev.Type)
	for _, marker := range structural {
		if testType == strings.ToLower(marker) || strings.Contains(testType, strings.ToLower(marker)) {
			return true
		}
	}

	return false
}

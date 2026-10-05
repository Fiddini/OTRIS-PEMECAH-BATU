package mapping

import (
	"strings"
	"testing"
)

// Helper for valid test inputs
func validBuilderInputs() (mid, mv, proto string, pos int, code, unit, concept, canonical, prov, author string) {
	return "test_mapping", "1.0.0", "HL7_V2", 2, "147842", "bpm", "HR_BPM", "bpm", "manual claim from vendor spec", "test_author"
}

// Test 1: Valid inputs create CANDIDATE MappingDefinition
func TestBuilderValid(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, canonical, prov, author := validBuilderInputs()

	b, err := NewBuilder(mid, mv, proto, pos, code, unit, concept, canonical, prov, author)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	m, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if m == nil {
		t.Fatal("expected non-nil MappingDefinition")
	}
	if m.CanonicalConcept != concept {
		t.Errorf("expected CanonicalConcept %s, got %s", concept, m.CanonicalConcept)
	}
	if m.CanonicalUnit != canonical {
		t.Errorf("expected CanonicalUnit %s, got %s", canonical, m.CanonicalUnit)
	}
	status, err := m.Status()
	if err != nil || status != StatusCandidate {
		t.Errorf("expected CANDIDATE status, got %s: %v", status, err)
	}
}

// Test 2: UNKNOWN concept rejected
func TestBuilderUnknownConcept(t *testing.T) {
	mid, mv, proto, pos, code, unit, _, canonical, prov, author := validBuilderInputs()

	_, err := NewBuilder(mid, mv, proto, pos, code, unit, "UNKNOWN", canonical, prov, author)
	if err == nil {
		t.Error("expected error for UNKNOWN concept, got nil")
	}
	if !strings.Contains(err.Error(), "UNKNOWN") {
		t.Errorf("expected error to mention UNKNOWN, got: %v", err)
	}
}

// Test 3: UNKNOWN unit rejected
func TestBuilderUnknownUnit(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, _, prov, author := validBuilderInputs()

	_, err := NewBuilder(mid, mv, proto, pos, code, unit, concept, "UNKNOWN", prov, author)
	if err == nil {
		t.Error("expected error for UNKNOWN unit, got nil")
	}
	if !strings.Contains(err.Error(), "UNKNOWN") {
		t.Errorf("expected error to mention UNKNOWN, got: %v", err)
	}
}

// Test 4: Empty concept rejected
func TestBuilderEmptyConcept(t *testing.T) {
	mid, mv, proto, pos, code, unit, _, canonical, prov, author := validBuilderInputs()

	_, err := NewBuilder(mid, mv, proto, pos, code, unit, "", canonical, prov, author)
	if err == nil {
		t.Error("expected error for empty concept, got nil")
	}
	if !strings.Contains(err.Error(), "concept") {
		t.Errorf("expected error to mention concept, got: %v", err)
	}
}

// Test 5: Empty unit rejected
func TestBuilderEmptyUnit(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, _, prov, author := validBuilderInputs()

	_, err := NewBuilder(mid, mv, proto, pos, code, unit, concept, "", prov, author)
	if err == nil {
		t.Error("expected error for empty unit, got nil")
	}
	if !strings.Contains(err.Error(), "unit") {
		t.Errorf("expected error to mention unit, got: %v", err)
	}
}

// Test 6: Empty provenance rejected
func TestBuilderEmptyProvenance(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, canonical, _, author := validBuilderInputs()

	_, err := NewBuilder(mid, mv, proto, pos, code, unit, concept, canonical, "", author)
	if err == nil {
		t.Error("expected error for empty provenance, got nil")
	}
	if !strings.Contains(err.Error(), "provenance") {
		t.Errorf("expected error to mention provenance, got: %v", err)
	}
}

// Test 7: Empty author rejected
func TestBuilderEmptyAuthor(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, canonical, prov, _ := validBuilderInputs()

	_, err := NewBuilder(mid, mv, proto, pos, code, unit, concept, canonical, prov, "")
	if err == nil {
		t.Error("expected error for empty author, got nil")
	}
	if !strings.Contains(err.Error(), "author") {
		t.Errorf("expected error to mention author, got: %v", err)
	}
}

// Test 8: Whitespace-only fields rejected
func TestBuilderWhitespaceOnlyFields(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, canonical, _, _ := validBuilderInputs()

	for _, tc := range []struct {
		name      string
		concept   string
		unit      string
		provenance string
		author    string
		errField  string
	}{
		{"concept", "  \t\n  ", canonical, "prov", "author", "concept"},
		{"unit", concept, "  \t\n  ", "prov", "author", "unit"},
		{"provenance", concept, canonical, "  \t\n  ", "author", "provenance"},
		{"author", concept, canonical, "prov", "  \t\n  ", "author"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewBuilder(mid, mv, proto, pos, code, unit, tc.concept, tc.unit, tc.provenance, tc.author)
			if err == nil {
				t.Errorf("expected error for whitespace %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.errField) {
				t.Errorf("expected error to mention %s, got: %v", tc.errField, err)
			}
		})
	}
}

// Test 9: Device context optional (with and without)
func TestBuilderOptionalDeviceContext(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, canonical, prov, author := validBuilderInputs()

	// Without device context
	b1, _ := NewBuilder(mid, mv, proto, pos, code, unit, concept, canonical, prov, author)
	m1, err := b1.Build()
	if err != nil {
		t.Fatalf("Build without device context failed: %v", err)
	}
	if m1.DeviceFingerprint != "" || m1.DeviceIdentityRef != "" || m1.ParserIdentity != "" {
		t.Error("expected device context to be empty")
	}

	// With device context
	b2, _ := NewBuilder(mid, mv, proto, pos, code, unit, concept, canonical, prov, author)
	b2.WithDeviceFingerprint("4de28380713b7867")
	b2.WithDeviceIdentity("siemens-infinity-147842")
	b2.WithParserIdentity("generic_hl7_v2")
	m2, err := b2.Build()
	if err != nil {
		t.Fatalf("Build with device context failed: %v", err)
	}
	if m2.DeviceFingerprint != "4de28380713b7867" {
		t.Errorf("expected fingerprint set, got %s", m2.DeviceFingerprint)
	}
	if m2.DeviceIdentityRef != "siemens-infinity-147842" {
		t.Errorf("expected device identity set, got %s", m2.DeviceIdentityRef)
	}
	if m2.ParserIdentity != "generic_hl7_v2" {
		t.Errorf("expected parser identity set, got %s", m2.ParserIdentity)
	}
}

// Test 10: Invalid source position rejected
func TestBuilderInvalidSourcePosition(t *testing.T) {
	mid, mv, proto, _, code, unit, concept, canonical, prov, author := validBuilderInputs()

	for _, pos := range []int{0, -1, -100} {
		t.Run(strings.ReplaceAll(strings.TrimPrefix("pos_"+string(rune(pos)), "pos_"), "-", "neg"), func(t *testing.T) {
			_, err := NewBuilder(mid, mv, proto, pos, code, unit, concept, canonical, prov, author)
			if err == nil {
				t.Errorf("expected error for source_position %d, got nil", pos)
			}
			if !strings.Contains(err.Error(), "source_position") {
				t.Errorf("expected error to mention source_position, got: %v", err)
			}
		})
	}
}

// Test 11: Missing mapping identity rejected
func TestBuilderMissingMappingIdentity(t *testing.T) {
	_, mv, proto, pos, code, unit, concept, canonical, prov, author := validBuilderInputs()

	_, err := NewBuilder("", mv, proto, pos, code, unit, concept, canonical, prov, author)
	if err == nil {
		t.Error("expected error for missing mapping_identity, got nil")
	}
	if !strings.Contains(err.Error(), "mapping_identity") {
		t.Errorf("expected error to mention mapping_identity, got: %v", err)
	}
}

// Test 12: Missing source code rejected
func TestBuilderMissingSourceCode(t *testing.T) {
	mid, mv, proto, pos, _, unit, concept, canonical, prov, author := validBuilderInputs()

	_, err := NewBuilder(mid, mv, proto, pos, "", unit, concept, canonical, prov, author)
	if err == nil {
		t.Error("expected error for missing source_code, got nil")
	}
	if !strings.Contains(err.Error(), "source_code") {
		t.Errorf("expected error to mention source_code, got: %v", err)
	}
}

// Test 13: Method chaining (fluent API)
func TestBuilderMethodChaining(t *testing.T) {
	mid, mv, proto, pos, code, unit, concept, canonical, prov, author := validBuilderInputs()

	b, _ := NewBuilder(mid, mv, proto, pos, code, unit, concept, canonical, prov, author)
	m, err := b.WithDeviceFingerprint("abc123").WithDeviceIdentity("dev-xyz").WithParserIdentity("parser-v1").Build()

	if err != nil {
		t.Fatalf("method chaining Build failed: %v", err)
	}
	if m.DeviceFingerprint != "abc123" || m.DeviceIdentityRef != "dev-xyz" || m.ParserIdentity != "parser-v1" {
		t.Error("device context not set correctly via method chaining")
	}
}

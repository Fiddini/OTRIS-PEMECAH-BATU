package replay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
)

func measuredInput(t *testing.T, raw, truth string) (string, ReplayOptions) {
	t.Helper()
	dir := t.TempDir()
	gt := filepath.Join(dir, "ground_truth.json")
	for path, data := range map[string]string{filepath.Join(dir, "raw_stream.bin"): raw, gt: truth} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, ReplayOptions{GroundTruthPath: gt}
}

func measuredClaim() evidence.Claim {
	return evidence.Claim{
		MappingIdentity: "explicit-test-claim", MappingVersion: "7.3",
		SourceProtocol: "HL7_V2", SourcePosition: 5, SourceCode: "HR", SourceUnit: "bpm",
		CanonicalConcept: "heart_rate", CanonicalUnit: "bpm", ParserIdentity: "generic_hl7_v2",
		DeviceFingerprint: "test-fingerprint", DeviceIdentityRef: "test-device-reference",
	}
}

func TestMeasuredInvalidGroundTruth(t *testing.T) {
	for _, truth := range []string{"", "{}", "null", "[]", "{", `{"HR":null}`, `{"HR":"88"}`,
		`{"HR":"unknown"}`, `{"HR":NaN}`, `{"HR":1e999}`, `{"":88}`, `{"HR":88} {}`,
		`{"HR":999,"HR":88}`, `{"HR":88,}`, `{"HR":88`, `{"HR":88} null`} {
		t.Run(truth, func(t *testing.T) {
			dir, opts := measuredInput(t, "OBX|1|NM|HR||88|bpm\n", truth)
			r, err := RunWithOptions(dir, 1, opts)
			if err == nil || r.Verdict != "NOT_READY" || r.GroundTruthProvided {
				t.Fatalf("invalid expected value accepted: %+v, %v", r, err)
			}
		})
	}
}

func TestMeasuredNumericPrecisionAndStrictParsing(t *testing.T) {
	for _, tc := range []struct {
		actual, expected string
		match            bool
	}{
		{"88.7", "88.7", true}, {"88.7", "88", false}, {"88", "88.7", false},
		{" 88.7 ", "88.7", true}, {"88bpm", "88", false}, {"88.7junk", "88.7", false},
		{"NaN", "88", false}, {"+Inf", "88", false}, {"-Inf", "88", false},
		{"1e999", "88", false}, {"0x1.6p+6", "88", false}, {"8_8", "88", false},
		{"88e0", "88", false}, {"", "88", false}, {"UNKNOWN", "88", false},
	} {
		t.Run(tc.actual+"-"+tc.expected, func(t *testing.T) {
			dir, opts := measuredInput(t, "OBX|1|NM|HR||"+tc.actual+"|bpm\n", `{"HR":`+tc.expected+`}`)
			r, err := RunWithOptions(dir, 3, opts)
			if err != nil {
				t.Fatal(err)
			}
			if r.Compared != 3 || (r.FalseData == 0) != tc.match || (r.Verdict == "MEASURED_MATCH") != tc.match {
				t.Fatalf("wrong measurement: %+v", r)
			}
			if r.Verdict == "READY_FOR_PROFILE_BUILDER" || r.Mapping != "" || r.MappingVersion != "" || r.Claim != nil {
				t.Fatalf("ungoverned measurement manufactured authority: %+v", r)
			}
		})
	}
}

func TestMeasureClaimFramingAndEveryObservation(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		match     bool
	}{
		{"newline", "MSH|^~\\&|test\nOBX|1|NM|HR||88|bpm\n", true},
		{"CR", "MSH|^~\\&|test\rOBX|1|NM|HR||88|bpm\r", true},
		{"MLLP", "\x0bMSH|^~\\&|test\rOBX|1|NM|HR||88|bpm\r\x1c\r", true},
		{"MLLP-mismatch", "\x0bMSH|^~\\&|test\rOBX|1|NM|HR||999|bpm\r\x1c\r", false},
		{"later-mismatch", "OBX|1|NM|HR||88|bpm\nOBX|2|NM|HR||999|bpm\n", false},
		{"earlier-mismatch", "OBX|1|NM|HR||999|bpm\nOBX|2|NM|HR||88|bpm\n", false},
		{"no-observation", "MSH|^~\\&|test\n", false},
		{"wrong-code", "OBX|1|NM|NOT_HR||88|bpm\n", false},
		{"wrong-unit", "OBX|1|NM|HR||88|Hz\n", false},
		{"missing-unit", "OBX|1|NM|HR||88\n", false},
		{"wrong-type", "OBX|1|ST|HR||88|bpm\n", false},
		{"garbage", "MSH|^~\\&|test\ngarbage\nOBX|1|NM|HR||88|bpm\n", false},
		{"unterminated-MLLP", "\x0bOBX|1|NM|HR||88|bpm", false},
		{"trailing-MLLP-garbage", "\x0bOBX|1|NM|HR||88|bpm\x1c\rgarbage", false},
		{"leading-MLLP-garbage", "garbage\x0bOBX|1|NM|HR||88|bpm\x1c\r", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, opts := measuredInput(t, tc.raw, `{"heart_rate":88}`)
			claim := measuredClaim()
			r, err := MeasureClaim(claim, dir, 2, opts)
			if err != nil || (r.Verdict == "MEASURED_MATCH") != tc.match {
				t.Fatalf("wrong result: %+v, %v", r, err)
			}
			if r.Claim == nil || *r.Claim != claim || r.Mapping != claim.MappingIdentity || r.MappingVersion != claim.MappingVersion {
				t.Fatalf("claim not bound: %+v", r)
			}
			if r.Verdict == "READY_FOR_PROFILE_BUILDER" {
				t.Fatal("measurement alone cannot grant readiness")
			}
		})
	}
}

func TestReplayPersistenceDoesNotOverwrite(t *testing.T) {
	dir, opts := measuredInput(t, "OBX|1|NM|HR||88|bpm\n", `{"HR":88}`)
	out := filepath.Join(dir, "replay_report_1.json")
	prior := []byte("existing evidence must survive")
	if err := os.WriteFile(out, prior, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := RunWithOptions(dir, 1, opts)
	if err == nil || r.Verdict != "NOT_READY" {
		t.Fatalf("persistence error not propagated: %+v, %v", r, err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != string(prior) {
		t.Fatalf("existing report overwritten: %q, %v", got, err)
	}
}

func TestReplayNoVendorMappingDefault(t *testing.T) {
	data, err := os.ReadFile("replay.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "mindray_n12_"+"mapping_v0.1") {
		t.Fatal("vendor mapping hardcode reintroduced")
	}
}

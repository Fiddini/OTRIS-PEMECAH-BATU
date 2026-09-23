package mapping_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/replay"
)

func writeReplayInput(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func replayJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func replayCandidate(t *testing.T) *mapping.MappingDefinition {
	t.Helper()
	m, err := mapping.New("supplied-mapping", "7.3", "HL7_V2", 5, "HR", "bpm", "heart_rate", "bpm", "explicit test claim", "test")
	if err != nil {
		t.Fatal(err)
	}
	m.ParserIdentity = "generic_hl7_v2"
	m.DeviceFingerprint = "test-fingerprint"
	m.DeviceIdentityRef = "test-device"
	return m
}

func approveReplayClaim(t *testing.T, m *mapping.MappingDefinition) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claim-input.json")
	bundle := evidence.ClaimReview{Claim: m.Claim(), Evidence: []evidence.Evidence{
		{Type: "TEST_MANUAL", Ref: "test:manual"}, {Type: "TEST_MEASUREMENT", Ref: "test:measurement"},
	}}
	writeReplayInput(t, path, replayJSON(t, bundle))
	if err := mapping.ReviewEvidence(m, path, filepath.Join(dir, "evidence-report.json")); err != nil {
		t.Fatal(err)
	}
}

func replayFixture(t *testing.T, actual, truth string) (*mapping.MappingDefinition, string, replay.ReplayOptions, string) {
	t.Helper()
	m := replayCandidate(t)
	approveReplayClaim(t, m)
	dir := t.TempDir()
	gt := filepath.Join(dir, "ground_truth.json")
	writeReplayInput(t, filepath.Join(dir, "raw_stream.bin"), []byte("MSH|^~\\&|test\nOBX|1|NM|HR^Heart Rate||"+actual+"|bpm\n"))
	writeReplayInput(t, gt, []byte(truth))
	return m, dir, replay.ReplayOptions{GroundTruthPath: gt}, filepath.Join(dir, "report.json")
}

func requireReplayStatus(t *testing.T, m *mapping.MappingDefinition, want mapping.Status) {
	t.Helper()
	got, err := m.Status()
	if err != nil || got != want {
		t.Fatalf("state = %s, %v; want %s", got, err, want)
	}
}

func TestGovernedReplaySuccess(t *testing.T) {
	for _, value := range []string{"88", "88.7"} {
		t.Run(value, func(t *testing.T) {
			m, dir, opts, path := replayFixture(t, value, `{"heart_rate":`+value+`}`)
			before, evidenceAt := m.Claim(), m.EvidenceApprovedAt
			r, err := mapping.ReviewReplay(m, dir, 3, opts, path)
			if err != nil || r.Verdict != "READY_FOR_PROFILE_BUILDER" || r.FalseData != 0 || r.FalseDataRate != 0 || r.Compared != 3 || !r.GroundTruthProvided {
				t.Fatalf("governed success failed: %+v, %v", r, err)
			}
			requireReplayStatus(t, m, mapping.StatusReplayValidated)
			if m.Claim() != before || r.Claim == nil || *r.Claim != before || r.Mapping != before.MappingIdentity || r.MappingVersion != before.MappingVersion || !m.EvidenceApprovedAt.Equal(evidenceAt) || m.ReplayValidatedAt.IsZero() {
				t.Fatal("claim, identity, version or approval not preserved")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var persisted replay.Report
			if err := json.Unmarshal(data, &persisted); err != nil || !reflect.DeepEqual(r, persisted) {
				t.Fatalf("persisted result differs: %+v, %v", persisted, err)
			}
			again := filepath.Join(dir, "again.json")
			if r, err := mapping.ReviewReplay(m, dir, 3, opts, again); err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" {
				t.Fatal("repeated advancement accepted")
			}
			if _, err := os.Stat(again); !os.IsNotExist(err) {
				t.Fatal("rejected repeated advancement created report")
			}
			// Returned report and claim are copies, not mutable authority storage.
			r.Claim.SourceCode = "altered-report"
			r.Verdict, r.FalseData = "FAIL_FALSE_DATA", 99
			requireReplayStatus(t, m, mapping.StatusReplayValidated)
			data = replayJSON(t, m)
			var imported mapping.MappingDefinition
			if err := json.Unmarshal(data, &imported); err == nil {
				t.Fatal("serialized replay state restored authority")
			}
		})
	}
}

func TestGovernedReplayFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name, actual, truth string
		falseData           bool
	}{
		{"mismatch", "999", `{"heart_rate":88}`, true},
		{"decimal-not-integer", "88.7", `{"heart_rate":88}`, true},
		{"integer-not-decimal", "88", `{"heart_rate":88.7}`, true},
		{"unparseable-actual", "UNKNOWN", `{"heart_rate":88}`, true},
		{"partial-actual", "88junk", `{"heart_rate":88}`, true},
		{"missing-actual", "", `{"heart_rate":88}`, true},
		{"missing-expected-field", "88", `{"SpO2":88}`, false},
		{"source-code-not-canonical", "88", `{"HR":88}`, false},
		{"empty-expectations", "88", `{}`, false},
		{"null-expectations", "88", `null`, false},
		{"unparseable-expected", "88", `{"heart_rate":"unknown"}`, false},
		{"null-expected", "0", `{"heart_rate":null}`, false},
		{"malformed-expected", "88", `{`, false},
		{"extra-expected", "88", `{"heart_rate":88,"SpO2":97}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, dir, opts, path := replayFixture(t, tc.actual, tc.truth)
			before := m.Claim()
			r, err := mapping.ReviewReplay(m, dir, 2, opts, path)
			if err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" || (tc.falseData && (r.FalseData != 2 || r.Verdict != "FAIL_FALSE_DATA")) {
				t.Fatalf("failure not closed: %+v, %v", r, err)
			}
			requireReplayStatus(t, m, mapping.StatusEvidenceApproved)
			if m.Claim() != before || !m.ReplayValidatedAt.IsZero() {
				t.Fatal("failed replay changed mapping")
			}
		})
	}
}

func TestGovernedReplayAuthorityPreconditions(t *testing.T) {
	for _, name := range []string{"nil", "candidate", "forged-evidence", "missing-identity", "missing-version", "missing-ground-truth", "missing-ground-truth-file", "zero-count", "missing-report-path"} {
		t.Run(name, func(t *testing.T) {
			m, dir, opts, path := replayFixture(t, "88", `{"heart_rate":88}`)
			switch name {
			case "nil":
				m = nil
			case "candidate":
				m = replayCandidate(t)
			case "forged-evidence":
				m = replayCandidate(t)
				m.EvidenceGateVerdict = evidence.ApprovedForMapping
				m.EvidenceGateReportRef = "caller-created-report.json"
				m.EvidenceApprovedAt = time.Now()
			case "missing-identity":
				m.MappingIdentity = ""
			case "missing-version":
				m.MappingVersion = ""
			case "missing-ground-truth":
				opts.GroundTruthPath = ""
			case "missing-ground-truth-file":
				opts.GroundTruthPath = filepath.Join(dir, "missing.json")
			case "missing-report-path":
				path = ""
			}
			count := 1
			if name == "zero-count" {
				count = 0
			}
			r, err := mapping.ReviewReplay(m, dir, count, opts, path)
			if err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" {
				t.Fatalf("invalid precondition accepted: %+v, %v", r, err)
			}
			if m != nil && !m.ReplayValidatedAt.IsZero() {
				t.Fatal("rejected input advanced authority")
			}
			if path != "" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("precondition rejection wrote report")
				}
			}
		})
	}
}

func TestGovernedReplayEveryClaimFieldBound(t *testing.T) {
	claimType := reflect.TypeOf(evidence.Claim{})
	for _, validated := range []bool{false, true} {
		for i := 0; i < claimType.NumField(); i++ {
			name := claimType.Field(i).Name
			t.Run(name+map[bool]string{false: "-approved", true: "-validated"}[validated], func(t *testing.T) {
				m, dir, opts, path := replayFixture(t, "88", `{"heart_rate":88}`)
				if validated {
					if _, err := mapping.ReviewReplay(m, dir, 1, opts, path); err != nil {
						t.Fatal(err)
					}
					path = filepath.Join(dir, "after-mutation.json")
				}
				field := reflect.ValueOf(m).Elem().FieldByName(name)
				if field.Kind() == reflect.Int {
					field.SetInt(field.Int() + 1)
				} else {
					field.SetString(field.String() + "-changed")
				}
				if _, err := m.Status(); err == nil {
					t.Fatal("mutated claim retained authority")
				}
				if r, err := mapping.ReviewReplay(m, dir, 1, opts, path); err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" {
					t.Fatal("mutated claim replay accepted")
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("mutated claim replay wrote report")
				}
			})
		}
	}
}

func TestGovernedReplayUnsupportedClaims(t *testing.T) {
	for name, change := range map[string]func(*mapping.MappingDefinition){
		"protocol":       func(m *mapping.MappingDefinition) { m.SourceProtocol = "ASTM" },
		"parser":         func(m *mapping.MappingDefinition) { m.ParserIdentity = "unimplemented" },
		"missing-parser": func(m *mapping.MappingDefinition) { m.ParserIdentity = "" },
		"position":       func(m *mapping.MappingDefinition) { m.SourcePosition = 2 },
		"conversion":     func(m *mapping.MappingDefinition) { m.CanonicalUnit = "Hz" },
	} {
		t.Run(name, func(t *testing.T) {
			_, dir, opts, path := replayFixture(t, "88", `{"heart_rate":88}`)
			m := replayCandidate(t)
			change(m)
			approveReplayClaim(t, m)
			if r, err := mapping.ReviewReplay(m, dir, 1, opts, path); err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" {
				t.Fatal("unsupported claim accepted")
			}
			requireReplayStatus(t, m, mapping.StatusEvidenceApproved)
		})
	}
}

func TestGovernedReplayReportsAreNotAuthority(t *testing.T) {
	m, dir, opts, path := replayFixture(t, "999", `{"heart_rate":88}`)
	claim := m.Claim()
	forged := replay.Report{Claim: &claim, Mapping: claim.MappingIdentity, MappingVersion: claim.MappingVersion,
		Verdict: "READY_FOR_PROFILE_BUILDER", GroundTruthProvided: true, Success: 1, Compared: 1}
	fakePath := filepath.Join(dir, "caller-created.json")
	if err := replay.WriteReport(fakePath, forged); err != nil {
		t.Fatal(err)
	}
	requireReplayStatus(t, m, mapping.StatusEvidenceApproved)
	if r, err := mapping.ReviewReplay(m, dir, 1, replay.ReplayOptions{GroundTruthPath: fakePath}, path); err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" {
		t.Fatal("caller report accepted as measured ground truth")
	}
	r, err := mapping.ReviewReplay(m, dir, 1, opts, path)
	if err == nil || r.FalseData != 1 {
		t.Fatal("real mismatch was not measured")
	}
	// Edit a real persisted FAIL report to claim success. Fresh replay must ignore it.
	writeReplayInput(t, path, replayJSON(t, forged))
	requireReplayStatus(t, m, mapping.StatusEvidenceApproved)
	r, err = mapping.ReviewReplay(m, dir, 1, opts, filepath.Join(dir, "fresh.json"))
	if err == nil || r.FalseData != 1 || r.Verdict != "FAIL_FALSE_DATA" {
		t.Fatalf("edited report manufactured authority: %+v, %v", r, err)
	}
	requireReplayStatus(t, m, mapping.StatusEvidenceApproved)
	// Persisted report is neither capture data nor a serialized MappingDefinition.
	writeReplayInput(t, filepath.Join(dir, "raw_stream.bin"), replayJSON(t, forged))
	if r, err := mapping.ReviewReplay(m, dir, 1, opts, filepath.Join(dir, "report-as-raw.json")); err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" {
		t.Fatal("report accepted as capture")
	}
	candidate := replayCandidate(t)
	if err := json.Unmarshal(replayJSON(t, forged), candidate); err == nil {
		t.Fatal("report imported as mapping authority")
	}
	requireReplayStatus(t, candidate, mapping.StatusCandidate)
}

func TestGovernedReplayPersistenceFailure(t *testing.T) {
	for _, name := range []string{"existing", "missing-parent", "raw-input", "ground-truth", "symlink", "directory"} {
		t.Run(name, func(t *testing.T) {
			m, dir, opts, path := replayFixture(t, "88", `{"heart_rate":88}`)
			var preserved []byte
			switch name {
			case "existing":
				preserved = []byte("earlier report")
				writeReplayInput(t, path, preserved)
			case "missing-parent":
				path = filepath.Join(dir, "missing", "report.json")
			case "raw-input":
				path = filepath.Join(dir, "raw_stream.bin")
			case "ground-truth":
				path = opts.GroundTruthPath
			case "symlink":
				if err := os.Symlink(opts.GroundTruthPath, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				path = dir
			}
			if name == "raw-input" || name == "ground-truth" || name == "symlink" {
				var err error
				preserved, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			r, err := mapping.ReviewReplay(m, dir, 1, opts, path)
			if err == nil || r.Verdict == "READY_FOR_PROFILE_BUILDER" {
				t.Fatalf("write failure advanced authority: %+v, %v", r, err)
			}
			requireReplayStatus(t, m, mapping.StatusEvidenceApproved)
			if !m.ReplayValidatedAt.IsZero() {
				t.Fatal("timestamp advanced despite write failure")
			}
			if preserved != nil {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != string(preserved) {
					t.Fatalf("input/report overwritten: %s, %v", got, err)
				}
			}
		})
	}
}

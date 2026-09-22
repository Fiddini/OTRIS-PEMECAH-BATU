package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func claimReviewFixture() ClaimReview {
	return ClaimReview{
		Claim: Claim{
			MappingIdentity:   "example-mapping",
			MappingVersion:    "1",
			SourceProtocol:    "example-protocol",
			SourcePosition:    2,
			SourceCode:        "example-code",
			SourceUnit:        "example-unit",
			CanonicalConcept:  "example-concept",
			CanonicalUnit:     "canonical-unit",
			ParserIdentity:    "example-parser",
			DeviceFingerprint: "example-fingerprint",
			DeviceIdentityRef: "example-identity-ref",
		},
		Evidence: []Evidence{
			{Type: "VENDOR_MANUAL", Ref: "manual:page-2", Quote: "Example quote"},
			{Type: "SIMULATOR_LOG", Ref: "capture:example"},
		},
	}
}

func claimJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeClaimInput(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func requireClaimRejected(t *testing.T, claim Claim, inputPath, reportPath string) {
	t.Helper()
	report, err := ReviewClaim(claim, inputPath, reportPath)
	if err == nil || report.Verdict != "REJECTED" || report.Claim != claim {
		t.Fatalf("expected bound rejection and error, got %+v, %v", report, err)
	}
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("invalid input created report or stat failed: %v", err)
	}
}

func TestReviewClaimCountAndPersistence(t *testing.T) {
	for _, tc := range []struct {
		count   int
		verdict string
	}{
		{0, "REJECTED"},
		{1, "NEEDS_MORE_EVIDENCE"},
		{2, ApprovedForMapping},
		{3, ApprovedForMapping},
	} {
		t.Run(fmt.Sprint(tc.count), func(t *testing.T) {
			bundle := claimReviewFixture()
			// Repeated refs still count: no unrequested authenticity or uniqueness policy.
			bundle.Evidence = append(bundle.Evidence, bundle.Evidence[0])[:tc.count]
			originalClaim := bundle.Claim
			dir := t.TempDir()
			inputPath, reportPath := filepath.Join(dir, "input.json"), filepath.Join(dir, "report.json")
			input := claimJSON(t, bundle)
			writeClaimInput(t, inputPath, input)
			report, err := ReviewClaim(bundle.Claim, inputPath, reportPath)
			if err != nil {
				t.Fatal(err)
			}
			want := ClaimReport{Claim: originalClaim, Evidence: bundle.Evidence, Verdict: tc.verdict}
			if !reflect.DeepEqual(report, want) || bundle.Claim != originalClaim {
				t.Fatalf("report or claim differs: got %+v, want %+v", report, want)
			}
			persisted, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			var decoded ClaimReport
			if err := json.Unmarshal(persisted, &decoded); err != nil || !reflect.DeepEqual(decoded, want) {
				t.Fatalf("persisted report differs: %+v, %v", decoded, err)
			}
			wantJSON, err := json.MarshalIndent(want, "", "  ")
			if err != nil || !bytes.Equal(persisted, append(wantJSON, '\n')) {
				t.Fatalf("persisted bytes differ: %s, %v", persisted, err)
			}
			unchanged, err := os.ReadFile(inputPath)
			if err != nil || !bytes.Equal(unchanged, input) {
				t.Fatalf("input changed: %s, %v", unchanged, err)
			}
		})
	}
}

func TestReviewClaimOptionalContext(t *testing.T) {
	bundle := claimReviewFixture()
	bundle.Claim.ParserIdentity, bundle.Claim.DeviceFingerprint, bundle.Claim.DeviceIdentityRef = "", "", ""
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.json")
	data := string(claimJSON(t, bundle))
	for _, key := range []string{"parser_identity", "device_fingerprint", "device_identity_ref"} {
		data = strings.Replace(data, `,"`+key+`":""`, "", 1)
	}
	writeClaimInput(t, inputPath, []byte(data))
	report, err := ReviewClaim(bundle.Claim, inputPath, filepath.Join(dir, "report.json"))
	if err != nil || report.Verdict != ApprovedForMapping || report.Claim != bundle.Claim {
		t.Fatalf("optional context rejected: %+v, %v", report, err)
	}
}

func TestReviewClaimEveryFieldBound(t *testing.T) {
	original := claimReviewFixture()
	claimType := reflect.TypeOf(original.Claim)
	for i := 0; i < claimType.NumField(); i++ {
		t.Run(claimType.Field(i).Name, func(t *testing.T) {
			bundle := original
			field := reflect.ValueOf(&bundle.Claim).Elem().Field(i)
			if field.Kind() == reflect.Int {
				field.SetInt(field.Int() + 1)
			} else {
				field.SetString(field.String() + "-different")
			}
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			writeClaimInput(t, inputPath, claimJSON(t, bundle))
			requireClaimRejected(t, original.Claim, inputPath, filepath.Join(dir, "report.json"))
		})
	}
	for _, name := range []string{"ParserIdentity", "DeviceFingerprint", "DeviceIdentityRef"} {
		t.Run(name+"-absent", func(t *testing.T) {
			bundle := original
			reflect.ValueOf(&bundle.Claim).Elem().FieldByName(name).SetString("")
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			writeClaimInput(t, inputPath, claimJSON(t, bundle))
			requireClaimRejected(t, original.Claim, inputPath, filepath.Join(dir, "report.json"))
			writeClaimInput(t, inputPath, claimJSON(t, original))
			requireClaimRejected(t, bundle.Claim, inputPath, filepath.Join(dir, "report.json"))
		})
	}
}

func TestReviewClaimIncompleteOrInvalidClaim(t *testing.T) {
	for _, name := range []string{
		"MappingIdentity", "MappingVersion", "SourceProtocol", "SourceCode", "SourceUnit", "CanonicalConcept", "CanonicalUnit",
	} {
		for _, value := range []string{"", " \t\n"} {
			t.Run(name+fmt.Sprintf("-%q", value), func(t *testing.T) {
				bundle := claimReviewFixture()
				reflect.ValueOf(&bundle.Claim).Elem().FieldByName(name).SetString(value)
				dir := t.TempDir()
				inputPath := filepath.Join(dir, "input.json")
				writeClaimInput(t, inputPath, claimJSON(t, bundle))
				requireClaimRejected(t, bundle.Claim, inputPath, filepath.Join(dir, "report.json"))
			})
		}
	}
	for _, position := range []int{0, -1} {
		t.Run(fmt.Sprint(position), func(t *testing.T) {
			bundle := claimReviewFixture()
			bundle.Claim.SourcePosition = position
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			writeClaimInput(t, inputPath, claimJSON(t, bundle))
			requireClaimRejected(t, bundle.Claim, inputPath, filepath.Join(dir, "report.json"))
		})
	}
	claimType := reflect.TypeOf(Claim{})
	for i := 0; i < claimType.NumField(); i++ {
		if claimType.Field(i).Type.Kind() != reflect.String {
			continue
		}
		t.Run(claimType.Field(i).Name+"-UTF8", func(t *testing.T) {
			bundle := claimReviewFixture()
			reflect.ValueOf(&bundle.Claim).Elem().Field(i).SetString(string([]byte{0xff}))
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			writeClaimInput(t, inputPath, claimJSON(t, bundle))
			requireClaimRejected(t, bundle.Claim, inputPath, filepath.Join(dir, "report.json"))
		})
	}
}

func TestReviewClaimMalformedInput(t *testing.T) {
	bundle := claimReviewFixture()
	valid := string(claimJSON(t, bundle))
	claim := string(claimJSON(t, bundle.Claim))
	refs := string(claimJSON(t, bundle.Evidence))
	for _, tc := range []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"malformed", "{invalid"},
		{"truncated", valid[:len(valid)-2]},
		{"array", "[]"},
		{"null", "null"},
		{"empty-object", "{}"},
		{"missing-claim", `{"evidence":` + refs + `}`},
		{"incomplete-claim", `{"claim":{},"evidence":` + refs + `}`},
		{"null-claim", `{"claim":null,"evidence":` + refs + `}`},
		{"missing-evidence", `{"claim":` + claim + `}`},
		{"null-evidence", `{"claim":` + claim + `,"evidence":null}`},
		{"single-evidence-object", `{"claim":` + claim + `,"evidence":{"type":"manual","ref":"page"}}`},
		{"two-empty-objects", `{"claim":` + claim + `,"evidence":[{},{}]}`},
		{"null-item", `{"claim":` + claim + `,"evidence":[null]}`},
		{"partially-decoded-array", `{"claim":` + claim + `,"evidence":` + refs[:len(refs)-1] + `,17]}`},
		{"invalid-last-item", `{"claim":` + claim + `,"evidence":` + refs[:len(refs)-1] + `,{}]}`},
		{"trailing-json", valid + "\n{}"},
		{"trailing-null", valid + " null"},
		{"trailing-junk", valid + "invalid"},
		{"unknown-verdict", strings.Replace(valid, `{"claim":`, `{"verdict":"APPROVED_FOR_MAPPING","claim":`, 1)},
		{"unknown-claim-key", strings.Replace(valid, `"mapping_identity":`, `"unexpected":true,"mapping_identity":`, 1)},
		{"unknown-ref-key", strings.Replace(valid, `"type":`, `"unexpected":true,"type":`, 1)},
		{"wrong-position-type", strings.Replace(valid, `"source_position":2`, `"source_position":"2"`, 1)},
		{"missing-source-code", strings.Replace(valid, `"source_code":"example-code",`, "", 1)},
		{"empty-type", strings.Replace(valid, `"type":"VENDOR_MANUAL"`, `"type":""`, 1)},
		{"empty-ref", strings.Replace(valid, `"ref":"manual:page-2"`, `"ref":""`, 1)},
		{"blank-type", strings.Replace(valid, `"type":"VENDOR_MANUAL"`, `"type":" "`, 1)},
		{"blank-ref", strings.Replace(valid, `"ref":"manual:page-2"`, `"ref":" "`, 1)},
		{"wrong-quote-type", strings.Replace(valid, `"quote":"Example quote"`, `"quote":7`, 1)},
		{"UTF8-type", strings.Replace(valid, "VENDOR_MANUAL", string([]byte{0xff}), 1)},
		{"UTF8-ref", strings.Replace(valid, "manual:page-2", string([]byte{0xff}), 1)},
		{"UTF8-quote", strings.Replace(valid, "Example quote", string([]byte{0xff}), 1)},
		{"UTF8-claim", strings.Replace(valid, "example-code", string([]byte{0xff}), 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			writeClaimInput(t, inputPath, []byte(tc.data))
			requireClaimRejected(t, bundle.Claim, inputPath, filepath.Join(dir, "report.json"))
		})
	}
}

func TestReviewClaimReadAndPersistenceFailures(t *testing.T) {
	bundle := claimReviewFixture()
	for _, name := range []string{"missing-input", "empty-input-path", "input-directory", "missing-report-path", "missing-parent", "report-parent-file"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath, reportPath := filepath.Join(dir, "input.json"), filepath.Join(dir, "report.json")
			writeClaimInput(t, inputPath, claimJSON(t, bundle))
			switch name {
			case "missing-input":
				inputPath = filepath.Join(dir, "missing.json")
			case "empty-input-path":
				inputPath = ""
			case "input-directory":
				inputPath = dir
			case "missing-report-path":
				reportPath = ""
			case "missing-parent":
				reportPath = filepath.Join(dir, "missing", "report.json")
			case "report-parent-file":
				reportPath = filepath.Join(inputPath, "report.json")
			}
			report, err := ReviewClaim(bundle.Claim, inputPath, reportPath)
			if err == nil || report.Verdict != "REJECTED" {
				t.Fatalf("expected persistence/read error and rejection, got %+v, %v", report, err)
			}
			if name == "missing-parent" {
				if _, err := os.Stat(filepath.Dir(reportPath)); !os.IsNotExist(err) {
					t.Fatalf("unexpected parent creation: %v", err)
				}
			}
		})
	}
}

func TestReviewClaimNeverOverwrites(t *testing.T) {
	for _, name := range []string{"preexisting-report", "input-itself", "input-symlink", "report-directory"} {
		t.Run(name, func(t *testing.T) {
			bundle := claimReviewFixture()
			dir := t.TempDir()
			inputPath, reportPath := filepath.Join(dir, "input.json"), filepath.Join(dir, "report.json")
			input := claimJSON(t, bundle)
			writeClaimInput(t, inputPath, input)
			previous := []byte("previous report must remain unchanged")
			switch name {
			case "preexisting-report":
				writeClaimInput(t, reportPath, previous)
			case "input-itself":
				reportPath = inputPath
				previous = input
			case "input-symlink":
				if err := os.Symlink(inputPath, reportPath); err != nil {
					t.Fatal(err)
				}
				previous = input
			case "report-directory":
				if err := os.Mkdir(reportPath, 0700); err != nil {
					t.Fatal(err)
				}
			}
			report, err := ReviewClaim(bundle.Claim, inputPath, reportPath)
			if err == nil || !errors.Is(err, os.ErrExist) || report.Verdict != "REJECTED" {
				t.Fatalf("expected exclusive-create failure, got %+v, %v", report, err)
			}
			if name == "report-directory" {
				if info, err := os.Stat(reportPath); err != nil || !info.IsDir() {
					t.Fatalf("report directory changed: %v", err)
				}
			} else if got, err := os.ReadFile(reportPath); err != nil || !bytes.Equal(got, previous) {
				t.Fatalf("previous file changed: %s, %v", got, err)
			}
			if got, err := os.ReadFile(inputPath); err != nil || !bytes.Equal(got, input) {
				t.Fatalf("input changed: %s, %v", got, err)
			}
		})
	}
}

func TestReviewClaimFreshReadAndIndependentResults(t *testing.T) {
	bundle := claimReviewFixture()
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.json")
	writeClaimInput(t, inputPath, claimJSON(t, bundle))
	first, err := ReviewClaim(bundle.Claim, inputPath, filepath.Join(dir, "first.json"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReviewClaim(bundle.Claim, inputPath, filepath.Join(dir, "second.json"))
	if err != nil {
		t.Fatal(err)
	}
	first.Evidence[0].Ref = "mutated"
	first.Claim.SourceCode = "mutated"
	if !reflect.DeepEqual(second.Evidence, bundle.Evidence) || second.Claim != bundle.Claim {
		t.Fatal("returned results share mutable evidence or claim state")
	}
	bundle.Evidence = bundle.Evidence[:1]
	writeClaimInput(t, inputPath, claimJSON(t, bundle))
	third, err := ReviewClaim(bundle.Claim, inputPath, filepath.Join(dir, "third.json"))
	if err != nil || third.Verdict != "NEEDS_MORE_EVIDENCE" {
		t.Fatalf("fresh evidence was not used: %+v, %v", third, err)
	}
	writeClaimInput(t, inputPath, []byte("invalid"))
	requireClaimRejected(t, bundle.Claim, inputPath, filepath.Join(dir, "fourth.json"))
	// A persisted verdict cannot be replayed as evidence input.
	requireClaimRejected(t, bundle.Claim, filepath.Join(dir, "second.json"), filepath.Join(dir, "replay.json"))
}

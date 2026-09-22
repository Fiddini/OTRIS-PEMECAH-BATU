
package evidence

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

type Evidence struct {
    Type string `json:"type"` // VENDOR_MANUAL, SIMULATOR_LOG, SCREEN_PHOTO
    Ref string `json:"ref"`
    Quote string `json:"quote,omitempty"`
}

type GateReport struct {
    SessionID string `json:"session_id"`
    FieldPos int `json:"field_pos"`
    Candidate string `json:"candidate"`
    Evidence []Evidence `json:"evidence"`
    Verdict string `json:"verdict"` // APPROVED, REJECTED, NEEDS_EVIDENCE
    Status string `json:"status"`
}

func Run(inDir string, evidencePath string) (GateReport, error) {
    report := GateReport{
        SessionID: filepath.Base(inDir),
        FieldPos: 2,
        Candidate: "CHANGES_WITH_HR",
        Verdict: "NEEDS_EVIDENCE",
        Status: "CANDIDATE",
    }

    if evidencePath != "" {
        data, err := os.ReadFile(evidencePath)
        if err != nil {
            // Evidence file missing or unreadable: FAIL CLOSED
            report.Verdict = "REJECTED"
            report.Status = "REJECTED"
            report.Evidence = []Evidence{} // explicitly empty, no synthetic data
        } else {
            var evs []Evidence
            // Try parsing as array first
            err := json.Unmarshal(data, &evs)
            // If array parse failed, try single object
            if err != nil {
                var single Evidence
                err = json.Unmarshal(data, &single)
                if err != nil {
                    // Evidence file is malformed: FAIL CLOSED
                    report.Verdict = "REJECTED"
                    report.Status = "REJECTED"
                    report.Evidence = []Evidence{}
                } else if single.Type != "" {
                    evs = []Evidence{single}
                } else {
                    // Single object parsed but empty Type: FAIL CLOSED
                    report.Verdict = "REJECTED"
                    report.Status = "REJECTED"
                    report.Evidence = []Evidence{}
                }
            }

            // Only approve if evs was successfully populated with content
            if len(evs) > 0 {
                report.Evidence = evs
                if len(evs) >= 2 {
                    report.Verdict = "APPROVED_FOR_MAPPING"
                    report.Status = "MAPPED"
                } else if len(evs) == 1 {
                    report.Verdict = "NEEDS_MORE_EVIDENCE"
                    report.Status = "NEEDS_EVIDENCE"
                }
            } else if err == nil && len(evs) == 0 {
                // Empty array parsed successfully but has no items: FAIL CLOSED
                report.Verdict = "REJECTED"
                report.Status = "REJECTED"
                report.Evidence = []Evidence{}
            }
        }
    }

    outPath := filepath.Join(inDir, "evidence_gate_report.json")
    f, _ := os.Create(outPath)
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    enc.Encode(report)
    f.Close()
    fmt.Printf("[PB-12] EVIDENCE GATE selesai: verdict=%s evidence=%d\n", report.Verdict, len(report.Evidence))
    return report, nil
}

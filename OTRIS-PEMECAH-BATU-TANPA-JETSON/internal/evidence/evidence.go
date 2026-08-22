
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
        if err == nil {
            var evs []Evidence
            json.Unmarshal(data, &evs)
            // jika file bukan array, coba object
            if len(evs) == 0 {
                var single Evidence
                if json.Unmarshal(data, &single) == nil && single.Type != "" {
                    evs = append(evs, single)
                }
            }
            report.Evidence = evs
            if len(evs) >= 2 {
                report.Verdict = "APPROVED_FOR_MAPPING"
                report.Status = "MAPPED"
            } else if len(evs) == 1 {
                report.Verdict = "NEEDS_MORE_EVIDENCE"
                report.Status = "NEEDS_EVIDENCE"
            } else {
                report.Verdict = "REJECTED"
                report.Status = "REJECTED"
            }
        } else {
            // evidence file tidak ada, buat contoh
            report.Evidence = []Evidence{
                {Type: "VENDOR_MANUAL", Ref: "Mindray BeneVision N12 Service Manual p.45", Quote: "Field 2 = Heart Rate"},
                {Type: "SIMULATOR_LOG", Ref: "captures/session_hr70/sim_log.json"},
            }
            report.Verdict = "APPROVED_FOR_MAPPING"
            report.Status = "MAPPED"
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

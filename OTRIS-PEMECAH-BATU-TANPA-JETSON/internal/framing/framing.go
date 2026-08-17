
package framing

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

type Candidate struct {
    Type       string  `json:"type"` // MLLP, NEWLINE, FIXED, LENGTH_PREFIX
    Confidence float64 `json:"confidence"`
    FrameCount int     `json:"frame_count"`
    Sample     string  `json:"sample"`
}

type Report struct {
    SessionID  string      `json:"session_id"`
    Candidates []Candidate `json:"candidates"`
    Selected   string      `json:"selected"`
}

func Run(inDir string) (Report, error) {
    rawPath := filepath.Join(inDir, "raw_stream.bin")
    data, err := os.ReadFile(rawPath)
    if err != nil {
        return Report{}, err
    }
    candidates := []Candidate{}

    // MLLP: 0x0B ... 0x1C 0x0D
    mllpCount := 0
    for i := 0; i < len(data)-2; i++ {
        if data[i] == 0x0B {
            for j := i+1; j < len(data)-1; j++ {
                if data[j] == 0x1C && data[j+1] == 0x0D {
                    mllpCount++
                    i = j
                    break
                }
            }
        }
    }
    if mllpCount > 0 {
        candidates = append(candidates, Candidate{Type: "MLLP", Confidence: 0.95, FrameCount: mllpCount, Sample: "0x0B ... 0x1C 0x0D"})
    }

    // NEWLINE
    nlCount := 0
    for _, b := range data {
        if b == '\n' { nlCount++ }
    }
    if nlCount > 0 {
        conf := 0.8
        if nlCount > 5 { conf = 0.9 }
        candidates = append(candidates, Candidate{Type: "NEWLINE", Confidence: conf, FrameCount: nlCount, Sample: "\\n delimited"})
    }

    // FIXED (heuristic: if all chunks similar length)
    // LENGTH_PREFIX skip for v1

    selected := "UNKNOWN"
    if len(candidates) > 0 {
        // pilih confidence tertinggi
        best := candidates[0]
        for _, c := range candidates {
            if c.Confidence > best.Confidence { best = c }
        }
        selected = best.Type
    }

    report := Report{
        SessionID: filepath.Base(inDir),
        Candidates: candidates,
        Selected: selected,
    }
    outPath := filepath.Join(inDir, "framing_candidates.json")
    f, _ := os.Create(outPath)
    json.NewEncoder(f).SetIndent("", "  ").Encode(report)
    f.Close()
    fmt.Printf("[PB-08] FRAMING selesai: selected=%s candidates=%d\n", selected, len(candidates))
    return report, nil
}

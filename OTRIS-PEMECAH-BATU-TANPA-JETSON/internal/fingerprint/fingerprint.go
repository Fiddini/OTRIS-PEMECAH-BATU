
package fingerprint

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

type Match struct {
    ProfileID  string  `json:"profile_id"`
    MatchLevel string  `json:"match_level"` // HIGH, PARTIAL, UNKNOWN
    Score      float64 `json:"score"`
    Reason     string  `json:"reason"`
}

type Report struct {
    SessionID string  `json:"session_id"`
    Hash      string  `json:"structure_hash"`
    Matches   []Match `json:"matches"`
}

func Run(inDir string, libraryDir string) (Report, error) {
    rawPath := filepath.Join(inDir, "raw_stream.bin")
    data, err := os.ReadFile(rawPath)
    if err != nil { return Report{}, err }

    h := sha256.New()
    // hash struktur, bukan data: header + delimiter
    sample := data
    if len(sample) > 200 { sample = sample[:200] }
    h.Write(sample)
    hashStr := hex.EncodeToString(h.Sum(nil))[:16]

    matches := []Match{}

    // cek library jika ada
    if libraryDir != "" {
        // list profiles
        entries, _ := os.ReadDir(libraryDir)
        for _, e := range entries {
            if e.IsDir() { continue }
            // simple heuristic: kalau nama mengandung mindray dan data mengandung MSH
            if strings.Contains(strings.ToLower(e.Name()), "mindray") && strings.Contains(string(data), "MSH") {
                matches = append(matches, Match{
                    ProfileID: e.Name(),
                    MatchLevel: "PARTIAL",
                    Score: 0.6,
                    Reason: "nama mindray mirip",
                })
            }
        }
    }

    if len(matches) == 0 {
        matches = append(matches, Match{
            ProfileID: "none",
            MatchLevel: "UNKNOWN",
            Score: 0.0,
            Reason: "tidak ada profile cocok di library",
        })
    }

    report := Report{
        SessionID: filepath.Base(inDir),
        Hash: hashStr,
        Matches: matches,
    }
    outPath := filepath.Join(inDir, "fingerprint_match.json")
    f, _ := os.Create(outPath)
    json.NewEncoder(f).SetIndent("", "  ").Encode(report)
    f.Close()
    fmt.Printf("[PB-10] FINGERPRINT selesai: hash=%s matches=%d\n", hashStr, len(matches))
    return report, nil
}

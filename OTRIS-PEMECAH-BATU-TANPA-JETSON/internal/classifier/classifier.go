
package classifier

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

type Report struct {
    SessionID      string  `json:"session_id"`
    ProtocolFamily string  `json:"protocol_family"` // HL7_V2, ASTM, DELIMITED, BINARY, UNKNOWN
    Confidence     float64 `json:"confidence"`
    Reason         string  `json:"reason"`
    ParserIdentity string  `json:"parser_identity"`
}

func Run(inDir string) (Report, error) {
    // baca inspection + framing
    rawPath := filepath.Join(inDir, "raw_stream.bin")
    data, err := os.ReadFile(rawPath)
    if err != nil { return Report{}, err }
    text := string(data)

    family := "UNKNOWN"
    conf := 0.3
    reason := "tidak ada pola dikenal"
    parser := "generic_unknown"

    if strings.Contains(text, "MSH|^~\\&") || strings.Contains(text, "MSH|") {
        family = "HL7_V2"
        conf = 0.95
        reason = "ditemukan MSH|^~\\& header HL7 v2"
        parser = "generic_hl7_v2"
    } else if strings.Contains(text, "H|") && strings.Contains(text, "L|") {
        family = "ASTM"
        conf = 0.85
        reason = "pola H| L| ASTM"
        parser = "generic_astm"
    } else if strings.Contains(text, "|") && strings.Count(text, "|") > 5 {
        family = "DELIMITED"
        conf = 0.7
        reason = "banyak delimiter | tapi bukan HL7"
        parser = "generic_delimited"
    } else if isBinary(data) {
        family = "BINARY"
        conf = 0.6
        reason = "binary ratio tinggi"
        parser = "generic_binary"
    }

    report := Report{
        SessionID: filepath.Base(inDir),
        ProtocolFamily: family,
        Confidence: conf,
        Reason: reason,
        ParserIdentity: parser,
    }
    outPath := filepath.Join(inDir, "protocol_classification.json")
    f, _ := os.Create(outPath)
    json.NewEncoder(f).SetIndent("", "  ").Encode(report)
    f.Close()
    fmt.Printf("[PB-09] CLASSIFIER selesai: %s (%.2f) parser=%s\n", family, conf, parser)
    return report, nil
}

func isBinary(data []byte) bool {
    bin := 0
    for _, b := range data {
        if b < 32 && b != 10 && b != 13 && b != 9 { bin++ }
    }
    return float64(bin)/float64(len(data)) > 0.3
}

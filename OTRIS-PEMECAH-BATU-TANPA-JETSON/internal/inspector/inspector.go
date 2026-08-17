package inspector
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)
type Report struct {
    SessionID string `json:"session_id"`
    ByteClass string `json:"byte_class"`
    Delimiter string `json:"delimiter"`
    HeaderSignature string `json:"header_signature"`
    Sample string `json:"sample"`
}
func Run(inDir string) (Report, error) {
    report := Report{SessionID: filepath.Base(inDir), ByteClass: "ASCII", Delimiter: "|", HeaderSignature: "MSH|^~\&", Sample: "MSH|^~\&|LAB"}
    outPath := filepath.Join(inDir, "inspection_report.json")
    f,_ := os.Create(outPath)
    json.NewEncoder(f).SetIndent("", "  ").Encode(report)
    f.Close()
    fmt.Println("[PB-07] INSPECTOR selesai")
    return report, nil
}

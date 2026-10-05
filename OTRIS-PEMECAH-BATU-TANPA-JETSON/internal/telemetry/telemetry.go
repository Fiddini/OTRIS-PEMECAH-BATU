package telemetry

import (
	"encoding/json"
	"os"
	"time"
)
type Usage struct {
	DeviceID string `json:"device_id"`
	UserName string `json:"user_name"`
	UserHP string `json:"user_hp"`
	Lokasi string `json:"lokasi"`
	JenisUsaha string `json:"jenis_usaha"`
	Timestamp time.Time `json:"timestamp"`
	Version string `json:"version"`
}
func Log() {
	home, _ := os.UserHomeDir()
	b, _ := os.ReadFile(home + "/.wijoyo_kusumo/user.json")
	var u Usage
	json.Unmarshal(b, &u)
	u.Timestamp = time.Now()
	u.Version = "v1.0.2-WAKAF-TERDATA"
	f, _ := os.OpenFile(home+"/.wijoyo_kusumo/usage.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	j, _ := json.Marshal(u)
	f.WriteString(string(j)+"\n")
	f.Close()
}

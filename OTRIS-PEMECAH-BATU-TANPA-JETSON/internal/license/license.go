package license

import (
	"fmt"
	"os"
	"time"
)
func CheckMaintenance() {
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(home + "/.wijoyo_kusumo/license")
	if err != nil { return }
	exp, _ := time.Parse("2006-01-02", string(b[:10]))
	if time.Now().After(exp) {
		fmt.Printf("🔴 MASA RAWAT HABIS %s - Hubungi OTRIS untuk perpanjangan\n", exp.Format("2006-01-02"))
	}
}
func SetExpiry(days int) {
	home, _ := os.UserHomeDir()
	exp := time.Now().AddDate(0,0,days).Format("2006-01-02")
	os.WriteFile(home+"/.wijoyo_kusumo/license", []byte(exp), 0644)
}

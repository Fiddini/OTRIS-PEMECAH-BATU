package auth

import (
	"fmt"
	"os"
	"strings"
)
const SecretEnv = "WIJOYO_KUSUMO_KEY"
const ExpectedKey = "WAKAF-WIJOYO-KUSUMO-2026-PEKANBARU" // GANTI INI piak - hanya ambo tau

func Check() {
	key := os.Getenv(SecretEnv)
	if key == "" {
		home, _ := os.UserHomeDir()
		b, _ := os.ReadFile(home + "/.wijoyo_kusumo/key")
		key = strings.TrimSpace(string(b))
	}
	if key != ExpectedKey {
		fmt.Println("🔒 WIJOYO_KUSUMO TERKUNCI - WAKAF BERKELANJUTAN")
		fmt.Println("Milik Hanif Ahdi Fiddini - OTRIS Pekanbaru")
		fmt.Println("Butuh konfirmasi ke HP ambo.")
		fmt.Printf("export %s=KUNCI_ANDA\n", SecretEnv)
		os.Exit(1)
	}
}

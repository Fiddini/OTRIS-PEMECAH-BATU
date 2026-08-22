package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// CLI paling dasar, tanpa deps mahal - sesuai janji private dulu ke Habib
	var (
		showVersion = flag.Bool("version", false, "tampilkan versi fondasi")
		showHelp    = flag.Bool("help", false, "bantuan")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("pemecah-batu foundation v0.1.0 - Tahap 1")
		fmt.Println("canonical: otris.canonical/v0.1")
		fmt.Println("status: private - belum open source, sesuai sepakat Habib")
		return
	}

	if *showHelp || flag.NArg() == 0 {
		fmt.Println(`pemecah-batu - Fondasi OTRIS & Wijoyo Kusumo

Penggunaan:
  pemecah-batu --version
  pemecah-batu validate <file.json>  (Tahap 2 nanti)
  pemecah-batu run                  (Tahap 6-7 nanti)

Tahap Fondasi:
  1. Modul Go dan CLI dasar [SELESAI]
  2. Kontrak kanonik OTRIS v0.1 + validator
  3. Konstitusi dan state machine
  4. Artifact immutable hash-chain
  5. Penyimpanan atomik content-address
  6. Kontrak pipeline Engine 1-12
  7. Deployment Jetson Orin Nano

Untuk sekarang private dulu sampai terbukti jalan - sesuai arahan Habib.`)
		return
	}

	cmd := flag.Arg(0)
	switch cmd {
	case "validate":
		fmt.Println("Tahap 2 belum aktif - canonical.go sudah siap, validator menyusul")
		os.Exit(1)
	case "run":
		fmt.Println("Tahap 6-7 belum aktif - tunggu konstitusi & state machine")
		os.Exit(1)
	default:
		fmt.Printf("perintah tidak dikenal: %s\n", cmd)
		fmt.Println("pakai --help untuk lihat perintah")
		os.Exit(1)
	}
}
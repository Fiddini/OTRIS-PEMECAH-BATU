package capture

// PB-06 CAPTURE ENGINE - Tukang Rekam
// Simpan apa adanya, raw_stream.bin adalah kebenaran

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/buffer"
	"github.com/otris-id/otris-pemecah-batu/internal/conditioner"
	"github.com/otris-id/otris-pemecah-batu/internal/ingress"
	"github.com/otris-id/otris-pemecah-batu/internal/model"
	"github.com/otris-id/otris-pemecah-batu/internal/timestamp"
	"github.com/otris-id/otris-pemecah-batu/internal/validator"
)

type Config struct {
	Ingress ingress.Config
	OutDir  string
}

type Manifest struct {
	SessionID      string    `json:"session_id"`
	StartedAtUTC   time.Time `json:"started_at_utc"`
	EndedAtUTC     time.Time `json:"ended_at_utc"`
	Transport      string    `json:"transport"`
	ListenAddr     string    `json:"listen_addr"`
	TotalBytes     int64     `json:"total_bytes"`
	TotalChunks    int64     `json:"total_chunks"`
	SHA256         string    `json:"sha256_raw_stream"`
	Truncated      bool      `json:"truncated"`
	ValidatorMetrics model.ValidatorMetrics `json:"validator_metrics"`
}

func Run(cfg Config, stopAfter time.Duration) error {
	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return err
	}

	rawPath := filepath.Join(cfg.OutDir, "raw_stream.bin")
	metaPath := filepath.Join(cfg.OutDir, "meta.json")
	manifestPath := filepath.Join(cfg.OutDir, "manifest.json")
	logPath := filepath.Join(cfg.OutDir, "connection.log")

	rawFile, err := os.Create(rawPath)
	if err != nil {
		return err
	}
	defer rawFile.Close()

	logFile, _ := os.Create(logPath)
	defer logFile.Close()

	// Init engines
	ing := ingress.New(cfg.Ingress)
	cond := conditioner.New()
	tsNorm := timestamp.New()
	buf := buffer.New(1000)
	val := validator.New()

	ctx := context.Background()
	ch, err := ing.Listen(ctx)
	if err != nil {
		return err
	}

	// Pipeline
	go func() {
		for raw := range ch {
			c := cond.Process(raw)
			t := tsNorm.Process(c)
			buf.Write(t)
		}
	}()

	var totalBytes int64
	var totalChunks int64
	hasher := sha256.New()
	start := time.Now().UTC()

	fmt.Printf("[PB-06] Capture mulai -> %s\n", cfg.OutDir)
	fmt.Printf("[PB-06] Tekan Ctrl+C untuk stop, atau tunggu %s\n", stopAfter)

	timeout := time.After(stopAfter)

loop:
	for {
		select {
		case <-timeout:
			fmt.Printf("\n[PB-06] Timeout %s tercapai, stop capture\n", stopAfter)
			break loop
		case chunk, ok := <-buf.Read():
			if !ok {
				break loop
			}
			validated := val.Process(chunk)
			if !validated.Valid {
				fmt.Fprintf(logFile, "[DROP] %s reason=%s\n", validated.WallUTC.Format(time.RFC3339), validated.DropReason)
				continue
			}
			// Tulis apa adanya ke raw_stream.bin
			n, _ := rawFile.Write(validated.Bytes)
			hasher.Write(validated.Bytes)
			totalBytes += int64(n)
			totalChunks++
			fmt.Fprintf(logFile, "[PASS] %s bytes=%d suspect=%v source=%s\n", validated.WallUTC.Format(time.RFC3339), len(validated.Bytes), validated.Suspect, validated.Source)
		}
	}

	end := time.Now().UTC()
	hashStr := hex.EncodeToString(hasher.Sum(nil))

	manifest := Manifest{
		SessionID: filepath.Base(cfg.OutDir),
		StartedAtUTC: start,
		EndedAtUTC: end,
		Transport: cfg.Ingress.Transport,
		ListenAddr: cfg.Ingress.ListenAddr,
		TotalBytes: totalBytes,
		TotalChunks: totalChunks,
		SHA256: hashStr,
		Truncated: false,
		ValidatorMetrics: val.Metrics(),
	}

	// tulis manifest
	mf, _ := os.Create(manifestPath)
	enc := json.NewEncoder(mf)
	enc.SetIndent("", "  ")
	enc.Encode(manifest)
	mf.Close()

	// tulis meta
	meta := map[string]interface{}{
		"session_id": manifest.SessionID,
		"transport": cfg.Ingress.Transport,
		"listen_addr": cfg.Ingress.ListenAddr,
		"started_at": start,
		"note": "raw_stream.bin adalah kebenaran, jangan diubah",
	}
	mf2, _ := os.Create(metaPath)
	enc2 := json.NewEncoder(mf2)
	enc2.SetIndent("", "  ")
	enc2.Encode(meta)
	mf2.Close()

	fmt.Printf("[PB-06] SELESAI\n")
	fmt.Printf("  File: %s\n", rawPath)
	fmt.Printf("  Bytes: %d Chunks: %d SHA256: %s\n", totalBytes, totalChunks, hashStr[:16]+"...")
	fmt.Printf("  Manifest: %s\n", manifestPath)
	fmt.Printf("  Metrics: %+v\n", val.Metrics())

	return nil
}

// helper agar tidak import context berat di contoh

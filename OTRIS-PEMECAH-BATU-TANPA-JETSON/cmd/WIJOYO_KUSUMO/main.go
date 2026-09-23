
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "time"

    "github.com/otris-id/otris-pemecah-batu/internal/auth"
	"github.com/otris-id/otris-pemecah-batu/internal/capture"
	"github.com/otris-id/otris-pemecah-batu/internal/license"
	"github.com/otris-id/otris-pemecah-batu/internal/telemetry"
    "github.com/otris-id/otris-pemecah-batu/internal/classifier"
    "github.com/otris-id/otris-pemecah-batu/internal/differential"
    "github.com/otris-id/otris-pemecah-batu/internal/evidence"
    "github.com/otris-id/otris-pemecah-batu/internal/fingerprint"
    "github.com/otris-id/otris-pemecah-batu/internal/framing"
    "github.com/otris-id/otris-pemecah-batu/internal/ingress"
    "github.com/otris-id/otris-pemecah-batu/internal/inspector"
    "github.com/otris-id/otris-pemecah-batu/internal/mapping"
    "github.com/otris-id/otris-pemecah-batu/internal/profilebuilder"
    "github.com/otris-id/otris-pemecah-batu/internal/replay"
    "github.com/spf13/cobra"
)

func main() {
	auth.Check()
	license.CheckMaintenance()
    rootCmd := &cobra.Command{
        Use: "WIJOYO_KUSUMO",
        Short: "OTRIS WIJOYO KUSUMO - Universal Integration Workbench (Wakaf) PB-01 s/d PB-13",
        Long: `WIJOYO KUSUMO: dari RAW BYTES jadi profile.yaml yang bisa di-share.

Hukum:
- UNKNOWN JANGAN DITEBAK
- CHANGES_WITH_HR bukan IS_HR
- false_data harus 0`,
    }

    // PB-06 CAPTURE
    captureCmd := &cobra.Command{
        Use:   "capture",
        Short: "PB-06: Capture raw bytes (passive first)",
        Run: func(cmd *cobra.Command, args []string) {
            transport, _ := cmd.Flags().GetString("transport")
            listen, _ := cmd.Flags().GetString("listen")
            target, _ := cmd.Flags().GetString("target")
            out, _ := cmd.Flags().GetString("out")
            mode, _ := cmd.Flags().GetString("mode")
            dur, _ := cmd.Flags().GetDuration("duration")
            file, _ := cmd.Flags().GetString("file")
            port, _ := cmd.Flags().GetString("port")
            baud, _ := cmd.Flags().GetInt("baud")
            if (mode == "serial" || transport == "serial") && file == "" {
                file = port
            }
            if out == "" {
                out = fmt.Sprintf("captures/session_%d", time.Now().Unix())
            }
            cfg := capture.Config{
                OutDir: out,
                Ingress: ingress.Config{
                    Mode: mode, Transport: transport, ListenAddr: listen, TargetAddr: target, FilePath: file,
                    SerialPort: port, BaudRate: baud,
                },
            }
            if err := capture.Run(cfg, dur); err != nil {
                fmt.Printf("Error: %v\n", err)
                os.Exit(1)
            }
        },
    }
    captureCmd.Flags().String("transport", "tcp", "tcp|serial|file")
    captureCmd.Flags().String("mode", "passive", "passive|active|file|serial")
    captureCmd.Flags().String("listen", "0.0.0.0:4000", "listen addr")
    captureCmd.Flags().String("target", "192.168.1.50:4000", "target addr")
    captureCmd.Flags().String("file", "", "file path untuk file mode ATAU port path untuk serial")
    captureCmd.Flags().String("port", "/dev/ttyUSB0", "serial port")
    captureCmd.Flags().Int("baud", 9600, "baudrate")
    captureCmd.Flags().String("out", "", "output dir")
    captureCmd.Flags().Duration("duration", 30*time.Second, "durasi capture")

    inspectCmd := &cobra.Command{
        Use:   "inspect [dir]",
        Short: "PB-07: Inspector",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            _, err := inspector.Run(args[0])
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }

    framingCmd := &cobra.Command{
        Use:   "framing [dir]",
        Short: "PB-08: Framing Detector",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            _, err := framing.Run(args[0])
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }

    classifyCmd := &cobra.Command{
        Use:   "classify [dir]",
        Short: "PB-09: Protocol Classifier",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            _, err := classifier.Run(args[0])
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }

    fingerprintCmd := &cobra.Command{
        Use:   "fingerprint [dir]",
        Short: "PB-10: Fingerprint Engine",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            lib, _ := cmd.Flags().GetString("library")
            _, err := fingerprint.Run(args[0], lib)
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }
    fingerprintCmd.Flags().String("library", "profiles", "library dir")

    diffCmd := &cobra.Command{
        Use:   "differential [dirA] [dirB]",
        Short: "PB-11: Differential Analyzer - buktikan CHANGES_WITH_HR",
        Args: cobra.ExactArgs(2),
        Run: func(cmd *cobra.Command, args []string) {
            what, _ := cmd.Flags().GetString("what")
            _, err := differential.Run(args[0], args[1], what)
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }
    diffCmd.Flags().String("what", "", "what_changed.yaml path")

    correlateCmd := &cobra.Command{
        Use:   "correlate [dir]",
        Short: "PB-11: Multi-sample correlation",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            field, _ := cmd.Flags().GetInt("field")
            _, err := differential.Correlate(args[0], field)
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }
    correlateCmd.Flags().Int("field", 2, "field position")

    evidenceCmd := &cobra.Command{
        Use:   "evidence-gate [dir]",
        Short: "PB-12: Evidence Gate",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            ev, _ := cmd.Flags().GetString("evidence")
            _, err := evidence.Run(args[0], ev)
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }
    evidenceCmd.Flags().String("evidence", "", "evidence.json path")

    replayCmd := &cobra.Command{
        Use:   "replay [dir]",
        Short: "PB-12: Replay Lab 1000x",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            count, _ := cmd.Flags().GetInt("count")
            _, err := replay.Run(args[0], count)
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }
    replayCmd.Flags().Int("count", 1000, "replay count")

    buildCmd := &cobra.Command{
        Use:   "build-profile [dir]",
        Short: "PB-13: Profile Builder -> profile.yaml",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            out, _ := cmd.Flags().GetString("out")
            ver, _ := cmd.Flags().GetString("version")
            if out == "" {
                out = fmt.Sprintf("profiles/experimental/%s/v%s", args[0], ver)
            }
            _, err := profilebuilder.Build(args[0], out, ver)
            if err != nil { fmt.Println(err); os.Exit(1) }
        },
    }
    buildCmd.Flags().String("out", "", "output dir")
    buildCmd.Flags().String("version", "1.0.0", "version")

    // GOVERNED PIPELINE - orchestrates ReviewEvidence → ReviewReplay → BuildFromMappingDefinition
    governedPipelineCmd := &cobra.Command{
        Use:   "governed-pipeline",
        Short: "D3G: Governed integration profile from explicit MappingDefinition",
        Long: `Governed pipeline: MappingDefinition CANDIDATE → ReviewEvidence → ReviewReplay → BuildFromMappingDefinition → Integration Profile Candidate.

Requires explicit MappingDefinition JSON file (CANDIDATE status only; authority restored fresh via ReviewEvidence and ReviewReplay).

All flags are mandatory except --count (default 1000).`,
        Run: func(cmd *cobra.Command, args []string) {
            mappingPath, _ := cmd.Flags().GetString("mapping")
            evidencePath, _ := cmd.Flags().GetString("evidence")
            inputDir, _ := cmd.Flags().GetString("input")
            groundTruthPath, _ := cmd.Flags().GetString("ground-truth")
            outDir, _ := cmd.Flags().GetString("out")
            version, _ := cmd.Flags().GetString("version")
            count, _ := cmd.Flags().GetInt("count")
            manufacturer, _ := cmd.Flags().GetString("manufacturer")
            model, _ := cmd.Flags().GetString("model")
            transport, _ := cmd.Flags().GetString("transport")
            firmwareTested, _ := cmd.Flags().GetString("firmware")

            // Validate required flags
            for name, val := range map[string]string{
                "--mapping": mappingPath,
                "--evidence": evidencePath,
                "--input": inputDir,
                "--ground-truth": groundTruthPath,
                "--out": outDir,
                "--version": version,
                "--manufacturer": manufacturer,
                "--model": model,
                "--transport": transport,
                "--firmware": firmwareTested,
            } {
                if val == "" {
                    fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: missing required flag %s\n", name)
                    os.Exit(1)
                }
            }
            if count <= 0 {
                count = 1000
            }

            fmt.Println("=== D3G GOVERNED PIPELINE ===")

            // Step 1: Load MappingDefinition from JSON (CANDIDATE status only)
            fmt.Printf("[D3G-1] Loading MappingDefinition from %s\n", mappingPath)
            mappingData, err := os.ReadFile(mappingPath)
            if err != nil {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: cannot read mapping file: %v\n", err)
                os.Exit(1)
            }

            m := &mapping.MappingDefinition{}
            if err := json.Unmarshal(mappingData, m); err != nil {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: invalid MappingDefinition JSON: %v\n", err)
                os.Exit(1)
            }

            status, err := m.Status()
            if err != nil || status != mapping.StatusCandidate {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: MappingDefinition must be CANDIDATE, got %s: %v\n", status, err)
                os.Exit(1)
            }
            fmt.Printf("[D3G-1] MappingDefinition loaded: %s/%s CANDIDATE\n", m.MappingIdentity, m.MappingVersion)

            // Step 2: ReviewEvidence - request fresh evidence approval
            fmt.Printf("[D3G-2] Reviewing evidence from %s\n", evidencePath)
            evidenceReportPath := inputDir + "/governed_evidence_report.json"
            if err := mapping.ReviewEvidence(m, evidencePath, evidenceReportPath); err != nil {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: evidence review rejected: %v\n", err)
                os.Exit(1)
            }

            // Verify status transitioned to EVIDENCE_APPROVED
            status, err = m.Status()
            if err != nil || status != mapping.StatusEvidenceApproved {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: evidence approval did not transition status, got %s: %v\n", status, err)
                os.Exit(1)
            }
            fmt.Printf("[D3G-2] Evidence approved: %s\n", evidenceReportPath)

            // Step 3: ReviewReplay - request fresh replay measurement and validation
            fmt.Printf("[D3G-3] Replaying claim from %s with ground truth %s (%d iterations)\n", inputDir, groundTruthPath, count)
            replayReportPath := inputDir + "/governed_replay_report.json"
            replayOpts := replay.ReplayOptions{GroundTruthPath: groundTruthPath}
            replayReport, err := mapping.ReviewReplay(m, inputDir, count, replayOpts, replayReportPath)
            if err != nil {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: replay validation rejected: %v\n", err)
                os.Exit(1)
            }

            // Verify status transitioned to REPLAY_VALIDATED and report verdict
            status, err = m.Status()
            if err != nil || status != mapping.StatusReplayValidated {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: replay validation did not transition status, got %s: %v\n", status, err)
                os.Exit(1)
            }
            if replayReport.Verdict != "READY_FOR_PROFILE_BUILDER" {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: replay verdict is %s, expected READY_FOR_PROFILE_BUILDER\n", replayReport.Verdict)
                os.Exit(1)
            }
            fmt.Printf("[D3G-3] Replay validated: %s (false_data=%.1f)\n", replayReportPath, replayReport.FalseDataRate)

            // Step 4: BuildFromMappingDefinition - construct Integration Profile Candidate
            fmt.Printf("[D3G-4] Building Integration Profile Candidate\n")
            profile, err := profilebuilder.BuildFromMappingDefinition(m, manufacturer, model, transport, firmwareTested, outDir, version)
            if err != nil {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: profile build rejected: %v\n", err)
                os.Exit(1)
            }

            // Verify output is CANDIDATE
            if profile.Status != "CANDIDATE" || profile.ValidationStatus != "CANDIDATE" {
                fmt.Printf("[D3G] GOVERNED PIPELINE FAILED: output status is not CANDIDATE (got %s/%s)\n", profile.Status, profile.ValidationStatus)
                os.Exit(1)
            }

            fmt.Println("=== D3G GOVERNED PIPELINE SUCCEEDED ===")
            fmt.Printf("  Integration Profile Candidate: %s/profile.yaml\n", outDir)
            fmt.Printf("  Status: CANDIDATE (requires engineer approval for next stage)\n")
            fmt.Printf("  Authority chain:\n")
            for _, auth := range profile.AuthorityChain {
                fmt.Printf("    - %s\n", auth)
            }
        },
    }
    governedPipelineCmd.Flags().String("mapping", "", "path to MappingDefinition JSON (CANDIDATE status only)")
    governedPipelineCmd.Flags().String("evidence", "", "path to evidence data file")
    governedPipelineCmd.Flags().String("input", "", "input directory containing raw session data")
    governedPipelineCmd.Flags().String("ground-truth", "", "path to ground truth expectations JSON")
    governedPipelineCmd.Flags().String("out", "", "output directory for profile.yaml")
    governedPipelineCmd.Flags().String("version", "", "profile version (e.g., 1.0.0)")
    governedPipelineCmd.Flags().String("manufacturer", "", "device manufacturer")
    governedPipelineCmd.Flags().String("model", "", "device model")
    governedPipelineCmd.Flags().String("transport", "", "transport protocol (tcp|serial|etc)")
    governedPipelineCmd.Flags().String("firmware", "", "firmware version tested")
    governedPipelineCmd.Flags().Int("count", 1000, "replay iteration count")

    // full pipeline
    pipelineCmd := &cobra.Command{
        Use:   "pipeline [dir]",
        Short: "Jalanin PB-07 s/d PB-13 sekaligus",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            dir := args[0]
            fmt.Println("=== PIPELINE PB-07 s/d PB-13 ===")
            inspector.Run(dir)
            framing.Run(dir)
            classifier.Run(dir)
            fingerprint.Run(dir, "profiles")
            evidence.Run(dir, "")
            replay.Run(dir, 1000)
            out := fmt.Sprintf("profiles/experimental/%s/v1.0.0", dir)
            profilebuilder.Build(dir, out, "1.0.0")
            fmt.Println("=== PIPELINE SELESAI ===")
            fmt.Printf("Profile di: %s/profile.yaml\n", out)
        },
    }

    rootCmd.AddCommand(captureCmd, inspectCmd, framingCmd, classifyCmd, fingerprintCmd, diffCmd, correlateCmd, evidenceCmd, replayCmd, buildCmd, governedPipelineCmd, pipelineCmd)

    
    var registerCmd = &cobra.Command{
        Use: "register",
        Short: "Daftar WAKAF WIJOYO KUSUMO - untuk TK Cia & kuliah Lacita",
        Run: func(cmd *cobra.Command, args []string) {
            nama, _ := cmd.Flags().GetString("nama")
            hp, _ := cmd.Flags().GetString("hp")
            lokasi, _ := cmd.Flags().GetString("lokasi")
            usaha, _ := cmd.Flags().GetString("usaha")
            home, _ := os.UserHomeDir()
            os.MkdirAll(home+"/.wijoyo_kusumo", 0755)
            data := fmt.Sprintf(`{"nama":"%s","hp":"%s","lokasi":"%s","usaha":"%s","tgl":"%s"}`, nama, hp, lokasi, usaha, time.Now().Format("2006-01-02"))
            os.WriteFile(home+"/.wijoyo_kusumo/user.json", []byte(data), 0644)
            license.SetExpiry(30)
            fmt.Printf("✅ WIJOYO KUSUMO terdaftar: %s - %s\n", nama, lokasi)
            telemetry.Log()
        },
    }
    registerCmd.Flags().String("nama", "", "")
    registerCmd.Flags().String("hp", "", "")
    registerCmd.Flags().String("lokasi", "", "")
    registerCmd.Flags().String("usaha", "", "")
    rootCmd.AddCommand(registerCmd)

	if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

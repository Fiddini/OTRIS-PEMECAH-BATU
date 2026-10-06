
package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "time"

    "github.com/otris-id/otris-pemecah-batu/internal/approval"
    "github.com/otris-id/otris-pemecah-batu/internal/auth"
	"github.com/otris-id/otris-pemecah-batu/internal/candidate"
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
    "github.com/otris-id/otris-pemecah-batu/internal/qc"
    "github.com/otris-id/otris-pemecah-batu/internal/registry"
    "github.com/otris-id/otris-pemecah-batu/internal/replay"
    "github.com/spf13/cobra"
)

func main() {
	auth.Check()
	license.CheckMaintenance()
    if err := newRootCommand().Execute(); err != nil {
        os.Exit(1)
    }
}

func newRootCommand() *cobra.Command {
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

Profile flags are mandatory except --count (default 1000).
Admission is optional: supplying any of --approver, --decision, --issuer-key,
--issuer-key-id or --sequence requires all five. Decision must be APPROVE or REJECT.
Admission requires a new --out directory; approval.json is audit data only.`,
        RunE: func(cmd *cobra.Command, args []string) (runErr error) {
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

            // D3J governed handoff: supplying any admission flag makes all of them
            // mandatory. With none supplied, D3G behavior below is unchanged.
            approverFlag, _ := cmd.Flags().GetString("approver")
            decisionFlag, _ := cmd.Flags().GetString("decision")
            issuerKeyFlag, _ := cmd.Flags().GetString("issuer-key")
            issuerKeyIDFlag, _ := cmd.Flags().GetString("issuer-key-id")
            sequenceFlag, _ := cmd.Flags().GetUint64("sequence")
            admission := admissionInputs{
                approver: approverFlag, decision: decisionFlag, issuerKeyPath: issuerKeyFlag,
                issuerKeyID: issuerKeyIDFlag, sequence: sequenceFlag,
            }
            admissionMode := false
            for _, name := range []string{"approver", "decision", "issuer-key", "issuer-key-id", "sequence"} {
                admissionMode = admissionMode || cmd.Flags().Changed(name)
            }
            if admissionMode {
                if err := admission.validate(); err != nil {
                    return fmt.Errorf("[D3J] GOVERNED HANDOFF FAILED: %w", err)
                }
                if err := requireNewOutput(outDir); err != nil {
                    return fmt.Errorf("[D3J] GOVERNED HANDOFF FAILED: %w", err)
                }
            }

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
                    return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: missing required flag %s", name)
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
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: cannot read mapping file: %w", err)
            }

            m := &mapping.MappingDefinition{}
            if err := json.Unmarshal(mappingData, m); err != nil {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: invalid MappingDefinition JSON: %w", err)
            }

            status, err := m.Status()
            if err != nil || status != mapping.StatusCandidate {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: MappingDefinition must be CANDIDATE, got %s: %v", status, err)
            }
            fmt.Printf("[D3G-1] MappingDefinition loaded: %s/%s CANDIDATE\n", m.MappingIdentity, m.MappingVersion)

            // Step 2: ReviewEvidence - request fresh evidence approval
            fmt.Printf("[D3G-2] Reviewing evidence from %s\n", evidencePath)
            evidenceReportPath := inputDir + "/governed_evidence_report.json"
            if err := mapping.ReviewEvidence(m, evidencePath, evidenceReportPath); err != nil {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: evidence review rejected: %w", err)
            }

            // Verify status transitioned to EVIDENCE_APPROVED
            status, err = m.Status()
            if err != nil || status != mapping.StatusEvidenceApproved {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: evidence approval did not transition status, got %s: %v", status, err)
            }
            fmt.Printf("[D3G-2] Evidence approved: %s\n", evidenceReportPath)

            // Step 3: ReviewReplay - request fresh replay measurement and validation
            fmt.Printf("[D3G-3] Replaying claim from %s with ground truth %s (%d iterations)\n", inputDir, groundTruthPath, count)
            replayReportPath := inputDir + "/governed_replay_report.json"
            replayOpts := replay.ReplayOptions{GroundTruthPath: groundTruthPath}
            replayReport, err := mapping.ReviewReplay(m, inputDir, count, replayOpts, replayReportPath)
            if err != nil {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: replay validation rejected: %w", err)
            }

            // Verify status transitioned to REPLAY_VALIDATED and report verdict
            status, err = m.Status()
            if err != nil || status != mapping.StatusReplayValidated {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: replay validation did not transition status, got %s: %v", status, err)
            }
            if replayReport.Verdict != "READY_FOR_PROFILE_BUILDER" {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: replay verdict is %s, expected READY_FOR_PROFILE_BUILDER", replayReport.Verdict)
            }
            fmt.Printf("[D3G-3] Replay validated: %s (false_data=%.1f)\n", replayReportPath, replayReport.FalseDataRate)

            // Step 3.5: QC Applicability Decision and Review [D3G-3.5]
            qcPolicyPath, _ := cmd.Flags().GetString("qc-policy")
            qcRefPath, _ := cmd.Flags().GetString("qc-reference")
            qcChallengesPath, _ := cmd.Flags().GetString("qc-challenges")

            qcDecision, err := ReviewQCGovernance(m, qcPolicyPath, qcRefPath, qcChallengesPath, approverFlag, version)
            if err != nil {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: QC governance failed: %w", err)
            }
            qcRecord := qcDecision.Record

            // Step 4: Build into private sibling staging only in admission mode.
            buildDir := outDir
            if admissionMode {
                stagingDir, err := newStagingDir(outDir)
                if err != nil {
                    return fmt.Errorf("[D3J] GOVERNED HANDOFF FAILED: %w", err)
                }
                // RunE returns before main exits, so cleanup runs on every failure.
                defer func() {
                    if runErr == nil {
                        return // Successful rename already consumed staging.
                    }
                    if err := os.RemoveAll(stagingDir); err != nil {
                        runErr = errors.Join(runErr, fmt.Errorf("cleanup staging: %w", err))
                    }
                }()
                buildDir = stagingDir
            }
            fmt.Printf("[D3G-4] Building Integration Profile Candidate\n")
            profile, err := profilebuilder.BuildFromMappingDefinition(m, manufacturer, model, transport, firmwareTested, buildDir, version, qcRecord)
            if err != nil {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: profile build rejected: %w", err)
            }

            // Verify output is CANDIDATE
            if profile.Status != "CANDIDATE" || profile.ValidationStatus != "CANDIDATE" {
                return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: output status is not CANDIDATE (got %s/%s)", profile.Status, profile.ValidationStatus)
            }

            // Step 5: Live approval, signed admission, self-verification, publication.
            if admissionMode {
                fmt.Printf("[D3J-5] Engineer approval and signed registry admission (%s)\n", admission.decision)
                if err := runGovernedAdmission(&profile, m, admission, buildDir, outDir, qcRecord, qcDecision.Applicability); err != nil {
                    return fmt.Errorf("[D3J] GOVERNED HANDOFF FAILED: %w", err)
                }
                fmt.Println("=== D3J GOVERNED HANDOFF SUCCEEDED ===")
                fmt.Printf("  Published: %s\n", outDir)
                fmt.Printf("  Candidate: CANDIDATE/CANDIDATE (approval and admission are separate artifacts)\n")
                if qcRecord != nil {
                    fmt.Printf("  QC: applicability=%s, verdict=%s, digest=%s\n", qcDecision.Applicability, qcRecord.Verdict, qcRecord.Integrity[:16])
                } else if qcDecision.Applicability != "" {
                    fmt.Printf("  QC: applicability=%s (no record)\n", qcDecision.Applicability)
                }
                fmt.Printf("  Approval: %s by %s (approval.json is audit only)\n", approval.StatusEngineerApproved, admission.approver)
                fmt.Printf("  Registry: %s sequence %d issuer %s\n", registry.StatusRegistryAdmitted, admission.sequence, admission.issuerKeyID)
                return nil
            }

            fmt.Println("=== D3G GOVERNED PIPELINE SUCCEEDED ===")
            fmt.Printf("  Integration Profile Candidate: %s/profile.yaml\n", outDir)
            if qcRecord != nil {
                fmt.Printf("  QC: applicability=%s, verdict=%s, digest=%s\n", qcDecision.Applicability, qcRecord.Verdict, qcRecord.Integrity[:16])
            } else if qcDecision.Applicability != "" {
                fmt.Printf("  QC: applicability=%s (no record)\n", qcDecision.Applicability)
            }
            fmt.Printf("  Status: CANDIDATE (requires engineer approval for next stage)\n")
            fmt.Printf("  Authority chain:\n")
            for _, auth := range profile.AuthorityChain {
                fmt.Printf("    - %s\n", auth)
            }
            return nil
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
    // QC governance flags (optional, define QC applicability)
    governedPipelineCmd.Flags().String("qc-policy", "", "path to QC acceptance policy JSON (optional, defines REQUIRED/OPTIONAL/NOT_APPLICABLE)")
    governedPipelineCmd.Flags().String("qc-reference", "", "path to QC reference definition JSON (required if qc-policy applicability is REQUIRED)")
    governedPipelineCmd.Flags().String("qc-challenges", "", "path to QC reference challenges JSON (required if qc-policy applicability is REQUIRED)")
    // D3J admission flags. Supplying any one makes all of them mandatory; supplying
    // none preserves existing D3G behavior. Only the key PATH is accepted, never key bytes.
    governedPipelineCmd.Flags().String("approver", "", "engineer identity recorded in the approval audit")
    governedPipelineCmd.Flags().String("decision", "", "explicit engineer decision: APPROVE or REJECT")
    governedPipelineCmd.Flags().String("issuer-key", "", "path to registry issuer Ed25519 private key (64 raw bytes or 128 hex chars)")
    governedPipelineCmd.Flags().String("issuer-key-id", "", "registry issuer key id recorded in the attestation")
    governedPipelineCmd.Flags().Uint64("sequence", 0, "governed admission sequence (must be greater than zero)")

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

    // PB-13 build candidates from real device
    buildCandidatesCmd := &cobra.Command{
        Use:   "build-candidates [capture-dir]",
        Short: "PB-13: Build candidate claim records for real device codes (wire provenance only)",
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            captureDir := args[0]
            out, _ := cmd.Flags().GetString("out")
            if out == "" {
                out = "candidates"
            }

            fmt.Println("=== PB-13 CANDIDATE BUILDER ===")
            fmt.Printf("Building candidates from: %s\n", captureDir)
            fmt.Println("Status: wire provenance only (semantic authority OPEN)")

            candidates, err := candidate.BuildRealDeviceCandidates(captureDir)
            if err != nil {
                fmt.Printf("Error building candidates: %v\n", err)
                os.Exit(1)
            }

            outPath, err := candidate.SaveCandidatePackage(out, candidates)
            if err != nil {
                fmt.Printf("Error saving candidates: %v\n", err)
                os.Exit(1)
            }

            fmt.Printf("\n✓ Candidates saved to: %s\n", outPath)
            for code, c := range candidates {
                fmt.Printf("  [%s] %s v%s - awaiting: %v\n", code, c.MappingIdentity, c.MappingVersion, c.AwaitingAuthority)
            }
            fmt.Println("\nNext step: provide semantic authority + ground truth, then ReviewClaim → ReviewReplay → BuildFromMappingDefinition")
        },
    }
    buildCandidatesCmd.Flags().String("out", "", "output directory for candidate records")

    // PB-14 MAPPING-CREATE - build MappingDefinition CANDIDATE from explicit semantic claim
    mappingCreateCmd := &cobra.Command{
        Use:   "mapping-create",
        Short: "PB-14: Create MappingDefinition CANDIDATE from explicit semantic claim",
        Long: `Build production MappingDefinition CANDIDATE from explicit semantic fields.

Fails closed: UNKNOWN concept/unit, missing provenance, or malformed input are rejected.
Output is always CANDIDATE status; no automatic transitions.
Device/parser context is optional; provenance fields are mandatory.`,
        RunE: func(cmd *cobra.Command, args []string) error {
            // Extract all flags
            mappingID, _ := cmd.Flags().GetString("mapping-id")
            version, _ := cmd.Flags().GetString("version")
            sourceProto, _ := cmd.Flags().GetString("source-protocol")
            sourcePos, _ := cmd.Flags().GetInt("source-position")
            sourceCode, _ := cmd.Flags().GetString("source-code")
            sourceUnit, _ := cmd.Flags().GetString("source-unit")
            concept, _ := cmd.Flags().GetString("concept")
            unit, _ := cmd.Flags().GetString("unit")
            provenance, _ := cmd.Flags().GetString("provenance")
            author, _ := cmd.Flags().GetString("author")
            fingerprint, _ := cmd.Flags().GetString("fingerprint")
            deviceIdentity, _ := cmd.Flags().GetString("device-identity")
            parserID, _ := cmd.Flags().GetString("parser-id")
            outPath, _ := cmd.Flags().GetString("out")

            // Validate required flags
            for name, val := range map[string]string{
                "--mapping-id":      mappingID,
                "--version":         version,
                "--source-protocol": sourceProto,
                "--source-code":     sourceCode,
                "--concept":         concept,
                "--unit":            unit,
                "--provenance":      provenance,
                "--author":          author,
                "--out":             outPath,
            } {
                if val == "" {
                    return fmt.Errorf("[PB-14] MAPPING CREATE FAILED: missing required flag %s", name)
                }
            }
            if sourcePos <= 0 {
                return fmt.Errorf("[PB-14] MAPPING CREATE FAILED: source-position must be positive integer")
            }

            fmt.Println("=== PB-14 MAPPING CLAIM BUILDER ===")

            // Create builder (fail-closed validation of semantic fields)
            fmt.Printf("[PB-14-1] Creating builder for concept %s / unit %s\n", concept, unit)
            b, err := mapping.NewBuilder(
                mappingID, version,
                sourceProto, sourcePos, sourceCode, sourceUnit,
                concept, unit,
                provenance, author,
            )
            if err != nil {
                return fmt.Errorf("[PB-14-1] builder creation failed: %w", err)
            }

            // Add optional device context
            if fingerprint != "" {
                b.WithDeviceFingerprint(fingerprint)
            }
            if deviceIdentity != "" {
                b.WithDeviceIdentity(deviceIdentity)
            }
            if parserID != "" {
                b.WithParserIdentity(parserID)
            }

            // Build MappingDefinition CANDIDATE
            fmt.Println("[PB-14-2] Building MappingDefinition CANDIDATE")
            m, err := b.Build()
            if err != nil {
                return fmt.Errorf("[PB-14-2] build failed: %w", err)
            }

            // Validate output
            status, err := m.Status()
            if err != nil || status != mapping.StatusCandidate {
                return fmt.Errorf("[PB-14-2] build did not produce CANDIDATE, got %s: %v", status, err)
            }
            fmt.Printf("[PB-14-2] MappingDefinition CANDIDATE created: %s/%s\n", m.MappingIdentity, m.MappingVersion)

            // Export to JSON
            fmt.Printf("[PB-14-3] Writing to %s\n", outPath)
            data, err := json.MarshalIndent(m, "", "  ")
            if err != nil {
                return fmt.Errorf("[PB-14-3] marshal failed: %w", err)
            }

            if err := os.WriteFile(outPath, data, 0600); err != nil {
                return fmt.Errorf("[PB-14-3] write failed: %w", err)
            }

            fmt.Println("\n=== PB-14 MAPPING CLAIM BUILDER SUCCEEDED ===")
            fmt.Printf("  MappingDefinition: %s/%s CANDIDATE\n", m.MappingIdentity, m.MappingVersion)
            fmt.Printf("  Concept: %s → %s\n", m.SourceCode, m.CanonicalConcept)
            fmt.Printf("  Output: %s\n", outPath)
            fmt.Printf("  Next step: governed-pipeline --mapping %s (ReviewEvidence → ReviewReplay → BuildFromMappingDefinition)\n", outPath)
            return nil
        },
    }
    mappingCreateCmd.Flags().String("mapping-id", "", "mapping identity (e.g., siemens-infinity-147842-001)")
    mappingCreateCmd.Flags().String("version", "", "mapping version (e.g., 1.0.0)")
    mappingCreateCmd.Flags().String("source-protocol", "", "source protocol (HL7_V2, ASTM_H, etc.)")
    mappingCreateCmd.Flags().Int("source-position", 0, "source position (field/column number)")
    mappingCreateCmd.Flags().String("source-code", "", "source code (e.g., 147842)")
    mappingCreateCmd.Flags().String("source-unit", "", "source unit as specified in protocol")
    mappingCreateCmd.Flags().String("concept", "", "canonical concept (e.g., HR_BPM) — cannot be UNKNOWN")
    mappingCreateCmd.Flags().String("unit", "", "canonical unit (e.g., BPM) — cannot be UNKNOWN")
    mappingCreateCmd.Flags().String("provenance", "", "narrative provenance (how claim was determined)")
    mappingCreateCmd.Flags().String("author", "", "provenance author (role/person responsible for claim)")
    mappingCreateCmd.Flags().String("fingerprint", "", "device fingerprint (optional)")
    mappingCreateCmd.Flags().String("device-identity", "", "device identity reference (optional)")
    mappingCreateCmd.Flags().String("parser-id", "", "parser identity (optional)")
    mappingCreateCmd.Flags().String("out", "", "output JSON file path (exclusive create, no overwrite)")

    // PB-14 MAPPING-EXPORT - export existing MappingDefinition to stdout/file
    mappingExportCmd := &cobra.Command{
        Use:   "mapping-export",
        Short: "PB-14: Export MappingDefinition JSON to file or stdout",
        RunE: func(cmd *cobra.Command, args []string) error {
            inputPath, _ := cmd.Flags().GetString("mapping")
            outPath, _ := cmd.Flags().GetString("out")

            if inputPath == "" {
                return fmt.Errorf("[PB-14-export] missing required flag --mapping")
            }

            fmt.Printf("=== PB-14 MAPPING EXPORT ===\n")
            fmt.Printf("[PB-14-export-1] Reading %s\n", inputPath)

            data, err := os.ReadFile(inputPath)
            if err != nil {
                return fmt.Errorf("[PB-14-export-1] read failed: %w", err)
            }

            m := &mapping.MappingDefinition{}
            if err := json.Unmarshal(data, m); err != nil {
                return fmt.Errorf("[PB-14-export-1] parse failed: %w", err)
            }

            status, err := m.Status()
            if err != nil {
                return fmt.Errorf("[PB-14-export-1] validation failed: %w", err)
            }
            fmt.Printf("[PB-14-export-1] Loaded: %s/%s %s\n", m.MappingIdentity, m.MappingVersion, status)

            // Pretty-print
            fmt.Println("[PB-14-export-2] Formatting")
            formatted, err := json.MarshalIndent(m, "", "  ")
            if err != nil {
                return fmt.Errorf("[PB-14-export-2] format failed: %w", err)
            }

            // Output
            if outPath == "" {
                // stdout
                fmt.Println(string(formatted))
            } else {
                // file
                fmt.Printf("[PB-14-export-3] Writing to %s\n", outPath)
                if err := os.WriteFile(outPath, formatted, 0600); err != nil {
                    return fmt.Errorf("[PB-14-export-3] write failed: %w", err)
                }
                fmt.Printf("\n=== PB-14 MAPPING EXPORT SUCCEEDED ===\n")
                fmt.Printf("  Output: %s\n", outPath)
            }
            return nil
        },
    }
    mappingExportCmd.Flags().String("mapping", "", "input MappingDefinition JSON file")
    mappingExportCmd.Flags().String("out", "", "output file (omit for stdout)")

    rootCmd.AddCommand(captureCmd, inspectCmd, framingCmd, classifyCmd, fingerprintCmd, diffCmd, correlateCmd, evidenceCmd, replayCmd, buildCmd, governedPipelineCmd, pipelineCmd, buildCandidatesCmd, mappingCreateCmd, mappingExportCmd)
    registerGovernedCandidatePipelineCmd(rootCmd)

    
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

    return rootCmd
}

// QC Helper Functions
func loadQCPolicy(path string) (*qc.AcceptanceCriteria, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read QC policy: %w", err)
	}
	var policy qc.AcceptanceCriteria
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("parse QC policy: %w", err)
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("validate QC policy: %w", err)
	}
	return &policy, nil
}

func loadQCReference(path string) (*qc.ReferenceDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read QC reference: %w", err)
	}
	var ref qc.ReferenceDefinition
	if err := json.Unmarshal(data, &ref); err != nil {
		return nil, fmt.Errorf("parse QC reference: %w", err)
	}
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("validate QC reference: %w", err)
	}
	return &ref, nil
}

func loadQCChallenges(path string) ([]*qc.ReferenceChallenge, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read QC challenges: %w", err)
	}
	var challenges []*qc.ReferenceChallenge
	if err := json.Unmarshal(data, &challenges); err != nil {
		return nil, fmt.Errorf("parse QC challenges: %w", err)
	}
	return challenges, nil
}

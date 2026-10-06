package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/otris-id/otris-pemecah-batu/internal/approval"
	"github.com/otris-id/otris-pemecah-batu/internal/identity"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/profilebuilder"
	"github.com/otris-id/otris-pemecah-batu/internal/registry"
	"github.com/otris-id/otris-pemecah-batu/internal/replay"
	"github.com/spf13/cobra"
)

// candidatePipelineInputs carries candidate lookup and governance inputs.
type candidatePipelineInputs struct {
	// Candidate sources (lookup)
	candidateIdentityJSON string // serialized IntegrationKnowledgeIdentity

	// Current mapping source
	mappingPath string // live MappingDefinition (evidence-derived)

	// Governance inputs
	evidencePath     string
	inputDir         string
	groundTruthPath  string
	outDir           string
	version          string
	count            int
	manufacturer     string
	model            string
	transport        string
	firmwareTested   string
	qcPolicyPath     string
	qcRefPath        string
	qcChallengesPath string

	// Optional admission
	approver      string
	decision      string
	issuerKeyPath string
	issuerKeyID   string
	sequence      uint64
}

func newGovernedCandidatePipelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "governed-candidate-pipeline",
		Short: "D3G-C: Governed integration from historical candidate + current evidence",
		Long: `Governed candidate pipeline: Registry Lookup result (CandidateKnowledgeMatch) → ApplicabilityVerification → CONFIRMED → existing governance.

Requires:
- Serialized IntegrationKnowledgeIdentity (from lookup) as JSON string
- Live MappingDefinition JSON file (current evidence-derived, CANDIDATE status)
- Evidence data, input directory, ground truth expectations
- Profile flags (manufacturer, model, transport, firmware, version)

Applicability must be CONFIRMED. Rejected or insufficient candidates fail closed.
No reconstruction of MappingDefinition from historical Attestation; current mapping
must come from existing evidence gates.

Admission is optional (same as governed-pipeline): supplying any of --approver,
--decision, --issuer-key, --issuer-key-id or --sequence requires all five.`,
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			// Load inputs
			candidateIDJSON, _ := cmd.Flags().GetString("candidate-identity")
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
			qcPolicyPath, _ := cmd.Flags().GetString("qc-policy")
			qcRefPath, _ := cmd.Flags().GetString("qc-reference")
			qcChallengesPath, _ := cmd.Flags().GetString("qc-challenges")

			// Admission flags
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
					return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: %w", err)
				}
				if err := requireNewOutput(outDir); err != nil {
					return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: %w", err)
				}
			}

			// Validate required flags
			for name, val := range map[string]string{
				"--candidate-identity": candidateIDJSON,
				"--mapping":            mappingPath,
				"--evidence":           evidencePath,
				"--input":              inputDir,
				"--ground-truth":       groundTruthPath,
				"--out":                outDir,
				"--version":            version,
				"--manufacturer":       manufacturer,
				"--model":              model,
				"--transport":          transport,
				"--firmware":           firmwareTested,
			} {
				if val == "" {
					return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: missing required flag %s", name)
				}
			}
			if count <= 0 {
				count = 1000
			}

			fmt.Println("=== D3G-C GOVERNED CANDIDATE PIPELINE ===")

			// Step 1: Parse historical candidate identity
			fmt.Printf("[D3G-C-1] Parsing candidate identity\n")
			candidateIKI := &identity.IntegrationKnowledgeIdentity{}
			if err := json.Unmarshal([]byte(candidateIDJSON), candidateIKI); err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: invalid candidate identity JSON: %w", err)
			}
			if err := candidateIKI.Verify(); err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: candidate identity verification failed: %w", err)
			}
			fmt.Printf("[D3G-C-1] Candidate identity verified: mapping=%s/%s\n",
				candidateIKI.MappingIdentity, candidateIKI.MappingVersion)

			// Step 2: Load live MappingDefinition (current evidence-derived, CANDIDATE status)
			fmt.Printf("[D3G-C-2] Loading current MappingDefinition from %s\n", mappingPath)
			mappingData, err := os.ReadFile(mappingPath)
			if err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: cannot read mapping file: %w", err)
			}

			m := &mapping.MappingDefinition{}
			if err := json.Unmarshal(mappingData, m); err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: invalid MappingDefinition JSON: %w", err)
			}

			status, err := m.Status()
			if err != nil || status != mapping.StatusCandidate {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: MappingDefinition must be CANDIDATE, got %s: %v", status, err)
			}
			fmt.Printf("[D3G-C-2] Current MappingDefinition loaded: %s/%s CANDIDATE\n", m.MappingIdentity, m.MappingVersion)

			// Step 3: Verify identity match (applicability check)
			fmt.Printf("[D3G-C-3] Verifying candidate applicability\n")
			currentIKI, err := identity.New(
				m.DeviceFingerprint,
				m.MappingIdentity,
				m.MappingVersion,
				"", // ReferenceIdentity from current mapping if available
				m.SemanticEvidenceDigest,
			)
			if err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: cannot build current identity: %w", err)
			}

			// Create mock candidate for applicability check
			mockCandidate := &identity.CandidateKnowledgeMatch{
				IntegrationIdentity:       candidateIKI,
				FingerprintRelationship:   identity.RelationshipExact,
				ApplicabilityStatus:       identity.ApplicabilityNotYetEstablished,
				HistoricalAuthorityStatus: registry.StatusRegistryAdmitted,
				CandidateStatement:        "Historical knowledge from this source is candidate for new context and does not automatically grant authority",
			}

			av := registry.NewApplicabilityVerification(mockCandidate, m, currentIKI)
			if err := av.Verify(); err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: applicability verification failed: %w", err)
			}

			if !av.IsCandidateApplicable() {
				decision, reason := av.GetDecision()
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: candidate not applicable (%s): %s", decision, reason)
			}
			fmt.Printf("[D3G-C-3] Candidate applicability CONFIRMED\n")

			// Step 4: Enter existing governed pipeline (ReviewEvidence → ReviewReplay → gates)
			// This is identical to governed-pipeline from this point forward.

			fmt.Printf("[D3G-4] Reviewing evidence from %s\n", evidencePath)
			evidenceReportPath := inputDir + "/governed_evidence_report.json"
			if err := mapping.ReviewEvidence(m, evidencePath, evidenceReportPath); err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: evidence review rejected: %w", err)
			}

			status, err = m.Status()
			if err != nil || status != mapping.StatusEvidenceApproved {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: evidence approval did not transition status, got %s: %v", status, err)
			}
			fmt.Printf("[D3G-4] Evidence approved: %s\n", evidenceReportPath)

			fmt.Printf("[D3G-5] Replaying claim from %s with ground truth %s (%d iterations)\n", inputDir, groundTruthPath, count)
			replayReportPath := inputDir + "/governed_replay_report.json"
			replayOpts := replay.ReplayOptions{GroundTruthPath: groundTruthPath}
			replayReport, err := mapping.ReviewReplay(m, inputDir, count, replayOpts, replayReportPath)
			if err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: replay validation rejected: %w", err)
			}

			status, err = m.Status()
			if err != nil || status != mapping.StatusReplayValidated {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: replay validation did not transition status, got %s: %v", status, err)
			}
			if replayReport.Verdict != "READY_FOR_PROFILE_BUILDER" {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: replay verdict is %s, expected READY_FOR_PROFILE_BUILDER", replayReport.Verdict)
			}
			fmt.Printf("[D3G-5] Replay validated: %s (false_data=%.1f)\n", replayReportPath, replayReport.FalseDataRate)

			// QC Review [D3G-5.5]
			qcDecision, err := ReviewQCGovernance(m, qcPolicyPath, qcRefPath, qcChallengesPath, approverFlag, version)
			if err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: QC governance failed: %w", err)
			}
			qcRecord := qcDecision.Record

			// Build (in staging if admission mode)
			buildDir := outDir
			if admissionMode {
				stagingDir, err := newStagingDir(outDir)
				if err != nil {
					return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: %w", err)
				}
				defer func() {
					if runErr == nil {
						return
					}
					if err := os.RemoveAll(stagingDir); err != nil {
						runErr = errors.Join(runErr, fmt.Errorf("cleanup staging: %w", err))
					}
				}()
				buildDir = stagingDir
			}

			fmt.Printf("[D3G-6] Building Integration Profile Candidate\n")
			profile, err := profilebuilder.BuildFromMappingDefinition(m, manufacturer, model, transport, firmwareTested, buildDir, version, qcRecord)
			if err != nil {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: profile build rejected: %w", err)
			}

			if profile.Status != "CANDIDATE" || profile.ValidationStatus != "CANDIDATE" {
				return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: output status is not CANDIDATE (got %s/%s)", profile.Status, profile.ValidationStatus)
			}

			// Approval and admission (if enabled)
			if admissionMode {
				fmt.Printf("[D3J-7] Engineer approval and signed registry admission (%s)\n", admission.decision)
				if err := runGovernedAdmission(&profile, m, admission, buildDir, outDir, qcRecord, qcDecision.Applicability); err != nil {
					return fmt.Errorf("[D3G-C] GOVERNED CANDIDATE PIPELINE FAILED: %w", err)
				}
				fmt.Println("=== D3G-C GOVERNED CANDIDATE PIPELINE SUCCEEDED ===")
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

			fmt.Println("=== D3G-C GOVERNED CANDIDATE PIPELINE SUCCEEDED ===")
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
}

func registerGovernedCandidatePipelineCmd(rootCmd *cobra.Command) {
	cmd := newGovernedCandidatePipelineCmd()

	// Candidate source
	cmd.Flags().String("candidate-identity", "", "serialized IntegrationKnowledgeIdentity (JSON string from lookup)")

	// Mapping source
	cmd.Flags().String("mapping", "", "path to live MappingDefinition JSON (CANDIDATE status, evidence-derived)")

	// Governance inputs
	cmd.Flags().String("evidence", "", "path to evidence data file")
	cmd.Flags().String("input", "", "input directory containing raw session data")
	cmd.Flags().String("ground-truth", "", "path to ground truth expectations JSON")
	cmd.Flags().String("out", "", "output directory for profile.yaml")
	cmd.Flags().String("version", "", "profile version (e.g., 1.0.0)")
	cmd.Flags().String("manufacturer", "", "device manufacturer")
	cmd.Flags().String("model", "", "device model")
	cmd.Flags().String("transport", "", "transport protocol (tcp|serial|etc)")
	cmd.Flags().String("firmware", "", "firmware version tested")
	cmd.Flags().Int("count", 1000, "replay iteration count")

	// QC governance flags (optional)
	cmd.Flags().String("qc-policy", "", "path to QC acceptance policy JSON (optional)")
	cmd.Flags().String("qc-reference", "", "path to QC reference definition JSON (required if qc-policy REQUIRED)")
	cmd.Flags().String("qc-challenges", "", "path to QC reference challenges JSON (required if qc-policy REQUIRED)")

	// Admission flags (optional, all-or-nothing)
	cmd.Flags().String("approver", "", "engineer identity recorded in the approval audit")
	cmd.Flags().String("decision", "", "explicit engineer decision: APPROVE or REJECT")
	cmd.Flags().String("issuer-key", "", "path to registry issuer Ed25519 private key (64 raw bytes or 128 hex chars)")
	cmd.Flags().String("issuer-key-id", "", "registry issuer key id recorded in the attestation")
	cmd.Flags().Uint64("sequence", 0, "governed admission sequence (must be greater than zero)")

	rootCmd.AddCommand(cmd)
}

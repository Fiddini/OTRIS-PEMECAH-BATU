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

// reverificationPipelineInputs carries re-verification decision inputs.
type reverificationPipelineInputs struct {
	// Observation source
	observationPath string // serialized FingerprintIdentityObservation

	// Required for REVERIFY_REQUIRED path only
	candidateIdentityJSON string // serialized IntegrationKnowledgeIdentity
	mappingPath           string // live MappingDefinition (CANDIDATE status, current evidence-derived)

	// Governance inputs (required if reverification needed)
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

func newGovernedReverificationPipelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "governed-reverification-pipeline",
		Short: "D3G-REV: Governed re-verification after fingerprint identity change observation",
		Long: `Governed re-verification pipeline: FingerprintIdentityObservation → DecideFromObservation() → ReverificationRequirement.

Decision outcomes:
- NO_ACTION: No re-verification needed (INITIAL observations). Returns decision record only.
- INSUFFICIENT_EVIDENCE: Change detected but unresolved (UNKNOWN, UNDER_INVESTIGATION). Returns requirement; does NOT enter governance.
- REVERIFY_REQUIRED: Legitimate or suspicious change (confirmed UPDATE, POSSIBLE_CORRUPTION). Enters existing governance gates.

REVERIFY_REQUIRED flow:
1. Verify REVERIFY_REQUIRED decision outcome from observation
2. Load current MappingDefinition (MUST be evidence-derived, CANDIDATE status)
3. Load historical candidate identity from registry lookup
4. Re-check applicability with current identity and mapping
5. If applicable: ReviewEvidence → ReviewReplay → QC → Profile → optional Approval/Admission
6. Historical attestation remains unchanged; new decision is separate record

Critical constraints:
- Current MappingDefinition must come from existing evidence gates (not reconstructed)
- Historical authority is immutable; re-verification creates new decision pathway
- Applicability re-check is mandatory
- All existing governance gates remain required`,
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			// Load observation path
			obsPath, _ := cmd.Flags().GetString("observation")

			// Optional governance inputs
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
					return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: %w", err)
				}
				if err := requireNewOutput(outDir); err != nil {
					return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: %w", err)
				}
			}

			// Validate required observation flag
			if obsPath == "" {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: missing required flag --observation")
			}

			fmt.Println("=== D3G-REV GOVERNED REVERIFICATION PIPELINE ===")

			// Step 1: Load and parse observation
			fmt.Printf("[D3G-REV-1] Loading observation from %s\n", obsPath)
			obsData, err := os.ReadFile(obsPath)
			if err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: cannot read observation file: %w", err)
			}

			obs := &identity.FingerprintIdentityObservation{}
			if err := json.Unmarshal(obsData, obs); err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: invalid observation JSON: %w", err)
			}

			if err := obs.Verify(); err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: observation verification failed: %w", err)
			}
			fmt.Printf("[D3G-REV-1] Observation loaded: id=%s, change=%s, status=%s\n",
				obs.ObservationID, obs.ChangeType, obs.InvestigationStatus)

			// Step 2: Decide from observation (deterministic decision function)
			fmt.Printf("[D3G-REV-2] Evaluating decision from observation\n")
			decision := identity.DecideFromObservation(obs)
			fmt.Printf("[D3G-REV-2] Decision: %s\n", decision)

			// Step 3: Create immutable ReverificationRequirement record
			fmt.Printf("[D3G-REV-3] Creating re-verification requirement record\n")
			rrq, err := identity.NewReverificationRequirement(
				fmt.Sprintf("rrq-%s-%d", obs.ObservationID, obs.ObservedAt.UnixMilli()),
				obs.ObservationID,
				decision,
				fmt.Sprintf("Observation %s triggered decision %s (change=%s, confidence=%.1f)",
					obs.ObservationID, decision, obs.ChangeType, obs.Confidence),
				obs.DeviceFingerprint,
				string(obs.ChangeType),
				obs.InvestigationStatus,
			)
			if err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: cannot create requirement: %w", err)
			}
			rrq.SetObservationReference(obsPath)
			fmt.Printf("[D3G-REV-3] Requirement sealed: %s\n", rrq.RequirementID)

			// Step 4: Handle decision outcomes
			switch decision {
			case identity.ReverificationNoAction:
				fmt.Println("=== D3G-REV DECISION: NO_ACTION ===")
				fmt.Printf("  No re-verification needed (observation is INITIAL or no change detected)\n")
				fmt.Printf("  Requirement: %s\n", rrq.RequirementID)
				fmt.Printf("  Status: sealed immutable\n")
				return nil

			case identity.ReverificationInsufficientEvidence:
				fmt.Println("=== D3G-REV DECISION: INSUFFICIENT_EVIDENCE ===")
				fmt.Printf("  Identity change recorded but insufficient data to decide\n")
				fmt.Printf("  Requirement: %s\n", rrq.RequirementID)
				fmt.Printf("  Action: Operator review required; awaiting investigation completion\n")
				fmt.Printf("  Status: sealed immutable (does NOT enter governance pipeline)\n")
				return nil

			case identity.ReverificationRequired:
				// Fall through to governance pipeline entry
				fmt.Println("[D3G-REV-4] Decision: REVERIFY_REQUIRED → entering governed candidate pipeline")

			default:
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: unknown decision outcome: %s", decision)
			}

			// Step 5: REVERIFY_REQUIRED path - validate current required inputs
			fmt.Printf("[D3G-REV-5] Validating governance pipeline inputs for re-verification\n")
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
					return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: REVERIFY_REQUIRED decision requires %s", name)
				}
			}
			if count <= 0 {
				count = 1000
			}

			// Step 6: Parse historical candidate identity
			fmt.Printf("[D3G-REV-6] Parsing historical candidate identity\n")
			candidateIKI := &identity.IntegrationKnowledgeIdentity{}
			if err := json.Unmarshal([]byte(candidateIDJSON), candidateIKI); err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: invalid candidate identity JSON: %w", err)
			}
			if err := candidateIKI.Verify(); err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: candidate identity verification failed: %w", err)
			}
			fmt.Printf("[D3G-REV-6] Candidate identity verified: mapping=%s/%s\n",
				candidateIKI.MappingIdentity, candidateIKI.MappingVersion)

			// Step 7: Load current MappingDefinition (evidence-derived)
			fmt.Printf("[D3G-REV-7] Loading current MappingDefinition from %s\n", mappingPath)
			mappingData, err := os.ReadFile(mappingPath)
			if err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: cannot read mapping file: %w", err)
			}

			m := &mapping.MappingDefinition{}
			if err := json.Unmarshal(mappingData, m); err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: invalid MappingDefinition JSON: %w", err)
			}

			status, err := m.Status()
			if err != nil || status != mapping.StatusCandidate {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: MappingDefinition must be CANDIDATE, got %s: %v", status, err)
			}
			fmt.Printf("[D3G-REV-7] Current MappingDefinition loaded: %s/%s CANDIDATE\n", m.MappingIdentity, m.MappingVersion)

			// Step 8: Build current identity from MappingDefinition for re-applicability check
			fmt.Printf("[D3G-REV-8] Building current identity context\n")
			currentIKI, err := identity.New(
				m.DeviceFingerprint,
				m.MappingIdentity,
				m.MappingVersion,
				"",
				m.SemanticEvidenceDigest,
			)
			if err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: cannot build current identity: %w", err)
			}

			// Step 9: Create mock candidate for applicability re-check
			// Note: Uses historical candidate identity; does NOT fabricate current mapping
			fmt.Printf("[D3G-REV-9] Re-checking applicability with current context\n")
			mockCandidate := &identity.CandidateKnowledgeMatch{
				IntegrationIdentity:       candidateIKI,
				FingerprintRelationship:   identity.RelationshipExact,
				ApplicabilityStatus:       identity.ApplicabilityNotYetEstablished,
				HistoricalAuthorityStatus: registry.StatusRegistryAdmitted,
				CandidateStatement:        "Historical knowledge from this source is re-verified in current context; historical authority is immutable",
			}

			av := registry.NewApplicabilityVerification(mockCandidate, m, currentIKI)
			if err := av.Verify(); err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: applicability re-verification failed: %w", err)
			}

			if !av.IsCandidateApplicable() {
				decision, reason := av.GetDecision()
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: candidate not applicable in current context (%s): %s", decision, reason)
			}
			fmt.Printf("[D3G-REV-9] Applicability re-check CONFIRMED (can proceed with governance)\n")

			// Step 10: Enter existing governed pipeline (same gates as governed_candidate_pipeline)
			// This is identical from this point forward.

			fmt.Printf("[D3G-REV-10] Reviewing evidence from %s\n", evidencePath)
			evidenceReportPath := inputDir + "/governed_evidence_report.json"
			if err := mapping.ReviewEvidence(m, evidencePath, evidenceReportPath); err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: evidence review rejected: %w", err)
			}

			status, err = m.Status()
			if err != nil || status != mapping.StatusEvidenceApproved {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: evidence approval did not transition status, got %s: %v", status, err)
			}
			fmt.Printf("[D3G-REV-10] Evidence approved: %s\n", evidenceReportPath)

			fmt.Printf("[D3G-REV-11] Replaying claim from %s with ground truth %s (%d iterations)\n", inputDir, groundTruthPath, count)
			replayReportPath := inputDir + "/governed_replay_report.json"
			replayOpts := replay.ReplayOptions{GroundTruthPath: groundTruthPath}
			replayReport, err := mapping.ReviewReplay(m, inputDir, count, replayOpts, replayReportPath)
			if err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: replay validation rejected: %w", err)
			}

			status, err = m.Status()
			if err != nil || status != mapping.StatusReplayValidated {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: replay validation did not transition status, got %s: %v", status, err)
			}
			if replayReport.Verdict != "READY_FOR_PROFILE_BUILDER" {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: replay verdict is %s, expected READY_FOR_PROFILE_BUILDER", replayReport.Verdict)
			}
			fmt.Printf("[D3G-REV-11] Replay validated: %s (false_data=%.1f)\n", replayReportPath, replayReport.FalseDataRate)

			// QC Review [D3G-REV-12]
			qcDecision, err := ReviewQCGovernance(m, qcPolicyPath, qcRefPath, qcChallengesPath, approverFlag, version)
			if err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: QC governance failed: %w", err)
			}
			qcRecord := qcDecision.Record

			// Build (in staging if admission mode)
			buildDir := outDir
			if admissionMode {
				stagingDir, err := newStagingDir(outDir)
				if err != nil {
					return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: %w", err)
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

			fmt.Printf("[D3G-REV-13] Building Re-Verified Integration Profile Candidate\n")
			profile, err := profilebuilder.BuildFromMappingDefinition(m, manufacturer, model, transport, firmwareTested, buildDir, version, qcRecord)
			if err != nil {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: profile build rejected: %w", err)
			}

			if profile.Status != "CANDIDATE" || profile.ValidationStatus != "CANDIDATE" {
				return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: output status is not CANDIDATE (got %s/%s)", profile.Status, profile.ValidationStatus)
			}

			// Approval and admission (if enabled)
			if admissionMode {
				fmt.Printf("[D3G-REV-14] Engineer approval and signed registry admission (%s)\n", admission.decision)
				if err := runGovernedAdmission(&profile, m, admission, buildDir, outDir, qcRecord, qcDecision.Applicability); err != nil {
					return fmt.Errorf("[D3G-REV] GOVERNED REVERIFICATION FAILED: %w", err)
				}
				fmt.Println("=== D3G-REV GOVERNED REVERIFICATION SUCCEEDED ===")
				fmt.Printf("  Original observation: %s\n", obs.ObservationID)
				fmt.Printf("  Decision: %s\n", identity.ReverificationRequired)
				fmt.Printf("  Re-verification requirement: %s\n", rrq.RequirementID)
				fmt.Printf("  Published: %s\n", outDir)
				fmt.Printf("  Re-verified candidate: CANDIDATE/CANDIDATE\n")
				if qcRecord != nil {
					fmt.Printf("  QC: applicability=%s, verdict=%s, digest=%s\n", qcDecision.Applicability, qcRecord.Verdict, qcRecord.Integrity[:16])
				}
				fmt.Printf("  Approval: %s by %s\n", approval.StatusEngineerApproved, admission.approver)
				fmt.Printf("  Registry: %s sequence %d issuer %s\n", registry.StatusRegistryAdmitted, admission.sequence, admission.issuerKeyID)
				fmt.Printf("  Historical attestation: UNCHANGED (separate decision pathway)\n")
				return nil
			}

			fmt.Println("=== D3G-REV GOVERNED REVERIFICATION SUCCEEDED ===")
			fmt.Printf("  Original observation: %s\n", obs.ObservationID)
			fmt.Printf("  Decision: %s\n", identity.ReverificationRequired)
			fmt.Printf("  Re-verification requirement: %s\n", rrq.RequirementID)
			fmt.Printf("  Re-verified profile: %s/profile.yaml\n", outDir)
			if qcRecord != nil {
				fmt.Printf("  QC: applicability=%s, verdict=%s, digest=%s\n", qcDecision.Applicability, qcRecord.Verdict, qcRecord.Integrity[:16])
			}
			fmt.Printf("  Status: CANDIDATE (requires engineer approval for next stage)\n")
			fmt.Printf("  Historical attestation: UNCHANGED (separate decision pathway)\n")
			fmt.Printf("  Authority chain:\n")
			for _, auth := range profile.AuthorityChain {
				fmt.Printf("    - %s\n", auth)
			}
			return nil
		},
	}
}

func registerGovernedReverificationPipelineCmd(rootCmd *cobra.Command) {
	cmd := newGovernedReverificationPipelineCmd()

	// Observation source (required)
	cmd.Flags().String("observation", "", "path to FingerprintIdentityObservation JSON")

	// Candidate and mapping (required for REVERIFY_REQUIRED)
	cmd.Flags().String("candidate-identity", "", "serialized IntegrationKnowledgeIdentity (JSON string from lookup)")
	cmd.Flags().String("mapping", "", "path to live MappingDefinition JSON (CANDIDATE status, evidence-derived)")

	// Governance inputs (required for REVERIFY_REQUIRED)
	cmd.Flags().String("evidence", "", "path to evidence data file")
	cmd.Flags().String("input", "", "input directory containing raw session data")
	cmd.Flags().String("ground-truth", "", "path to ground truth expectations JSON")
	cmd.Flags().String("out", "", "output directory for profile.yaml or re-verification results")
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

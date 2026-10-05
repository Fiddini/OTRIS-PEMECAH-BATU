package main

import (
	"fmt"
	"strings"

	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/qc"
)

// QCGovernanceDecision holds the outcome of QC applicability decision and review
type QCGovernanceDecision struct {
	Applicability qc.QCApplicability
	Record        *qc.QCKnowledgeRecord
}

// ReviewQCGovernance handles QC applicability decision and execution [D3G-3.5]
// REQUIRED: fail-closed (PASS only, everything else stops)
// OPTIONAL: non-blocking (proceeds with audit)
// NOT_APPLICABLE: skips QC (preserves justification)
func ReviewQCGovernance(
	m *mapping.MappingDefinition,
	qcPolicyPath, qcRefPath, qcChallengesPath string,
	engineerApprover, version string,
) (*QCGovernanceDecision, error) {
	decision := &QCGovernanceDecision{
		Applicability: qc.ApplicabilityNotApplicable,
		Record:        nil,
	}

	// [D3G-3.5-A] Determine QC applicability from policy
	if strings.TrimSpace(qcPolicyPath) == "" {
		// No policy supplied: NOT_APPLICABLE (default)
		return decision, nil
	}

	// Load QC policy
	fmt.Printf("[D3G-3.5-A] Loading QC policy from %s\n", qcPolicyPath)
	policy, err := loadQCPolicy(qcPolicyPath)
	if err != nil {
		return nil, fmt.Errorf("[D3G-3.5-B] QC policy load failed: %w", err)
	}

	decision.Applicability = policy.Applicability

	// [D3G-3.5-C] Execute ReviewQC if REQUIRED or OPTIONAL
	if decision.Applicability == qc.ApplicabilityRequired || decision.Applicability == qc.ApplicabilityOptional {
		fmt.Printf("[D3G-3.5-C] QC applicability=%s, loading inputs\n", decision.Applicability)

		// Load reference definition
		if strings.TrimSpace(qcRefPath) == "" {
			if decision.Applicability == qc.ApplicabilityRequired {
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC: reference definition path required")
			}
			fmt.Printf("[D3G-3.5-C] OPTIONAL QC: reference absent (auditable)\n")
			return decision, nil
		}

		refDef, err := loadQCReference(qcRefPath)
		if err != nil {
			if decision.Applicability == qc.ApplicabilityRequired {
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC: reference definition load failed: %w", err)
			}
			fmt.Printf("[D3G-3.5-C] OPTIONAL QC: reference load failed (auditable): %v\n", err)
			return decision, nil
		}

		// Load challenges
		if strings.TrimSpace(qcChallengesPath) == "" {
			if decision.Applicability == qc.ApplicabilityRequired {
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC: challenges path required")
			}
			fmt.Printf("[D3G-3.5-C] OPTIONAL QC: challenges absent (auditable)\n")
			return decision, nil
		}

		challenges, err := loadQCChallenges(qcChallengesPath)
		if err != nil {
			if decision.Applicability == qc.ApplicabilityRequired {
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC: challenges load failed: %w", err)
			}
			fmt.Printf("[D3G-3.5-C] OPTIONAL QC: challenges load failed (auditable): %v\n", err)
			return decision, nil
		}

		// Call ReviewQC
		fmt.Printf("[D3G-3.5-C] Calling ReviewQC (applicability=%s)\n", decision.Applicability)
		qcRecord, err := qc.ReviewQC(
			m.MappingIdentity, m.MappingVersion, m.DeviceFingerprint,
			"governed-pipeline", engineerApprover, version,
			challenges, refDef, policy,
		)

		// FAIL-CLOSED GATE for REQUIRED
		if decision.Applicability == qc.ApplicabilityRequired {
			if err != nil {
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC review failed: %w", err)
			}
			if qcRecord == nil {
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC: no record returned")
			}

			// Check test fixture (fail-closed)
			if qcRecord.IsTestFixture {
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC: test fixture cannot reach production (fail-closed)")
			}

			// PASS only for REQUIRED
			switch qcRecord.Verdict {
			case qc.VerdictPass:
				fmt.Printf("[D3G-3.5-C] REQUIRED QC PASSED\n")
			case qc.VerdictSuspect:
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC SUSPECT (fail-closed): %s", qcRecord.Notes)
			case qc.VerdictFail:
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC FAILED (fail-closed): %s", qcRecord.Notes)
			case qc.VerdictInsufficientEvidence:
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC INSUFFICIENT_EVIDENCE (fail-closed): %s", qcRecord.Notes)
			case qc.VerdictNotApplicable:
				return nil, fmt.Errorf("[D3G-3.5-C] REQUIRED QC returned NOT_APPLICABLE (contradiction)")
			}
		}

		// OPTIONAL: non-PASS is auditable but not blocking
		if decision.Applicability == qc.ApplicabilityOptional {
			if err != nil {
				fmt.Printf("[D3G-3.5-C] OPTIONAL QC review error: %v (auditable)\n", err)
				return decision, nil
			}
			if qcRecord != nil {
				fmt.Printf("[D3G-3.5-C] OPTIONAL QC completed (verdict=%s, will be auditable)\n", qcRecord.Verdict)
				if qcRecord.Verdict != qc.VerdictPass {
					fmt.Printf("[D3G-3.5-C] WARNING: OPTIONAL QC non-PASS (%s), but proceeding\n", qcRecord.Verdict)
				}
				decision.Record = qcRecord
			}
		}

		decision.Record = qcRecord
	}

	// [D3G-3.5-D] Handle NOT_APPLICABLE
	if decision.Applicability == qc.ApplicabilityNotApplicable {
		// Validate justification present
		if strings.TrimSpace(policy.Justification) == "" {
			return nil, fmt.Errorf("[D3G-3.5-D] NOT_APPLICABLE requires explicit justification (fail-closed)")
		}
		fmt.Printf("[D3G-3.5-D] QC NOT_APPLICABLE: %s\n", policy.Justification)
	}

	return decision, nil
}

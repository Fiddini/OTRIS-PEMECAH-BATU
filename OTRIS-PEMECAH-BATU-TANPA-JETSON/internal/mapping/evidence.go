package mapping

import (
	"fmt"
	"slices"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
)

// evidenceApproval is private integrity state, never imported from a report.
type evidenceApproval struct {
	claim             evidence.Claim
	provenance        string
	author            string
	differentialRef   string
	createdAt         time.Time
	reportRef         string
	approvedAt        time.Time
	refs              []Evidence
}

// Claim returns a value copy; changing it does not change this definition.
func (m *MappingDefinition) Claim() evidence.Claim {
	return evidence.Claim{
		MappingIdentity: m.MappingIdentity, MappingVersion: m.MappingVersion,
		SourceProtocol: m.SourceProtocol, SourcePosition: m.SourcePosition,
		SourceCode: m.SourceCode, SourceUnit: m.SourceUnit,
		CanonicalConcept: m.CanonicalConcept, CanonicalUnit: m.CanonicalUnit,
		ParserIdentity: m.ParserIdentity, DeviceFingerprint: m.DeviceFingerprint,
		DeviceIdentityRef: m.DeviceIdentityRef,
	}
}

func (m *MappingDefinition) validateAuthority() error {
	switch m.status {
	case StatusCandidate:
		if m.approval != nil || m.replay != nil || m.EvidenceGateReportRef != "" || m.EvidenceGateVerdict != "" ||
			!m.EvidenceApprovedAt.IsZero() || !m.ReplayValidatedAt.IsZero() {
			return fmt.Errorf("CANDIDATE contains approval metadata")
		}
	case StatusEvidenceApproved, StatusReplayValidated:
		a := m.approval
		if a == nil || m.Claim() != a.claim || m.Provenance != a.provenance ||
			m.ProvenanceAuthor != a.author || m.DifferentialReportRef != a.differentialRef ||
			!m.CreatedAt.Equal(a.createdAt) || m.EvidenceGateReportRef != a.reportRef ||
			m.EvidenceGateVerdict != evidence.ApprovedForMapping ||
			!m.EvidenceApprovedAt.Equal(a.approvedAt) || !slices.Equal(m.EvidenceRefs, a.refs) {
			return fmt.Errorf("evidence authority missing or approved definition modified")
		}
		if m.status == StatusEvidenceApproved {
			if m.replay != nil || !m.ReplayValidatedAt.IsZero() {
				return fmt.Errorf("EVIDENCE_APPROVED contains replay authority metadata")
			}
		} else if m.replay == nil || m.replay.claim != a.claim || m.replay.validatedAt.IsZero() ||
			!m.ReplayValidatedAt.Equal(m.replay.validatedAt) {
			return fmt.Errorf("replay authority missing or validated definition modified")
		}
	default:
		return fmt.Errorf("unsupported authoritative status: %q", m.status)
	}
	return nil
}

// ReviewEvidence requests a fresh Evidence Gate decision, not a caller-supplied verdict.
// The caller owns m and must not write its fields concurrently with this operation.
func ReviewEvidence(m *MappingDefinition, evidencePath, reportPath string) error {
	status, err := m.Status()
	if err != nil {
		return err
	}
	if status != StatusCandidate {
		return fmt.Errorf("evidence review requires CANDIDATE, got %s", status)
	}
	before := evidenceApproval{
		claim: m.Claim(), provenance: m.Provenance, author: m.ProvenanceAuthor,
		differentialRef: m.DifferentialReportRef, createdAt: m.CreatedAt,
	}
	report, err := evidence.ReviewClaim(before.claim, evidencePath, reportPath)
	if err != nil {
		return err
	}
	if report.Verdict != evidence.ApprovedForMapping || report.Claim != before.claim {
		return fmt.Errorf("Evidence Gate did not approve this claim: %s", report.Verdict)
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if m.status != StatusCandidate || m.Claim() != before.claim || m.Provenance != before.provenance ||
		m.ProvenanceAuthor != before.author || m.DifferentialReportRef != before.differentialRef ||
		!m.CreatedAt.Equal(before.createdAt) {
		return fmt.Errorf("mapping changed during evidence review")
	}
	if !m.status.CanTransitionTo(StatusEvidenceApproved) {
		return fmt.Errorf("illegal evidence approval transition")
	}

	before.reportRef = reportPath
	before.approvedAt = time.Now().UTC()
	before.refs = slices.Clone(report.Evidence)
	m.EvidenceRefs = slices.Clone(report.Evidence)
	m.EvidenceGateReportRef = reportPath
	m.EvidenceGateVerdict = report.Verdict
	m.EvidenceApprovedAt = before.approvedAt
	m.approval = &before
	m.status = StatusEvidenceApproved
	return nil
}

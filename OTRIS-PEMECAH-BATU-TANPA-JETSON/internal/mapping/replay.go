package mapping

import (
	"fmt"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/evidence"
	"github.com/otris-id/otris-pemecah-batu/internal/replay"
)

type replayApproval struct {
	claim       evidence.Claim
	validatedAt time.Time
}

// ReviewReplay governs a fresh measured execution, never a caller-supplied report.
// The caller owns m and must not mutate it concurrently with this operation.
// reportPath must be new; persistence failure leaves Evidence authority unchanged.
func ReviewReplay(m *MappingDefinition, inDir string, count int, opts replay.ReplayOptions, reportPath string) (replay.Report, error) {
	rejected := replay.Report{Verdict: "NOT_READY"}
	status, err := m.Status()
	if err != nil {
		return rejected, err
	}
	if status != StatusEvidenceApproved {
		return rejected, fmt.Errorf("governed replay requires EVIDENCE_APPROVED, got %s", status)
	}
	if reportPath == "" {
		return rejected, fmt.Errorf("replay report path is required")
	}
	before := m.Claim()
	report, err := replay.MeasureClaim(before, inDir, count, opts)
	if err != nil {
		return report, err
	}
	status, err = m.Status()
	if err != nil || status != StatusEvidenceApproved || m.Claim() != before {
		report.Verdict = "NOT_READY"
		return report, fmt.Errorf("mapping changed during replay: %v", err)
	}
	if report.Claim == nil || *report.Claim != before || report.Mapping != before.MappingIdentity ||
		report.MappingVersion != before.MappingVersion || report.Verdict != "MEASURED_MATCH" ||
		!report.GroundTruthProvided || report.Compared <= 0 || report.Success <= 0 ||
		report.FailedParse != 0 || report.FalseData != 0 || report.FalseDataRate != 0 {
		if err := replay.WriteReport(reportPath, report); err != nil {
			return report, err
		}
		return report, fmt.Errorf("replay did not validate approved claim: %s", report.Verdict)
	}
	if err := m.ValidateTransitionTo(StatusReplayValidated); err != nil {
		report.Verdict = "NOT_READY"
		return report, err
	}
	next := *m
	next.ReplayValidatedAt = time.Now().UTC()
	next.replay = &replayApproval{claim: before, validatedAt: next.ReplayValidatedAt}
	next.status = StatusReplayValidated
	if err := next.Validate(); err != nil {
		report.Verdict = "NOT_READY"
		return report, err
	}
	report.Verdict = "READY_FOR_PROFILE_BUILDER"
	if err := replay.WriteReport(reportPath, report); err != nil {
		report.Verdict = "NOT_READY"
		return report, err
	}
	*m = next
	return report, nil
}

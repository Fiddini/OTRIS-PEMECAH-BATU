package mapping

// Status represents the validation stage of a MappingDefinition.
type Status string

const (
	// CANDIDATE: explicit mapping claim exists but has not passed Evidence Gate.
	StatusCandidate Status = "CANDIDATE"

	// EVIDENCE_APPROVED: Evidence Gate has approved the mapping claim.
	StatusEvidenceApproved Status = "EVIDENCE_APPROVED"

	// REPLAY_VALIDATED: Replay has validated the mapping against ground truth (false_data_rate = 0.0).
	StatusReplayValidated Status = "REPLAY_VALIDATED"
)

// IsValid returns true if status is a recognized status.
func (s Status) IsValid() bool {
	switch s {
	case StatusCandidate, StatusEvidenceApproved, StatusReplayValidated:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if a transition from current status to target status is legal.
// This enforces the state machine without assuming what authority is performing the transition.
// The actual transition execution is deferred to future integration layers (Evidence Gate, Replay).
func (s Status) CanTransitionTo(target Status) bool {
	if !target.IsValid() {
		return false // unknown target status
	}
	if s == target {
		return false // no same-state transitions
	}

	switch s {
	case StatusCandidate:
		// CANDIDATE can only go to EVIDENCE_APPROVED
		return target == StatusEvidenceApproved
	case StatusEvidenceApproved:
		// EVIDENCE_APPROVED can only go to REPLAY_VALIDATED
		return target == StatusReplayValidated
	case StatusReplayValidated:
		// REPLAY_VALIDATED cannot transition further
		return false
	default:
		return false
	}
}

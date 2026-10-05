package qc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// QCKnowledgeRecord is the comprehensive assembly of QC validation results.
// Immutable: integrity enforced via digest. Separate from MappingDefinition
// (does not modify status, remains independent artifact).
type QCKnowledgeRecord struct {
	// Binding
	MappingIdentity string `json:"mapping_identity"` // exact from MappingDefinition
	MappingVersion  string `json:"mapping_version"`  // exact from MappingDefinition
	DeviceRef       string `json:"device_ref"`       // device fingerprint

	// Reference Results
	ReferenceSet  []QCStatistics    `json:"reference_set"`   // []{level, N, mean, SD, CV%, min, max}
	TransferFunc  *TransferFunction `json:"transfer_func,omitempty"` // candidate factor

	// Acceptance
	Criteria AcceptanceCriteria `json:"criteria"` // policy applied
	Verdict  QCVerdict          `json:"verdict"`  // PASS, SUSPECT, FAIL, INSUFFICIENT_EVIDENCE, NOT_APPLICABLE
	Notes    string             `json:"notes"`    // why verdict (e.g., "CV exceeds threshold")

	// Challenge History (for audit trail)
	ChallengeCount int       `json:"challenge_count"` // how many reference challenges measured
	ChallengesRef  []string  `json:"challenges_ref"`  // references to challenge records (if stored separately)

	// Authority
	Provenance string `json:"provenance"` // how QC was performed
	Reviewer   string `json:"reviewer"`   // who conducted QC review
	CreatedAt  time.Time `json:"created_at"` // when QC completed

	// Versioning & Status
	Version    string `json:"version"`    // semantic version of this QC result
	Status     string `json:"status"`    // always "CANDIDATE" (no auto-promotion)
	Integrity  string `json:"integrity"` // SHA256 digest for immutability

	// Flags
	IsTestFixture bool `json:"is_test_fixture"` // fail-closed: test data marked explicitly
}

// NewQCRecord creates a new QC knowledge record with integrity digest.
// All fields validated before sealing.
func NewQCRecord(
	mappingID, mappingVersion, deviceRef, provenance, reviewer, version string,
	referenceSet []QCStatistics,
	criteria AcceptanceCriteria,
	verdict QCVerdict,
	notes string,
	challengeCount int,
) (*QCKnowledgeRecord, error) {
	// Validate required fields
	if err := validateRequiredField("mapping_identity", mappingID); err != nil {
		return nil, fmt.Errorf("QC knowledge record: %w", err)
	}
	if err := validateRequiredField("mapping_version", mappingVersion); err != nil {
		return nil, fmt.Errorf("QC knowledge record: %w", err)
	}
	if err := validateRequiredField("device_ref", deviceRef); err != nil {
		return nil, fmt.Errorf("QC knowledge record: %w", err)
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("QC knowledge record: %w", err)
	}
	if err := validateRequiredField("reviewer", reviewer); err != nil {
		return nil, fmt.Errorf("QC knowledge record: %w", err)
	}
	if err := validateRequiredField("version", version); err != nil {
		return nil, fmt.Errorf("QC knowledge record: %w", err)
	}

	// Validate criteria
	if err := criteria.Validate(); err != nil {
		return nil, fmt.Errorf("QC knowledge record: %w", err)
	}

	// Validate verdict code
	switch verdict {
	case VerdictPass, VerdictSuspect, VerdictFail, VerdictInsufficientEvidence, VerdictNotApplicable:
		// OK
	default:
		return nil, fmt.Errorf("QC knowledge record: unknown verdict %q", verdict)
	}

	// Validate notes (if present)
	if notes != "" && !utf8.ValidString(notes) {
		return nil, errors.New("QC knowledge record: notes invalid UTF-8")
	}

	// Challenge count must be >= 0
	if challengeCount < 0 {
		return nil, fmt.Errorf("QC knowledge record: invalid challenge count %d", challengeCount)
	}

	// Validate reference statistics
	if len(referenceSet) > 0 {
		if err := ValidateIsolation(referenceSet); err != nil {
			return nil, fmt.Errorf("QC knowledge record: %w", err)
		}
	}

	record := &QCKnowledgeRecord{
		MappingIdentity: mappingID,
		MappingVersion:  mappingVersion,
		DeviceRef:       deviceRef,
		ReferenceSet:    referenceSet,
		Criteria:        criteria,
		Verdict:         verdict,
		Notes:           notes,
		ChallengeCount:  challengeCount,
		Provenance:      provenance,
		Reviewer:        reviewer,
		CreatedAt:       time.Now().UTC(),
		Version:         version,
		Status:          "CANDIDATE", // always candidate, never auto-promoted
		IsTestFixture:   false,
	}

	// Compute integrity digest
	record.Integrity = record.computeDigest()

	return record, nil
}

// computeDigest returns immutable integrity hash.
func (qr *QCKnowledgeRecord) computeDigest() string {
	// Canonical representation (deterministic)
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%d|%s|%d|%d|%v",
		qr.MappingIdentity,
		qr.MappingVersion,
		qr.DeviceRef,
		qr.Verdict,
		qr.CreatedAt.UnixNano(),
		qr.Status,
		len(qr.ReferenceSet),
		qr.ChallengeCount,
		qr.IsTestFixture,
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// Verify checks integrity of this record.
func (qr *QCKnowledgeRecord) Verify() error {
	expected := qr.computeDigest()
	if qr.Integrity != expected {
		return fmt.Errorf("QC record: integrity violation (expected %s, got %s)", expected, qr.Integrity)
	}
	// Status must remain CANDIDATE
	if qr.Status != "CANDIDATE" {
		return fmt.Errorf("QC record: invalid status %q (must be CANDIDATE)", qr.Status)
	}
	return nil
}

// IsProductionReady checks if record meets minimum QC requirements for profile building.
// Returns true only if verdict is PASS and no test fixture markers.
func (qr *QCKnowledgeRecord) IsProductionReady() bool {
	if qr.IsTestFixture {
		return false // fail-closed: test fixtures never reach production
	}
	return qr.Verdict == VerdictPass
}

// String provides human-readable summary
func (qr *QCKnowledgeRecord) String() string {
	return fmt.Sprintf("QCKnowledgeRecord{mapping=%s/%s, device=%s, verdict=%s, challenges=%d, status=%s}",
		qr.MappingIdentity, qr.MappingVersion, qr.DeviceRef, qr.Verdict, qr.ChallengeCount, qr.Status)
}

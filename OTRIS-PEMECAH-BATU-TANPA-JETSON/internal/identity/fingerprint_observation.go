package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// ChangeType categorizes the fingerprint observation.
type ChangeType string

const (
	ChangeTypeInitial            ChangeType = "INITIAL"            // First observation
	ChangeTypeUpdate             ChangeType = "UPDATE"             // Legitimate device change
	ChangeTypePossibleCorruption ChangeType = "POSSIBLE_CORRUPTION" // Unexpected/suspicious change
	ChangeTypeUnknown            ChangeType = "UNKNOWN"            // Cannot determine cause
)

// FingerprintIdentityObservation is an immutable record of observed fingerprint
// identity state. It does NOT imply device failure; it records identity deviation
// that may require investigation.
type FingerprintIdentityObservation struct {
	// Observation Identity
	ObservationID string `json:"observation_id"` // unique identifier

	// The Values
	DeviceFingerprint     string `json:"device_fingerprint"`      // current structural identity
	PreviousFingerprintRef string `json:"previous_fingerprint_ref,omitempty"` // reference to prior observation (if known)
	DeviceIdentityRef     string `json:"device_identity_ref,omitempty"`       // vendor/model reference (optional context)

	// Classification
	ChangeType ChangeType `json:"change_type"` // INITIAL, UPDATE, POSSIBLE_CORRUPTION, UNKNOWN
	Confidence float64    `json:"confidence"`  // 0.0-1.0 (how confident in this observation?)

	// Detection Evidence
	DetectionMethod string `json:"detection_method"` // how fingerprint was observed
	Provenance      string `json:"provenance"`       // evidence/source reference

	// Metadata
	ObservedAt time.Time `json:"observed_at"` // when observation made
	ObservedBy string    `json:"observed_by"` // what/who observed (e.g., "PB-10", "edge-discovery")

	// Investigation (initially empty)
	RequiresInvestigation bool      `json:"requires_investigation"` // true if change is unexpected
	InvestigationStatus   string    `json:"investigation_status"`   // UNREVIEWED, UNDER_INVESTIGATION, CLOSED
	ReviewedAt            time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy            string    `json:"reviewed_by,omitempty"`
	ReviewNotes           string    `json:"review_notes,omitempty"`

	// Integrity
	Digest  string `json:"digest"`  // SHA256 immutable hash
	Version string `json:"version"` // schema version

	sealed bool
}

// NewObservation creates a new fingerprint identity observation.
// Fail-closed: all required fields validated before sealing.
func NewObservation(
	observationID, deviceFingerprint, detectionMethod, provenance, observedBy string,
	changeType ChangeType,
	confidence float64,
	observedAt time.Time,
) (*FingerprintIdentityObservation, error) {
	// Validate required fields
	if err := validateRequiredField("observation_id", observationID); err != nil {
		return nil, fmt.Errorf("fingerprint observation: %w", err)
	}
	if err := validateRequiredField("device_fingerprint", deviceFingerprint); err != nil {
		return nil, fmt.Errorf("fingerprint observation: %w", err)
	}
	if err := validateRequiredField("detection_method", detectionMethod); err != nil {
		return nil, fmt.Errorf("fingerprint observation: %w", err)
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("fingerprint observation: %w", err)
	}
	if err := validateRequiredField("observed_by", observedBy); err != nil {
		return nil, fmt.Errorf("fingerprint observation: %w", err)
	}

	// Validate change type
	switch changeType {
	case ChangeTypeInitial, ChangeTypeUpdate, ChangeTypePossibleCorruption, ChangeTypeUnknown:
		// OK
	default:
		return nil, fmt.Errorf("fingerprint observation: unknown change type %q", changeType)
	}

	// Validate confidence (0.0-1.0)
	if confidence < 0.0 || confidence > 1.0 {
		return nil, fmt.Errorf("fingerprint observation: confidence must be 0.0-1.0, got %f", confidence)
	}

	// Validate timestamp
	if observedAt.IsZero() {
		return nil, errors.New("fingerprint observation: observed_at required")
	}

	fpo := &FingerprintIdentityObservation{
		ObservationID:         observationID,
		DeviceFingerprint:     deviceFingerprint,
		ChangeType:            changeType,
		Confidence:            confidence,
		DetectionMethod:       detectionMethod,
		Provenance:            provenance,
		ObservedAt:            observedAt.UTC(),
		ObservedBy:            observedBy,
		RequiresInvestigation: changeType != ChangeTypeInitial && confidence > 0.7, // auto-flag uncertain changes
		InvestigationStatus:   "UNREVIEWED",
		Version:               "1.0.0",
	}

	fpo.Digest = fpo.computeDigest()
	fpo.sealed = true

	return fpo, nil
}

// SetPreviousReference sets the reference to the prior fingerprint observation.
// Returns error if already sealed (after creation, cannot modify).
func (fpo *FingerprintIdentityObservation) SetPreviousReference(ref string) error {
	if fpo.sealed {
		return errors.New("fingerprint observation: cannot modify sealed observation")
	}
	if ref != "" && !utf8.ValidString(ref) {
		return errors.New("fingerprint observation: previous reference invalid UTF-8")
	}
	fpo.PreviousFingerprintRef = ref
	fpo.Digest = fpo.computeDigest()
	return nil
}

// SetDeviceContext sets optional device identity reference.
func (fpo *FingerprintIdentityObservation) SetDeviceContext(identityRef string) error {
	if fpo.sealed {
		return errors.New("fingerprint observation: cannot modify sealed observation")
	}
	if identityRef != "" && !utf8.ValidString(identityRef) {
		return errors.New("fingerprint observation: device identity ref invalid UTF-8")
	}
	fpo.DeviceIdentityRef = identityRef
	fpo.Digest = fpo.computeDigest()
	return nil
}

// Review records the outcome of investigation.
// Returns error if observation is not sealed or already reviewed.
func (fpo *FingerprintIdentityObservation) Review(
	reviewedBy, notes string,
	confirmed bool,
) error {
	if !fpo.sealed {
		return errors.New("fingerprint observation: cannot review unsealed observation")
	}
	if fpo.ReviewedAt != (time.Time{}) {
		return errors.New("fingerprint observation: already reviewed")
	}
	if err := validateRequiredField("reviewed_by", reviewedBy); err != nil {
		return fmt.Errorf("fingerprint observation: %w", err)
	}

	fpo.ReviewedAt = time.Now().UTC()
	fpo.ReviewedBy = reviewedBy
	fpo.ReviewNotes = notes
	fpo.InvestigationStatus = "CLOSED"

	if confirmed {
		// Change is legitimate (e.g., firmware update, device replacement)
		// ChangeType remains as recorded; no modification
	}
	// If not confirmed, ChangeType remains as recorded (POSSIBLE_CORRUPTION, UNKNOWN, etc.)

	return nil
}

// computeDigest returns immutable integrity hash.
func (fpo *FingerprintIdentityObservation) computeDigest() string {
	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%f|%s|%d|%d|%s|%s",
		fpo.ObservationID,
		fpo.DeviceFingerprint,
		fpo.PreviousFingerprintRef,
		fpo.ChangeType,
		fpo.DetectionMethod,
		fpo.Confidence,
		fpo.Provenance,
		fpo.ObservedAt.Unix(),
		fpo.ObservedAt.Nanosecond(),
		fpo.ObservedBy,
		fpo.Version,
	)
	return hex.EncodeToString(sha256.New().Sum([]byte(canonical)))
}

// Verify checks integrity of this observation.
func (fpo *FingerprintIdentityObservation) Verify() error {
	if !fpo.sealed {
		return errors.New("fingerprint observation: not sealed")
	}
	expected := fpo.computeDigest()
	if fpo.Digest != expected {
		return fmt.Errorf("fingerprint observation: digest mismatch (expected %s, got %s)", expected, fpo.Digest)
	}
	return nil
}

// IsInitial returns true if this is the first fingerprint observation.
func (fpo *FingerprintIdentityObservation) IsInitial() bool {
	return fpo.ChangeType == ChangeTypeInitial
}

// IsConfirmedChange returns true if review confirmed the change is legitimate.
func (fpo *FingerprintIdentityObservation) IsConfirmedChange() bool {
	return fpo.InvestigationStatus == "CLOSED" && fpo.ChangeType == ChangeTypeUpdate
}

// String provides human-readable summary.
func (fpo *FingerprintIdentityObservation) String() string {
	return fmt.Sprintf("FingerprintIdentityObservation{id=%s, type=%s, fp=%s..., status=%s}",
		fpo.ObservationID, fpo.ChangeType, fpo.DeviceFingerprint[:16], fpo.InvestigationStatus)
}

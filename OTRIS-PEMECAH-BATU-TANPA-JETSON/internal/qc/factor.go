package qc

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// TransferFunction represents a candidate factor/transformation between raw and canonical values.
// Always CANDIDATE status: requires explicit re-verification if reused across fingerprints/mappings.
// Versioning enforces immutability + traceability.
type TransferFunction struct {
	// Identity
	Identity string `json:"identity"` // e.g., "scalar_factor", "linear_transform", "opaque_reference"

	// Binding (device-specific, immutable)
	DeviceFingerprint string `json:"device_fingerprint"` // fingerprint change requires re-verification
	MappingIdentity   string `json:"mapping_identity"`   // mapping change requires re-verification
	MappingVersion    string `json:"mapping_version"`    // version change requires re-verification

	// Parameters (interpretation-free, stored as strings)
	Params map[string]string `json:"params"` // e.g., {"factor": "2.5", "offset": "0"}

	// Authority
	Algorithm string `json:"algorithm"` // "scalar_multiply", "linear_equation", "lookup_table", "opaque"
	Method    string `json:"method"`    // how factor was computed/derived
	Provenance string `json:"provenance"` // evidence/reference for this factor
	Author    string `json:"author"`    // who derived the factor

	// Versioning (immutable)
	Version string `json:"version"` // semantic version; change signals re-verification needed
	Status  string `json:"status"`  // always "CANDIDATE" (no auto-promotion)
}

// NewTransferFunction creates a new transfer function candidate.
// Fail-closed: all fields required, device/mapping binding explicit.
func NewTransferFunction(
	identity, algorithm, method, provenance, author, version string,
	deviceFingerprint, mappingID, mappingVersion string,
	params map[string]string,
) (*TransferFunction, error) {
	// Validate all required identity fields
	if err := validateRequiredField("identity", identity); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}
	if err := validateRequiredField("algorithm", algorithm); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}
	if err := validateRequiredField("method", method); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}
	if err := validateRequiredField("provenance", provenance); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}
	if err := validateRequiredField("author", author); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}
	if err := validateRequiredField("version", version); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}

	// Device/mapping binding mandatory (explicit, never inferred)
	if err := validateRequiredField("device_fingerprint", deviceFingerprint); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}
	if err := validateRequiredField("mapping_identity", mappingID); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}
	if err := validateRequiredField("mapping_version", mappingVersion); err != nil {
		return nil, fmt.Errorf("transfer function: %w", err)
	}

	// Parameters must be provided (even if empty map, it's explicit)
	if params == nil {
		params = make(map[string]string)
	}
	// Validate parameter values (all must be valid UTF-8)
	for k, v := range params {
		if !utf8.ValidString(k) || !utf8.ValidString(v) {
			return nil, fmt.Errorf("transfer function: parameter %q has invalid UTF-8", k)
		}
	}

	return &TransferFunction{
		Identity:          identity,
		Algorithm:         algorithm,
		Method:            method,
		Provenance:        provenance,
		Author:            author,
		Version:           version,
		DeviceFingerprint: deviceFingerprint,
		MappingIdentity:   mappingID,
		MappingVersion:    mappingVersion,
		Params:            params,
		Status:            "CANDIDATE", // always candidate, no auto-promotion
	}, nil
}

// CanReuseWith checks if factor can be applied to a different mapping/fingerprint.
// Returns error if material binding changes (requires re-verification).
func (tf *TransferFunction) CanReuseWith(newDeviceFingerprint, newMappingID, newMappingVersion string) error {
	// Fingerprint change requires re-verification
	if newDeviceFingerprint != tf.DeviceFingerprint {
		return fmt.Errorf(
			"transfer function: device fingerprint change detected (was %q, now %q) — requires re-verification",
			tf.DeviceFingerprint, newDeviceFingerprint,
		)
	}
	// Mapping identity change requires re-verification
	if newMappingID != tf.MappingIdentity {
		return fmt.Errorf(
			"transfer function: mapping identity change detected (was %q, now %q) — requires re-verification",
			tf.MappingIdentity, newMappingID,
		)
	}
	// Mapping version change requires re-verification
	if newMappingVersion != tf.MappingVersion {
		return fmt.Errorf(
			"transfer function: mapping version change detected (was %q, now %q) — requires re-verification",
			tf.MappingVersion, newMappingVersion,
		)
	}
	return nil
}

// Validate enforces internal consistency.
// Status must always be CANDIDATE (no auto-promotion).
func (tf *TransferFunction) Validate() error {
	if tf.Status != "CANDIDATE" {
		return fmt.Errorf("transfer function: invalid status %q (must be CANDIDATE)", tf.Status)
	}
	// All binding fields must remain non-empty
	for name, value := range map[string]string{
		"device_fingerprint": tf.DeviceFingerprint,
		"mapping_identity":   tf.MappingIdentity,
		"mapping_version":    tf.MappingVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("transfer function: binding field %s became empty", name)
		}
	}
	return nil
}

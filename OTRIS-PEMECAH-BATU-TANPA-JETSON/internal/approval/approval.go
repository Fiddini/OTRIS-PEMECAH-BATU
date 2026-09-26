package approval

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/profilebuilder"
)

const (
	StatusEngineerApproved = "ENGINEER_APPROVED"
	DecisionApprove        = "APPROVE"
	DecisionReject         = "REJECT"
)

// Approval exposes audit data, not a credential that can be restored from a file.
// Callers must not mutate the approval, candidate or mapping concurrently with review or validation.
type Approval struct {
	ApprovalStatus     string    `json:"approval_status" yaml:"approval_status"`
	CandidateDigest    string    `json:"candidate_digest" yaml:"candidate_digest"`
	ProfileID          string    `json:"profile_id" yaml:"profile_id"`
	ProfileVersion     string    `json:"profile_version" yaml:"profile_version"`
	MappingIdentity    string    `json:"mapping_identity" yaml:"mapping_identity"`
	MappingVersion     string    `json:"mapping_version" yaml:"mapping_version"`
	ApproverIdentity   string    `json:"approver_identity" yaml:"approver_identity"`
	Decision           string    `json:"decision" yaml:"decision"`
	ApprovedAt         time.Time `json:"approved_at" yaml:"approved_at"`
	EvidenceReference  string    `json:"evidence_reference" yaml:"evidence_reference"`
	EvidenceApprovedAt time.Time `json:"evidence_approved_at" yaml:"evidence_approved_at"`
	ReplayValidatedAt  time.Time `json:"replay_validated_at" yaml:"replay_validated_at"`
	MappingProvenance  string    `json:"mapping_provenance" yaml:"mapping_provenance"`
	ProvenanceAuthor   string    `json:"provenance_author" yaml:"provenance_author"`

	authority *approvalAuthority
}

type approvalAuthority struct {
	mapping         *mapping.MappingDefinition
	mappingJSON     string
	auditJSON       string
	candidateDigest [sha256.Size]byte
}

// Review records an explicit human decision about these exact candidate contents.
// Approver identity is supplied by the caller; this package does not authenticate users.
func Review(candidate *profilebuilder.Profile, liveMapping *mapping.MappingDefinition, approver, decision string) (*Approval, error) {
	digest, err := candidateDigest(candidate, liveMapping)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(approver) == "" || !utf8.ValidString(approver) {
		return nil, errors.New("approver identity is required and must be valid UTF-8")
	}
	switch decision {
	case DecisionApprove:
	case DecisionReject:
		return nil, errors.New("engineer rejected candidate")
	default:
		return nil, errors.New("explicit APPROVE or REJECT decision required")
	}
	mappingJSON, err := json.Marshal(liveMapping)
	if err != nil {
		return nil, fmt.Errorf("snapshot live mapping: %w", err)
	}
	a := &Approval{
		ApprovalStatus: StatusEngineerApproved, CandidateDigest: fmt.Sprintf("%x", digest),
		ProfileID: candidate.ProfileID, ProfileVersion: candidate.Version,
		MappingIdentity: liveMapping.MappingIdentity, MappingVersion: liveMapping.MappingVersion,
		ApproverIdentity: approver, Decision: decision, ApprovedAt: time.Now().UTC(),
		EvidenceReference: liveMapping.EvidenceGateReportRef, EvidenceApprovedAt: liveMapping.EvidenceApprovedAt,
		ReplayValidatedAt: liveMapping.ReplayValidatedAt, MappingProvenance: liveMapping.Provenance,
		ProvenanceAuthor: liveMapping.ProvenanceAuthor,
	}
	auditJSON, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("snapshot approval: %w", err)
	}
	a.authority = &approvalAuthority{
		mapping: liveMapping, mappingJSON: string(mappingJSON),
		auditJSON: string(auditJSON), candidateDigest: digest,
	}
	return a, nil
}

// Validate requires the original live mapping and unchanged candidate and audit contents.
func (a *Approval) Validate(candidate *profilebuilder.Profile, liveMapping *mapping.MappingDefinition) error {
	if a == nil || a.authority == nil {
		return errors.New("live engineer approval authority required")
	}
	if liveMapping != a.authority.mapping {
		return errors.New("approval belongs to a different live mapping")
	}
	digest, err := candidateDigest(candidate, liveMapping)
	if err != nil {
		return err
	}
	if digest != a.authority.candidateDigest {
		return errors.New("candidate changed after approval")
	}
	mappingJSON, err := json.Marshal(liveMapping)
	if err != nil {
		return fmt.Errorf("validate live mapping: %w", err)
	}
	if string(mappingJSON) != a.authority.mappingJSON {
		return errors.New("mapping changed after approval")
	}
	auditJSON, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("validate approval: %w", err)
	}
	if string(auditJSON) != a.authority.auditJSON {
		return errors.New("approval audit data changed")
	}
	return nil
}

// UnmarshalJSON rejects imports, including decoding into an already-authoritative receiver.
func (a *Approval) UnmarshalJSON([]byte) error {
	if a != nil {
		*a = Approval{}
	}
	return errors.New("approval JSON is audit data; authority cannot be imported")
}

// UnmarshalYAML rejects imports through the callback-based YAML unmarshaler contract.
func (a *Approval) UnmarshalYAML(func(any) error) error {
	if a != nil {
		*a = Approval{}
	}
	return errors.New("approval YAML is audit data; authority cannot be imported")
}

func candidateDigest(p *profilebuilder.Profile, m *mapping.MappingDefinition) ([sha256.Size]byte, error) {
	var empty [sha256.Size]byte
	if p == nil {
		return empty, errors.New("candidate required")
	}
	status, err := m.Status() // Status calls Validate, including private evidence and replay bindings.
	if err != nil {
		return empty, fmt.Errorf("invalid live mapping: %w", err)
	}
	if status != mapping.StatusReplayValidated {
		return empty, errors.New("live mapping must be REPLAY_VALIDATED")
	}
	if p.GovernanceStatus != "CANDIDATE" {
		return empty, errors.New("governance_status must be CANDIDATE (not yet approved for production)")
	}
	for _, field := range []struct{ name, value string }{
		{"profile_id", p.ProfileID}, {"version", p.Version},
		{"manufacturer", p.Manufacturer}, {"model", p.Model},
		{"firmware_tested", p.FirmwareTested}, {"transport", p.Transport},
		{"mapping_identity", p.MappingIdentity}, {"mapping_version", p.MappingVersion},
		{"parser_identity", p.ParserIdentity}, {"device_fingerprint", p.DeviceFingerprint},
		{"device_identity_ref", p.DeviceIdentityRef},
	} {
		if strings.TrimSpace(field.value) == "" || !utf8.ValidString(field.value) {
			return empty, fmt.Errorf("missing or invalid %s", field.name)
		}
	}
	for _, limitation := range p.KnownLimitations {
		if !utf8.ValidString(limitation) {
			return empty, errors.New("invalid UTF-8 in known limitations")
		}
	}
	if p.ProfileID != fmt.Sprintf("%s_%s_%s_v%s", p.Manufacturer, p.Model, p.FirmwareTested, p.Version) {
		return empty, errors.New("profile identity does not match candidate metadata")
	}
	if p.MappingIdentity != m.MappingIdentity || p.MappingVersion != m.MappingVersion || p.MappingProvenance != m.Provenance {
		return empty, errors.New("candidate mapping identity, version or provenance mismatch")
	}
	if p.EvidenceReference != m.EvidenceGateReportRef {
		return empty, errors.New("candidate evidence reference mismatch")
	}
	if p.ReplayReference != "" || p.ValidatedAt != m.ReplayValidatedAt.Format("2006-01-02") || p.FalseDataRate != 0 {
		return empty, errors.New("candidate replay metadata mismatch")
	}
	claim := m.Claim()
	if p.Framing != "" || p.ProtocolFamily != claim.SourceProtocol || p.ParserIdentity != claim.ParserIdentity ||
		p.DeviceFingerprint != claim.DeviceFingerprint || p.DeviceIdentityRef != claim.DeviceIdentityRef {
		return empty, errors.New("candidate protocol or device context mismatch")
	}
	// Verify field mapping matches claim (compare individual fields, not slices)
	if len(p.Mapping) != 1 {
		return empty, errors.New("candidate field mapping count mismatch")
	}
	mapped := p.Mapping[0]
	if mapped.SourcePos != claim.SourcePosition || mapped.SourceCode != claim.SourceCode ||
		mapped.SourceUnit != claim.SourceUnit || mapped.Canonical != claim.CanonicalConcept ||
		mapped.Unit != claim.CanonicalUnit || mapped.Confidence != 0 {
		return empty, errors.New("candidate field mapping mismatch")
	}
	// ponytail: D3F's fixed four-entry chain. Update this check only with an approved candidate schema change.
	chain := []string{
		fmt.Sprintf("Evidence approved: %s", m.EvidenceGateReportRef),
		fmt.Sprintf("Replay validated: false_data=0.0 at %s", m.ReplayValidatedAt.Format(time.RFC3339Nano)),
		fmt.Sprintf("Mapping: %s v%s by %s", claim.MappingIdentity, claim.MappingVersion, m.ProvenanceAuthor),
		fmt.Sprintf("Parser: %s", claim.ParserIdentity),
	}
	if !slices.Equal(p.AuthorityChain, chain) {
		return empty, errors.New("candidate authority chain mismatch")
	}
	// Canonical here means compact encoding/json of the fixed typed schema: stable field order,
	// no maps, all represented fields included. Reject invalid UTF-8 above rather than normalize it.
	data, err := json.Marshal(p)
	if err != nil {
		return empty, fmt.Errorf("encode candidate: %w", err)
	}
	return sha256.Sum256(data), nil
}

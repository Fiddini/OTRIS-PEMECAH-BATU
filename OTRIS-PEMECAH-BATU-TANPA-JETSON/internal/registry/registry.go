package registry

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/otris-id/otris-pemecah-batu/internal/approval"
	"github.com/otris-id/otris-pemecah-batu/internal/mapping"
	"github.com/otris-id/otris-pemecah-batu/internal/profilebuilder"
	"github.com/otris-id/otris-pemecah-batu/internal/qc"
)

const (
	// SchemaV1 versions the canonical admission payload; unknown versions are rejected.
	SchemaV1 = "otris.registry.admission.v1"

	// StatusRegistryAdmitted means only that governed admission accepted this exact
	// ENGINEER_APPROVED candidate. It is not VERIFIED, REAL_DEVICE_VALIDATED,
	// PRODUCTION_VALIDATED, REGISTERED, CONNECTED or ACTIVE.
	StatusRegistryAdmitted = "REGISTRY_ADMITTED"

	// domain separates these signatures from any other Ed25519 use in this project.
	domain = "OTRIS-REGISTRY-ADMISSION"
)

// QCBinding represents QC governance binding in the registry attestation
type QCBinding struct {
	Applicability string    `json:"applicability"`              // "REQUIRED", "OPTIONAL", "NOT_APPLICABLE"
	Verdict       string    `json:"verdict,omitempty"`          // verdict (only if relevant)
	Digest        string    `json:"digest,omitempty"`           // QCKnowledgeRecord.Integrity
	Justification string    `json:"justification,omitempty"`    // for NOT_APPLICABLE
	ReviewedBy    string    `json:"reviewed_by,omitempty"`      // who reviewed
	ReviewedAt    time.Time `json:"reviewed_at,omitempty"`      // when reviewed
}

// Attestation is persistable. Unlike D3H approval, its authority is the signature
// over canonical payload bytes, so verification needs no live governed process.
type Attestation struct {
	SchemaVersion          string    `json:"schema_version"`
	AdmissionStatus        string    `json:"admission_status"`
	ProfileID              string    `json:"profile_id"`
	ProfileVersion         string    `json:"profile_version"`
	CandidateDigest        string    `json:"candidate_digest"`
	MappingIdentity        string    `json:"mapping_identity"`
	MappingVersion         string    `json:"mapping_version"`
	EvidenceReference      string    `json:"evidence_reference"`
	SemanticEvidenceDigest string    `json:"semantic_evidence_digest,omitempty"` // SHA256 of semantic evidence (established by C1)
	ReplayValidatedAt      time.Time `json:"replay_validated_at"`
	MappingProvenance      string    `json:"mapping_provenance"`
	ProvenanceAuthor       string    `json:"provenance_author"`
	ApproverIdentity       string    `json:"approver_identity"`
	ApprovalTimestamp      time.Time `json:"approval_timestamp"`
	IssuerKeyID            string    `json:"issuer_key_id"`
	IssuedAt               time.Time `json:"issued_at"`
	Sequence               uint64    `json:"sequence"`
	QC                     *QCBinding `json:"qc,omitempty"`           // QC governance binding (optional)
	Signature              string    `json:"signature"`
}

// Admit requires live governed authority in this process and returns a signed
// attestation. The caller supplies the issuer key, key id and monotonic sequence;
// this package never generates or persists key material.
func Admit(
	candidate *profilebuilder.Profile,
	liveMapping *mapping.MappingDefinition,
	engineerApproval *approval.Approval,
	qcRecord *qc.QCKnowledgeRecord,
	qcApplicability qc.QCApplicability,
	issuerKey ed25519.PrivateKey,
	issuerKeyID string,
	sequence uint64,
) (*Attestation, error) {
	if candidate == nil {
		return nil, errors.New("candidate required")
	}
	if liveMapping == nil {
		return nil, errors.New("live mapping required")
	}
	if engineerApproval == nil {
		return nil, errors.New("live engineer approval required")
	}
	// Live D3H authority: rejects non-authoritative approvals, candidate or mapping
	// mutation, approval reuse and any mapping that is not REPLAY_VALIDATED.
	if err := engineerApproval.Validate(candidate, liveMapping); err != nil {
		return nil, fmt.Errorf("engineer approval authority invalid: %w", err)
	}
	if len(issuerKey) != ed25519.PrivateKeySize {
		return nil, errors.New("issuer private key must be a valid Ed25519 private key")
	}
	if strings.TrimSpace(issuerKeyID) == "" || !utf8.ValidString(issuerKeyID) {
		return nil, errors.New("issuer key id is required and must be valid UTF-8")
	}
	if sequence == 0 {
		return nil, errors.New("admission sequence must be greater than zero")
	}
	if err := checkGovernedBindings(candidate, liveMapping, engineerApproval); err != nil {
		return nil, err
	}
	digest, err := candidateDigest(candidate)
	if err != nil {
		return nil, err
	}
	// Independently recomputed digest must equal the approved digest, so later
	// verification can rebind the candidate without D3H private state.
	if subtle.ConstantTimeCompare([]byte(digest), []byte(engineerApproval.CandidateDigest)) != 1 {
		return nil, errors.New("candidate digest does not match approved digest")
	}
	a := &Attestation{
		SchemaVersion: SchemaV1, AdmissionStatus: StatusRegistryAdmitted,
		ProfileID: candidate.ProfileID, ProfileVersion: candidate.Version,
		CandidateDigest: digest,
		MappingIdentity: liveMapping.MappingIdentity, MappingVersion: liveMapping.MappingVersion,
		EvidenceReference: liveMapping.EvidenceGateReportRef, SemanticEvidenceDigest: liveMapping.SemanticEvidenceDigest,
		ReplayValidatedAt: liveMapping.ReplayValidatedAt,
		MappingProvenance: liveMapping.Provenance, ProvenanceAuthor: liveMapping.ProvenanceAuthor,
		ApproverIdentity: engineerApproval.ApproverIdentity, ApprovalTimestamp: engineerApproval.ApprovedAt,
		IssuerKeyID: issuerKeyID, IssuedAt: time.Now().UTC(), Sequence: sequence,
	}

	// Add QC binding if applicable and present
	if qcApplicability == qc.ApplicabilityRequired && qcRecord != nil && qcRecord.Verdict == qc.VerdictPass {
		// REQUIRED + PASS: bind QC to attestation
		a.QC = &QCBinding{
			Applicability: string(qcApplicability),
			Verdict:       string(qcRecord.Verdict),
			Digest:        qcRecord.Integrity,
			ReviewedBy:    qcRecord.Reviewer,
			ReviewedAt:    qcRecord.CreatedAt,
		}
	} else if qcApplicability == qc.ApplicabilityNotApplicable {
		// NOT_APPLICABLE: bind justification to attestation
		a.QC = &QCBinding{
			Applicability: string(qcApplicability),
			Justification: qcRecord.Criteria.Justification,
			ReviewedBy:    qcRecord.Reviewer,
			ReviewedAt:    qcRecord.CreatedAt,
		}
	}
	// OPTIONAL: only bind if explicitly required by policy (not implemented in initial phase)

	payload, err := a.canonicalPayload()
	if err != nil {
		return nil, err
	}
	a.Signature = hex.EncodeToString(ed25519.Sign(issuerKey, payload))
	return a, nil
}

// VerifyAttestation checks a persisted attestation against the exact candidate and an
// explicitly trusted issuer key. It never restores D3H private approval authority.
func VerifyAttestation(a *Attestation, candidate *profilebuilder.Profile, trustedPublicKey ed25519.PublicKey, expectedKeyID string) error {
	if a == nil {
		return errors.New("attestation required")
	}
	if candidate == nil {
		return errors.New("candidate required")
	}
	if len(trustedPublicKey) != ed25519.PublicKeySize {
		return errors.New("trusted public key must be a valid Ed25519 public key")
	}
	if strings.TrimSpace(expectedKeyID) == "" {
		return errors.New("expected issuer key id is required")
	}
	if a.IssuerKeyID != expectedKeyID {
		return errors.New("attestation issuer key id is not the expected issuer")
	}
	payload, err := a.canonicalPayload()
	if err != nil {
		return err
	}
	signature, err := hex.DecodeString(a.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("attestation signature is malformed")
	}
	if !ed25519.Verify(trustedPublicKey, payload, signature) {
		return errors.New("attestation signature is not valid for this issuer and payload")
	}
	digest, err := candidateDigest(candidate)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(digest), []byte(a.CandidateDigest)) != 1 {
		return errors.New("candidate does not match the admitted candidate digest")
	}
	if candidate.ProfileID != a.ProfileID || candidate.Version != a.ProfileVersion {
		return errors.New("candidate profile identity or version mismatch")
	}
	if candidate.MappingIdentity != a.MappingIdentity || candidate.MappingVersion != a.MappingVersion {
		return errors.New("candidate mapping identity or version mismatch")
	}
	if candidate.MappingProvenance != a.MappingProvenance || candidate.EvidenceReference != a.EvidenceReference {
		return errors.New("candidate provenance or evidence reference mismatch")
	}
	if candidate.SemanticEvidenceDigest != a.SemanticEvidenceDigest {
		return errors.New("candidate semantic evidence digest mismatch")
	}
	return nil
}

// checkGovernedBindings re-proves identity, version and provenance agreement
// independently of the approval's own integrity snapshots.
func checkGovernedBindings(p *profilebuilder.Profile, m *mapping.MappingDefinition, a *approval.Approval) error {
	status, err := m.Status()
	if err != nil {
		return fmt.Errorf("invalid live mapping: %w", err)
	}
	if status != mapping.StatusReplayValidated {
		return errors.New("live mapping must be REPLAY_VALIDATED")
	}
	if p.GovernanceStatus != "CANDIDATE" {
		return errors.New("governance_status must remain CANDIDATE (not yet approved for production)")
	}
	if a.ApprovalStatus != approval.StatusEngineerApproved || a.Decision != approval.DecisionApprove {
		return errors.New("approval must be an ENGINEER_APPROVED APPROVE decision")
	}
	if strings.TrimSpace(a.ApproverIdentity) == "" || !utf8.ValidString(a.ApproverIdentity) {
		return errors.New("approver identity is required")
	}
	if a.ApprovedAt.IsZero() {
		return errors.New("approval timestamp is required")
	}
	if m.ReplayValidatedAt.IsZero() {
		return errors.New("replay validation timestamp is required")
	}
	if a.ProfileID != p.ProfileID || a.ProfileVersion != p.Version {
		return errors.New("approval profile identity or version mismatch")
	}
	if a.MappingIdentity != m.MappingIdentity || a.MappingVersion != m.MappingVersion ||
		p.MappingIdentity != m.MappingIdentity || p.MappingVersion != m.MappingVersion {
		return errors.New("mapping identity or version mismatch")
	}
	if a.EvidenceReference != m.EvidenceGateReportRef || p.EvidenceReference != m.EvidenceGateReportRef {
		return errors.New("evidence reference mismatch")
	}
	if a.MappingProvenance != m.Provenance || p.MappingProvenance != m.Provenance ||
		a.ProvenanceAuthor != m.ProvenanceAuthor {
		return errors.New("mapping provenance mismatch")
	}
	if !a.ReplayValidatedAt.Equal(m.ReplayValidatedAt) {
		return errors.New("replay provenance mismatch")
	}
	return nil
}

// candidateDigest reproduces D3H's candidate digest without needing the live mapping:
// SHA256 over compact encoding/json of the fixed typed candidate schema.
func candidateDigest(p *profilebuilder.Profile) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("encode candidate: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// canonicalPayload builds the exact signed bytes. The encoding is deliberately not
// JSON: no map iteration, no indentation choice and no field-name parsing affects it.
//
// Layout, in this fixed field order and nothing else:
//
//	"OTRIS-REGISTRY-ADMISSION\n"
//	for each field: <byte-length> ":" <value> "\n"
//
// Every field is length-prefixed, so no combination of values can shift a boundary
// and forge a different payload with identical bytes. Times are RFC3339Nano in UTC;
// the sequence is decimal. Signature is excluded: it signs these bytes.
// Adding, removing or reordering a field is a breaking change and needs a new
// SchemaVersion; the frozen golden test in this package fails if bytes change.
func (a *Attestation) canonicalPayload() ([]byte, error) {
	if a.SchemaVersion != SchemaV1 {
		return nil, fmt.Errorf("unsupported admission schema version %q", a.SchemaVersion)
	}
	if a.AdmissionStatus != StatusRegistryAdmitted {
		return nil, fmt.Errorf("unsupported admission status %q", a.AdmissionStatus)
	}
	if a.Sequence == 0 {
		return nil, errors.New("admission sequence must be greater than zero")
	}
	if a.ApprovalTimestamp.IsZero() || a.IssuedAt.IsZero() || a.ReplayValidatedAt.IsZero() {
		return nil, errors.New("approval, issue and replay timestamps are required")
	}
	fields := []string{
		a.SchemaVersion, a.AdmissionStatus, a.ProfileID, a.ProfileVersion, a.CandidateDigest,
		a.MappingIdentity, a.MappingVersion, a.EvidenceReference,
		a.SemanticEvidenceDigest, // Semantic evidence digest bound in canonical payload
		a.ReplayValidatedAt.UTC().Format(time.RFC3339Nano),
		a.MappingProvenance, a.ProvenanceAuthor, a.ApproverIdentity,
		a.ApprovalTimestamp.UTC().Format(time.RFC3339Nano),
		a.IssuerKeyID, a.IssuedAt.UTC().Format(time.RFC3339Nano),
		strconv.FormatUint(a.Sequence, 10),
	}
	var payload strings.Builder
	payload.WriteString(domain + "\n")
	for i, value := range fields {
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("invalid UTF-8 in canonical field %d", i)
		}
		// SemanticEvidenceDigest (field 8) may be empty (omitempty if not bound)
		// All other fields are required
		if i != 8 && strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("missing canonical field %d", i)
		}
		fmt.Fprintf(&payload, "%d:%s\n", len(value), value)
	}

	// Include QC binding in canonical form if present (backward-compatible: conditional)
	if a.QC != nil {
		qcFields := []string{
			a.QC.Applicability,
			a.QC.Verdict,
			a.QC.Digest,
			a.QC.Justification,
		}
		for i, value := range qcFields {
			if !utf8.ValidString(value) {
				return nil, fmt.Errorf("invalid UTF-8 in QC canonical field %d", i)
			}
			fmt.Fprintf(&payload, "%d:%s\n", len(value), value)
		}
	}

	return []byte(payload.String()), nil
}

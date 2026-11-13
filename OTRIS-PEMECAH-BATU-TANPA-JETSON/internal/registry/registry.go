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
	// FROZEN: TestCanonicalPayloadIsFrozenAndDeterministic asserts its exact byte
	// layout. V1 attestations carry no expiry, applicability, or revocation epoch --
	// canonicalPayload() must keep producing exactly the same bytes for V1 forever.
	SchemaV1 = "otris.registry.admission.v1"

	// SchemaV2 additively extends V1's canonical bytes with three more signed
	// fields: ExpiresAt, ApplicableFingerprints (joined, length-prefixed per
	// element so no fingerprint can shift a boundary) and RevocationEpoch. V1's
	// own field bytes are emitted identically first; V2 only appends after them,
	// so a V2 verifier could still re-derive the V1 prefix unchanged.
	SchemaV2 = "otris.registry.admission.v2"

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

	// V2-only fields (SchemaV2). Zero/empty on a V1 attestation and never
	// read, signed, or enforced for one -- canonicalPayload() only appends
	// them when SchemaVersion == SchemaV2, so a V1 attestation's signed
	// bytes are unaffected by their presence or absence in the struct.
	ExpiresAt              int64     `json:"expires_at,omitempty"`              // required for V2: Unix seconds
	ApplicableFingerprints []string  `json:"applicable_fingerprints,omitempty"` // required for V2: non-empty
	RevocationEpoch        uint64    `json:"revocation_epoch,omitempty"`        // required for V2

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

// AdmitV2 performs the identical live-governed admission Admit() does (same
// validation, same candidate/mapping/approval binding -- this calls Admit()
// itself rather than duplicating its checks), then extends the result with
// the three V2 authorization-scope fields and re-signs under SchemaV2.
// Admit() itself is untouched and keeps producing V1 attestations: existing
// callers see no behavior change.
//
// expiresAt, applicableFingerprints and revocationEpoch all become part of
// the SIGNED bytes (see appendV2CanonicalFields) -- they are not metadata
// bolted on after signing, so an Edge verifier can trust them exactly as
// much as it trusts the rest of the attestation.
func AdmitV2(
	candidate *profilebuilder.Profile,
	liveMapping *mapping.MappingDefinition,
	engineerApproval *approval.Approval,
	qcRecord *qc.QCKnowledgeRecord,
	qcApplicability qc.QCApplicability,
	issuerKey ed25519.PrivateKey,
	issuerKeyID string,
	sequence uint64,
	expiresAt int64,
	applicableFingerprints []string,
	revocationEpoch uint64,
) (*Attestation, error) {
	a, err := Admit(candidate, liveMapping, engineerApproval, qcRecord, qcApplicability, issuerKey, issuerKeyID, sequence)
	if err != nil {
		return nil, err
	}
	a.SchemaVersion = SchemaV2
	a.ExpiresAt = expiresAt
	a.ApplicableFingerprints = applicableFingerprints
	a.RevocationEpoch = revocationEpoch

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

// VerifyAttestationV2Authorization checks the V2 authorization-scope fields
// ONLY -- call this AFTER VerifyAttestation has already proven the signature
// and candidate binding. Checking authorization scope on an attestation
// whose signature was never verified would mean trusting attacker-supplied
// expiry/fingerprint/revocation bytes, which is exactly what Stage C's
// original stub got wrong.
//
// A V1 attestation has no authorization scope to check: this function
// returns an error for one rather than silently treating "no V2 fields" as
// "authorization granted" -- callers that need V2 guarantees must receive a
// V2 attestation, not fall back to a weaker V1 one.
//
// now is passed in (not time.Now() internally) so callers can test expiry
// deterministically and so a single call site controls what "now" means.
func VerifyAttestationV2Authorization(a *Attestation, now time.Time, deviceFingerprint string, minValidRevocationEpoch uint64) error {
	if a == nil {
		return errors.New("attestation required")
	}
	if a.SchemaVersion != SchemaV2 {
		return fmt.Errorf("attestation schema %q carries no V2 authorization scope", a.SchemaVersion)
	}
	if a.ExpiresAt <= 0 {
		return errors.New("v2 attestation missing expires_at")
	}
	if now.Unix() > a.ExpiresAt {
		return fmt.Errorf("attestation expired at %d (now %d)", a.ExpiresAt, now.Unix())
	}
	if len(a.ApplicableFingerprints) == 0 {
		return errors.New("v2 attestation missing applicable_fingerprints")
	}
	if strings.TrimSpace(deviceFingerprint) == "" {
		return errors.New("device fingerprint is required")
	}
	matched := false
	for _, fp := range a.ApplicableFingerprints {
		if subtle.ConstantTimeCompare([]byte(fp), []byte(deviceFingerprint)) == 1 {
			matched = true
			break
		}
	}
	if !matched {
		return errors.New("device fingerprint is not in this attestation's applicable scope")
	}
	// A historically-valid signature must never outrun a later revocation: an
	// attestation signed under epoch 3 is not reauthorized just because its
	// bytes still verify -- minValidRevocationEpoch is the floor the caller's
	// current trusted policy requires.
	if a.RevocationEpoch < minValidRevocationEpoch {
		return fmt.Errorf("attestation revocation epoch %d is below required minimum %d", a.RevocationEpoch, minValidRevocationEpoch)
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
// Adding, removing or reordering a V1 field is a breaking change to the frozen
// golden test (TestCanonicalPayloadIsFrozenAndDeterministic) -- V1's own fields
// and QC block below are untouched by SchemaV2. SchemaV2 only appends ExpiresAt,
// ApplicableFingerprints (each element length-prefixed, then the whole list
// length-prefixed, so no fingerprint can forge a boundary) and RevocationEpoch
// strictly after everything V1/QC already writes.
func (a *Attestation) canonicalPayload() ([]byte, error) {
	if a.SchemaVersion != SchemaV1 && a.SchemaVersion != SchemaV2 {
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

	if a.SchemaVersion == SchemaV2 {
		if err := a.appendV2CanonicalFields(&payload); err != nil {
			return nil, err
		}
	}

	return []byte(payload.String()), nil
}

// appendV2CanonicalFields writes ExpiresAt, ApplicableFingerprints and
// RevocationEpoch after everything canonicalPayload already wrote. All three
// are required (non-zero / non-empty) for V2 -- a V2 attestation that omits
// authorization scope is not a weaker V2, it is a malformed one, so this
// fails closed rather than signing an empty/permissive scope.
func (a *Attestation) appendV2CanonicalFields(payload *strings.Builder) error {
	if a.ExpiresAt <= 0 {
		return errors.New("schema v2 requires expires_at greater than zero")
	}
	if len(a.ApplicableFingerprints) == 0 {
		return errors.New("schema v2 requires at least one applicable fingerprint")
	}
	// RevocationEpoch has no "required non-zero" rule of its own -- epoch 0 is
	// a legitimate first epoch. What matters is that it is IN the signed bytes.

	fmt.Fprintf(payload, "%d:%s\n", len(strconv.FormatInt(a.ExpiresAt, 10)), strconv.FormatInt(a.ExpiresAt, 10))

	var fpList strings.Builder
	for i, fp := range a.ApplicableFingerprints {
		if !utf8.ValidString(fp) {
			return fmt.Errorf("invalid UTF-8 in applicable fingerprint %d", i)
		}
		if strings.TrimSpace(fp) == "" {
			return fmt.Errorf("empty applicable fingerprint at index %d", i)
		}
		fmt.Fprintf(&fpList, "%d:%s,", len(fp), fp)
	}
	fpListStr := fpList.String()
	fmt.Fprintf(payload, "%d:%s\n", len(fpListStr), fpListStr)

	revocationStr := strconv.FormatUint(a.RevocationEpoch, 10)
	fmt.Fprintf(payload, "%d:%s\n", len(revocationStr), revocationStr)

	return nil
}

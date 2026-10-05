# PHASE 3B: Revised Design Specification — Fail-Closed QC Governance + Cryptographic Binding

**Baseline**: commit 6c3a02f (Reference & QC Intelligence Engine v1.0)  
**Based on**: PHASE 3A Inspection Report + User Corrections  
**Status**: DESIGN SPECIFICATION (NO CODE YET)

**Two Required Corrections Addressed**:
1. **REQUIRED QC is fail-closed governance condition** — QC FAIL/SUSPECT/INSUFFICIENT_EVIDENCE STOP pipeline
2. **QC digest cryptographically bound to admission** — Registry attestation includes QC digest in signed payload

---

## 1. Revised Governed Call Sequence

### Current (3A Report)
```
ReviewEvidence
    ↓
ReviewReplay
    ↓
[Optional: ReviewQC]  ← Not fail-closed
    ↓
BuildFromMappingDefinition
    ↓
[Optional: Approval.Review + Registry.Admit]
```

### Revised (Phase 3B)
```
ReviewEvidence
    ↓
ReviewReplay (REPLAY_VALIDATED)
    ↓
QC APPLICABILITY DECISION GATE
    ├─ REQUIRED → must call ReviewQC(), must PASS
    ├─ OPTIONAL → may call ReviewQC(), non-PASS is auditable but not blocking
    └─ NOT_APPLICABLE → skip ReviewQC(), preserve justification/provenance
    ↓
[Fail-Closed: if REQUIRED and QC missing/fail/suspect/insufficient → STOP]
    ↓
BuildFromMappingDefinition (m remains REPLAY_VALIDATED)
    ↓
Profile + [optional QCKnowledgeRecord reference]
    ↓
[Optional: Approval.Review (live, no changes)]
    ↓
[Optional: Registry.Admit with QC digest in canonical signed payload]
    ↓
Attestation with BOUND QC digest (if REQUIRED)
```

**Exact flow**:

**Pseudo-code** (main.go, new [D3G-3.5] step):
```go
// [D3G-3] Replay validation complete, m.Status == REPLAY_VALIDATED

// [D3G-3.5-A] Determine QC applicability
var qcApplicability qc.QCApplicability = qc.ApplicabilityNotApplicable  // default
var qcPolicyPath string = ""
if cmd.Flags().Changed("qc-policy") {
  qcPolicyPath, _ := cmd.Flags().GetString("qc-policy")
  qcApplicability = determineApplicability(qcPolicyPath)  // loads JSON policy, extracts Applicability
}

// [D3G-3.5-B] Load QC inputs if policy present
var qcPolicy *qc.AcceptanceCriteria
var qcChallenges []*qc.ReferenceChallenge
var qcRefDef *qc.ReferenceDefinition
if qcApplicability != qc.ApplicabilityNotApplicable {
  qcPolicy, err = loadQCPolicy(qcPolicyPath)
  if err != nil {
    return fmt.Errorf("[D3G-3.5-B] QC policy load failed: %w", err)
  }
  if err := qcPolicy.Validate(); err != nil {
    return fmt.Errorf("[D3G-3.5-B] QC policy validation failed: %w", err)
  }
}

// [D3G-3.5-C] Execute ReviewQC if applicability requires or allows
var qcRecord *qc.QCKnowledgeRecord
if qcApplicability == qc.ApplicabilityRequired || qcApplicability == qc.ApplicabilityOptional {
  challengesPath, _ := cmd.Flags().GetString("qc-challenges")
  refDefPath, _ := cmd.Flags().GetString("qc-reference")
  
  // Load challenges and reference definition
  qcChallenges, err = loadQCChallenges(challengesPath)
  if err != nil && qcApplicability == qc.ApplicabilityRequired {
    return fmt.Errorf("[D3G-3.5-C] REQUIRED QC: challenges load failed: %w", err)
  }
  
  qcRefDef, err = loadQCReference(refDefPath)
  if err != nil && qcApplicability == qc.ApplicabilityRequired {
    return fmt.Errorf("[D3G-3.5-C] REQUIRED QC: reference definition load failed: %w", err)
  }
  
  // Call ReviewQC
  fmt.Printf("[D3G-3.5-C] Reviewing QC (applicability=%s)\n", qcApplicability)
  qcRecord, err = qc.ReviewQC(
    m.MappingIdentity, m.MappingVersion, m.DeviceFingerprint,
    "governed-pipeline", engineerApprover, version,  // provenance, reviewer
    qcChallenges, qcRefDef, qcPolicy,
  )
  
  // FAIL-CLOSED GATE: REQUIRED QC must succeed
  if qcApplicability == qc.ApplicabilityRequired {
    if err != nil {
      return fmt.Errorf("[D3G-3.5-C] REQUIRED QC review failed: %w", err)
    }
    if qcRecord == nil {
      return fmt.Errorf("[D3G-3.5-C] REQUIRED QC: no record returned")
    }
    
    // Check verdict: PASS only, everything else STOPS
    switch qcRecord.Verdict {
    case qc.VerdictPass:
      fmt.Printf("[D3G-3.5-C] REQUIRED QC PASSED\n")
    case qc.VerdictSuspect:
      return fmt.Errorf("[D3G-3.5-C] REQUIRED QC SUSPECT (fail-closed): %s", qcRecord.Notes)
    case qc.VerdictFail:
      return fmt.Errorf("[D3G-3.5-C] REQUIRED QC FAILED (fail-closed): %s", qcRecord.Notes)
    case qc.VerdictInsufficientEvidence:
      return fmt.Errorf("[D3G-3.5-C] REQUIRED QC INSUFFICIENT_EVIDENCE (fail-closed): %s", qcRecord.Notes)
    case qc.VerdictNotApplicable:
      return fmt.Errorf("[D3G-3.5-C] REQUIRED QC returned NOT_APPLICABLE (contradiction)")
    }
  }
  
  // OPTIONAL QC: non-PASS is auditable but not blocking
  if qcApplicability == qc.ApplicabilityOptional && qcRecord != nil {
    fmt.Printf("[D3G-3.5-C] OPTIONAL QC completed (verdict=%s, will be auditable)\n", qcRecord.Verdict)
    if qcRecord.Verdict != qc.VerdictPass {
      fmt.Printf("[D3G-3.5-C] WARNING: OPTIONAL QC non-PASS (%s), but proceeding\n", qcRecord.Verdict)
    }
  }
}

// [D3G-3.5-D] Handle NOT_APPLICABLE: preserve applicability + justification
if qcApplicability == qc.ApplicabilityNotApplicable {
  fmt.Printf("[D3G-3.5-D] QC NOT_APPLICABLE (will be audited with justification)\n")
}

fmt.Printf("[D3G-3] Replay and QC validated\n")

// [D3G-4] Profile Builder (m still REPLAY_VALIDATED, unchanged)
fmt.Printf("[D3G-4] Building Integration Profile Candidate\n")
profile, err := profilebuilder.BuildFromMappingDefinition(
  m, manufacturer, model, transport, firmwareTested, buildDir, version, qcRecord,
)
if err != nil {
  return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: profile build rejected: %w", err)
}

// ... rest unchanged ...
```

---

## 2. Applicability Decision Table

| Applicability | QC Input | ReviewQC() Called | PASS Result | FAIL Result | SUSPECT Result | INSUFFICIENT_EVIDENCE Result | NOT_APPLICABLE Result |
|---|---|---|---|---|---|---|---|
| **REQUIRED** | policy + challenges + refDef | YES (mandatory) | ✅ PROCEED | ❌ STOP (fail-closed) | ❌ STOP (fail-closed) | ❌ STOP (fail-closed) | ❌ ERROR (contradiction) |
| **REQUIRED** | missing any input | NO | N/A | ❌ STOP (missing inputs fail-closed) | N/A | N/A | N/A |
| **OPTIONAL** | policy + challenges + refDef | YES (if supplied) | ✅ PROCEED (audited) | ✅ PROCEED (audited, non-PASS noted) | ✅ PROCEED (audited) | ✅ PROCEED (audited) | ⚠️ Audited as skipped |
| **OPTIONAL** | missing any input | NO | N/A | ✅ PROCEED (no QC, auditable absence) | N/A | N/A | N/A |
| **NOT_APPLICABLE** | policy with justification | NO (never called) | N/A | N/A | N/A | N/A | ✅ PROCEED (preserve justification) |
| **NOT_APPLICABLE** | no justification supplied | ERROR | N/A | N/A | N/A | N/A | ❌ STOP (invalid policy) |

**Key principles**:
- **REQUIRED**: all inputs mandatory, PASS only, everything else STOPS
- **OPTIONAL**: inputs optional, absence auditable, non-PASS auditable but not blocking, no fabricated PASS
- **NOT_APPLICABLE**: skip QC entirely, preserve justification/provenance in audit

---

## 3. Exact Fail-Closed Stop Conditions

### REQUIRED QC — All of these trigger STOP:
1. **Policy load fails** → STOP: `"[D3G-3.5-B] QC policy validation failed: <error>"`
2. **Challenges missing/load fails** → STOP: `"[D3G-3.5-C] REQUIRED QC: challenges load failed: <error>"`
3. **Reference definition missing/load fails** → STOP: `"[D3G-3.5-C] REQUIRED QC: reference definition load failed: <error>"`
4. **ReviewQC() returns error** → STOP: `"[D3G-3.5-C] REQUIRED QC review failed: <error>"`
5. **ReviewQC() returns nil record** → STOP: `"[D3G-3.5-C] REQUIRED QC: no record returned"`
6. **Verdict = FAIL** → STOP: `"[D3G-3.5-C] REQUIRED QC FAILED (fail-closed): <notes>"`
7. **Verdict = SUSPECT** → STOP: `"[D3G-3.5-C] REQUIRED QC SUSPECT (fail-closed): <notes>"`
8. **Verdict = INSUFFICIENT_EVIDENCE** → STOP: `"[D3G-3.5-C] REQUIRED QC INSUFFICIENT_EVIDENCE (fail-closed): <notes>"`
9. **Verdict = NOT_APPLICABLE** (contradiction) → STOP: `"[D3G-3.5-C] REQUIRED QC returned NOT_APPLICABLE (contradiction)"`
10. **Test fixture detected** → STOP: `"[D3G-3.5-C] REQUIRED QC: test fixture cannot reach production (fail-closed)"`

### OPTIONAL QC — None of these trigger STOP:
1. QC policy absent → LOG: `"[D3G-3.5] QC absent (OPTIONAL)"`; CONTINUE
2. Challenges missing → LOG: `"[D3G-3.5-C] OPTIONAL QC: challenges absent (will be audited)"`; CONTINUE
3. Reference definition missing → LOG: `"[D3G-3.5-C] OPTIONAL QC: reference absent (will be audited)"`; CONTINUE
4. ReviewQC() returns error → LOG: `"[D3G-3.5-C] OPTIONAL QC review error: <error> (will be audited)"`; qcRecord=nil; CONTINUE
5. Verdict = FAIL → LOG: `"[D3G-3.5-C] OPTIONAL QC FAILED (proceeding with audit)"`; CONTINUE
6. Verdict = SUSPECT → LOG: `"[D3G-3.5-C] OPTIONAL QC SUSPECT (proceeding with audit)"`; CONTINUE
7. Verdict = INSUFFICIENT_EVIDENCE → LOG: `"[D3G-3.5-C] OPTIONAL QC INSUFFICIENT_EVIDENCE (proceeding with audit)"`; CONTINUE
8. Verdict = NOT_APPLICABLE → LOG: `"[D3G-3.5-C] OPTIONAL QC NOT_APPLICABLE (will be audited)"`; CONTINUE
9. Test fixture detected → LOG: `"[D3G-3.5-C] OPTIONAL QC: test fixture (will be audited, not eligible for production)"` ; qcRecord persists but marked; CONTINUE

### NOT_APPLICABLE QC — All of these should continue:
1. Policy absent → CONTINUE (default)
2. Policy present with valid justification → CONTINUE (preserve in audit)
3. Policy present without justification → STOP: `"[D3G-3.5-D] NOT_APPLICABLE requires explicit justification (fail-closed)"`

---

## 4. QC Artifact/Digest Binding

### Binding Architecture

**Decision point**: Does QC applicability affect governance admission?

**Rule**:
- **REQUIRED QC with PASS**: YES, bind to attestation (QC governs admission decision)
- **OPTIONAL QC with PASS**: OPTIONAL, bind if policy requires (determined by flag)
- **OPTIONAL QC with non-PASS**: NO, never bind (non-PASS is audited but not governing)
- **NOT_APPLICABLE QC**: YES, bind justification/provenance to audit chain (for traceability)

### Binding Implementation

**New struct in registry/registry.go**:
```go
type QCBinding struct {
  Applicability  string    `json:"applicability"`  // "REQUIRED", "OPTIONAL", "NOT_APPLICABLE"
  Verdict        string    `json:"verdict,omitempty"` // only if PASS or auditable non-PASS
  Digest         string    `json:"digest,omitempty"` // QCKnowledgeRecord.Integrity (only if PASS)
  Justification  string    `json:"justification,omitempty"` // for NOT_APPLICABLE
  ReviewedBy     string    `json:"reviewed_by,omitempty"` // who reviewed the QC
  ReviewedAt     time.Time `json:"reviewed_at,omitempty"` // when QC completed
}
```

**Updated Attestation struct** (registry.go):
```go
type Attestation struct {
  // ... existing fields ...
  
  // QC BINDING (new)
  QC *QCBinding `json:"qc,omitempty"` // included if REQUIRED+PASS or NOT_APPLICABLE or policy-mandated OPTIONAL
  
  // ... existing fields ...
  Signature string `json:"signature"` // signed over canonical payload INCLUDING QC
}
```

**Canonical payload update**:
```go
func (att *Attestation) canonicalPayload() string {
  qcPart := ""
  if att.QC != nil {
    qcPart = fmt.Sprintf(
      "%s|%s|%s|%s",
      att.QC.Applicability,
      att.QC.Verdict,
      att.QC.Digest,
      att.QC.Justification,
    )
  }
  
  return fmt.Sprintf(
    "%s|%s|%s|%s|%s|%s|%s|%d|%s|%d|%d|%s",
    att.SchemaVersion,
    att.CandidateDigest,
    att.MappingIdentity,
    att.MappingVersion,
    att.EvidenceReference,
    att.ApproverIdentity,
    att.Sequence,
    att.IssuedAt.UnixNano(),
    att.ApprovalTimestamp.UnixNano(),
    att.IssuerKeyID,
    qcPart,  // ← QC fields included in canonical form
  )
}
```

**Why this binding**:
- ✅ Registry signature proves "THIS profile admitted WITH THIS QC result"
- ✅ No second authority (same Ed25519 key, same registry issuer)
- ✅ QC digest immutable (proves QC record was specific, not later substituted)
- ✅ Backward compatible (QC binding optional in JSON, omitempty)
- ✅ Audit transparent (canonical payload includes QC in signed data)
- ✅ Not semantic authority (QC is complementary measurement, not claim validator)

---

## 5. Canonical Signed Payload Impact

### Current (3A Report) Registry Signature
```
SchemaVersion|CandidateDigest|MappingIdentity|MappingVersion|
EvidenceReference|ApproverIdentity|Sequence|IssuedAt|
ApprovalTimestamp|IssuerKeyID
```

### Revised (3B) Registry Signature
```
SchemaVersion|CandidateDigest|MappingIdentity|MappingVersion|
EvidenceReference|ApproverIdentity|Sequence|IssuedAt|
ApprovalTimestamp|IssuerKeyID|QCApplicability|QCVerdict|QCDigest|QCJustification
```

**Impact on verification**:
- **Existing verifiers** (no QC support): Will fail if canonical payload includes QC fields (signature mismatch)
- **Updated verifiers** (QC support): Will verify QC binding alongside profile and approval digests
- **Backward compatibility**: Requires schema version bump or conditional payload logic

**Recommended approach**:
- **Option A**: Bump schema version (breaking change)
  - `SchemaV2 = "otris.registry.admission.v2"`
  - Existing verifiers reject v2 (they don't understand QC fields)
  - New verifiers accept v1 and v2
  - **Pro**: Clean, explicit versioning. **Con**: Breaks existing verifier deployments

- **Option B**: Conditional payload (non-breaking)
  - If `att.QC != nil`, include in canonical payload
  - If `att.QC == nil`, omit (backward compatible with v1 verifiers)
  - Verifier logic: if QC present in JSON, verify its presence in signature; if absent, accept signature without QC
  - **Pro**: Backward compatible. **Con**: Verifier logic more complex

**RECOMMENDATION**: **Option B** (conditional payload):
- Signatures created WITH QC only include QC in payload if QC != nil
- Signatures created WITHOUT QC (OPTIONAL non-PASS) remain unchanged
- Schema version stays V1
- Existing verifiers continue to work for profiles without QC
- New verifiers can verify QC binding when present

**Conditional canonical payload**:
```go
func (att *Attestation) canonicalPayload() string {
  payload := fmt.Sprintf(
    "%s|%s|%s|%s|%s|%s|%d|%d|%d|%s",
    att.SchemaVersion,
    att.CandidateDigest,
    att.MappingIdentity,
    att.MappingVersion,
    att.EvidenceReference,
    att.ApproverIdentity,
    att.Sequence,
    att.IssuedAt.UnixNano(),
    att.ApprovalTimestamp.UnixNano(),
    att.IssuerKeyID,
  )
  
  // Include QC only if present (backward compat)
  if att.QC != nil {
    payload = fmt.Sprintf("%s|%s|%s|%s|%s",
      payload,
      att.QC.Applicability,
      att.QC.Verdict,
      att.QC.Digest,
      att.QC.Justification,
    )
  }
  
  return payload
}
```

---

## 6. Backward Compatibility Impact

### What breaks / what's compatible:

| Component | Existing Behavior | Revised Behavior | Impact | Mitigation |
|---|---|---|---|---|
| **governed-pipeline (D3G)** | No QC flags | +3 optional QC flags | ✅ Additive (backward compat) | Default flags absent |
| **BuildFromMappingDefinition** | No QC parameter | +optional qcRecord | ✅ Additive (backward compat) | Default qcRecord=nil |
| **Profile struct** | 47 fields | +QCDigest, QCVerdictRef | ✅ Additive (backward compat) | omitempty = skipped if absent |
| **Registry Attestation** | Fixed payload | Conditional QC in payload | ⚠️ Conditional (requires careful handling) | Include QC only if present, canonical logic handles both |
| **Registry verifier** | Verifies 10-field payload | Verifies 10 or 14-field payload | ⚠️ Logic update | Verifier must check QC presence and parse accordingly |
| **Approval.Review()** | Takes profile + mapping | Accepts same (no change) | ✅ No impact | No change to signature |
| **Profile YAML output** | 47 fields | +qc_digest, qc_verdict_ref | ✅ Additive (backward compat) | YAML omits if absent |
| **Existing profiles (D3G output)** | Already created | No retroactive change | ✅ Safe | New profiles include QC if present |
| **Existing attestations (D3J)** | Already signed | No retroactive signature | ✅ Safe | New attestations include QC if present |

### Backward Compatibility Strategy

**For Phase 3B**:
1. **All QC inputs optional** in CLI (flags absent by default)
2. **QC record optional** in BuildFromMappingDefinition (qcRecord parameter defaults to nil)
3. **Profile QC fields omitempty** (skipped in JSON/YAML if nil)
4. **Attestation QC binding conditional** (included only if QC present and relevant)
5. **Canonical payload includes QC** only if att.QC != nil (verifier must handle both)

**Testing**:
- ✅ D3G without QC flags → profile builds identically to today
- ✅ D3J without QC flags → attestation signed same as today (no QC fields)
- ✅ D3G with QC policy but non-PASS OPTIONAL → profile includes non-PASS verdict (audited, not binding)
- ✅ D3G with REQUIRED QC PASS → profile includes QC digest, proceeding as normal
- ✅ D3J with REQUIRED QC PASS → attestation includes QC binding in signature

**Risk**: Existing verifiers of attestations:
- ✅ Safe for attestations WITHOUT QC (same canonical payload as today)
- ⚠️ Unsafe for attestations WITH QC (different canonical payload, signature mismatch)
- **Mitigation**: Document that "attestations with QC binding require updated verifier code"

---

## 7. Exact Files to Modify

### 1. `cmd/WIJOYO_KUSUMO/main.go` (~60 lines added)

**Changes**:
- Add 3 QC input flags to `governedPipelineCmd.Flags()`:
  - `--qc-policy path` (optional, defines applicability + criteria)
  - `--qc-challenges path` (optional, reference challenge records)
  - `--qc-reference path` (optional, reference definition)

- Add new function `determineApplicability(policyPath string) qc.QCApplicability`:
  - Load JSON policy file
  - Extract and return Applicability field
  - Validate (fail if invalid)

- Add new function `loadQCPolicy(path string) (*qc.AcceptanceCriteria, error)`:
  - Load JSON policy file
  - Unmarshal to AcceptanceCriteria
  - Return with validation result

- Add new function `loadQCChallenges(path string) ([]*qc.ReferenceChallenge, error)`:
  - Load JSON challenges file
  - Unmarshal array of ReferenceChallenge
  - Validate each challenge

- Add new function `loadQCReference(path string) (*qc.ReferenceDefinition, error)`:
  - Load JSON reference file
  - Unmarshal to ReferenceDefinition
  - Validate

- Update `governedPipelineCmd.RunE()` main logic:
  - After ReviewReplay success (line 322), add [D3G-3.5] section:
    - Determine QC applicability
    - Load QC inputs (with fail-closed logic for REQUIRED)
    - Call qc.ReviewQC() if applicable
    - Check verdict (PASS only for REQUIRED, fail-closed)
    - Log result
  - Pass qcRecord to BuildFromMappingDefinition (new parameter)
  - Pass qcApplicability to admission functions (new parameter)

- Update D3G success output (line 367–374):
  - Add: `"[D3G-3.5] QC: applicability=<X>, verdict=<Y>" (if present)`

- Update D3J admission call (line 356):
  - Pass qcRecord and qcApplicability to runGovernedAdmission()

**Lines added**: ~60 (flags, 5 helper functions, main logic)

### 2. `internal/profilebuilder/profilebuilder.go` (~25 lines added)

**Changes**:
- Add to Profile struct (lines 16–47):
  ```go
  QCApplicability string `yaml:"qc_applicability,omitempty" json:"qc_applicability,omitempty"`
  QCVerdict       string `yaml:"qc_verdict,omitempty" json:"qc_verdict,omitempty"`
  QCDigest        string `yaml:"qc_digest,omitempty" json:"qc_digest,omitempty"`
  ```

- Update BuildFromMappingDefinition signature:
  ```go
  func BuildFromMappingDefinition(m *mapping.MappingDefinition, manufacturer, model, 
      transport, firmwareTested, outDir, version string, qcRecord *qc.QCKnowledgeRecord) (Profile, error)
  ```

- In function body (around line 150–170):
  - If qcRecord != nil:
    - Set profile.QCApplicability = qcRecord.Criteria.Applicability
    - Set profile.QCVerdict = string(qcRecord.Verdict)
    - Set profile.QCDigest = qcRecord.Integrity

- In AuthorityChain (around line 162–167):
  - If qcRecord != nil and qcRecord.Verdict == PASS:
    - Append: `"QC validated: digest=<16-char-prefix> by <reviewer>"`

- In YAML output (around line 195–217):
  - Add qc_applicability, qc_verdict, qc_digest to field output loop

**Lines added**: ~25 (struct fields + conditional logic)

### 3. `internal/registry/registry.go` (~50 lines added)

**Changes**:
- Add new struct QCBinding (before Attestation):
  ```go
  type QCBinding struct {
    Applicability  string    `json:"applicability"`
    Verdict        string    `json:"verdict,omitempty"`
    Digest         string    `json:"digest,omitempty"`
    Justification  string    `json:"justification,omitempty"`
    ReviewedBy     string    `json:"reviewed_by,omitempty"`
    ReviewedAt     time.Time `json:"reviewed_at,omitempty"`
  }
  ```

- Add to Attestation struct (lines 36–54):
  ```go
  QC *QCBinding `json:"qc,omitempty"`
  ```

- Update Admit() signature:
  ```go
  func Admit(
    candidate *profilebuilder.Profile,
    liveMapping *mapping.MappingDefinition,
    engineerApproval *approval.Approval,
    qcRecord *qc.QCKnowledgeRecord,  // NEW
    qcApplicability string,          // NEW
    issuerKey ed25519.PrivateKey,
    issuerKeyID string,
    sequence uint64,
  ) (*Attestation, error)
  ```

- In Admit() body, add QC binding logic (after line 66):
  - If qcApplicability == "REQUIRED" and qcRecord != nil:
    - Create QCBinding with: Applicability, Verdict=PASS only, Digest, ReviewedBy, ReviewedAt
    - Set att.QC = &binding
  - If qcApplicability == "NOT_APPLICABLE":
    - Create QCBinding with: Applicability, Justification only, ReviewedBy, ReviewedAt
    - Set att.QC = &binding
  - If qcApplicability == "OPTIONAL" and policy requires binding:
    - Create QCBinding with: Applicability, Verdict, Digest (if PASS), ReviewedBy, ReviewedAt
    - Set att.QC = &binding

- Update canonicalPayload() method:
  - Change signature to work with conditional QC (see section 5)
  - If att.QC != nil: append QC fields to canonical string
  - Otherwise: canonical string unchanged (backward compat)

- Update signature computation (around line ~140):
  - Verify canonical payload includes QC if att.QC != nil

**Lines added**: ~50 (QCBinding struct, Admit signature, QC binding logic, canonical payload update)

### 4. `cmd/WIJOYO_KUSUMO/governed_handoff.go` (~40 lines added)

**Changes**:
- Update `runGovernedAdmission()` signature:
  ```go
  func runGovernedAdmission(profile *profilebuilder.Profile, m *mapping.MappingDefinition, 
      qcRecord *qc.QCKnowledgeRecord, qcApplicability string, 
      admission admissionInputs, buildDir, outDir string) error
  ```

- In runGovernedAdmission() body:
  - Call Approval.Review() (unchanged, no QC parameter needed)
  - Call Registry.Admit() with qcRecord and qcApplicability parameters (line ~180):
    ```go
    attestation, err := registry.Admit(profile, m, engineerApproval, 
        qcRecord, qcApplicability, issuerKey, issuerKeyID, admission.sequence)
    ```

- Update D3J success output (line ~220):
  - Add: `"QC: applicability=<X>, digest=<Y> (in attestation signature)" (if present)`

**Lines added**: ~40 (signature, Admit call with new params, output update)

### 5. `internal/qc/qc.go` (new file, or update review.go) (~10 lines)

**Changes** (if new exports needed):
- Ensure ReviewQC() returns qc.QCKnowledgeRecord with all required fields
- Ensure verdict enum values accessible (VerdictPass, VerdictFail, etc.)
- Verify QCKnowledgeRecord marshals correctly to JSON

**Lines added**: ~10 (minimal, mostly exists in PHASE 2 implementation)

### 6. Tests (new file: `cmd/WIJOYO_KUSUMO/main_qc_test.go`) (~150 lines)

**Creation** (new test file):
- Integration tests for governed-pipeline + QC logic

**Lines added**: ~150 (detailed in section 8)

---

## 8. Exact Tests Required

### Test File: `cmd/WIJOYO_KUSUMO/main_qc_test.go` (new)

**Test 1: REQUIRED QC PASS → Profile succeeds**
```
Inputs:
  - MappingDefinition CANDIDATE
  - Evidence policy
  - Replay ground truth
  - QC policy with Applicability=REQUIRED
  - QC challenges (PASS expected)
  - QC reference definition
  
Expected:
  - ReviewEvidence succeeds (EVIDENCE_APPROVED)
  - ReviewReplay succeeds (REPLAY_VALIDATED)
  - ReviewQC called, returns PASS
  - BuildFromMappingDefinition succeeds
  - profile.QCDigest populated
  - profile.QCApplicability = "REQUIRED"
  - No error returned
```

**Test 2: REQUIRED QC FAIL → Pipeline stops (fail-closed)**
```
Inputs:
  - Same as Test 1, but QC challenges designed to FAIL
  
Expected:
  - ReviewEvidence succeeds
  - ReviewReplay succeeds
  - ReviewQC called, returns FAIL
  - Error returned: "[D3G-3.5-C] REQUIRED QC FAILED (fail-closed)"
  - BuildFromMappingDefinition NOT called
  - Pipeline exits with error
```

**Test 3: REQUIRED QC SUSPECT → Pipeline stops (fail-closed)**
```
Inputs:
  - Same as Test 1, but QC challenges designed to SUSPECT
  
Expected:
  - ReviewQC called, returns SUSPECT
  - Error returned: "[D3G-3.5-C] REQUIRED QC SUSPECT (fail-closed)"
  - Pipeline stops
```

**Test 4: REQUIRED QC INSUFFICIENT_EVIDENCE → Pipeline stops (fail-closed)**
```
Inputs:
  - Same as Test 1, but QC challenges insufficient
  
Expected:
  - ReviewQC called, returns INSUFFICIENT_EVIDENCE
  - Error returned: "[D3G-3.5-C] REQUIRED QC INSUFFICIENT_EVIDENCE (fail-closed)"
  - Pipeline stops
```

**Test 5: REQUIRED QC missing challenges → Pipeline stops (fail-closed)**
```
Inputs:
  - Same as Test 1, but --qc-challenges flag absent
  
Expected:
  - Error returned: "[D3G-3.5-C] REQUIRED QC: challenges load failed"
  - Pipeline stops before ReviewQC called
```

**Test 6: REQUIRED QC missing reference → Pipeline stops (fail-closed)**
```
Inputs:
  - Same as Test 1, but --qc-reference flag absent
  
Expected:
  - Error returned: "[D3G-3.5-C] REQUIRED QC: reference definition load failed"
  - Pipeline stops
```

**Test 7: OPTIONAL QC absent → Profile succeeds (backward compat)**
```
Inputs:
  - No QC flags supplied
  
Expected:
  - ReviewEvidence succeeds
  - ReviewReplay succeeds
  - ReviewQC NOT called
  - profile.QCDigest empty
  - profile.QCApplicability empty
  - BuildFromMappingDefinition succeeds
  - No error
```

**Test 8: OPTIONAL QC FAIL → Profile succeeds (audited)**
```
Inputs:
  - QC policy with Applicability=OPTIONAL
  - QC challenges designed to FAIL
  
Expected:
  - ReviewQC called, returns FAIL
  - profile.QCDigest populated (record created)
  - profile.QCApplicability = "OPTIONAL"
  - profile.QCVerdict = "FAIL"
  - BuildFromMappingDefinition succeeds (non-PASS not blocking)
  - Audit log shows: "[D3G-3.5-C] OPTIONAL QC FAILED (proceeding with audit)"
  - No error
```

**Test 9: OPTIONAL QC SUSPEND → Profile succeeds (audited)**
```
Inputs:
  - Same as Test 8, but SUSPECT result
  
Expected:
  - ReviewQC called, returns SUSPECT
  - profile.QCVerdict = "SUSPECT"
  - BuildFromMappingDefinition succeeds
  - Audit log shows: "[D3G-3.5-C] OPTIONAL QC SUSPECT (proceeding with audit)"
```

**Test 10: NOT_APPLICABLE QC → Profile succeeds (justification preserved)**
```
Inputs:
  - QC policy with Applicability=NOT_APPLICABLE
  - Justification: "Categorical data, no numeric reference"
  
Expected:
  - ReviewQC NOT called
  - profile.QCApplicability = "NOT_APPLICABLE"
  - profile.QCDigest empty
  - Audit log shows: "[D3G-3.5-D] QC NOT_APPLICABLE (will be audited with justification)"
  - BuildFromMappingDefinition succeeds
```

**Test 11: NOT_APPLICABLE without justification → Pipeline stops (fail-closed)**
```
Inputs:
  - QC policy with Applicability=NOT_APPLICABLE
  - Justification empty
  
Expected:
  - Error returned: "[D3G-3.5-D] NOT_APPLICABLE requires explicit justification (fail-closed)"
  - Pipeline stops
```

**Test 12: D3J admission with REQUIRED QC PASS → Attestation includes QC binding**
```
Inputs:
  - Same as Test 1 (REQUIRED QC PASS)
  - Add approval flags (--approver, --decision, --issuer-key, --issuer-key-id, --sequence)
  
Expected:
  - D3G path succeeds with qcRecord (REQUIRED PASS)
  - Approval.Review succeeds
  - Registry.Admit called with qcRecord and qcApplicability
  - Attestation.QC populated with: Applicability, Verdict, Digest, ReviewedBy, ReviewedAt
  - Canonical payload includes QC fields
  - Signature computed over canonical payload WITH QC
  - Attestation written to admission.json
```

**Test 13: D3J admission with OPTIONAL QC FAIL → Attestation includes verdict but not digest**
```
Inputs:
  - Same as Test 8 (OPTIONAL QC FAIL)
  - Add approval flags
  
Expected:
  - D3G path succeeds with qcRecord (OPTIONAL FAIL, non-blocking)
  - Approval.Review succeeds
  - Registry.Admit called with qcRecord and qcApplicability
  - Attestation.QC populated with: Applicability=OPTIONAL, Verdict=FAIL, Digest empty
  - Canonical payload includes QC fields (Verdict=FAIL means not signing QC as governing)
  - Attestation written
```

**Test 14: D3J admission with NOT_APPLICABLE QC → Attestation includes justification**
```
Inputs:
  - Same as Test 10 (NOT_APPLICABLE)
  - Add approval flags
  
Expected:
  - Approval.Review succeeds
  - Registry.Admit called with qcRecord=nil, qcApplicability="NOT_APPLICABLE"
  - Attestation.QC populated with: Applicability=NOT_APPLICABLE, Justification=<text>
  - Canonical payload includes QC fields
  - Signature proves QC justification was part of admission decision
```

**Test 15: D3J admission without QC → Attestation unchanged (backward compat)**
```
Inputs:
  - No QC flags
  - Add approval flags
  
Expected:
  - D3G path succeeds with qcRecord=nil
  - Approval.Review succeeds
  - Registry.Admit called with qcRecord=nil, qcApplicability=""
  - Attestation.QC empty (nil)
  - Canonical payload unchanged from 3A (no QC fields)
  - Signature matches pre-3B format (backward compat)
  - Existing verifiers can verify this attestation
```

**Test 16: Profile with REQUIRED QC PASS includes digest in authority chain**
```
Inputs:
  - Same as Test 1
  
Expected:
  - profile.AuthorityChain includes entry:
    `"QC validated: digest=<16-char-prefix> by governed-pipeline"`
  - Entry inserted in correct position (after Replay, before Mapping)
```

**Test 17: Profile with test fixture QC → Pipeline stops (fail-closed)**
```
Inputs:
  - QC policy with Applicability=REQUIRED
  - QC record with IsTestFixture=true
  
Expected:
  - ReviewQC called, returns record with IsTestFixture=true
  - Pipeline stops before BuildFromMappingDefinition:
    `"[D3G-3.5-C] REQUIRED QC: test fixture cannot reach production (fail-closed)"`
```

**Test 18: MappingDefinition unchanged after ReviewQC**
```
Inputs:
  - Same as Test 1
  
Expected:
  - After ReviewQC succeeds:
    - m.Status() still returns "REPLAY_VALIDATED" (not changed)
    - m.Claim() unchanged
    - m.Provenance, m.ProvenanceAuthor unchanged
    - All other fields of m unchanged
  - Verify m.Validate() passes (integrity intact)
```

**Test 19: QC policy validation enforced**
```
Inputs:
  - QC policy with:
    - Applicability=REQUIRED, but no criteria (missing MaxCV, etc.)
    - MaxCV = -1.0 (invalid, negative)
  
Expected:
  - Policy.Validate() fails
  - Error returned: "[D3G-3.5-B] QC policy validation failed"
  - Pipeline stops
```

**Test 20: QC challenges and reference definition must match**
```
Inputs:
  - QC policy with Applicability=REQUIRED
  - QC challenges specify mapping_id="siemens-infinity-hr"
  - QC reference defines mapping_id="siemens-infinity-hr-v2" (mismatch)
  
Expected:
  - ReviewQC called
  - Challenge.BindsTo(reference) fails (mapping mismatch)
  - ReviewQC returns error
  - Pipeline stops: "[D3G-3.5-C] REQUIRED QC review failed"
```

**Total Tests**: 20 (covers all applicability modes, fail-closed conditions, binding scenarios, backward compat)

---

## Summary

### Governed Call Sequence (Revised)
```
ReviewEvidence → ReviewReplay → [QC APPLICABILITY GATE] → ReviewQC (if REQUIRED/OPTIONAL)
→ [FAIL-CLOSED CHECK if REQUIRED] → BuildFromMappingDefinition → Profile
→ [Optional: Approval.Review] → [Optional: Registry.Admit with QC binding]
```

### Applicability Decision Table
| Mode | Input Required | Call ReviewQC | PASS | FAIL | NOT_APPLICABLE |
|------|---|---|---|---|---|
| REQUIRED | All | YES | ✅ PROCEED | ❌ STOP | ❌ ERROR |
| OPTIONAL | None | Maybe | ✅ PROCEED (audited) | ✅ PROCEED (audited) | ⚠️ Audited |
| NOT_APPLICABLE | Justification | NO | N/A | N/A | ✅ PROCEED |

### Fail-Closed Stops (REQUIRED)
1. Missing inputs → STOP
2. ReviewQC error → STOP
3. Verdict = FAIL, SUSPECT, INSUFFICIENT_EVIDENCE → STOP
4. Test fixture detected → STOP

### QC Binding
- REQUIRED + PASS: QCBinding included in Attestation signature
- NOT_APPLICABLE: QCBinding included with justification in signature
- OPTIONAL non-PASS: Not binding (audited only)

### Files to Modify
1. `main.go` (~60 lines)
2. `profilebuilder.go` (~25 lines)
3. `registry.go` (~50 lines)
4. `governed_handoff.go` (~40 lines)
5. `qc.go` (~10 lines minimal)
6. `main_qc_test.go` (new, ~150 lines)

**Total**: ~335 lines across 6 files

### Tests Required
20 comprehensive tests covering:
- All applicability modes (REQUIRED, OPTIONAL, NOT_APPLICABLE)
- All fail-closed conditions
- QC binding in attestation
- Backward compatibility
- Test fixture rejection
- MappingDefinition integrity

---

**STATUS**: DESIGN SPECIFICATION COMPLETE — READY FOR PHASE 3B IMPLEMENTATION


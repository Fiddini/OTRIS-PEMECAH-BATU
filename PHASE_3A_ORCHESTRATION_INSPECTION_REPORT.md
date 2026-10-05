# PHASE 3A: Governed Pipeline Integration Design — Orchestration Inspection Report

**Baseline**: commit 6c3a02f (Reference & QC Intelligence Engine v1.0)  
**Inspection Date**: 2026-10-05  
**Status**: INSPECTION ONLY — NO CODING YET

---

## A. Exact Existing Production Orchestration Entrypoint

**Primary Entry**: `cmd/WIJOYO_KUSUMO/main.go` line 202–376  
**Command**: `governed-pipeline` (Cobra command)

**Location**: `./cmd/WIJOYO_KUSUMO/main.go:202:governedPipelineCmd := &cobra.Command`

**Key invocation** (main.go:213–365):
```
RunE: func(cmd *cobra.Command, args []string) error {
  // D3G GOVERNED PIPELINE (lines 271–375)
  // D3J GOVERNED HANDOFF (lines 226–240, 354–365) [OPTIONAL admission mode]
}
```

**Two modes**:
1. **D3G mode** (default): `ReviewEvidence → ReviewReplay → BuildFromMappingDefinition` → exit  
   - Output: `profile.yaml` with status CANDIDATE (requires engineer approval for next stage)
   - No admission flags supplied

2. **D3J mode** (optional): D3G + `Approval.Review() → Registry.Admit()` → signed admission  
   - Output: `profile.yaml` + `approval.json` + registry attestation
   - Requires: `--approver`, `--decision`, `--issuer-key`, `--issuer-key-id`, `--sequence`

---

## B. Exact Call Sequence Today

### Step 1: Load MappingDefinition
**Location**: `main.go:274–289`
```go
mappingData, err := os.ReadFile(mappingPath)
m := &mapping.MappingDefinition{}
json.Unmarshal(mappingData, m)
status, err := m.Status()  // Must be CANDIDATE
```
**Precondition**: Status = `CANDIDATE` (only)  
**No modifications to m at this step**

### Step 2: ReviewEvidence (Semantic Gate)
**Location**: `main.go:292–303`
```go
mapping.ReviewEvidence(m, evidencePath, evidenceReportPath)
// m.Status() transitions CANDIDATE → EVIDENCE_APPROVED
```
**Signature**: `func ReviewEvidence(m *MappingDefinition, evidencePath, reportPath string) error`  
**Called from**: `internal/mapping/evidence.go`

**Internal sequence** (evidence.go):
1. Verify status = CANDIDATE
2. Call `evidence.ReviewClaim(claim, evidencePath)` → validate semantic meaning
3. Verify Claim unchanged and authority binding intact
4. Write `evidence.Report` to reportPath (audit)
5. **Transition**: `m.status = StatusEvidenceApproved` (mutates m in-place)
6. Set timestamps: `m.EvidenceApprovedAt`, `m.EvidenceGateReportRef`

**Gate**: Evidence report Verdict must = "CLAIM_APPROVED" (fail-closed)  
**Authority**: VENDOR_MANUAL evidence required (≥2 items, ≥1 VENDOR_MANUAL)

### Step 3: ReviewReplay (Replay Validation Gate)
**Location**: `main.go:306–322`
```go
replayReport, err := mapping.ReviewReplay(m, inputDir, count, replayOpts, replayReportPath)
// m.Status() transitions EVIDENCE_APPROVED → REPLAY_VALIDATED
```
**Signature**: `func ReviewReplay(m *MappingDefinition, inDir string, count int, opts replay.ReplayOptions, reportPath string) (replay.Report, error)`  
**Called from**: `internal/mapping/replay.go:19–69`

**Internal sequence** (replay.go):
1. Verify status = EVIDENCE_APPROVED (fail-closed)
2. Snapshot Claim before measurement: `before := m.Claim()`
3. Call `replay.MeasureClaim(before, inDir, count, opts)` → measure claim 1000× against ground truth
4. Verify Claim unchanged and authority binding intact
5. Validate replay.Report: `Verdict = "MEASURED_MATCH"`, `FalseData = 0`, `FalseDataRate = 0.0` (fail-closed)
6. **Transition**: `m.status = StatusReplayValidated` (mutates m in-place)
7. Set timestamps: `m.ReplayValidatedAt = time.Now().UTC()`
8. Write `replay.Report` to reportPath (audit)
9. Return: `report.Verdict = "READY_FOR_PROFILE_BUILDER"`

**Gate**: Replay must prove `false_data_rate = 0.0` (fail-closed)  
**Authority**: Ground truth provided by test harness, not by claim

### Step 4: BuildFromMappingDefinition (Profile Builder)
**Location**: `main.go:342–351`
```go
profile, err := profilebuilder.BuildFromMappingDefinition(m, manufacturer, model, transport, 
    firmwareTested, buildDir, version)
// Output: profile.yaml (CANDIDATE status)
```
**Signature**: `func BuildFromMappingDefinition(m *mapping.MappingDefinition, ...) (Profile, error)`  
**Called from**: `internal/profilebuilder/profilebuilder.go:74–227`

**Internal sequence** (profilebuilder.go):
1. Verify status = REPLAY_VALIDATED (fail-closed)
2. Validate MappingDefinition integrity: `m.Validate()`
3. Extract Claim from m: `claim := m.Claim()`
4. Require device context: ParserIdentity, DeviceFingerprint, DeviceIdentityRef (fail-closed)
5. Build Profile struct:
   - `ProfileID = fmt.Sprintf("%s_%s_%s_v%s", manufacturer, model, firmware, version)`
   - `GovernanceStatus = "CANDIDATE"` (always)
   - `CapabilitySupport = "SUPPORTED"`
   - `CompatibilityStatus = "REAL_DEVICE_VALIDATED"`
   - Reference previous authorities in `AuthorityChain`:
     ```
     "Evidence approved: <evidence_reference>",
     "Replay validated: false_data=0.0 at <timestamp>",
     "Mapping: <mapping_id> v<mapping_version> by <author>",
     "Parser: <parser_id>"
     ```
6. Write profile.yaml and profile.json to output directory
7. Return Profile (no mutation to m)

**Schema**: See `internal/profilebuilder/profilebuilder.go:16–47` (Profile struct)

### Step 5a: Engineer Approval (D3G mode exits here)
**Location**: `main.go:367–374` (D3G success path)
```go
fmt.Println("=== D3G GOVERNED PIPELINE SUCCEEDED ===")
fmt.Printf("Status: CANDIDATE (requires engineer approval for next stage)\n")
return nil
```
**No approval record created in D3G mode**

### Step 5b: Engineer Approval + Registry Admission (D3J mode only)
**Location**: `main.go:354–365`
```go
if admissionMode {
  if err := runGovernedAdmission(&profile, m, admission, buildDir, outDir); err != nil {
    return fmt.Errorf("[D3J] GOVERNED HANDOFF FAILED: %w", err)
  }
  return nil
}
```

**Subroutine**: `runGovernedAdmission()` (cmd/WIJOYO_KUSUMO/governed_handoff.go, approximately line ~150–250)

**Internal sequence** (governed_handoff.go):
1. **Approval.Review()**
   - Call: `approval.Review(&profile, m, approverIdentity, decision)`
   - Returns: `*approval.Approval` (audit record, not credential)
   - Signature capture: candidate digest computed from profile + live mapping
   - Stores: approver, decision (APPROVE|REJECT), timestamps, evidence reference
   
2. **Registry.Admit()**
   - Call: `registry.Admit(&profile, m, engineerApproval, issuerKey, issuerKeyID, sequence)`
   - Returns: `*registry.Attestation` (signed artifact)
   - Verifies: approval.Validate() (live mapping + candidate unchanged)
   - Computes: canonical payload bytes
   - Signs: Ed25519 signature over payload (domain = "OTRIS-REGISTRY-ADMISSION")
   - Stores: schema_version, admission_status, profile info, candidate digest, evidence reference, replay timestamp, signature
   
3. **Write artifacts** to output directory
   - `profile.yaml` (same as D3G output)
   - `approval.json` (approval.Approval marshaled)
   - Registry attestation (registry.Attestation marshaled)

4. **Return success**
   ```
   fmt.Printf("Published: %s\n", outDir)
   fmt.Printf("Approval: %s by %s (approval.json is audit only)\n", approval.StatusEngineerApproved, approver)
   fmt.Printf("Registry: %s sequence %d issuer %s\n", registry.StatusRegistryAdmitted, sequence, issuerKeyID)
   ```

---

## C. Exact Insertion Point for ReviewQC()

**Key Finding**: ReviewQC() must fit BETWEEN ReviewReplay completion and BuildFromMappingDefinition entry.

**Insertion Location**: `main.go:305–341` (between replay success and profile builder invocation)

**Current code structure** (main.go):
```go
// Line 305–322: ReviewReplay (m transitions EVIDENCE_APPROVED → REPLAY_VALIDATED)
replayReport, err := mapping.ReviewReplay(m, inputDir, count, replayOpts, replayReportPath)
if err != nil { return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: replay validation rejected: %w", err) }
status, err = m.Status()
if err != nil || status != mapping.StatusReplayValidated {
  return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: replay validation did not transition status, got %s: %v", status, err)
}
if replayReport.Verdict != "READY_FOR_PROFILE_BUILDER" {
  return fmt.Errorf("[D3G] GOVERNED PIPELINE FAILED: replay verdict is %s, expected READY_FOR_PROFILE_BUILDER", replayReport.Verdict)
}
fmt.Printf("[D3G-3] Replay validated: %s (false_data=%.1f)\n", replayReportPath, replayReport.FalseDataRate)

// ← INSERTION POINT: ReviewQC() would go here (new [D3G-3.5] step)

// Line 342–346: BuildFromMappingDefinition (m status remains REPLAY_VALIDATED)
fmt.Printf("[D3G-4] Building Integration Profile Candidate\n")
profile, err := profilebuilder.BuildFromMappingDefinition(m, manufacturer, model, transport, firmwareTested, buildDir, version)
```

**Why this point?**
- ✅ MappingDefinition is REPLAY_VALIDATED (precondition for QC optional inputs)
- ✅ ReplayReport with ground truth available (can supply to QC if needed)
- ✅ No modification to m.Status required (QC returns separate artifact)
- ✅ Profile builder can receive optional QCKnowledgeRecord as context
- ✅ BuildFromMappingDefinition contracts unchanged (MappingDefinition input/output identical)

---

## D. How QC Applicability Should Be Supplied

**Three patterns**:

### Pattern 1: CLI Flag for QC Policy (Simplest, Recommended)
Add optional flag to `governed-pipeline` command:
```go
governedPipelineCmd.Flags().String("qc-policy", "", "path to QC policy JSON (optional, defines applicability)")
```

Parse and load policy:
```go
var qcPolicy *qc.AcceptanceCriteria
if qcPolicyPath != "" {
  policyData, _ := os.ReadFile(qcPolicyPath)
  json.Unmarshal(policyData, &qcPolicy)
  if err := qcPolicy.Validate(); err != nil {
    return fmt.Errorf("[D3G-3.5] QC policy validation failed: %w", err)
  }
}
```

At insertion point:
```go
// [D3G-3.5] Review QC (optional)
var qcRecord *qc.QCKnowledgeRecord
if qcPolicy != nil {
  fmt.Printf("[D3G-3.5] Reviewing QC against policy %s\n", qcPolicyPath)
  qcRecord, err := qc.ReviewQC(
    m.MappingIdentity, m.MappingVersion, m.DeviceFingerprint,
    "governed-pipeline", engineerApprover, version,
    challenges, refDef, qcPolicy,
  )
  if err != nil && qcPolicy.Applicability == qc.ApplicabilityRequired {
    return fmt.Errorf("[D3G-3.5] REQUIRED QC failed: %w", err)
  }
  if qcRecord != nil {
    fmt.Printf("[D3G-3.5] QC completed: %s\n", qcRecord.Verdict)
  }
}
```

### Pattern 2: Hardcoded Policy per Mapping (Less Flexible)
Embed QC applicability in MappingDefinition:
```go
type MappingDefinition struct {
  // ... existing fields ...
  QCApplicability string `json:"qc_applicability"` // "REQUIRED", "OPTIONAL", "NOT_APPLICABLE"
  QCJustification string `json:"qc_justification"` // when NOT_APPLICABLE
}
```

Extract at insertion point:
```go
var qcPolicy *qc.AcceptanceCriteria
if m.QCApplicability != "" {
  qcPolicy = &qc.AcceptanceCriteria{
    Applicability: m.QCApplicability,
    Justification: m.QCJustification,
    Authority: "embedded in mapping definition",
  }
}
```

### Pattern 3: External QC Record Mapping (Future Enhancement)
Pair QC policy with reference challenges via structured metadata file:
```json
{
  "mapping_identity": "siemens-infinity-hr",
  "mapping_version": "1.0.0",
  "qc": {
    "applicability": "REQUIRED",
    "reference_challenges": [
      { "level": "LOW", "raw_value": "55" },
      { "level": "NORMAL", "raw_value": "75" },
      { "level": "HIGH", "raw_value": "95" }
    ],
    "criteria": {
      "max_cv_percent": 5.0,
      "min_observations": 10
    }
  }
}
```

**RECOMMENDATION**: Start with **Pattern 1** (CLI flag) for Phase 3B:
- Keeps MappingDefinition contract unchanged ✓
- Allows policy variation without rebuild ✓
- Applicability explicit (fail-closed) ✓
- Easy to test with fixtures ✓

---

## E. How QCKnowledgeRecord Should Be Referenced Downstream

### Architecture Decision
QCKnowledgeRecord is a **separate immutable governed artifact**, NOT embedded in profile or approval.

**Why separate?**
- Profile schema should not grow with QC history
- Approval/Registry attest to profile, not to QC measurement details
- QC can be optional; profile must work without it
- Future QC results for same profile should not invalidate approval

### Reference Strategy

**Option 1: Digest-only reference (Minimal, Recommended)**
Profile references QC via digest only (no data):
```go
type Profile struct {
  // ... existing fields ...
  QCDigest string `json:"qc_digest,omitempty"` // SHA256 of QCKnowledgeRecord.Integrity
  QCVerdictRef string `json:"qc_verdict_ref,omitempty"` // "PASS", "FAIL", "NOT_APPLICABLE" (summary only)
}
```

At build time:
```go
if qcRecord != nil {
  profile.QCDigest = qcRecord.Integrity
  profile.QCVerdictRef = string(qcRecord.Verdict)
}
```

AuthorityChain includes:
```go
if qcRecord != nil && qcRecord.Verdict == qc.VerdictPass {
  profile.AuthorityChain = append(profile.AuthorityChain,
    fmt.Sprintf("QC validated: digest=%s by %s", qcRecord.Integrity[:16], qcRecord.Reviewer),
  )
}
```

**Option 2: Full path reference (Traceable)**
Profile stores path to separate QC artifact:
```go
type Profile struct {
  QCRecordPath string `json:"qc_record_path,omitempty"`
}
```

At build time:
```go
if qcRecord != nil {
  qcPath := filepath.Join(buildDir, "qc_record.json")
  qcData, _ := json.MarshalIndent(qcRecord, "", "  ")
  os.WriteFile(qcPath, qcData, 0644)
  profile.QCRecordPath = qcPath
}
```

**Option 3: No reference, write alongside (Decoupled)**
QC record written to output directory separately:
```
buildDir/
  ├── profile.yaml
  ├── profile.json
  ├── qc_record.json (separate, no reference in profile)
  ├── qc_reference_challenges.json
  └── approval.json (if admission mode)
```

**RECOMMENDATION**: **Option 1** (digest-only):
- Profile remains lean and self-contained ✓
- QC verdict visible in authority chain without data bloat ✓
- Integrity checkable without full artifact ✓
- Future-proof for QC result versioning ✓

---

## F. Whether Profile Schema Requires Minimal Immutable QC Reference/Digest

**Answer**: YES, minimal reference recommended.

**Minimal addition to Profile struct**:
```go
type Profile struct {
  // ... existing ~47 fields ...
  QCDigest     string `yaml:"qc_digest,omitempty" json:"qc_digest,omitempty"`
  QCVerdictRef string `yaml:"qc_verdict_ref,omitempty" json:"qc_verdict_ref,omitempty"`
  // No QC history, no mutable data, no embedding
}
```

**Schema change impact**:
- ✅ Backward compatible (omitempty)
- ✅ Non-invasive (2 optional fields)
- ✅ Audit-safe (digest immutable)
- ✅ Version-safe (verdictRef is enum string, not complex object)

**YAML output example**:
```yaml
profile_id: "siemens-infinity-147842_001_v1.0.0"
version: "1.0.0"
# ... existing fields ...
qc_digest: "a3f4b8c2e1d9f6..."
qc_verdict_ref: "PASS"
authority_chain:
  - "Evidence approved: <evidence_reference>"
  - "Replay validated: false_data=0.0 at 2026-10-05T12:34:56Z"
  - "QC validated: digest=a3f4b8c2e1d9 by governed-pipeline"
  - "Mapping: siemens-infinity-hr v1.0.0 by author"
  - "Parser: parser_id"
```

---

## G. Whether Registry Attestation Requires QC Digest Binding

**Answer**: OPTIONAL but recommended as audit enhancement.

**Current Attestation schema** (registry.go:36–54):
```go
type Attestation struct {
  SchemaVersion     string
  AdmissionStatus   string
  ProfileID         string
  ProfileVersion    string
  CandidateDigest   string  // ← profile digest
  MappingIdentity   string
  MappingVersion    string
  EvidenceReference string
  ReplayValidatedAt time.Time
  MappingProvenance string
  ProvenanceAuthor  string
  ApproverIdentity  string
  ApprovalTimestamp time.Time
  IssuerKeyID       string
  IssuedAt          time.Time
  Sequence          uint64
  Signature         string  // ← signed over canonical payload
}
```

**Option A: No QC binding (minimal change)**
- Registry attestation remains QC-agnostic
- Verifier can optionally check QC digest separately
- **Risk**: Someone could substitute a FAIL QC record and claim it was PASS

**Option B: Optional QC binding (audit transparency)**
Add optional fields:
```go
type Attestation struct {
  // ... existing fields ...
  QCDigest   string `json:"qc_digest,omitempty"`
  QCVerdict  string `json:"qc_verdict_ref,omitempty"`
  QCReviewedBy string `json:"qc_reviewed_by,omitempty"`
}
```

Attestation.Signature includes QC fields in canonical payload:
```go
canonicalPayload := fmt.Sprintf(
  "%s|%s|%s|%s|%s|%s|%s|%d|%s|%s|%s",
  att.SchemaVersion, att.CandidateDigest, att.MappingIdentity,
  att.MappingVersion, att.EvidenceReference, att.ApproverIdentity,
  att.QCDigest, att.Sequence, att.IssuerKeyID, att.IssuedAt.Unix(),
  att.ApprovalTimestamp.Unix(),
)
```

**RECOMMENDATION**: **Option B** (optional binding):
- ✅ Registry attestation captures QC status if present
- ✅ Backward compatible (omitempty)
- ✅ Enables auditable QC traceability
- ✅ Signature includes QC in canonical form (fraud-proof)
- ✅ Future verifiers can validate end-to-end chain: Evidence → Replay → QC → Approval → Registry

---

## H. CLI Changes Required, If Any

### Minimal Changes (Phase 3B)

**1. Add flag to `governed-pipeline` command** (main.go:377–394)
```go
governedPipelineCmd.Flags().String("qc-policy", "", 
  "path to QC policy JSON defining applicability (REQUIRED|OPTIONAL|NOT_APPLICABLE)")
governedPipelineCmd.Flags().String("qc-challenges", "", 
  "path to reference challenges JSON (required if QC policy is REQUIRED)")
governedPipelineCmd.Flags().String("qc-reference", "", 
  "path to reference definition JSON (required if QC policy is REQUIRED)")
```

**2. Optional new subcommand `qc-create` (for convenience)**
```
WIJOYO_KUSUMO qc-create \
  --mapping-id siemens-infinity-hr \
  --mapping-version 1.0.0 \
  --device-fingerprint <hash> \
  --level LOW --expected-value 55 \
  --level NORMAL --expected-value 75 \
  --level HIGH --expected-value 95 \
  --out qc_definition.json
```
Creates ReferenceDefinition + AcceptanceCriteria + sample challenges.

**3. Optional new subcommand `qc-measure` (for testing)**
```
WIJOYO_KUSUMO qc-measure \
  --mapping siemens-infinity-hr \
  --challenges qc_challenges.json \
  --reference qc_definition.json \
  --criteria qc_policy.json \
  --out qc_record.json
```
Runs ReviewQC() standalone, returns QCKnowledgeRecord.

**4. New flag for `build-profile` (legacy path, low priority)**
```go
buildCmd.Flags().String("qc-record", "", "optional QC knowledge record JSON (for legacy Build path)")
```

**5. Print QC result in output** (main.go:368–374, D3G success path)
```go
fmt.Println("=== D3G GOVERNED PIPELINE SUCCEEDED ===")
fmt.Printf("  Integration Profile Candidate: %s/profile.yaml\n", outDir)
if qcRecord != nil {
  fmt.Printf("  QC Record: %s/qc_record.json (verdict=%s)\n", outDir, qcRecord.Verdict)
}
fmt.Printf("  Status: CANDIDATE (requires engineer approval for next stage)\n")
```

**6. Update D3J output** (governed_handoff.go, line ~365)
```go
fmt.Printf("  QC: digest=%s verdict=%s (in profile and attestation)\n", qcRecord.Integrity[:16], qcRecord.Verdict)
```

---

## I. Files That Would Need Modification

### Minimum Set (Phase 3B)

**1. `cmd/WIJOYO_KUSUMO/main.go` (35 lines added)**
- Add QC policy/challenges/reference flags to `governedPipelineCmd`
- Parse flags at entry point
- Call ReviewQC() at insertion point (between replay and profile builder)
- Pass qcRecord to BuildFromMappingDefinition
- Print QC results in output summary

**2. `internal/profilebuilder/profilebuilder.go` (20 lines added)**
- Add QCDigest and QCVerdictRef to Profile struct (lines 16–47)
- Accept optional qcRecord parameter in BuildFromMappingDefinition
- Set profile.QCDigest and profile.QCVerdictRef if qcRecord present
- Include QC entry in AuthorityChain if qcRecord.Verdict == PASS
- Update YAML output to include qc fields (lines 195–217)

**3. `internal/registry/registry.go` (15 lines added)**
- Add QCDigest, QCVerdict, QCReviewedBy to Attestation struct (lines 36–54)
- Update Admit() to accept optional qcRecord
- Include QC fields in canonical payload for signature (lines ~120–140)
- Update attestation output

**4. `cmd/WIJOYO_KUSUMO/governed_handoff.go` (10 lines added)**
- Pass qcRecord to registry.Admit() if present
- Print QC digest/verdict in admission output (lines ~365)

### Total Changes: ~80 lines of code

### NO Changes to:
- ✅ internal/mapping/ (ReviewEvidence, ReviewReplay, MappingDefinition unchanged)
- ✅ internal/evidence/ (Claim, Evidence, semantic verification boundary unchanged)
- ✅ internal/replay/ (false_data_rate validation unchanged)
- ✅ internal/approval/ (Approval.Review logic unchanged; optional QC in output only)
- ✅ WIJOYO-KUSUMO source code
- ✅ otris-edge-agent source code

---

## J. Risks of Accidentally Creating Duplicate Authority

### Risk 1: QC results modify MappingDefinition status
**Scenario**: Someone adds `m.QCValidatedAt` or `m.Status = StatusQCValidated`  
**Consequence**: Parallel status machine; QC becomes authority gate; breaks governed pipeline

**Mitigation**:
- ✅ ENFORCE: ReviewQC() returns NEW QCKnowledgeRecord, NEVER mutates MappingDefinition
- ✅ TEST: Verify MappingDefinition.Status() unchanged after ReviewQC() call
- ✅ COMMENT: Add explicit note in ProfileBuilder: "QC is separate artifact; MappingDefinition status remains REPLAY_VALIDATED"

### Risk 2: QC verdict used as admission gate
**Scenario**: Engineer approval or registry admission require QC PASS  
**Consequence**: QC becomes semantic/admission authority; profile can't exist without QC

**Mitigation**:
- ✅ ENFORCE: Approval.Review() and Registry.Admit() accept profile WITH OR WITHOUT QC
- ✅ DESIGN: Profile outputs with or without QC fields (both valid)
- ✅ TEST: Build profile with qcRecord=nil; ensure no errors

### Risk 3: QC digest embedded in profile signature
**Scenario**: Approval/Registry signature includes QC digest; later QC result invalidates signature  
**Consequence**: Decoupling lost; profile and QC become inseparable

**Mitigation**:
- ✅ ENFORCE: profile.QCDigest is reference only, NOT signed
- ✅ ENFORCE: Approval signature based on profile.CandidateDigest (existing), NOT including QC
- ✅ ENFORCE: Registry signature includes QC in Attestation (optional field), not in profile
- ✅ TEST: Verify profile changeable without breaking approval/registry signatures

### Risk 4: REQUIRED QC blocks profile build
**Scenario**: BuildFromMappingDefinition fails if qcRecord == nil and qc_policy.Applicability == REQUIRED  
**Consequence**: QC becomes mandatory gate; can't test without QC setup

**Mitigation**:
- ✅ ENFORCE: BuildFromMappingDefinition accepts MappingDefinition only (QC is caller responsibility)
- ✅ ENFORCE: QC failure returns error to caller (main.go), NOT to ProfileBuilder
- ✅ TEST: Call ProfileBuilder with qcRecord=nil in both D3G and D3J paths

### Risk 5: Duplicate evidence/replay references
**Scenario**: QC references stored in both Profile and Attestation; someone "updates" one  
**Consequence**: Divergent evidence authority; which is canonical?

**Mitigation**:
- ✅ ENFORCE: Profile.EvidenceReference and Profile.ReplayReference unchanged (from MappingDefinition)
- ✅ ENFORCE: QC stores only its own digest/metadata, NOT evidence/replay references
- ✅ ENFORCE: Attestation includes QC digest + profile digest (not redundant)
- ✅ COMMENT: "Profile evidence/replay references are authoritative; QC digest is supplemental"

### Risk 6: Test fixtures contaminate production
**Scenario**: QC record with IsTestFixture=true somehow reaches approval or registry  
**Consequence**: Test data certified as production-ready

**Mitigation**:
- ✅ ENFORCE: ReviewQC() only, call IsProductionReady() before passing to profile builder
- ✅ ENFORCE: Test in main.go: `if qcRecord != nil && !qcRecord.IsProductionReady() { reject }`
- ✅ TEST: Verify test fixture rejection in pipeline integration test

---

## K. Proposed Minimal Phase 3B Implementation

### Objective
Wire ReviewQC() into `governed-pipeline` command with zero changes to existing authority gates.

### Scope
**D3G path only** (Profile builder, no approval/admission yet):
1. Accept optional QC policy + challenges + reference flags
2. Call ReviewQC() at insertion point
3. Attach QC result to profile (digest reference only)
4. Print QC status in output

**D3J path** (approval/admission):
1. Pass QC record to Approval.Review() (audit only, no validation change)
2. Include QC digest in Attestation
3. Print QC status in output

### Implementation Checklist

- [ ] **cmd/WIJOYO_KUSUMO/main.go** (35 lines)
  - Add 3 new flags: qc-policy, qc-challenges, qc-reference
  - Parse flags at entry (parse JSON files)
  - Call qc.ReviewQC() after replay success
  - Handle REQUIRED QC failure (return error to caller, don't block profile builder)
  - Handle OPTIONAL QC absence (no error, proceed)
  - Handle NOT_APPLICABLE QC (no error, preserve justification)
  - Pass qcRecord to profilebuilder.BuildFromMappingDefinition
  - Print QC result in D3G output summary

- [ ] **internal/profilebuilder/profilebuilder.go** (20 lines)
  - Update Profile struct signature: `BuildFromMappingDefinition(..., qcRecord *qc.QCKnowledgeRecord) error`
  - Add QCDigest, QCVerdictRef fields to Profile struct
  - If qcRecord != nil: set profile.QCDigest, profile.QCVerdictRef
  - Include QC entry in AuthorityChain if qcRecord.Verdict == PASS
  - Update YAML/JSON output formatting

- [ ] **internal/registry/registry.go** (15 lines)
  - Update Attestation struct: add QCDigest, QCVerdict, QCReviewedBy
  - Update Admit() signature to accept optional qcRecord
  - Include QC fields in canonical payload for signature
  - Update Attestation output

- [ ] **cmd/WIJOYO_KUSUMO/governed_handoff.go** (10 lines)
  - Pass qcRecord to registry.Admit()
  - Print QC status in D3J output

- [ ] **Tests** (50+ lines)
  - Unit: ReviewQC() returns correct verdict (existing qc_test.go)
  - Integration: main.go + ProfileBuilder + qc.ReviewQC() together
  - Scenario 1: REQUIRED QC passes → profile succeeds
  - Scenario 2: REQUIRED QC fails → governed-pipeline fails
  - Scenario 3: OPTIONAL QC absent → profile succeeds
  - Scenario 4: NOT_APPLICABLE QC → profile includes justification
  - Scenario 5: qcRecord=nil (no QC) → profile builds (backward compat)
  - Scenario 6: Test fixture QC rejected before profile builder
  - Scenario 7: D3J admission includes QC digest in attestation
  - Scenario 8: Profile QC digest unchanged after approval validation

### Deliverables
- **Code**: cmd/WIJOYO_KUSUMO/main.go, profilebuilder.go, registry.go, governed_handoff.go
- **Tests**: cmd_test.go + integration_test.go
- **Docs**: Updated README with qc-policy flag usage
- **Commit message**: `feat(pipeline): integrate QC Intelligence Engine into D3G governed-pipeline (optional REQUIRED/OPTIONAL/NOT_APPLICABLE)`

### Success Criteria
- ✅ `governed-pipeline --qc-policy policy.json <other flags>` runs successfully
- ✅ `governed-pipeline <other flags without QC>` still works (backward compat)
- ✅ Profile outputs with/without QC fields (both valid)
- ✅ MappingDefinition status unchanged (still REPLAY_VALIDATED)
- ✅ No new authority gates (QC fail → early return, not blocker)
- ✅ Test fixtures rejected before reaching profile builder
- ✅ Registry attestation includes QC digest if present
- ✅ All existing tests pass (no regression)

---

## Summary: Single-Sentence Integration Plan

**ReviewQC() inserted between ReviewReplay and BuildFromMappingDefinition as optional parallel measurement (not authority gate), returning immutable QCKnowledgeRecord referenced by digest in profile and attestation, with explicit REQUIRED/OPTIONAL/NOT_APPLICABLE applicability supplied via JSON policy file, touching only 4 source files (~80 lines total, zero changes to existing authority gates).**

---

**STATUS**: INSPECTION COMPLETE — READY FOR PHASE 3B CODING APPROVAL


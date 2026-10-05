# PHASE 1 EXPLORATION REPORT
## Reference & QC Intelligence Engine v1.0 Integration Points

**Repository**: /Users/macbook/Documents/WIJOYO-KUSUMO/OTRIS-PEMECAH-BATU-TANPA-JETSON
**Checkpoint**: Commit 19a713d (Semantic Evidence Verification Boundary)
**Date**: 2026-10-05
**Purpose**: Map exact integration points and reusable components for Reference & QC Intelligence Engine

---

## 1. EXACT INTEGRATION POINT(s)

### Primary Integration: Between Replay Validation and Profile Builder

**Current Flow** (19a713d):
```
MappingDefinition (CANDIDATE)
    ↓
ReviewEvidence() [evidence.go:67]
    → evidence.ReviewClaim()
    → Semantic Evidence Gate: ≥2 items, ≥1 VENDOR_MANUAL
    → MappingDefinition transitions to EVIDENCE_APPROVED
    ↓
ReviewReplay() [replay.go:19]
    → replay.MeasureClaim()
    → false_data_rate = 0.0 requirement
    → MappingDefinition transitions to REPLAY_VALIDATED
    ↓
BuildFromMappingDefinition() [profilebuilder.go ~line 71]
    → Requires REPLAY_VALIDATED MappingDefinition
    → Builds Integration Profile (profile.yaml)
    ↓
Approval.Review() [approval.go ~line 45]
    → Engineer decision on candidate
    → No technical validation (assumes upstream valid)
    ↓
Registry (REGISTRY_ADMITTED status)
```

**Integration Opportunity**: NEW QC layer fits BETWEEN ReviewReplay() completion and BuildFromMappingDefinition() call.

**Rationale**:
- ReviewReplay() validates false_data_rate = 0.0 (no fabrication in parsing)
- NEW layer validates consistent reference behavior (QC/statistical authority)
- BuildFromMappingDefinition() creates profile only after BOTH gates pass
- This preserves:
  - Semantic Evidence Gate (VENDOR_MANUAL requirement)
  - Replay validation (false_data_rate = 0.0)
  - Approval authority (engineer decision)
  - Registry admission (Ed25519 signature)

---

## 2. REUSABLE EXISTING COMPONENTS

### Directly Reusable (No Modification)

**2.1 MappingDefinition Structure** [mapping/definition.go]
- Identity: `MappingIdentity`, `MappingVersion`
- Explicit claim: source_protocol, source_position, source_code, source_unit, canonical_concept, canonical_unit
- Device context: `DeviceFingerprint`, `DeviceIdentityRef`, `ParserIdentity`
- Provenance chain: `Provenance`, `ProvenanceAuthor`
- Status model: `Status` (StatusCandidate, StatusEvidenceApproved, StatusReplayValidated)
- Timestamps: `CreatedAt`, `EvidenceApprovedAt`, `ReplayValidatedAt`

**Reuse for QC**:
- Extend MappingDefinition with QC metadata fields WITHOUT modifying existing fields
- Maintain same status transition rules
- Bind QC results to same mapping identity/version

**2.2 Evidence Structure** [evidence/claim.go]
- `Claim` type: comparable value binding all mapping claim fields
- `Evidence` type: evidence items with Type, Ref, Quote
- `ClaimReport` type: claim + evidence + verdict

**Reuse for QC**:
- Reuse `Claim` for reference validation binding
- Extend verdict space (add QC-specific verdict codes without removing APPROVED_FOR_MAPPING)
- Use same evidence reference structure for QC evidence tracking

**2.3 Status Model** [mapping/status.go]
- `Status` type with validation
- `CanTransitionTo()` method for enforced transitions

**Reuse for QC**:
- NEW status: `StatusQCValidated` (NEW state between REPLAY_VALIDATED and profile builder)
- Enforce `REPLAY_VALIDATED` → `StatusQCValidated` → (then profile builder)
- Or: keep profile builder flexible and use QC verdict as gate instead of new status

**2.4 Replay Report Structure** [replay/replay.go]
- `Report` type with: claim, success count, failed_parse, false_data, false_data_rate, verdict

**Reuse for QC**:
- Similar structure: QCReport with reference level, challenge results, statistics

**2.5 Fingerprint & Device Identity** [mapping/definition.go]
- `DeviceFingerprint` (structural hash from PB-10)
- `DeviceIdentityRef` (device model/vendor reference)

**Reuse for QC**:
- Reference challenges MUST bind to same fingerprint
- Fingerprint mismatch REQUIRES re-verification before reuse

**2.6 Timestamp & Provenance** [mapping/definition.go]
- ISO-8601 UTC timestamps
- `ProvenanceAuthor` string
- Narrative `Provenance` field

**Reuse for QC**:
- Same timestamp precision (UTC)
- Same author field for QC operator/system identity
- Same narrative provenance tracking

**2.7 Profile Builder Integration** [profilebuilder.go ~line 71]
- `BuildFromMappingDefinition()` requires REPLAY_VALIDATED
- Takes mapping, device/parser context, firmware, transport
- Creates Profile with MappingProvenance, EvidenceReference, ReplayReference

**Reuse for QC**:
- Pass QC verdict/report path as additional optional input
- BuildFromMappingDefinition() can optionally accept QCReport
- Add QC metadata to generated Profile if QC completed

### Conditionally Reusable (Minimal Wrapping)

**2.8 Review Functions Pattern** [mapping/evidence.go:67, mapping/replay.go:19]
- `ReviewEvidence()` governance pattern
- `ReviewReplay()` governance pattern
- Both:
  - Validate status precondition
  - Call external measurement/validation function
  - Validate definition integrity
  - Update status + approval metadata
  - Return verdict

**Reuse for QC**:
- Create similar `ReviewQC()` function following same pattern
- Validate `StatusReplayValidated` precondition
- Call external QC measurement function
- Update QC verdict fields
- Transition to new state or return verdict

---

## 3. MINIMAL PROPOSED MODULE/FILE STRUCTURE

### New Package: `internal/qc`

```
internal/qc/
├── reference.go           ← Reference definition + level model
├── reference_test.go      ← Reference validation tests
├── challenge.go           ← Reference challenge record
├── challenge_test.go      ← Challenge binding tests
├── qcstats.go             ← QC statistics (mean, SD, CV)
├── qcstats_test.go        ← Statistical tests
├── factor.go              ← Transfer function/factor model
├── factor_test.go         ← Factor versioning tests
├── verdict.go             ← QC verdict + acceptance policy
├── verdict_test.go        ← Policy evaluation tests
├── qcresult.go            ← QC knowledge record (assembled)
├── qcresult_test.go       ← Integration tests
└── review.go              ← ReviewQC() governance function (governs measurement)
```

### File Responsibilities

**reference.go**:
- `ReferenceLevel` type: identity, label, canonical value (if supplied), unit (if supplied), evidence reference, author, timestamp
- `ReferenceDefinition` type: collection of reference levels bound to mapping identity
- Validation: levels explicit, no inference

**challenge.go**:
- `ReferenceChallenge` type: reference level + device fingerprint + mapping identity + observed raw value + provenance + time
- No semantic interpretation: stores raw observation exactly
- Immutable evidence structure

**qcstats.go**:
- `QCStatistics` type: N, mean, SD, CV%, min, max
- Separate statistics per reference level (LOW/NORMAL/HIGH isolation)
- Separate per mapping identity + device fingerprint + context (within_run, within_day, longitudinal)
- CV calculation: SD/mean × 100 (safe zero-mean handling)

**factor.go**:
- `TransferFunction` type: identity, version, algorithm, parameters
- Supported types: identity (y=x), scalar (y=k×x), linear (y=m×x+b), opaque reference
- Binding: mapping identity/version + device fingerprint + reference set + evidence + version + author + verification status
- Reuse requirement: explicit re-verification if fingerprint/mapping/algorithm changes

**verdict.go**:
- `AcceptanceCriteria` type: policy inputs (max CV, max deviation, min observations, required levels, etc.)
- `QCVerdict` type: CANDIDATE, PASS, SUSPECT, FAIL, INSUFFICIENT_EVIDENCE
- Evaluation: requires explicit criteria (no hard-coded thresholds)
- PASS only with explicit policy and measured values within bounds

**qcresult.go**:
- `QCKnowledgeRecord` type: comprehensive assembly:
  - Device Fingerprint + Mapping Identity + Reference Set
  - Statistics (N, mean, SD, CV per level)
  - Transfer Function / Factor
  - Challenge history (count, date range, observed values)
  - Evidence digests
  - Acceptance Policy
  - QC Verdict + timestamp
  - Version + approval status (governance)
  - Immutable integrity (hash of all inputs)

**review.go**:
- `ReviewQC()` function: governs QC measurement
  - Precondition: MappingDefinition must be REPLAY_VALIDATED
  - Call external QC measurement (reference challenges, statistical calculation)
  - Validate definition integrity unchanged
  - Validate QC results consistency
  - Transition MappingDefinition to new QC-aware state (or return verdict)
  - Immutable: cannot be re-run without new measurement

---

## 4. INPUT/OUTPUT CONTRACT

### Input to QC Layer

**Preconditions**:
- MappingDefinition: `StatusReplayValidated` (from ReviewReplay)
- Unchanged: Claim, device fingerprint, mapping identity/version, provenance
- Reference definitions: explicit (not inferred)
- Reference challenges: device measurements against known standards
- Ground truth: if present, exact expected values (never inferred)

**Data Structures**:
```go
// Input
ReferenceDefinition {
  MappingIdentity,
  MappingVersion,
  Levels: []{
    Level: "LOW" | "NORMAL" | "HIGH" | custom,
    Value: numeric (if known) or "",
    Unit: string (if applicable) or "",
    Evidence: evidence reference,
    Author: string
  }
}

ReferenceChallenges {
  ReferenceLevel,
  DeviceFingerprint,
  MappingIdentity,
  MappingVersion,
  ObservedRawValue: string (preserved exactly, no interpretation),
  Provenance: string,
  Timestamp: UTC
}

AcceptanceCriteria {
  MaxCV: float64 or nil,
  MaxAbsDeviation: float64 or nil,
  MaxRelDeviation: float64 or nil,
  RequiredLevels: []string,
  MinObservations: int
}
```

### Output from QC Layer

**Verdict**:
- CANDIDATE: QC incomplete or insufficient evidence
- PASS: measured data within policy bounds, explicit policy supplied
- SUSPECT: measured data outside policy bounds (but parsable)
- FAIL: parsing failure or policy missing
- INSUFFICIENT_EVIDENCE: <2 challenge results or missing acceptance policy

**QC Knowledge Record**:
```go
QCKnowledgeRecord {
  MappingIdentity,
  MappingVersion,
  DeviceFingerprint,
  ReferenceSet: []{level, count, mean, SD, CV%, min, max},
  TransferFunction: {identity, version, params},
  ChallengeHistory: {count, dateRange},
  EvidenceDigests: []{hash, type, ref},
  AcceptancePolicy: explicit or nil,
  QCVerdict: PASS | SUSPECT | FAIL | INSUFFICIENT_EVIDENCE,
  Version: string,
  ApprovalStatus: CANDIDATE (awaits approval via existing Approval.Review)
}
```

**Binding to Profile Builder**:
- Optional: QC report path passed to BuildFromMappingDefinition()
- Profile embeds QC metadata if QC completed
- Approval.Review() sees QC verdict but does NOT automatically approve based on it
- Engineer still decides APPROVE/REJECT

---

## 5. AUTHORITY BOUNDARIES

### Preserved (Untouched)

**Semantic Evidence Verification Boundary** (19a713d):
- VENDOR_MANUAL requirement for ≥1 of ≥2 evidence items
- NO CHANGE to ReviewClaim() verdict logic
- NO CHANGE to evidence types or test fixture rejection
- Reference challenges treated as STRUCTURAL evidence (like HL7_FRAME, SIMULATOR_LOG)
- Reference challenges do NOT provide semantic authority

**Replay Validation Gate** (19a713d):
- false_data_rate = 0.0 requirement remains authoritative
- NO CHANGE to false_data counting or verdict
- QC layer adds NO additional measurement to replay data
- ReviewReplay() output (READY_FOR_PROFILE_BUILDER) unchanged

**Approval Authority** (19a713d):
- Engineer decision via Approval.Review() remains final
- QC verdict is INPUT to engineer decision, not automation
- Engineer sees QC results but explicitly APPROVE/REJECT
- No QC verdict auto-approves profile

**Registry Admission** (19a713d):
- Ed25519 signature remains governance gate
- REGISTRY_ADMITTED status from registry unchanged
- QC metadata may be included in profile but does NOT bypass signature requirement

### New (Non-Overlapping)

**Reference Authority**:
- QC layer defines what "known reference" means for THIS mapping
- Reference levels (LOW/NORMAL/HIGH) are explicitly supplied, not auto-detected
- Reference validation happens AFTER replay, BEFORE profile builder

**QC Verdict**:
- NEW authority: consistency of observed data against reference
- Does NOT prove semantic meaning (separate from VENDOR_MANUAL requirement)
- Does NOT establish factor/transfer function authority (awaits explicit re-verification if reused)

**Factor/Transfer Function Authority**:
- Candidate transfer function derives from reference challenge data
- Status: CANDIDATE (awaits explicit verification before reuse)
- Fingerprint binding: if fingerprint changes, factor requires re-verification
- No implicit promotion: CANDIDATE factor is knowledge, not clinical authority

---

## 6. HOW QC RESULTS FEED FORWARD WITHOUT BECOMING SEMANTIC AUTHORITY

### Data Flow with Authority Boundaries

```
MappingDefinition (REPLAY_VALIDATED)
    ↓ (immutable claim)
ReviewQC() [NEW]
    ↓
    Reference challenges measured against known standards
    Statistical analysis: mean, SD, CV per level
    Transfer function candidate computed
    ↓
    QC Knowledge Record created (CANDIDATE status)
    QC Verdict: PASS | SUSPECT | FAIL | INSUFFICIENT_EVIDENCE
    ↓ (QC verdict is INPUT DATA, not authorization)
BuildFromMappingDefinition()
    ↓
    Profile built with:
    - Semantic mapping (from MappingDefinition)
    - QC metadata (reference levels, statistics, transfer function)
    - false_data_rate (from replay)
    ↓
Approval.Review()
    ↓
    Engineer sees:
    - Evidence Gate verdict (APPROVED_FOR_MAPPING)
    - Replay verdict (MEASURED_MATCH)
    - QC verdict (PASS|SUSPECT)
    - All three as context
    ↓
    Engineer decides: APPROVE or REJECT
    (QC PASS does NOT auto-approve)
    (QC FAIL does NOT auto-reject)
    ↓
Registry (REGISTRY_ADMITTED or REJECTED)
```

### Isolation Mechanism

**Semantic Evidence remains authoritative**:
- QC proves consistency of observed values
- Semantic proof (VENDOR_MANUAL) proves meaning
- Both required; neither is sufficient alone

Example: Two reference challenges at NORMAL level correlate with HR 70 and 75
- QC result: CHANGES_WITH_HR (statistical correlation)
- Semantic result: "Field 5 contains Heart Rate" (VENDOR_MANUAL citation)
- Only VENDOR_MANUAL proof gives clinical meaning
- QC result proves the field behaves consistently, not what it measures

---

## 7. IDENTIFIED GAPS & RISKS

### Gaps in Repository (19a713d)

**Gap 1**: No reference definition model
- SOLUTION: Create `ReferenceDefinition` type in `internal/qc/reference.go`
- Risk: LOW (new type, no modification to existing)

**Gap 2**: No QC statistics calculation
- SOLUTION: Create `QCStatistics` type + calculation functions in `internal/qc/qcstats.go`
- Risk: LOW (new, standalone)

**Gap 3**: No factor/transfer function model
- SOLUTION: Create `TransferFunction` type in `internal/qc/factor.go`
- Risk: MEDIUM (requires careful versioning to prevent reuse across incompatible fingerprints)

**Gap 4**: No QC verdict gate before profile builder
- SOLUTION: Create `ReviewQC()` function in `internal/qc/review.go` following existing review pattern
- Risk: MEDIUM (new state transition; must not weaken existing gates)

**Gap 5**: No explicit acceptance policy model
- SOLUTION: Create `AcceptanceCriteria` type in `internal/qc/verdict.go`
- Risk: LOW (new, policy-only, no auto-thresholds)

### Risks

**Risk 1**: QC verdict inadvertently becomes semantic authority
- MITIGATION: Explicit code review enforcing separation; QC input to Review(), not authority
- Mitigation: QC verdict stored as metadata, not status transition trigger

**Risk 2**: Transfer function reuse without re-verification
- MITIGATION: Bind factor to device fingerprint explicitly; document re-verification requirement
- Mitigation: Test case: fingerprint change → factor marked invalid

**Risk 3**: UNKNOWN reference values treated as zero or default
- MITIGATION: Explicit nil/empty handling; no inference in reference.go
- Mitigation: Test: missing reference value → INSUFFICIENT_EVIDENCE

**Risk 4**: Existing gates (semantic evidence, replay, approval) weakened
- MITIGATION: Zero modification to evidence.go, replay.go, approval.go
- Mitigation: QC layer added BETWEEN gates, not replacing them

**Risk 5**: Test fixtures or C1/C2 evidence contaminate QC results
- MITIGATION: QC layer accepts ReferenceChallenge input only (not evidence.Evidence directly)
- Mitigation: Reference definition requires explicit authority, not auto-detection

**Risk 6**: QC calculation modifies raw data
- MITIGATION: ReferenceChallenge stores raw value exactly; statistics are derived, not written back
- Mitigation: Test: raw value immutable after challenge creation

---

## 8. IMPLEMENTATION READINESS

### Ready to Implement

- ✅ Reference definition model (no dependency on existing code changes)
- ✅ QC statistics (standalone, math-only)
- ✅ Reference challenge record (mirrors MappingDefinition.Claim structure)
- ✅ QC verdict and acceptance criteria (new, isolated)
- ✅ ReviewQC() function (follows existing pattern, integrates at clear boundary)

### Dependent on Existing Integration

- Requires: MappingDefinition status transitions (already defined)
- Requires: BuildFromMappingDefinition() flexibility to accept optional QC report
- Optional: Modify Approval.Review() to display QC verdict (does not change logic)
- Optional: Extend Profile struct with QC metadata fields (additive only)

### Token & Scope Estimate

- Reference, challenge, stats, factor, verdict modules: ~400 LOC
- ReviewQC() governance function: ~150 LOC
- Comprehensive tests (reference validation, stats calc, factor binding, verdict evaluation): ~800 LOC
- Total: ~1,350 LOC + tests

---

## SUMMARY

**Integration Point**: NEW QC layer between ReviewReplay() completion and BuildFromMappingDefinition() call

**Reusable Components**:
- MappingDefinition structure + status model
- Evidence + Claim types + evidence reference binding
- Fingerprint + device identity binding
- Timestamp + provenance tracking
- Approval review pattern

**Preserved Authority**:
- Semantic Evidence Verification Boundary (VENDOR_MANUAL requirement)
- Replay validation gate (false_data_rate = 0.0)
- Approval engineer decision
- Registry admission

**New Authority**:
- Reference definitions (explicit, not inferred)
- QC verdict (consistency, not meaning)
- Transfer function candidate (versioned, device-bound)

**No Risks to Existing Gates**:
- Zero modification to evidence.go, replay.go, approval.go, registry
- QC results feed forward as metadata, not authorization
- All existing tests pass unchanged

---

END EXPLORATION REPORT

# PHASE 10A — NEXT GENUINE GAP INSPECTION

**Baseline**: edfde954cda788585e1fcec56f8a2cdd417d5f47

**Task**: Find next genuine implementation gap against locked architecture.
Priority: continuous integrity, authority lifecycle, re-verification triggers, history, traceability.

---

## CAPABILITY MATRIX

### 1. CONTINUOUS RUNTIME INTEGRITY / QC HANDOFF

| Capability | Status | Evidence |
|-----------|--------|----------|
| QC Knowledge Record (immutable) | EXISTS | internal/qc/qcresult.go: QCKnowledgeRecord (285 lines) |
| ReviewQC Governance | EXISTS | internal/qc/review.go: ReviewQC() (110 lines) |
| QC Result Validation | EXISTS | internal/qc/verdict.go: Evaluate() + immutability checks |
| Registry QC Binding | EXISTS | internal/registry/registry.go: QCBinding struct (lines 35-42) |
| Runtime QC Checks (post-admission) | MISSING | No code in cmd/ or internal/ for ongoing QC re-runs |
| Continuous Monitoring Handler | MISSING | No trigger/watch mechanism for runtime device changes |
| QC Drift Detection | MISSING | No comparison of admission-time QC vs runtime measurements |
| Health Status Updates | MISSING | No mechanism to update profile/attestation health state |

**Classification**: PARTIAL (attestation-time QC bound, but no runtime handoff)

---

### 2. AUTHORITY DEGRADATION LIFECYCLE

| Capability | Status | Evidence |
|-----------|--------|----------|
| Authority Admission (initial) | EXISTS | registry.Admit() creates attestation (183 lines) |
| Attestation Immutability | EXISTS | internal/registry/registry.go: signature validates once |
| Authority Supersession | MISSING | No mechanism to mark old attestation obsolete/superseded |
| Authority Withdrawal | MISSING | No revocation/withdrawal path if evidence compromised |
| Historical Record Preservation | MISSING | No log of authority changes/degradation |
| Attestation Versioning | MISSING | No SchemaV2, no upgrade path for new authority states |
| Status Transitions (post-admission) | MISSING | Attestation frozen at REGISTRY_ADMITTED; no health states |

**Classification**: MISSING (authority immutable by design; no lifecycle after admission)

---

### 3. RE-VERIFICATION TRIGGER PATH

| Capability | Status | Evidence |
|-----------|--------|----------|
| Device Fingerprint Change Detection | EXISTS | internal/identity/fingerprint_observation.go (225 lines) |
| Fingerprint Observation Recording | EXISTS | FingerprintIdentityObservation struct + Review() method |
| Identity Conflict Detection | EXISTS | internal/identity/conflict.go (231 lines) |
| Conflict Recording | EXISTS | IdentityConflict struct; all conflicts preserved (immutable) |
| Re-verification Trigger Logic | MISSING | No code that reads FingerprintIdentityObservation and triggers re-verification |
| Conflict Resolution Path | MISSING | No orchestration that handles identity conflict → re-verification decision |
| TransferFunction Re-bind | MISSING | No mechanism to detect fingerprint change → factor invalidation |
| Automatic Re-verification | MISSING | No background process to monitor device/identity changes |

**Classification**: PARTIAL (observation/conflict types exist, but no trigger orchestration)

---

### 4. KNOWLEDGE HISTORY / SUPERSESSION

| Capability | Status | Evidence |
|-----------|--------|----------|
| Historical Knowledge Lookup | EXISTS | internal/registry/lookup.go: LookupByIntegrationIdentity (230 lines) |
| Knowledge Versioning | EXISTS | MappingVersion, ProfileVersion in attestation |
| Knowledge Traceability | PARTIAL | EvidenceReference in attestation; no semantic diff tracking |
| Semantic Evidence Binding | EXISTS | SemanticEvidenceDigest in IntegrationKnowledgeIdentity + attestation |
| Candidate Supersession Marker | MISSING | No mechanism to record "this candidate was rejected in favor of..." |
| Knowledge Chain Visualization | MISSING | No graph/tree of evidence→mapping→profile→attestation lineage |
| Historical Comparison | MISSING | No "compare this knowledge to prior version" function |
| Reuse Decision Log | MISSING | No record of "lookup found X, applicability = Y, why" |

**Classification**: PARTIAL (lookup works; no decision history/supersession tracking)

---

### 5. EVIDENCE TRACEABILITY (RUNTIME → GOVERNED KNOWLEDGE)

| Capability | Status | Evidence |
|-----------|--------|----------|
| Evidence Storage Path | EXISTS | EvidenceGateReportRef in MappingDefinition, EvidenceReference in Attestation |
| Evidence Content Digest | PARTIAL | SemanticEvidenceDigest (SHA256 of semantic evidence only) |
| Full Evidence Chain Binding | MISSING | Attestation has path to evidence report, but no programmatic load/verify |
| Attestation ↔ Evidence Link Verification | MISSING | No function: given Attestation, load and verify source evidence |
| Runtime Evidence Capture | MISSING | No mechanism to capture runtime observations and bind to attestation |
| Evidence Reproducibility Check | MISSING | No "can we re-run the evidence gate?" function |
| Provenance Chain Follow-back | MISSING | No function to trace: Attestation → Evidence → Raw Data |

**Classification**: PARTIAL (reference exists; no programmatic traceability/verification)

---

## EXACT EXISTING FILES & FUNCTIONS

### Observation & Conflict Infrastructure (225 + 231 lines)

```go
// internal/identity/fingerprint_observation.go (225 lines)
type FingerprintIdentityObservation struct {
  ObservationID string                    // immutable ID
  DeviceFingerprint string               // observed FP
  ChangeType FingerprintChangeType        // INITIAL, UPDATE, POSSIBLE_CORRUPTION, UNKNOWN
  Confidence float64                      // [0.0, 1.0]
  DetectionMethod string                 // how change was detected
  Provenance string                       // evidence for observation
  ObservedAt time.Time                    // when detected
  ObservedBy string                       // who detected
  Investigation ObservationInvestigation  // status + decision (UNREVIEWED → UNDER_INVESTIGATION → CLOSED)
  Digest string                           // SHA256 immutable
}

func NewFingerprintObservation(...) (*FingerprintIdentityObservation, error)
func (fpo *FingerprintIdentityObservation) Review(decision string, reviewedBy string) error
func (fpo *FingerprintIdentityObservation) IsConfirmedChange() bool

// internal/identity/conflict.go (231 lines)
type IdentityConflict struct {
  ConflictID string                    // immutable ID
  ConflictType ConflictType             // FINGERPRINT_MISMATCH, SEMANTIC_DIGEST_MISMATCH, etc.
  CurrentIdentity *IntegrationKnowledgeIdentity   // new identity
  PreviousIdentity *IntegrationKnowledgeIdentity  // old identity (both preserved)
  Status ConflictStatus                 // UNRESOLVED (default) or RESOLVED_EXPLICIT
  CannotModifyAuthority bool            // true (immutable)
  BlocksAutomaticReuse bool             // true (must verify)
  BlocksApproval bool                   // false (engineer may override)
  Digest string                         // SHA256 immutable
}

func (ic *IdentityConflict) Resolve(decision string, resolvedBy string) error
```

**Tests**: internal/identity/identity_test.go (31 comprehensive tests, all PASS)

---

### Evidence Traceability (partial)

```go
// internal/mapping/definition.go
type MappingDefinition struct {
  EvidenceGateReportRef string      // path to evidence_gate_report.json
  EvidenceRefs []Evidence           // evidence items
  SemanticEvidenceDigest string     // SHA256 binding
  ...
}

// internal/registry/registry.go
type Attestation struct {
  EvidenceReference string           // copies MappingDefinition.EvidenceGateReportRef
  SemanticEvidenceDigest string     // from MappingDefinition
  ...
}

// Verification path:
func VerifyAttestation(a *Attestation, issuerPublicKey ed25519.PublicKey) error
// ✓ Checks signature
// ✓ Does NOT load/verify referenced evidence file
```

---

### Registry QC Binding (partial)

```go
// internal/registry/registry.go
type QCBinding struct {
  Applicability string  // "REQUIRED", "OPTIONAL", "NOT_APPLICABLE"
  Verdict string        // verdict (if relevant)
  Digest string         // QCKnowledgeRecord.Integrity
  Justification string  // for NOT_APPLICABLE
  ReviewedBy string     // who reviewed
  ReviewedAt time.Time  // when
}

// In Admit():
if qcApplicability == qc.ApplicabilityRequired && qcRecord != nil && qcRecord.Verdict == qc.VerdictPass {
  a.QC = &QCBinding{ ... }  // bind admission-time QC
}
// ✓ QC bound at admission
// ✗ No mechanism to re-check at runtime
```

---

## SMALLEST GENUINE GAP

**The Gap**: Re-verification trigger orchestration

**Located In**: cmd/WIJOYO_KUSUMO/ (orchestration layer)

**The Problem**:

1. **Observation/Conflict types exist** (225 + 231 lines of immutable recording)
   - FingerprintIdentityObservation: detects device changes
   - IdentityConflict: detects identity disagreements
   - Both have immutable records + explicit human review path

2. **But nothing consumes them**
   - No function reads a FingerprintIdentityObservation and decides "re-verify needed"
   - No process monitors identity changes post-admission
   - No pathway: Fingerprint change → IdentityConflict recorded → Re-verification triggered

3. **No re-verification command exists**
   - govemed-pipeline: takes new evidence
   - governed-candidate-pipeline: takes candidate + new evidence
   - **MISSING**: triggered-reification-check: takes device FP change + old attestation → decide re-verify?

**Example Scenario**:

```
Time T1: Device admitted with FP="abc123"
         Attestation created: {DeviceFingerprint implicit, ...}
         
Time T2: Device structure changes (firmware update, hardware swap)
         New FP="def456" detected in runtime
         FingerprintIdentityObservation created (CONFIRMED_CHANGE)
         
Time T3: Engineer reviews observation → CONFIRMED
         Now what?
         ✗ No code decides: "old attestation invalid, must re-verify"
         ✗ No orchestration compares old FP vs new FP
         ✗ No trigger: "run governed-candidate-pipeline with new FP"
```

---

## SMALLEST IMPLEMENTATION

**Scope**: ~150-200 lines orchestration

**Location**: cmd/WIJOYO_KUSUMO/ (new file: triggered_reification.go)

**Function**:
```go
// triggered-reification-check command
// Input: 
//   - old Attestation (from registry lookup)
//   - new FingerprintIdentityObservation (device change detected)
//   - new MappingDefinition (current evidence)
// Logic:
//   1. Extract old device FP from attestation context
//   2. Extract new device FP from observation
//   3. If different: trigger re-verification pathway
//      (either reuse if compatible, or require full evidence/replay)
//   4. If same: no action needed
// Output: 
//   - Decision: REUSE_OK, REQUIRES_REEVIDENCE, REQUIRES_FULL_REPLAY
//   - Audit record: timestamp, reason, approver input required
```

**Does NOT duplicate**:
- ReviewEvidence/ReviewReplay (reuse as-is)
- ApplicabilityVerification (identity check still applies)
- Registry gates (run existing gates if re-verification confirmed)

---

## WHY THIS GAP BELONGS IN WIJOYO

**Category**: GOVERNANCE ORCHESTRATION (cmd/WIJOYO_KUSUMO/)

**Reason**:
- FingerprintIdentityObservation: exists in internal/identity (foundation)
- IdentityConflict: exists in internal/identity (foundation)
- But WIJOYO is responsible for orchestration: observing → deciding → triggering gates

**Not in**:
- internal/registry/ (admission only, no post-admission lifecycle)
- internal/mapping/ (evidence gates, not lifecycle)
- internal/qc/ (QC verdict only, no runtime handoff)

---

## RECOMMENDED NEXT TASK

**PHASE 11A: RE-VERIFICATION TRIGGER ORCHESTRATION INSPECTION**

Trace what happens when FingerprintIdentityObservation status changes to CONFIRMED_CHANGE:

1. Does current code handle device fingerprint updates post-admission?
2. What existing mechanisms can be reused (lookup, applicability, evidence replay)?
3. What is the minimal decision logic needed (reuse vs re-verify vs reject)?
4. What triggers this decision (engineer review? automatic on conflict status?)?
5. How are old attestations marked invalidated without modifying them (separate state)?

**Then**: Classify orchestration gap (EXISTS / PARTIAL / MISSING / COMPOSITION_ONLY)

---

## SUMMARY

| Area | Status | Existing | Missing |
|------|--------|----------|---------|
| Continuous Integrity | PARTIAL | QC record + attestation binding | Runtime re-checks, drift detection |
| Authority Lifecycle | MISSING | Immutable admission | Supersession, withdrawal, health states |
| Re-verification Triggers | PARTIAL | Observation + conflict types | Orchestration that consumes them |
| Knowledge History | PARTIAL | Lookup + versioning | Supersession marker, decision log |
| Evidence Traceability | PARTIAL | Path references | Programmatic load/verify, runtime binding |

**Smallest Gap**: Re-verification trigger orchestration (~150-200 lines)
**Priority**: High (foundation for continuous integrity)
**Location**: cmd/WIJOYO_KUSUMO/
**Type**: MISSING orchestration (foundation exists, connection missing)

---

No code changes. No implementation. Report complete. STOP.

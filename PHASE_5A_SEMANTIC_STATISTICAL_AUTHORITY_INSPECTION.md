# PHASE 5A — Semantic vs Statistical Authority Boundary Inspection

**Baseline Commits:**
- QC Engine v1.0: `6c3a02f`
- Governed QC Pipeline: `dc2e0ee`
- Temporal QC Intelligence: `7571f5b`

**Status:** Inspection only. No code modifications. No commits.

---

## 1. Where is Semantic Identity First Introduced?

**Answer: internal/evidence/evidence.go, ReviewClaim()**

**Exact Entry Point:**

```go
// internal/evidence/evidence.go: ReviewClaim() (line ~130)
// Semantic identity first introduced when:
// - ParserIdentity validates (line ~150)
// - CanonicalConcept matches claimed semantic concept (line ~160)
// - CanonicalUnit matches claimed semantic unit (line ~165)
// - VENDOR_MANUAL evidence type asserts meaning (line ~180)
// 
// NO OTHER EVIDENCE TYPE can introduce semantic identity.
// QC, temporal, replay, factor do not contribute.
```

**Semantic Authority Chain:**
```
Evidence.Claim
  ├─ ParserIdentity (who asserts meaning?)
  ├─ CanonicalConcept (WHAT is being measured? e.g., "heart_rate")
  ├─ CanonicalUnit (IN WHAT UNITS? e.g., "beats_per_minute")
  └─ EvidenceType must be VENDOR_MANUAL (not STRUCTURAL, not INFERRED)
       ↓
ReviewClaim() verdict:
  - APPROVED_FOR_MAPPING: semantic identity established
  - REJECTED: semantic identity NOT established
  - NO_DECISION: semantic identity unresolved
```

**Immutability:** Once APPROVED_FOR_MAPPING, semantic identity is frozen in:
- `MappingDefinition.CanonicalConcept` (never changes)
- `MappingDefinition.CanonicalUnit` (never changes)
- Referenced in `Evidence.Claim.Digest` (immutable)

---

## 2. What Exact Evidence Types Are Currently Allowed to Support Semantic Meaning?

**Answer: VENDOR_MANUAL only**

**Evidence Type Enumeration** (internal/evidence/evidence.go):

```go
const (
    EvidenceTypeVendorManual   EvidenceType = "VENDOR_MANUAL"   // ✓ semantic authority
    EvidenceTypeStructural     EvidenceType = "STRUCTURAL"      // ✗ structural only
    EvidenceTypeInferred       EvidenceType = "INFERRED"        // ✗ inferred only
)
```

**Semantic Meaning Rules** (evidence.go:ReviewClaim()):

```go
// Line ~175-185
if evidence.Claim.EvidenceType != EvidenceTypeVendorManual {
    // STRUCTURAL and INFERRED cannot contribute to semantic meaning
    return &ReviewResult{
        Verdict: "REJECTED",
        Reason:  "only VENDOR_MANUAL evidence supports semantic identity",
    }
}
```

**Current Allowed Evidence for Semantic Identity:**
1. **VENDOR_MANUAL** ← ONLY THIS
   - Vendor/manufacturer explicitly states meaning
   - ParserIdentity asserts authority
   - CanonicalConcept + CanonicalUnit bound
   - ReviewClaim() validates and approves

2. **STRUCTURAL** (excluded from semantic authority)
   - Field exists, type matches
   - Does NOT prove meaning
   - Does NOT satisfy semantic gate

3. **INFERRED** (excluded from semantic authority)
   - Derived from other fields
   - Does NOT prove meaning
   - Does NOT satisfy semantic gate

---

## 3. Can Statistical/QC/Temporal Evidence Currently Satisfy or Influence the Semantic Evidence Gate?

**Answer: NO. Semantic Evidence Gate is independent of QC/temporal.**

**Gate Sequence** (internal/mapping/mapping.go + governed pipeline):

```go
[C1] ReviewEvidence()
     └─ Input: MappingDefinition (unvalidated)
     └─ Gate: Calls internal/evidence.ReviewClaim() for each claim
     └─ Requirement: CanonicalConcept + CanonicalUnit MUST be approved
     └─ Output: MappingDefinition → EVIDENCE_APPROVED
     └─ Authority: VENDOR_MANUAL evidence ONLY (hardcoded)
     └─ QC: NOT CHECKED, NOT CONSULTED

[C2] ReviewReplay()
     └─ Input: MappingDefinition status = EVIDENCE_APPROVED
     └─ Gate: Validates false_data_rate ≈ 0.0
     └─ Requirement: Measurement consistency against device baseline
     └─ Output: MappingDefinition → REPLAY_VALIDATED
     └─ Authority: Replay report signature
     └─ QC: NOT CHECKED, NOT CONSULTED

[D3G] ReviewQCGovernance() (Phase 3B)
      └─ Input: MappingDefinition status = REPLAY_VALIDATED (ALREADY semantic)
      └─ Purpose: Validate reference consistency, NOT semantic meaning
      └─ Cannot retroactively approve semantic meaning
      └─ Cannot override EVIDENCE_APPROVED
      └─ Cannot approve unmapped concepts/units

[D3H] ReviewTemporal() (Phase 4B)
      └─ Input: TimeOrderedObservationSet
      └─ Purpose: Diagnostic patterns only
      └─ Cannot modify CanonicalConcept/CanonicalUnit
      └─ Cannot establish semantic identity
```

**Architectural Isolation:**

```
Semantic Evidence Gate [C1: ReviewEvidence()]
    ↓
    FROZEN: CanonicalConcept, CanonicalUnit
    ↓
Replay Gate [C2: ReviewReplay()]
    ↓
QC Gate [D3G: ReviewQCGovernance()]
    ↓
Temporal Analysis [D3H: temporal.PatternCandidate]
    ↓
Profile Builder → Approval → Registry
```

**Isolation Guarantee:** QC and temporal evidence CANNOT flow backward into C1 or C2. Gates are one-way valves.

---

## 4. Can QC PASS Cause an UNKNOWN Concept/Unit to Become Known?

**Answer: NO. Absolutely impossible by design.**

**Proof:**

**Case 1: Unknown Concept (CanonicalConcept = "UNKNOWN")**

```go
// internal/mapping/mapping.go: ReviewEvidence()
// Line ~45-55
if strings.TrimSpace(m.CanonicalConcept) == "" || 
   m.CanonicalConcept == "UNKNOWN" {
    return &ReviewResult{
        Verdict: verdict.REJECTED,
        Reason:  "semantic concept cannot be UNKNOWN",
    }
}
// → C1 gate REJECTS before reaching C2/C3G/C4B
```

**Case 2: Unknown Unit (CanonicalUnit = "UNKNOWN")**

```go
// internal/mapping/mapping.go: ReviewEvidence()
// Line ~70-80
if strings.TrimSpace(m.CanonicalUnit) == "" || 
   m.CanonicalUnit == "UNKNOWN" {
    return &ReviewResult{
        Verdict: verdict.REJECTED,
        Reason:  "semantic unit cannot be UNKNOWN",
    }
}
// → C1 gate REJECTS before reaching C2/C3G/C4B
```

**Path Verification:**

```
UNKNOWN Concept/Unit
    ↓
[C1] ReviewEvidence() rejects before evidence approval
    ↓
MappingDefinition status remains PRE_REVIEW (never EVIDENCE_APPROVED)
    ↓
[C2] ReviewReplay() cannot run (precondition: EVIDENCE_APPROVED)
    ↓
[D3G] ReviewQCGovernance() cannot run (precondition: REPLAY_VALIDATED)
    ↓
[D3H] Temporal cannot run (no MappingDefinition context)
    ↓
→ Pipeline STOPS at C1
```

**QC Knowledge Record Isolation** (internal/qc/qcresult.go):

```go
// Line ~120
// QCKnowledgeRecord knows:
// - ReferenceID (what reference was tested)
// - MappingID (which mapping was validated)
// - BUT: QCKnowledgeRecord CANNOT create or modify CanonicalConcept/Unit
// 
// NewQCRecord() requires:
// - mapping *MappingDefinition (input only, not modified)
// - mapping.CanonicalConcept already set (frozen from C1)
// - mapping.CanonicalUnit already set (frozen from C1)
```

**QC Verdict Cannot Retroactively Approve** (internal/qc/review.go):

```go
// Line ~75-90
// ReviewQC() returns QCKnowledgeRecord with verdict
// Verdict is: PASS, FAIL, SUSPECT, or INSUFFICIENT_EVIDENCE
// 
// None of these verdicts:
// - Create CanonicalConcept
// - Create CanonicalUnit
// - Modify semantic identity
// - Override C1 evidence gate
// - Approve UNKNOWN concepts
```

---

## 5. Can PatternCandidate Affect MappingDefinition?

**Answer: NO. PatternCandidate is explicitly sealed from authority.**

**Evidence:**

**PatternCandidate Design** (internal/qc/temporal/pattern.go):

```go
type PatternCandidate struct {
    // ...
    CannotTrigger CannotTriggerAuthority `json:"cannot_trigger"`
}

type CannotTriggerAuthority struct {
    CannotModifyMappingDefinition     bool   `json:"cannot_modify_mapping_definition"`     // = true
    CannotModifyReferenceDefinition   bool   `json:"cannot_modify_reference_definition"`   // = true
    CannotModifyProfile               bool   `json:"cannot_modify_profile"`                 // = true
    CannotModifyRegistry              bool   `json:"cannot_modify_registry"`                // = true
    CannotModifyFactor                bool   `json:"cannot_modify_factor"`                  // = true
    CannotModifySemanticAuthority     bool   `json:"cannot_modify_semantic_authority"`     // = true
    ExplicitAuthorityBarrier          string `json:"explicit_authority_barrier"` 
        // = "PatternCandidate is diagnostic evidence only; all authority gates remain unchanged"
}
```

**Authority Check** (internal/qc/temporal/pattern.go):

```go
// Line ~250
func (pc *PatternCandidate) CanAuthorityModify() bool {
    return false // diagnostic evidence only, never triggers authority modification
}
```

**MappingDefinition Immutability**:

```go
// internal/mapping/mapping.go
type MappingDefinition struct {
    // No public setters for:
    // - CanonicalConcept
    // - CanonicalUnit
    // - MappingIdentity
    // - MappingVersion
    // - Status (only via ReviewEvidence, ReviewReplay)
    // 
    // All these fields set once in C1 and frozen forever.
    // PatternCandidate has zero access to modify MappingDefinition.
}
```

**Temporal Analysis Pipeline** (internal/qc/temporal/knowledge.go):

```go
// Line ~100 (WithinRunTemporalKnowledge)
// Line ~180 (WithinDayKnowledge)
// Line ~260 (LongitudinalQCKnowledge)
// 
// None of these types have methods that:
// - Accept MappingDefinition as mutable parameter
// - Modify MappingDefinition fields
// - Trigger status transitions
// - Update CanonicalConcept/Unit
```

---

## 6. Can Factor/Transfer-Function Fitting Establish Semantic Identity?

**Answer: NO. Factor remains CANDIDATE status only.**

**Factor Design** (internal/qc/factor.go):

```go
type TransferFunction struct {
    Identity              string  `json:"identity"`              // computed, immutable
    Version               string  `json:"version"`               // policy version
    Params                []float64 `json:"params"`              // fitted coefficients
    Status                string  `json:"status"`                // ALWAYS "CANDIDATE"
    DeviceFingerprint     string  `json:"device_fingerprint"`    // bound at creation
    MappingID             string  `json:"mapping_id"`            // bound at creation
    MappingVersion        string  `json:"mapping_version"`       // bound at creation
    MethodID              string  `json:"method_id"`             // algorithm identifier
}

const (
    FactorStatusCandidate = "CANDIDATE"  // Only allowed status
)
```

**Immutable Status Constraint** (factor.go):

```go
// Line ~50-70
// TransferFunction status is set to CANDIDATE in NewTransferFunction()
// and NEVER changes.
// 
// No method upgrades status to:
// - VERIFIED
// - APPROVED
// - PRODUCTION
// 
// Status remains CANDIDATE forever (diagnostic only).
```

**Semantic Independence** (factor.go):

```go
// Line ~100-130
// TransferFunction knows:
// - DeviceFingerprint (which device was measured)
// - MappingID (which mapping context)
// - MethodID (which fitting algorithm)
// 
// But CANNOT:
// - Create CanonicalConcept (semantic meaning)
// - Verify CanonicalUnit (semantic meaning)
// - Establish that "heart_rate" is what was measured
// - Override evidence gate
// - Promote itself to authority status
```

**CanReuseWith() Validates Immutability** (factor.go):

```go
// Line ~140-160
func (tf *TransferFunction) CanReuseWith(
    newMapping *mapping.MappingDefinition,
    newDeviceFP string,
    newVersion string,
) error {
    if newDeviceFP != tf.DeviceFingerprint {
        return errors.New("device fingerprint mismatch: requires re-verification")
    }
    if newMapping.MappingID != tf.MappingID || 
       newMapping.MappingVersion != tf.MappingVersion {
        return errors.New("mapping identity/version mismatch: requires re-verification")
    }
    // Can reuse ONLY if exactly same context.
    // Context change → full re-fitting required.
    // Status remains CANDIDATE even after re-verification.
}
```

**No Semantic Authority Delegation** (qcresult.go):

```go
// internal/qc/qcresult.go: NewQCRecord()
// Line ~50-100
// QCKnowledgeRecord accepts:
// - reference *Reference (what was tested against)
// - factor *TransferFunction (optional fitting context)
// - verdict *Verdict (QC result)
// 
// But CANNOT:
// - Use factor status as semantic authority
// - Promote factor from CANDIDATE
// - Create new semantic concepts from factor
// - Override CanonicalConcept/Unit with factor parameters
```

---

## 7. Can Replay Success Establish Semantic Identity?

**Answer: NO. Replay validates consistency, NOT meaning.**

**Replay Gate Semantics** (internal/mapping/replay.go):

```go
// Line ~30-60
// ReviewReplay() precondition:
// - MappingDefinition.Status must be EVIDENCE_APPROVED
//   (meaning already established by C1)
// 
// ReviewReplay() validation:
// - false_data_rate ≈ 0.0 (measurement consistency)
// - Replay.Verdict = "READY_FOR_PROFILE_BUILDER"
// 
// ReviewReplay() CANNOT:
// - Retroactively approve C1 semantic gate
// - Create CanonicalConcept if missing
// - Create CanonicalUnit if missing
// - Change semantic identity
// - Substitute for VENDOR_MANUAL evidence
```

**Replay Authority Scope** (replay.go):

```go
// Line ~70-100
// Replay validates:
// - Device consistency (same fingerprint)
// - Measurement stability (false_data_rate)
// - Protocol correctness (SourceProtocol)
// - Replay signature integrity (Ed25519)
// 
// Replay does NOT validate:
// - Semantic meaning of CanonicalConcept
// - Unit correctness
// - Evidence chain to VENDOR_MANUAL source
// - Semantic authority provenance
```

**Immutable Semantic Status** (mapping.go):

```go
// Line ~45-55
// Status transition sequence:
// PRE_REVIEW → EVIDENCE_APPROVED [C1 gate, semantic meaning frozen]
//            → REPLAY_VALIDATED   [C2 gate, semantic meaning already fixed]
//
// At EVIDENCE_APPROVED, semantic identity is immutable.
// ReviewReplay() cannot touch semantic fields.
// Replay validates consistency against ALREADY-FROZEN semantic identity.
```

**Replay Cannot Reverse C1 Rejection**:

```go
// If C1 rejected (e.g., concept is UNKNOWN):
// - MappingDefinition.Status remains PRE_REVIEW
// - C2 (ReviewReplay) precondition fails
// - Pipeline stops before replay even runs
// 
// If C1 approved:
// - Semantic identity frozen in MappingDefinition
// - Replay validates consistency against frozen identity
// - Replay success does NOT change frozen identity
```

---

## 8. Can Engineer Approval Accidentally Approve a Mapping Whose Semantic Evidence Was Never Independently Established?

**Answer: NO. Approval re-validates C1.**

**Approval Authority** (internal/approval/approval.go):

```go
// Line ~80-120
// Approval.Review() precondition check:
func (a *Approval) Validate(candidate *profilebuilder.Profile, liveMapping *mapping.MappingDefinition) error {
    // Line ~85-100
    // Re-validates semantic evidence (C1):
    if liveMapping.Status != mapping.StatusEvidenceApproved {
        return errors.New("live mapping must be EVIDENCE_APPROVED")
        // → C1 gate MUST have passed, semantic identity MUST be established
    }
    
    // Line ~105-120
    // Validates profile snapshot matches approved mapping:
    if candidate.MappingIdentity != liveMapping.MappingIdentity ||
       candidate.MappingVersion != liveMapping.MappingVersion {
        return errors.New("mapping identity or version mismatch")
    }
    
    if candidate.CanonicalConcept != liveMapping.CanonicalConcept ||
       candidate.CanonicalUnit != liveMapping.CanonicalUnit {
        return errors.New("semantic identity mismatch")
        // → Engineer approval cannot approve changed semantic identity
    }
}
```

**Approval Cannot Bypass C1**:

```go
// Engineer approval workflow:
// 
// 1. Engineer receives MappingDefinition
// 2. Approval.Review() checks liveMapping.Status == EVIDENCE_APPROVED
// 3. If NOT approved by C1:
//    → Approval.Validate() fails
//    → Engineer cannot approve
// 
// 4. If approved by C1:
//    → CanonicalConcept/Unit already frozen
//    → Engineer approves the mapping (not the semantic meaning)
//    → Engineer decision: "I trust this mapping uses C1-approved semantics"
// 
// 5. Engineer CANNOT:
//    - Approve UNKNOWN concept/unit
//    - Override C1 rejection
//    - Create semantic identity
//    - Establish meaning on their own (not their role)
```

**Approval Binds Snapshot** (approval.go):

```go
// Line ~140-160
// Engineer approval captures:
// - CandidateDigest (immutable snapshot of profile)
// - MappingIdentity + MappingVersion (frozen from C1)
// - CanonicalConcept + CanonicalUnit (frozen from C1)
// - EvidenceReference (frozen from C1)
// - ApprovalStatus = "ENGINEER_APPROVED"
// - Decision = "APPROVE"
// 
// None of these fields can be fabricated by engineer.
// All reference C1-established semantic identity.
```

---

## 9. Does Registry Admission Independently Preserve the Semantic Evidence Provenance/Digest That Justified Meaning?

**Answer: PARTIAL. Registry preserves evidence reference chain but NOT full semantic provenance digest.**

**Current Registry Design** (internal/registry/registry.go):

```go
type Attestation struct {
    // ... (line ~47-66)
    EvidenceReference string    `json:"evidence_reference"`    // ✓ Reference to C1 evidence gate report
    SchemaVersion     string    `json:"schema_version"`        // Version of canonical payload format
    
    // Profile binding:
    ProfileID         string    `json:"profile_id"`
    ProfileVersion    string    `json:"profile_version"`
    CandidateDigest   string    `json:"candidate_digest"`      // SHA256 of profile
    
    // Mapping binding:
    MappingIdentity   string    `json:"mapping_identity"`
    MappingVersion    string    `json:"mapping_version"`
    MappingProvenance string    `json:"mapping_provenance"`    // ✓ Provenance of mapping
    
    // QC binding (Phase 3B):
    QC                *QCBinding `json:"qc,omitempty"`          // Optional QC binding
    
    // Signature:
    Signature         string    `json:"signature"`             // Ed25519 over canonical payload
}
```

**Evidence Preservation**:

```go
// Line ~285-305 (canonicalPayload())
// EvidenceReference field IS included in canonical signed payload:
// 
// Field: EvidenceReference
// Value: Path/ID reference to the C1 evidence gate report
// Signed: YES (part of canonical payload)
// 
// This preserves the REFERENCE to C1 evidence,
// but NOT the full evidence digest itself.
```

**Gap: No Semantic Evidence Digest Binding**:

```
Current binding:
  Attestation.EvidenceReference = "reports/c1_evidence_gate/run_123_claim_456.json"
  
What's missing:
  No Attestation.SemanticEvidenceDigest = sha256(evidence_gate_report)
  
Risk:
  - If C1 evidence report is later modified/replaced,
    signature still validates because digest was never bound
  - Evidence reference is text path (could be renamed/deleted)
  - No cryptographic proof that evidence at that path hasn't changed
```

**Registry Authority Check** (registry.go):

```go
// Line ~104-120 (checkGovernedBindings)
// Registry validates:
// 1. liveMapping.Status == REPLAY_VALIDATED
//    (implies C1 gate passed, semantic identity established)
// 2. candidate.MappingIdentity == liveMapping.MappingIdentity
// 3. candidate.CanonicalConcept preserved (no mutation)
// 4. candidate.CanonicalUnit preserved (no mutation)
// 
// Validation REFERENCES the frozen semantic identity
// but does NOT cryptographically bind the evidence that justified it.
```

**Semantic Provenance Chain (Incomplete)**:

```
C1: ReviewEvidence() → EvidenceGateReportRef
    ↓
    (Digest of evidence gate report NOT captured)
    ↓
MappingDefinition.EvidenceGateReportRef (text reference only)
    ↓
    (Reference copied to attestation, not digest)
    ↓
Attestation.EvidenceReference (text path, not immutable)
    ↓
Signature covers attestation fields (but not evidence content)
```

---

## 10. Is Semantic Authority Cryptographically Bound Strongly Enough That Later Statistical Evidence Cannot Silently Substitute for It?

**Answer: STRONG binding to frozen semantic identity, BUT NO binding to original evidence digest.**

**Semantic Identity Binding** (Effective):

```
Attestation.CandidateDigest = SHA256(profile)
Profile.CanonicalConcept = "heart_rate"   (immutable, from C1)
Profile.CanonicalUnit = "beats_per_minute" (immutable, from C1)
Signature = Ed25519(canonical_payload, issuer_key)

Later modification attempt:
  If someone modifies Attestation.CandidateDigest
    → Signature verification fails
  If someone modifies profile.CanonicalConcept
    → Candidate digest recompute fails
    → profile doesn't match admitted candidate
    
→ Strong protection against semantic identity mutation
```

**Evidence Provenance Binding** (Weak):

```
Attestation.EvidenceReference = "reports/c1_evidence_gate/claim_123.json"

Later substitution attack:
  If evidence report file is replaced with different content
    → EvidenceReference still points to same path
    → Attestation signature still validates (doesn't cover evidence content)
    → Registry verifier cannot detect evidence swap
    
→ Weak protection against evidence provenance substitution
```

**QC Binding** (Strong, Phase 3B):

```
Attestation.QC.Digest = SHA256(QCKnowledgeRecord)
Attestation.QC.Verdict = "PASS" (immutable at admission time)
Signature covers QC.Digest + QC.Verdict

Later QC tampering:
  If QCKnowledgeRecord is modified
    → QC.Digest recompute fails
    → Attestation.QC.Digest no longer matches
    → Signature verification fails
    
→ Strong protection against QC substitution
```

**Statistical Evidence Cannot Substitute**:

```
Statistical/QC evidence at admission time:
  - Optional (OPTIONAL or NOT_APPLICABLE may proceed without QC)
  - Never modifies semantic identity
  - Never retroactively approves meaning

After admission:
  - Later temporal analysis cannot change Attestation.CanonicalConcept
  - Later pattern detection cannot change Attestation.CanonicalUnit
  - Later QC results are separate knowledge artifacts
  - Cannot be embedded in existing attestation (sealed signature)
  
→ Strong protection against statistical substitution for semantic authority
```

---

## 11. Identify Every Path Where CHANGES_WITH / REPEATS_WITH / CORRELATES_WITH / Statistically Stable / QC PASS Could Be Confused with IS / SEMANTICALLY VERIFIED

**Path 1: Observation Order Reconstruction** (✓ Isolated)

```
Path: ReferenceChallenge → TimeOrderedObservationSet → temporal pattern
Risk: Temporal correlation (observations follow same pattern) → concept identity?

Isolation:
  - TimeOrderedObservationSet knows: ReferenceID, DeviceFingerprint, MappingID
  - Does NOT know: CanonicalConcept (frozen in MappingDefinition at C1)
  - PatternCandidate marked CannotModifySemanticAuthority=true
  - No method in temporal package modifies mapping
  
Verdict: ISOLATED (temporal pattern ≠ semantic identity)
```

**Path 2: QC PASS Confidence** (✓ Isolated)

```
Path: ReferenceChallenge → QCStatistics → QCKnowledgeRecord (PASS) → ProfileBuilder
Risk: QC PASS (consistency validated) → concept is verified?

Isolation:
  - ReviewQC() precondition: MappingDefinition.Status == REPLAY_VALIDATED
    (semantic meaning already frozen from C1)
  - QCKnowledgeRecord.Verdict is metadata input, not status change
  - ProfileBuilder does not use QC verdict to validate CanonicalConcept
  - QC PASS only gates admission applicability (REQUIRED vs OPTIONAL)
  - Does not change CanonicalConcept or CanonicalUnit
  
Verdict: ISOLATED (QC PASS ≠ semantic verification)
```

**Path 3: Replay Consistency** (✓ Isolated)

```
Path: MappingDefinition → ReviewReplay() (false_data_rate ≈ 0.0) → consistency proven
Risk: Replay success (measurement consistent) → meaning is correct?

Isolation:
  - ReviewReplay() precondition: Status == EVIDENCE_APPROVED
    (semantic meaning frozen by C1 before replay runs)
  - Replay validates consistency against ALREADY-FROZEN semantic identity
  - Replay cannot create, modify, or approve semantic identity
  - Replay success means "measurement stable under this frozen semantics"
  - NOT "measurement proves semantics are correct"
  
Verdict: ISOLATED (replay consistency ≠ semantic authority)
```

**Path 4: Factor Fitting** (✓ Isolated)

```
Path: QCStatistics → TransferFunction → params fitted → accuracy high
Risk: Factor fits perfectly (CHANGES_WITH proof) → factor becomes semantic?

Isolation:
  - TransferFunction.Status = "CANDIDATE" (immutable)
  - CanReuseWith() enforces fingerprint/mapping/version match
  - Factor cannot be promoted to VERIFIED/APPROVED/PRODUCTION status
  - Factor CANNOT modify MappingDefinition
  - Factor CANNOT establish semantic identity (CanonicalConcept/Unit)
  - Factor is diagnostic: "transfer function exists" ≠ "concept is correct"
  
Verdict: ISOLATED (factor consistency ≠ semantic authority)
```

**Path 5: Evidence Gate Shortcut** (✓ Protected)

```
Path: Engineer sees ReviewQC PASS → might assume C1 was thorough?
Risk: "QC validated reference" → "semantic evidence must be solid"?

Protection:
  - Approval.Validate() explicitly checks: liveMapping.Status == EVIDENCE_APPROVED
  - Engineer approval cannot proceed without C1 gate passing
  - Engineer sees frozen CanonicalConcept/Unit in profile
  - Engineer role: "I trust this mapping uses approved semantics"
  - NOT: "I'm approving the semantic meaning itself"
  - C1 evidence gate is mandatory before any later gate
  
Verdict: PROTECTED (approval re-validates C1)
```

**Path 6: Multiple Observations Suggest Identity** (✓ Isolated)

```
Path: ReferenceChallenge (repeated) → observations agree → pattern consistent
Risk: "Million consistent observations" → "must prove what concept is"?

Isolation:
  - TimeOrderedObservationSet knows ReferenceID + MappingID
  - Does not know which CanonicalConcept was claimed
  - Observations are challenge digests (no raw values in set)
  - Statistical summary cannot modify semantic fields
  - PatternCandidate marked cannot_trigger all authority
  - C1 gate: "UNKNOWN remains UNKNOWN without semantic evidence"
  
Verdict: ISOLATED (observation repetition ≠ semantic identity)
```

**Path 7: Profile Builder Implicit Validation** (✓ Protected)

```
Path: BuildFromMappingDefinition() accepts REPLAY_VALIDATED mapping
      → builds profile with CanonicalConcept/Unit
      → profile represents claimed meaning
Risk: "Profile was built" → "semantic meaning must be correct"?

Protection:
  - Profile is built FROM MappingDefinition (which is frozen)
  - Profile does NOT re-validate semantic meaning
  - Profile preserves frozen semantic identity from C1
  - Profile builder is NOT a semantic authority
  - Profile field notes: "no_governed_authority" if not approved
  
Verdict: PROTECTED (profile is snapshot, not validator)
```

**Path 8: Registry Signature Over Semantic Fields** (✓ Protected)

```
Path: Attestation.Signature covers canonical_payload
      canonical_payload includes CanonicalConcept/Unit
      → semantic identity is "signed by registry"?
Risk: Registry signature might appear to validate semantic meaning?

Protection:
  - Registry signature validates ATTESTATION INTEGRITY
  - Does NOT validate semantic meaning (no authority to do so)
  - Registry signs what was already decided upstream
  - Registry precondition: REPLAY_VALIDATED + ENGINEER_APPROVED
  - Semantic meaning was established by C1, before registry
  - Registry adds governance (admission decision) not semantic authority
  
Verdict: PROTECTED (registry signature covers frozen semantics, not validates them)
```

**Path 9: QC Digest Binding Appears Authoritative** (✓ Protected)

```
Path: Attestation.QC.Digest = SHA256(QCKnowledgeRecord)
      Signature covers QC.Digest
      → QC appears to be "sealed in attestation"?
Risk: "QC bound to registry" → "QC validates semantic meaning"?

Protection:
  - QC binding (Phase 3B) is governance binding, not semantic binding
  - QCKnowledgeRecord.Verdict = PASS only gates admission applicability
  - QC PASS does not mean semantic identity is correct
  - QC binding field notes: "applicability" not "semantic_validation"
  - If QC omitted (NOT_APPLICABLE), admission still valid
  
Verdict: PROTECTED (QC binding is governance, not semantic authority)
```

**Path 10: Clinical Concept Similarity** (Human Judgment Risk)

```
Path: Temporal analysis detects trend in "heart_rate" readings
      Engineer notices trend matches known physiological pattern
      → Engineer might assume "concept is definitely heart_rate"?
Risk: Correlates_with physiological pattern → is heart_rate?

Isolation:
  - Engineer role: Decision authority (approve/reject profile)
  - Engineer is not semantic authority (C1 evidence gate is)
  - PatternCandidate marked NeedsHumanReview (flagged for investigation)
  - Pattern analysis is diagnostic (cannot trigger authority changes)
  - Engineer approval still requires C1-approved semantic identity
  - No technical path to bypass semantic evidence requirement
  
Verdict: PARTIALLY ISOLATED (human judgment risk, not technical)
```

---

## 12. Is a SemanticAuthorityBinding Artifact Needed?

**Answer: NO. Existing bindings sufficient BUT with one clarification needed.**

**Current Semantic Authority Chain:**

```
Evidence.Claim (VENDOR_MANUAL, ParserIdentity, CanonicalConcept, CanonicalUnit)
    ↓ [C1 gate validates]
MappingDefinition (CanonicalConcept, CanonicalUnit frozen)
    ↓ [status = EVIDENCE_APPROVED]
Profile (preserves frozen CanonicalConcept, CanonicalUnit)
    ↓ [engineer approval snapshot]
Attestation (includes CanonicalConcept, CanonicalUnit in canonical payload)
    ↓ [signature covers all fields]
Sealed in Ed25519 signature
```

**Why SemanticAuthorityBinding Not Needed:**

1. **Semantic identity is frozen at C1**, never modified downstream
2. **Profile snapshots semantic identity** from MappingDefinition
3. **Attestation includes semantic fields** in signed payload
4. **Registry verification can rebind** to original profile and verify CanonicalConcept/Unit match
5. **QC binding (Phase 3B) is optional** and doesn't affect semantic fields

**What IS Missing:**

```
Attestation.SemanticEvidenceDigest (NOT PRESENT)

Current:
  Attestation.EvidenceReference = "reports/c1_evidence_gate/claim_123.json"
  
Gap:
  No Attestation.SemanticEvidenceDigest = SHA256(evidence_report_content)
  
Impact:
  - Verifier can validate that profile matches admission
  - But CANNOT validate that evidence reference hasn't been replaced
  - Evidence report could be modified post-signature without detection
```

**Recommendation:**

Rather than create new `SemanticAuthorityBinding` artifact:

→ Add `SemanticEvidenceDigest` to `Attestation` (Phase 5B)
→ Include in canonical payload (add to signature)
→ Preserve during registry verification

This is a **field addition**, not a new artifact (lighter weight).

---

## 13. Determine the Smallest Safe Extension, If Any

**Answer: Add SemanticEvidenceDigest binding to Attestation (one field, existing structures).**

**Minimal Extension:**

```go
// File: internal/registry/registry.go
// Type: Attestation

type Attestation struct {
    // ... existing fields ...
    EvidenceReference      string    `json:"evidence_reference"`  // ← already present
    SemanticEvidenceDigest string    `json:"semantic_evidence_digest,omitempty"` // ← ADD THIS
    // ... rest of fields ...
}

// Add during Admit():
// Line ~130-150 (after checkGovernedBindings)
// If liveMapping has evidence reference:
//   Load evidence gate report from EvidenceReference path
//   Compute SemanticEvidenceDigest = SHA256(report_content)
//   Store in attestation
//   Include in canonical payload for signature
```

**Updated canonicalPayload()**:

```go
// internal/registry/registry.go: canonicalPayload()
// Line ~306-325

// After existing fields, add:
if a.SemanticEvidenceDigest != "" {
    fmt.Fprintf(&payload, "%d:%s\n", len(a.SemanticEvidenceDigest), a.SemanticEvidenceDigest)
}
```

**Updated VerifyAttestation()**:

```go
// internal/registry/registry.go: VerifyAttestation()
// Line ~174-202

// After signature validation succeeds:
// If a.SemanticEvidenceDigest != "":
//   Load evidence report from a.EvidenceReference
//   Recompute SHA256(report_content)
//   Compare to a.SemanticEvidenceDigest
//   REJECT if mismatch (evidence was modified)
```

**Impact:**

- **Files modified:** `internal/registry/registry.go` (+ `registry_test.go`)
- **Backward compatible:** YES (field omitempty, existing attestations without digest still validate)
- **Breaking change:** NO (new field optional, old signatures still work)
- **Cryptographic strength:** Increases from "reference path" to "evidence content digest"

**Safety:**

```
Before:
  Registry signature covers EvidenceReference (text path)
  → Evidence report could be renamed/replaced/modified
  → Signature would still validate

After:
  Registry signature covers SemanticEvidenceDigest (SHA256)
  → Evidence report cannot be modified without breaking signature
  → If reference path changes but file unchanged → digest still matches
  → If file changed → digest mismatch → admission invalid
```

---

## 14. Define Tests Proving Constraints

**Test Suite Structure** (Phase 5B implementation):

### Test Group 1: Semantic Identity Cannot Be Created by Non-C1 Evidence

```go
Test_1_UNKNOWN_Concept_Remains_UNKNOWN_Without_VENDOR_MANUAL()
  // Verify that STRUCTURAL/INFERRED evidence cannot create concept
  // Expected: C1 gate rejects, EVIDENCE_APPROVED never reached

Test_2_UNKNOWN_Unit_Remains_UNKNOWN_Without_VENDOR_MANUAL()
  // Verify that STRUCTURAL/INFERRED evidence cannot create unit
  // Expected: C1 gate rejects

Test_3_QC_PASS_Does_Not_Establish_UNKNOWN_Concept()
  // Create mapping with UNKNOWN concept
  // Try to run QC (should fail precondition)
  // Expected: QC cannot run, concept remains UNKNOWN

Test_4_QC_PASS_Does_Not_Establish_UNKNOWN_Unit()
  // Create mapping with UNKNOWN unit
  // Try to run QC (should fail precondition)
  // Expected: QC cannot run, unit remains UNKNOWN

Test_5_Temporal_Pattern_Does_Not_Establish_Concept()
  // Create TimeOrderedObservationSet
  // Detect pattern (trend, shift, recurrence)
  // Try to use pattern to approve mapping
  // Expected: Pattern is diagnostic, mapping unchanged
```

### Test Group 2: Repeated Observations Cannot Establish Meaning

```go
Test_6_Million_Consistent_Observations_Do_Not_Name_Concept()
  // Create 1,000,000 observations of same value
  // Compute statistics (perfect consistency)
  // Try to establish semantic identity from consistency
  // Expected: Semantic identity frozen from C1, not statistics

Test_7_Perfect_Correlation_Does_Not_Establish_Identity()
  // Create pattern: observations perfectly correlate with time
  // QC PASS (perfect consistency)
  // Try to use correlation to change CanonicalConcept
  // Expected: Semantic identity immutable

Test_8_Repeated_Trend_Does_Not_Establish_Meaning()
  // Temporal analysis detects stable trend
  // PatternCandidate.TrendSlope is consistent
  // Try to use trend to approve unknown concept
  // Expected: Trend is diagnostic, concept remains unchanged
```

### Test Group 3: QC PASS Cannot Establish Meaning

```go
Test_9_QC_PASS_Does_Not_Override_C1_Rejection()
  // C1 gate rejects mapping (semantic evidence missing)
  // MappingDefinition.Status = PRE_REVIEW
  // Try to run ReviewQC (should fail precondition)
  // Expected: QC cannot run (requires REPLAY_VALIDATED)

Test_10_QC_PASS_Does_Not_Retroactively_Approve_Semantic()
  // Mapping approved by C1 (semantic identity established)
  // C2 replay succeeds
  // D3G QC verdict = PASS
  // Profile built with frozen CanonicalConcept/Unit
  // Change profile.CanonicalConcept
  // Registry.Admit() called
  // Expected: Admission rejects (CandidateDigest mismatch)

Test_11_QC_INSUFFICIENT_EVIDENCE_Does_Not_Prevent_Profile_Build()
  // Mapping approved by C1
  // C2 replay succeeds
  // D3G QC verdict = INSUFFICIENT_EVIDENCE (not enough data)
  // Applicability = OPTIONAL
  // Profile builder proceeds
  // Expected: Profile built (QC is optional for this mapping)

Test_12_QC_Binding_Is_Governance_Not_Semantic()
  // Attestation.QC.Verdict = "PASS"
  // Attestation.QC.Digest = SHA256(QCKnowledgeRecord)
  // Modify Attestation.CanonicalConcept
  // Registry.VerifyAttestation() called
  // Expected: Signature fails (semantic field was modified)
```

### Test Group 4: Temporal Patterns Cannot Establish Meaning

```go
Test_13_Trend_Candidate_Cannot_Modify_MappingDefinition()
  // Create PatternCandidate (TREND_CANDIDATE)
  // Set TrendSlope = 0.5 (indicating trend)
  // PatternCandidate.CanAuthorityModify() returns false
  // Try to use trend to change mapping
  // Expected: No method exists to modify mapping from pattern

Test_14_Shift_Candidate_Cannot_Trigger_Semantic_Change()
  // Create PatternCandidate (SHIFT_CANDIDATE)
  // Shift detected at sequence 500
  // Try to modify mapping at that point
  // Expected: Pattern is diagnostic, mapping frozen

Test_15_Recurrence_Candidate_Cannot_Establish_Periodicity_Meaning()
  // Create PatternCandidate (RECURRENCE_CANDIDATE)
  // RecurrencePeriod = 60 observations
  // Try to claim "period proves concept is X"
  // Expected: Semantic identity already frozen from C1
```

### Test Group 5: Factor Consistency Cannot Establish Meaning

```go
Test_16_Transfer_Function_Status_Always_CANDIDATE()
  // Create TransferFunction with perfect params fit
  // Verify TransferFunction.Status == "CANDIDATE"
  // Try to promote to VERIFIED
  // Expected: No method to change status, remains CANDIDATE forever

Test_17_Factor_Cannot_Create_Semantic_Identity()
  // Mapping with unknown concept
  // Fit TransferFunction (params converge)
  // Try to use factor to establish concept
  // Expected: Factor is diagnostic, concept remains UNKNOWN

Test_18_Factor_Fingerprint_Binding_Enforced()
  // Create TransferFunction with fingerprint = "dev1"
  // Try CanReuseWith() new fingerprint = "dev2"
  // Expected: CanReuseWith() returns error

Test_19_Factor_Reuse_Does_Not_Change_Semantic_Status()
  // Factor reused with same fingerprint/mapping
  // TransferFunction.Status still = "CANDIDATE"
  // Expected: Reuse is technical, not semantic authority
```

### Test Group 6: Replay Success Cannot Establish Meaning

```go
Test_20_Replay_Validates_Consistency_Not_Meaning()
  // Replay validates false_data_rate ≈ 0.0
  // Replay succeeds (READY_FOR_PROFILE_BUILDER)
  // Modify CanonicalConcept after replay
  // Expected: Semantic identity was frozen at C1, replay didn't validate it

Test_21_Replay_Cannot_Retroactively_Approve_UNKNOWN()
  // Create mapping with UNKNOWN concept
  // Try to run replay
  // Expected: C1 precondition fails (UNKNOWN rejected)

Test_22_Replay_Success_Means_Consistency_Not_Correctness()
  // Mapping approved with concept = "heart_rate" (from C1)
  // Actual device measures "SpO2" (human error in production)
  // Replay succeeds (measurement consistent)
  // Attestation created with CanonicalConcept = "heart_rate"
  // Expected: Replay validates consistency, not semantic correctness
```

### Test Group 7: Approval Cannot Bypass C1

```go
Test_23_Engineer_Approval_Requires_EVIDENCE_APPROVED()
  // Mapping status = PRE_REVIEW (C1 never ran)
  // Try Approval.Review()
  // Expected: Validation fails (liveMapping.Status != EVIDENCE_APPROVED)

Test_24_Approval_Re_Validates_Semantic_Identity_Match()
  // Approval snapshot includes CanonicalConcept/Unit from C1
  // Later, someone modifies CanonicalConcept in profile
  // Try to create attestation with modified profile
  // Expected: Registry.Admit() rejects (semantic mismatch)

Test_25_Engineer_Cannot_Approve_If_C1_Concept_Changed()
  // C1 approves concept = "heart_rate"
  // MappingDefinition.CanonicalConcept = "heart_rate"
  // Profile.CanonicalConcept = "heart_rate" (initially)
  // Someone changes profile.CanonicalConcept = "SpO2"
  // Approval.Validate() called
  // Expected: Validation rejects (concept mismatch)
```

### Test Group 8: Registry Cannot Reconstruct Semantic Authority

```go
Test_26_Registry_Verify_Rejects_Semantic_Mutation()
  // Attestation.CanonicalConcept = "heart_rate" (from C1)
  // Registry signature covers attestation
  // Someone creates modified profile with CanonicalConcept = "SpO2"
  // VerifyAttestation() called with modified profile
  // Expected: Candidate digest mismatch, verification fails

Test_27_Registry_Cannot_Admit_Without_C1_Evidence_Reference()
  // Mapping approved by C1 (EvidenceGateReportRef set)
  // Replay validated
  // QC optional (not checked for this test)
  // Registry.Admit() called
  // Expected: Attestation.EvidenceReference preserved

Test_28_SemanticEvidenceDigest_Binding_Prevents_Evidence_Swap()
  // Admission with SemanticEvidenceDigest = SHA256(original_evidence)
  // Later, evidence report file replaced with different content
  // Registry verifier loads new evidence
  // Recomputes digest → different value
  // Expected: Digest mismatch, admission invalid
```

### Test Group 9: Semantic Identity Provenance Traceability

```go
Test_29_Semantic_Identity_Traces_To_C1_Evidence()
  // Create attestation
  // Follow CanonicalConcept backward to Evidence.Claim
  // Verify Evidence.Claim.EvidenceType == VENDOR_MANUAL
  // Expected: Semantic identity traceable to vendor assertion

Test_30_Attestation_Includes_Evidence_Reference()
  // Attestation.EvidenceReference points to C1 report
  // Load report, verify it approves CanonicalConcept
  // Expected: Provenance chain complete

Test_31_SemanticEvidenceDigest_Proves_Unmodified_Evidence()
  // Recompute SHA256(evidence_report)
  // Compare to Attestation.SemanticEvidenceDigest
  // Expected: Digest matches (evidence unmodified)
```

### Test Group 10: Phase 3B/4B Backward Compatibility

```go
Test_32_Phase_3B_Attestation_Without_QC_Still_Validates()
  // Old attestation (before QC binding)
  // QC field = nil
  // VerifyAttestation() called
  // Expected: Signature validates (QC field optional)

Test_33_Phase_4B_Temporal_Does_Not_Affect_Phase_3B_Attestation()
  // Attestation admits mapping
  // Later, temporal analysis runs on same data
  // PatternCandidate created
  // Try to retroactively add temporal binding to attestation
  // Expected: Attestation signature is sealed, cannot be modified

Test_34_Adding_SemanticEvidenceDigest_Backward_Compatible()
  // Old attestation without SemanticEvidenceDigest
  // New verifier with SemanticEvidenceDigest validation
  // VerifyAttestation() called
  // Expected: Validates successfully (field is omitempty)

Test_35_New_Attestation_With_SemanticEvidenceDigest()
  // New admission includes SemanticEvidenceDigest
  // Verify signature covers digest
  // Modify evidence report
  // Recompute digest → mismatch
  // Expected: Verification fails (backward compatible increase)
```

### Test Group 11: UNKNOWN Preservation Across Pipeline

```go
Test_36_UNKNOWN_Concept_Stops_Pipeline_At_C1()
  // Create claim with CanonicalConcept = "UNKNOWN"
  // ReviewClaim() called
  // Expected: Rejected immediately

Test_37_UNKNOWN_Unit_Stops_Pipeline_At_C1()
  // Create claim with CanonicalUnit = "UNKNOWN"
  // ReviewClaim() called
  // Expected: Rejected immediately

Test_38_Empty_Concept_Rejected_At_C1()
  // Create claim with CanonicalConcept = ""
  // ReviewClaim() called
  // Expected: Rejected immediately

Test_39_Whitespace_Only_Concept_Rejected_At_C1()
  // Create claim with CanonicalConcept = "   "
  // ReviewClaim() called
  // Expected: Rejected immediately

Test_40_UNKNOWN_Concept_Cannot_Reach_ProfileBuilder()
  // C1 rejects UNKNOWN
  // MappingDefinition never reaches EVIDENCE_APPROVED
  // BuildFromMappingDefinition() precondition check
  // Expected: Build fails (mapping never approved)
```

---

## PHASE 5A STATUS

### 1. Semantic Authority Source

**Single point of truth:** `internal/evidence/evidence.go:ReviewClaim()`

- Only VENDOR_MANUAL evidence can establish semantic identity
- CanonicalConcept and CanonicalUnit frozen after C1 approval
- No other gate, component, or evidence type can create or modify semantic meaning
- Semantic identity immutable throughout all downstream gates (C2, D3G, D3H, approval, registry)

### 2. Current Protection

**Strong:**
- ✓ Semantic gates (C1, C2) must pass before profile builder
- ✓ CanonicalConcept/Unit immutable after C1
- ✓ ProfileBuilder binds to frozen semantic identity
- ✓ Engineer approval re-validates semantic field match
- ✓ Registry signature covers semantic fields
- ✓ QC binding (Phase 3B) is optional, governance-only
- ✓ PatternCandidate explicitly marked cannot_trigger authority
- ✓ TransferFunction locked in CANDIDATE status
- ✓ Temporal analysis isolated (diagnostic only)

**Weak:**
- ✗ EvidenceReference is text path, not content digest
- ✗ Registry signature doesn't cover evidence file content
- ✗ Post-signature evidence replacement undetectable
- ✗ SemanticEvidenceDigest not yet bound in Attestation

### 3. Discovered Gaps

**Gap 1: Semantic Evidence Digest Not Bound**

```
Risk: Evidence report file could be modified/replaced after signature
Mitigation: Add SemanticEvidenceDigest to Attestation (Phase 5B)
Severity: Medium (requires file system access to evidence store)
```

**Gap 2: No Explicit Semantic Authority Artifact**

```
Risk: SemanticAuthorityBinding concept exists implicitly but not documented
Mitigation: Documentation (not new artifact needed)
Severity: Low (protection exists, just implicit)
```

**No other gaps discovered.**

Semantic authority is effectively protected by architectural isolation.

### 4. Minimum Safe Extension

**Single addition:** `SemanticEvidenceDigest` field in `Attestation`

```go
type Attestation struct {
    // ... existing fields ...
    EvidenceReference      string    `json:"evidence_reference"`
    SemanticEvidenceDigest string    `json:"semantic_evidence_digest,omitempty"` // ← ADD
}
```

**Implementation scope:**
- Add field to Attestation struct
- Compute digest during Admit() (load evidence file, SHA256)
- Include in canonicalPayload() signature
- Validate in VerifyAttestation() (recompute, compare)
- Add ~30 lines to registry.go
- Add ~15 tests to registry_test.go

**Backward compatibility:** YES (field optional, existing signatures still work)

### 5. Files Potentially Affected

**Phase 5B (if implemented):**
- `internal/registry/registry.go` (add SemanticEvidenceDigest field + logic)
- `internal/registry/registry_test.go` (add binding/verification tests)

**Phase 5A (inspection only):**
- No files modified

### 6. Proposed Phase 5B Tests

**Test Categories:**
1. SemanticEvidenceDigest binding (5 tests)
2. Evidence modification detection (3 tests)
3. Backward compatibility (3 tests)
4. Semantic identity immutability (8 tests)
5. UNKNOWN preservation (5 tests)
6. QC/temporal non-interference (6 tests)
7. Factor status immutability (4 tests)
8. Approval re-validation (3 tests)

**Total:** ~40 tests

### 7. Backward Compatibility Risks

**No breaking changes identified.**

**Risk Assessment:**

```
1. Adding SemanticEvidenceDigest field:
   - Backward compatible (omitempty, old attestations work)
   - New verifiers handle missing digest gracefully
   - No change to signature format (field added to payload)

2. Existing Phase 3B QC binding:
   - Remains compatible (QC field already optional)
   - Phase 5B SemanticEvidenceDigest independent of QC

3. Phase 4B temporal:
   - Unaffected (temporal artifacts separate from attestation)
   - PatternCandidate isolation unchanged

4. Phase 2 QC Engine v1.0:
   - All types immutable (new temporal package doesn't modify)
   - Review.go unchanged (QC logic isolated)

5. Profile builder + approval + existing tests:
   - No modifications required
   - Existing tests continue to pass
```

**Conclusion:** Adding SemanticEvidenceDigest is purely additive, zero breaking changes.

---

## Inspection Complete

**Result:** Semantic authority boundary is strong. QC, temporal, factor, replay, and approval are all properly isolated. Statistical evidence cannot substitute for semantic authority.

**Single weakness:** Evidence file content not cryptographically bound at admission time (Phase 5B remediation: add SemanticEvidenceDigest).

**Constitutional law upheld:**
- READABLE ≠ UNDERSTOOD ✓
- UNDERSTOOD ≠ VERIFIED ✓
- STATISTICALLY CONSISTENT ≠ SEMANTICALLY IDENTIFIED ✓
- CHANGES_WITH ≠ IS ✓
- REPEATS_WITH ≠ IS ✓
- CORRELATES_WITH ≠ IS ✓

**A MILLION CONSISTENT OBSERVATIONS SHALL NOT NAME AN UNKNOWN PARAMETER** ✓ (enforced by C1 gate)

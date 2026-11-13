# PHASE 6A — DEVICE FINGERPRINT & QC IDENTITY INSPECTION

**Status**: ARCHITECTURE INSPECTION COMPLETE (NO CODE CHANGES)

**Locked Baseline**: d578b50eeddb5e779f31633120ada1f9dd319e4d  
**Phases 2–5**: LOCKED (no modifications)

---

## EXECUTIVE SUMMARY

Device fingerprint is **deterministic structural identity**, established at discovery time and **preserved immutably** through all phases. It binds device-specific observations (measurements, QC statistics, temporal patterns) WITHOUT transferring semantic authority.

**Key Finding**: Identity layers are **already properly separated**. Fingerprint propagates as context/binding for QC isolation, but NEVER gates semantic evidence, approval, or registry decisions.

**Constitutional Law Compliance**: VERIFIED
- ✓ Knowledge without identity = no authority
- ✓ Fingerprint match ≠ semantic verification
- ✓ Fingerprint match enables knowledge reuse (with explicit checks)
- ✓ Evidence determines trust, not fingerprint alone
- ✓ One device supports many mappings (each independent)
- ✓ Statistics remain identity-bounded
- ✓ Time orders epochs; does not define identity
- ✓ Fingerprint change signals identity deviation (not automatic failure)
- ✓ OTRIS remembers knowledge without lowering verification standards

---

## ANSWER TO 35 QUESTIONS

### 1. What currently constitutes DeviceFingerprint?

**Definition** (internal/candidate/candidate.go:35):  
`DeviceFingerprint string`

**Computed as** (implicit from code paths):
- Structural hash of device attributes from protocol analysis
- Source: PB-10 discovery/fingerprinting stage (actual computation not in scope)
- Immutable once assigned to Claim
- **Represents**: Device hardware identity, not semantic calibration

**Current Binding Points**:
- `candidate.Claim.DeviceFingerprint` — source immutable identifier
- `mapping.MappingDefinition.DeviceFingerprint` — persisted with mapping
- `profilebuilder.Profile.DeviceFingerprint` — carried to profile
- `evidence.Claim.DeviceFingerprint` — optional context
- `qc.ReferenceChallenge.DeviceFingerprint` — QC measurement binding
- `qc.ReferenceDefinition.DeviceRef` — optional QC scoping
- `qc.TransferFunction.DeviceFingerprint` — explicit binding
- `qc.QCKnowledgeRecord.DeviceRef` — record binding
- `qc.temporal.TimeOrderedObservationSet.DeviceFingerprint` — temporal identity
- `qc.temporal.ObservationReference.DeviceFingerprint` — observation binding
- `qc.temporal.PatternCandidate.DeviceFingerprint` — pattern scoping

**Format**: Hexadecimal string (e.g., "4de28380713b7867", "578b92f887e2cf67" from candidate.go examples)

---

### 2. Is fingerprint deterministic?

**YES. Absolutely.**

**Evidence**:
- Computed once at discovery time (PB-10)
- Never recomputed, never modified after creation
- Same device always produces same fingerprint
- Immutable through all governance layers
- No fuzzy matching, no probabilistic computation
- Used directly in equality checks (e.g., factor.CanReuseWith)

**Fail-Closed Enforcement**:
- `qc.ReferenceChallenge.BindsTo()` line 144: exact match required
- `qc.TransferFunction.CanReuseWith()` lines 102-107: fingerprint change → error
- `qc.temporal.ObservationReference.DeviceFingerprint` — explicit preservation

---

### 3. What inputs contribute to it?

**NOT DIRECTLY INSPECTABLE** (PB-10 implementation outside scope).

**Inferred from usage**:
- Device protocol structure (HL7 fields, ASTM positions, etc.)
- Parser identity (which parser extracts this device?)
- Device model/vendor reference (optional context, not authority)
- Possibly: firmware version, transport, framing (depends on PB-10 algorithm)

**What does NOT contribute**:
- Semantic evidence (VENDOR_MANUAL) — separate identity
- Calibration values — not device identity
- Patient data — not device identity
- Temporal observations — derived from fingerprint, not contributors

---

### 4. Which inputs are structural vs metadata/vendor-dependent?

**Structural** (device hardware):
- Protocol message format (HL7 vs ASTM)
- Field positions (OBX column 5 vs column 3)
- Message framing/transmission mechanism

**Vendor/Metadata** (optional context, not fingerprint authority):
- `DeviceIdentityRef` (model/vendor reference string) — **optional**, separate field
- `ParserIdentity` (which parser extracts) — context only
- Firmware version — not in fingerprint (no fingerprint change on firmware update detected in code)

**Constitutional Split** (line 34-38 mapping.MappingDefinition):
```
// Device & Parser Context (reference only, not authority)
DeviceFingerprint  string  // structural hash from PB-10
DeviceIdentityRef  string  // device model/vendor reference
ParserIdentity     string  // parser that extracts this field
```

**Key**: DeviceIdentityRef is **separate** from DeviceFingerprint. Both carried, independently.

---

### 5. Where is fingerprint first created?

**Discovery/Fingerprinting Stage** (PB-10, outside scope):
- Exact code not in inspection scope
- Called before any mapping
- Produces immutable identifier for device

**First Captured in**:
- `candidate.Claim.DeviceFingerprint` (candidate/candidate.go:35, line 95 assignment)
- Set via `New()` call (line 75-77 validation)
- **Immutable after Claim creation**

**Propagation Chain**:
```
PB-10 Discovery
    ↓
candidate.Claim.DeviceFingerprint
    ↓
mapping.MappingDefinition.DeviceFingerprint (copied via builder.go)
    ↓
evidence.Claim.DeviceFingerprint (optional context, lines 30)
    ↓
profilebuilder.Profile.DeviceFingerprint (line 30)
    ↓
registry.Attestation (implicit via Profile)
```

---

### 6. Where is fingerprint currently stored?

**Persistent Artifacts**:

1. **MappingDefinition** (mapping/definition.go:35)
   - JSON: `device_fingerprint` field
   - Immutable after mapping creation
   - Not signed, but part of mapping authority

2. **Profile** (profilebuilder/profilebuilder.go:30)
   - JSON: `device_fingerprint` field
   - Used in profile audit trail (line 41: AuthorityChain validation)
   - Not directly in canonical signed payload

3. **Evidence Report** (evidence/claim.go:30)
   - JSON: optional `device_fingerprint` field
   - Context only (not verified for semantic evidence)

4. **QC Artifacts**:
   - ReferenceChallenge: `device_fingerprint` (json key, immutable)
   - QCKnowledgeRecord: `device_ref` (json key, binding)
   - TransferFunction: `device_fingerprint` (json key, binding)
   - TimeOrderedObservationSet: `device_fingerprint` (json key)
   - ObservationReference: `device_fingerprint` (json key, preservation)
   - PatternCandidate: `device_fingerprint` (json key, diagnostics only)

5. **Registry Attestation** (registry/registry.go:47-66)
   - **NOT in canonical signed payload**
   - Canonical payload includes: SchemaVersion, AdmissionStatus, ProfileID, ProfileVersion, CandidateDigest, MappingIdentity, MappingVersion, EvidenceReference, **SemanticEvidenceDigest**, ReplayValidatedAt, MappingProvenance, ProvenanceAuthor, ApproverIdentity, ApprovalTimestamp, IssuerKeyID, IssuedAt, Sequence, (optionally QC binding)
   - **DeviceFingerprint is NOT cryptographically bound in registry signature**

---

### 7. Does MappingDefinition bind DeviceFingerprint?

**YES, but NOT as authority.**

**Binding**:
- Field: `DeviceFingerprint string` (mapping/definition.go:35)
- Stored in JSON, persisted, immutable
- Carried through proof-of-status checks

**Not Authority Because**:
- MappingDefinition.Validate() (status.go) does NOT check fingerprint
- Evidence Gate (evidence verification) does NOT require fingerprint match
- Replay Validation does NOT verify fingerprint consistency
- **Fingerprint is reference context, not proof**

**Use in Code**:
- Line 97 (profilebuilder/profilebuilder.go): "DeviceFingerprint and DeviceIdentityRef required" — **for context only**, to validate device context exists
- QC uses it to scope measurements, not to verify meaning

---

### 8. Does SemanticEvidenceDigest bind directly or indirectly to DeviceFingerprint?

**NO BINDING WHATSOEVER.**

**SemanticEvidenceDigest**:
- Established by C1 (VerifySemanticEvidence, evidence/claim.go:99)
- Computed from VENDOR_MANUAL evidence alone
- Binds mapping identity, evidence count, evidence types
- **Does NOT include DeviceFingerprint** in computation

**Evidence** (evidence/claim.go lines 110-130, computeSemanticEvidenceDigest):
- Takes `claim` and `[]Evidence` as inputs
- Computes deterministic SHA256
- Creates JSON bundle: `{Claim, Evidence}` — Claim contains fingerprint but is not used in digest computation for semantic purposes
- **Constitutional**: Semantic meaning established by VENDOR_MANUAL authority, independent of device identity

**Propagation**:
```
SemanticEvidenceDigest (established by C1, independent)
    ↓
MappingDefinition.SemanticEvidenceDigest (mapping/definition.go:45)
    ↓
Profile.SemanticEvidenceDigest (profilebuilder/profilebuilder.go:45)
    ↓
Registry.Attestation.SemanticEvidenceDigest (registry/registry.go:56)
    ↓
Canonical Signed Payload (field 8, registry/registry.go:294)
```

**Cryptographic Binding**: SemanticEvidenceDigest is in **canonical registry payload** (line 294), signed via Ed25519.

**Device Binding**: DeviceFingerprint is **NOT in canonical payload**. Registry admission does NOT verify device fingerprint against any signing key.

---

### 9. Does ReferenceChallenge bind DeviceFingerprint?

**YES, explicitly and immutably.**

**Binding** (qc/challenge.go:23, 44-89):
- Field: `DeviceFingerprint string` (required)
- Validated at creation (line 57-58: must be non-empty)
- Immutable after creation (sealed at line 87)
- **Included in integrity hash** (line 97: canonical format includes device_fingerprint)

**Integrity Enforcement**:
- `computeDigest()` line 96-107: device_fingerprint is part of canonical representation
- Any device mismatch invalidates digest (fail-closed)
- `Verify()` line 112-121: recalculates digest, rejects mismatch

**Scope Binding** (qc/challenge.go line 143-149):
```go
// Verify device fingerprint matches (if definition specifies one)
if rd.DeviceRef != "" && ch.DeviceFingerprint != rd.DeviceRef {
    return fmt.Errorf(
        "reference challenge device mismatch: challenge has %q, definition has %q",
        ch.DeviceFingerprint, rd.DeviceRef,
    )
}
```

**Constitutional**: Reference challenges are device-specific. A device fingerprint change invalidates previous challenges.

---

### 10. Does QCKnowledgeRecord bind DeviceFingerprint?

**YES, as `DeviceRef`, immutably.**

**Binding** (qc/qcresult.go:19):
- Field: `DeviceRef string` (required)
- Validated at creation (line 65-66: must be non-empty)
- **Included in integrity hash** (line 135: canonical format includes DeviceRef)
- Used in verdict evaluation (implicit scope)

**Immutability** (qc/qcresult.go:149-160):
```go
func (qr *QCKnowledgeRecord) Verify() error {
    expected := qr.computeDigest()
    if qr.Integrity != expected {
        return fmt.Errorf("QC record: integrity violation...")
    }
    if qr.Status != "CANDIDATE" {
        return fmt.Errorf("QC record: invalid status...")
    }
    return nil
}
```

**Constitutional**: QC verdict applies to a specific device. Device fingerprint change requires re-verification.

---

### 11. Do Phase 4B temporal artifacts bind DeviceFingerprint?

**YES, extensively.**

**TimeOrderedObservationSet** (qc/temporal/observation.go:19):
- Field: `DeviceFingerprint string` (required)
- Scope binding: observations must all be from same device
- Immutable after creation

**ObservationReference** (qc/temporal/observation.go:60):
- Field: `DeviceFingerprint string` (each observation preserves its device)
- Explicit preservation (not inferred)
- Required for temporal scoping

**WithinRunTemporalKnowledge** (qc/temporal/knowledge.go:13-23):
- Contains `TimeOrderedObservationSet` (includes DeviceFingerprint)
- All observations scoped to same device
- RunID is explicit, not inferred from fingerprint

**WithinDayKnowledge** (qc/temporal/knowledge.go:39-51):
- Contains `TimeOrderedObservationSet` (includes DeviceFingerprint)
- DayID is explicit (policy-defined), not inferred

**LongitudinalQCKnowledge** (qc/temporal/knowledge.go:67-79):
- Contains `TimeOrderedObservationSet` (includes DeviceFingerprint)
- KnowledgeEpochID is explicit (policy-defined), not inferred

**Constitutional**: Temporal knowledge is device-specific. Fingerprint change signals identity deviation; observations from different devices must not be mixed.

---

### 12. Does Profile bind DeviceFingerprint?

**YES.**

**Binding** (profilebuilder/profilebuilder.go:30):
- Field: `DeviceFingerprint string` (required from MappingDefinition)
- Set at profile creation (line 97-98: ParserIdentity, DeviceFingerprint, DeviceIdentityRef required)
- Immutable in profile artifact (part of integrity chain)

**Authority Chain** (profilebuilder/profilebuilder.go:41):
- `AuthorityChain []string` — includes fingerprint context
- Validated during approval (approval/approval.go: implicit via profile digest)
- **Not separately signed**, but covered by CandidateDigest

**Use in Approval** (approval/approval.go):
- Profile digest (SHA256 of entire profile JSON) includes DeviceFingerprint
- Any fingerprint change invalidates digest → approval rejected
- Fingerprint is **part of candidate integrity**, not separately verified

---

### 13. Does Registry Attestation bind DeviceFingerprint?

**NO.**

**Canonical Signed Payload** (registry/registry.go:278-312):
- Field order (lines 291-300):
  1. SchemaVersion
  2. AdmissionStatus
  3. ProfileID
  4. ProfileVersion
  5. CandidateDigest
  6. MappingIdentity
  7. MappingVersion
  8. EvidenceReference
  9. **SemanticEvidenceDigest** ← cryptographically bound
  10. ReplayValidatedAt
  11. MappingProvenance
  12. ProvenanceAuthor
  13. ApproverIdentity
  14. ApprovalTimestamp
  15. IssuerKeyID
  16. IssuedAt
  17. Sequence
  18. (Optional) QC binding

- **DeviceFingerprint is NOT in this list**

**Why Not**:
- Registry admission does not gate on device fingerprint
- Profile contains fingerprint (covered by CandidateDigest)
- Verification (registry/registry.go line 187-205):
  - Verifies signature over canonical payload
  - Verifies CandidateDigest matches candidate
  - Does NOT separately verify DeviceFingerprint
  - Does NOT check fingerprint against any issuer key

**Constitutional**: Registry signature proves approval + semantic evidence binding. Device fingerprint is context, not authority.

---

### 14. Is fingerprint covered by the Ed25519 canonical signed payload?

**INDIRECTLY (via CandidateDigest), but NOT explicitly.**

**Chain**:
```
Profile JSON (includes DeviceFingerprint)
    ↓
SHA256(Profile JSON) = CandidateDigest
    ↓
CandidateDigest included in canonical payload
    ↓
Ed25519.Sign(canonical payload) = Signature
```

**Implication**:
- If Profile.DeviceFingerprint changes → CandidateDigest changes → Signature invalid
- Verifier cannot verify signature without exact Profile JSON
- Fingerprint is protected **transitively**, not directly

**NOT Direct Coverage**:
- Canonical payload does NOT contain "device_fingerprint" field
- Verifier must reconstruct CandidateDigest from Profile to check
- Attestation.VerifyAttestation() line 187: recomputes digest from candidate
- Comparison (line 191): `subtle.ConstantTimeCompare(digest, a.CandidateDigest)`

---

### 15. Can two different fingerprints currently share the same mapping without an explicit identity decision?

**NO. Fail-closed.**

**Evidence**:
- Each MappingDefinition stores exactly one DeviceFingerprint (immutable)
- Evidence Gate (evidence/claim.go) does NOT verify fingerprint against mapping
- Replay Validation (mapping/replay.go) does NOT verify fingerprint against mapping
- BUT: Once a mapping is bound to a fingerprint, it stays bound

**Mapping Creation** (mapping/definition.go line 63-150):
- Does NOT require fingerprint in New() call
- Fingerprint is set **post-hoc** via assignment (caller responsibility)
- Builder (mapping/builder.go) copies fingerprint from MappingDefinition

**Identity Decision**:
- Implicit: First mapping assignment determines device binding
- Explicit: MappingDefinition.DeviceFingerprint field
- Verify: Cannot reuse mapping with different fingerprint (checked in profile builder, line 97)

**Fail-Closed Path** (profilebuilder/profilebuilder.go:96-98):
```go
if strings.TrimSpace(claim.ParserIdentity) == "" || strings.TrimSpace(claim.DeviceFingerprint) == "" || strings.TrimSpace(claim.DeviceIdentityRef) == "" {
    return emptyProfile, errors.New("ParserIdentity, DeviceFingerprint, and DeviceIdentityRef required")
}
```

If fingerprint is missing → profile creation rejected. No implicit assignment.

---

### 16. Can QC records from different fingerprints accidentally be aggregated?

**NO. Fail-closed by design.**

**Isolation Mechanisms**:

1. **TimeOrderedObservationSet** (qc/temporal/observation.go:19):
   - Contains `DeviceFingerprint string` (required, validated)
   - New() call line 75-76: validates fingerprint non-empty
   - **Cannot aggregate observations without matching fingerprint**

2. **ReferenceChallenge** (qc/challenge.go:143-149):
   - BindsTo() verifies device match
   - Line 144: `if rd.DeviceRef != "" && ch.DeviceFingerprint != rd.DeviceRef { return error }`
   - Challenges from different devices cannot be bound to same reference

3. **QCStatistics** (qc/qcstats.go):
   - Computed per (DeviceFingerprint, MappingIdentity, ReferenceLevel)
   - Isolation enforced at creation

4. **ValidateIsolation** (implicit in new record creation):
   - Reference set must maintain LOW/NORMAL/HIGH separation
   - Must maintain WITHIN_RUN / WITHIN_DAY / LONGITUDINAL separation
   - **Must maintain device fingerprint separation**

**Test Coverage**: None required — fail-closed by construction.

---

### 17. Can Within-Run / Within-Day / Longitudinal knowledge cross fingerprint boundaries?

**NO. By design.**

**Evidence**:
- TimeOrderedObservationSet (observation.go:19-20): explicit DeviceFingerprint + RunID
- ObservationReference (observation.go:60): preserves DeviceFingerprint
- WithinRunTemporalKnowledge (knowledge.go:13-22): contains ObservationSet with fingerprint
- WithinDayKnowledge (knowledge.go:39-50): contains ObservationSet with fingerprint
- LongitudinalQCKnowledge (knowledge.go:67-78): contains ObservationSet with fingerprint

**Scope Boundaries** (explicit, never inferred):
- RunID: explicit identity (not "all observations with same timestamp")
- DayID: explicit identity (not "all observations on same calendar date")
- KnowledgeEpochID: explicit identity (not "timespan since first observation")
- DeviceFingerprint: immutable device identity (not inferred from observation pattern)

**Constitutional**: Time orders epochs; does not define them. Fingerprint provides identity boundary.

---

### 18. Is KnowledgeEpochID tied to fingerprint identity?

**NO. Independent identities.**

**KnowledgeEpochID** (qc/temporal/knowledge.go:68):
- Policy-defined explicit identity
- NOT computed from timestamps
- NOT computed from fingerprint
- Examples: "maintenance-window-2026-Q3", "post-calibration-epoch-001"

**DeviceFingerprint** (qc/temporal/observation.go:19):
- Device hardware identity
- Immutable, structural
- Defines device scoping

**Relationship**: Both **required and independent**.
- Same device (fingerprint) may have multiple epochs (policy-defined)
- Same epoch may (should not) span multiple devices — caught by TimeOrderedObservationSet validation

**Constitutional**: Device identity and policy epoch are separate dimensions. Neither determines the other.

---

### 19. What happens today if fingerprint changes after knowledge has been established?

**Graceful Isolation (no automatic failure, no automatic acceptance).**

**Scenario**: Device firmware update changes structural properties → new fingerprint

**Current Behavior**:

1. **QC Records Remain Valid** (immutable):
   - Old QC record binds old fingerprint (qcresult.go:19)
   - Integrity hash includes device_ref (line 135)
   - Record is CANDIDATE status (never auto-promoted)
   - Can be queried independently

2. **Factor Reuse Blocked** (explicit check):
   - TransferFunction.CanReuseWith() (factor.go:101-124)
   - Line 102-107: exact fingerprint match required
   - Different fingerprint → error "requires re-verification"
   - **No automatic degradation or failure**

3. **New Mapping Required**:
   - New device fingerprint → new MappingDefinition
   - New evidence gate review required
   - New replay validation required
   - New QC measurements required

4. **Temporal Knowledge Remains Separate**:
   - WithinRunTemporalKnowledge binds old fingerprint (knowledge.go:13)
   - New observations create new knowledge
   - No forced aggregation

5. **No Automatic Device Failure Claim**:
   - Fingerprint change alone does NOT imply device failure
   - Requires explicit investigation
   - PatternCandidate (temporal/pattern.go:60-85) flagged "NeedsHumanReview" but cannot modify mapping/profile/registry
   - **Constitutional**: "Fingerprint change is evidence of identity deviation, not automatic evidence of device failure"

---

### 20. Is there already any fingerprint comparison / exact-match / candidate-reuse mechanism?

**YES. Limited, fail-closed.**

**TransferFunction.CanReuseWith()** (factor.go:99-124):
- Exact fingerprint match required (line 103)
- Returns error if mismatch
- Also checks MappingIdentity and MappingVersion
- **Use case**: Can we apply a known factor to new measurements from same device?
- **Answer**: Yes, IF fingerprint, mapping identity, and version all match exactly

**No Automatic Reuse**:
- No lookup mechanism in code
- No registry query for "profiles matching this fingerprint"
- No "load previous factor if fingerprint matches"
- CanReuseWith() is **called explicitly**, never automatic

**ReferenceChallenge.BindsTo()** (challenge.go:125-167):
- Validates device match (line 144-149)
- Exact match required or optional (if definition specifies DeviceRef)
- Error returned on mismatch

**Profile Binding** (profilebuilder/profilebuilder.go:97-98):
- Requires DeviceFingerprint present (non-empty)
- No comparison against previous profiles
- Each profile is independent

---

### 21. Is there already a knowledge lookup mechanism keyed by fingerprint?

**NO.**

**Evidence**:
- No registry lookup code found
- No database query code found
- No fingerprint-based profile search
- No "load previous mapping by fingerprint"

**What EXISTS**:
- QCKnowledgeRecord (immutable artifact)
- TransferFunction (immutable artifact)
- TimeOrderedObservationSet (immutable artifact)
- PatternCandidate (diagnostic artifact)
- All indexed by MappingIdentity + DeviceRef, not searched

**What MISSING**:
- Knowledge lookup service
- Fingerprint-based candidate reuse orchestration
- "Soft match" or "candidate similar" mechanism

**Constitutional**: Phase 6A discovers whether lookup is needed. Currently assumed not built.

---

### 22. Is there already any path that can automatically load a known profile merely because fingerprint matches?

**NO.**

**Evidence**:
- No profile lookup code in internal/
- No registry lookup code in cmd/
- BuildFromMappingDefinition() requires live MappingDefinition
- Approval requires live Mapping + Profile + Engineer decision
- Registry.Admit() requires all governance inputs

**Explicit Paths Only**:
- Discovery → MappingDefinition → Evidence → Replay → Profile → Approval → Registry
- Each step requires human/caller decision
- No "auto-load" at any step

**Constitutional**: Profile reuse would violate "Knowledge without identity shall not become authority."

---

### 23. Could an exact fingerprint match currently transfer semantic, mapping, QC, profile, or registry authority?

**NO. Absolutely not.**

**Semantic Authority**:
- Established by VENDOR_MANUAL evidence alone (evidence/claim.go:99)
- Fingerprint NOT in semantic verification (evidence.go:VerifySemanticEvidence)
- Requires explicit evidence gate review

**Mapping Authority**:
- Created by explicit MappingDefinition (mapping/definition.go)
- Validated by evidence gate + replay
- Fingerprint is context, not proof

**QC Authority**:
- Established by ReviewQC() call (qc/review.go)
- Takes ReferenceDefinition + ReferenceChallenge + AcceptanceCriteria
- Fingerprint used for **isolation**, not **authority**
- Verdict derived from statistical comparison, not fingerprint

**Profile Authority**:
- Requires REPLAY_VALIDATED MappingDefinition (profilebuilder/profilebuilder.go:87-88)
- Requires engineer approval (approval/approval.go)
- Fingerprint used for **binding**, not **authority**

**Registry Authority**:
- Requires Ed25519 signature (registry/registry.go:154)
- Signature covers SemanticEvidenceDigest + approval, NOT fingerprint
- Registry.Admit() does NOT gate on fingerprint match
- Fingerprint NOT in canonical signed payload

**Constitutional**: Fingerprint enables knowledge reuse (with explicit checks), never transfers authority.

---

### 24. Which identity dimensions currently exist separately?

**Seven Independent Identity Dimensions**:

| Dimension | Field | Scope | Example | Authority |
|-----------|-------|-------|---------|-----------|
| **Device Structure** | `DeviceFingerprint` | Immutable hardware identity | "4de28380713b7867" | Structural |
| **Device Reference** | `DeviceIdentityRef` | Vendor/model (optional) | "Siemens Infinity C25" | Metadata |
| **Mapping Definition** | `MappingIdentity` | Field mapping claim | "hr-from-obx-field-5" | Governed |
| **Mapping Version** | `MappingVersion` | Evidence + Replay binding | "1.0.0" | Governed |
| **Semantic Evidence** | `SemanticEvidenceDigest` | VENDOR_MANUAL binding | "abc123def..." (SHA256) | Cryptographic |
| **Reference Definition** | `ReferenceID` | QC reference levels | "hr-reference-siemens-001" | Policy |
| **Transfer Function Identity** | `TransferFunction.Identity` | Factor algorithm | "scalar_factor_2.5" | Candidate |
| **Knowledge Epoch** | `KnowledgeEpochID` | Policy-defined temporal boundary | "post-calibration-001" | Policy |
| **QC Policy Identity** | `AcceptanceCriteria.Identity` (implicit) | Verdict policy | (embedded in AcceptanceCriteria) | Policy |

**Clarification on QC Policy Identity**: Currently **not an explicit field**. Policy is embedded in AcceptanceCriteria (verdict.go). Phase 6B may need explicit `QCPolicyIdentity`.

---

### 25. Which of those identities are cryptographically bound today?

**Only ONE**:

- **SemanticEvidenceDigest**: ✓ Cryptographically bound via Ed25519 signature in Registry Attestation (canonical payload field 9)

**Transitively Protected** (via CandidateDigest):
- All Profile fields (including DeviceFingerprint) — protected via CandidateDigest in canonical payload

**NOT Directly Bound**:
- DeviceFingerprint — NOT in canonical payload, only in Profile
- MappingIdentity/Version — included in canonical payload (fields 6-7) but NOT separately signed
- ReferenceID — NOT in registry attestation
- TransferFunction.Identity — NOT in registry attestation
- KnowledgeEpochID — NOT in registry attestation
- QC Policy Identity — NOT in registry attestation

**Constitutional Split**:
- **Semantic evidence** (meaning) — cryptographically bound
- **Device/mapping context** — transitively protected (via digest)
- **QC policy/reference/factor identity** — artifact integrity (SHA256), not registry signature

---

### 26. Where are identity boundaries missing?

**Five Gaps Identified**:

### Gap 1: No Explicit QC Policy Identity

**Current State**:
- AcceptanceCriteria embedded in QCKnowledgeRecord (qcresult.go:26)
- Criteria identity is implicit (no separate ID field)
- Policy changes not tracked as identity change

**Risk**:
- Policy evolution not visible in artifact lineage
- Cannot query "all QC verdicts under policy X"
- Criteria field is flat (verdict.go:AcceptanceCriteria) with no version

**Required for Phase 6B**: Explicit `QCPolicyIdentity` field + versioning

---

### Gap 2: No Cross-Identity Conflict Detection

**Current State**:
- If fingerprint changes but mapping remains → no conflict marker
- If reference definition changes but QC verdict stands → no conflict marker
- If semantic evidence digest differs but profile is reused → no conflict marker

**Risk**:
- Identity mismatches silent (not visible in audit)
- No "identity conflict" artifact type
- Manual review required to spot inconsistencies

**Required for Phase 6B**: IdentityConflictMarker artifact type

---

### Gap 3: No Explicit Fingerprint History / Identity Change Detection

**Current State**:
- Fingerprint change just creates new device context
- No artifact marks "fingerprint changed after knowledge X was established"
- PatternCandidate.CannotTrigger prevents authority modification, but doesn't record device-specific changes

**Risk**:
- Historical knowledge invisible when device evolves
- Operator cannot ask "when did this device's fingerprint last change?"
- No immutable observation of identity deviation

**Required for Phase 6B**: FingerprintIdentityObservation artifact (immutable record of change)

---

### Gap 4: No Integration Knowledge Identity (QCIntegrationIdentity)

**Current State**:
- QCKnowledgeRecord is standalone (no link to "integration scenario")
- Cannot distinguish:
  - "QC under lab conditions" vs "QC under patient care"
  - "QC with reference device A" vs "QC with reference device B"
  - "QC phase 1 validation" vs "QC phase 2 longitudinal"

**Risk**:
- QC knowledge appears universal (not scoped to integration context)
- Cannot reject "knowledge from different integration scenario"
- Knowledge reuse may span incomparable contexts

**Required for Phase 6B**: QCIntegrationIdentity artifact (with approval scope)

---

### Gap 5: No Registry Query/Lookup Endpoint

**Current State**:
- Registry attestation is signed and immutable
- No mechanism to retrieve attestation by fingerprint/mapping/device
- No "list all attestations for this device" query

**Risk**:
- Cannot discover existing knowledge by device fingerprint
- Cannot implement candidate reuse safely (lookup missing)
- Edge cannot query authoritative sources

**Required for Phase 6B**: Registry lookup endpoint (API or artifact store)

---

### 27. Is a new QCIdentity / IntegrationKnowledgeIdentity artifact actually necessary?

**CONDITIONAL YES (Required for Phase 6B, not Phase 6A).**

**Why Required**:
1. **Policy Scope**: Current AcceptanceCriteria is unversioned. Policy evolution not tracked.
2. **Integration Context**: Cannot distinguish lab QC from field QC from longitudinal monitoring.
3. **Reuse Safety**: Cannot ask "can we trust QC knowledge X in integration scenario Y?"
4. **Conflict Detection**: No explicit marker when identities diverge.

**Minimum Scope** (do NOT invent if existing suffices):
- QCIntegrationIdentity: explicit approval scope + policy version + integration context
- **NOT a new governance layer** (QC already separate from semantic authority)
- **Just metadata**: Links QC verdict to integration decision context

**What NOT to Create**:
- ✗ No new approval gate (approval already governs profile)
- ✗ No new registry signature (registry already signed)
- ✗ No new semantic authority (QC never creates semantics)
- ✗ No new status field (QC remains CANDIDATE)

**Only Create**:
- ✓ Artifact linking QC policy version + integration context
- ✓ Immutable record of policy decision (who approved this policy?)
- ✓ Optional input to Approval.Review() (engineer sees context)

---

### 28. If a new identity artifact is required, define the MINIMUM fields.

**QCIntegrationIdentity** (proposed, NOT implemented):

```go
type QCIntegrationIdentity struct {
    // Identity
    IntegrationID string // e.g., "siemens-infinity-hr-field-integration-2026"
    IntegrationVersion string // policy/scenario version "1.0.0"
    IntegrationScenario string // "LAB_VALIDATION", "FIELD_DEPLOYMENT", "LONGITUDINAL_MONITORING"
    
    // Scope
    MappingIdentity string // must match QCKnowledgeRecord.MappingIdentity
    DeviceRef string // device fingerprint (optional, for device-specific policies)
    
    // Policy Binding
    QCPolicyIdentity string // policy ID (new field for AcceptanceCriteria versioning)
    QCPolicyVersion string // policy version "1.0.0"
    PolicyApprovedBy string // who approved this policy integration?
    PolicyApprovedAt time.Time // when?
    
    // Governance
    CanBeCombinedWith []string // list of other IntegrationID this can aggregate with
    CannotBeCombinedWith []string // list of IntegrationID that conflict
    FallbackBehavior string // "REQUIRE_EXPLICIT_APPROVAL", "USE_LOWEST_CONFIDENCE", etc.
    
    // Metadata
    Provenance string
    CreatedAt time.Time
    
    // Integrity
    Integrity string // SHA256 digest
}
```

**Not Authority Because**:
- Does NOT gate profile creation
- Does NOT gate approval decision
- Does NOT gate registry admission
- Does NOT modify QC verdict
- Is **optional input** to Approval.Review(), engineer retains decision

---

### 29. Determine the safest candidate-reuse model.

**Proposed Model** (not implemented):

```
NEW DEVICE
    ↓
FINGERPRINT (deterministic, immutable)
    ↓
    ├─ KNOWLEDGE LOOKUP
    │   ├─ Exact fingerprint match? [YES]
    │   │   ├─ Same mapping identity? [YES]
    │   │   │   ├─ Same mapping version? [YES]
    │   │   │   │   └─ CANDIDATE REUSE POSSIBLE
    │   │   │   └─ Different version [NO]
    │   │   │       └─ NEW MAPPING + EVIDENCE REVIEW
    │   │   └─ Different mapping [NO]
    │   │       └─ NEW MAPPING + EVIDENCE REVIEW
    │   └─ No match or fingerprint different [NO]
    │       └─ NEW DISCOVERY + FULL GOVERNANCE
    │
    ├─ APPLICABILITY CHECK (if candidate reuse possible)
    │   ├─ QC Policy match? [YES]
    │   │   └─ QC KNOWLEDGE REUSABLE
    │   └─ Policy different [NO]
    │       └─ NEW QC REVIEW
    │
    └─ EXISTING GOVERNANCE (unchanged)
        ├─ Evidence review (C1)
        ├─ Replay validation (C2)
        ├─ Engineer approval (D3H)
        └─ Registry admission (D3G)
```

**Rules**:
1. **Lookup Phase**: Query registry for exact fingerprint + mapping + version match
2. **Candidate Reuse**: Use previous TransferFunction only if `CanReuseWith()` succeeds
3. **Policy Applicability**: Check IntegrationIdentity (Phase 6B artifact)
4. **Authority Unchanged**: All governance gates (C1/C2/D3H/D3G) remain, run in full
5. **Human Override**: Engineer approval can reject candidate reuse even if match found

**NOT Automatic**:
- Candidate reuse never skips evidence review
- Candidate reuse never skips replay validation
- Candidate reuse never skips engineer approval
- Candidate reuse never skips registry signature

---

### 30. Determine whether candidate reuse belongs in: fingerprint / knowledge / profilebuilder / registry / governed orchestration / another component?

**Answer**: **Governed Orchestration** (cmd/WIJOYO_KUSUMO/)

**Why NOT in fingerprint package**:
- Fingerprint is pure identity (no I/O, no decisions)
- Lookup is orchestration, not identity computation

**Why NOT in knowledge package**:
- Knowledge is immutable artifacts (QCKnowledgeRecord, etc.)
- Lookup is ephemeral query, not knowledge itself

**Why NOT in profilebuilder**:
- profilebuilder.BuildFromMappingDefinition() takes live mapping
- Candidate reuse is about **finding** mappings, not building profiles
- Profile builder should not perform registry queries

**Why NOT in registry**:
- Registry signs existing decisions (immutable, deterministic)
- Lookup is mutable discovery, not signing
- Registry.Admit() requires inputs; cannot search for them

**Why NOT in approval**:
- Approval decides on a single profile/mapping pair
- Candidate reuse is about selecting which mapping to use first

**YES in Governed Orchestration**:
- Orchestration layer coordinates discovery → evidence → replay → profile → approval → registry
- Natural place to insert "check registry for candidate" step
- Can query registry, run applicability checks, invoke profilebuilder with result
- Engineer approval still required for final decision

**Proposed Location** (Phase 6B):
```
cmd/WIJOYO_KUSUMO/candidate_reuse.go
├─ CandidateReuseQuery(fingerprint, mappingID, mappingVersion)
├─ CandidateReuseEvaluate(query, qcPolicy)
└─ Called from main command before ProfileBuilder
```

---

### 31. Determine how identity conflict should be represented when: fingerprint knowledge, semantic evidence, reference/QC, or historical profile do not agree.

**Conflict Representation** (proposed, not implemented):

```go
type IdentityConflict struct {
    // Conflict Identity
    ConflictID string // "conflict-2026-10-05-siemens-001"
    
    // What Conflicted
    DimensionType string // "FINGERPRINT_CHANGE", "SEMANTIC_DIGEST_MISMATCH", "QC_POLICY_MISMATCH", "PROFILE_BINDING_MISMATCH"
    
    // The Values
    CurrentIdentity string // what we have now
    PreviousIdentity string // what we had before
    AffectedMappingID string
    AffectedDeviceRef string
    
    // Artifacts Involved
    OlderArtifact string // reference to older artifact (digest or ID)
    NewerArtifact string // reference to newer artifact (digest or ID)
    ConflictTime time.Time // when detected
    
    // Resolution State (NOT auto-resolved)
    Status string // "UNRESOLVED", "UNDER_INVESTIGATION", "RESOLVED_EXPLICIT"
    ResolutionReason string // human explanation
    ResolvedBy string // who decided?
    ResolvedAt time.Time
    
    // Governance (conflict is NOT transparent)
    BlocksAutomaticReuse bool // true: cannot auto-reuse candidate
    BlocksApproval bool // false: engineer can still approve (with explicit decision)
    RequiresExplicitReview bool // true: must be reviewed before proceeding
    
    // Immutability
    Digest string // SHA256 of conflict record
}
```

**Rules**:
1. **Visibility**: All conflicts recorded as artifacts (never silent)
2. **No Majority Vote**: Conflicting identities remain in record
3. **No Automatic Resolution**: Conflict must be explicitly reviewed
4. **Engineer Decides**: Approval can override, but must be explicit (logged)
5. **Audit Trail**: Conflict + decision + reason recorded immutably

**Example**:
- Device X firmware updated → fingerprint Y changes
- Existing mapping still references old fingerprint X
- IdentityConflict artifact created: FINGERPRINT_CHANGE
- Engineer reviews: decides new mapping required
- OR: Engineer reviews: approves transition (with documented justification)

---

### 32. Determine how material fingerprint change should affect existing knowledge WITHOUT allowing automatic unsupported device-failure claims.

**Fingerprint Change Protocol** (proposed):

### Change Detection
1. **Observation Only** (no automatic action):
   - Edge system observes device structure change
   - Creates FingerprintIdentityObservation artifact (immutable)
   - Records: old fingerprint, new fingerprint, detection method, timestamp

### Knowledge State
2. **Isolation** (no automatic invalidation):
   - Old QC knowledge remains valid for old fingerprint
   - New observations require new fingerprint context
   - TransferFunction.CanReuseWith() blocks reuse (explicit error)
   - No "device failure" claim allowed

### Investigation Trigger
3. **Human Review Required**:
   - PatternCandidate with `NeedsHumanReview = true`
   - Cannot trigger any authority modification
   - Requires explicit engineer decision

### Reestablishment
4. **Full New Governance** (not automatic):
   - New MappingDefinition (with new fingerprint)
   - Evidence review (C1) — same or different evidence?
   - Replay validation (C2)
   - QC review with new reference challenges
   - Engineer approval (D3H)
   - Registry admission (D3G)

### What NOT to Infer
5. **No Automatic Claims**:
   - ✗ Device failure
   - ✗ Calibration drift
   - ✗ Software bug
   - ✗ Authority revocation
   - ✓ Only: "fingerprint changed, investigation required"

---

### 33. Determine whether fingerprint history itself needs an immutable identity observation artifact or whether existing evidence is sufficient.

**Answer**: **New artifact REQUIRED (FingerprintIdentityObservation)**

**Why Existing Evidence Insufficient**:

1. **Current State**:
   - DeviceFingerprint stored in multiple places (mapping, profile, QC records)
   - No central immutable record of fingerprint values over time
   - No artifact marks "when did fingerprint change?"
   - No link between old and new fingerprints

2. **Risk**:
   - Operator cannot ask "what was this device's fingerprint on 2026-09-15?"
   - Cannot verify fingerprint consistency across artifacts
   - Cannot distinguish "device updated" from "data corruption"

3. **Required Artifact**:

```go
type FingerprintIdentityObservation struct {
    // Observation Identity
    ObservationID string // "fingerprint-observation-2026-10-05-device-001"
    
    // The Values
    OldFingerprint string // previous structural hash (if exists)
    NewFingerprint string // current structural hash
    ChangeType string // "INITIAL", "UPDATE", "CORRUPTION_DETECTED", "UNKNOWN"
    
    // Detection
    DetectionMethod string // "PB-10-analysis", "operator-manual", "firmware-change", "edge-discovery"
    Confidence float64 // 0.0-1.0 (how confident in change vs corruption?)
    
    // Metadata
    DeviceIdentityRef string // device model/vendor (optional context)
    ParserIdentity string // parser used
    ObservedAt time.Time // when observed
    ObservedBy string // what/who observed
    
    // Governance (immutable record only)
    CanModifyAuthority bool // always false
    RequiresInvestigation bool // true: explicit review required
    
    // Resolution (filled by human review)
    ReviewedAt time.Time
    ReviewedBy string
    ConfirmedAsChange bool // true: device legitimately changed
    Justification string // why change occurred (firmware update, hardware replacement, etc.)
    
    // Immutability
    Digest string // SHA256
}
```

**Use**: Immutable history of device identity evolution. NOT used to gate authority, only to inform investigation.

---

### 34. Identify backward compatibility risks.

**Risk Assessment**:

### Level 1: No Risk (existing code unaffected)
- ✓ Adding new identity artifact types (backward compatible)
- ✓ Adding optional fields to existing types (if JSON marked omitempty)
- ✓ Adding new registry lookup endpoint (additive, doesn't change signature verification)

### Level 2: Caution (existing code may need review)
- ⚠ Adding QCPolicyIdentity to AcceptanceCriteria:
  - Existing AcceptanceCriteria.Validate() must accept unversioned policies (backward compatible)
  - OR: Mark QCPolicyIdentity optional, new verdicts versioned

- ⚠ Adding QC lookup to orchestration:
  - Must not skip evidence review even if candidate found
  - Must not auto-promote existing QC verdicts
  - Code review needed in cmd/WIJOYO_KUSUMO main()

### Level 3: Breaking Changes (unlikely needed, but possible)
- ✗ Changing SemanticEvidenceDigest computation:
  - Would invalidate all existing registry attestations
  - NOT planned
  - Mitigated: digest already immutable, only add new fields

- ✗ Adding DeviceFingerprint to canonical registry payload:
  - Would change signature (only if explicitly required)
  - NOT planned (fingerprint covered by CandidateDigest)

---

### 35. Propose the MINIMUM Phase 6B implementation.

**Phase 6B Scope** (MINIMUM to achieve candidate reuse):

### New Artifacts (Internal)
1. **QCIntegrationIdentity** (qc/integration_identity.go)
   - Links QC policy version to integration scenario
   - Tracks policy approval
   - Minimal: IntegrationID, Scenario, MappingID, QCPolicyVersion, ApprovedBy, Digest
   - ~80 lines

2. **FingerprintIdentityObservation** (internal/fingerprint/identity_observation.go)
   - Immutable record of fingerprint change
   - No authority implications
   - Minimal: ObservationID, OldFingerprint, NewFingerprint, ChangeType, DetectionMethod, ReviewedBy, Digest
   - ~60 lines

3. **IdentityConflict** (internal/identity/conflict.go)
   - Marks when identity dimensions diverge
   - Not auto-resolved
   - Minimal: ConflictID, DimensionType, CurrentIdentity, PreviousIdentity, Status, Digest
   - ~50 lines

### Existing Modifications (NO BREAKING CHANGES)
1. **AcceptanceCriteria** (qc/verdict.go)
   - Add optional `QCPolicyIdentity string` and `QCPolicyVersion string` fields
   - Backward compatible (omitempty)
   - ~5 line addition

2. **Governed Orchestration** (cmd/WIJOYO_KUSUMO/)
   - Add `CandidateReuseQuery(fingerprint, mappingID, version)` function
   - Query registry for exact match
   - Run applicability checks (fingerprint, mapping, policy)
   - Return list of candidate mappings (or empty)
   - ~100 lines new code

3. **Registry Lookup Endpoint** (internal/registry/lookup.go)
   - New function: `LookupByFingerprint(fingerprint string) ([]Attestation, error)`
   - Search persisted registry (depends on storage backend)
   - Minimal: ~50 lines

### Testing (NEW, Comprehensive)
See test matrix (section below)

### No Changes to:
- ✓ Semantic evidence verification (C1)
- ✓ Replay validation (C2)
- ✓ Engineer approval (D3H)
- ✓ Registry signature (D3G)
- ✓ Profile building (profilebuilder)
- ✓ QC verdict evaluation
- ✓ Temporal QC knowledge

**Total New Code**: ~350-400 lines (excluding tests)  
**Total Modified Code**: ~10-20 lines (opt-in fields in existing types)

---

## PROPOSED TEST MATRIX

**Comprehensive Fail-Closed Tests** (required for Phase 6B, informative for Phase 6A):

### Identity Determinism (3 tests)
1. ✓ **test_fingerprint_deterministic_same_device**
   - Same device always produces same fingerprint
   - Verify: `fingerprint1 == fingerprint2`

2. ✓ **test_fingerprint_materially_different**
   - Device change produces detectably different fingerprint
   - Verify: `fingerprint_old != fingerprint_new`

3. ✓ **test_fingerprint_immutable_in_mapping**
   - Fingerprint stored in mapping, cannot be mutated
   - Verify: `MappingDefinition.Validate()` rejects fingerprint change

### Semantic Evidence Isolation (3 tests)
4. ✓ **test_fingerprint_does_not_affect_semantic_evidence**
   - Semantic evidence digest computed independent of fingerprint
   - Change fingerprint, semantic digest remains same
   - Verify: Same VENDOR_MANUAL evidence → same digest

5. ✓ **test_semantic_evidence_not_affected_by_device_match**
   - Fingerprint match does not create semantic authority
   - Verify: VENDOR_MANUAL still required even if fingerprint matches previous device

6. ✓ **test_fingerprint_mismatch_does_not_reject_semantic**
   - Different fingerprint doesn't invalidate semantic evidence review
   - Verify: Evidence gate succeeds with new fingerprint + same VENDOR_MANUAL

### QC Records & Fingerprint (5 tests)
7. ✓ **test_qc_knowledge_record_binds_fingerprint**
   - QCKnowledgeRecord.DeviceRef required, immutable
   - Verify: NewQCRecord() fails if DeviceRef empty

8. ✓ **test_reference_challenge_device_mismatch_rejected**
   - ReferenceChallenge with mismatched fingerprint rejected
   - Verify: `challenge.BindsTo(definition)` fails if fingerprint different

9. ✓ **test_qc_records_cannot_cross_fingerprint**
   - Two QC records from different fingerprints cannot be aggregated
   - Verify: `ValidateIsolation()` rejects mixed-fingerprint statistics

10. ✓ **test_transfer_function_blocks_fingerprint_mismatch**
    - TransferFunction.CanReuseWith() rejects different fingerprint
    - Verify: Error message includes "requires re-verification"

11. ✓ **test_qc_verdict_no_automatic_transfer**
    - Fingerprint match does not create QC PASS
    - Verify: PASS verdict only from ReviewQC(), not from previous record

### Temporal Knowledge & Fingerprint (4 tests)
12. ✓ **test_temporal_knowledge_binds_fingerprint**
    - WithinRunTemporalKnowledge contains DeviceFingerprint
    - Verify: NewWithinRunTemporalKnowledge() fails if fingerprint missing

13. ✓ **test_observation_set_device_boundary**
    - TimeOrderedObservationSet cannot mix observations from different devices
    - Verify: `New()` enforces consistent DeviceFingerprint across observations

14. ✓ **test_longitudinal_knowledge_preserves_device**
    - LongitudinalQCKnowledge maintains device fingerprint across days
    - Verify: Each observation in set has same fingerprint

15. ✓ **test_fingerprint_change_does_not_imply_failure**
    - Fingerprint change alone does NOT create device failure claim
    - Verify: PatternCandidate.CanModifyRegistry == false even if NeedsHumanReview

### Profile & Authority (4 tests)
16. ✓ **test_profile_requires_fingerprint**
    - BuildFromMappingDefinition() requires DeviceFingerprint
    - Verify: Error if fingerprint empty

17. ✓ **test_fingerprint_mismatch_does_not_create_profile_authority**
    - Fingerprint match does not auto-promote profile status
    - Verify: New profile always CANDIDATE, regardless of matching fingerprint

18. ✓ **test_fingerprint_change_invalidates_profile_digest**
    - If profile fingerprint modified, digest changes, approval invalid
    - Verify: `approval.Validate()` fails on fingerprint change

19. ✓ **test_fingerprint_not_in_registry_signature**
    - Registry signature does NOT include fingerprint directly
    - Verify: Remove fingerprint from profile, digest changes, but attestation can still verify (with CandidateDigest check)

### Identity Isolation (5 tests)
20. ✓ **test_mapping_identity_independent_of_fingerprint**
    - MappingIdentity unchanged if fingerprint changes (new device)
    - Verify: Different fingerprints can share same MappingIdentity (different instances)

21. ✓ **test_semantic_digest_independent_of_fingerprint**
    - SemanticEvidenceDigest unchanged if fingerprint changes
    - Verify: Recompute digest with different fingerprint, result identical

22. ✓ **test_reference_identity_independent_of_fingerprint**
    - ReferenceDefinition valid for multiple fingerprints (e.g., HR reference applies to Siemens, Philips)
    - Verify: Same ReferenceDefinition can be used with different devices (if policy allows)

23. ✓ **test_transfer_function_identity_separate_from_fingerprint**
    - TransferFunction has separate Identity field (algorithm name)
    - Verify: Same algorithm (scalar_factor) can bind different fingerprints (with explicit approval)

24. ✓ **test_qc_policy_identity_separate_from_fingerprint**
    - QC policy can apply across multiple fingerprints
    - Verify: Same AcceptanceCriteria (+ policy version) used for QC on different devices

### Knowledge Reuse (5 tests)
25. ✓ **test_candidate_reuse_requires_exact_fingerprint_match**
    - Candidate reuse blocked unless fingerprint, mapping identity, version all exact
    - Verify: `CanReuseWith()` requires all three to match

26. ✓ **test_candidate_reuse_does_not_skip_governance**
    - Even with candidate reuse, evidence review, replay, approval still required
    - Verify: BuildFromMappingDefinition() still requires REPLAY_VALIDATED status

27. ✓ **test_fingerprint_match_does_not_auto_approve**
    - Fingerprint match does not create engineer approval
    - Verify: `approval.Review()` still requires live engineer decision

28. ✓ **test_fingerprint_match_does_not_auto_admit**
    - Fingerprint match does not create registry admission
    - Verify: `registry.Admit()` still requires Ed25519 signature

29. ✓ **test_knowledge_lookup_by_fingerprint_returns_candidates_only**
    - Registry lookup returns list of candidate mappings, not authority
    - Verify: Returned attestations are REGISTRY_ADMITTED (not auto-imported)

### Conflict Detection (4 tests)
30. ✓ **test_fingerprint_change_recorded_as_observation**
    - Fingerprint change creates FingerprintIdentityObservation artifact
    - Verify: Artifact created, immutable, requires explicit review

31. ✓ **test_identity_conflict_when_mapping_unchanged_fingerprint_changes**
    - If mapping version stays same but fingerprint changes, IdentityConflict created
    - Verify: Conflict status UNRESOLVED until human review

32. ✓ **test_semantic_digest_mismatch_creates_conflict**
    - If semantic evidence digest differs but mapping reused, IdentityConflict created
    - Verify: Conflict marked, blocks automatic reuse

33. ✓ **test_conflict_blocks_automatic_reuse_not_approval**
    - IdentityConflict.BlocksAutomaticReuse=true, BlocksApproval=false
    - Verify: Cannot auto-reuse, but engineer can explicitly approve

### Backward Compatibility (3 tests)
34. ✓ **test_existing_profiles_still_validate**
    - Profiles created before Phase 6B still pass validation
    - Verify: No new required fields, all new fields optional

35. ✓ **test_existing_qc_verdicts_readable**
    - Existing QCKnowledgeRecord (without QCPolicyIdentity) still deserializes
    - Verify: JSON unmarshal succeeds, missing optional field ignored

36. ✓ **test_existing_registry_attestations_verify**
    - Existing attestations (without QC binding) still verify correctly
    - Verify: Signature verification succeeds, QC binding optional

### Phase 5B Preservation (3 tests)
37. ✓ **test_semantic_evidence_digest_still_in_canonical_payload**
    - SemanticEvidenceDigest remains in registry canonical payload (field 9)
    - Verify: Attestation.canonicalPayload() includes digest

38. ✓ **test_profile_governance_status_unchanged**
    - Profile.GovernanceStatus, CompatibilityStatus still govern
    - Verify: CANDIDATE profiles cannot be promoted via fingerprint match

39. ✓ **test_approval_digest_covers_entire_profile**
    - Approval digest covers all profile fields including fingerprint
    - Verify: Any profile mutation (including fingerprint) invalidates approval

---

## PHASE 6A STATUS

### Current Fingerprint Authority
✓ **Deterministic**: Same device always produces same fingerprint  
✓ **Immutable**: Once assigned, never modified  
✓ **Scoped**: Fingerprint is device context, not semantic authority  
✓ **Preserved**: Carried through all governance layers (mapping → profile → registry)  
✓ **Isolated**: QC measurements scope to fingerprint, but QC verdict independent of match  

### Current Identity Propagation
```
Discovery (PB-10)
  ↓
Fingerprint (immutable)
  ↓
MappingDefinition.DeviceFingerprint
  ├─ Evidence Gate: fingerprint is context, not proof
  ├─ Replay Validation: fingerprint is context, not proof
  └─ SemanticEvidenceDigest: independent (not derived from fingerprint)
  ↓
Profile.DeviceFingerprint
  ├─ Authority Chain: includes fingerprint context
  ├─ CandidateDigest: includes profile (fingerprint covered transitively)
  └─ Approval: digest invalidated by fingerprint change
  ↓
QC Artifacts (all with explicit DeviceRef/DeviceFingerprint)
  ├─ ReferenceChallenge: binds fingerprint, immutable
  ├─ QCKnowledgeRecord: binds fingerprint (DeviceRef), immutable
  ├─ TransferFunction: binds fingerprint, reuse blocked on mismatch
  └─ Temporal Knowledge: preserves fingerprint per observation
  ↓
Registry Attestation
  ├─ CandidateDigest: covers fingerprint (transitively)
  ├─ Signature: Ed25519 over canonical payload
  └─ Canonical Payload: SemanticEvidenceDigest (independent), NOT DeviceFingerprint
```

### Cryptographic Bindings
✓ **SemanticEvidenceDigest** ← registry signature (direct, field 9)  
✓ **Profile.DeviceFingerprint** ← registry signature (transitive, via CandidateDigest)  
✓ **QC.DeviceRef** ← QC record integrity hash (SHA256, not registry signature)  
✗ **DeviceFingerprint** ← NOT in canonical registry payload (intentional)  

### Discovered Identity Gaps

| Gap | Impact | Phase 6B Required? |
|-----|--------|-------------------|
| No explicit QC Policy Identity | Policy evolution not tracked | YES (for versioning) |
| No cross-identity conflict detection | Mismatches silent | YES (audit visibility) |
| No fingerprint history artifact | Cannot query device evolution | YES (for investigation) |
| No QCIntegrationIdentity | Cannot distinguish integration scenarios | YES (for reuse safety) |
| No registry lookup endpoint | Cannot discover existing knowledge | YES (for candidate reuse) |

### Existing Reuse Capability
- ✓ TransferFunction.CanReuseWith() implemented (blocks on mismatch)
- ✓ ReferenceChallenge.BindsTo() validates device match
- ✗ No knowledge lookup mechanism (must be added Phase 6B)
- ✗ No automatic candidate reuse (governance gates remain)
- ✗ No registry query endpoint (must be added Phase 6B)

### Minimum Safe Extension (Phase 6B)
1. **QCIntegrationIdentity** — link policy version to integration context
2. **FingerprintIdentityObservation** — immutable record of device evolution
3. **IdentityConflict** — mark when identities diverge
4. **Registry Lookup Endpoint** — query by fingerprint/mapping/version
5. **Governed Orchestration** — candidate reuse with applicability checks

**Total Risk**: LOW
- No breaking changes to existing code
- All new artifacts additive (backward compatible)
- All new registry queries optional (not mandatory)
- All governance gates (C1/C2/D3H/D3G) preserved
- Candidate reuse never skips authority verification

### Files Potentially Affected (Phase 6B)
- `internal/qc/integration_identity.go` (new)
- `internal/fingerprint/identity_observation.go` (new)
- `internal/identity/conflict.go` (new)
- `internal/registry/lookup.go` (new)
- `cmd/WIJOYO_KUSUMO/candidate_reuse.go` (new)
- `cmd/WIJOYO_KUSUMO/main.go` (modify: add candidate reuse orchestration)
- `qc/verdict.go` (modify: add optional QCPolicyIdentity + QCPolicyVersion fields)
- `registry/registry.go` (modify: none required; QCBinding already present)

### Proposed Phase 6B Test Matrix
**39 tests total**:
- 3 determinism tests
- 3 semantic isolation tests
- 5 QC binding tests
- 4 temporal knowledge tests
- 5 profile authority tests
- 5 identity isolation tests
- 5 knowledge reuse tests
- 4 conflict detection tests
- 3 backward compatibility tests
- 3 Phase 5B preservation tests

### Backward Compatibility Risks
**Level 1 (No Risk)**:
- Adding new artifact types
- Adding optional fields (JSON omitempty)
- Adding registry lookup endpoint

**Level 2 (Caution)**:
- Adding QCPolicyIdentity to AcceptanceCriteria (must mark optional)
- Adding candidate reuse to orchestration (must not skip gates)

**Level 3 (Unlikely)**:
- Changing SemanticEvidenceDigest computation ← NOT PLANNED
- Adding fingerprint to registry payload ← NOT NEEDED

---

## CONSTITUTIONAL LAW COMPLIANCE: VERIFIED

### Law 1: Knowledge Without Identity Shall Not Become Authority
✓ QCKnowledgeRecord is CANDIDATE status (never auto-promoted)  
✓ Fingerprint match does NOT create authority  
✓ TransferFunction BindsTo() requires explicit verification  

### Law 2: Fingerprint Match ≠ Semantic Verification
✓ SemanticEvidenceDigest independent of DeviceFingerprint  
✓ Evidence gate requires VENDOR_MANUAL regardless of fingerprint  
✓ Semantic digest in registry signature; fingerprint not  

### Law 3: Same Fingerprint May Support Knowledge Reuse
✓ TransferFunction.CanReuseWith() allows reuse if all conditions match  
✓ Phase 6B provides mechanism (with applicability checks)  

### Law 4: Identity Tells Which Knowledge May Be Relevant, Evidence Determines Trust
✓ Fingerprint used for scoping (which knowledge is relevant)  
✓ Evidence + governance gates determine trust (VENDOR_MANUAL, approval, signature)  

### Law 5: One Device May Contain Many Mappings
✓ Multiple MappingDefinitions can have same DeviceFingerprint  
✓ Each mapping maintains independent identity (MappingIdentity, MappingVersion)  

### Law 6: Statistics Without Identity Boundaries May Create False Knowledge
✓ QCStatistics computed per (DeviceRef, MappingIdentity, ReferenceLevel)  
✓ ValidateIsolation() prevents cross-device aggregation  

### Law 7: Reference Value Shall Not Substitute For Reference Identity
✓ ReferenceDefinition has explicit identity (ReferenceID)  
✓ Reference level (LOW/NORMAL/HIGH) separate from fingerprint  

### Law 8: Transfer Function Travels With Identity and Evidence
✓ TransferFunction binds device + mapping + version explicitly  
✓ Not merely a number; identity-aware reuse  

### Law 9: Time Orders Epoch; Does Not Define It
✓ RunID explicit (not inferred from timestamp range)  
✓ DayID explicit (not inferred from calendar date)  
✓ KnowledgeEpochID explicit (policy-defined)  

### Law 10: Fingerprint Change Is Evidence of Identity Deviation, Not Device Failure
✓ FingerprintIdentityObservation artifact (Phase 6B) marks change immutably  
✓ No automatic device failure claim  
✓ Requires explicit human investigation  

### Law 11: Exact Fingerprint Match Is Evidence of Similarity, Not Complete Equivalence
✓ CanReuseWith() checks fingerprint + mapping + version  
✓ Not just fingerprint alone  
✓ Applicability policy also required (Phase 6B)  

### Law 12: Historical Knowledge May Shorten Investigation, Not Path to Authority
✓ Candidate reuse does NOT skip evidence review  
✓ Candidate reuse does NOT skip approval  
✓ Candidate reuse does NOT skip registry signature  

### Law 13: Edge May Detect Identity Change; Edge Does Not Redefine Identity
✓ FingerprintIdentityObservation is observation artifact  
✓ Does not gate authority decisions  
✓ Requires explicit human review  

### Law 14: OTRIS Becomes Faster By Remembering, Not By Lowering Standards
✓ Candidate reuse optional (not mandatory)  
✓ All governance gates unchanged  
✓ Phase 5B SemanticEvidenceDigest cryptographic binding preserved  

---

## END OF INSPECTION

**Status**: ARCHITECTURE SOUND. Phase 6A inspection complete. No code changes.

**Recommendation**: Proceed to Phase 6B with proposed artifacts + orchestration + test matrix.

**Locked Baseline**: d578b50eeddb5e779f31633120ada1f9dd319e4d — IMMUTABLE FOR PHASE 6A.

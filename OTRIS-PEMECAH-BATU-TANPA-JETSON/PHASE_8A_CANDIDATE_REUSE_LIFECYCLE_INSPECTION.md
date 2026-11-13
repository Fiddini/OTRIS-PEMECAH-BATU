# PHASE 8A — CANDIDATE REUSE LIFECYCLE INSPECTION

**Baseline**: Phase 7 commit 1e7365b9e6d2e15f7d02b2db6d2759c38552a368

**Task**: Trace CandidateKnowledgeMatch lifecycle from lookup through new authority. Inspection only.

---

## FINDINGS

### 1. WHERE CANDIDATEKNOWLEDGEMATCH ENTERS GOVERNED WORKFLOW

**Current Entry Point**: DOES NOT EXIST

**Current Workflow Entry** (explicit MappingDefinition required):
```
cmd/WIJOYO_KUSUMO/main.go (governed-pipeline command)
  → MappingDefinition (CANDIDATE status) supplied explicitly
  → mapping.ReviewEvidence() (C1: semantic evidence gate)
  → mapping.ReviewReplay() (C2: replay validation gate)
  → profilebuilder.BuildFromMappingDefinition() (builds profile candidate)
  → ReviewQCGovernance() (optional QC review)
  → approval.Review() (D3H: engineer approval)
  → registry.Admit() (D3G: registry admission)
  → new current authority (REGISTRY_ADMITTED)
```

**Missing Link**:
- Registry lookup returns `CandidateKnowledgeMatch`
- CandidateKnowledgeMatch contains `SourceAttestation` (historical knowledge reference)
- No path exists to convert historical Attestation → live MappingDefinition for governance

**Evidence**: 
- `cmd/WIJOYO_KUSUMO/main.go`: governed-pipeline uses explicit MappingDefinition file only
- `internal/registry/lookup.go`: returns CandidateKnowledgeMatch but has no downstream connection
- No function accepts CandidateKnowledgeMatch as input

---

### 2. WHETHER APPLICABILITY-VERIFICATION FUNCTION/PATH ALREADY EXISTS

**Answer**: NO explicit path exists.

**What Exists**:
- `ApplicabilityStatus` enum (identity/candidate_match.go) with states:
  - NOT_YET_ESTABLISHED (lookup result)
  - CHECKING (in progress)
  - CONFIRMED (governance confirmed)
  - REJECTED (governance rejected)
- `ValidateLookupResult()` (registry/lookup.go) enforces applicability = NOT_YET_ESTABLISHED

**What's Missing**:
- No function accepts CandidateKnowledgeMatch
- No function extracts historical Attestation and verifies applicability
- No decision logic for "is this candidate applicable in current context?"

**Where It Should Be**:
- Bridge between `LookupByIntegrationIdentity()` result and existing governance gates

---

### 3. WHICH EXISTING GATES CAN BE REUSED UNCHANGED

**All Current Gates Reusable**:

| Gate | Function | Reusable? | Entry Point |
|------|----------|-----------|------------|
| **C1: Semantic Evidence** | `mapping.ReviewEvidence()` | ✓ YES (unchanged) | MappingDefinition required |
| **C2: Replay Validation** | `mapping.ReviewReplay()` | ✓ YES (unchanged) | MappingDefinition required |
| **QC Review** | `qc.ReviewQC()` | ✓ YES (unchanged) | Optional, QC policy specified |
| **D3H: Engineer Approval** | `approval.Review()` | ✓ YES (unchanged) | Profile + live Mapping + approval input |
| **D3G: Registry Admission** | `registry.Admit()` | ✓ YES (unchanged) | Profile + Mapping + Approval + keys |

**Critical Requirement**: All gates require **LIVE MappingDefinition**, not historical Attestation

---

### 4. THE EXACT MISSING CONNECTION, IF ANY

**The Gap**: CandidateKnowledgeMatch → Live MappingDefinition

**Current Attestation Has**:
```
type Attestation struct {
  MappingIdentity        string  // "hr-obx-field-5"
  MappingVersion         string  // "1.0.0"
  SemanticEvidenceDigest string  // "abc123..."
  EvidenceReference      string  // path to evidence report
  ReplayValidatedAt      time.Time
  MappingProvenance      string
  ProvenanceAuthor       string
  // ... other fields
}
```

**What Governance Gates Need**:
```
type MappingDefinition struct {
  MappingIdentity        string
  MappingVersion         string
  SourceProtocol         string
  SourcePosition         int
  SourceCode             string
  SourceUnit             string
  CanonicalConcept       string
  CanonicalUnit          string
  DeviceFingerprint      string  // ← NOT in Attestation
  ParserIdentity         string  // ← NOT in Attestation
  EvidenceGateReportRef  string  // ← Attestation has "EvidenceReference"
  EvidenceRefs           []Evidence // ← NOT in Attestation
  SemanticEvidenceDigest string
  Provenance             string
  ProvenanceAuthor       string
  ReplayValidatedAt      time.Time
  // ... status fields, timestamps, etc.
}
```

**The Missing Piece**:
- CandidateKnowledgeMatch contains AttestationReference (historical metadata only)
- Attestation does NOT contain full MappingDefinition fields (SourceCode, SourceUnit, etc.)
- **Cannot reconstruct live MappingDefinition from historical Attestation alone**

**Why**: By design. Attestation is immutable, signed record. MappingDefinition must come from live evidence gates (C1/C2).

---

### 5. PHASE 8 REQUIREMENT CLASSIFICATION

**Answer**: SMALL IMPLEMENTATION

**Not EXISTS** (Attestation doesn't have all fields)  
**Not COMPOSITION_ONLY** (can't reconstruct MappingDefinition from Attestation)  
**YES: SMALL IMPLEMENTATION** (bridge function required)

**Scope**:
- Extract identity from CandidateKnowledgeMatch.SourceAttestation
- Reconstruct minimal MappingDefinition with available fields
- **Or**: Query source evidence/replay records if stored separately
- **Or**: Require applicability check before running full C1/C2 gates

**Smallest Path**:
```go
// Pseudo-code: what needs to be built
func ApplicabilityCheckAndReconstructMapping(
  candidate *CandidateKnowledgeMatch,
  currentContext *CurrentDeviceContext, // device fingerprint, parser, etc.
) (*MappingDefinition, *ApplicabilityDecision, error) {
  // 1. Verify historical identity matches current context
  // 2. If DeviceFingerprint changed: REJECTED
  // 3. If MappingIdentity/Version matches current candidate: CONFIRMED
  // 4. Reconstruct MappingDefinition for governance
  // 5. Return applicability decision
}
```

---

### 6. EXACT FILES/FUNCTIONS/TESTS SUPPORTING CONCLUSION

**Current Lookup Implementation**:
- `internal/registry/lookup.go` (230 lines)
  - `LookupByIntegrationIdentity()`: returns CandidateKnowledgeMatch
  - No downstream consumer defined
  - Tests verify lookup works but don't test downstream path

**Current Governance Workflow**:
- `cmd/WIJOYO_KUSUMO/main.go` (governed-pipeline command)
  - Line ~650+: calls `mapping.ReviewEvidence(m, ...)`
  - Line ~680+: calls `mapping.ReviewReplay(m, ...)`
  - Line ~700+: calls `profilebuilder.BuildFromMappingDefinition(m, ...)`
  - Requires live MappingDefinition (from file)

- `cmd/WIJOYO_KUSUMO/governed_handoff.go`
  - `runGovernedAdmission()` (line ~80): entry to approval + admission
  - Requires: Profile + live Mapping + Approval + keys

**Attestation Structure** (identity/candidate_match.go):
- `type AttestationReference struct` (lines 68-76)
  - Has: AdmissionStatus, ProfileID, MappingIdentity, MappingVersion, SemanticEvidenceDigest
  - Missing: DeviceFingerprint, SourceCode, SourceUnit, ParserIdentity, Evidence refs

**MappingDefinition Structure** (internal/mapping/definition.go):
- Lines 23-58: Full structure including SourceProtocol, SourcePosition, etc.
- Governance gates (C1/C2) depend on these fields

**Tests Showing No Downstream Path**:
- `internal/registry/lookup_test.go`: Tests lookup in isolation
- No test integrates lookup result with governance gates
- No test shows CandidateKnowledgeMatch → applicability check → profile building

---

## CONCLUSION

**Phase 8 Requirement**: SMALL IMPLEMENTATION

**The Lifecycle Path Exists** (C1 → C2 → D3H → D3G):
- ✓ ReviewEvidence (C1) ready
- ✓ ReviewReplay (C2) ready
- ✓ Engineer Approval (D3H) ready
- ✓ Registry Admission (D3G) ready

**The Missing Connection**:
- ✗ CandidateKnowledgeMatch → applicability verification
- ✗ Historical Attestation → live MappingDefinition reconstruction
- ✗ Identity conflict detection (fingerprint change, mapping mismatch)
- ✗ Decision: accept candidate or reject as inapplicable

**Size**: ~150-200 lines (small function + tests)
- Extract identity from CandidateKnowledgeMatch
- Verify applicability in current context (fingerprint, mapping identity)
- Return ApplicabilityDecision (CONFIRMED / REJECTED / NEEDS_EVIDENCE)
- Orchestration calls full governance gates only if applicability confirmed

**Authority Preservation**:
- ✓ Historical authority NOT transferred (ApplicabilityStatus immutable until verified)
- ✓ Current applicability verified by existing gates (C1/C2/D3H/D3G)
- ✓ Semantic digest binding unchanged
- ✓ Registry signature unchanged

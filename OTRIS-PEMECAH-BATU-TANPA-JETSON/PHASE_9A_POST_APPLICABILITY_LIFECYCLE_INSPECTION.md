# PHASE 9A — POST-APPLICABILITY LIFECYCLE GAP INSPECTION

**Baseline**: d0c40053d8a4dca4c84eb74b371a1653cb14e4ad

**Task**: Trace what happens AFTER Applicability = CONFIRMED. Determine whether current code connects confirmed candidate into governance gates.

---

## FINDINGS

### 1. IS CONFIRMED APPLICABILITY ACTUALLY CONSUMED BY THE GOVERNED PIPELINE?

**Answer**: NO

**Current Status**:
- ApplicabilityVerification type exists (internal/registry/applicability.go)
- ApplicabilityVerification.IsCandidateApplicable() method returns true/false
- But NO code in cmd/ calls ApplicabilityVerification
- Governed pipeline never invokes lookup or applicability check

**Evidence**:
- `cmd/WIJOYO_KUSUMO/main.go` (lines 202-397): governed-pipeline command
  - Line 274-290: Loads MappingDefinition from JSON file (explicit input only)
  - Line 293-304: ReviewEvidence(m, ...)
  - Line 307-323: ReviewReplay(m, ...)
  - Line 325-334: ReviewQCGovernance(m, ...)
  - Line 354-358: BuildFromMappingDefinition(m, ...)
  - Line 368: runGovernedAdmission(&profile, m, ...)
  - **NO lookup call. NO applicability check. NO candidate integration.**

- Grep search (complete): ApplicabilityVerification, LookupByIntegrationIdentity, LookupByDeviceAndMapping not called anywhere in cmd/ or orchestration

---

### 2. IS THERE ALREADY ORCHESTRATION CONNECTING IT TO EXISTING GATES?

**Answer**: NO

**What Exists**:
- ReviewEvidence: lines 293-304, takes MappingDefinition only
- ReviewReplay: lines 307-323, takes MappingDefinition only
- ReviewQCGovernance: lines 325-334, takes MappingDefinition only
- BuildFromMappingDefinition: line 355, takes MappingDefinition only
- runGovernedAdmission: line 368, takes Profile + MappingDefinition only

**What's Missing**:
- NO entry point in governed-pipeline that accepts CandidateKnowledgeMatch
- NO reconstruction of MappingDefinition from historical Attestation
- NO call to ApplicabilityVerification.Verify() before ReviewEvidence
- NO conditional logic: "if applicability CONFIRMED, proceed to ReviewEvidence"

**Current Data Flow**:
```
MappingDefinition (from JSON file)
→ ReviewEvidence
→ ReviewReplay
→ ReviewQCGovernance
→ BuildFromMappingDefinition
→ runGovernedAdmission
```

**Missing Data Flow**:
```
CandidateKnowledgeMatch (from lookup)
→ ApplicabilityVerification.Verify()
→ if CONFIRMED: proceed to ReviewEvidence
→ ...rest of gates...
```

---

### 3. CAN A CONFIRMED CANDIDATE CURRENTLY REACH NEW REGISTRY ADMISSION WITHOUT MANUAL RECONSTRUCTION?

**Answer**: NO

**Why**:
1. CandidateKnowledgeMatch contains historical Attestation reference (non-authoritative)
2. ReviewEvidence requires live MappingDefinition with SourceCode, SourceUnit, EvidenceRefs (NOT in Attestation)
3. No function reconstructs MappingDefinition from CandidateKnowledgeMatch
4. Governed-pipeline only accepts explicit MappingDefinition file (line 215: `mappingPath, _ := cmd.Flags().GetString("mapping")`)
5. Applicability verification confirms identity match, but does NOT build live MappingDefinition

**Sequence Required**:
```
1. Lookup returns CandidateKnowledgeMatch
2. ApplicabilityVerification.Verify() → CONFIRMED
3. (MISSING) Reconstruct or load live MappingDefinition matching candidate identity
4. Call ReviewEvidence(reconstructedMapping, ...)
5. Call ReviewReplay(reconstructedMapping, ...)
6. ... rest of gates
7. runGovernedAdmission → Registry admission
```

**Gap**: Step 3 does not exist.

---

### 4. WHAT EXACT CONNECTION IS MISSING, IF ANY?

**The Missing Bridge** (between applicability verification and ReviewEvidence):

**Input**: CandidateKnowledgeMatch (applicability = CONFIRMED)
**Output**: Live MappingDefinition ready for ReviewEvidence

**Currently Needed**:
1. Load or reconstruct MappingDefinition matching candidate identity
   - Cannot reconstruct from Attestation alone (missing fields)
   - Must come from evidence repository, file, or other source
2. Verify reconstructed mapping identity matches candidate identity
3. Pass to ReviewEvidence as normal governance flow

**Implementation Location**:
- New function or orchestration in cmd/WIJOYO_KUSUMO/main.go
- Or new package: internal/orchestration/ or internal/pipeline/
- Takes: CandidateKnowledgeMatch + ApplicabilityDecision + source mapping data
- Returns: MappingDefinition ready for ReviewEvidence
- Calls existing ReviewEvidence/ReviewReplay/BuildFromMappingDefinition unchanged

**Files Affected**:
- No modifications to ReviewEvidence, ReviewReplay, ReviewQC, BuildFromMappingDefinition, approval, registry
- New orchestration only (accepts candidate + loads/provides mapping)

---

### 5. CLASSIFY NEXT WORK

**Answer**: SMALL_IMPLEMENTATION

**Not EXISTS**: Applicability is consumed nowhere
**Not COMPOSITION_ONLY**: Requires orchestration to connect phases
**YES: SMALL_IMPLEMENTATION** (~100-150 lines)

**Scope**:
- New command or orchestration function in cmd/WIJOYO_KUSUMO/
- Inputs: CandidateKnowledgeMatch, ApplicabilityVerification result, mapping source path
- Loads matching MappingDefinition (from file or registry)
- Validates it matches candidate identity
- Enters existing governed pipeline (ReviewEvidence → ReviewReplay → gates)

**Does NOT require**:
- MappingDefinition changes
- ReviewEvidence/ReviewReplay/ReviewQC/Approval/Registry changes
- New governance gates
- New semantic authority

**Size**: ~150 lines (orchestration + minimal validation)
- Governed-candidate-pipeline command (similar to governed-pipeline)
- Takes: candidate metadata + mapping source
- Calls ApplicabilityVerification
- If CONFIRMED: enters existing pipeline unchanged
- If REJECTED/NEEDS_EVIDENCE: fails with clear reason

---

### 6. EXACT FILES/FUNCTIONS/TESTS

**Current Proven Chain**:
- `internal/registry/lookup.go` (283 lines)
  - `LookupByIntegrationIdentity(store, iki)` → CandidateKnowledgeMatch
  - `LookupByDeviceAndMapping(store, fp, id, ver)` → CandidateKnowledgeMatch
  - Tests: 15 tests in lookup_test.go (all PASS)

- `internal/registry/applicability.go` (325 lines)
  - `NewApplicabilityVerification(candidate, currentMapping, currentIdentity)` → ApplicabilityVerification
  - `Verify()` → decision (CONFIRMED/REJECTED/NEEDS_EVIDENCE)
  - `GetDecision()`, `IsCandidateApplicable()`, `IsVerified()`
  - Tests: 15 tests in applicability_test.go (all PASS)

- `internal/identity/candidate_match.go` (124 lines)
  - `CandidateKnowledgeMatch` struct
  - `NewMatch()` constructor
  - ApplicabilityStatus immutable = NOT_YET_ESTABLISHED

**Existing Governance Gates** (unchanged):
- `internal/mapping/evidence.go`: `ReviewEvidence(m, evidencePath, reportPath)` (requires live MappingDefinition)
- `internal/mapping/replay.go`: `ReviewReplay(m, inputDir, count, opts, reportPath)` (requires live MappingDefinition)
- `cmd/WIJOYO_KUSUMO/governed_qc.go`: `ReviewQCGovernance(m, ...)` (requires live MappingDefinition)
- `internal/profilebuilder/profilebuilder.go`: `BuildFromMappingDefinition(m, ...)` (requires live MappingDefinition)
- `internal/approval/approval.go`: `Review(profile, liveMapping, approver, decision)` (requires Profile + live MappingDefinition)
- `internal/registry/registry.go`: `Admit(profile, liveMapping, approval, qcRecord, ...)` (requires Profile + live MappingDefinition)

**Command Entry Point**:
- `cmd/WIJOYO_KUSUMO/main.go` (lines 202-397)
  - `governedPipelineCmd`: accepts explicit MappingDefinition file
  - **Missing**: governed-candidate-pipeline command (accepts CandidateKnowledgeMatch)

**Test Framework**:
- `internal/registry/applicability_test.go` (422 lines, 15 tests)
  - Proves applicability verification works
  - Proves mismatch detection works
  - Proves authority NOT transferred
  - Proves immutability preserved

---

## CONCLUSION

**Post-Applicability Lifecycle**:

Current proven chain (7 phases):
1. ✓ Registry Lookup (lookup.go: LookupByIntegrationIdentity)
2. ✓ CandidateKnowledgeMatch returned (non-authoritative)
3. ✓ Applicability Verification (applicability.go: Verify)
4. ✓ CONFIRMED / REJECTED / NEEDS_EVIDENCE decision
5. ✗ **Missing bridge**: Load live MappingDefinition matching candidate
6. ✓ ReviewEvidence (requires live MappingDefinition)
7. ✓ ReviewReplay → ReviewQC → Approval → Registry (unchanged)

**The Gap**: Between ApplicabilityVerification result and ReviewEvidence entry

**What's Missing**:
- Orchestration command or function
- Takes confirmed CandidateKnowledgeMatch
- Loads matching MappingDefinition (from evidence repository)
- Enters existing governed pipeline unchanged

**Size**: SMALL_IMPLEMENTATION (~150 lines)
- New command: governed-candidate-pipeline
- Reuses: lookup, applicability, ReviewEvidence, ReviewReplay, all existing gates

**Authority Preservation**:
- ✓ Applicability verified before gates
- ✓ Historical knowledge is candidate only (NOT_YET_ESTABLISHED immutable)
- ✓ Live evidence gates unchanged and required
- ✓ Existing governance path unchanged
- ✓ New authority created through same approval + registry admission

# PHASE 4A: Temporal QC Intelligence — Inspection Report

**Baseline**: Governed QC Pipeline Integration (dc2e0ee)  
**Inspection Date**: 2026-10-05  
**Status**: INSPECTION ONLY — NO IMPLEMENTATION

---

## A. What Temporal Information Is Already Stored Today?

**ReferenceChallenge** (internal/qc/challenge.go:16–38):
- ✅ `Timestamp time.Time` — when measurement was taken (line 31)
- ✅ Preserved exactly and included in SHA256 digest (line 34, 105)
- ✅ Validated as required field (line 70 in NewChallenge)
- ✅ Immutable: sealed into digest, cannot be modified post-creation

**QCStatistics** (internal/qc/qcstats.go:13–30):
- ✅ `Scope string` — "WITHIN_RUN", "WITHIN_DAY", "LONGITUDINAL" (line 15)
- ✅ `Level string` — "LOW", "NORMAL", "HIGH" or domain-specific (line 18)
- ❌ No explicit **observation order** (temporal sequence is lost after grouping)
- ❌ No **sequence number** field
- ❌ No **run identity** field (only scope string)
- ❌ No **day boundary marker**
- ❌ No **epoch/period identity** for longitudinal

**QCKnowledgeRecord** (internal/qc/qcresult.go:15–46):
- ✅ `CreatedAt time.Time` — when QC review was completed (line 37)
- ✅ `ChallengeCount int` — how many challenges measured (line 31)
- ✅ `ChallengesRef []string` — references to challenge records (line 32, currently unused)
- ❌ No **challenge digests or immutable references** to original ReferenceChallenge records
- ❌ No **timestamp of first/last challenge** in the set
- ❌ No **temporal ordering metadata**

**ReferenceDefinition** (internal/qc/reference.go):
- ✅ Identity + version (mapping-specific reference)
- ❌ No temporal binding (no "validated on date X" marker)

---

## B. Does ReferenceChallenge Currently Preserve Timestamp and/or Sequence?

**Answer: YES and NO**

- ✅ **Timestamp preserved**: Explicit `time.Time` field, required at creation, immutable in digest
- ✅ **Timestamp validated**: Non-zero required (line 70, NewChallenge), converted to UTC (line 82)
- ❌ **Sequence NOT preserved**: No `sequence` field in ReferenceChallenge struct
- ❌ **Order NOT tracked**: When multiple challenges are grouped by level in ReviewQC (line 56, review.go), temporal order is lost
- ⚠️ **Implication**: Within-run observations can be reordered without detection (same digest, statistics unchanged)

---

## C. Can Current QCStats Distinguish Multiple Observations by Temporal Order, or Only as a Statistical Population?

**Answer: STATISTICAL POPULATION ONLY**

**Current behavior** (review.go:55–73, qcstats.go:35–100):
1. Challenges are grouped by `RefLevel` only (line 56, review.go):
   ```go
   levelGroups := make(map[string][]float64)
   for _, ch := range challenges {
     levelGroups[ch.RefLevel] = append(levelGroups[ch.RefLevel], 0.0) // placeholder
   }
   ```

2. Temporal information is **discarded** before statistics calculation:
   - Timestamp exists in ReferenceChallenge but is never extracted
   - RawValue is never parsed to float64
   - Statistics computed on empty `[]float64{0.0, 0.0, ...}` (placeholder values)

3. QCStatistics fields computed:
   - `N` (count), `Mean`, `StdDev`, `CV`, `Min`, `Max`, `Median` — **population summaries only**
   - No `first_timestamp`, `last_timestamp`, `observation_sequence`
   - No `outlier_flags` or `temporal_variance`

4. **Result**: All observations at a given level are interchangeable; temporal order is lost.

---

## D. How Are WITHIN_RUN, WITHIN_DAY, and LONGITUDINAL Represented Today?

**Answer: STRUCTURALLY PRESENT, FUNCTIONALLY UNUSED**

**Current representation** (qcstats.go:14–15):
```go
Scope string // "WITHIN_RUN", "WITHIN_DAY", "LONGITUDINAL"
```

**Reality**:
- ✅ Scope enum is defined and validated (qcstats.go, verdict.go tests)
- ✅ ValidateIsolation() prevents scope mixing (qcstats.go:102–132)
- ❌ Scope is **hardcoded** to "WITHIN_RUN" at calculation time (review.go:68):
  ```go
  stats, err := Calculate("WITHIN_RUN", level, values, nil)
  ```
- ❌ No mechanism to **populate** WITHIN_DAY or LONGITUDINAL observations
- ❌ No **day boundary definition** (no DayID, no date marker in QCStatistics)
- ❌ No **run identity** field to link WITHIN_RUN to a specific run

**Implication**: Scope validation framework exists but the upstream data model (run/day/epoch identity) does not yet exist to populate it.

---

## E. What Information Is Currently Lost After Statistics Are Calculated?

**Critical losses**:

1. **All raw observations**:
   - Timestamp from each ReferenceChallenge (lost at line 62, review.go)
   - RawValue string (never parsed; treated as placeholder)
   - Operator, Provenance (lost)
   - DeviceFingerprint (lost)

2. **Observation order** (temporal sequence):
   - No sequence number preserved
   - No first/last timestamp recorded
   - No observation sequence in result

3. **Challenge identity**:
   - Individual challenge digests not stored in QCKnowledgeRecord
   - ChallengesRef field exists but unused (not populated)
   - Cannot trace statistics back to specific challenges

4. **Temporal metadata**:
   - Run identity (what run are these observations from?)
   - Day boundary (what calendar day?)
   - Longitudinal epoch (across how many runs/days?)
   - Observation sequence within run

5. **Outlier information**:
   - Min/Max are calculated but outlier status unclear
   - No flags marking observations as outliers vs. nominal

---

## F. What Is the Smallest Data Model Extension Needed to Preserve Time-Ordered Observations?

**Minimum extension** (non-breaking):

### 1. **ReferenceChallenge: Add Sequence** (optional, backward-compat)
```go
type ReferenceChallenge struct {
  // ... existing fields ...
  Sequence int64  `json:"sequence,omitempty"` // observation order within run (0 = no ordering)
  RunID    string `json:"run_id,omitempty"`   // run identity (from governor)
}
```
- Does NOT require validation (optional field)
- Does NOT break existing digests (added to new records only)
- Allows temporal ordering without changing existing challenge format

### 2. **QCStatistics: Add Temporal Bounds** (backward-compat)
```go
type QCStatistics struct {
  // ... existing fields ...
  
  // Temporal metadata (optional)
  ObservationCount    int       `json:"observation_count,omitempty"`    // != N (after outlier filter)
  FirstObservationAt  time.Time `json:"first_observation_at,omitempty"`
  LastObservationAt   time.Time `json:"last_observation_at,omitempty"`
  
  // Challenge references (optional)
  ChallengeDigests    []string  `json:"challenge_digests,omitempty"`    // SHA256 of each input challenge
  ChallengeSequences  []int64   `json:"challenge_sequences,omitempty"`  // temporal order within run
  
  // Outlier metadata (optional)
  OutlierFlags        []bool    `json:"outlier_flags,omitempty"`        // per-observation outlier status
  OutlierCount        int       `json:"outlier_count,omitempty"`        // how many excluded
}
```
- Does NOT change calculation logic
- Does NOT break existing QCVerdict evaluation (ignores new fields)
- Allows drill-down to original observations

### 3. **QCKnowledgeRecord: Add Temporal Epoch** (backward-compat)
```go
type QCKnowledgeRecord struct {
  // ... existing fields ...
  
  // Temporal binding (optional)
  RunID              string     `json:"run_id,omitempty"`              // run identity
  DayBoundary        string     `json:"day_boundary,omitempty"`        // "2026-10-05" (YYYY-MM-DD)
  LongitudinalEpoch  string     `json:"longitudinal_epoch,omitempty"`  // "2026-W40" or policy-defined
  FirstChallengeAt   time.Time  `json:"first_challenge_at,omitempty"`
  LastChallengeAt    time.Time  `json:"last_challenge_at,omitempty"`
}
```
- Does NOT change ReviewQC() logic
- Does NOT change acceptance criteria evaluation
- Allows QC record to be linked to a specific run/day/epoch
- Enables temporal aggregation without losing grain

---

## G. Should Temporal Observations Be a New Immutable Artifact Rather Than Modifying Raw Evidence?

**Answer: NEW ARTIFACT, NOT RAW EVIDENCE MODIFICATION**

**Rationale**:
1. **ReferenceChallenge is immutable** — do NOT modify its digest or structure
2. **Evidence layer is separate** — ReferenceChallenge ≠ evidence.Claim
3. **QCStatistics are derived** — they summarize ReferenceChallenge, do NOT replace them
4. **New temporal records should be separate immutable artifacts**

**Proposed structure**:
```
ReferenceChallenge (immutable, existing)
  ↓
TimeOrderedObservationSet (new, immutable)
  - Preserve timestamps, sequences, digests
  - Reference ReferenceChallenge by digest (not by copy)
  - Track run/day/epoch boundaries
  - Mark outliers without removing them
  ↓
QCStatistics (derived summary, existing)
  - Computed from TimeOrderedObservationSet
  - Policy-applied (outlier thresholds, etc.)
  - Backward-compatible with existing verdict logic
```

**Why separate**:
- ✅ Immutability preserved (each layer sealed independently)
- ✅ Audit trail intact (can drill-down from verdict → stats → ordered observations → raw challenges)
- ✅ No modification to existing layers (ReferenceChallenge, QCStatistics contracts unchanged)
- ✅ Temporal intelligence added without breaking Phase 3B

---

## H. What Identity Should Define: Run, Day, Longitudinal Knowledge Epoch?

**Proposed identities** (non-breaking, metadata level):

### **Run Identity**
```
RunID = <MappingIdentity>:<DeviceFingerprint>:<ISO8601-start-time>:<ISO8601-end-time>
Example: "siemens-infinity-hr:device-hash-xyz:2026-10-05T14:30:00Z:2026-10-05T14:35:00Z"
```
- **Supplied by**: Governance layer (ReviewQCGovernance or upper orchestration)
- **Immutable**: Once set, never changed
- **Unique**: Mapping + device + temporal boundary
- **Preserved in**: ReferenceChallenge.RunID (optional field), QCStatistics, QCKnowledgeRecord

### **Day Boundary**
```
DayID = <MappingIdentity>:<DeviceFingerprint>:<YYYY-MM-DD>
Example: "siemens-infinity-hr:device-hash-xyz:2026-10-05"
```
- **Supplied by**: Governance layer (ReviewQCGovernance or upper orchestration)
- **Immutable**: Date is immutable (no redefining "today")
- **Matches**: ISO 8601 date in UTC
- **Preserved in**: QCStatistics.DayBoundary, QCKnowledgeRecord.DayBoundary

### **Longitudinal Epoch**
```
LongitudinalEpochID = <MappingIdentity>:<DeviceFingerprint>:<policy-epoch-identifier>
Example: "siemens-infinity-hr:device-hash-xyz:2026-W40" (ISO 8601 week)
Or: "siemens-infinity-hr:device-hash-xyz:2026-Q4" (quarter)
Or: "siemens-infinity-hr:device-hash-xyz:firmware-v2.1.0" (version epoch)
```
- **Supplied by**: Governance policy (not inferred)
- **Policy-defined**: No universal rule (week/month/quarter/version/maintenance-cycle)
- **Immutable**: Once assigned, epoch identifier does not change
- **Preserved in**: QCKnowledgeRecord.LongitudinalEpoch

**Key principle**: Identities are **supplied by governance, not inferred**, and marked in immutable records.

---

## I. How Can Within-Day Reference Existing Within-Run Records/Digests Without Copying or Flattening Their Raw Observations?

**Architecture**: Reference by digest, not by copy

### **TimeOrderedObservationSet (new, immutable artifact)**
```go
type TimeOrderedObservationSet struct {
  // Identity
  RunID              string     `json:"run_id"`              // identifies the run
  DayBoundary        string     `json:"day_boundary"`        // "2026-10-05"
  
  // Reference to immutable within-run data
  WithinRunQCDigest  string     `json:"within_run_qc_digest"` // SHA256 of the WITHIN_RUN QCRecord
  
  // Within-day aggregation (WITHOUT copying raw observations)
  AllWithinRunDigests []string  `json:"within_run_digests"`  // digests of all WITHIN_RUN records in this day
  
  // Temporal ordering across runs
  RunSequenceWithinDay []int64  `json:"run_sequence_within_day"` // which run is this (1st, 2nd, etc.)?
  FirstRunAt          time.Time `json:"first_run_at"`
  LastRunAt           time.Time `json:"last_run_at"`
  
  // Aggregated statistics (computed from within-run stats, not raw observations)
  DayLevelStatistics  QCStatistics `json:"day_statistics"`   // mean of within-run means, etc.
  
  // Integrity
  Integrity           string    `json:"integrity"`          // SHA256 of above (immutable)
}
```

**Key properties**:
- ✅ References WITHIN_RUN QCKnowledgeRecord by digest (immutable link)
- ✅ Does NOT copy raw observations (references only)
- ✅ Preserves run identity and sequence
- ✅ Allows drill-down: DayStatistics → WithinRunDigest → ReferenceChallenge
- ✅ Day aggregation does NOT erase within-run granularity
- ✅ Separate immutable artifact (integrity sealed)

**Within-Day governance flow**:
1. Collect all WITHIN_RUN QCRecords for day (reference by digest only)
2. Verify none are test fixtures, all are PASS/PASS-family verdict
3. Compute day-level statistics from within-run statistics (aggregation, not re-computation)
4. Create TimeOrderedObservationSet with references to each within-run record
5. Seal with digest, return as immutable artifact
6. Do NOT modify or copy the within-run records

---

## J. How Can Longitudinal Reference Within-Day Records/Digests While Preserving Version Boundaries?

**Architecture**: Digest-based versioning boundary

### **LongitudinalQCKnowledgeSet (new, immutable artifact)**
```go
type LongitudinalQCKnowledgeSet struct {
  // Identity
  MappingIdentity      string   `json:"mapping_identity"`
  MappingVersion       string   `json:"mapping_version"`     // mapping version boundary
  DeviceRef            string   `json:"device_fingerprint"`
  LongitudinalEpoch    string   `json:"longitudinal_epoch"`  // "2026-Q4", "fw-v2.1.0", etc.
  
  // Version boundaries (immutable markers)
  MappingVersionEpochs []string `json:"mapping_version_epochs"` // which mapping versions included?
  DeviceFirmwareEpochs []string `json:"device_firmware_epochs"` // which firmware versions?
  FactorVersionEpochs  []string `json:"factor_version_epochs"`  // which TransferFunction versions?
  
  // Within-day records (referenced by digest, not copied)
  WithinDayDigests     []string `json:"within_day_digests"`  // SHA256 of each day's TimeOrderedObservationSet
  DaySequence          []string `json:"day_sequence"`        // chronological order: "2026-10-01", "2026-10-02", ...
  FirstDayAt           time.Time `json:"first_day_at"`
  LastDayAt            time.Time `json:"last_day_at"`
  
  // Longitudinal statistics (aggregated across days, respecting version boundaries)
  LongitudinalStatistics QCStatistics `json:"longitudinal_statistics"` // mean of day means, etc.
  
  // Version change detectors (flags, NOT automatic modification)
  MappingVersionChanges []VersionChangeEvent `json:"mapping_version_changes"`
  FirmwareVersionChanges []VersionChangeEvent `json:"firmware_version_changes"`
  FactorVersionChanges  []VersionChangeEvent `json:"factor_version_changes"`
  
  // Integrity
  Integrity            string   `json:"integrity"`          // SHA256 of above (immutable)
}

type VersionChangeEvent struct {
  DayBoundary    string    `json:"day_boundary"`
  OldVersion     string    `json:"old_version"`
  NewVersion     string    `json:"new_version"`
  DigestBefore   string    `json:"digest_before"`
  DigestAfter    string    `json:"digest_after"`
  ChangeMarked   bool      `json:"change_marked"`          // was this formally marked in governance?
}
```

**Key properties**:
- ✅ Preserves mapping/firmware/factor version boundaries explicitly
- ✅ References within-day records by digest (immutable, no copying)
- ✅ Tracks version changes WITHOUT automatically modifying baselines
- ✅ Aggregate statistics computed from day-level statistics (not raw observations)
- ✅ Drill-down: LongitudinalStats → WithinDayDigest → TimeOrderedObservationSet → ReferenceChallenge
- ✅ Version boundaries marked but NOT enforced (drift detection is advisory, not automatic)

**Longitudinal governance flow**:
1. Collect within-day TimeOrderedObservationSet records for epoch
2. Group by mapping version, firmware version, factor version
3. Detect version changes (flags set, NOT automatic re-baselining)
4. Compute longitudinal statistics respecting version boundaries
5. Create LongitudinalQCKnowledgeSet with references and version markers
6. Seal with digest
7. Return for pattern analysis (drift, shift, trend candidates) — NOT authority modification

---

## K. Where Should Trend / Shift / Recurrence Analysis Live?

**Proposed location: NEW MODULE `internal/qc/temporal/` (Phase 4B)**

### **Structure**:
```
internal/qc/temporal/
├── patterns.go          // Pattern definition (Trend, Shift, Recurrence, Drift)
├── detector.go          // Pattern detection (advisory, diagnostic, NOT authority)
├── candidate.go         // Pattern candidate record (immutable diagnosis)
└── temporal_test.go     // Tests proving no auto-modification
```

### **Key principle**: Pattern analysis is **DIAGNOSTIC, NOT AUTHORITY**

**PatternCandidate struct**:
```go
type PatternCandidate struct {
  // Pattern identity
  PatternType         string    `json:"pattern_type"`       // "trend", "shift", "recurrence", "drift"
  Confidence          float64   `json:"confidence"`         // 0.0–1.0 statistical likelihood
  
  // Evidence
  LongitudinalDigest  string    `json:"longitudinal_digest"` // which LongitudinalQCKnowledgeSet?
  DayStart            string    `json:"day_start"`
  DayEnd              string    `json:"day_end"`
  
  // Diagnosis (NOT action)
  Characteristic      string    `json:"characteristic"`     // "mean increasing 5%/day", etc.
  Evidence            string    `json:"evidence"`           // statistical basis
  
  // Important: NOT authority
  DoesNotModify       []string  `json:"does_not_modify"`    // ["factor", "mapping", "reference", "acceptance_criteria"]
  RequiresHumanReview bool      `json:"requires_human_review"`
  
  // Integrity
  Integrity           string    `json:"integrity"`          // SHA256 (immutable diagnosis)
}
```

### **Constraints**:
- ✅ Pattern candidates are **created** but NOT **acted upon** automatically
- ✅ Diagnosis is **traced back** to immutable within-day/within-run records
- ✅ No automatic baseline recentering
- ✅ No automatic factor/mapping updates
- ✅ Human review required for any governance change
- ✅ Pattern candidate digest is immutable (diagnosis sealed)

---

## L. How Should Candidate Patterns Be Represented Without Turning Them Into Authority?

**Answer: AS DIAGNOSTIC ARTIFACTS, NOT POLICY ARTIFACTS**

**Key distinction**:
- **Policy artifact** (has authority): factor, reference, mapping, acceptance criteria, verdict
- **Diagnostic artifact** (has no authority): pattern candidate, observation set, temporal statistics

**PatternCandidate immutability**:
```go
type PatternCandidate struct {
  // Diagnostic fields
  PatternType         string    // "drift", "shift", "trend", "recurrence"
  Confidence          float64   // statistical measure
  Evidence            string    // "mean increased 3.2±0.8% per day for 7 days"
  Characteristic      string    // "increasing trend"
  
  // Authority boundary (explicit)
  CannotTrigger       []string `json:"cannot_trigger"` // ["reference_update", "factor_adjustment", "mapping_change"]
  RequiresExplicitApproval bool `json:"explicit_approval_required"` // always true
  
  // Traceability
  DiagnosedFrom       string `json:"diagnosed_from"`      // LongitudinalQCKnowledgeSet digest
  DiagnosisTimestamp  time.Time
  DiagnosisAuthor     string
  
  // Integrity (sealed diagnostic, not authority)
  Integrity           string `json:"integrity"`
}
```

**Usage model** (Phase 4B+):
1. **Detect** pattern candidate from longitudinal data
2. **Create** immutable PatternCandidate record
3. **Alert** human: "Drift pattern detected (drift_20261005_abc123)"
4. **Human reviews** and decides: ignore / investigate / update factor / update mapping
5. **If action taken**: create separate change artifact (does NOT modify PatternCandidate)
6. **Traceability preserved**: pattern → decision → action, all separate and immutable

---

## M. How Should QC Diagnostic Memory Aggregate History While Retaining Drill-Down to Immutable Evidence?

**Proposed: QCDiagnosticMemory (new, semi-mutable but auditable)**

```go
type QCDiagnosticMemory struct {
  // Identity
  MappingIdentity     string `json:"mapping_identity"`
  DeviceRef           string `json:"device_fingerprint"`
  CreatedAt           time.Time
  
  // Aggregated view (computed, not immutable)
  LatestWithinRunQCDigest   string `json:"latest_within_run_qc_digest"`
  LatestWithinDayQCDigest   string `json:"latest_within_day_qc_digest"`
  LatestLongitudinalDigest  string `json:"latest_longitudinal_digest"`
  
  // Pattern history (immutable log of candidates)
  PatternCandidateDigests []string `json:"pattern_candidate_digests"` // chronological
  PatternCandidateCount   int      `json:"pattern_candidate_count"`
  
  // Statistics summary (for quick access, but always drill-down available)
  RecentMeanValue         float64  `json:"recent_mean_value"` // last within-run mean
  RecentTrendDirection    string   `json:"recent_trend_direction"` // "increasing", "stable", "decreasing"
  
  // Integrity (aggregated view is re-computed, not sealed)
  LastComputedAt         time.Time `json:"last_computed_at"`
}
```

**Key design**:
- ✅ Aggregated view is **convenience, not authority**
- ✅ Every summary field can be **re-derived** from immutable records
- ✅ **Drill-down path preserved**: memory → latest digest → immutable artifact → raw observations
- ✅ **No loss of granularity**: can always reconstruct full history
- ✅ **Audit trail intact**: all immutable digests stored chronologically

**Usage**:
1. User accesses QCDiagnosticMemory for quick summary
2. User clicks on "latest within-run" → loads QCKnowledgeRecord by digest
3. User clicks on "pattern candidates" → list of PatternCandidate digests
4. User selects one → loads immutable PatternCandidate
5. User clicks "drill down" → loads LongitudinalQCKnowledgeSet
6. User clicks "within-day" → loads TimeOrderedObservationSet
7. User clicks "challenge" → loads ReferenceChallenge with timestamp

**Traceability preserved**, granularity intact, no loss of evidence.

---

## N. Which Existing Structs/Functions Require Modification?

**MINIMAL MODIFICATIONS (backward-compatible only)**

### **DO NOT MODIFY**:
- ✅ ReferenceChallenge struct (add optional Sequence, RunID only if needed)
- ✅ QCStatistics struct (add optional temporal fields only)
- ✅ ReviewQC() function signature (does NOT change)
- ✅ QCKnowledgeRecord verdict logic (UNCHANGED)
- ✅ AcceptanceCriteria evaluation (UNCHANGED)
- ✅ Registry signature or canonical payload (UNCHANGED)
- ✅ Profile builder (UNCHANGED)

### **MINIMAL ADD-ONS (optional fields, no breaking changes)**:
1. **ReferenceChallenge**: Add optional `Sequence`, `RunID` fields (NOT required)
2. **QCStatistics**: Add optional temporal bounds (NOT required for calculation)
3. **QCKnowledgeRecord**: Add optional `RunID`, `DayBoundary`, `LongitudinalEpoch`, temporal bounds (NOT required)

### **NEW STRUCTURES** (no modification of existing):
- TimeOrderedObservationSet (new)
- LongitudinalQCKnowledgeSet (new)
- PatternCandidate (new)
- QCDiagnosticMemory (new)
- VersionChangeEvent (new)

**Critical**: All new structures are **optional extensions**, not modifications of existing governance layer.

---

## O. Which New Files/Modules, If Any, Are Actually Necessary?

**MINIMUM NEW FILES** (Phase 4B):

### **internal/qc/temporal/** (new module)
1. **patterns.go** (~150 lines)
   - PatternCandidate struct
   - PatternType enum (trend, shift, recurrence, drift)
   - New() constructor with validation
   - Verify() integrity check

2. **detector.go** (~200 lines)
   - DetectTrend() diagnostic function
   - DetectShift() diagnostic function
   - DetectRecurrence() diagnostic function
   - All return PatternCandidate, NOT authority

3. **observation.go** (~100 lines)
   - TimeOrderedObservationSet struct
   - New() constructor
   - Verify() integrity check
   - DrillDownToChallenge() reference resolver

4. **longitudinal.go** (~150 lines)
   - LongitudinalQCKnowledgeSet struct
   - New() constructor
   - Verify() integrity check
   - VersionBoundaryMarkers() metadata

5. **temporal_test.go** (~200 lines)
   - Tests proving immutability
   - Tests proving no auto-modification
   - Tests proving drill-down traceability

### **NO MODIFICATIONS NECESSARY** to:
- internal/qc/challenge.go
- internal/qc/reference.go
- internal/qc/verdict.go
- internal/qc/qcstats.go
- internal/qc/qcresult.go
- internal/qc/review.go
- internal/qc/factor.go
- cmd/WIJOYO_KUSUMO/main.go
- cmd/WIJOYO_KUSUMO/governed_qc.go
- internal/profilebuilder/*
- internal/registry/*

**Total**: ~800 lines new code, zero modifications to existing governance layer.

---

## P. Which Existing APIs/Signatures Can Remain Unchanged?

**ALL EXISTING APIS UNCHANGED**:

✅ `ReviewQC()` — signature identical, functionality identical
✅ `Calculate()` — signature identical, functionality identical
✅ `Evaluate()` — signature identical, functionality identical
✅ `NewQCRecord()` — signature identical, functionality identical
✅ `BuildFromMappingDefinition()` — signature identical (qcRecord is optional)
✅ `Admit()` — signature identical (qcRecord is optional)
✅ `Registry.Verify*()` — signature identical (ignores QC binding in old attestations)
✅ `Approval.Review()` — signature identical, approval logic unchanged

**Backward compatibility guarantee**:
- Old code calling ReviewQC() works identically
- Old QCRecords are still valid (no schema change breaks them)
- Old attestations still verify (Registry canonical payload is conditional on QC presence)
- New records can OPTIONALLY include temporal data, old records lack it (both valid)

---

## Q. What Backward-Compatibility Risks Exist?

### **ZERO BREAKING RISKS** (if design properly scoped)

| Component | Risk | Mitigation |
|---|---|---|
| **Phase 3B Governed Pipeline** | Temporal fields are optional; old challenges still validate | Add fields as `omitempty` in JSON, make validation optional |
| **Profile Builder** | No change to BuildFromMappingDefinition() | Do NOT modify signature |
| **Registry Canonical Payload** | Current payload is fixed; adding QC is conditional | QC binding only in Attestation.QC (optional field); canonical payload conditional on QC!=nil |
| **Ed25519 Signatures** | Attestation signature format fixed | Do NOT change canonical payload structure; append QC as separate optional fields |
| **Existing QC Records** | Old records lack temporal data | New records include temporal data optionally; old records still PASS verdict logic |
| **QCStatistics Evaluation** | Verdict calculation unchanged | Do NOT modify Evaluate() logic; temporal fields are metadata only |

### **Identified Risk**: Canonical Payload Mutation
- **Problem**: If canonical payload ALWAYS includes QC fields (even when nil), old verifiers reject new attestations
- **Solution**: Make QC fields conditional in canonical payload (only include if QC!=nil) — already designed this way in Phase 3B
- **Proof**: Registry.go line 126 uses conditional QC in payload, signatures backward-compatible

---

## R. Propose the Minimum Phase 4B Implementation Scope

**PHASE 4B: Temporal QC Intelligence — Implementation Scope**

### **Goals**:
1. Preserve temporal information from ReferenceChallenge to QC verdict
2. Support within-run, within-day, and longitudinal analysis without parallel pipelines
3. Enable pattern diagnosis (trend/shift/recurrence/drift candidates) as diagnostic, NOT authority
4. Maintain immutability and audit trail end-to-end
5. Zero breaking changes to Phase 3B

### **Deliverables**:

**1. TimeOrderedObservationSet** (~100 lines)
- Immutable artifact preserving challenge digests, timestamps, sequences
- References within-run QCRecord by digest
- New constructor, verify() integrity check
- JSON marshaling

**2. LongitudinalQCKnowledgeSet** (~150 lines)
- Immutable artifact aggregating within-day records
- Marks version boundaries (mapping/firmware/factor changes)
- References within-day records by digest
- New constructor, verify() integrity check

**3. PatternCandidate** (~80 lines)
- Diagnostic artifact for trend/shift/recurrence detection
- Immutable sealed diagnosis
- NO authority to modify factor/mapping/reference
- New constructor, verify() integrity check

**4. Detector functions** (~150 lines)
- DetectTrend() — increasing/decreasing pattern
- DetectShift() — step change detection
- DetectRecurrence() — periodic pattern
- All return PatternCandidate (diagnostic only)

**5. Tests** (~300 lines)
- Within-run temporal ordering preserved
- Within-day isolation (multiple runs aggregated correctly)
- Longitudinal version boundaries respected
- Pattern candidates do NOT trigger auto-modification
- Drill-down to ReferenceChallenge possible
- Missing observations remain missing
- Outliers marked but not deleted
- Low/Normal/High populations isolated
- All Phase 3B tests still PASS

### **No modifications to**:
- ReviewQC() logic
- QCStatistics calculation
- Registry signatures
- Profile builder
- Approval logic
- Existing verdict evaluation

**Estimated effort**: 800 lines new code, 300 lines new tests, ~2-3 days

---

## S. Propose Tests Proving Constraints

**PHASE 4B TEST MATRIX** (20+ tests, all diagnostic):

### **Temporal Ordering Tests**:
1. ✅ Within-run observations preserve timestamp order
2. ✅ Sequence numbers increase monotonically
3. ✅ Out-of-order timestamps detected (advisory flag, no reordering)

### **Observation Preservation Tests**:
4. ✅ Raw values preserved in TimeOrderedObservationSet
5. ✅ Challenge digest points to immutable ReferenceChallenge
6. ✅ Drill-down from statistics to raw challenge timestamp works

### **Run Isolation Tests**:
7. ✅ Multiple within-run records within same day are distinct
8. ✅ Run boundaries (start/end timestamp) marked in identity
9. ✅ RunID is immutable (part of record digest)

### **Day Isolation Tests**:
10. ✅ Within-day aggregation does NOT flatten runs
11. ✅ Day boundary is calendar date (UTC), not inferred
12. ✅ Different days have different DayBoundary values
13. ✅ Within-day statistics computed from within-run statistics (not raw obs)

### **Longitudinal Boundary Tests**:
14. ✅ Version boundaries (mapping/firmware/factor) marked explicitly
15. ✅ Version change flags set WITHOUT automatic baseline update
16. ✅ Statistics computed respecting version boundaries
17. ✅ LongitudinalEpochID is immutable

### **Missing Data Tests**:
18. ✅ Zero observations ≠ zero mean (missing != fabricated)
19. ✅ Observations marked as missing remain in history
20. ✅ CV undefined for zero-mean preserved (not recentered)

### **Outlier Tests**:
21. ✅ Outliers flagged but not deleted
22. ✅ Count of outliers tracked
23. ✅ Statistics computed with/without outlier flags available

### **Population Isolation Tests**:
24. ✅ LOW/NORMAL/HIGH populations isolated in all temporal scopes
25. ✅ Aggregate statistics do NOT mix LOW and NORMAL

### **Pattern Diagnostic Tests**:
26. ✅ PatternCandidate marked with "does_not_modify" list
27. ✅ Trend detection returns diagnostic, not authority
28. ✅ Shift detection does NOT trigger automatic factor update
29. ✅ Pattern candidate has requires_human_review=true
30. ✅ Pattern candidate digest immutable (diagnosis sealed)

### **Immutability Tests**:
31. ✅ TimeOrderedObservationSet digest matches canonical representation
32. ✅ LongitudinalQCKnowledgeSet digest immutable
33. ✅ PatternCandidate digest immutable
34. ✅ Modification of temporal artifact fails (digest mismatch)

### **Backward Compatibility Tests**:
35. ✅ Phase 3B ReviewQC() unchanged
36. ✅ Phase 3B verdict evaluation unchanged
37. ✅ Registry attestation without QC still verifies
38. ✅ Old QCRecords still pass validation
39. ✅ Phase 3B tests all still PASS

### **Authority Preservation Tests**:
40. ✅ PatternCandidate cannot trigger factor modification
41. ✅ PatternCandidate cannot trigger mapping modification
42. ✅ PatternCandidate cannot trigger reference update
43. ✅ PatternCandidate cannot trigger acceptance criteria change
44. ✅ Trend diagnosis ≠ cause (data only)
45. ✅ Shift diagnosis ≠ semantic authority

---

## PHASE 4A STATUS

### **Existing Capability**
✅ Temporal data captured in ReferenceChallenge (timestamp immutable)  
✅ Scope framework defined (WITHIN_RUN, WITHIN_DAY, LONGITUDINAL)  
✅ Immutability enforcement via SHA256 digest  
✅ Authority boundary separation preserved  
✅ Phase 3B governance fully operational  

### **Missing Capability**
❌ Observation ordering information (no sequence field in challenge)  
❌ Run/day/epoch identity binding (no RunID, DayBoundary in records)  
❌ Temporal bounds in aggregates (no first/last timestamp in stats)  
❌ Challenge traceability (no digest references in stats/records)  
❌ Within-day aggregation (scoped WITHIN_DAY unimplemented)  
❌ Longitudinal analysis (scoped LONGITUDINAL unimplemented)  
❌ Version boundary tracking (no firmware/factor epoch markers)  
❌ Pattern detection (trend/shift/recurrence unimplemented)  
❌ Temporal drill-down (cannot trace verdict back to timestamp)  

### **Minimum Safe Extension**
1. **Add optional temporal fields** (non-breaking):
   - ReferenceChallenge: `Sequence int64`, `RunID string` (omitempty)
   - QCStatistics: `FirstObservationAt`, `LastObservationAt`, `ChallengeDigests[]` (omitempty)
   - QCKnowledgeRecord: `RunID`, `DayBoundary`, `LongitudinalEpoch` (omitempty)

2. **Create new immutable artifacts** (no modification):
   - TimeOrderedObservationSet (within-run temporal ordering)
   - LongitudinalQCKnowledgeSet (longitudinal aggregation with version boundaries)
   - PatternCandidate (diagnostic drift/trend/shift without authority)

3. **Add detective functions** (diagnostic only):
   - DetectTrend() — statistical trend analysis
   - DetectShift() — step-change detection
   - DetectRecurrence() — periodic pattern
   - All return diagnostic records, NOT policy modifications

### **Files Affected**
- ✅ internal/qc/temporal/patterns.go (new)
- ✅ internal/qc/temporal/detector.go (new)
- ✅ internal/qc/temporal/observation.go (new)
- ✅ internal/qc/temporal/longitudinal.go (new)
- ✅ internal/qc/temporal/temporal_test.go (new)
- ⚠️ ReferenceChallenge (optional fields only, backward-compat)
- ⚠️ QCKnowledgeRecord (optional fields only, backward-compat)

### **Backward Compatibility Risks**
✅ ZERO BREAKING RISKS (if scoped correctly)
- All temporal fields optional (omitempty in JSON)
- ReviewQC() signature unchanged
- Registry attestation backward-compatible (QC binding optional)
- Ed25519 signatures unchanged
- Phase 3B tests all still PASS

### **Proposed Phase 4B Test Matrix**
✅ 45 tests covering:
- Temporal ordering preservation
- Run/day/longitudinal isolation
- Missing data handling (missing ≠ zero)
- Outlier preservation (flagged, not deleted)
- Pattern diagnostics (NOT authority)
- Immutability enforcement
- Authority boundaries unchanged
- Phase 3B backward compatibility

---

**PHASE 4A COMPLETE**: Inspection shows temporal data IS captured (timestamps in ReferenceChallenge), but temporal IDENTITY (run/day/epoch) and ANALYSIS (pattern detection) are missing. Minimum safe extension adds optional metadata fields and three new immutable artifact types (TimeOrderedObservationSet, LongitudinalQCKnowledgeSet, PatternCandidate) with ZERO modifications to Phase 3B governance layer.


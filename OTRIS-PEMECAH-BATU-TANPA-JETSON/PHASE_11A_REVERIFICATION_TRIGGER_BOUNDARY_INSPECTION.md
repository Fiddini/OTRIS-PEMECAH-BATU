# PHASE 11A — RE-VERIFICATION TRIGGER BOUNDARY INSPECTION

**Baseline**: edfde954cda788585e1fcec56f8a2cdd417d5f47

**Task**: Define smallest safe trigger for re-verification when observed identity change occurs.
Keep inspection narrow: only what is necessary for decision logic.

---

## 1. EXISTING FINGERPRINT OBSERVATION/REVIEW STATES

**FingerprintIdentityObservation** (lines 1-226):

| State | Definition | Initiates Re-verification? |
|-------|-----------|---------------------------|
| **ChangeType = INITIAL** | First fingerprint observation | NO (baseline observation) |
| **ChangeType = UPDATE** | Legitimate device change (firmware, hardware, location) | YES (if confirmed by engineer) |
| **ChangeType = POSSIBLE_CORRUPTION** | Unexpected/suspicious change | YES (requires investigation) |
| **ChangeType = UNKNOWN** | Cannot determine cause | CONDITIONAL (pending review) |

**Investigation Status**:
- UNREVIEWED (initial, auto-flagged if confidence > 0.7)
- UNDER_INVESTIGATION (engineer is reviewing)
- CLOSED (engineer has made decision via Review())

**Review Method** (line 152-178):
```go
func (fpo *FingerprintIdentityObservation) Review(
  reviewedBy, notes string,
  confirmed bool,  // ← KEY: engineer explicitly confirms change is legitimate
) error
```

**IsConfirmedChange()** (line 217-219):
```go
func (fpo *FingerprintIdentityObservation) IsConfirmedChange() bool {
  return fpo.InvestigationStatus == "CLOSED" && fpo.ChangeType == ChangeTypeUpdate
}
```

**Trigger State Summary**:
- ✓ **INITIAL**: No re-verification needed (baseline)
- ✓ **UPDATE + CLOSED + confirmed=true**: Engineer approved change; re-verification required
- ✓ **POSSIBLE_CORRUPTION + CLOSED**: Engineer reviewed suspicious change; re-verification required
- ✓ **UNKNOWN + CLOSED**: Engineer decision recorded; re-verification decision needed
- ✓ **UNREVIEWED** or **UNDER_INVESTIGATION**: No trigger yet; pending engineer review

---

## 2. IMMUTABLE REGISTRY ATTESTATION & SEPARATE RE-VERIFICATION STATE

**Registry Attestation** (immutable):
```go
type Attestation struct {
  AdmissionStatus string        // "REGISTRY_ADMITTED" (final, never changes)
  // ... other fields (all immutable once signed)
  Signature string              // Ed25519 signature over canonical payload
}
```

**Key Insight**: Attestation remains unchanged. A separate artifact tracks re-verification requirement.

**Proposed Pattern**:
```
Registry Attestation (immutable)
  ↓ (references)
FingerprintIdentityObservation (observation recorded)
  ↓ (references)
Re-verification Requirement (NEW, separate state)
  - Status: NO_ACTION | REVERIFY_REQUIRED | INSUFFICIENT_EVIDENCE
  - Initiator: which observation triggered it
  - Authority barrier: cannot modify attestation authority
```

**Result**: Attestation unchanged. New immutable record links observation → decision.

---

## 3. MINIMUM DECISION RESULT NEEDED

**Existing Type Suggestion**: No perfect existing match. Create minimal enum:

```go
type ReverificationStatus string

const (
  // Observation does not require re-verification
  ReverificationNoAction ReverificationStatus = "NO_ACTION"
  
  // Identity change confirmed; re-verification required
  ReverificationRequired ReverificationStatus = "REVERIFY_REQUIRED"
  
  // Insufficient evidence to decide; escalation needed
  ReverificationInsufficientEvidence ReverificationStatus = "INSUFFICIENT_EVIDENCE"
)
```

**Decision Logic** (minimal):
```
IF FingerprintIdentityObservation.IsConfirmedChange() == true
  → ReverificationRequired
ELSE IF ChangeType == POSSIBLE_CORRUPTION AND InvestigationStatus == CLOSED
  → ReverificationRequired
ELSE IF ChangeType == UNKNOWN AND InvestigationStatus == CLOSED
  → InsufficientEvidence (engineer judgment call)
ELSE IF InvestigationStatus != CLOSED
  → NoAction (pending review)
ELSE
  → NoAction (legitimate, no change needed)
```

---

## 4. NARROWEST RULE FOR FINGERPRINT CHANGE

**Fingerprint Change Rule** (immutable constraint):

```
Fingerprint Change Event
  ↓
FingerprintIdentityObservation created (immutable record)
  ↓
Engineer Review decides: legitimate or suspicious?
  ↓
IF change confirmed as legitimate (UPDATE + confirmed=true)
  ├─ Status: REVERIFY_REQUIRED
  └─ Reason: Device hardware changed; re-verify mapping against new hardware
     (e.g., firmware version, calibration state may differ)
  
IF change detected as suspicious (POSSIBLE_CORRUPTION)
  ├─ Status: REVERIFY_REQUIRED
  └─ Reason: Device integrity uncertain; full re-verification mandatory
  
IF change is UNKNOWN and engineer reviewed it
  ├─ Status: INSUFFICIENT_EVIDENCE
  └─ Reason: Engineer judgment needed; may require additional investigation

IF change is INITIAL
  └─ Status: NO_ACTION (baseline observation)
```

**Critical Boundaries** (NOT crossed):
- ✗ Fingerprint change ≠ automatic device failure (observation, not diagnosis)
- ✗ Fingerprint change ≠ automatic attestation revocation (immutable)
- ✗ Fingerprint change ≠ automatic new authority (existing gates required)
- ✓ Fingerprint change = identity deviation → POSSIBLE re-verification need (engineer-confirmed)

---

## 5. WHO MAY TRIGGER RE-VERIFICATION

**Separation of Concerns**:

| Phase | Actor | Action | Authority |
|-------|-------|--------|-----------|
| **Detection** | Automatic (PB-10, edge-discovery) | Observe FP change → create Observation | None (recording only) |
| **Investigation** | Engineer | Review observation → call Review(confirmed=true/false) | Governance decision |
| **Decision** | System (orchestration) | Read InvestigationStatus + ChangeType → decide ReverificationStatus | Applies decision logic |
| **Execution** | Engineer | Approve/reject re-verification (if REVERIFY_REQUIRED) | Final authority |

**Trigger Sources**:
1. **Automatic Detection** (reads Observation):
   - When FingerprintIdentityObservation.InvestigationStatus == "CLOSED"
   - Compute ReverificationStatus from ChangeType + engineer's confirmed flag

2. **Engineer Review** (writes Observation):
   - Engineer calls Observation.Review(reviewedBy, notes, confirmed)
   - This transition from UNREVIEWED → CLOSED permits downstream decision

3. **No automatic approval** of re-verification:
   - ReverificationStatus = REVERIFY_REQUIRED means "candidate for re-verification"
   - Still requires existing gates (ReviewEvidence, approval, etc.)

---

## 6. SMALLEST IMPLEMENTATION REQUIRED

**New Type** (location: internal/identity/reverification.go, ~100 lines):

```go
package identity

// ReverificationStatus describes whether identity change requires re-verification.
type ReverificationStatus string

const (
  ReverificationNoAction              ReverificationStatus = "NO_ACTION"
  ReverificationRequired              ReverificationStatus = "REVERIFY_REQUIRED"
  ReverificationInsufficientEvidence  ReverificationStatus = "INSUFFICIENT_EVIDENCE"
)

// ReverificationRequirement links a fingerprint observation to re-verification decision.
type ReverificationRequirement struct {
  ObservationID string                 // which observation triggered this
  Status ReverificationStatus          // decision
  Reason string                        // why
  DetectedAt time.Time
  DecidedAt time.Time
  DecidedBy string
  Digest string                        // immutable integrity
}

// DecideFromObservation evaluates whether observation requires re-verification.
func DecideFromObservation(obs *FingerprintIdentityObservation) ReverificationStatus {
  if obs.InvestigationStatus != "CLOSED" {
    return ReverificationNoAction  // pending review
  }
  
  if obs.ChangeType == ChangeTypeInitial {
    return ReverificationNoAction  // baseline
  }
  
  if obs.ChangeType == ChangeTypeUpdate {
    return ReverificationRequired  // legitimate change needs re-verify
  }
  
  if obs.ChangeType == ChangeTypePossibleCorruption {
    return ReverificationRequired  // suspicious change needs investigation
  }
  
  if obs.ChangeType == ChangeTypeUnknown {
    return ReverificationInsufficientEvidence  // pending more info
  }
  
  return ReverificationNoAction
}
```

**No modification to**:
- FingerprintIdentityObservation (already perfect)
- IdentityConflict (already perfect)
- Registry attestation (immutable by design)
- Existing gates (ReviewEvidence/Replay/Approval/Registry)

---

## 7. EXACT FILES/FUNCTIONS/TESTS

**Existing Primitives** (already complete):
- `internal/identity/fingerprint_observation.go` (226 lines)
  - FingerprintIdentityObservation struct
  - ChangeType enum (INITIAL, UPDATE, POSSIBLE_CORRUPTION, UNKNOWN)
  - Review() method (engineer decision)
  - IsConfirmedChange() (query method)
  
- `internal/identity/conflict.go` (231 lines)
  - IdentityConflict struct
  - ConflictType enum (FINGERPRINT_MISMATCH, etc.)
  - Resolve() method (engineer decision)
  - IsResolved() (query method)

**Tests** (31 tests, all PASS):
- `internal/identity/identity_test.go`: Tests for observation immutability, conflict recording, review workflows

**New Implementation** (minimal):
- `internal/identity/reverification.go` (~100 lines)
  - ReverificationStatus enum (NO_ACTION, REVERIFY_REQUIRED, INSUFFICIENT_EVIDENCE)
  - ReverificationRequirement struct
  - DecideFromObservation(obs) ReverificationStatus function

**Orchestration** (later):
- `cmd/WIJOYO_KUSUMO/triggered_reverification.go` (future, not in scope)
  - Will read ReverificationRequirement
  - Will decide whether to call existing gates

---

## BOUNDARY SUMMARY

| Concept | Definition | Implementation |
|---------|-----------|-----------------|
| **Detection** | Observation of FP change | FingerprintIdentityObservation (immutable record) |
| **Investigation** | Engineer reviews observation | Observation.Review(confirmed=bool) |
| **Decision** | Should we re-verify? | ReverificationStatus enum (NO_ACTION / REQUIRED / INSUFFICIENT) |
| **Authority** | Can we modify attestation? | NO (immutable) |
| **Next Step** | If REQUIRED, what happens? | Existing gates unchanged; orchestration decides flow |

**Safe Constraint**:
- Observation is immutable (cannot be fabricated)
- Engineer decision is explicit (Review() called, recorded)
- ReverificationStatus is decision-only (no authority transfer)
- Old attestation unchanged (new record references it)

---

No implementation. No code changes. Inspection complete. STOP.

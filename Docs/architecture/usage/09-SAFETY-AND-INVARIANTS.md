# usage — Safety and Invariants

> Last verified: **2026-07-13**

## Constitutional / Mangle surface

| Item | Status |
|------|--------|
| Local `.mg` rules | **None** |
| `Decl` predicates | **None** |
| `permitted(...)` interaction | **None** |
| Default-deny impact | **N/A** — package cannot authorize effects |

Safety role: **do not become a backdoor into effectful systems**. Keep usage free of filesystem targets beyond `.nerd/usage.json`, free of network, free of shell.

## Invariants

### I1 — Tracker optional

`FromContext` returns nil safely. Callers must nil-check before Track. ZAI honors this.

### I2 — No panic on bad context metadata

String keys may hold non-strings. Track uses comma-ok assertions. Covered by `TestTracker_Track_WhenNonStringContextValues_ShouldNotPanic`.

### I3 — Mutex serializes mutable aggregate maps

Concurrent Track/Stats/Save must not race map writes. All entry points lock `mu`.

### I4 — Stats isolation

Returned maps are copies. UI mutation cannot corrupt durable state until next Save of wrong data (which would require Track, not Stats mutation).

### I5 — Load rehydrates nil maps

Partial JSON without map fields must not leave nil maps that panic on first Track. Load initializes all five maps if nil.

### I6 — Soft boot on corrupt store

Corrupt `usage.json` must not abort agent boot. NewTracker returns working empty tracker (error from Load discarded).

### I7 — Path confinement

Tracker only writes its configured `filePath` under `.nerd`. No user-controlled relative path traversal API.

### I8 — Typed tracker key privacy

`contextKey` is unexported; external packages cannot forge tracker context without `NewContext`.

## Concurrency invariants (and known weak spots)

| Invariant | Strength |
|-----------|----------|
| Map integrity under concurrent Track | **Strong** (mutex) |
| Debounced Save eventual consistency | **Weak** — dirty re-arm race |
| Multi-process single-file writers | **None** — last writer wins / tear risk |
| AfterFunc after process exit | **None** — may drop last dirty burst if no explicit Save |

## Data integrity invariants

| Rule | Notes |
|------|-------|
| Total ≈ sum of dimensions | Not formally enforced; each Track updates all dimensions with same in/out, so TotalProject should match sum of any one full partition if keys cover all events |
| `"unknown"` is a valid partition key | Missing attribution lands here |
| Version string present | Set to `"1.0"` on create; not migrated automatically |

## Threat model (lightweight)

| Threat | Mitigation |
|--------|------------|
| Malicious JSON in workspace | Unmarshal failure → empty tracker; no code exec |
| Shared workspace multi-user | No auth; OS file permissions only (0644) |
| Token count spoofing via context | Not a security boundary; self-reported telemetry |
| Panic DoS via context type confusion | Fixed with comma-ok |

## What this package must never do

1. Execute tools or open arbitrary files based on usage thresholds.  
2. Assert Mangle facts that change `next_action` without an explicit higher-level design.  
3. Store API keys or prompt bodies in `usage.json` (only counters and ids).  
4. Panic the session because metering failed.

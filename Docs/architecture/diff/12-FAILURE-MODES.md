# 12 — Failure Modes: `internal/diff`

> Last verified against codebase: 2026-07-13  
> Concrete modes with detection, impact, and mitigation.

## FM1 — Binary content rendered as text garbage (prevented)

| | |
|--|--|
| **Trigger** | Content contains `0x00` |
| **Behavior** | `IsBinary=true`, empty `Hunks`, no Myers |
| **Impact** | None if UI checks flag; bad UX if ignored |
| **Mitigation** | Engine short-circuit; UI shows “Binary file - diff not shown” |
| **Residual** | Binary without NUL still processed as text |

## FM2 — Pathological minified input hangs agent (mitigated)

| | |
|--|--|
| **Trigger** | Multi-MB single-line / adversarial Myers cost |
| **Behavior** | `DiffTimeout` (5s) bounds library work |
| **Impact** | Up to ~5s latency; possibly incomplete diff result depending on library |
| **Mitigation** | Timeout set in `NewEngine` |
| **Residual** | No package-level “timed out” flag; caller cannot distinguish timeout vs tiny delta |

## FM3 — Cache returns wrong hunks (hash collision)

| | |
|--|--|
| **Trigger** | Distinct content pairs share FNV-1a (old,new) key |
| **Behavior** | Cache hit returns prior `FileDiff` hunks with new paths |
| **Impact** | Incorrect review / wrong approve decision |
| **Likelihood** | Very low for 64-bit pair |
| **Mitigation today** | None (no content verify) |
| **Hardening** | Store lengths + sample hash, or full content key / SHA |

## FM4 — Cache poisoning via shared slice mutation

| | |
|--|--|
| **Trigger** | Caller mutates `Hunks` or `Lines` after cache hit return |
| **Behavior** | Shallow copy shares slice headers → cache mutated |
| **Impact** | Cross-request corruption |
| **Mitigation today** | Discipline: treat as immutable |
| **Hardening** | Deep copy on hit / copy-on-write |

## FM5 — Unbounded memory from unique pairs

| | |
|--|--|
| **Trigger** | Long session, many unique old/new content pairs on one Engine |
| **Behavior** | `sync.Map` grows without eviction |
| **Impact** | Memory pressure / OOM |
| **Mitigation today** | Manual `ClearCache`; use short-lived engines |
| **Hardening** | LRU / max entries |

## FM6 — Both-empty dual flags confuse callers

| | |
|--|--|
| **Trigger** | `oldContent == "" && newContent == ""` |
| **Behavior** | `IsNew && IsDelete`, zero hunks |
| **Impact** | Misclassification if caller assumes XOR |
| **Mitigation** | Documented + tested; callers must handle both |

## FM7 — Empty hunks for identical files misread as error

| | |
|--|--|
| **Trigger** | Identical non-empty content |
| **Behavior** | Zero hunks, flags false |
| **Impact** | UI “empty” state |
| **Mitigation** | Expected; tests assert 0 hunks |

## FM8 — Concurrent ClearCache races

| | |
|--|--|
| **Trigger** | `ClearCache` concurrent with `ComputeDiff` Store/Load |
| **Behavior** | Map swap; possible stores to abandoned map; Load may miss |
| **Impact** | Lost cache entries (performance), unlikely data race on Map itself if only sync.Map ops — but field reassignment needs care under race detector |
| **Mitigation** | Prefer clear at quiet points |

## FM9 — Newline / empty-line edge mis-hunks

| | |
|--|--|
| **Trigger** | Trailing newline-only changes, empty line adds |
| **Behavior** | Split/trim logic in `diffsToOperations` may surprise |
| **Impact** | Missed or extra empty-line ops |
| **Mitigation** | Tests for empty lines / trailing newline (partial) |
| **Residual** | Subtle EOF newline cases listed in TEST_GAP |

## FM10 — Word-level API panics / empty (low)

| | |
|--|--|
| **Trigger** | Empty strings, odd Unicode |
| **Behavior** | Tests assert no panic; empty-to-empty may return empty/equal set |
| **Impact** | UI falls back to full-line styling |
| **Mitigation** | Comprehensive word-level tests |

## FM11 — Consumer applies edits without policy (integration)

| | |
|--|--|
| **Trigger** | Miswiring outside this package |
| **Behavior** | Diff package still only describes |
| **Impact** | Safety incident if apply skips `permitted` |
| **Mitigation** | Architectural boundary (this corpus); not enforceable inside package |

## Summary table

| ID | Severity | Handled? |
|----|----------|----------|
| FM1 Binary | High if unhandled | Yes in engine + UI |
| FM2 Timeout | Medium | Yes (time bound) |
| FM3 Collision | High if occurs | No |
| FM4 Shallow mutate | High if occurs | No (convention only) |
| FM5 Cache OOM | Medium long-session | Partial (manual clear) |
| FM6 Dual flags | Low | Documented |
| FM7 Empty hunks | Low | Expected |
| FM8 Clear race | Low–medium | Partial |
| FM9 Newlines | Low–medium | Partial tests |
| FM10 Word empty | Low | Tests |
| FM11 Policy bypass | Critical if elsewhere | Out of package |

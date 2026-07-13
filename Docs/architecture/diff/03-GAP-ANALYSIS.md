# 03 — Gap Analysis: `internal/diff`

> Last verified against codebase: 2026-07-13  
> Compares package vision / test TODOs / consumer needs against living `internal/diff`.

## 1. Spec vs reality matrix

| Capability | Desired | Reality | Gap? |
|------------|---------|---------|------|
| Line-level file diffs | Yes | Implemented via sergi line reduction | No |
| Hunk + context model | Yes | `groupIntoHunks` default 3 | No |
| Binary detection | Yes | NUL short-circuit | No |
| Cost bound | Yes | 5s `DiffTimeout` | Minor — no observer of timeout outcome |
| Content cache | Yes | FNV + `sync.Map` | Partial — unbounded, shallow clone |
| Word-level diffs | Yes | Returns sergi `[]Diff` | Partial — UI paints incompletely; type leak |
| Configurable context | Nice | Internal clamp only; public API fixed 3 | Yes — low priority |
| Deep clone cache hits | Correctness | Shallow struct copy | Yes — medium |
| Cache eviction | Long sessions | None | Yes — medium for long TUI sessions |
| Hash collision guard | Paranoid correctness | None | Yes — low probability, high severity if hit |
| Logging | Ops | None | Yes — low for library |
| Unified-diff I/O | Optional | None | Non-goal unless demanded |
| Kernel wiring | N/A | None | **Non-gap** |
| Mangle Decl surface | N/A | None | **Non-gap** |
| Pre-implementation package | No | Living code | **Non-gap** — do not claim 0% |

## 2. Prioritized gaps

### P1 — Correctness / safety under real use

1. **Shallow cache clone**  
   - Symptom: mutate `FileDiff.Hunks` after cache hit → corrupts later results.  
   - Fix shape: deep-copy hunks/lines on hit, or store immutable snapshots.  
   - Evidence: TEST_GAP comments in `diff_test.go`; implementation at cache hit branch.

2. **Unbounded cache growth**  
   - Symptom: long agent sessions with many unique file versions → memory pressure.  
   - Fix shape: max entries LRU, or per-engine disable, or size budget.  
   - Evidence: no eviction path in `Engine.cache`.

### P2 — API / consumer ergonomics

3. **`ComputeWordLevelDiff` returns sergi types**  
   - Couples `cmd/nerd/ui` (and any future callers) to third-party `Diff` struct.  
   - Fix shape: internal `WordSpan{Type, Text}` or reuse `Line` with char offsets.

4. **No `ComputeDiff` options**  
   - Context lines, disable-cache, force-binary flags all hardwired.  
   - Fix shape: `DiffOptions` struct with zero-value defaults matching today.

5. **`LineHeader` never produced**  
   - Dead enum for engine; UI still maps it. Harmless but slightly confusing.

### P3 — Observability & extreme tests

6. No counters for binary short-circuits / cache hits / timeouts.  
7. Remaining TEST_GAP items: collision, ClearCache race, pure-delete huge file memory, trailing-newline nuances (some partial coverage exists).

## 3. Explicit non-gaps

| Claim | Verdict |
|-------|---------|
| “Package is pre-impl / 0%” | **False** — full `diff.go` + dense tests |
| “Must integrate Mangle” | **False** — pure library by north star |
| “Must live in VirtualStore” | **False** — consumers pass strings |
| “Needs Vectryx” | **False** |
| “Missing basic add/remove tests” | **False** — extensive coverage |
| “No consumer” | **False** — `diffview.go` is load-bearing for approval UX |

## 4. Consumer-side gaps (not package bugs)

Documented in UI, not fixed by changing `internal/diff` alone:

- Full word-highlight painting still partial (`renderLineWithWordHighlights` placeholders).  
- Side-by-side view is a UI TODO.  
- Whitespace ignore filtering is UI-side (`filterHunkLines`), not in this package.

## 5. Recommended acceptance for “done enough”

For architecture corpus purposes, the package is **production-adequate**. Closing P1 items
would raise long-session robustness; they are not blockers for documenting current behavior.

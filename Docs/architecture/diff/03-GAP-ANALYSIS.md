---
doc-class: shipped-with-future
subsystem: diff
implementation-status: partial
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# 03-GAP-ANALYSIS — internal/diff

This file is `shipped-with-future`: each row pairs a shipped current state
(cited like a shipped claim, repo-relative path plus symbol and line, read
2026-09-21) with a future target state. Nothing in the Target column is
claimed as built. The shipped record lives in `02-CURRENT-STATE.md` and
`IMPLEMENTED_SPEC.md`; on any disagreement, `IMPLEMENTED_SPEC.md` wins.

Vision trace: this package serves `agents.md:43-46` — tools condense the
search space, reduce the turns to complete the task, and offload cognition
to deterministic code. The seams are `diff.ComputeDiff` at
`internal/session/turn_diff.go:62` (agent repair loop via `DefaultEngine`)
and `uiDiffEngine.ComputeDiff` at `cmd/nerd/ui/diffview.go:911` (TUI view
via `var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907`). Finished behaviour
is defined in `01-VISION.md` § "The finished behaviour" (5 items).

ID note: `01-VISION.md:69-75` lists `GAP-DIFF-01..05` as vision-level
acceptance shorthand. The buildable definitions in the matrix below share
that ID namespace and are authoritative for the build queue (R7 recursion):
where wording differs, this file wins for work ordering. A future vision
edit should reconcile its shorthand table to these rows. IDs are stable and
never deleted; closed gaps stay marked closed with their closing commit.

## Gap matrix

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-DIFF-01 | Proven cache keys | Widened `struct cacheKey` (`internal/diff/diff.go:121-129`) plus `func fingerprint` (`internal/diff/diff.go:141-157`); `method diffCache.get` (`internal/diff/cache.go:96-121`) with verify branch (`internal/diff/cache.go:109-116`) counting `Collisions` (`internal/diff/cache.go:42` / `internal/diff/cache.go:110`); `VerifyCacheContent` off-by-default (`internal/diff/diff.go:183-190`); both prod engines use zero-`Options` (`var DefaultEngine` at `internal/diff/diff.go:238` via `func ComputeDiff` at `internal/diff/diff.go:310-312` called at `internal/session/turn_diff.go:62`; `var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907` via `func NewEngine` at `internal/diff/diff.go:214-216`) | Collision cannot silently serve wrong hunks on any path where a diff is applied; `Collisions` alerts rather than logs. Spec: `01-VISION.md` finished-behaviour (4) opt-in trust; future `05-cache-trust.md` (not yet built) | Medium (High if diffs ever drive apply; today display-only) | Hardening (Phase 3) | `internal/diff/diff.go:114-157` + `internal/diff/cache.go:96-136`; decision on verify-for-apply | `go test ./internal/diff -run TestVerifyCacheContent_Collision -count=1` passes: forced key-collision asserts miss plus `Stats.Collisions==1` and entry dropped; OR ADR witness accepting trusted-key risk marked `accepted-not-implemented` |
| GAP-DIFF-02 | Binary edits visible to repair round | NUL sentinel `func containsNullByte` (`internal/diff/diff.go:24-26`) short-circuits to `IsBinary` with empty hunks (`internal/diff/diff.go:261-265`, flags at `internal/diff/diff.go:98-105`, counter `method diffCache.markBinary` at `internal/diff/cache.go:205-209`); `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) maps nil/zero-hunk to `""` (`internal/session/turn_diff.go:63-64`); no binary-marker branch witnessed in `func turnDiffSection` (`internal/session/turn_diff.go:24-52`) or `func renderFileDiff` (`internal/session/turn_diff.go:55-61`) by full-range read 2026-09-21 | Binary write produces a one-line marker (e.g. `(binary, not shown)`) so the repair round does not spend turns re-diagnosing an invisible edit. Spec: `01-VISION.md` finished-behaviour (1) + (3) | Medium | Prompt-correctness (Phase 1) | `internal/session/turn_diff.go:24-52,55-76` + `internal/diff/diff.go:261-265` + `internal/diff/cache.go:205-209` | `go test ./internal/session -run TestRenderFileDiff_BinaryMarker -count=1` passes: `renderFileDiff(binA,binB)` returns non-empty containing `binary`; `turnDiffSection` includes it instead of `""` |
| GAP-DIFF-03 | Delete note symmetric with create note | `func newFileNote` (`internal/session/turn_diff.go:78-83`) marks only `IsNew`; `IsDelete` exists on `struct FileDiff` (`internal/diff/diff.go:98-105`) and is set on empty-new-side (`internal/diff/diff.go:250-255`) with no witnessed note branch | Turn that deletes a file gets a note (`deleted by this turn`) mirroring the create note. Spec: `01-VISION.md` finished-behaviour (1) | Low | Prompt-correctness (Phase 1) | `internal/session/turn_diff.go:78-83` + `internal/diff/diff.go:250-255` | `go test ./internal/session -run TestRenderFileDiff_DeleteNote -count=1` passes: `IsDelete` `FileDiff` output asserts note string present in `renderFileDiff` result |
| GAP-DIFF-04 | Bounded repair-prompt diff budget | `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`) has no truncation symbol; `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) composes `@@` framing (`internal/session/turn_diff.go:69`) with `func diffMarker` (`internal/session/turn_diff.go:85-94`) and no budget check; `func turnDiffSection` (`internal/session/turn_diff.go:24-52`) concatenates per-file with no budget check (absence by full-range read 2026-09-21, not a symbol) | Per-file plus per-turn byte/line budget with an explicit truncation marker, so condense-search-space holds under large diffs. Spec: `01-VISION.md` finished-behaviour (1) + (3); future `05-prompt-budget.md` (not yet built) | High (unbounded concat degrades the model; breaks V3) | Prompt-budget (Phase 2) | `internal/diff/diff.go:243-307` + `internal/session/turn_diff.go:24-76`; may reuse `struct Options` tuning (`internal/diff/diff.go:162-191`) | `go test ./internal/session -run TestTurnDiffSection_Budget -count=1` passes: synthetic large diff asserts output `len <= BUDGET` and contains truncation marker; budget constant cited with symbol+line |
| GAP-DIFF-05 | Per-caller engine tuning | `struct Options` non-defaults (`internal/diff/diff.go:162-191`: `ContextLines` at `:166`, `DisableCache` at `:170`, `MaxCacheEntries` at `:173`, `MaxCacheBytes` at `:177`, `Timeout` at `:181`, `VerifyCacheContent` at `:190`) via `func NewEngineWith` (`internal/diff/diff.go:220-230`); prod builds only zero-`Options` (`var DefaultEngine` at `internal/diff/diff.go:238` used at `internal/session/turn_diff.go:62`; `var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907` equals `func NewEngineWith(Options{})` per `func NewEngine` at `internal/diff/diff.go:214-216`) | TUI vs agent loop differ in context width/timeout/cache bounds where justified, or explicitly decided not to. Spec: `01-VISION.md` finished-behaviour (2) + (3) | Low | Tuning (Phase 3+) | Both seams `internal/session/turn_diff.go:62` + `cmd/nerd/ui/diffview.go:907,911,970` | `go test ./cmd/nerd/ui -run TestEngineTuning -count=1` passes: one seam constructs `func NewEngineWith` (`internal/diff/diff.go:220-230`) with non-zero `Options` and pins behaviour; OR ADR witness marking tuning `accepted-not-implemented` |
| GAP-DIFF-06 | Cache lifecycle wired or retired | `method Engine.ClearCache` (`internal/diff/diff.go:518-520`, preserves counters) exists; no prod caller witnessed at `internal/session/turn_diff.go:62` or `cmd/nerd/ui/diffview.go:911,916,970` by full-range read 2026-09-21 (test callers not cited per citation rules) | Long sessions either clear on a lifecycle event or the LRU is documented as sufficient and `ClearCache` is marked test-only. Spec: `01-VISION.md` finished-behaviour (4) | Low | Lifecycle (Phase 3+) | Session lifecycle or explicit retire decision; `internal/diff/cache.go:191-197` `method diffCache.clear` | Either a prod call-site plus `go test ./internal/diff -run TestClearCache_Concurrent -count=1 -race` passes (clear during concurrent compute preserves counters), or doc/ADR witness retiring it as test-only |
| GAP-DIFF-07 | Cache observability wired or retired | `func DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`, delegates at `cmd/nerd/ui/diffview.go:916` to `method Engine.Stats` at `internal/diff/diff.go:233-235` over `struct Stats` at `internal/diff/cache.go:28-43`); only witnessed caller is the pinning test `func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView` (`cmd/nerd/ui/word_highlight_test.go:141-162`) | Engine `Stats` (Hits/Misses/Computes/Binary/Evicted/Entries/Bytes/Collisions) feeds a log, gate, or policy, or is declared test-only. Spec: `01-VISION.md` finished-behaviour (4) | Low | Observability (Phase 3+) | `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`) | Prod caller of `func DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`) with `go test ./cmd/nerd/ui -run TestDiffEngineStats -count=1` asserting counters advance; OR witness marking it test-only |

## Detail rows (buildable)

### GAP-DIFF-01 — Proven cache keys

- Current (shipped): widened key plus dual `fingerprint`, opt-in verify counting
  `Collisions`, both prod engines on zero-`Options` — see matrix row.
- Target: no silent wrong-hunk serve on any apply path; `Collisions` is an
  alert, not a log line.
- Exit (machine-checkable): new collision test passes
  (`go test ./internal/diff -run TestVerifyCacheContent_Collision -count=1`,
  predicate `Stats.Collisions==1` with entry dropped), or an ADR names a
  witness accepting trusted-key risk as `accepted-not-implemented`.

### GAP-DIFF-02 — Binary edits visible to repair round

- Current (shipped): binary short-circuit plus `""` mapping — see matrix row.
- Target: one-line binary marker in `renderFileDiff` / `turnDiffSection`
  output.
- Exit: `go test ./internal/session -run TestRenderFileDiff_BinaryMarker -count=1`
  passes with predicate `strings.Contains(out, "binary")`.

### GAP-DIFF-03 — Delete note symmetric with create note

- Current (shipped): `newFileNote` handles `IsNew` only — see matrix row.
- Target: mirrored delete note.
- Exit: `go test ./internal/session -run TestRenderFileDiff_DeleteNote -count=1`
  passes with predicate note-present.

### GAP-DIFF-04 — Bounded repair-prompt diff budget

- Current (shipped): no truncation or budget symbol in the two consumer
  functions — absence by full-range read, not a positive witness.
- Target: per-file and per-turn budget with truncation marker.
- Exit: `go test ./internal/session -run TestTurnDiffSection_Budget -count=1`
  passes with predicates `len(out) <= BUDGET` and marker-present; budget
  constant cited with symbol+line.

### GAP-DIFF-05 — Per-caller engine tuning

- Current (shipped): full `Options` surface exists, prod uses none of it —
  see matrix row.
- Target: justified per-caller `NewEngineWith` values, or a decision not to.
- Exit: `go test ./cmd/nerd/ui -run TestEngineTuning -count=1` passes pinning
  the non-zero construction, or an ADR witness marks tuning
  `accepted-not-implemented`.

### GAP-DIFF-06 — Cache lifecycle wired or retired

- Current (shipped): `ClearCache` exists with counter-preserving `clear`;
  unwired shape is absence-by-read, not a symbol.
- Target: lifecycle call-site or documented retire.
- Exit: prod call-site plus
  `go test ./internal/diff -run TestClearCache_Concurrent -count=1 -race`
  passes, or a doc/ADR witness retires it.

### GAP-DIFF-07 — Cache observability wired or retired

- Current (shipped): `DiffEngineStats` defined, only test caller witnessed.
- Target: prod log/gate/policy consumer or test-only declaration.
- Exit: prod caller plus
  `go test ./cmd/nerd/ui -run TestDiffEngineStats -count=1` passes asserting
  counters advance, or a witness marks it test-only.

## Closed gaps

None. Closed gaps stay listed here with their closing commit; IDs are never
reused or deleted.

## Grounded vs hypothesized

- Observed (High, opened 2026-09-21): every prod symbol+line in the Current
  column above (`internal/diff/diff.go`, `internal/diff/cache.go`,
  `internal/session/turn_diff.go:24-52,55-76,78-94`,
  `cmd/nerd/ui/diffview.go:907,910-917,970`,
  `cmd/nerd/ui/word_highlight_test.go:141-162` for the pinning test).
- Hypothesized (do not cite as shipped): vision-target wording (no `01-VISION`
  code citations by design); absence claims (no truncation symbol, no prod
  caller for `ClearCache` / `Stats` / non-default `Options`) — consistent
  with the two-prod-importer shape, proven only by full-range reads, not
  positive witnesses; every granular test-line pin beyond the two files above.

---
doc-class: governance
subsystem: diff
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# TODO — internal/diff build queue

Leaf work only. Each item traces to one `GAP-DIFF-*` row in
`03-GAP-ANALYSIS.md`, which is authoritative for work ordering (R7
recursion); where wording differs, `03-GAP-ANALYSIS.md` wins. Vision
targets live in `01-VISION.md` § "The finished behaviour". No item below
is a mega task: each is one code branch, one test, or one ADR/doc
decision with its own machine-checkable exit. IDs (`TODO-DIFF-*`) are
stable; closed items stay listed with their closing commit and are never
deleted or reused.

Priority for R7: Phase 1 (prompt-correctness) first, then Phase 2
(prompt-budget), then Phase 3 hardening/tuning/lifecycle/observability.
Severity is copied from the gap row.

## Phase 1 — Prompt-correctness

- [ ] **TODO-DIFF-02a — Return a binary marker from `renderFileDiff`.**
  Advances `GAP-DIFF-02` (Binary edits visible to repair round). In
  `func renderFileDiff` (`internal/session/turn_diff.go:55-76`), add the
  binary branch before the nil/zero-hunk guard at
  `internal/session/turn_diff.go:63-64` so an `IsBinary` `struct FileDiff`
  (`internal/diff/diff.go:98-105`, set at `internal/diff/diff.go:261-265`
  via `func containsNullByte` at `internal/diff/diff.go:24-26`) renders a
  one-line marker (e.g. `(binary, not shown)`) instead of `""`.
  Spec: `01-VISION.md` finished-behaviour (1) + (3).
  Exit: `go test ./internal/session -run TestRenderFileDiff_BinaryMarker -count=1`
  passes with predicate `strings.Contains(out, "binary")` on the
  `renderFileDiff` result.
- [ ] **TODO-DIFF-02b — Propagate the binary marker through `turnDiffSection`.**
  Advances `GAP-DIFF-02`. In `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`), include the `TODO-DIFF-02a`
  marker in the per-file concatenation instead of dropping the file
  section (today the `""` from the `63-64` guard drops it). No new budget
  logic here; that is `GAP-DIFF-04`.
  Exit: same test as `TODO-DIFF-02a` asserts `turnDiffSection` output
  contains the marker instead of `""`.
- [ ] **TODO-DIFF-03a — Mirror the create note for deletes in `newFileNote`.**
  Closes `GAP-DIFF-03` (Delete note symmetric with create note). In
  `func newFileNote` (`internal/session/turn_diff.go:78-83`), add the
  `IsDelete` branch mirroring the existing `IsNew` note, for
  `struct FileDiff` (`internal/diff/diff.go:98-105`) with `IsDelete` set
  on the empty-new-side path (`internal/diff/diff.go:250-255`).
  Spec: `01-VISION.md` finished-behaviour (1).
  Exit: `go test ./internal/session -run TestRenderFileDiff_DeleteNote -count=1`
  passes with predicate note-present in the `renderFileDiff` result.

## Phase 2 — Prompt-budget

- [ ] **TODO-DIFF-04a — Define the repair-prompt diff budget constants.**
  Advances `GAP-DIFF-04` (Bounded repair-prompt diff budget). Add named
  per-file and per-turn byte/line budget constants plus a truncation
  marker string near `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`); cite the new symbols with
  file+line once landed. Reuses `struct Options` tuning
  (`internal/diff/diff.go:162-191`) only for context-width defaults, not
  for the budget itself. No rendering change in this item.
  Exit: `go vet ./internal/session` passes and the constants are cited by
  symbol+line in `TODO-DIFF-04b`/`04c`.
- [ ] **TODO-DIFF-04b — Truncate oversized per-file diffs with a marker.**
  Advances `GAP-DIFF-04`. In `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`, framing at
  `internal/session/turn_diff.go:69` via `func diffMarker` at
  `internal/session/turn_diff.go:85-94`), enforce the `TODO-DIFF-04a`
  per-file budget and append the truncation marker on cut-off.
  Spec: `01-VISION.md` finished-behaviour (1) + (3); future
  `05-prompt-budget.md` (not yet built).
  Exit: synthetic large single-file input asserts `len(out) <= BUDGET`
  and marker-present (covered finally by `TODO-DIFF-04d`).
- [ ] **TODO-DIFF-04c — Truncate oversized per-turn concatenations.**
  Advances `GAP-DIFF-04`. In `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`), enforce the `TODO-DIFF-04a`
  per-turn budget across the per-file loop with the same truncation
  marker. No per-file logic change; that is `TODO-DIFF-04b`.
  Exit: synthetic multi-file large diff asserts `len(out) <= BUDGET` and
  marker-present (covered finally by `TODO-DIFF-04d`).
- [ ] **TODO-DIFF-04d — Pin the budget with `TestTurnDiffSection_Budget`.**
  Closes `GAP-DIFF-04`. Add the test asserting both predicates
  (`len(out) <= BUDGET`, truncation-marker present) over synthetic large
  input through `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`).
  Exit: `go test ./internal/session -run TestTurnDiffSection_Budget -count=1`
  passes.

## Phase 3 — Hardening / tuning / lifecycle / observability

- [ ] **TODO-DIFF-01a — Add a forced-collision cache test.**
  Advances `GAP-DIFF-01` (Proven cache keys). Cover `struct cacheKey`
  (`internal/diff/diff.go:121-129`) plus `func fingerprint`
  (`internal/diff/diff.go:141-157`) through `method diffCache.get`
  (`internal/diff/cache.go:96-121`) with the verify branch
  (`internal/diff/cache.go:109-116`) counting `Collisions`
  (`internal/diff/cache.go:42`): force a key collision and assert a miss
  plus `Stats.Collisions==1` with the entry dropped. Both prod engines
  use zero-`Options` (`var DefaultEngine` at `internal/diff/diff.go:238`
  via `func ComputeDiff` at `internal/diff/diff.go:310-312` called at
  `internal/session/turn_diff.go:62`; `var uiDiffEngine` at
  `cmd/nerd/ui/diffview.go:907` via `func NewEngine` at
  `internal/diff/diff.go:214-216`), so the test constructs its own engine
  with `VerifyCacheContent` (`internal/diff/diff.go:183-190`) enabled.
  Spec: `01-VISION.md` finished-behaviour (4) opt-in trust; future
  `05-cache-trust.md` (not yet built).
  Exit: `go test ./internal/diff -run TestVerifyCacheContent_Collision -count=1`
  passes.
- [ ] **TODO-DIFF-01b — Decide verify-for-apply in an ADR with a witness.**
  Closes `GAP-DIFF-01` with `TODO-DIFF-01a` (either/or exit). Write
  `adr/ADR-NNN-cache-trust.md` naming a witness (the `TODO-DIFF-01a`
  test, a `VerifyCacheContent` call-site, or a `Collisions` alert) and
  deriving status from it: either collision-safe-apply is proven, or the
  trusted-key risk is marked `accepted-not-implemented` (display-only
  today; Medium now, High if diffs ever drive apply).
  Exit: ADR exists with witness, or the `TODO-DIFF-01a` test passes as the
  proof leg of the gap exit.
- [ ] **TODO-DIFF-05a — Evaluate per-caller engine tuning values.**
  Advances `GAP-DIFF-05` (Per-caller engine tuning). Spike, no prod
  change: determine whether the TUI seam (`var uiDiffEngine` at
  `cmd/nerd/ui/diffview.go:907` via `func CreateDiffFromStrings` at
  `cmd/nerd/ui/diffview.go:910-912`) and the agent-loop seam
  (`internal/session/turn_diff.go:62` via `var DefaultEngine` at
  `internal/diff/diff.go:238`) justify different `struct Options`
  (`internal/diff/diff.go:162-191`: `ContextLines` at `:166`,
  `DisableCache` at `:170`, `MaxCacheEntries` at `:173`,
  `MaxCacheBytes` at `:177`, `Timeout` at `:181`) through
  `func NewEngineWith` (`internal/diff/diff.go:220-230`). Record the
  numbers or the no-difference finding in the `TODO-DIFF-05b` ADR/test.
  Exit: findings written down; no prod behaviour change.
- [ ] **TODO-DIFF-05b — Land one non-zero `NewEngineWith` or retire tuning.**
  Closes `GAP-DIFF-05`. Either construct `func NewEngineWith`
  (`internal/diff/diff.go:220-230`) with non-zero `Options` at one seam
  (`internal/session/turn_diff.go:62` or `cmd/nerd/ui/diffview.go:907`)
  pinned by a test, or add an ADR witness marking tuning
  `accepted-not-implemented`.
  Exit: `go test ./cmd/nerd/ui -run TestEngineTuning -count=1` passes
  pinning the non-zero construction; OR the ADR witness exists.
- [ ] **TODO-DIFF-06a — Wire `ClearCache` to a session lifecycle event.**
  Advances `GAP-DIFF-06` (Cache lifecycle wired or retired), first leg of
  an either/or exit. Add a prod call-site for
  `method Engine.ClearCache` (`internal/diff/diff.go:518-520`, preserving
  counters via `method diffCache.clear` at
  `internal/diff/cache.go:191-197`) on a session lifecycle event covering
  both seams (`internal/session/turn_diff.go:62`,
  `cmd/nerd/ui/diffview.go:911,916,970`), plus a concurrency test proving
  clear-during-compute preserves counters.
  Exit: `go test ./internal/diff -run TestClearCache_Concurrent -count=1 -race`
  passes with the prod call-site present.
- [ ] **TODO-DIFF-06b — Or retire `ClearCache` as test-only.**
  Closes `GAP-DIFF-06`, second leg of the either/or exit (do not do both).
  Add a doc/ADR witness declaring the bounded LRU sufficient and
  `method Engine.ClearCache` (`internal/diff/diff.go:518-520`) test-only.
  Exit: witness exists; `TODO-DIFF-06a` is then out of scope.
- [ ] **TODO-DIFF-07a — Feed `DiffEngineStats` into a prod consumer.**
  Advances `GAP-DIFF-07` (Cache observability wired or retired), first leg
  of an either/or exit. Add a prod caller of `func DiffEngineStats`
  (`cmd/nerd/ui/diffview.go:915-917`, delegating at
  `cmd/nerd/ui/diffview.go:916` to `method Engine.Stats` at
  `internal/diff/diff.go:233-235` over `struct Stats` at
  `internal/diff/cache.go:28-43`) that feeds a log, gate, or policy from
  `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`), plus a test
  asserting counters advance. Today the only witnessed caller is the
  pinning test `func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView`
  (`cmd/nerd/ui/word_highlight_test.go:141-162`).
  Exit: `go test ./cmd/nerd/ui -run TestDiffEngineStats -count=1` passes
  with a prod caller present.
- [ ] **TODO-DIFF-07b — Or declare engine stats test-only.**
  Closes `GAP-DIFF-07`, second leg of the either/or exit (do not do both).
  Add a witness marking `func DiffEngineStats`
  (`cmd/nerd/ui/diffview.go:915-917`) test-only.
  Exit: witness exists; `TODO-DIFF-07a` is then out of scope.

## Closed items

None. Closed items stay listed here with their closing commit; IDs are
never reused or deleted.

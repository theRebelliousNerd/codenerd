---
doc-class: shipped
subsystem: diff
implementation-status: shipped
last-verified: 2026-09-25
verified-against: c91c6b3
supersedes: []
---

# diff: wiring and what is NOT built

Verified 2026-09-25 against `c91c6b3` by reading `internal/diff/diff.go`,
`internal/diff/cache.go`, `internal/session/turn_diff.go`,
`internal/session/session_config.go`, `internal/config/session.go` and
`cmd/nerd/ui/diffview.go`, and by running the tests named below. The
2026-09-21 version of this page described `turn_diff.go` before it was
rewritten; its anchors into that file are gone.

## Wired and reachable

- **Repair-prompt diff.** `turnDiffSection`
  (`internal/session/turn_diff.go:79`) renders the turn's edits for the
  build and test repair prompts (`internal/session/build_verify.go:239,341`)
  within `ExecutorConfig.repairDiffBudget()`
  (`internal/session/session_config.go:74`), which is the session
  section's `repair_diff_file_bytes` (default 8192) and
  `repair_diff_turn_bytes` (default 24576)
  (`internal/config/session.go:54,57`). Per file, `renderFileDiff`
  (`:223`) calls `diff.ComputeDiff` and renders:
  - hunks as a ```` ```diff ```` block;
  - a binary edit as `<path> (binary: N bytes before, M after; diff not
    shown)` — it used to render nothing (`TestRenderFileDiff_BinaryMarker`);
  - a created or deleted file with `(created by this turn)` /
    `(deleted by this turn)`, from the preimage and the disk, not from
    `FileDiff.IsNew/IsDelete`, which only mean one side is empty
    (`fileNote`, `:263`; `TestRenderFileDiff_DeleteNote`). A deleted file
    used to be skipped because it could not be read back.
  `assembleTurnDiff` (`:129`) cuts a section over its budget at a line
  boundary with a `[diff truncated: N of M lines … read_file for the
  rest]` marker (`truncateSection`, `:203`), and names files past the turn
  budget in one closing line, so none goes unmentioned
  (`TestTurnDiffSection_Budget`).
- **Saved attempt patch.** `leaveBuildableTree` saves the turn's whole
  attempt with `turnDiffPatch` (`internal/session/turn_diff.go:85`,
  called at `internal/session/buildable_tree.go:51`), which is unbounded:
  it is the record a person restores from, not a prompt.
- **TUI engine.** `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`) is the
  one package engine; `DiffApprovalView.diffEngine` (`:146`) is set to it
  in `NewDiffApprovalView` (`:181`). `CreateDiffFromStrings` computes on it
  (`:911`), and the side-by-side word path uses it (`:696`, `:970`), pinned
  by `TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView`
  (`cmd/nerd/ui/word_highlight_test.go:141`).
- **Display-only importers, enforced.** `TestDiffImporters_AreDisplayOnly`
  (`internal/diff/importers_test.go`) walks the module and requires every
  production importer of `internal/diff` to be a known display-only
  consumer (today `cmd/nerd/ui` and `internal/session`). See the cache-key
  decision below.

## Exists, and stays test-only by decision

- `Options` non-defaults via `NewEngineWith`
  (`internal/diff/diff.go:162,220`): both production engines are
  zero-`Options` (`DefaultEngine` via `diff.ComputeDiff`,
  `internal/diff/diff.go:310`; `diff.NewEngine()`,
  `cmd/nerd/ui/diffview.go:907`). Neither seam has shown a need for
  different tuning; reopen with a measurement (TODO-DIFF-05).
- `(*Engine).ClearCache` (`internal/diff/diff.go:518`): the cache is a
  bounded LRU (512 entries / 32 MiB, `internal/diff/cache.go:18,23`), so no
  lifecycle event needs to clear it. Concurrency is pinned by
  `TestClearCache_ConcurrentWithComputeDiff_ShouldNotRace`
  (`internal/diff/cache_test.go:167`) (TODO-DIFF-06b).
- `DiffEngineStats` (`cmd/nerd/ui/diffview.go:915`): the counter worth
  alerting on, `Stats.Collisions` (`internal/diff/cache.go:42`), is always
  zero with verification off, which both production engines are
  (TODO-DIFF-07b).

## Assumed by the design, and the decision behind it

- **Cache keys are trusted, not proven, by default.** `fingerprint`
  (`internal/diff/diff.go:141`, dual FNV-1a plus lengths) keys the cache;
  exact verification runs on `diffCache.get`
  (`internal/diff/cache.go:96`) only with `Options.VerifyCacheContent`
  (`internal/diff/diff.go:190`). The collision path is proven by
  `TestCache_WhenVerifyEnabledAndKeyCollides_ShouldRecomputeRatherThanServeWrongDiff`
  (`internal/diff/word_span_test.go:94`). A collision in production would
  show one file's hunks for another's: a display fault, because nothing
  applies these diffs. That premise is enforced by
  `TestDiffImporters_AreDisplayOnly`; a consumer that applies diffs must
  turn verification on (TODO-DIFF-01b).

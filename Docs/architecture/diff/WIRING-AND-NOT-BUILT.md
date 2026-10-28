---
doc-class: shipped
subsystem: diff
implementation-status: shipped
last-verified: 2026-10-27
verified-against: 67bf9ee4
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
- **TUI engine.** `var uiDiffEngine` (`cmd/nerd/ui/diffview.go`) is the
  one package engine; `NewDiffApprovalView` sets `DiffApprovalView.diffEngine`
  to it, and it computes only word-level spans inside a diff the view is
  handed (`ComputeWordLevelDiff` in the inline and side-by-side paths). The
  view never computes a decision diff: it renders `approval.Request.Preview.Diff`.
  `CreateDiffFromStrings` and `DiffEngineStats` had no production caller and
  were removed (2026-10-27).
- **Decision previews, uncached.** `internal/approval` builds every approval
  preview on `previewEngine` (`internal/approval/preview.go`), an engine with
  `Options{DisableCache: true}`, pinned by
  `TestPreviewEngine_NeverServesFromCache`.
- **Importers classified, enforced.** `TestDiffImporters_AreDisplayOnlyOrDecisionSafe`
  (`internal/diff/importers_test.go`) walks the module and requires every
  production importer of `internal/diff` to be either a known display-only
  consumer (`cmd/nerd/ui`, `internal/session`) or a decision importer whose
  engine cannot serve a wrong cached diff (`internal/approval`). See the
  cache-key decision below.

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
- `Engine.Stats` outside tests: the counter worth alerting on,
  `Stats.Collisions` (`internal/diff/cache.go:42`), is always zero with
  verification off, which both caching production engines are
  (TODO-DIFF-07b). `DiffEngineStats`, its only UI wrapper, was removed with
  no caller (2026-10-27).

## Hosted: the approval view (2026-10-27)

`DiffApprovalView` now has its host. The chat TUI attaches itself as the
human-approval channel at boot (`cmd/nerd/chat/approval_host.go`,
`attachApprovalHost`) and opens the view modally while any request awaits a
decision. The view renders host-provided, immutable `approval.Request`
snapshots — summary, effects, what is not modeled, what is not shown, the
digest and the expiry — and emits `ApprovalDecisionMsg` decisions carrying the
request ID, version and digest it displayed; `approval.Host.Decide` commits
them. Approve-all approves exactly the undecided requests the person viewed,
each at the version viewed. The flow behind it is
`Docs/audits/request-bound-human-approval.md`: the constitution's consent arm
is `human_approval_granted/5`, joined to one exact `pending_action`, and the
action-type `signed_approval`/`admin_override` arm the earlier version of this
page described is gone. `/delete_file` is reachable two ways, neither of them
broad: a tracked, clean target git can restore (`file_recoverable`), and one
exact request a person approved; the action linter no longer reports it.

## Assumed by the design, and the decision behind it

- **Cache keys are trusted, not proven, by default.** `fingerprint`
  (`internal/diff/diff.go:141`, dual FNV-1a plus lengths) keys the cache;
  exact verification runs on `diffCache.get`
  (`internal/diff/cache.go:96`) only with `Options.VerifyCacheContent`
  (`internal/diff/diff.go:190`). The collision path is proven by
  `TestCache_WhenVerifyEnabledAndKeyCollides_ShouldRecomputeRatherThanServeWrongDiff`
  (`internal/diff/word_span_test.go:94`). A collision in production would
  show one file's hunks for another's: a display fault for the two caching
  engines, because neither applies or decides with a diff. The approval
  preview, which a person authorizes against, runs uncached. That split is
  enforced by `TestDiffImporters_AreDisplayOnlyOrDecisionSafe`; a consumer
  that applies or decides with diffs must disable the cache or turn
  verification on (TODO-DIFF-01b).

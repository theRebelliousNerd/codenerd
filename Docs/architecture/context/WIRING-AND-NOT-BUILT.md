# context: wiring and what is NOT built

> Re-verified 2026-09-21 (caller index + body reads; full line table in
> `.nerd/campaigns/aab9612b/artifacts/task_aab9612b_2_0.md`).
> Supersedes the 2026-09-20 pass against `456e521`.

## Wired and reachable

- Selection path: `WorkingSet.Select`
  (`internal/context/working_set.go:308-512`, previously cited as `:297`
  — wrong) funnels retrieval through control facts (`:323-324`), two-hop
  `dependency_link` traversal capped at 64 entities (cap check `:346`),
  per-entity `working_revision` (`:361`), `code_defines` /
  `code_element` queries (`:365-371`), `Candidates(ctx,entities,256)`
  (`:373-376`), missing-recents fetch (`:390-407`),
  `working_observation` / `digest` / `span` asserts (`:408-414`),
  `ReplaceControlFacts` (`:430`), and `should_include_context`
  decisions (`:455-462`) via `buildKernelDerivedContext` (`:473`).
  Whole-body reads go through `Read(ctx,r.ID,0,0)` (`:491`) against the
  `WorkingRecord` store opened by `OpenWorkingStore`
  (`internal/context/working_store.go:52-84`; structs `:27-38`).
- Tool-reachable retrieval: `RecallContextTool`
  (`internal/tools/core/context_recall.go:13-80`) dispatches Search
  (`:75`) and Recall (`:77`) and is registered in `core.RegisterAll`
  (`internal/tools/core/register.go:12`). This wires
  `WorkingSet.Search` (`working_set.go:82-84`) and `WorkingSet.Recall`
  (`:88-103`) into the production tool path.
- Context build: `Compressor.BuildContext`
  (`internal/context/compressor.go:645-742`) queries
  `should_include_context` (`:688`) with Go-activation fallback
  (`:707-712`), always includes safety facts via `getCoreFacts`
  (`:719-720`; safety list `permitted, dangerous_action,
  admin_override, security_violation, block_commit` at `:759`, query
  failures Warn+continue at `:752-765`, never silent), and renders via
  `builder.Build` (`:725-731`). Production caller:
  `retrieval.SeedIssueFacts` (`internal/retrieval/facts.go:227`).
- Package import edges (NOT re-traced this pass, carried from 09-20):
  `internal/session/working_context.go` and
  `internal/session/working_meter.go` import it as `working`;
  `cmd/nerd/chat/*` import it as `ctxcompress`;
  `cmd/nerd/cmd_context_stats.go` reads the feedback store.

## Exists but uncalled or test-only

- `Compressor.ProcessTurn` (`internal/context/compressor_turns.go:30-184`):
  the 2026-09-20 pass listed it as the wired turn path. The 2026-09-21
  caller index (16 rows) shows every caller is a `*_test.go` file; there
  are zero production callers. Marked `exists-but-uncalled`, not the turn
  path, until a production driver is traced.
- `Compressor.GetContextString` (`compressor.go:774-784`): only caller
  is `TestBuildContext` (`internal/context/compressor_test.go:164`).
  Test-only; production builds context via `BuildContext` directly.
- `WorkingSet.Select` (`working_set.go:308-512`): the caller index shows
  tests plus same-name methods on other types — the `browser` hit at
  `internal/browser/progressive_action.go:352` is rod `element.Select`,
  and `prompt.PredicateSelector.SelectFor*` are predicate selection. No
  production driver tied by package; the session-loop driver is untraced
  in this pass. Do not cite `Select` as the live path without tracing it.
- `WorkingSet.Continue` (`working_set.go:175-243`, per package outline;
  body not re-read): only callers are two tests
  (`working_set_test.go:131,136`). Dormant loop-control helper, same as
  the 09-20 finding. (`TranscriptRounds :254-266`, `SectionCeiling
  :271-283`, `RepeatThreshold :291-303` — outline only, consumers
  untraced.)
- `WorkingStore.Search` (`internal/context/working_store.go:106-134`):
  reachable via the recall tool (above), but `Select` itself uses only
  `Candidates`/`Records`/`Read`.
- `working_set.mg` (182 lines: `working_observation`,
  `working_revision`, `working_digest`, `working_span`,
  `working_recent`, `working_selected`, ...) EXISTS under
  `internal/context/` — the 09-20 claim that "no `.mg` files live under
  `internal/context/`" is wrong. Whether the engine loads it in the
  compilation scope is unverified in this pass.

## Assumed by the design, not done by the code

- Far context is dropped silently: past two hops or 64 entities,
  `Select` just `continue`s (`internal/context/working_set.go:325-359`)
  with no overflow signal.
- Kernel-derived facts bypass budget selection:
  `buildKernelDerivedContext` "never filters or reorders"
  (`internal/context/compressor_metrics.go:549-551`), so a large kernel
  answer is bounded only by the one-eighth share, not by selection.
- Persistence is assumed durable but coded best-effort: both
  `StoreCompressedState` and `LogActivation` errors are discarded
  (`internal/context/compressor_turns.go:168-181`).
- `UnmarshalCompressedState` validates nothing beyond JSON shape — no
  version or migration check
  (`internal/context/serializer.go:599-605`).
- Counts are broker estimates, not measurements; `Confidence()` and
  `Ratio()` (`internal/context/tokens.go:58-60,63`) let a caller check
  calibration, but no caller in this package does.
- The feedback loop is half-wired: `SetFeedbackStore`
  (`internal/context/compressor.go:543`) and `computeFeedbackScore`
  (`internal/context/activation_scoring.go:454`) both exist, but the
  end-to-end path from stored feedback into a selection decision was not
  traced in this pass — flagged, not asserted.
- The rules behind `working_selected` / `should_include_context`
  (`internal/context/working_set.go:433-462`) are decided where
  `working_set.mg` is loaded; that load site was not verified here.

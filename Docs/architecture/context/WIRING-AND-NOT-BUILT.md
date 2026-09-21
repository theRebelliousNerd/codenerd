# context: wiring and what is NOT built

> Verified 2026-09-20 against `456e521` (`main`).

## Wired and reachable

- Turn path: `ProcessTurn` commits atoms (`kernel.AssertBatch`), marks
  recency (`activation.MarkNewFacts`), recalculates the budget, and
  persists state plus activation analytics
  (`internal/context/compressor_turns.go:58-181`).
- Selection path: `Select` reads observations through
  `store.Candidates` / `store.Records` / `store.Read`
  (`internal/context/working_set.go:362,386,480`) against the
  `WorkingRecord` store opened by `OpenWorkingStore`
  (`internal/context/working_store.go:27-84`).
- Package import edges (search-verified; call sites not traced in this
  pass): `internal/session/working_context.go` and
  `internal/session/working_meter.go` import it as `working`;
  `cmd/nerd/chat/*` import it as `ctxcompress`;
  `cmd/nerd/cmd_context_stats.go` reads the feedback store.

## Exists but the read paths do not call

- `WorkingStore.Search` (`internal/context/working_store.go:106-134`):
  `Select` uses only `Candidates`/`Records`/`Read`. External callers, if
  any, were not traced in this pass.
- `Continue`, `TranscriptRounds`, `SectionCeiling`, `RepeatThreshold`
  (`internal/context/working_set.go:170-292`): loop-control helpers
  `Select` never consults; the consumer is presumably the session meter
  above, untraced here.

## Assumed by the design, not done by the code

- Far context is dropped silently: past two hops or 64 entities,
  `Select` just `continue`s (`internal/context/working_set.go:314-337`)
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
  `Ratio()` (`internal/context/tokens.go:55-63`) let a caller check
  calibration, but no caller in this package does.
- The feedback loop is half-wired: `SetFeedbackStore`
  (`internal/context/compressor.go:543`) and `computeFeedbackScore`
  (`internal/context/activation_scoring.go:454`) both exist, but the
  end-to-end path from stored feedback into a selection decision was not
  traced in this pass — flagged, not asserted.
- The rules behind `working_selected` / `should_include_context`
  (`internal/context/working_set.go:419-444`) are not in this package:
  no `.mg` files live under `internal/context/`, so the engine's
  definitions were not verified here.

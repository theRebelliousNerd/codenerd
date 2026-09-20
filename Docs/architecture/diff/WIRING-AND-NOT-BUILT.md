# diff: wiring and what is NOT built

Verified 2026-09-20 against commit `231cfa7` (`main`). Read from
`internal/diff/diff.go`, `internal/diff/cache.go`,
`internal/session/turn_diff.go`, and `cmd/nerd/ui/diffview.go`.

## Wired and reachable

- Turn prompt: `renderFileDiff` in `internal/session/turn_diff.go:55-76` calls
  `diff.ComputeDiff` (`internal/session/turn_diff.go:62`) and renders hunks as
  `@@ -a,b +c,d @@` itself (`internal/session/turn_diff.go:69`). The engine
  never formats headers; composition is the caller's job.
- TUI: `cmd/nerd/ui/diffview.go:181` wires `uiDiffEngine` into the diff view;
  `CreateDiffFromStrings` (`cmd/nerd/ui/diffview.go:909-912`) and
  `DiffEngineStats` (`cmd/nerd/ui/diffview.go:914-916`) share that one engine,
  pinned by `cmd/nerd/ui/word_highlight_test.go:141-159`.
- `LineHeader` is referenced by the UI layer as the type of header rows it
  synthesizes; the engine never produces one
  (`internal/diff/diff.go:48-56`,
  `TestComputeDiff_WhenAnyInput_ShouldNeverEmitLineHeader`).

## Exists but nothing in production calls

- `NewEngineWith` with non-zero `Options`: both production call sites use
  defaults — `diff.ComputeDiff` runs on `DefaultEngine`
  (`internal/diff/diff.go:310-312`) and the UI calls `diff.NewEngine()`
  (`cmd/nerd/ui/diffview.go:907`). Non-default `ContextLines`, `Timeout`,
  `MaxCacheEntries`, `MaxCacheBytes`, `DisableCache`, and
  `VerifyCacheContent` appear only in `internal/diff/cache_test.go`,
  `internal/diff/benchmark_test.go:23-36`, and
  `internal/diff/word_span_test.go:95`.
- `(*Engine).ClearCache` (`internal/diff/diff.go:518-520`): called only from
  tests (`internal/diff/cache_test.go:191-208`,
  `internal/diff/diff_comprehensive_test.go:288`,
  `internal/diff/diff_test.go:200`).
- `DiffEngineStats` (`cmd/nerd/ui/diffview.go:914-916`): the only caller is
  the single-engine pinning test
  (`cmd/nerd/ui/word_highlight_test.go:150-152`).
- `Stats.Collisions` (`internal/diff/cache.go:42`): permanently zero unless
  `VerifyCacheContent` is on, which no production engine enables — the
  counter currently has no production writer.

## Assumed by the design, not done by the code

- Cache keys are trusted, not proven, by default. A fingerprint collision
  would serve one file's hunks as another's with no error; exact verification
  is opt-in (`VerifyCacheContent`, `internal/diff/diff.go:183-190`) and off.
- Binary input yields zero hunks (`IsBinary=true`,
  `internal/diff/diff.go:261-265`), so `renderFileDiff` returns `""`
  (`internal/session/turn_diff.go:62-64`): a binary edit is silently absent
  from the repair prompt, not flagged.
- `newFileNote` marks only `IsNew` (`internal/session/turn_diff.go:78-83`);
  a turn that deletes a file gets no note.
- The repair prompt assumes the diff fits: there is no truncation or size
  budget between `ComputeDiff` and the prompt builder.

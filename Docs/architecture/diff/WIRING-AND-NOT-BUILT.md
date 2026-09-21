# diff: wiring and what is NOT built

Verified 2026-09-20 against commit `231cfa7` (`main`). Read from
`internal/diff/diff.go`, `internal/diff/cache.go`,
`internal/session/turn_diff.go`, and `cmd/nerd/ui/diffview.go`.

## Wired and reachable

- Turn prompt entry: `turnDiffSection` (`internal/session/turn_diff.go:24-52`)
  renders the turn's edits file by file, returning `""` when nothing was
  written or no preimage was recorded.
- Turn prompt per file: `renderFileDiff` (`internal/session/turn_diff.go:55-76`)
  short-circuits identical content (`internal/session/turn_diff.go:59-61`),
  calls `diff.ComputeDiff` (`internal/session/turn_diff.go:62`), returns `""`
  on nil/zero-hunk (`internal/session/turn_diff.go:63-64`), composes
  `@@ -%d,%d +%d,%d @@` itself (`internal/session/turn_diff.go:69`) with
  `diffMarker` (`internal/session/turn_diff.go:71`, defined at
  `internal/session/turn_diff.go:85-94`), and appends `newFileNote`
  (`internal/session/turn_diff.go:78-83`, marks only `IsNew`).
- TUI engine: `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`, rationale
  `cmd/nerd/ui/diffview.go:900-906`) is the one package engine; view struct
  `DiffApprovalView` (`cmd/nerd/ui/diffview.go:132-151`) holds it as field
  `diffEngine` (`cmd/nerd/ui/diffview.go:146`) wired in `NewDiffApprovalView`
  (`cmd/nerd/ui/diffview.go:164-212`) at `cmd/nerd/ui/diffview.go:181`.
- TUI calls: `CreateDiffFromStrings` (`cmd/nerd/ui/diffview.go:910-912`)
  calls `uiDiffEngine.ComputeDiff` (`cmd/nerd/ui/diffview.go:911`);
  `DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`) calls
  `uiDiffEngine.Stats()` (`cmd/nerd/ui/diffview.go:916`); side-by-side word
  path calls `d.diffEngine.ComputeWordLevelDiff`
  (`cmd/nerd/ui/diffview.go:970`), pinned by
  `TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView`
  (`cmd/nerd/ui/word_highlight_test.go:141-162`, `Computes` routing check
  `cmd/nerd/ui/word_highlight_test.go:154-156`).
- `LineHeader` contract: `LineContext/LineAdded/LineRemoved/LineHeader`
  (`internal/diff/diff.go:41-56`, never-emit comment
  `internal/diff/diff.go:48-56`); framing lives in `Hunk`
  (`internal/diff/diff.go:89-95`), rendering in the callers above.

## Exists but nothing in production calls

- `Options` non-defaults (`internal/diff/diff.go:162-191`: `ContextLines:166`,
  `DisableCache:170`, `MaxCacheEntries:173`, `MaxCacheBytes:177`, `Timeout:181`,
  `VerifyCacheContent:190`) via `NewEngineWith`
  (`internal/diff/diff.go:220-230`): production builds engines only as
  `DefaultEngine` (`internal/diff/diff.go:238`, via `diff.ComputeDiff`
  `internal/diff/diff.go:310-312` called at `internal/session/turn_diff.go:62`)
  and `diff.NewEngine()` (`cmd/nerd/ui/diffview.go:907`, which is
  `NewEngineWith(Options{})` per `internal/diff/diff.go:214-216`) — both
  zero-`Options`, so non-default tuning has no witnessed production caller.
  Test-only caller lines were not re-opened this turn and are not cited per
  Rule 1a.
- `(*Engine).ClearCache` (`internal/diff/diff.go:518-520`): existence
  witnessed; no caller witnessed at either production seam
  (`internal/session/turn_diff.go:62`,
  `cmd/nerd/ui/diffview.go:911/916/970`). Test caller lines were not
  re-opened this turn and are not cited per Rule 1a.
- `DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`,
  `uiDiffEngine.Stats()` at `cmd/nerd/ui/diffview.go:916`): the witnessed
  production callers on that engine are `CreateDiffFromStrings`
  (`cmd/nerd/ui/diffview.go:910-912` via `:911`) and the word path
  (`cmd/nerd/ui/diffview.go:970`); the only witnessed `DiffEngineStats`
  caller is the single-engine pinning test
  (`cmd/nerd/ui/word_highlight_test.go:141-162`, before/call/after at
  `:150/:151/:152`, `Computes` routing check `:154-156`).
- `Stats.Collisions` (`internal/diff/cache.go:42`, struct `Stats`
  `internal/diff/cache.go:28-43`): counted on the verified `get` path
  (`internal/diff/cache.go:96-121`); production engines run with
  `VerifyCacheContent` off (`internal/diff/diff.go:183-190`, both prod
  engines zero-`Options` per above), so no production writer witnessed.

## Assumed by the design, not done by the code

- Cache keys are trusted, not proven, by default: `struct cacheKey`
  (`internal/diff/diff.go:114-129`, two hashes + both lens + contextLines)
  built by `fingerprint` (`internal/diff/diff.go:141-157`, dual FNV-1a);
  exact verification runs only on the `diffCache.get` path
  (`internal/diff/cache.go:96-121`, verify `internal/diff/cache.go:109-116`,
  counts `Collisions` at `internal/diff/cache.go:110`) when
  `Options.VerifyCacheContent` (`internal/diff/diff.go:183-190`,
  off-by-default, doubles memory) is on — and both production engines run
  zero-`Options` (`internal/diff/diff.go:238` via
  `internal/diff/diff.go:310-312` at `internal/session/turn_diff.go:62`;
  `cmd/nerd/ui/diffview.go:907` via `internal/diff/diff.go:214-216`), so a
  collision would serve one file's hunks as another's with no error.
- Binary input yields zero hunks and is silently absent from the repair
  prompt: NUL sentinel `containsNullByte` (`internal/diff/diff.go:24-26`) →
  `IsBinary=true`, `markBinary()` (`internal/diff/cache.go:205-209`,
  `Stats.Binary` at `internal/diff/cache.go:32`), empty hunk list
  (`internal/diff/diff.go:261-265`, flags `struct FileDiff`
  `internal/diff/diff.go:98-105`) → `renderFileDiff` nil/zero-hunk guard
  (`internal/session/turn_diff.go:63-64`) returns `""`. No binary branch was
  witnessed in `turnDiffSection` (`internal/session/turn_diff.go:24-51`) or
  `renderFileDiff` (`internal/session/turn_diff.go:55-61` identical
  short-circuit) beyond that guard.
- `newFileNote` marks only `IsNew` (`internal/session/turn_diff.go:78-83`);
  `IsDelete` exists (`struct FileDiff` `internal/diff/diff.go:98-105`, set at
  `internal/diff/diff.go:250-255`) but has no witnessed note branch, so a
  turn that deletes a file gets no note.
- The repair prompt assumes the diff fits: `Engine.ComputeDiff`
  (`internal/diff/diff.go:243-307`) has no truncation symbol; `renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) composes `@@ -%d,%d +%d,%d @@`
  itself (`internal/session/turn_diff.go:69`) with `diffMarker`
  (`internal/session/turn_diff.go:85-94`) and no budget check; `turnDiffSection`
  (`internal/session/turn_diff.go:24-52`) concatenates per-file sections with
  no budget check — absence witnessed by full-range reads of those ranges,
  not by a symbol, so this stays a design gap until a budget or test pins it.
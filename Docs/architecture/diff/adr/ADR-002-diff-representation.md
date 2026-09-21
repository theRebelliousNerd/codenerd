---
doc-class: governance
subsystem: diff
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# ADR-002: Diff representation is FileDiff/Hunk/Line with UI-owned framing and own-typed word spans

## Context

`internal/diff` serves two callers that render independently: the agent repair
round, which is shown what the turn changed, and the TUI approval view. Both
need hunk framing as data they compose themselves — never as embedded `@@`
rows that would shift every consumer's line arithmetic — and both need
word-level runs without importing the third-party diff library to interpret
them. The repair-round rationale records word-level highlighting sitting
unimplemented while the UI typed the parameter as `any`
(`internal/diff/diff.go:70-75`).

## Decision

The engine returns `FileDiff` → `Hunk` → `Line`, with framing in the `Hunk`
counters and rendering in the caller:

- `FileDiff{OldPath, NewPath, Hunks, IsNew, IsDelete, IsBinary}`
  (`internal/diff/diff.go:98-105`) → `Hunk{OldStart, OldCount, NewStart,
  NewCount, Lines}` (`internal/diff/diff.go:89-95`) → `Line{LineNum,
  Content, Type}` (`internal/diff/diff.go:82-86`), where `Type` is one of
  `LineContext` / `LineAdded` / `LineRemoved` (`internal/diff/diff.go:43-46`).
- `LineHeader` stays in the enum but the engine never emits it: hunk framing
  lives in the `Hunk` fields, so a renderer composes its own
  `"@@ -a,b +c,d @@"` (`internal/diff/diff.go:48-56`). The repair round does
  exactly that (`internal/session/turn_diff.go:69`, with `diffMarker`
  `internal/session/turn_diff.go:85-94` mapping added to `+` and removed
  to `-`). Treat `LineHeader` as the UI-owned member of the enum; the engine
  emitting one is a bug.
- Word-level comparison returns `[]WordSpan{Type, Text}`
  (`internal/diff/diff.go:76-79`), with `Type` in `SpanEqual` / `SpanDelete` /
  `SpanInsert` (`internal/diff/diff.go:62-66`), computed per visible line pair
  by `Engine.ComputeWordLevelDiff` (`internal/diff/diff.go:530-551`, package
  wrapper `:554-556`) and deliberately uncached. No consumer imports
  `sergi/go-diff` to read the result; the side-by-side TUI path consumes spans
  directly (`cmd/nerd/ui/diffview.go:970`).

## Consequences

- Both use-sites share one representation: the agent loop via package
  `diff.ComputeDiff` (`internal/session/turn_diff.go:62`) and the TUI via
  `uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`, called at `:911`).
- Renderers own their framing rows and can mix synthesized header rows into
  their own line lists without the engine's line counts drifting.
- Numbering note: ADR-001's consequences line names a future "ADR-002 bounded
  cost"; that pointer predates this numbering and is stale — the bounded-cost
  timeout/context decision (D2) should take the next free ADR number.

## Witness

Status is derived from these witnesses per exemplar ADR-014
(`Docs/journeys/09-architecture-doc-standard.md:82-88`): each must resolve
by opening the file, finding the symbol, and confirming the line. If any
witness below did not resolve, status would be `accepted-not-implemented`.

Code witnesses (opened 2026-09-21):

- `internal/diff/diff.go:82-86` — `struct Line` (`LineNum`, `Content`,
  `Type`).
- `internal/diff/diff.go:89-95` — `struct Hunk` (`OldStart`/`OldCount`/
  `NewStart`/`NewCount`, `Lines`).
- `internal/diff/diff.go:98-105` — `struct FileDiff` (`OldPath`, `NewPath`,
  `Hunks`, `IsNew`/`IsDelete`/`IsBinary`).
- `internal/diff/diff.go:43-57` — `LineContext`/`LineAdded`/`LineRemoved` at
  `:44/45/46` plus the never-emit contract at `:48-56`.
- `internal/diff/diff.go:62-79` — `SpanEqual`/`SpanDelete`/`SpanInsert` at
  `:63/64/65`, `struct WordSpan` at `:76-79`, rationale comment at `:70-75`.
- `internal/diff/diff.go:530-551` — `Engine.ComputeWordLevelDiff`
  (`DiffMain` + `DiffCleanupSemantic` at `:531-532`); package wrapper at
  `:554-556`.
- `internal/session/turn_diff.go:69` — repair round composes
  `@@ -%d,%d +%d,%d @@` itself from the `Hunk` counters; `diffMarker` at
  `:85-94`.
- `cmd/nerd/ui/diffview.go:970` — side-by-side word path calls
  `d.diffEngine.ComputeWordLevelDiff(line.Content, nextLine.Content)`.

Test witnesses (opened 2026-09-21):

- `internal/diff/word_span_test.go:64-92` —
  `TestComputeDiff_WhenAnyInput_ShouldNeverEmitLineHeader` drives seven
  inputs (insert/delete/replace/multi-hunk/empty-old/empty-new/
  trailing-newline at `:69-78`) through `NewEngine().ComputeDiff` (`:82`)
  and fails on any `LineHeader` (`:85-86`). Proves the never-emit contract.
- `internal/diff/word_span_test.go:8-46` —
  `TestComputeWordLevelDiff_WhenLinesDiffer_ShouldReturnCodeNERDSpans`
  rebuilds both sides exactly from delete+equal / insert+equal spans
  (`:18-42`) and requires both a delete and an insert span (`:43-45`).
  Proves spans carry the full line content in old-then-new reading order.
- `internal/diff/diff_test.go:292-314` — `TestComputeWordLevelDiff` calls
  `engine.ComputeWordLevelDiff` (`:297`) and asserts the `brown` → `red`
  change is detected (`:304-313`). Proves the word path executes.

## Status

`shipped` — all code and test witnesses above resolve at
`verified-against: 231cfa7` (flag drift if the commit moved; code re-read
2026-09-21). Closes no gap; parents V1 (offload cognition to deterministic
code: one shared representation for both use-sites).

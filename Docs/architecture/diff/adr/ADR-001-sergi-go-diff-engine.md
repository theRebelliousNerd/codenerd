---
doc-class: governance
subsystem: diff
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# ADR-001: Diff computation uses sergi/go-diff, not hand-rolled LCS or LLM edits

## Context

`internal/diff` must produce line diffs for two callers: the agent repair
round, which is shown what the turn changed, and the TUI approval view.
A hand-rolled LCS is a defect surface; LLM hand-editing diffs breaks the
north-star rule that a change with a blast radius is carried out by the tool
(`agents.md:43-46` via task_b4eeb4a4_1_0). The repair-round rationale records
ladder runs R1-12/R1-15 spending three attempts re-diagnosing a failure
because the round never saw its own edits
(`internal/session/turn_diff.go:12-19`, comment on `turnDiffSection`).

## Decision

Offload all diff computation to the deterministic third-party engine
`sergi/go-diff` (`diffmatchpatch`), driven through `Engine.ComputeDiff`
with a line-level reduction (`DiffLinesToChars` / `DiffMain` /
`DiffCleanupSemantic` / `DiffCharsToLines`).

## Consequences

- Diffs are deterministic Go, shared by both use-sites: the agent loop via
  package `diff.ComputeDiff` (`internal/session/turn_diff.go:62`,
  i.e. `DefaultEngine`) and the TUI via `uiDiffEngine`
  (`cmd/nerd/ui/diffview.go:907`, called at `:911`).
- Correctness of line splitting, semantic cleanup, and hunk grouping is
  inherited from the library; `internal/diff` owns only conversion to line
  operations and hunk grouping (`Engine.diffsToOperations`,
  `internal/diff/diff.go:343-400`; `Engine.groupIntoHunks`,
  `internal/diff/diff.go:403-486`).
- Cost and trust bounds are separate decisions (bounded cost, next free ADR
  number — the "ADR-002" label here predates ADR-002-diff-representation.md;
  ADR-005/ADR-006 cache key and trust-by-default).

## Witness

Status is derived from these witnesses per exemplar ADR-014
(`Docs/journeys/09-architecture-doc-standard.md:82-88`): each must resolve
by opening the file, finding the symbol, and confirming the line. If any
witness below did not resolve, status would be `accepted-not-implemented`.

Code witnesses (opened 2026-09-21):

- `internal/diff/diff.go:9` — `import diffmatchpatch
  "github.com/sergi/go-diff/diffmatchpatch"` (only third-party import).
- `internal/diff/diff.go:243-307` — `Engine.ComputeDiff`; engine chain at
  `:293-296` (`DiffLinesToChars` / `DiffMain(a, b, false)` /
  `DiffCleanupSemantic` / `DiffCharsToLines`).
- `internal/session/turn_diff.go:62` — `fd := diff.ComputeDiff(path, path,
  before, after)` (agent-loop seam on `DefaultEngine`).
- `cmd/nerd/ui/diffview.go:907` — `var uiDiffEngine = diff.NewEngine()`
  (one engine for the UI package).
- `cmd/nerd/ui/diffview.go:911` — `uiDiffEngine.ComputeDiff` inside
  `CreateDiffFromStrings` (`:910-912`).

Test witness (opened 2026-09-21):

- `internal/diff/diff_test.go:13-44` —
  `TestComputeDiff_SimpleAddition` constructs `NewEngine()` and calls
  `engine.ComputeDiff` at `:18`, asserting one hunk and the added line
  `line2.5` (`:24-43`). Proves the engine path above executes.

## Status

`shipped` — all code and test witnesses above resolve at
`verified-against: 231cfa7` (flag drift if the commit moved; code re-read
2026-09-21). Closes no gap; parents GAP-DIFF-01 context and V1
(offload cognition to deterministic code).

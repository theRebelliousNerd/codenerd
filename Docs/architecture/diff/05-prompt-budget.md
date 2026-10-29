---
doc-class: shipped-with-future
subsystem: diff
implementation-status: target-state
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# 05 Prompt Budget — bounded repair-prompt diffs

This file is `shipped-with-future` with `implementation-status: target-state`.
Shipped seams below are claims the code bears out today, each cited with
repo-relative path plus symbol and line read 2026-09-21. Nothing in the
Target, Predicates, Failure-modes-planned, or Proving-tests-planned sections
is claimed as built: those sections use future tense throughout (`would`,
`will`, `proposed`) per plan-layer Rule 2. On any disagreement, the shipped
record in `IMPLEMENTED_SPEC.md` wins.

Scope (Rule 3): `06-CALLER-INTEGRATION.md` answers how the engine attaches to
its two callers (seams and data flow). This file answers one narrower
question: how the agent-repair-loop rendering would stay within a fixed
prompt budget. TUI rendering is out of scope here and stays as specified in
`06-CALLER-INTEGRATION.md` §2.2; no TUI budget behaviour is proposed.

## 1. Vision trace

This capability would serve one sentence of the project vision (`agents.md`,
"The Vision", Steve 2026-09-18, `agents.md:43-46`):

> Tools exist for exactly three things: condense the search space, reduce the
> turns to complete the task, and offload cognition to deterministic code
> (a change with a blast radius is carried out by the tool, not by the
> LLM hand-editing).

The budget serves the first clause directly ("condense the search space"):
the repair round must see a condensed view of its own edits, never an
unbounded dump. It is the V3 target from the gap derivation
(`.nerd/campaigns/b4eeb4a4/artifacts/task_b4eeb4a4_3_0.md`, V3 prompt stays
bounded) and finished-behaviour (3) in `01-VISION.md` §"The finished
behaviour" (large/hostile inputs stay bounded). The anchor seam is the agent
loop call `fd := diff.ComputeDiff(path, path, before, after)` at
(`internal/session/turn_diff.go:62`), which runs on the `DefaultEngine`
convenience wrapper `func ComputeDiff`
(`internal/diff/diff.go:310-312`) over `var DefaultEngine`
(`internal/diff/diff.go:238`).

## 2. Shipped seams (present tense, cited)

The agent-loop rendering path today is unbounded at both levels:

- `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`) has no
  truncation symbol: on miss it runs `DiffLinesToChars/DiffMain/
  DiffCleanupSemantic/DiffCharsToLines` (`internal/diff/diff.go:293-296`),
  converts via `method Engine.diffsToOperations`
  (`internal/diff/diff.go:343-400`) and groups via
  `method Engine.groupIntoHunks` (`internal/diff/diff.go:403-486`), returning
  the full hunk list. Engine-level bounded cost exists only for time and
  context width (`const diffTimeout` at `internal/diff/diff.go:14`,
  `func clampContextLines` at `internal/diff/diff.go:30-38`,
  `method Options.timeout` at `internal/diff/diff.go:202-211`), not for
  output size.
- `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) composes
  `@@ -%d,%d +%d,%d @@` (`internal/session/turn_diff.go:69`) with
  `func diffMarker` (`internal/session/turn_diff.go:85-94`) and contains no
  budget-check symbol (absence by full-range read 2026-09-21, not a symbol).
- `func turnDiffSection` (`internal/session/turn_diff.go:24-52`)
  concatenates one `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) block per file and contains no
  budget-check symbol (absence by full-range read 2026-09-21, not a symbol).
  Identical content short-circuits (`internal/session/turn_diff.go:59-61`);
  nil or zero-hunk results map to `""`
  (`internal/session/turn_diff.go:63-64`).

The existing per-shape types that a budget would operate on are
`struct FileDiff` (`internal/diff/diff.go:98-105`),
`struct Hunk` (`internal/diff/diff.go:89-95`), and
`struct Line` (`internal/diff/diff.go:82-86`).

## 3. Target state (future tense — not built)

When this capability is finished, the agent seam would enforce two nested
budgets with an explicit truncation marker:

- A per-file output cap would bound what one
  `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) block may
  contribute.
- A per-turn output cap would bound what
  `func turnDiffSection` (`internal/session/turn_diff.go:24-52`) concatenates
  across files.
- Any truncation would append a machine-checkable marker line (for example a
  line containing `... [truncated N lines/N bytes] ...`) so the model can see
  that output was cut rather than silently reasoning over a partial diff.
- The binary marker proposed for GAP-DIFF-02 and the delete note proposed
  for GAP-DIFF-03 would be exempt from truncation: a truncated file block
  would still carry its one-line `(binary, ...)` or delete-note line, so
  budgeting would never re-hide a signal another gap made visible.

No budget constant, truncation function, or marker string exists today; none
is cited with a line. The first implementation would add them with
symbol+line citations in the proving test below.

## 4. Data shapes (proposed — do not exist today)

The following Go shapes are proposed and would live beside the existing
`struct FileDiff` (`internal/diff/diff.go:98-105`) seam types. They are
sketched here so a future implementation has a fixed contract; no symbol+line
is claimed because no such symbol exists.

```go
// Proposed — does not exist today. Names are illustrative; the shipped
// implementation would cite its real symbol+line in the proving test.
type TurnDiffBudget struct {
    MaxBytesPerFile int    // hard cap on one renderFileDiff block
    MaxBytesPerTurn int    // hard cap on one turnDiffSection result
    MaxLinesPerFile int    // companion line cap so wide-char content is bounded too
    TruncationMarker string // e.g. "... [turn-diff truncated: %d lines, %d bytes omitted] ..."
}

type TurnDiffOutcome struct {
    Text         string // budgeted prompt text (already marker-terminated on cut)
    Truncated    bool   // whether any per-file or per-turn cap fired
    DroppedBytes int    // bytes omitted by truncation (for the Mangle fact below)
    DroppedFiles []string // paths that were cut or skipped entirely
}
```

Enforcement order would be: render each file block, cut it to the per-file
cap with the marker, then concatenate files until the per-turn cap fires,
cutting with the marker and recording the skipped paths. Existing copy
semantics would be preserved: truncation would operate on the owned
`method FileDiff.Clone` (`internal/diff/cache.go:229-246`) values the cache
already hands out, never on cached backing arrays.

## 5. Predicates (proposed — do not exist today)

No Mangle predicate for prompt budgeting exists today. The following
declarations are proposed so the kernel can hold the budget as policy and
observe when it fires. Per project rules every predicate needs a `Decl`
before use, variables are UPPERCASE, atoms are `/lowercase`, and every
numeric slot is `/number` (int64 only — ratios would be scaled before they
reach a fact). Sketched declarations the implementation would add to the
schema (names illustrative):

```mangle
// Proposed — not present in any schemas file today.
Decl turn_diff_budget(MaxBytesPerFile, MaxBytesPerTurn, MaxLinesPerFile) bound [/number, /number, /number].
Decl turn_diff_truncated(TurnId, DroppedBytes) bound [/string, /number].
```

Intended meaning: `turn_diff_budget/3` would be seeded from Go or session
config as the standing policy for the repair loop; `turn_diff_truncated/2`
would be projected by the Go seam after each turn that truncates, so policy
can count how often the budget fires (repeated truncation is a signal the
edit strategy, not just the renderer, needs attention). Both arities are
part of the proposal: a `Decl` at a different arity would be a different
predicate and would break the join, per the Decl-contract rule.

## 6. Go–Mangle split

| Concern | Owner | What would happen |
|---|---|---|
| Deterministic truncation, marker text, per-file then per-turn accounting | Go (`internal/session/turn_diff.go:24-52,55-76` seam) | The seam would cut bytes/lines, append the marker, and return the `TurnDiffOutcome` shape above. All string surgery stays in Go, beside `func diffMarker` (`internal/session/turn_diff.go:85-94`), never in policy. |
| Budget values as policy | Mangle (`turn_diff_budget/3`, proposed §5) | The kernel would own the numbers (per-file, per-turn, lines) so tuning is a policy edit with a witness, not a scattered constant change. Go would read the resolved values at the seam. |
| Truncation observation | Mangle (`turn_diff_truncated/2`, proposed §5) | Go would project one fact per truncated turn; policy rules (alerting, retry-hinting) would join on it. The cache counters in `struct Stats` (`internal/diff/cache.go:28-43`) stay engine-lifetime telemetry and are not the truncation signal. |
| Cache and compute | Unchanged engine | `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`) and `method diffCache.get` (`internal/diff/cache.go:96-121`) / `method diffCache.put` (`internal/diff/cache.go:126-172`) keep current behaviour; budgeting applies at rendering, not at compute or cache. |

What would NOT cross the seam: Mangle would never construct diff text, and
Go would never decide budget policy. The seam stays the existing call at
(`internal/session/turn_diff.go:62`) plus a budget read before rendering and
one fact projection after a truncating turn.

## 7. Failure modes

Shipped behaviour in each row is cited; planned handling is future tense and
ends in its gap ID.

### FM-BUDGET-01 — unbounded concat floods the repair prompt (GAP-DIFF-04)

- Shipped: the ranges `method Engine.ComputeDiff`
  (`internal/diff/diff.go:243-307`), `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`), and `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`) contain no truncation or budget
  symbol (absence by full-range read, not a symbol). A large edit therefore
  passes through the agent seam (`internal/session/turn_diff.go:62`)
  unbounded, breaking V3 (prompt stays bounded).
- Planned: per-file plus per-turn caps with the §3 marker would bound output
  — see GAP-DIFF-04.

### FM-BUDGET-02 — truncation hides the binary/delete signal (GAP-DIFF-02/03 interaction)

- Shipped: binary short-circuits to `IsBinary` with empty hunks via
  `func containsNullByte` (`internal/diff/diff.go:24-26`) at
  (`internal/diff/diff.go:261-265`), and `func newFileNote`
  (`internal/session/turn_diff.go:78-83`) marks only `IsNew` on
  `struct FileDiff` (`internal/diff/diff.go:98-105`).
- Planned: the one-line binary marker (GAP-DIFF-02) and the symmetric delete
  note (GAP-DIFF-03) would be truncation-exempt per §3, so budgeting would
  never re-hide the signals those gaps make visible. This file does not
  specify the marker/note text; GAP-DIFF-02/GAP-DIFF-03 own it.

### FM-BUDGET-03 — per-file cap without per-turn cap still floods on many files (GAP-DIFF-04)

- Shipped: `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`) concatenates one block per file
  with no total-cap symbol (absence by read, not a symbol), so N small files
  can jointly exceed any single-file bound.
- Planned: the per-turn cap in §3 would fire after the per-file caps, with
  skipped paths recorded in `DroppedFiles` (§4) — see GAP-DIFF-04.

### FM-BUDGET-04 — silent cut misleads the model into reasoning over partial hunks (GAP-DIFF-04)

- Shipped: `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) maps nil/zero-hunk to `""`
  (`internal/session/turn_diff.go:63-64`) with no marker vocabulary for cuts.
- Planned: every cut would terminate in the §3 marker containing the omitted
  counts, and each truncating turn would project `turn_diff_truncated/2`
  (§5), so silence is never the truncation signal — see GAP-DIFF-04.

## 8. Proving tests (planned — future tense, machine-checkable)

No budget test exists today; no test name+line is cited as shipped. Each
check below would be added with the implementation and uses future tense.

- PT-BUDGET-01 (GAP-DIFF-04, V3 — authoritative exit): a synthetic
  large-diff test, `TestTurnDiffSection_Budget` in `./internal/session`,
  would construct a multi-file change whose unbounded rendering exceeds the
  proposed caps, then would assert (a) `len(out) <= BUDGET` where `BUDGET`
  is the then-shipped per-turn constant cited with its real symbol+line, and
  (b) the output contains the truncation marker. Command:
  `go test ./internal/session -run TestTurnDiffSection_Budget -count=1`.
- PT-BUDGET-02 (per-file cap): a single-file oversize case through
  `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) would assert
  the block is cut at the per-file cap with the marker present and the hunk
  framing `@@ -%d,%d +%d,%d @@`
  (`internal/session/turn_diff.go:69`) intact above the marker.
- PT-BUDGET-03 (signal exemption): a binary-plus-large-text turn would assert
  the binary marker line survives truncation, and an `IsDelete`
  (`internal/diff/diff.go:253-255` sets it on `struct FileDiff` at
  `internal/diff/diff.go:98-105`) file block would assert its delete note
  survives. Text ownership stays with GAP-DIFF-02/GAP-DIFF-03; this test
  would assert exemption only.
- PT-BUDGET-04 (many small files): an N-file turn of individually small
  blocks would assert the per-turn cap fires, `DroppedFiles` names the
  skipped paths, and `turn_diff_truncated/2` (§5) is projected with
  `DroppedBytes > 0`.

## 9. Gap linkage and exits

- Primary: GAP-DIFF-04 (Bounded repair-prompt diff budget) as defined in
  `03-GAP-ANALYSIS.md` §"Gap matrix". Its exit is PT-BUDGET-01 above:
  `go test ./internal/session -run TestTurnDiffSection_Budget -count=1`
  passes with `len(out) <= BUDGET` and marker-present, the budget constant
  cited with symbol+line once shipped. This file is the `Target state`
  pointer for that row (the `05-prompt-budget.md` future named there).
- Interacting (owned elsewhere, exemption asserted here): GAP-DIFF-02
  (binary marker) and GAP-DIFF-03 (delete note) per §7 FM-BUDGET-02 and §8
  PT-BUDGET-03.
- Explicitly not in this file: GAP-DIFF-01 (cache-key proof),
  GAP-DIFF-05 (per-caller engine tuning), GAP-DIFF-06 (cache lifecycle),
  GAP-DIFF-07 (observability). Those keep their own exits in
  `03-GAP-ANALYSIS.md`.

## 10. Grounded vs hypothesized

- Grounded (High, opened 2026-09-21): every prod symbol+line in §2
  (`internal/diff/diff.go:14,30-38,82-105,202-211,238,243-307,310-312,343-400,403-486`;
  `internal/diff/cache.go:28-43,96-172,229-246`;
  `internal/session/turn_diff.go:24-52,55-76,59-64,69,78-94`).
- Hypothesized (do not cite as shipped): all of §3–§8 — budget constants,
  marker text, `TurnDiffBudget`/`TurnDiffOutcome`, `turn_diff_budget/3`,
  `turn_diff_truncated/2`, and the four proving tests. Absence claims (no
  truncation/budget symbol) rest on full-range reads of the cited ranges,
  not on positive witnesses, consistent with the two-prod-importer shape
  (`internal/session/turn_diff.go:62` via `var DefaultEngine` at
  `internal/diff/diff.go:238`; `var uiDiffEngine` at
  `cmd/nerd/ui/diffview.go:907` via `func CreateDiffFromStrings` at
  `cmd/nerd/ui/diffview.go:910-912`, call at `cmd/nerd/ui/diffview.go:911`).

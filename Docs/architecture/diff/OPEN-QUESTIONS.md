---
doc-class: governance
subsystem: diff
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# Open Questions — internal/diff

Governance for `internal/diff`: the unresolved design questions a future
author must answer before closing gaps, plus the standing invariants
("tripwires") no change may break. This file answers neither what runs today
nor what the finished package looks like — it records what is still undecided
and what must stay true regardless. On any disagreement about what the code
does today, `IMPLEMENTED_SPEC.md` wins.

Vision trace: this package serves `agents.md:43-46` — tools condense the
search space, reduce the turns to complete the task, and offload cognition to
deterministic code. The seams are `func ComputeDiff` at
`internal/diff/diff.go:310-312` (via `var DefaultEngine` at
`internal/diff/diff.go:238`) called at `internal/session/turn_diff.go:62`, and
`var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907` diffing at
`cmd/nerd/ui/diffview.go:911` inside `func CreateDiffFromStrings` at
`cmd/nerd/ui/diffview.go:910-912`.

Gap source: `03-GAP-ANALYSIS.md` GAP-DIFF-01..07. Risk source:
`RISK-REGISTER-AND-DECISION-LOG.md` R1..R7. Decision source: `adr/`
slugs `ADR-001` through `ADR-012` (proposed; status in each ADR is derived
from its witness, never asserted). Every shipped claim below cites a
repo-relative path plus a symbol and line read 2026-09-21.

## Open questions

Each question names its shipped anchor (cited like a shipped claim), the
undecided choice, the options on the table, and the machine-checkable
condition that resolves it. Answering a question means filing or updating its
ADR and closing its gap — not editing this file to assert the answer.

### OQ-DIFF-01 — Verify-for-apply, or accept trusted-key risk?

Anchor: widened `struct cacheKey` (`internal/diff/diff.go:121-129`) built by
`func fingerprint` (`internal/diff/diff.go:141-157`); verify-on-hit branch in
`method diffCache.get` (`internal/diff/cache.go:109-116`) counting
`Collisions` (`internal/diff/cache.go:110` over `struct Stats` at
`internal/diff/cache.go:28-43`); `VerifyCacheContent` off-by-default
(`internal/diff/diff.go:183-190`); both production engines run zero-`Options`
(`var DefaultEngine` at `internal/diff/diff.go:238` via `func ComputeDiff` at
`internal/diff/diff.go:310-312` called at `internal/session/turn_diff.go:62`;
`var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907` via `func NewEngine` at
`internal/diff/diff.go:214-216`).

Question: when (if ever) a diff drives an apply rather than a display, must
the cache prove its keys? Today diffs are display-only, so trust-by-default is
a defensible performance choice; the moment a diff selects what gets written,
a silent collision becomes a wrong write.

Options: (a) turn `VerifyCacheContent` on for the applying engine only and pay
double memory there (`internal/diff/cache.go:132-136` charges verify bytes);
(b) keep trust-by-default everywhere and accept the risk in an ADR witness;
(c) replace the key with a stronger hash and keep verify off.

Resolves: GAP-DIFF-01; R1; `adr/ADR-006-trust-by-default-verify-opt-in.md`.
Resolution condition: `go test ./internal/diff -run
TestVerifyCacheContent_Collision -count=1` passes (forced key-collision
asserts miss plus `Stats.Collisions==1` with entry dropped), OR an ADR names a
witness accepting trusted-key risk as `accepted-not-implemented`.

### OQ-DIFF-02 — What one-line marker stands in for a binary edit?

Anchor: NUL sentinel `func containsNullByte` (`internal/diff/diff.go:24-26`)
short-circuits to `IsBinary` with empty hunks (`method Engine.ComputeDiff` at
`internal/diff/diff.go:261-265`, flags on `struct FileDiff` at
`internal/diff/diff.go:98-105`, counter `method diffCache.markBinary` at
`internal/diff/cache.go:205-209`); `func renderFileDiff`
(`internal/session/turn_diff.go:55-76`) maps nil/zero-hunk to `""`
(`internal/session/turn_diff.go:63-64`); no binary-marker branch witnessed in
`func turnDiffSection` (`internal/session/turn_diff.go:24-52`) by full-range
read 2026-09-21 (absence, not a symbol).

Question: what text should the repair round see for a binary write so it does
not spend turns re-diagnosing an invisible edit — `(binary, not shown)`,
`(binary file changed)`, size + path, or something the model already
recognises? Wording matters: it must not look like a unified-diff hunk, must
not claim hunks exist, and must survive `func diffMarker`
(`internal/session/turn_diff.go:85-94`) rendering unchanged.

Resolves: GAP-DIFF-02; R3; `adr/ADR-binary-marker-for-repair-round.md`
(proposed `accepted-not-implemented`).
Resolution condition: `go test ./internal/session -run
TestRenderFileDiff_BinaryMarker -count=1` passes:
`renderFileDiff(binA,binB)` returns non-empty containing `binary`, and
`turnDiffSection` includes it instead of `""`.

### OQ-DIFF-03 — Does a deleted file get a symmetric note?

Anchor: `func newFileNote` (`internal/session/turn_diff.go:78-83`) marks only
`IsNew`; `IsDelete` exists on `struct FileDiff`
(`internal/diff/diff.go:98-105`) and is set on the empty-new-side branch
(`internal/diff/diff.go:253-255`) with no witnessed note branch.

Question: should a turn that deletes a file get a `(deleted by this turn)`
note mirroring the create note, or is the hunk list self-explanatory?
Symmetric notes cost one branch; asymmetric notes cost repair-round clarity on
every deletion.

Resolves: GAP-DIFF-03; R4; `adr/ADR-delete-note-symmetry.md` (proposed
`accepted-not-implemented`).
Resolution condition: `go test ./internal/session -run
TestRenderFileDiff_DeleteNote -count=1` passes: `IsDelete` output asserts the
delete-note string present in the `renderFileDiff` result.

### OQ-DIFF-04 — How big may the repair-prompt diff be, and what marks truncation?

Anchor: `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`) has no
truncation symbol; `func renderFileDiff`
(`internal/session/turn_diff.go:55-76`) composes `@@` framing
(`internal/session/turn_diff.go:69`) with `func diffMarker`
(`internal/session/turn_diff.go:85-94`) and no budget check;
`func turnDiffSection` (`internal/session/turn_diff.go:24-52`) concatenates
per-file sections with no budget check (absence by full-range read
2026-09-21, not a symbol).

Question: what is the per-file plus per-turn byte/line budget, where is it
enforced (engine vs `renderFileDiff` vs `turnDiffSection`), and what explicit
marker (`... truncated ...`, budget constant name) tells the model output was
cut? An unbounded concat breaks the condense-search-space half of the vision
sentence exactly on large diffs. Reuse of `struct Options`
(`internal/diff/diff.go:162-191`) tuning vs a new budget constant is part of
this question — see OQ-DIFF-05.

Resolves: GAP-DIFF-04; R2; `adr/ADR-012-prompt-diff-budget.md` (proposed
`accepted-not-implemented`).
Resolution condition: `go test ./internal/session -run
TestTurnDiffSection_Budget -count=1` passes: synthetic large diff asserts
output `len <= BUDGET` and contains the truncation marker; the budget constant
is cited with symbol+line.

### OQ-DIFF-05 — Per-caller engine tuning, or explicitly never-tune?

Anchor: `struct Options` (`internal/diff/diff.go:162-191`:
`ContextLines` at `:166`, `DisableCache` at `:170`, `MaxCacheEntries` at
`:173`, `MaxCacheBytes` at `:177`, `Timeout` at `:181`,
`VerifyCacheContent` at `:190`) via `func NewEngineWith`
(`internal/diff/diff.go:220-230`, `dmp.DiffTimeout` set at
`internal/diff/diff.go:224`); production builds only `var DefaultEngine` at
`internal/diff/diff.go:238` (used at `internal/session/turn_diff.go:62`) and
`var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907` (equal to
`NewEngineWith(Options{})` per `func NewEngine` at
`internal/diff/diff.go:214-216`) — both zero-`Options`.

Question: should the TUI and the agent loop ever differ in context width,
timeout, or cache bounds, or is uniform zero-`Options` the deliberate
contract? Non-default knobs with no production caller are dead surface that
invites speculative tuning; per-caller tuning without a pinned behavioural
difference is the same defect in the other direction.

Resolves: GAP-DIFF-05; R5;
`adr/ADR-009-one-engine-per-package.md` context plus proposed
`adr/ADR-per-caller-engine-tuning.md`.
Resolution condition: one seam constructs `func NewEngineWith` at
`internal/diff/diff.go:220-230` with non-zero `Options` and a test pins the
behavioural difference; OR an ADR witness marks tuning
`accepted-not-implemented`.

### OQ-DIFF-06 — Does `ClearCache` have a lifecycle event, or is it test-only?

Anchor: `method Engine.ClearCache` (`internal/diff/diff.go:518-520`,
counter-preserving `method diffCache.clear` at
`internal/diff/cache.go:191-197`) with no witnessed production caller at
`internal/session/turn_diff.go:62` or `cmd/nerd/ui/diffview.go:911,916,970`
(absence by full-range read; test callers not cited per citation rules).
Bounded eviction (`method diffCache.evictLocked` at
`internal/diff/cache.go:175-187`, skip-single-oversize at
`internal/diff/cache.go:159-161`) means growth alone does not force a clear.

Question: is there a session lifecycle event (memory pressure, stale-entry
bug, long-session reset) that should call `ClearCache`, or is the bounded LRU
sufficient and `ClearCache` a test helper wearing API clothes? Wiring a caller
without a lifecycle need adds a seam nobody exercises; leaving it unwired
leaves readers unable to tell API from helper.

Resolves: GAP-DIFF-06; R6; `adr/ADR-011-clearcache-lifecycle.md`
(mechanics shipped, lifecycle `accepted-not-implemented`).
Resolution condition: either a production call-site plus `go test
./internal/diff -run TestClearCache_Concurrent -count=1 -race` passes (clear
during concurrent compute preserves counters), or a doc/ADR witness retires it
as test-only.

### OQ-DIFF-07 — Does `Stats` feed production, or is it test-only?

Anchor: `func DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`,
delegating at `cmd/nerd/ui/diffview.go:916` to `method Engine.Stats` at
`internal/diff/diff.go:233-235` over `struct Stats` at
`internal/diff/cache.go:28-43`); only witnessed caller is the pinning test
`func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView` at
`cmd/nerd/ui/word_highlight_test.go:141-162` (before/call/after at
`:150/:151/:152`, `Computes` routing check at `:154-156`).

Question: should `Hits`/`Misses`/`Computes`/`Binary`/`Evicted`/`Collisions`
feed a log line, a gate, or a policy — or is observability through tests
enough? Counters that advance silently give a cache pathology (thrashing,
collision burst) no alert path; a production consumer nobody reads is
ceremony.

Resolves: GAP-DIFF-07; R7; proposed
`adr/ADR-stats-observability.md`.
Resolution condition: production caller of `func DiffEngineStats` at
`cmd/nerd/ui/diffview.go:915-917` with `go test ./cmd/nerd/ui -run
TestDiffEngineStats -count=1` asserting counters advance; OR a witness marks
it test-only.

## Tripwire invariants (standing — must survive any answer above)

A future change that breaks any tripwire is wrong even if it closes a gap. Each
tripwire cites the symbol that pins it and the test (or command) that would
catch the break.

- **T1 — The engine never emits `LineHeader`.** Hunk framing lives in
  `struct Hunk` (`internal/diff/diff.go:89-95`); the `LineHeader` member
  (`internal/diff/diff.go:48-56`) is UI-owned and the engine emitting one is a
  bug per its own contract comment. Callers compose `@@` themselves
  (`internal/session/turn_diff.go:69` with `func diffMarker` at
  `internal/session/turn_diff.go:85-94`). Tripwire: the never-emit test named
  in the contract comment must keep passing; any new renderer must synthesise
  its own header rows, never expect one from the engine.
- **T2 — Binary input yields `IsBinary`, never hunks.** `func containsNullByte`
  (`internal/diff/diff.go:24-26`) into `method Engine.ComputeDiff`
  (`internal/diff/diff.go:261-265`, flags `internal/diff/diff.go:98-105`) plus
  `method diffCache.markBinary` (`internal/diff/cache.go:205-209`) is the only
  binary path. A change that lets NUL input reach the
  `DiffLinesToChars`/`DiffMain`/`DiffCleanupSemantic`/`DiffCharsToLines` chain
  (`internal/diff/diff.go:293-296`) breaks bounded-cost as well as correctness.
- **T3 — Cache entries are isolated by deep copy on both sides.** `method
  diffCache.get` (`internal/diff/cache.go:96-121`) returns `Clone`, `method
  diffCache.put` (`internal/diff/cache.go:126-172`) stores `Clone` at
  `internal/diff/cache.go:130` (via `method FileDiff.Clone` at
  `internal/diff/cache.go:229-246`). Callers may mutate the returned
  `struct FileDiff` (`internal/diff/diff.go:98-105`) freely; no mutation may
  leak into — or out of — the cache.
- **T4 — Zero `Options` means defaults, on both engines.** `func NewEngine`
  (`internal/diff/diff.go:214-216`) equals `NewEngineWith(Options{})`;
  `method Options.contextLines` (`internal/diff/diff.go:194-199`) maps 0 to
  `defaultContextLines` (`internal/diff/diff.go:17`) through
  `func clampContextLines` (`internal/diff/diff.go:30-38`), and `method
  Options.timeout` (`internal/diff/diff.go:202-211`) maps 0 to `diffTimeout`
  (`internal/diff/diff.go:14`, applied at `internal/diff/diff.go:224`). Both
  production engines run zero-`Options` today (`internal/diff/diff.go:238` at
  `internal/session/turn_diff.go:62`; `cmd/nerd/ui/diffview.go:907`); any
  tuning answer (OQ-DIFF-05) must keep the zero-means-defaults rule or update
  both call-sites and their tests together.
- **T5 — A negative `Timeout` genuinely disables the bound.** `method
  Options.timeout` (`internal/diff/diff.go:202-211`) maps negative to 0
  (no-timeout). This is a footgun, not a feature: any caller passing a
  computed timeout must clamp before construction, and any future budget answer
  (OQ-DIFF-04) must not confuse "no timeout" with "default timeout".
- **T6 — One engine per use-site; never per call.** The agent loop shares `var
  DefaultEngine` (`internal/diff/diff.go:238`); the TUI shares `var
  uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`, rationale at
  `cmd/nerd/ui/diffview.go:900-906`, consumed at `cmd/nerd/ui/diffview.go:911`
  and `cmd/nerd/ui/diffview.go:970`). Constructing an engine per diff
  reintroduces the diverged-caches surprise the single-engine shape removed.
- **T7 — Word diff stays uncached, per-line-pair, in the own type.** `struct
  WordSpan` (`internal/diff/diff.go:76-79`) via `method
  Engine.ComputeWordLevelDiff` (`internal/diff/diff.go:530-551`, wrapper at
  `internal/diff/diff.go:554-556`, uncached rationale at
  `internal/diff/diff.go:522-529`) consumed at `cmd/nerd/ui/diffview.go:970`.
  No raw `diffmatchpatch.Diff` crosses the API, and no caller may assume word
  spans are cached or stable across lines.
- **T8 — The repair round sees its own edits, or explicitly nothing.**
  `func turnDiffSection` (`internal/session/turn_diff.go:24-52`) returns `""`
  when nothing was written; `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) short-circuits identical content
  (`internal/session/turn_diff.go:59-61`) and nil/zero-hunk
  (`internal/session/turn_diff.go:63-64`). Binary (OQ-DIFF-02) and delete
  (OQ-DIFF-03) answers must land inside these two functions — a third
  rendering path is a new seam and needs its own wiring entry.
- **T9 — `ClearCache` keeps cumulative counters.** `method Engine.ClearCache`
  (`internal/diff/diff.go:518-520`) delegates to `method diffCache.clear`
  (`internal/diff/cache.go:191-197`), which preserves counters. Any lifecycle
  answer (OQ-DIFF-06) that resets stats alongside entries breaks the
  observability on which OQ-DIFF-07 depends.

## How to resolve a question

1. Open the anchor symbols above — not the previous docs (citation rules,
   Rule 4: the shipped layer is written from the Go).
2. Draft or update the linked ADR under `adr/` with context, decision,
   consequences, and a witness (test, symbol, predicate, or file whose
   resolution derives the status).
3. Implement to the gap's exit criterion in `03-GAP-ANALYSIS.md` and retire
   the linked risk in `RISK-REGISTER-AND-DECISION-LOG.md`.
4. Leave closed gap rows in place, marked closed with their closing commit —
   IDs are stable and never deleted.

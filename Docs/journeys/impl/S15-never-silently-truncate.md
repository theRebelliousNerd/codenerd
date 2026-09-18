# S15 — Nothing in the window is cut silently

## Status

last updated: 2026-09-18

seam: S15, contract `C-17` (`Docs/journeys/04-contract-registry.md:420`). branch
`worktree-agent-a0fe46d265c393352`, based on `dogfood/c2-closure` @ `9443b8a0`.

The rule: model- or user-bound content is never silently shortened. Where content must
leave the window it is either **(a) restated** — handed back to the model to be said again
within the limit — or **(b) elided with the pipeline's own marker** naming what was
dropped, count and kind, plus a handle where the dropped content still exists. A count cap
that drops content with no marker is the defect.

Half (a) already existed and was left alone: `internal/broker/compression.go` sends a cut
completion back with `compressionInstruction` up to `maxCompressionRetries` times and
returns the typed `types.OutputTruncated` error rather than the partial. Half (b) existed
as `internal/prompt/limits.go` and reached only the prompt compiler's own callers.

done:
- The marker convention moved to a leaf package so the whole pipeline can leave it
  (`internal/types/elide.go`), old location deleted, 12 callers repointed.
- Eight silent cuts marked; three of them also made recoverable or already were.
- `internal/tactile`'s `Truncated`/`TruncatedBytes` now reach the model-facing text —
  this was the study's open question, and the answer was "silently shortened".
- Nine named tests plus the invariant test through the real assembly.

open: see [Open](#open).

---

## Inventory of every cut

Re-derived by grep over `internal/`, `cmd/` for slices by byte/token/count limit
(`[:max…]`, `[:limit]`, `min(` in a slice expression, `TruncatedBytes`), then filtered to
content that reaches the model or the user. Operator-facing log lines are excluded: a
`logging.Context(...)` cut is not in anybody's window.

| # | Site | What is cut | Marker before | Recoverable before | Now |
|---|------|-------------|---------------|--------------------|-----|
| 1 | `internal/tactile/types.go` `ExecutionResult.Output` | stdout+stderr at `MaxOutputBytes` (10 MiB default), set by all six backends | **none** — flags stayed on the struct | no (bytes discarded at capture) | marker + byte count, in `Output` and in `MarkTruncated` for the split stdout/stderr projection |
| 2 | `internal/core/virtual_store_actions.go` `Exec` | same capture, returned as a `(stdout, stderr)` pair | **none** | no | marker on stderr |
| 3 | `internal/session/executor.go` `priorTurnMessages` | whole conversation messages, oldest first, against `HistoryTurnWindow` + `HistoryCharBudget` | **none** — debug log only | in memory, unrecorded | marker on the oldest surviving message; evicted messages retained via `recordHistoryEviction` / `recoverHistoryEviction` |
| 4 | `internal/context/compressor_metrics.go` `collectKeyAtoms` | 5 result atoms per turn, 64 atoms per segment | **none** | no | dropped count returned, carried on `HistorySegment.DroppedAtoms`, rendered as a marker in the block |
| 5 | `internal/context/compressor_metrics.go` `trimToTokens` | a history segment's summary to a token budget | **none** | no | marker with count, budgeted for so the trim still fits |
| 6 | `internal/context/compressor_turns.go` unmasked-turn atom cap (×2) | `ResultAtoms[:min(3,…)]` in both summary generators | **none** (the *masked* branch three lines away has always named what it removes) | no | `writeCappedResultAtoms` emits `TruncationNotice` naming the turn |
| 7 | `internal/context/compressor_turns.go` key-atom shed | `seg.KeyAtoms = nil` under reserve pressure | **none** | no | count moves to `DroppedAtoms`, so the block still announces it |
| 8 | `internal/context/serializer.go` `truncateFact` | each fact argument at 47 runes | `"..."` — no count, reads as the author's own ellipsis | no | `ClampInline` with count; the cut is skipped when the marker would cost more than it saves |
| 9 | `cmd/nerd/chat/process_continuation.go` `truncateSummary` | `shard_result/5`, `pending_test`, `pending_review` fact bodies at 30–200 chars | `"..."` | no | `ClampInline` with count |
| 10 | `internal/core/shards/manager_spawn.go` `ResultToFacts` | `shard_output/2` at 4000 chars | `"... (truncated)"` — no count, head-only | no | `ClampText` (head+tail, marked) |
| 11 | `internal/core/shards/manager_spawn.go` `extractSummary` | `recent_shard_context/2` at 200 chars | **none** | no | `ClampInline` with count |
| 12 | `internal/session/working_context.go` archived observation | whole oversize tool result replaced by a pointer | own wording ("Observation archived: … recall_context id=…") — size and handle present, prefix not the pipeline's | **yes**, `recall_context` | the pipeline's marker alongside the existing prefix and handle |

Already compliant, left alone (verified, not changed): `internal/prompt/compiler.go` and
`compiler_specialists.go`, `internal/articulation/prompt_assembler.go`,
`internal/perception/transducer_llm.go`, `internal/projectdoc/facts.go`,
`internal/session/executor.go`'s `appendToHistory` and per-result tool clamp,
`internal/context/serializer.go`'s 64 KB block clamp,
`internal/observation`'s `EncodeReturn` (elides with an `obs:sa:` handle redeemed by
`subagent_expand`), and the broker's restatement path.

### Not a silent cut, and deliberately not touched

- `withProjectInstructions` (`executor.go:1064`) injects `nerd.md` **unbudgeted**. Nothing
  is dropped, so nothing is unannounced; the risk is overflow, not silence. Out of scope
  for S15 — noted in Open.
- Display cuts in test failure messages and TUI status lines that are not sent to a model.

## Design

**One marker, one package.** `internal/prompt/limits.go` held the only marker the rule is
written against, in a package `internal/tactile` and `internal/core` cannot import —
`internal/prompt` already depends on both. So the shell backend that caps a 40 MB build log
had no way to say so in the same words as the prompt compiler. The primitives moved to
`internal/types/elide.go`, the lowest package every cutter already imports, and sit next to
`truncation.go`'s `OutputTruncated`, which is half (a) of the same rule. No shim: the old
file is deleted and all 12 callers repointed.

Three additions to the convention, all emitting the same `clampMarkerPrefix`:

- `ClampInline(text, maxChars, label)` — for positions where a newline cannot go (a Mangle
  fact argument, a status line). The alternative was every inline caller inventing `"..."`.
- `DroppedNotice(dropped, total, unit, handle)` — for content that left a message *whole*
  rather than being shortened in place. `handle` is passed only when a verb in this process
  resolves it; a handle nobody can redeem sends the reader to a "not found" that is
  indistinguishable from expiry.
- `TruncationMarker(detail)` — the bare marker, for the one case where the source genuinely
  did not report a size (`Truncated` true, `TruncatedBytes` zero). Everything that knows how
  much it dropped says how much it dropped.

**Where the marker goes.** At the point that has both the shortened text and the knowledge
that it was shortened. For tool output that is `ExecutionResult.Output`, which every
model-facing consumer passes through, rather than the six backends or the many callers. For
history eviction it is the oldest surviving message, which is the first thing the model
reads. For the compressor's atom caps it is the rendered block, because a count that stays
on a struct is not a marker.

**Marker cost is budgeted, not bolted on.** `trimToTokens` exists to make something fit, so
a notice appended afterwards would defeat its caller. It reserves the marker's worst-case
length off the budget first. Below the floor where a marker cannot fit at all, the marker is
what the budget buys: a 20-character fragment of a 4600-character segment with nothing to
distinguish it from a 20-character segment is the exact failure the rule exists to prevent.
That is the one case where a result exceeds its ceiling, and two pre-existing tests were
updated to the new contract (see Tests).

**Recoverability.** Three grades, and the code now says which one applies:
1. *Retained and redeemable by the model* — the archived observation (`recall_context`) and
   subagent returns (`subagent_expand`). Unchanged; the marker joined them.
2. *Retained in process* — evicted history. `recoverHistoryEviction` returns the dropped
   messages; the marker says the session still holds them but does not publish a handle,
   because no verb redeems one today (Open).
3. *Gone* — bytes discarded at capture by `MaxOutputBytes`, atoms dropped by a cap. The
   marker states the count and claims nothing more.

## Changes

- `516f800c` `refactor(elide)` — `internal/prompt/limits.go` → `internal/types/elide.go`
  (+ `limits_test.go` → `elide_test.go`), package header rewritten to say why it lives
  there; 12 callers across `internal/prompt`, `internal/perception`,
  `internal/articulation`, `internal/session`, `internal/context` repointed.
- `a3b7b5f4` `fix(elide)` — inventory rows 1–12.
- (tests commit) — the nine named tests and the invariant test.

## Tests

Fail-before evidence was taken by blinding the marker emission at each site while keeping
every signature intact (so the tests still compile), running the new tests, and restoring.
The shared stash was not used: a temporary WIP commit held the work and
`git checkout HEAD -- <paths>` restored it.

| Test | Package | Pins |
|------|---------|------|
| `TestToolResult_TruncationVisibleToModel` | `internal/tactile` | rows 1–2, all four `Truncated` shapes incl. the no-byte-count case |
| `TestMarkTruncated_CarriesTheCutOnASeparateProjection` | `internal/tactile` | row 2 |
| `TestHistoryEviction_MarksAndRetains` | `internal/session` | row 3, marker + position + recoverability |
| `TestHistoryEviction_SaysNothingWhenNothingWasEvicted` | `internal/session` | row 3, the converse |
| `TestTrimToTokens_MarksTheCut` | `internal/context` | row 5, incl. "still fits the budget" |
| `TestCollectKeyAtoms_MarksDroppedAtoms` | `internal/context` | row 4, count *and* its arrival in the rendered block |
| `TestUnmaskedTurnAtomCap_MarksTheCut` | `internal/context` | row 6, both generators |
| `TestUnmaskedTurnAtomCap_SaysNothingWhenNothingWasCut` | `internal/context` | row 6, the converse |
| `TestKeyAtomShed_MarksTheCut` | `internal/context` | row 7 |
| `TestFactSerializer_MarksDroppedArgChars` | `internal/context` | row 8 |
| `TestTruncateSummary_MarksTheCut` | `cmd/nerd/chat` | row 9 |
| `TestResultToFacts` (updated) / `TestExtractSummary_MarksTheCut` | `internal/core/shards` | rows 10–11 |
| **`TestNoSilentCutsInAssembledMessages`** | `internal/session` | the invariant, through `prepareWorkingRequest` for a tool-loop turn and `priorTurnMessages`/`renderHistoryTranscript` for a chat turn |
| `TestNoSilentCutsInAssembledMessages_LeavesAnIntactTurnAlone` | `internal/session` | the converse: intact content is delivered byte-identical and unmarked |

The invariant test is the net under the per-site tests: it indexes source content by tool-use
ID and by needle, drives the real assembly at a window small enough to force shedding, and
fails when any emitted segment is shorter than its source without the marker — including a
segment that left the request entirely. Both halves assert that shedding actually occurred,
so a future change that stops exercising the path fails rather than passing vacuously.

### Two pre-existing tests updated, and why

- `TestCompressorTrimToTokens` asserted a 5-token trim produces ≤ 5 tokens. No honest marker
  fits in 5 tokens. The assertion is now "fits the budget, **or** is exactly the bare
  marker", plus a new roomy-budget case asserting the budget is respected exactly whenever
  the marker fits.
- `TestRebuildRollingSummaryText_WhenBlockOutgrowsReserve` set a 40-token reserve to force
  merging. The render frame alone is ~25 tokens, so the reserve is below the floor of frame
  + one marker. The assertion is now "fits the reserve, **or** overflows carrying a marker
  that says why", bounded at 2× the reserve so a real regression still fails.

Both are contract changes the seam makes deliberately, not numbers moved until green.

## Full test run

See the report accompanying this branch.

## Open

1. **No verb redeems evicted history.** `recoverHistoryEviction` makes the dropped turns
   recoverable in process, and the marker says the session still holds them, but there is no
   `history_expand`-style tool the model can call. Minting a handle with no redeemer would be
   worse than none (`internal/observation/subagent.go` says so explicitly). The natural
   shape is an `internal/observation` codec over `internal/retain` plus a read-effect tool,
   which is a tool-surface change (registration, `internal/tools/effects.go`, the
   constitution's `safe_action`) and belongs in its own seam.
2. **`nerd.md` is still unbudgeted** (`withProjectInstructions`, `executor.go:1064`). Not a
   silent cut — nothing is dropped — but an unbounded append into a budgeted window.
3. **The rolling-summary render frame is fixed-size** (~25 tokens of header) regardless of
   `HistoryReserve`, which is what makes a small reserve unreachable once a marker is
   mandatory. Shrinking the frame under pressure would raise the floor problem, not just
   move it.
4. **`internal/browser`, `internal/campaign`, `internal/mcp` and `internal/init` hold further
   count caps** (`clampContainers`, `truncateAuditSections`, `maxSnippets`,
   `maxFactStringLen`, …). Several already carry their own `Truncated` flags and notes. They
   were not inventoried here because their outputs reach the model through paths this seam
   did not trace; the invariant test does not cover them.
5. **The invariant test drives two turn shapes.** A campaign/headless turn assembles its
   window elsewhere (`internal/campaign`), and whether it has an equivalent of
   `prepareWorkingRequest` is the study's own open item.

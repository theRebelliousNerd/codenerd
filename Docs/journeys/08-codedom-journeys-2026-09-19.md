# 08 — CodeDOM user journeys: what the raw exchanges show

Steve, 2026-09-19 21:4x: *"you should study codedom user journeys... the back and forth with an
llm using the codedom in long horizon or multi turn type stuff, or just different scenarios to
the the raw exchange."*

This is that study, built from transcripts rather than from the code. Every number below comes
from `.nerd/logs/*_llm_io.log` — the actual request/response bodies of real `nerd fix` runs.

## Status

- last updated: 2026-09-19 21:55
- corpus: **11 runs**, 75 MB of transcripts, 2026-09-19 19:21 → 2026-09-20 00:42 (UTC filenames;
  local 19:21 → 20:42), ~4 hours of agent work, ladder runs R1-10 … R1-17
- done: census, per-run journeys, every failed exchange, the two-implementations finding
- open: whether `code_element` should be populated or the query dropped (D3 — an architecture
  call, not a brief); a measured re-run after any fix

## Method

Each request block in `llm_io.log` reprints the whole conversation, so one tool call appears
once per later request. Counting occurrences over-reports by 3-8x. Every number here counts
**distinct `call_…` ids**, first appearance only — the turn the model actually made the call.

Script: `scratchpad/codedom_journeys.py`. It pairs `[tool_use id=… name=…]` with its
`[tool_result id=…]`, keeps the assistant sentence that introduced the call, and writes
`codedom_calls.json`.

## The census

Distinct calls across all 11 runs:

| tool | calls | errors |
|---|---|---|
| `edit_lines` | 29 | 5 (17%) |
| `get_elements` | 11 | 0 |
| `insert_lines` | 7 | 0 |
| `get_element` | 2 | 1 |
| `replace_element` | **0** | — |
| `apply_edits` | **0** | — |
| `edited_refs` | **0** | — |
| `run_impacted_tests` | **0** | — |
| `delete_lines` | **0** | — |
| `open_file`, `refresh_scope`, `close_scope` | **0** | — |

And against the tools codedom competes with, same corpus:

| tool | distinct calls |
|---|---|
| `read_file` | **261** |
| `recall_context` | 130 |
| `grep` | 54 |
| `edit_lines` | 38 |
| `run_tests` | 33 |
| `glob` | 14 |
| `get_elements` + `get_element` | **13** |

**20 raw file reads for every structural read.** In R1-16, a run whose entire subject was
`get_element`, the model used the structural tools 4 times and `read_file` 19.

## The journeys

### J1 — orient, edit, re-orient (the only multi-turn pattern that appears)

R1-13 (`20260919_222711`), the longest codedom journey in the corpus and the one whose fix
landed. 17 calls:

```
 1 get_elements  elements.go
 2 edit_lines    (refused: delimiter balance)
 3 edit_lines
 4 insert_lines
 5 edit_lines
 6 edit_lines    (refused: delimiter balance)
 7 edit_lines
 8 insert_lines
 9 get_elements  elements.go          <- re-orient after 7 edits
10 edit_lines    extent_realfile_test.go L54-58
11 edit_lines    elements.go L364-368
12 get_elements  elements.go          <- re-orient
13 get_elements  elements.go          <- again
14 get_elements  elements_test.go
15 edit_lines
16 edit_lines
17 get_elements  elements_test.go     <- again
```

`get_elements` is used as a **re-synchronisation tool**: after a batch of line edits the model's
line numbers are stale, so it re-lists to recover them. That is the right instinct and the wrong
mechanism — the model is hand-maintaining a projection the scope layer already knows how to
maintain (D3).

### J2 — `get_element` is a test harness, not a navigator

Only 2 calls in 11 runs, both in R1-16, both at the very end, introduced by:

> "The fix is present and tests pass. Now verifying each required behavior directly:"

The model used `get_element` to check its own fix, never to find anything. Navigation is
`read_file` + `grep`.

### J3 — edit, then edit inside what you just edited

R1-11 (`20260919_210749`), 2 calls:

```
1 edit_lines  write_set_lock_manager.go L111-118   ok
2 edit_lines  write_set_lock_manager.go L112-115   REFUSED
```

The second edit lands inside the span the first one wrote. It is refused for delimiter balance.
Nothing in the first result warned that the region's shape had changed.

### J4 — a malformed call gets an error about something else

R1-10 (`20260919_203409`). The model sent `edit_lines` with an **empty argument object** and got:

```
blocked by nerd.md: unknown is write-protected (write target is missing or uses an
unrecognized path argument)
```

The call had no path; the answer is about write protection. The model then burned rounds — the
same turn carries the orchestrator's *"17 rounds of reading and no file written for a change
task."*

## Findings

### D1 the structural tools are not sold

`read_file`'s description is five sentences: line prefixes, how to cite `file:line`, what to
strip before passing text to another tool, how long files come back as a region plus an outline
(`internal/tools/core/file_ops.go:25`). `get_elements`'s description, in full
(`internal/tools/codedom/elements.go:78`):

> "List code elements (functions, classes, methods) in a file"

Eight words, no reason to prefer it, nothing about what it saves. The repo's tool doctrine says
a tool exists to condense the search space, reduce turns, or offload cognition. `get_elements`
does the first; nothing tells the model so, and the measured ratio is 20:1 against it.

### D2 RETRACTED -- the delimiter guard was right, and this finding was not checked

**This finding was wrong, and it was briefed to codeNERD before anyone verified it.** It is left
here rather than deleted because the way it was wrong is the useful part.

What it claimed: 4 of 5 `edit_lines` refusals were the guard rejecting valid restructurings, and
*"the file that edit produces is valid: it parses, it compiles."* That last sentence was never
measured. It was inferred from the refusals looking like refactors, and it went into an R1 brief
as evidence.

Why it is false, by arithmetic rather than by opinion. **Net delimiter balance is conserved.** For
a file that starts balanced, replacing span `S` with content `N` leaves the file at
`0 - net(S) + net(N)`. The old guard refused exactly when `net(S) != net(N)` -- which is exactly
when the result is unbalanced. On a balanced file the guard was correct *by construction*, and no
refactor that keeps the file balanced can trip it, however far a block boundary moves.

The five refusals with reported deltas were `+2 -> +1`, `+0 -> -1`, `-1 -> +0`, `+0 -> +1` and
`-1 -> +0`. Every one would have left the file unbalanced. The guard caught five partial edits;
the model widened its range and continued, which is the loop working.

What was actually wrong with the guard is smaller, and none of it was in the brief:

- **It refused repairs.** A file that is already unbalanced could not be edited back into shape,
  because any repairing edit necessarily changes the span's net.
- **It counted instead of nesting.** Net counts accept `}{`; only a stack rejects it.
- Its message told the model to "widen the range to cover both ends", which is right, and the
  model did exactly that each time.

**The lesson about briefs.** A symptom-only brief carries evidence; this one carried a conclusion
formatted as evidence. Given a confident false premise a good agent does what R1-19 did -- it
fixes the stated problem competently. The four-line check that would have refuted the premise
(apply the edit, run `gofmt -e`) was available the whole time and was not run.

R1-19's change is judged on its own merits in the ledger: it keeps every refusal that mattered,
adds the repair path and the nesting check, and does not fix the symptom the brief named, because
there was none.

### D3 the structural fact layer is produced by a path the model cannot reach

`code_element` is queried by the working context on every entity, every round, and never
answers. Measured on R1-16's audit log:

| predicate | queries | non-zero | time |
|---|---|---|---|
| `dependency_link` | 1418 | 1142 | 84.9 s |
| `code_defines` | 1175 | **950** | 9.8 s |
| `code_element` | 1175 | **0** | 0.0 s |

`code_defines("internal/logging/logger.go", …)` returns 59 rows. The world model is alive. Only
the scope layer is dark, and the kernel log confirms it: **`code_element` is asserted zero
times** in the whole run.

The reason is that there are two codedom implementations with the same tool names:

| | model-facing | fact-producing |
|---|---|---|
| package | `internal/tools/codedom/*` | `internal/core/virtual_store_codedom.go` |
| reached by | the tool registry (`GetElementsTool`, …) | kernel-routed `ActionOpenFile`/`ActionEditLines` |
| `edit_lines` says | `"Replaced lines %d-%d (%d lines) with %d new lines in %s."` | `"Edited lines %d-%d in %s"` |
| asserts `code_element` | **no** | yes — via `FileScope.ScopeFacts()` |

The fact-producing handlers emit scope facts only under `if scope != nil && scope.IsInScope(path)`
(`virtual_store_codedom.go:382`), and the only thing that opens a scope is `handleOpenFile`
(`ActionOpenFile`), which no registered tool exposes. So no agent run ever has a scope open, and
`code_element` — together with `element_signature`, `element_visibility`, `element_parent`,
`code_interactable` — never exists.

So the working context pays for 1175 queries a run into a fact space that is empty by
construction, and the richest structural facts codedom can produce never reach the window.

**S13 decided (Steve, 2026-09-19 21:5x): "fix the codedom dude! add capability."** Agent runs
open a scope. The delete is off the table.

#### Why the scope is empty — the full chain, verified

It is not only that `code_element` is session-scope. Four independent links are broken, and any
one of them alone would be enough:

1. **The turn's target is prose.** The `user_intent` fact a `nerd fix` run asserts is
   `user_intent(/task_intent_1, /mutation, /fix, "<the entire brief text>")`. Every kernel rule
   that keys on the target being a file therefore cannot match.
2. **The intent id does not match either.** `codedom_edit.mg` keys on `/current_intent`; the run
   asserts `/task_intent_1`.
3. **The session executor never consults `next_action`.** Zero references in `internal/session`.
   So `next_action(/open_file)` is unreachable from `nerd fix` whatever the facts say — the
   VirtualStore codedom handlers are reachable only from the chat path.
4. **Only an open scope emits the facts.** `handleEditLines` and friends emit `ScopeFacts()`
   under `if scope != nil && scope.IsInScope(path)`, and only `handleOpenFile` opens one.

The cost of link 1 is larger than the scope: **all 197 lines of `codedom_edit.mg` derive
nothing.** Edit safety (`edit_unsafe`, `element_edit_blocked`), breaking-change risk
(`breaking_change_risk` over `element_visibility` and `element_parent`), API-handler awareness,
and the CodeDOM activation boosts are all keyed on `code_element`. The policy is written,
stratified and dead.

Note what is *not* a defect: the working context's `"."` focus. `normalizeWorkingEntity`
(`internal/session/working_context.go:171`) resolves a non-path target to the workspace root on
purpose — *"an intent target is often a phrase describing the change, not a file"* — so
observations are not filed under a sentence. D5 is the documented behaviour of link 1, not a
separate bug.

#### The fix

Everything needed already exists; nothing new produces facts:

| piece | where |
|---|---|
| parse a file into `code_element` + `element_signature`/`_visibility`/`_parent`/`code_interactable` | `world.ParserFactory.EmitAllFacts`, via `FileScope.ScopeFacts()` |
| open the scope and return those facts | `VirtualStore.handleOpenFile` (`ActionOpenFile`) |
| a handle to dispatch it | `Executor.virtualStore` |
| the kernel those facts land in | the main kernel — `SetWorkingWorld(bctx.kernel)`, so the working set's `code_element` query reads the same store |
| staleness after an edit | `handleEditLines`/`handleInsertLines` refresh the scope once it is open; `clearCodeDOMFacts` replaces it |

#### The wiring gap the first attempt shipped with

`e78e7241` wired the source onto the session executor in `factory.go` and was measured dark on the
very next run: R1-19 made 24 tool calls and logged not one `codedom scope` line. `nerd fix`
delegates to a coder shard, and the **Spawner** builds that shard's executor itself, forwarding the
parent's state one call at a time -- `SetProjectDoc`, `SetFileContextProvider`, `SetWorkingWorld`
(`spawner.go:355-366`). A new provider that is not added to that list reaches the session executor
and never the shard that does the work.

This is the repo's own warning about itself ("this codebase frequently has partially wired features
and dormant integration points") reproduced in a single commit, by the change that was fixing
another instance of it. The tests covered the scope's behaviour -- facts land, an edit replaces
them, an unchanged file parses once -- and nothing asserted that a spawned shard could reach it,
which is the only thing that mattered in production. A capability test is not a wiring test.

So the change is: **an agent run establishes a CodeDOM scope over the files it is working on.**
The executor measures the focus (it already knows every path the turn reads and writes); the
kernel keeps every decision that follows. That adds a writer row to the ownership matrix in
`internal/world/world_predicates.go`, whose comment today lists only "CodeDOM scope (session)"
with a session lifetime — an agent turn is the run's equivalent of "what the user is looking
at", and gets a turn lifetime.

### D4 no blast-radius edit has ever been made

`replace_element`, `apply_edits`, `edited_refs`, `run_impacted_tests`: **zero calls in 11 runs.**
Every change in the corpus was made by line number. The doctrine's third purpose — *a change
with a blast radius is carried out by the tool, not by the LLM hand-editing* — has never once
been exercised in a measured run.

### D5 the focus entity is sometimes the literal `"."`

Early rounds of a run query `code_defines(".", …)`, `code_element(Ref, Kind, ".", …)` and
`dependency_link(".", Dep, Kind)` — all 0. A `nerd fix` brief names a symptom, not a file, so
there is no target on the first rounds and the focus falls back to the workspace root
(`internal/context/working_context.go:159`, `normalizeWorkingEntity(intentTarget, root)`). Later
rounds query real paths and return rows.

## What this says about the journey

The model's actual codedom journey is: **read raw text, edit by line number, re-list to recover
the line numbers the edit invalidated, repeat.** It never opens a scope, never edits an element,
never asks what a change touches. Every structural affordance codedom has — refs, parents,
signatures, visibility, impacted tests — is either unreachable from the tool registry or
unadvertised in the one sentence the model is given about it.

Ordered by measured cost:

1. **D3** — the scope layer. Decided and landed (`e78e7241` + the Spawner wiring); this was the
   whole of the gap between "the kernel has the math" and what a turn actually gets.
2. **D1** — the tool descriptions, and a JIT atom that fires when a task names a symbol. This is
   now the largest measured cost: 261 raw reads against 13 structural ones.
3. **D4** — follows from D1; the structural tools cannot be chosen while they are unadvertised
   and the line tools are the only ones that work.
4. ~~**D2**~~ — retracted above. The 17% refusal rate is real and was the right thing to notice;
   the conclusion drawn from it was not, and the check that would have caught that took four
   lines. **D5** is likewise not a defect: the `"."` focus is documented, deliberate behaviour.

What the retraction costs is worth stating: one 24-minute run spent fixing a non-problem, and a
number in a study that has to be walked back. What it bought is the rule that replaces the habit
that produced it -- **a finding does not enter a brief until the falsifying check has been run**,
and for a refusal the check is always "apply it and see whether the result is valid."

Then measure again: the read_file-to-structural ratio, the refusal rate, and how much of the
working section's code-fact budget is actually filled.

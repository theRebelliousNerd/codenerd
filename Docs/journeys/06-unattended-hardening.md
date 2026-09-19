# 06 — Unattended hardening: codeNERD running for weeks

Program of record for making codeNERD safe to leave running unmanaged. Sibling of the elite
ladder (`05-elite-harness-ladder.md`): the ladder measures what codeNERD can change; this measures
whether it can keep running while it does.

## Status

- last updated: 2026-09-19 12:30
- landed: **H1 run-level wall clocks**, all four steps -- the repair episode clock (`4316415f`),
  the CLI `--timeout` default (`02e4c8dd`), the `llm_timeouts` operation tier, per-shard and
  session ceilings (`f91c39b6`), campaign defaults (`856ff1fe`). What bounds a run now: the
  user's own `--timeout`, the progress stops (`working_stop`, the repair-attempt bound), and
  per-request bounds.
- next in H1: `defaultToolTimeout`; the client request bounds themselves (the OpenAI-compatible
  vendors' 10 m is a constant in `DefaultOpenAICompatConfig`). The critic's 3 m and 5 m landed
  `8958ffb0`: the review and its uplift round carry no clock of their own.
- open: the census tool (one inventory per class below, generated, not hand-kept); a ratchet
  test per class; limit hits and gaps asserted as facts the kernel can derive repair obligations
  from.

## The bar

Steve, 2026-09-19, after ladder run R1-4d's critic was cut by `nerd fix`'s 25-minute default:

> why is there a 25 minute timeout? there should not be timeouts like that... some agentic runs
> are like hours long.

> there is a lot in this codebase like hardcoded timeouts, arbitrary truncations, token limits,
> and probably entire classes of hardcoded things that should probably have repair loops and
> introspective systems that allow codenerd to heal many classes of its own bugs...

> and probably a lot of logging blind spots where we have logs that dont get persisted in the log
> files ... if we want this thing to be able to run for weeks unmanaged, we need to harden it in
> that direction...

What that requires, stated so each item can fail:

1. Nothing stops work that is progressing: no default wall clock and no count ceiling on a run.
   A limit the user sets is an opt-in constraint; unset means none.
2. What does stop a run is derived from evidence: a stall, a repeated failure, a cancel.
3. Every decision and every limit hit is persisted where both the kernel and a human can read
   it -- which limit, what it cut, what happened next.
4. Nothing grows without bound: logs, tables, caches, maps, history.
5. A failure becomes a repair obligation the harness derives, not a log line someone greps.

## How a finding is worked

Each member of a class is classified before it is touched:

| kind | meaning | action |
|---|---|---|
| request bound | bounds one model request, one HTTP call, one subprocess | keep; derive the value where a measured fact exists |
| derivable | a number standing in for a fact the harness can measure (a model's max tokens, a gate's duration, a window size) | replace with the measured fact |
| progress stop | a ceiling standing in for "it stopped making progress" | replace with a derived stall / repeated-failure stop |
| cruft | protects nothing | delete |

A finding with evidence (a symptom, `file:line`, what was observed) is also a ladder brief: hand
it to codeNERD when it is one or two files and not gate or permission logic; build it by hand
when it is, or when it stops codeNERD from running at all. Record which in the ladder's Runs
table.

## Classes

### H1 wall clocks on a run

Measured 2026-09-19 (grep of `WithTimeout` sites and duration literals; the census tool will
replace this list):

- **CLI `--timeout`** (`cmd/nerd/main.go:185`): global flag, default 25 minutes, wrapped around
  the whole run of `nerd fix` and the other direct actions (`cmd_direct_actions.go:310`),
  `campaign` start and resume (`cmd_campaign.go:297`, `:674`), `campaign recurse`
  (`cmd_campaign_recurse.go:139`), each interactive turn (`cmd_interactive.go:93`), `query`
  (`cmd_query.go:68`, `:202`), `init` (`cmd_init_scan.go:519`, and into the initializer's config
  at `:170`), `instruction` (`cmd_instruction.go:46`), `swebench` (`cmd_swebench.go:98`) and eight
  `browser` commands. `--timeout 0` was an already-expired deadline in all of those, and `dream`
  and `tool generate` silently turned 0 into 25 minutes (`cmd_advanced.go:156`, `:868`). R1-4d:
  the critic was cut at 25:01.
- **`llm_timeouts` operation tier** (`internal/config/llm_timeouts.go`, "calibrated for
  GLM-4.7"): shard execution 30 m (`session/spawner.go:344`, `session/task_executor.go:402`,
  chat delegation and helpers), OODA loop 30 m per chat turn (`chat/process.go:76`, `:890`,
  `cmd_chat.go:158`), campaign phase 30 m (`chat/campaign.go:52`), document processing 20 m
  (`chat/helpers_scan.go:234`, `chat/ingest.go:25`), Ouroboros 10 m (`chat/helpers_tools.go:175`).
  Articulation and follow-up (5 m each) bound one call: request bounds.
- **Campaign defaults** (`internal/campaign/orchestrator_init.go:330`, `:333`): 4 h per campaign,
  30 m per task; `campaign assault` 900 s per stage.
- **Init defaults**: `DefaultInitConfig` 30 m (`internal/init/initializer.go:123`), agent
  registration 30 m (`agents_registration.go:54`).
- **Repair episode clock**: 5 m plus the measured gate time (`session/repair_loop.go`). R1-4d:
  the coverage round's call was cut at 368 s with nothing returned. Step 1 removes it.
- Request bounds, kept for now: HTTP 10 m, per call 10 m, streaming 15 m, slot acquisition 10 m.
  **`criticTimeout` 3 m and `criticUpliftTimeout` 5 m** (`session/build_verify.go`) are request
  bounds tighter than the per-call bound, pinned by `TestCriticTimeouts_AreBounded` (review
  <= 5 m). They were set after a review hung for twenty minutes when the client had no bound of
  its own; it has one now. A reasoning model reviewing a large change can legitimately take
  longer (R1-4d's coverage call ran 368 s), and a cut review is a missing opinion: derivable
  (the slot's measured call durations), not a constant. Measured on the planner slot
  (muse-spark-1.3), 2026-09-19: R1-4e 2 m 11 s, R1-5 2 m 50 s (10 s from the cut; 15.5k output
  tokens for "NO FINDINGS"), R1-6 1 m 59 s, R1-7 2 m 20 s (one finding, pre-existing), and R1-8
  **cut at 3 m** ("adversarial review failed (context deadline exceeded); turn continues") -- on
  a change with a defect a review could have found (R1-6's critic ran on another such change and
  found nothing).
  **`defaultToolTimeout` 5 m** (`session/executor.go:363`) is per tool run, but this repo's full
  suite takes 5 minutes and `cmd/nerd/chat` alone 2-8 minutes under load, so a model's
  `run_tests` on either is cut: derivable (a command's measured duration), not a constant.
- Wait windows: `waitForSystemResults` 1.5 s / 1.2 s / 3 s at its call sites
  (`chat/process_dream.go:557`, `chat/process_dream_delegation.go:228`, `cmd_instruction.go:292`)
  -- unclassified.

### H2 truncations and count caps

Standing rule (Steve, 2026-09-10): never truncate model or tool output; a limited output goes back
to the model to be restated. Census pending; the 2026-09-10 audit's leftovers are the
`formatSpecialists` roster cap, the kernel-context row cap and `ClampLines`.

### H3 token numbers

Census pending: literals standing in for a model's real context window or output ceiling.

### H4 logging blind spots

Found 2026-09-19, while reconstructing R1-4d from its logs:

- The working policy logs when it closes exploration, never when it reopens it; R1-4d's reads
  after "closed exploration" had to be inferred.
- A repair attempt that ends in an error records `llm_calls=0 tool_calls=0` even when calls and
  tools completed before the error (R1-4d: one call, one read).
- Each receipt's `estimate_error_pct` is computed against uncached input only
  (`internal/broker/broker.go:195`), so an estimate accurate to 0.1% against input+cached reads as
  60% error; the reconciler and the calibrator already use input+cached.

Census pending: output that never reaches a log file (`fmt.Print*`, `log.Print*`, stderr outside
the CLI's own output), goroutines whose panics are not logged.

### H5 unbounded growth

Census pending: maps and slices that only grow, tables with no retention, logs with no rotation,
caches with no eviction.

Measured 2026-09-19 18:35, this workspace's `.nerd/` after the dogfood weeks: **~4 GB**, none of
it bounded by anything.

| path | size | shape |
|---|---|---|
| `marathon-supervision` | 1.2 G | per-run supervision artifacts |
| `knowledge.db` (+20 M wal) | 1.0 G | the knowledge graph |
| `campaigns` | 773 M | per-campaign artifacts |
| `logs_archive` | 711 M | rotated logs |
| `logs` | 159 M | 280 files, a set per run |
| `context` | 86 M | 170 working-context archives, one per executor (N13) |
| `prompts`, `shards`, `tools` | 34 M | |

A machine left running for weeks grows this without bound while the disk guard's floor is real.
The pieces differ in kind -- an archive a later run should reopen (N13), a log a reviewer may
want, a campaign artifact whose campaign is finished -- so the answer is a derived retention and
not a blanket delete: what a record is for, and whether anything can still read it, decides how
long it is kept. Hand-built when it is worked: it deletes files.

### H7 memory safety

A data race in Go is undefined behaviour, not a wrong value: a torn slice or interface header
writes through a stale pointer, and the damage lands wherever the layout puts it. For a process
meant to run for weeks that is the worst class there is -- the failure appears far from its cause,
in whatever runs next, and moves when unrelated code changes the binary's layout.

- **Found 2026-09-19, landing N03**: four chat tests drove the Bubble Tea model from 5-100
  goroutines (unlocked `append` to `m.history`, concurrent `View()`/`Update()`), swallowing panics
  with `recover()`. They wrote over a Go runtime global (the Green Tea GC's AVX-512 feature flag,
  proven with a linkname probe: false at init, true mid-run) and the GC then executed AVX-512 on a
  CPU without it (`0xc000001d` in `expandAVX512_60`). The trigger was an unrelated change to
  embedded `.mg` files that moved the data layout. The race detector: 25 reports in those four
  tests. Deleted -- Bubble Tea confines the model to its program goroutine, so there was no
  contract to test.
- Census pending: `go test -race` over the whole tree, then a ratchet (no new race reports).

### H6 swallowed failures


- **`HolographicCodeScope.ensureDeepFacts`** (`internal/system/holographic_code_scope.go:111`),
  the thesis's Case A (08): six silent exits -- a failed deep scan is a Warn and a return, the
  kernel's retract and load errors are `_ =` in both branches, a failed stat or map is
  `continue`, and `Open`/`Refresh` return nil regardless. The store-less branch records a fresh
  fingerprint after a failed load, so the file's facts stay missing until it changes again.

- **`RealKernel.Assert` accepts a fact whose arity disagrees with its `Decl`**, without an error.
  Found while landing F1 (`662699eb`): the corpus test still asserted the old 6-argument
  `turn_evidence` against the new 7-argument Decl, the assert succeeded, and the fact derived
  nothing. Any producer left on an old shape after a Decl change goes silent rather than failing.
  `validatePredicateDeclaration` exists (`core/mangle_updates.go`) but only the model-update
  filter calls it.
- **The campaign policy defines the same rules in two or three files**: `campaign_core.mg`,
  `campaign_phases.mg`, `campaign_planning.mg` and `campaign_tasks.mg` each carry
  `campaign_blocked /no_eligible_phases`, `/all_tasks_blocked`, `phase_eligible`,
  `has_incomplete_hard_dep`, `current_phase` and more. Rules union, so an edit to one copy leaves
  the other deriving the old conclusion; N03 had to change both copies of `/no_eligible_phases`.
  Consolidate to one home per rule.
- **A test ID inside a production gate.** `writeSetLockManager.acquire` skipped its workspace
  containment check when the task was named `"t1"` (`if taskID == "t1" { continue }`, since the
  2026-05-28 "sync"), so `TestWriteSetLockManager_TypeCoercion` could assert that a path climbing
  out of the workspace is granted. Any campaign task given that ID took a lease outside the
  workspace. Removed with the test corrected (L2). The class to hunt: a literal from a test file
  (an ID, a path, a name) compared inside non-test code -- a gate bent to fit a test instead of the
  test fixed. A census is a grep for the IDs the test files use (`"t1"`, `"task_1"`, `"test-`)
  against non-test sources. Run 2026-09-19 14:03
  over non-test Go (`(==|!=) "t1"|"task_1"|"test-..."|"mock..."|"fake-..."`): one hit, this one;
  `emitter.go`'s exact `["file_topology","test_state"]` check is placeholder detection (a model
  parroting the schema's example values), not a shim.

Census pending: `_ = err` on a path that matters, `recover()` that does not log, errors reduced to
a boolean.


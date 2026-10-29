# Stubs and swallowed errors

Date: 2026-09-09
Branch: `claude/codenerd-system-hardening-1xorkw`
Scope: round 2. Live code paths that report success they did not earn, and
failures on those paths that left no trace.

The pattern from round 1 held, in two variants:

> **A stub sits on a live path and returns the shape of success.** Nothing
> errors. No test goes red. The caller records a result it never got.

> **A failure is dropped as a bare statement.** The system continues with a
> different reality than the one it reports.

The single most consequential finding is neither of the twelve targets on the
list. It is the reason several of them were unfixable in place:

> **`core.Kernel.Assert` returned `nil` for a fact the kernel had thrown
> away.** Every `if err := kernel.Assert(f); err != nil` guard in the tree —
> including the ones added in this pass — was ornamental until that was fixed.
> See §B0.

---

## A. Live stubs

### A1. `ValidateProgram` computed the analysis and threw it away

`internal/mangle/schema_validator.go:311`

**Found.** The function parsed the program, ran `analysis.AnalyzeOneUnit`, then
discarded the result:

```go
// Suppress "programInfo declared but not used" error by using it
_ = programInfo
```

Validation was then `strings.Split(programText, "\n")` plus a substring test for
`:-`, calling `ValidateRule` on any line that matched. That stand-in was blind in
both directions:

- **False negative.** A rule wrapped across lines — which is nearly every rule in
  `internal/core/defaults/` — had only its FIRST fragment validated. The
  continuation lines carry no `:-`, so the scan skipped them entirely, and the
  fragment's body (`next_action(/review) :-` → body `""`) contained no predicates
  at all. Every predicate below the wrap went unchecked.
- **False positive.** A `:-` inside a string literal or a trailing comment was
  split on and the remainder validated as a rule body, so
  `note("… :- bar(X)").` reported `bar` undefined.

A `nil` return from whole-program validation therefore meant almost nothing.

**Did.** Validation now walks `programInfo.Rules`. Premise predicates come from
the desugared AST (`ast.Atom`, `ast.NegAtom`, `ast.TemporalAtom`,
`ast.TemporalLiteral`), which has already resolved wrapping, negation, string
literals and comments. A body predicate is sourced if the system schema declares
it, if Mangle builds it in (`PredicateSym.IsBuiltin` / `IsInternalPredicate`), or
if the program itself produces it (rule head or standalone fact). Anything else
is drift: analysis accepts it because the program declared it locally, and it
never fires because nothing in the running system asserts it.

The docstring now states what a `nil` return buys, so callers do not read it as
"this program is correct" — syntax comes from `ParseUnit`, binding and
stratification from `AnalyzeOneUnit`, both returned verbatim.

**Pinned by** `internal/mangle/schema_validator_program_test.go`:
`TestValidateProgram_WrappedRuleBodyIsValidated` (the false negative),
`TestValidateProgram_ColonDashInsideStringIsNotARule` (the false positive),
plus locally-derived-predicate and negation cases.

---

### A2. `swebench_evaluate` scored predictions without running tests

`internal/core/virtual_store_python.go:500`

**Found.** The handler asserted `swebench_evaluation_started` and
`swebench_environment(…, /evaluating, …)` and returned `Success: true` — having
executed nothing. `benchmarks.mg` declares
`swebench_evaluation_result(InstanceID, Resolved, PassedCount, FailedCount)` and
derives `swebench_resolved/1` from it; **no Go code in the repo has ever produced
that fact.** A harness reading the action result saw every prediction "evaluate"
cleanly while the environment sat in a `/evaluating` state nothing advances.

**Did.** Refused honestly. The handler returns `Success: false` with an error
naming the instance, the model and the patch size, logs at Error, and parks the
environment in `/error` — the state `benchmarks.mg` already models — instead of a
phantom `/evaluating`. It no longer asserts `swebench_evaluation_started` for an
evaluation that never started.

**Not done, and why.** Scoring needs a live container.
`swebench.Harness.Evaluate` (`internal/tactile/swebench/harness.go:125`) applies
the patch, runs FAIL_TO_PASS and PASS_TO_PASS in the instance image and computes
the verdict — it is real. But `VirtualStore` holds no harness and no
`PersistentDockerExecutor`, and the rest of the SWE-bench family never creates a
container either (`swebench_environment.ContainerID` is `""` on every path:
setup, apply_patch, run_tests, snapshot, restore). Wiring a harness in requires a
field on `VirtualStore` (`internal/core/virtual_store.go`, owned by another
agent) and is a feature, not a hardening fix. Adding an unused evaluator hook
would have recreated the exact defect this pass exists to remove.

The rest of the SWE-bench handlers are equally fact-only, but they record
lifecycle intent; `evaluate` is the one whose `Success: true` means "the
benchmark ran". Left as-is deliberately — listed here so it is not rediscovered
as new.

**Pinned by** `internal/core/virtual_store_python_test.go`
`TestVirtualStorePython_SWEBenchEvaluate`, rewritten: it now fails if the handler
reports success, if it asserts `swebench_evaluation_started`, or if the
environment is not parked in `/error`.

---

### A3. `delete_lines` post-action validation was unreachable *and* vacuous

`internal/core/validator_codedom.go:277` (`LineEditValidator.Validate`)

**Found.** Two independent defects in the same branch:

- **Unreachable.** It read the range with `req.Payload["start_line"].(int)`.
  Every payload reaching VirtualStore from the kernel or a tool call has been
  through JSON, so those values are `float64` — `handleDeleteLines` itself reads
  `.(float64)`. `hasStart`/`hasEnd` were false on every production delete, which
  fell through to the generic tail: `Verified: true, Confidence: 0.85`.
- **Vacuous when reached.** On the Go-caller path that did pass ints, the only
  check was `len(strings.Split(string(content), "\n")) == 0`, which no string can
  satisfy. It returned `Verified: true, Confidence: 0.8` for every delete,
  including one that removed nothing at all.

The comment said the previous line count was unknowable. It is not needed.

**Did.** `validateDeleteLines` now:

| Check | Source | Verdict on failure |
|---|---|---|
| Range parsed (int, int64, float64, json.Number, string) | `payloadInt` | `Verified: true, Confidence: 0.3` with the observed payload type — only readability was checked, and it says so |
| `lines_deleted > 0` | handler metadata | not verified — "removed nothing" |
| `lines_deleted <= end-start+1` | handler metadata | not verified — impossible count |
| file retains ≥ `start_line-1` lines | file on disk | not verified — over-deletion outside the range |
| short count ⟹ file truncated at exactly `start_line-1` | both | not verified — count and file disagree |

`FileEditor.DeleteLines` clamps `end_line` to the file length and reports what it
actually removed as `metadata["lines_deleted"]`, and the untouched prefix pins
the floor — so the previous count is not required. Confidence now reports what
was checked: 0.9 for count-and-floor, 0.5 when the handler supplied no count,
0.3 when the range never arrived. Line counting matches `FileEditor`'s
`bufio.Scanner` semantics (no phantom trailing line) and normalizes CRLF.

**Pinned by** `internal/core/validator_codedom_delete_lines_test.go`, seven
cases including `..._ValidatesJSONNumberPayload` (the unreachability) and
`..._NoOpIsNotVerified` (the vacuity).

---

### A4. `/review` consulted no specialists and no knowledge

`cmd/nerd/chat/review_aggregator.go`

**Found.** Two stubs, both on the live path:

- `matchSpecialistsForReview` (`:97`) — `return nil`, under a comment saying the
  JIT executor dispatches specialists directly. It does not:
  `spawnMultiShardReview` (`:340`) calls this, ranges over the result and spawns
  one shard per match. `/review` ran the generic reviewer alone on every project,
  logging "Matched 0 specialists for review" — which reads as a result, not a
  stub.
- `loadAndQueryKnowledgeBase` (`:119`) — `// Stub: return empty knowledge`. Every
  specialist task was built with an empty `Knowledge` field, so the entire
  `/ingest` pipeline (which writes `knowledge_atoms` and `prompt_atoms` into each
  agent's own SQLite DB) fed nothing back into review.

A third, found while fixing those: `formatSpecialistReviewTask` (`:264`) returned
`"review files for <agent>"` and **dropped both `Files` and `Knowledge`**. Fixing
the two stubs without this would have produced capability that runs and changes
nothing — the same defect in a new place.

**Did.**

- `matchSpecialistsForReview` delegates to `shards.MatchSpecialistsForTask` with
  the `/review` verb config (min confidence 0.3, ≤3 specialists, parallel). That
  matcher is real, tested (`internal/shards/matching_test.go`), and already
  drives `/fix`, `/refactor` and `/create` through `spawnShardWithSpecialists`.
- The chat-local `RegisteredAgent` gained `KBSize` and **`Status`**. Status is
  load-bearing: `getAvailableAgents` only considers agents marked `"ready"`, so
  decoding `.nerd/agents.json` without it made every agent unavailable — the
  delegation adapter would have matched nothing even after the stub was replaced.
- `loadAndQueryKnowledgeBase` opens the agent DB (ProfileQuery pragmas) and runs
  `store.SearchKnowledgeAtomsLexicalDB` with keywords built from the review's
  file base names and extensions. Bounded: 5 atoms, 400 chars each — it is
  user-ingested free text riding into a prompt. A missing DB is not an error
  (normal for a never-ingested agent) and does not create one; a failing query
  is returned, so a corrupt DB is distinguishable from an empty one.
- `formatSpecialistReviewTask` emits `review files:<comma list>` plus the
  knowledge block, matching the reviewer's own `baseTask` and the parallel-mode
  specialist task in `delegation_modes.go:39`. `buildSpecialistTask` prefers the
  specialist's matched files over the whole review set.

**Pinned by** `cmd/nerd/chat/review_aggregator_specialists_test.go`, six cases
including `..._StatusIsDecodedFromRegistryJSON` and
`TestFormatSpecialistReviewTask_CarriesFilesAndKnowledge`.

---

## B. Swallowed errors

### B0. `Assert` reported success for facts it rejected (not on the list; found while fixing B)

`internal/core/kernel_facts.go`

**Found.** `addFactIfNewLocked` returned a single bool for two opposite
outcomes — *already present* (a no-op) and *rejected, and never going to be
present*. `Assert` read that bool as "duplicate" and returned `nil` for both:

```go
if !k.addFactIfNewLocked(fact) {
    // Duplicate assert is a no-op — suppress debug to avoid log spam
    k.mu.Unlock()
    return nil
}
```

The dominant rejection is `coerceAtomToDeclLocked` refusing a fractional float in
a slot the `Decl` bounds `/number` — which it must, because this Mangle fork
compares int64 only and one such fact aborts the whole fixpoint. Reproduced
directly: `Assert(dream_preference("…", 0.85))` returns `nil` and the fact is not
in the store afterwards. The EDB-cap path did the same thing.

This is why several targets below could not be fixed in place: adding an error
check to a call whose callee never reports errors changes nothing.

**Did.** Split into `addFactIfNewLockedErr(f) (bool, error)`; the old bool
wrapper stays for the callers that legitimately do not care (`LoadFacts`,
`AssertWithoutEval`, transactions). `Assert` returns the rejection.
`AssertBatch` keeps the facts it could add — a batch is a convenience, not a
transaction — and returns a joined error naming those it could not, so the
retry-individually fallbacks have something to act on. A duplicate is still
`(false, nil)`.

**Pinned by** `internal/core/kernel_assert_rejection_test.go`:
`TestAssert_RejectedFactReturnsError`,
`TestAssert_DuplicateIsStillANoOpNotAnError`,
`TestAssertBatch_ReportsRejectedFactsAndKeepsTheRest`.

---

### B1. Campaign context paging — nine bare `Assert` calls

`internal/campaign/context_pager.go`

**Found.** Seven sites had the same shape: `AssertBatch` (atomic — one bad fact
rejects the slice), and on failure a per-fact retry loop with the error dropped
as a bare statement. A paging pass that landed nothing looked identical to one
that landed everything, so a phase could run with the wrong activation profile —
wrong files boosted, browser schemas never suppressed, compressed atoms never
decayed — with nothing anywhere saying why.

The remaining two (`:443` `boostPattern`, `:452` `suppressSchema`) were
**unused private methods**: grep across the repo found only their own
definitions. Their entire body was a swallowed `Assert`.

**Did.** The seven fallbacks now call one helper,
`assertFactsWithFallback(kernel, facts, what)` (`internal/campaign/kernel_assert.go`).
The fallback itself is deliberate and stays. The silence does not: it counts
per-fact failures and logs at Warn with the count, the batch error and the first
per-fact cause. These facts are advisory — activation weights and phase context
atoms that steer which facts the JIT prompt sees, and the campaign executes
without them — so a partial failure is tolerated rather than escalated, and the
comment says so.

`boostPattern` and `suppressSchema` deleted (audit-before-delete: no callers, no
test references; they duplicated the inline batch logic in `ActivatePhase` steps
2 and 4 at worse granularity).

**Pinned by** `internal/campaign/kernel_assert_test.go`
`TestAssertFactsWithFallback_GoodFactsLandWhenTheBatchIsRejected`.

---

### B2. Campaign phase and task state facts

| Site | Fact | Consequence of the drop | Now |
|---|---|---|---|
| `orchestrator_phases.go:306` (`startNextPhase`) | `campaign_phase(…, /in_progress, …)` | Preceded by a `RetractFact`, so the kernel is left with **no** row for the phase: `phase_eligible`, task gating and every phase-scoped rule go blind while memory says in-progress | logs at Error and returns a wrapped error; `startNextPhase` already returns `error` and the run loop records `lastError` and retries |
| `orchestrator_phases.go:386` (`completePhase`) | `campaign_phase(…, /completed, …)` | Same retract-then-assert hazard; the next phase may never become eligible | logged at Error; no error to return and the in-memory campaign is already correct |
| `orchestrator_tasks.go:580` (`completeTask`) | `task_result(ID, /success, …)` | The kernel keeps seeing the task in flight while the campaign counts it done — the split that stalls a campaign with no recorded cause | logged at Error |
| `orchestrator_utils.go:127` (`runPhaseCheckpoint`) | `phase_checkpoint(…)` | A verification that did not happen, as far as every downstream rule is concerned, while the loop still returns `allPassed` | logged at Error; the checkpoint verdict itself is unaffected, so it is not failed |

---

### B3. `_ = tx.Commit()` — four sites

A failed commit discards the **whole** buffered batch.

| Site | Batch | Now |
|---|---|---|
| `campaign/orchestrator_execution.go:307` | `campaign_heartbeat` retract + assert | Warn — the next tick retries, but a stale (or, post-retract, absent) heartbeat reads to the health rules as a campaign that stopped breathing |
| `campaign/orchestrator_lifecycle.go:173` (`resetInProgress`) | every `campaign_task` this restart moved back to pending | Error — dropped, the kernel believes those tasks are still in flight while memory has them pending: nothing schedules them, nothing completes them |
| `shards/system/executive.go:475,486` | `executive_error(Message, Timestamp)` | both call sites now route through `recordExecutiveError`, which logs at Error — the fact is the only trace of a policy-evaluation failure that any rule or UI can see |

The non-transactional branch of the heartbeat (`_ = o.kernel.Assert(...)`) was
fixed alongside it, for the same reason.

---

### B4. `_ = o.saveCampaign()` — nine sites

`orchestrator_{execution,phases,failure,lifecycle}.go`, `assault_tasks.go`

**Found.** `saveCampaign` logs its own cause, but every caller dropped the
outcome, so nothing recorded *which* checkpoint was lost and no operator-facing
surface heard about it at all. A campaign kept running with completed phases,
replans and autosaves that existed only in memory; a crash rolled it back to
whichever snapshot happened to succeed.

**Did.** All nine call `o.persistCampaign(checkpoint)`
(`orchestrator_lifecycle.go`), which logs at Error naming the checkpoint and the
campaign, and emits `EventSnapshotWriteFailed` — added to the closed event set in
`orchestrator_events.go` and rendered by both campaign event listeners in
`cmd/nerd/cmd_campaign.go` as `⚠️ Campaign snapshot NOT written — progress is
not on disk`. Persistence failure is not recoverable in place (the in-memory
campaign is still correct), so it is reported, not escalated. Checkpoints:
`cancellation pause`, `campaign completion`, `pause`, `autosave`, `phase
completion`, `attempt-cap replan`, `task failure`, `restart reset`, `assault
phase expansion`.

**Pinned by** `internal/campaign/persist_campaign_test.go`, including
`TestSnapshotWriteFailedIsAKnownEventType` — a new event type that is not in the
closed set is dropped by every UI's default branch, which is the invisibility
`orchestrator_events.go` exists to prevent.

---

### B5. Shadow Mode failed open

`internal/core/shadow_mode.go`

Shadow Mode is the safety pre-flight: it projects an action's effects into a
cloned kernel and asks the constitutional rules what they imply. Three kernel
writes dropped their errors, and **every one of them failed open** — a fact that
never lands cannot trigger a violation, so the action was pronounced safe
precisely because the evidence against it was lost. Same shape as the
`queryOrBlock` fix already in `checkViolations`: an empty result must mean "the
kernel answered and found nothing", never "the kernel was not told".

| Site | Was | Now |
|---|---|---|
| `:152` `StartSimulation` | `shadowKernel.Assert(shadowStateFact)` — the fact that marks the shadow kernel a live simulation, which the projection rules gate on | returns an error and unwinds `simulations`/`activeSimID`/`shadowKernel` so the shadow mode is not left wedged as "already active" |
| `:217` `SimulateAction` | `sm.shadowKernel.Assert(effectFact)` for each projected effect, immediately before `checkViolations` queries for what they imply | marks the simulation unsafe and failed, and returns an error — an incomplete projection yields no verdict |
| `:423` `CommitSimulation` | `parentKernel.Assert` / `RetractExactFact` per effect | every effect is still attempted (stopping halfway is worse), then failures are joined and returned; the commit no longer returns `nil` having applied part of itself |

**Pinned by** `internal/core/shadow_mode_assert_test.go` (forces rejection by
capping the EDB at its current size).

---

### B6. `file_written` after a committed transaction

`internal/core/transaction_manager.go:409`

The files are already on disk and the transaction is committed, so a rejected
fact is **not** grounds to fail the commit — a rollback here would undo work that
succeeded. But `file_written` is what the impact chain, test selection and the
world model key off, so a lost fact makes the edit invisible downstream while the
commit reports success. Now best-effort with a per-file Error log plus a summary
count. The comment states why it is not escalated.

---

### B7. `DreamRouter` — the three learning predicates never landed

`internal/core/dream_router.go`

**Found.** `dream_tool_need`, `dream_risk_pattern` and `dream_preference` are all
declared `/number` in their confidence slot (`schemas_dreamer.mg:48,53,57`). The
router asserted `l.Confidence` — a raw 0..1 float. Every one was rejected by the
kernel's `Decl` coercion, silently (see §B0), on every route since the predicates
were added. Two of the three then set `result.Success = true` with
`Destination: "Kernel:…"`, and `RouteLearnings` marked the learning `Persisted`,
so it was never retried either. A complete Dream State learning loop with a
Decl, a Go producer, and no rows.

**Did.** All three scale with `types.PercentFromRatio` (0.85 → 85, the repo
convention), check the `Assert` error, and — on the branches where the kernel is
the only destination — report `Success: false` with the cause rather than
claiming a destination that holds nothing.

**Pinned by** `internal/core/dream_router_persist_test.go`, three cases that
route a learning and then **query the kernel** for the row, asserting the stored
confidence is integer percent.

---

## C. Unchecked deserialization

### C1. Prompt atom selector dimensions

`internal/store/local_prompt.go:311` (`scanPromptAtoms`)

Thirteen bare `json.Unmarshal` calls hydrating the columns that **are** the
atom's selector — the ones deciding whether the JIT compiler includes it for a
given mode, phase, verb, language or shard. A malformed column produced an atom
with no dimensions, which does not error, does not warn and does not disappear:
it stays in the pool and matches nothing — or, for a dimension where empty means
*any*, matches everything.

Now `decodeAtomSelector(atomID, column, raw, dst)`: empty/`null` is normal and
leaves the slice nil; a column that is present and does not parse is logged at
Warn with the atom id and column name, and the dimension is left nil rather than
half-decoded.

**Pinned by** `internal/store/local_prompt_selector_test.go` (round-trip through
a real store, plus corrupt- and empty-column cases).

### C2. Tool learning store loaded a corrupt file as empty — then overwrote it

`internal/autopoiesis/feedback.go:545` (`LearningStore.load`)

`json.Unmarshal(data, &ls.learnings)` with the error dropped. A truncated or
half-written `tool_learnings.json` loaded as an empty store — every tool's
success rate, known issues and anti-patterns gone — with no error, no log line
and no difference from a first run. **Worse: the next `saveBytes` overwrote the
damaged file with that empty map**, making the loss permanent and destroying the
evidence.

Now: decode into a temporary map (so a partial parse cannot merge into a live
store), log at Error, and move the file aside as
`tool_learnings.json.corrupt-<unix>` — which both preserves it for recovery and
stops the next save from silently overwriting it.

**Pinned by** `internal/autopoiesis/feedback_load_test.go`
`..._CorruptFileIsPreservedNotSilentlyDiscarded`.

### C3. Learning statistics reported zero for a broken store

`internal/store/learning.go:426,430,435,440` (`GetStats`)

Two `QueryRow.Scan`s, a `Query` and a `rows.Scan`, all unchecked. A shard whose
`learnings` table was missing, locked or schema-drifted reported
`total_learnings: 0`, `avg_confidence: 0` and an empty `by_predicate` — exactly
what a healthy shard that has learned nothing reports. "The agent has learned
nothing yet" and "the learning store is broken" are opposite conclusions and this
function could not tell them apart.

All four now propagate, with the shard named. `AVG` over an empty table is `NULL`
— which is not an error but the honest answer for a shard with no learnings — so
it scans through `sql.NullFloat64` rather than failing into a swallowed zero.

**Pinned by** `internal/store/learning_stats_test.go`, including
`..._BrokenTableIsReported` (drops the table) and `..._EmptyShardIsNotAnError`.

### C4. Prompt-evolution feedback collector

`internal/autopoiesis/prompt_evolution/feedback_collector.go`

- `loadStats` (`:182,185`) — both `COUNT` Scans unchecked, so a missing or
  unreadable `execution_records` table left `totalRecorded`/`totalFailures` at
  zero, indistinguishable from a collector that has recorded nothing. The
  failure-driven evolution trigger simply never fired. Now logged at Error, and
  the counters are left **stale rather than zeroed** by a failed Scan.
- `scanRecords` (`:485,488,497,512,516`) — five bare unmarshals for
  `agent_actions`, `execution_result`, `atom_ids`, `grounding_sources` and
  `verdict`. A corrupt column produced a record with no actions or no result,
  which downstream reads as a task that apparently did nothing — and prompt
  evolution scores atoms on exactly these fields. Each failure is now logged with
  the record id and column. The record is still returned (a partially-readable
  history beats a dropped one) but a half-decoded `verdict` is nulled: it would
  otherwise count as a judged execution with an empty verdict string, which reads
  as neither PASS nor FAIL everywhere it is consumed.

**Pinned by** `internal/autopoiesis/prompt_evolution/feedback_collector_stats_test.go`.

---

## Deliberately not changed

| Thing | Why |
|---|---|
| The other six SWE-bench handlers (`setup`, `apply_patch`, `run_tests`, `snapshot`, `restore`, `teardown`) | Fact-only like `evaluate`, but they record lifecycle *intent*, not a verdict. `evaluate` is the one whose `Success: true` means "the benchmark ran". Wiring the harness needs a `VirtualStore` field (another agent's file) and is a feature. |
| `approach_learned` (`dream_router.go:158`) | `routeProcedural` saves it to the learning store under a predicate name with **no `Decl` anywhere in the tree**. Hydrating it into the kernel would be schema drift. Adding the `Decl` means editing `internal/core/defaults/**`, owned by another agent. Reported, not fixed. |
| `buildSpecialistTask` receiving the whole review file set from `spawnMultiShardReview:548` | The caller's choice, pre-existing. Mitigated inside `buildSpecialistTask`, which now prefers the specialist's own matched files. |
| The chat TUI's `campaignEventMsg` handler (`model_update.go:291`) | It drops **every** orchestrator event, not just the new one. Fixing that is a UI feature, out of scope. The CLI listeners, which are the operator surface for campaigns, do render `snapshot_write_failed`. |
| `AssertWithoutEval`, `LoadFacts`, kernel transactions | Left on the bool-returning `addFactIfNewLocked` wrapper. These callers batch large fact sets where a per-fact rejection is genuinely tolerable, and `assertWithoutEvalChecked` already exists for the callers that must fail closed. |

---

## Verification

`go build ./...`, `go vet ./...` clean. `go test ./...` green.

New regression tests, each naming the defect it pins:

| File | Cases |
|---|---|
| `internal/mangle/schema_validator_program_test.go` | 4 |
| `internal/core/validator_codedom_delete_lines_test.go` | 7 |
| `internal/core/kernel_assert_rejection_test.go` | 3 |
| `internal/core/shadow_mode_assert_test.go` | 2 |
| `internal/core/dream_router_persist_test.go` | 3 |
| `internal/campaign/kernel_assert_test.go` | 2 |
| `internal/campaign/persist_campaign_test.go` | 3 |
| `cmd/nerd/chat/review_aggregator_specialists_test.go` | 6 |
| `internal/store/local_prompt_selector_test.go` | 3 |
| `internal/store/learning_stats_test.go` | 3 |
| `internal/autopoiesis/feedback_load_test.go` | 2 |
| `internal/autopoiesis/prompt_evolution/feedback_collector_stats_test.go` | 2 |

Rewritten: `internal/core/virtual_store_python_test.go`
`TestVirtualStorePython_SWEBenchEvaluate` — it previously asserted the fake
success.

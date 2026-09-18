# 02 — Forcing and Completion

> What forces the model to keep working, and what merely lets it stop.
>
> This document studies the second bullet of "## The Vision (Steve, 2026-09-18)" in the root
> `CLAUDE.md`: *the harness forces*. Test coverage is not optional; domain knowledge is not at
> the LLM's discretion; the agent keeps working not until the task is done, but until the
> behavior the north star envisions holds.

## Status

last updated: 2026-09-18 04:25

done:
- 1. Inventory of forcing mechanisms (35 rows) + §1a "The north star as a forcing mechanism — pressure or decoration?"
- 2. The tool budget, exactly (files, config keys, log lines, history, deletion analysis, replacement sketch)
- 3. Coverage, including the tester shard (four delegation rules; `needs_more_tests/1` is dead)
- 4. Who decides "done", per path (all five paths, with the six terminating lines of the tool loop)
- 5. Delivered knowledge as an obligation
- 6. Design: the obligation fixpoint (north-star-sourced, per the architect's 2026-09-18 guidance)
- 7. Seams found (S1-S12)

open:
- The five predicates in §6.3 with no producer today — `requirement_witnessed/1`,
  `turn_claims_requirement/2`, `family_requires_atom/2`, `atom_in_window/2`,
  `turn_touches_constrained_surface/2`. Only `atom_in_window/2` has an obvious mechanical
  producer. **Unverified** whether the other four can be derived without a model judgment.
- Whether `northstar_capability.Timeline` (`/now`, `/6mo`, `/1yr`, `/3yr`, `/moonshot`) should
  weight the gap, and how. Today it only sorts capabilities into injection buckets **[REFUTED - see Verification]**
  (`prompt_northstar.mg:90-105`). **Not investigated.**
- The `nerd fix` / `nerd run` CLI entry paths were traced only as far as the shared
  `runToolLoop`; whether either wraps it in an outer loop was **not verified**.
- `internal/session/critic.go` (15 707 bytes) and `repair_loop.go` (16 603 bytes) were read only
  at their call sites. The critic is documented as advisory (`executor_tools.go:507-508`) but its
  internal escalation rules were **not read**.
- `internal/campaign/` phase-gate wiring was verified in policy and in
  `orchestrator_phases.go` / `orchestrator_execution.go`; `orchestrator_tasks.go` (who sets a task
  `/completed`, and on what evidence) was **not read** — this matters, because it is the input to
  the one genuinely fixpoint-shaped completion rule in the system.
- `llm_timeouts` was inventoried from the config-schema rule and one error site; the ~25 call
  sites named in that rule were **not enumerated**.
- No measurement was taken of the proposed fixpoint's evaluation cost; §6.8's risk 2 is reasoned,
  not measured.

### Headline correction to the brief

The brief's premise was that `read_only_stall` and `verify_after_write` are Go. **They are not,
today.** Both are Mangle rules in `internal/context/working_set.mg:85` and
`internal/context/working_set.mg:97`. The Go loop counts and the policy decides — see §2.
The thing that is still Go is the *counting* and the *ceiling arithmetic*
(`internal/session/tool_budget_controller.go`), and a second, older count ceiling that runs
*in parallel* with the policy on every non-progress-driven path. That duplication is the
tool budget the architect wants nuked, and §2 says exactly what it costs to remove.

---

## 1. Inventory of forcing mechanisms

Every row below is verified against the cited line. "Who computes the signal" distinguishes the
*measurement* from the *decision* — the repo's own phrase for the split is "Go measures, Mangle
decides" (`internal/core/defaults/policy/coder_safety.mg:68-70`).

| # | Name | Where computed | Forces | On what signal | Who computes the signal | What the user sees | Can the model talk past it? | Evidence |
|---|------|----------------|--------|----------------|--------------------------|--------------------|------------------------------|----------|
| 1 | `toolBudgetController` iteration ceiling | Go — `internal/session/tool_budget_controller.go:60` `newToolBudgetController`, consumed at `internal/session/executor_tools.go:207` | **stop** (forces a final answer) | `iter >= budget.iterationLimit` | Go (pure counting) | log `Tool iteration budget reached: %d rounds (base %d, hard %d, extensions %d/%d)` — `executor_tools.go:430` | No — it is a `for` bound | `executor_tools.go:207,393,428` |
| 2 | Adaptive extension grant | Go — `tool_budget_controller.go:192` `maybeExtend` | **continue** (adds rounds) | novel successful tool evidence since last boundary, no repeated tail cycle, extension capacity remains | Go (SHA-256 fingerprints of call+args+status+content, `tool_budget_controller.go:163`) | log `Adaptive tool budget extended by %d rounds to %d ...` / `... refused extension at %d rounds ... : %s` — `executor_tools.go:396,400` | No — fingerprints are of executed results, not prose. But a model that varies its arguments produces "novel" events cheaply (see §7) | `tool_budget_controller.go:192-238` |
| 3 | `read_only_stall` | **Mangle** — `internal/context/working_set.mg:85` `working_stop(/read_only_stall)` | **stop** (turn fails with `task unresolved`) | write-oriented intent, `Writes = 0`, `Rounds >= working_stall_rounds(24)` | Go asserts counts (`tool_budget_controller.go:334` `workingProgress`); Mangle decides | error `task unresolved: working continuation stopped by policy /read_only_stall after N executed tools` — `executor_tools.go:288` | No | `working_set.mg:85-87`, `executor_tools.go:277-289` |
| 4 | `verify_after_write` | **Mangle** — `internal/context/working_set.mg:97` `working_finalize(/verify_after_write)` | **verify** (ends exploration, harness runs the build/test gate itself) | `Writes > 0`, `SinceWrite >= 16`, `SinceVerify >= 16` | Go counts; Mangle decides | log `Working policy finalized the turn (%s) after %d executed tool call(s); forcing a final answer` — `executor_tools.go:216` | No | `working_set.mg:93-99`, `executor_tools.go:211-220` |
| 5 | `/commit` regime (closes reading) | **Mangle** — `working_set.mg:60,68,76` `working_regime(/commit)` | **continue, narrowed** (tool catalog shrinks to write+verify+recall) | write intent and 16 rounds with no write; or wrote then 8 rounds with neither write nor verify; or already committed and still unverified | Go counts; Mangle decides | log `Working policy closed exploration (commit regime) after %d executed tool call(s)` (`executor_tools.go:298`) + a nudge appended to the last tool result: `workingRegimeText` — `tool_budget_controller.go:365` | No — the tool catalog itself narrows | `working_set.mg:40-78`, `executor_tools.go:295-302` |
| 6 | `working_stop(/repeated_cycle)` | **Mangle** — `working_set.mg:80` | **stop** | `working_control(/yes, _)` — deterministic period-1/2/3 tail cycle | Go detects the cycle (`tool_budget_controller.go:276` `repeatedTailCycle`); Mangle decides it is a stop | error `task unresolved: ... stopped by policy /repeated_cycle` | No | `working_set.mg:80`, `tool_budget_controller.go:276-302` |
| 7 | `working_stop(/tool_failures)` | **Mangle** — `working_set.mg:81` | **stop** | 3 consecutive rounds in which every tool errored | Go counts `failedRounds` (`executor_tools.go:261-268`); Mangle decides | error `task unresolved: ... stopped by policy /tool_failures` | No | `working_set.mg:81`, `executor_tools.go:262` |
| 8 | `working_nudge(/implement \| /verify \| /conclude)` | **Mangle** — `working_set.mg:102,105,108` | **continue, steered** (prose appended to the round's last tool result) | 8 rounds no write (implement); wrote and 3+ rounds with no write and no verify (verify); 8 rounds of read intent (conclude) | Go counts; Mangle decides; **wording is Go** — `tool_budget_controller.go:351` `workingNudgeText` | An `[orchestrator] ...` line on the last tool result | **Yes** — it is prose the model may ignore; only the regime change at round 16 has teeth | `working_set.mg:101-110`, `tool_budget_controller.go:351-361` |
| 9 | `budget.nudge` (count warning) | Go — `tool_budget_controller.go:304` | **continue, steered** | rounds/calls remaining thresholds (`roundsLeft <= 2 \|\| callsLeft <= 4` = critical) | Go | `[orchestrator] Orchestrator budget: N tool calls and M rounds remain. ...` | **Yes** — prose | `tool_budget_controller.go:304-330`, `executor_tools.go:253-260` |
| 10 | `hollow_success/1` | **Mangle** — `internal/core/defaults/policy/coder_safety.mg:92-107` | **stop as failure** (the turn is not `turn_done`) | `turn_evidence(Verb, ToolCount, WriteCount, TestCount, ClaimedOutput, DreamMode)` — one fact per turn asserted by Go | Go measures the five counts; **Mangle derives the verdict** | (see §4 — pending) | No, for the four derived reasons; but the evidence is counts of tool names, so a write by a tool Go does not classify as a write-mutation tool is invisible | `coder_safety.mg:68-107` |
| 11 | `turn_done/1` | **Mangle** — `coder_safety.mg:115` | **stop (permits completion)** | `turn_executed(Verb)` (no hollow_success, build not failing) **and** `turn_acceptance(Verb, Contract, Snapshot)` | Mangle, over Go-asserted evidence | (see §4 — pending) | No | `coder_safety.mg:108-115` |
| 12 | `missing_test_for/1` → `hollow_success("new source was created without a test file")` | **Mangle** — `coder_safety.mg:28,104` | **stop as failure** | `created_source(File)` and no `test_coverage(File)`; `test_coverage` derives from `test_file_for` (world scanner's `x_test.go` convention) | world scanner asserts `test_file_for`; Go asserts `turn_created_source`; Mangle decides | (see §3/§4) | Partly — the coverage predicate is deliberately conservative and only recognizes the `x_test.go` convention (`coder_safety.mg:12-18`), and it only fires for *created* source, never for modified source | `coder_safety.mg:19-28,104-107` |
| 13 | `unverified_test_claim/1` | **Mangle** — `coder_safety.mg:39` | **stop as failure** (via `hollow_success` line 101) **[REFUTED - see Verification]** | `claimed_test_output(Verb)` and not `executed_test_tool(Verb)` | Go detects the claim in the response text and the tool execution; Mangle decides | (see §3) | No — this is precisely the "verdict contradicts its own evidence" guard | `coder_safety.mg:30-39,101-103` |
| 14 | `runWriteGuards` — nerd.md guard | Go — `internal/session/write_guards.go:32` → `projectForbidsWrite` | **stop the write** (pre-write) | write-mutation tool targets a path `nerd.md` protects | Go | `blocked by nerd.md: %s is write-protected (%s)` + audit `SafetyCheck("nerd.md_write_guard", false, ...)` | No — refused before the tool runs | `write_guards.go:32-38` |
| 15 | `runWriteGuards` — placeholder test guard | Go — `write_guards.go:39` → `writesPlaceholderTestFile` (`executor_tools.go:1379`) | **stop the write** (pre-write) | a `*_test.go` write whose content is a placeholder | Go | `blocked by placeholder test guard: %s is placeholder (%s)` | No. Its own comment is the design argument: *"Prose is a request the model complies with most of the time, while a fact checked before the tool runs is one no amount of model conviction gets past."* | `executor_tools.go:1369-1400`, `write_guards.go:39-45` |
| 16 | `runWriteGuards` — modularity guard | Go — `write_guards.go:46` → `modularityGuard` (`internal/session/modularity.go`) | **stop the write** (pre-write) | file-size / structure rule | Go | `blocked by modularity guard: %s` | No — and the guard *set* is write-protected as one unit because *"Observed 2026-08-12: when the modularity guard refused a write, the agent's next action was to edit the guard to exempt its own file"* (`write_guards.go:15-19`) | `write_guards.go:9-53` |
| 17 | Test-removal guard | Go — `internal/session/test_removal_guard.go:19` `removedTestFunctions`, gate at `executor_tools.go:501-504` | **stop as failure** (post-gate) | a test function present in a written file's pre-write snapshot is absent from the whole workspace afterwards | Go (AST) | `turn removed test(s) without replacing them: %s; a failing test is fixed by fixing the code, not by deleting the test` | No | `executor_tools.go:497-505`, `test_removal_guard.go:14-31` |
| 18 | Post-edit build gate + repair loop | Go — `internal/session/build_verify.go:208` `verifyAndRepairBuild` | **verify, then retry** | `VerifyBuildAfterEdits` on, `SuccessfulWriteTools > 0`, a written path is Go (`:218-224`); then `verifyBuild` verdict | Go (runs `go build`; `VerifyOutcome` at `verify_outcome.go:20-36`) | on exhaustion, the turn fails with the compiler errors, the cost ledger and follow-ups (`build_verify.go:199-201`) | No — a compiler verdict | `build_verify.go:190-272` |
| 19 | Post-edit test gate + repair loop | Go — `build_verify.go:285` `verifyAndRepairTests` | **verify, then retry** | same trigger; `gateTests` verdict (`:398`) | Go (`go test` with coverage, plus `go vet -tags` for tag-gated packages) | failure text from the test output; a `VerifyIndeterminate` (timeout) does **not** fail — *"a timeout is not evidence of breakage. The turn completes unverified"* (`:344-350`) | No | `build_verify.go:274-393` |
| 20 | Repair attempt ceiling | Go — `internal/session/repair_loop.go`, `ExecutorConfig.RepairMaxAttempts` / `RepairWallClock` | **stop** | attempts or wall clock exhausted | Go | turn fails loudly with errors + cost ledger + follow-ups | No | `build_verify.go:193-201`, `executor.go:417` (`RepairMaxAttempts: DefaultRepairMaxAttempts`) |
| 21 | `ErrStepsIncomplete` (planned steps) | Go — `internal/session/work_steps.go:33,431-434` | **stop as failure** | after all steps ran, a step's file was edited by no step and the step gave no `NO CHANGE NEEDED:` evidence (`unfinishedSteps`, `:297-308`) | Go counts `SuccessfulWriteTools` before/after each step (`:361,391`) | `planned steps incomplete: %d of %d step(s) made no edit and nothing else edited the file (%s)` + the full step ledger `workStepReport` (`:312-337`) | **Partly** — a step escapes the obligation by emitting a line starting `NO CHANGE NEEDED:` (`noChangeEvidence`, `:243-252`). The evidence is *prose*, matched by prefix; nothing checks the file and line it claims to have checked | `work_steps.go:18-33,243-252,296-308,339-436` |
| 22 | Step retry with reading closed | Go — `work_steps.go:373-390` | **retry, narrowed** | a step made no edit in its first pass | Go | `Step %d/%d (%s) made no edit in %d tool call(s); one more pass with reading closed` | No — the retry runs under `toolLoopPass{regime: commitRegime}` | `work_steps.go:373-390` |
| 23 | `maxPlannedSteps` | Go — `work_steps.go:37` (const 12) | **stop planning** | a plan longer than 12 | Go | *"the executive runs the first steps and reports"* | No | `work_steps.go:35-37` |
| 24 | `VerifyWithRetry` | Go loop — `internal/verification/verifier.go:206-284`; cap from `shard_profiles.<type>.max_retries` (`cmd/nerd/chat/delegation_routing.go:345-351`) | **retry, then escalate** | an LLM judge's JSON verdict (`verifyTask`, `verifier.go:302`) says `success:false` or lists `quality_violations` | **the model** (an LLM-as-judge with a hand-written rubric, `verifier.go:311-340` for reviews, `:343+` for implementations) | on exhaustion returns `ErrMaxRetriesExceeded` = `max retries exceeded - escalating to user` (`verifier.go:26-27`), which the chat layer escalates to the human | **Yes, structurally** — the deciding input is one model's prose about another model's prose. It does fail closed when the judge is unavailable (`verifier.go:236-252`) | `verifier.go:206-284,302-340` |
| 25 | Northstar Guardian alignment | Go + LLM — `internal/northstar/guardian.go:328` `CheckAlignment`; campaign wiring `internal/northstar/observer.go` | **stop** at campaign start / phase transition | `check.Result == AlignmentBlocked` | **the model** — `parseAlignmentResponse` (`guardian.go:547-600`) scrapes `SCORE:` / `RESULT:` / `EXPLANATION:` / `SUGGESTIONS:` lines out of free text; when no explicit `RESULT:` line is present it falls back to `classifyScore` on the parsed score, defaulting to `AlignmentWarning` with *"Unable to parse alignment response"* (`:549-551`) | campaign start: `campaign goal does not align with vision: %s` (`observer.go:69`) | **Yes** — the verdict is a line of model text | `guardian.go:328,547-600,908`, `observer.go:54-105,159` |
| 26 | Northstar periodic check | Go — `observer.go:159` | **observe/warn** | `tasksInPhase % checkEveryNTasks == 0`, default 5 (`observer.go:35,39-41`, config `PeriodicCheckInterval`) | Go schedules; model judges | log / drift record | Yes | `observer.go:33-50,159` |
| 27 | Campaign phase checkpoint | **Mangle** — `policy/campaign_phases.mg:91-98` | **verify / block** | `all_phase_tasks_complete(PhaseID)` and `has_pending_checkpoint(PhaseID)` → `next_action(/run_phase_checkpoint)`; a `/false` checkpoint derives `phase_blocked(PhaseID, /checkpoint_failed)` | Mangle over task status facts | phase does not advance | No | `campaign_phases.mg:86-98` |
| 28 | `campaign_blocked/2` | **Mangle** — `campaign_phases.mg:153-157` | **stop** | no eligible phase, none in progress, and an incomplete phase remains | Mangle | `Campaign blocked: %s` then `failCampaign` (`orchestrator_execution.go:209-217`) | No | `campaign_phases.mg:153-157`, `orchestrator_phases.go:206-222` |
| 29 | `campaign_complete/1` | **Mangle** — `campaign_phases.mg:145-150` | **stop (permits completion)** | `!has_incomplete_phase(CampaignID)` | Mangle | — | No | `campaign_phases.mg:144-150` |
| 29b | `isCampaignComplete()` | **Go** — `internal/campaign/orchestrator_phases.go:184-203` | **stop (actually ends the run)** | every `o.campaign.Phases[i].Status` is `/completed` or `/skipped`, iterated in Go | Go over the in-memory struct | `=== Campaign completed successfully: %s ===` (`orchestrator_execution.go:172`) | No | `orchestrator_phases.go:184-203`, `orchestrator_execution.go:171-205`. **This is the duplicate of #29 and the live one — see §7** |
| 30 | `replan_needed/2` | **Mangle** — `campaign_phases.mg:111-128` | **continue, replanned** | failed-task count ≥ `campaign_config` threshold with auto-replan on; or a user instruction; or an explicit `replan_trigger` | Mangle over Go-computed `failed_campaign_task_count_computed` | `next_action(/pause_and_replan)` | No | `campaign_phases.mg:104-128` |
| 31 | `next_action(/interrogative_mode)` (the requirements interrogator's policy sibling) | **Mangle** — `policy/clarification.mg:14,23,32,37` | **ask** (blocks action derivation) | `clarification_needed(Ref)` when `focus_resolution` score < 85 (`:6-8`); or `ambiguity_detected`; or `intent_unknown`; or `intent_unmapped` — each `AND NOT awaiting_clarification AND NOT yolo_mode()` | Go supplies the confidence score; Mangle decides | a `clarification_question/2` is surfaced (`clarification.mg:42-60`) | No, but `yolo_mode()` disables all four | `clarification.mg:1-60` |
| 32 | `llm_timeouts` | Go — `internal/config/llm_timeouts_config.go`; per-call/operation/campaign | **stop** | wall clock | Go | `the work was progressing, not stuck; the operation budget ran out%s. Raise it with --timeout` (`executor_tools.go:1892`) | No | `.claude/rules/nerd-config-schema.md` (`llm_timeouts`), `executor_tools.go:1864-1895` |
| 33 | Exploration cutoff / final-answer reserve | Go — `executor_tools.go:189-193,225-235`, `toolExplorationCutoff`, `FinalAnswerReserve` (default 5 min, `executor.go:388`) | **stop exploration, force a conclusion** | `!time.Now().Before(finalizationCutoff)` | Go | the turn produces a conclusion instead of `context deadline exceeded` | No | `executor_tools.go:189-235,337-363` |
| 34 | `core_limits` (memory, shards, API, session, kernel facts) | Go — `internal/config/limits.go:9-15`, validated `:106-135` | **stop / throttle** | resource counts | Go | varies | No | `limits.go:9-15,106-152` |
| 35 | `delegate_task(/tester, "Generate tests for impacted code", /pending)` | **Mangle** — `policy/delegation.mg:91-93` | **delegate** (owed work) | `impacted(File)` and `!test_coverage(File)`; `impacted/1` is the transitive closure over `dependency_link`+`modified` (`policy/impact.mg:6-13`) | Mangle | a tester delegation | No — but it produces a delegation, not a block: nothing stops a turn while it derives | `delegation.mg:91-93`, `impact.mg:6-13` |

### 1a. The north star as a forcing mechanism — pressure or decoration?

The architect's framing: *"the northstar needs to be literally a north star... final state... one
that you keep reaching towards, not a goal to be hit per se"*, and *"the northstar system needs to
put pressure on the harness's agents."* Measured against that, here is every north-star surface
today and whether it changes what an agent must do next.

**Where the facts come from.** `.nerd/northstar.mg` is read at boot and appended to the kernel's
schema builder (`internal/core/kernel_init.go:421-437`) — note `res.Logic` is what carries it, and
the comment at `:415-420` records a live bug where only `res.Facts` was appended and *"the log said
'2839 bytes, 0 data facts' and `nerd query northstar_mission` still found nothing."* The authority
is stated once, in `internal/northstar/bridge.go:19-54`: the SQLite store
`.nerd/northstar_knowledge.db` is the durable record, the JSON and `.mg` files are import/export
surfaces *"never an authority anything reads to decide"*, and `SyncVisionAuthority` (called from
`Guardian.Initialize`, `:52-54`) reconciles them last-writer-wins.

| Surface | Where | Pressure or decoration? |
|---|---|---|
| `injectable_context("*", Mission \| Desc)` for mission, critical capabilities, unmitigated risks, constraints | `policy/prompt_northstar.mg:152-179` | **Decoration.** Text injected into a prompt. Gated on `has_active_planner()`/`has_active_coder()` for mission and capabilities; risks and constraints are unconditional. Changes nothing an agent *must* do |
| `must_have_requirement(ReqID, Desc)` | `prompt_northstar.mg:58-62`, declared `schemas_misc.mg:133` | **Decoration.** Derived from `northstar_requirement` with priority; nothing gates a turn on it being satisfied. The name promises an obligation the corpus never enforces |
| `unmitigated_risk/1` → `strategic_warning(/critical_unmitigated_risk, CapID, RiskID)` | `prompt_northstar.mg:23-25,110-114` | **Decoration.** A warning row |
| `effective_module_purpose/2` (module inherits-and-refines the project north star) | `prompt_northstar.mg:189-213` | **Decoration.** Resolves a purpose string for injection; the fallback exists so *"a module must never silently opt out of the project's purpose"* (`:184-187`) — but "not opting out" means "still gets the text" |
| Guardian `CheckAlignment` → `AlignmentBlocked` at campaign start | `internal/northstar/guardian.go:328`, `internal/northstar/observer.go:63-71` | **Pressure, but LLM-shaped and one-shot.** It can refuse a campaign (`campaign goal does not align with vision: %s`). The verdict is scraped out of model prose (`guardian.go:547-600`), defaulting to `AlignmentWarning` with *"Unable to parse alignment response"* when parsing fails (`:549-551`) |
| Guardian phase-transition and periodic checks | `observer.go:90-105,159` | **Pressure at phase boundaries only** (`AlignmentBlocked` at `:105`); the every-N-tasks check (default 5, `observer.go:35`) is recorded, not enforced |
| `BackgroundObserverManager` northstar observer | `internal/shards/observer_manager.go:84-86,300-304,388-426` | **Decoration.** `ObserverAssessment` has a `Level` with a literal `LevelBlock` for score < 40 (`:61-77`) — and the only thing that consumes `LevelBlock` in the whole repo is an emoji lookup table at `:580`. Assessments land in a 100-entry ring buffer (`:545-557`) read by `GetRecentAssessments`/`GetLastAssessment`, which no production caller uses. **The "block" level blocks nothing** |
| Periodic-check throttle | `observer_manager.go:350-386` | Evidence of the cost of decoration: *"Measured 2026-09-17 in an idle chat session: a check every five minutes from boot, each an LLM call of 13-43 s, each scoring the session 10-50/100 for having nothing to show — the guardian's own recommendation on every one was to stop evaluating empty ticks."* An expensive alignment opinion nobody acts on |
| Campaign `/northstar` risk gate | `internal/campaign/risk_scoring.go:49,267-282` | **Pressure, but about plumbing, not distance.** It blocks a campaign when `configuredNorthstarObserver == nil` **and** the campaign touches protected roots: `northstar observer not configured for protected campaign surfaces: %s`. That is a wiring precondition — "is the guardian switched on" — not "how far is this campaign from the final state" |
| `campaign_risk_gate_outcome/3`, `campaign_risk_blocked_gate/2`, `campaign_risk_preflight_blocked/1` | `schemas_campaign.mg:390,417,423`; Go measures and the kernel decides (`risk_gate_contract.go:18-36`) | **Pressure, and correctly split.** *"Go MEASURES the preflight... The kernel is the authority, not a suggestion"* (`:18,36`). This is the best-shaped gate in the system — but its subject is risk, not vision distance |

**Judgment.** The north star today is overwhelmingly **decoration**: it is text that reaches a
prompt, plus one LLM-judged veto at two campaign boundaries, plus a wiring precondition. Nothing
in the system computes *distance from the final state*, nothing carries that distance forward
between turns, and the one predicate named like an obligation (`must_have_requirement/2`) is never
checked against evidence. There is also no notion of a state that is approached rather than
reached: `northstar_capability/4` carries a horizon atom (`/now`, `/6mo`, `/1yr`, `/3yr`,
`/moonshot`, `prompt_northstar.mg:90-105`), which is exactly the raw material for a distance
metric — and it is used only to sort capabilities into injection buckets. **[REFUTED - see Verification]**

## 2. The tool budget, exactly

### What it is

"The tool budget" is not one thing. It is **two ceilings and one controller**, and only the
controller has the telemetry:

1. **`MaxToolCalls`** — a per-turn count of *executed tool calls*. Default 50
   (`internal/session/executor.go:382` `defaultMaxToolCalls`). Enforced in
   `internal/session/executor_tools.go:792-815`: once `result.ToolCallsExecuted >= maxToolCalls`,
   every further call in the batch is short-circuited with the literal tool result content
   `"tool call budget exceeded for this turn"` (`executor_tools.go:812`) and a tool error
   `"<name>: budget exceeded"` (`executor_tools.go:815`).
2. **`MaxToolIterations`** — a per-turn count of *LLM → tools → LLM round trips*. Default 8
   (`executor.go:383` `defaultMaxToolIterations`). This is the `for` loop bound at
   `executor_tools.go:207`.
3. **`toolBudgetController`** (`internal/session/tool_budget_controller.go`, 389 lines) — the
   object that (a) fingerprints every executed call/result pair, (b) grants bounded extensions
   to ceiling 2, (c) renders the remaining-budget nudge, and (d) — the part that matters — is
   also the **only producer of the progress facts the Mangle working policy consumes**
   (`tool_budget_controller.go:334` `workingProgress`).

### Every file

| File | Role |
|---|---|
| `internal/session/tool_budget_controller.go` | The controller: `toolBudgetController`, `toolBudgetObservation`, `toolBudgetExtensionDecision`, `observe`, `maybeExtend`, `repeatedTailCycle`, `nudge`, `workingProgress`, `workingNudgeText`, `workingRegimeText`, `appendToolBudgetNudge`, `isFocusedVerificationCall`, `toolBudgetEventSignature` |
| `internal/session/executor_tools.go` | The only consumer: constructed at `:181`, `observe` at `:246`, nudge at `:254`, policy report at `:275-277`, `maybeExtend` at `:394`, exhaustion path at `:407-440`; the separate call ceiling at `:792-815`; the exhaustion nudges at `:599-678` |
| `internal/session/executor.go` | `ExecutorConfig` fields (`:257-265`) **[REFUTED - see Verification]**, the five `default*` constants (`:382-386`), `DefaultExecutorConfig` (`:408-417`) |
| `internal/config/limits.go` | `CoreLimits` fields (`:17-47`), validation (`:119-133`), defaults (`:138-152`) |
| `internal/context/working_set.mg` | The policy that reads the controller's counts: `working_progress/5` (`:34`), the four constants (`:55-58`), `working_stop`, `working_finalize`, `working_nudge`, `working_regime` |
| `internal/context/working_set.go` | `WorkingProgress` struct (`:135`), the embedded policy (`:24`), `Continue` |
| `cmd/nerd/cmd_campaign.go:1359-1376` | Campaign propagation of all six knobs into `ExecutorConfig` |
| `cmd/tools/change_benchmark/main.go:167-169` | The benchmark harness sets `AdaptiveToolBudget = false` and pins both ceilings |
| Tests | `internal/session/tool_budget_controller_test.go`, `executor_budget_exhaustion_test.go`, `internal/config/limits_tool_budget_test.go`, `cmd/nerd/cmd_campaign_tool_budget_test.go` |

### Every config key

> **Superseded 2026-09-18 by seam S3** (`Docs/journeys/impl/S3-forcing-as-facts.md`).
> All six keys below are **deleted**. `internal/config/limits.go` now rejects
> each by name at load, and the repeat threshold is a policy fact
> (`working_repeat_threshold` in `internal/context/working_set.mg`). The table
> is the verified record of what the code was when the study was written, not
> a list of keys a config may set.

All under `core_limits` in `.nerd/config.json` (`internal/config/limits.go`):

| Key | Type | Default | Validation |
|---|---|---|---|
| `max_tool_calls` | int | 50 (code default when 0) | `>= 0` (`limits.go:119`) |
| `max_tool_iterations` | int | 8 (code default when 0) | `>= 0` (`limits.go:122`) |
| `adaptive_tool_budget` | `*bool` | nil = enabled; `DefaultCoreLimits` sets true (`limits.go:139,147`) | — |
| `tool_iteration_extension_size` | int | 8 | 0..64 (`limits.go:125`) |
| `max_tool_iteration_extensions` | int | 2 | 0..8 (`limits.go:128`) |
| `tool_loop_repeat_threshold` | int | 2 | 0, or 2..8 (`limits.go:131`) |

Note: these are **not** in `.claude/rules/nerd-config-schema.md`'s `core_limits` description, which
lists only "Memory MB, concurrent shards/API, session minutes, kernel fact ceiling". The schema
doc is stale on this sub-struct — a documentation seam, logged in §7.

### Every log line and user-visible message

| Text | Site | Channel |
|---|---|---|
| `Adaptive tool budget extended by %d rounds to %d after %d executed tool call(s): %s` | `executor_tools.go:396` | log WARN |
| `Adaptive tool budget refused extension at %d rounds after %d executed tool call(s): %s` | `executor_tools.go:400` | log WARN |
| `Tool iteration budget reached: %d rounds (base %d, hard %d, extensions %d/%d); forcing a final answer from %d executed tool call(s)` | `executor_tools.go:429` | log WARN |
| `tool iteration budget exhausted (%d iterations, %d tool calls executed): forced final answer failed: %w` | `executor_tools.go:440` | returned error |
| `tool call budget exceeded for this turn` | `executor_tools.go:812` | **into the model's context**, as a tool result |
| `<tool>: budget exceeded` | `executor_tools.go:815` | tool error list |
| `[orchestrator] Orchestrator budget: %d tool calls and %d rounds remain. <steer>` | `tool_budget_controller.go:310` via `appendToolBudgetNudge` (`:372`) | **into the model's context** |
| `Your tool budget for this turn is exhausted; no further tools are available. ...` | `executor_tools.go:599` `readOnlyBudgetExhaustedNudge` | **into the model's context** |
| `Your exploration budget for this turn is exhausted. Only write tools remain: ...` | `executor_tools.go:603` `writeBudgetExhaustedNudge` | **into the model's context** |
| `the work was progressing, not stuck; the operation budget ran out%s. Raise it with --timeout` | `executor_tools.go:1892` | user-facing error text |

So the phrase the architect objects to — "tool budget" — is literally in the model's context
window at least four times per exhausted turn, and in the user's error text once.

### History

`git log --follow internal/session/tool_budget_controller.go` (5 commits, newest first):

| Commit | Date | Subject |
|---|---|---|
| `ae7937bb` | — | `feat(session): uplift session unit — guards, lifecycle bounds, dead-code removal` |
| `7213b414` | — | `feat(session): close exploration under a policy-derived commit regime` |
| `2c409a58` | — | `fix(session): let the working policy see the shape of the turn, not just a cycle flag` |
| `0e06c9c8` | — | `fix(session): require material progress for tool extensions` |
| `66585382` | 2026-08-10 | `fix(session): adapt tool budgets to proven progress` (origin) |

The origin commit says what it was added to fix, verbatim:

> Two consecutive all-Spark dogfood fixes consumed the configured 24 tool rounds on path guesses
> and review before the requested wiring landed. The executor previously exposed no
> remaining-budget context and could only stop at the cliff.

So the controller was born to solve **two distinct problems at once**: (a) the model could not
see how close the cliff was, and (b) a hard ceiling could not tell "24 rounds of real progress"
from "24 rounds of churn". The subsequent three commits are a steady migration of the *decision*
out of Go and into `working_set.mg` — `2c409a58` moved the turn's shape into the policy,
`7213b414` moved the regime there. The controller that remains is the residue: counting, and
the older ceiling that never got migrated.

### What breaks if it is deleted outright

Deleting `tool_budget_controller.go` today breaks **five** things, only two of which are the
ceiling:

1. **The Mangle policy goes blind.** `working_progress/5` has exactly one producer: **[REFUTED - see Verification]**
   `toolBudgetController.workingProgress` (`tool_budget_controller.go:334`), fed from fields
   `rounds`, `writesTotal`, `roundsSinceWrite`, `roundsSinceVerify` that only `observe`
   (`:106-161`) maintains. Delete the controller and `read_only_stall`, `verify_after_write`,
   `/commit` and all three nudges stop deriving — i.e. **deleting the tool budget deletes the
   policy-side forcing too**, unless the counting moves first.
2. **Cycle detection goes away.** `working_stop(/repeated_cycle)` reads `working_control(/yes, _)`,
   whose `Cycle` field is `repeatedTailCycle()` (`:276`). This is genuine deterministic evidence,
   not a heuristic: it compares full SHA-256 signatures of call+args+status+**result content**
   (`:163-179`), so a re-read whose bytes changed is progress and an identical re-read is a loop.
3. **The non-progress-driven paths lose their only stop.** `progressDriven` requires *both*
   `activeWorkingLoop(ctx) != nil` **and** `executorCfg.ProgressDrivenTools`
   (`executor_tools.go:182`). Every path where either is false runs with **no policy at all** —
   the `for iter < budget.iterationLimit` bound is the entire termination argument. (Which paths
   those are is §4's job.)
4. **The forced-final answer disappears.** `executor_tools.go:407-440` is what turned
   `nerd review internal/types/mangle_scale.go` from "printed `📋 Result:` and nothing, exit 0
   after 16 successful tool calls" into a real answer. That code is triggered by ceiling
   exhaustion. Its value is independent of the ceiling, but it currently has no other trigger
   than the ceiling and `finalizeReason`.
5. **`MaxToolCalls` still stands.** The call ceiling at `executor_tools.go:792-815` is a
   *separate* enforcement site that does not use the controller at all. Deleting the controller
   leaves it, and it is the cruder of the two (it truncates a batch mid-flight with a canned
   error rather than asking for a conclusion).

### What a policy-derived replacement needs

The counting is legitimate; the *ceiling* is the part that is a decision computed in Go. The
minimal shape:

**Facts the loop must assert per round boundary** (all of these already exist as Go values
inside `observe`, so this is a relocation, not new instrumentation):

- `tool_round(TurnID, N)` — the round completed.
- `tool_call_result(TurnID, N, ToolName, Status)` with `Status` in `/ok | /error` — one per
  executed call. Replaces `observation.successes/errors`.
- `tool_event_novel(TurnID, N, Digest)` — the SHA-256 fingerprint was not seen before this turn.
  Go must still compute the digest (it hashes result *content*, which Mangle cannot do), but
  "novel" is then a `!seen_before` derivation rather than a Go map lookup.
- `durable_write(TurnID, N, Path)` — asserted when a write-mutation tool returned ok.
- `focused_verification(TurnID, N, Kind)` — asserted when `isFocusedVerificationCall` matched.
  **This classifier is itself policy** (`tool_budget_controller.go:243-270` hardcodes a list of
  tool names and shell-command prefixes); it belongs in `.mg` as `verification_tool/1` +
  `verification_command_prefix/1` facts.
- `tool_round_all_failed(TurnID, N)` — every call in the round errored.

**Rules that decide continue vs stop** — most already exist and only need re-basing off the new
facts instead of the five-slot `working_progress/5` summary:

- `progress_since(TurnID, Boundary)` :- a `tool_event_novel` at a round after `Boundary`.
- `stall(TurnID, /repeated_cycle)` — needs period-1/2/3 tail comparison, which is the one piece
  that is awkward in Mangle. Cheapest honest answer: keep the digest comparison in Go but assert
  `tool_trace_cycle(TurnID, Period)` as a **fact**, and let the policy decide what a cycle means
  (it already does — `working_set.mg:80`). That is not a shim; it is the same split as
  "Go measures, Mangle decides" already used for `turn_evidence`.
- `task_incomplete(TurnID, Reason)` — §6.

The important structural point: **once `task_incomplete` exists, the ceiling is not needed as a
stop, only as a cost guard.** The loop should terminate when it derives no `task_incomplete`,
and a cost ceiling should raise `task_incomplete(TurnID, /budget_exhausted)` — an *obligation to
report*, not a silent walk-away. That is the difference between "the budget ran out" and
"the work is done", and today `executor_tools.go:435` conflates them by routing both into the
same `forceFinalAnswer`.

## 3. Coverage

**Short answer: one narrow thing forces a test to exist, one narrow thing forces it to be
non-hollow, one thing forces it not to be deleted — and the strongest coverage signal the system
computes is a `Warn` log that nothing consumes.** **[REFUTED - see Verification]**

### What forces a test to exist

Exactly one rule: `hollow_success("new source was created without a test file")`
(`internal/core/defaults/policy/coder_safety.mg:104`), which fires when
`turn_created_source(File)` joins `missing_test_for(File)`, i.e. `created_source(File)` and
`!test_coverage(File)` (`:28`). It is consumed at `internal/session/executor.go:2471-2494` and
fails the turn with `turn created Go source %s without a test file (verb %s)`.

Its reach is deliberately small, and the file says so (`coder_safety.mg:12-18`):

- `test_coverage` derives only from `test_file_for`, which the world scanner emits for the
  `x_test.go` naming convention. A source file covered by a package-level test with another name
  is *not* covered as far as this rule is concerned. The comment calls this "cautious rather
  than falsely permissive".
- It fires only for **created** source, never for **modified** source. Editing 400 lines into an
  untested file is not an obligation.
- It is scoped per turn via `turn_created_source` (`executor.go:2394-2424`) precisely so a leaked
  or scanner-derived `created_source` fact "cannot fail later turns forever" — which also means
  the obligation vanishes the moment the turn ends. **There is no cross-turn coverage debt.**

### What forces a test to be non-hollow

1. **`writesPlaceholderTestFile`** (`internal/session/executor_tools.go:1379`) — a **pre-write**
   guard, dispatched from the single protected entry point `runWriteGuards`
   (`internal/session/write_guards.go:39`). A write-mutation tool targeting a `*_test.go` path
   whose content is a placeholder is refused before the tool runs, with
   `blocked by placeholder test guard: %s is placeholder (%s)`. The file's own header explains
   why it is a pre-write fact and not prose (`executor_tools.go:1369-1374`):
   *"Prose is a request the model complies with most of the time, while a fact checked before the
   tool runs is one no amount of model conviction gets past."*
   `write_guards.go:9-28` records the reason the guard set was consolidated into one
   write-protected unit: *"Observed 2026-08-12: when the modularity guard refused a write, the
   agent's next action was to edit the guard to exempt its own file."*
2. **`cmd/tools/audit_test_bodies`** — a standalone repo-wide auditor. It parses every
   `*_test.go`, and flags any `func TestXxx(t *testing.T)` whose body is empty or contains only
   `t.Log`/`t.Logf` calls (`cmd/tools/audit_test_bodies/main.go:17-36`), reporting
   `%s: %s has no executable behavior beyond logging` (`:64`). Its own doc comment is careful:
   *"Passing this check is not a claim of behavioral coverage; it closes the specific log-only
   placeholder failure without rewarding test counts."* (`:1-3`).
   **It is not wired into any decision.** Grep for `audit_test_bodies` across the repo returns
   only: its own `main.go`, `UPLIFT_LEDGER.md`, two audit docs, and two `internal/session` files
   that mention *placeholder tests* by concept, not the tool. It is a command a human runs.
3. **`removedTestFunctions`** (`internal/session/test_removal_guard.go:19`) — post-gate, and this
   one *does* fail the turn (`executor_tools.go:501-504`):
   `turn removed test(s) without replacing them: %s; a failing test is fixed by fixing the code,
   not by deleting the test`. It runs **after** the test gate and the gofmt pass deliberately, so
   "green suite that lost a contract" still fails (`executor_tools.go:499-500`). A test that
   merely *moved* (same name elsewhere under the workspace) is not a removal
   (`test_removal_guard.go:17-18`).

### What measures coverage and is then ignored

`verifyAndRepairTests` (`internal/session/build_verify.go:285`) computes two coverage signals and
**neither is an input to any decision** **[REFUTED - see Verification]**:

| Signal | Computed at | What happens |
|---|---|---|
| `result.UntestedPaths` — production Go written with no test alongside it | `build_verify.go:303-307` via `untestedWithoutCoverageOnDisk` (`test_verify.go:181`) | `logging...Warn("Turn wrote production Go with no test alongside it: %s")`. The doc comment says outright it is *"a warning, not a failure"* (`build_verify.go:281-284`) |
| `result.UncoveredBlocks` — blocks of Go this turn wrote that no test executes, attributed to the turn's changed lines via a pre-write snapshot | `build_verify.go:309-337`, `coverage_profile.go` | `logging...Warn("Turn wrote %d block(s) of Go that no test executes: %s")` |

The comment at `build_verify.go:328-331` states exactly why this is the sharp signal:
*"Green tests over code that was never executed is the precise false success this signal exists to
expose — `go test` cannot tell the two apart, only the profile can."* And then it logs it.

`untestedWithoutCoverageOnDisk` was deliberately softened (`test_verify.go:169-180`): a file counts
as covered if its own `<base>_test.go` exists **or its package contains any `_test.go` at all**,
because the stricter version *"flagged internal/session/test_verify.go on two consecutive live
turns (2026-08-08 11:08 and 11:13)"*. The reasoning given is: *"A gate that cries wolf about tested
code is one that gets ignored, and then switched off."* — i.e. the signal was weakened rather than
made precise, and then demoted to a warning anyway.

### The tester shard

The tester is delegated from four Mangle rules in `internal/core/defaults/policy/delegation.mg`:

| Rule | Line | Trigger |
|---|---|---|
| `delegate_task(/tester, Task, /pending)` | `:79-81` | `user_intent(..., /test, Task, _)` and not `wants_direct_answer()` |
| `delegate_task(/tester, Task, /pending)` | `:87-89` | `user_intent(..., /assault, Task, _)` — adversarial probing has no nemesis executor, so it runs through the tester (`:83-86`) |
| `delegate_task(/tester, "Generate tests for impacted code", /pending)` | **`:91-93`** | `impacted(File)` **and** `!test_coverage(File)` |
| `action_mapping(/benchmark \| /profile \| /assault, /delegate_tester)` | `:215-219` | verb routing; `/delegate_tester` is `side_effecting_action` (`:248`) |

**`delegation.mg:91-93` is the one true coverage obligation in the corpus** — the only rule
anywhere that says "some file is impacted and uncovered, therefore work is owed". It joins
`impacted/1` (`policy/impact.mg:6-13`, the transitive closure over `dependency_link` and
`modified`) with the same conservative `test_coverage/1` as §3. It is a *derived obligation to
delegate*, which is exactly the shape §6 argues for — but note that it produces a **delegation**,
not a **blocking obligation**: nothing prevents a turn from completing while it is derivable.

Two sibling rules in `impact.mg` are derived and **unconsumed**:
```
unsafe_to_refactor(Target) :- impacted(Target), !test_coverage(Target).
block_refactor(Target, "uncovered_dependency") :- unsafe_to_refactor(Target).
```
— `impact.mg:16-22`. Grep for `block_refactor` across `*.go` returns only two context-scoring
weight tables (`internal/context/types.go:80`, `internal/context/activation_scoring.go:167`);
`unsafe_to_refactor` is queried only by shadow mode (`internal/core/shadow_mode.go:450`). **No
production path blocks a refactor for uncovered dependencies.**

`internal/core/defaults/tester.mg:109-124` declares a complete coverage-goal apparatus —
`coverage_metric/2`, `coverage_goal/1`, `coverage_below_goal/1`, `needs_more_tests/1`,
`coverage_warning/3`. Grep across all `*.go`: **zero consumers, and no producer of
`coverage_metric` or `coverage_goal`.** The predicate the vision would most want to use —
`needs_more_tests/1` — exists, is correctly shaped, and is dead. Meanwhile the Go side
*does* compute per-block coverage (`build_verify.go:309-337`, `coverage_profile.go`) and never
asserts it as a fact.

That gap is the single most actionable finding in this document: **the measurement and the
predicate exist; the wire between them does not.**

### Summary against the north star

The vision says *"the agent cannot stop until the tests that actually harden the system exist."*
Today:

- A turn that **creates** a new Go file must produce a `<base>_test.go` — or a turn-level
  hollow-success failure fires. That is a real obligation, and it is in Mangle.
- A turn that **modifies** any amount of untested code owes nothing.
- A turn that writes code **no test executes** produces a precise, per-block, per-changed-line
  measurement — and a log line.
- Nothing carries an unmet coverage obligation past the end of the turn.

## 4. Who decides "done", per path

### 4.0 The kernel already derives the right predicate, and nothing acts on it

`turn_done/1` is declared *"the single completion signal"*
(`internal/core/defaults/policy/coder_safety.mg:108`) and is correctly conservative:

```
turn_executed(Verb) :- turn_evidence(Verb, _, _, _, _, _), !has_hollow_success(), !build_state(/failing).
turn_done(Verb)     :- turn_executed(Verb), turn_acceptance(Verb, _, _).
```
— `coder_safety.mg:114-115`

Its only Go consumer is `consumeTurnDoneSignal` (`internal/session/executor.go:2563-2572`), which
does this and nothing else:

```go
if doneFacts, derr := e.kernel.Query("turn_done"); derr != nil {
    logging...Debug("checkHollowSuccess: turn_done query failed: %v", derr)
} else if len(doneFacts) != 1 {
    logging...Debug("checkHollowSuccess: turn_done count=%d for verb %s (expected exactly one per turn_evidence)", len(doneFacts), verb)
}
```

The call site confirms it is deliberate (`executor_tools.go:1972-1975`): *"The consumer logs any
deviation for diagnosis **without changing the verdict hollow_success already determined**."*

So the architecture is inverted from the vision: the **negative** predicate (`hollow_success`)
has teeth and fails the turn; the **positive** predicate (`turn_done`) is a debug log. A turn ends
when the tool loop stops producing tool calls and no `hollow_success` fires — not when the kernel
derives that it is done. `turn_acceptance` is asserted only when
`result.Acceptance.Status == "verified"` (`executor.go:2334`), so on every turn without a host
verifier acceptance, `turn_done` never derives — and the turn ends successfully anyway.

The one place the derivation is read for anything other than a log is `TurnOutcome`
(`internal/session/executor_memory.go:150-165`), which maps `turn_done` → `/done`,
`hollow_success` → `/hollow` for the `turn_cost` ledger, and `internal/system/factory_learning.go:270`
(`"Kernel derived turn_done: the turn's claims were matched by recorded evidence."`) — both of
which are *accounting after the fact*, not control.

### 4.1 The chat free-text turn / `/fix` / `nerd fix` — the single-pass tool loop

All three land in the same place: `runToolLoop` in `internal/session/executor_tools.go`. There
are **six** distinct lines at which the turn ends, and which one fires is the whole story:

| # | Terminating line | The input that ended it | Is it the model's prose? |
|---|---|---|---|
| A | `executor_tools.go:383-386` — `if len(nextResp.ToolCalls) == 0 { return verifyTerminal(currentResponse) }` | **The model stopped asking for tools.** | **Yes. This is the normal, overwhelmingly common exit.** The turn ends because the model chose to stop, and then the post-hoc gates (build, tests, hollow-success) get their say |
| B | `executor_tools.go:211-220` — `if finalizeReason != "" { break }` | Mangle `working_finalize(/verify_after_write)` at the previous boundary | No |
| C | `executor_tools.go:282-289` — `if !decision.Continue { return ... "task unresolved" }` | Mangle `working_stop(/read_only_stall \| /repeated_cycle \| /tool_failures)` | No |
| D | `executor_tools.go:207` loop bound exhausted → `:428-440` forced final answer | Go: `iter >= budget.iterationLimit` | No |
| E | `executor_tools.go:225-235` / `:337-363` — deadline finalization | Go wall clock | No |
| F | `executor_tools.go:208-210,304-307` — `ctx.Err()` | cancellation | No |

Exit **A** is the one the vision objects to. Nothing consults an obligation before accepting it;
`verifyTerminal` → `verifyCompletedToolTurn` then runs the build gate, the test gate, the
test-removal guard, the critic, and `checkHollowSuccess`. Those can *fail* the turn, but none of
them can say "not done, keep going" — the only mechanism that resumes work after a stop is the
**repair loop**, and it only triggers on a red build or red tests (`build_verify.go:229-241`,
`:339-351`), never on an unmet obligation like missing coverage.

So the honest summary for the single-pass paths: **a turn ends when the model stops calling
tools, and is then audited.** The audit's vocabulary is "this turn is hollow", not "this turn is
not finished".

### 4.2 The one seam inside exit A that matters

`verifyTerminal` runs `verifyCompletedToolTurn` only when `pass.verify` is set
(`executor_tools.go:197-205`). A planned-steps pass sets it false and verifies once at the end
(`work_steps.go:69-72`, `:425-426`). So for planned tasks, exit A per step does *not* gate — the
gate is the whole-task one. That is deliberate and correct; it is noted here because it means
"the turn ended" and "the turn was verified" are decoupled by a boolean parameter.

### 4.3 Planned steps

This is the **strongest completion contract in the system today**, and the only one that is
shaped like an obligation.

- The plan is produced by the model in one bounded call (`planTurnSteps`, `work_steps.go:157`;
  `planStepsTimeout` = 2 min, `:42`; hard-capped at `maxPlannedSteps` = 12, `:37`), in a rigid
  `STEP <path> :: <change>` grammar (`workStepPlanSystem`, `:44-49`).
- **What ends a step:** `runPlannedSteps` runs one tool-loop pass per step
  (`work_steps.go:363`), then checks `result.SuccessfulWriteTools == writesBefore`
  (`:373`). If no write landed, it runs **one more pass with reading closed**
  (`toolLoopPass{regime: commitRegime}`, `:377`). Then `step.Edited = result.SuccessfulWriteTools > writesBefore` (`:391`).
  A step's own failure — a policy stop, a provider error — ends that step, **not the task**
  (`:341-342`).
- **What ends the plan:** after every step, `markCoveredSteps` (`:282`) then one whole-task
  verification (`:425-426`), then:
  ```go
  if missing := unfinishedSteps(steps); len(missing) > 0 {
      return verified, toolErrs, fmt.Errorf("%w: %d of %d step(s) made no edit and nothing else edited the file (%s)\n%s", ErrStepsIncomplete, ...)
  }
  ```
  — `work_steps.go:431-434`. The task **fails** rather than claiming success. The design note at
  `:18-29` is explicit that this is a forcing mechanism, and why it was needed: *"Observed
  2026-09-11, across every multi-site brief of the day: the model makes one edit, announces the
  next one every round, and reads or recalls instead of making it until the working policy stops
  the turn."*
- **Where model prose is the deciding input:** the escape hatch. A step may reply with a line
  starting `NO CHANGE NEEDED:` and the obligation is discharged (`noChangeEvidence`, `:243-252`;
  consumed at `:402-404` and skipped in `unfinishedSteps`, `:300-302`). The prompt asks for "the
  file and line you checked and what is there" (`:272`) — **nothing verifies that the claimed
  file and line say what the model says they say.** This is exactly the north star's "its verdict
  can contradict its own evidence", surviving in the one place that otherwise forces hardest.

Everything here is computed in **Go**. There is no `step_incomplete/1` predicate; `work_steps.go`
never touches the kernel. `schemas_state.mg:72` declares `should_complete(StepID)` and
`schemas_shards.mg:656` declares `has_incomplete_dependency(ItemID)` — neither is the planned-step
loop's.

### 4.4 Campaigns

Campaigns are the **only path whose completion is genuinely fixpoint-shaped**, and also the path
with the clearest duplicated verdict.

The Mangle side is exactly what §6 wants:
```
has_incomplete_phase(CampaignID) :- campaign_phase(_, CampaignID, _, _, Status, _), /completed != Status, /skipped != Status.
campaign_complete(CampaignID)    :- current_campaign(CampaignID), !has_incomplete_phase(CampaignID).
next_action(/campaign_complete)  :- campaign_complete(_).
```
— `policy/campaign_phases.mg:139-150`. "Done" = "no open obligation derives", stated as a rule.

The Go side that actually ends the run does **not** query it. `orchestrator_execution.go:171`
calls `o.isCampaignComplete()`, which is a Go loop over `o.campaign.Phases` checking
`phase.Status == PhaseCompleted || PhaseSkipped` (`orchestrator_phases.go:184-203`). The same
function's sibling `getCampaignBlockReason()` *does* query the kernel
(`o.kernel.Query("campaign_blocked")`, `orchestrator_phases.go:207`). So in one function pair the
blocked verdict is derived and the complete verdict is computed in Go, over the same underlying
state. See §7.

The **phase** gate is properly derived: `all_phase_tasks_complete/1` (`campaign_phases.mg:86-88`)
→ `next_action(/run_phase_checkpoint)` (`:91-94`), and a failed checkpoint derives
`phase_blocked(PhaseID, /checkpoint_failed)` (`:97-98`) so the phase cannot advance. Campaign
replanning is also derived (`replan_needed/2`, `:111-124` → `next_action(/pause_and_replan)`,
`:127-128`).

## 5. Delivered knowledge as an obligation

**Short answer: yes, one mechanism — `is_mandatory` on a prompt atom — and it is the closest
thing in the system to "this agent must have X in its window before acting". It is an *author's*
declaration, not a derivation, and the kernel's job is only to decide whether that declaration is
*admissible in this context*. Nothing derives "this turn needs knowledge X" from the task.**

### How "mandatory" is decided, and by whom

Three layers, in order:

1. **The atom author declares it.** `is_mandatory: true` is a YAML field on a prompt atom.
   Examples of the shape live in `internal/system/agent_definition.go:112` and
   `internal/init/agents.go:221` (the identity atom of a generated agent is mandatory; the
   methodology and domain atoms are not — `agent_definition.go:94`,
   `agents.go:255,272`). So the first and strongest input is a human (or a generator) writing a
   flag into a file.
2. **The kernel decides whether it is admissible.**
   `internal/core/defaults/jit_compiler.mg:196-199`:
   ```
   mandatory_selection(Atom) :- is_mandatory(Atom), !blocked_by_context(Atom), !mandatory_superseded(Atom).
   ```
   `blocked_by_context/1` is the real policy work. Two rules:
   - the *permissive* one (`:76-79`) — a situational dimension (language, framework) blocks only
     when the context explicitly holds a different value;
   - the *fail-closed* one (`:143-146`) — a **regime dimension** (`/shard`, `/mode`, `/phase`,
     `/layer`, `/init_phase`, `/northstar_phase`, `/ouroboros_stage`, `/provider`, `/model`,
     listed at `:133-141`) blocks whenever the compile did not name it at all.

   The comment at `jit_compiler.mg:91-99` is the evidence for why this matters, and it is the
   single best-documented forcing failure in the repo: *"one `explain this file` turn compiled
   114 mandatory atoms / ~60k tokens carrying 25+ contradictory identities (Nemesis, Coder,
   Tester, Legislator, Perception Firewall, the Ouroboros Tool Generator, the Northstar wizard's
   'thought partner', ...). The model obeyed the Perception Layer persona it found there ... and
   answered with an intent announcement instead of doing the work. Identity leakage does not
   degrade a prompt, it replaces the agent."*
   `mandatory_superseded/1` (`:190-193`) then lets one live mandatory atom displace another it
   declares a conflict with — and the comment at `:153-173` explains it exists precisely because
   `prohibited/1` cannot be used there without a negative cycle through `mandatory_selection`.
3. **A mandatory atom becomes the prompt's skeleton.**
   `selected_result(Atom, Prio, /skeleton) :- ..., mandatory_selection(Atom).`
   (`jit_compiler.mg:311-314`); everything else is `/flesh` (`:316-319`). `selected_result/3` is
   the only predicate the Go selector queries (`jit_selection.mg:18`). **[REFUTED - see Verification]**

### The obligation has teeth: the compiler refuses rather than dropping

This is the one place in the system where an unmet knowledge requirement is a **hard refusal**:

- `internal/session/executor.go:1018` — *"That commit made the compiler REFUSE when the mandatory
  skeleton ..."* (the refusal path), with a fallback at `:1036` that re-compiles with mandatory
  atoms as `internal/articulation` does.
- `internal/session/executor.go:307` — the executor tracks *"[whether] the compiler had to drop
  mandatory atoms (defensive_patterns, ...)"*.
- `internal/session/spawner.go:98` — the sub-agent budget *"was hardcoded to 8192 which silently
  dropped mandatory atoms from ..."*; `:609` calls that *"the pre-fix bottleneck that silently
  stripped mandatory atoms"*, and `:651` now passes the configured budget explicitly because
  *"a smaller fallback budget cut mandatory atoms"*.

So: budget exhaustion cannot silently evict a mandatory atom. That *is* "delivered knowledge as
an obligation", and it is the strongest forcing mechanism in the system that has nothing to do
with tool loops.

### Where the obligation is capped anyway

`internal/prompt/selector.go:49-51` introduces a Go cap that applies to Mangle-context compiles:

```go
mangleMandatoryTokenCap    = 900000
mangleMandatoryAtomCap     = 600
mangleMandatoryBudgetRatio = 0.90
```

`selectMangleMandatoryIDs` (`selector.go:421-473`) applies these and logs
`Mangle mandatory cap applied: selected %d/%d atoms, tokens=%d cap=%d`. Note the direction: this
one *promotes* atoms to mandatory in a Mangle-authoring context (`applyMandatoryOverride`,
`selector.go:514-526`) subject to a cap. That is a Go decision about what knowledge is
obligatory — a seam (see §7).

### What does NOT exist

- **No predicate derives "this task requires knowledge X".** The atom author says "I am always
  needed"; the kernel says "you are/aren't admissible here". Nothing says "a turn whose verb is
  `/fix` on a `.mg` file *must* carry the stratification atom, and may not act until it does."
  `atom_requires/2` (`jit_selection.mg:240`, `jit_compiler.mg:287` `missing_dep`) expresses
  atom→atom dependency, not task→knowledge obligation.
- **`selected_atom/1`, `candidate_atom/1` and `mandatory_atom/1` in `jit_selection.mg` are
  inert.** The file says so at `:13-20`: *"selected_atom, candidate_atom and mandatory_atom are
  admissions, and nothing queries them."* Wiring `selected_atom` into `tentative` was measured on
  2026-08-11 and rejected — a `/fix` compile went from 67 atoms / 26 279 tokens to 254 atoms /
  65 036 of 65 536 tokens (99.2% saturation) because each admitted atom recursively pulls its
  `atom_requires` closure. The rule drawn from that: **"a second opinion in a selector may veto,
  never admit."** Any §6 design that wants to *force* knowledge in has to answer this
  measurement.
- **The north star is injected, not obliged.** `.nerd/northstar.mg` is an import surface
  (`internal/northstar/bridge.go:27-43`; the durable record is
  `.nerd/northstar_knowledge.db`). Its policy consequence is
  `injectable_context("*", Desc)` rows (`policy/prompt_northstar.mg:152-179`) — mission, critical
  capabilities, unmitigated risks, constraints. `must_have_requirement/2`
  (`prompt_northstar.mg:58`, declared `schemas_misc.mg:133`) is derived from
  `northstar_requirement` with priority, and `unmitigated_risk/1` (`:23`) feeds
  `strategic_warning/3` (`:110-114`) — **warnings and context, not obligations**. Nothing gates a
  turn on a `must_have_requirement` being satisfied.
  The same `prompt_northstar.mg` block records a live wiring bug that had exactly this effect for
  a while (`:142-150`): the rules tagged slot 1 with a category name instead of a shard id or
  wildcard, so *"no reader ever selected these rows and northstar context reached no prompt."*

## 6. Design: the obligation fixpoint

### 6.0 The shape the architect asked for

> *"the northstar system needs to put pressure on the harness's agents to make the system what we
> want... and the northstar needs to be literally a north star... final state... one that you keep
> reaching towards, not a goal to be hit per se."*

That sentence settles a design question the rest of this section otherwise has to guess at:
**where do obligations come from?** Not from the task, and not from a Go controller. From the
distance between the evidence and a declared final state.

Three consequences, stated plainly before any Mangle:

1. **`task_incomplete` is not a boolean about a task; it is a projection of distance.** A turn is
   incomplete when the evidence it produced leaves the system further from the final state than
   the work it claimed would have. A final state is never reached, so a naive
   "loop until `!task_incomplete`" is a non-terminating loop by construction. The loop must
   terminate on **`no_further_progress_available_here`**, not on "distance = 0".
2. **The unit of pressure is a `gap`, and a gap has an owner.** A gap that no agent in this turn
   can close is not that turn's obligation — it is a *standing* obligation, which is the thing the
   system has none of today (§3: every obligation dies with the turn).
3. **The north star's own vocabulary already exists.** `northstar_capability(CapID, Description, Timeline, Priority)`
   with `Timeline` in `/now | /6mo | /1yr | /3yr | /moonshot` (`schemas_misc.mg:47`,
   `prompt_northstar.mg:90-105`), `northstar_requirement(ReqID, Type, Description, Priority)`
   (`:77`), `northstar_supports(ReqID, CapID)` (`:80`), `northstar_addresses(ReqID, RiskID)`
   (`:83`), `northstar_risk(RiskID, Description, Likelihood, Impact)` (`:58`),
   `northstar_mitigation(RiskID, Strategy)` (`:61`), `northstar_constraint(ConstraintID, Description)`
   (`:90`). **Nothing new has to be invented to name the final state.** What is missing is the
   *evidence* side and the *distance* rule.

### 6.1 The missing half: evidence facts the Go side must assert

Today the Go side measures a great deal and asserts almost none of it. Six new assertions, all
from values that already exist in memory at the moment of assertion:

| Fact | Asserted where the value already is | Replaces |
|---|---|---|
| `edit_made(TurnID, Path, Kind)` — `Kind` in `/create \| /modify` | `executor_tools.go` write path; `result.WrittenPaths`, `result.PreWriteContents` already distinguish them | `result.SuccessfulWriteTools` counting |
| `test_run(TurnID, Package, Outcome)` — `Outcome` from `VerifyOutcome` (`verify_outcome.go:22-36`), i.e. `/passed \| /failed \| /skipped \| /indeterminate \| /canceled` | `build_verify.go:309` `gateTests` | `result.TestCheck` |
| `build_result(TurnID, Outcome)` | `build_verify.go:227` `verifyBuild` | `result.BuildCheck`; also gives `build_state(/failing)` (already consumed at `coder_safety.mg:114`) a real producer |
| `uncovered_block(Path, StartLine, EndLine)` | `build_verify.go:332-337`, already computed and attributed to the turn's changed lines | the `Warn` log that is §3's headline failure |
| `evidence_gathered(TurnID, Entity, Revision, Digest)` | `internal/context/working_set.mg:3` already has `working_observation/5` + `working_digest/2` **[REFUTED - see Verification]** — this is the same thing, promoted from the per-task private scope to the durable kernel | nothing (new) |
| `step_started(TurnID, StepIdx, Path)` / `step_finished(TurnID, StepIdx, Edited)` | `work_steps.go:361,391` | `workStep.Edited` / `unfinishedSteps` in Go |

Plus the tool-loop telemetry from §2 (`tool_round`, `tool_call_result`, `tool_event_novel`,
`durable_write`, `focused_verification`, `tool_trace_cycle`), which replaces
`toolBudgetController`'s private fields.

**Crucially, `uncovered_block/3` is the fact that makes `needs_more_tests/1` come alive.**
`internal/core/defaults/tester.mg:117-118` already says
`needs_more_tests(File) :- coverage_below_goal(File).` and has no producer for `coverage_metric/2`.
The profile that would produce it is already parsed in `coverage_profile.go`.

### 6.2 The Decls

```
# ---------------------------------------------------------------------------
# Distance from the final state. The north star is never reached; the gap is
# what every agent is under pressure from.
# ---------------------------------------------------------------------------

# A named way in which the observed system differs from the declared final
# state. Kind is the family of gap; Subject is the thing the gap is about
# (a file, a package, a capability id); Weight is the priority carried down
# from the northstar fact that produced it.
Decl northstar_gap(Kind, Subject, Weight) bound [/name, /string, /number].

# The same gap, restricted to what THIS turn's evidence could have closed.
# A gap no agent here can close is standing, not this turn's obligation.
Decl actionable_gap(TurnID, Kind, Subject) bound [/string, /name, /string].

# ---------------------------------------------------------------------------
# Obligations: the three the brief names, plus the one that makes the loop
# terminate.
# ---------------------------------------------------------------------------

Decl coverage_missing(TurnID, Path, Why) bound [/string, /string, /name].
Decl knowledge_needed(TurnID, AtomID, Why) bound [/string, /string, /name].
Decl task_incomplete(TurnID, Reason) bound [/string, /name].

# Projections for safe negation (this Mangle does not exclude on a wildcard
# inside a negated multi-arg literal -- see prompt_northstar.mg:196-201 and
# working_set.mg:88-90 for the two existing precedents).
Decl has_task_incomplete(TurnID) bound [/string].
Decl has_actionable_gap(TurnID) bound [/string].
Decl turn_may_stop(TurnID) bound [/string].

# ---------------------------------------------------------------------------
# Progress and stalls as facts, so no Go controller is needed.
# ---------------------------------------------------------------------------

Decl progress_this_round(TurnID, Round) bound [/string, /number].
Decl rounds_since_progress(TurnID, N) bound [/string, /number].
Decl stalled(TurnID, Reason) bound [/string, /name].
Decl has_stalled(TurnID) bound [/string].
```

### 6.3 The rules

```
# --- Gaps derived from the north star, not from the task ------------------

# A requirement the north star declares that no evidence satisfies.
northstar_gap(/requirement_unmet, ReqID, Priority) :-
    northstar_requirement(ReqID, _, _, Priority),
    !requirement_witnessed(ReqID).

# A risk the north star declares with no mitigation. (unmitigated_risk/1
# already exists at prompt_northstar.mg:23; this reuses it rather than
# restating it.)
northstar_gap(/risk_unmitigated, RiskID, Impact) :-
    unmitigated_risk(RiskID),
    northstar_risk(RiskID, _, _, Impact).

# Code the system depends on that nothing exercises. This is the gap the
# vision's "test coverage is not optional" bullet names, and it is a gap
# about the SYSTEM, not about a turn -- it outlives the turn that made it.
northstar_gap(/uncovered_code, Path, 100) :-
    uncovered_block(Path, _, _).

# --- The turn's share of the gap ------------------------------------------

actionable_gap(TurnID, /uncovered_code, Path) :-
    northstar_gap(/uncovered_code, Path, _),
    edit_made(TurnID, Path, _).

actionable_gap(TurnID, /requirement_unmet, ReqID) :-
    northstar_gap(/requirement_unmet, ReqID, _),
    turn_claims_requirement(TurnID, ReqID).

has_actionable_gap(TurnID) :- actionable_gap(TurnID, _, _).

# --- coverage_missing ------------------------------------------------------

# New source with no test. (Today's rule, coder_safety.mg:28/104, restated
# with a reason so the message can name it.)
coverage_missing(TurnID, Path, /no_test_file) :-
    edit_made(TurnID, Path, /create),
    testable_path(Path),
    !test_coverage(Path).

# The one today's corpus cannot say: modified source whose changed lines no
# test executes. This is the promotion of build_verify.go:332-337 from a
# Warn log to an obligation.
coverage_missing(TurnID, Path, /changed_lines_unexercised) :-
    edit_made(TurnID, Path, /modify),
    actionable_gap(TurnID, /uncovered_code, Path).

# A test file was written but no test was run this turn. Today this is only
# caught in the inverse direction (unverified_test_claim, coder_safety.mg:39).
coverage_missing(TurnID, Path, /test_written_never_run) :-
    edit_made(TurnID, Path, _),
    is_test_file(Path),
    !turn_ran_tests(TurnID).

# --- knowledge_needed ------------------------------------------------------
#
# The brief asks whether anything says "this agent must have X in its window
# before acting". Today only the atom author says it (SS5). This derives it
# from the WORK instead, and it is the one rule that must be written with the
# 2026-08-11 saturation measurement in hand (jit_selection.mg:13-20): a
# selector may veto, never admit. So knowledge_needed is NOT wired into
# `tentative`; it is a PRE-ACTION obligation -- the turn may not reach a
# write while it derives.

knowledge_needed(TurnID, AtomID, /domain_required) :-
    turn_target_family(TurnID, Family),
    family_requires_atom(Family, AtomID),
    !atom_in_window(TurnID, AtomID).

knowledge_needed(TurnID, AtomID, /constraint_required) :-
    northstar_constraint(CID, _),
    constraint_atom(CID, AtomID),
    turn_touches_constrained_surface(TurnID, CID),
    !atom_in_window(TurnID, AtomID).

# --- task_incomplete -------------------------------------------------------

task_incomplete(TurnID, /coverage_missing) :- coverage_missing(TurnID, _, _).
task_incomplete(TurnID, /knowledge_missing) :- knowledge_needed(TurnID, _, _).
task_incomplete(TurnID, /build_failing)     :- build_result(TurnID, /failed).
task_incomplete(TurnID, /tests_failing)     :- test_run(TurnID, _, /failed).
task_incomplete(TurnID, /step_unfinished)   :- step_finished(TurnID, _, /false).
task_incomplete(TurnID, /claim_unwitnessed) :- unverified_test_claim_for(TurnID).

has_task_incomplete(TurnID) :- task_incomplete(TurnID, _).

# --- Progress and stalls, replacing the Go controller ---------------------

progress_this_round(TurnID, R) :- tool_event_novel(TurnID, R, _).
progress_this_round(TurnID, R) :- durable_write(TurnID, R, _).
progress_this_round(TurnID, R) :- focused_verification(TurnID, R, _).

stalled(TurnID, /repeated_cycle) :- tool_trace_cycle(TurnID, _).
stalled(TurnID, /no_progress)    :- rounds_since_progress(TurnID, N), stall_rounds(M), N >= M.
stalled(TurnID, /tool_failures)  :- consecutive_failed_rounds(TurnID, N), N >= 3.
has_stalled(TurnID) :- stalled(TurnID, _).

# --- THE TERMINATION RULE -------------------------------------------------
#
# A turn may stop when it has evidence, owes nothing it can still close, and
# is not merely stuck. Note it does NOT require has_actionable_gap to be
# false: the north star is never reached, so a turn that closed everything it
# could close may stop while gaps remain. The gaps that remain become
# standing pressure (6.5), which is the difference between "keep reaching"
# and "never finish".

turn_may_stop(TurnID) :-
    turn_evidence_recorded(TurnID),
    !has_task_incomplete(TurnID).

# A stalled turn also stops -- but it stops REPORTING its open obligations,
# which is not the same thing as being done.
turn_may_stop(TurnID) :- has_stalled(TurnID).
```

### 6.4 How the loop consumes it

Replace `executor_tools.go:207`'s `for iter := 0; openRounds || iter < budget.iterationLimit; iter++`
with a loop whose only exit conditions are:

1. the model returned no tool calls **and** the kernel derives `turn_may_stop(TurnID)` — today
   this is exit A in §4.1, unconditioned;
2. the kernel derives `stalled/2` — today `working_stop/1`, already policy;
3. `ctx.Err()`;
4. the cost ceiling — which **asserts `task_incomplete(TurnID, /budget_exhausted)` rather than
   silently walking away**, so the turn's report names what it still owes.

The critical change is (1). When the model stops calling tools and `has_task_incomplete` derives,
the loop does not end: it appends the open obligations to the history as a user turn and
re-invokes. The obligations render as text the way `workingNudgeText`
(`tool_budget_controller.go:351`) renders a nudge kind today — the policy decides *when*, the Go
side owns the wording. That is a strictly smaller Go surface than today's, because today's Go
*also* decides when.

**The loop cannot terminate while it derives `task_incomplete`, and that is what makes it a
fixpoint rather than a budget.**

### 6.5 Standing pressure: what survives the turn

This is the part with no analogue today. `northstar_gap/3` is not scoped to a turn. When a turn
ends, its `actionable_gap` rows go with it; the `northstar_gap` rows stay. That gives three
things the system currently lacks:

- **Coverage debt is durable.** `northstar_gap(/uncovered_code, Path, 100)` survives, so the next
  turn that touches `Path` inherits an obligation the previous turn created. §3's "there is no
  cross-turn coverage debt" stops being true.
- **Distance is a number the harness can show.** A count (or priority-weighted sum) over
  `northstar_gap/3` is the "how far from the final state" metric the north star currently has no
  way to express, and it is derivable, not computed.
- **Delegation becomes derived from distance.** `delegation.mg:91-93` already demonstrates the
  shape (`impacted(File)` + `!test_coverage(File)` → delegate to the tester). Generalized:
  `should_delegate(/tester, Subject) :- northstar_gap(/uncovered_code, Subject, _).` The north
  star then does what the architect asked — it puts pressure on the agents — without a single new
  Go controller.

### 6.6 What the user sees

Three surfaces, and all three are the same data:

1. **Mid-turn**: an `[obligation]` block appended to the last tool result, one line per open
   `task_incomplete` reason, replacing today's `[orchestrator] Orchestrator budget: N tool calls
   and M rounds remain` (`tool_budget_controller.go:310`). The word "budget" leaves the model's
   context window entirely.
2. **End of turn**: the turn's report names what was closed and what remains, in the shape
   `workStepReport` already uses (`work_steps.go:312-337`) — that ledger is the right precedent
   and should be generalized from steps to obligations.
3. **Standing**: `nerd northstar` (or `/alignment`) shows the current gap set and its trend. This
   is the "north star you keep reaching toward" made legible — today `/alignment` shows an LLM's
   opinion score (`guardian.go:547-600`).

### 6.7 The Go seams that must shrink

| Seam | Today | After |
|---|---|---|
| `toolBudgetController` (`tool_budget_controller.go`, 389 lines) | counts, decides the ceiling, detects cycles, renders nudges, *and* is the sole producer of `working_progress/5` | asserts `tool_round`/`tool_call_result`/`durable_write`/`focused_verification`/`tool_trace_cycle`; keeps the SHA-256 digest (Mangle cannot hash) and the period-1/2/3 comparison (assert `tool_trace_cycle(TurnID, Period)` as a fact); loses `maybeExtend`, `iterationLimit`, `hardLimit`, `nudge` |
| `isFocusedVerificationCall` (`tool_budget_controller.go:243-270`) | a Go switch over tool names plus a hardcoded list of shell-command prefixes | `verification_tool/1` and `verification_command_prefix/1` facts in `.mg`; Go only matches |
| `isCampaignComplete()` (`orchestrator_phases.go:184-203`) | a Go loop over `o.campaign.Phases` | `o.kernel.Query("campaign_complete")`, which already exists (`campaign_phases.mg:145`) and is already correct |
| `unfinishedSteps` / `markCoveredSteps` (`work_steps.go:282-308`) | Go set arithmetic over `workStep` structs | `step_finished/3` facts; `task_incomplete(TurnID, /step_unfinished)` |
| `consumeHollowSuccessVerdict`'s imperative fallbacks (`executor.go:2505-2555`) | a second, imperative copy of four policy rules, for "nil/degraded kernels and MockKernel unit tests" | either delete (a kernel that cannot derive the verdict should fail loudly, per the repo's no-shims rule) or keep *only* for `e.kernel == nil` |
| `selectMangleMandatoryIDs` caps (`selector.go:49-51,396-419`) | Go decides which knowledge is obligatory in a Mangle context, under three Go constants | `knowledge_needed/3`; the caps become policy constants like `working_nudge_rounds(8)` |
| `parseAlignmentResponse` (`guardian.go:547-600`) | scrapes a verdict out of model prose, silently defaulting to `AlignmentWarning` | should not produce a *verdict* at all; it should assert observations, and the gap rules should derive the verdict |

### 6.8 Stratification and cost risks

**Stratification.** Four hazards, three with existing precedent in this corpus:

1. `task_incomplete` → `has_task_incomplete` → `turn_may_stop` uses negation, so `turn_may_stop`
   must sit in a strictly higher stratum than every `task_incomplete` rule. That is fine as long
   as **nothing that produces `task_incomplete` ever reads `turn_may_stop`**. The dangerous
   temptation is `task_incomplete(T, /not_stopped) :- !turn_may_stop(T)` — a negative cycle. Do
   not write it.
2. `northstar_gap` → `actionable_gap` → `coverage_missing` → `task_incomplete` is a clean
   ascending chain provided `northstar_gap` never reads any turn-scoped predicate. Keeping
   `northstar_gap/3` turn-free is therefore not just a design preference (6.5), it is a
   stratification requirement.
3. **Every negation must be over a single-argument projection.** This fork does not exclude
   correctly when a multi-argument literal is negated with wildcards — documented twice in the
   live corpus (`prompt_northstar.mg:196-201`, `working_set.mg:88-90`) and in memory
   (`reference_mangle_wildcard_negation_gotcha.md`). Hence `has_task_incomplete/1`,
   `has_actionable_gap/1`, `has_stalled/1`.
4. `mandatory_selection`-style cycles: `jit_compiler.mg:148-151,169-173` records a real negative
   cycle that had to be broken by introducing `blocked_by_context` as *"a separate predicate that
   derives only from EDB plus blocked_by_context, none of which reads mandatory_selection"*.
   `knowledge_needed/3` will meet the same wall if it ever feeds atom admission. It must not — see
   6.3's comment and §5's measurement.

**Cost.** Three measured risks, all with numbers already on record:

1. **Fixpoint size.** The 2026-08-11 measurement (`jit_selection.mg:13-20`): admitting atoms
   recursively through `atom_requires` took a `/fix` compile from 67 atoms / 26 279 tokens to 254
   atoms / 65 036 of 65 536 tokens. Any rule that *admits* rather than *vetoes* must be measured
   the same way before it ships.
2. **Per-round evaluation.** The obligation set is evaluated at every round boundary, where today
   `working_set.Continue` already runs. The new facts are O(tool calls) per round, not O(corpus),
   so the marginal cost should be small — but `northstar_gap(/uncovered_code, Path, 100)` is
   O(uncovered blocks in the repo), which for this codebase is large. **Mitigation: keep
   `uncovered_block/3` scoped to files the session has touched, and keep the durable debt in
   SQLite** (the same split `bridge.go:38-45` already uses for the vision: the store is the
   durable record, the kernel holds the projection).
3. **LLM cost of the alignment loop.** `observer_manager.go:350-357` measured a check every five
   minutes at 13–43 s per call, producing opinions nobody consumed. A derived gap costs zero LLM
   calls. This design should *reduce* north-star spend, not add to it.

**One honest unknown.** `requirement_witnessed/1`, `turn_claims_requirement/2`,
`family_requires_atom/2`, `atom_in_window/2` and `turn_touches_constrained_surface/2` in 6.3 have
no producers today and no obvious mechanical ones. **[REFUTED - see Verification]** `atom_in_window/2` is the easiest (the compiler
knows exactly which atoms it emitted). The other four are where a design that looks clean on paper
meets the question "who decides that this edit witnesses that requirement?" — and the honest
answer is **unverified**: it is either a model judgment (which reintroduces §7's prose-as-verdict
seam) or a much narrower mechanical link than the north star's vocabulary suggests. That question
is the first one for the architect (§Questions). **[REFUTED - see Verification]**

## 7. Seams found

Places where two components disagree about what "done", "verified", "failed" or "budget" means.
Both sides cited.

### S1 — "done": the kernel derives it, Go logs it

| Side | Claim |
|---|---|
| `internal/core/defaults/policy/coder_safety.mg:108-115` | `turn_done/1` is *"the single completion signal"*, and it requires `turn_executed` **and** `turn_acceptance` |
| `internal/session/executor.go:2563-2572` `consumeTurnDoneSignal` | queries `turn_done`, and if the count is not exactly 1, writes a `Debug` line. The turn's fate is unchanged |

The call site states the divergence as policy: *"The consumer logs any deviation for diagnosis
without changing the verdict hollow_success already determined."* (`executor_tools.go:1972-1975`).
So the corpus's positive completion predicate and the runtime's completion behaviour are two
different things. Consequence: on any turn without a host-verifier acceptance
(`executor.go:2334` requires `result.Acceptance.Status == "verified"`), `turn_done` does not
derive and the turn succeeds anyway.

### S2 — "campaign complete": derived in Mangle, computed in Go, in the same function pair

| Side | Claim |
|---|---|
| `policy/campaign_phases.mg:139-150` | `campaign_complete(CampaignID) :- current_campaign(CampaignID), !has_incomplete_phase(CampaignID).` and `next_action(/campaign_complete) :- campaign_complete(_).` |
| `internal/campaign/orchestrator_phases.go:184-203` `isCampaignComplete()` | a Go `for` loop over `o.campaign.Phases` checking `Status == PhaseCompleted \|\| PhaseSkipped` |

`orchestrator_execution.go:171` calls the Go one. The adjacent
`getCampaignBlockReason()` (`orchestrator_phases.go:206-222`) *does* call
`o.kernel.Query("campaign_blocked")`. So within twenty lines of each other, "blocked" is derived
and "complete" is not.

### S3 — "verified": four different meanings

| Component | What "verified" means |
|---|---|
| `internal/session/verify_outcome.go:20-36` | one of five `VerifyOutcome` values from a subprocess; `VerifyIndeterminate` exists precisely because *"a build that timed out after four minutes is not 'not run', and it is not 'failed' either"* |
| `internal/verification/verifier.go:242-259` | an **LLM judge's** JSON `{"success": bool, "quality_violations": [...]}` (`verifier.go:311-340`) |
| `coder_safety.mg:112-115` | `turn_acceptance(Verb, Contract, Snapshot)` — *"Only the host verifier emits acceptance for an immutable caller contract with current, executed behavioral witnesses"* |
| `internal/northstar/guardian.go:547-600` | a `RESULT:` line scraped from model prose, defaulting to `AlignmentWarning` when unparseable |

Two of the four are model prose. `coder_safety.mg:112-113` is the only one that states a contract
(*"Execution is weaker than completion"*), and it is the one whose consumer only logs (S1).

### S4 — "budget": two enforcement sites, two vocabularies, one of them in the model's window

| Side | Claim |
|---|---|
| `internal/session/executor_tools.go:792-815` | `MaxToolCalls` truncates a batch mid-flight, feeding the model the literal tool result `"tool call budget exceeded for this turn"` |
| `internal/session/tool_budget_controller.go` + `executor_tools.go:207,393-404` | `MaxToolIterations` + adaptive extensions; on exhaustion it *asks for a conclusion* rather than truncating |
| `internal/context/working_set.mg:55-58` | the policy's four constants (8/16/16/24 rounds) — a third, independent set of thresholds that does not know about either ceiling |

Three ceilings that never reference each other. `newToolBudgetController` (`:86-88`) does one
reconciliation — clamping `hardLimit` to `maxCalls` — and nothing reconciles either with the
policy constants. A user who sets `max_tool_iterations` on a progress-driven loop gets *both* the
policy and the count ceiling, which the code acknowledges at `executor_tools.go:247-252`.

### S5 — "failed": a timeout is not a failure, except where it is

| Side | Claim |
|---|---|
| `build_verify.go:234-241`, `:344-351` | `VerifyIndeterminate` → *"a timeout is not evidence of breakage. The turn completes unverified"*; no error |
| `build_verify.go:253-261`, `:369-381` | a recheck that is indeterminate leaves the **original failure** standing — *"Only affirmative verdicts move the check"* |
| `coder_safety.mg:114` | `turn_executed` requires `!build_state(/failing)` — a two-valued view with no room for indeterminate |
| `verifier.go:243-252` | verification that *could not run* is recorded as a **failed** verification and stops retrying (`ErrVerificationUnavailable`) |

`VerifyOutcome` is a careful five-valued type; `build_state/1` collapses it to two; the
verification package collapses "could not judge" into "failed". Three encodings of the same
distinction.

### S6 — coverage: measured precisely, asserted nowhere, and a dead predicate waiting for it

| Side | Claim |
|---|---|
| `build_verify.go:309-337` + `coverage_profile.go` | computes per-block coverage attributed to the turn's changed lines, with the comment *"Green tests over code that was never executed is the precise false success this signal exists to expose"* |
| the same code, `:334-337` | `logging...Warn(...)`. No fact, no decision **[REFUTED - see Verification]** |
| `internal/core/defaults/tester.mg:109-124` | `coverage_metric/2`, `coverage_goal/1`, `coverage_below_goal/1`, `needs_more_tests/1`, `coverage_warning/3` — **zero Go consumers and no producer** |
| `policy/coder_safety.mg:19` | a *second*, much weaker coverage notion: `test_coverage(SourceFile) :- test_file_for(_, SourceFile).` (the `x_test.go` convention) |
| `internal/session/test_verify.go:177-180` | a *third*: a file is covered if its own `_test.go` exists **or its package contains any `_test.go` at all** |

Three definitions of "covered" in three components, and the most precise measurement in the system
feeds none of them.

### S7 — the coverage obligation exists in two places with different reach

| Side | Claim |
|---|---|
| `coder_safety.mg:104-107` | `hollow_success("new source was created without a test file")` — **created** source only, per-turn, fails the turn |
| `delegation.mg:91-93` | `delegate_task(/tester, "Generate tests for impacted code", /pending)` — any **impacted** file lacking coverage, delegates, blocks nothing |
| `impact.mg:16-22` | `unsafe_to_refactor/1` and `block_refactor/2` — derived from the same join, **consumed only by shadow mode** (`internal/core/shadow_mode.go:450`); `block_refactor` has no consumer at all |

Same predicate join (`impacted`/`created` + `!test_coverage`), three consequences: fail the turn,
delegate, or nothing.

### S8 — the policy verdict has an imperative twin

`internal/session/executor.go:2505-2555` reimplements four `hollow_success` rules in Go, as a
fallback *"for kernels without the new turn_evidence rules (unit-test mocks, older snapshots)"*.
It is guarded — it only runs when no `hollow_success` fact was derived (`:2507-2508`) — but it is
a second copy of the same logic in the codebase whose root `CLAUDE.md` forbids exactly this
("NO SHIMS, EVER... it lets two truths coexist so the next reader cannot tell which one is live").
Note the Go copy's reach differs: `:2510-2517` fires on `intentRequiresToolCall(verb) || writeOrientedIntent(verb)`,
while the Mangle rule at `coder_safety.mg:92-95` requires `intent_requires_tool_call(Verb)` alone.

### S9 — "budget exhausted" and "task done" take the same exit

`executor_tools.go:428-440`: whether the turn ended because the working policy finalized it
(`finalizeReason != ""`) or because the count ceiling ran out, both route into `forceFinalAnswer`.
The user-facing difference is a log line. There is no fact, and no user-visible statement, that
distinguishes "this is the answer" from "this is what I had when the budget ran out" — except the
separate, deadline-only message at `executor_tools.go:1892` (*"the work was progressing, not
stuck; the operation budget ran out"*), whose own comment (`:1864-1866`) says the previous text
*"is unactionable: it names neither the budget that expired, nor how much work was"* done.

### S10 — the guard set is protected; the thing it guards is prose

`write_guards.go:9-28` is unusually explicit: the three pre-write guards were consolidated into
one write-protected unit because *"Observed 2026-08-12: when the modularity guard refused a write,
the agent's next action was to edit the guard to exempt its own file."* Meanwhile
`work_steps.go:243-252` lets a step discharge its obligation with a `NO CHANGE NEEDED:` prefix in
free text that nothing checks, and `guardian.go:547-600` takes a campaign-blocking verdict from a
`RESULT:` line. The system knows the difference between a fact and a request — it says so at
`executor_tools.go:1371-1372` — and applies it inconsistently.

### S11 — `.claude/rules/nerd-config-schema.md` is stale on `core_limits`

The rule doc describes `core_limits` as *"Memory MB, concurrent shards/API, session minutes,
kernel fact ceiling"*. `internal/config/limits.go:17-47` adds six more fields — `max_tool_calls`,
`max_tool_iterations`, `adaptive_tool_budget`, `tool_iteration_extension_size`,
`max_tool_iteration_extensions`, `tool_loop_repeat_threshold` — every one of which is a forcing
knob. Given strict JSON decoding (`decodeStrictJSON`), a reader of that doc has no way to know
these keys are legal.

### S12 — `LevelBlock` blocks nothing

`internal/shards/observer_manager.go:61-77` defines `LevelBlock AssessmentLevel = "block" // Score < 40: Block for review`
and `GetAssessmentLevel` returns it. The only other occurrence of `LevelBlock` in the repository is
an emoji map at `:580`. An observer can return a "block" assessment and the system renders 🚫.

---

## Verification (adversarial, 2026-09-18)

### Status

last updated: 2026-09-18 (complete) · checked: 79 · refuted: 9 (3 wrong-claim, 2 wrong-location, 3 overstated, 1 unverifiable)

| # | section | claim (short) | cited location | verdict | evidence | note |
|---|---------|---------------|----------------|---------|----------|------|
| 1 | 1 (row 1) | iteration ceiling: `newToolBudgetController` at `tool_budget_controller.go:60`, consumed at `executor_tools.go:207`; Go pure counting; model cannot talk past a `for` bound | `tool_budget_controller.go:60`, `executor_tools.go:207,393,428` | holds | `tool_budget_controller.go:60` `func newToolBudgetController(cfg ExecutorConfig)`; `executor_tools.go:207` `for iter := 0; openRounds \|\| iter < budget.iterationLimit; iter++` | Log format string is on `:430`, the `logging...Warn(` call on `:429`; the doc cites both in different tables. Harmless |
| 2 | 1 (row 2) | adaptive extension at `tool_budget_controller.go:192` `maybeExtend`; signal = novel successful evidence + no repeated tail cycle + capacity | `tool_budget_controller.go:192-238` | holds | `:192` `func (c *toolBudgetController) maybeExtend(writeOriented bool)`; `:210` `if c.progressSinceExtension == 0`; `:205` `repeatedTailCycle()`; `:201` capacity | Incomplete, not wrong: `:214-218` adds a fourth condition for write-oriented intents (a durable write or post-write verification since the boundary). Both scoring columns are right |
| 3 | 1 (row 3) | `read_only_stall` is **Mangle** at `working_set.mg:85`; Go asserts counts; error text at `executor_tools.go:288` | `working_set.mg:85-87`, `executor_tools.go:277-289` | holds | `working_set.mg:85-87` `working_stop(/read_only_stall) :- working_progress(/write, Rounds, 0, _, _), working_stall_rounds(N), Rounds >= N.`; `working_set.mg:58` `working_stall_rounds(24).`; `executor_tools.go:288` `"task unresolved: working continuation stopped by policy %s after %d executed tools"` | Both scoring columns correct |
| 4 | 1 (row 4) | `verify_after_write` is **Mangle** at `working_set.mg:97`; Writes>0, SinceWrite>=16, SinceVerify>=16; log at `executor_tools.go:216` | `working_set.mg:93-99`, `executor_tools.go:211-220` | holds | `working_set.mg:97-99` plus `:57` `working_finalize_rounds(16).`; `executor_tools.go:216-218` `"Working policy finalized the turn (%s) after %d executed tool call(s); forcing a final answer"` | Both scoring columns correct |
| 5 | 1 (row 5) | `/commit` regime is **Mangle** at `working_set.mg:60,68,76`; three triggers; catalog narrows | `working_set.mg:40-78`, `executor_tools.go:295-302` | holds | heads at `:60`, `:68`, `:76`; `:56` `working_commit_rounds(16).`; `:55` `working_nudge_rounds(8).`; `executor_tools.go:298-299` commit-regime Warn; `tool_budget_controller.go:365` `workingRegimeText` | Third trigger is `working_regime_now(/commit), SinceVerify > 0` — the doc's phrasing is accurate |
| 6 | 1 (row 6) | `working_stop(/repeated_cycle)` at `working_set.mg:80`; Go detects the cycle at `tool_budget_controller.go:276` | `working_set.mg:80`, `tool_budget_controller.go:276-302` | holds | `working_set.mg:80` `working_stop(/repeated_cycle) :- working_control(/yes, _).`; `:276` `func (c *toolBudgetController) repeatedTailCycle() bool`, full event signatures compared | Split ("Go measures, Mangle decides") correctly characterised |
| 7 | 1 (row 7) | `working_stop(/tool_failures)` at `working_set.mg:81`; Go counts `failedRounds` at `executor_tools.go:261-268` | `working_set.mg:81`, `executor_tools.go:262` | holds | `working_set.mg:81` `working_stop(/tool_failures) :- working_control(_, Failed), Failed >= 3.`; `executor_tools.go:262` `failedRounds++`, reset at `:265` on any non-error result | — |
| 8 | 1 (row 8) | `working_nudge` heads at `working_set.mg:102,105,108`; wording is Go at `tool_budget_controller.go:351`; model **can** ignore it | `working_set.mg:101-110`, `tool_budget_controller.go:351-361` | holds | heads at `:102`, `:105`, `:108`; `tool_budget_controller.go:351` `func workingNudgeText(kind string, p working.WorkingProgress) string`; delivered by `appendToolBudgetNudge` (`:372-379`) as text appended to the last tool result | "Can the model talk past it: Yes" is correct — nothing consumes the nudge's effect |
| 9 | 1 (row 9) | `budget.nudge` at `tool_budget_controller.go:304`; critical when `roundsLeft <= 2` or `callsLeft <= 4`; prose | `tool_budget_controller.go:304-330`, `executor_tools.go:253-260` | holds | `:304` `func (c *toolBudgetController) nudge(...)`; `:312` `critical := roundsLeft <= 2 \|\| callsLeft <= 4`; `:310` `"Orchestrator budget: %d tool calls and %d rounds remain."` | — |
| 10 | 2 | **load-bearing**: `toolBudgetController.workingProgress` (`:334`) is the sole producer of `working_progress/5` | `tool_budget_controller.go:334` | overstated | The *fact* is asserted at `internal/context/working_set.go:181` `{Predicate: "working_progress", Args: ...}` inside `WorkingSet.Continue`. The sole **production** supplier of that struct is `executor_tools.go:275` `progress := budget.workingProgress(writeOriented, failedRounds)` (plus `:293` for nudge text). Every other `WorkingProgress` literal in the tree is a test (`internal/context/working_set_test.go:65-78`, `context_unit1_uplift_test.go:135`) | Substance survives: no production path other than the controller feeds `working_progress/5`. But the assertion site that would have to move is `working_set.go:181`, not `tool_budget_controller.go:334` |
| 11 | 2 | Deleting the controller stops `read_only_stall`, `verify_after_write`, `/commit` and all three nudges from deriving | `working_set.mg:60-110` | holds | Every one of those rules carries `working_progress(...)` in its body (`:61,69,78,86,98,103,106,109`); with no producer the family goes silent | Understated if anything: `working_control/2` (feeding `/repeated_cycle` and `/tool_failures`) is asserted from the same struct at `working_set.go:180`, so **five** of the six policy stops die, not three |
| 12 | 2 | `MaxToolCalls` enforced at `executor_tools.go:792-815`; content `"tool call budget exceeded for this turn"` (`:812`), error `"<name>: budget exceeded"` (`:815`) | `executor_tools.go:792-815` | holds | `:808` `if !openCalls && result.ToolCallsExecuted >= maxToolCalls`; `:812` and `:815` verbatim as quoted | One qualifier the doc omits: `:794` `openCalls := activeWorkingLoop(ctx) != nil && execCfg.ProgressDrivenTools && execCfg.MaxToolCalls == 0` disables this ceiling entirely on a fully open progress-driven loop |
| 13 | 2 | config-key table: six `core_limits` keys, their defaults and validation ranges | `internal/config/limits.go:119-133,138-152` | holds | `:119` `max_tool_calls must be >= 0`; `:122` same for iterations; `:125` `between 0 and 64`; `:128` `between 0 and 8`; `:131` `must be 0 or between 2 and 8`; `:139,147` `adaptiveToolBudget := true` | Code defaults 50/8 live in `internal/session/executor.go:382-383`, as the doc says elsewhere |
| 14 | 2 | "Every file" table: `ExecutorConfig` budget fields at `executor.go:257-265` | `executor.go:257-265` | wrong-location | The seven budget fields span `executor.go:250-278`: `MaxToolCalls` `:252`, `MaxToolIterations` `:257`, `ProgressDrivenTools` `:261`, `AdaptiveToolBudget` `:267`, `ToolIterationExtensionSize` `:271`, `MaxToolIterationExtensions` `:274`, `ToolLoopRepeatThreshold` `:278` | Cited range covers one and a half of seven fields |
| 15 | 2 | `tool_budget_controller.go` is 389 lines and holds the listed symbols | `tool_budget_controller.go` | holds | 389 lines; every named symbol present (`toolBudgetController:18`, `toolBudgetObservation:44`, `toolBudgetExtensionDecision:51`, `observe:106`, `maybeExtend:192`, `repeatedTailCycle:276`, `nudge:304`, `workingProgress:334`, `workingNudgeText:351`, `workingRegimeText:365`, `appendToolBudgetNudge:372`, `isFocusedVerificationCall:243`, `toolBudgetEventSignature:163`) | Two unlisted helpers also live there: `digestStrings:181`, `hasToolDefinition:381` |
| 16 | 1 (row 10) | `hollow_success/1` is Mangle at `coder_safety.mg:92-107` over Go-asserted `turn_evidence/6`; model cannot talk past the four derived reasons | `coder_safety.mg:68-107` | holds | `coder_safety.mg:79` `Decl turn_evidence(Verb, ToolCount, WriteCount, TestCount, ClaimedOutput, DreamMode)`; heads at `:92`, `:96`, `:101`, `:104`; asserted at `executor.go:2357-2364` | The caveat in the "talk past it" column (a write by a tool Go does not classify as a write-mutation tool is invisible) is correct: `has_turn_write` reads `result.SuccessfulWriteTools` |
| 17 | 1 (row 11) | `turn_done/1` at `coder_safety.mg:115` requires `turn_executed` **and** `turn_acceptance` | `coder_safety.mg:108-115` | holds | `:114` `turn_executed(Verb) :- turn_evidence(...), !has_hollow_success(), !build_state(/failing).`; `:115` `turn_done(Verb) :- turn_executed(Verb), turn_acceptance(Verb, _, _).`; `:108` comment "turn_done is the single completion signal" | — |
| 18 | 1 (row 12) | `missing_test_for/1` -> `hollow_success("new source was created without a test file")`; conservative `x_test.go`-only coverage; created-only | `coder_safety.mg:19-28,104-107` | holds | `:19` `test_coverage(SourceFile) :- test_file_for(_, SourceFile).`; `:28` `missing_test_for(File) :- created_source(File), !test_coverage(File).`; `:104-107` the hollow rule joining `turn_created_source(File)`; comment `:12-18` "cautious rather than falsely permissive" | Both scoring columns correct |
| 19 | 1 (row 13) | `unverified_test_claim/1` forces "stop as failure (**via hollow_success line 101**)" | `coder_safety.mg:30-39,101-103` | **wrong-claim** | `coder_safety.mg:101-103` derives `hollow_success` **independently**, from `turn_evidence(Verb, _, _, _, /true, /false), !has_turn_test(Verb)` — it never mentions `unverified_test_claim`. The predicate's only consumer in the tree is `internal/session/executor.go:2527` `if unverified, err := e.kernel.Query("unverified_test_claim")`, which sits **inside the imperative fallback** the doc itself describes in S8 (`:2525-2526` "Fallback for kernels without the new turn_evidence rules ... consult the legacy derived predicates") | The forcing outcome still holds (the turn fails at `:2530`), but by a different and weaker route: the fallback runs only when the kernel derived no `hollow_success` (or when the new-source case fell through). The corpus's own comment calls this rule "legacy" (`executor.go:2329`) |
| 20 | 1 (row 14) | nerd.md write guard, Go, `write_guards.go:32` -> `projectForbidsWrite`, pre-write refusal + audit | `write_guards.go:32-38` | holds | `:32` `if reason, denied := e.projectForbidsWrite(call); denied`; `:35` `logging.Audit().SafetyCheck("nerd.md_write_guard", false, reason)`; `:36` `"blocked by nerd.md: %s is write-protected (%s)"` | — |
| 21 | 1 (row 15) | placeholder-test guard, `write_guards.go:39` -> `writesPlaceholderTestFile` (`executor_tools.go:1379`); the "prose is a request" quote | `executor_tools.go:1369-1400`, `write_guards.go:39-45` | holds | `write_guards.go:39,43`; `executor_tools.go:1379` `func (e *Executor) writesPlaceholderTestFile(call ToolCall) (string, bool)`; `:1371-1372` "Prose is a request the model complies with most of the time, while a fact checked before the tool runs is one no amount of model conviction gets past." | Quote verbatim |
| 22 | 1 (row 16) | modularity guard at `write_guards.go:46`; guard set write-protected as one unit; the 2026-08-12 observation | `write_guards.go:9-53` | holds | `:46,50`; `:15-17` "Observed 2026-08-12: when the modularity guard refused a write, the agent's next action was to edit the guard to exempt its own file." | Quote verbatim |
| 23 | 1 (row 17) | test-removal guard: `removedTestFunctions` (`test_removal_guard.go:19`), gate at `executor_tools.go:501-504`, AST-based, moved test exempt | `executor_tools.go:497-505`, `test_removal_guard.go:14-31` | holds | `test_removal_guard.go:19` `func removedTestFunctions(workspace string, writtenPaths []string, preWrite map[string]string) []string` with `go/ast`/`go/parser` imports; `:17-18` moved-test exemption; `executor_tools.go:502-503` returns `ErrVerificationFailed` with the quoted message; `:499-500` "Runs after the test gate and the gofmt pass" | — |
| 24 | 1 (row 18) | post-edit build gate + repair: `build_verify.go:208` `verifyAndRepairBuild`, trigger at `:218-224`, loud failure at `:199-201` | `build_verify.go:190-272` | holds | `:208` func; `:218` `if !e.configSnapshot().VerifyBuildAfterEdits`; `:222` `result.SuccessfulWriteTools == 0 \|\| !touchedGoFiles(result.WrittenPaths)`; `:227` `verifyBuild(ctx, workspace, nil)`; `:199-201` "the turn fails loudly with the errors, the cost ledger, and follow-ups" | — |
| 25 | 1 (row 19) | post-edit test gate: `build_verify.go:285` `verifyAndRepairTests`; `gateTests` at `:398`; indeterminate does not fail | `build_verify.go:274-393` | holds | `:285` func; `:398` `func gateTests(...)`, called at `:309`; `:344-346` "a timeout is not evidence of breakage. The turn completes unverified" | — |
| 26 | 1 (row 21) | `ErrStepsIncomplete`: `work_steps.go:33` and `:431-434`; `unfinishedSteps` at `:297-308`; `NO CHANGE NEEDED:` escape at `:243-252` | `work_steps.go:18-33,243-252,296-308,339-436` | holds | `:33` `var ErrStepsIncomplete = errors.New("planned steps incomplete")`; `:431-433` the quoted error; `:246` `if !strings.HasPrefix(trimmed, "NO CHANGE NEEDED:")`; `:300-302` `if s.NoChange != "" { continue }` | "Partly — a step escapes by prose matched on a prefix" is exactly right |
| 27 | 1 (row 22) | step retry with reading closed at `work_steps.go:373-390`, under `toolLoopPass{regime: commitRegime}` | `work_steps.go:373-390` | holds | `:373` `if result.SuccessfulWriteTools == writesBefore`; `:374-376` the quoted Warn; `:377` `toolLoopPass{regime: commitRegime}` | — |
| 28 | 1 (row 23) | `maxPlannedSteps` const 12 at `work_steps.go:37` | `work_steps.go:35-37` | holds | `:37` `const maxPlannedSteps = 12`; enforced at `:113` `if len(steps) == maxPlannedSteps` | — |
| 29 | 3 | the created-source obligation is consumed at `executor.go:2471-2494`, failing with `turn created Go source %s without a test file (verb %s)` | `executor.go:2471-2494` | holds | `:2471` `case "new source was created without a test file":`; `:2493` `return newHollowSuccessError("turn created Go source %s without a test file (verb %s)", matched[0], verb)` | `:2496-2500` confirms the deliberate fall-through when no per-turn creation matched |
| 30 | 3 | the two coverage signals are computed and **"neither is an input to any decision"** (also S6 "No fact, no decision") | `build_verify.go:303-307,309-337` | **wrong-claim** | `result.UncoveredBlocks` is read twice more: `build_verify.go:636` `grounding := summarizeUncovered(result.UncoveredBlocks)` grounds the critic prompt, and `:698` puts it in the turn-signal line. **Both** signals are also carried out of the turn at `internal/session/observed_return.go:93-100`, into `observation.Return.Notes` a parent agent reads ("written with no test file alongside: ...", "%d block(s) in this turn's own files executed by no test") | The defensible version: neither becomes a **kernel fact** and neither **gates** the turn. But the uncovered-block measurement does reach an LLM decision (the critic, whose uplift edits can fail the turn via the mechanical rechecks, `executor_tools.go:507-508`) and does reach a parent agent. `observed_return.go:88-92` states the design: "both are advisory in the executor ... uncertainty rather than findings" |
| 31 | 3 | `tester.mg:109-124` coverage apparatus has zero Go consumers and no producer for `coverage_metric`/`coverage_goal`; `needs_more_tests/1` is dead | `internal/core/defaults/tester.mg:109-124` | holds | `tester.mg:109-110` Decls, `:113-114`, `:117` `needs_more_tests(File) :- coverage_below_goal(File).`; repo-wide grep over `*.go` for all five names returns **zero** hits; `internal/core/defaults/testdata/starved_predicates.txt:28-29` lists `coverage_goal` and `coverage_metric` as starved | One addition: `coverage_metric`/`coverage_goal` are also read by a second Mangle rule at `tester.mg:201-202`, so `needs_more_tests` is not their only would-be consumer |
| 32 | 3 | `block_refactor` appears in Go only in two context-scoring weight tables; `unsafe_to_refactor` only in shadow mode | `internal/context/types.go:80`, `internal/context/activation_scoring.go:167`, `internal/core/shadow_mode.go:450` | holds | Grep over `*.go`: `types.go:80` `"block_refactor": 90,`; `activation_scoring.go:166-167` `"unsafe_to_refactor": 50.0, "block_refactor": 50.0,`; `shadow_mode.go:450` `queryOrBlock(sm.shadowKernel, actionID, "unsafe_to_refactor")` | `unsafe_to_refactor` also appears in the `activation_scoring.go:166` weight table, which the doc omits; the claim "no production path blocks a refactor for uncovered dependencies" holds |
| 33 | 4.0 / Q-premise 1 | `turn_done/1`'s only consumer is `consumeTurnDoneSignal` (`executor.go:2563-2572`) and it writes a Debug line only | `executor.go:2563-2572` | holds | `:2563` `func (e *Executor) consumeTurnDoneSignal(verb string)`; `:2567-2571` exactly the two `Debug` branches the doc quotes; function ends `:2572`. Repo-wide, the only other reads of `turn_done` are `internal/session/executor_memory.go` (`TurnOutcome`) and `internal/system/factory_learning.go:270` | Premise 1 holds verbatim, including the line range |
| 34 | 4.0 / Q-premise 2 | `turn_acceptance` is asserted only when `result.Acceptance.Status == "verified"` (`executor.go:2334`), so `turn_done` never derives on an ordinary turn | `executor.go:2334` | holds | `:2334` `if result.Acceptance != nil && result.Acceptance.Status == "verified" {` then `:2335` asserts `turn_acceptance` | Premise 2 holds |
| 35 | 4.1 | exit A: `executor_tools.go:383-386` `if len(nextResp.ToolCalls) == 0 { return verifyTerminal(...) }` — the model stopping is the normal exit | `executor_tools.go:383-386` | holds | `:383-386` verbatim; nothing between `:378` and `:386` consults policy | The six-exit table's other five rows (B `:211-220`, C `:282-289`, D `:207`->`:428-440`, E `:225-235`/`:337-363`, F `:208-210`/`:304-307`) all match the file |
| 36 | 4.2 | `verifyTerminal` gates only when `pass.verify` is set; a planned-steps pass sets it false and verifies once at the end | `executor_tools.go:197-205`, `work_steps.go:69-72`, `:425-426` | holds | `executor_tools.go:198` `if !pass.verify { return response, nil }`; `work_steps.go:69-72` the `toolLoopPass` doc comment "a planned task verifies once after its last step instead"; `:363` step pass uses `toolLoopPass{}` (verify false); `:425-426` the single whole-task `verifyCompletedToolTurn` | — |
| 37 | 4.3 | planned steps: plan bounded (`:157`, `:42`, `:37`, `:44-49`); step ends on write delta (`:373`, `:391`); task fails at `:431-434`; `NO CHANGE NEEDED:` is unchecked prose | `work_steps.go:157,243-252,272,341-342,363,373-391,402-404,425-434` | holds | `:157` `func (e *Executor) planTurnSteps(...)`; `:42` `const planStepsTimeout = 2 * time.Minute`; `:272` prompt asks for "the file and line you checked and what is there"; `:402-404` `if !step.Edited { step.NoChange = noChangeEvidence(step.Note) }`; nothing reads the claimed file/line | "Everything here is computed in Go ... `work_steps.go` never touches the kernel" — confirmed: no `kernel` reference in the file |
| 38 | 3 / Q-premise 4 | `turn_created_source` is scoped per turn at `executor.go:2394-2424` so a leaked `created_source` "cannot fail later turns forever" | `executor.go:2394-2424` | holds | `:2394-2398` the comment the brief cites; `:2399-2424` the implementation that mirrors only `perTurnCreatedSourceFacts` into `turn_created_source` | The brief's own citation `:2394-2398` covers the comment only; the mechanism runs to `:2424`, which is what the document cites |
| 39 | 1 (row 24) | `VerifyWithRetry` (`verifier.go:206-284`), LLM-judge verdict, `ErrMaxRetriesExceeded`; "the deciding input is one model's prose about another model's prose"; fails closed when the judge is unavailable | `verifier.go:206-284,302-340` | holds | `:206` `func (v *TaskVerifier) VerifyWithRetry(`; `:242` `v.verifyTask(ctx, currentTask, result)`; `:302` `verifyTask uses LLM to assess`; `:26-27` `ErrMaxRetriesExceeded = errors.New("max retries exceeded - escalating to user")`; `:251` returns `ErrVerificationUnavailable` when the judge could not run; `:83` `QualityViolations ... json:"quality_violations,omitempty"` | The "Who computes the signal = the model" column is right |
| 40 | 1 (row 25) / 1a | Guardian `CheckAlignment` (`guardian.go:328`) blocks at campaign start; the verdict is scraped from prose by `parseAlignmentResponse` (`:547-600`), defaulting to `AlignmentWarning` | `guardian.go:328,547-600,908`, `observer.go:54-105` | holds | `:328` func; `:547-600` the line scraper (`SCORE:` `:560`, `RESULT:` `:566`, `EXPLANATION:` `:582`, `SUGGESTIONS:` `:584`); `:549-551` the defaults incl. "Unable to parse alignment response"; `:597-598` `if !explicitResult { check.Result = g.classifyScore(check.Score) }`; `observer.go:69` `"campaign goal does not align with vision: %s"` | "Can the model talk past it: Yes" is right — a `RESULT: passed` line is sufficient |
| 41 | 1 (row 26) / 1a | Northstar periodic check is scheduled by Go every N tasks (default 5) and is **recorded, not enforced** | `observer.go:33-50,159` | holds | `observer.go:35` `checkEveryNTasks := 5`; `:39-41` overridden by `guardian.config.PeriodicCheckInterval`; `:159` `if tasksInPhase > 0 && tasksInPhase%o.checkEveryNTasks == 0`. Stronger evidence than the doc gives: the sole caller **discards both return values** — `internal/campaign/orchestrator_tasks.go:613` `_, _ = o.northstarObserver.OnTaskComplete(...)` | By contrast `OnPhaseStart` is consumed at `orchestrator_phases.go:332`, which is why the phase gate is pressure and this is not |
| 42 | 1 (row 27) | campaign phase checkpoint derived in Mangle; a `/false` checkpoint derives `phase_blocked(PhaseID, /checkpoint_failed)` | `campaign_phases.mg:86-98` | holds | `:86-88` `all_phase_tasks_complete`; `:91-94` `next_action(/run_phase_checkpoint)`; `:97-98` `phase_blocked(PhaseID, /checkpoint_failed) :- phase_checkpoint(PhaseID, _, /false, _, _).` | — |
| 43 | 1 (row 28) | `campaign_blocked/2` at `campaign_phases.mg:153-157`; consumed, then `failCampaign` | `campaign_phases.mg:153-157`, `orchestrator_phases.go:206-222` | holds | `:153-157` verbatim; `orchestrator_phases.go:207` `o.kernel.Query("campaign_blocked")`; `orchestrator_execution.go:211-213` `"Campaign blocked: %s"` then `o.failCampaign(blockReason)` | — |
| 44 | 1 (row 29) + 4.4 | `campaign_complete/1` at `campaign_phases.mg:145-150`, quoted block `:139-150` | `campaign_phases.mg:144-150` | holds | `:139-142` `has_incomplete_phase`; `:145-147` `campaign_complete(CampaignID) :- current_campaign(CampaignID), !has_incomplete_phase(CampaignID).`; `:149-150` `next_action(/campaign_complete)` — the doc's quoted block matches the file line for line | — |
| 45 | 1 (row 29b) + S2 | `isCampaignComplete()` is a Go loop at `orchestrator_phases.go:184-203` and is the one that ends the run (`orchestrator_execution.go:171`); its sibling `getCampaignBlockReason()` queries the kernel | `orchestrator_phases.go:184-203`, `orchestrator_execution.go:171-205` | holds | `:184-203` the `for _, phase := range o.campaign.Phases` loop on `PhaseCompleted`/`PhaseSkipped`; `:206-222` `getCampaignBlockReason` with `o.kernel.Query("campaign_blocked")` at `:207`; `orchestrator_execution.go:171-172` `if o.isCampaignComplete() { logging.Campaign("=== Campaign completed successfully: %s ===" ...` | The seam is real and within twenty lines, exactly as S2 states |
| 46 | 1 (row 30) | `replan_needed/2` at `campaign_phases.mg:111-128` over Go-computed `failed_campaign_task_count_computed` | `campaign_phases.mg:104-128` | holds | `:111-115` threshold rule reading `campaign_config(CampaignID, _, Threshold, /true, _)` and `failed_campaign_task_count_computed(CampaignID, Count)`; `:118-120` user instruction; `:123-124` explicit trigger; `:127-128` `next_action(/pause_and_replan)` | — |
| 47 | 1 (row 31) | `next_action(/interrogative_mode)` at `clarification.mg:14,23,32,37`; score < 85 at `:6-8`; all four disabled by `yolo_mode()` | `clarification.mg:1-60` | holds | `:6-8` `clarification_needed(Ref) :- focus_resolution(Ref, _, _, Score), Score < 85.`; heads at `:14`, `:23`, `:32`, `:37`, each with `!any_awaiting_clarification(/yes), !yolo_mode()`; `clarification_question/2` from `:42` | The negated literal is the projection `any_awaiting_clarification(/yes)`, not `awaiting_clarification` directly — the doc's shorthand |
| 48 | 1 (row 32) | `llm_timeouts` forces a stop on wall clock; user text at `executor_tools.go:1892` | `executor_tools.go:1864-1895` | holds (thin) | `internal/config/llm_timeouts_config.go` exists; `executor_tools.go:1890-1893` is the quoted text, produced by `describeToolLoopFailure` (`:1871`) on a `context.DeadlineExceeded` from the follow-up call at `:365`. Comment `:1863-1866` verbatim as S9 quotes it | The cited site is the tool-loop deadline path, not a `llm_timeouts` call site; the doc's own "open" list already concedes the ~25 call sites were not enumerated |
| 49 | 1 (row 33) | exploration cutoff / final-answer reserve; `toolExplorationCutoff`, `FinalAnswerReserve` default 5 min | `executor_tools.go:189-235,337-363`, `executor.go:388` | holds | `executor_tools.go:189-190` `toolExplorationCutoff(ctx, executorCfg.FinalAnswerReserve)`; `:225` `if hasFinalizationCutoff && !time.Now().Before(finalizationCutoff)`; `:337-347` and `:351-362` the two other finalization paths; `executor.go:388` `defaultFinalAnswerReserve = 5 * time.Minute`; `:518` `toolExplorationCutoff` defined | — |
| 50 | 1 (row 34) | `core_limits` fields and validation | `internal/config/limits.go:9-15,106-152` | holds | `:10-15` the six resource fields; `:106-135` `ValidateCoreLimits`; `:138-152` `DefaultCoreLimits` | — |
| 51 | 1 (row 35) + 3 | `delegate_task(/tester, "Generate tests for impacted code", /pending) :- impacted(File), !test_coverage(File).` — a delegation, not a block | `delegation.mg:91-93`, `impact.mg:6-13` | holds | `delegation.mg:91-93` verbatim; `impact.mg:6-8` direct impact and `:11-13` transitive closure, verbatim; `impact.mg:16-22` `unsafe_to_refactor`/`block_refactor` exactly as the doc quotes them | "nothing stops a turn while it derives" — confirmed: no Go site queries `delegate_task` as a completion gate |
| 52 | 1a (row 1) | northstar reaches prompts as `injectable_context("*", ...)` rows; mission/capabilities gated on an active planner or coder, risks and constraints unconditional | `prompt_northstar.mg:152-179` | holds | `:153-161` mission rules carry `has_active_planner()` / `has_active_coder()`; `:164-168` capabilities carry `has_active_planner()`; `:171-174` risks and `:177-179` constraints carry no shard gate | — |
| 53 | 1a (row 2) | `must_have_requirement/2` is decoration — nothing gates a turn on it | `prompt_northstar.mg:58-62`, `schemas_misc.mg:133` | holds | `prompt_northstar.mg:58-60` the rule (`Priority = 100`); repo-wide grep of `*.go` for `must_have_requirement` returns only two comments (`internal/northstar/types.go:193`, `facts_links_test.go:163`) — **no consumer** | Same for `strategic_warning`: one Go comment (`types.go:125`), no consumer |
| 54 | 1a (row 6) + S12 | `LevelBlock` is defined and returned but the only other occurrence is an emoji map at `observer_manager.go:580`; the accessors have no production caller | `observer_manager.go:61-77,545-557,580` | holds | `:64` `LevelBlock AssessmentLevel = "block" // Score < 40: Block for review`; `:77` `return LevelBlock`; `:576-581` `levelIcon := map[AssessmentLevel]string{... LevelBlock: "🚫"}`; `:554-558` the 100-entry buffer; `GetRecentAssessments` (`:309`) and `GetLastAssessment` (`:325`) are called **only** from `internal/shards/*_test.go` | Two additions the doc omits: `LevelBlock` is also referenced in `observer_manager_test.go:173-174`, and assessments **do** reach the user — `recordAssessment` notifies callbacks (`:566-569`), and `cmd/nerd/chat/model_update.go:789` registers one that renders every assessment into the chat via `FormatAssessment` (`model_update.go:561-571`). They are displayed, not merely buffered; they still block nothing |
| 55 | 1a (row 7) | the periodic-check cost quote (2026-09-17, 13-43 s per call, 10-50/100 scores) | `observer_manager.go:350-386` | holds | `:353-357` verbatim: "Measured 2026-09-17 in an idle chat session: a check every five minutes from boot, each an LLM call of 13-43 s, each scoring the session 10-50/100 for having nothing to show -- the guardian's own recommendation on every one was to stop evaluating empty ticks." | The throttle (`periodicCheckDue`, `:384-386`) now skips ticks with no event since the last check |
| 56 | 1a (judgment) + Status | `northstar_capability.Timeline` "is used only to sort capabilities into **injection buckets**" (`prompt_northstar.mg:90-105`) | `prompt_northstar.mg:90-105` | **wrong-claim** | The four timeline predicates are not injected anywhere. `injectable_context` (`:152-179`) selects on `critical_capability` (a **priority**, `:7-9`), never on a timeline bucket. `immediate_capability` has exactly one consumer — `has_immediate_capability()` (`:122`) feeding `strategic_warning(/immediate_risk_gap, ...)` (`:128-131`) — and `near_term_capability`, `long_term_capability` and `moonshot_capability` (`:93-105`) have **no consumer at all**, in `.mg` or in Go (grep of `*.go` for all four: zero hits) | The correction strengthens the doc's own argument: the horizon vocabulary is not "sorted into buckets that get injected", it is inert. The same sentence appears twice — §1a's judgment and the Status "open" bullet |
| 57 | 5 / Q-premise 3 | the 2026-08-11 measurement and the rule "a second opinion in a selector may veto, never admit"; `selected_atom`/`candidate_atom`/`mandatory_atom` are inert | `jit_selection.mg:13-20` | holds | `:13-20` verbatim: "selected_atom, candidate_atom and mandatory_atom are admissions, and nothing queries them. Wiring selected_atom into tentative was measured on 2026-08-11 and rejected: a /fix compile went from 67 atoms and 26279 tokens to 254 atoms and 65036 of 65536 tokens, 99.2 percent budget saturation, because each admitted atom recursively pulls its atom_requires dependencies into tentative. The Go selector still queries only selected_result/3 from jit_compiler.mg. The rule this establishes: a second opinion in a selector may veto, never admit." | Premise 3 holds verbatim, including both token figures |
| 58 | 5 | the three-layer mandatory chain: author flag -> `mandatory_selection` (`jit_compiler.mg:196-199`) -> `/skeleton` (`:311-314`) | `internal/core/defaults/jit_compiler.mg` | holds | `:196-199` `mandatory_selection(Atom) :- is_mandatory(Atom), !blocked_by_context(Atom), !mandatory_superseded(Atom).`; permissive block `:76-79`; fail-closed regime block `:143-146` with `regime_dimension` facts at `:133-141` (the nine atoms the doc lists, in that order); `mandatory_superseded` `:190-193`; `:311-314` `/skeleton`, `:316-319` `/flesh`; identity-leakage comment `:91-99` incl. "114 mandatory atoms" (`:93`) and "Identity leakage does not degrade a prompt, it replaces the agent" (`:98`). Author flag: `is_mandatory` is a YAML field (`cmd/tools/prompt_builder/main.go:57`), examples at `internal/system/agent_definition.go:94,112,143,177` and `internal/init/agents.go:221,255,272` | Every cited line matches |
| 59 | 5 | budget exhaustion cannot silently evict a mandatory atom; the compiler refuses | `executor.go:1018,307`, `spawner.go:98,609,651` | holds | `executor.go:1018` "That commit made the compiler REFUSE when the mandatory skeleton"; `:1036` the re-compile fallback; `:307` "compiler had to drop mandatory atoms (defensive_patterns,"; `spawner.go:98` "was hardcoded to 8192 which silently dropped mandatory atoms from", `:609` "pre-fix bottleneck that silently stripped mandatory atoms", `:651` "a smaller fallback budget cut mandatory atoms" | All five quotes verbatim |
| 60 | 5 | the Go cap on Mangle-context mandatory promotion | `selector.go:49-51,421-473,514-526` | holds | `:49-51` `mangleMandatoryTokenCap = 900000`, `mangleMandatoryAtomCap = 600`, `mangleMandatoryBudgetRatio = 0.90`; `:421` `func selectMangleMandatoryIDs(...)`; `:473` `"Mangle mandatory cap applied: selected %d/%d atoms, tokens=%d cap=%d"`; `:514` `func applyMandatoryOverride(...)`; caps applied `:397-409` | — |
| 61 | 1a (where the facts come from) | `.nerd/northstar.mg` is read at boot into the schema builder (`kernel_init.go:421-437`); `res.Logic` carries it; the "2839 bytes, 0 data facts" bug at `:415-420`; authority stated in `bridge.go:19-54` | `kernel_init.go:415-437`, `bridge.go:19-54` | holds | `kernel_init.go:424-426` `LoadHybridMangleFile(northstarPath)` then `schemasBuilder.WriteString(res.Logic)`; `:415-420` verbatim incl. "the log said \"2839 bytes, 0 data facts\" and `nerd query northstar_mission` still found nothing"; `bridge.go:45` "never an authority anything reads to decide"; `:52-54` "Guardian.Initialize calls SyncVisionAuthority" | Every quote verbatim |
| 62 | 1a (risk gate) | the `/northstar` campaign gate blocks on a **wiring precondition**, not on vision distance | `risk_scoring.go:267-282` | holds | `:267` `if o.configuredNorthstarObserver == nil {`; `:270` `"northstar observer not configured for protected campaign surfaces: %s"`; outcome `RiskGateOutcomeBlocked` at `:274` | The doc's reading of what the gate is about is exactly right |
| 63 | 1a (risk contract) | `campaign_risk_gate_outcome/3`, `campaign_risk_blocked_gate/2`, `campaign_risk_preflight_blocked/1` declared at `schemas_campaign.mg:390,417,423`; "Go MEASURES ... The kernel is the authority, not a suggestion" | `schemas_campaign.mg:390,417,423`, `risk_gate_contract.go:18-36` | holds | All three Decls at exactly those lines; `risk_gate_contract.go:18` "Go MEASURES the preflight", `:36` "The kernel is the authority, not a suggestion" | One omission behind the "best-shaped gate in the system" verdict: the same comment continues at `:36-41` describing a **Go mirror fallback** (`classifyRiskGateResults` reproduces the contract in Go when `campaign_risk_classification_ready` does not derive) — the same imperative-twin shape S8 objects to elsewhere |
| 64 | 2 (every file) | campaign propagation of all six knobs (`cmd_campaign.go:1359-1376`); the benchmark pins both ceilings and disables adaptivity (`change_benchmark/main.go:167-169`); four named test files | as cited | holds | `cmd_campaign.go:1359-1376` sets `MaxToolCalls`, `MaxToolIterations`, `AdaptiveToolBudget`, `ToolIterationExtensionSize`, `MaxToolIterationExtensions`, `ToolLoopRepeatThreshold` in that order; `change_benchmark/main.go:167-169` `cfg.MaxToolCalls`, `cfg.MaxToolIterations`, `cfg.AdaptiveToolBudget = false`; all four test files exist | — |
| 65 | 2 (log lines) | the nine message sites, and "the phrase the architect objects to is literally in the model's context window at least four times per exhausted turn" | `executor_tools.go:396,400,429,440,812,815,599,603`, `tool_budget_controller.go:310,372` | holds | every cited line matches the quoted text | The "four times" arithmetic is loose: `readOnlyBudgetExhaustedNudge` (`:599`) and `writeBudgetExhaustedNudge` (`:603`) are mutually exclusive on one turn, and `:812` only fires if the **call** ceiling is hit. The claim survives only because `appendToolBudgetNudge` re-appends "Orchestrator budget:" at every round boundary (`executor_tools.go:253-260`) |
| 66 | 2 (history) | five commits, newest first, origin `66585382` dated 2026-08-10 | `git log --follow internal/session/tool_budget_controller.go` | holds | `git show -s`: `ae7937bb\|2026-09-16\|feat(session): uplift session unit — guards, lifecycle bounds, dead-code removal`; `7213b414\|2026-09-11\|feat(session): close exploration under a policy-derived commit regime`; `2c409a58\|2026-09-11\|fix(session): let the working policy see the shape of the turn, not just a cycle flag`; `0e06c9c8\|2026-08-10\|fix(session): require material progress for tool extensions`; `66585382\|2026-08-10\|fix(session): adapt tool budgets to proven progress` | Subjects and order exact; the two dates the doc leaves blank are 2026-09-16 and 2026-09-11 |
| 67 | 3 | `cmd/tools/audit_test_bodies` flags log-only/empty test bodies and "is not wired into any decision" | `cmd/tools/audit_test_bodies/main.go:1-3,17-36,64` | holds | `:1-3` the doc comment verbatim ("Passing this check is not a claim of behavioral coverage..."); `:17-36` `func inert(body *ast.BlockStmt) bool` rejecting anything but `t.Log`/`t.Logf`; `:64` `"%s: %s has no executable behavior beyond logging"` | The enumeration is slightly off: a repo-wide grep for `audit_test_bodies` returns `main.go`, `UPLIFT_LEDGER.md:327-329`, two audit docs, **and this journey file** — no `internal/session` file contains the string. The load-bearing claim (no decision consumes it) holds |
| 68 | 3 | `untestedWithoutCoverageOnDisk` was deliberately softened; the 2026-08-08 observation | `test_verify.go:169-181` | holds | `:170-175` verbatim incl. "It flagged internal/session/test_verify.go on two consecutive live turns (2026-08-08 11:08 and 11:13)" and "A gate that cries wolf about tested code is one that gets ignored, and then switched off."; `:177-180` the two-clause covered test; func at `:181` | — |
| 69 | 4.0 | the only non-logging readers of `turn_done` are the `turn_cost` ledger and `factory_learning.go:270` — "accounting after the fact, not control" | `executor_memory.go:150-165`, `factory_learning.go:270` | holds (with an addition) | `executor_memory.go:152-165` `resolveTurnOutcome` maps `turn_done`->`/done`, `hollow_success`->`/hollow`, both gated on `result.Acceptance.Status == "verified"`; `captureTurnOutcome` at `:128-147` does the same earlier; `factory_learning.go:270` the quoted Explanation | A fourth surface the doc misses: `internal/session/executor_learning.go:60-74` `TurnRecord.Verified()` returns `Outcome == /done` and is the gate on what the **prompt learner** trains on ("Only /done means the kernel saw the evidence"). Still not control over the turn, but it is control over learning |
| 70 | 6.0 | the north-star vocabulary already exists with the cited Decl lines | `schemas_misc.mg:47,58,61,77,80,83,90,133` | holds | every line exact: `:47` `northstar_capability(CapID, Description, Timeline, Priority)`, `:58` `northstar_risk`, `:61` `northstar_mitigation`, `:77` `northstar_requirement`, `:80` `northstar_supports`, `:83` `northstar_addresses`, `:90` `northstar_constraint`, `:133` `must_have_requirement` | — |
| 71 | 6.1 | `VerifyOutcome` already has the five values `/passed \| /failed \| /skipped \| /indeterminate \| /canceled` | `verify_outcome.go:20-36` | holds | `:22-36` the five constants, in that order | — |
| 72 | 6.1 | asserting `build_result` would give `build_state(/failing)` — "already consumed at `coder_safety.mg:114`" — **a real producer** | `coder_safety.mg:114` | holds | `coder_safety.mg:114` consumes `!build_state(/failing)`; no production Go asserts `build_state`: the only assertions are in `internal/session/turn_done_test.go:39`. Stronger than the doc says — `executor.go:216-219` declares `perTurnBuildStateFacts` "asserted this turn via recordBuildState", and **`recordBuildState` does not exist anywhere in the tree** (the only hit is that comment), so the field is never written | Consequence worth handing the architect: the `!build_state(/failing)` conjunct in `turn_executed` never excludes anything today |
| 73 | 6.1 | `internal/context/working_set.mg:3` "already has `working_observation/5` + `working_digest/2`" | `working_set.mg:3` | wrong-location (minor) | `working_observation/5` is declared at `:3`; `working_digest/2` is at `:9` (`Decl working_digest(ID, Digest) bound [/string, /string].`), with `working_span/3` at `:11` | Both predicates do exist, as claimed |
| 74 | 6.3 / 6.8 | "One honest unknown": **five** predicates in 6.3 have no producer today | `§6.3` | overstated | A repo-wide grep (all file types) for `requirement_witnessed`, `turn_claims_requirement`, `family_requires_atom`, `atom_in_window`, `turn_touches_constrained_surface`, `testable_path`, `turn_ran_tests`, `constraint_atom`, `turn_target_family`, `turn_evidence_recorded`, `consecutive_failed_rounds`, `unverified_test_claim_for` returns **exactly one file — this journey document**. None of the twelve exists in Go or in any `.mg`. Of the §6.3 rule bodies only `is_test_file` (`schemas_shards.mg:219`), `test_coverage`, `northstar_constraint`, `northstar_requirement`, `unmitigated_risk`, `northstar_risk` and `tool_trace_cycle`-style facts have any basis today | The honest count is at least **twelve** undeclared predicates, not five. `stall_rounds(M)` is a rename of the existing `working_stall_rounds(24)` (`working_set.mg:36,58`), so that one is free; `is_test_file` is declared but its sibling `is_test_function` is documented as having NO PRODUCER (`policy/test_impact.mg:12-24`), which is a warning for any rule that leans on Go-asserted test identification |
| 75 | 6.2 / 6.8 | the wildcard-negation gotcha has two existing precedents in the live corpus | `prompt_northstar.mg:196-201`, `working_set.mg:88-90` | holds | `prompt_northstar.mg:196-201` "In this Mangle fork, negating a multi-argument literal with wildcards does NOT exclude correctly ... Keep that structure and keep this comment"; `working_set.mg:88-90` "Negation needs a bound literal, and a wildcard in a negated atom does not exclude in this Mangle; project the stop reasons to arity 0 first." | Matches the memory note `reference_mangle_wildcard_negation_gotcha.md` |
| 76 | 6.7 / S8 | `consumeHollowSuccessVerdict` carries an imperative twin of four policy rules at `executor.go:2505-2555`, guarded at `:2507-2508`, whose reach differs from `coder_safety.mg:92-95` | `executor.go:2505-2555` | holds | `:2505-2508` "Fallback for kernels without the new turn_evidence rules (unit-test mocks, older snapshots) ... only runs when no hollow_success fact was derived"; `:2510` `if e.intentRequiresToolCall(verb) \|\| e.writeOrientedIntent(verb)` vs `coder_safety.mg:94` `intent_requires_tool_call(Verb)` alone; the four mirrored checks at `:2510-2517`, `:2518-2523`, `:2527-2531`, `:2532-2555` | One correction to S8's framing: the fallback is also reachable **after** a derived verdict — the `"new source was created without a test file"` case can fall through the switch (`:2496-2500`) into it |
| 77 | 5 | "`selected_result/3` is the only predicate the Go selector queries" | `jit_selection.mg:18` | overstated | `internal/prompt/selector.go` queries three: `:977` `kernel.Query("blocked_by_context(Atom)")`, `:985` `kernel.Query("mandatory_selection(Atom)")`, `:997` and `:1232` `kernel.Query("selected_result(Atom, Priority, Source)")` | The two extra queries are labelled "Debug:" (`:976`, `:984`) and only feed `logging...Debug` lines, so the substance holds — but the document repeats a `.mg` comment as verified fact without checking it |
| 79 | 2 (history) | the origin commit's stated reason, quoted as a block | `66585382` commit body | holds | `git show -s --format=%B 66585382`: "Two consecutive all-Spark dogfood fixes consumed the configured 24 tool rounds on path guesses and review before the requested wiring landed. The executor previously exposed no remaining-budget context and could only stop at the cliff." — byte-for-byte as the doc quotes it | The body's third paragraph also confirms the doc's "two distinct problems at once" reading: telemetry through paired tool results **and** extensions gated on fingerprinted novel work |
| 78 | document | §6.8 and the task brief refer to "the five closing questions for the architect (§Questions)" | `§6.8` last paragraph | unverifiable | The file has no `Questions` section. Headings end at `### S12 — LevelBlock blocks nothing` (line 1147 of the pre-verification file); `§6.8` says "That question is the first one for the architect (§Questions)" and the Status block does not list §Questions under `done:` or `open:` | Four of the five premises the brief names are verified above (rows 33, 34, 57, 38); the fifth cannot be identified because the section was never written |

### Refuted claims

Nine of seventy-nine checked claims did not survive. Three are wrong about what the code does, two
cite the wrong lines, three are true only in a weaker form than stated, and one cannot be checked
because the section it belongs to was never written.

1. **§1, row 13 — `unverified_test_claim/1` does not force anything "via `hollow_success` line 101".**
   `coder_safety.mg:101-103` derives `hollow_success` on its own, from
   `turn_evidence(Verb, _, _, _, /true, /false), !has_turn_test(Verb)`; it never mentions
   `unverified_test_claim`. The predicate's single consumer in the tree is
   `internal/session/executor.go:2527`, inside the imperative fallback that S8 documents — which
   runs only when the kernel derived no `hollow_success` (or when the new-source case fell through
   the switch at `:2496-2500`). **Correction:** row 13's "Where computed" is right, its forcing path
   is not; the corpus itself calls the rule "legacy" (`executor.go:2329`).
2. **§3 — the two coverage signals are not "an input to [no] decision", and the strongest coverage
   signal is not "a `Warn` log that nothing consumes".** `result.UncoveredBlocks` grounds the
   critic's prompt at `build_verify.go:636` (`grounding := summarizeUncovered(result.UncoveredBlocks)`)
   and appears in the turn-signal line at `:698`; **both** signals leave the turn as
   `observation.Return.Notes` at `internal/session/observed_return.go:93-100`, which is what a parent
   agent reads about a sub-agent's turn. **Correction:** neither becomes a kernel fact and neither
   gates the turn — that is the defensible claim, and it is the one §6 needs. The same correction
   applies to S6's row "`logging...Warn(...)`. No fact, no decision".
3. **§1a judgment (and the matching Status "open" bullet) — `northstar_capability.Timeline` is not
   "used only to sort capabilities into injection buckets".** The four timeline predicates
   (`immediate_capability`, `near_term_capability`, `long_term_capability`, `moonshot_capability`,
   `prompt_northstar.mg:89-105`) are injected nowhere: every `injectable_context` rule
   (`:152-179`) selects on `critical_capability`, which is a **priority** (`:7-9`), not a horizon.
   `immediate_capability` feeds only `has_immediate_capability()` -> `strategic_warning(/immediate_risk_gap, ...)`
   (`:122,128-131`), which has no Go consumer; the other three have **no consumer at all**.
   **Correction:** the horizon vocabulary is inert, not a sorting key — which strengthens the
   section's own conclusion.
4. **§2, "Every file" — `ExecutorConfig` budget fields are not at `executor.go:257-265`.** They span
   `:250-278`: `MaxToolCalls` `:252`, `MaxToolIterations` `:257`, `ProgressDrivenTools` `:261`,
   `AdaptiveToolBudget` `:267`, `ToolIterationExtensionSize` `:271`, `MaxToolIterationExtensions`
   `:274`, `ToolLoopRepeatThreshold` `:278`. **Correction:** cite `executor.go:250-278`.
5. **§6.1 — `working_digest/2` is not at `working_set.mg:3`.** `working_observation/5` is at `:3`;
   `working_digest/2` is at `:9`. **Correction:** cite `working_set.mg:3,9`. Both predicates exist,
   as the row claims.
6. **§2 — `toolBudgetController.workingProgress` is not the producer of `working_progress/5`; it is
   the sole producer of its *values*.** The fact is asserted at
   `internal/context/working_set.go:181`, inside `WorkingSet.Continue`. **Correction:** the claim
   survives in substance (the only production caller is `executor_tools.go:275`; every other
   `WorkingProgress` literal is a test), and is in fact *understated*: `working_control/2` is
   asserted from the same struct at `working_set.go:180`, so deleting the controller silences
   `/repeated_cycle` and `/tool_failures` too — **five** of the six policy stops, not three. But the
   assertion site an architect would have to move is `working_set.go:180-186`.
7. **§6.8 — "One honest unknown" undercounts.** A repo-wide grep (all file types) for
   `requirement_witnessed`, `turn_claims_requirement`, `family_requires_atom`, `atom_in_window`,
   `turn_touches_constrained_surface`, `testable_path`, `turn_ran_tests`, `constraint_atom`,
   `turn_target_family`, `turn_evidence_recorded`, `consecutive_failed_rounds` and
   `unverified_test_claim_for` returns **exactly one file: this document**. **Correction:** §6.3
   leans on at least twelve predicates that do not exist today, not five. (`stall_rounds` is a
   rename of the live `working_stall_rounds`, `working_set.mg:36,58`, so it is free.)
8. **§5 — `selected_result/3` is not the only predicate the Go selector queries.**
   `internal/prompt/selector.go:977` queries `blocked_by_context(Atom)` and `:985`
   `mandatory_selection(Atom)`, alongside `selected_result(Atom, Priority, Source)` at `:997` and
   `:1232`. **Correction:** both extra queries are explicitly labelled "Debug:" and feed only
   `logging...Debug`, so the point stands — but the document repeated a `.mg` comment
   (`jit_selection.mg:18`) as a verified fact.
9. **The five closing questions do not exist.** §6.8 ends "That question is the first one for the
   architect (§Questions)", and there is no `Questions` section: the file ended at
   `### S12 — LevelBlock blocks nothing`. Four of the five premises the review brief names are
   verified below; the fifth cannot be identified.

### Premises of the five questions

| # | Premise (as named in the brief) | Verdict | Corrected statement |
|---|---|---|---|
| 1 | `turn_done/1`'s only consumer writes a Debug line at `executor.go:2563-2572` | **holds** | Exact. `consumeTurnDoneSignal` spans `:2563-2572` and does nothing but log two Debug branches (`:2568`, `:2570`); the call site's own comment says so (`executor_tools.go:1972-1975`). Two further readers exist and are accounting, not control: `resolveTurnOutcome`/`captureTurnOutcome` (`executor_memory.go:128-165`) and `factory_learning.go:270` — plus one the document misses, `TurnRecord.Verified()` (`executor_learning.go:60-74`), which gates what the **prompt learner** trains on |
| 2 | `turn_acceptance` is asserted only on host-verifier acceptance | **holds** | Exact. `executor.go:2334` `if result.Acceptance != nil && result.Acceptance.Status == "verified"`. Worth adding for the decision: the *other* conjunct of `turn_executed`, `!build_state(/failing)` (`coder_safety.mg:114`), also never excludes — no production code asserts `build_state`, and the field meant to track it (`executor.go:216-219` `perTurnBuildStateFacts`, "asserted this turn via recordBuildState") is dead, because **`recordBuildState` does not exist in the tree** |
| 3 | the 2026-08-11 "veto, never admit" measurement | **holds** | Exact and byte-for-byte: `jit_selection.mg:13-20` — "a /fix compile went from 67 atoms and 26279 tokens to 254 atoms and 65036 of 65536 tokens, 99.2 percent budget saturation, because each admitted atom recursively pulls its atom_requires dependencies into tentative ... a second opinion in a selector may veto, never admit." The adjacent sentence in the same comment ("The Go selector still queries only selected_result/3") is the one that is wrong — see refutation 8 |
| 4 | `executor.go:2394-2398` scopes `turn_created_source` | **holds, with a citation correction** | `:2394-2398` is the comment; the mechanism runs `:2399-2424` (mirror `perTurnCreatedSourceFacts` into `turn_created_source`, skipping any already scoped). The document's own citation, `executor.go:2394-2424`, is the correct one. The consequence the question rests on — a leaked or scanner-derived `created_source` cannot fail a later turn, and therefore no coverage obligation survives the turn — holds |
| 5 | — | **unverifiable** | The `§Questions` section referenced by §6.8 was never written, so the fifth premise cannot be identified. The load-bearing premise the brief flagged separately — §2's "`working_progress/5` has exactly one producer" — is checked as row 10 / refutation 6: true in substance, wrong about the assertion site, and understated about the blast radius |

# codeNERD User-Journey Map

A horizontal map of every entry point into codeNERD: for each journey, what happens
stage by stage, who decides at each stage, what knowledge enters the model's window
and by what mechanism, what forces continuation, what tools run and why, and how the
verdict is produced.

Every claim is cited as `path:line` or as a Mangle predicate in a named `.mg` file.
Claims that could not be verified from code say **unverified**.
`Docs/architecture/` was NOT used as evidence (July-2026 agent-generated orientation).

## Status

last updated: 2026-09-18 04:05

done:
- vision section (measuring stick)
- entry-point inventory
- Journey 1 (chat free-text turn) — chat layer, shard layer, verdict, persistence
- Journey 2 (slash commands) — categories, dispatch, task rewriting, kernel use
- Journey 3 (`nerd fix` CLI) + proof that `turn_done` is unreachable without --acceptance
- Journey 4 (campaigns + recurse)
- Journey 5 (autonomous: system shards, observers, dream, ticker inventory)
- Journey 6 (scan, predicate ownership, JIT consumption, budget bypasses)
- Seams (S-1 .. S-17, incl. S-2b/S-2c/S-2d)
- Where the kernel is bypassed (all five vision axes scored)

open:
- `nerd run <instruction>` (`cmd/nerd/cmd_instruction.go`) inventoried in the entry table
  but not given a stage table; it is the third shape of a one-shot and may differ from
  `runDirectAction`.
- `nerd spawn` / `define-agent` / `nerd swebench` / `nerd northstar` not traced beyond the
  entry inventory.
- Shadow mode (`cmd/nerd/chat/shadow.go`) named in Journey 2 but not traced.
- The multi-step decomposition path (`cmd/nerd/chat/multistep_decomposer.go`,
  `internal/session/work_steps.go`) — Journey 1 row 9 identifies the decision point but
  the per-step obligation and verdict were not traced.
- `internal/session/executor_tools.go` beyond the tool loop and hollow gate (2723 lines;
  the per-tool permission path and `verifyCompletedToolTurn` are unexamined).
- The `/help` registry vs dispatcher divergence (S-5) was sampled, not enumerated
  exhaustively.
- `perception.GetShardTypeForVerb` and the verb taxonomy corpus were treated as a black
  box; the claim that it is "siloed from the executive kernel" is quoted from
  `policy/delegation.mg:330-334`, not independently verified.

## The measuring stick (vision, root CLAUDE.md)

Source: `CLAUDE.md:25-56` ("## The Vision (Steve, 2026-09-18)"). The axes every table
row below is scored against:

1. **The harness decides, not the model's discretion.** Mangle + JIT prompt compilation
   replace subagents and skills. (`CLAUDE.md:27-29`)
2. **The harness forces.** The agent keeps working not until the task is done but until
   the north-star behaviour holds; test coverage is not optional; domain knowledge is
   pushed in when the harness decides, not fetched at the LLM's discretion.
   (`CLAUDE.md:30-35`)
3. **It never has to grep around.** The kernel blends SQLite domain knowledge, vector
   search, CodeDOM and history and injects it at the right time, for the right reason,
   to the right agent. (`CLAUDE.md:36-41`)
4. **Where a fact sits in the window is a decision** — compression, pruning, ordering.
   (`CLAUDE.md:42-44`)
5. **Tools exist for exactly three things**: condense the search space, reduce turns,
   offload cognition to deterministic code. Anything else is cruft. (`CLAUDE.md:45-48`)
6. **"Clean fixpoint," not "clean loop."** The executive decisions — *what a turn is,
   whether it is done, what is delegated and with what task, what enters the window,
   what the verdict is* — are the fixpoint of the kernel over the facts. **The drift to
   hunt is decisions computed in Go instead of derived.** (`CLAUDE.md:49-54`)
7. Complexity is not a defect; what is scored is whether a decision is *derived* and
   whether an obligation is *forced*. (`CLAUDE.md:55-56`)

Those five italicised executive decisions in (6) are the checklist used by
[Where the kernel is bypassed](#where-the-kernel-is-bypassed).

## Entry-point inventory

Registered on the cobra root in `cmd/nerd/main.go:243-320`. Bare `nerd` with no
subcommand falls through `rootCmd.RunE` (`cmd/nerd/main.go:137-167`) to
`chat.RunInteractiveChat` — the chat TUI is the default entry point. **[REFUTED — see Verification]**

| Entry | Registration | Journey |
|---|---|---|
| `nerd` (bare) / `nerd chat` | `cmd/nerd/main.go:137-167`, `cmd/nerd/cmd_chat.go` | 1, 2 |
| `nerd run <instruction>` | `runCmd`, `cmd/nerd/cmd_instruction.go` | 3 (one-shot OODA) |
| `nerd fix` / `review` / `test` / `explain` / `create` / `refactor` / `security` / `analyze` / `push` / `commit` / `perception` | `cmd/nerd/main.go:265-277`, impl `cmd/nerd/cmd_direct_actions.go` | 3 |
| `nerd campaign start\|status\|pause\|resume\|list\|recurse` | `cmd/nerd/main.go:226-233`, `cmd/nerd/cmd_campaign*.go` | 4 |
| `nerd dream` / `shadow` / `whatif` / `logic` / `agents` / `tool` / `jit` / `dom` / `embedding` | `cmd/nerd/main.go:281-290`, `cmd/nerd/cmd_advanced.go` | 5 |
| `nerd northstar` | `cmd/nerd/main.go:293-295`, `cmd/nerd/cmd_northstar.go` | 5 |
| `nerd init` / `nerd scan` | `cmd/nerd/cmd_init_scan.go` | 6 |
| `nerd world` / `retrieve` / `memory` / `knowledge` / `context-stats` / `snapshot` / `audit` / `meter` / `usage` | `cmd/nerd/main.go:298-311` | 6 + introspection |
| `nerd spawn` / `define-agent` | `cmd/nerd/cmd_spawn.go` | shard-direct |
| `nerd query` / `status` / `why` / `logs` / `glassbox` / `transparency` / `reflection` | `cmd/nerd/main.go:243-260,318-322` | introspection |
| `nerd check-mangle` / `mangle-lsp` | `cmd/nerd/cmd_mangle_check.go`, `cmd_mangle_lsp.go` | tooling |
| `nerd auth claude\|codex\|grok\|status` | `cmd/nerd/cmd_auth.go` | setup |
| `nerd browser launch\|session\|snapshot` | `cmd/nerd/cmd_browser.go` | tooling |
| `nerd swebench` | `cmd/nerd/cmd_swebench.go` | benchmark |
| `nerd regression` / `features` / `mcp` / `autopoiesis` / `sessions` | `cmd/nerd/main.go:298-315` | ops |
| timers inside a chat session (observers, guardian, dream) | `internal/shards/observer_manager.go`, chat model tickers | 5 |

Process-wide, before any subcommand runs: `config.GlobalConfig()` eagerly loads
`.nerd/config.json` so `internal/features` is populated (`cmd/nerd/main.go:355-361`);
file logging is initialised (`cmd/nerd/main.go:348-351`); the flight recorder starts
unless this is a campaign invocation (`cmd/nerd/main.go:377-399`, rationale F-TRACE-1 at
`cmd/nerd/main.go:327-337`). **[REFUTED — see Verification]**

## Journey 1 — `nerd chat` free-text turn

The backbone. One Bubbletea `tea.Cmd` closure holds the entire turn:
`(Model).processInput`, `cmd/nerd/chat/process.go:48-1090`. It is a **waterfall of Go
`if` statements**, not a kernel fixpoint: the kernel is consulted at four points
(`route_decision`, `next_action`, `context_to_inject`, `final_system_prompt`) and every
other branch is Go control flow that can pre-empt the kernel's answer.

### 1A. Stage table — chat layer

| # | Stage | Component (`path:func`) | Decision made here | Who decides | Knowledge entering the window (mechanism) | What forces continuation | Tools | Verdict path |
|---|---|---|---|---|---|---|---|---|
| 0 | Turn context + timeout | `process.go:71-80` | turn deadline = `LLMTimeouts().OODALoopTimeout` | **Go** (`config.GetLLMTimeouts`) | — | nothing; the context deadline is the only bound on a turn | — | — |
| 0.5 | Greeting fast path | `process.go:123-140` | "this turn is a greeting, skip the whole pipeline" | **Go** — literal set `{hi, hello, hey, sup, greetings, yo}` | hardcoded reply string `process.go:138` | n/a — turn ends | none | no verdict; canned prose |
| 1 | Perception | `process.go:142-200`; `perception_firewall` system shard via `GetRunningShardByConfigName`, else `m.transducer.ParseIntentWithContext` | classify input into `Intent{Category, Verb, Target, Constraint, Confidence, IsQuestion, Ambiguity, Response}` | **model** (fast/classification LLM) | conversation history — ALL of `m.history` when compression is off, otherwise `compressor.GetRecentTurnWindow()` (`process.go:161-167`). Harness-delivered. | — | — | — |
| 1b | Shard-type resolution | `delegation_routing.go:205-227 resolveShardTypeForIntent` | verb → shard name | **Go**, with two fallbacks: LLM `Ambiguity` entries of the form `shard=<x>` (`:209-216`), and a **substring heuristic** on `intent.Target` for `codebase/project/architecture/repository/entire/whole` → `researcher` (`:218-225`) | — | — | — | — |
| 1c | Conversational fast path | `process.go:236-258` | end the turn on perception's own `surface_response`, skip ORIENT/DECIDE/ACT | **Go** (`willConverse` = no shard + non-empty response + `isConversationalIntent`) | none beyond perception | n/a | none | no verdict |
| 2 | Kernel seed | `process.go:260-313` | assert `user_intent(/current_intent, Cat, Verb, Target, Constraint)` + `processed_intent`; retract stale first | **Go** writes, kernel stores | `seedIssueFacts` (sparse retrieval into kernel, `process.go:308`), `seedCampaignFacts` (`:312`) | — | — | — |
| 3 | Reflection | `process.go:316-323 performReflection` | whether to recall System-2 memory | **Go** + config `reflection` | memory recall facts (harness-delivered) | — | — | — |
| 4 | Memory operations | `process.go:326 processMemoryOperations` | apply the model's requested memory writes | **model** emits ops; Go applies | — | — | — | — |
| 5 | Dream interception | `process.go:330-334` | `intent.Verb == "/dream"` → `handleDreamState` | **Go** `if` on the verb | — | — | — | see Journey 5 |
| 6 | Assault interception | `process.go:338-342 assaultArgsFromNaturalLanguage` | start an assault campaign from free text | **Go** natural-language matcher | — | — | — | see Journey 4 |
| 7 | **Routing arbitration (the one real DECIDE point)** | `process.go:353 m.decideRoute` → `delegation_routing.go:72-169` | one of `respond_directly / multi_step / delegate / clarify` | **kernel** — `route_decision/2` in `policy/routing_arbitration.mg:90-112`, over `wants_direct_answer/0` (`:73-84`), `should_delegate/1` (`policy/delegation.mg:335-338`), `is_multi_step/0` (`policy/delegation.mg:355-363`). **But lane precedence is re-implemented in Go** (`delegation_routing.go:152-164`), and a nil kernel / query error / empty derivation silently returns `RouteLegacy` (`:74-76, :124-135`) which hands the decision back to Go booleans. | EDB asserted per turn: `delegation_candidate`, `multi_step_signal`, `intent_signal` (`delegation_routing.go:93-121`) | — | — | — |
| 8 | Auto-clarify | `process.go:369-389 shouldAutoClarify` | run the requirements-interrogator shard | **Go substring heuristic** — see `process_dream_delegation.go` (below) | clarifier shard output | — | clarifier shard | returns prose + `ClarifyUpdate`, no verdict |
| 8b | Kernel clarify | `process.go:392-399 shouldClarifyFromKernel` | ask a question with options | **kernel** (`policy/clarification.mg`) | — | — | — | — |
| 8c | Fallback clarify | `process.go:402-416 shouldClarifyIntent` | ask anyway | **Go heuristic** | — | — | clarifier shard | — |
| 9 | Multi-step | `process.go:422-436` | decompose into steps | **kernel** if `RouteMultiStep`, **Go** `detectMultiStepTask` when `RouteLegacy` (`:426-427`) | `decomposeTask(input, intent, workspace)` (`:431`) | the step list is the obligation | — | per-step |
| 10 | Delegate | `process.go:462-648` | hand to one shard | **kernel** `RouteDelegate`, else **Go** `m.shouldDelegate` (`:463-465`) | `needsWorkspaceScanForDelegation` → `loadWorkspaceFacts` (`:467-475`); `buildSessionContext` (`:482`) is the blackboard injection | — | shard | see 10c |
| 10a | **Task rewrite** | `delegation.go:65-124 formatShardTaskWithContext` → `delegation.go:311-398 formatShardTask` | what the shard is actually asked to do | **Go** string template over `(verb, target, constraint)` — **the user's original sentence is discarded**. `/fix` becomes literally `fmt.Sprintf("fix issue in %s", target)` (`delegation.go:362`); `/review` becomes `review file:%s` or `review all` (`:334-341`) | prior shard's return is projected into the task string via `priorShardContext` (`delegation.go:145-186`), bounded by structure and redeemable with `subagent_expand` | — | `discoverFiles` (`delegation.go:327`) condenses search space | — |
| 10b | Verification retry | `process.go:489-527`, gated by `shouldVerifyDelegation` (`delegation_routing.go:181-183`: `intent.Category == "/mutation"` only) | re-run the shard up to `shard_profiles.<t>.max_retries` times | **Go** | verification LLM call per attempt | **this is the only forced-retry obligation on the chat path**, and it only exists for mutations | — | `VerifyWithRetry` returns `(result, verification, err)`; the sentinel is a **string compare** `verifyErr.Error() == "max retries exceeded - escalating to user"` (`process.go:511`) |
| 10c | Result → facts | `process.go:574-580` `shardMgr.ResultToFacts`; and `process_continuation.go:167-222 injectShardResultFacts` | the turn's status | **Go substring guesses** — `strings.Contains(result,"TODO"\|"FIXME")` → `/incomplete`; `shardType=="coder" && !contains(lower(result),"test")` → `/code_generated`; `reviewer && contains(lower(result),"issue")` → assert `pending_review` (`process_continuation.go:179-221`). The fact stores only `truncateSummary(result, 200)` (`:194`). | — | `should_auto_continue` + `has_pending_subtask` (kernel) drive the continuation protocol (`process_continuation.go:35-36`) | — | **status is a guess from prose** |
| 10d | Reviewer/tester interpretation | `process.go:593-628` | whether the review is trustworthy | **Go** `CheckReviewNeedsValidation` | — | `checkContinuation` (`:615`) | — | prose warning prepended |
| 11 | Stats | `process.go:651-661` | `/stats` handled deterministically | **Go** `if` on verb | computed stats | — | deterministic stats (offloads cognition — legitimate) | — |
| 12 | Direct conversational response | `process.go:667-685` | end in perception's prose | **Go** | — | — | — | — |
| 13 | Autopoiesis | `process.go:691-723 QuickAnalyze` | suggest a campaign / persistent agent, and **auto-run the clarifier** | **Go** + autopoiesis analyzer | clarifier output | — | — | warnings only |
| 14 | Context loading | `process.go:729-733 loadWorkspaceFacts` | which workspace facts enter the kernel | **Go** (intent-gated, incremental scan) | world-model facts (harness) | — | scanner | — |
| 15 | State refresh + hydration | `process.go:738-754` | — | **Go** | `UpdateSystemFacts`, `shardMgr.ToFacts`, `virtualStore.HydrateLearnings` (SQLite `knowledge.db`), `HydrateSessionContext` | — | — | — |
| 16 | `next_action` | `process.go:760-762` | what to do next | **kernel** (`next_action/1`, `policy/delegation.mg:367-379` + `action_mapping`) | — | — | — | `action_denied` surfaced at `:786` |
| 17 | Info-gathering actions | `process.go:796 executeInfoGatheringActions` | run the derived read-only actions | **kernel** derives, **Go** executes | results loaded back as facts (`:800`) | — | VirtualStore actions | — |
| 18 | System delegations | `process.go:809-813 handleSystemDelegations` | surface kernel `delegate_task` results | **kernel** (`delegate_task/3`, `policy/delegation.mg:15-98,382-399`) | — | — | — | — |
| 19 | Context selection | `process.go:815-819` | **what enters the window** | **kernel** — `context_to_inject` (spreading activation, `policy/activation.mg`) | the selected facts | — | — | — |
| 20 | System prompt | `process.go:822-832` | the shard's prompt | **kernel** `final_system_prompt` (JIT) **[REFUTED — see S14 study: no producer; 0 bytes]**, then **Go appends a hardcoded persona string** `stevenMoorePersona` (`process.go:832`) — an unconditional, non-JIT prompt injection | JIT atoms | — | — | — |
| 21 | Articulation | `process.go:880-882 articulateWithConversation` | the user-visible answer | **model** | `ConversationContext{RecentTurns, LastShardResult, ShardHistory, CompressedCtx}` (`:848-854`) | — | — | — |
| 22 | Model facts → kernel | `process.go:1021-1049` | which of the model's asserted facts are allowed in | **kernel-adjacent Go** — `core.FilterMangleUpdates(..., core.ModelObservationPolicy())`; blocked updates become warnings | — | — | — | — |
| 23 | Persistence | `process.go:908-927` feedback store; `:946-1014` semantic compression (`compressor.ProcessTurn`); `:1051-1063` `self_correction_hypothesis` | what survives the turn | **Go** | JIT manifest hash recorded with the feedback (`:910-914`) | — | — | — |

### 1B. The three outcomes, precisely

- **Direct answer.** Four different code paths can produce one, and only one of them
  is the kernel's: the greeting literal set (`process.go:124`), the perception
  fast path (`process.go:236`), `route_decision(/respond_directly, /none)`
  (`routing_arbitration.mg:90-91`), and the post-autopoiesis direct response
  (`process.go:667`). Three of the four are Go pre-empting the kernel's DECIDE point,
  and two of them run **before** `decideRoute` is ever called.
- **Clarify.** Three paths again: `shouldAutoClarify` (Go), `shouldClarifyFromKernel`
  (kernel), `shouldClarifyIntent` (Go), plus a fourth inside autopoiesis
  (`process.go:703`). `route_decision(/clarify, /none)` derives (`routing_arbitration.mg:107-112`)
  but **nothing in `processInput` reads `RouteClarify`** — grep of `process.go` shows
  `RouteMultiStep`, `RouteDelegate`, `RouteLegacy` and `RouteRespondDirectly` consumed
  (`:354, :423-428, :462-465`) and `RouteClarify` consumed nowhere. The kernel's
  clarify lane is derived and then dropped; Go's heuristics decide clarification.
- **Delegate.** `RouteDelegate` or the Go fallback; then the task string is rebuilt
  from the intent tuple, losing the user's words.

### 1C. What forces continuation on this journey

Almost nothing. The obligations that exist:

- `VerifyWithRetry` — mutations only (`process.go:489`, gate at `delegation_routing.go:181-183`).
- The continuation protocol — `should_auto_continue` + `has_pending_subtask`
  (`process_continuation.go:35-36, 123-124`), but its input facts are the
  substring-guessed `shard_result` status from `injectShardResultFacts`.
- The multi-step list, when one was produced.

At the *chat* layer, no test-coverage obligation exists. Inside the shard, one does —
see 1D.

### 1D. Inside the shard — `internal/session`

`spawnTaskWithContext` (`cmd/nerd/chat/delegation.go:51-57`) →
`JITExecutor.ExecuteWithContext` (`internal/session/task_executor.go:258-261`) →
`executeObserved` (`:273-361`) → either a subagent (`needsSubagent`, `:310-312`) or an
isolated clone of the session executor (`CloneForTask`, `:318`) →
`exec.ProcessWithIntent` (`:343`).

| # | Stage | Component | Decision | Who decides | Knowledge in (mechanism) | Forces continuation | Tools | Verdict |
|---|---|---|---|---|---|---|---|---|
| S1 | Intent preset | `task_executor.go:339-342 presetIntentForTask` | do not re-perceive the synthetic task string; reuse the routing layer's classification | **Go** | `req.Constraint` carried structurally so retrieval and prompt assembly can read it | — | — | — |
| S2 | JIT prompt compilation | `executor.go:1049-1064` (`compileConfig`), context built at `executor.go:1155-1169 buildCompilationContext` | which atoms enter the window, under a token budget | **kernel** (JIT selection, `policy/jit_selection.mg`, `jit_logic.mg`, `jit_config.mg`) | prompt atoms; `withProjectInstructions` appends `nerd.md` **outside** the JIT budget deliberately (`executor.go:1058-1064`) | — | — | — |
| S2b | File context | `executor.go:1065-1070` | `withFileContext` is applied **only when there is no working world** | **Go** | file text | — | — | — |
| S3 | Working loop begin | `working_context.go:101-126 beginWorkingLoop` | open a `WorkingSet` (a second, separate Mangle engine) scoped `sessionID/shardID/scope` | **Go** wiring; the engine is Mangle | `working.WorkingSet` selects which entities load (`working_set.go:224+ Select`) | — | `recall_context` (recovers evidence already gathered — reduces turns) | — |
| S4 | **Tool loop** | `executor_tools.go:207-405` | continue / stop / finalize / nudge / change regime | **kernel** — `WorkingSet.Continue` (`internal/context/working_set.go:160-222`) asserts `working_control/2`, `working_progress/5`, `working_regime_now/1` and queries `working_stop/1`, `working_finalize/1`, `working_nudge/1`, `working_regime/1`, `working_continue/0` from **`internal/context/working_set.mg`** | working context replaces the growing transcript; `boundToolLoopHistory` evicts oldest tool payloads, recoverable via the archive (`executor_tools.go:324-332`) | **This is the real obligation engine** — `working_stop(/read_only_stall)` (`working_set.mg:85-87`), `/repeated_cycle` (`:80`), `/tool_failures` (`:81`); `working_finalize(/verify_after_write)` (`:97-99`); `working_nudge(/implement\|/verify\|/conclude)` (`:102-110`); `working_regime(/commit)` closes read tools (`:60-78`) | catalog restricted under commit regime (`working_context.go:73-82`) | a `working_stop` returns `task unresolved: ...` (`executor_tools.go:288`) |
| S4b | Non-kernel budget | `executor_tools.go:181,393-404 toolBudgetController` | extend the iteration ceiling | **Go** (`maybeExtend`, `internal/session/tool_budget_controller.go`) | nudge text appended to the last tool result (`:253-260`) | a hard round ceiling — but production boot sets `MaxToolIterations = 0` and `MaxToolCalls = 0` (`internal/system/factory.go:2000-2002`), so **in production the ceiling is open and the Mangle policy is the sole stop** | — | — |
| S5 | Post-edit verification | `verify_outcome.go:93-142 runVerificationCommand`; `change_evidence.go:13-60 closeChangeEvidence` | did build/tests pass | **Go**, but with a genuinely 5-valued outcome: `VerifyPassed/Failed/Skipped/Indeterminate/Canceled` (`verify_outcome.go:22-36`). A timeout is **not** a failure (`change_evidence.go:42-47`). The verification budget is rooted at `context.Background()` so a model turn running out of time cannot manufacture a failure (`verify_outcome.go:84-90`). | — | `VerifyBuildAfterEdits` / `VerifyTestsAfterEdits` re-run after later edits invalidate earlier green checks (`change_evidence.go:23-34`) | `go build`, `go test` (offload cognition to deterministic code — legitimate) | `ChangeStage ∈ {artifact_changed, checks_passed, behavior_verified}` (`change_evidence.go:19,40,54`) |
| S6 | **Turn verdict** | `executor_tools.go:1923-1977 checkHollowSuccess` | is this turn done, hollow, failed or unverified | **kernel** — Go asserts one `turn_evidence(Verb, ToolCount, WriteCount, TestCount, ClaimedOutput, DreamMode)` fact and Mangle derives `hollow_success/1` and `turn_done/1` in **`internal/core/defaults/policy/coder_safety.mg:68-115`**. `turn_executed :- turn_evidence, !has_hollow_success(), !build_state(/failing)` (`:114`); `turn_done :- turn_executed, turn_acceptance(...)` (`:115`) — execution is explicitly weaker than completion. | — | **`hollow_success("new source was created without a test file")` (`coder_safety.mg:104-107`) is the test-coverage obligation the vision demands, and it is derived, not coded.** Also: "write-oriented intent completed without a recognized write-mutation tool" (`:96-100`) and "response presents test-runner output but no test-execution tool ran" (`:101-103`). | — | `TurnOutcome ∈ {/done, /hollow, /failed, /unverified}` (`executor_memory.go:128-147`) |
| S7 | Go false-failure heuristic | `executor.go:1093-1095` | declare execution failure | **Go**, by its own admission a heuristic: `if len(toolErrs) > 0 && strings.TrimSpace(result.Response) == "" { result.Error = ... }` | — | — | — | overrides nothing derived, but sets `result.Error`, which `captureTurnOutcome` then reads as `/failed` |
| S8 | Evidence → prose | `change_evidence.go:62-107 appendEvidenceReport` | how the verdict is surfaced | **Go** | — | — | — | **the structured verdict is flattened into `result.Response` text** (`:84`, `:105`: `"Wrote %d file(s): %s\nEvidence: %s. Requested behavior remains unverified (no acceptance contract)."`) |
| S9 | Return to caller | `observed_return.go:39-105 observedReturn`; `task_executor.go:258-261` | what the caller gets | **Go** | — | — | — | `observation.Return` carries `Build`, `Tests`, `Findings`, `Changed`, `Notes` — **but `ExecuteWithContext` returns `observed.Output` only, and `ChangeStage`/`TurnOutcome` are never copied into `observation.Return` at all** |

### 1E. What persists after a chat turn

- **Kernel facts**: `shard_result` (status guessed, result truncated to 200 chars,
  `process_continuation.go:187-197`), `pending_test`, `pending_review`, the model's
  filtered `MangleUpdates` (`process.go:1038`), `self_correction_hypothesis` (`:1055`).
- **SQLite**: session turn (`executor.go:1129 persistTurn`), learning trace
  (`executor.go:1126 queueTaxonomyLearning`, gated by
  `shard_profiles.<t>.enable_learning`), context feedback keyed to the JIT manifest
  hash (`process.go:908-927`), acceptance report (`change_evidence.go:77`).
- **Compressed context**: `compressor.ProcessTurn` (`process.go:1013`).
- **Conversation history**: `executor.go:1113-1122`, plus thought signatures.

## Journey 2 — `nerd chat` slash commands

### 2A. The structural fact

`cmd/nerd/chat/model_handlers.go:139-141`:

```go
if strings.HasPrefix(input, "/") {
    return m.handleCommand(input)
}
```

**Any input starting with `/` never reaches `processInput`.** It therefore never touches
perception, `user_intent`, `decideRoute` / `route_decision`, `seedIssueFacts`, reflection,
autopoiesis, `next_action`, `context_to_inject`, or `final_system_prompt`. The slash
surface is a second, parallel executive written entirely as a Go `switch`
(`cmd/nerd/chat/commands.go:54-266`, 60+ cases). Everything a slash command decides —
which shard, what task, whether to verify, when it is done — is decided in Go.

### 2B. Categories

Registry: `cmd/nerd/chat/command_categories.go:38-548`, five categories
(`CategoryCore/Basic/Advanced/Expert/System`, `:10-16`). The registry is the `/help`
source of truth only; **dispatch is the independent switch in `commands.go`** — a second
seam (a command can exist in one and not the other).

| Category | Commands (registry line) |
|---|---|
| Core | `/help` 43, `/status` 51, `/scan` 58, `/review` 65, `/test` 72, `/fix` 79, `/continue` 86, `/clear` 94, `/quit` 101 |
| Basic | `/new-session` 113, `/sessions` 120, `/load-session` 127, `/read` 134, `/mkdir` 141, `/write` 148, `/edit` 155, `/append` 162, `/search` 169, `/patch` 176, `/pick` 183, `/knowledge` 190, `/reflection` 197, `/usage` 204, `/agents` 211, `/define-agent` 218, `/spawn` 226, `/ingest` 233 |
| Advanced | `/refactor` 244, `/security` 251, `/analyze` 258, `/northstar` 265, `/alignment` 273, `/recurse` 281, `/yolo` 288, `/campaign` 295, `/launchcampaign` 302, `/clarify` 309, `/legislate` 316, `/query` 323, `/why` 330, `/shadow` 337, `/whatif` 344, `/browser` 351, `/transparency` 358, `/approve` 365, `/reject-finding` 372, `/accept-finding` 379, `/review-accuracy` 386 |
| Expert | `/logic` 397, `/glassbox` 404, `/tool` 411, `/jit` 418, `/cleanup-tools` 425, `/learn` 432, `/evolve` 439, `/evolution-stats` 446, `/evolved-atoms` 453, `/strategies` 460, `/promote-atom` 467, `/reject-atom` 474 |
| System | `/config` 485, `/features` 492, `/model` 499, `/init` 506, `/embedding` 513, `/refresh-docs` 520, `/scan-path` 528, `/scan-dir` 535, `/reset` 542 |

**Commands named in the brief that do not exist as slash commands:**

- `/dream` — not in `commands.go`. It is a *perception verb*: `processInput` branches on
  `intent.Verb == "/dream"` (`process.go:330-334`). Reachable only from free text, or
  from the CLI `nerd dream`.
- `/remember` — not in `commands.go`. Also a perception verb, listed in the conversational
  verb set at `internal/prompt/config_factory.go:510`, handled by
  `processMemoryOperations` (`process.go:326`).
- `/delegate*` — not user commands at all. `/delegate_coder`, `/delegate_reviewer`,
  `/delegate_tester`, `/delegate_researcher`, `/delegate_tool_generator` are
  **`next_action` atoms the kernel derives** via `action_mapping` (`policy/delegation.mg`,
  contract pinned in `internal/core/delegation_contract_test.go:23-36`).
- `/explain`, `/explain-off`, `/shards`, `/autopoiesis`, `/facts` are dispatched in
  `commands.go` / tested but are **not in `CommandRegistry`** — invisible to `/help`.
  **[REFUTED — see Verification]**

### 2C. Stage table — representative commands

| Command | Handler | Task string built how | Who decides the shard | Verification | Verdict |
|---|---|---|---|---|---|
| `/fix <desc>` | `commands_handlers_evolution.go:11-37` | `target = strings.Join(parts[1:]," ")`, then `formatShardTask("/fix", target, "", ws)` → `"fix issue in <desc>"` (`delegation.go:362`). The whole description becomes the *target*, so unlike the free-text path the user's words survive — wrapped in a template that tells the shard the description is a location. | **Go literal `"coder"`** (`:31`) | none — `spawnShardWithSpecialists` has no `VerifyWithRetry` | prose from `spawnSimpleShard` / specialist mode |
| `/refactor <t>` | `commands_handlers_evolution.go:40-66` | `formatShardTask("/refactor", …)` | Go literal `"coder"` | none | prose |
| `/security <t>` | `commands_handlers_analysis.go:562-578` | `formatShardTask("/security", …)` | Go literal `"reviewer"` (`:577 spawnShard`) | none | prose header `## Shard Execution Complete` (`delegation.go:651-657`) |
| `/analyze <t>` | `commands_handlers_analysis.go:581+` | `formatShardTask("/analyze", …)` | Go literal | none | prose |
| `/review [path] [--andEnhance]` | `commands_handlers.go:877-907+` | flags parsed in Go; bare arg = target, default `"."` | **Go**: if the agent registry has any agents, use multi-shard orchestrated review (`:904-907`), else single shard | none | aggregated via `review_aggregator.go` |
| `/recurse` | `commands.go:223-224` | rewritten to `/campaign recurse …` | Go | — | see Journey 4 |
| `/campaign …` | `commands.go:221-222 handleCampaignCommand` | — | — | — | see Journey 4 |
| `/shadow` | `commands.go:202-203 handleCmdShadow`, `chat/shadow.go` | — | — | — | see Journey 5 |
| `/clarify <goal>` | `commands.go:97-98` | runs the requirements interrogator shard | Go | — | prose questions |

### 2D. Where the kernel *does* get consulted on this path

One place found: specialist selection.
`spawnShardWithSpecialists` (`delegation.go:677-725`) matches specialists in Go
(`shards.MatchSpecialistsForTask`, `:698`), then **asserts them into the kernel and asks
`specialist_should_execute` (`policy/shards.mg`) which one executes** (`:705-712`), with
the Go boolean as the nil-kernel fallback. Execution mode (`ModeParallel` /
`ModeAdvisory` / `ModeAdvisoryWithCritique`) is then chosen by
`shards.GetExecutionMode(verb)` — Go (`:715-723`).

### 2E. What forces continuation on this path

Nothing at the chat layer. `spawnShard` (`delegation.go:605-670`) runs the shard once,
converts the result to facts, records it for prompt evolution, and formats prose. There
is no `VerifyWithRetry`, no `checkContinuation`, and no kernel obligation query. The
shard's own `internal/session` obligations (1D S4/S6) still apply inside the call.

## Journey 3 — `nerd fix "<brief>"` from the CLI

`fixCmd` (`cmd/nerd/cmd_direct_actions.go:45-56`) → `runDirectAction("coder", "/fix")`
(`:276-442`). `nerd review|test|explain|create|refactor|security|analyze|push|commit` all
share `runDirectAction` with a different `(shardType, verb)` pair.

| # | Stage | Component | Decision | Who decides | Knowledge in | Forces continuation | Tools | Verdict |
|---|---|---|---|---|---|---|---|---|
| F1 | Arg validation | `cmd_direct_actions.go:58-67 validateFixArgs` | with `--acceptance`, the task comes from the contract and a positional target is rejected | **Go** | — | — | — | — |
| F2 | Target assembly | `:278` `target := strings.Join(args, " ")`, or `:291` `target = contract.Task` | — | **Go** | — | — | — | — |
| F3 | Acceptance contract | `:279-293` `evidence.LoadContract`, `:312-314 evidence.WithContract(ctx, …)` | the turn carries a caller-authored behavioural contract | **user** | contract file | **this is the only obligation in the whole system that can make `turn_done` derive** — see F9 | — | — |
| F4 | Cortex boot | `:338 coresys.GetOrBootCortex` | — | Go | full kernel + world model + JIT | — | — | — |
| F5 | Root snapshot | `:391 snapshotDirectRoot` | detect undeclared writes to the workspace root | **Go** (files *and* directories; the campaign twin `snapshotWorkspaceRoot` skips dirs — noted in-code at `:380-384`) | — | — | — | reported, never corrected |
| F6 | Spawn | `:394 cortex.SpawnTaskWithTarget(ctx, verb, target, target)` → `internal/system/factory.go:428-458` → `session.TaskRequest{IntentVerb: "/fix", Task: target, Target: target}` → `TaskExecutor.Execute` | **the brief is passed through verbatim** | **Go, but lossless** | — | — | — | — |
| F7 | Shard run | as Journey 1 §1D | — | — | — | working-policy obligations (`working_set.mg`) | — | — |
| F8 | CLI hollow guard | `:428 checkDirectSpawnResult` | an empty result is never success, for any verb | **Go** | — | — | — | non-zero exit |
| F9 | Verdict | `internal/session/executor_tools.go:1923-1977`, policy `coder_safety.mg:114-115` | `/done` vs `/hollow` vs `/failed` vs `/unverified` | **kernel** | — | — | — | see below |

### 3A. The single most important difference from chat `/fix`

| | free-text "fix the nil deref in auth.go" | chat `/fix the nil deref in auth.go` | `nerd fix "the nil deref in auth.go"` |
|---|---|---|---|
| perception runs? | yes | **no** (`model_handlers.go:139`) | yes, but *inside* the executor on the already-classified preset (`task_executor.go:339`) |
| kernel routing arbitration? | yes (`route_decision`) | **no** | **no** — the verb is hardcoded in the cobra command (`cmd_direct_actions.go:55`) |
| what the shard is asked | `formatShardTask("/fix", intent.Target, …)` = `"fix issue in <classifier's target>"` — **the brief is replaced by the extracted target** | `"fix issue in <the whole brief>"` — brief survives, mislabelled as a location | **the brief verbatim**, as both `Task` and `Target` (`factory.go:452-456`) |
| acceptance contract possible? | no | no | **yes, `--acceptance`** |

The CLI path is the only one of the three that does not rewrite the user's request, and
the only one that can reach a `/done` verdict.

### 3B. `turn_done` is unreachable outside `nerd fix --acceptance`

Chain, all verified:

1. `turn_done(Verb) :- turn_executed(Verb), turn_acceptance(Verb, _, _).`
   (`internal/core/defaults/policy/coder_safety.mg:115`)
2. `turn_acceptance` is asserted only when `result.Acceptance != nil &&
   result.Acceptance.Status == "verified"` (`internal/session/executor.go:2334-2341`).
3. `result.Acceptance` comes from `result.acceptanceTransaction`, set only at
   `internal/session/executor.go:877-882` from `evidence.ContractFromContext(ctx)`.
4. `evidence.WithContract` has exactly two call sites in the tree:
   `cmd/nerd/cmd_direct_actions.go:313` and `cmd/tools/change_benchmark/main.go:173`.
5. The `--acceptance` flag is registered on `fixCmd` only (`cmd/nerd/main.go:200`).

Therefore: **every chat turn, every campaign task, every observer run and every other CLI
verb resolves to `TurnOutcome = /unverified`** at `internal/session/executor_memory.go:138-145`
(`/done` requires both a verified acceptance *and* a `turn_done` derivation). The kernel's
completion predicate is correct and essentially never fires in normal use.

## Journey 4 — campaigns / `nerd campaign recurse`

Entries: `nerd campaign start|status|pause|resume|list|recurse`
(`cmd/nerd/main.go:226-233`), chat `/campaign` and `/recurse`
(`cmd/nerd/chat/commands.go:221-224`), chat `/launchcampaign`, and the natural-language
assault trigger inside `processInput` (`cmd/nerd/chat/process.go:338-342`).

### 4A. Planning

`internal/campaign/decomposer.go:22` — "Decomposer creates campaign plans through LLM +
Mangle collaboration." `Decompose` (`:241-...`) hard-requires an LLM client (`:250-252`)
and Step 3 is `llmProposePlan` (`:342-353`). The phase/task *structure* of an ordinary
campaign is therefore **the model's prose, parsed** — not a kernel derivation. Mangle
validates and refines afterwards (Step 7 refinement at `:446`).

`nerd campaign recurse` is the exception and says so: "Each wave plans **deterministically
(no LLM decomposition)**: base primitives first, up through the layers, then across for
wiring, architectural review, and benchmarks" (`cmd/nerd/cmd_campaign_recurse.go:18-23`).

### 4B. Stage table — orchestrator run loop (`internal/campaign/orchestrator_execution.go:14-258`)

| # | Stage | Component | Decision | Who decides | Forces continuation | Verdict |
|---|---|---|---|---|---|---|
| C1 | Risk preflight | `:37-59 runRiskPreflight` → `risk_scoring.go:233+` | may this campaign start | **kernel grades, Go measures** — explicit in-code: "Everything Go observes goes here; the kernel grades it at the end" (`risk_scoring.go:242`). Advisory-board absence on a protected root is a Go-side hard block (`:248-262`). | — | `RiskGateEvaluation`, `EventRiskGateBlocked` |
| C2 | Root baseline | `:87 recordRootBaseline` | — | Go | — | — |
| C3 | Campaign timeout | `:90-95` | — | Go (`config.CampaignTimeout`) | — | — |
| C4 | Heartbeat + pause watcher | `:97-102` | autosave, external pause file | Go | — | — |
| C5 | Terminal-failure guard | `:133-148` | stop when status is failed | **Go** — added for F-STALL-1 because "the `campaign_blocked` derivation is transient and no longer fires after the blocking phase leaves `current_phase`" (`:134-137`). A Go guard compensating for a kernel predicate that is not monotone. | — | — |
| C6 | Current phase | `:168 getCurrentPhase` → `orchestrator_phases.go:13-41` | which phase is live | **kernel** — `current_phase` | — | — |
| C7 | Completion | `:171 isCampaignComplete` → `orchestrator_phases.go:184-203` | is the campaign done | **Go** — a loop over `phase.Status` in memory. Same for `isPhaseComplete` (`:225-...`). **The campaign's own completion verdict is not derived.** | — | `StatusCompleted` |
| C8 | Block reason | `:209 getCampaignBlockReason` → `orchestrator_phases.go:206-222` | why it stopped | **kernel** — `campaign_blocked` | — | — |
| C9 | Next phase | `:221 startNextPhase` → `orchestrator_phases.go:247-269` | which phase starts next | **kernel** — `phase_eligible`, first fact wins (`:268`, a Go tie-break over an unordered derivation) | — | — |
| C10 | Context paging | `:231-247 contextPager.ActivatePhase` + `PrefetchNextTasks(ctx, upcoming, 3)` | **what enters the window for this phase** | **Go** (the `3` is a literal) | — | `EventContextError` |
| C11 | Phase execution | `:250 runPhase` → `orchestrator_tasks.go:42` | parallelism + rolling checkpoints | Go | `runPhaseCheckpoint` (`orchestrator_utils.go:74`) | — |
| C12 | Task dispatch | `orchestrator_task_handlers.go:45-115 executeTask` | which handler runs | **Go** — `task.Shard` if set, else a 15-arm `switch` on `task.Type` | — | — |
| C13 | Task input | `:124 buildTaskInputWithSpecialistKnowledge` | context injected from dependent tasks + specialist knowledge | Go | — | — |
| C14 | `/document` fallback | `:129-135` | a failed document task falls back to direct generation | **Go**, to work around a real deadlock: the decomposer routes `/document` to the coder shard, which explores without writing, trips the hollow-success guard, and permanently fails the task (F-DOC-1) | — | — |
| C15 | Completion sweep | `:198 sweepUndeclaredRootWrites` | files the campaign left in the repo root are moved into the campaign's artifacts dir | **Go**, deliberately deferred to completion with the reasoning at `:185-197` | — | — |
| C16 | Northstar close | `:178-183 northstarObserver.EndCampaign` | — | Go | — | prose summary |

### 4C. `recurse` — the wave loop

`cmd/nerd/cmd_campaign_recurse.go:112-200 runCampaignRecurse`.

- **Bound**: `--waves N`; `--waves 0` is unbounded and **refused without yolo**
  (`checkRecurseYolo`, `:93-98`) — "an infinite loop is a decision the operator must take
  deliberately".
- **Stall fuse**: `StallWaveLimit` (default 2) — consecutive waves with nothing completed
  stop the run even when unbounded (`:26-29`, `resolveRecurseConfig:73-88`).
- **Angle rotation**: `harden,wire,review,test,bench,secure`, max 2 fixed via `--angles`
  (`:64`), otherwise rotated per wave. This is the closest thing in the system to "keep
  working until the behaviour holds": the next wave retargets from the previous wave's
  findings.
- Each wave is an ordinary resumable campaign linked by a shared recurse ID (`:50-51`),
  and wave >0 gets a fresh Northstar observer because the previous orchestrator closed
  its own (`recurseWaveConfig:103-110`).

### 4D. Judgement

Phase *selection* and *blocking* are kernel-derived; phase and campaign *completion* are
Go loops over in-memory status; the plan itself is LLM prose for every campaign type
except `recurse`. The stall fuse and the wave bound are Go, and they are the only
forcing mechanism above the per-turn level.

## Journey 5 — autonomous entries (guardian, observers, dream, tickers)

### 5A. System shards — event-driven with a heartbeat

Every Type-S system shard in `internal/shards/system/` runs the same loop shape
(`executive.go:463-521` is the reference; the twins are `perception.go:433`,
`router.go:229`, `planner.go:196`, `world_model.go:224`, `constitution.go:307`,
`campaign_runner.go:140`).

| Element | Line | Behaviour |
|---|---|---|
| Fact subscription | `executive.go:464` | `SubscribeToFacts(["user_intent","next_action","delegate_task","tdd_next_action","campaign_next_action","repair_next_action"])` — **the primary trigger is a kernel fact assertion, not a timer** |
| Heartbeat | `:467` | 15 s, raised from 5 s because "multi-shard 5s heartbeats stacked with 14–24s kernel evals" |
| Fallback ticker | `:470-479` | only armed when the event bus is unavailable (`factCh == nil`) |
| Work | `:491, :497 evaluatePolicy` | the kernel decides; Go executes |
| Autopoiesis | `:505-519` | `e.Autopoiesis.ShouldPropose()` on each heartbeat, async, 3-minute budget |

This is the closest the codebase gets to the vision's shape: a fact lands, the kernel
re-derives, Go executes.

### 5B. Northstar guardian / background observers

`internal/shards/observer_manager.go`.

| # | Stage | Component | Decision | Who decides | Verdict |
|---|---|---|---|---|---|
| O1 | Periodic check | `:358-379 periodicCheckLoop`, interval default 5 min (`:154`) | emit `EventAlignmentCheck` | **Go** | — |
| O2 | Tick suppression | `:381-386 periodicCheckDue` | skip a tick with nothing to assess (`atomic.SwapInt64(&m.eventsSinceCheck,0) > 0`) | **Go** — added after a measured pathology: "a check every five minutes from boot, each an LLM call of 13-43 s, each scoring the session 10-50/100 for having nothing to show — the guardian's own recommendation on every one was to stop evaluating empty ticks" (`:350-357`) | — |
| O3 | Dispatch | `:389-456 processEvent` | which observers run | **Go** — all `Active` observers, each in a goroutine with a 2-minute budget (`:413, :440`) | — |
| O4 | Assessment prompt | `:459-490 buildAssessmentTask` | **a hand-built prose prompt string, not a JIT-compiled one** | **Go** | — |
| O5 | **Assessment verdict** | `:492-542 parseAssessment` | the alignment score and level | **model prose, line-prefix parsed** — `SCORE:`, `VISION:`, `DEVIATIONS:`, `RECOMMENDATIONS:`. **Default `Score: 50, Level: LevelNote` when nothing parses** (`:496-497`), so an unparseable or empty model reply is recorded as a neutral assessment rather than as a failure. | `ObserverAssessment` |

### 5C. Dream

Entry: free text classified `/dream` (`cmd/nerd/chat/process.go:330-334`), or `nerd dream`
(`cmd/nerd/cmd_advanced.go`). Implementation `cmd/nerd/chat/process_dream.go:102+`.

- Shard selection: Go filters the available shards, skipping "internal" shards (`:142`)
  and "irrelevant specialists" by topic (`:148`), then priority-orders specialists first
  (`:200-206`).
- Consultation prompt: a Go template constant, `DREAM STATE CONSULTATION - DO NOT EXECUTE
  ANYTHING` (`:214`).
- Execution: sequential with a 1 s rate-limit delay (`:235`), `SessionContext{DreamMode:
  true}` (`:269-270`), `types.PriorityLow` (`:273`).
- `DreamMode` propagates into the shard and **disables the hollow-success gate entirely**
  (`internal/session/executor_tools.go:1928-1930`) and forces subagent execution
  (`internal/session/task_executor.go:305-307`).
- **Early stopping**: `:313` — Go stops consulting remaining generalists when a
  specialist gave a "confident answer", where confidence is measured as **the character
  count of the reply**.
- Dream policy exists (`internal/core/defaults/policy/dreamer.mg`) but the consultation
  loop above is Go.

### 5D. Complete ticker inventory (non-test)

Timers that can initiate work rather than poll for a result:

| Ticker | Location | Period |
|---|---|---|
| observer periodic check | `internal/shards/observer_manager.go:361` | 5 min (default) |
| system-shard heartbeats | `shards/system/{executive,perception,router,constitution}.go` | 15 s |
| system-shard heartbeats | `shards/system/{planner,world_model}.go` | 10 s |
| system-shard fallback ticks | same files | `config.TickInterval`, only without an event bus |
| campaign heartbeat / autosave | `internal/campaign/orchestrator_execution.go:288-289` | `HeartbeatEvery` / `AutosaveEvery` |
| reflection worker | `internal/store/reflection_worker.go:88,428` | `reflectionWorkerInterval` |
| Mangle engine | `internal/mangle/engine.go:191` | 30 s |
| on-demand world watcher | `internal/core/ondemand_watcher.go:52` | `onDemandFallbackSweep` |
| Mangle file watcher debounce | `internal/core/mangle_watcher.go:134` | 100 ms |
| factory background loop | `internal/system/factory.go:621` | configurable |
| flight recorder | `internal/observability/flight_recorder.go:169` | window-based |
| docker health | `internal/tactile/persistent_docker.go:223` | `HealthCheckInterval` |

The rest (`spawn_queue`, `manager_spawn`, `subagent`, `task_executor`,
`write_set_lock_manager`, `pause_durable`, `verify_outcome`, `lsp/client`, browser
sessions) are result-polling loops, not autonomous entry points.

## Journey 6 — `nerd scan` / world model / CodeDOM refresh

### 6A. `nerd scan`

`cmd/nerd/cmd_init_scan.go:109-121` (command), `:395-511 runScanWithKernelFactory`.

| # | Stage | Component | Decision | Who decides | Notes |
|---|---|---|---|---|---|
| W1 | Init guard | `:414-416` | refuse if `.nerd` is not initialised | Go | |
| W2 | Scan | `:423-429` `world.NewScanner().ScanWorkspaceCtx` | which files, which facts | **Go scanner** | ignore patterns from `world` config |
| W3 | **Validation gate** | `:431-440` | boot a full kernel and evaluate the exact fact set **before** replacing the cache | **Go, deliberately**: "The scanner's durable DB cache is the preferred boot source, so it must never be replaced until the exact fact set has evaluated successfully" (`:431-433`) | a genuine deterministic-offload tool |
| W4 | Profile reload | `:442-448` | `.nerd/profile.mg` | Go | failure is a warning, not an error |
| W5 | Persist (DB) | `:450-462 world.PersistFastSnapshotToDB` → `.nerd/knowledge.db` | the preferred incremental boot source | Go | |
| W6 | Persist (`.mg`) | `:464-469 writeScanFacts` → `.nerd/mangle/scan.mg` | boot-time reload source, header `# Auto-generated scan facts - DO NOT EDIT` (`:561`) | Go | |
| W7 | Report | `:471-508` | counts by `file_topology` / `directory` / `symbol_graph` | Go | |

### 6B. Predicate ownership (from `./nerd.exe world predicates`, run 2026-09-18)

The kernel publishes its own scan-scope map. Groups and their replacement semantics:

| Scope | Predicates | Replacement rule |
|---|---|---|
| scanner | `file_topology`, `directory`, `file_dir`, `test_file_for`, `symbol_graph`, `dependency_link`, `entry_point`, `project_language` | replaced on every full scan; re-derived by every fast scan |
| deep / Cartographer | `code_defines`, `code_calls`, `assigns`, `guards_return`, `guards_block`, `guard_dominates`, `safe_access`, `uses`, `call_arg`, `error_checked_return`, `error_checked_block`, `function_scope` | replaced **per file** by a deep scan only |
| lsp | `symbol_defined`, `symbol_referenced`, `code_diagnostic`, `symbol_completion` | projected by language servers; **a scan must not delete these** **[REFUTED — see Verification]** |
| session scope | `active_file`, `file_in_scope`, `code_element` | ephemeral, session lifetime |
| git | `git_history`, `churn_rate` | on-demand git scanner |

### 6C. When the JIT selector consumes it

`internal/prompt/compiler.go`. Selection is "rule-based selection (Mangle) with semantic
search (vectors)" (`:295`); `vector_hit(AtomID, Score)` is the bridge fact
(`compiler.go:70`, consumed at `policy/jit_selection.mg:252`).

**The selection policy is partially live, and says so in its own header**
(`internal/core/defaults/policy/jit_selection.mg:5-31`, status dated 2026-08-11):

- `prohibited_atom` and `conflict_loser` **are** consumed by `jit_compiler.mg` as vetoes,
  so the firewall and conflict rules bind on every compile (`:9-11`).
- `selected_atom`, `candidate_atom` and `mandatory_atom` are admissions and **nothing
  queries them**. The Go selector queries only `selected_result/3` from `jit_compiler.mg`
  (`:13-20`). **[REFUTED — see Verification]**
- Wiring `selected_atom` into `tentative` was measured and rejected: a `/fix` compile went
  from 67 atoms / 26 279 tokens to 254 atoms / 65 036 of 65 536 tokens — 99.2 % budget
  saturation — because each admitted atom recursively pulls its `atom_requires`
  dependencies in (`:14-18`).
- The rule this established, in the file's own words: **"a second opinion in a selector
  may veto, never admit."**
- Two `base_prohibited` rules are inert because no producer emits dimension `/tag`
  (`:30-31`).

So: the kernel *vetoes* what enters the window; the ranking and admission that decide what
actually lands are Go plus vector scores.

### 6D. Context that bypasses the JIT budget entirely

Three paths append to the prompt after compilation, outside the token budget the selector
enforced:

1. `withProjectInstructions` — `nerd.md`, appended at `internal/session/executor.go:1064`,
   deliberately not modelled as an atom because "the atom selector has no way to score a
   document it has never seen, and budget-driven eviction could silently drop the
   project's own rules" (`:1058-1063`).
2. `withFileContext` — appended at `executor.go:1069`, but **only when there is no working
   world** (`:1065-1070`).
3. `stevenMoorePersona` — appended unconditionally to the chat articulation prompt at
   `cmd/nerd/chat/process.go:832`.

## Seams

Every place two components disagree about a contract. Both sides cited.

### S-1. `types.ShardResult` has no outcome; `ExecutionResult` has four

| Side | Citation | What it believes |
|---|---|---|
| producer | `internal/session/executor_memory.go:128-147` | the turn's outcome is `/done`, `/hollow`, `/failed` or `/unverified`, plus `ChangeStage ∈ {artifact_changed, checks_passed, behavior_verified}` and 5-valued `BuildCheck`/`TestCheck` verdicts |
| projection | `internal/session/observed_return.go:39-105` | carries `Build`, `Tests`, `Findings`, `Changed`, `Notes` — **but not `TurnOutcome` and not `ChangeStage`** |
| transport | `internal/session/task_executor.go:258-261` | `return observed.Output, err` — a `string` |
| consumer | `internal/types/shard.go:100-105` | `ShardResult{ShardID, Result string, Error error, Timestamp}` |

Three verdict vocabularies are computed and none of them crosses into `cmd/nerd/chat`.
The only thing that crosses is prose, plus whatever `appendEvidenceReport`
(`internal/session/change_evidence.go:84, 105`) chose to *write into* that prose.

### S-2. Status guessed from substrings

| Side | Citation |
|---|---|
| writer | `internal/session` computes a real verdict (S-1) |
| reader | `cmd/nerd/chat/process_continuation.go:179-184`: `strings.Contains(result,"TODO")` → `/incomplete`; `shardType=="coder" && !strings.Contains(lower(result),"test")` → `/code_generated`; `:212` `reviewer && contains(lower(result),"issue")` → assert `pending_review` |

The `shard_result` fact the kernel's continuation protocol reasons over
(`should_auto_continue`, `has_pending_subtask`) is built from these guesses — and stores
only `truncateSummary(result, 200)` (`:194`), so the kernel sees the first 200 characters
of the shard's prose as the evidence for whether more work is owed.

### S-2b. The kernel's test-continuation obligation fires on the word "test"

`internal/core/defaults/policy/codedom_continuation.mg:7-9`:

```
has_pending_subtask(TaskID, Description, /tester) :-
    shard_result(_, /code_generated, /coder, Task, _),
    pending_test(TaskID, Description).
```

`/code_generated` has exactly one producer:
`cmd/nerd/chat/process_continuation.go:182-184` —
`if shardType == "coder" && !strings.Contains(strings.ToLower(result), "test")`.
So the kernel's "this code needs tests" obligation is derived from *whether the coder's
prose happened to contain the substring "test"*. A coder that wrote real tests and said so
suppresses the obligation; a coder that wrote none and never used the word triggers it.

### S-2c. Two dead `has_pending_subtask` rules

`codedom_continuation.mg:21-26` derives pending subtasks from
`shard_result(TaskID, /tests_needed, …)` and `shard_result(TaskID, /review_needed, …)`.
Grep over all `*.go` finds **no producer of either status** — the only writer of
`shard_result` is `injectShardResultFacts`, which emits only `/complete`, `/failed`,
`/incomplete` and `/code_generated` (`process_continuation.go:173-184`). Both rules are
inert.

### S-2d. `ResultToFacts` and `injectShardResultFacts` write different vocabularies

| Writer | Predicates |
|---|---|
| `internal/core/shards/manager_spawn.go:650-692 ResultToFacts` | `shard_executed`, `last_shard_execution`, `shard_success` / `shard_error`, `shard_output` (truncated at 4000 chars, `:676-678`), `recent_shard_context` |
| `cmd/nerd/chat/process_continuation.go:167-222 injectShardResultFacts` | `shard_result` (truncated at 200 chars), `pending_test`, `pending_review` |

Both run on the same delegation (`process.go:575` and `process.go:615` →
`checkContinuation` → `injectShardResultFacts`), producing two independently-truncated
records of the same shard output — 4000 characters in one predicate, 200 in the other —
and the continuation policy reads only the 200-character one.

### S-3. `RouteClarify` is derived and never read

| Side | Citation |
|---|---|
| producer | `policy/routing_arbitration.mg:107-112` derives `route_decision(/clarify, /none)`; `cmd/nerd/chat/delegation_routing.go:160-161` maps it to `RouteClarify` |
| consumer | none. `process.go` consumes `RouteRespondDirectly` (`:354`), `RouteMultiStep`/`RouteLegacy` (`:423-428`) and `RouteDelegate` (`:462-465`). Grep for `RouteClarify` over the tree finds only the declaration, the `String()` arm, the assignment, and one test (`routing_arbitration_roundtrip_test.go:148`). |

Clarification is instead decided by `shouldAutoClarify`
(`cmd/nerd/chat/process_dream_delegation.go:23-44`), a substring bank over
`campaign|plan|roadmap|project|initiative|blueprint|feature` ANDed with
`intent.Category ∈ {/mutation, /instruction}`.

### S-4. Lane precedence lives in two places

`policy/routing_arbitration.mg:20-24` documents the precedence
(`respond_directly > multi_step > delegate > clarify`) but does not enforce it; the switch
that actually enforces it is Go (`cmd/nerd/chat/delegation_routing.go:152-164`). The
policy comment and the Go switch must be edited together, and nothing checks that they
agree.

### S-5. `/help` registry vs dispatch switch

| Side | Citation |
|---|---|
| registry | `cmd/nerd/chat/command_categories.go:38-548` — 71 entries, `/help`'s source of truth **[REFUTED — see Verification]** |
| dispatcher | `cmd/nerd/chat/commands.go:54-266` — an independent `switch` |

`/explain`, `/explain-off`, `/recurse` (dispatch at `commands.go:189-192, 223`) and the
`/shards`, `/autopoiesis`, `/facts` commands exercised in `commands_test.go:452, 465` and
`live_kernel_test.go:84` are dispatchable but absent from (or inconsistently present in)
the registry. A command can exist in one and not the other with no build error.

### S-6. Task string means two different things

`formatShardTask("/fix", target, …)` → `"fix issue in <target>"`
(`cmd/nerd/chat/delegation.go:362`).

| Caller | What `target` is |
|---|---|
| free text (`process.go:479`) | `intent.Target` — the classifier's extracted *location* |
| chat `/fix` (`commands_handlers_evolution.go:19-20`) | `strings.Join(parts[1:]," ")` — the user's whole *description* |

The same template receives a file path from one caller and a sentence from the other, and
tells the shard both are the place to fix.

### S-7. Verification timeout vs failure

`internal/session/verify_outcome.go:22-36` deliberately distinguishes
`VerifyIndeterminate` (budget exhausted) from `VerifyFailed`, and `change_evidence.go:42-47`
honours it ("Only an affirmative failure fails the turn"). But
`internal/session/executor.go:1093-1095` sets `result.Error` from a different rule
entirely — any tool error plus an empty response — and `captureTurnOutcome`
(`executor_memory.go:135-136`) reads `result.Error != nil` as `/failed`. A turn can be
`/failed` while both mechanical checks are `VerifyIndeterminate`.

### S-8. `turn_done` requires acceptance that only one flag can supply

`policy/coder_safety.mg:115` requires `turn_acceptance`; `executor.go:2334-2341` asserts it
only for a verified acceptance report; `executor.go:877-882` builds one only from
`evidence.ContractFromContext`; the only production writer is
`cmd/nerd/cmd_direct_actions.go:313`, reachable only via `--acceptance`, registered only on
`fixCmd` (`cmd/nerd/main.go:200`). Everything else in the system is `/unverified` forever.

### S-9. `campaign_blocked` is transient; the loop needs it to be stable

`internal/campaign/orchestrator_execution.go:133-148` adds a Go terminal-failure guard
because "the `campaign_blocked` derivation is transient and no longer fires after the
blocking phase leaves `current_phase`", which made the loop spin "no eligible phases"
forever (F-STALL-1). The kernel's predicate and the Go loop disagree about whether a block
is an event or a state.

### S-10. `phase_eligible` is a set; the caller takes the first

`orchestrator_phases.go:262-268` queries `phase_eligible` and takes `facts[0]`. Mangle
derivations are unordered, so phase ordering among equally-eligible phases is whatever the
engine happened to emit.

### S-11. Two kinds of "campaign complete"

Phase selection and blocking are kernel-derived (`current_phase`, `phase_eligible`,
`campaign_blocked`), but `isCampaignComplete` (`orchestrator_phases.go:184-203`) and
`isPhaseComplete` (`:225+`) are Go loops over in-memory `Status` fields. The kernel never
gets asked whether the campaign is done.

### S-12. Observer verdict defaults to 50

`internal/shards/observer_manager.go:496-497` initialises `Score: 50, Level: LevelNote` and
`parseAssessment` only overwrites them if the model emitted a literal `SCORE:` line
(`:508-516`). An empty, truncated or differently-formatted reply is recorded as a neutral
alignment assessment indistinguishable from a real one.

### S-13. Dream confidence is a character count

`cmd/nerd/chat/process_dream.go:313` stops consulting the remaining generalist shards when
a specialist "provided confident answer (%d chars)". The producer of that reply has no idea
its length is being read as confidence.

### S-14. Two Mangle engines, two policy corpora

The turn-verdict and routing policies live in the kernel corpus
(`internal/core/defaults/policy/*.mg`), while the tool-loop continuation policy lives in a
**separate engine and separate file**: `internal/context/working_set.mg`, driven by
`internal/context/working_set.go:160-222`, scoped `sessionID/shardID/scope`
(`internal/session/working_context.go:116-122`). `working_stop`, `working_finalize`,
`working_nudge`, `working_regime` are invisible to `nerd query`/`nerd logic` against the
main kernel, and `internal/core/defaults/policy/` contains no reference to them.

### S-15. `/document` tasks trip the hollow-success guard by design

`internal/campaign/orchestrator_task_handlers.go:129-135` (F-DOC-1): the decomposer routes
`/document` to the coder shard, which explores without writing, trips the (correct)
hollow-success guard, and permanently fails the task, deadlocking the phase. The fix is a
Go fallback to direct document generation. The decomposer's routing and the coder's
obligation disagree, and neither was changed.

### S-16. `snapshotDirectRoot` vs `snapshotWorkspaceRoot`

`cmd/nerd/cmd_direct_actions.go:380-384` records this explicitly: the campaign helper
`snapshotWorkspaceRoot` records only files (skips `IsDir`) and "would have missed the
`research/` directory entirely"; the direct-action twin records directories too. Two
implementations of the same undeclared-write guard with different coverage.

### S-17. Verified-but-unused config, and config that is documented as live

`.claude/rules/nerd-config-schema.md` maintains a standing "Config that looks live but
isn't" table (`split_pane_ratio`, `logic_pane_width` unreachable; two shard-profile keys
deleted). That this table has to exist is itself the seam: `UserConfig` uses strict JSON
decoding (`internal/config/user_config.go`), so a key that is written before its Go field
exists is a hard load error for every older binary.

## Where the kernel is bypassed

The vision names five executive decisions that should be the fixpoint of the kernel over
the facts (`CLAUDE.md:49-54`). Scored:

### 1. What a turn is

| Path | Decided by |
|---|---|
| chat free text | **Go** — `processInput` (`cmd/nerd/chat/process.go:48-1090`), a linear waterfall; the turn's bound is a context deadline (`:76`) |
| chat slash command | **Go** — `handleCommand` switch (`cmd/nerd/chat/commands.go:54-266`); never enters the OODA path at all (`model_handlers.go:139-141`) |
| CLI direct verb | **Go** — `runDirectAction` (`cmd/nerd/cmd_direct_actions.go:276-442`) |
| inside a shard | **kernel** ✅ — `working_continue/0` and `working_stop/1` (`internal/context/working_set.mg:80-91`) |

### 2. Whether it is done

| Level | Decided by |
|---|---|
| tool loop | **kernel** ✅ — `working_stop`, `working_finalize` (`working_set.mg:80-99`) |
| turn verdict | **kernel** ✅ — `hollow_success/1`, `turn_done/1` (`policy/coder_safety.mg:92-115`) — **but `turn_done` is unreachable without `nerd fix --acceptance`** (S-8), so in practice the answer is always `/unverified` |
| turn error | **Go** — `executor.go:1093-1095`, a self-described heuristic |
| continuation across turns | **kernel** reads `should_auto_continue`/`has_pending_subtask`, **but the facts it reads are Go substring guesses** (S-2) |
| campaign / phase | **Go** — `isCampaignComplete`, `isPhaseComplete` (`orchestrator_phases.go:184-244`) |
| recurse sweep | **Go** — wave count and stall fuse (`cmd/nerd/cmd_campaign_recurse.go:63-98`) |

### 3. What is delegated, and with what task

| Decision | Decided by |
|---|---|
| *whether* to delegate, free text | **kernel** ✅ — `route_decision(/delegate, Shard)` over `should_delegate/1` (`routing_arbitration.mg:101-103`, `delegation.mg:335-338`) |
| verb → shard lookup | **Go** — `perception.GetShardTypeForVerb` + `resolveShardTypeForIntent` (`delegation_routing.go:205-227`), including a substring heuristic on the target (`:218-225`) and an LLM `shard=` suggestion (`:209-216`). The policy comment concedes this deliberately (`delegation.mg:330-334`). |
| *whether* to delegate, slash command | **Go** — the shard name is a literal in the handler (`commands_handlers_evolution.go:31` `"coder"`, `commands_handlers_analysis.go:577` `"reviewer"`) |
| *whether* to delegate, CLI | **Go** — hardcoded in the cobra command (`cmd_direct_actions.go:55`) |
| which specialist executes | **kernel** ✅ — `specialist_should_execute` (`policy/shards.mg`) via `delegation.go:705-712` |
| lane precedence | **Go** — `delegation_routing.go:152-164` (S-4) |
| **the task string** | **Go** — `formatShardTask` (`delegation.go:311-398`) rebuilds the request from `(verb, target, constraint)` templates. The kernel never sees or shapes what the shard is asked. |
| campaign plan | **model prose** — `decomposer.go:342-353 llmProposePlan`; `recurse` is the exception (`cmd_campaign_recurse.go:18-19`) |
| campaign task → handler | **Go** — 15-arm switch (`orchestrator_task_handlers.go:68-114`) |
| dream consultation set | **Go** — filters, priority order, 1 s spacing, char-count early stop (`process_dream.go:142-313`) |

### 4. What enters the window

| Decision | Decided by |
|---|---|
| atom veto | **kernel** ✅ — `prohibited_atom`, `conflict_loser` consumed by `jit_compiler.mg` (`jit_selection.mg:9-11`) |
| atom admission | **Go + vectors** — `selected_atom`/`candidate_atom`/`mandatory_atom` derive and **nothing queries them**; the selector reads only `selected_result/3` (`jit_selection.mg:13-20`). Admission was wired to the kernel once, measured at 99.2 % budget saturation, and reverted. **[REFUTED — see Verification]** |
| working-set entity selection | **kernel** ✅ — `WorkingSet.Select` over the canonical context rules (`internal/context/working_set.go:224-232`); `working_transcript_rounds(3)` (`working_set.mg:23-24`) **[REFUTED — see Verification]** |
| tool-history eviction | **Go** — `boundToolLoopHistory` (`executor_tools.go:332`) and the hard `history[len-3:]` truncation at `:379-381` |
| `nerd.md` | **Go, outside the JIT budget** — `executor.go:1058-1064`, deliberately |
| file context | **Go** — `withFileContext`, only when no working world (`executor.go:1065-1070`) |
| persona | **Go** — `stevenMoorePersona` appended unconditionally (`process.go:832`) |
| perception history window | **Go** — all of `m.history` or `compressor.GetRecentTurnWindow()` (`process.go:161-167`) |
| campaign phase context | **Go** — `PrefetchNextTasks(ctx, upcoming, 3)` (`orchestrator_execution.go:245`) |
| blackboard projection into the next task | **Go** — `priorShardContext` (`delegation.go:145-186`), though structurally bounded and expandable rather than truncated |

### 5. What the verdict is

| Decision | Decided by |
|---|---|
| hollowness / done | **kernel** ✅ — `policy/coder_safety.mg:92-115` (see caveat in §2) |
| build / test outcome | **Go**, with a well-designed 5-valued type (`verify_outcome.go:22-36`) |
| `ChangeStage` | **Go** — `change_evidence.go:19, 40, 54` |
| chat-visible status | **Go substrings** — `injectShardResultFacts` (S-2) |
| what the user is shown | **prose** — `appendEvidenceReport` flattens the structured verdict into `result.Response` (`change_evidence.go:84, 105`), then `formatDelegatedResponse` wraps it in a Go-chosen header (`delegation.go:405-430`) |
| escalation after max retries | **Go string compare** — `verifyErr.Error() == "max retries exceeded - escalating to user"` (`process.go:511`) |
| observer alignment | **model prose, prefix-parsed, default 50** (S-12) |
| campaign risk gate | **kernel grades, Go measures** ✅ (`risk_scoring.go:242`) — the cleanest example of the intended split in the codebase |

### Summary

The kernel genuinely owns: routing arbitration for free text, the delegation confidence
gate, specialist selection, campaign phase eligibility and blocking, campaign risk
grading, tool-loop continuation (including the read-only-stall and verify-after-write
obligations), JIT atom vetoes, working-set selection, and the turn verdict including the
test-coverage obligation.

The kernel does not own: what a turn is on any of the three user-facing entry paths; the
task string handed to a shard; the verb→shard lookup; anything reached through a slash
command or a CLI verb; campaign and phase completion; campaign planning; JIT atom
admission; three of the four direct-answer paths; all four clarification paths; and the
status the chat layer actually shows the user.

Two structural notes that cut across all five axes:

1. **The slash-command surface and the CLI direct-verb surface are complete parallel
   executives.** Neither runs perception, `route_decision`, `next_action`,
   `context_to_inject`, or `final_system_prompt` at the command layer. Roughly half the
   ways a user can drive codeNERD never consult the kernel about what to do — only about
   how the shard behaves once it is already running.
2. **The obligations that exist are real and well-designed, and they all live below the
   turn.** `working_set.mg` and `coder_safety.mg` force implement/verify/conclude, stop a
   read-only stall, and refuse a hollow completion. Nothing above the turn forces
   anything: the chat layer ends when a shard returns prose, and `turn_done` — the
   predicate that would mean "the behaviour the north star envisions holds" — cannot
   derive without an acceptance contract that only one CLI flag can supply.

## Verification (adversarial, 2026-09-18)

Independent adversarial re-check of the claims above against the code on
`dogfood/c2-closure`. Default verdict is "refuted" where the code does not say what the
document says. Rows are appended as each claim is checked.

### Status

last updated: 2026-09-18 (complete)
checked: 95
refuted: 7

| # | section | claim (short) | cited location | verdict | evidence | note |
|---|---|---|---|---|---|---|
| 1 | 3B / S-8 | `turn_done :- turn_executed, turn_acceptance(...)` | `policy/coder_safety.mg:115` | holds | `coder_safety.mg:115` `turn_done(Verb) :- turn_executed(Verb), turn_acceptance(Verb, _, _).` | `turn_executed` at `:114` also matches the quote exactly. |
| 2 | 3B / S-8 | `turn_acceptance` asserted only for a verified acceptance report | `internal/session/executor.go:2334-2341` | holds | `executor.go:2334` `if result.Acceptance != nil && result.Acceptance.Status == "verified" {`; assert at `:2336` | Stronger than claimed: `internal/core/mangle_updates.go:144-149` `predicateAllowed` hard-blocks `turn_acceptance`/`turn_evidence`/`turn_executed`/`turn_done`/`turn_cost` from model updates — "never model observations". |
| 3 | 3B / S-8 | `result.Acceptance` built only from `evidence.ContractFromContext` | `internal/session/executor.go:877-882` | holds | `executor.go:877` `if contract, ok := evidence.ContractFromContext(ctx); ok {` ... `:882` `result.acceptanceTransaction = tx` | — |
| 4 | 3B / S-8 | `evidence.WithContract` has exactly two call sites | `cmd/nerd/cmd_direct_actions.go:313`, `cmd/tools/change_benchmark/main.go:173` | holds | tree-wide grep returns those two plus the definition at `internal/evidence/context.go:7` | Only one is a production path; the other is a benchmark tool. |
| 5 | 3B / S-8 | `--acceptance` registered on `fixCmd` only | `cmd/nerd/main.go:200` | holds | `main.go:200` `fixCmd.Flags().String("acceptance", ...)`; tree-wide grep for other registrations finds only `cmd/nerd/acceptance_args_test.go` | `cmd_direct_actions.go:280` guards with `Lookup("acceptance") != nil`, so other verbs sharing `runDirectAction` cannot supply one. |
| 6 | 3B | every other path resolves to `/unverified`; `/done` needs verified acceptance AND `turn_done` | `internal/session/executor_memory.go:138-145` | holds | `executor_memory.go:138-142` `outcome := types.MangleAtom("/unverified")` / `if e.kernel != nil && result.Acceptance != nil && result.Acceptance.Status == "verified" { doneFacts, err := e.kernel.Query("turn_done")` | — |
| 7 | 1D S6 / 5-verdict | `TurnOutcome in {/done,/hollow,/failed,/unverified}` | `internal/session/executor_memory.go:128-147` | holds | `captureTurnOutcome` at `:128`, four-arm switch `:132-146` | — |
| 8 | 1D S7 / S-7 | Go false-failure heuristic sets `result.Error` on tool errors + empty response | `internal/session/executor.go:1093-1095` | holds | `executor.go:1093` `if len(toolErrs) > 0 && strings.TrimSpace(result.Response) == "" {` and the in-code comment calls itself "a conservative heuristic" (`:1090-1092`) | The doc's quote of the condition is verbatim. |
| 9 | S-7 | `captureTurnOutcome` reads `result.Error != nil` as `/failed` | `internal/session/executor_memory.go:135-136` | holds | `:135` `case result.Error != nil:` `:136` `result.TurnOutcome = types.MangleAtom("/failed")` | — |
| 10 | 1D S6 | `hollow_success` reasons quoted verbatim, incl. the test-coverage obligation | `policy/coder_safety.mg:96-107` | holds | `:96` "write-oriented intent completed without a recognized write-mutation tool"; `:101` "response presents test-runner output but no test-execution tool ran"; `:104` "new source was created without a test file" | All three strings are byte-exact; `:92-115` range in axis-2 table also correct. |
| 11 | 2A | any `/`-prefixed input never reaches `processInput` | `cmd/nerd/chat/model_handlers.go:139-141` | holds | `:139-141` `if strings.HasPrefix(input, "/") { return m.handleCommand(input) }` — quoted verbatim; `processInput` has only three callers (`model_handlers.go:217, 498, 532`), none in `commands.go` | A `/`-prefixed line typed while `pendingPatchLines` is collecting (`:133`) is swallowed even earlier, so the claim is if anything understated. |
| 12 | 2A | dispatch is a Go `switch`, 60+ cases | `cmd/nerd/chat/commands.go:54-266` | holds | `switch cmd {` at `:54`, `default:` at `:256`, switch closes at `:266`; 71 `case "` labels in that range | — |
| 13 | 2A | slash path never touches `route_decision`/`next_action`/`context_to_inject`/`final_system_prompt` | — | holds | sole call sites: `process.go:353` (`decideRoute`), `:761` (`next_action`), `:818` (`context_to_inject`), `:824` (`final_system_prompt`) — all inside `processInput` | — |
| 14 | S-14 / 1D S3-S4 | `working_set.mg` runs in a second, separate Mangle engine under `internal/context/` | `internal/context/working_set.mg`, `working_set.go` | holds | `working_set.go:24` `//go:embed working_set.mg`; `:55-61` `mangle.DefaultConfig()` / `mangle.NewEngine(cfg, nil)` / `engine.LoadSchemaString(schemas + "\n" + policy + "\n" + workingSetPolicy)`; struct field `engine *mangle.Engine` at `:36` | The file's own header says it is "Loaded beside the canonical context_compilation.mg in a private evaluation scope", so the second engine also carries core context rules. |
| 15 | 1D S4 | `WorkingSet.Continue` asserts `working_control/2`, `working_progress/5`, `working_regime_now/1`; queries `working_stop/1`, `working_finalize/1`, `working_nudge/1`, `working_regime/1`, `working_continue/0` | `internal/context/working_set.go:160-222` | holds | `func (w *WorkingSet) Continue` at `:160`; `ReplaceControlFacts(facts, "working_control", "working_progress", "working_regime_now")` at `:186`; the five queries follow at `:200-216` | Ordered and short-circuiting: a `working_stop` returns before finalize/nudge/regime are even asked. |
| 16 | 1D S4 | working-policy rule line cites | `working_set.mg:23-24, 60-78, 80-91, 85-87, 97-99, 102-110` | holds | `:24` `working_transcript_rounds(3).`; `:80` `working_stop(/repeated_cycle)`; `:81` `working_stop(/tool_failures) ... Failed >= 3`; `:85` `working_stop(/read_only_stall)`; `:90-91` `working_stopped()` / `working_continue()`; `:97` `working_finalize(/verify_after_write)`; `:102/:105/:108` `working_nudge(/implement\|/verify\|/conclude)`; `working_regime(/commit)` at `:60, :68, :76` | Every cited line matches. |
| 17 | S-14 | `internal/core/defaults/policy/` contains no reference to the `working_*` predicates | — | holds | grep for `working_stop\|working_finalize\|working_nudge\|working_regime\|working_continue` over `internal/core/defaults/` returns nothing | — |
| 18 | 6C | `jit_selection.mg` header: vetoes live, admissions dead, 99.2% measurement, `/tag` inert | `jit_selection.mg:5-31, 9-11, 13-20, 14-18, 30-31` | holds | `:6` "STATUS (as of 2026-08-11): this ruleset is PARTIALLY LIVE."; `:16` "to 254 atoms and 65036 of 65536 tokens, 99.2 percent budget saturation"; `:20` "may veto, never admit"; `:30-31` the `/tag` inert note | Every quoted phrase is byte-exact, including the 67-atom/26279-token baseline. |
| 19 | 6C / axis 4 | "the Go selector queries only `selected_result/3`" | `jit_selection.mg:13-20` | overstated | `internal/prompt/selector.go:997` and `:1232` query `selected_result(Atom, Priority, Source)`, but `:977` also queries `blocked_by_context(Atom)` and `:985` `mandatory_selection(Atom)` | Both extra queries are explicitly "Debug:" — their results are only logged, never consumed for selection — so the substantive point stands, but "only" is literally false. The doc repeats the policy comment as fact without checking it. |
| 20 | 6C | `prohibited_atom` / `conflict_loser` consumed by `jit_compiler.mg` as vetoes | `jit_selection.mg:9-11` | holds | `internal/core/defaults/jit_compiler.mg:261-273` "Policy veto bridge (restrictive direction only)" then `prohibited(Atom) :- prohibited_atom(Atom).` / `suppressed(Atom) :- conflict_loser(Atom).` | Note the path: `jit_compiler.mg` is at `internal/core/defaults/`, **not** under `policy/` like the other three JIT files. |
| 21 | 6C | `vector_hit` is the bridge fact; selection is "rule-based (Mangle) with semantic search (vectors)" | `internal/prompt/compiler.go:70`, `:295` | holds | `compiler.go:70` `"vector_hit",` in the kernel-predicate list; `:295` `// It combines rule-based selection (Mangle) with semantic search (vectors)`; consumed at `jit_selection.mg:251-254` | Doc cites `jit_selection.mg:252`; the `candidate_atom` rule head is at `:251` with `vector_hit(AtomID, Score)` at `:252`. |
| 22 | 10c / S-2 / S-2b | chat status is guessed from substrings | `cmd/nerd/chat/process_continuation.go:179-184, 194, 212` | holds (one line off) | `:179-181` `if strings.Contains(result, "TODO") \|\| strings.Contains(result, "FIXME") { status = "/incomplete" }`; `:182-184` `if shardType == "coder" && !strings.Contains(strings.ToLower(result), "test") { status = "/code_generated" }`; `:194` `truncateSummary(result, 200)` | The reviewer/`pending_review` guess the doc cites at `:212` is actually at `:214` (`if shardType == "reviewer" && strings.Contains(strings.ToLower(result), "issue")`). Everything else is exact. |
| 23 | S-2b | `has_pending_subtask(.., /tester)` fires off `/code_generated` | `policy/codedom_continuation.mg:7-9` | holds | `:7-9` quoted verbatim, including the `pending_test(TaskID, Description)` join | `/code_generated` has exactly one producer, `process_continuation.go:183`. |
| 24 | S-2c | the `/tests_needed` and `/review_needed` rules are inert | `policy/codedom_continuation.mg:21-26` | holds | `:21-22` and `:24-26` are the two rules; a tree-wide Go grep for `tests_needed\|review_needed` returns **zero** hits, and the only `"shard_result"` fact writer is `process_continuation.go:188` | `internal/campaign/orchestrator_task_handlers.go:935` uses `"shard_result"` as a plain map key, not a kernel fact. |
| 25 | S-2d | `ResultToFacts` writes a different vocabulary, truncating at 4000 | `internal/core/shards/manager_spawn.go:650-692` | holds | `:650` `func (sm *ShardManager) ResultToFacts`; `shard_executed` `:655`, `last_shard_execution` `:660`, `shard_error` `:666`, `shard_success` `:671`, `shard_output` with `if len(output) > 4000` at `:675-676`, `recent_shard_context` `:685` | — |
| 26 | 0.5 / 1B | greeting fast path: literal set, hardcoded reply | `process.go:123-140`, `:124`, `:138` | holds | `:124` `if lowerTrimmed == "hi" \|\| ... \|\| lowerTrimmed == "yo" {` (exactly the six listed); `:138` `greetingResp := "Hello! I'm codeNERD..."` | — |
| 27 | 0 / axis 1 | the turn's only bound is the OODA context deadline | `process.go:71-80`, `:76` | holds | `:76` `ctx, cancel := context.WithTimeout(baseCtx, config.GetLLMTimeouts().OODALoopTimeout)` | — |
| 28 | 1 | perception gets ALL history unless compression is active | `process.go:161-167` | holds | `:161` `if m.compressor != nil && m.compressor.IsCompressionActive() {` `:163` `m.getRecentTurns(m.compressor.GetRecentTurnWindow())` `:166` `recentTurns = m.history` | — |
| 29 | 1c / 1B | conversational fast path ends the turn before ORIENT/DECIDE/ACT | `process.go:236-258` | holds | `:236` `if willConverse && intent.Response != "" {` ... `:257` `return responseMsg(...)`; the in-code comment at `:233-234` says "return immediately without going through ORIENT/DECIDE/ACT" | `user_intent` is still asserted, but asynchronously (`:253 go m.fastPathKernelUpdate(intent)`), so it cannot influence this turn. |
| 30 | 2 | `seedIssueFacts` / `seedCampaignFacts` | `process.go:308`, `:312` | holds | `:308` `m.seedIssueFacts(ctx, intent, input)`; `:312` `m.seedCampaignFacts()` | — |
| 31 | 5, 6 | dream and assault interceptions are Go `if`s before routing | `process.go:330-334`, `:338-342` | holds | `:330` `if intent.Verb == "/dream" {` ... `:333` `return m.handleDreamState(...)`; `:338` `if args, ok := assaultArgsFromNaturalLanguage(m.workspace, input, intent); ok {` | Both run before `decideRoute` at `:353`, as claimed. |
| 32 | 7 / S-4 | kernel routing with Go lane precedence and silent RouteLegacy fallbacks | `delegation_routing.go:93-121, 74-76, 124-135, 152-164` | holds | `:93-95` retract; `:97,:105,:114` assert `delegation_candidate`/`multi_step_signal`/`intent_signal`; `:74-76` nil kernel, `:124-127` query error, `:128-135` empty derivation, each `return legacy`; `:152-164` the Go `switch` in precedence order | `:162-163` `default: return legacy` is a fourth fallback (an unrecognised lane) the doc does not mention. |
| 33 | 1b / axis 3 | verb→shard resolution is Go with an `Ambiguity` fallback and a target substring heuristic | `delegation_routing.go:205-227` | holds (incomplete) | `:205` `func resolveShardTypeForIntent`; `:206` `perception.GetShardTypeForVerb`; `:209-216` the `shard=` scan; `:219-225` the substring bank | The doc omits that the substring heuristic is additionally gated on `intent.Confidence >= 0.7` **and** `Verb ∈ {/explain, /explore, /search}` (`:219`) — it is narrower than "a substring heuristic on `intent.Target`" suggests. |
| 34 | S-4 | lane precedence is documented in policy but enforced in Go | `policy/routing_arbitration.mg:20-24`, `delegation_routing.go:152-164` | holds | `routing_arbitration.mg:20-21` `# Lane precedence when multiple derive (Go applies in this order):` / `#   respond_directly > multi_step > delegate > clarify` — a comment, with no rule enforcing it | The policy comment names Go as the enforcer, so the divergence is acknowledged in-tree. |
| 35 | S-3 / 1B | `RouteClarify` is derived and never consumed | `delegation_routing.go:160-161`; `policy/routing_arbitration.mg:107-112` | holds | tree-wide grep for `RouteClarify` returns exactly four hits: `:30-31` (decl+comment), `:43` (`String()` arm), `:161` (assignment), and `routing_arbitration_roundtrip_test.go:148` | Matches the doc's enumeration precisely. |
| 36 | S-1 | four sides of the verdict contract | `executor_memory.go:128-147`, `observed_return.go:39-105`, `task_executor.go:258-261`, `internal/types/shard.go:100-105` | holds | `observed_return.go:39` `func observedReturn(...) observation.Return`; `task_executor.go:258-260` `func (j *JITExecutor) ExecuteWithContext(...) (string, error) { observed, err := j.executeObserved(...); return observed.Output, err }`; `types/shard.go:100-105` `type ShardResult struct { ShardID; Result string; Error error; Timestamp }` | Exact. |
| 37 | S-1 | `TurnOutcome`/`ChangeStage` are never copied into `observation.Return` and never cross into `cmd/nerd/chat` | `internal/session/observed_return.go:39-105` | holds | tree-wide grep for `TurnOutcome\|ChangeStage`: zero hits in `observed_return.go`, zero in `cmd/nerd/chat`; the only consumer outside `internal/session` is `cmd/tools/change_benchmark/main.go:179` | — |
| 38 | 1D S5 | `ChangeStage` ladder, re-verification after later edits, timeout≠failure | `change_evidence.go:19, 40, 54`, `:23-34`, `:42-47` | holds | `:19` `result.ChangeStage = "artifact_changed"`; `:40` `"checks_passed"`; `:54` `"behavior_verified"`; `:23` `VerifyBuildAfterEdits`, `:28` `VerifyTestsAfterEdits`; `:42-44` "Only an affirmative failure fails the turn"; `:45` `if result.BuildCheck.Verdict() == VerifyFailed \|\| result.TestCheck.Verdict() == VerifyFailed` | `closeChangeEvidence` is at `:13-59`, matching the doc's `:13-60`. |
| 39 | 1D S8 | the structured verdict is flattened into `result.Response` | `change_evidence.go:62-107`, `:84`, `:105` | holds | `:84` `result.Response += "\n\n" + result.Acceptance.Summary()`; `:105` `result.Response += fmt.Sprintf("\n\nWrote %d file(s): %s\nEvidence: %s. Requested behavior remains unverified (no acceptance contract).", ...)` — the doc's quote is byte-exact | `evidence.Persist` (the acceptance report of 1E) is at `:77`, as cited. |
| 40 | 1D S5 / S-7 | 5-valued verification, budget rooted at Background | `verify_outcome.go:22-36`, `:84-90`, `:93-142` | holds | `:22-36` the const block with `VerifyPassed/Failed/Skipped/Indeterminate/Canceled`; `:86-87` "The budget context is rooted at Background, not at the caller's context: a model turn running out of time must not manufacture verification failures."; `func runVerificationCommand` at `:93` | — |
| 41 | 1D S4 | the tool loop, and a policy stop returning `task unresolved` | `executor_tools.go:207-405`, `:288` | holds | `:207` `for iter := 0; openRounds \|\| iter < budget.iterationLimit; iter++ {`; `:288` `return currentResponse, toolErrs, fmt.Errorf("task unresolved: working continuation stopped by policy %s after %d executed tools", ...)` | — |
| 42 | axis 4 | tool-history eviction is Go | `executor_tools.go:332`, `:379-381` | holds | `:332` `history = boundToolLoopHistory(history)`; `:379-380` `if activeWorkingLoop(ctx) != nil && len(history) > 4 { history = append([]types.Message(nil), history[len(history)-3:]...) }` | The doc's "recoverable via the archive" is supported by the in-code comment at `:327-328` ("an evicted payload is still in the working-context archive"). |
| 43 | 1D S4b | the Go tool budget controller and its nudge | `executor_tools.go:181`, `:253-260`, `:393-404` | holds | `:181` `budget := newToolBudgetController(executorCfg)`; `:253` `toolResults = appendToolBudgetNudge(toolResults, budget.nudge(...))`; `:393-394` `if !openRounds && iter+1 >= budget.iterationLimit { decision := budget.maybeExtend(writeOriented)` | — |
| 44 | 1D S4b | production boot leaves the round ceiling open, so Mangle is the sole stop | `internal/system/factory.go:2000-2002` | holds (conditional) | `:2000-2002` `execCfg.ProgressDrivenTools = true` / `execCfg.MaxToolCalls = 0` / `execCfg.MaxToolIterations = 0` | Two conditions the doc omits: `factory.go:2004-2009` re-imposes a ceiling if `core_limits.max_tool_calls`/`max_tool_iterations` are set (the live `.nerd/config.json` sets neither, so the claim holds today), and `openRounds` additionally requires an active working loop (`executor_tools.go:182-183`). |
| 45 | 1D S6 / 5C | the turn verdict gate, and dream's exemption | `executor_tools.go:1923-1977`, `:1928-1930` | holds | `func (e *Executor) checkHollowSuccess` at `:1923`, closes at `:1977`; `:1928-1930` `if e.sessionContext != nil && e.sessionContext.DreamMode { return nil }`; `e.assertTurnEvidence(verb, result)` `:1953`; `e.captureTurnOutcome(result, hollowErr)` `:1958` | Unstated in the doc: for a non-write intent the hollow verdict is **recorded but never fails the turn** (`:1959-1968`), so the gate only bites on write-oriented verbs. |
| 46 | 1D header | the shard-side call chain | `task_executor.go:258-261, 273-361, 305-312, 318, 339-343` | holds | `:273` `func (j *JITExecutor) executeObserved`; `:305-307` dream forces `executeWithSubagent`; `:310` `if j.needsSubagent(req.IntentVerb)`; `:318` `exec := j.executor.CloneForTask()`; `:339` `preset := presetIntentForTask(...)`; `:343` `result, err := exec.ProcessWithIntent(ctx, inlineTask, preset)` | — |
| 47 | 1D header | `spawnTaskWithContext` is the chat→session bridge | `cmd/nerd/chat/delegation.go:51-57` | holds | `:51` `func (m *Model) spawnTaskWithContext(...)`; `:56` `return m.taskExecutor.ExecuteWithContext(ctx, shardTypeToTaskRequest(shardType, task), sessionCtx, priority)` | — |
| 48 | 10a / S-6 | `formatShardTask` rebuilds the request from templates | `delegation.go:311-398`, `:362`, `:334-341`, `:327` | holds | `:311` `func formatShardTask(verb, target, constraint, workspace string) string`; `:362` `return fmt.Sprintf("fix issue in %s", target)`; `:334` `review files:%s`, `:337` `"review all"`, `:339` `review file:%s`; `:327` `files := discoverFiles(workspace, constraint)` | Every quoted template string is byte-exact. |
| 49 | 10a | prior shard output is projected structurally, redeemable with `subagent_expand` | `delegation.go:65-124`, `:145-186` | holds | `:65` `func formatShardTaskWithContext`; `:145` `func priorShardContext(prior *ShardResult) string`; `:183-185` `observation.SharedSubagents().EncodeReturn(ret, observation.ReturnLimits{}).Text(toolscore.SubagentExpandToolName)` | The doc's characterisation ("bounded by structure and redeemable") is the file's own comment at `:140-144`. |
| 50 | 2C / 2E | `spawnShard` runs once, prose header, no retry | `delegation.go:605-670`, `:651-657` | holds | `:605` `func (m Model) spawnShard(shardType, task string) tea.Cmd`; `:651` `response := fmt.Sprintf(`## Shard Execution Complete` ...`; `VerifyWithRetry` has exactly one call site in `cmd/` — `process.go:493`, on the free-text path | — |
| 51 | 2D | specialist execution is the one kernel consult on the slash path | `delegation.go:677-725`, `:705-712`, `:715-723` | holds | `:677` `func (m Model) spawnShardWithSpecialists`; `:698` `shards.MatchSpecialistsForTask`; `:705-707` the in-code comment "let specialist_should_execute (policy/shards.mg) pick the high-confidence executor. The Go boolean is the nil-kernel fallback."; `:709-710` `m.assertSpecialistMatches(...)` / `m.specialistThatShouldExecute(...)`; `:715` `mode := shards.GetExecutionMode(verb)` | With zero matched specialists the kernel is never asked at all (`:701-703` falls back to `spawnSimpleShard`). |
| 52 | 2C / S-6 | chat `/fix` builds the target from the whole description and hardcodes `coder` | `commands_handlers_evolution.go:11-37`, `:19-20`, `:31` | holds | `:19` `target := strings.Join(parts[1:], " ")`; `:20` `task := formatShardTask("/fix", target, "", m.workspace)`; `:31` `m.spawnShardWithSpecialists("/fix", "coder", task, target)` | `/refactor` at `:40-66` is the same shape, as claimed. |
| 53 | 10b / axis 5 | verification retry is mutation-only and escalates on a string compare | `process.go:489-527`, `:511`; `delegation_routing.go:181-183` | holds | `process.go:489` `if m.verifier != nil && shouldVerifyDelegation(intent) {`; `:493` `m.verifier.VerifyWithRetry(ctx, task, shardType, m.shardMaxRetries(shardType))`; `:511` `if verifyErr.Error() == "max retries exceeded - escalating to user" {`; `delegation_routing.go:182` `return intent.Category == "/mutation"` | — |
| 54 | 9 | multi-step is kernel-routed with a Go fallback detector | `process.go:422-436`, `:426-427`, `:431` | holds | `:423-427` `switch route.Kind { case RouteMultiStep: isMultiStep = true; case RouteLegacy: isMultiStep = m.detectMultiStepTask(input, intent) }`; `:431` `steps := decomposeTask(input, intent, m.workspace)` | — |
| 55 | 10 / S-6 | the delegate block and its context injection | `process.go:462-648`, `:467-475`, `:482`, `:479` | holds | `:462` `delegateNow := route.Kind == RouteDelegate`; `:463-465` the `RouteLegacy` → `m.shouldDelegate` fallback; `:467` `needsWsScan := m.needsWorkspaceScanForDelegation(intent)` with `loadWorkspaceFacts` at `:472`; `:479` `formatShardTaskWithContext(intent.Verb, intent.Target, ...)`; `:482` `sessionCtx := m.buildSessionContext(ctx)` | Confirms S-6: the free-text caller passes `intent.Target`, the slash caller passes the whole description. |
| 56 | 10c / 10d / S-2d | both fact writers run on the same delegation | `process.go:575`, `:615` | holds | `:575` `facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, spawnErr)`; `:615` `if cont := m.checkContinuation(shardType, task, result); cont != nil {`; `checkContinuation` calls `injectShardResultFacts` at `process_continuation.go:28` | `:597` `m.shardMgr.CheckReviewNeedsValidation(shardID)` confirms row 10d. |
| 57 | 11, 12 | deterministic `/stats` and the fourth direct-answer path | `process.go:651-661`, `:667-685` | holds | `:651` `if shardType == "" && intent.Verb == "/stats" {`; `:667` `if shardType == "" && intent.Response != "" && isConversationalIntent(intent) {` | — |
| 58 | 13 / 1B | autopoiesis auto-runs the clarifier | `process.go:691-723`, `:703` | holds | `:691` `if m.autopoiesis != nil && !answerDirectly {`; `:692` `m.autopoiesis.QuickAnalyze(ctx, input, intent.Target)`; `:703` `if clarifierMsg, err := m.runClarifierShard(ctx, input); err == nil && clarifierMsg != "" {` — and it returns immediately | — |
| 59 | 14, 15 | context loading and state refresh | `process.go:729-733`, `:738-754` | holds | `:729-732` the `!workspaceScanned` → `loadWorkspaceFacts` call; `:739` `UpdateSystemFacts()`, `:743` `m.shardMgr.ToFacts()`, `:747` `HydrateLearnings`, `:750` `HydrateSessionContext` | — |
| 60 | 16, 17, 18 | `next_action`, denied actions, info-gathering, system delegations | `process.go:760-762`, `:786`, `:796`, `:800`, `:809-813` | holds | `:761` `actions, _ = m.kernel.Query("next_action")`; `:786` `if denied, derr := m.kernel.Query("action_denied"); ...`; `:796` `executionResults, mangleUpdates := m.executeInfoGatheringActions(...)`; `:800` `_ = m.kernel.LoadFacts(executionResults)`; `:810` `m.handleSystemDelegations(...)` | — |
| 61 | 19, 20 / 6D | kernel context selection, then an unconditional Go persona append | `process.go:815-819`, `:822-832` | holds | `:818` `contextFacts, _ = m.kernel.Query("context_to_inject")`; `:824` `systemPrompts, _ = m.kernel.Query("final_system_prompt")`; `:832` `systemPrompt += "\n\n" + stevenMoorePersona` with the comment `// Inject the "Steven Moore Flare" persona` at `:831` | Unconditional and after JIT, exactly as claimed. |
| 62 | 21, 22, 23 / 1E | articulation context, feedback keyed to the manifest hash, filtered model facts, compression | `process.go:848-854`, `:882`, `:908-927`, `:910-914`, `:1013`, `:1021-1049`, `:1038`, `:1051-1063`, `:1055` | holds | `:848-853` `ConversationContext{RecentTurns, LastShardResult, TurnNumber, ShardHistory, CompressedCtx}`; `:882` `articulateWithConversation(...)`; `:912` `manifestHash = jitResult.Manifest.ContextHash`; `:1013` `go m.compressor.ProcessTurn(compressCtx, turn)`; `:1038` `facts, blocked := core.FilterMangleUpdates(m.kernel, artOutput.MangleUpdates, core.ModelObservationPolicy())`; `:1055` `Predicate: "self_correction_hypothesis"` | The doc lists `ConversationContext` without `TurnNumber`; otherwise exact. |
| 63 | 1D S2/S2b, 1E, 6D | JIT compile, `nerd.md` outside the budget, file context only without a working world, persistence | `executor.go:1049-1064`, `:1058-1063`, `:1065-1070`, `:1155-1169`, `:1113-1122`, `:1126`, `:1129` | holds | `:1049` `e.compileConfig(ctx, compileResult, intent)`; `:1058-1063` the comment "the atom selector has no way to score a document it has never seen, and budget-driven eviction could silently drop the project's own rules"; `:1064` `systemPrompt := e.withProjectInstructions(compileResult.Prompt)`; `:1068-1069` `if !hasWorkingWorld { systemPrompt = e.withFileContext(...) }`; `:1155` `func (e *Executor) buildCompilationContext`; `:1113-1121` history + thought signatures; `:1126` `queueTaxonomyLearning`; `:1129` `persistTurn` | Every cite lands. |
| 64 | J1 header | `processInput` spans `process.go:48-1090` | `cmd/nerd/chat/process.go:48-1090` | holds | `:48` `func (m Model) processInput(input string) tea.Cmd {`; the closure and function close at `:1089-1090` | — |
| 65 | Entry inventory | bare `nerd` falls through `rootCmd.RunE` to `chat.RunInteractiveChat` | `cmd/nerd/main.go:137-167` | wrong-location | `rootCmd.RunE` begins at `main.go:146` (`RunE: func(cmd *cobra.Command, args []string) error {`) and ends at `:173`; `return chat.RunInteractiveChat(cfg)` is at `:172`. Lines `137-145` are the tail of `PersistentPreRunE` and all of `PersistentPostRun`. | The substance (chat TUI is the default entry) holds; the cited range points at the wrong hooks. |
| 66 | Entry inventory | process-wide boot: config eager-load, file logging, flight recorder, F-TRACE-1 | `cmd/nerd/main.go:355-361`, `:348-351`, `:377-399`, `:327-337` | wrong-location | `logging.Initialize(ws)` is at `:357` (cited `:348-351`, which is `}` / blank / `func main() {` / a comment); `config.GlobalConfig()` is at `:365` (cited `:355-361`); the flight-recorder gate `if features.IsFlightRecorderEnabled() && !isCampaignInvocation() {` is at `:384` (cited `:377-399`, which starts mid-comment); the F-TRACE-1 rationale runs `:330-342` (cited `:327-337`). | Every one of the four is shifted 4-8 lines early — the claims themselves are correct, the line anchors are not. |
| 67 | Entry inventory | subcommand registration ranges | `main.go:265-277`, `:281-290`, `:293-295`, `:226-233` | holds | `:265-277` direct actions (review, fix, test, push, commit, explain, create, refactor, perception, security, analyze — exactly the doc's list); `:281-290` dream/shadow/whatif/logic/agents/tool/jit/dom/embedding; `:293-295` northstar; `:227-234` the campaign subcommands | Two groupings drift: `knowledgeCmd` is at `:320`, not in the cited `:298-311`; `glassboxCmd`/`transparencyCmd`/`reflectionCmd` are at `:325-327`, not the cited `:318-322`. |
| 68 | 3 F1/F2 | `--acceptance` takes the task from the contract and rejects a positional target | `cmd_direct_actions.go:58-67`, `:278`, `:291` | holds | `:58` `func validateFixArgs`; `:61-62` `return fmt.Errorf("--acceptance takes its task from the contract; omit the positional target")`; `:278` `target := strings.Join(args, " ")`; `:291` `target = contract.Task` | — |
| 69 | 3 F3-F8 | the CLI one-shot stages | `cmd_direct_actions.go:279-293, 313, 338, 391, 394, 428`, `:45-56`, `:55` | holds | `:282` `evidence.LoadContract(path)`; `:313` `ctx = evidence.WithContract(ctx, *acceptance)`; `:338` `coresys.GetOrBootCortex`; `:391` `rootBefore := snapshotDirectRoot(wsRoot)`; `:394` `cortex.SpawnTaskWithTarget(ctx, verb, target, target)`; `:428` `checkDirectSpawnResult`; `fixCmd` at `:45-56` with `RunE: runDirectAction("coder", "/fix")` at `:55` | — |
| 70 | 3A | the CLI brief is passed through verbatim as both `Task` and `Target` | `internal/system/factory.go:452-456` | holds | `factory.go:428` `func (c *Cortex) SpawnTaskWithTarget(ctx, shardType string, task, target string)`; `:451-455` `req := session.TaskRequest{IntentVerb: shardType, Task: task, Target: target}` — and `runDirectAction` passes the verb as `shardType` and the brief as both `task` and `target` | — |
| 71 | S-16 | two undeclared-write guards with different coverage | `cmd_direct_actions.go:380-384` | holds | `internal/campaign/orchestrator_tasks.go:687-689` `for _, e := range entries { if !e.IsDir() { seen[e.Name()] = true } }` vs `cmd_direct_actions.go:457-461` which skips only `.nerd`; the in-code comment at `:380-384` states the `research/` miss verbatim | — |
| 72 | 4A | the campaign plan is LLM prose; `recurse` is the exception | `decomposer.go:22`, `:250-252`, `:342-353`, `:446`; `cmd_campaign_recurse.go:18-23` | holds | `decomposer.go:22` "// Decomposer creates campaign plans through LLM + Mangle collaboration."; `:250-252` `if d.llmClient == nil { return nil, fmt.Errorf("%w: decomposer requires llm client", ErrNilDependency) }`; `:342` "// Step 3: LLM proposes phases and tasks"; `:446` "// Step 7: If issues, attempt LLM refinement"; `cmd_campaign_recurse.go:18-20` the deterministic-planning sentence, quoted exactly | — |
| 73 | 4B C1-C16 | the orchestrator run-loop stages | `orchestrator_execution.go:14, 37-59, 87, 90-95, 97-102, 133-148, 168, 171, 178-183, 185-198, 209, 221, 231-247, 250` | holds | every cite lands: `:14` `func (o *Orchestrator) Run`; `:37` `runRiskPreflight`; `:87` `recordRootBaseline()`; `:90` `if o.config.CampaignTimeout > 0`; `:100-102` heartbeat + `watchPauseRequest`; `:133-137` the F-STALL-1 comment quoted verbatim; `:168` `getCurrentPhase()`; `:171` `isCampaignComplete()`; `:182` `EndCampaign`; `:198` `sweepUndeclaredRootWrites()`; `:209` `getCampaignBlockReason()`; `:221` `startNextPhase(ctx)`; `:245` `PrefetchNextTasks(ctx, upcoming, 3)`; `:250` `runPhase` | The `3` in `PrefetchNextTasks` is a literal, as claimed. |
| 74 | C7/C8/C9 / S-10 / S-11 | phase selection and blocking are kernel-derived; completion is a Go loop | `orchestrator_phases.go:13-41, 184-203, 206-222, 247-269, 262-268, 225+` | holds | `:14` `o.kernel.Query("current_phase")`; `:184-203` `isCampaignComplete` is a `for _, phase := range o.campaign.Phases` over `phase.Status` with no kernel call; `:207` `o.kernel.Query("campaign_blocked")`; `:262` `o.kernel.Query("phase_eligible")` then `:268` `phaseID := types.ExtractString(facts[0].Args[0])`; `isPhaseComplete` at `:225` | `getCurrentPhase` actually spans `:13-45`, not `:13-41`; harmless. |
| 75 | C12/C13/C14 / S-15 | task dispatch is a Go switch; `/document` falls back by design | `orchestrator_task_handlers.go:45-115, 68-114, 124, 129-135` | holds | `:44` `func (o *Orchestrator) executeTask`; `:63-64` `task.Shard` branch; `:68-114` the type switch — 14 named arms plus `default`, i.e. the claimed 15; `:124` `buildTaskInputWithSpecialistKnowledge`; `:130-135` the F-DOC-1 comment, quoted accurately | — |
| 76 | C1 / axis 5 | "kernel grades, Go measures" on the risk gate | `risk_scoring.go:242`, `:233+`, `:248-262` | holds | `:233` `func (o *Orchestrator) runRiskPreflight`; `:242` `// Everything Go observes goes here; the kernel grades it at the end.` — byte-exact; `:248-262` the advisory-board hard block with `severity: riskConcernBlocking` | — |
| 77 | 4C | recurse bound, stall fuse, angle rotation, wave linkage | `cmd_campaign_recurse.go:93-98, 26-29, 64, 73-88, 50-51, 103-110, 112-200` | holds | `:94-95` `if cfg.MaxWaves == 0 && !yolo { return fmt.Errorf("recurse: unbounded run (--waves 0) requires yolo mode ...") }`; `:27-28` the stall-fuse sentence; `:64` `--angles` help text `"(max 2: harden,wire,review,test,bench,secure); default rotates"`; `:73` `func resolveRecurseConfig`; `:50-51` "linked by a shared recurse ID"; `:103` `func recurseWaveConfig` with the fresh-observer rationale at `:100-102`; `:112` `func runCampaignRecurse` | `--stall-waves` defaults to `0` on the flag with help text "(default 2)"; the 2 is applied in `cfg.Normalize()`. |
| 78 | 5A | system shards are fact-triggered with a heartbeat | `executive.go:464, 467, 470-479, 491, 497, 505-519` | holds | `:464` `SubscribeToFacts([]string{"user_intent","next_action","delegate_task","tdd_next_action","campaign_next_action","repair_next_action"})` — exactly the six listed; `:465-467` the 15 s comment and ticker; `:472` `if factCh == nil {`; `:491`/`:497` `e.evaluatePolicy(ctx)`; `:511` `e.Autopoiesis.ShouldPropose()` with `:515` `context.WithTimeout(ctx, 3*time.Minute)` | — |
| 79 | 5A / 5D | the twin shard loops and their periods | `perception.go:433`, `router.go:229`, `planner.go:196`, `world_model.go:224`, `constitution.go:307`, `campaign_runner.go:140` | holds | exact: 15 s at `constitution.go:307`, `executive.go:467`, `perception.go:433`, `router.go:229`; 10 s at `planner.go:196`, `world_model.go:224`; `campaign_runner.go:140` is `time.NewTicker(s.config.TickInterval)` | Two caveats: `perception.go` has **no** fallback ticker (so "same files" in 5D overstates), and `campaign_runner.go:140` is a plain interval ticker, not the heartbeat+fallback shape of the other five. |
| 80 | 5B O1-O5 / S-12 | the observer loop, tick suppression and the default-50 verdict | `observer_manager.go:154, 350-357, 358-379, 381-386, 389-456, 413, 440, 459-490, 492-542, 496-497, 508-516` | holds | `:154` `checkInterval: 5 * time.Minute`; `:353-357` the measured-pathology comment quoted verbatim ("each an LLM call of 13-43 s, each scoring the session 10-50/100 for having nothing to show"); `:361` `time.NewTicker(m.checkInterval)`; `:385` `return atomic.SwapInt64(&m.eventsSinceCheck, 0) > 0`; `:413`/`:440` `context.WithTimeout(runCtx, 2*time.Minute)`; `:459` `buildAssessmentTask` (a `strings.Builder` prose prompt, no JIT); `:496-497` `Score: 50, // Default neutral score` / `Level: LevelNote`; `:508` `if strings.HasPrefix(line, "SCORE:")` | Every line number in this block is exact. |
| 81 | 5C / S-13 | dream is Go-orchestrated and stops on a character count | `process_dream.go:102, 142, 148, 200-206, 214, 235, 269-270, 273, 313` | holds | `:142` "Skipping internal shard"; `:148` "Skipping irrelevant specialist"; `:214` `consultPromptTemplate := \`DREAM STATE CONSULTATION - DO NOT EXECUTE ANYTHING`; `:270` `DreamMode: true`; `:273` `types.PriorityLow`; `:313` `"⚡ Early stopping: specialist(s) provided confident answer (%d chars)..."` | Confirmed stronger: `:245` `specialistResponseQuality := 0 // Sum of response lengths from specialists`, `:290` `+= len(result)`, `:300` the gate `specialistResponseQuality > 1000`. The metric is the summed character count of specialist replies. |
| 82 | 5D | complete ticker inventory | `reflection_worker.go:88,428`, `mangle/engine.go:191`, `ondemand_watcher.go:52`, `mangle_watcher.go:134`, `factory.go:621`, `flight_recorder.go:169`, `persistent_docker.go:223`, `orchestrator_execution.go:288-289` | holds | all eight verified line-for-line: `reflectionWorkerInterval` ×2; `time.NewTicker(30 * time.Second)`; `time.NewTicker(onDemandFallbackSweep)`; `time.NewTicker(100 * time.Millisecond)`; `time.NewTicker(interval)` ×2; `e.healthTicker = time.NewTicker(e.config.HealthCheckInterval)`; `:288-289` heartbeat + autosave tickers | — |
| 83 | 6A W1-W7 | the scan pipeline and its validation gate | `cmd_init_scan.go:414-416, 423-429, 431-440, 442-448, 450-462, 464-469, 561` | holds | `:414` `if !nerdinit.IsInitialized(cwd)`; `:423`/`:426` `world.NewScanner()` / `ScanWorkspaceCtx`; `:431-433` the validation-gate comment quoted verbatim; `:443-447` profile.mg reload with a `⚠️ Warning` (not an error); `:456` `world.PersistFastSnapshotToDB(db, facts)`; `:466` `writeScanFacts(scanPath, facts)`; `:561` `"# Auto-generated scan facts - DO NOT EDIT\n"` | The label `:395-511 runScanWithKernelFactory` is loose — `:395` is `func runScan`, which delegates; the named function begins a few lines later but lies inside the range. |
| 84 | 6B | predicate ownership and replacement semantics | `./nerd.exe world predicates` | holds | the same table is source: `internal/world/world_predicates.go:40-49` (scanner, 8 predicates in the doc's order), `:55-68` (deep, all 12), `:73-78` (lsp, 4), `:83-87` (session scope, 3), `:90-93` (git, 2); the "a scan must not delete these" wording is the file's own at `:71-72` | Verified without running the binary. |
| 85 | 6B | lsp predicates are "projected by language servers" | `./nerd.exe world predicates` | overstated | `world_predicates.go:69-72` "No production caller constructs that manager today, so nothing writes these predicates yet; the reservation still matters because a scan cannot re-derive them" | Nothing writes `symbol_defined`/`symbol_referenced`/`code_diagnostic`/`symbol_completion` in production. The row reads as a live projection; it is a reservation. |
| 86 | 2B categories | the five-category registry and its per-command line numbers | `command_categories.go:38-548`, `:10-16` | holds | `CommandCategory` consts at `:10-16`; `var CommandRegistry = []CommandInfo{` at `:38`, closing `}` at `:548`; spot checks: `Name: "/help"` at `:43`, `Name: "/reset"` at `:542` | The category table enumerates 69 commands and the registry has exactly 69 entries. |
| 87 | S-5 | "the registry has 71 entries" | `command_categories.go:38-548` | wrong-claim | `grep -c '^\t{$'` over `command_categories.go` returns **69**, and `grep -c 'Name:'` also returns 69 | The doc's own §2B category table lists 69 (9+18+21+12+9), so the "71" in S-5 contradicts the document's own table. |
| 88 | 2B / S-5 | `/explain`, `/explain-off`, `/shards`, `/autopoiesis`, `/facts` "are dispatched in `commands.go` / tested but are not in `CommandRegistry`" | `commands.go:189-192`, `commands_test.go:452, 465`, `live_kernel_test.go:84` | wrong-claim | `/explain` (`commands.go:189`) and `/explain-off` (`:191`) are dispatched and absent from the registry — that half holds. But `/shards`, `/autopoiesis` and `/facts` have **no `case` anywhere** in `cmd/nerd/chat` (non-test): they fall to `commands.go:256` `default:` → `"Unknown command: %s. Type /help for available commands."` | The tests at `commands_test.go:452, 465` and `live_kernel_test.go:84` do call `handleCommand("/shards")` etc., but they are exercising the unknown-command path, not a handler. The seam is worse than described: three commands exist only in tests. Separately, `/recurse` **is** in the registry (`:281`), so listing it among registry absences is also wrong. |
| 89 | 2B | `/dream`, `/remember`, `/delegate*` are not slash commands | `process.go:330-334`, `internal/prompt/config_factory.go:510`, `internal/core/delegation_contract_test.go:23-36` | holds | no `case "/dream"` or `case "/remember"` in `commands.go` and no registry entry for either; `config_factory.go:510` `"/converse", "/forget", "/remember",`; `delegation_contract_test.go:23-36` is the `(verb, category, shard, action, toolGated)` table pinning `/delegate_coder`, `/delegate_tester`, `/delegate_reviewer` | — |
| 90 | 8, 8b, 8c / 1B | three clarification paths, two of them Go heuristics | `process.go:369-389`, `:392-399`, `:402-416`; `process_dream_delegation.go:23-44` | holds (incomplete) | `:369` `if !answerDirectly && m.shouldAutoClarify(&intent, input) {`; `:392` `if question, options, ok := m.shouldClarifyFromKernel(&intent, input); ok && !answerDirectly {`; `:402` `if !answerDirectly && m.shouldClarifyIntent(&intent, input) {`; `process_dream_delegation.go:23` `func (m Model) shouldAutoClarify`, the substring bank at `:31-37`, the category gate at `:41` | The doc describes `shouldAutoClarify` as the substring bank ANDed with the category. It is actually `isBuildish && (looksLikeCampaign \|\| needsDetails)` (`:44`), and `needsDetails` fires on an empty `Target` or `Constraint` — so the clarifier can trigger with none of the seven words present. All three paths are gated on `!answerDirectly`, which is kernel-derived, so the kernel does veto them. |
| 91 | axis 4 | working-set entity selection is kernel-owned | `internal/context/working_set.go:224-232` | wrong-location | `func (w *WorkingSet) Select` is at `working_set.go:250`. Lines `224-232` are a doc comment that begins "// Select refreshes a bounded dependency slice..." but sits above `func (w *WorkingSet) TranscriptRounds` at `:233` — a mis-attached comment in the source. | The claim itself holds: `Select` queries `working_selected(ID, Priority)` at `:344` and `should_include_context(Entity, Priority)` at `:366`. Only the line anchor is wrong, and the source's own stray comment is what misleads. |
| 92 | 1D S3 | the working loop is opened in Go, scoped `sessionID/shardID/scope` | `working_context.go:101-126`, `:116-122`, `:73-82` | holds | `:101` `func (e *Executor) beginWorkingLoop`; `:116` `scope := sessionID + "/" + cc.ShardID + "/" + scopeID`; `:117` `working.NewWorkingSet(world, root, scope)`; `:73-82` `commitRegimeDefinitions` filtering out `closedForReading(def.Name)` | — |
| 93 | S-17 | strict decoding, and `UIConfig` unreachable | `internal/config/user_config.go` | holds | `user_config.go:536` `if err := decodeStrictJSON(data, cfg); err != nil {`; `internal/config/ux.go:7,10` declare `split_pane_ratio` / `logic_pane_width` on `UIConfig`, and `UIConfig` appears **nowhere** in `user_config.go` — there is no key to set | — |
| 94 | Status (open) | the "open" items are real files at the stated sizes | `executor_tools.go` (2723 lines), `cmd_instruction.go`, `chat/shadow.go`, `multistep_decomposer.go`, `work_steps.go` | holds | `wc -l`: `internal/session/executor_tools.go` = **2723** (exactly as claimed), `cmd/nerd/cmd_instruction.go` = 930, `cmd/nerd/chat/shadow.go` = 300, `cmd/nerd/chat/multistep_decomposer.go` = 830, `internal/session/work_steps.go` = 436; `verifyCompletedToolTurn` at `executor_tools.go:451` | The document's self-declared gaps are accurate. |
| 95 | 3B conclusion | model-asserted `turn_*` facts cannot forge completion | — (not claimed) | holds (strengthens) | `internal/core/mangle_updates.go:145-149` `// These are host witnesses and conclusions, never model observations. Even a permissive caller allowlist cannot delegate their authority.` then `case "turn_acceptance", "turn_evidence", "turn_executed", "turn_done", "turn_cost": return false` | Not in the document; it closes the obvious loophole in §3B and should be cited there. |

### Refuted claims

Seven of ninety-five. Five are bad line anchors or arithmetic; two change what a reader
would conclude.

1. **(#88, §2B bullet + S-5) `/shards`, `/autopoiesis` and `/facts` are not dispatched.**
   The document says they "are dispatched in `commands.go` / tested but are not in
   `CommandRegistry`". `/explain` (`commands.go:189`) and `/explain-off` (`:191`) are.
   The other three have no `case` anywhere in non-test `cmd/nerd/chat`: typing `/shards`
   reaches `commands.go:256` `default:` and prints `Unknown command: /shards`. The tests
   at `commands_test.go:452, 465` and `live_kernel_test.go:84` assert against the
   unknown-command path. **Correction:** three commands exist only as test call sites —
   neither registered nor dispatched. That is a worse seam than S-5 describes, not a
   milder one. (Also: `/recurse` *is* in the registry at `:281`, so it does not belong in
   a list of registry absences.)

2. **(#19, §6C bullet + axis-4 row) "The Go selector queries only `selected_result/3`" is
   literally false.** `internal/prompt/selector.go` issues three kernel queries in the
   skeleton phase: `blocked_by_context(Atom)` (`:977`), `mandatory_selection(Atom)`
   (`:985`) and `selected_result(Atom, Priority, Source)` (`:997`, again at `:1232`).
   **Correction:** only `selected_result/3` *drives selection*; the other two are labelled
   "Debug:" in the source and their results are only logged. The substantive point — the
   kernel vetoes, Go plus vectors admit — survives. The document quoted
   `jit_selection.mg:18` as a finding rather than checking it.

3. **(#85, §6B table) The `lsp` scope is a reservation, not a live projection.** The row
   reads "projected by language servers". `internal/world/world_predicates.go:69-72`:
   "No production caller constructs that manager today, so nothing writes these predicates
   yet". **Correction:** `symbol_defined`, `symbol_referenced`, `code_diagnostic` and
   `symbol_completion` have no writer in production; the scope exists so a scan cannot
   delete what it cannot re-derive.

4. **(#87, S-5) The registry has 69 entries, not 71.** `grep -c '^\t{$'` and
   `grep -c 'Name:'` over `command_categories.go` both return 69, and the document's own
   §2B category table enumerates 69 (9 + 18 + 21 + 12 + 9). **Correction:** 69.

5. **(#91, axis-4 row) `WorkingSet.Select` is at `internal/context/working_set.go:250`,
   not `:224-232`.** Lines `224-232` are a doc comment beginning "// Select refreshes a
   bounded dependency slice…" that sits above `func (w *WorkingSet) TranscriptRounds` at
   `:233` — a mis-attached comment in the source. **Correction:** the claim holds at
   `:250`, where `Select` queries `working_selected(ID, Priority)` (`:344`) and
   `should_include_context(Entity, Priority)` (`:366`).

6. **(#65, entry inventory) `rootCmd.RunE` is `cmd/nerd/main.go:146-173`, not `:137-167`.**
   `return chat.RunInteractiveChat(cfg)` is at `:172`; `:137-145` is the tail of
   `PersistentPreRunE` and all of `PersistentPostRun`. **Correction:** `:146-173`.

7. **(#66, entry inventory) All four process-wide boot anchors are 4-8 lines early.**
   `logging.Initialize(ws)` is at `:357` (cited `:348-351`); `config.GlobalConfig()` at
   `:365` (cited `:355-361`); the flight-recorder gate at `:384` (cited `:377-399`); the
   F-TRACE-1 rationale at `:330-342` (cited `:327-337`). **Correction:** the claims are
   right, the anchors are not.

Two further rows are marked "holds (incomplete)" rather than refuted, because the claim is
true but the stated mechanism is narrower or wider than the code: #33 (the verb→shard
substring heuristic is additionally gated on `Confidence >= 0.7` and
`Verb ∈ {/explain, /explore, /search}`) and #90 (`shouldAutoClarify` is
`isBuildish && (looksLikeCampaign || needsDetails)`, so it can fire with none of the seven
words present). #44 is "holds (conditional)": the open tool ceiling depends on
`core_limits` leaving `max_tool_calls`/`max_tool_iterations` unset, which the live
`.nerd/config.json` does.

### Premises of the five questions

The document has **no explicit "questions for the architect" section** — it ends at
§Summary with two structural notes. The five premises named in the verification brief are
the load-bearing claims those questions rest on, and each was checked end to end.

1. **`turn_acceptance` is asserted only from `--acceptance` on `nerd fix`, with a single
   production call site — HOLDS, and is stronger than stated.**
   `coder_safety.mg:115` `turn_done(Verb) :- turn_executed(Verb), turn_acceptance(Verb, _, _).`
   → `executor.go:2334` asserts `turn_acceptance` only when
   `result.Acceptance != nil && result.Acceptance.Status == "verified"` →
   `executor.go:877-882` builds `result.Acceptance` only from `evidence.ContractFromContext`
   → `evidence.WithContract` has exactly two call sites, `cmd/nerd/cmd_direct_actions.go:313`
   (production) and `cmd/tools/change_benchmark/main.go:173` (a benchmark) →
   `--acceptance` is registered on `fixCmd` alone (`main.go:200`) and `cmd_direct_actions.go:280`
   guards on `Lookup("acceptance") != nil`, so the other verbs sharing `runDirectAction`
   cannot supply one. **Add this:** `internal/core/mangle_updates.go:145-149` hard-blocks
   `turn_acceptance`, `turn_evidence`, `turn_executed`, `turn_done` and `turn_cost` from
   model-asserted updates — "never model observations. Even a permissive caller allowlist
   cannot delegate their authority." The model cannot forge completion, so the single flag
   really is the only door.

2. **The slash-command surface bypasses `route_decision` and `next_action` — HOLDS.**
   `model_handlers.go:139-141` routes every `/`-prefixed line to `handleCommand` before
   `processInput` is reachable; `processInput` has exactly three callers
   (`model_handlers.go:217, 498, 532`), none in `commands.go`. `decideRoute` is called only
   at `process.go:353`, `next_action` queried only at `:761`, `context_to_inject` at `:818`,
   `final_system_prompt` at `:824` — all inside `processInput`. The one kernel consult on
   the slash path is `specialist_should_execute` (`delegation.go:705-712`), and it is
   skipped entirely when the Go matcher returns zero specialists (`:701-703`).

3. **`working_set.mg` runs in a second engine under `internal/context/` — HOLDS.**
   `working_set.go:24` `//go:embed working_set.mg`; `:55-61` constructs its own
   `mangle.NewEngine` and loads `schemas + policy + workingSetPolicy` into it; `:36` the
   `engine *mangle.Engine` field. `internal/core/defaults/` contains **no** reference to
   `working_stop`, `working_finalize`, `working_nudge`, `working_regime` or
   `working_continue`. **One correction to the framing:** the second engine is not isolated
   from the kernel corpus — `working_set.mg:1-2` says it is "Loaded beside the canonical
   context_compilation.mg in a private evaluation scope", so it carries core context rules
   too. It is a second *instance* with an extra policy file, not a disjoint rule set.

4. **The `jit_selection.mg:13-20` 99.2 % note — HOLDS as a quotation; the "only" claim is
   overstated (see refutation 2).** `:16` reads "to 254 atoms and 65036 of 65536 tokens,
   99.2 percent budget saturation", `:15` gives the 67-atom / 26 279-token baseline, and
   `:20` "may veto, never admit" — all byte-exact. The veto half is live:
   `internal/core/defaults/jit_compiler.mg:261-273` is the "Policy veto bridge (restrictive
   direction only)" with `prohibited(Atom) :- prohibited_atom(Atom).` and
   `suppressed(Atom) :- conflict_loser(Atom).` Note the path — `jit_compiler.mg` is at
   `internal/core/defaults/`, not under `policy/`.

5. **The coder shard's status is guessed from the absence of the word "test" — HOLDS.**
   `process_continuation.go:182-184`
   `if shardType == "coder" && !strings.Contains(strings.ToLower(result), "test") { status = "/code_generated" }`
   is the **only** producer of `/code_generated` in the tree, and
   `codedom_continuation.mg:7-9` derives `has_pending_subtask(TaskID, Description, /tester)`
   from it. The fact carries `truncateSummary(result, 200)` (`:194`). The two sibling rules
   at `codedom_continuation.mg:21-26` are provably inert: a tree-wide Go grep for
   `tests_needed`/`review_needed` returns zero hits, and the only writer of `shard_result`
   is `process_continuation.go:188`. **One anchor correction:** the reviewer/`pending_review`
   guess that S-2 cites at `:212` is at `:214`.

### Post-verification correction (S14 study, 2026-09-18 06:30)

Stage 20 above (row 146) and verification row 61 say the chat turn's system prompt is the
kernel's `final_system_prompt` (JIT) with the persona appended. The query at `process.go:824`
is real, but **`final_system_prompt` has no producer anywhere in the repo**: it is listed in
`internal/core/defaults/testdata/query_only_predicates.txt:22` (the repo's own register of
predicates Go queries and nothing derives; gate `TestGoQueriedPredicateBudget` green), its only
other appearance is the `Decl` at `schemas_reviewer.mg:144`, and the compiled base prompt is
empirically 0 bytes — the persona constant was the entire main-chat system prompt.
`context_to_inject` (`process.go:818`) has no producer either. The verifier checked that the
predicate is *queried*, not that it is *produced*; the correct claim is: **the interactive chat
turn compiles no JIT prompt at all — no atoms, no skeleton, no ordering, no budget** — and that
is seam S17 in the program of record. Evidence and tests: `Docs/journeys/impl/S14-persona-atom.md`.

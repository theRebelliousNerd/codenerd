# codeNERD composition map — as built vs as designed (2026-09-03)

Produced by a read-only sweep of the vision docs and entry points, then
verified at the cited lines. It is the plan behind the composition campaign
(`campaign_compose.md`) and the reference for "where does the model do the
harness's job".

## Intended composition (turn)

1. Perception owns "what did the human mean" → `user_intent/5`
   (`internal/session/executor.go:695-704`); the verb vocabulary comes from
   the Mangle taxonomy (`internal/perception/transducer.go:72-95`).
2. Kernel owns "who acts, what next" → `persona/1`, `intent_action_type/1`,
   `next_action/1` (`policy/intent_routing_rules.mg`, `system_ooda.mg`).
3. Prompt JIT owns "what the model is told" → `CompilationContext` →
   `CompilationResult.Prompt` (`executor.go:838-965`, `internal/prompt/compiler.go`).
4. ConfigFactory owns "what the model may touch" →
   `EffectiveAgentRuntimeConfig` (`internal/prompt/config_factory.go:95`).
5. Session executor owns the bounded turn (`internal/session/executor_tools.go:47`).
6. Constitution owns "may this exact call happen": executor asserts
   `pending_action/5`, queries `permitted/3`, retracts
   (`executor_tools.go:2310-2341`; default deny at `constitution.mg:7-11`).
7. Dreamer + validators own simulate-before / verify-after
   (`internal/core/virtual_store_interactive_gate.go:118,162`).
8. Post-edit gates own build / test / critic (`internal/session/build_verify.go`).
9. Articulation owns surface text + control packet (`executor.go:1598`).

Campaign: decomposer → phases → `eligible_task`/`next_campaign_task`
(`internal/campaign/orchestrator_phases.go:75,159`) → `TaskExecutor.Execute`
with preset intent → `CheckpointRunner.Run` (`checkpoint.go:48`) →
`completePhase` → rolling-wave replan (`orchestrator_tasks.go:279`).

## Contracts and orphans

| Contract | Producer | Consumer | Status |
|---|---|---|---|
| `user_intent/5` | executor, chat, perception firewall | routing rules, executive shard | healthy |
| `pending_action/5` | executor **and** executive shard (`executive.go:604`) | constitution rule **and** ConstitutionGateShard (`constitution.go:307`) | **shared predicate, two pipelines** |
| `permitted/3` | `constitution.mg` | `executor_tools.go:2341` | the live gate |
| `permitted_action/5`, `permission_check_result/4` | ConstitutionGateShard | TactileRouter (`router.go:302`) | second pipeline |
| `next_action/1` | policy, `tdd_loop.go:355`, campaign | executive shard, chat (`process.go:761`) | **never read by the session executor** |
| `route_action/2` | policy | `router.go:573` | thin; a ~120-entry Go table is the real router |
| `modular_tool_allowed/2` | `intent_routing_rules.mg:277` | none | orphan |
| `test_framework/1` | `intent_routing_rules.mg:154-165` | none | orphan |
| `campaign_complete/1` | `campaign_core.mg:13,17` | none in Go | orphan |
| `unverified_test_claim`, `missing_test_for` | executor asserts evidence, Mangle decides | `checkHollowSuccess` | **the reference pattern: Go measures, Mangle decides** |

## Composition breaks

a. Two action pipelines share `pending_action/5`: the executor holds it for a
   synchronous `permitted/3` query while `ConstitutionGateShard` polls it,
   re-decides in Go (`constitution.go:442`), asserts `permitted_action/5`,
   retracts the fact (`constitution.go:388`), and `TactileRouterShard` can
   execute the tool itself (`router.go:302,440`). Both are auto-start shards.
b. `next_action/1` is never consulted by the session executor; the model's
   `tool_calls` are the decision and Mangle is only a veto
   (`virtual_store_interactive_gate.go:13-27` says so deliberately).
c. `apply_edits` is a first-class write tool with no `safe_action`, unmapped in
   the Dreamer/validator table, yet steered to by the budget nudge.
d. Tool grants live in Go (`config_factory.go:226-330`), `safe_action/1` is a
   second hand list, `modular_tool_allowed/2` is unread — three lists drift
   (`constitution.mg:57-62` records an outage caused by exactly this).
e. Prose decides campaign completion: checkpoint parses PASS/FAIL from the
   reviewer's first line (`checkpoint.go:262-315`); `campaign_complete/1` unread.
f. Project shape is detected four times (`checkpoint.go:378,405`,
   `build_verify.go`, `test_framework/1`).
g. Prompts outside the atom library on the hottest paths: repair prompts
   (`build_verify.go:338,358`), budget nudges (`tool_budget_controller.go:277-302`),
   checkpoint review prompt (`checkpoint.go:241-261`).
h. Two prompt-assembly front doors with different defaults (`executor.go:862-882`
   documents the vector-selection outage this caused).
i. `getEligibleTasks` silently falls back from Mangle to an in-memory walk
   (`orchestrator_phases.go:97-132`).

## What the harness knows but asks the model to re-derive

The intent verb in chat (full perception call per turn; CLI verbs already use
preset intents); file structure (holographic section only for target-bearing
turns, on a possibly stale scan — `nerd fix/create/review` never rescan); the
tool budget (exact integers rendered as English each round); the build/test
command; the denial reason (a paragraph instead of removing the tool); the
files this turn already wrote; prior turns (prose transcript, not atoms).

## Memory

Persists: `learned.mg`, `knowledge.db`, session JSON, LocalDB `session_turns`,
cached world facts, prompt corpora. Read back: boot reads world facts and
`learned.mg`; everything else is chat-only (`HydrateLearnings`,
`HydrateSessionContext` at `process.go:750-754`; compressor and PromptEvolver
chat-only). `session.Executor` — the path for `nerd run/fix/create/review`,
every SubAgent and every campaign task — writes memory (`persistTurn`,
`executor.go:1544`) and never reads it; its `atomsJSON` column is hardcoded
empty (`executor.go:1579`).

## Ranked seams (model → harness)

S1 one capability table in Mangle (`tool_capability/5`) projected into
AllowedTools, `safe_action`, routes, Dreamer map. S2 collapse or namespace the
two action pipelines. S3 project shape derived once (`test_framework/1`,
`build_command/2`) and injected. S4 structural budget enforcement (shrink
`toolDefs` as budget drains; `forceFinalAnswer` already proves it). S5 free the
file facts (scan on every boot; holographic section for every touched file).
S6 memory on the live path (`HydrateLearnings`/`HydrateSessionContext` in
`ProcessWithIntent`; persist `atomsJSON`). S7 verdicts as facts
(`checkpoint_verdict/4` → `phase_complete/1` → `campaign_complete/1`). S8 one
`CompilationContext` builder. S9 intent carried, not re-derived, for
unambiguous chat surfaces. S10 denial as capability removal
(`denied_this_turn/2`). S11 generalize "Go measures, Mangle decides"
(`turn_evidence` → `hollow_success/1`). S12 repair prompts as atoms.

Prerequisite for measuring any of it: a `turn_cost(SessionID, TurnNum,
PromptTokens, CompletionTokens, ToolCalls, VerifiedOutcome)` fact written by
`persistTurn` — the denominator of tokens per verified work.

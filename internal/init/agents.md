# Init package guidance

- Treat `.nerd/.gitignore`, `.nerd/config.json`, `mangle/extensions.mg`, `mangle/policy_overrides.mg` and `session.json` as user-owned after first creation: seed atomically, never replace (also under `--force`). The seeded `config.json` names no provider, model, engine, embedding provider or provider-tuned window.
- Boot `RealKernel` with the resolved workspace before it loads Mangle files.
- Required artifact or structural-validation failures belong in `InitResult.Failures` and make `Success` false. Optional research or LLM enrichment belongs in `Warnings` and must remain visibly degraded.
- Every artifact init writes needs a production reader; delete what nothing reads. Never write filler (static boilerplate atoms, "unknown", "no vision"), never turn a reply that is not the requested structure into knowledge.
- Every `NewInitializer` caller that runs `Initialize`, or any other method, must call `Close`.

# LLM routing

- Route every init model request through `withJITPrompt(ctx, initCall, handler)`; the handler receives the routed client. It records one outcome per provider attempt per serving client (`InitLLMMetrics.Clients`).
- `slotPlanner` (`InitConfig.PlannerClient`, else `LLMClient`): document roles, north-star draft, domain-expert proposal, strategic-knowledge synthesis, specialist prompts, project tool selection. `slotMain` (`LLMClient`): per-chunk document extraction, grounded domain research. Context7 topic research is not a model call.
- Compile each prompt for the client that reads it: provider/model from the client chain (`clientIdentity`), budget capped by `broker.InputWindow`; northstar phases via `northstar.PhasePromptFor`.
- `InitConfig.Timeout` bounds one request; zero (what every caller passes) reads `llm_timeouts.per_call_timeout` when the request is made; non-positive configured value adds no clock. No pass or run deadlines.
- No model or provider literal in this package or its entry points: clients come from `.nerd/config.json` (CLI: `configureInitLLM`; chat: session client + `Cortex.PlannerLLM()`).

# Phases and incrementality

- Orientation runs first and its engine is retained; later phases consume its measurements. Unchanged documents keep their role claims; a stored north star is not redrafted while every oriented document is unchanged.
- The roster is Mangle-derived (`orient/ecosystem_agents.mg`, `orient/vision_agents.mg`): stack catalog, planner-proposed domain experts (cited to the north star or an oriented document, bounded by `orient_param`), core pair kept on evidence (north star, tests, security surface), imported subagents bounded by recency. Names folding to the same letters are one agent.
- Re-init redoes only what changed: domain-expert proposal and project tools reuse their records while their request is unchanged; covered Context7 topics and fresh domain answers are not re-researched; fast-moving answers past `orient_research_stale_days` are; strategic knowledge is replaced, not appended; a topic two agents share is fetched once per run.
- Agent tool grants are limited to the tools phase 10 registers; `agents.json` keeps agents registered elsewhere unless declined or their KB is gone.
- Init writes no human output unless `InitConfig.Output` is set; the chat reads `ProgressChan` (phase number and elapsed time are measured; there is no ETA).
- Never start system shards or reconfigure a caller's `ShardManager`; only define created agents' profiles on it.

# Initialization contracts

- `/init --force` preserves curated `.nerd/agents/*/prompts.yaml` files; a missing one is generated on every run. Invalid existing atoms produce a warning naming the file.
- The initializer owns one cached JIT compiler backed by its Mangle kernel. Each compilation uses an isolated scope; close the compiler with the initializer. Its adapter and scopes expose QueryAll against their owned kernel with detached fact containers; keep this aligned with the system and north-star adapters.
- Disk-discovered experts need the unified knowledge schema before prompt synchronization. Empty knowledge remains explicitly empty.
- Ecosystem atoms reach `corpus.db` through the shared hot SQLite pragma profile and that same handle's AtomLoader; knowledge.db holds no prompt atoms.
- No-documentation and ungrounded answers are failures, never knowledge atoms; domain research atoms carry `Sources:` and `Retrieved:`. Prompt synchronization errors must reach the initialization result.
- Validate with focused generation, kernel, discovery and prompt-reload regressions; root owns the complete repository integration gate after concurrent authors finish.

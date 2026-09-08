# Init package guidance

- Treat `.nerd/.gitignore`, `mangle/extensions.mg`, and `mangle/policy_overrides.mg` as user-owned after first creation. Seed them atomically and never replace them during force init.
- Boot `RealKernel` with the resolved workspace before it loads Mangle files.
- Required artifact or structural-validation failures belong in `InitResult.Failures` and make `Success` false. Optional research or LLM enrichment belongs in `Warnings` and must remain visibly degraded.
- Route every init LLM attempt through `withJITPrompt` so provider outcome metrics remain complete and race-safe.
- Legacy `QualityScore` fields measure atom-count population only; never present them as semantic or LLM quality.
- Honor `InitConfig.Timeout` for library callers and check cancellation between phases.
- Every `NewInitializer` caller that runs `Initialize` must call `Close`, including long-lived chat handlers.

# Initialization contracts

- `/init --force` preserves curated `.nerd/agents/*/prompts.yaml` files even when their knowledge database is absent. Invalid existing atoms produce an actionable error.
- Specialist generation uses the configured provider deadline, bounded by the caller context. Keep task metadata separate from JIT system guidance and count actual provider calls, including failures.
- The initializer owns one cached JIT compiler backed by its Mangle kernel. Each compilation uses an isolated scope; close the compiler with the initializer.
- Disk-discovered experts need the unified knowledge schema before prompt synchronization. Empty knowledge remains explicitly empty; do not create synthetic research to satisfy population counts.
- No-documentation responses are failures, never knowledge atoms. Prompt synchronization errors must reach the initialization result.
- Validate with focused generation, kernel, discovery and prompt-reload regressions; root owns the complete repository integration gate after concurrent authors finish.

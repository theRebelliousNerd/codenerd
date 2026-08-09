# Init package guidance

- Treat `.nerd/.gitignore`, `mangle/extensions.mg`, and `mangle/policy_overrides.mg` as user-owned after first creation. Seed them atomically and never replace them during force init.
- Boot `RealKernel` with the resolved workspace before it loads Mangle files.
- Required artifact or structural-validation failures belong in `InitResult.Failures` and make `Success` false. Optional research or LLM enrichment belongs in `Warnings` and must remain visibly degraded.
- Route every init LLM attempt through `withJITPrompt` so provider outcome metrics remain complete and race-safe.
- Legacy `QualityScore` fields measure atom-count population only; never present them as semantic or LLM quality.
- Honor `InitConfig.Timeout` for library callers and check cancellation between phases.
- Every `NewInitializer` caller that runs `Initialize` must call `Close`, including long-lived chat handlers.

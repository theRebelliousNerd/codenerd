# session — Open Questions

> Last verified: 2026-07-13

## Q1 — Should empty AllowedTools mean unrestricted or no tools?

**Context:** `isToolAllowed` returns true when the list is empty. Failed ConfigFactory yields empty config → unrestricted modular names still pass allow-list (safety remains).  

**Options:** (a) keep unrestricted bootstrap, (b) treat empty as no tools, (c) distinguish nil vs empty slice.

## Q2 — Who owns campaign session assembly?

**Context:** Cortex boot builds session in `initFinalExecutors`; campaign cobra rebuilds a parallel stack.  

**Question:** Should campaign always take TaskExecutor from Cortex, or is intentional isolation required?

## Q3 — Piggyback multi-turn protocol shape

**Context:** Native path uses `ToolResultsProvider`. Piggyback needs a structured follow-up envelope.  

**Question:** Reuse CompleteWithSystem with synthesized history, or extend provider interfaces?

## Q4 — Ouroboros tools in native CompleteWithTools

**Context:** `buildToolDefinitions` only maps modular registry tools. Ouroboros appears in Piggyback catalog + execute path.  

**Question:** Should native providers also advertise Ouroboros tools in ToolDefinition lists?

## Q5 — Transactionality after post-validate failure

**Context:** Validate runs after modular execute; failure returns error but FS side effect may exist.  

**Question:** Should validate-failure trigger compensating actions, or always leave rollback to higher layers?

## Q6 — Token budget derivation

**Context:** Default 65536; comments say UserConfig.ContextWindow.MaxTokens should drive budget.  

**Question:** Is derivation fully wired in chat boot for Executor.SetConfig / SpawnerConfig.TokenBudget, or only partial?

## Q7 — SharedTaxonomy learning success heuristic

**Context:** Success = no toolErrs and no result.Error; empty response with toolErrs becomes Error.  

**Question:** Is that sufficient for learning quality, or should side-effect validators gate Success?

## Q8 — Legacy package README slogan

**Context:** “No spawn. No factories.” contradicts Spawner + ConfigFactory.  

**Question:** Update package README to historical note, or delete slogan entirely?

## Q9 — System SubAgentType usage

**Context:** `determineAgentType` returns system for category `/system`, but long-running system agent lifecycle is thin vs ephemeral.  

**Question:** Are system-type agents fully productized, or reserved?

## Q10 — Wait polling cost under mass parallel campaigns

**Context:** 100ms ticker per Wait.  

**Question:** Need event-based completion before assault-scale concurrency?

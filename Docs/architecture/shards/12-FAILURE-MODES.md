# 12 — Failure Modes: shards

> Last verified against codebase: 2026-07-13  
> Concrete failures and mitigations

## FM1 — Infinite OODA on session start

**Symptom:** CPU/kernel evaluate thrash; repeated actions without user input.  
**Causes:** Stale `user_intent`/`next_action`/`pending_action` rehydrated; boot guard disabled too early.  
**Mitigations:** Executive startup retracts intent/pending facts; boot guard defaults on; only disable after user/CLI initiation.

## FM2 — Actions never execute

**Symptom:** Intents parse; no tool effects.  
**Causes:** Boot guard still active; barriers (`block_commit`); constitution deny; router not started; unmapped action; rate limit.  
**Diagnosis path:** `next_action` → `pending_action` → `permission_check_result` → `permitted_action` → `exec_request`.

## FM3 — Everything denied

**Symptom:** All actions `security_violation`.  
**Causes:** Missing/broken `permitted` policy rules; StrictMode + query failure; dangerous pattern false positive; domain not allowlisted.  
**Mitigations:** Fix policy corpus; appeal path; expand allowlist carefully; never disable StrictMode casually.

## FM4 — Hollow shard

**Symptom:** Shard runs but no kernel facts / LLM errors “no client”.  
**Causes:** Factory registered without RegistryContext deps; wrong kernel type (`SetParentKernel` rejects non-Real/Cortex).  
**Mitigations:** Always pass full RegistryContext; check logs for “Invalid kernel type”.

## FM5 — Dual registration drift

**Symptom:** Works in CLI Cortex, fails in chat (or reverse); missing GlassBox/browser/campaign manager.  
**Causes:** session_boot vs factory diverge.  
**Mitigations:** Unify registration; re-register overrides only for extras; lock with tests.

## FM6 — LLM cost runaway

**Symptom:** Provider rate limits; huge bills.  
**Causes:** Perception/planner loops without CostGuard path; validation budget not reset.  
**Mitigations:** GuardedLLMCall; session caps; ResetValidationBudget per turn; backoff on errors.

## FM7 — Invalid learned Mangle poisons kernel

**Symptom:** Evaluate failures; `debug_program_ERROR.mg`; bad rules in learned layer.  
**Causes:** Repair interceptor not wired; repair rejected but caller ignored; legislator sandbox skipped.  
**Mitigations:** Always `SetRepairInterceptor` on boots; max 3 repairs then reject; pre-validator + stratification checks.

## FM8 — Campaign auto-runs on boot

**Symptom:** Campaigns resume unexpectedly.  
**Causes:** campaign_runner Auto startup (should be OnDemand); explicit start path.  
**Mitigations:** Keep OnDemand profile; require explicit supervisor start.

## FM9 — Specialist mismatch

**Symptom:** Wrong expert consulted; empty match list.  
**Causes:** Pattern heuristics; agent not `ready` in registry; confidence thresholds.  
**Mitigations:** Tune DefaultVerbConfigs; ensure agents.json status; future embeddings.

## FM10 — Observer event channel full

**Symptom:** Dropped events; stale Northstar assessments.  
**Causes:** Buffer 100; slow spawner.  
**Mitigations:** Direct NorthstarHandler; increase buffer carefully; monitor EventsReceived.

## FM11 — Requirements interrogator hard-fails

**Symptom:** Error “JIT prompt compilation failed”.  
**Causes:** Missing atoms / assembler not ready.  
**Mitigations:** Ship system requirements_interrogator atoms; only static fallback when LLM nil.

## FM12 — Kernel evaluate saturation

**Symptom:** Multi-second Query latency; tickers pile up.  
**Causes:** Too-aggressive poll intervals; large world EDB.  
**Mitigations:** Event bus subscription; 2s fallback ticks; 15s heartbeats (current code comments document this history).

## FM13 — Concurrent Stop vs autopoiesis

**Symptom:** Race on learning stores after shard stop.  
**Causes:** Fire-and-forget proposal goroutines (fixed with WaitGroup on executive).  
**Mitigations:** Keep `autopoiesisWg` wait in defer; apply same pattern to other async LLM shards.

## FM14 — Payload decode failure

**Symptom:** Lost intent_id; empty payloads between executive and constitution.  
**Causes:** Non-JSON payload strings; pseudo-map formatting.  
**Mitigations:** Use `encodeActionPayload` consistently; payloads.go accepts map/JSON/pseudo-map.

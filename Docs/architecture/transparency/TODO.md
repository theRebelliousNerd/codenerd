# transparency — TODO

> Last verified: 2026-07-13  
> Prioritized backlog for `internal/transparency` and **required** consumer wiring.  
> DOCS ONLY rebuild does not implement these.

## P0 — Honesty & split-brain

- [ ] **Unify or dual-feed shard visibility:** call `TransparencyManager.StartShard` / `UpdateShardPhase` / `EndShard` from ShardManager lifecycle **or** stop advertising Active Operations from unfed Observer.  
- [ ] **Type `SetTransparencyManager`:** replace `any` with a small interface in `internal/types` (Enable/phase methods) **or** remove dead storage.  
- [ ] **Status honesty:** mark `StreamReasoning` / `JITExplain` / `OperationSummaries` as experimental in `GetStatus` until wired, or implement wiring.

## P1 — Product completeness

- [ ] Auto `ReportSafetyViolation` on constitutional / `permitted` deny with rule + action + target.  
- [ ] Emit `CategoryJIT` events from prompt compiler when JIT explain is on.  
- [ ] Align config comments (`GlassBoxCategories`) with `CategoryRouting`.  
- [ ] Wire `OperationSummaries` to post-turn summary using `FormatOperationSummary`.  
- [ ] Add drop counters to `GlassBoxBusStats` and ToolEventBus.

## P2 — Hardening

- [ ] Mutex (or single-owner docs + race tests) for `SafetyReporter`.  
- [ ] Stress test: multi-goroutine Emit + Subscribe drain under `-race`.  
- [ ] Expand `explainRule` map from real policy rule names used in `.mg` corpus.  
- [ ] Structured error types at VirtualStore boundaries to reduce ClassifyError ambiguity.

## P3 — Future surfaces

- [ ] Optional JSON/NDJSON event sink for headless campaign runs.  
- [ ] OTel bridge (optional) mapping categories → span events.  
- [ ] Per-turn Glass Box export attached to campaign assault artifacts.  
- [ ] Machine-checkable invariant tests that ToolEvent still flows when Glass Box disabled.

## Done (living — do not re-open without evidence)

- [x] GlassBoxEventBus batch + verbose immediate paths  
- [x] ToolEventBus always-on design  
- [x] Explainer over DerivationTrace  
- [x] Error classifier + recovery guides  
- [x] Chat boot constructs and injects buses  
- [x] VirtualStore routing/tool emit  
- [x] ShardManager CategoryShard emit  
- [x] `/transparency` slash command  

## Notes

- Prefer wiring audits (`integration-auditor` skill) before deleting APIs.  
- Keep root AGENTS.md free of encyclopedia detail; deep notes stay in this corpus.

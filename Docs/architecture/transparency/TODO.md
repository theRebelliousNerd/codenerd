# transparency — TODO

> Last verified: 2026-08-16  
> Prioritized backlog for `internal/transparency` and **required** consumer wiring.  
> DOCS ONLY rebuild does not implement these.

## P0 — Honesty & split-brain

- [x] **Unify or dual-feed shard visibility:** `ShardManager.SpawnAsyncWithContext` now calls `StartShard` / `UpdateShardPhase(PhaseExecuting)` / `EndShard` (and the panic path ends the execution). Tracking is gated on `ShardPhases` alone so `/transparency on` mid-run sees in-flight shards; the observer's tracked set is bounded (`pruneTerminalLocked`). Evidence: `internal/core/shards/transparency_feed_test.go`.  
- [x] **Type `SetTransparencyManager`:** `types.TransparencyManager` (+ `types.ShardPhase`, `types.OperationRecord`) replaces the `any` field. `transparency.ShardPhase` is now an alias of the types definition.  
- [x] **Status honesty:** `JITExplain` and `OperationSummaries` are wired; `StreamReasoning` is labelled **experimental** in `GetStatus` because no producer reads it. Evidence: `TestGetStatus_WhenStreamReasoningSet_ShouldLabelItExperimental`.

## P1 — Product completeness

- [x] Auto `ReportSafetyViolation` on constitutional / `permitted` / dreamer deny with rule + action + target, via `transparency.ReportDeny` at the VirtualStore routing deny sites. Evidence: `internal/core/transparency_deny_test.go`.  
- [x] Emit `CategoryJIT` events from the prompt compiler when JIT explain is on (`emitJITGlassBox` in `internal/prompt/compiler.go` → `transparency.EmitJIT`).  
- [x] Align config comments (`GlassBoxCategories`) with `CategoryRouting`.  
- [x] Wire `OperationSummaries`: `TransparencyManager.RecordOperation` + ring + `FormatLastOperation`, fed by shard completion and every routed VirtualStore action; surfaced as "Recent Operations" in `GetStatus`. (Producer is per-operation, not per-turn — the chat turn boundary lives outside this package.)  
- [x] Add drop counters to `GlassBoxBusStats` (`Dropped`/`Delivered`/`SinkCount`) and `ToolEventBus.Stats()`.

## P2 — Hardening

- [x] Mutex for `SafetyReporter` (denials now arrive from shard goroutines) + race test.  
- [x] Stress test: multi-goroutine Emit + Subscribe drain under `-race`.  
- [x] Expand `explainRule` map from real policy rule names. The old map was keyed by `rule_metadata` symbolic names while `DerivationNode.RuleName` is the head predicate, so no key ever matched; the new glossary is keyed by real rule heads and audited by `TestRuleGlossary_EveryEntry_ShouldExistInMangleCorpus`.  
- [x] Structured error types at VirtualStore boundaries (`transparency.BoundaryError` / `NewSafetyError`, honored first by `ClassifyError`).

## P3 — Future surfaces

- [x] Optional JSON/NDJSON event sink for headless campaign runs (`NDJSONSink`, auto-attached from `CODENERD_GLASSBOX_NDJSON`).  
- [ ] OTel bridge (optional) mapping categories → span events. **Deliberately deferred:** nothing in this repo configures a `TracerProvider`, so the bridge would emit into no-op spans. `EventSink` is the extension point when a provider exists (rationale recorded at the `EventSink` declaration).  
- Needs design - Per-turn Glass Box export attached to campaign assault artifacts. **Primitive exists** (`NDJSONSink.OnlyTurn`); attaching it to assault artifacts requires `internal/campaign` to own the sink lifecycle. The primitives are all present: NewNDJSONFileSink creates parent directories and appends, NDJSONSink.OnlyTurn scopes a sink to one turn, and assault artifacts already live under .nerd/campaigns/<slug>/assault/. Three concrete blockers stand in the way, all of them design rather than typing: (1) GlassBoxEventBus has AddSink (event_bus.go:81) but NO RemoveSink, so a per-turn sink attached today would accumulate for the life of the process - a leak, not an export. (2) internal/campaign has no transparency dependency at all; nothing in that package imports internal/transparency, so the orchestrator cannot reach a bus. (3) The bus is constructed only in the TUI boot paths (cmd/nerd/chat/session_boot.go:553 and session_shared_boot.go:226). An assault launched from the CLI has no bus at all, so "attach a sink" is undefined on that path unless a bus is also constructed there. The design decision is who owns the bus reference across the two launch paths, and what the sink's lifetime is bound to. Note that an env-var opt-in already exists (attachEnvNDJSONSink, keyed on NDJSONEventEnvVar) and is the cheaper alternative worth weighing before adding campaign-owned lifecycle.
- [x] Machine-checkable invariant tests that ToolEvent still flows when Glass Box disabled (`TestToolEventBus_WhenGlassBoxDisabled_ShouldStillDeliver`).

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

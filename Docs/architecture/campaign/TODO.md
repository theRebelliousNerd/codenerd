# TODO — campaign architecture / engineering backlog

> Last verified: **2026-07-13**  
> Ordered by dependency, not calendar estimates.

## P0 — Safety & correctness

- [ ] Audit every production `NewOrchestrator` call site sets non-nil `TaskExecutor`
- [ ] Surface risk preflight blocks in CLI/chat UX (not only CategoryCampaign logs)
- [ ] Golden tests for `ToFacts` predicate/arity stability
- [ ] Confirm checkpoint-fail never completes phase (regression test)

## P1 — Wiring completeness

- [ ] Default-wire IntelligenceGatherer when Cortex boot has world/git/MCP
- [ ] Define hard vs soft advisory blocking contract; implement hard path
- [ ] Document or implement Cobra assault configuration parity with chat
- [ ] Nested `campaign_ref` e2e for propagate/absorb/transform policies

## P2 — JIT & maintainability

- [ ] Migrate high-traffic roles from `prompts.go` into `internal/prompt/atoms/`
- [ ] Keep `StaticPromptProvider` as thin fallback only
- [ ] Refresh `internal/campaign/README.md` modular map + date (code package doc)

## P3 — Durability ops

- [ ] Journal verify/replay operator command
- [ ] Chaos test: kill during snapshot rename
- [ ] Assault summary export (aggregate results → single report file)

## P4 — Observability

- [ ] Closed enum or constants file for OrchestratorEvent.Type strings
- [ ] Optional metrics hooks (task duration histograms) without coupling to one backend

## Docs (this corpus)

- [x] Full rebuild 2026-07-13 under `Docs/architecture/campaign/`
- [ ] Re-verify line counts after large refactors
- [ ] Cross-link from CLI corpus assault section when CLI docs change

## Explicit non-goals

- Per-file assault tasks for huge repos  
- Replacing kernel `permitted` with campaign-local ACL  
- Client-app-specific campaign types in this package  

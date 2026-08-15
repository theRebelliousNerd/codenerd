# usage — TODO

> Last verified: **2026-07-13**  
> Docs-only backlog derived from code. No commitment that items are scheduled.

## P0 — Metering completeness

- [ ] Call `usage.FromContext` + `Track` from **every** production `perception.LLMClient` that receives usage metadata (not only ZAI).
- [ ] Confirm streaming completion paths attach the same context and Track **once** with final billed tokens.
- [ ] Standardize provider string ids (match config engine names).

## P1 — Persistence correctness

- [x] Atomic save: write temp file then rename onto `usage.json`.
- [x] Fix dirty re-arm: under one critical section, Save then if mutations occurred while saving, keep dirty and re-arm timer.
- [x] Flush on Cortex close / chat shutdown (`Save` if dirty).
- [x] Use or remove `autoSaveTimer` field; prefer cancelable timer.

## P2 — Product surface

- [x] Either implement bounded `Events` ring **or** document reserved + stop implying raw event log.
- [x] Cost estimation: static price table keyed by model → fill `TokenCounts.Cost`.
- [x] UI: render `BySession`; optional cost column (`cmd/nerd/ui/usage_page.go` TODOs align).
- [x] Log Load/Save failures through `internal/logging`.

## P3 — Attribution & ops

- [x] Aggregate by shard **name** (or composite name+type) if operators need specialist-level spend.
- [x] Optional CLI: `nerd usage` / dump JSON to stdout for scripts.
- [x] Cap or prune `BySession` for long-lived workspaces.
- [x] Reject negative token inputs in `Track`.

## P4 — Architecture hygiene

- [ ] Unify chat session tracker with Cortex tracker (single owner per process).
- [ ] Consider typed context keys for shard metadata (breaking; needs coordinated shards change).
- [x] Integration test: boot → NewContext → mock client Track → Save → reload.

## Explicit non-todos (unless product asks)

- Mangle predicates for hard token budgets  
- Cloud billing reconciliation  
- Vectryx / multi-workspace fleet dashboards  

## Doc maintenance

- [ ] When Track producers expand, update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) producer table.  
- [ ] Keep IMPLEMENTED_SPEC status table in sync after code changes.

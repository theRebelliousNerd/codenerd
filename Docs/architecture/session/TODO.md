# session — TODO

> Last verified: 2026-07-13  
> Prioritized backlog from code-grounded gap analysis. **Docs only in this pass — no code claimed done.**

## P0

- [ ] Audit all VirtualStore adapters used by Executor for `InteractiveExecutiveGate` method forwarding  
- [ ] Keep safety fail-closed suite green under real-kernel tests after any policy changes  

## P1

- [ ] Implement multi-iteration Piggyback tool result feedback loop (`runToolLoop` Piggyback branch)  
- [ ] Clarify empty `AllowedTools` semantics (unrestricted vs no-tools) when ConfigFactory fails  
- [ ] Share single factory helper for campaign and Cortex session stack assembly (reduce dual-wire drift)  

## P2

- [ ] Populate `atomsJSON` in `persistTurn`  
- [ ] Wire Piggyback memory operations to cold storage / SessionPersister.StoreCompressedState  
- [ ] Call StoreCompressedState from SubAgent compression path  
- [ ] SetOuroborosRegistry from boot when generation is enabled; e2e covering execute path  

## P3

- [ ] Document or unify `Spawn` (no auto-run) vs `SpawnSpecialist` (auto-run) contract  
- [ ] Optional: completion channels instead of 100ms Wait polling  
- [ ] Refresh `internal/session/README.md` to remove “No spawn” marketing drift  
- [ ] Add structured spans/trace IDs for Process → tool → gate  

## Docs hygiene

- [x] Full architecture corpus under `Docs/architecture/session/` (2026-07-13 rebuild)  
- [ ] Remove or redirect legacy misnamed files (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-SESSION.md`, etc.) if still present after rebuild  

# TODO — Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Prioritized engineering backlog (docs corpus rebuild does not implement these)

## P0

- [ ] Route all production tool creation through `ExecuteOuroborosLoop` (chat `generate_tool`, `ExecuteAction`).  
- [ ] Fail closed when `go_safety.mg` fails to load (no empty policy).  
- [ ] Audit default `AllowExec: true` — document or tighten for untrusted workspaces.

## P1

- [ ] Parity check post-boot: registry tool count vs `tool_registered` facts.  
- [ ] Confirm `StartKernelListener` started on all interactive boot paths; document poll interval.  
- [ ] Expand e2e: scripted multi-stage Ouroboros (safety fail → regen → thunderdome survive).  
- [ ] Campaign pregen always uses same safety depth as chat Ouroboros helpers.

## P2

- [ ] Unify Yaegi vs binary execution policy (config switch + docs).  
- [ ] Human-in-the-loop default for SPL auto-promote.  
- [ ] Agent spec → runtime scheduler ownership decision (shards vs autopoiesis).  
- [ ] Export optional metrics (generation latency, reject rates).  
- [ ] Golden suite per `ViolationType`.

## P3 / hygiene

- [ ] Refresh package `internal/autopoiesis/README.md` date/architecture version to match 2026 corpus.  
- [ ] Remove or redirect legacy architecture filenames if still present beside this corpus.  
- [ ] Reduce dual templates vs JIT prompt residual prose over time.

## Done recently (context)

- Modular orchestrator split (`autopoiesis_*.go`).  
- Thunderdome + PanicMaker in default Ouroboros path.  
- Kernel batch sync of restored tools.  
- Hot-reload parent fact propagation (GAP-019 comments).  
- Learning context refresh after generation/execution.

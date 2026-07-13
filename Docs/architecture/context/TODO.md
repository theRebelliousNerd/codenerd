# TODO — Context package backlog

> Last verified against codebase: 2026-07-13  
> Prioritized engineering backlog for `internal/context` and its wires  
> Docs-only corpus does not implement these items.

## P0 — correctness / safety

- [ ] Audit every chat path that injects history for `IsCompressionActive` parity (perception + articulation + session context).  
- [ ] Keep race coverage green: `go test -race ./internal/context/...` on activation changes.  
- [ ] Preserve issue weight clamp + score caps when editing `activation_scoring.go`.

## P1 — kernel/Go hybrid

- [ ] Measure frequency of Go fallback vs kernel inclusion in production logs; reduce dual-path drift.  
- [ ] Expand tests with loaded `context_compilation.mg` so `should_include_context` path is first-class.  
- [ ] Finish C3: consume `should_mask_observation` in Go when building summaries (assert path already present).

## P2 — quality of compression

- [ ] Validate target compression ratio on real multi-hour sessions (campaign assault artifacts).  
- [ ] Optional provider-aligned tokenizer adapter behind `TokenCounter`.  
- [ ] Ensure `LoadState` + `RefreshBudget` always paired on session rehydrate.

## P3 — learning & JIT

- [ ] Wire audit: confirm prompt JIT actually calls `GetActivationScores` each turn when expected.  
- [ ] Surface feedback store stats in glass-box / transparency UI.  
- [ ] Document operator workflow for inspecting helpful vs noise predicates.

## P4 — docs hygiene

- [ ] Align `internal/context/README.md` defaults (200k, current date, file list including feedback_store).  
- [ ] Remove or relocate crash-dump `debug_program_ERROR.mg` from package tree if not intentional.

## Done recently (do not re-open without evidence)

- Concurrent map fix on ActivationEngine.  
- Turn age calculation fix (turnNumber − turn id).  
- Core facts Query error logging (no silent empty safety set).  
- Issue keyword weight clamping.

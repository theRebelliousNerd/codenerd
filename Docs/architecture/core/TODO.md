# core — TODO (docs-tracked backlog)

> Last verified: **2026-07-13**  
> Docs-only rebuild does not implement these. Priority for future engineering.

## P1 — Safety / correctness

- [ ] Maintain a single registry or generated check: every `ActionType` has handler + `safe_action`/`dangerous` classification + destructive flag.
- [ ] Audit all production paths that call `Exec` without Dreamer; document or gate.
- [ ] Ensure HotLoad / AppendPolicy invalidates Dreamer cache + permission cache.
- [ ] Expand Dreamer `projectEffects` for campaign/python destructive verbs if any miss projections.

## P1 — Architecture clarity

- [ ] Unify narrative in `internal/core/README.md`: session owns OODA; `core/shards` is spawn plumbing (not “removed”).
- [ ] Document preferred kernel shape per binary mode (single RealKernel vs Cortex).

## P2 — Performance

- [ ] Publish ops recommendation for diff-eval (default off until caveats closed).
- [ ] Optional selective schema load profiles for lightweight CLI verbs.
- [ ] Measure + cap dreamer clone cost under multi-agent pressure.

## P2 — Testing

- [ ] Table test: ActionType completeness.
- [ ] Diff-eval retract/query regression suite.
- [ ] Cortex ownership conflict hard-fail option (today last-wins + warn).

## P3 — Observability

- [ ] Export APIScheduler / cortex route metrics to a standard metrics sink.
- [ ] Structured “deny reason code” field on security_violation for UI.

## P3 — Corpus hygiene

- [ ] Keep policy modules under line budgets; split if any file grows past maintainability.
- [ ] Revisit dead policy modules with wiring auditor before deletion.

## Done in this docs rebuild

- [x] Full architecture corpus under `Docs/architecture/core/` at CLI depth
- [x] Dense IMPLEMENTED_SPEC + Mangle surface deep-dive
- [x] Honest dual-path and gap documentation

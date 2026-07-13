# core — Gap Analysis

> Last verified: **2026-07-13**  
> Vision: [01-VISION.md](01-VISION.md) · Reality: [02-CURRENT-STATE.md](02-CURRENT-STATE.md) · Spec: [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md)

## Method

Gaps are **spec/vision vs code**, not “missing docs.” Non-gaps are listed so future agents do not re-litigate solved problems.

Priority: **P0** safety/correctness · **P1** architecture clarity · **P2** performance/DX · **P3** polish.

---

## Spec vs reality matrix

| Vision item | Reality | Gap? | Pri |
|-------------|---------|------|-----|
| Default deny `permitted/3` | Exact canonical envelope; classification cache cannot allow | No | — |
| Fail-closed Dreamer | Required on destructive route/preflight; checked clone staging | Projection fidelity only | P2 |
| Stratified trust (learned after constitution) | `rebuildProgram` order | No | — |
| Quiescent boot | Chat guard through rehydration; command boot is explicit mode | No, mode must stay named | — |
| Single effect gateway | RouteAction primary; direct Exec exists | Partial bypass | P1 |
| One clear orchestration path | Session + ShardManager dual | Yes | P1 |
| Diff-eval optional/safe | Flagged; comments caution production | Yes (ops clarity) | P2 |
| Multi-domain Cortex | Implemented + feature flag | Optional path complexity | P2 |
| Selective schema load | Load-all modular list at boot | Yes (size/latency) | P2 |
| Full OS sandbox Dreamer | Projected facts only | By design / future | P3 |
| Package README accurate | Migration story partially stale | Yes (docs-in-tree) | P3 |
| Provenance always on | Off by default (memory cost) | Intentional | — |
| Complete ActionType handlers | Large enum; coverage tests extensive | Residual unknown verbs | P2 |
| Permission cache correctness | Cache is classification-only; exact query always decides | No authorization gap | — |

---

## Prioritized gaps

### P0 — none open as structural product blockers

Constitution + RouteAction + Dreamer form a working multi-layer gate. The earlier
authorization, nil-kernel, schema, timestamp, and validation defects have focused
negative regressions. Residual risk is contract drift and projection coverage,
not a known fail-open route in the principal `RouteAction` pipeline.

### P1 — architecture & safety coverage

0. **Resolved repair set (2026-07-13)**
   `Dreamer.SimulateAction` previously asserted `hypothetical/1` into the live
   kernel before cloning. It now projects that fact only into the sandbox. The
   regression proves the live EDB count does not change. Checked projection
   staging also fails closed at the fact limit and on invalid facts. Exact
   `permitted/3` matching replaced cache-based authorization; Cortex co-locates
   the permission envelope on its policy shard; missing Dreamer/kernel paths deny;
   failure facts match their declarations; post-validation returns errors;
   pruning uses the timestamp field; and router results preserve executive IDs.

1. **Destructive classification drift**  
   New `ActionType` constants must update `isDestructiveAction` and often `safe_action`/`dangerous_action` in policy. No single registry enforces both Go and Mangle lists.

2. **Direct `Exec` / session tool path**  
   `VirtualStore.Exec` applies binary/env and traversal checks but not full Dreamer simulation. Document as intentional fast path or route through RouteAction.

3. **Dual orchestration**  
   Cognitive load and wiring bugs when boot injects both session TaskDelegator and ShardManager. Prefer one narrative in boot code comments + this corpus.

4. **Dreamer cache invalidation — OPEN QUESTION**
   Policy or critical-path fact changes must call `InvalidateCache`; audit all hot-load sites.

5. **CodeDOM metadata contract proof**
   Edit results now carry the concrete file used by semantic validation. The full
   core suite covers the handler, but a named negative test proving metadata beats
   a symbolic request target would make this seam independently falsifiable.

### P2 — performance & scale

1. **Boot program size** — concatenating all policy modules increases parse/analyze time.  
2. **Full re-eval vs diff-eval** — default strategy needs an explicit ops recommendation (eval comments already lean cautious).  
3. **Kernel Clone cost** under burst destructive simulations.  
4. **EDB growth** — action-log pruning now reads the correct timestamp slot;
   long campaigns still need measured retention evidence.

### P3 — polish

1. Align `internal/core/README.md` with live `shards/` role.  
2. Reduce illustrative-only samples that look like real GetFacts switch.  
3. Metrics export for APIScheduler beyond in-memory structs.

---

## Non-gaps (do not “fix”)

| Topic | Why not a gap |
|-------|----------------|
| LLM inside kernel | Correctly external |
| Vector search in Mangle | Correctly external (`store`/`embedding`) |
| Domain persona prompts in core | Belong in prompt atoms / session |
| Provenance off by default | Documented memory tradeoff |
| Cortex optional | Feature flag preserves single-kernel default |
| Large ActionType enum | Reflects real product surface |
| Policy tests in `defaults/policy` | Goldens already exercise logic |

---

## Wiring-risk inventory (delete carefully)

These look “optional” but have reverse deps / tests:

| Symbol / subsystem | Consumers |
|--------------------|-----------|
| `CortexKernel` | system boot, tests, per-shard features |
| `ShardFactRouter` | feature-flagged federation |
| `ShadowMode` | CLI dream/shadow, e2e safety tests |
| `DreamPlanManager` | dream plan lifecycle |
| `APIScheduler` | multi-shard LLM |
| `TransactionManager` | multi-file edits |
| `core/shards.ShardManager` | VS default, spawn plumbing, e2e |

Always grep registration and e2e before removal.

---

## Gap closure checklist (documentation done ≠ code done)

- [ ] Single table of ActionType → destructive? → safe_action? → handler  
- [ ] Explicit session bypass matrix  
- [ ] Kernel/policy mutation → Dreamer cache invalidation audit
- [ ] Dedicated CodeDOM metadata-to-semantic-validator negative test
- [ ] Diff-eval production recommendation in ops docs  
- [ ] Refresh package README migration section  

(Code changes are **out of scope** for this docs-only rebuild.)

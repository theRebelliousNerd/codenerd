# core — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Against: codeNERD north star (LLM creative; Mangle executive; constitutional default deny; JIT prompts; wiring before deletion)
> This 10-dimension narrative is retained for orientation. The current signed
> 14-dimension superstar score and receipts live in [_progress.md](_progress.md).

## Scoring method

Each dimension: **0–5** with evidence from `internal/core/` only (not aspirational docs).

| Score | Meaning |
|------:|---------|
| 5 | Fully realized in code and used on hot paths |
| 4 | Implemented with minor gaps or dual paths |
| 3 | Partial / feature-flagged / competing designs |
| 2 | Scaffold or weakly wired |
| 1 | Spec only / misleading docs |
| 0 | Absent |

---

## Dimensions

### 1. Inversion of control (LLM creative, logic executive) — **5/5**

**Evidence:** Kernel evaluates IDB rules to derive `next_action`, `permitted`, personas, routing helpers. VirtualStore does not invent policy; it executes envelopes under gates (`virtual_store_routing.go`). LLM is a dependency of shards/session, not the permit authority.

### 2. Constitutional safety (`permitted`, default deny) — **5/5**

**Evidence:**

- `defaults/policy/constitution.mg` requires positive `permitted` derivation from `safe_action` + pending envelope + not dangerous content.
- VS `CheckKernelPermitted` denies on nil kernel, query failure, or any mismatch
  in the exact action/target/canonical-payload triple. `safe_action` cache entries
  classify only.
- Go constitution + Dreamer add defense-in-depth layers.

**Gap (minor):** Some tools may bypass RouteAction via direct `Exec`/`session` paths — still binary/env filtered, but not the full dreamer stack on every call.

### 3. Speculative safety (Dreamer) — **4/5**

**Evidence:** Destructive RouteAction and interactive preflight require a usable
Dreamer. `RealKernel` and Cortex-primary backing are supported; sandbox facts are
checked and never asserted into live truth. Nil dependency, invalid/over-limit
projection, eval failure, and missing `panic_state` all deny.

**Gap:** Projection fidelity is rule-based (projected_fact), not full OS sandbox
simulation; cache freshness still lacks one mutation-epoch contract.

### 4. Deterministic executive corpus (schemas/policy embed) — **5/5**

**Evidence:** `//go:embed defaults/*.mg …` in `kernel_types.go`; boot fails if constitution will not compile (`NewRealKernel`). Modular schemas + stratified policy load order in `loadMangleFiles`.

### 5. Effect isolation (VirtualStore as sole effect boundary) — **4/5**

**Evidence:** Broad action enum and handlers; tactile executor; MCP map; inject facts back.

**Gap:** Historical dual paths (modern vs legacy executor, TaskDelegator vs residual ShardManager spawn) mean some effects can be reached through multiple entry points.

### 6. JIT-first LLM-facing behavior — **3/5**

**Evidence:** Kernel loads `schemas_prompts.mg`, `jit_compiler.mg`, policy `jit_*.mg`; boot prompts via hybrid loader. Primary JIT compiler lives in `internal/prompt` (correct separation).

**Gap:** Core still hosts large static policy for coder/campaign that overlaps “agent behavior” — intentional for executive control, but not all LLM prose is atomized outside core.

### 7. Wiring honesty (no orphan “unused” half-systems) — **3/5**

**Evidence:** Cortex multi-shard, differential eval, provenance, shadow mode, rule court exist and have tests.

**Gap:** Several features are flag-gated or secondary paths; package README still claims full ShardManager removal while `shards/` remains important plumbing. Risk of deleting “unused” wiring without reverse-dep audit.

### 8. Observability / glass box — **4/5**

**Evidence:** Categorized logging, audit action complete, Glass Box + tool buses on VS and ShardManager, debug program dump.

**Gap:** No single structured metrics registry in core; scheduler has metrics struct but not a full OTEL export.

### 9. Long-horizon / campaign executive support — **4/5**

**Evidence:** Campaign schemas + policy modules; campaign action types in VS; system OODA policy files.

**Gap:** Campaign *orchestration runtime* lives largely outside core (`internal/campaign`, chat); core supplies facts/actions/safety.

### 10. Concurrency safety — **4/5**

**Evidence:** Kernel mutex + atomic dirty bit + eval serialization; VS mutex; dreamer cache locks; scheduler slots.

**Gap:** Clone cost under high dreamer pressure; per-shard facts feature adds routing complexity (lock-order comments in cortex).

---

## Aggregate

| Area | Score |
|------|------:|
| Executive purity | 5 |
| Safety stack | 4.5 |
| Corpus completeness | 5 |
| Architectural clarity (single path) | 3.5 |
| Docs honesty (pre-rebuild) | was weak → this corpus targets 5 |

**Weighted overall: ~4.3 / 5** — core is the most complete “north star” package in the tree; residual complexity is dual orchestration and feature-flagged federation, not missing constitution.

---

## Recommendations (alignment, not build tasks)

1. Treat `RouteAction` + permitted + Dreamer as the **only** preferred effect path; document any deliberate bypasses in session.  
2. Keep README migration story accurate: session executor *owns OODA*, core shards *own spawn plumbing*.  
3. Keep fail-closed Dreamer; expand projected_fact coverage rather than soft-fail.  
4. Prefer wiring audits before removing Cortex/diff-eval/shadow — tests and e2e depend on them.

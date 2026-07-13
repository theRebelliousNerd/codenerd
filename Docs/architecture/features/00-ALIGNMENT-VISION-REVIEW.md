# 00 — Alignment & Vision Review: features

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded  
> Source: `internal/features/` (1 non-test Go file, 351 lines; 3 tests; 0 `.mg`)

## 1. North-star statement

codeNERD separates **LLM as creative center** from **logic as executive**. Feature flags are an **executive instrumentation layer**: they select which deterministic subsystems are live (eval engine variant, provenance recorder, system shards, scanners) without moving decision authority into the model.

A well-aligned features package:

1. Stays a **leaf** (no internal imports) so core can read flags without cycles.  
2. Uses **explicit, testable precedence** (env > config > default).  
3. Defaults **conservatively** for paths that change semantics or cost.  
4. Never becomes a second policy engine that bypasses `permitted(...)`.  
5. Documents each flag’s consumer so “unused” is never a deletion guess.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Pure Go registry; no LLM surface; consumers are kernel/boot/scanner (`features.go` package comment) |
| Fact-flow fidelity | **3** | Orthogonal to `user_intent → next_action`; gates eval/boot only — correct role, not spine |
| Constitutional safety | **4** | Does not implement permission; does not grant capabilities. Residual risk: flags enabling powerful paths (DiffEval, PerShardFacts) if mis-seeded |
| JIT / atom discipline | **5** | N/A by design (no prompts); score high for **not** smuggling LLM prose into a leaf |
| Leaf purity / layering | **5** | Only `fmt`, `os`, `sync/atomic`; logging owned by config caller |
| Test grounding | **5** | Precedence, defaults, copy semantics, config round-trip external tests |
| Observability | **3** | `Summary()` for Boot; no metrics, no live dump CLI |
| Consumer wiring completeness | **4** | Most flags wired; TaxonomyFast accessor unused by tool |
| Doc/comment honesty | **3** | Some stale comments in core/tests about defaults and “short-circuit” |

**Overall alignment: 4.1 / 5** — small, correct architectural leaf; residual risk is wiring drift and comment rot, not conceptual misplacement.

## 3. What “good” looks like (features-specific)

| Good | Bad |
|------|-----|
| Three-way lockstep when adding a flag (field + accessor + default/seed) | Orphan JSON keys or env vars with no accessor |
| Env override for CI/tests without editing disk | Only disk config, forcing test file writes |
| Conservative compile defaults for eval cost | DiffEval ON in unit tests by default |
| Boot log of live flags via `Summary()` | Silent process with unknown flag state |
| Master switch separate from per-shard disable list | One overloaded env that means both |
| Wiring audit before removing “unused” accessor | Delete TaxonomyFast because tool greps env only |

## 4. Related corpora

- `Docs/architecture/config/` — loads JSON and calls `SetActive`  
- `Docs/architecture/core/` — DiffEval, PerShardFacts, provenance enablement  
- `Docs/architecture/cli/` — flight recorder boot, dark mode, session_boot  
- `Docs/architecture/world/` — scanner tunables  
- `Docs/architecture/ux/` — onboarding skip  

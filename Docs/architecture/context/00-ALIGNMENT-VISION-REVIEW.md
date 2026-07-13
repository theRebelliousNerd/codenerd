# 00 — Alignment & Vision Review: Context (`internal/context`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/context/` (9 non-test Go files ≈ 4.7k lines; 11 test files)

## 1. North-star statement

codeNERD separates **creative** (LLM) from **executive** (Mangle kernel). The context package is the **working-memory membrane** between those roles:

- The kernel holds durable logical truth (`user_intent`, `permitted`, diagnostics, campaign state).  
- The LLM needs a **bounded, high-signal** view of that truth each turn.  
- Context **selects and compresses** — it does not authorize actions and does not invent next_action.

Aligned behavior:

1. Prefer logical atoms over chat surface text.  
2. Keep constitutional facts in the core reserve.  
3. Let policy/kernel influence inclusion where wired (`should_include_context`).  
4. Bound the window with hard errors rather than silent overflow.  
5. Feed JIT/prompt systems with activation scores, not ad-hoc prose dumps.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Compresses for LLM; asserts only control-packet atoms; does not implement `next_action` (`compressor.go`, `compressor_turns.go`) |
| Fact-flow fidelity | **4** | Consumes `user_intent` + control packets; injects via chat process; async ProcessTurn can lag same-turn build slightly |
| Infinite-context mechanism | **4** | Sliding window + rolling summary + activation selection implemented; target 100:1 is heuristic estimate not guaranteed |
| Kernel-first selection | **3** | Hybrid: tries `should_include_context` then falls back to Go 9-component scorer (`BuildContext`, NERD-EVOLVE markers) |
| Constitutional safety surface | **4** | `getCoreFacts` always queries safety predicates with warn-on-error; action auth still owned by core policy |
| Learning loop | **4** | Context feedback store + activation feedback component; requires LLM ratings + min samples |
| Concurrency correctness | **4** | Mutexes on compressor/activation; race test; historical concurrent-map fix documented in comments |
| Observability | **4** | `CategoryContext` timers/logs; budget metrics for UI |
| Test grounding | **4** | Broad unit coverage on scoring/budget/serialize/feedback; less package-level chat E2E |
| JIT / prompt coupling | **3** | `GetActivationScores` / high keys exported; prompt package comments reference them — ensure live wire stays audited |

**Overall alignment: 3.9 / 5** — production-grade window manager with a clear north-star fit; residual risk is hybrid Go/Mangle scoring and estimated tokens.

## 3. What “good” looks like (context-specific)

| Good | Bad |
|------|-----|
| Atoms in window; surface discarded | Dumping full chat forever |
| Core always includes permission-related facts | Empty core when Query fails silently (mitigated with Warn) |
| Threshold before budget select | Flooding with low-score recency noise |
| Issue weights clamped | Adversarial keyword weights dominate safety facts |
| Kernel inclusion when rules fire | Ignoring declared `should_include_context` once available |
| Persist compressed state for rehydrate | Rehydrate with 50 raw messages only |

## 4. Related corpora

- `Docs/architecture/core/` — kernel, schemas_context, policy compilation  
- `Docs/architecture/cli/` — chat boot and ProcessTurn call sites  
- `Docs/architecture/prompt/` — JIT activation score consumers  
- `Docs/architecture/perception/` — ControlPacket production  
- `Docs/architecture/store/` — compressed state + activation logs  

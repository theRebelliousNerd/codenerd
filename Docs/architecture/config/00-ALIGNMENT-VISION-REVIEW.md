# 00 — Alignment & Vision Review: config (`internal/config`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/config/` (17 non-test Go files ≈ 3.1k lines; 5 test files)

## 1. North-star statement

codeNERD separates **creative** work (LLM) from **executive** control (Mangle kernel). Configuration must not become a second executive that silently re-routes providers, bypasses allowlists, or invents model IDs. Its job is to **declare budgets, backends, and operator preferences** so that:

1. Perception/articulation know which engine and model to use.
2. Kernel and scheduler receive enforceable ceilings.
3. JIT, embedding, world, and tactile layers share one workspace-rooted truth (`.nerd/config.json`).
4. Feature flags reach leaf packages without circular imports (`features.SetActive`).

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Config supplies engines/keys/limits only; no `next_action` or `permitted` derivation (`user_config.go`, `limits.go`) |
| Config-is-boss fidelity | **5** | Explicit `GetActiveProvider` refuses silent provider fallback (`user_config.go`) |
| Fact-flow support | **4** | Limits and timeouts feed kernel/scheduler/perception; config itself is off the fact graph |
| Dual-path honesty | **3** | Live JSON path strong; YAML `Config` + env overrides still parallel and can drift (`config.go` vs `user_config.go`) |
| Engine multi-backend | **5** | api / claude-cli / codex-cli / xai-oauth with subscription-aware scheduler defaults (`llm.go`, `GetEffectiveAPISchedulerPolicy`) |
| JIT / atom discipline | **4** | `JITConfig` + clamp to context window; TraceLLMIO default true in defaults (`jit.go`) |
| Safety data at edge | **4** | Execution allowlists and concurrency floors; enforcement lives in tactile/core (`execution.go`, `ValidateCoreLimits`) |
| Workspace hygiene | **5** | go.mod-first `FindWorkspaceRoot` prevents nested/home `.nerd` traps |
| Test grounding | **4** | Comprehensive tests for load, providers, engines, scheduler, workspace; less coverage for every UX/feature field |
| Observability | **4** | Boot logging on load; `LoggingConfig` categories; JIT/LLM I/O trace flags |

**Overall alignment: 4.3 / 5** — mature living package; residual risk is dual-aggregate drift and incomplete env parity on the JSON path.

## 3. What “good” looks like (config-specific)

| Good | Bad |
|------|-----|
| One workspace-rooted `config.json` | Stray nested `.nerd` capturing state |
| Explicit provider + matching key | Silent switch to another provider’s key |
| Get* defaults for zero values | Callers hardcoding `embeddinggemma` without tag |
| Engine MaxConcurrentCalls ≤ core ceiling | Unbounded parallel subscription calls |
| CLI engines as subprocess **LLM APIs** | Treating codex/claude CLI as unconstrained agents (sandbox/shell defaults) |
| Features installed once at load | Leaf packages re-parsing JSON |

## 4. Related corpora

- `Docs/architecture/cli/` — primary mutator and boot consumer  
- `Docs/architecture/core/` — core_limits / API scheduler consumers  
- `Docs/architecture/perception/` — engines and clients  
- `Docs/architecture/prompt/` — JIT token budgets  

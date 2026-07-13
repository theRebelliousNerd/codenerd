# OPEN QUESTIONS — JIT config

> Last verified: **2026-07-13**  
> Real open questions from code review of `internal/jit` + consumers.

## Q1 — Who owns tool-loop budgets?

Should loop limits live on `EffectiveAgentRuntimeConfig.ToolLoop` (per-agent YAML), on `session.ExecutorConfig` (per-process), or both with documented precedence?

**Evidence:** Factory sets ToolLoop `{5,50,false}`; executor uses MaxToolIterations default **8**.

## Q2 — What should canonical `Policies` membership mean at runtime?

Should the validated canonical members remain evidence about the globally loaded
default corpus, or should session create a selectively loaded per-agent kernel?
If the latter, where do stable set identity and version travel?

**Evidence:** `core.DefaultAgentPolicySetFiles` resolves stable set IDs to
canonical boot-inventory members; both prompt providers use it and `Validate`
rejects nonmembers. No session code loads `cfg.Policies` selectively or records a
set ID/version.

## Q3 — What typed degradation should replace the deny-all zero value?

Nil and empty allowlists now deny modular and Ouroboros execution. Should a
compile/factory failure remain text-only deny-all, or should a separately typed
degraded mode grant a named read-only set with a machine-readable reason?

## Q4 — Should generated and fallback configs share specialist validation?

Specialist YAML now invokes `Validate`; factory and zero-value fallback paths do
not uniformly do so. Should factory output be rejected, or should failures return
an explicit degraded type that is not required to satisfy full-config invariants?

## Q5 — Is `Persona` redundant with `IntentVerb`?

Both exist; routing is intent-verb-centric. Keep for YAML clarity or collapse?

## Q6 — Compiler dual generation?

Compiler may attach config on `CompilationResult`, and executor may call ConfigFactory again. When do both fire, and which wins outside SubAgent inject?

## Q7 — Multi-intent merge priority?

ConfigAtom.Merge keeps higher priority and unions tools/policies. Is multi-intent generate used in production chat, or single-verb only?

## Q8 — Specialist factory fallback with empty prompt?

`loadSpecialistConfig` falls back to `Generate(..., CompilationResult{Prompt:""}, "/"+name)`. That **fails Validate** if ever checked. Intended?

## Q9 — Package path longevity?

Should types move under `planned:internal/session/config` or
`planned:internal/agentconfig` for clarity, or remain `internal/jit/config` as the
public name for the JIT architecture?

## Q10 — Relationship to global user config

How should `internal/config` LLM model/provider interact with per-agent `Model` if that field is activated?

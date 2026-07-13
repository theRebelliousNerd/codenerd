# OPEN QUESTIONS — JIT config

> Last verified: **2026-07-13**  
> Real open questions from code review of `internal/jit` + consumers.

## Q1 — Who owns tool-loop budgets?

Should loop limits live on `EffectiveAgentRuntimeConfig.ToolLoop` (per-agent YAML), on `session.ExecutorConfig` (per-process), or both with documented precedence?

**Evidence:** Factory sets ToolLoop `{5,50,false}`; executor uses MaxToolIterations default **8**.

## Q2 — What does `Policies` mean at runtime?

Are policy file names loaded into the kernel per agent, layered on the global corpus, or only a Validate-time constitutional checklist?

**Evidence:** Factory always sets `base.mg` + persona; no session code found that loads `cfg.Policies` as files.

## Q3 — Empty allowlist semantics?

Is `AllowedTools: []` “no tools” (deny all) or “unrestricted”? `isToolAllowed` / `buildToolDefinitions` special-case empty lists — product intent should be explicit.

## Q4 — Should Validate be mandatory on all production paths?

YAML load and empty fallbacks currently skip Validate. Is that intentional degrade UX or accidental leniency?

## Q5 — Is `Persona` redundant with `IntentVerb`?

Both exist; routing is intent-verb-centric. Keep for YAML clarity or collapse?

## Q6 — Compiler dual generation?

Compiler may attach config on `CompilationResult`, and executor may call ConfigFactory again. When do both fire, and which wins outside SubAgent inject?

## Q7 — Multi-intent merge priority?

ConfigAtom.Merge keeps higher priority and unions tools/policies. Is multi-intent generate used in production chat, or single-verb only?

## Q8 — Specialist factory fallback with empty prompt?

`loadSpecialistConfig` falls back to `Generate(..., CompilationResult{Prompt:""}, "/"+name)`. That **fails Validate** if ever checked. Intended?

## Q9 — Package path longevity?

Should types move under `internal/session/config` or `internal/agentconfig` for clarity, or remain `internal/jit/config` as the public name for the JIT architecture?

## Q10 — Relationship to global user config

How should `internal/config` LLM model/provider interact with per-agent `Model` if that field is activated?

# prompt — Vision

> Last verified: **2026-07-13**  
> Package: `internal/prompt`

## Product vision

Every LLM call in codeNERD receives a **compiled** system prompt: a minimal, context-true assembly of **atoms** chosen by **deterministic logic** (and optional semantic retrieval), never a monolithic hand-authored string frozen in a shard.

Operators and agent authors extend capability by:

1. Adding **YAML atoms** (identity, methodology, language encyclopedias, campaign phases, …).  
2. Registering **ConfigAtoms** (allowed tools + policy files) per intent/persona.  
3. Optionally storing project/agent atoms in **SQLite** with embeddings for semantic flesh.

The kernel remains the executive: it decides *what* to do and *whether* it is permitted. The prompt system decides *what the model is told* and *which tools the session will allow* for that turn.

## Architectural vision

### Infinite effective prompt length

Through atomic decomposition + context selection + polymorphic content (`content` / `content_concise` / `content_min`) + category budgets, the system presents only the slice of a large atom library that fits the turn — without losing constitutional identity/safety skeleton.

### System-2 bifurcation (skeleton / flesh)

| Layer | Categories (core) | Selection | Failure mode |
|-------|-------------------|-----------|--------------|
| **Skeleton** | identity, protocol, safety, methodology | Mangle rules only | **CRITICAL** — compilation fails |
| **Flesh** | language, framework, domain, context, exemplars, campaign, … | Vector hits + Mangle filter + context match | **Degrade** — continue with skeleton |

This is intentional: safety and identity must never depend on embedding quality.

### Dual output of compilation

`Compile` yields more than text:

1. **System prompt string** (assembled atoms).  
2. **`EffectiveAgentRuntimeConfig`** (when ConfigFactory attached): identity prompt, allowed tools, policies, tool-loop and safety knobs.

Session executor uses both for the LLM tool loop and policy load path.

### Atom sources (target model)

```
┌─────────────────────┐
│ Embedded YAML atoms │  go:embed atoms/**  (always)
└──────────┬──────────┘
┌──────────▼──────────┐
│ Project corpus.db   │  .nerd/prompts/corpus.db (+ hybrid PROMPT: ingest)
└──────────┬──────────┘
┌──────────▼──────────┐
│ Agent knowledge DBs │  .nerd/agents|shards/*_knowledge.db
└──────────┬──────────┘
┌──────────▼──────────┐
│ Evolved atoms (SPL) │  .nerd/prompts/evolved/{pending,promoted}
└──────────┬──────────┘
┌──────────▼──────────┐
│ Kernel ephemeral    │  injectable_context, specialist_knowledge
│ Knowledge/Learning  │  LocalStore / LearningStore bridge
└─────────────────────┘
```

### Non-goals

- Fuzzy NL pattern banks in Mangle (use embeddings then assert structure).  
- Client-app-specific prompt features in core (general substrate only).  
- Replacing constitutional `permitted(...)` with “please don’t” prose alone.

## Success criteria

| Criterion | Indicator |
|-----------|-----------|
| Atoms-first | New behavior lands as YAML under `atoms/` or agent `prompts.yaml` |
| Skeleton integrity | Every production compile has identity+safety path via kernel |
| Budget compliance | Fitted tokens ≤ available budget; oversized mandatory logged and skipped |
| Flight recorder | Manifest + stats explain selected vs dropped atoms |
| Session integration | Executor never hardcodes system prompts when JIT is configured |
| Regenerability | Baked `prompt_corpus.db` rebuildable from YAML |

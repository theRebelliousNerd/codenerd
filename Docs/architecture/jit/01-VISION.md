# 01 — Vision: JIT agent runtime configuration

> Last verified against codebase: **2026-07-13**  
> Package: `internal/jit` (schema) · ecosystem: prompt + session + mangle routing

## 1. Product vision

Every coding agent turn should receive a **fresh, intent-specific constitution**:

- **Who it is** (`IdentityPrompt` / persona atoms)  
- **What it may call** (`AllowedTools`)  
- **Which Mangle policies bind it** (`Policies`)  
- **How hard it may thrash tools** (`ToolLoop`)  
- **Whether policy enforcement is mandatory** (`Safety`)  
- **Where it may touch the workspace** (`Workspace`)

That constitution is assembled **just in time** from declarative intent routing and prompt/config atoms — never by shipping a new Go type per specialty.

## 2. Architectural vision

```
                    ┌─────────────────────────┐
                    │  user_intent / IntentVerb│
                    └────────────┬────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              ▼                                     ▼
   ┌────────────────────┐              ┌────────────────────┐
   │ JIT Prompt Compiler│              │   ConfigFactory    │
   │ (prompt atoms)     │              │ (ConfigAtoms)      │
   └─────────┬──────────┘              └─────────┬──────────┘
             │ Identity text                      │ tools + policies
             └──────────────────┬─────────────────┘
                                ▼
                 ┌──────────────────────────────────┐
                 │ EffectiveAgentRuntimeConfig      │
                 │ (internal/jit/config)            │
                 └──────────────────┬───────────────┘
                                    ▼
                 ┌──────────────────────────────────┐
                 │ Session Executor (universal loop)│
                 │ LLM + tool allowlist + gates     │
                 └──────────────────────────────────┘
```

**Vision for this package specifically:** remain a **pure schema package** — no I/O, no kernel queries, no LLM calls. All intelligence stays in producers (prompt/mangle) and consumers (session/core).

## 3. Target properties

| Property | Target |
|----------|--------|
| Stability | Schema changes are major; tag-compatible YAML for specialists |
| Validation | Every hot-path config either validates or is an explicitly labeled degraded mode |
| Completeness | Every field has a single documented consumer or is removed |
| Extensibility | New personas = new ConfigAtoms + policy files, not new Go agent types |
| Safety | Default deny tools (empty allowlist blocks unknown tools); policies non-empty when accepting “full” mode |
| Portability | Same type used by in-process factory output and on-disk specialist YAML |

## 4. Non-goals

- Implementing ConfigFactory or prompt compilation here  
- Owning tool implementations  
- Owning Mangle policy *content* (only **paths/names** of `.mg` files)  
- Per-provider model routing (global engine config remains `internal/config`)  
- sibling-platform/foreign-product-surface product surfaces

## 5. Success criteria

1. Adding a persona never requires a new type under `internal/jit`.  
2. `go test ./internal/jit/...` stays green and documents Validate edges.  
3. Session tool execution never widens beyond `AllowedTools` when a non-empty list is present.  
4. Specialist YAML unmarshals into the same struct as factory output.  
5. Architecture docs and skill references use **one** name: `EffectiveAgentRuntimeConfig` (not legacy `AgentConfig`).

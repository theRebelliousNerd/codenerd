# 09 — Safety and Invariants: JIT config

> Last verified against codebase: **2026-07-13**  
> Package: `internal/jit/config` + consumer enforcement sites

## 1. Package-local invariants

| ID | Invariant | Enforcement |
|----|-----------|-------------|
| I1 | Valid “full” config has non-empty identity after trim | `Validate()` |
| I2 | Valid “full” config has ≥1 policy reference | `Validate()` |
| I3 | Package performs no I/O or network | Import set |
| I4 | Package performs no tool execution | Import set |
| I5 | Types are pure data (value-friendly) | No mutexes/pointers required on structs |

## 2. System invariants (consumers must uphold)

| ID | Invariant | Status |
|----|-----------|--------|
| S1 | Tool calls from the model must be in `AllowedTools` when allowlist non-empty | **Enforced** in `session.isToolAllowed` |
| S2 | Specialist names cannot path-escape `.nerd/agents` | **Enforced** in `loadSpecialistConfig` |
| S3 | Specialist YAML size bounded | **Enforced** (1 MiB) |
| S4 | Production agents always pass `Validate()` | **Partial** — skipped on YAML load and empty fallbacks |
| S5 | `RequirePolicyEnforcement` true means policies loaded | **Not enforced** as of 2026-07-13 |
| S6 | ToolLoop limits bound runaway tool thrash | **Partial** — executor limits exist, schema ToolLoop not used |
| S7 | Constitutional `permitted(...)` still applies | **Owned by core/policy**, independent of this schema |
| S8 | Default deny for unknown tools | Depends on empty vs non-empty allowlist behavior |

## 3. Constitutional safety relationship

codeNERD default deny:

```
action executes only if permitted(...) derives true
```

`EffectiveAgentRuntimeConfig` is **not** a substitute for Mangle permission. It is a **capability envelope** for the LLM creative surface:

- Narrows which tools the model is **offered** and **allowed by name**.  
- Names policy files that **should** ground the executive layer.  
- Does not assert facts or run Dreamer simulation itself.

Interactive path also uses `InteractiveExecutiveGate` on VirtualStore for destructive preflight (`session/executor.go` comments).

## 4. Concurrency

| Concern | Assessment |
|---------|------------|
| Config struct | Immutable-by-convention after handoff; slices are shared if not copied |
| ConfigFactory provider | `DefaultConfigAtomProvider` uses `sync.RWMutex` (prompt package) |
| Executor holding cfg | Protected by executor `mu` when reading injected config |
| Race tests | e2e orchestrator race tests import jit config mocks |

**Invariant:** do not mutate `AllowedTools`/`Policies` slices after handing config to concurrent executors without cloning.

## 5. Mangle Decl

Not applicable inside `internal/jit`. Policy file **names** may correspond to Decl-bearing corpora under `internal/core/defaults/policy/` and related `.mg` sources, but those are outside this package.

## 6. Threat model (config-centric)

| Threat | Mitigation | Residual risk |
|--------|------------|---------------|
| Agent without identity | Validate | Bypass if Validate skipped |
| Agent without policies | Validate | Same |
| Path traversal via specialist name | Spawner name checks | Must keep checks on all load sites |
| Oversized YAML DoS | 1 MiB cap | Other load sites must share cap |
| Tool allowlist bypass | `isToolAllowed` | Empty config / nil cfg edges need audits |
| False safety via unused flags | Docs honesty + TODO wiring | Operators may trust YAML that is ignored |

## 7. Recommended hard gates (not all implemented)

1. After any YAML unmarshal: `if err := cfg.Validate(); err != nil { return nil, err }`.  
2. If `Safety.RequirePolicyEnforcement` and `len(Policies)==0` → refuse execute.  
3. If `len(AllowedTools)==0` and intent is side-effecting (kernel `intent_requires_tool_call`) → treat as misconfig.  
4. Prefer cloning slices when injecting into SubAgents from shared factory output.

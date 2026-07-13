# 09 — Safety and Invariants: JIT config

> Last verified against codebase: **2026-07-13**  
> Package: `internal/jit/config` + consumer enforcement sites

## 1. Package-local invariants

| ID | Invariant | Enforcement |
|----|-----------|-------------|
| I1 | Valid “full” config has non-empty identity after trim | `Validate()` |
| I2 | Valid “full” config has ≥1 unique canonical embedded policy reference | `Validate()` + `core.IsDefaultPolicyFile` |
| I3 | Package performs no I/O or network | Import set |
| I4 | Package performs no tool execution | Import set |
| I5 | Types are pure data (value-friendly) | No mutexes/pointers required on structs |

## 2. System invariants (consumers must uphold)

| ID | Invariant | Status |
|----|-----------|--------|
| S1 | Tool calls from the model must be in `AllowedTools` | **Enforced** for modular and Ouroboros catalog/execution paths; nil/empty/unlisted deny before handler execution |
| S2 | Specialist names cannot path-escape `.nerd/agents` | **Enforced** in `loadSpecialistConfig` |
| S3 | Specialist YAML size bounded | **Enforced** (1 MiB) |
| S4 | Production agents always pass `Validate()` | **Partial** — specialist YAML validates; generated/empty fallback paths are not uniformly validated |
| S5 | `RequirePolicyEnforcement` true has a distinct runtime meaning | **Not enforced**; canonical references are validated, but the flag is unread in session |
| S6 | ToolLoop limits bound runaway tool thrash | **Partial** — executor limits exist, schema ToolLoop not used |
| S7 | Constitutional `permitted(...)` still applies | **Owned by core/policy**, independent of this schema |
| S8 | Default deny for unknown tools | **Enforced at the JIT capability gate** — nil/empty/unlisted deny; downstream constitutional permission remains separate and mandatory |

## 3. Constitutional safety relationship

codeNERD default deny:

```
action executes only if permitted(...) derives true
```

`EffectiveAgentRuntimeConfig` is **not** a substitute for Mangle permission. It is a **capability envelope** for the LLM creative surface:

- Narrows which tools the model is **offered** and **allowed by name**.  
- Carries canonical members of core's embedded global boot inventory. Default
  references resolve and validate, but are **not selectively applied per agent**.
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

Not applicable inside `internal/jit`. Policy paths are validated against
Decl-bearing embedded corpora under `internal/core/defaults/policy/` and related
root `.mg` modules, but declarations and loading remain owned by core.

## 6. Threat model (config-centric)

| Threat | Mitigation | Residual risk |
|--------|------------|---------------|
| Agent without identity | Validate at specialist boundary | Generated/fallback paths still need uniform validation |
| Agent without or with invented policies | Validate at specialist boundary and canonical core inventory | Generated/zero-value fallbacks may bypass validation; selective per-agent semantics remain absent |
| Path traversal via specialist name | Spawner name checks | Must keep checks on all load sites |
| Oversized YAML DoS | 1 MiB cap | Other load sites must share cap |
| Tool allowlist bypass | `isToolAllowed` applies to modular and Ouroboros tools; nil/empty deny | Remaining risk is producer/consumer drift, not registry-based capability |
| False safety via unused flags | Docs honesty + TODO wiring | Operators may trust YAML that is ignored |

## 7. Hard-gate status

1. **Verified:** call `Validate` after specialist YAML unmarshal and return its
   path-qualified error.
2. **Verified slice plus residual:** stable default set IDs resolve to unique
   canonical members of the embedded boot inventory and invented paths fail;
   carry set/version identity and define selective-versus-global enforcement.
3. **Partial:** nil/empty normal-mode grants are deny-all; any future read-only
   degradation still needs an explicit, separately tested mode.
4. **Verified:** intersect both modular and Ouroboros registries with the
   effective grants for catalog and execution.
5. **Open:** prefer cloning slices when injecting into SubAgents from shared factory output.

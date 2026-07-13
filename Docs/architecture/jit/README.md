# jit — Architecture Corpus (`internal/jit`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/jit/config`  
> Scale: **1** non-test Go file (59 lines) · **1** test file (67 lines) · **0** Mangle sources

## Scope

This corpus documents **`internal/jit`**: the **effective agent runtime config schema** that is the contract between:

1. **JIT producers** — `internal/prompt.ConfigFactory` (intent → tools/policies/identity) and optional specialist YAML under `.nerd/agents/<name>/config.yaml`
2. **Universal consumers** — `internal/session.Executor` / `Spawner` / `SubAgent` (tool allowlists, identity injection)

It is **not** the JIT prompt compiler itself (`Docs/architecture/prompt/`), **not** the session clean loop (`Docs/architecture/session/`), and **not** a product Spec template set (`Docs/Spec/`).

### Naming honesty

In codeNERD product language, “JIT” means the whole **just-in-time agent configuration** path:

```
intent → prompt atoms + ConfigAtoms → EffectiveAgentRuntimeConfig → universal executor
```

The **Go tree** `internal/jit/` only holds the **shared config types**. Compiler, atoms, and ConfigFactory live under `internal/prompt/`. Execution lives under `internal/session/`. This package is the **typed handshake** in the middle.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, field semantics |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and methods |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, session, specialist YAML |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Validation, policy requirements, fallbacks |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Package + consumer tests |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories and debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

### Superseded thin names (rebuild 2026-07-13)

Older auto-inventory filenames under this folder (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-JIT.md`, `03-GAP-ANALYSIS-JIT.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md`) are **replaced** by the map above. Prefer the new names.

## Verify

```powershell
# Package unit tests
go test ./internal/jit/...

# ConfigFactory consumers that construct EffectiveAgentRuntimeConfig
go test ./internal/prompt/ -run "ConfigFactory|DefaultConfigFactory"

# Session consumers of the schema
go test ./internal/session/ -run "Config|Spawner|Specialist|Executor"

# Optional: integration tests that import jit/config
go test ./tests/e2e/ -run "Specialist|Session|Config" -tags=integration
```

## Related corpora

| Corpus | Relationship |
|--------|----------------|
| [prompt](../prompt/) | ConfigFactory + JIT compiler produce this schema |
| [session](../session/) | Universal executor / spawner consume it |
| [core](../core/) | Kernel + VirtualStore execute permitted tools |
| [cli](../cli/) | Boot wires `NewDefaultConfigFactory`; UI JIT pages |
| [tools](../tools/) | Tool names in `AllowedTools` resolve via registry |

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring journals, honest partial wiring of schema fields — **not** auto-generated inventory stubs.

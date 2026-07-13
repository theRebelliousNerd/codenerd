# system — Architecture Corpus

> Last verified against codebase: **2026-07-13**  
> Status: Living reference — code-grounded full corpus  
> Package: `internal/system/`  
> Role: **Motherboard factory** — boots and caches the fully wired `Cortex` runtime

## Scope

`internal/system` is the single assembly point that wires codeNERD’s runtime stack into a `Cortex` value: kernel, VirtualStore, perception, JIT prompts, shards, session executors, stores, embeddings, browser, autopoiesis, and holographic code scope.

It is **not** the OODA loop itself, **not** the Mangle policy corpus, and **not** the CLI command tree. Those live in `session`, `core`/`defaults/policy`, and `cmd/nerd` respectively. This package **constructs** the object graph those surfaces consume.

Primary entry points:

| Entry | When to use |
|-------|-------------|
| `GetOrBootCortex` | **All CLI command handlers** — keyed cache, maintenance schedule |
| `BootCortex` / `BootCortexWithConfig` | Direct boot (tests, TUI shared boot with overrides) |
| `Cortex.Close` | Teardown + cache eviction |

## Source snapshot

| Kind | Count | Notes |
|------|------:|-------|
| Non-test `.go` | **5** | factory, adapters, agent registry, holographic scope, close |
| Test `.go` | **11** | unit, boot, DOM e2e, adapters |
| Package `.mg` | **0** intended | `debug_program_ERROR.mg` is a crash dump artifact, not source |
| Approx. source LOC | **~2,100** | Dominated by `factory.go` (~1,150 lines) |

## Document map

| Doc | Purpose |
|-----|---------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living spec — boot pipeline, GetOrBootCortex, inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target architecture vision for the motherboard |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, boot stages, data/control flow |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported surface with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream imports and reverse consumers |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | CLI / TUI / kernel / shard registration paths |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Cache, concurrency, lifecycle, soft-fail policy |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Existing tests, gaps, verify commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories and debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Corpus rebuild progress |

## Fact-flow position

```
cmd/nerd  ──GetOrBootCortex──►  Cortex
chat TUI  ──BootCortexWithConfig──►  Cortex
                                    │
         user input → perception → user_intent → kernel next_action
              → VirtualStore / shards / tools → articulation
```

`system` does not implement that loop; it **wires** every participant so the loop can run.

## Verify

```powershell
go test ./internal/system/...
go test ./internal/system/... -short   # skips full BootCortex e2e
```

Full boot e2e (no network LLM required — uses `missingLLMClient` when unconfigured):

```powershell
go test ./internal/system/ -run TestBootCortexEndToEnd -count=1
```

## Related corpora

- [`../core/`](../core/) — kernel, VirtualStore, API scheduler  
- [`../session/`](../session/) — Executor / Spawner / TaskExecutor  
- [`../prompt/`](../prompt/) — JIT compiler, atom loader  
- [`../cli/`](../cli/) — primary reverse consumer of `GetOrBootCortex`

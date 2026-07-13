# testing — Dependency Map

> Last verified: 2026-07-13

## Upstream (what context_harness imports)

| Package | Usage | Evidence |
|---------|-------|----------|
| `codenerd/internal/core` | `*RealKernel`, `core.Fact`, `LoadFacts`, kernel factory | engines, seeder, factory, simulator types |
| `codenerd/internal/context` | `ActivationEngine`, `Compressor`, activation context types, `CompressorConfig` / `DefaultConfig` | `real_engine.go`, `engine_interface.go`, CLI |
| `codenerd/internal/perception` | `LLMClient` for live responses | `real_engine.go` |
| `codenerd/internal/store` | `*LocalStore` for compressor construction | `real_engine.go` |
| stdlib | `context`, `sync`, `io`, `os`, `encoding/json`, `time`, `fmt`, `strings`, `path/filepath` | throughout |

### Import direction (allowed)

```
context_harness
  → core
  → context
  → perception
  → store
```

**Forbidden reverse:** core/context/perception/store must **not** import `context_harness`.

## Downstream (who imports context_harness)

| Consumer | Path | Role |
|----------|------|------|
| CLI | `cmd/nerd/cmd_test_context.go` | Sole operator entry; boots Cortex; constructs engines + harness |

Grep evidence (module-wide): only `cmd/nerd/cmd_test_context.go` imports `codenerd/internal/testing/context_harness`.

Parent package `codenerd/internal/testing` is not imported by production code.

## CLI transitive graph

```
cmd/nerd (test-context)
  → context_harness
  → coresys.GetOrBootCortex   (internal/system)
  → cortex.Kernel (*core.RealKernel)
  → cortex.LocalDB (*store.LocalStore)
  → cortex.LLMClient (perception)
  → internalcontext.DefaultConfig()
  → NewRealIntegrationEngine | NewMockContextEngine
  → NewHarnessWithObservability
```

## Peer systems (not imported, but related)

| Peer | Relationship |
|------|----------------|
| `internal/prompt` | Observed conceptually via JITTracer; not imported |
| `internal/campaign` | Campaign phases appear in scenarios/metadata; campaign pager not directly called |
| `internal/session` | Production OODA executor — separate test surface |
| `internal/articulation` | Piggyback control packets mirrored in tracers; production path not invoked in mock |

## Dependency risks

| Risk | Detail |
|------|--------|
| Heavy CLI path | Even mock mode boots full Cortex via `GetOrBootCortex` |
| Kernel interface drift | CLI type-asserts `*core.RealKernel`; alternate kernel types fail loud |
| Activation API drift | Real engine maps `ScoredFact` fields into `ActivationBreakdown` — renames break compile |
| Perception API | Live mode depends on `CompleteWithSystem` |
| Store coupling | Compressor construction needs LocalStore even if compress path underuses it |

## Layering diagram

```
┌────────────────────────────────────────┐
│ cmd/nerd  (operator)                   │
└─────────────────┬──────────────────────┘
                  │
┌─────────────────▼──────────────────────┐
│ internal/testing/context_harness       │
│ (test-only orchestration)              │
└───┬─────────┬──────────┬───────────┬───┘
    │         │          │           │
    ▼         ▼          ▼           ▼
 internal/  internal/  internal/  internal/
   core     context   perception   store
```

## Mangle surface

No `.mg` files in this package. Predicates are:

- **Emitted** as `core.Fact` from engines  
- **Parsed** from strings in seeder/factory  
- **Assumed** to be acceptable to the booted kernel’s schema/defaults  

Schema ownership remains in `internal/core/defaults/` and related Mangle corpora — not here.

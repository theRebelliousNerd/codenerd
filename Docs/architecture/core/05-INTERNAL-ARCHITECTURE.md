# core — Internal Architecture

> Last verified: **2026-07-13**

## 1. Component diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Consumers                                  │
│   session.Executor · system boot · cmd/nerd · domain shards · e2e       │
└────────────┬───────────────────────────────┬────────────────────────────┘
             │ Kernel API                    │ RouteAction / Exec / tools
             ▼                               ▼
┌────────────────────────┐      ┌────────────────────────────────────────┐
│ RealKernel             │◄────►│ VirtualStore                           │
│  facts[] + factIndex   │ set  │  tactile Executor                      │
│  store / programInfo   │kernel│  constitution[]                        │
│  schemas/policy/learned│      │  Dreamer ──► Clone+panic_state         │
│  eventBus              │◄─────│  injectFacts                           │
│  evaluate()            │      │  validators / TransactionManager       │
└───────────┬────────────┘      │  shards.ShardManager / TaskDelegator   │
            │                   │  MCP clients / tools / CodeScope       │
            ▼                   └────────────────────────────────────────┘
   mangle-go engine
   (stratify + eval)

Optional:
┌──────────────────┐     ┌─────────────────┐
│ CortexKernel     │────►│ KernelShard×N   │
│ predicateOwner   │     │ owned preds     │
│ ShardFactRouter  │     │ inner RealKernel│
└──────────────────┘     └─────────────────┘
```

## 2. RealKernel state machine (evaluation)

```
                 NewRealKernel
                      │
                      ▼
              loadMangleFiles
                      │
                      ▼
              evaluate() [eager boot]
                      │
          ┌───────────┴───────────┐
          │ initialized=true      │
          └───────────┬───────────┘
                      │
     Assert / Retract / LoadPolicy / HotLoad
                      │
          ┌───────────▼───────────┐
          │ policyDirty?          │──yes──► rebuildProgram (parse+analyze+stratify)
          └───────────┬───────────┘         invalidate diff engine
                      │ no
          ┌───────────▼───────────┐
          │ factsDirty / eager?   │
          └───────────┬───────────┘
                      │
          ┌───────────▼───────────┐
          │ full eval or diff     │
          │ update store          │
          │ clear dirty           │
          └───────────────────────┘
                      │
     Query / QueryAll ──ensureEvaluated──► results
```

### Key fields (`kernel_types.go`)

| Field | Purpose |
|-------|---------|
| `facts` / `cachedAtoms` / `factIndex` | EDB + dedup + atom cache |
| `store` | mangle-go FactStore post-eval |
| `programInfo` / `strata` / `predToStratum` | Compiled program |
| `schemas` / `policy` / `learned` | Program source layers |
| `policyDirty` / `factsDirty` | Rebuild triggers |
| `diffEngine` | Optional incremental eval |
| `proofRecorder` | Optional provenance |
| `virtualStore` | Virtual predicate callbacks |
| `eventBus` | Mutation pub/sub |
| `maxFacts` / `derivedFactLimit` | Safety caps |

## 3. VirtualStore RouteAction state machine

```
fact in
  │
  ├─ bootGuardActive ──► error (blocked)
  │
  ├─ parseActionFact ──► error
  │
  ├─ isDestructive? ──yes──► Dreamer.SimulateAction
  │                              │
  │                         unsafe? ──► security_violation + error
  │
  ├─ checkConstitution ──► error + security_violation
  │
  ├─ CheckKernelPermitted ──false──► deny + security_violation
  │
  ├─ executeAction(handler)
  │       │
  │       fail ──► execution_error
  │
  ├─ validators ── high-confidence fail ──► flip Success
  │
  └─ inject execution_result + FactsToAdd + audit/events
```

## 4. Dreamer data flow

```
ActionRequest
    │
    ├─ cache lookup (type:target)
    │
    ├─ assert hypothetical
    ├─ projectEffects → projected_action / projected_fact*
    │
    ▼
kernel.Clone()
    │
    assert projected (WithoutEval) + Evaluate
    │
    Query panic_state
    │
    ├─ match actionID → Unsafe + reason
    └─ else Safe
    │
    store cache
```

Critical path prefixes (Go + Mangle facts): `.git`, `.nerd`, `internal/mangle`, `internal/core`, `cmd/nerd`.

## 5. Cortex routing

```
Assert(fact) / Query(pred)
        │
        ▼
predicateOwner[pred] ?
        │
   yes ─┴─ no → cortexDomain shard
        │
   KernelShard.inner kernel
        │
   optional ShardFactRouter if per-shard feature on
```

## 6. Fact categories (context helpers)

`fact_categories.go` classifies predicates for paging/activation consumers (intent, world, diagnostic, action, context, learning, session). Used by context/campaign/prompt systems, not by Mangle engine itself.

## 7. Scheduler (API)

`APIScheduler` tracks per-shard phases (`ShardPhase`) and slots so concurrent LLM work does not thrash shared rate limits. `ScheduledLLMCall` wraps `types.LLMClient` to acquire slots around `Complete`/`Stream` style calls.

## 8. Validation registry

`ValidatorRegistry` maps action classes to `ActionValidator` implementations (file write/edit/delete, exec, syntax, CodeDOM, paranoid). Aggregate confidence decides whether VS should treat a “successful” I/O as failed.

## 9. Transaction manager

`TransactionManager` coordinates multi-file edits with shadow validation (parse/safety) before commit — complementary to Dreamer (which is pre-route speculative logic), operating closer to multi-file apply semantics.

## 10. Embed & hybrid load

```
coreLogic embed.FS
  defaults/schemas.mg + schemas_*.mg  → schemas string
  defaults/policy/*.mg + root modules → policy string
  defaults/learned.mg + user learned  → learned string
  hybrid LoadHybridMangleFile(.nerd/mangle/*)
       → Logic append + bootFacts/intents/prompts
```

## 11. Threading model (summary)

| Component | Locking |
|-----------|---------|
| RealKernel | `mu` RWMutex; atomic `factsDirty`; eval singleflight mutex |
| VirtualStore | `mu` for config/DI; handlers take locks narrowly |
| Dreamer | `mu` for kernel/cache pointers; cache own mutex |
| CortexKernel | `mu`; avoid lock inversion with shard.mu (documented) |
| ShardManager | `mu` over maps; spawn queue separate |

## 12. Extension points

| Extension | How |
|-----------|-----|
| New Decl | `defaults/schemas_*.mg` + ensure load list if new file |
| New rule | `defaults/policy/*.mg` |
| New effect | ActionType + handler switch + permitted/safe_action |
| New virtual predicate | VS predicates module + kernel virtual hooks |
| New validator | Implement ActionValidator; register in `initValidators` |
| User override | `.nerd/mangle/extensions.mg`, `policy_overrides.mg`, `learned.mg` |

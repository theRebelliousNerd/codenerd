# Internal architecture: shards

## Component topology

```text
internal/shards
  registration ---------- factories, profiles, canonical ownership manifest
  matching -------------- heuristic candidate scoring and classifications
  consultation ---------- spawned advice and short-lived cache
  observer_manager ------ event fan-out, Northstar path, assessment buffer
  requirements_interrogator -- ephemeral JIT-backed ShardAgent
       |
       +-- system/
             base -------- DI, state, CostGuard, autopoiesis, event subscription
             perception -- language -> structured intent
             executive --- strategies/barriers -> pending action envelope
             constitution - exact permission + denial/appeal audit
             router ------- permitted envelope -> route -> VirtualStore
             world_model -- workspace facts
             planner ------ agenda/checkpoint state
             campaign_runner -- long-horizon supervisor
             legislator --- structured candidate-rule synthesis
             mangle_repair - learned-rule validation/repair interceptor

external owners
  internal/core/shards ---- spawn queue, active/result maps, panic cleanup
  internal/core ----------- Cortex/RealKernel, policy, VirtualStore
  internal/prompt --------- atoms and JIT compiler
  internal/session -------- persona clean-loop execution
```

## Action sequence

```text
Intent input
  -> PerceptionFirewallShard.Perceive
  -> user_intent
  -> Mangle derives next_action
  -> ExecutivePolicyShard.evaluatePolicy
  -> pending_action(ID, Type, Target, Payload, Ts)
  -> ConstitutionGateShard.processPendingActions
       -> permission_check_result(ID, /deny, Reason, Ts)
          + routing_result(ID, /failure, Reason, Ts)
          + security_violation(Type, Reason, Ts)
       OR
       -> permission_check_result(ID, /permit, Reason, Ts)
          + permitted_action(ID, Type, Target, Payload, Ts)
  -> TactileRouterShard.processPermittedActions
  -> route_action(ID, Tool) preference or deterministic local route
  -> VirtualStore.RouteAction(next_action(ID, Type, Target, Payload))
  -> routing_result + execution_result
```

The router's `RequiresSafe` flag documents the route contract but does not grant
safety. The permitted fact and VirtualStore's exact revalidation do.

## Registration and override layers

```text
RegistryContext
  -> RegisterAllShardFactories
       -> package factories
       -> profiles
  -> runtime enricher
       Cortex: router browser + campaign manager + shared assembler
       Chat: GlassBox + ToolEventBus + ToolStore + learning candidates
       Campaign/init: reduced dependencies
  -> disabled-name filter
  -> StartSystemShards
       profiles StartupAuto + valid activate_shard/1 derivations
       -> SpawnQueue critical detached submissions
```

**PARTIAL:** the intended layering exists, but enrichers replace factories
manually and have no shared descriptor/parity contract. The exported ownership
manifest is not in this construction path.

## Shard state and concurrency

| State | Owner | Lock/bound | Lifetime |
|---|---|---|---|
| active shards, results, profiles, factories | core ShardManager | manager mutex; limits enforcer | process/session |
| queued spawn requests | SpawnQueue | four bounded channels; workers; deadline | until spawn/timeout/stop |
| system shard state and dependencies | BaseSystemShard | RWMutex; one StopCh/event subscription | one execution |
| cost counters | CostGuard | mutex; minute/session/retry caps | shard instance |
| unhandled/proposed/applied rules | AutopoiesisLoop | mutex; threshold but no durable receipt | shard instance |
| consultation pending/cache | ConsultationManager | RWMutex; 100 entries; five minutes | manager instance |
| observer events/assessments | BackgroundObserverManager | RWMutex + atomic count; buffers 100/100 | one constructor context |
| router routes/rate limits/calls | TactileRouterShard | shard and limiter mutexes | shard instance |
| durable learning | LearningStore | store owner | workspace |

The race suite proves the exercised accesses are race-free, including a restarted
observer generation. Focused route tests prove exactly-once terminal behavior in
both unmapped modes. Bounded unconsumed manager results and observer-drop
telemetry remain unproven.

## System loops

Most continuous shards subscribe to relevant FactBus predicates. If no bus is
available, selected shards use bounded tickers. Heartbeats are periodic and
facts are pruned or upserted to limit evaluate pressure. Context cancellation and
StopCh are terminal; on-demand shards may also stop on idle conditions.

Executive waits for its tracked autopoiesis goroutines before completing.
Background observer Stop waits separately for event loops and spawned tasks,
drains stale events, and a later Start creates a fresh context generation.

## Specialist collaboration flow

```text
verb + files + on-disk agent registry
  -> DefaultVerbConfigs
  -> CoreTechnologyPatterns score extension/path/import/content
  -> registered ready agent mapping
  -> typed executor/advisor/observer classification
  -> top-N SpecialistMatch
       -> execute via TaskExecutor/session
       -> consult via ConsultationSpawner
       -> observe via BackgroundObserverManager
```

Candidate matching is creative routing support, not policy. `CanExecute` and
confidence do not bypass tool permissions.

## JIT seams

`RegistryContext` creates PromptAssemblers with configured token budget,
reserved tokens, semantic top-k, and enabled flag. Concrete shards select by
ShardType and sometimes semantic query. Required-JIT paths fail visibly;
optional autopoiesis skips creative proposals. Mangle repair and consultations
are compatibility seams that do not share one policy yet.

## Compatibility and bypass paths

- `DefaultShardPredicateManifests`: now consumed by production KernelShard
  construction; factory/profile/runtime-enricher metadata remains distributed.
- permissive-unmapped router mode: disabled by default; when enabled it records
  one learning case, emits one failure, and consumes the permission.
- observer restart: fresh contexts and separate loop/task joins are verified;
  input overflow still drops without a counter.
- scanner registration with empty RegistryContext: useful for discovery but not
  proof that spawned dependency-heavy shards are ready.
- direct session tool execution: outside tactile-router fact flow, still expected
  to pass VirtualStore/constitutional validation.

These are explicit audit targets, not deletion candidates.

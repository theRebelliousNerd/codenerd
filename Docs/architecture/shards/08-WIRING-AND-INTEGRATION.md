# Wiring and integration: shards

## Production construction

```text
BootCortexWithConfig
  -> initCoreComponents / storage / kernel / execution / intelligence
  -> ShardManager + discovered user-agent profiles
  -> initShardManagement
       -> LimitsEnforcer
       -> SpawnQueue.Start
       -> RegistryContext
       -> RegisterAllShardFactories
       -> JIT DB registrar/unregistrar
       -> router factory override (browser + shared assembler)
       -> campaign factory override (manager + shared assembler)
       -> DisableSystemShard set
       -> StartSystemShards
  -> final session executors
```

`internal/system/factory.go#initShardManagement` is the main Cortex integration.
It intentionally enriches two factories after package registration. That is live
behavior, but a manual override must remain in parity with base construction.

## Boot selection and readiness

`internal/core/shards/manager_spawn.go#ShardManager.StartSystemShards` selects:

1. profiles with system type and StartupAuto;
2. additional valid system profiles named by `activate_shard/1`;
3. minus explicitly disabled names.

With a running SpawnQueue, each is a critical detached request. The method
proves successful submission, not successful execution or readiness. Per-shard
spawn failures are logged and iteration continues.

**PARTIAL:** BootCortex may finish before every auto shard enters its Execute
loop. There is no boot generation, required-ready set, or aggregate degraded
result.

## Chat wiring

The current chat stack has shared Cortex boot and compatibility construction
surfaces. Chat adds dependencies that a headless Cortex may not have:

| Dependency | Recipient/use |
|---|---|
| GlassBoxEventBus | shard lifecycle and router visibility |
| ToolEventBus and ToolStore | visible/persisted tool outcomes |
| BrowserManager | browser tool routes |
| classification client | lower-cost perception classification |
| learning-candidate store | taxonomy/perception feedback |
| post-spawn hook | on-demand shard enrichment |
| observer and consultation managers | Northstar and specialist advice |

The system-shards master feature and per-name disable lists are applied by chat
surfaces. `NERD_DISABLE_SYSTEM_SHARDS` is a per-name compatibility input; it is
not the same as the master system-shards flag.

## Other live consumers

- campaign CLI registers factories and consultation adapters for long-horizon
  work;
- init scanning performs structural registration with a reduced context;
- chat delegation uses matching and consultation helpers;
- shared boot constructs background observers and a direct Northstar handler;
- action-linter tooling imports route definitions;
- session TaskExecutor handles persona work and may call VirtualStore without
  spawning a legacy domain shard.

A reduced registration context proves discoverability, not readiness. Dependency
heavy shards must fail visibly or remain unspawned when required components are
absent.

## Action dispatch wiring

| Step | Producer | Consumer | Exact contract |
|---|---|---|---|
| decision | Mangle | executive | `next_action` plus source fact |
| permission request | executive | constitution | `pending_action(ID, Type, Target, Payload, Ts)` |
| permit/deny receipt | constitution | Mangle/operators | `permission_check_result(ID, Result, Reason, Ts)` |
| route input | constitution | router | `permitted_action(ID, Type, Target, Payload, Ts)` |
| policy route | Mangle | router | `route_action(ID, Tool)` |
| effect request | router | VirtualStore | `next_action(ID, Type, Target, Payload)` |
| terminal results | router/VirtualStore | session/policy/operators | `routing_result/4`, `execution_result/6` |

Production `defaultKernelShardConfigs` places the exact permission envelope in
one policy domain. The shards package's exported manifest does not currently
drive this wiring and is missing three members of that envelope.

## Lifecycle and teardown

ShardManager injects dependencies and active/status facts before launching one
execution goroutine. On normal return or error it records the result, retracts
active/status facts, removes the shard, and unregisters a dynamic JIT DB. Panic
recovery follows a cleanup path. Cortex Close stops maintenance, spawn queue,
active shards, and stores outside this package.

System shard Stop flushes learning and closes StopCh only from running state.
Event subscriptions are removed. Executive joins tracked autopoiesis work.
Background observer Stop cancels and joins loops then tasks, drains stale events,
and a later Start owns a fresh run context.

## Wiring checklist for a new shard

1. Pin creative versus deterministic responsibility and fact contracts.
2. Add declarations/policy to core before asserting new predicates.
3. Define constructor, dependencies, profile, startup, resource bounds, health,
   cancellation, and terminal outcome.
4. Register in the package descriptor/registrar and every temporary adapter;
   add parity tests.
5. Add prompt atoms and explicit JIT fallback policy for every LLM call.
6. Prove default-deny and exact action correlation for effects.
7. Test missing dependencies, disable, cancellation, panic, restart/one-shot,
   and teardown.
8. Update this corpus only from runnable evidence.

## Known compatibility and bypass routes

| Route | Status | Required proof |
|---|---|---|
| exported predicate manifest | production authority; uniqueness and exact envelope verified | preserve parity as descriptor expands |
| per-shard facts feature surface | manifest now feeds production configs | broader end-to-end ownership routing, cross-domain join, rollback |
| permissive unmapped router | both modes consume once and emit terminal failure | preserve regression on new branches |
| observer restart | fresh generation verified under race | add overflow/drop counter and snapshot ownership |
| direct session tool calls | bypass tactile router, but should not bypass VirtualStore permission | constitutional negative integration test |
| scanner empty-context factories | structural registration only | fail-visible spawn dependency tests |

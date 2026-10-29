# Observability: shards

## Correlation hierarchy

| Identity | Current scope | Strongest surfaces |
|---|---|---|
| executive ActionID | permission through effect | pending/permitted/permission/routing/execution facts, ToolStore |
| ShardID | one spawned execution | audit, active/status facts, Glass Box, result map |
| task/request ID | consultation/planner/campaign-specific | manager structs and subsystem facts |
| session ID | chat/tool persistence | ToolStore and usage context |
| boot generation | **absent** | proposed activation receipt |

ActionID is the best current cross-component key. The pipeline test proves it is
not replaced by the router.

## Signal inventory

### Logs and events

- system shard info/debug/error categories expose lifecycle, policy, JIT, and
  budget failures;
- routing and tool categories expose route selection and effect execution;
- audit logs record shard spawn/execute/complete;
- Glass Box emits immediate shard lifecycle and selected router events;
- ToolEventBus shows bounded tool results; ToolStore persists full execution
  records under its own retention contract.

### Kernel facts

`system_heartbeat`, campaign/world/planner status, `executive_error`,
`executive_blocked`, `ooda_timeout`, `permission_check_result`,
`security_violation`, `routing_result`, and `execution_result` provide structured
diagnosis. Permission and routing results prune on bounded cadence; heartbeat
updates are optimized in core.

### In-memory metrics

Executive decisions/blocks/strategies, CostGuard call/retry counts, router calls,
observer events/assessments, queue depth/reject/timeout/spawn metrics, plan
progress, and constitution violations exist across shard and manager objects.

## Known blind spots

- consultation failures are returned but not normalized into a cross-operation
  lifecycle receipt;
- observer input overflow is silently dropped and has no counter;
- boot has no required/optional readiness outcome or generation;
- inline/fallback JIT calls do not emit one normalized atom/budget receipt;
- unobserved asynchronous manager results have no bounded operator-facing
  retention metric in this corpus;
- Cortex and chat enrichers do not expose a single factory-parity report.

## Operator diagnosis

### Intent exists but no effect

Trace `user_intent` -> `next_action` -> `pending_action` ->
`permission_check_result` -> `permitted_action` -> `routing_result` ->
`execution_result`. Check executive boot guard, barriers, constitution reason,
router activity, route mapping, rate limiter, and VirtualStore error in that
order. Keep the same ActionID throughout.

### Specialist advice is missing

Inspect the requested target list, on-disk agent readiness, matcher scores,
TaskExecutor spawn results, and consultation adapter. Current batches return
joined errors while preserving successful peers; inspect both values.

### System shard appears alive but does nothing

Compare active/status facts, heartbeat age, ShardManager active list, and Execute
loop logs. Background observers now restart with fresh run contexts; missing
assessments can still reflect uncounted channel overflow or a failing handler.

## Proposed lifecycle receipt

A future redacted, size-capped receipt should include version, boot generation,
ShardID/type, task/action correlation, descriptor fingerprint, required/optional
dependencies, queue/start/ready/end timestamps, terminal status/error class,
effective JIT atom IDs and budget summary, dropped-event counters, and rollback
mode. It must exclude raw prompts, secrets, full source, and unbounded outputs.

The receipt is diagnostic. It cannot satisfy `permitted/3` or a health gate by
itself.

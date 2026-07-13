# Failure modes and recovery: shards

| Failure | Symptom | Detection | Current containment | Residual / recovery |
|---|---|---|---|---|
| stale boot action | effects without a new user turn | boot guard, repeated pending facts | executive clears stale intent/pending state and starts guarded | boot readiness remains asynchronous; disable guard only on real ingress |
| missing exact permission | action never reaches tool | correlated deny reason | strict constitution and VirtualStore fail closed | repair policy/declarations; never use `safe_action/1` as fallback |
| authorization envelope split | permitted join disappears under per-shard stores | manifest uniqueness and exact Cortex envelope tests | canonical manifest drives production and co-locates all four predicates | preserve tests as factory/profile descriptors evolve |
| action ID drift | waiter cannot join effect result | compare pending, routing, execution IDs | router now preserves executive ID; pipeline regression | require exact-ID tests on every adapter |
| unmapped route | correlated no-handler failure | `routing_result(/failure, "no_handler")` | both modes consume once; learning mode records one case; second-pass regression | preserve completePermittedAction on every terminal branch |
| batch consultation target fails | partial responses plus non-nil joined error | per-target error text and input-ordered successes | partial/total/nil-spawner regressions | callers must inspect both responses and error; pin cancellation/cache identity next |
| observer restart | new generation misses events | Start/Stop/Start regression under race | fresh context, separate loop/task joins, stale-event drain | diagnose handler failure/overflow; keep Stop idempotent |
| observer overload | stale/missing assessments | no drop metric | 100-event buffer, direct Northstar handler | add drop counter/backpressure/coalescing; never call absence alignment success |
| hollow factory | nil kernel/client/store behavior | dependency logs, spawn error, absent facts | RegistryContext and runtime enrichers | scanner/reduced contexts are discovery only; fail visibly on real spawn |
| boot shard fails | Cortex returns but participant never ready | logs, missing active/heartbeat | manager continues other starts | no aggregate readiness; inspect required spine before dependent effects |
| queue saturation | spawn rejected or times out | queue metrics/errors | priority/backpressure/deadlines | preserve named failure in operation outcome; avoid unbounded retries |
| LLM storm | provider throttling/cost rise | CostGuard logs/counters | minute/session caps and cooldown | audit every system LLM call; optional cognition degrades without effects |
| JIT unavailable | interrogator/planner/legislator error or optional autopoiesis skip | explicit log/error | required paths fail, optional proposal skips | normalize fallback policy; do not add hidden prompt constants |
| invalid learned rule | parse/evaluate failure, debug dump | repair result and kernel validation | synth/schema/safety/stratification and finite retries | verify interceptor on every boot/persistence route; reject is terminal |
| system goroutine outlives shard | state mutates after completion | race/leak test, post-stop events | executive joins tracked autopoiesis; observers join loops/tasks per generation | apply owned-work WaitGroups/generation cancellation consistently |
| campaign auto-runs | unexpected long-horizon work | active campaign facts/logs | campaign runner profile on demand | require explicit action/activation decision to change startup |
| payload drift | target/intent data lost or permission mismatch | exact envelope facts, codec tests | shared encode/decode helpers | reject malformed canonical payload rather than guessing authority |
| result retention grows | memory growth after async spawns nobody awaits | no complete package metric | synchronous and queued paths consume results | bound/expire unobserved results in core manager and emit tombstones |

## Recovery order for the action pipeline

1. Preserve the ActionID and snapshot the exact facts; do not reissue an effect.
2. Identify the last terminal or nonterminal stage.
3. If permission is absent/denied, repair input or policy and create a new action
   ID after user authorization; never mutate the old receipt into success.
4. If permission exists but routing is absent, consume the old permission into a
   failure before retrying with a new action.
5. If execution may have started, inspect `execution_started/completed/result`
   and tool-store records before retry; assume non-idempotent until proven.
6. Cancel/join dependent shard work and retract exact transient facts.
7. Record degradation and rollback mode in operator-visible output.

## Rollback boundaries

- registry uplift: restore a local table copied from the canonical manifest,
  retain full policy envelope and parity regressions;
- terminal outcomes: retain compatibility string/slice APIs as adapters over
  typed results, never erase errors;
- JIT migrations: disable only the migrated atom family and use its declared
  compatibility behavior;
- activation generations: turn off readiness gating while leaving receipts
  diagnostic-only;
- router repair: force `AllowUnmappedActions=false` if a future branch cannot
  preserve the verified exactly-once semantics.

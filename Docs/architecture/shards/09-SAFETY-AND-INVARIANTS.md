# Safety and invariants: shards

## Effect authority

### S1. Exact permission is mandatory

An external effect requires `permitted(ActionType, Target, Payload)` for the
same envelope submitted by the executive. `safe_action/1`, a profile permission,
a route match, an LLM judgment, or a health fact is classification only.

### S2. Correlation is immutable

The executive owns ActionID. Constitution, router, and VirtualStore preserve it.
Payload is encoded canonically between facts. The action pipeline regression
proves one read-file ID reaches `execution_result`.

### S3. Default deny survives dependency failure

Strict constitution mode denies on absent or failed permission query. VirtualStore
denies if kernel, Dreamer, exact permission, or validated projection is missing.
Unmapped router actions fail by default.

### S4. The authorization join stays co-located

Production KernelShard policy ownership includes `pending_action`,
`permitted_action`, `permission_check_result`, and `permitted`. Splitting this
set can make exact joins disappear across shard-local stores. The canonical
manifest now drives production configs, rejects duplicate ownership in tests,
and has an exact Cortex target/payload mismatch regression.

### S5. Denial is observable and schema-correct

Constitution emits `permission_check_result/4`, `routing_result/4`,
`security_violation/3`, and `appeal_available/4` as applicable. Specialist/runtime
errors use their declared arities; consumers must not invent richer facts without
updating declarations and tests.

## Deterministic guard order

`ConstitutionGateShard.checkPermitted` evaluates active overrides, dangerous
content, network allowlist, and exact Mangle permission. Model-proposed rules are
outside the immediate decision and must pass repair/schema/stratification gates
before policy changes.

Appeals are explicit state. Temporary override expiry is timestamp-based.
Approval changes future permission evaluation; it does not let the router skip
the current correlated envelope.

## Boot and lifecycle safety

- Executive boot guard begins active and suppresses actions until genuine user
  interaction disables it.
- Executive clears stale intent/processed/pending EDB facts when its loop starts.
- Campaign runner remains on demand.
- Auto-start queue submission does not prove readiness; a future readiness gate
  must distinguish required and optional shards.
- Cancellation and Stop must join owned goroutines or identify detached work.
- Background observer Start creates a fresh generation; Stop joins loops/tasks,
  drains stale events, and is idempotent.

## Resource bounds

| Boundary | Current bound |
|---|---|
| system LLM calls | CostGuard minute/session caps and error cooldown |
| learned-rule repair | per-rule and session validation budgets |
| executive action storm | MaxActionsPerTick |
| spawn pressure | total/per-priority queue, workers, deadlines, limits enforcer |
| observer input/history | 100 events / 100 assessments; overflow currently silent |
| consultation cache | 100 entries, five-minute freshness |
| permission/routing history | 15-minute retention with prune cadence |
| tool visibility | bounded event preview; ToolStore owns full persistence |

Silent dropping is not a complete bound. Observer overflow and consultation
failure need counters/outcomes.

## Mangle contract

Declarations live under core defaults; shards are producers/consumers.

| Predicate | Arity | Producer -> consumer |
|---|---:|---|
| `pending_action` | 5 | executive -> constitution/core policy |
| `permission_check_result` | 4 | constitution -> core policy/operators |
| `permitted_action` | 5 | constitution -> router |
| `permitted` | 3 | core policy -> constitution/VirtualStore query |
| `route_action` | 2 | core policy -> router |
| `routing_result` | 4 | constitution/router -> policy/session |
| `security_violation` | 3 | constitution -> policy/audit |
| `exec_request` | 5 | router fallback -> effect integration |
| `active_shard` | 2 | manager -> prompt/policy context |
| `shard_status` | 3 | manager -> lifecycle consumers |
| `system_heartbeat` | 2 | system shards -> health policy |
| `activate_shard` | 1 | core policy -> StartSystemShards |

Atoms such as `/permit`, `/deny`, `/success`, and `/failure` remain Mangle names,
not arbitrary strings. Go must bind all variables before any negation in policy;
aggregation rules belong in core policy and use the Mangle pipeline syntax.

## Learned-rule boundary

Legislator requires structured synthesis. Mangle repair checks syntax, unsafe
variables/negation, declared predicates and arity, and stratification, then uses
a finite repair loop. Core schema validation protects control-plane heads.

**PARTIAL:** Mangle repair retains a legacy prompt fallback and the package suite
does not prove every boot path installs the interceptor before learned-rule
persistence.

## Safety negative controls

Required tests for any safety-affecting change:

1. missing exact permission denies;
2. target or payload mismatch denies despite matching action type;
3. dangerous shell/network content cannot reach the executor;
4. action ID survives permit, route, and effect;
5. envelope predicates cannot be split across configured kernel shards;
6. an unmapped permission remains consumed once with one terminal result in both
   learning-disabled and learning-enabled modes;
7. stale heartbeat/readiness cannot satisfy a fresh boot generation;
8. canceled/failed optional specialist cannot authorize or erase the primary
   outcome;
9. receipts redact secrets and enforce size/retention limits.

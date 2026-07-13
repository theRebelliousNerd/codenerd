# Testing alignment: shards

## Verification ladder

```powershell
go test -count=1 ./internal/shards/...
go test -race -count=1 -timeout=240s ./internal/shards/...
go test -count=1 ./internal/core/shards/... ./internal/system/...
go test -count=1 ./tests/e2e/...
```

Only the first two commands were run for this corpus receipt. Broader commands
are risk gates for product changes, not claimed passes here.

## Current evidence matrix

| Risk | Existing evidence | Verdict |
|---|---|---|
| factory/profile names | `registration_test.go#TestRegisterAllShardFactories` | **VERIFIED CURRENT** for package catalog |
| matcher classification and ranking | matching table/classification tests | **VERIFIED CURRENT** for heuristic matcher |
| consultation terminal results | single/success/partial/total/nil-spawner tests | **VERIFIED CURRENT**: ordered successes and joined errors; cancellation/cache semantics remain |
| observer concurrency and restart | integration + Start/Stop/Start regression + race suite | **VERIFIED CURRENT** for generation lifecycle; overflow metric absent |
| requirements no-LLM/no-JIT | interrogator tests | **VERIFIED CURRENT** for those branches |
| CostGuard, base state, payload codecs | base coverage tests | **VERIFIED CURRENT** for unit contracts |
| executive boot guard/OODA/delegation | executive tests | **VERIFIED CURRENT** for focused paths |
| constitution local rules/appeals | constitution coverage tests | **PARTIAL**: broad helper coverage, limited exact negative integration |
| route selection/no-route | route selection/escalation tests | **VERIFIED CURRENT** for default mode |
| exact action pipeline | `action_pipeline_test.go#TestPendingActionPipelineProducesRoutingResult` | **VERIFIED CURRENT** read-file permit/route/effect ID |
| Mangle repair helpers/selectors | repair and helper tests | **PARTIAL** end-to-end boot/persistence |
| world/planner/campaign lifecycle | helper/coverage tests | **PARTIAL** long-running recovery |

## Missing decisive gates

| Test | Positive control | Negative control | Why it matters |
|---|---|---|---|
| full descriptor parity | all boots enumerate one factory/profile/dependency set | missing runtime enricher fails visibly | predicate ownership is already unified; completes boot parity |
| consultation cancellation/cache | cancellation names target and preserves completed peers | cache hit cannot leak mutable manager state or wrong correlation | completes collaboration ownership |
| observer overflow | bounded coalescing/drop metric preserves diagnosis | overflow cannot silently imply healthy alignment | completes observer operations |
| router branch invariant | every new route terminal calls exact consumption | second cycle finds no permission/result amplification | preserves the now-verified fix |
| boot readiness | required set reaches ready | failed required blocks dependent effect; optional only degrades | submission is not readiness |
| JIT inventory | every LLM call maps to atoms/fallback | new inline system behavior fails lint | enforces JIT-first |
| cancellation/recovery | queue/spawn/shard waiters all terminate | no active/status/JIT DB residue | long-horizon stability |
| observability redaction | receipt correlates bounded IDs | marked secret and oversized output rejected | safe diagnosis |

## Test design rules

- Use a real RealKernel for exact permission, predicate type, and routing joins.
- Use table tests for profiles, route branches, payload variants, and status
  transitions.
- Use deterministic channels/hooks instead of sleeps for lifecycle assertions.
- Run race tests whenever state, callbacks, subscriptions, queues, or caches
  change.
- Add fuzzing for payload codecs, consultation parsing, and action normalization.
- Add adversarial Mangle fixtures for arity, atoms versus strings, unsafe
  negation, protected heads, and stratification.
- Campaign tests should inject time and resource ceilings; never rely on a live
  provider for the only acceptance gate.

## Corpus receipts

The strict corpus validator runs `go test -count=1 ./internal/shards/...` from
`corpus.toml`. The independent race receipt is recorded in
[_progress.md](_progress.md). Structural validation cannot prove semantic wiring;
the exact-ID action test is the strongest current end-to-end package discriminator.

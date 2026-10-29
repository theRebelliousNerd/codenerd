# Current state: shards

> Evidence snapshot: 2026-07-13, HEAD
> `cfc537e96495e1fbccd7efff8bb8e4001c93ca9c`. The tree was dirty; the
> verification receipt and current fingerprint are recorded in
> [_progress.md](_progress.md).

## Realized boundary

`internal/shards` contains 18 production Go files and 25 Go test files. Five
root files implement registration, technology matching, consultation,
background observation, and requirements interrogation. Thirteen files under
`internal/shards/system` implement permanent or on-demand OODA participants.
There is no package-owned `.mg` file: declarations and policy live under core
defaults.

**VERIFIED CURRENT:** `go test -count=1 ./internal/shards/...` passed both
packages. `go test -race -count=1 -timeout=240s ./internal/shards/...` also
passed. These are bounded package receipts, not proof that every boot adapter or
negative configuration path has a regression.

## Component inventory

| Component | Status | Current contract and evidence |
|---|---|---|
| Registration | **VERIFIED CURRENT** for factory/profile presence | `internal/shards/registration.go#RegisterAllShardFactories` registers requirements, image aliases, and nine system shard factories/profiles; `internal/shards/registration_test.go#TestRegisterAllShardFactories` checks named profiles |
| Predicate manifest | **VERIFIED CURRENT** | `defaultKernelShardConfigs` consumes `DefaultShardPredicateManifests`; manifest uniqueness and Cortex exact-envelope tests pass |
| Perception firewall | **VERIFIED CURRENT** for fallback and validation slices | `PerceptionFirewallShard.Perceive`; transient LLM and unknown-verb tests prove visible fallback facts |
| Executive policy | **VERIFIED CURRENT** for boot guard, OODA timeout, delegate translation, and pending envelope construction | `ExecutivePolicyShard.evaluatePolicy`; `TestExecutiveOODATimeoutRespectsBootGuard`; exact action correlation continues in the pipeline test |
| Constitution gate | **VERIFIED CURRENT** for ordinary permit and denial schema | `ConstitutionGateShard.processPendingActions` emits `permission_check_result/4`, `routing_result/4`, and `security_violation/3`; `TestPendingActionPipelineProducesRoutingResult` proves permit path |
| Tactile router | **VERIFIED CURRENT** for route selection, ordinary effect correlation, and both unmapped modes | `TactileRouterShard.processPermittedActions`; default/learning-enabled no-handler tests prove one failure, exact consumption, and no second-pass amplification |
| World model ingestor | **PARTIAL** | scan, incremental update, facts, heartbeat, and copy-safe accessors exist; this corpus run did not execute a full workspace scan campaign |
| Session planner | **PARTIAL** | agenda, checkpoints, views, and JIT decomposition exist; tests focus on formatting/helpers rather than a full cancellation/recovery journey |
| Campaign runner | **PARTIAL** | explicit on-demand supervision and restart backoff exist; full campaign lifecycle is owned and tested mainly by the campaign subsystem |
| Legislator | **PARTIAL** | JIT + structured synth-required rule generation exists; live persistence still depends on downstream kernel validation and corpus availability |
| Mangle repair | **VERIFIED CURRENT** for selected predicate corpus, schema-capable client, retry helpers; **PARTIAL** end to end | `MangleRepairShard` is installed through `RealKernel.SetRepairInterceptor`; focused tests do not cover every boot and persistence route |
| Requirements interrogator | **VERIFIED CURRENT** for static no-LLM fallback and fail-visible no-JIT behavior | `RequirementsInterrogatorShard.Execute`; focused tests cover empty task and missing JIT |
| Matching | **VERIFIED CURRENT** as deterministic heuristic matching | `MatchSpecialistsForTask` scores extension, path, import, and content hints; it does not use embeddings |
| Consultation | **VERIFIED CURRENT** for single, ordered batch, partial/total failure, and nil-spawner behavior | `RequestBatchConsultation` preserves successes and returns `errors.Join`; three negative regressions pass |
| Background observer manager | **VERIFIED CURRENT** for first and restarted generations | fresh per-run context, loop/task joins, stale-event drain, idempotent Stop; restart regression passes under race |

## Registry and boot truth

### Package registrar

`RegisterAllShardFactories` performs dependency injection through
`RegistryContext`, registers factories, then defines profiles. `createAssembler`
attaches the provided JIT compiler and configured budgets. Learning-store
adapters are injected into perception and executive. Mangle repair locates the
primary RealKernel and installs itself as the repair interceptor.

### Cortex boot

**VERIFIED CURRENT:** `internal/system/factory.go#initShardManagement` creates a
bounded spawn queue, calls the package registrar, installs JIT DB callbacks,
then replaces the router and campaign factories with richer versions. The
router override adds BrowserManager and the shared prompt assembler; the
campaign override adds ShardManager. Disabled system names are applied before
`ShardManager.StartSystemShards`.

**PARTIAL:** auto-start requests are submitted to the queue as detached work.
Submission is observable, but BootCortex does not wait for a generation-wide
ready set. Individual shard errors are logged rather than returned by
`StartSystemShards`.

### Interactive and auxiliary boot

Chat shared boot and legacy session boot retain additional wiring for Glass Box,
ToolEventBus, ToolStore, learning candidates, and observer/consultation managers.
Campaign commands and init scanning also call registration helpers. These are
live consumers, not removable copies merely because Cortex boot exists.

## Action and fact contract

| Stage | Producer | Fact/query | Consumer | Current discriminator |
|---|---|---|---|---|
| intent | perception/transducer | `user_intent` | Mangle and executive | fallback/validation tests |
| decision | Mangle policy | `next_action` | executive | delegate and OODA tests |
| permission request | executive | `pending_action/5` | constitution | exact ID/payload source code and action pipeline |
| decision receipt | constitution | `permission_check_result/4` | Mangle/diagnostics | `/permit` asserted in pipeline test |
| authorized route input | constitution | `permitted_action/5` | router | exact ID checked in pipeline test |
| denial audit | constitution | `security_violation/3` | policy/observability | Go args match `schemas_shards.mg` declaration |
| route choice | core policy | `route_action/2` | router | policy route preferred by exact action ID |
| route result | router | `routing_result/4` | waiting session/policy | success and no-route tests |
| effect result | VirtualStore | `execution_result/6` | session/observability | action pipeline asserts ID equality |

**VERIFIED CURRENT:** the four authorization predicates are co-located in the
production policy KernelShard configuration generated from the canonical shards
manifest. `TestDefaultShardPredicateManifestsAreUnambiguous` rejects duplicate
domain/predicate ownership; `TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`
proves exact target/payload mismatches remain denied.

## JIT and agent behavior

| Call family | Current behavior |
|---|---|
| Requirements interrogation | JIT required when an LLM exists; semantic top-k 5; static questions only for nil LLM |
| Session planning | JIT required; user goal remains bounded task input |
| Legislator | JIT required; structured synth required; semantic top-k 100 |
| Executive/constitution/router/world autopoiesis | JIT required for optional proposal; failure skips creative proposal, not deterministic execution |
| Mangle repair | JIT preferred, legacy system prompt retained as fallback |
| Perception | Prompt assembler feeds the canonical transducer; fallback parsing remains available and emits confidence/failure facts |
| Consultation | Protocol text is built inline and delegated through a spawner |

**PARTIAL:** new LLM behavior is intended to be JIT-first, but the package has no
machine-readable inventory proving every call site and fallback mode.

## State, concurrency, and lifetime

- ShardManager owns factory/profile/active/result maps outside this source root.
- Each concrete system shard owns local counters, queues, cases, and loop state.
- `BaseSystemShard` owns one event subscription and stops by closing `StopCh`.
- CostGuard bounds calls per minute, per session, cooldown, and validation retry.
- SpawnQueue bounds queued and active non-system work and prioritizes four lanes.
- Background observers use a 100-event channel and retain 100 assessments.
- Consultation caches 100 responses for five minutes.
- Router permission and routing-result histories are pruned on bounded cadence.

**VERIFIED CURRENT:** the package race suite passes, including observer
concurrency and system shard state tests.

**PARTIAL:** observer generations and router consumption now have lifecycle
regressions under race. Dropped observer events still have no counter;
asynchronous results have no package-level retention contract; boot readiness is
eventual rather than generation-scoped.

## Recovery and observability

Current recovery includes context deadlines, queue timeout, panic capture in the
manager, exact active/status fact retraction, system-shard Stop channels,
learning flush, campaign backoff, parser/repair budgets, and optional fallback
parsing. Signals include category logs, audit lifecycle records, Glass Box,
ToolEventBus, ToolStore, heartbeats, permission/routing facts, and in-memory
metrics.

**PARTIAL:** there is no single bounded lifecycle receipt joining boot
generation, shard ID, task/action correlation, readiness, cancellation, and
terminal outcome. Consultation failures are now returned; observer channel drops
and boot readiness remain weak observability seams.

## Explicit absences

- No generic actor mailbox or distributed shard transport.
- No package-owned policy corpus or declarations.
- No Go domain persona implementations; JIT/session owns those behaviors.
- No embedding matcher; current matching is deterministic heuristics.
- No proof that profile permission slices authorize effects; they do not.
- No generation-wide boot readiness barrier.
- No generation-wide boot readiness barrier or unified lifecycle receipt.
- No uniform JIT fallback contract for every LLM call.

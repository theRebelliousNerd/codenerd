# Shards: specialists around a logic-governed action pipeline

> Corpus owner: `shards`
>
> Realized source root: `internal/shards`
>
> Source review: 2026-07-13 at HEAD `cfc537e96495e1fbccd7efff8bb8e4001c93ca9c`
> plus the dirty-tree receipt in [_progress.md](_progress.md)

## In one minute

Shards give codeNERD focused workers without giving those workers executive
authority. Long-lived system shards translate input, observe the workspace,
shape plans, challenge unsafe actions, and route already-authorized effects.
Short-lived specialists answer one bounded question or perform one delegated
task. The user-visible result should be a responsive agent that can bring the
right expertise to a problem while keeping every real action correlated to a
deterministic policy decision.

The package owns two related toolkits:

- the concrete system shards and their factories under `internal/shards/system`;
- consultation, matching, background-observer, and requirements-interrogation
  helpers under `internal/shards`.

It does **not** own the shard process manager. Spawn queues, lifecycle maps, and
result collection live in `internal/core/shards`. It also does not own persona
execution: coder, reviewer, tester, and researcher behavior now runs through the
JIT/session clean loop.

## Its place in codeNERD

```text
creative center                               deterministic executive
LLM-backed perception / planner / advisor
          | structured facts and proposals
          v
   Mangle schemas + policy -> next_action(ActionID, Type, Target, Payload)
                                  |
                                  v
                         executive_policy
                                  |
                           pending_action/5
                                  v
                         constitution_gate
                    deny /               \ permit
           security_violation/3       permitted_action/5
                                             |
                                             v
                                       tactile_router
                                             |
                                      VirtualStore effect
                                             |
                                routing/execution receipts -> user
```

The LLM may classify, decompose, consult, or propose a rule. It does not grant
permission. `internal/core/defaults/policy/constitution.mg#permitted/3` and the
exact action envelope remain authoritative. The recently repaired router path
preserves the executive-issued action ID through
`internal/shards/system/router.go#TactileRouterShard.processPermittedActions`;
`internal/shards/system/action_pipeline_test.go#TestPendingActionPipelineProducesRoutingResult`
proves the same ID reaches `execution_result`.

| Boundary | Owner | Shards-side evidence |
|---|---|---|
| Factory and profile catalog | `internal/shards` | `internal/shards/registration.go#RegisterAllShardFactories` |
| Spawn, queue, cleanup, results | `internal/core/shards` | `internal/core/shards/manager_spawn.go#ShardManager.SpawnAsyncWithContext` |
| Session persona execution | `internal/session` | `internal/system/factory.go#Cortex.SpawnTask` |
| Constitutional policy and declarations | core defaults | `internal/core/defaults/schemas_shards.mg#pending_action/5`, `internal/core/defaults/policy/constitution.mg#permitted/3` |
| Authorized effect execution | VirtualStore | `internal/shards/system/router.go#TactileRouterShard.processPermittedActions` |

## A representative journey

Suppose a user asks codeNERD to read `hello.txt` and explain a failure.

1. **Ingress.** `internal/shards/system/perception.go#PerceptionFirewallShard.Perceive`
   uses the canonical transducer and a JIT prompt assembler when available to
   produce structured intent facts. Its heuristic fallback is observable and
   does not itself execute the request.
2. **Decision.** Mangle policy derives a `next_action`. The executive obtains a
   stable action ID, hydrates target and payload from the current intent, and
   asserts `pending_action/5` in
   `internal/shards/system/executive.go#ExecutivePolicyShard.evaluatePolicy`.
3. **Permission.** The constitution shard checks dangerous content, active
   overrides, and the exact `permitted(ActionType, Target, Payload)` query. It
   emits either `permission_check_result(...,/deny,...)` plus
   `security_violation/3`, or `permitted_action/5`. Unknown actions remain
   denied in strict mode.
4. **Effect.** The router consumes the permitted fact, chooses a Mangle-derived
   or deterministic local tool route, and passes the same action ID and canonical
   payload to `VirtualStore.RouteAction`.
5. **Response.** `routing_result/4`, `execution_result/6`, tool-store data, logs,
   and Glass Box events make the outcome visible to the waiting session and the
   user.
6. **Failure.** A missing route emits a correlated failure instead of executing.
   A blocked action emits a reason and optional appeal surface. A canceled shard
   records a result and removes active lifecycle facts.

**PARTIAL:** ordinary exact-ID permit and execution behavior is verified. The
same repair wave also made the predicate manifest production authority,
preserved partial consultation errors, made observer generations restartable,
and consumed unmapped permissions exactly once. Boot readiness remains
asynchronous, factory/profile construction still has runtime-specific overrides,
and JIT/fallback behavior has no complete inventory.

## What exists today

The source root has 18 non-test Go files and 25 test files. Both
`go test -count=1 ./internal/shards/...` and
`go test -race -count=1 -timeout=240s ./internal/shards/...` passed on the
reviewed tree. Those receipts cover package behavior and concurrency exercised
by the current tests; they do not prove the known negative controls that have no
regressions yet.

| Component | Claim | Evidence and discriminator |
|---|---|---|
| Factory/profile registration | **VERIFIED CURRENT** | `internal/shards/registration.go#RegisterAllShardFactories`; `internal/shards/registration_test.go#TestRegisterAllShardFactories` checks ten named profiles |
| Authorization action stream | **VERIFIED CURRENT** for ordinary read path | `internal/shards/system/action_pipeline_test.go#TestPendingActionPipelineProducesRoutingResult` proves pending -> permit -> route -> execution with one action ID |
| Constitution denial facts | **VERIFIED CURRENT** | `internal/shards/system/constitution.go#ConstitutionGateShard.processPendingActions` emits `security_violation/3`, matching `internal/core/defaults/schemas_shards.mg#security_violation/3` |
| Predicate ownership manifest | **VERIFIED CURRENT** | `internal/system/factory.go#defaultKernelShardConfigs` consumes `DefaultShardPredicateManifests`; `internal/shards/registration_manifest_test.go#TestDefaultShardPredicateManifestsAreUnambiguous` and `internal/system/cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard` prove uniqueness and exact envelope ownership |
| System-shard boot | **PARTIAL** | `internal/core/shards/manager_spawn.go#ShardManager.StartSystemShards` selects auto and Mangle-activated profiles; queued detached startup does not establish a readiness barrier |
| JIT behavior | **PARTIAL** | requirements interrogator, planner, legislator, and system autopoiesis are JIT-gated; consultation prompts and Mangle repair retain inline or legacy behavior |
| Specialist matching | **VERIFIED CURRENT** as heuristic matching | `internal/shards/matching.go#MatchSpecialistsForTask`; table tests distinguish paths, extensions, imports, and classifications; it is not semantic retrieval |
| Consultation | **VERIFIED CURRENT** for ordered batch terminal outcomes | `RequestBatchConsultation` returns successes in input order plus joined partial/total errors; missing spawner fails visibly; focused regressions pass |
| Background observers | **VERIFIED CURRENT** for restart lifecycle | fresh per-run contexts, separate loop/task joins, idempotent Stop, stale-event drain; `TestBackgroundObserverManager_RestartProcessesNewEvents` passes under race |
| Router unmapped modes | **VERIFIED CURRENT** | both default and learning-enabled modes emit one `no_handler` failure and consume the permission; a second pass cannot amplify it |

### Applicability matrix

| Lane | Status | Package-specific answer |
|---|---|---|
| Mangle | **VERIFIED CURRENT** for declared action predicates and production ownership | Shards produce and consume `pending_action/5`, `permitted_action/5`, `permission_check_result/4`, `routing_result/4`, `security_violation/3`, `route_action/2`, and lifecycle predicates declared under core defaults. Core owns declarations/rules; the package manifest now drives production KernelShard ownership. |
| Permission and safety | **VERIFIED CURRENT** on the exercised action route | `ConstitutionGateShard.checkPermitted` defaults to denial; the router receives only `permitted_action/5`; VirtualStore rechecks the exact envelope. Profiles are advisory capability declarations, not a substitute for `permitted/3`. |
| Fact flow | **PARTIAL** | Perception -> intent -> policy -> executive -> constitution -> router -> effect is live. Interactive and Cortex boot paths still duplicate some construction, and readiness is not one atomic contract. |
| JIT and agents | **PARTIAL** | Prompt atoms such as `system/requirements_interrogator/identity`, `system/autopoiesis/router`, and `system/mangle_repair/safety` are selected by shard type and semantic query. Some package prompts remain inline, and fallback policy is inconsistent. |
| Wiring | **PARTIAL** | predicate ownership has one production authority. `initShardManagement` still intentionally overrides router/campaign factories for extra dependencies, chat owns enrichers, and reduced scanner registration is not a readiness proof. |
| State and concurrency | **PARTIAL** | Manager/core own maps and backpressure; concrete shards guard local state; observer restart and package race suites pass. Results can outlive unobserved async callers, observer overflow is uncounted, and boot health is eventual. |
| Recovery | **PARTIAL** | Context deadlines, panic recovery, queue cancellation, observer generations, exact permission consumption, learning flush, campaign backoff, and repair budgets exist. There is no unified lifecycle/boot receipt. |
| Observability | **PARTIAL** | Audit logs, Glass Box, tool events/store, heartbeats, permission/routing facts, joined consultation errors, and metrics exist. Boot readiness and dropped observer events remain weak. |
| Testing | **PARTIAL** | Unit, integration, race, manifest ownership, batch failure, observer restart, pipeline, learning, and both unmapped modes pass. Missing decisive gates cover full boot readiness, JIT inventory, overflow, and unobserved-result retention. |

## North star

A shard should be a typed, observable participant in one logic-governed graph:

- one registry defines its identity, dependencies, startup mode, prompt atoms,
  owned facts, permissions, resource budget, health contract, and teardown;
- Mangle decides whether and when the shard may participate, while Go performs
  bounded lifecycle and external effects;
- every ingress, consultation, spawn, permission, route, and completion carries
  a stable correlation identity and one terminal outcome;
- restarting or degrading a specialist is explicit, testable, and visible;
- the user sees useful specialist work, not orchestration noise or silent loss.

Non-goals: shards will not become an actor framework, a second policy engine, a
new hard-coded persona hierarchy, or an excuse to run every specialist. Domain
expertise stays in JIT atoms and agent knowledge. The system must not let an LLM,
a profile permission list, or a route table grant its own effect authority.

## Improvement frontier

The first slice of `shards-registration-contract-v1` is now real: one manifest
drives production predicate ownership, rejects duplicate ownership in tests, and
keeps the complete authorization envelope in policy. The broader descriptor for
factory/profile/dependency parity remains in progress.

The first reliability slice of `shards-terminal-outcomes-v1` is also real:
ordered batch consultation preserves joined errors, observer Start/Stop/Start
uses fresh generations, and both unmapped router modes consume one permission
into one failure. A unified typed lifecycle receipt remains proposed.

The bounded longer-horizon move is `shards-policy-certified-activation-v1`:
Mangle derives an activation plan from typed capabilities and dependencies, Go
starts the plan under budgets, and an action cannot leave the system until the
required safety and routing participants have emitted healthy readiness for that
boot generation. The design does not let a health fact grant permission.

All cards, positive and negative acceptance, rollback, and dependencies live in
[TODO.md](TODO.md). The evidence and order live in
[03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

## Choose a reading route

**90-second orientation**

1. Read this page through the representative journey.
2. Scan [02-CURRENT-STATE.md](02-CURRENT-STATE.md) for exact realized boundaries.
3. Read P0/P1 in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

**10-minute architecture tour**

1. [01-VISION.md](01-VISION.md) — desired user experience and non-goals.
2. [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) — shard families and state.
3. [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) — registry, boot, overrides, dispatch, teardown.
4. [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) — action envelope and fail-closed gates.
5. [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) — evidence and missing falsifiers.
6. [TODO.md](TODO.md) — authoritative uplifts.

**Deep implementation route**

| Document | Responsibility |
|---|---|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Flagship realized specification and behavioral contracts |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment and tensions |
| [01-VISION.md](01-VISION.md) | Target system and success measures |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Source-grounded current truth |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Ordered current-to-target deltas |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design rules |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, state, and concurrency |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported contracts |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Compile-time and runtime dependencies |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Constructors, registry, boot, dispatch, teardown |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety and Mangle invariants |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Verification matrix |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Signals and diagnosis |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failure, degradation, and recovery |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Choices that remain unpinned |
| [_progress.md](_progress.md) | Signed score and verification receipts |

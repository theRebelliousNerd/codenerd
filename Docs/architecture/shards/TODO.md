# Authoritative uplift cards: shards

> This is the sole `NERD_FEATURE` authority for the `shards` corpus. Status is
> evidence, not intent. Supporting backlog is subordinate to these cards.

<!-- NERD_FEATURE
id: shards-registration-contract-v1
owner: shards
status: in_progress
kind: truth-gap
depends_on: []
affects: [core, system, shards, safety]
-->

## Safe uplift: one validated shard registration contract

**Value.** A shard registered in one runtime surface should have the same
factory dependencies, startup semantics, predicate ownership, and safety
envelope everywhere else.

**Evidence.** `internal/shards/registration.go#DefaultShardPredicateManifests`
now owns the complete authorization envelope and
`internal/system/factory.go#defaultKernelShardConfigs` consumes it. Manifest
uniqueness and an exact Cortex permission journey have focused regressions. Boot
still re-registers the router and campaign runner after the package registrar to
inject additional dependencies.

**Observed gap.** Predicate ownership now has one production truth. Factory,
profile, dependency, prompt-selector, health, and teardown metadata still live
across the registrar, manager, Cortex enrichers, and chat compatibility wiring.

**Desired behavior.** Define one typed descriptor per shard/domain covering
identity, factory, profile, startup, dependencies, owned predicates, prompt
selectors, health, and teardown. Generate or consume the production kernel
shard configurations from it. Validate duplicate names/predicates, missing
factories, incomplete authorization envelopes, and invalid startup dependencies
before spawning.

**Non-goals.** Do not merge `internal/core/shards` into this package, auto-start
on-demand shards, remove chat-specific dependency injection, or let descriptor
permissions replace constitutional `permitted/3`.

**Affected contracts.** `RegisterAllShardFactories`,
`DefaultShardPredicateManifests`, `defaultKernelShardConfigs`, chat/session boot,
scanner initialization, and registration tests.

**Positive acceptance.** One registry enumeration produces identical names,
profiles, and predicate ownership in Cortex and chat boot. The policy domain
contains the four exact-envelope predicates plus `permitted`. A full action ID,
target, and canonical payload join and execute through per-shard facts.

**Negative acceptance.** Boot rejects duplicate predicate ownership, a profile
without a factory, a system shard with an impossible dependency, or any split of
`pending_action`, `permitted_action`, `permission_check_result`, and `permitted`.
No fallback table may silently restore a second authority.

**Rollback.** Restore a local `defaultKernelShardConfigs` table from the now
verified manifest snapshot while retaining uniqueness/exact-envelope tests and
the complete policy ownership set.

**Verified slice.** `TestDefaultShardPredicateManifestsAreUnambiguous` rejects
duplicate domains/predicates and asserts the four authorization predicates are
owned by policy. `TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard` builds
from the generated configs, derives the exact permission, and denies mismatched
target and payload. The overall card remains in progress because the unified
factory/profile/dependency descriptor is not built.

<!-- NERD_FEATURE
id: shards-terminal-outcomes-v1
owner: shards
status: in_progress
kind: truth-gap
depends_on: []
affects: [campaign, shards, observability, system]
-->

## Reliability uplift: one terminal outcome per operation

**Value.** A user should never mistake missing specialist work or a replayed
permission for success. Every consultation, observer generation, route, and
shard execution needs one correlated terminal outcome.

**Evidence.** The audited defects are repaired: batch consultation returns
input-ordered successes plus joined failures and guards nil spawners; observer
Start creates a fresh run context and Stop joins loops/tasks and drains stale
events; every unmapped route mode emits one failure and consumes the permission.
The incident packets remain under `.corpus-build/findings/shards-*`.

**Observed gap.** The three highest-risk negative paths now have focused and race
coverage. Cross-operation outcome schemas remain heterogeneous; observer drops,
unobserved async result retention, JIT selection, and boot readiness do not join
one bounded lifecycle receipt.

**Desired behavior.** Introduce versioned result types that preserve operation
ID, target, state, error class, cancellation, partial successes, start/end time,
and bounded diagnostics. A consumed permission has exactly one terminal routing
receipt. Observer generations are explicitly one-shot or restartable, never
apparently alive with no workers.

**Non-goals.** Do not make every consultation fail-fast, retain unbounded output,
persist raw prompts, retry dangerous effects, or make observability facts grant
authority.

**Affected contracts.** consultation batch API and adapters, observer lifecycle,
router terminal branches, shard result storage, Glass Box/tool events, and
campaign advisory handling.

**Positive acceptance.** Mixed-success consultation returns responses plus the
failed target and cause; Start/Stop/Start either works with a new generation or
returns an explicit terminal error; one permissive-unmapped action produces one
unhandled record and one terminal receipt; cancellation reaches all waiters.

**Negative acceptance.** Total failure cannot return nil error; a stopped
observer cannot report live; a consumed `permitted_action` cannot remain for a
second pass; callback or persistence failure cannot erase the primary outcome.

**Rollback.** Keep compatibility adapters that expose existing response slices
and strings, but derive them from typed outcomes and never map a non-empty
failure set to success.

**Verified slice.** Partial/total/nil-spawner consultation tests,
`TestBackgroundObserverManager_RestartProcessesNewEvents`, and both no-route
router regressions pass; the race package suite covers the restarted observer.
The card remains in progress because a unified typed receipt, drop telemetry,
and result-retention contract are absent.

<!-- NERD_FEATURE
id: shards-jit-prompt-boundary-v1
owner: shards
status: proposed
kind: leverage
depends_on: [shards-terminal-outcomes-v1]
affects: [articulation, prompt, shards, session]
-->

## Leverage uplift: a consistent JIT prompt boundary for shard cognition

**Value.** Operators should be able to inspect which prompt atoms shaped a
specialist without hunting for hidden inline system instructions or ambiguous
fallbacks.

**Evidence.** Requirements interrogation, planning, legislation, and most
system autopoiesis require JIT assembly. `MangleRepairShard.buildRepairPrompt`
retains a legacy system-prompt fallback, while consultation and campaign
consultation construct model-facing protocol text directly in Go.

**Observed gap.** The system has several definitions of “JIT-first”: fail closed,
skip optional cognition, fall back to legacy, or use inline protocol prose.

**Desired behavior.** Inventory every LLM call in this package. Move stable
system behavior into typed prompt atoms, keep user/task data in bounded user
payloads, declare an explicit per-call fallback policy, and emit selected atom
IDs, budget, truncation, and fallback reason in a redacted receipt.

**Non-goals.** Do not turn deterministic constitution or routing checks into LLM
calls, store raw secrets, require the network for boot, or hide creative content
inside Mangle policy.

**Affected contracts.** prompt atoms, `PromptAssembler`, consultation spawners,
planner/interrogator/legislator/repair/autopoiesis call sites, and JIT telemetry.

**Positive acceptance.** A test enumerates every shards-owned LLM call and its
atom family/fallback mode. Required-JIT paths fail visibly; optional cognition
degrades without effect authority; receipts show bounded selector and budget
data.

**Negative acceptance.** No new system prompt constant or inline behavioral
protocol can bypass atom validation. JIT failure cannot silently change an
executive or constitutional outcome, and trace mode cannot retain marked
secrets.

**Rollback.** Disable a migrated atom family and use the previous explicitly
declared fallback for that call only; keep inventory and telemetry tests.

<!-- NERD_FEATURE
id: shards-policy-certified-activation-v1
owner: shards
status: proposed
kind: north-star
depends_on: [shards-registration-contract-v1, shards-terminal-outcomes-v1]
affects: [core, mangle, observability, shards, system]
-->

## Bounded north star: policy-certified activation generations

**Value.** codeNERD should know which specialists are required for a task,
whether they became ready, and why it is safe to proceed before an effect leaves
the process.

**Evidence.** `ShardManager.StartSystemShards` combines profile auto-start with
Mangle `activate_shard/1`, but queued startup is detached and boot has no
generation-wide readiness barrier. Heartbeats and active facts exist, yet they
do not form one dependency or degradation contract.

**Observed gap.** A Cortex can return from boot while auto-start work is still
queued. Operators can see logs and later heartbeats but cannot inspect one
bounded activation plan showing required, ready, disabled, failed, and degraded
participants.

**Desired behavior.** Mangle derives a finite activation plan from typed
capabilities, dependencies, user configuration, and resource budgets. Go starts
that plan under a generation ID and emits bounded readiness outcomes. Effects
that require the system action pipeline wait for the constitution and routing
participants for that generation; optional specialists degrade explicitly.

**Non-goals.** Do not make health imply `permitted/3`, wait for every optional
shard, build a distributed scheduler, or allow an LLM to assert readiness.

**Affected contracts.** planned: activation-plan predicates and declarations,
registry descriptors, StartSystemShards, queue detached semantics, health facts,
boot receipts, and shutdown generations.

**Positive acceptance.** Deterministic fixtures derive the same acyclic plan;
required shards become ready or produce a named boot failure; disabled optional
shards produce explicit degradation; shutdown cancels and joins one generation;
an action carries both boot generation and executive action ID.

**Negative acceptance.** A health fact cannot authorize an action, a stale
heartbeat cannot satisfy a new generation, dependency cycles fail before spawn,
and optional specialist failure cannot deadlock boot.

**Rollback.** Disable readiness gating and retain current profile plus
`activate_shard/1` startup. Preserve generation receipts as diagnostics only.

## Supporting backlog, subordinate to the cards

- Preserve manifest uniqueness and exact authorization-envelope regressions as
  factory/profile descriptors evolve.
- Preserve input-ordered partial consultation outcomes and joined errors through
  every campaign/chat adapter.
- Add observer drop counters and return snapshots rather than internal pointers.
- Preserve exactly-once permitted-action consumption on every new router branch.
- Bound and expire unobserved asynchronous `ShardManager` results.
- Replace timing sleeps in observer integration tests with deterministic signals.
- Add boot generation, disable reason, and dropped-event counters to operator
  diagnostics.

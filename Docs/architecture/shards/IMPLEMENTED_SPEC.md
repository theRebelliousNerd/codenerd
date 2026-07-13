# Shards implemented specification

> Status: source-grounded living reference
>
> Owner: `internal/shards`
>
> Verified: 2026-07-13 at HEAD `cfc537e96495e1fbccd7efff8bb8e4001c93ca9c`
> plus the dirty-tree receipt in `_progress.md`

## 1. Purpose and boundary

`internal/shards` implements focused participants around codeNERD's Mangle
executive. System shards observe, translate, plan, gate, repair, and route.
Package-level helpers match and consult specialists, run background alignment
observers, and elicit requirements.

The historical domain-shard hierarchy is not current architecture. Persona work
is compiled from prompt atoms and executed by the session clean loop. The live
boundary is:

| Owned here | Owned elsewhere |
|---|---|
| shard factory/profile definitions | active maps, spawn queue, panic recovery, result lifecycle: `internal/core/shards` |
| system shard implementations | kernel shards and Mangle program assembly: `internal/core` |
| matching, consultation, observers, interrogation | JIT compiler and atoms: `internal/prompt` |
| action-envelope production/consumption in system shards | constitutional declarations/rules: core defaults |
| router selection and effect request | effect handlers and revalidation: VirtualStore/tactile |

**VERIFIED CURRENT:** the source root contains 18 production Go files and 25
test files. The ordinary and race package suites pass.

## 2. Families

### 2.1 Long-lived or on-demand system shards

| Name | Startup profile | Creative or deterministic role | Core output |
|---|---|---|---|
| `perception_firewall` | auto | LLM/transducer with bounded heuristic fallback | intent and perception facts |
| `executive_policy` | auto | logic-primary OODA coordinator; optional LLM autopoiesis | `pending_action/5` |
| `constitution_gate` | auto | deterministic default-deny gate; optional rule proposal | `permission_check_result/4`, `permitted_action/5`, denial facts |
| `mangle_repair` | auto | deterministic validation plus bounded LLM repair | accepted/rejected learned-rule outcome |
| `world_model_ingestor` | on demand | scanner/AST primary; optional interpretation | topology, symbol, diagnostic, heartbeat facts |
| `tactile_router` | on demand | deterministic Mangle/local route selection | `routing_result/4`, effect request |
| `session_planner` | on demand | JIT LLM decomposition plus deterministic agenda state | agenda/checkpoint/status facts |
| `campaign_runner` | on demand | deterministic supervisor around campaign orchestrator | campaign lifecycle facts |
| `legislator` | on demand | JIT creative rule synthesis behind strict schema mode | candidate learned rule |

### 2.2 Ephemeral and library participants

`requirements_interrogator` is an actual ephemeral ShardAgent with a JIT-required
LLM path and a static nil-LLM fallback. Matching, consultation, and observers are
libraries used by chat and campaign adapters rather than factories for large Go
persona classes.

## 3. Registration contract

### 3.1 Dependency context

`internal/shards/registration.go#RegistryContext` carries kernel, LLM,
VirtualStore, workspace, JIT compiler/config, and optional classification LLM.
Factories inject dependencies before a shard starts. `createAssembler` builds a
PromptAssembler against the supplied kernel, installs the JIT compiler, copies
effective budgets, and enables JIT according to configuration.

`RegisterAllShardFactories` performs this ordered work:

1. attach VirtualStore to ShardManager;
2. register requirements interrogation and image-generation aliases;
3. register perception and world-model factories;
4. register executive, constitution, legislator, and Mangle repair;
5. register router, campaign runner, and session planner;
6. define ephemeral, image, and system profiles.

**VERIFIED CURRENT:** `TestRegisterAllShardFactories` checks the named profiles.
Image aliases have a dedicated image-client gate in the manager outside this
source root.

### 3.2 Profile semantics

Profiles name type, startup mode, capability hint, timeout, memory budget, and a
coarse permission set. They are scheduling/injection metadata. They do not
authorize effects. The live external-action contract still requires
`permitted(Action, Target, Payload)`.

Auto-start profiles are perception, executive, constitution, and Mangle repair.
World, router, planner, campaign runner, and legislator are on-demand. System
timeouts are generally 24 hours; requirements interrogation is bounded to five
minutes.

### 3.3 Predicate ownership drift

`DefaultShardPredicateManifests` exports routing, world, tools, policy, campaign,
prompts, and catch-all domains. `internal/system/factory.go#defaultKernelShardConfigs`
now copies those manifests into production KernelShard configs.

**VERIFIED CURRENT:** the canonical manifest keeps `pending_action`,
`permitted_action`, `permission_check_result`, and `permitted` in the policy
KernelShard. `TestDefaultShardPredicateManifestsAreUnambiguous` rejects duplicate
ownership, and `TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard` proves
the exact join while denying target/payload mismatches.

**PARTIAL:** predicate ownership is unified; factory/profile/dependency metadata
and runtime enrichers still do not have one typed descriptor.

## 4. Boot and lifecycle

### 4.1 Cortex boot

`internal/system/factory.go#initShardManagement` configures limits and SpawnQueue,
calls the package registrar, installs per-agent JIT DB callbacks, then replaces
two factories:

- router gains BrowserManager and the shared prompt assembler;
- campaign runner gains ShardManager and the shared prompt assembler.

Disabled names are applied before StartSystemShards. That manager method selects
auto profiles and adds valid system profiles derived by `activate_shard/1`.
With a running queue, starts are critical-priority detached submissions.

**PARTIAL:** boot submission is not boot readiness. Individual startup failures
are logged and the method returns nil after continuing. There is no generation
contract for required-ready versus optional-degraded participants.

### 4.2 Chat and auxiliary boot

Chat boot enriches factories with Glass Box, tool events/store, learning
candidates, classification, and per-turn integrations. Campaign CLI and init
scanner use registration helpers in reduced contexts. These paths are live and
must be treated as adapters around one intended graph, not assumed dead copies.

### 4.3 Spawn, execute, stop

The external ShardManager creates a unique ID, checks resource limits, selects a
factory/profile, injects dependencies, asserts `active_shard` and `shard_status`,
then executes under a timeout. It records success/error before lifecycle fact
cleanup and catches panics. System shards run event/ticker loops until context or
StopCh closes; BaseSystemShard Stop flushes learning and unsubscribes its event
channel.

**VERIFIED CURRENT:** package race tests pass and observer integration covers a
first graceful shutdown.

**VERIFIED CURRENT:** BackgroundObserverManager now creates a fresh context per
Start, separates loop/task WaitGroups, makes Stop idempotent, drains stale events,
and processes a new event after Start/Stop/Start under the race suite.

**PARTIAL:** unobserved async result retention is owned by the core manager and
has no bounded receipt in this corpus; observer overflow remains uncounted.

## 5. The OODA action envelope

### 5.1 Perception

`PerceptionFirewallShard.Perceive` uses the canonical perception transducer,
optional lower-cost classification client, target resolution, confidence
thresholds, learning candidates, and observable heuristic fallback. It emits
structured facts; it does not execute the intent.

### 5.2 Executive

`ExecutivePolicyShard.evaluatePolicy` queries strategies and barriers, obtains
next actions, enforces boot guard and a five-actions-per-tick ceiling, hydrates
target/payload from the latest intent, and asserts:

```text
pending_action(ActionID, ActionType, Target, CanonicalPayload, Timestamp)
```

It retracts one-shot asserted next actions and records the current intent as
consumed when applicable. Event subscriptions drive evaluation; a bounded poll
exists when no event bus is present. OODA timeout facts diagnose intent without
progress.

### 5.3 Constitution

`ConstitutionGateShard.processPendingActions` checks each exact envelope. Its
local guard rejects dangerous target/content and disallowed network domains;
then `checkPermitted` queries the kernel for the exact action type, target, and
payload. Strict mode defaults to denial.

Permit emits:

```text
permitted_action(ActionID, ActionType, Target, Payload, Timestamp)
permission_check_result(ActionID, /permit, Reason, Timestamp)
```

Deny emits a correlated permission result and routing failure plus:

```text
security_violation(ActionType, Reason, Timestamp)
appeal_available(ActionID, ActionType, Target, Reason)
```

**VERIFIED CURRENT:** `security_violation` now has exactly the declared three
arguments. The exact pending fact is retracted after either result.

### 5.4 Router and VirtualStore

`TactileRouterShard.processPermittedActions` reads `permitted_action/5`, prefers
the exact-ID `route_action/2` derived by Mangle, then uses deterministic local
routes. It constructs a VirtualStore `next_action` with the original action ID,
type, target, and payload. It emits a correlated `routing_result/4`; VirtualStore
emits `execution_result/6`.

**VERIFIED CURRENT:** `TestPendingActionPipelineProducesRoutingResult` proves a
read-file action keeps the executive action ID through permission, routing, and
execution. Route-selection tests prove normalized exact match outranks prefix
and contains.

**VERIFIED CURRENT:** both values of `AllowUnmappedActions` emit one correlated
`no_handler` failure and consume the permission. The true mode additionally
records one autopoiesis case; a second processing pass cannot amplify the
terminal outcome.

## 6. Specialist collaboration

### 6.1 Matching

`MatchSpecialistsForTask` reads supplied files, calculates scores from extension,
path, import, and content hints, maps technology patterns to registered agent
names, caps the result according to verb configuration, and sorts by score. It is
deterministic heuristic matching, not embedding retrieval or authority.

Classifications separate executor, advisor, and observer roles plus technical,
strategic, and domain knowledge tiers. `ShouldSpecialistExecuteTask` requires an
executor classification and high confidence.

### 6.2 Consultation

`ConsultationManager` stores pending requests, delegates through a spawner,
parses structured response sections, and caches up to 100 answers for five
minutes. Batch consultation launches one worker per target.

**VERIFIED CURRENT:** single and all-success batch tests pass. Batches preserve
successful responses in input order and return joined partial/total failures. A
nil spawner returns an explicit error and clears pending state.

**PARTIAL:** cache hits return manager-owned response pointers and reuse cached
correlation data; defensive-copy and identity semantics remain unpinned.

### 6.3 Background observers

`BackgroundObserverManager` fans bounded events to registered observer
classifications. Northstar can use a direct handler; other observers use a
spawner. Assessment state and the last 100 assessments are retained in memory,
and callbacks receive completed assessments outside the manager lock.

**VERIFIED CURRENT:** concurrent event handling, direct Northstar integration,
graceful shutdown, and a fresh restarted generation pass under the race suite.

**PARTIAL:** the 100-entry input channel drops on overflow without a metric, and
GetLastAssessment returns an internal pointer.

## 7. JIT prompt contract

| Shard cognition | Selector / behavior | Failure policy |
|---|---|---|
| requirements interrogator | `system/requirements_interrogator/*`, semantic top-k 5 | error if LLM exists but JIT unavailable; static nil-LLM questions |
| planner | campaign planner atoms, semantic top-k 5 | fail decomposition |
| legislator | legislator identity/syntax/safety/output, semantic top-k 100 | fail rule generation |
| system autopoiesis | `system/autopoiesis/{executive,router,world_model,constitution}` | skip optional proposal |
| Mangle repair | `system/mangle_repair/*` | legacy prompt fallback remains |
| perception | assembler attached to transducer | structured heuristic fallback is visible |
| consultation | inline protocol task | no package atom inventory |

**PARTIAL:** the system has good atom coverage but inconsistent fallback policy.
Stable consultation protocol text and repair legacy behavior remain outside one
machine-checkable JIT inventory.

## 8. State, resources, and recovery

- CostGuard defaults: 10 calls/minute, 100/session, exponential cooldown capped
  at 60 seconds, three validation retries, 20 session validation attempts.
- Autopoiesis proposes after three unhandled cases and uses a 0.8 confidence
  threshold unless overridden.
- SpawnQueue defaults: 100 total, 30 per priority, two workers, five-minute wait,
  30-second drain.
- Router rate limiters are per tool; permission/routing results prune after 15
  minutes on a ten-second minimum cadence.
- World scanning, planner agenda, campaign supervisor, and learning stores own
  additional bounded state described in the supporting documents.

Recovery is explicit but uneven: deadlines, cancellation, learning flush,
campaign retry, repair rejection, observer restart, batch partial failure, and
router idempotency exist. Boot readiness, observer-drop diagnosis, cached-value
ownership, and unobserved-result retention do not yet share that contract.

## 9. Observability

The strongest correlation key is the executive action ID. It appears in
permission, routing, execution, and tool-store records. Shard IDs appear in
audit and Glass Box lifecycle events plus active/status facts. System heartbeat,
world/planner/campaign status facts expose selected health. Cost, route, queue,
observer, and plan statistics are in memory.

**PARTIAL:** there is no versioned bounded lifecycle receipt joining boot
generation, dependency readiness, shard task, action ID, cancellation, and
terminal state. Consultation failures are returned; observer drops can still be
invisible.

## 10. Verification contract

Current package gates:

```powershell
go test -count=1 ./internal/shards/...
go test -race -count=1 -timeout=240s ./internal/shards/...
```

Highest-value missing regressions:

1. factory/profile/dependency descriptor parity across boot enrichers;
2. consultation cancellation plus cache correlation/defensive-copy semantics;
3. observer overflow/drop accounting and snapshot ownership;
4. delayed/failed required versus optional shard boot readiness;
5. complete JIT call-site and fallback inventory;
6. bounded unobserved-result retention and lifecycle receipts.

## 11. Status and next decisions

The ordinary action pipeline is real and exact-ID correlated. The corpus is not
claiming completion for registration, readiness, recovery, or JIT consistency.
The authoritative proposed changes are in [TODO.md](TODO.md), evidence order is
in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md), and unresolved ownership choices are
in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).

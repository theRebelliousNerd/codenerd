# System: the motherboard that makes one safe runtime

> Corpus owner: `system`
>
> Realized source root: `internal/system`
>
> Source review: 2026-07-13 at `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`

## In one minute

`internal/system` turns configuration and a workspace into a usable codeNERD
runtime. It constructs the kernel, stores, VirtualStore, JIT compiler, shards,
session executors, browser support, and adapters, then returns them as one
`Cortex`. The user-visible outcome is that `nerd query`, `nerd spawn`, and chat
operate on the same kind of logic-first machine instead of each assembling a
different partial agent.

The package is a **motherboard**, not another reasoning agent. The LLM remains
the creative center. Mangle policy remains the executive. System's job is to
connect those roles without losing the exact action identity, authorization
payload, workspace, model, or lifecycle owner at a package boundary.

## Its place in codeNERD

```text
configuration + workspace
          |
          v
  internal/system boot pipeline
    | kernel shards       | JIT compiler
    | VirtualStore        | shard manager
    | stores/adapters     | session executors
          |
          v
user -> perception -> user_intent -> Mangle decision -> next_action
          ^                                      |
          |                                      v
   articulation <- execution_result <- VirtualStore / registered tool
```

System owns construction, registration order, boundary adapters, and teardown.
It does not own constitutional rules, prompt prose, tool implementations, the
OODA loop, or CLI presentation. Those boundaries are evidenced by
`internal/system/factory.go#BootCortexWithConfig`,
`internal/core/defaults/policy/constitution.mg#permitted/3`,
`internal/session/executor.go#Executor`, and `cmd/nerd/`.

## A representative journey

Suppose a user runs `nerd query` in a Go repository.

1. **Ingress and identity.** The command calls
   `internal/system/factory.go#GetOrBootCortex`. System resolves the workspace
   and configuration, hashes the current cache identity, and either reuses or
   boots a Cortex. A failed boot is never inserted into the cache.
2. **Executive construction.** `BootCortexWithConfig` registers kernel domains.
   The policy shard owns `pending_action`, `permitted_action`,
   `permission_check_result`, and `permitted`, so one RealKernel can join the
   exact authorization envelope. This is **VERIFIED CURRENT** by
   `internal/system/cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`.
3. **Creative context.** System loads the embedded prompt corpus and constructs
   the JIT compiler. Each compilation obtains a private clone through
   `internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope`, so
   selector facts cannot mutate the live Cortex or cross concurrent prompts.
4. **Decision and safety.** Perception asserts structured intent; Mangle derives
   the next action and exact `permitted(Action, Target, Payload)`. The
   VirtualStore preserves the executive-issued action ID, simulates destructive
   effects with a Dreamer bound to the primary RealKernel, checks permission,
   executes, validates, and emits correlated result facts.
5. **Response.** Session execution and articulation turn the result into the
   user response. System owns the adapters that keep those packages cycle-free.
6. **Failure and teardown.** Kernel, embedded-corpus, or system-shard startup
   errors fail the boot through `rollbackBootContext`. Missing credentials and
   unavailable embeddings degrade explicitly. `Cortex.Close` is idempotent,
   stops maintenance before SQLite, releases the enumerated MCP/browser/JIT/
   store resources, and evicts a cached instance. Cleanup errors join rather
   than replace the named boot-stage error.

**VERIFIED CURRENT:** the normalized disabled-system-shard set is shared by
cache identity and boot behavior, late failure runs aggregate rollback, and the
canonical shard manifest drives production kernel ownership. **PARTIAL:**
engine/provider mode is not yet keyed, and cleanup is not an exact reverse-order
typed acquisition registry.

## What exists today

The realized package contains five non-test Go files, 19 Go test files, and 61
named tests. The fixed system corpus verification profile ran
`go test -count=1 ./internal/system/...` on the settled dirty tree and passed in
79.191 seconds. This proves the package suite, not all CLI,
network, race, or long-horizon behavior.

| Component | Claim | Evidence and discriminator |
|---|---|---|
| Nine-stage Cortex boot, assembly, and rollback | **VERIFIED CURRENT** | `defaultBootSteps`, `bootCortexWithSteps`, full boot test, and forced late-failure regression |
| Canonical policy-shard authorization envelope | **VERIFIED CURRENT** | `defaultKernelShardConfigs` consumes `DefaultShardPredicateManifests`; uniqueness and exact target/payload mismatch tests pass |
| Primary-RealKernel Dreamer | **VERIFIED CURRENT** | `internal/system/factory.go#initExecutionLayer`, `internal/core/virtual_store.go#realKernelForDreamer`; Cortex exposes `GetPrimaryRealKernel` and VirtualStore lazily binds the Dreamer |
| Executive action correlation | **VERIFIED CURRENT** | `internal/core/virtual_store_routing.go#VirtualStore.parseActionFact` carries the supplied ID into `execution_error/2` and `execution_result/6`; direct-route tests use exact pending envelopes |
| Prompt compilation isolation | **VERIFIED CURRENT** | `internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope`; concurrent, error, cancellation, and retry regressions in `prompt_kernel_scope_test.go` |
| Keyed cache | **VERIFIED CURRENT** for normalized disabled sets, **PARTIAL** for engine mode | reuse, set split, failure retry, order/duplicate normalization, and secret-safe hash tests pass |
| Maintenance shutdown | **VERIFIED CURRENT** | `factory.go#Cortex.StartMaintenanceSchedule`, `cortex_close.go#Cortex.Close`, and three tests in `maintenance_schedule_test.go` |
| Boundary adapters | **PARTIAL** | prompt and MCP kernel adapters are tested; `sessionVirtualStoreAdapter.ReadFile/WriteFile` still use raw `os` calls |
| Resource teardown | **VERIFIED CURRENT** for enumerated owners, **PARTIAL** for typed registry | idempotent Close and rollback cover maintenance, shard queue/workers, MCP, browser, closable embedding, JIT/DBs, and perception; exact reverse order and caller-owned override policy remain open |

### Applicability matrix

| Lane | Status | System-specific answer |
|---|---|---|
| Mangle | **VERIFIED CURRENT** for boot ownership, **PARTIAL** overall | System owns no `.mg` source. It converts the canonical `DefaultShardPredicateManifests` into routing, world, tools, policy, campaign, prompts, and cortex kernel shards. The policy shard co-locates the complete permission envelope. Declarations, bound negation, recursion, and aggregation remain owned by core policy and are a hard boot dependency. |
| Permission and safety | **VERIFIED CURRENT** for exact envelope wiring | External effects still require `permitted(Action, Target, Payload)`. The VirtualStore fails closed without a kernel or Dreamer for mapped destructive actions. System preserves one policy owner and attaches the primary RealKernel. Raw session adapter file I/O remains a bypass risk. |
| Fact flow | **VERIFIED CURRENT** for construction | System wires perception, kernel, VirtualStore, task executor, and articulation. It does not originate user intent or decide actions. Exact action IDs flow from `next_action` through execution and result facts. |
| JIT and agents | **VERIFIED CURRENT** for compiler construction and isolation, **PARTIAL** for policy references | Embedded/project corpora, optional vector search, user-agent DBs, and assembler budgets are wired at boot. Each compile gets a cloned kernel. Agent-specific policy reference semantics remain owned by JIT/session and are reviewed in their corpora. |
| Wiring | **PARTIAL** | Constructor order, shard registry, system-shard start, TaskExecutor bridge, and CLI consumers are live. Chat intentionally calls direct `BootCortexWithConfig`, creating a second identity/lifecycle path. |
| State and concurrency | **PARTIAL** | Cache state and Close are mutex protected; maintenance/MCP have cancel/done ownership; prompt scopes are private clones. Engine identity is incomplete, boot serializes all keys, and Start/Close concurrency is not fully specified. |
| Recovery | **VERIFIED CURRENT** for forced late failure, **PARTIAL** overall | Failed boots are uncached and rolled back; maintenance stops before DB close; close steps are bounded/idempotent. Exact reverse-order registry semantics and reset-and-close remain open. |
| Observability | **PARTIAL** | Category logs expose boot, embedding, context, tools, store, and session events. There is no redacted boot receipt that fingerprints stages, config identity, degraded components, or teardown ownership. |
| Testing | **PARTIAL** | Package tests cover full boot, DI, adapters, permission/manifest ownership, prompt isolation, maintenance, cache reuse/split/retry, forced late rollback, DOM, routing, and agent discovery. Focused lifecycle race passes; whole-package race and all-resource fault injection remain open. |

## North star

One workspace configuration should produce one inspectable Cortex identity and
one owned lifecycle. Every component should declare whether it is required or
degradable, every external effect should retain its exact executive correlation,
and every opened resource should be registered for reverse-order teardown or
boot rollback. A compact boot receipt should let an operator answer: what was
wired, which policy and prompt corpora were used, what degraded, and what must be
closed—without exposing credentials.

Non-goals: system will not absorb policy text, invent prompt behavior, implement
tools, become a service locator used by every package, or make the LLM an
executive. It should remain a composition root with thin, explicit adapters.

## Improvement frontier

The completed safe uplift is **exact executive-envelope wiring**. The policy
shard now owns all four authorization predicates; VirtualStore routes fail
closed, keeps the executive ID, and lazily binds Dreamer to the primary
RealKernel. The verified card is `system-exact-executive-envelope-v1` in
[TODO.md](TODO.md).

The second uplift, `system-cache-identity-and-rollback-v1`, is now **in
progress**: normalized disabled-shard identity, failure retry, aggregate
rollback, optional-resource Close, and idempotence are verified. Engine identity,
exact reverse acquisition order, and caller-owned overrides keep the larger
card open. The bounded longer-horizon option is
`system-boot-receipt-registry-v1`, a typed resource registry and redacted boot
receipt rather than more ad-hoc fields and close calls.

## Choose a reading route

**90-second orientation**

1. Finish this page through the improvement frontier.
2. Read [02-CURRENT-STATE.md](02-CURRENT-STATE.md) for current truth.
3. Scan [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md).

**10-minute architecture tour**

1. [01-VISION.md](01-VISION.md) — target and non-goals.
2. [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) — boot stages and state machines.
3. [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) — constructors, registries, dispatch, and teardown.
4. [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) — exact authorization and lifecycle gates.
5. [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) — receipts and missing gates.

**Deep implementation route**

| Document | Responsibility |
|---|---|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Flagship live implemented specification |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | Signed north-star review |
| [01-VISION.md](01-VISION.md) | Desired product experience |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Source-grounded inventory and behavior |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Evidence-ranked deltas |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding composition rules |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, stages, and ownership |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Public and adapter contracts |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream and reverse dependencies |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, registry, dispatch, and close wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Default deny and lifecycle invariants |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Risk-selected verification |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Signals, correlation, and diagnosis |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failure, degradation, and recovery |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Decisions that remain unpinned |
| [_progress.md](_progress.md) | Corpus receipts and signed score |

# Authoritative uplift cards: system

> This file is the sole `NERD_FEATURE` authority for the `system` corpus.
> Current behavior belongs in `02-CURRENT-STATE.md`; dependency and priority
> belong in `03-GAP-ANALYSIS.md`.

<!-- NERD_FEATURE
id: system-exact-executive-envelope-v1
owner: system
status: verified
kind: truth-gap
depends_on: []
affects: [core, session, system]
-->

## Verified uplift: exact executive envelope and fail-closed routing

**Value.** A user-approved action must be the same action the executive checks,
the Dreamer simulates, the VirtualStore executes, and telemetry reports. Target
or payload substitution must deny rather than inherit a broad classification.

**Evidence before repair.** The Cortex boot configuration split authorization
predicates across domains; direct-route tests relied on permissive shortcuts;
some routes could mint or lose correlation at the effect boundary.

**Desired behavior.** The default policy shard owns `pending_action/5`,
`permitted_action/5`, `permission_check_result/4`, and `permitted/3`. The
executive-issued action ID and canonical JSON payload survive dispatch. A mapped
destructive action requires a usable Dreamer backed by the primary RealKernel.

**Non-goals.** This card does not rewrite constitutional policy, make
`safe_action/1` an authorization grant, or replace the session executor.

**Affected contracts.** `internal/shards/registration.go#DefaultShardPredicateManifests`,
`internal/system/factory.go#defaultKernelShardConfigs`,
`internal/core/virtual_store_routing.go#VirtualStore.RouteAction`, and session's
exact `pending_action` envelope.

**Positive acceptance.** An exact read-file action derives `permitted/3`, routes,
and emits results under the supplied action ID. A mapped destructive action can
obtain a Dreamer from a default Cortex kernel.

**Negative acceptance.** A different target or payload is denied. Missing kernel
or Dreamer cannot authorize a destructive action. `safe_action/1` alone never
authorizes execution.

**Rollback.** Revert the routing packet as one unit and disable affected routes;
never restore wildcard permissions or test-only production bypasses.

**Verification receipt.** Production boot consumes the canonical manifest.
`internal/shards/registration_manifest_test.go#TestDefaultShardPredicateManifestsAreUnambiguous`
proves unique ownership, and
`internal/system/cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`
proves policy ownership plus exact target/payload denial. The focused system
suite and core routing regressions passed on 2026-07-13.

<!-- NERD_FEATURE
id: system-cache-identity-and-rollback-v1
owner: system
status: in_progress
kind: truth-gap
depends_on: [system-exact-executive-envelope-v1]
affects: [cli, config, system]
-->

## Safe uplift: complete cache identity and transactional boot rollback

**Value.** A caller must never receive a Cortex booted under different lifecycle
inputs, and a failed boot must not leave background workers or database handles
behind.

**Verified uplift.** `cortexKey` now includes the normalized disabled-shard set,
and the same normalization reaches boot behavior. Named boot stages share
`rollbackBootContext`; Close is idempotent and enumerates MCP, browser, closable
embedding, JIT/DB/store, shard, maintenance, and perception ownership. Behavioral
tests cover normalized reuse, set split, failed-boot retry, and late rollback.

**Remaining gap.** Separately configured engine/provider mode is absent from
identity. Cleanup reuses an enumerated aggregate rather than a typed exact-
reverse-order acquisition registry, and caller-owned override semantics are not
pinned.

**Desired behavior.** Define one canonical, redacted `CortexIdentity` containing
every boot-shaping input, including engine and a normalized disabled-shard set.
Track successful stage acquisitions and unwind them in reverse order on error.
Failed instances remain uncached.

**Non-goals.** Do not add a general dependency-injection framework, persist
credentials, or parallelize boot.

**Affected contracts.** Get-or-boot identity, Reset/Close eviction, BootConfig,
system-shard lifecycle, store ownership, and CLI expectations.

**Positive acceptance.** Equal normalized identities share one Cortex. Different
disabled-shard sets or engines boot distinct instances. A forced failure after
LocalDB/JIT/shard-queue acquisition leaves no cache entry, open database, or
live goroutine and permits a clean retry.

**Negative acceptance.** Identity serialization never exposes an API key.
Rollback must not close a caller-owned override. A cleanup error must be joined
with the original boot error, not replace it.

**Rollback.** Disable caching for ambiguous identities and keep the aggregate
cleanup helper. The safe fallback is a fresh Cortex, never reuse across an
unproven identity.

**Verification receipt.** `factory_cache_test.go`,
`factory_helpers_test.go#TestCortexKeyNormalizesDisabledShardSet`, and
`factory_rollback_test.go` pass with focused race coverage. The full system
package passed on 2026-07-13. The card remains `in_progress` because its engine
identity and typed registry acceptance criteria are not complete.

<!-- NERD_FEATURE
id: system-virtualstore-adapter-policy-v1
owner: system
status: proposed
kind: truth-gap
depends_on: [system-exact-executive-envelope-v1]
affects: [core, session, system]
-->

## Safe uplift: policy-preserving session file adapter

**Value.** Session tasks should not escape constitutional routing merely because
an interface boundary asks for `ReadFile` or `WriteFile`.

**Observed gap.** `internal/system/factory_adapters.go#sessionVirtualStoreAdapter.ReadFile`
and `.WriteFile` call `os.ReadFile` and `os.WriteFile` directly. `ReadRaw` and
`Exec` already delegate to VirtualStore where possible.

**Desired behavior.** Add typed VirtualStore file capabilities that apply path
containment, exact permission, Dreamer preflight when destructive, execution
correlation, and post-action validation without recursive or double execution.

**Non-goals.** Do not make tests grant wildcard permission, expose
`VirtualStore.HandleAction`, or silently fall back to raw OS writes when the
executive is unavailable.

**Affected contracts.** `types.VirtualStore`, session Executor/Spawner,
VirtualStore file handlers, action IDs, and permission tests.

**Positive acceptance.** An allowed session read/write reaches the same handler,
target normalization, result fact, and validator path as an equivalent routed
action.

**Negative acceptance.** Missing kernel, mismatched target/payload, workspace
escape, unsafe Dreamer verdict, or validation failure denies visibly. No effect
executes twice.

**Rollback.** Disable session file mutation and retain read-only contained access
until the typed adapter is proven; do not restore silent unrestricted writes.

<!-- NERD_FEATURE
id: system-boot-receipt-registry-v1
owner: system
status: proposed
kind: north-star
depends_on: [system-cache-identity-and-rollback-v1]
affects: [logging, observability, system]
-->

## Bounded north star: resource registry and redacted boot receipt

**Value.** An operator should be able to explain exactly what a Cortex booted,
what degraded, which resources it owns, and whether teardown completed without
reading a thousand-line factory.

**Observed gap.** Resource pointers remain spread across `bootContext` and
`Cortex`. MCP/browser/closable-embedding ownership and aggregate rollback are now
real, but no typed acquisition record or correlated machine-readable receipt
describes order, ownership class, degradation, and cleanup outcome.

**Desired behavior.** Each boot stage registers a typed acquisition with owner,
required/degradable class, redacted config fingerprint, close function, and
bounded diagnostic. The registry produces a size-capped receipt and drives
reverse-order rollback and normal Close.

**Non-goals.** No secret values, unbounded prompts, whole environment dumps,
remote control plane, or alternate executive.

**Affected contracts.** Boot stages, Cortex Close, logging retention, failure
diagnostics, MCP/browser/embedding ownership, and campaign evidence.

**Positive acceptance.** A successful and a forced-failure boot each emit a
deterministic receipt with the same stage IDs, explicit degraded/skipped states,
one correlation ID, and complete close outcomes.

**Negative acceptance.** Receipts contain no API key, prompt body, user file
content, or arbitrary error payload beyond bounded redacted summaries. A receipt
cannot authorize or execute an action.

**Rollback.** Keep the registry as an internal cleanup mechanism and disable
receipt persistence independently.

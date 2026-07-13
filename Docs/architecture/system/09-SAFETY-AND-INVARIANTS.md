# Safety and invariants: system

> System does not author constitutional rules. It is responsible for building
> the one object graph in which those rules can actually govern effects.

## S1 — One exact authorization owner

**VERIFIED CURRENT.** `internal/system/factory.go#defaultKernelShardConfigs`
assigns `pending_action`, `permitted_action`, `permission_check_result`, and
`permitted` to the policy shard. Splitting them prevents one RealKernel store
from joining the same action ID, action type, target, and payload.

The declaration and derivation remain in core policy. System's invariant is
ownership and boot order, not policy authorship.

## S2 — Default deny survives every adapter

An effect requires exact `permitted(Action, Target, Payload)`. Broad
`safe_action/1` classification cannot grant a request. Missing kernel, query
error, target mismatch, payload mismatch, malformed action, or missing Dreamer
for a mapped destructive route denies visibly.

`internal/system/cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`
proves exact match and mismatch behavior through a default Cortex shard layout.

**PARTIAL:** `sessionVirtualStoreAdapter.ReadFile` and `.WriteFile` are outside
this route and call the OS directly. They remain a known bypass to close.

## S3 — Executive correlation is immutable

The first `next_action` argument is the action ID. VirtualStore must preserve it
through handler dispatch, `execution_error/2`, `execution_result/6`, validator
facts, and operator signals. Neither system adapters nor route helpers may mint
a replacement correlation ID.

Dreamer's internal simulation ID is a separate sandbox correlation; it must not
overwrite the executive ID used for the real effect.

## S4 — Destructive simulation is fail closed

`internal/system/factory.go#initExecutionLayer` attaches the boot kernel before
DreamRouter and DreamPlanManager. `core.VirtualStore` resolves a RealKernel from
either a direct RealKernel or `CortexKernel.GetPrimaryRealKernel`, constructs the
Dreamer lazily outside `VirtualStore.mu`, and blocks a mapped destructive action
when no Dreamer is available.

Counterfactual Dreamer facts remain inside a cloned kernel; they must never
become live executive facts.

## S5 — Prompt selection is not executive state

`internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope` clones the
primary RealKernel for each prompt compilation. Transient `compile_context`,
atom, vector-hit, and selection facts are scoped to that clone and discarded on
success, error, cancellation, and concurrent completion.

This isolation is **VERIFIED CURRENT** by the four regressions in
`internal/system/prompt_kernel_scope_test.go`.

## C1 — Failed boot never poisons cache

`GetOrBootCortex` inserts only after `BootCortex` returns a non-nil Cortex with no
error. An error leaves no failed entry.

**VERIFIED CURRENT:** `TestGetOrBootCortexFailureIsNotCached` forces a transient
failure, proves the retry boots again, and proves only the successful retry is
cached. Stage failures also run aggregate rollback before returning.

## C2 — Cache identity covers constructed behavior

The current SHA-256 key uses length-delimited workspace, provider, API key,
model, and the normalized disabled-system-shard set. Normalization is shared by
identity and the boot call. Hashing avoids putting credentials in the map key
as plaintext.

**VERIFIED CURRENT** for those inputs by helper and behavioral cache tests.
**PARTIAL:** separately configured engine/provider mode remains outside identity.
The safe fallback for an unproven identity is fresh boot, not reuse.

## C3 — Cache transitions are serialized

Lookup uses an RLock; miss uses the global write lock and rechecks before boot.
This prevents duplicate insertion and duplicate maintenance for one current key.
Global cross-key serialization is accepted until measurement justifies a more
complex per-key lock.

## L1 — Maintenance stops before SQLite

**VERIFIED CURRENT.** A fresh cached Cortex stores maintenance cancel/done
handles. The first maintenance cycle waits one full interval. `Cortex.Close`
cancels and briefly joins the goroutine before LocalDB close. Repeated Close is
safe for the covered minimal Cortex case.

## L2 — Close is bounded, idempotent, and enumerated

Each covered close step has an eight-second timeout, panics are converted to
errors, and multiple failures are joined. A mutex/closed bit makes repeated
Close idempotent. Covered resources are maintenance, shard queue/manager, MCP
connect/bridge, browser, closable embedding, JIT, LocalDB, LearningStore,
initialized perception, and cache.

Boot errors build a partial aggregate and reuse that Close path; an
untransferred project DB is closed explicitly. **PARTIAL:** the implementation
does not record exact acquisition order or distinguish caller-owned overrides in
a typed registry. A timeout also allows teardown to continue while the timed-out
goroutine may still be running; close methods must therefore be concurrency-safe.

## P1 — Secrets are redacted at identity and observability boundaries

The API key participates in the internal hash but must not be logged, persisted
in a receipt, or exposed through a public identity value. Payload-key logging at
effect denial must not include payload values.

## B1 — Hard core and soft periphery are explicit

| Hard failure | Explicit degradation |
|---|---|
| kernel shard construction/register/evaluate | missing LLM uses an always-error client |
| embedded prompt corpus and JIT construction | embedding health failure |
| system shard startup | taxonomy, agent sync, hybrid ingest, modular tool hydration, MCP connection |

Changing a row requires a product decision, a failure-path test, and corpus
reconciliation.

## Reviewer gate

Before approving a system change, verify:

1. every predicate has one boot owner and the permission envelope remains whole;
2. exact action ID, target, and payload survive adapters;
3. missing executive dependencies fail closed;
4. prompt compilation uses a private clone;
5. every acquisition has normal Close and failure rollback ownership;
6. cache identity includes every behavior-shaping input;
7. logs and receipts are bounded and redact secrets;
8. focused behavioral and race tests falsify the changed boundary.

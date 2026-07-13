# core — codeNERD's executive engine

> **VERIFIED CURRENT** on 2026-07-13 against `internal/core` and focused tests.
> `corpus.toml` owns the source boundary; [_progress.md](_progress.md) owns receipts.

## In one minute

The model can suggest an edit, a command, or a delegation. `core` decides what
that suggestion means to the running system, whether policy permits it, and which
effect adapter may execute it. It is the difference between an LLM saying “delete
that file” and codeNERD proving that a specific deletion is authorized, bounded,
simulated, executed, checked, and reported.

The package combines the Mangle-backed `RealKernel`, the `VirtualStore` effect
gateway, Dreamer's counterfactual safety check, embedded schemas and constitution,
and low-level shard/executive plumbing. The visible outcome is a coding agent whose
creative center remains flexible while its executive path is deterministic and
default-deny.

## Its place in codeNERD

`core` owns executive truth and privileged effect routing. It does not own natural-
language interpretation, prompt prose, the session turn loop, domain specialist
personas, or user-facing response language.

```text
perception             core executive                         articulation
user_intent  ->  RealKernel -> next_action/permitted  ->  result facts -> response
                               |
                               v
                    Dreamer -> VirtualStore -> effect
```

- The creative center lives outside this package in the model-facing perception,
  prompt, shard, and articulation systems.
- `internal/core/kernel_types.go#RealKernel` owns EDB/IDB state and Mangle program
  evaluation.
- `internal/core/virtual_store_routing.go#RouteAction` owns the principal action
  safety, permission, dispatch, validation, and result-fact pipeline.
- `internal/core/dreamer.go#SimulateAction` evaluates destructive actions in a
  clone before their real effect.
- `internal/session/executor.go#Executor` owns the turn lifecycle; core supplies
  executive decisions and effects rather than replacing it.

## A representative journey

Consider a model proposing an edit to `internal/core/kernel.go`:

1. Perception/session establishes `user_intent/5`; Mangle rules may derive
   `next_action/1` as declared by
   `internal/core/defaults/schemas_execution.mg#next_action/1`.
2. The session passes the action fact to
   `internal/core/virtual_store_routing.go#RouteAction`. The boot guard rejects it
   if no live user turn has enabled effects.
3. Because the edit is destructive, `RouteAction` requires a Dreamer. Dreamer
   projects `hypothetical/1`,
   `projected_action/3`, file-change facts, critical-path hits, and available code-
   graph impact into a cloned kernel. Counterfactual facts never enter live truth;
   `internal/core/dreamer_test.go#TestDreamer_SimulateAction_DoesNotMutateLiveHypotheticals`
   proves this negative boundary. Checked staging also rejects an invalid or
   over-limit projected fact instead of silently dropping it.
4. Go constitutional checks run, then the kernel must derive
   `internal/core/defaults/schemas_safety.mg#permitted/3`. Query failure or absent
   permission denies the action.
5. The selected handler performs the effect. Validators can turn a superficially
   successful result into failure when postconditions do not hold.
6. `execution_result/6` and effect-specific facts return to the kernel, while audit
   and Glass Box signals make the route visible. Articulation consumes the resulting
   state rather than inventing success.

If Dreamer, constitutional policy, permission lookup, execution, or validation
fails, the path returns an error and injects a bounded failure/security fact. The
failure is part of the executive story, not an exception to it.

## What exists today

### Evidence-backed snapshot

| Claim | Status | Evidence |
|---|---|---|
| Embedded schemas and policy boot a `RealKernel` | **VERIFIED CURRENT** | `internal/core/kernel_init.go#NewRealKernel`; core package tests |
| Destructive `RouteAction` and interactive preflight fail closed without a usable Dreamer | **VERIFIED CURRENT** | `TestRouteActionFailsClosedWhenCortexHasNoDreamerKernel`; `TestPreflightDestructiveToolCallFailsClosedWithoutDreamer` |
| Exact target/payload-bound `permitted/3` authorizes effects; `safe_action/1` is classification only | **VERIFIED CURRENT** | `TestPermissionCacheIsClassificationOnly`; `TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard` |
| Dreamer counterfactuals are sandbox-only and checked before evaluation | **VERIFIED CURRENT** | `TestDreamer_SimulateAction_DoesNotMutateLiveHypotheticals`; `TestDreamer_EvaluateProjectionFailsClosedAtFactLimit`; `TestDreamer_EvaluateProjectionFailsClosedOnInvalidFact` |
| Predicate-corpus loading is lazy and one-time | **VERIFIED CURRENT** | `TestRealKernelPredicateCorpusLoadsLazilyOnce` |
| Unicode-blank learned rules are rejected before any kernel dependency | **VERIFIED CURRENT** | `TestRuleCourt_UnicodeBlankRejectedBeforeKernelRequirement` |
| Session is the preferred OODA path while `core/shards` still supplies spawn plumbing | **PARTIAL** | `internal/session/executor.go#Executor`; `internal/core/shards/manager.go#ShardManager`; the old package README still overstates removal |
| Dream result caching is bounded | **VERIFIED CURRENT** | `internal/core/dreamer.go#DreamCache`; `internal/core/dreamer_gaps_test.go#TestDreamerGap_BoundedDreamCache` |
| Cache invalidation is complete across every fact/policy mutation | **OPEN QUESTION** | `internal/core/dreamer.go#InvalidateCache` exists, but a single mutation-epoch contract is not proven |
| Differential evaluation is the production default | **REJECTED** | it remains feature-flagged while correctness caveats are evaluated |

The deeper live inventory is in [02-CURRENT-STATE.md](02-CURRENT-STATE.md) and
[IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md). Those documents must not promote the
uplifts below into current behavior.

### Applicability matrix

| Lane | Core contract and evidence |
|---|---|
| Mangle | Owns schemas and executive program assembly. `permitted/3`, `next_action/1`, safe bound negation, load order, and producers/consumers are detailed in [13-MANGLE-SURFACE.md](13-MANGLE-SURFACE.md). |
| Permission and safety | `internal/core/defaults/policy/constitution.mg#permitted/3` is default-deny at the effect boundary; `RouteAction` adds boot, Dreamer, Go constitution, and postcondition layers. Resource limits live in `internal/core/limits.go`. |
| Fact flow | `user_intent/5` enters from perception/session; policy derives `next_action/1`; VirtualStore executes; result/security facts return; articulation owns language. |
| JIT and agents | **N-A for prompt composition:** `internal/prompt/compiler.go#CompilerConfig` owns selection and budgets. Core owns the declarations/facts and task-executor attachment that JIT-selected behavior can drive. |
| Wiring | `SetKernel` clears Dreamer state; both `RealKernel` and a Cortex primary kernel can back destructive simulation. The policy shard owns the complete permission envelope. Chat rehydration keeps the boot guard until the first user turn; command-oriented `BootCortex` disables it while booting an already requested command. |
| State and concurrency | `RealKernel` owns mutex-protected EDB/program state and atomic dirty state; VirtualStore owns effect dependencies and caches; Dreamer owns a bounded concurrent cache. Session and request identities bound effect facts. |
| Recovery | Boot fails on invalid constitution; destructive simulation, nil-kernel permission checks, and post-validation failures fail closed. Rehydrated chat state remains quiescent behind the boot guard. Mid-evaluation cancellation is still partial. |
| Observability | Kernel/VirtualStore/Dream logs, audit completion, security/execution facts, tool events, Glass Box routing, and debug program dumps provide diagnosis. Payload values require redaction. |
| Testing | Core has unit, policy-golden, concurrency, gap, performance, and integration tests. The minimum risky change gate is a focused negative control plus `go test ./internal/core/...` with an explicit timeout. |

## North star

Core should make every privileged effect explainable as a chain from observed
intent, through declared facts and constitutional rules, to a permission decision,
bounded effect, verification result, and user-visible outcome. The model remains
free to be imaginative; it never becomes the authority that declares its own
action safe or successful.

Non-goals:

- moving fuzzy language interpretation or vector retrieval into Mangle;
- embedding domain prompt prose in effect handlers;
- silently defaulting to allow when policy or safety machinery is unavailable;
- treating logs or future receipts as mutable authorization truth;
- making Cortex, differential evaluation, or autonomous learned policy mandatory
  before their ownership and recovery contracts are proven.

## Improvement frontier

The authoritative proposals live in [TODO.md](TODO.md):

1. **Verified truth repairs:** counterfactual Dreamer facts stay in the checked
   sandbox; the exact permission envelope, Cortex ownership, failure facts,
   pruning, and post-validation now retain their contracts end to end.
2. **Safe leverage:** generate one action-contract parity registry spanning enum,
   handler, Mangle classification, Dreamer, tests, and telemetry.
3. **North-star advance:** emit a bounded, redacted executive decision receipt that
   correlates intent, permission, effect, and verification without authorizing them.
4. **Deferred moonshot:** compare candidate policy against normalized receipts in a
   no-effect shadow only after the receipt and action identity contracts exist.

The action registry is the next recommended implementation because it closes a
real drift surface without changing constitutional ownership. The shadow policy
comparison remains deferred until effect isolation and redaction are falsifiably
proven.

## Choose a reading route

### 90-second orientation

Read this page, then [01-VISION.md](01-VISION.md) and the feature cards in
[TODO.md](TODO.md).

### 10-minute system tour

Read [02-CURRENT-STATE.md](02-CURRENT-STATE.md),
[05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md),
[08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md), and
[12-FAILURE-MODES.md](12-FAILURE-MODES.md).

### Deep implementation and assurance

- Implementers: [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md),
  [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md), and
  [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md).
- Policy/safety reviewers: [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md)
  and [13-MANGLE-SURFACE.md](13-MANGLE-SURFACE.md).
- Test authors/operators: [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md),
  [11-OBSERVABILITY.md](11-OBSERVABILITY.md), and [_progress.md](_progress.md).

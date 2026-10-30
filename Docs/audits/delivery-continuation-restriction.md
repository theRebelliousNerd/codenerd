---
doc-class: deep-dive
subsystem: cli
implementation-status: proposed-refinement
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 8d373fb6ca05ef03fec59b923c5f1925066d623a
continues: Docs/audits/perception-restriction-continuity.md
scope: verified foreground delegation to request-delivery continuation
---

# Delivery continuations must retain the originating request's restriction

## Decision and evidence boundary

Carry the originating request's typed restriction into the continuation that completes its missing parts. The current foreground path correctly attaches the restriction to its execution context, and the session executor correctly enforces a restriction it receives. The later **verified return → delivery continuation → fresh preset worker** handoff retains the request as prose but has no typed restriction carrier. The continuation constructs a fresh session-operation context, so the already-recognized obligation need not reach the next worker.

This is a narrow continuation of the existing [restriction-continuity audit][restriction-audit], not a claim that its implemented restriction descriptor, constitutional gate or human-approval path is absent. The established source-level consequence is loss of deterministic inheritance at this boundary. An actual write without a newly required human decision is a conditional downstream consequence, not an observed incident or measured rate.

Public main was re-resolved at approximately 07:42 UTC on 2026-10-30 to [8d373fb6][main]. Source, guidance, existing audit status sections and the then-visible research branch were inspected. An independent source challenger confirmed the carrier loss and narrowed the concrete witness to non-dream coder `/fix` work. No repository program, Go test, model inference, failure injection or benchmark was run. No implementation code was changed during this review. All implementation links below are pinned to that source revision.

## What already works

The restriction work is substantive and should remain intact:

- The immutable `perception.Restriction` represents recognized `read_only`, `no_changes`, `dry_run`, `hypothetical` and `confirm_first` obligations. `Merge` only adds kinds, and `WithRestriction` merges with its parent context. Unsupported prose is separately identified, not represented as enforced. [Descriptor and merge][restriction-value]
- Chat merges perception and Jev restrictions into the original turn's context before delegation. The first verified worker receives that same context. [Foreground attachment][attach] [Verified spawn][verified]
- The executor merges preset intent and context restrictions, asserts turn-owned facts, withholds barred tools and binds actual calls to their owning turn. Missing enforcement infrastructure for a recognized restriction fails closed. [Executor merge][merge] [Restriction bridge][bridge]
- `confirm_first` uses the existing exact request-bound approval path. `Broker.AwaitPerson` deliberately ignores standing yolo autonomy; no attached human host cannot count as consent. A hard prohibition is not lifted by confirmation. [Broker][broker] [Approval selection][approval-selection] [Implemented audit status][restriction-built]
- Mutating foreground delegation is not allowed to fall through without its derived verification obligation or configured verifier, and a missing/rejected verdict is not presented as verified success. [Foreground guards][verified]

The residual is not a flaw in `Merge`: merging two empty inputs cannot reconstruct a value lost earlier. Nor is it a reason to remove the useful preset path and ask a second model to rediscover an already-understood restriction.

## The production handoff

### 1. A verified foreground request can create a delivery continuation

After `VerifyWithRetry` succeeds and its verdict is accepted, `processInput` creates a delivery chain and calls `checkContinuation`. The source explicitly allows a verified attempt to have delivered only part of the user's request. If a next step exists, it returns `continuationInitMsg` rather than a terminal assistant reply. [Verified-to-delivery transition][transition]

Delivery is conditional: `startDelivery` requires a kernel, Jev client, configuration with instruction coverage enabled, and nonempty request fragments. `nextStep` first consults ordinary pending obligations, then asks the delivery judge if none exists. A delivery step requires the kernel's missing-part/pushback derivation for the active request. Cut evidence cannot justify “missing”; a named blocker or lack of progress stops pushback. A judge failure produces no fabricated delivered verdict and no delivery offer. [Chain construction][delivery-start] [Judge and offer][delivery-judge] [Selection][delivery-next] [Policy][delivery-policy]

These conditions matter: this is not a claim that every verified mutation automatically spawns a second writer.

### 2. The asynchronous carrier drops the typed obligation

`deliveryChain` stores request identity, request text, chosen shard, fragments, reports, changed paths, round and summary note. It has no restriction descriptor. `continueMsg` and the resulting `Subtask` carry step identity, description, shard and mutation classification, not the parent intent or restriction. `handleContinuationInitMsg` installs the delivery pointer, renders the first result and passes the next subtask to `offerOrDispatch`. [Chain fields][delivery-start] [Message fields][messages] [Subtask fields][subtask] [Conversion and dispatch][ledger] [Update handoff][update]

The prose is **not** lost. `gapStepTask` includes the missing fragments and the whole original request, and tells the child to report obstacles it cannot clear. That is valuable context and may cause the model to honor the restriction voluntarily. It is not the typed input read by the deterministic restriction gate. [Gap task][gap-task]

### 3. The next worker has a new context and a synthetic preset

`executeSubtask` creates `sessionOperationContext`, builds session context, and calls:

```go
spawnTaskWithContext(ctx, shardTypeToTaskRequest(shardType, description), ...)
```

The new operation context derives from `m.shutdownCtx` (or Background) and restores usage/session/progress metadata. It does not reattach the original turn's restriction. `shardTypeToTaskRequest` constructs only the routing key and task text. The spawn adapter adds model configuration but does not reconstruct the restriction. [Continuation worker][execute] [New context][operation-context] [Task construction][task-request] [Spawn adapter][spawn]

This fresh lifetime is intentional: the original `processInput` context is canceled when its command returns. Keeping that context alive in a queued message would mix the completed command's lifetime with the later step. The missing item is immutable request data, not an obligation to reuse a canceled context. [Original lifetime][original-context]

The task executor normalizes the shard through its authoritative routing table, clones its executor for inline work, and deliberately constructs a preset rather than re-perceiving the synthetic text. The preset has no restriction of its own; its structural constraint comes only from `req.Constraint`, which the continuation's two-field constructor did not fill. `ProcessWithIntent` then merges `intent.Restriction` and `RestrictionFromContext(ctx)`; in this handoff neither contains the original recognized limit. Session prompt/context material is not an alternate typed input to that merge. [Preset construction][preset] [Task admission][task-admission] [Merge][merge]

### 4. The restriction gate cannot bind a value it never receives

For a non-enforced/zero restriction, `assertTurnRestriction` asserts nothing and `bindRestriction` returns the call without a restricted turn. `callRestrictionFacts` then returns no turn/effect binding for that restriction. The kernel's `confirmation_owed` rules require the pending action's turn and that turn's `confirm_first` fact. The earlier worker's correctly scoped facts are not a session-wide fallback. [Restriction bridge][bridge] [Confirmation derivation][confirmation]

Consequently the parent's confirm-first requirement is absent from this child's deterministic gate input. Other permission, allowlist, effect, approval and verification checks still apply. The finding is not a universal action-permission bypass.

## Conditional source-derived witness

A useful acceptance fixture is the existing delivery journey's shape, augmented with a recognized restriction:

1. In a yolo session, the user asks for a code option and its README documentation, adding “confirm with me before writes.” Perception or Jev produces the supported `confirm_first` kind. Do not substitute an unrecognized paraphrase and call that transport failure.
2. The request is admitted to non-dream foreground verified mutation delegation to coder `/fix`, rather than decomposition or consultation. The first worker receives the restriction and obtains the person's exact decisions for its consequential operations. It implements the option but omits the documentation; its verifier returns an accepted verdict.
3. Instruction coverage is enabled. With no earlier pending obligation selected, the delivery judge identifies the documentation as a requested, missing part, not unsettled evidence, a blocker or a stalled repeated round. The kernel derives a new coder step.
4. The continuation message reaches Update. Yolo does not pause at the work-mode step boundary. The second worker starts with the original request present in task prose but without its typed `confirm_first` descriptor. Under the inspected default policy, coder maps to `/fix`, which is not an isolated verb; with dream mode off this reaches the inline preset path traced above. [Persona and isolation table][persona-table]
5. If that worker proposes an otherwise-permitted write and does not independently seek the person's decision, the parent's confirm-first gate does not itself force `AwaitPerson` for this new effect.

The production [delivery journey test][journey-test] already constructs a yolo request whose first result implements a JSON option, then expects a coder continuation for missing README documentation. Reading this test supports the intended route, not a claim that this review executed it or that its fake executor proves final gate behavior.

**What can prevent the downstream effect:** the first verifier can reject incomplete work; the delivery judge can decline to identify a gap; the model can obey the retained prose; the selected tool or its exact payload may be disallowed; another independent approval requirement may stop it; the user can cancel. Each is a real limiting condition. None establishes preservation of the already-recognized typed restriction.

In propose mode, mutating steps pause; explicit mode pauses before every step. This reduces automatic dispatch but is not equivalent to the exact frozen-operation approval required by `confirm_first`. Enter approves running the offered step, not every future target/payload that its model may choose. The primary witness uses yolo to keep these two distinct authorities unambiguous. [Work mode][work-mode] [Step offer][ledger]

No broader witness is needed here. In particular, an initial `no_changes` mutation that is correctly barred or a hypothetical request routed to consultation is not evidence that this verified-mutating route is reachable for it.

## Scope and nonduplication

This is a leaf exit under [perception restriction continuity][restriction-audit]. Its as-built section correctly records foreground context inheritance, preset merging and exact final admission, but its statement that every delegated/multi-step task thereby inherits the restriction needs this later message boundary qualified. The earlier audit also identifies detached background transport and non-session action admission; neither is the verified-return delivery route traced here.

The [background ownership audit][background-audit] concerns completion ownership, successor queues and origin-session delivery. The [continuation-generation audit][generation-audit] concerns old observations suppressing newly raised test debt. Neither is repaired by this proposal; this proposal does not claim a cross-request stale-result bug. Current normal-turn cleanup retracts shard results and pending obligations. [Cleanup][cleanup]

The then-visible `research-main-20261030-0715` added four notes concerning classification root identity, expert test identity, mission receipts and deliberation budget continuation. Its inventory did not add a delivery-restriction finding. [Research inventory][research-inventory]

Authorization data is attached beside restriction in the original turn and also deserves an explicit carrier review. It is deliberately not folded into this finding: permission-bearing request information and narrowing restrictions have different composition semantics. Losing authorization can reduce permission rather than increase it; that is not evidence of the same consequence. Restoring a restriction must not manufacture a broader grant.

## Proposed contract

Amend the existing capability/continuation specification before implementation, consistent with the repository's [typed, Mangle-owned executive guidance][guidance]. These are proposed obligations, not existing APIs.

1. **One request owner.** A continuation belongs to the request that raised it. Store its immutable recognized restriction value with an explicit origin/chain identity before the first command returns. Carry that binding through the initial message, queued/offered subtask, derived successors and the dispatch boundary. An immutable field or a checked immutable chain reference can work; avoid parallel mutable copies that can drift.
2. **Presence is explicit.** Distinguish a bound request with no recognized restriction from a missing/unbound carrier. For a continuation known to originate from a restricted request, a missing carrier must hold or fail explicitly before effect dispatch, not default to unrestricted. Define compatibility for legacy/standalone plan producers separately; do not silently impose a global restriction on every task.
3. **New lifetime, same obligation.** Continue deriving execution contexts from the live session shutdown context, preserving cancellation, metering and progress. Rehydrate the captured restriction with `WithRestriction` before session-context construction and task execution. Do not retain the old command's context, use `WithoutCancel` to escape lifecycle control, or mutate `m.shutdownCtx` into a request-global obligation store. Go's [context documentation][go-context] distinguishes request-scoped values, derived contexts and cancellation; the design inference here is to carry immutable request data across the message boundary and create the proper new execution context.
4. **Monotone inheritance.** Every successor merges parent restrictions with any additional supported child restriction. Synthetic task prose, model output, “done,” a successful verification verdict, a work-mode go-ahead and the first child's exact grant cannot remove the parent's kinds. An authorized new user instruction that changes scope needs an explicit new request/transition, not mutation of the old chain.
5. **Reuse the existing gate.** Preserve task-scoped turns, effect measurement, catalog narrowing, exact pending-action admission, frozen preview/claim/revalidation, `AwaitPerson`, no-host refusal and hard-denial precedence. A second effect requires its own exact decision where policy requires it; do not carry approval grants as a stand-in for restriction transport.
6. **Honest lifecycle and receipt.** Retain the owner while a step is offered, interrupted or queued. End/supersede only that chain's binding when its lifecycle ends. Diagnostic receipts may record origin, step, restriction digest and missing-carrier disposition without raw prompts or hidden reasoning. The receipt is evidence of transport, not permission.

A delivery-only field may close this concrete path, but implementation review should enumerate every producer of `continueMsg`/`Subtask` before claiming universal continuation inheritance. Do not expand this audit into a new scheduler or delete the existing delivery judge.

## Costs and fair evaluation

The bounded repair should need no additional model call to reconstruct a recognized restriction. It does add carrier bytes, identity checks, context wrapping, lifecycle cases and maintenance obligations across asynchronous messages. If durable continuation serialization is used, version/presence migration must be accounted for; an old record lacking a binding cannot be reported as validated.

A correct repair can intentionally increase visible human decisions and elapsed completion time for confirm-first tasks. Count that as honoring the request, not a performance regression to optimize away. It may also expose genuine blocked/no-host outcomes that used to proceed without this particular obligation. Measure false withholding, repeated approval prompts, orphaned offered work, cancellation latency and bookkeeping cost as well as successful completion. No cost or benefit has been measured here.

Compare at least:
- current behavior;
- narrow immutable restriction transport and fresh-context reattachment;
- any broader carrier refactor, if proposed.

Hold models, prompts, tools, policy/configuration, initial workspace state and scripted outputs constant. Include unrestricted, reading-only and already-approved first-step controls. A solution that disables delivery continuation, forces all sessions into explicit mode, or rejects every second task cannot pass merely because write-handler counts fall to zero.

## Proposed validation, not executed

Use the real TUI `processInput` → message Update → offered/automatic step → `executeSubtask` path, real preset executor and production restriction policy, with deterministic clients and reviewed counting tools. Capture the original restriction digest, chain/step owner, worker context restriction, asserted turn facts, exact approval requests and final handler calls. A helper-only `WithRestriction` test is insufficient.

| Case | Required observation |
|---|---|
| Confirm-first + yolo + accepted first result + missing README fragment | Both workers inherit the same parent kinds; second consequential effect remains undispatched until its own exact human decision |
| Same route with second decision rejected or no host | Zero second-effect handler calls; explicit terminal/held reason, not “delivered” |
| First effect approved; second target/payload differs | First grant cannot settle the second pending action |
| Propose/explicit mode and Enter resume | Work-mode pause survives; the resumed worker still owes exact confirmation |
| Unrestricted request with the same missing part | Continuation remains usable without invented confirmation obligations |
| Confirm-first continuation only reads | No unnecessary effect-confirmation request for a declared read |
| Child adds a stronger supported restriction | Parent kinds survive and the new obligation narrows behavior |
| Missing/wrong owner or carrier in a known restricted chain | Explicit non-dispatch disposition; no zero-value downgrade or borrowing another request's restriction |
| Interrupted/offered step, new user request and later resume | Correct chain binding; no restriction leak into unrelated work and no loss on resume |
| Cancel during running step or approval wait | Existing shutdown/cancel propagation remains effective |
| Jev disabled/error, unsettled/cut evidence, blocker or no progress | Existing no-offer/stopping behavior and honest delivery notes remain intact |
| Existing first-worker restriction and approval suites | Retain their as-built behavior; do not weaken gates to make the journey pass |

**Acceptance:** normal-entry evidence demonstrates that a continuation completing a restricted request receives at least that request's enforceable kinds, under the correct new lifetime and owner, and that the actual effect boundary still requires the appropriate exact decision. This closes the delivery handoff only; it does not certify interpretation quality, external-effect coverage, background transport, or every other continuation producer.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/8d373fb6ca05ef03fec59b923c5f1925066d623a
[restriction-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/perception-restriction-continuity.md
[restriction-built]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/perception-restriction-continuity.md#as-built-2026-10-28
[restriction-value]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/restriction.go#L12-L195
[attach]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/process.go#L276-L287
[verified]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/process.go#L599-L665
[transition]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/process.go#L672-L686
[merge]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/executor.go#L1415-L1427
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/turn_restriction.go#L21-L125
[broker]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/approval/broker.go#L141-L170
[approval-selection]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/human_approval.go#L287-L312
[delivery-start]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/request_delivery.go#L29-L69
[delivery-judge]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/request_delivery.go#L110-L152
[delivery-next]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/request_delivery.go#L238-L286
[delivery-policy]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/request_delivery.mg#L45-L115
[messages]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/model_types.go#L853-L874
[subtask]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/model_types.go#L171-L177
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/continuation_ledger.go#L19-L98
[update]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/model_update.go#L1093-L1115
[gap-task]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/request_delivery.go#L219-L235
[execute]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/process_continuation.go#L40-L107
[operation-context]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/operation_context.go#L10-L38
[task-request]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/delegation_routing.go#L361-L379
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/delegation.go#L82-L105
[original-context]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/process.go#L111-L133
[preset]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/task_executor.go#L234-L252
[task-admission]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/task_executor.go#L358-L434
[persona-table]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/delegation.mg#L509-L536
[confirmation]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/constitution.mg#L820-L854
[journey-test]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/request_delivery_test.go#L143-L182
[work-mode]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/work_mode.go#L43-L52
[background-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/background-outcome-and-continuation-ownership.md
[generation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/continuation-obligation-generation.md
[cleanup]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/cmd/nerd/chat/process_helpers.go#L13-L24
[research-inventory]: https://github.com/theRebelliousNerd/codenerd/tree/493e65a2892d7c5262de95999b8c2e256f6a3c4f/Docs/audits
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/agents.md
[go-context]: https://pkg.go.dev/context#pkg-overview


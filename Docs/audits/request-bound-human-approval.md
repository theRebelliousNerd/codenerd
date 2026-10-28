---
doc-class: deep-dive
subsystem: session-core-ui
implementation-status: planned
last-verified: 2026-10-28
verified-against: 24eb5faaa65a4f55cdc6061e809b09fc25d8376a
verification-method: static-source-inspection
supersedes: []
---

# Human approval that authorizes exactly what was reviewed

**Decision:** complete the missing human-approval integration as a request-bound, host-owned lifecycle. Do not simply connect an Approve button to an action-type permission fact. The approved operation, its destination, its arguments, its relevant source revisions, and the human decision must refer to one immutable request.

**Status:** proposed design grounded in source, not shipped behavior or an executed security assessment. No implementation, tests, model inference, training, benchmark, or live operation was performed. The source baseline is `24eb5faaa65a4f55cdc6061e809b09fc25d8376a`; conditional examples are proposed fixtures, not reproduced incidents.

## 1. Why this is a production capability, not just a dialog

A coding harness must sometimes stop at an authority boundary without abandoning the task or asking the model to reconstruct the operation after a human replies. Reliable integration means the operator can see what is proposed, authorize that exact scope, and have the executor resume through its existing checks. A human's approval is neither a passing test nor permission to bypass unrelated constraints.

This serves the root North Star's typed policy-mediated operations and logic-owned executive control (`agents.md`, **North Star** and **The Vision**, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/agents.md#north-star)). It also serves accuracy and efficiency: preserve the frozen plan while waiting rather than spending another model turn rediscovering its parameters. Those are goals, not measured improvements.

The missing UI host is **already recognized**, not a new discovery. The existing diff wiring document explicitly asks for the signer, signature scope, and rejection-to-kernel flow (`Docs/architecture/diff/WIRING-AND-NOT-BUILT.md`, **Exists with no host, and why it is kept**, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/Docs/architecture/diff/WIRING-AND-NOT-BUILT.md#exists-with-no-host-and-why-it-is-kept)). This document supplies a concrete lifecycle and acceptance design for that unfinished obligation. It is distinct from [tool effect receipts](tool-effect-receipts-and-validation-completeness.md), which begins with execution/effect evidence, and [structural refactor preconditions](structure-coverage-and-refactor-preconditions.md), which establishes the source evidence supporting a plan. Approval consumes such a plan; it does not replace either mechanism.

## 2. Current source: implemented pieces, missing integration, and stale claims

| Boundary | Inspected source | What it establishes |
|---|---|---|
| JIT capability check | `executeToolCall`, [`internal/session/executor_tools.go:2354–2363`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/session/executor_tools.go#L2354-L2363) | Registry membership is not capability; the effective tool allowlist is checked before dispatch. Preserve it after approval. |
| Current denied-call experience | `executeToolCall`, [`internal/session/executor_tools.go:2424–2466`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/session/executor_tools.go#L2424-L2466) | A denied action classified as requiring permission returns an error telling the agent it cannot self-authorize and should ask the user to perform it or choose another way. This branch does not create a resumable human-approval request. |
| Exact pending request | `checkSafetyWithLease`, pending-action construction and exact query, [`internal/session/executor_tools.go:2839–2863,2900–2907`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/session/executor_tools.go#L2839-L2907) | Current authorization already observes action, target, serialized payload and a call ID. Preserve this precision rather than introducing a broad bypass. |
| Dormant human permission arm | `permitted/3`, [`internal/core/defaults/policy/constitution.mg:13–17`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/core/defaults/policy/constitution.mg#L13-L17) | Requires dangerous action, `signed_approval(Action)`, a pending action, and `admin_override(User)`. The approval is action-type scoped; the user variable is not joined to that request or signature. This is insufficient granularity for a new per-request consent service. It is not evidence that such a service is currently exploitable. |
| Existing view model | `PendingMutation`, [`cmd/nerd/ui/diffview.go:110–121`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/cmd/nerd/ui/diffview.go#L110-L121) | Stores ID, path, diff, reason, warnings and decision booleans. It has no explicit payload digest, source revision, policy revision, actor binding or expiry. |
| Existing view decisions | `ApproveCurrent`, `RejectCurrent`, `ApproveAll`, [`cmd/nerd/ui/diffview.go:323–355`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/cmd/nerd/ui/diffview.go#L323-L355) | They mutate view booleans. `ApproveAll` iterates every mutation and clears rejection, despite the comment saying pending mutations. That behavior needs an explicit product decision before this view becomes an authority input. |
| Host integration evidence | `NewDiffApprovalView`, [`cmd/nerd/ui/diffview.go:163–180`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/cmd/nerd/ui/diffview.go#L163-L180) and constructor search | Repository search found the definition, tests, docs and dead-code baseline, but no production caller. This supports the documented missing host; it is a static search conclusion, not a live TUI exercise or proof about future revisions. |
| Narrow recoverable deletion | `gitRestoresExactly`, `recoverableDeleteFact`, [`internal/session/delete_recoverable.go:63–85,116–127`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/session/delete_recoverable.go#L63-L127); `permitted(/delete_file,...)`, [`internal/core/defaults/policy/constitution.mg:19–35`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/core/defaults/policy/constitution.mg#L19-L35) | There is already a separate path for workspace-contained files Git reports tracked and clean. The old diff document's general statement that deletion is unreachable must not be repeated as current truth. Approval work must not broaden this exception or incorrectly require a prompt for every permitted delete. |

Search for `signed_approval` located policy/schema, protections, documentation and test assertions, but no production human-consent producer in the searched results. Combined with the denial branch and disconnected view, the supported conclusion is an unfinished approval integration, not “CodeNERD has no safety.”

### MCP risk acknowledgement is not human consent

`checkRisk` explicitly describes `confirm_risk` as agent-settable deliberateness, not a security boundary (`internal/mcp/controlplane_invoke.go:184–208`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/mcp/controlplane_invoke.go#L184-L208)). Do not connect that flag to a human-approved state.

The actual MCP boundary has additional protections: `dispatchRemoteCall` requires current connection, unique metadata and host-reviewed effect; later it requires a unique positive `mcp_remote_permitted` result, checks canonical arguments and final registration identity, then invokes the frozen tree (`internal/mcp/remote_authority.go:186–223,367–415`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/mcp/remote_authority.go#L186-L415)). The policy joins exact pending-action permission separately from risk acknowledgement (`mcp_remote_permitted`, `internal/core/defaults/policy/policy_mcp.mg:111–122`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/core/defaults/policy/policy_mcp.mg#L111-L122)). This proposal extends human evidence, not a claim that setting one flag currently bypasses those checks.

## 3. The integration trap: a display becomes decision-critical

The current diff engine can skip exact input verification on cache hits. `Options.VerifyCacheContent` documents that choice (`internal/diff/diff.go:180–187`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/diff/diff.go#L180-L187)); `diffCache.get` performs the optional exact comparison (`internal/diff/cache.go:96–124`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/diff/cache.go#L96-L124)). The importer test's rationale is that current consumers only display diffs rather than apply them (`TestDiffImporters_AreDisplayOnly`, `internal/diff/importers_test.go:15–33`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/diff/importers_test.go#L15-L33)).

Once a displayed diff influences an authorization, “display only” is no longer enough to dismiss a wrong preview as harmless. This does not require inventing a cache collision incident. The proposed approval host must render from the exact frozen inputs, enable exact cache verification or bypass the cache for decision previews, and bind the decision to the underlying request rather than the mutable rendered object.

A diff also cannot summarize every effect: a command can access the network, a deletion can remove a directory, and a remote tool can mutate a service. The review model needs structured operation-specific effects and unknowns, not a reassuring patch pane for every action. Truncation, hidden whitespace, binary payloads and collapsed details must be visibly identified. A digest is an integrity identifier, not an explanation a person can meaningfully approve.

## 4. Proposed request and decision contracts

The following names are design notation, not existing Go APIs or Mangle declarations.

```text
ApprovalRequest:
  request_id, execution_intent_id, request_version
  workspace_identity, session_identity, requesting_agent_identity
  backend_identity, tool_identity, tool_registration_version
  canonicalization_version, canonical_arguments_digest
  action, resolved_targets, effect_scope
  relevant_source_revisions, remote_endpoint_and_schema_identity
  policy_revision, required_approver_role
  preview_digest, preview_completeness, displayed_effect_summary
  expires_at, status

ApprovalDecision:
  request_id, request_version, request_digest
  actor_identity, trusted_channel_identity, decision_event_id
  approve | reject | cancel
  decided_at, reason_optional

ExecutionClaim:
  execution_intent_id, request_digest, decision_event_id
  reserved | dispatch_started | completed | outcome_unknown
  linked_effect_receipt_id
```

For request R, use a versioned canonical encoding and collision-resistant digest:

```text
RequestDigest = H(domain_separator || canonicalization_version || Encode(R_authority_fields))
```

Keep object key order deterministic, array order meaningful, string bytes and numeric distinctions well-defined, and unsupported representations rejected. Do not equate semantically different paths or endpoints merely by normalizing their display strings. Resolve identities under the same path and server rules used by the executor. If canonicalization rules change, invalidate incompatible requests rather than silently reuse old hashes.

The model may propose an operation and inspect a bounded approval status. It must not possess a bearer credential that it can replay as consent. A trusted host callback resolves a request ID in a protected store and verifies actor/channel provenance; model prose, repository files, tool output, or a string saying “Steve approved” cannot create the decision. A local-only UI need not invent cryptographic signing infrastructure: a protected host event plus immutable record may suffice for its threat model. Distributed approvers require authenticated transport, replay protection and explicit trust roots. Calling a boolean “signed” proves none of those properties.

### State transitions

```text
proposed -> awaiting_human
awaiting_human -> approved | rejected | canceled | expired | superseded
approved -> reserved | revoked | expired | superseded
reserved -> dispatch_started | admission_failed
dispatch_started -> completed | outcome_unknown
```

Each transition is atomic with a version check. Rejected, canceled, expired and superseded requests cannot become approved by a bulk action. A materially revised request needs a new displayed version; a duplicate UI event is idempotent. A queued request is not active permission.

Bulk approval names the exact displayed set of request IDs and versions. It excludes rejected entries and entries added after the user saw the batch. It neither means “approve all future work” nor changes policy. If the product intentionally supports revisiting a rejection, expose it as a separate explicit decision on that exact request, not as a side effect of approving pending items.

## 5. Admission and resume algorithm

1. **Classify the denial.** Distinguish human-approvable obligations from hard prohibition, missing evidence, unsupported operation and unavailable policy. Only a policy-declared approvable operation creates an approval request. The host must not convert every refusal into an override prompt.
2. **Freeze before presenting.** Capture the canonical operation and relevant revisions. Construct an operation-specific preview from those same inputs. Release executor/kernel locks while waiting; retaining a request is not retaining a live permission lease indefinitely.
3. **Suspend the dependent action.** Persist bounded task continuation state and surface one request. Do not make repeated model calls while no new evidence can change the decision. Other authorized independent work can continue without inheriting the pending scope.
4. **Receive trusted decision.** Check actor authority, channel/session binding, request version, expiry and duplicate event ID. Commit the decision before announcing approval. Rejection is delivered as a typed executive fact, not merely prose for the model to reinterpret.
5. **Revalidate current state.** On resume, resolve current tool/version/endpoint and resource revisions; rerun capability, project, secret-path, constitutional and required speculative gates. Human consent satisfies one obligation, not all of them. A material change yields superseded/stale request and no execution.
6. **Claim exactly one execution intent.** Atomically reserve the approved intent with current policy admission. Concurrent workers cannot each consume the same approval. Record dispatch start durably where crash recovery requires it, then dispatch through the existing backend, never an alternate approval-only executor.
7. **Record outcome independently.** Link effects and validation evidence to the intent. A failed validator does not imply no effect. An unknown result remains unknown; approval is not a reason to repeat a non-idempotent operation.
8. **Resume the task from evidence.** Completed outcome reuse requires current read authorization. A known no-dispatch admission failure may return to an eligible state only under explicit lifecycle rules. An uncertain dispatch must be reconciled; it cannot automatically release the grant for another attempt.

This is an at-most-one **host dispatch claim** under the chosen storage/coordination assumptions, not an exactly-once guarantee for arbitrary remote effects. A crash between effect and outcome persistence needs backend idempotency or reconciliation. A local check followed by an unlocked filesystem write also leaves an external-writer race; use the supported ownership/compare-and-swap mechanism or disclose/refuse the stronger guarantee.

### Go, Mangle and UI ownership

- **Go host:** freezes requests, captures trusted decision events, records lifecycle transitions and time observations, validates resource/tool identity, implements the atomic execution claim, and reports effects.
- **Mangle executive:** decides whether the action is approvable, whether current host evidence satisfies consent, which constraints still block it, and what continuation is allowed. Permission remains derived from current evidence.
- **UI:** renders a host-provided immutable request and submits a decision event; it does not directly assert `permitted` or widen the allowlist.
- **Model/JIT:** explains the outstanding obligation and available next steps using selected prompt atoms. It does not manufacture consent, relabel a rejection, or choose a shell equivalent to evade a denied effect.

The current `grantConsent` classification protects consent predicates within the grant-path analysis (`internal/mangle/grant_path.go:43–58`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/mangle/grant_path.go#L43-L58)); `ValidateLearnedRuleProtected` and the protected heads include approvals (`internal/mangle/schema_validator.go:221,373–376`, [source](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/mangle/schema_validator.go#L373-L376)). A new approval predicate must be declared, assigned a canonical owner, protected against learned/model-authored grants, and routed only to necessary consumers. Do not add an unprotected host-looking fact or broadly mirror authority across shards. Update the current broad approval arm rather than asserting both broad and exact grants side by side.

## 6. Costs, efficiency and simpler alternatives

Record a resource vector, not a fictional single “token cost”:

```text
model: request explanation + resumed context + retries + reasoning/output
host: canonicalization + hashing + preview construction + policy evaluation
storage: request/preview bytes + decision log + execution claims + retention
human: review time + interruptions + stale-request re-review
outcome: unauthorized attempts, correct completion, blocked tasks, repair effort
```

For argument size A and relevant source/preview input size B, serialization and hashing over an already ordered representation cost O(A+B). General canonicalization additionally pays for object-key ordering: comparison sorting takes O(k log k) comparisons for an object with k keys, with string-comparison cost depending on key lengths, summed over nested objects. Retained request data is O(A+B) if full inputs are stored. Content-addressed references can reduce duplicate storage but still need retention ownership, access checks and garbage collection. Large source manifests and repeated policy evaluation are real costs. Request expiry must release references without erasing the minimum durable evidence required to prevent replay or explain an uncertain execution.

Compare at unchanged authority:

1. **Current refusal/manual completion:** no integrated request service; measure user work and reconstruction, not just harness tokens.
2. **Structured refusal + frozen plan export:** user performs the action outside CodeNERD; no runtime approval grants. This can improve usability at lower trusted-computing-base cost.
3. **Single-request trusted approval:** minimal end-to-end lifecycle, no bulk approval or cross-device signing. Establish correctness first.
4. **Batched/versioned requests:** add only if review burden and stale-plan rates justify complexity.

Do not compare the old system's refusal with a new system silently allowed more authority and call the resulting completion rate an intelligence gain. Use tasks the same operator is willing and allowed to authorize, and record the human's actual decisions. The desired efficiency gain is less repeated explanation/reconstruction for the same verified outcome, not fewer meaningful consent decisions.

## 7. Failure modes and proposed evaluation

No evaluation below was executed.

| Fixture | Required result |
|---|---|
| User approves one file deletion; another same-action request is pending | Only the exact approved request can become eligible. |
| Args, target, tool version or remote schema change after display | Stale request; no effect until revised scope is reviewed. |
| Relevant source changes while waiting | Refuse or re-present according to declared revision contract; do not overwrite concurrent work. |
| Model emits approval-like text or `confirm_risk=true` | No human decision is created; existing independent MCP authority remains enforced. |
| One mutation rejected, another pending, then approve pending batch | Rejection stays rejected; later-added entries stay pending. |
| Duplicate click and two workers race to resume | One decision identity and at most one dispatch claim. |
| Policy revocation or capability removal after approval | Current admission fails; historical consent does not resurrect authority. |
| Preview cache returns wrong inputs under forced key collision | Exact verification/recompute prevents incorrect decision preview. |
| Preview is truncated, binary or effectful beyond a patch | Explicit incompleteness/effect summary; no misleading clean-diff implication. |
| Crash before dispatch / during effect / after effect before receipt | Distinct not-started versus unknown states; no blind replay. |
| Disconnect/restart while awaiting human | Restore pending state without inventing a decision or holding stale kernel facts. |
| Approver loses role or UI channel is impersonated | Refuse decision/admission under declared policy; log bounded diagnostic evidence. |
| Recoverable tracked delete already permitted | Preserve existing policy behavior; avoid unnecessary consent regression. |
| Hard-prohibited resource with affirmative user UI click | Remains prohibited; dialog is not a universal override. |

Exercise the registered executor path and actual UI-to-host boundary, not only direct kernel facts or view booleans. Test source presence alone does not establish deployed coverage. Use deterministic event schedules and a fake effect counter for race/replay fixtures, then controlled integration fixtures for each supported backend.

For end-to-end evaluation, pair identical coding tasks and snapshots across alternatives, hold model and budgets fixed, randomize assignment, and include human decision time. Vary concurrent pending requests, size of previews, wait duration, source churn and restart frequency. Report verified completion, correct refusal, stale-request rate, duplicate dispatches, inappropriate approval scope, total tokens, host resource use and human effort with uncertainty intervals. Do not optimize by hiding failures or removing mandatory review; a zero-dispatch system trivially avoids duplicate effects but does not satisfy the capability.

## 8. Primary foundations, with bounded claims

- [Saltzer and Schroeder, *The Protection of Information in Computer Systems*, basic principles](https://web.mit.edu/saltzer/www/publications/protection/Basic.html): complete mediation, least privilege and fail-safe defaults motivate current-state admission and narrow consent. The paper does not prove this proposed implementation safe.
- [RFC 9396, OAuth 2.0 Rich Authorization Requests, §§2, 6.1 and 12](https://www.rfc-editor.org/rfc/rfc9396.html): structured fine-grained authorization details are a useful precedent for binding actions and resources instead of a coarse verb. This is a design analogy, not a proposal to require OAuth for a local TUI or claim conformance.
- [RFC 8785, JSON Canonicalization Scheme, §3](https://www.rfc-editor.org/rfc/rfc8785.html#section-3): repeatable hashing needs an explicit canonical representation and input constraints. Choose a compatible encoding for CodeNERD's actual argument types; merely using JSON is not a canonicalization proof.
- [RFC 9110, HTTP Semantics, §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2): non-idempotent replay requires knowledge that it is safe or that the original was not applied. The analogous lesson is that human approval does not resolve uncertain execution.

These are established principles, not a novelty or superiority claim.

## 9. Buildable proposal queue

These IDs are local research work items, not claims of adopted repository gaps. Implementation would require a separate authorized change to the owning specs and code; this document changes neither.

| ID | Deliverable | Checkable exit |
|---|---|---|
| APPROVAL-01 | Typed request, policy classification and continuation | Denied approvable call yields one frozen request; hard denial cannot create an override request; waiting consumes no repeated model loop. |
| APPROVAL-02 | Trusted UI decision and exact preview | Real UI event produces matching immutable decision; model text cannot; forced wrong-cache-input fixture recomputes or rejects. |
| APPROVAL-03 | Exact Mangle admission and protected fact flow | Approval for one request does not admit a sibling; schema/owner/routing and learned-grant protections cover new predicates; old broad fact cannot leak authority. |
| APPROVAL-04 | One-shot dispatch and uncertain recovery | Concurrent resume dispatch counter stays one; injected crash outcomes remain distinguishable and uncertain calls are not replayed automatically. |
| APPROVAL-05 | Human-effort and whole-task evaluation | Same-authority paired study reports full costs and verified outcomes, including manual completion and structured-export baseline. |

The first useful milestone is not “Approve button appears.” It is one authorized operation completing through the ordinary gates, with evidence that a changed, rejected, duplicate, or unrelated operation cannot borrow that approval.

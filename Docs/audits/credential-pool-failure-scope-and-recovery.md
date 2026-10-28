---
doc-class: deep-dive
subsystem: perception
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Credential-pool failure scope and recovery

**Decision:** separate whether a request may be retried from whether its credential should be disabled for unrelated work. A request-specific HTTP 403 must not establish that its credential is unusable for every future request. Conversely, preserving failure scope must **not** become permission to route around moderation, guardrails, model restrictions, or organization budgets.

**Evidence status:** source inspection and proposed design, not a reproduced outage. No tests, model calls, application probes, or benchmarks were run, and no implementation is supplied. Acceptance fixtures below are proposed, not passed results.

**Source pins:** main [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871); `feat/conversation-brief` [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c); research inventory [`1e1a893e7995c799b8b547c229a765ca4772a3c6`](https://github.com/theRebelliousNerd/codenerd/commit/1e1a893e7995c799b8b547c229a765ca4772a3c6). Inspection occurred on 2026-10-28. Main implementation links below use that immutable revision.

## Scope and relationship to existing work

This develops the explicitly deferred credential-pool candidate in [the stream-completion audit](https://github.com/theRebelliousNerd/codenerd/blob/1e1a893e7995c799b8b547c229a765ca4772a3c6/Docs/audits/stream-completion-contract.md#deferred-candidate-and-exact-limits). It does not repeat that audit's non-stream recovery, classification-entry-point, or stream-terminal findings. Its distinct subject is a failed request's mutation of shared credential health and the resulting availability of later, independently permitted work.

The repository's [root guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md) keeps executive authority in the harness and Mangle. [Perception guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/agents.md) requires preserving typed provider failures, avoiding secret disclosure, and respecting explicitly configured providers/models. This proposal refines transport evidence and scoped state; it does not authorize substitute models or move permission decisions into a provider retry heuristic.

Three separate questions need answers:

1. Is this particular request permitted and retryable?
2. What request, resource, credential, endpoint, account, or time window does the failure concern?
3. What shared health state may consequently change?

## Primary provider evidence

The unambiguous witness is a **content-filter 403**, not a claim about every provider's budget status code. OpenRouter's official guardrail documentation states that a custom content filter can block a matching request with HTTP 403 before it reaches the model. It separately states that model allowlists and data-region restrictions also produce 403 responses. Those restrictions remain authoritative; another credential must not be tried merely to escape them. [Official guardrail contract](https://openrouter.ai/docs/guides/features/guardrails)

OpenRouter's errors guide also describes moderation and runtime guardrail 403 responses. Its budget discussion contains structured scope/reset information, but budget status codes and remedies vary by limit category and documentation section. Budget handling below is therefore a **conditional, provider-dependent extension**, not the basis for the confirmed source mismatch. An implementation must pin the applicable provider contract and fixture; it must not assume that every budget failure is 403 or that a reset timestamp guarantees subsequent admission. [Official errors guide](https://openrouter.ai/docs/api_reference/errors-and-debugging)

Anthropic separately distinguishes 401 authentication errors from 403 lack of permission for the specified resource. That supports retaining resource scope, but does not establish that every Anthropic 403 is narrow or recoverable. [Official API errors](https://platform.claude.com/docs/en/api/errors)

These documented cases substantiate the earlier deferred hypothesis. They do not establish which provider, configuration, or error the user encountered.

## Source-grounded behavior

The current implementation intentionally shares pool health. Its README explicitly documents process/pool-lifetime disabling on 401/403; this is not an undocumented accidental branch. [Current contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/README.md#L93-L115)

The narrower issue is that this status-only policy conflates distinct provider outcomes:

| Boundary | Current behavior |
|---|---|
| Pool identity | `sharedKeyPool` hashes provider plus resolved key records. Endpoint and model are absent. Existing identity returns the same mutable health object. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool.go#L33-L52) |
| Failure observation | The transport captures status, `Retry-After`, and network-failure state. It reads a bounded error body, but does not carry a typed semantic cause or failure scope into pool bookkeeping. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool_transport.go#L18-L38), [HTTP handling](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool_transport.go#L59-L74) |
| Shared mutation | Every 401 or 403 sets `health[i].disabled = true`. There is no expiry in that state. Selection skips disabled entries and fails if none remain. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool.go#L63-L118) |
| Retry decision | `finish` replaces a recorded status with a status-only pooled error and permits an unpinned 403 to rotate. `Do` then tries each remaining credential once. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool.go#L187-L246) |
| Actual OpenRouter request | `CompleteWithSystem` sends `/chat/completions`; a non-200 403 returns an HTTP-status error after closing the observed body. Thus the documented provider response reaches the pool policy above. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/client_openrouter.go#L139-L183) |
| Streaming | The corresponding pre-stream HTTP error follows the same observed transport. The pool retries only before content/thought emission, but that restriction does not distinguish a pre-output guardrail rejection from bad credentials. [OpenRouter source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/client_openrouter.go#L292-L339), [pool source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool.go#L249-L307) |

### Production reachability

Explicit API pools enter `newPooledClient` through both normal and classification factories. Each credential retains its own brokered client, while all leaves use the shared health pool. Preserve that existing metering and same-key admission arrangement. [Factories](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/client_factory.go#L898-L917), [pooled construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool_factory.go#L29-L74)

Worker/planner construction resolves the provider's same pool while accepting a different model and an explicit per-slot endpoint. Therefore endpoint/model differences are genuinely configurable, not merely hypothetical direct-constructor inputs. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/client_factory.go#L789-L827)

The checked test source already exercises sharing across boot slots and 401 suppression across models. Credit these intended behaviors; neither inspected fixture distinguishes a scoped 403 from credential invalidity. This statement describes those fixtures, not an executed test result or proof that no other coverage exists. [Boot-slot fixture](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_key_pool_test.go#L18-L83), [401 fixture](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/key_pool_test.go#L123-L157)

## Source-derived witness—not executed

Configure two valid OpenRouter keys under the same request-blocking content filter and use the ordinary pooled factory. Let both broker admissions succeed, keep the parent context alive, and have each endpoint response return the documented HTTP 403 guardrail shape before output.

The inspected path:

1. Records the first 403.
2. Permanently disables that key in shared pool state.
3. Rotates to the second key.
4. Receives the same request block and disables that key.
5. Returns the last failure.

A subsequent independently permitted request using that identical pool configuration cannot select either key, even if the provider would now accept it. It can fail locally before reaching the endpoint. Another model or slot sharing the pool inherits that state.

The two-key example generalizes only while each attempt reaches the endpoint and receives that response. Cancellation, broker rejection, earlier success, pinned resources, or a different pool identity alter the trace. This is not a claim that every request makes the configured maximum number of calls.

A second witness uses a forbidden model followed by an allowed model. A third uses a rejected endpoint followed by the configured permitted endpoint. Neither witness should repair the original denial by silently choosing another model, region, or credential.

## Proposed scoped-health contract

Introduce a bounded, sanitized provider failure descriptor containing:

- provider and configured endpoint identity;
- credential alias or non-secret identity;
- typed cause and confirmed scope;
- affected resource/model where known;
- reset/revalidation information, when applicable and supported by the provider contract;
- retry disposition, separately from health mutation.

Do not retain raw flagged input, provider body, credentials, or arbitrary metadata in receipts. Parse only bounded allowlisted metadata needed for the decision; unknown fields and provider prose must not grant retry authority.

| Established cause | Proposed disposition |
|---|---|
| Invalid/revoked credential | Stop using that credential within its established authentication domain; retain existing revocation protection. |
| Content moderation or runtime guardrail block | End the request with a safe typed denial. No automatic alternate-key retry and no unrelated credential-wide poisoning. |
| Model/resource/region denial | Keep the denied operation blocked. Preserve the scope; do not infer that all permitted operations using the credential fail. |
| Budget window exhausted, where a provider-specific contract establishes it | Preserve the actual budget scope and reset evidence regardless of status spelling. Do not rotate to evade the budget. Revalidation after a legitimate reset is not guaranteed success. |
| Rate limit or transient outage | Retain bounded cancellation-aware cooldown/retry, scoped according to trustworthy provider evidence. |
| Unclassified 403 | Fail the operation conservatively without pretending either "invalid key everywhere" or "safe to retry elsewhere" has been proved. |

Load balancing and health need not use identical keys. A shared load counter may remain useful across models, while credential authentication, endpoint availability, model admission, and quota windows have different identities.

Simply adding model to the entire pool key is insufficient: it loses beneficial credential-wide revocation sharing without addressing request-specific blocks. Likewise, blanket timed re-enabling could repeatedly use revoked credentials. Preserve intentional disable semantics where supported, and define explicit recovery or configuration-change transitions for the other scopes.

No automatic change to credentials, provider, model, endpoint, region, or account policy is implied. A safe recovery contract distinguishes a later independently authorized request from a retry intended to bypass the original denial.

## Proposed verification—not run

Exercise the actual factory → broker/router → observed transport → provider adapter path, rather than only direct calls to pool helpers:

| Fixture | Required observation |
|---|---|
| Content-filter 403, then a permitted request | No alternate-key replay of the blocked request; the later request remains selectable. |
| Denied model, then an allowed model | First denial remains enforced without poisoning the second. |
| Denied endpoint, then permitted configured endpoint | No automatic region change; endpoint-local failure does not silently become credential-global. |
| Revoked key plus healthy key | Existing authorized credential-failure behavior remains correct; no revoked-key revival. |
| Provider-specific budget reset, absent reset, and lifetime budget | Distinct state under the exact applicable wire contract; no early retry or budget circumvention. |
| Unknown, malformed, oversized, or contradictory error metadata | Safe conservative disposition and bounded parsing; missing metadata does not imply safe failover. |
| Streaming pre-output denial and post-output failure | Unchanged no-splicing guarantee; semantics govern pre-output retry eligibility. |
| Concurrent slots observing different scopes | Deterministic health transitions without reviving revoked credentials or globally spreading a request-specific block. |
| Pinned cached-content resource | Health refinement never moves an account-owned handle to another credential. |
| Reconstruct client with unchanged configuration | Documented pool state persists; supported recovery does not require secret mutation merely to change a digest. |
| Parent cancellation during classification or revalidation | No newly admitted attempt after cancellation; health outcome remains attributable to actual provider evidence. |

Proposed fixtures should use synthetic provider responses through the real production wiring. A subsequent live provider validation requires separately authorized configuration and should not intentionally bypass or weaken guardrails.

## Costs and acceptance

Measure admitted attempts, rejected attempts, unnecessary credential rotations, later permitted-request success, time to legitimate recovery, false global disables, and false credential revivals. Include provider/token-count requests, billed tokens where reported, host work, user waiting, and intervention. A pre-inference guardrail block must not be assumed to incur generation tokens; zero recorded usage must not be mistaken for zero operational cost.

Use the existing broker/usage receipt infrastructure rather than a parallel billing counter. Keep failure scope attributable to each attempt and distinguish absent usage from observed zero; the separate detailed-usage audit remains responsible for its broader accounting changes.

Acceptance requires both greater availability for **independently permitted** work and no weakening of provider/user restrictions. Compare the status-only baseline against one scoped-health change at a time, including legitimate global revocation and ambiguous-error controls. Report failures and interventions alongside latency or request-count reductions. No speedup, cost reduction, or production incident frequency has been measured.

## Branch comparison and delivery boundary

`key_pool.go`, `key_pool_transport.go`, `key_pool_factory.go`, `client_openrouter.go`, `key_pool_test.go`, and `factory_key_pool_test.go` have identical Git blob SHAs between the pinned main and feature heads. Consequently this source-supported seam remains present on the inspected feature branch; its unrelated admission and conversation work should not be credited as resolving it.

The smallest useful delivery is a provider-aware failure-scope descriptor plus policy that prevents request-specific 403s from globally disabling credentials or triggering alternate-key replay. Preserve true credential invalidation and the existing stream emission, metering, cancellation, and account-affinity boundaries. Broader quota-window recovery follows only after its provider-specific wire contract is independently pinned.

This standalone proposal is a substantive extension of the stream audit's deferred candidate, not an assertion that the owning perception spec has adopted or implemented it.

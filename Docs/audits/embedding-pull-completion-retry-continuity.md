---
title: "Ollama pull completion: preserve failure before publishing readiness"
doc_class: audit
subsystem: embedding
status: proposed
evidence_status: source-only
last_verified: 2026-11-05
source_revision: 50157d8f0b2770bc7aabd261e25f173840f7dd9f
method: static source inspection
supersedes: []
---

# Ollama pull completion: preserve failure before publishing readiness

## Finding and proposed decision

**The Ollama embedding adapter can publish model readiness without a complete, successful pull response.** `pullModel` discards the error from reading the response body and accepts HTTP 200 even when the body is empty, malformed, or contains only progress. `EnsureModel` interprets that nil return as success, sets `modelReady`, and retains `pullAttempted`. When a startup-only cancellation occurs after response headers, this can bypass the explicit cancellation-retry reset. If the model remains absent, a later embedding request can invalidate readiness but cannot make another automatic pull on that engine. [Pull consumer][pull] [Readiness transition][ensure] [Missing-model recovery][missing]

The new boundary is **the readiness owner's completion evidence and its downstream retry state**, not another caller waiting for the owner's mutex. The gap exists with one caller, one model, no vector store, and no long-input splitting.

**Proposed decision:** publish readiness only after a fully admitted pull result or an independently successful exact-model inventory observation. Propagate response-body failures, distinguish terminal success from progress or an unreadable response, and preserve the existing retry allowance after caller-context cancellation. Keep optional startup degradation, exact model selection, synchronized state, and vector validation. Changing the retained-attempt policy for genuine non-cancellation failures is a separate decision.

**Important reachability limit:** current upstream Ollama's non-streaming handler waits for its operation channel to finish before returning JSON. A normally slow direct `stream:false` download therefore does not itself establish the post-header, still-missing-model witness below. That witness deliberately exercises an interrupted/compatibility/intermediary response at the consumer's HTTP boundary. CodeNERD explicitly contains a compatibility path for NDJSON despite requesting non-streaming, but this audit has not identified a deployed version that needs that path. The upstream behavior is a control, not a reason to infer that arbitrary HTTP 200 bodies establish readiness. [Upstream request dispatch][upstream-pull] [Upstream non-streaming response][upstream-wait]

This is a source-supported admission defect with designed, unrun witnesses. No production occurrence, download failure rate, permanent universal outage, incorrect accepted vector, performance improvement, or repair is claimed. No tests, builds, application runs, failure injection, model pulls, inference, or training were performed.

## Intent, present status, and non-overlap

The [root requirements][root] assign deterministic truth and typed effect boundaries to the harness. [Embedding guidance][guidance] says provider responses are untrusted, mutable readiness/pull state must remain synchronized, and a short bootstrap ensure timeout must not permanently disable a later request-scoped pull. The architecture's accepted bootstrap extension distinguishes an expired optional startup cap from cancellation of the whole parent operation. Those are appropriate requirements to preserve. Historical architecture entries still name older model-selection helpers; implementation claims here come from the pinned source, not those stale descriptions. [Architecture][architecture] [Source-over-stale-prose requirement][root-source]

| Status | Evidence and limitation |
| --- | --- |
| Implemented | Caller-derived startup ensure context, optional-service construction, exact-model inventory resolution, one synchronized readiness owner, separate HTTP request/pull timeouts, recognized provider-error rejection, and vector validation. |
| Partially implemented | Cancellation permits a later pull only when `pullModel` returns an error. A body-read cancellation can be discarded before that branch, so the intended startup retry guarantee is not complete across response phases. |
| Proposed here | Complete pull-result admission, truthful readiness publication, phase-sensitive failure evidence, and a regression matrix covering body failures and later recovery. |
| Unverified | Runtime frequency, affected real server/proxy combinations, the NDJSON compatibility claim, download resource impact, acceptance of a repair, and all evaluations below. |

Public main was inspected at [`50157d8f0b2770bc7aabd261e25f173840f7dd9f`][pin]. Repository guidance, embedding architecture, current source and test bodies, the current audit tree, and targeted repository searches were inspected before choosing this topic. The following related work remains authoritative for its own boundary:

- [Readiness-waiter cancellation][prior-waiter] concerns a second caller unable to leave a network-held mutex. It explicitly does not establish false success. This report instead follows the owner's received bytes into readiness and later retry state. Releasing waiters independently would not fix the ignored body error.
- [Embedding representation and migration][prior-representation] concerns input/vector identity, truncation controls and durable eligibility. Here no vector has yet been accepted. Repairing namespace or migration semantics does not establish pull completion.
- [A closed stream is not a completed answer][prior-stream] already establishes the general distinction between transport termination and application completion for generated answers. This report does not claim that principle is new. Its distinct contribution is the model-acquisition state machine: a misclassified response suppresses the existing cancellation recovery branch and can retain a later-pull refusal.
- [MCP discovery shutdown ownership][prior-mcp] concerns joining producers before tearing down dependencies. No detached discovery worker, shutdown store, or publication race is needed here.

No matching pull-completion/retry-state finding was identified in this bounded review. That is not a claim about all historical branches or unpublished work.

## Current source trace

### From normal boot to a retained embedding engine

The normal factory calls `embedding.NewEngineWithContext`. For Ollama, that constructor performs best-effort readiness under a child timeout, normally eight seconds. It logs either an ensure warning or readiness, then checks the **parent** context before returning the engine. That distinction correctly permits a live parent to continue when an optional startup cap expires. [Factory entry][factory] [Constructor][constructor]

The factory then runs `HealthCheck`. That method checks whether `GET /api/tags` returns HTTP 200; it does not inspect the body for the configured model. A responsive tags endpoint can therefore admit the engine even when that model is still absent. The factory passes the admitted instance to the prompt atom loader and stores, and later to the compiler's default vector searcher. This is production wiring, not just a directly invoked test helper. It does not imply that every boot or prompt immediately needs an embedding. [Health check][health] [Attachment][factory] [Searcher][searcher]

A healthy service is useful evidence. It is simply a different fact from successful acquisition of a particular model.

### Readiness and pull-attempt state

With `ensureMu` held, `EnsureModel`:

1. Returns immediately when `modelReady` is already true.
2. Lists models and accepts only the configured identity, including the existing bare-name-to-`:latest` rule.
3. If the model is absent and `pullAttempted` is true, returns a retained-attempt refusal.
4. Otherwise sets `pullAttempted=true` and calls `pullModel`.
5. On a returned error, resets `pullAttempted` only if the caller context is canceled.
6. On nil, sets `modelReady=true` and logs the model as pulled and ready.

There is no inventory read between step 4's nil result and step 6's readiness publication. [State machine][ensure] [Exact identity][identity]

The inventory is consulted before the retained-attempt refusal. If the server completed installation despite an interrupted client response, or another authorized actor subsequently installed the model, a fresh inventory observation after invalidation can recover readiness. The finding is consequently not an irreversible state corrupting every engine for its entire lifetime.

### What the pull consumer actually admits

`pullModel` sends `stream:false`, calls `Client.Do`, and returns its error if no successful response is obtained. After it has a response, it closes the body on exit but ignores `io.ReadAll`'s error. It rejects non-200 status and recognized nonempty `error` fields. For compatibility it also scans lines backward for the last decodable status object. It never requires `Status == "success"`, and a failure to decode any object does not cause an error. [Request and consumer][pull]

These distinctions follow directly from the branches, without running them:

| Delivered response observation | Current pull result |
| --- | --- |
| `Do` returns an error before providing the response | Error. A canceled caller can reach the retry reset. |
| HTTP status is not 200 | Error after body acquisition, regardless of whether its read error was discarded. |
| Complete HTTP 200 with `{"status":"success"}` | Nil; intended happy path. |
| Complete HTTP 200 with a recognized nonempty JSON `error` | Error. |
| HTTP 200 with zero body bytes | Nil; no terminal evidence is required. |
| HTTP 200 with only malformed body text | Nil; decoding failure is not propagated. |
| HTTP 200 with `{"status":"pulling manifest"}` and clean end | Nil; progress is accepted as completion. |
| HTTP 200 whose body read returns empty or partial bytes plus cancellation/transport error, with no recognized error object | Nil; the body-read error is discarded. |
| A decodable progress line followed by an incomplete error line | Nil if no other recognized error controls the return; the reverse scan can skip the damaged tail and accept earlier progress. |

The last rows are consumer counterexamples, not assertions that a conforming current upstream server deliberately emits those successful responses. They distinguish missing transport completeness, missing syntax validity, and missing semantic completion. Checking only one layer would leave the others unresolved.

## Conditional causal witnesses

### W1. A complete HTTP body without pull completion

Use an isolated engine with a configured model absent from a deterministic fake inventory. The endpoint returns HTTP 200 and a complete body containing only `{"status":"pulling manifest"}`.

The current pull consumer returns nil. `EnsureModel` sets readiness despite having no terminal success object or post-pull inventory confirmation. A second direct ensure returns from the ready fast path without checking inventory. Replacing the body with empty or malformed bytes produces the same acceptance through different branches.

This is the minimal semantic-admission witness. It requires no timing race, cancellation, concurrency, or storage. Its server response intentionally violates the normal non-streaming response contract. Treating provider responses as untrusted means that violation should remain observable, not become readiness.

### W2. Startup child cancellation after headers loses later retry

Use one configured model, a live parent context, and the normal constructor's finite startup ensure context. Use short, valid embedding input later so splitting, batch fallback, and dimensional mismatches are not involved.

1. `GET /api/tags` completes with the configured model absent.
2. The first pull response explicitly sends and flushes HTTP 200 headers, keeps its body unfinished, and does not install the model. Do not set `Content-Length: 0`, which would make the body complete. The fixture must establish client receipt of the headers and entry into body reading before child expiry, using an observable transport/body-read boundary. Server emission alone is not sufficient: a delayed client could otherwise exercise only the already-working pre-header failure branch.
3. Let only the startup child deadline expire while the parent remains alive. The outgoing HTTP request's context also governs reading the response body. `Do` has already returned a response; the body read now produces an error. [Go request lifetime][go-http] [ReadAll result][go-readall]
4. The consumer discards that error and has no recognized error object to reject. It returns nil. `EnsureModel` skips its error branch, retains `pullAttempted=true`, sets `modelReady=true`, and logs readiness.
5. The constructor sees a live parent and returns the engine. In the full factory variant, the tags endpoint still returns 200, so the liveness health check can retain the same engine.
6. A later fresh-context `Embed` passes the ready preflight and receives the recognized missing-model 404. It calls `invalidateModel`, which clears readiness but not `pullAttempted`.
7. The ensuing ensure lists models, still finds the configured model absent, and reaches the prior-attempt refusal before sending another pull. The embedding call returns the auto-pull failure. [Embedding path][single] [Recovery branch][missing] [Invalidation][invalidate] [Retained refusal][ensure]

The state at the decisive boundary is:

- **Observed:** headers received; body acquisition interrupted; no terminal success received.
- **Published by current code:** model ready; pull attempt consumed.
- **Later observation:** configured model absent.
- **Current consequence:** no fresh automatic pull through this engine while absence and the retained-attempt state persist.

This witness uses no second waiter, so even a complete repair of the waiter-cancellation audit leaves it intact. Merely changing the constructor to check its child context would also be incomplete: direct `EnsureModel` and ordinary request-time pulls still share the same consumer, and clean progress-only bodies need not cancel any context.

### What the witness does not establish

Current upstream Ollama, separately pinned at [`cdd4b55b43ae14ea56946802f30f0661223a05cf`][upstream-pin], routes a false stream flag to `waitForStream`, which consumes progress and returns one JSON response after the channel closes. Its [API documentation][upstream-api] describes a single terminal success object for non-streaming, and progress followed by success for streaming. This inspection does **not** establish the server version used by any CodeNERD installation.

Thus W2's early successful headers while the model is absent are a stated fault/compatibility condition. A direct conforming non-streaming server can instead be canceled before headers, in which case `Do` returns an error and the existing retry reset works. If an interruption occurs only after the server already installed the model, later inventory can recover without another download. These negative controls must be included in evaluation, rather than presenting every eight-second startup timeout as poisoned.

Cancellation of the **parent** is another control: the constructor checks it and refuses the engine, even if the inner ensure misclassified its response. The report does not claim cancellation always returns a usable engine.

A successful readiness publication also does not bypass embedding-result checks. Single and native-batch paths validate the vectors they actually receive. This gap can produce false readiness, misleading logs, avoidable failures, and lost automatic recovery under its conditions; it does not by itself put an invalid vector into a store or authorize another model. [Vector acceptance][vectors] [Batch acceptance][batch-native]

## Proposed completion and recovery contract

These are implementation requirements to consider, not shipped APIs or authorization to change model-download policy.

1. **Preserve acquisition failure.** Any response-body read failure remains an error with its cause preserved for `errors.Is`/`errors.As` where applicable. Closing the body is still required. Successful headers or a parseable prefix cannot erase an interrupted read.
2. **Admit the complete protocol result.** For the requested non-streaming form, require a well-formed response carrying the defined terminal success, with no provider error or invalid trailing content. An empty object, empty body, arbitrary string, or progress-only object is not success.
3. **Define compatibility rather than silently guessing.** If NDJSON remains supported, parse its entire admitted sequence in order with bounded records. Require an accepted terminal success and refuse truncation, malformed records, conflicting terminal evidence, or a provider error. Do not scan backward past an undecodable tail and promote an older progress record. If compatibility is removed, document the supported server contract and test the resulting refusal; do not imply an upstream version was qualified without evidence.
4. **Publish readiness from evidence, under existing ownership.** Only an exact-model inventory match or an admitted pull completion may set ready. Keep synchronized selected-model, readiness and attempt state. Network-lock restructuring belongs to the waiter audit and must compose with this rule, not replace it.
5. **Preserve cancellation retry semantics.** A canceled startup ensure must remain not-ready/unknown and retain the existing ability for a later live caller to re-observe inventory and, if still absent, retry. Return the context cause without performing a new recovery request under the canceled context. A cancellation/completion race needs an explicit settlement rule; do not publish ready solely because a read returned some success-shaped prefix.
6. **Do not conflate unknown with absent.** A failed response does not prove the server rolled back a download. The existing next live inventory read is valuable reconciliation. If it finds the exact model, reuse it; do not force a redundant pull. A new inventory failure is not proof of absence.
7. **Keep retry policy separate.** A pull client's own timeout can occur while the supplied caller context is still live. Propagating that read error makes the result truthful but, under today's policy, need not clear `pullAttempted`. Protocol failures and other non-cancellation failures likewise need an explicit policy decision before adding retries. Do not turn this narrow repair into an unlimited download loop.
8. **Keep degradation truthful.** With a live parent, construction may still return an optional, not-yet-ready engine. Logs should distinguish unreachable service, absent model, incomplete pull, provider refusal, and admitted completion. The health probe remains a service-liveness probe unless deliberately expanded; do not label it model verification.

A compact internal outcome such as completed / provider-failed / canceled / incomplete could make settlement clearer, but an enum is not necessary to satisfy the contract. Typed errors plus a correctly placed success branch can suffice. New Mangle facts are not required merely to fix parsing and Go resource/state ownership. If readiness is later projected into the kernel, it must carry the same evidence distinction rather than infer success from a generic nil/error wrapper elsewhere.

### Alternatives and limitations

- **Propagate only `ReadAll` errors:** necessary and sufficient for the specific body-cancellation reset in W2, but W1's complete malformed/progress-only body still passes.
- **Check only `ctx.Err()` at the end:** catches some cancellation but not premature clean EOF, invalid JSON, provider semantics, or client-timeout errors when the caller remains live.
- **Require only one `"success"` substring:** accepts text outside the schema, damaged tails, or contradictory records. Completion is a parsed protocol fact.
- **Always relist after any 200:** independently observing the exact model can establish availability, but adds a request and a new failure window to every successful pull. It does not justify reporting a failed pull response as successful. It may be a deliberate reconciliation policy, not a free substitute for response integrity.
- **Always clear `pullAttempted` on any failure:** changes the retained-attempt policy and can repeat expensive downloads for persistent protocol or service errors. Preserve the cancellation-specific repair unless broader retry behavior is explicitly chosen.
- **Treat every uncertainty as fatal boot failure:** removes intended optional-service degradation. Readiness and the ability to continue without the optional service are different decisions.

## Costs and operational failure modes

The current happy path avoids a post-pull inventory request. Preserve that advantage if an admitted completion is adequate for the supported server contract. The pull response contains status metadata, not the downloaded model weights; the weights move between Ollama and its registry and consume the server's bandwidth and storage. Do not equate response bytes with downloaded model bytes. No embedding inference is needed merely to establish this parser property.

Let `B` be received pull-response bytes and `R` the number of records. Current `ReadAll` retains `O(B)` data; JSON decoding and conversion/splitting for the fallback add work and can add storage proportional to `B` and `R`. An HTTP timeout bounds duration, not response size. A basic strict decoder remains linear in received bytes and may retain the same buffer. An incremental bounded-record design can reduce retained parsing data toward the maximum admitted record size plus small state, but requires record, aggregate-work, and termination limits suitable for legitimate long pulls. No numeric limit is justified by this inspection.

Checking terminal status adds negligible conceptual work compared with a download but still requires measured allocation, parsing and error-path costs. Compatibility parsing, error typing, and outcome telemetry add maintenance burden and regression cases. A post-failure inventory reconciliation adds a tags round trip only when a later live caller actually performs it. An unconditional post-success check adds one to every pull; report that cost if chosen.

For W2, while the model remains absent, repeated nonempty single embedding calls can spend inventory and missing-model requests without reaching another pull. The first call after false readiness can skip its preflight inventory; later calls begin not-ready and can perform an inventory preflight, an embedding request despite that preflight error, and another inventory in 404 recovery. Exact counts depend on other failures and state changes. No universal latency or bandwidth saving is claimed. [Best-effort preflight and request loop][single] [Missing-model path][missing]

A proper cancellation repair may cause a later live request to resume or repeat a download that partially ran remotely. The upstream API describes resumption and shared download progress, but that is not evidence of zero extra network, CPU, disk, or coordination cost for every version. Measure attempted pulls, returned outcomes and actual server-side downloaded bytes separately. Never infer rollback from local cancellation or manufacture exactly-once execution.

Useful bounded telemetry includes: failure phase (before headers / body / decode / terminal admission), response bytes, admitted record count, terminal status class, ensure outcome, whether inventory or pull established readiness, later recovery result, tags/pull request counts and cancellation-to-return latency. Avoid full unbounded response bodies, model credentials, registry authorization, or input text. Diagnostics must not create a new network operation simply to describe a failure.

Repair hazards include accepting success before later corrupt bytes, ignoring an earlier error because a later line decodes, resetting attempt ownership while another operation is active, widening model-name matching, losing typed cancellation, doing reconciliation under a canceled context, adding unbounded retries, and rejecting a legitimately supported response form without an explicit compatibility decision.

## Primary rationale

Saltzer, Reed and Clark's [End-to-End Arguments in System Design][end-to-end] explains why delivery acknowledgement and application completion are different responsibilities. Here the narrow lesson is to make the embedding adapter validate the completion condition needed by its readiness consumer. The paper does not prescribe an Ollama status schema, mandate another HTTP request, or establish a CodeNERD failure rate.

The [Go request contract][go-http] and [`io.ReadAll` contract][go-readall] locate the failure boundary: receiving headers does not complete body acquisition, and partial bytes can be returned alongside an error. The [upstream pull API][upstream-api] supplies the success/progress distinction; the inspected [upstream handler][upstream-wait] supplies the important non-streaming negative control. None of these sources substitutes for qualification against the server version actually deployed.

## Evaluation plan: all cases unrun

Use deterministic local HTTP fixtures first, with the real consumer and state transitions. No model download or model inference is necessary for these tests. Keep configured identity, startup duration, request/pull timeout configuration, fake inventory, and later embedding results the same between baseline and candidate. Record source revision and configuration for every future run.

| Case | Required observation |
| --- | --- |
| Complete single success | One absent-model inventory, one pull, admitted terminal result, readiness true; a subsequent ensure does not duplicate the pull. |
| Installed exact-model control | Inventory alone establishes readiness; no pull. Preserve the bare-name/`:latest` rule and refuse nearby names. |
| Empty / malformed / progress-only 200 | No successful pull or ready publication. Distinct failure classification must survive through ensure and logs. |
| Properly framed but truncated application result | A clean HTTP end without terminal success is incomplete, not success. |
| Network body truncation | Explicit incomplete Content-Length or interrupted chunk framing produces a preserved read error, not readiness. |
| Caller cancellation after headers | Confirm flushed headers and pending body before cancellation. Direct ensure returns the cancellation cause, stays not-ready, and clears the attempt according to the existing cancellation policy. |
| Optional startup child deadline | Parent remains alive; constructor may return a degraded engine. A later live request observes absence, makes a new permitted pull, receives complete success, and can embed valid short input. Assert state and pull counts, not just eventual output. |
| Whole-parent cancellation | Constructor returns the parent cancellation and no usable engine; compare with cancellation before headers. |
| Client-only body timeout | Caller remains live. Return a timeout/incomplete error and no readiness. Do not silently grant new retry policy merely because this test now sees the failure. |
| Server completed despite lost response | Later exact-model inventory recovers readiness without forcing another pull. Do not require duplicate downloading as the only accepted recovery. |
| Provider error and malformed tail | Recognized error remains failure; a damaged last record cannot be skipped in favor of earlier progress. If NDJSON is retained, include progress → error, progress → success, success → garbage and conflicting terminal cases. |
| Compatibility decision | Exercise every explicitly supported response form and reject unsupported ones predictably. Pin any real server versions separately; a fake is not live-provider qualification. |
| Normal factory retention | A 200 liveness probe with absent model cannot convert a failed ensure into proven readiness. Verify truthful degraded state and later recovery on the same factory-shared engine. |
| Single / batch consumers | Both use the same truthful ensure result without weakening actual vector checks or triggering unapproved retries. Include one-item and native-batch paths. |
| Concurrency composition | With the separate waiter repair if implemented, owner settlement remains single, canceled waiters do not alter the owner's attempt state, and no duplicate owner or stale readiness is introduced. |
| Cost and resource controls | Measure response parsing allocations/peak memory, tags/pull counts, cancellation latency, error-log volume, warm-ready overhead, and cleanup after every failure. Compare a valid long progress sequence with bounded malformed/oversized inputs. |

The decisive W2 test must establish **headers received and body pending before the child expires**. A handler that waits before writing headers only retests the existing working branch. Release or cancel all handler barriers and join test goroutines on failure; a passing return-time assertion alone does not establish resource cleanup.

Existing test definitions provide useful controls, not coverage of this seam:

- `TestEnsureModel_BootstrapDeadlineDoesNotPoisonLaterPull` holds the first handler before writing a response. It checks the correct pre-header cancellation reset, not a body failure after `Do` returned. [Test][test-ensure]
- `TestPullModel_ClientTimeoutFollowsInstalledBound` likewise waits before its successful header. It checks the pull client's timeout but not discarded body-read errors. [Test][test-timeout]
- `TestColdBootEmbedding_OptionalStartupCapDoesNotPoisonRetry` uses an already-expired local cap and requires no startup HTTP request. That does not exercise a partially received pull. [Test][test-cold]
- The happy-path ensure test emits `Status: "success"`; the embedding coverage fixtures deliberately mark engines ready to isolate embedding requests from tags/pulls. Their vector and cancellation assertions remain valuable but do not establish pull-result admission. [Success test][test-success] [Coverage seam][test-coverage]

Acceptance requires truthful pull completion, preservation of the existing canceled-startup recovery promise, exact-model identity, legitimate optional degradation, and settled resources. A parser-only unit pass or a live successful download alone would not establish all of those properties.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/50157d8f0b2770bc7aabd261e25f173840f7dd9f
[root]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/agents.md#L3-L28
[root-source]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/agents.md#L60-L65
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/agents.md#L1-L17
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/Docs/architecture/embedding/INTERNALS.md#L3-L9
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/system/factory.go#L1731-L1774
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/engine.go#L101-L168
[health]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L425-L456
[searcher]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/system/factory.go#L1825-L1830
[ensure]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L459-L505
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L631-L645
[pull]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L552-L610
[single]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L95-L141
[missing]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L189-L208
[invalidate]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L419-L423
[vectors]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L234-L267
[batch-native]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama.go#L285-L327
[test-ensure]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama_ensure_test.go#L148-L191
[test-timeout]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama_pull_timeout_test.go#L12-L66
[test-cold]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/coldboot_context_test.go#L54-L80
[test-success]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama_ensure_test.go#L99-L146
[test-coverage]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/internal/embedding/ollama_coverage_test.go#L91-L97
[prior-waiter]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/Docs/audits/embedding-readiness-waiter-cancellation.md
[prior-representation]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/Docs/audits/embedding-representation-and-migration.md
[prior-stream]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/Docs/audits/stream-completion-contract.md
[prior-mcp]: https://github.com/theRebelliousNerd/codenerd/blob/50157d8f0b2770bc7aabd261e25f173840f7dd9f/Docs/audits/mcp-discovery-shutdown-ownership.md
[upstream-pin]: https://github.com/ollama/ollama/commit/cdd4b55b43ae14ea56946802f30f0661223a05cf
[upstream-api]: https://github.com/ollama/ollama/blob/cdd4b55b43ae14ea56946802f30f0661223a05cf/docs/api.md#L1530-L1599
[upstream-pull]: https://github.com/ollama/ollama/blob/cdd4b55b43ae14ea56946802f30f0661223a05cf/server/routes.go#L1486-L1553
[upstream-wait]: https://github.com/ollama/ollama/blob/cdd4b55b43ae14ea56946802f30f0661223a05cf/server/routes.go#L2598-L2623
[go-http]: https://pkg.go.dev/net/http#NewRequestWithContext
[go-readall]: https://pkg.go.dev/io#ReadAll
[end-to-end]: https://web.mit.edu/Saltzer/www/publications/endtoend/endtoend.pdf


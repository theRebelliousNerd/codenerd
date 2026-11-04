---
doc-class: deep-dive
subsystem: embedding-prompt
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 013a5b92acb46ed5febda858deb473c4b79bf44f
verification-method: static-source-inspection
supersedes: []
---

# Ollama readiness waits outlive the waiting caller

## Finding and decision

**A caller waiting for another caller's Ollama readiness operation cannot leave that wait when its own context is canceled.** `EnsureModel` takes `ensureMu` before checking readiness and holds it across model-listing and model-pull HTTP. The mutex has no context-aware wait. A second `Embed`, `EmbedBatch`, or direct `EnsureModel` call therefore inherits the current owner's remaining network wait, even when the second call's deadline has expired. [Readiness owner][ensure] [Single entry][single] [Batch entry][batch]

This defeats a concrete production fallback boundary. The JIT selector creates a short semantic-search context, calls the default vector searcher, and expects to continue without vector scores when that search times out. Its worker is joined before selection continues. When the shared Ollama engine is already checking or pulling a model for another operation, the search's own timeout cannot release the mutex wait and the logic-only fallback arrives late. The factory shares that engine with independently running store reflection work. [Selector][selector-timeout] [Worker join][selector-worker] [Default searcher][searcher] [Shared wiring][factory]

**Recommended contract:** keep one readiness owner per engine, protect model/readiness state, and let each waiting caller stop on its own context without canceling another live owner's operation. Keep network I/O outside the short state-lock critical section. Model identity reads must not reacquire a network-held lock after a canceled readiness wait.

The finding is about cancellation responsiveness and isolation between callers. It does not establish an actual hung session, incorrect vector, false success, duplicate pull, or data race. A live owner, its deadline, its HTTP timeout, or a server response may eventually release the wait. The defect is that **the waiter's own deadline does not govern that wait**. No claim of universal indefinite blocking or a measured latency improvement is made.

## Evidence boundary and novelty

Source was inspected at public main [`013a5b92acb46ed5febda858deb473c4b79bf44f`][pin], observed November 4, 2026 at 14:08 UTC and rechecked unchanged at 14:20 UTC. Repository guidance, embedding architecture, current implementation, relevant test bodies, the audit tree and targeted repository searches were inspected before selecting this topic. All CodeNERD links below pin that revision. No application, test, build, benchmark, model inference, model pull, failure injection or repository mutation was performed. The witnesses and evaluations are designed, unrun cases.

[Root guidance][root] separates model creativity from deterministic execution and calls for production-path verification. [Embedding guidance][guidance] requires synchronized mutable model state, caller-aware initialization, cancellation-aware retry waits and preservation of later retries after an optional bootstrap timeout. Those guarantees should remain intact. The proposed change belongs in the embedding host adapter; it does not ask Mangle or an LLM to decide whether a blocked mutex should release.

The [architecture's completed bootstrap repair][architecture] concerns the context that **owns** an HTTP request. It does not establish cancellation of a different caller waiting to acquire the engine mutex. The current constructor passes its context correctly, and that repair is not being reopened.

Nearby audits are distinct:

- [Embedding representation and migration][prior-representation] concerns vector/input identity, provider coverage and durable publication. Its unresolved model-alias and representation questions do not explain this wait.
- [Embedding statistics observation boundary][prior-stats] concerns provider/index mutation during a nominally observational command. This finding needs neither that command nor index recreation.
- [Recursive pooling][prior-pooling] concerns vector composition. The cancellation witness uses short input and never needs to split it.
- [NULL-stamped migration continuation][prior-migration] concerns a SQL selector on retry. This witness needs no database migration or legacy vectors.
- [MCP outbound cancellation][prior-mcp] demonstrates a different transport's synchronous pipe-write obstruction. Its repair requires owning and interrupting a write/child process. Here the owner HTTP already has a working cancellation mechanism; the missing boundary is independent waiter withdrawal and short-lived engine state access.

No matching readiness-waiter finding was identified in this bounded review. That is not a claim about every historical branch or unpublished investigation. Recursion mission accounting, source metrics and gate-detection candidates were also screened, but no distinct publication-ready recursion finding was established.

## Current contract and production reachability

### An owner already has useful cancellation

`EnsureModel` serializes access to `modelReady`, `pullAttempted` and the selected model. It lists installed models, resolves only the configured identity, or pulls exactly that model. A successful owner publishes readiness before releasing the lock. When a pull returns an error and its owner's context is canceled, `EnsureModel` resets `pullAttempted`, allowing a later request to retry; a non-cancellation pull failure has a different retained-attempt policy. [State machine][ensure]

Both `listModels` and `pullModel` use `http.NewRequestWithContext(ctx, ...)`. The listing uses the engine's request-timeout client; the pull uses the separately configured pull-timeout client. The response body is read before the owner returns. The default configuration supplies 60 seconds for an ordinary embedding HTTP request and 30 minutes for a model pull, but a shorter owner context still constrains its own request. These values are not observed wait durations and are not a 30-minute guarantee for every caller. [Listing][list] [Pull][pull] [Configuration][config-timeouts] [Defaults][config-defaults]

Go's HTTP contract associates the outgoing request's context with connection acquisition, sending and response reading. That explains why canceling the **owner** can work. It does not make unrelated mutex waiters observe their own contexts. [HTTP context contract][http-context]

### A waiter reaches cancellation too late

The first statement in `EnsureModel` acquires `ensureMu`; it accepts no cancellation branch while waiting. Go's `Mutex.Lock` contract blocks the goroutine until the mutex is available. Thus a second caller cannot inspect readiness or return a context error while the owner holds the lock. Adding a context check only before acquisition would still leave cancellation occurring after that check unhandled. [Readiness lock][ensure] [Mutex contract][mutex]

`EmbedBatch` invokes `EnsureModel` before its loop checks `ctx.Err()`. For nonempty valid input, its cancellation check is therefore downstream of the blocked acquisition. Single embedding goes through `embedLong` into `embedOnce`, which also ensures first. This witness uses a short input so recursive splitting is irrelevant. [Batch ordering][batch] [Single ordering][single] [Long-input entry][long-entry]

There is an additional completeness requirement for any repair: single `embedOnce` treats an ensure error as best-effort, then evaluates `e.Model()` in a logging call before checking `ctx.Err()` in its retry loop. Function arguments are evaluated even if the debug log is ultimately suppressed. `Model` takes the same mutex; `Name` delegates to `Model`. Merely making the ensure gate cancelable can therefore return a canceled waiter straight into another network-held lock. [Single ordering][single] [Metadata readers][metadata]

This metadata coupling is also visible outside embedding: trace reflection reads `engine.Name()` before creating its 45-second embedding context. A concurrent readiness owner can delay that read before the local timer even exists. This is the same lock-lifetime mechanism, not a separate claim that the reflection service as a whole has a strict 45-second deadline. [Reflection entry][reflection-entry]

### JIT and reflection really share the engine

The normal factory calls `NewEngineWithContext`, retains the admitted engine, gives it to the prompt atom loader, and attaches it to LocalStore and LearningStore. It later constructs `CompilerVectorSearcher` with the same `bctx.embeddingEngine`. There is no new Ollama engine per semantic-search call in this wiring. [Factory construction and attachment][factory] [Searcher attachment][factory-search]

With reflection enabled and a relevant backlog, LocalStore launches a background worker. A tick reads the shared engine, selects candidates and calls the embedding helper under a fresh 45-second background-derived context. That helper uses the engine's task-aware method when supported, otherwise batch embedding with per-item fallback. This supplies a normal independently owned operation that can overlap interactive prompt compilation. The finding does not depend on a worker being enabled or busy in every workspace. [Worker lifetime][reflection-worker] [Reflection entry][reflection-entry] [Embedding dispatch][reflection-dispatch]

The JIT configuration describes a default 10,000-millisecond vector-search timeout and a logic-score fallback. The factory passes the configured value through. In the selector's flesh worker, `getVectorScoresWithConfig` creates a child context and calls `Search` directly. The default searcher embeds the query with the supplied context. Selection then waits for both worker goroutines; it does not abandon the blocked flesh worker at the child's deadline. [Configuration contract][jit-config] [Factory timeout][factory-search] [Search call][selector-timeout] [Joined worker][selector-worker] [Query embedding][searcher]

A canceled search will normally return its context error after the readiness owner releases the lock, and the selector can then drop vector scores. The source-supported consequence is **late fallback**, not a proof that fallback never occurs.

## Conditional causal witnesses

### Two callers and one stalled owner

Use one real `OllamaEngine`, two independent caller contexts and a deterministic local fake HTTP server. Inputs are nonempty and short. The fake is only an evaluation design; none was started here.

1. Caller A enters `EnsureModel` with a live context. Its `/api/tags` response reports the configured model missing, and its `/api/pull` response is deliberately held at a server barrier. A retains `ensureMu`.
2. Caller B invokes `EmbedBatch` with valid input on the same engine. B reaches `EnsureModel` and waits for that mutex. B has not sent an embedding request.
3. Cancel B's context while A remains live and the pull barrier remains closed.
4. B cannot return through its own cancellation path: the acquisition precedes the context check. It remains blocked until A releases the mutex.
5. Release the server barrier successfully. A publishes readiness and unlocks. B acquires/releases the lock and then returns a cancellation error from its batch loop.

Let `t_cancel(B)` be B's cancellation time and `t_unlock(A)` the later time A releases `ensureMu`. Under this interleaving, B's return cannot precede `t_unlock(A)`. The excess wait is at least `t_unlock(A) - t_cancel(B)`, apart from how the test records simultaneous events. This is a control-flow deduction, not a timing measurement or scheduler upper bound.

The same obstruction can be constructed while A waits for `/api/tags`, including a response body that has not completed. It does not require an actual large model download. These cases distinguish readiness serialization from GPU inference latency.

### A normal semantic-search timeout during background repair

Use a long-lived normal Cortex with the factory-shared Ollama engine, enabled reflection, exactly one reflection candidate requiring a nonempty embedding, and a nonempty semantic query with selectable flesh atoms. No external service modification is required for a future test: the fake server supplies the conditions.

1. Boot with the configured model reported installed. The engine is ready and no prior pull attempt has occurred.
2. On a later background reflection embed, the fake reports the configured model missing. With exactly one target, `EmbedBatch` dispatches through its per-item `Embed` path. That path receives the recognized 404, invalidates readiness and enters ensure/pull. The pull remains pending under reflection's still-live context. [Single-target dispatch][batch-dispatch] [Missing-model recovery][recovery]
3. Start interactive JIT semantic selection on the same engine. Its configured search deadline expires earlier than reflection's remaining context. The search is waiting at `ensureMu` rather than inside its own HTTP operation.
4. The search cannot return at its sub-deadline. The flesh worker remains joined by selection, so the documented logic-only fallback is delayed until the readiness owner settles.
5. Once the owner settles, the canceled search returns and the existing fallback can proceed. With a live outer compilation context, it can continue without vector scores; an independently canceled outer operation retains its own cancellation obligations.

This is a production-path possibility, not a reproduced schedule. It assumes that the background request reaches the missing-model recovery branch before the interactive search; an already-ready engine with no concurrent invalidation is a negative control. If both callers share a parent and that parent cancels A as well, A's HTTP may settle promptly, so the long wait need not occur. The isolated-waiter case deliberately keeps A live.

### Metadata and partial-repair control

While A holds the readiness lock, call `Name` or `Model` and observe whether the read finishes without releasing A. Current source requires it to wait. In a proposed fix that adds only a cancellation-aware readiness gate, repeat B using single `Embed`, not only `EmbedBatch`: after cancellation, its best-effort path must not block again in the `Model()` logging argument.

This is a useful discriminator because a batch-only test could approve an incomplete fix. It does not require weakening model identity synchronization or deleting diagnostics.

## Proposed ownership and publication contract

The following is design, not implemented behavior or an approved change to automatic-pull policy.

1. **Short state access.** Read and publish model identity, readiness, pull-attempt state and any generation token under a short mutex. No network call, response-body wait or externally blocking work should run while that state mutex is held. Keep `Model` and `Name` synchronized; an unlocked field read is not a repair.
2. **One readiness owner.** Admit at most one active ensure operation per engine. Generation fencing governs publication; it does not authorize overlapping old and new pulls. Publish an in-flight record/completion signal under the state lock, then perform the existing bounded HTTP work outside it. Concurrent callers wait on that completion signal or their own `ctx.Done()`.
3. **Independent withdrawal.** A waiter canceled while another owner remains live returns its own cancellation/deadline cause without waiting for that owner's network completion. It neither cancels the owner nor changes `pullAttempted` or `modelReady` on the owner's behalf. An already-canceled call admits no new owner work.
4. **Explicit owner lifetime.** The minimal design can keep the initiating caller as the readiness owner. If that owner cancels, settle its generation, preserve the current cancellation-retry rule, and let a later live caller attempt readiness again. Moving ownership to an engine-lifetime background context is a separate decision with shutdown, consent, timeout and cleanup consequences; do not detach a download merely to make a waiter return.
5. **Ordered publication and invalidation.** Record which state generation authorized the owner. An older completion must not overwrite a newer invalidation or publish readiness for a changed identity. The current coarse mutex orders invalidation after an in-flight ensure; moving I/O outside the lock must retain equivalent ordering or define and verify generation-fenced semantics. Specify whether live waiters retry after an invalidated completion instead of consuming stale success.
6. **Cancellation before best-effort continuation.** Best-effort ensure failure may still permit embedding when the caller remains live, preserving optional-service behavior. Cancellation must return before metadata logging, retry bookkeeping, HTTP dispatch or another wait can extend the abandoned call. A completion/cancellation race needs a defined cancellation check before admitting subsequent work.
7. **Keep the existing model contract.** Resolve only the configured model/tag, preserve vector validation and engine-space stamping, retain the distinction between cancellation and non-cancellation pull failure, and do not alter ordinary request/pull timeouts as a substitute for ownership.

These obligations can be expressed with a short state lock and an in-flight record containing a completion channel, captured model/generation and settled result. This is a possible mechanism, not required API syntax. The readiness result and selected identity should come from a consistent state snapshot; callers should not combine one generation's success with a different generation's mutable name.

### Alternatives and their limits

- **Keep the current mutex and shorten pull timeouts.** This reduces some waits but leaves the waiter's deadline dependent on another operation. It also makes legitimate downloads harder and changes a separate user-visible policy.
- **Use an RWMutex around the same network critical section.** Metadata readers still wait behind the writer. It does not provide context-aware acquisition.
- **Use only a cancelable semaphore for ensure.** This can release a batch waiter, but does not fix the single-call `Model()` reentry or metadata readers if they still share a network-held state lock.
- **Use `singleflight.DoChan`.** It supplies duplicate suppression and a result channel suitable for a caller-side `select`, but does not supply engine-state publication, invalidation fencing or owner cancellation policy. `Do` alone retains an uninterruptible duplicate wait. `Forget` is not a cancellation primitive: allowing a replacement owner while the first still runs can duplicate pulls and race readiness publication. [Singleflight contract][singleflight]
- **Launch a detached goroutine per blocked call and return at the deadline.** This changes visible latency while retaining abandoned goroutines and possibly later work. It is not an ownership fix. A bounded shared owner and explicit completion are preferable to accumulating hidden waiters.
- **Give every consumer a separate engine.** This reduces local contention but duplicates readiness requests/pulls and mutable identity state, consumes more resources, and leaves the engine's own concurrent API behavior unresolved.

## Costs and operational failure modes

The current design has a real benefit: simple state serialization, one active readiness operation and no additional per-flight coordination structure. A warm ready check holds the mutex briefly. This report does not imply every embed pays model-listing or pulling costs.

When an owner is slow, `N` waiting goroutines retain their stacks, contexts and reachable request state until the owner releases the mutex. If they all cancel with the same remaining owner duration `L`, aggregate post-cancellation blocked goroutine time is at least approximately `N × L`; actual heap retention depends on captured data and should be measured, not estimated as a universal byte count. Blocked mutex waiters do not spin continuously, so this is not a claim of `N` busy CPU loops. JIT's retained worker also delays downstream selection and can hold compilation-related state alive. [Joined selection][selector-worker]

A split-state design adds an in-flight record, completion notification, result storage, generation comparison and a few short lock acquisitions per readiness episode. Waiters need a select and release any waiter bookkeeping on cancellation. A one-channel broadcast design can keep shared flight state constant-sized, though each live caller still consumes a goroutine and context. Completion wakes the remaining callers, so burst scheduling and subsequent embedding request fan-out require measurement. None of these costs automatically reduces provider calls, embedding tokens or model bytes downloaded.

Owner cancellation can force a later caller to retry work that partially completed on the server. That tradeoff already exists when bootstrap cancellation permits a later pull. A repair must count attempted requests and server-side side effects separately from returned success; it cannot assume canceling local HTTP rolls back a model download. Conversely, canceling one waiter must not restart a healthy owner's download. Any new retry/backoff policy is outside this minimal change.

Keep telemetry bounded and independent of the blocked operation. Useful measurements include readiness owner duration, waiter duration, waiter cancellations, in-flight count, tags/pull request count, cancellation-to-return latency, stale-generation completions, metadata-read latency and outstanding goroutines after all owners settle. Distinguish an owner error from a waiter cancellation. Avoid logging prompt text, credentials or unbounded per-waiter records, and avoid collecting the telemetry through a `Name()` call that can itself wait on network ownership.

Important repair failure modes are duplicate owners, canceled waiters resetting owner state, failure to close the completion signal on every exit, stale success after invalidation, retaining waiter entries after cancellation, races in model-name access, suppressing permitted retries, unbounded owner detachment, and falsely reporting cancellation before owned HTTP/goroutine cleanup has settled. A short return time alone is insufficient acceptance evidence.

## Primary references and their limits

The Go team's [Context article][context-article] explains request-scoped cancellation propagation and prompt release of request resources. The [mutex API][mutex] establishes the blocking primitive used here; the [HTTP API][http-context] establishes which operation the supplied context controls. Together they support the distinction between a cancelable HTTP owner and a noncancelable waiter. They do not prescribe this engine's retry policy.

The Go [singleflight API][singleflight] is a concrete reference for separating duplicate-work coordination from result waiting. It is an implementation option, not proof that importing the package would satisfy the proposed lifecycle contract.

Dean and Barroso's [The Tail at Scale][tail] discusses how temporary latency episodes affect interactive systems as shared services grow. The relevant design lesson is to measure the waiting caller's tail and end-to-end responsiveness, rather than only average provider duration. Its warehouse-scale results are not evidence for a particular codeNERD speedup or for a 30-minute production incident.

## Evaluation plan with no results claimed

All cases below are proposed and unrun. Use deterministic barriers and an independently observable HTTP handler, not a long arbitrary sleep or a mock `Search` that already returns perfectly on cancellation. Keep the same configured model identity, input, timeout values and semantic-selection policy between baseline and candidate. Record the actual revision and configuration of each future run.

| Case | Required observation |
|---|---|
| Isolated waiter during pull | Hold A's pull response, cancel B after confirming B entered readiness waiting, and require B's cancellation return before releasing A. A remains live; no extra pull or embedding request is attributed to B. |
| Isolated waiter during listing | Hold `/api/tags` before headers and separately during body reading. B withdraws independently in both cases. |
| Single and batch parity | Run both `Embed` and `EmbedBatch`, plus direct `EnsureModel`. Single-call logging/metadata access must not recreate the wait. |
| Metadata responsiveness | During held readiness I/O, `Name` and `Model` return a synchronized permitted snapshot without waiting for the server barrier. Verify exact identity and race safety, not only latency. |
| Pre-canceled caller | A pre-canceled B returns without joining or initiating network work, whether the engine is ready, idle or already ensuring. |
| Cancellation versus completion race | Repeated controlled interleavings preserve the selected precedence and do not dispatch an embedding request for an abandoned caller. Every admitted flight settles once. |
| Live coalesced callers | Many live callers cause one readiness operation per generation. All receive a valid settled state; release does not cause duplicate pulls or retained waiter records. |
| Owner cancellation and later retry | Cancel A's HTTP while B remains live. Preserve documented owner-cancellation recovery and exact model selection; B or a later live caller can retry under the chosen policy. |
| Non-cancellation failure control | Preserve the existing pull-attempt policy for a genuine non-cancellation failure unless a separate change is approved. Do not accidentally turn every waiter into a fresh download attempt. |
| Invalidation during flight | A newer invalidation/state generation cannot be overwritten by an older ready completion. Verify successful and failed owner exits plus live-waiter behavior. |
| Production JIT fallback | Use the normal factory/shared engine and actual default searcher, and an uncached compile verified to enter that searcher. Start with fresh, empty relevance carry when asserting logic-only output. Make a background readiness operation outlive the semantic-search sub-deadline. With the outer compile live, fallback completes without first releasing that unrelated owner; record selected logic-only behavior and ownership cleanup. |
| Whole-operation stop | Cancel the normal operation while a real default-searcher call is queued. Verify return and cleanup; a cooperative test searcher alone does not qualify Ollama contention. |
| Warm-path regression and cost | Compare already-ready throughput, tags/pull counts, CPU, allocations, metadata latency, cancellation latency distributions, peak retained memory and goroutines after settlement under identical concurrency. Report owner and waiter latency separately. |

For the held-owner cases, use a sufficiently generous scheduling allowance established by the test environment, but keep the decisive ordering assertion: **B must finish while A remains blocked at the barrier**. This avoids calling an owner that happened to finish quickly a cancellation success. Cleanup must release/cancel A and join every test goroutine even when the assertion fails. A race-detector run is useful for state safety but is not, by itself, a proof of cancellation responsiveness or absence of leaked ownership.

Existing authored tests remain valuable controls. `TestColdBootEmbedding_ParentCancellationStopsStartupHTTP` covers cancellation of the constructor's own HTTP; the optional-cap test covers later request retry. `TestEnsureModel_AutoPullsWhenMissing` checks sequential reuse, and `TestEnsureModel_BootstrapDeadlineDoesNotPoisonLaterPull` checks sequential retry after owner cancellation. `TestOllamaEngineModelAccessIsConcurrentSafe` exercises locked model reads/writes, not a slow network critical section. The pull-timeout test exercises the pull client's own timeout. None of these inspected cases establishes the isolated-waiter property. [Bootstrap tests][test-boot] [Ensure tests][test-ensure] [Model access test][test-model] [Pull timeout test][test-pull]

The JIT timeout test verifies configuration reaches the selector. Its shutdown test uses a searcher that directly waits on and returns from `ctx.Done()`, which is a useful cooperative-consumer control. Neither substitutes for the actual Ollama engine queued behind another readiness owner. [Timeout wiring test][test-jit-timeout] [Shutdown test double][test-shutdown]

Acceptance requires both parts: independent canceled-waiter return and preservation of single-owner state correctness, exact identity, later retry, ordinary optional-service fallback and settled resource ownership. No numerical performance threshold, live-provider qualification or completed repair is claimed by this source audit.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/013a5b92acb46ed5febda858deb473c4b79bf44f
[root]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/agents.md
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/agents.md#L1-L18
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/Docs/architecture/embedding/INTERNALS.md#L1-L7
[ensure]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L459-L505
[single]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L95-L130
[batch]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L334-L360
[batch-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L358-L391
[metadata]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L402-L423
[list]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L520-L549
[pull]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L552-L610
[recovery]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama.go#L186-L203
[long-entry]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/long_input.go#L52-L83
[config-timeouts]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/config/memory.go#L57-L73
[config-defaults]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/config/memory.go#L199-L209
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/system/factory.go#L1731-L1775
[factory-search]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/system/factory.go#L1810-L1828
[reflection-worker]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/store/reflection_worker.go#L40-L101
[reflection-entry]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/store/reflection_worker.go#L104-L179
[reflection-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/store/reflection_worker.go#L261-L288
[jit-config]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/config/jit.go#L82-L85
[selector-timeout]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/prompt/selector.go#L959-L989
[selector-worker]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/prompt/selector.go#L765-L854
[searcher]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/prompt/vector_searcher.go#L40-L85
[test-boot]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/coldboot_context_test.go#L26-L79
[test-ensure]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama_ensure_test.go#L99-L192
[test-model]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama_coverage_test.go#L331-L360
[test-pull]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/embedding/ollama_pull_timeout_test.go#L12-L66
[test-jit-timeout]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/prompt/vector_timeout_test.go#L8-L36
[test-shutdown]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/internal/prompt/compiler_shutdown_test.go#L13-L64
[prior-representation]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/Docs/audits/embedding-representation-and-migration.md
[prior-stats]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/Docs/audits/embedding-stats-observation-boundary.md
[prior-pooling]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/Docs/audits/embedding-recursive-pooling-weight-contract.md
[prior-migration]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/Docs/audits/vector-migration-null-stamp-continuation.md
[prior-mcp]: https://github.com/theRebelliousNerd/codenerd/blob/013a5b92acb46ed5febda858deb473c4b79bf44f/Docs/audits/mcp-stdio-outbound-cancellation.md
[mutex]: https://pkg.go.dev/sync#Mutex.Lock
[http-context]: https://pkg.go.dev/net/http#NewRequestWithContext
[context-article]: https://go.dev/blog/context
[singleflight]: https://pkg.go.dev/golang.org/x/sync/singleflight#Group.DoChan
[tail]: https://research.google/pubs/the-tail-at-scale/

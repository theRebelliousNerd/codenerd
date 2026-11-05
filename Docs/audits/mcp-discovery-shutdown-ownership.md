---
title: "MCP discovery: join workers before closing their store"
doc_class: audit
subsystem: mcp
status: proposed
last_verified: 2026-11-05
source_revision: b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6
method: static source inspection
supersedes: []
---

# MCP discovery: join workers before closing their store

## Finding and proposed decision

Normal Cortex shutdown can close the MCP catalog database while its initial discovery worker is still analyzing a tool. The factory owns a cancellable `ConnectAll` wrapper, but `Connect` starts the actual discovery with `context.Background()`. Cancelling the wrapper lets `WaitForDiscovery` return without joining that worker. Bridge shutdown disconnects transports and closes SQLite without waiting for discovery. The worker's final connection-identity check happens only after all schemas have been processed, including model analysis, optional embedding, persistence attempts, fact emission, and callbacks. [Factory ownership](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/system/factory_mcp.go#L24-L54); [detached launch](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L388-L403); [cancellable waiter](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L180-L196); [bridge close](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/integration.go#L273-L280).

The concrete gap is **dependency teardown without producer termination**, not merely a missing cancellation check. A completed tool-list request and a slow, otherwise cooperative analyzer are enough; the witness needs neither a blocked transport writer nor an expired cleanup timer. Returning from `ConnectAll`, observing its `mcpDone` channel, and closing SQLite are all individually possible while the discovery wait group is still nonzero.

**Proposed decision:** give discovery an explicit bridge/manager lifetime. Stop admitting discovery, cancel its active work, join the actual workers, and only then close their dependencies. Keep startup readiness separate from this termination proof. Carry the owner cancellation through analysis and embedding, and prevent retired work from starting later schemas or publishing metadata. Preserve valid offline catalogs and the existing exact authorization path.

This is a source-supported lifecycle deficiency and an unrun causal witness, not a reproduced production incident. The audit does not establish how often it occurs, that every provider continues after full Cortex shutdown, that SQLite accepts writes after closing, or that stale descriptions authorize remote execution. No tests, builds, application runs, inference, or training were performed.

## Intent, status, and novelty

The repository's north star gives Mangle executive responsibility and Go the drivers and typed effect boundaries. Resource ownership belongs in those drivers; an LLM must not decide whether its own shutdown obligation is complete. Current system guidance treats boot as a resource transaction and explicitly distinguishes joining cooperative work from an elapsed cleanup timer. [Root requirements](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/agents.md#L3-L28); [system ownership guidance](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/system/agents.md#L19-L23).

The MCP architecture calls for durable catalog reuse, JIT rather than full-catalog prompt injection, and recovery of background failures. Those are intended requirements, not evidence that cancellation and joining are implemented. Preserve them while repairing ownership. Historical architecture prose is not substituted for the current source trace. [MCP principles](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/Docs/architecture/mcp/04-ARCHITECTURAL-PRINCIPLES.md); [source-over-stale-docs guidance](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/agents.md#L60-L65).

| Status | Evidence and limit |
| --- | --- |
| Implemented | Configured background connection, discovery accounting with a wait group, a cancellable readiness wait, connection removal on disconnect, database close, and a final connection-identity check before replacing the connection's tool slice. These are useful mechanisms, not a shutdown join. |
| Partially implemented | Cortex cancels and awaits the factory's `ConnectAll` wrapper. The wrapper's cancellation return does not establish that its detached discovery producers have stopped using the catalog store, prompt compiler, embedder, or fact emitter. |
| Proposed here | Owner-scoped discovery admission/cancellation/draining, shutdown-aware fallback, and publication boundaries that cannot outlive the relevant owner. No new implementation is claimed. |
| Unverified | Runtime incidence, provider-specific late-call behavior, resource and billing impact, repair performance, and the complete acceptance matrix below. Static inspection does not make those measurements. |

### What earlier work already establishes

This audit does not claim that analyzer cancellation masking is newly discovered. [March analyzer QA](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/.quality_assurance/mcp/2026-03-27_04-22-EST_mcp_analyzer_boundary_analysis.md#L374-L405) explicitly calls out fallback after cancellation. [May analyzer QA](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/.quality_assurance/mcp/2026-05-11_04-21-40-EST_mcp_analyzer_boundary_analysis.md#L137-L170) revisits it. The current fallback can also call the embedding engine, so the older description of fallback as purely local is not a current implementation guarantee.

The new contribution is the end-to-end ownership mismatch: the actual factory and Cortex close path can retire storage while the producer is live, even if the analyzer never receives a cancellation error. A later cache lookup error can then fall through to another schema's analysis. Fixing only the older analyzer error branch would leave this witness intact.

Other nearby audits cover different boundaries:

- [MCP stdio outbound cancellation](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/Docs/audits/mcp-stdio-outbound-cancellation.md) concerns a blocked transport writer retaining the lock needed for teardown. Here listing is finished and transport teardown can succeed normally.
- [MCP versioned wire fidelity](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/Docs/audits/mcp-versioned-wire-fidelity.md) already requires catalog generation fences. This audit is not a second general proposal for reconnect fencing; it identifies the missing owner join and dependency-close ordering that a final publication fence alone cannot repair.
- [MCP legacy schema-hash backfill](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/Docs/audits/mcp-legacy-schema-hash-backfill.md) concerns cache provenance, not shutdown. [Embedding readiness waiter cancellation](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/Docs/audits/embedding-readiness-waiter-cancellation.md) concerns initialization admission, not MCP's detached producer.
- [Recurse TUI shutdown ownership](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/Docs/audits/recurse-tui-shutdown-ownership.md) shares the general owner-before-dependencies principle. Its producer, completion signal, and affected resources are different. This audit does not reopen recursive pass retention or the completed-pass retirement audit.

The overlap check covered current audit names, targeted current-revision content searches, relevant MCP QA, and source/test inspection. It is not a claim to have exhaustively searched every historical revision.

## The production ownership chain

### 1. Boot owns a wrapper, while discovery uses a separate lifetime

`initMCPBridge` creates the analyzer with the Cortex's LLM adapter, role-prompt compiler, and embedding engine. It creates `mcpCtx` from the boot context, stores `mcpCancel` and `mcpDone`, and launches a goroutine that closes `mcpDone` when bridge `ConnectAll(mcpCtx)` returns. This is real boot wiring, not a direct test-only manager invocation. [Factory path](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/system/factory_mcp.go#L12-L54); [bridge construction](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/integration.go#L83-L111).

After connection, manager `Connect` registers a discovery worker with `m.discovering.Add(1)` and defers `Done`, but calls `DiscoverTools(context.Background(), serverID)`. The comment deliberately protects discovery from a short connection context. That is a reasonable distinction between connection-call lifetime and useful background work, but `Background` supplies no replacement owner. [Launch and rationale](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L388-L403).

The production adapter forwards that supplied context unchanged. The broker passes it through to the underlying client; a supported concrete example, `OpenAIClient`, adds a per-call timeout when no deadline exists. A finite provider timeout limits a call but does not connect it to `mcpCancel`. The witness below starts after admission and dispatch, so it does not assume that arbitrary model requests bypass the client's admission checks. [Adapter](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/system/factory_adapters.go#L255-L266); [broker forwarding](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/broker/broker.go#L400-L416); [provider deadline and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/perception/client_openai.go#L316-L347).

`WaitForDiscovery` creates a helper goroutine that waits on the discovery group, then selects either that completion or the caller's context. When the latter is cancelled, the caller returns while both the discovery producer and its waiting helper can remain live. `ConnectAll` explicitly treats cancellation as a reason to mark the bridge ready and return. Readiness unblocks callers; it is not proof that the worker stopped. [Wait implementation](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L180-L196); [cancelled readiness path](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/integration.go#L192-L240).

### 2. Close tears down storage before any actual discovery join

Cortex shutdown calls `mcpCancel`, runs `MCPBridge.Close`, and then waits for `mcpDone`. Bridge close calls `DisconnectAll` and the store's `Close`, which is `sql.DB.Close`. No discovery cancellation or join appears in that close method. The factory done channel certifies only that `ConnectAll` returned. [Cortex sequence](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/system/cortex_close.go#L119-L138); [bridge close](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/integration.go#L273-L280); [store close](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/store.go#L194-L197).

Disconnect removes the connection and reviewed effects before closing the transport. This is important protection, but it does not interrupt analysis of schemas already returned by `ListTools`. The transport need not be involved again for that analysis to continue. [Disconnect](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L409-L438).

Ordinary production callers do reach Cortex close. For example, the recurse CLI obtains a Cortex through `GetOrBootCortex` and defers `cortex.Close()`. The issue does not require changing recurse semantics, starting a mission, or adding a new execution route. [Example normal caller](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/cmd/nerd/cmd_campaign_recurse.go#L169-L186).

### 3. Work and publication precede the final identity check

Discovery first obtains schemas, then passes `beginRemoteToolRefresh`, then processes every returned schema. For each schema it performs cache lookup and, where needed, semantic analysis. It logs a per-tool failure and continues the loop. Only after that loop does it test whether the original connection is still current. A disconnect therefore prevents replacing the retired connection's tool slice, but not the earlier stages. [Discovery ordering](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L458-L535).

The per-tool path does not distinguish a failed cache query because the database has closed from an ordinary cache miss. It proceeds to analysis. A later `SaveTool` failure is logged, after which it still calls `EmitTool`. The emitter does not inspect the discovery owner's state; if the kernel remains available, it can replace descriptive tool facts. The outer loop can then invoke a discovery callback and move to the next schema. [Cache and analysis](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L565-L630); [save and publication](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/client.go#L640-L664); [fact emission](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/facts.go#L137-L153).

### 4. Fallback adds a second cancellation break

Successful model analysis passes its input context to optional embedding. Model failure, including cancellation, instead invokes `analyzeWithoutLLM`, whose embedding context is a fresh `Background`. The no-model branch uses the same fallback. Consequently replacing only the launch context does not establish the complete lifecycle contract: a cancelled model call can become a new embedding attempt under a context that cannot observe the owner stop. Role-prompt compilation failure follows a different path and returns an error; these paths must not be conflated. [Analyzer paths](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/analyzer.go#L47-L128).

## A concrete causal witness

This is a proposed deterministic schedule derived from the source, not an executed test or production trace.

**Preconditions:** an enabled auto-connect, auto-discover server; a valid successful tool list containing two uncached schemas; a healthy catalog store and fact emitter; a configured analyzer with a valid role-prompt compiler. The first analysis request has started but has not returned. Optional embedding is needed only to demonstrate the embedding branch, not the missing join. Choose a normal transport whose tool-list operation has fully returned and whose disconnect succeeds without delay.

1. The factory launches `ConnectAll(mcpCtx)`. Connection launches discovery with `Background` and increments its wait group. The wrapper begins waiting for discovery.
2. `ListTools` completes. The refresh's initial connection check succeeds. The first schema's role prompt compiles and its model call waits for a response. There is no database query in progress at this point.
3. The owner calls `Cortex.Close`. Cancelling `mcpCtx` makes `WaitForDiscovery` return. Bridge `ConnectAll` marks readiness and returns, so the factory closes `mcpDone`. None of those transitions cancels the discovery context.
4. Bridge close removes the connection, closes the now-idle transport, and closes SQLite. With no active SQL query, database close need not wait for the model request. The producer is still counted as active. The fundamental failing state is now established: **catalog store closed, discovery worker still live, no owner stop delivered through its context**.
5. In one legal interleaving, the first model response arrives after bridge close but before later Cortex dependency shutdown. Analysis can attempt embedding, attempt a database save that fails because the database is closed, and attempt fact publication. If a callback is registered, successful fact publication permits it to run; the inspected production bridge does not install one, so callback execution is an additional API-path consequence rather than a required part of this production witness. The second schema's closed-database lookup is not a usable cache hit, so it falls through to analysis while its dependencies remain usable.
6. After all schema processing, the connection-identity check rejects publication to the removed connection's tool slice. That rejection cannot undo model/embedding work, attempted writes, prior fact replacements, or callbacks.

For an eventual regression fixture, use explicit barriers at the analyzer and dependency-close boundaries rather than sleep-based timing. Releasing the analyzer between MCP bridge close and later dependency close is sufficient to demonstrate step 5. Step 4 alone falsifies the proposed ownership contract even when subsequent dependencies reject all new calls.

### Bound the claim

- `sql.DB.Close` prevents new queries; this is not a claim of successful post-close persistence or database corruption. The failure is that the producer is still entitled by its own control flow to try using a retired dependency.
- Later Cortex shutdown closes the prompt compiler, semantic searcher/embedding engine, perception layer, and drains/detaches its broker sink. These can reject, cancel, delay, or account for subsequent work. The source does **not** prove that a second model request is always accepted after the entire `Cortex.Close` has returned. [Later teardown](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/system/cortex_close.go#L241-L267); [broker drain](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/system/cortex_close.go#L288-L309).
- Disconnect removes live connection authority and schema-bound reviews. Descriptive facts are not a new permission to execute a remote tool. The audit establishes neither a review bypass nor an unauthorized invocation.
- Warm cache reuse, no configured servers, disabled auto-discovery, fast completion before shutdown, and missing analyzer dependencies can avoid or shorten the expensive path. They do not establish a general join guarantee.
- A worker stuck forever is not required. A normal request that returns shortly after storage teardown demonstrates the ordering defect. Conversely, a noncooperative provider requires an explicit incomplete-shutdown policy rather than an assumption that cancellation killed it.

## Completion contracts

The following are proposed requirements, not names of shipped APIs or predicates.

### A. Distinguish connection-call, discovery-owner, and waiter lifetimes

A short `Connect(ctx)` deadline must not arbitrarily cancel useful discovery that the bridge has adopted. The replacement for `Background` should be an explicit bridge/manager owner, not merely whatever context happened to connect the transport. Every worker must be registered under that owner before it starts and retain a bounded, identifiable ownership record until exit.

Closing admission and registering workers must be synchronized so a new `Add` cannot race a shutdown wait that has already concluded. Define repeated close, concurrent connect, explicit rediscovery, and owner replacement behavior. A closed owner must not accept discovery that can later reach retired dependencies. Do not solve only the initial automatic goroutine while leaving another discovery entry point with the same ownership escape.

`WaitForDiscovery(ctx)` can remain a cancellable readiness API. Its caller abandoning the wait must not masquerade as worker termination. Shutdown needs an actual worker-completion proof, not a return from that API or closure of the wrapper's done channel.

### B. Stop useful-work admission, propagate cancellation, then drain

When the owner stops, prevent admission to subsequent schema analysis, model requests, embedding, and publication. Signal active cooperative operations and join them before closing their stores, prompt compiler, embedding engine, emitter, and accounting dependencies. Keep a clear distinction between an already-started remote request settling and a newly admitted request after stop.

Carry the same owner through normal and fallback paths. Cancellation and deadline expiry must not be interpreted as ordinary provider failure that authorizes a new embedding request. A local heuristic classification may remain useful in ordinary degraded operation, but its completion is not permission to persist or publish after owner retirement. Preserve the requirement for the registered JIT role prompt when model analysis is used; do not introduce a hardcoded substitute prompt to make shutdown tests pass.

### C. Validate at the side-effect boundary, without long global locks

The final connection cache check is useful and should remain. It must be complemented by owner/current-generation checks coordinated with per-tool persistence, metadata publication, and callbacks, so a retired producer cannot republish after shutdown or replace newer observations. Reuse the existing catalog/authority ownership rather than creating a competing cache truth.

A check immediately before a blocking operation is not by itself atomic with a concurrent close. Define the lifetime lease or synchronized commit boundary that keeps dependencies valid through the operation. Do not hold the global manager or authority lock across model, network, or embedding calls merely to serialize shutdown. That would exchange the present ownership hole for a lock-dependent teardown stall.

Keep useful durable descriptions available as offline evidence. Do not delete the whole catalog, widen host reviews, turn schema annotations into trusted authority, or relax exact-call permission to simplify cleanup. Mangle consumes truthful lifecycle/availability observations; it does not certify that an unjoined Go goroutine has exited.

### D. Report unresolved shutdown honestly

Cancellation is cooperative. If a dependency ignores it and the shutdown budget expires, report which discovery work is still unresolved and what resources remain owned. A timeout must not produce a clean-drained status. Retain or isolate dependencies still needed by the worker according to an explicit policy, and make any eventual cleanup observable. Removing all timeouts and waiting forever is not an adequate availability contract either.

Preserve late request settlement and accounting. A cancelled client context does not prove that a provider charged nothing or that a remote effect never happened. Do not replay calls to make shutdown appear complete. This audit's discovery path does not need remote tool invocation, and its repair must not alter invocation authority.

## Why the lifecycle distinction is well-founded

The primary Go API documentation makes two separate guarantees: a [`CancelFunc`](https://pkg.go.dev/context#CancelFunc) requests abandonment but does not wait for work to stop, while [`Background`](https://pkg.go.dev/context#Background) is never cancelled. Those are precisely the two boundaries the current wrapper composes incorrectly as a termination signal. They do not imply that every dependency is uncancellable by some other mechanism.

The [`database/sql` close contract](https://pkg.go.dev/database/sql#DB.Close) prevents new database queries and waits for already-started database work. It is not a join for a model request that may issue its first write later. Treating a successful database close as proof that its upstream producer is finished reverses that dependency relationship.

Sameer Ajmani's primary Go engineering article, [“Go Concurrency Patterns: Pipelines and cancellation” (2014)](https://go.dev/blog/pipelines), explains why downstream termination must be communicated upstream and why abandoned goroutines retain resources rather than being garbage-collected. Nathaniel J. Smith's original design essay, [“Notes on structured concurrency, or: Go statement considered harmful” (2018)](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/), separates cancelling children from waiting for their cleanup before scope exit. The latter is design literature, not empirical evidence about CodeNERD. Neither source establishes this repository's runtime incidence; the pinned code supplies the causal trace.

## Costs and tradeoffs

Let `N` be acquired schemas and `U` the schemas requiring fresh analysis under a healthy cache. Discovery processes schemas sequentially within one worker, while different servers' workers can overlap. Normal work includes tool-list pages/bytes, prompt compilation, model requests for analyzed tools when configured, optional embedding, classification, SQLite access, fact replacement, and callbacks. Cache reuse reduces `U`; a closed-database lookup is no longer a valid cache hit and can instead enter fresh analysis. Do not equate every schema with a paid model call, or assume every embedding engine is remote and billable.

Maintain separate ledgers for:

- Model requests and actual input, cached-input, output, and retry usage; embedding requests/tokens and provider charges where observable. Unknown remote usage is unknown, not zero.
- Host CPU for schema hashing, prompt compilation, classification, serialization, and fact processing; database operations and retained catalog/vector storage.
- Network requests and bytes for listing and analyzer/embedding providers, including retries and cleanup. Do not count one settled call twice because it crossed shutdown.
- Peak live memory from schema slices, in-progress prompts/responses/embeddings, active workers, waiting goroutines, and any retained dependency leases.
- Startup readiness latency, owner-stop-to-cancellation latency, worker-drain latency, dependency-release latency, and end-to-end shutdown time. Do not add overlapping server durations as if they were serialized wall time.

The repair adds owner/admission bookkeeping, per-stage lifetime checks, and a drain path proportional to active workers. For independent cooperative workers, drain latency is governed by the slowest remaining worker and shared contention, not simply the sum of all worker durations. A bounded failure path may retain resources longer rather than unsafely close them. Cancellation can abandon useful partially completed metadata, increasing re-analysis on the next boot; cancellation that arrives after remote acceptance can still incur cost. Preserve valid completed cache rows so stopping a session does not force full rediscovery analysis on every restart.

Expected benefits are fewer useless late stages, truthful shutdown status, and removal of dependency-use races. No token savings, leak rate, billing reduction, or latency improvement is measured here. Lifecycle correctness is required even for a zero-cost local embedder.

## Proposed evaluation and acceptance

No evaluation below has been run. Start with deterministic transports, analyzers, embedders, and dependency probes; avoid live inference while establishing ordering. Keep the real configured factory, bridge, manager, and Cortex shutdown path in the integration fixture. Direct analyzer tests alone cannot establish owner lifetime.

The current [analyzer cancellation test](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/analyzer_test.go#L215-L242) deliberately expects fallback and uses no embedder. It does not test whether fallback dispatches new work after stop. The inspected [bridge close and ConnectAll coverage tests](https://github.com/theRebelliousNerd/codenerd/blob/b0dd7d3dbcfc2e7a3fb718ff044c01aad3cef1e6/internal/mcp/integration_coverage_test.go#L159-L185) use empty managers and do not put an analyzer across database close. These observations define missing evidence in the inspected tests, not a claim that the entire repository lacks lifecycle tests.

Compare three matched variants: the pinned implementation; a launch-context-only ablation; and the complete owner/cancellation/drain contract. Use identical catalogs, schema/cache identities, prompt behavior, provider stubs, fault schedules, permission rules, and store/kernel behavior. The ablation tests whether fixing one `Background` leaves fallback or the join unresolved; it is not an implementation recommendation.

| Witness or control | Required observation for completion |
| --- | --- |
| Two-schema normal list, first model call blocked | Stop reaches the worker. The actual worker exits before database close. No second schema starts paid/model/embedding work after owner stop. A wrapper's done channel alone is insufficient. |
| Cancellation during model analysis with embedder configured | A cancellation error does not start a fresh background embedding request. Non-cancellation provider failure can still follow the explicitly supported degraded path while the owner remains live. |
| No model, heuristic analysis with slow embedder | The same owner stop reaches embedding; the no-model configuration is not treated as purely local or exempt from draining. |
| Cancellation during listing, role compilation, embedding, persistence, and fact emission | Each boundary reports a truthful terminal or unresolved state. No dependency closes while an admitted worker still holds its use right. No retired callback or metadata publication occurs. |
| Store failure versus owner retirement | Ordinary database errors retain their documented degraded behavior; owner-stop/closed-dependency errors do not silently become permission for more fresh analysis. |
| Multiple servers; connect/refresh racing close | Admission is closed atomically with shutdown registration. Every adopted worker is counted and drained or explicitly unresolved; no worker appears after a successful drain. |
| Noncooperative worker | The shutdown budget produces a named unresolved outcome and the specified retained/isolated resource policy, not a false clean close. Release its barrier later and verify eventual cleanup without orphaned writes. |
| Warm cache, no servers, disabled discovery, normal complete discovery, repeated close | Preserve valid reuse, prompt selection behavior, no-work startup, and idempotent close. The fix must not cancel normal discovery just because a short connection-call context ends. |
| Disconnect and subsequent invocation | Offline descriptions remain descriptive. Existing current-connection, schema-bound review, and exact permission requirements still deny retired invocations. Use ordinary production authorization fixtures; do not introduce permissive bypasses. |
| Restart after interrupted discovery | Completed valid rows are reused; incomplete work is represented honestly and reprocessed only as needed. No new “analysis complete” marker is fabricated merely because shutdown ran. |

Record exact dispatch counts by stage, context cancellation delivery and acknowledgment, producer completion, database-close ordering, post-stop publication/callback attempts, late settlement, active worker counts, unresolved resources, and close result. Measure p50/p95 latency, CPU, memory, storage, and provider usage only after the deterministic ordering contract is established. Run race-focused and repeated interleaving checks when implementation work is authorized; a clean result from one timing schedule is not a general concurrency proof.

A later capability evaluation should use the same normal CLI entry paths, world-model/CodeDOM configuration, available tool catalog, and safety rules as the baseline. Measure useful task completion and warm-restart behavior as well as shutdown latency. A smaller visible catalog or premature cancellation that avoids discovery altogether is not an efficiency win. Actual model/provider experiments require separate authorization and should be reported separately from deterministic tests.

**Exit criterion:** a successful MCP/Cortex shutdown joins every discovery producer before closing its dependencies. A producer that cannot be joined is explicitly reported under a defined incomplete-shutdown resource policy, never counted as successful shutdown. Readiness, transport disconnect, and wrapper completion are no longer substituted for that proof.

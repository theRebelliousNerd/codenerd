# Request owned attribution for persisted interaction traces

**Repository:** `theRebelliousNerd/codenerd`  
**Inspected revision:** `29a448ad1c4ed74577d56aeb3c03badfc4db51e1`  
**Evidence status:** static source review on 2026-11-02. No application, test, reproduction, benchmark, model call, or race detector was run. This is a supported data-flow finding and a proposed repair contract, not a measured production incident or an implemented fix.

## Finding and decision

Connect interaction-trace attribution to the immutable request identity already carried through the executor's context. On the inspected production path, the executor writes shard name, role and session into `context.Context`, but `TracingLLMClient` snapshots a separate set of client-instance fields. Factory construction leaves those fields empty. The trace can therefore be successfully persisted with an empty `shard_type`, even when the provider request carries a known coder or reviewer identity. Chat then asks for named roles using an exact SQL filter and does not hydrate that row.

The consequence is **missing attribution and missing named-role trace metadata**, not proof that all memory is unavailable, that usage billing is misattributed, or that no traces are written. Broad recent-trace retrieval can still see the record. The bounded repair belongs at request metadata capture, not in SQL by pretending unknown rows belong to whichever role asked for them. [Factory construction][factory], [request scope][scope], [trace snapshot][snapshot], [exact retrieval][exact]

This fits the existing architecture: Go transports typed observations and performs persistence; Mangle can reason over the resulting facts. Neither missing telemetry nor a repaired trace should authorize work, change permissions, or establish verified task success.

## The production path

### Identity exists on the request

`ProcessWithIntent` supplies a session ID when the caller has not already supplied one. It installs per-turn usage identity, applies the preset persona before setup, and applies the observed or preset persona again before execution. `withShardUsage` establishes a named-task boundary, preserves that name during nested turn classification, and calls `usage.WithShardContext`. The usage accessor stores name, type and session as typed context values. It does not mutate a tracing client. [Executor setup][setup], [request scope][scope], [usage accessors][usage-context]

A delegated task explicitly passes its worker name into `withShardUsage` before calling `ProcessWithIntent`. Thus this is not merely an optional public helper with no production caller. The boundary distinguishes the named worker from its role; model/provider selection is a separate concern. [Delegated task boundary][task]

For a native tool-results turn, the inspected path is:

```text
TaskExecutor or Executor.ProcessWithIntent
  -> withShardUsage on request context
  -> Executor.completeWithWorkingContext
  -> sessionLLMAdapter.CompleteWithToolResults
  -> ScheduledLLMCall.CompleteWithToolResults
  -> selected TracingLLMClient.CompleteWithToolResults
  -> provider
```

The working-context method passes `ctx` onward. The session adapter adds a meter only when necessary, preserving its parent context. The scheduler selects the target client using that context and forwards the same context after API-slot acquisition. None of these steps converts its request-scoped shard identity into the tracing instance's fields. [Working request][working], [adapter context][adapter-context], [adapter forwarding][adapter-call], [scheduled call][scheduled-call]

### Factory wiring does not initialize trace ownership

The system factory opens `.nerd/knowledge.db` and wraps the base provider with a fresh `TracingLLMClient` when that succeeds. The main and default shard slots share that underlying traced client. A configured worker gets another fresh wrapper; a configured planner does likewise. The task executor is built with a session adapter around the shard client when available. [Factory construction][factory], [planner construction][planner], [executor construction][executor-factory]

Provider/model routing does not close the gap. The factory router returns a fresh traced client for the routed provider when the local store exists. The scheduler caches/selects routed clients separately from its base client. Its explicit `SetShardContext` forwarding method targets only `c.Client`; the normal request method does not invoke it. Setting a base client's mutable fields is therefore not a general routed-client repair. [Routed construction][router], [route selection][route-selection], [setter forwarding][setter-forwarding]

### The trace reads a different channel

The tracing constructor sets only its underlying provider and store. Its shard ID, type, category, session and task strings retain their zero values. `snapshotTraceContext()` accepts no request context; it copies those five instance fields under an RWMutex. The tool-results implementation calls that snapshot before invoking the provider, then constructs the trace from the captured copy. The caller's metadata reaches the provider but does not populate the trace. [Tracing state][trace-state], [trace snapshot][snapshot], [native trace call][trace-call]

`newReasoningTrace` directly copies the captured attribution into the record. Its `Success` means that the provider call returned no error; it is not independent verification that the coding task succeeded. The trace is handed to an asynchronous store goroutine. Store failure is logged rather than returned to the provider caller. The finding below assumes that this asynchronous write completes successfully; it does not need a shutdown race or a failed write. [Trace construction and persistence][trace-build]

### Empty attribution survives storage

The factory's store adapter passes the record to `LocalStore.StoreReasoningTrace`. That facade extracts the attribution fields and passes them to `storeReasoningTraceRaw`, which inserts them without enriching them from a request. The table declares the identity columns `NOT NULL`, but has no nonempty-string condition. An empty string is therefore a valid value on this path, not an insertion error that must alert the caller. [Store adapter][store-adapter], [facade conversion][facade], [schema][schema], [raw insert][insert]

The independent usage tracker reads its own shard and session metadata from context. Consequently, accurate usage attribution and empty trace attribution can coexist. This review does not claim every provider reports complete usage, nor does it reopen the separate detailed-usage accounting work. [Usage consumer][usage-consumer]

## The consumer that misses the record

On the normal chat turn path, chat calls `HydrateSessionContext` with `collectTraceShardTypes()`. That collector begins with coder, tester, reviewer, researcher, planner and security, adds known result types, normalizes and deduplicates, and drops empty names. [Chat hydration][chat-call], [role collection][roles]

The hydrator iterates those nonempty names and invokes `QueryTraces`. That method reaches `GetShardTraces`, whose SQL includes `WHERE shard_type = ?`. The empty stored type does not match `coder` or any other nonempty requested role. An empty result is not a query error, so it does not trigger the partial-hydration warning. The hydrator atomically installs the facts it did obtain. [Hydration][hydrate], [trace projection][projection], [exact retrieval][exact]

The `reasoning_trace` fact contains trace ID, role, category, session, provider-call success and duration. It does **not** contain the full response. The projection's conversion of empty names to an unknown atom happens only after retrieval; it cannot recover a row already excluded by SQL. The demonstrated outcome is a missing fact from this named-role projection, not a demonstrated loss of all response text supplied to future prompts. [Trace projection][projection]

Two further bounds matter:

- `Executor.hydrateMemory` passes **nil shardTypes**. That entry path asks for no trace rows, whether attribution is correct or not. Fixing the producer alone does not activate executor trace recall. Chat is the concrete inspected named-role consumer. [Executor hydration][executor-hydrate]
- Session turns and similar-content retrieval are separate hydration channels. Broad `GetRecentTraces` can retrieve the blank row, while exact per-type statistics also omit it from the named role's totals. No blanket claim about all learning, all logs, or all memory follows. [Hydration][hydrate], [broad retrieval][recent], [type statistics][stats]

## A falsifiable static witness

This is a source-derived scenario, **not an executed reproduction**.

1. Use the ordinary factory with a healthy local knowledge database, a configured scheduler, and a provider implementing tool-results calls.
2. Start a delegated `/fix` task with a nonempty session S and a worker name W. The request context carries W and its coder role through the scoped executor path.
3. Let that request reach a fresh tracing wrapper whose mutable attribution setter has not been called. Its snapshot has empty attribution even though the provider receives the request context.
4. Let the provider return a normal response and let the asynchronous trace write finish. Assume no unrelated ID collision, store failure, or premature process exit.
5. Read the stored record: the source path supplies empty `shard_type` and session fields. Query the same store for `coder`: the exact predicate excludes it.
6. On a later chat hydration against that store, the named-role requests likewise cannot produce that record's `reasoning_trace` fact.

This witness needs only one sequential request. Concurrency is a repair risk, not a prerequisite for the current omission. A caller that explicitly sets correct fields before an isolated call is a positive counterexample; so is a nonstandard construction that binds a dedicated wrapper to an identity. A failed database open bypasses tracing and is a different case. A provider without the required native interface fails before this tool-results trace is built. None falsifies the bounded fresh-factory, context-only path.

## Existing coverage and duplicate screen

The tracing tool-results uplift test manually invokes `SetShardContext` before calling with `context.Background()`, then checks the stored attribution. It proves that explicitly populated wrapper fields work; it does not exercise the production context-only producer. Store fixtures likewise construct records with explicit nonempty role values. These are useful controls to retain, not evidence that the factory-to-hydrator contract is already tested. [Tracing fixture][trace-test], [store fixture][store-test]

The concurrent-context test interleaves setter, getter and clear operations. It does not make traced requests and assert that each result retains its own request's identity. A passing race check on that fixture would not establish semantic request ownership. No such test was run in this review. [Concurrent fixture][concurrent-test]

Targeted repository searches for `SetShardContext`, `snapshotTraceContext`, tracing attribution terms, and relevant QA references found declarations, wrapper forwarding and setter-oriented tests, but no production executor bridge closing this channel mismatch. The nearest inspected documents are:

- [Interaction audit surface projection parity][prior-surface]: response representation and visible-output projection, not request attribution.
- [Chat session memory ownership and coherent recovery][prior-memory]: conversation checkpoints, compressor ownership and hydration replacement, not provider trace identity.
- [Detailed usage wiring and verified efficiency][prior-usage]: normalized accounting reports and receipt settlement, not the tracing wrapper's mutable metadata.

The perception specification says that the tracing wrapper sets shard context for attributable learning. That intent does not establish a caller in the inspected production path. This is a bounded duplicate screen over the current indexed repository and the relevant documents, not proof that no historical discussion ever mentioned attribution. [Perception specification][spec]

## Proposed ownership contract

**Proposed, not shipped:** every traced request captures its known attribution from that request's immutable context at wrapper entry. The captured value accompanies the response and asynchronous persistence. Later calls, cancellation, model routing and other workers must not rewrite it.

1. Define a typed metadata accessor or reuse existing typed accessors for fields whose meanings already match. Carry role and session without introducing a second owner.
2. Do not silently equate a stable worker name with a globally unique execution ID. Specify the `ShardID` meaning and any compatibility mapping. Keep persona role distinct from lifecycle category; the current usage context does not supply category or task description.
3. Specify presence and precedence. If legacy wrapper defaults remain supported, an explicit request identity must win as one coherent scope. Avoid combining one session's explicit ID with another request's stale role. Missing identity remains explicitly unknown, not guessed from prompts or provider names.
4. Snapshot only the small needed values before the provider call and copy them into the durable record. Do not retain a whole request context merely to recover metadata during asynchronous storage.
5. Apply the same rule to base, worker, planner and routed wrappers and all supported completion forms. Preserve caller context, cancellation and provider behavior.
6. Preserve unknown historical rows as unknown unless independently attributable evidence supports a migration. Do not change named-role SQL to absorb all blank records; that trades omission for fabricated ownership.
7. Keep trace persistence best-effort unless a separately approved durability contract changes it. Expose missing-attribution and write-failure counts separately, so successful persistence is not mistaken for useful named attribution.

Go's context documentation describes request-scoped values propagated across API boundaries and safe concurrent context use. OpenTelemetry's context specification similarly models execution-scoped immutable context. These support the ownership pattern, not a requirement to introduce an OpenTelemetry dependency or claim compliance. [Go context documentation](https://pkg.go.dev/context), [OpenTelemetry context specification](https://opentelemetry.io/docs/specs/otel/context/)

### Avoid a lock safe but request unsafe repair

The existing mutex protects field accesses. The evidence does **not** establish a Go data race in those attribution fields. Moreover, capturing before the call means a later setter cannot alter the already captured value.

A naive shared `Set(A); Call(A); Clear()` scheme still permits `Set(A), Set(B), Call(A)`, causing A to snapshot B. Individual locked operations can be memory-safe while the request association is wrong. `ClearShardContext` also currently leaves the session field intact, which must be considered in any legacy fallback rule. Holding the client mutex across the network call would serialize independent requests rather than establish their identity. [Tracing state][trace-state], [trace snapshot][snapshot]

The Go memory model's synchronization guarantees address data races and ordering, not the application's intended mapping of identity to a call. Use a deterministic ownership assertion in addition to a race detector. [Go memory model](https://go.dev/ref/mem)

## Costs and alternatives

Reading a fixed number of typed context fields and copying small immutable values requires no additional model request; lookup cost still depends on context depth. The retained values add record bytes and more meaningful identity cardinality. Existing exact role queries can retain their result limits and indexes; broad scans to compensate for missing metadata are unnecessary.

A new execution-ID contract or historical repair has larger costs: schema compatibility, migration validation and potentially unresolvable legacy records. Task descriptions can introduce privacy and retention concerns and should not be collected merely to fill an empty column. Separate attribution completeness from response-content retention.

Client-per-request construction can avoid shared mutable ownership but may sacrifice router caching and connection reuse. A set/call/clear protocol is superficially smaller but adds concurrency and route-target hazards. A request accessor reuses the channel already transported to the provider. None of these options has been benchmarked here.

Correct attribution may make existing role-specific retrieval return additional metadata, changing downstream context selection. That is intended coverage restoration, not evidence of better answers. Measure the full task outcome before asserting a quality, latency or token-efficiency improvement.

## Unrun validation and acceptance boundary

Future checks should use deterministic providers and the real wrapper, adapter, store and kernel seams before any live-model comparison. No proposed check below was executed.

| Check | Required observation |
| --- | --- |
| Context-only production construction | Without calling a mutable setter, a named request reaches a real trace row with the expected role and session. The provider sees the same identity. |
| Named retrieval and chat hydration | After explicit asynchronous-store completion, exact retrieval returns that row and normal chat role hydration includes its trace ID exactly once. Check metadata fields, not an invented response-bearing fact. |
| Honest controls | Explicit legacy setter behavior is either preserved by a documented rule or intentionally retired; absent identity remains unknown; unrelated named roles do not receive the record. |
| Shared-client interleaving | Barrier-controlled A and B requests use distinct roles/sessions and inverted completion order; every persisted trace retains its own request identity, without sleeps as synchronization. |
| Routed and secondary slots | Base, configured worker, planner and cached routed-client paths preserve request identity. Setting the base client must not be the hidden precondition. |
| Completion-method parity | Text, schema, streaming, native tools and tool-results paths follow the same attribution contract where supported; unsupported interfaces remain explicit errors. |
| Lifecycle and failures | Provider error/cancellation retain the owning identity; store failure is distinguished from missing attribution; shutdown/drain assumptions are explicit rather than inferred from provider return. |
| Nested task scope | Child work replaces the parent's worker identity; reclassification within one named task does not rename that worker or lose its session. |
| Consumer bounds | Executor hydration with nil shardTypes still requests no traces unless separately changed; broad recent reads and named statistics behave consistently with their distinct contracts. |

Keep model, prompts, responses, tool permissions and physical request counts fixed when comparing old and repaired instrumentation. Count trace write/read latency, database bytes, dropped records and attribution completeness; do not count telemetry rows as task successes. For later capability evaluation, use paired tasks and identical acceptance criteria, report all retries and failed tasks, and compare verified success before cost per verified success. Unknown or incomplete costs are not zero. The repair alone earns no benchmark or learning-quality claim.

**Closure:** a real request's known owner survives factory routing, provider completion, asynchronous persistence and the existing exact named consumer, without shared-client identity mutation or false ownership of unknown history. Static reasoning supports the gap; execution evidence is still required to claim the repair works.

[factory]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/system/factory.go#L1139-L1179
[planner]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/system/factory.go#L1181-L1196
[executor-factory]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/system/factory.go#L2544-L2571
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/session/executor.go#L877-L914
[setup]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/session/executor.go#L1350-L1417
[task]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/session/task_executor.go#L442-L464
[usage-context]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/usage/usage_tracker.go#L936-L953
[usage-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/usage/usage_tracker.go#L637-L651
[working]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/session/working_context.go#L969-L993
[adapter-context]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/system/factory_adapters.go#L562-L570
[adapter-call]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/system/factory_adapters.go#L646-L665
[scheduled-call]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/core/scheduled_llm_client.go#L373-L428
[router]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/system/factory.go#L3108-L3130
[route-selection]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/core/scheduled_llm_client.go#L41-L91
[setter-forwarding]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/core/scheduled_llm_client.go#L431-L452
[trace-state]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/perception/tracing_client.go#L58-L109
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/perception/tracing_client.go#L293-L309
[trace-call]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/perception/tracing_client.go#L411-L449
[trace-build]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/perception/tracing_client.go#L320-L358
[store-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/system/factory_adapters.go#L27-L41
[facade]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/store/local_verification.go#L167-L226
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/store/trace_store.go#L87-L125
[insert]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/store/trace_store.go#L168-L198
[exact]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/store/trace_store.go#L203-L237
[chat-call]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/cmd/nerd/chat/process.go#L884-L891
[roles]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/cmd/nerd/chat/process_dream_delegation.go#L460-L486
[hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/core/virtual_store_predicates.go#L727-L806
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/core/virtual_store_predicates.go#L396-L443
[executor-hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/session/executor_memory.go#L57-L85
[recent]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/store/trace_store.go#L350-L373
[stats]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/store/trace_store.go#L537-L559
[trace-test]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/perception/tracing_client_uplift_test.go#L28-L68
[store-test]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/store/trace_store_test.go#L10-L68
[concurrent-test]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/internal/perception/tracing_client_test.go#L393-L413
[prior-surface]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/Docs/audits/interaction-audit-surface-projection-parity.md
[prior-memory]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/Docs/audits/chat-session-memory-ownership-and-recovery.md
[prior-usage]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/Docs/audits/detailed-usage-wiring-and-verified-efficiency.md
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/29a448ad1c4ed74577d56aeb3c03badfc4db51e1/Docs/architecture/perception/IMPLEMENTED_SPEC.md#L479-L481


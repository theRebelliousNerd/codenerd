---
doc-class: deep-dive
subsystem: prompt
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
supersedes: []
---

# Preserve compilation ownership through strategy classification

## Conclusion and scope

Carry the shared compilation context through the learned-strategy provider into Jev problem-type classification. The production provider currently invokes a classifier wrapper that replaces this context with `context.Background()`. An answered, uncached Jev request therefore misses the context-based usage tracker and observer, and an outstanding request cannot receive the compiler owner's shutdown cancellation. The request retains its own timeout; the finding is not an unbounded network call or evidence that the user's cancellation cannot return promptly.

The source baseline is public main at [`fef98230c20a5751e4c32a2fd2757832a9d39cdc`](https://github.com/theRebelliousNerd/codenerd/commit/fef98230c20a5751e4c32a2fd2757832a9d39cdc), revalidated at 06:11 UTC on October 30, 2026. This review inspected source and existing test definitions. It ran no repository code, tests, models, training or benchmarks and made no implementation changes. The proposed fixtures below have not been executed.

This is a context handoff defect, not a proposal to remove Jev from strategy selection. The current architecture explicitly intends Jev to classify these requests, with regex fallback and a bounded call. The older regex-only provider comment no longer describes that behavior. [Current problem-type contract][problem-contract], [provider call and obsolete comment][provider].

## Primary contract reference

[Go’s context documentation](https://pkg.go.dev/context#WithoutCancel) defines how a derived context can retain values while detaching cancellation. CodeNERD deliberately adds its own compiler-stop ownership on that shared-work context; the proposed repair preserves that existing distinction rather than substituting the caller’s cancellation.

## The production handoff

Session execution places its usage tracker and a per-turn identity on the context before invoking JIT. The executor preserves an already-present tracker and otherwise attaches its own; the normal compile call receives that context. [Tracker attachment][meter-attachment], [turn metadata][turn-metadata], [compile invocation][compile-invocation].

The common factory constructs the prompt evolver, configures its Jev client when the TypeSafe configuration resolves, and registers the strategy provider obtained from that evolver. `NewStrategyAtomProviderFor` takes the evolver's store and classifier, so this route uses the Jev-configured classifier rather than a separate regex-only instance. The finding does not require an unconnected test adapter. [Jev wiring][factory-jev], [provider registration][factory-provider], [shared classifier][provider-construction].

JIT's source collector accepts a `context.Context`, but its strategy call passes only `CompilationContext`. The provider interface and forwarding collector likewise have no context parameter. `CompilationContext` describes prompt-selection inputs; it is not the runtime cancellation and metering carrier. [Source collection][source-collection], [provider interface][provider-interface].

The concrete provider calls `classifier.Classify(strategyQueryFor(cc))`. `Classify` calls `ClassifyWithContext(context.Background(), taskRequest, "")`. On a nonempty, uncached request with `problem_types` enabled, `classifyWithJev` then invokes `JudgeProblemType`. That adapter adds an operation label to the context and calls the TypeSafe client; it does not recover the discarded tracker or owner signal. [Provider][provider], [background wrapper][background-wrapper], [Jev classification][classification], [request adapter][request-adapter].

## What is lost and what remains

### Context based usage accounting

The TypeSafe client reports a successful response through `recordUsage`, which calls `usage.RecordFromContext`. That function delivers the report only to a tracker or observer carried by the supplied context. A background context with only the later operation label carries neither. Thus the answered strategy-classification request does not reach this tracker or observer at all; it is not merely assigned to the wrong turn. The task's ordinary model requests can still be metered correctly. [Client report][client-report], [TypeSafe usage adapter][typesafe-usage], [tracker and observer delivery][usage-delivery].

This is narrower than claiming all usage evidence disappears. `Client.Ask` also calls `sessionlog.Begin` and ends the exchange with response token counts. Exchange logging uses the globally current store when enabled, so it can still write a Jev exchange and its usage even when the tracker was lost. Its turn and task correlation are read from the supplied context, however; the discarded originating context is not recovered by that mechanism. A session exchange record is not the same operation as updating the context-based usage tracker or observer. [Client exchange reporting][client-report], [exchange usage][exchange-usage], [global exchange setup and context correlation][sessionlog-begin], [exchange contents][sessionlog-end].

The architecture's statement that every answered Jev request reaches session usage tracking is therefore too broad for this route. Its separate distinction between unknown pricing and free usage remains correct: absent a matching price, represented tokens should be unpriced, not silently missing. This review establishes missing delivery, not a measured monetary loss or provider invoice discrepancy. [Documented accounting contract][accounting-contract].

### Shared compiler cancellation

The compiler deliberately detaches shared singleflight work from an individual caller's cancellation using `context.WithoutCancel(ctx)`, then creates a new cancellation function connected to the compiler's stop context. This preserves context values while giving the shared operation an owner independent of one waiting caller. A caller can return on its own cancellation without destroying work needed by another consumer. That is an existing protection, not the defect. [Shared work and waiter ownership][shared-work].

The strategy edge discards that owner context as well. If shutdown begins while its Jev request is outstanding, the compiler calls `stop` and joins active work, but this request observes only its independently created timeout. Atom collection waits for its workers before checking cancellation again. Consequently, this request can hold the join until it answers or its own bound fires; later compilation stages still reject the canceled work. Do not describe this as successful prompt publication after shutdown or as premature database closure. [Compiler close][compiler-close], [collection join and cancellation check][collection-join], [background wrapper][background-wrapper].

The production call is bounded. The evolver takes `TurnJudgmentTimeout` for problem classification, whose default is five seconds. The TypeSafe client also applies its configured request timeout, default thirty seconds, over requests and retries. The effective request bound is whichever applicable deadline expires first. Configured durations may differ; five seconds is not a measured shutdown delay or a universal maximum. [Problem timeout mapping][problem-timeout], [configuration defaults][timeout-config], [HTTP timeout and retry loop][client-timeout].

## Conditions and counterexamples

A source-derived witness requires the factory learning setup and strategy store to be available, a registered provider with a nonempty shard type, a nonempty classification query, configured Jev with `problem_types` enabled, and no relevant compiler or classifier cache hit. For the accounting witness, the entering compilation context must carry a functioning tracker or observer and Jev must return an answered response. For the shutdown witness, the response must remain outstanding when the compiler owner stops. No actual user configuration or live response was inspected.

Several existing controls avoid or bound the behavior:

- A compiler cache hit returns before strategy collection. Identical complete compilation keys can share one miss through singleflight. [Cache and shared work][cache-controls].
- The classifier caches successful answers by shard and task text with a 512-entry bound. Its strategy wrapper currently supplies an empty shard to classification; changing classification scope or cache identity is not necessary to establish this context finding. [Classifier cache][classification].
- Missing provider/store/context or an empty shard returns without selecting strategies. Jev absent, `problem_types` off or an empty request bypasses the network classifier. [Provider guards][provider], [Jev guards][classification].
- Jev errors use the existing pattern fallback. A failed response is not treated as a negative classification. The request builder uses a 16,000-byte head-and-tail content allowance plus an omission marker and omitted-byte metadata. [Fallback][classification], [bounded state][bounded-state], [bounding helper][bounding-helper].
- Strategy atoms still pass ordinary selection and budget fitting. A classification request does not prove that a returned strategy entered the final prompt, changed model behavior or granted execution permission. [Selection and fitting][selection-fitting].

## Prior audits and existing tests

The earlier JIT generation-aware audit already covers strategy database failure propagation, partial reads, degraded fallback publication and strategy-cache locking. Those findings should not be repackaged as new here. Its strategy section still explicitly calls classification regex-based, which predates the inspected Jev implementation. The present finding concerns the runtime context crossing that same provider seam, not a new cache architecture or a replacement degradation policy. [Earlier strategy findings][prior-audit].

The current Jev classifier test covers classification, cached reuse, no-kind handling, failed-call fallback and the disabled switch. Its fake `Ask` ignores the supplied context, so those assertions cannot establish tracker or shutdown continuity. Strategy-provider tests exercise an unconfigured classifier. The compiler shutdown fixture proves cancellation against a context-aware blocking vector search, not a Jev-enabled strategy provider. These are useful existing controls, not evidence that the newly identified route was behaviorally verified. [Jev fake and tests][jev-fake], [classifier assertions][jev-tests], [provider fixture][provider-tests], [compiler shutdown fixture][shutdown-tests].

## Proposed contract and evaluation

Specify the change in the prompt and prompt-evolution contracts before implementation, consistent with the repository's spec-first guidance. Keep the current shared-work ownership model. Thread its runtime context through strategy collection and the provider into the context-aware classifier, preserving the local Jev timeout. Do not replace it with the cancellation signal of a particular waiting caller. Preserve the architecture's intentional Jev classifier and the existing pattern fallback rather than silently restoring regex-only behavior. [Repository guidance][guidance], [prompt ownership guidance][prompt-guidance].

One provider request should produce one usage report, even when multiple callers share its compilation. The minimum contract can retain the current shared work's originating tracker and turn identity; allocating shared request cost among several consumers is a separate policy decision. Do not double count the actual request once per waiter, cache hit or strategy atom. Preserve exchange logs independently and test their correlation separately from aggregate tracker delivery.

Future deterministic evaluation should cover:

| Fixture | Required observation |
|---|---|
| Cold strategy request with context tracker and observer | One answered Jev request reaches both with its reported counts and operation identity |
| Enabled exchange logging alongside tracking | Exchange usage remains present and correlation is checked independently; it does not substitute for tracker delivery |
| Compiler shutdown during a blocking strategy request | Owner cancellation reaches the request, active work joins, and owned resources close afterward |
| First caller cancels while another shares the compile | The canceled caller returns without canceling the other consumer's shared request |
| Jev timeout or transport failure | Existing bounded fallback and cancellation distinctions remain observable |
| Compiler hit, classifier hit, disabled Jev and absent strategy inputs | No extra classification request and no invented usage report |

Count actual HTTP attempts as well as logical requests, reported token classes, tracker and observer deliveries, exchange writes, cache hits, shared consumers, outstanding request lifetime and compiler-close latency. Run cold and warm controls with identical workloads. If classification results or selected atoms change, check prompt equivalence and downstream quality rather than attributing a token difference to a context-only repair.

Costs include an interface migration, context-aware test fixtures and clearer accounting of work already being performed. Reported token totals may increase because omitted requests become visible; that does not show that provider spend increased. Earlier shutdown can avoid some outstanding work, but no latency or cost savings have been measured. Unknown server-side consumption after cancellation remains unknown unless the provider reports it. Do not sum raw tokens, elapsed time, memory and monetary values into an invented efficiency score.

[problem-contract]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/architecture/jev-turn-judgments.md#L446-L461
[provider]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L59-L81
[meter-attachment]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/executor.go#L845-L875
[turn-metadata]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/executor.go#L1371-L1379
[compile-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/executor.go#L1536-L1557
[factory-jev]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory_learning.go#L91-L141
[factory-provider]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/system/factory_learning.go#L161-L168
[provider-construction]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L136-L150
[source-collection]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/compiler_sources.go#L106-L111
[provider-interface]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/strategy_atoms.go#L26-L67
[background-wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/classifier.go#L98-L103
[classification]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/classifier.go#L39-L86
[request-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/learnjudge/problemtype.go#L83-L106
[client-report]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/typesafe/client.go#L92-L102
[typesafe-usage]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/typesafe/usage.go#L36-L54
[usage-delivery]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/usage/usage_tracker.go#L1036-L1050
[exchange-usage]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/typesafe/client.go#L264-L274
[sessionlog-begin]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/sessionlog/call.go#L58-L98
[sessionlog-end]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/sessionlog/call.go#L120-L138
[accounting-contract]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/architecture/jev-turn-judgments.md#L872-L882
[shared-work]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/compiler_pipeline.go#L145-L189
[compiler-close]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/compiler.go#L408-L421
[collection-join]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/compiler_pipeline.go#L250-L282
[problem-timeout]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/jev.go#L30-L58
[timeout-config]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/config/typesafe.go#L232-L238
[client-timeout]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/typesafe/client.go#L182-L216
[cache-controls]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/compiler_pipeline.go#L111-L172
[bounded-state]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/learnjudge/problemtype.go#L52-L62
[bounding-helper]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/learnjudge/bound.go#L5-L33
[selection-fitting]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/compiler_pipeline.go#L303-L346
[prior-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/jit-generation-aware-optimization.md#L466-L539
[jev-fake]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/jev_test.go#L21-L35
[jev-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/jev_test.go#L185-L214
[provider-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/autopoiesis/prompt_evolution/strategy_atoms_test.go#L75-L110
[shutdown-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/compiler_shutdown_test.go#L13-L86
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/agents.md#L98-L104
[prompt-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/agents.md#L3-L14

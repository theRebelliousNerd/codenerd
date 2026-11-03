---
doc-class: research-proposal
subsystem: broker-context-system
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Bind context token counters to the serving model's calibration

## Finding and limits

**Normal bootstrap leaves the context token counters bound to the empty model identity, so they keep the seeded ratio even while the same process learns valid ratios from named-model provider responses.** The broker's request counters are not similarly disconnected: they estimate and train with the request's model. This is a missing identity handoff into upstream context sizing, not a claim that accounting or final request admission is universally broken.

The production bootstrap configures the meter's window, reserves and receipt sink, but never supplies `MeterConfig.PrimaryModel`. Every `internal/context.NewTokenCounter` requests `TextCounter("")`; that constructor resolves the meter's still-empty primary name once and stores it in the counter. The calibrator rejects observations whose model is empty. Therefore normal named observations cannot move these context counters out of `ConfidenceSeeded` or away from the default **3.5 runes per estimated token**. [Boot configuration][boot-config], [context counter][context-counter], [one-time binding][text-counter], [empty-model rejection and seed][calibration].

The source proves disconnected calibration, not a fixed percentage error. The seed can be a good estimate for a particular input; a learned ratio is also an estimate, not an exact tokenizer. Changes to selected context, compression timing, actual prompt tokens, refusals, answer quality and total task cost depend on the workload and decision boundaries.

Source pin: main [`388205138ba6d47c1cff0c5e97c896dfa642a391`][main], inspected October 30, 2026; main revalidated unchanged at approximately 15:43 UTC. An independent source reviewer corroborated the missing primary binding, captured empty-model key and intact named-request calibration. No tests, application execution, provider inference or benchmarks were run; only this scratch research document was written.

## Prior ownership and review scope

This is an integration residual, not the invention of calibrated counting:

- The existing broker already owns provider usage, request admission, shared calibration and reconciliation. Its context-facing `TextCounter` explicitly exists to replace a disconnected private ruler. Keep that architecture rather than introducing another estimator. [Broker contract][broker-contract], [text counter][text-counter].
- The **dark-field baseline already lists `MeterConfig.PrimaryModel`**. Credit that generic detection. This report adds the normal boot → empty key → rejected training → live context-consumer trace and its closure criterion; it does not claim first discovery of the unwritten field. [Existing baseline][dark-field].
- The detailed-usage audit and its as-built section own normalized usage propagation, retry-aware calibration and settlement completeness. Those repairs remain controls. The present seam occurs after a valid named observation has reached the shared calibrator, not in provider usage transport. [Prior usage audit][usage-audit].
- Broker documentation illustrates `PrimaryModel: model`, while current production boot omits it. The documented API example is not evidence that the production call supplies the field. A separate TODO about removing unused *getter* APIs does not establish that context sizing no longer needs model identity. [API example][api-example], [getter cleanup note][getter-note].

Review included root and scoped system/perception guidance, the context package guidance, main's audit inventory, and additions through [`research-main-20261030-1517`][research]. Those latest additions cover reproduction outcomes, consultation semantic admission, context-feedback cache publication and duplicate-tool substitutes; this finding is not a second version of the preceding feedback-score race.

The five active non-research heads were checked: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. In each, `internal/system/broker_meter.go`, `internal/broker/text.go` and `internal/context/tokens.go` are byte-identical to main. This is a comparison of the three relevant files, not whole-branch equivalence.

## Production trace

### 1. Normal bootstrap does configure the broker—but not this identity

Factory boot calls `configureBrokerMeter` before constructing its configured client. The configuration helper computes the workspace output, thinking and tool-use reserves and supplies the total window and sink. Its normal, nil-config and nonpositive-window branches supply no primary model. The default singleton starts from `NewMeter(MeterConfig{})`, and `Configure` changes `primaryModel` only for a nonempty supplied field. [Factory order][factory-order], [configuration helper][boot-config], [default and configure][configure].

Client construction is not an implicit repair. `InstallBroker` derives credentials, calls the default meter's `ConfigFor`, and wraps the client. `ConfigFor` places the credential model in that client's broker configuration; it does not assign the meter's primary identity. A named configured API client is therefore a clean witness: its request identity exists even though the separate context counter's identity does not. [Client installation][install], [per-client configuration][client-config].

This review conditions on a healthy normal chat bootstrap with a configured named model and admitted responses carrying usable input usage. It does not require an unavailable provider, absent credentials, malformed configuration, a second process or concurrent boots.

### 2. The real chat compressor captures the missing name

Shared chat boot constructs the session compressor through `newSessionCompressor` and `NewCompressorWithParams`. The common constructor creates its `TokenCounter`; the context counter creates the broker `TextCounter("")`. That object keeps the calibrator pointer **and a string model value**, not a callback resolving the primary name on each count. [Chat construction][chat-boot], [compressor constructor][compressor-constructor], [context counter][context-counter], [binding semantics][text-counter].

Consequently, adding a primary-model assignment *after* these counters exist would not fix the long-lived counter. Newly constructed unnamed counters could use the new identity while the old one remained empty, producing disagreement inside one compressor. Repair must define ownership and ordering, not merely add a late setter.

### 3. Provider observations train another key

For eligible settled responses, the broker calls its calibrating counter with `Observation{Model: req.Model, Chars: measure(req).Total(), ActualInputTokens: ...}`. It excludes partial and multi-attempt totals from calibration. The estimating request counter also looks up `req.Model`. These are matching, named keys. [Settlement guard][settlement], [observation identity][observation], [request estimator][request-counter].

The context counter instead queries `Ratio("")`. With no state for that key, `Ratio` returns the seed and seeded confidence. `Observe` expressly refuses `Model == ""`, so even an unnamed response cannot accidentally train that entry. Successful named traffic cannot heal the missing handoff. [Calibration behavior][calibration].

## A falsifiable witness without claiming measured savings

Use one fresh process, ordinary configured chat boot and a model named M:

1. Before a qualifying response, both M's estimator and the unnamed context counter legitimately use the seed.
2. Let one admitted, nonpartial, single-attempt response provide an observation with 7,000 measured request characters and 1,000 actual input tokens. This satisfies the existing minimum-size and ratio guards. At its first observation, M's ratio becomes 7.
3. Count an unchanged 3,500-rune string. Under the code's arithmetic, the named text counter estimates 500 tokens; the context counter still estimates 1,000 and reports seeded confidence.
4. Supply otherwise eligible context whose estimated cost crosses an unchanged reserve boundary between these values, accounting separately for fact syntax and other content. The current and correctly bound selectors can then choose different sets. Alternatively, use a ratio below 3.5 to examine underestimation and delayed compression rather than assuming the defect always overestimates.

The numeric example is a proposed controlled observation, not a statement about any actual provider or the correct tokenization of that string. A provider's request-level ratio includes its observed content mix and framing; applying it to bare fact text remains approximate. The precise invariant is that **the context consumer uses the intended model's current calibration**, not that every estimate equals a future bill.

## Live effects are broader than fallback scoring, but still bounded

Both activation selectors construct these unnamed counters: the Go path filters by its activation threshold and then fits the atom budget; the kernel-prefiltered path retains Mangle's relevance decision and performs budget fitting without that threshold. `buildKernelDerivedContext` invokes the latter. Thus a successful Mangle selection does not bypass the token-estimation seam. This differs from the prior feedback-score audit's rendered-context Go-fallback scope. [Both selectors][selectors], [kernel consumer][kernel-consumer].

The context block builder uses the same unnamed counter for core, atoms, history and recent-turn accounting. `ProcessTurn` counts original and compressed content and recalculates budget before considering compression. `IsCompressionActive` is always true once a rolling segment exists; otherwise it consults the usage threshold. Those controls mean a different estimate need not change activation state on every turn. [Builder][builder], [turn accounting][turn-accounting], [compression gate][compression-gate].

Important counterevidence and limits:

- Named broker request admission still uses the named estimate or the provider counting route. Correctly counted requests can be refused at that later boundary even if upstream assembly underestimated. Missing context binding is not permission to bypass that refusal.
- The serializer retains its independent 64 KiB clamping threshold with a visible elision marker (the marker can extend the returned text beyond the threshold). This defect does not establish an unbounded rendered context. [Serialization bound][serialization-bound], [clamp contract][clamp].
- Relevance, ordering, reserve sizes, sample guards and final request admission still operate. No argument here authorizes admitting an irrelevant fact merely because more estimated room appears.
- If M has no eligible observation, or its learned ratio equals 3.5, binding should not change estimates. Small prompts or facts far from a budget boundary may produce identical selections.
- Compression is currently kernel-driven observation masking rather than an extra summarization model call. Do not turn changed compression timing into an assumed saved inference call. [Current compression implementation][compression].
- This report does not establish the frequency of rejected requests, lost facts, repeated tools, bad answers or dollar savings. Recoverable history, source tools and later requests can supply omitted detail.

## Proposed repair: carry identity without changing the model

The narrow repair is an explicit model-identity handoff from the **resolved serving configuration** to the context estimator before consumers are constructed. Preserve the early window/reserve configuration; it exists to cover client construction and should not be postponed wholesale.

Prefer consumer-owned counter binding where different serving clients can coexist. If the current primary-model convention is retained, specify exactly which main conversational model it denotes, when it is assigned, and how existing compressor/builder/selector counters are rebound together after an authorized model change. Merely changing a process-global string cannot retarget counters that captured a previous value.

Use the same exact model key as the relevant request's calibration. Do not guess a provider default, normalize away an exact model pin, borrow the classifier's cheap model, or select another model to make calibration appear available. A legitimate secondary route or model switch needs an explicit estimation policy: bind to the selected route, maintain separately identified estimates, or disclose seeded/unknown identity until resolution. Multiple live Cortex instances must not silently steal one another's binding. These are repair safeguards, not additional proven failures needed for the fresh-process witness.

No provider call is inherently needed to bind the existing shared state. Preserve seed operation before the first valid observation, the ratio bounds, minimum observation size, rejection of partial/multi-attempt training and confidence labels. Do not replace a seeded estimate with a claim of exactness.

Update the production wiring test as well as the API test. Existing `TestTextCounterResolvesTheEmptyModelToThePrimary` explicitly constructs a meter *with* `PrimaryModel`; it proves the helper works when supplied, not that normal boot supplies it. The shared-ratio test correctly proves an already-created named counter follows later observations. Retain both controls. [Existing tests][counter-tests].

## Costs and fair evaluation — unrun

Binding itself adds little state and need not add inference, embeddings, database storage or an extra provider-count request. An explicit per-consumer design has constructor/plumbing and lifecycle costs; rebinding may require recomputing already cached budget totals. Keep synchronization coherent and avoid rebuilding the whole compressor or losing its history simply to change its estimator.

A larger learned ratio may retain more context and increase actual prompt tokens, provider latency and cost; a smaller ratio may compress earlier or omit facts and require recovery tools. Either direction can help or hurt quality. The request-level estimate can react to content-mix shifts, so compare calibrated and seeded errors on representative code, prose and multilingual data rather than treating adaptive counting as automatically superior.

Proposed evaluation:

1. **Production binding:** bootstrap through the real factory and shared chat path with a controlled named client; deliver valid usage through the ordinary broker settlement path. Verify the long-lived counter and newly created selector/builder counters refer to the same intended model and leave seeded confidence together.
2. **Negative controls:** no usage, subminimum input, invalid ratio, partial or multi-attempt report, equal-to-seed ratio, empty identity, and unrelated classifier/worker model observations. None may manufacture calibration for M.
3. **Selection:** exercise both Mangle-prefiltered and Go-fallback facts with costs just below/above a reserve, plus roomy budgets. Verify actual selected identities and the final serialized prompt, not only counter values.
4. **Lifecycle:** authorized model changes, fallback routes, repeated cached bootstrap, multiple live owners, failed boot and shutdown. An old counter must not silently retain the wrong identity; another owner must not be retargeted accidentally.
5. **Independent admission:** keep final broker and client input-window checks enabled. Include a case where downstream admission already rejects the baseline, a case where both variants fit, and the serialization cap. No estimator repair may loosen policy or truthfully reported refusal.
6. **End-to-end cost and quality:** pair normal-entry tasks at equal model, initial repository, authority and budgets with world/CodeDOM context enabled. Measure estimate-versus-actual error, compression transitions, selected facts, omitted-detail recovery, all model tokens/calls, tool work, latency, verified outcomes and failures. Count any extra retained context and repair turns. Report no-effect cases and uncertainty rather than claiming a fixed token reduction.

**Acceptance:** after an eligible observation for the actual serving model, normal context sizing uses that model's shared calibration and truthful confidence; it does not remain on the unreachable empty-model key, and model ownership, selection policy and final admission remain intact.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[research]: https://github.com/theRebelliousNerd/codenerd/commit/eb50ab1d51fc131a4d9af49f76f5aea62c855832
[boot-config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/broker_meter.go#L28-L75
[context-counter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/tokens.go#L38-L63
[text-counter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/text.go#L9-L65
[calibration]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/calibration.go#L8-L158
[broker-contract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/doc.go
[dark-field]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/scripts/testdata/dark-fields-baseline.txt#L24-L30
[usage-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/detailed-usage-wiring-and-verified-efficiency.md
[api-example]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/broker/06-PUBLIC-API-AND-TYPES.md
[getter-note]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/broker/TODO.md#L563-L568
[factory-order]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1090-L1112
[configure]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/default.go#L75-L109
[install]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/perception/broker_install.go#L20-L61
[client-config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/default.go#L255-L270
[chat-boot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L348-L361
[compressor-constructor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor.go#L622-L663
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/broker.go#L258-L270
[observation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/broker.go#L319-L344
[request-counter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/counter.go#L43-L62
[selectors]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/activation.go#L434-L482
[kernel-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_metrics.go#L661-L666
[builder]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/serializer.go#L629-L658
[turn-accounting]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L127-L183
[compression-gate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_metrics.go#L105-L125
[serialization-bound]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/serializer.go#L309-L325
[compression]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L330-L444
[counter-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/broker/text_default_test.go#L13-L93

[clamp]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/types/elide.go

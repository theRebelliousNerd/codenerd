# Ollama shard model overrides do not reach request bodies

## Finding

A valid model-only shard profile can request Ollama model **B** while the worker client is configured for **A**. The normal delegation path preserves B in `types.WithModelName`, but the scheduler retains the existing client when the context does not name a provider, and every Ollama generation request builder inspected takes its wire `model` from the configured client field. The resulting request selects A, not B.

This contradicts the explicit same-provider model-override contract of `ShardProfile.Model`; an omitted `Provider` is supported configuration, not an invalid cross-vendor request. [Profile contract][contract], [validation][validation], [dispatch][dispatch], [Ollama request builders][transport].

**Assessment:** a source-proven request-selection defect with a reachable normal entry path, not a measured model-quality failure, an authorization bypass, a defect in every provider, or a newly introduced regression. No builds, tests, local inference, or local Git operations were performed for this audit.

Repository: codeNERD. Current source pin: [`8decae5763cb88bbd842795ca81d3e2559ec2b20`][current]. Inspection date: 29 October 2026. Main advanced during review; the affected path was revalidated at this pin rather than relying on the earlier `bc610aeb` snapshot.

## The configuration is admissible

The narrow witness assumes:

- A healthy worker configured with provider `ollama` and model A, with A and distinct B both available at the intended local endpoint.
- A `coder` shard profile containing `Model: B` and no `Provider`. Other configuration and policy prerequisites are valid.
- Worker `max_input_tokens` is zero. A nonzero unsupported input cap is a separate construction refusal and is not evidence for this finding.
- No planner is configured, and an ordinary admissible `/spawn coder <task>` reaches its first model request. No worker-initialization failure or fallback is used.

A and B are symbolic tags, not claims about installed models, tag immutability, benchmark results, or particular model families.

`GetShardProfile` returns the explicit profile with defaults applied; `applyShardDefaults` defaults retries, not the provider or model. `ShardProfileContext` attaches a nonempty model but attaches a provider only when one was explicitly supplied. The checker accepts Ollama local tags and validates a profile without its own provider against the serving worker/main provider. Thus the missing provider is not a reason to erase B. [Profile lookup][lookup], [defaults and context][profile-context], [tag validation][tag-validation], [profile validation][validation].

The worker constructor uses the slot model to construct Ollama and checks input-cap support. Factory fallback after a failed worker initialization is real, but excluded by the healthy-worker witness. [Worker construction][worker-construction], [cap enforcement][cap], [worker wiring][worker-wiring].

## The normal spawn path preserves the requested model

The concrete entry path at the current pin is:

1. The command registry routes `/spawn` to `handleCmdSpawn`. The handler takes `coder` as the shard type and calls `spawnShard`. [Registry][registry], [handler][handler].
2. `spawnShard` creates a session operation context and calls `spawnTask`. That now delegates to `spawnTaskRequest`; the request retains bare `coder` until the task executor normalizes it. `spawnTaskRequest` first applies `withShardModelContext(ctx, req.IntentVerb)`. [Spawn][spawn], [task wrapper][task-wrapper], [request and profile helper][routing].
3. `withShardModelContext` reads the coder profile and calls `config.ShardProfileContext`. At this point the context contains requested model B and no provider override. [Profile helper][routing], [context attachment][profile-context].
4. `JITExecutor.intentFor` obtains the `coder -> /fix` mapping from `persona_verb`. The current isolation table does not mark `/fix` isolated. `Execute` goes through the verification wrapper and `executeObserved`, which runs a cloned session executor with the supplied context and preset intent. This trace establishes a path to generation, not a successful terminal verdict. [Persona resolution][persona-resolution], [policy rows][persona-policy], [task execution][task-execution].
5. The factory supplies the worker-backed scheduled client to the session adapter and installs the profile hook. The executor applies that hook, but `withShardProfile` explicitly preserves a context that already names a model. It therefore does not discard B. With no planner, `llmForVerb` selects the base task client. [Task client wiring][task-client], [profile hook][profile-hook], [profile application][profile-application], [profile preservation][preservation], [tier choice][tier].
6. Generation passes that context to the working-history/native-tool path, or to simple completion when no tools are configured. The session adapter adds metering context and forwards it. Neither step repairs the selected model. [Generation][generation], [adapter forwarding][adapter].
7. `ScheduledLLMCall.clientFor` returns its existing client immediately when `ProviderFromContext` is absent. Model B alone does not cause construction of a B client. The Ollama request builders then serialize configured A. [Dispatch][dispatch], [request builders][transport], [native history][history].

The current main added expert consultation before generation. That does not defeat this witness: the direct spawn request has an empty structured target, `selectExperts` returns an empty selection for an empty target, and the consultation helper returns the unchanged prompt when there are no spawns. This is a narrow no-consultation witness, not a claim that all expert paths are equivalent. [Request helper][routing], [expert selection][expert-selection], [consultation guard][expert-guard].

The keyless Ollama worker's broker layer is also not a hidden model router: the installed default broker configuration has no router, and its native-tool and history branches forward the supplied context to the underlying client. [Broker configuration][broker-config], [broker tool call][broker-tools], [broker history][broker-history].

## Every inspected generation mode has the same model source

This table concerns the Ollama client APIs and their actual request builders, not a claim that one TUI turn exercises every mode.

| Call mode | Request construction | Model-only B on an A client |
| --- | --- | --- |
| `Complete` | Delegates to `CompleteWithSystem` | Wire model A |
| `CompleteWithSystem` | Transport request sets `Model: c.model` | Wire model A |
| `CompleteWithTools` | Transport request sets `Model: c.model` | Wire model A |
| `CompleteWithToolResults` | Wrapper builds native history with `Model: c.model` | Wire model A |
| Raw `CompleteWithStreaming` | Transport streaming request sets `Model: c.model` | Wire model A |
| Session adapter streaming | `StreamWhole` wraps `CompleteWithSystem`; it is not the raw SSE path | Wire model A through the system path |

Sources: [wrapper forwarding and history][history], [plain and system requests][system-request], [raw streaming request][stream-request], [tool request and usage][tools-request], [session streaming adapter][adapter-stream].

Ollama's official compatibility documentation exposes `model` on its local `/v1/chat/completions` interface and documents tools and streaming. The server does not receive Go context values; in this implementation the relevant selector is the serialized request field. No SDK change is needed to inspect or repair that field. [Official Ollama documentation](https://docs.ollama.com/api/openai-compatibility).

The claim is about **model overrides**, not all profile settings. For example, the system, raw-stream, and single-turn tool builders do consult `types.TemperatureFor(ctx, ...)`. That is positive evidence that context can reach these builders while the model override remains unread. It does not establish complete sampling support in every mode. [System request][system-request], [stream request][stream-request], [tool request][tools-request].

## Requested identity is not the same as observed execution

| Identity layer in the narrow witness | Source-supported value |
| --- | --- |
| Profile request | B |
| Selected existing Ollama client | A |
| Serialized request `model` | A |
| Ollama client `GetModel` and `ModelIdentity` | A |
| Usage model label in the inspected paths | A |
| Actual backend weights or immutable model digest | Not established |

The current client and usage labels do **not** falsely report B while sending A. They honestly describe configured A selected on the wire. The unmet promise is that the accepted profile requested B and the serving path ignored it. The executor's compilation context takes identity and the input window from the selected client, not the context model; changing only the wire field risks creating a new disagreement between model-aware preparation and dispatch. [Ollama identity and setter][identity], [usage and request sources][transport], [native history usage][history], [serving identity][serving-identity], [compilation identity and budget][compilation].

An HTTP capture of `model: B` would prove dispatch selection after a fix, not prove which weights a remote or local endpoint actually ran. A provider-reported response model is another observation, not an immutable attestation. No answer self-description should substitute for request or receipt evidence.

## Counterexamples narrow rather than erase the finding

**Explicit provider routing works differently.** Adding `Provider: "ollama"` alongside B causes `clientFor` to invoke the router and cache a client by provider/model. `NewRoutedClientFromUserConfig` routes from the serving slot and constructs Ollama with the routed model. The routed client can therefore have configured model B, making its existing `c.model` builders select B. This is a real source-supported workaround under normal construction prerequisites, not a live run result and not proof that model-only profiles work. [Scheduler routing][dispatch], [routed construction][routed-construction].

**Changing the client's model works.** `OllamaClient.SetModel` updates both wrapper and transport. A client deliberately set to B is a useful positive control. Mutating a shared client for each shard request is not an acceptable per-call fix because overlapping A/B requests would then share mutable state. [Setter][identity].

**The omission predates this main update.** In the inspected older session-lane snapshot `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, native history already used `c.model`, and its then-shared OpenAI transport used `c.model` for system, streaming, and tool requests. This establishes an older occurrence, not the original introduction date or equivalence of all old branches. [Older history][old-history], [older system][old-system], [older stream][old-stream], [older tools][old-tools].

## Proposed contract and repair choices

Resolve the effective provider and model once after the serving tier is chosen and before model-dependent JIT selection, budget/admission checks, and request construction. An accepted same-provider model-only override must either be honored consistently or be refused explicitly before a model request; it must not silently become the configured model.

Keep resolution request-local, and keep the chosen identity consistent across a tool conversation. A bounded provenance record should distinguish the requested override, serving slot, resolved provider/model, wire model, operation/attempt identifier, and any separately labeled provider-reported identity. Store bounded identifiers rather than credentials or raw prompt bodies. Request construction, usage reporting, and model-aware preparation should consume the same resolution result. Absence of backend attestation must remain explicit.

| Choice | Benefit | Cost or obligation |
| --- | --- | --- |
| A request-local Ollama model resolver used by every builder | Smallest direct repair of the unread context value | Must also reconcile JIT/client identity, usage, broker behavior, retries, and all call modes; a wire-only edit is incomplete |
| Broaden scheduled routing to model-only overrides | Reuses construction of model-specific clients | Must preserve the selected tier's endpoint, credentials, options, effort, caps, and lifecycle; do not silently switch slots or vendors |
| Explicit `Provider: "ollama"` in affected profiles | Existing route can construct a B client without a code change | User configuration burden; does not fulfill the currently documented model-only contract |
| Reject unsupported model-only overrides | Prevents silent substitution | Compatibility break for currently accepted configuration; requires a clear error and corrected contract |
| Per-request global `SetModel` mutation | Appears simple | Reject as a design: cross-request state and concurrency risk |

The resolution itself requires no extra inference or prompt tokens. Its implementation cost is request-local resolution and provenance metadata, plus contract reconciliation and a multi-mode test surface. Actually using B instead of A may change load time, memory demand, latency, throughput, and output behavior in either direction. These are design tradeoffs, not measured costs; no speed, token, cost, correctness, or capability improvement has been measured here. Any implementation should follow the repository's spec-first rule and update the relevant capability/gap and current-state evidence alongside code. [Repository guidance][root-guide].

## Fair validation plan not yet run

Start with deterministic fake HTTP endpoints and public synthetic inputs; do not require real-model inference to prove which model is serialized.

1. **Model-selection matrix:** configured A with no override sends A; model-only B sends B after repair; an empty or whitespace-only profile model keeps A; explicit Ollama/B routing sends B; direct `SetModel(B)` is a positive control. Validate through the config/profile path as well as direct contexts.
2. **All call modes:** cover every table row, initial requests and tool-result continuations, retry attempts, raw stream completion, and cancellation. Check usage and failure labels as well as the request body.
3. **Concurrency isolation:** overlap A and B requests on the same base client, including history and streams. Each request must retain its own resolved identity; the default client must remain A.
4. **Routing and refusal controls:** preserve worker/planner selection, endpoint, options, effort, and caps; explicit provider without a router must fail rather than fall back. A denied admission must produce no HTTP request. An unavailable B must not silently retry as A.
5. **Production-door witness:** run the normal factory and `/spawn coder` path with CodeDOM/world-model context enabled, capture the first and subsequent request identities, and separately inspect JIT pinning and budget/admission inputs. A mock protocol test does not replace this wiring check.
6. **Accounting:** count provider requests and retries; verify usage is neither duplicated nor relabeled, and failed/retried attempts remain distinguishable from responses carrying provider usage. Distinguish requested, serialized, and provider-reported identity.
7. **Optional real-model comparison only after authorization:** use available public A/B fixtures, identical task and endpoint conditions, and matched sampling/output constraints. Compare model-only routing after repair with an explicit-provider B control, not only with baseline A. Separate warm and cold loads; report latency, resource use, request counts, and task-quality results independently.

Success means consistent selection or explicit refusal, not merely a plausible answer or a passing terminal task verdict. Neither this matrix nor a live quality benchmark was run.

## Current guidance and nearby audit inventory

The current configuration guidance forbids implicit model/provider substitution and requires consistent execution/provider settings across normal boot paths. Perception guidance treats slot/profile configuration as authoritative and requires reconciliation of provider behavior with its architecture documents. These are relevant maintenance contracts; the implementation citations above, rather than stale architecture prose, establish current behavior. [Configuration guidance][config-guide], [perception guidance][perception-guide], [root guidance][root-guide].

The research inventory was observed at **21:36 UTC on 29 October 2026** and its heads rechecked at **21:48 UTC**, not inferred from branch-name timestamps:

| Research branch snapshot | Pinned commit | Review extent relevant here |
| --- | --- | --- |
| `research-main-20261029-1018` | `75240b6649e9e7fce0c117ac5b03c6f4f0a95840` | Inventory contained 116 top-level audit documents; targeted exact-model audit read |
| `research-main-20261029-1950` | `76d6bff4c8cb17a9da185750045bfa285afc79f9` | Current inventory screen |
| `research-main-20261029-2049` | `66d071486a41e9928521c518ebfd5368d6e96756` | Current inventory screen and targeted OpenAI model-transition audit read |

The nearby [exact model pin and tag identity audit][prior-pin] concerns exact-tag pin collisions after model-name normalization, not loss of a valid per-call Ollama override. The [OpenAI model transition and effort admission audit][prior-openai] concerns a live `/model` transition and effort admission, not this Ollama request path. This targeted distinction is not a claim that the complete audit corpus was read or that no related report exists.

## Evidence boundary

This is a source-only audit of public codeNERD material, pinned repository files, targeted research snapshots, and official Ollama documentation. It includes no private-project examples. Source reachability is conditional on normal initialization and admission; no endpoint receipt, installed-model inventory, inference, end-to-end completion, or quality gain was observed. The decisive current defect is bounded: **B survives as the model-only request intent, while the inspected Ollama wire builders still select A.**

[current]: https://github.com/theRebelliousNerd/codenerd/commit/8decae5763cb88bbd842795ca81d3e2559ec2b20
[contract]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/config/shard.go#L14-L28
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/config/check.go#L245-L283
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/core/scheduled_llm_client.go#L40-L72
[transport]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_ollama_transport.go
[lookup]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/config/user_config.go#L1380-L1399
[profile-context]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/config/shard.go#L65-L138
[tag-validation]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/config/check.go#L65-L100
[worker-construction]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_factory.go#L676-L704
[cap]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_factory.go#L763-L776
[worker-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/system/factory.go#L1128-L1148
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/chat/commands.go#L176-L177
[handler]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/chat/commands_handlers_analysis.go#L574-L594
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/chat/delegation.go#L765-L780
[task-wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/chat/delegation.go#L42-L53
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/cmd/nerd/chat/delegation_routing.go#L350-L384
[persona-resolution]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/task_executor.go#L165-L219
[persona-policy]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/core/defaults/policy/delegation.mg#L505-L531
[task-execution]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/task_executor.go#L317-L441
[task-client]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/system/factory.go#L2511-L2540
[profile-hook]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/system/factory.go#L2576-L2582
[profile-application]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/executor.go#L1373-L1377
[preservation]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/shard_profile_context.go#L26-L42
[tier]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/executor.go#L508-L523
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/executor.go#L1814-L1875
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/system/factory_adapters.go#L606-L665
[history]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_ollama.go#L55-L104
[expert-selection]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/expert_context.go#L19-L36
[expert-guard]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/expert_spawn.go#L63-L72
[broker-config]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/broker/default.go#L243-L257
[broker-tools]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/broker/broker.go#L400-L415
[broker-history]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/broker/optional.go#L17-L46
[system-request]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_ollama_transport.go#L63-L100
[stream-request]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_ollama_transport.go#L234-L245
[tools-request]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_ollama_transport.go#L345-L410
[adapter-stream]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/system/factory_adapters.go#L669-L675
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_ollama.go#L107-L131
[serving-identity]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/executor.go#L525-L569
[compilation]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/session/executor.go#L1514-L1531
[routed-construction]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/client_factory.go#L654-L704
[old-history]: https://github.com/theRebelliousNerd/codenerd/blob/80af1863d3a3aa46a722ac8b0c8a0985582d4995/internal/perception/client_ollama.go#L85-L111
[old-system]: https://github.com/theRebelliousNerd/codenerd/blob/80af1863d3a3aa46a722ac8b0c8a0985582d4995/internal/perception/client_openai.go#L95-L110
[old-stream]: https://github.com/theRebelliousNerd/codenerd/blob/80af1863d3a3aa46a722ac8b0c8a0985582d4995/internal/perception/client_openai.go#L234-L250
[old-tools]: https://github.com/theRebelliousNerd/codenerd/blob/80af1863d3a3aa46a722ac8b0c8a0985582d4995/internal/perception/client_openai.go#L355-L369
[root-guide]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/agents.md
[config-guide]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/config/agents.md
[perception-guide]: https://github.com/theRebelliousNerd/codenerd/blob/8decae5763cb88bbd842795ca81d3e2559ec2b20/internal/perception/agents.md
[prior-pin]: https://github.com/theRebelliousNerd/codenerd/blob/75240b6649e9e7fce0c117ac5b03c6f4f0a95840/Docs/audits/exact-model-pin-tag-identity.md
[prior-openai]: https://github.com/theRebelliousNerd/codenerd/blob/66d071486a41e9928521c518ebfd5368d6e96756/Docs/audits/openai-model-transition-effort-admission.md


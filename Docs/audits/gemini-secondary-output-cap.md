---
doc-class: deep-dive
subsystem: model-routing
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2
verification-method: static-source-inspection
---

# Gemini secondary-slot output budgets must survive provider construction

## Finding and decision

A Gemini worker or planner can accept a per-slot `max_output_tokens` value yet send the workspace-wide Gemini ceiling instead. The secondary-slot factory carries the requested cap into `ProviderConfig.MaxOutputTokens`, but the shared Gemini constructor branch reads only `ProviderConfig.Gemini.MaxOutputTokens`. This loses the distinction between a cheap bulk worker and a larger-budget planner before either makes a request.

Resolve output-budget provenance before publishing a secondary client: an explicitly configured secondary cap should govern that slot, with a documented fallback for an unset cap. Preserve the separate main-client Gemini contract. If a secondary ceiling is unsupported, reject that explicit configuration rather than accept a setting that does nothing.

This source-only finding is pinned to main commit `d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2` (October 30, 2026, 04:07:39 UTC). No application, test, inference, benchmark, or provider request was run. The evidence proves a mismatch in constructed request limits, not observed excessive generation, truncation, billing, or end-to-end task failure.

## The distinction that matters: main versus secondary

The main `UserConfig.MaxOutputTokens` comment explicitly says Gemini keeps its own `gemini.max_output_tokens` setting. Ignoring the generic main field for Gemini is therefore not the reported defect. [Main contract](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/config/user_config.go#L70-L77).

The worker and planner share `SecondaryLLMConfig`. Its output field explicitly exists so a bulk worker can use a small budget while a planner uses a larger one. It promises a cap for the slot, not just for OpenAI-compatible providers. The structure has a per-slot Anthropic options block but no per-slot Gemini options block. Consequently, directing Gemini users to the single global field does not implement independently configurable worker and planner caps. [Secondary contract](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/config/user_config.go#L815-L855).

Google documents `GenerationConfig.maxOutputTokens` as a response-generation limit. Its thinking guide explains that the output parameter includes thought tokens. It is not a promise to generate that many tokens, nor a visible-answer-only allowance. These upstream semantics matter when defining and testing precedence; changing a cap can change the room available for reasoning as well as the answer. This report does not assert that the repository's 65,536 default is the current maximum of every Gemini model. [GenerationConfig reference](https://ai.google.dev/api/generate-content#v1beta.GenerationConfig), [Thinking and token limits](https://ai.google.dev/gemini-api/docs/generate-content/thinking).

## Source-derived witness

Consider an otherwise valid workspace with a Gemini worker, its explicitly selected model and credentials, `worker.max_output_tokens: 2048`, and `gemini.max_output_tokens: 8192`. Leave `worker.max_input_tokens` unset and do not override the serving provider/model for the witnessed call. The numeric values illustrate configuration transport; they are not recommended budgets for a particular task or model.

1. `GetWorkerLLMConfig` returns a resolved copy of the worker slot; its resolution fills reasoning effort and Ollama-specific defaults without replacing the output cap. [Resolution](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/config/user_config.go#L1000-L1047).
2. `newSecondarySlotClient` constructs a `ProviderConfig` containing the workspace Gemini options at line 729 and the worker's separate cap at line 742. It then calls the shared `NewClientFromConfig`. [Secondary construction](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_factory.go#L710-L760).
3. The Gemini switch begins with `DefaultGeminiConfig`, applies the model and endpoint, and copies Gemini-specific options. For output it copies only `config.Gemini.MaxOutputTokens`; it never consumes `config.MaxOutputTokens`. Thus the 2048 reaches the shared factory but is discarded there, while 8192 reaches `GeminiConfig`. [Gemini branch](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_factory.go#L554-L580).
4. `NewGeminiClientWithConfig` stores that value as `c.maxOutputTokens`. The system completion request serializes it in `GenerationConfig.MaxOutputTokens`. [Constructor](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_gemini.go#L135-L175), [Wire construction](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_gemini.go#L492-L503).

The source-derived result is therefore a request ceiling of 8192 rather than the configured worker ceiling of 2048. Actual generation could finish well below either ceiling. The converse matters too: a planner configured above the global Gemini ceiling cannot use its requested larger budget. This is configuration fidelity in both directions, not simply a recommendation to reduce output.

The same stored field appears in schema generation, streaming, native function calls, and tool-result continuation requests. Repairing only one serialization method would leave other modes inconsistent. [Schema](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_gemini.go#L770-L785), [Streaming](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_gemini_streaming.go#L85-L100), [Native tools](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_gemini_tools.go#L175-L188), [Tool results](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_gemini_tools.go#L284-L297).

## Production entry and ownership

This is not solely a direct-constructor configuration. The normal system factory calls the worker constructor, wraps the resulting client with tracing when available, installs a scheduled shard client, and separately constructs the optional planner. The configured worker becomes the session adapter's task client; the planner is installed on the session executor and spawner. [Boot installation](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/system/factory.go#L1149-L1188), [Task client](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/system/factory.go#L2535-L2555), [Planner installation](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/system/factory.go#L2742-L2748).

The executor chooses the planner only when the kernel marks the verb reasoning-intensive; otherwise it uses the base client. It intends to choose once per turn so follow-up requests stay on the same serving client. That correctly preserves provider ownership, but it also preserves the incorrectly constructed ceiling. [Slot choice](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/executor.go#L519-L533).

No change to CodeDOM, the world model, JIT selection, or permission policy is required to exhibit the loss. Those normal production mechanisms should remain enabled when future integration tests exercise the route. A provider-level request recorder is enough for the narrow wire proof, but not a substitute for the factory integration assertion.

## Existing safeguards and exclusions

- Missing credentials or model configuration can prevent construction. Factory boot can log a secondary initialization failure and retain another tier; that is not the witness. The claim requires successful construction and actual selection of the Gemini slot.
- Gemini cannot satisfy a positive slot input-cap contract in the inspected path. `slotInputCapEnforced` independently refuses a client that does not report the requested input window. Leaving the input cap unset isolates the output finding; removing that safeguard would not be an acceptable repair. [Input admission](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_factory.go#L752-L776).
- A profile naming a different provider/model can route elsewhere. The witness excludes it. The existing route contract carries token ceilings only when provider and model remain the same, so future fixes must not indiscriminately copy a cap across model changes. [Route contract](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/config/reasoning_effort.go#L119-L147).
- Dedicated classification construction is separate: it deliberately applies a label ceiling, with a cross-provider test covering Gemini. Do not generalize the finding to every Gemini call or remove that classification behavior. [Classification coverage](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/classification_ceiling_test.go).
- Existing Gemini length-stop checks reject truncated replies, including malformed partial tool calls. They observe the selected client limit; they cannot recover a slot cap discarded at construction. [Completion truncation](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_gemini.go#L607-L615).
- An early cancellation, other admission rejection, or short answer may avoid any visible consequence. Neither a high advertised cap nor a successful response establishes excess spend.

## Required behavior and options

The essential contract is that the effective secondary budget has one explainable source and reaches every eligible request mode.

| Approach | Benefit | Cost or constraint |
| --- | --- | --- |
| Resolve secondary cap into a copied Gemini config before calling the shared factory | Keeps the main-client exception intact; leverages existing request builders | Must distinguish an explicit secondary value from the shared provider default and avoid mutating workspace options |
| Add explicit slot-aware output resolution to the shared factory | Centralizes effective-value derivation and permits uniform diagnostics | Requires callers to convey slot/provenance; blindly honoring generic main MaxOutputTokens changes documented main behavior |
| Introduce a per-slot Gemini options block | Supports future thinking/grounding distinctions as well as output | Larger configuration surface, migration/documentation burden, precedence rules and validation matrix |
| Reject nonzero unsupported Gemini secondary caps | Prevents silent false acceptance | Does not provide independent budgets and breaks currently accepted configuration; error must identify the field |
| Lower the single global Gemini cap | Existing operational workaround for a one-budget setup | Alters all Gemini slots and cannot express different worker/planner budgets; not a general repair |

Do not solve this by setting a mutable shared client's output limit around each call: overlapping requests could borrow another slot's policy. Client construction should use a coherent resolved snapshot. Preserve endpoint, model, credential-pool selection, thinking and grounding options, tracing, and broker metering.

An unset secondary cap needs an explicit decision: inherit the workspace's Gemini provider default, or preserve the currently documented client-default semantics. Either choice must be stated and covered; this report does not silently settle that compatibility question. The explicit nonzero witness is unaffected by that ambiguity.

Reject invalid values and model-incompatible combinations through the appropriate configuration/admission contract rather than inventing a model, changing effort, or launching a probe. Keep Gemini's combined thinking/output meaning visible in documentation and diagnostics.

## Costs, tradeoffs, and fair evaluation

Configuration resolution adds a small deterministic field-selection/copy operation, but its exact CPU and allocation cost has not been measured. A broader typed configuration redesign adds migration, documentation, maintenance and test cost; per-slot options can multiply the compatibility surface. Reconstructing clients can affect transport reuse and lifecycle ownership, so a design should avoid unnecessary reconstruction.

Actually honoring a smaller budget can reduce available reasoning/answer space, increase truncation and retry or repair work, and worsen task quality. Honoring a larger planner budget can increase output, latency and billed usage while potentially permitting a complete plan. Neither direction is automatically an efficiency win. A ceiling is not expenditure; no token, latency, monetary, or quality improvement was measured here.

A fair evaluation separates three questions: was the configured limit transmitted, did provider generation respect the intended budget semantics, and did end-to-end useful work improve? Begin with deterministic fake endpoints and synthetic requests. Later live evaluation, only when authorized, must keep model, task, input, tools, thinking settings, grounding, and sampling controls matched while varying the budget policy deliberately.

Proposed tests, not run:

| Scenario | Required evidence |
| --- | --- |
| Worker cap below global Gemini cap | Constructed client and wire contain the worker value |
| Planner cap above global Gemini cap | Planner wire contains its explicit value |
| Distinct worker/planner caps, same model | Neither slot borrows the other's budget |
| Secondary cap unset | Declared fallback precedence, tested explicitly |
| Main generic cap and Gemini-specific cap differ | Existing documented main Gemini behavior preserved |
| Streaming, schema, tools, tool continuation | Same effective cap in every call mode |
| Credential pooling and tracing/broker wrappers | Correct limit survives normal wrapping; usage is not duplicated |
| Missing key/model, cancellation, positive unsupported input cap | Existing independent failures remain failures |
| Profile route to same versus different model/provider | Preserve the existing cap-transfer boundary |
| Overlapping worker/planner calls | No shared-setting mutation or cross-request leakage |
| Small cap with long output / partial tool arguments | Typed truncation remains visible, not successful completion |
| Normal factory task and reasoning route | Real slot selection with CodeDOM/world-model context enabled |

For any live comparison retain failed and retried tasks in the denominator. Record configured and resolved cap provenance, serialized cap, finish reason, candidate/thought usage when supplied, attempts, wall time, truncations, recovery turns and verified task outcomes. Compare the corrected per-slot configuration against both current behavior and an explicit global-cap control. Do not compute savings by subtracting ceiling values.

## Guidance, nearby work and evidence boundary

Root guidance requires spec-first changes, normal production validation, and tokens as a constraint rather than the goal. Configuration guidance treats explicit settings as executable policy and requires consistent provider settings across boot paths. These support repairing the acceptance/transport contract, not weakening output validation or automatically choosing cheaper models. [Root guidance](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/agents.md), [Configuration guidance](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/config/agents.md), [Perception guidance](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/agents.md).

The current `Docs/audits` inventory, `ASTRA_QUEUE.md`, targeted perception gap and implementation documents, and nearby Ollama model-override, OpenAI effort-transition and stream-completion reports were screened. The earlier Ollama report concerns selecting the requested model; OpenAI's concerns effective model/effort admission; stream completion concerns terminal outcome fidelity. None of those is this secondary Gemini budget-provenance loss. Existing worker-provider and planner-slot tests cover construction, endpoint selection, missing keys and tier identity without exercising this output-cap mismatch. [Queue](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/Docs/audits/ASTRA_QUEUE.md), [Worker tests](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_worker_providers_test.go), [Planner tests](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/perception/client_planner_slot_test.go).

The named `research-main-20261030-0211` branch was unavailable through read-only branch/tree lookup during this review; the current queue records its reports as merged. This targeted screen is not a claim to have read every audit body or proved universal novelty.

All examples are public-source-derived and synthetic. The decisive established result is narrow: an explicit Gemini secondary-slot cap reaches the shared provider config but does not reach the Gemini request ceiling.


# OpenAI model transitions must validate the effective reasoning effort

## Finding and decision

The OpenAI Responses integration validates model and reasoning effort when a client is created, but a live `/model` change can invalidate that same pair without revalidation. A healthy main OpenAI client using `gpt-6.1-terra` with an explicitly configured `reasoning_effort: none` can accept `/model gpt-6.1-sol`, persist the change, and retain `none` in the existing client. A subsequent eligible request without an effort override then combines the new model with the old, unsupported effort.

Treat model selection as a transition of effective configuration, not an isolated string assignment. Reject incompatible transitions before persistence or activation, or require an explicit compatible model and effort choice. Also validate the resolved pair at request admission. Do not silently replace the user's `none` with `low`, `medium`, or another effort.

This is a source-derived integration finding against commit `bc610aeb9e8223823c9e84bc132de993d11199c2`, not an observed runtime failure. The relevant files were first inspected at `9d26d9c3af600a5d01f904b138974e02ec7838c4` and verified unchanged at the newer revision. No application, tests, or model requests were run for this audit. The evidence establishes the accepted transition and incompatible request construction; it does not establish a newly observed HTTP status, universal request failure, or complete application startup failure.

## The compatibility rule already exists

OpenAI's model documentation, checked on October 29, 2026, lists `none`, `low`, `medium`, `high`, `xhigh`, and `max` for GPT-6.1 Terra, while GPT-6.1 Sol starts at `low` and does not list `none`. Both advertise 128,000 maximum output tokens, so this witness is not an output-ceiling mismatch. [Terra model documentation](https://developers.openai.com/api/docs/models/gpt-6.1-terra), [Sol model documentation](https://developers.openai.com/api/docs/models/gpt-6.1-sol).

The repository already encodes this distinction. `OpenAIModelEfforts` separates Sol and Astra from Terra, Luna, and GPT-6 Sol; `ValidateOpenAIEffort` checks both the vocabulary and known model compatibility. The missing capability is therefore not model knowledge. It is applying existing knowledge after mutable state changes. [Effort validation](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/config/openai.go#L166-L195).

The architecture document accurately describes checks at configuration load, client construction, and requests carrying a profile effort. Its separate report of a prior live Sol request receiving a 400 for `none` is historical repository evidence, not execution performed here. The new finding concerns the gap between those checkpoints during a supported live model switch. [Responses architecture](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/architecture/perception/14-OPENAI-RESPONSES.md#L44-L77).

## The accepted transition

The witness starts with the standard main provider set to OpenAI, a configured API key, Terra, and explicit effort `none`. Initial configuration loading and client construction succeed. The user issues the normal idle TUI command `/model gpt-6.1-sol`; no worker, planner, fast-tier, or scheduled configuration is needed.

Both names appear in the OpenAI model list. The command handler obtains the active provider and tests whether the requested name belongs to that provider's list. It then assigns `cfg.Model`, calls `Save`, updates the live client on successful save, and reports a successful switch. It does not check the existing effort against the new model. Successful persistence is an explicit precondition of this witness. [Advertised models](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/config_wizard.go#L107-L112), [Model command](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/commands_handlers_files.go#L44-L110).

`Save` marshals the configuration and writes it atomically, but does not call `Check`. Atomic file replacement protects the write operation; it does not establish semantic validity or a transaction spanning persisted and live state. [Configuration persistence](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/config/user_config.go#L792-L803).

The runtime update is not lost in a wrapper. `setModelRecursive` walks the client chain and invokes available setters. The broker updates its own model identity and forwards the change. The concrete OpenAI setter simply replaces `c.model`; it neither changes nor revalidates `c.reasoningEffort`. Thus the model can update correctly while the combined configuration becomes invalid. This is not the earlier class of stale broker-label or unforwarded-setter problem. [Recursive update](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/commands.go#L297-L318), [Broker forwarding](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/broker/broker.go#L55-L68), [Constructor and setter](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai.go#L99-L133).

## How the pair reaches request construction

The client's `modelFor` resolves a context model override or the client model. Its `effortFor` independently resolves a context effort override or the stored effort. However, `admit` invokes effort validation only when `ReasoningEffortFromContext` reports an override. With no override, the now-invalid inherited pair is not checked. `newRequest` subsequently selects Sol through `modelFor` and places inherited `none` in the reasoning field through `effortFor`. [Resolution and admission](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai.go#L146-L173), [Request composition](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai.go#L193-L231).

The concrete TUI route is conditional: a subsequent normal turn must actually reach main-client articulation. The turn context is built from the shutdown context and receives usage and session metadata, not an effort override at that construction point. Later, articulation receives `m.client` and a detached turn context. The articulation helper invokes streaming completion, optionally through a thought-streaming capability, or its nonstreaming fallback. The OpenAI streaming and nonstreaming paths both admit the request before constructing it. [Turn context](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/process.go#L76-L90), [Main-client articulation](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/process.go#L941-L970), [Streaming invocation](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/helpers_articulation.go#L305-L320), [Nonstreaming fallback](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/helpers_articulation.go#L360-L375), [OpenAI stream admission](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai_stream.go#L30-L38), [OpenAI plain admission](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai.go#L310-L325).

The witness excludes provider rerouting and context model overrides as well as effort overrides. A turn that returns through another path, fails another gate, or never reaches articulation is not this witness. Nor is a request with an explicit compatible effort. The claim is that the unsupported pair reaches the next eligible request, not that every turn or every client necessarily sends it.

## Persistence makes the inconsistency durable

After the successful command, the saved main configuration contains Sol with `none`. On a later load, `LoadUserConfig` invokes `cfg.Check` and returns a `ConfigError` for contradictions. `Check` validates the main provider, model, and reasoning effort together. The live command can therefore accept and save a pair that the configuration loader subsequently rejects. [Load validation](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/config/user_config.go#L653-L660), [Main-slot check](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/config/check.go#L137-L138).

This establishes a persistence and recovery problem without proving the whole application becomes unbootable: callers may handle loading errors. Likewise, the switch confirmation does not claim a downstream task succeeded. The defect is inconsistent configuration acceptance and request admission, not an authorization bypass or fabricated task verification.

## Controls and limits

Several controls distinguish this issue from an indiscriminate OpenAI failure:

- Terra with `medium` can switch to Sol without this incompatibility.
- Terra with `none` can switch to Luna under the repository's compatibility table; switching back to Terra also restores compatibility.
- An explicit valid context effort replaces inherited `none` for that request. An explicit invalid effort is already rejected by admission.
- A fresh Sol client with `none` is already rejected at construction. An unrecognized TUI model is rejected by list membership.
- Missing credentials, cancellation, input admission failure, or an earlier routing return can prevent transmission independently of this defect.

If streaming returns a terminal error, the articulation helper observes it and returns an articulation failure rather than silently accepting the partial output. This control does not determine a provider's HTTP status or the number of attempts. [Articulation error handling](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/helpers_articulation.go#L357-L375).

Profile settings are not a universal repair because the problematic request has no effort override. Conversely, separate worker, planner, or provider-routed scheduled clients must not be assumed to inherit the mutated main client. Fresh construction can enforce the existing compatibility check. Evaluation should inventory actual client ownership rather than extrapolate from one setter.

The existing tests exercise invalid Sol construction, valid Terra construction, invalid explicit profile effort, and an empty key without HTTP transmission. Configuration tests also distinguish model effort vocabularies. Those assertions protect useful boundaries but do not exercise a valid client becoming invalid through `SetModel`. [Client validation tests](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai_test.go#L188-L215), [Vocabulary tests](https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/config/openai_test.go#L26-L47).

Two observations are particularly diagnostic. First, `none` is explicit, not an empty value that may legitimately defer to a provider default. Second, the new model name reaches both broker identity and the underlying client. A receipt naming Sol alone therefore cannot prove the request was valid; the effort and its source are necessary to explain the failure. Conversely, a rejected request with a profile override does not demonstrate that inherited effort is protected. The transition case must remain separate in coverage.

This finding does not depend on assistant-phase replay or refusal representation. Those involve response interpretation after a request; the defect here precedes transmission and is independently observable in candidate state, persisted configuration, and request fields. That separation keeps remediation and evaluation focused.

## Required behavior without automatic selection

A transition should prepare a copied candidate configuration, resolve every affected slot's effective model and effort, and validate before writing or publishing it. An incompatible selection should leave the prior valid runtime and persisted state intact, with an error identifying the model, effort, and the effort's actual source. The user can then explicitly select a compatible pair. Configured, defaulted, and profile-supplied values need distinguishable provenance; an inherited slot setting should not be mislabeled as a profile override.

At request admission, validate the resolved model and effort regardless of whether the effort came from context. This closes both the live-switch gap and analogous combinations created by a model-only override. A UI-only guard cannot protect callers that bypass the command. A request-only guard prevents incompatible transmission but still permits an invalid saved configuration and misleadingly accepted transition.

Persistence and activation need one coherent publication contract. Candidate approaches include a fenced configuration revision or rebuilding validated clients before publishing the new generation. The existing void setter cannot report rejection, so a staged validating setter or rebuilding boundary may be necessary. This is a design requirement, not an implementation prescription. In-flight calls must retain a consistent valid generation rather than observe partially changed fields. Persistence failure and abandoned transitions must not leave half-activated state.

No provider probe, automatic model call, effort escalation, or cross-provider default rewrite is required. Compatibility can be checked deterministically from the existing supported-model rules.

For unknown models, preserve the current vocabulary-only validation policy unless that contract is explicitly changed; do not automatically rename or select a model.

## Costs and evaluation

Validation adds work proportional to the number of affected slots. Rebuilding clients may add allocations and requires explicit transport reuse and retirement ownership. A staged transition also introduces synchronization, failure handling, and wrapper interoperability costs. These should be measured rather than dismissed as free. Cache-prefix changes associated with a legitimate model change should not be credited to the guard as new savings or treated as proof of a regression.

The expected benefit is avoiding incompatible attempts and reducing recovery work after an accepted switch. There is no measured latency, token, or monetary improvement here. Do not assume a rejected provider request incurs a particular bill. Compare attempts, observed usage, and recovery time using actual receipts.

A future evaluation should compare current behavior, request-admission validation alone, and coordinated transition plus request validation under identical configurations and request fixtures. Keep failed runs in the denominator. Record invalid transmissions, persisted configurations rejected on reload, correct rejections, unchanged compatible calls, transition CPU and allocations, model attempts, reported tokens, latency, and time to recover. Receipts should expose the effective pair and setting provenance without including credentials.

The following matrix is proposed, not executed:

| Scenario | Required result |
| --- | --- |
| Terra with none to Sol | Reject before HTTP; preserve prior persisted and runtime configuration |
| Terra with medium to Sol | Accept; retain medium and correct model identity |
| Terra with none to Luna | Accept without silently changing effort |
| Compatible explicit context effort | Resolve and validate that pair normally |
| Invalid explicit context effort | Reject before HTTP |
| Context model override without effort override | Validate against inherited effort |
| Invalid persisted Sol and none | Preserve loader rejection |
| Save failure or canceled transition | No partial publication or misleading success |
| Missing key or unrelated admission failure | Preserve existing independent rejection |
| Broker and pooled wrappers | Validate every affected client and keep identities consistent |
| Separate planner and worker clients | Change only intended slots |
| In-flight transition | Each request uses one coherent configuration generation |

Use request recorders to distinguish admission failure from a constructed incompatible payload, including both streaming and nonstreaming routes. Compare cold construction, hot switching, and per-request resolution with the same compatibility cases. The acceptance criterion is consistency across those boundaries, not merely removal of one error message: pairs rejected by the declared compatibility contract never reach transmission, rejected transitions do not persist, and compatible user choices remain unchanged.


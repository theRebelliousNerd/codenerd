# Responses local and hosted tool name contract

The OpenAI Responses adapter can reject CodeNERD’s ordinary research tool catalog before sending it, even when hosted web search is disabled. The conflicting name belongs to the stock local DuckDuckGo tool, not the provider-hosted search feature. This is an availability and provider-serialization contract failure: one offered local function can prevent the model from choosing any tool, including an otherwise usable alternative.

**Source review:** inspected revision `bc610aeb9e8223823c9e84bc132de993d11199c2`, checked 2026-10-29 at approximately 20:46 UTC. This note reports source-derived behavior and proposed checks, not executed results. No tests, inference calls, or live provider probes were run. The full seven-commit, 62-file delta from `9d26d9c3af600a5d01f904b138974e02ec7838c4` was rechecked: relevant transport, policy, and catalog blobs were unchanged, and executor references were repinned after the expert-context insertion.

## The rejection precedes hosted tool attachment

`CompleteWithTools` delegates to `CompleteWithToolResults`. After input admission succeeds, that method calls `rejectHostedToolNames` before constructing the request, assigning automatic tool choice, finalizing the request, or executing it. The helper rejects every definition whose name is exactly `web_search`, reporting a collision with OpenAI hosted search. It does not condition rejection on hosted search being enabled. Consequently, definition presence is sufficient; the model need not request that function. An admission failure can still produce an earlier error. [OpenAI tool request path][openai-request]

Hosted attachment has a different condition: `finalize` appends the hosted search tool only when forced or enabled. That tool has provider type `web_search`; the configuration’s nil setting resolves to disabled. The unconditional name reservation therefore applies more broadly than actual hosted attachment. [Request finalization][openai-finalize] [Hosted tool definition][hosted-definition] [Configuration][config]

OpenAI’s documentation distinguishes custom function tools, with a function name, from hosted tools identified by their tool type. This establishes different representations, **not** proof that the API accepts a custom function named `web_search`, either alone or alongside hosted search. That compatibility question remains open; removing the guard is not yet a verified provider fix. [Function calling][function-docs] [Responses reference][responses-docs]

## The stock research catalog can reach this guard

The source supports the following witness, assuming successful normal registration, catalog compilation, request preparation, and admission, with OpenAI as the actual serving client.

1. **A local tool owns the canonical name.** `research.WebSearchTool` is named `web_search`, describes DuckDuckGo search, and accepts required `query` plus optional `max_results`. Its handler uses DuckDuckGo without an API key. `research.RegisterAll` includes it unconditionally, subject to skipping already-registered names. This is not a wrapper for provider-hosted search. [Local tool][local-tool] [Research registration][research-registration]

2. **Normal boot supplies it to the session.** The modular catalog includes the research family. `HydrateModularTools` registers tools with the virtual store and global registry; production boot calls hydration, and a session without an injected registry uses that global registry. Registration failures matter: global-registration errors are warned and skipped, so the witness requires that registration to succeed. [Catalog][catalog] [Hydration][hydration] [Boot][boot] [Session registry][session-registry]

3. **Research policy can offer it unchanged.** Researcher permissions include both `web_search` and `grounded_web_search`; research verbs map to that persona and derive allowed tools. The session derives the turn catalog, while precompiled configuration can take precedence. Offer construction emits a registered modular tool’s name and schema unchanged. Availability splitting regards registered modular tools as available without this provider-name reservation. Thus a locally available, policy-allowed tool can enter the JIT offer and subsequently fail adapter validation. [Research policy][policy] [Turn derivation][turn-derivation] [Compilation][compilation] [Offer construction and availability][offers]

4. **The native generation route consumes the definitions.** The OpenAI route uses `CompleteWithToolResults` through the working context when active, or `CompleteWithTools` otherwise. Working-context preparation applies its filters and then calls the provider. An initial generation error ends the tool loop and is settled as an error by the executor. A companion `grounded_web_search` definition cannot rescue this request: rejection happens before the model can select it. [Native consumer][native-consumer] [Working request][working-request] [Loop error][loop-error] [Error settlement][settlement]

Structural-first filtering is an important countercheck. It removes only tools classified as raw search: `grep`, `glob`, `list_files`, and `search_code`. It does **not** remove `web_search`. An ordinary initial, unrestricted, non-commit research request can therefore retain the conflicting definition. By contrast, commit-regime closed-for-reading filtering, explicit restrictions, or a precompiled catalog can omit it. [Working filters][working-filters] [Raw search classification][raw-search]

## The failure is narrower than all OpenAI turns

Catalogs without local `web_search` are not affected by this guard. Plain-text or streaming paths without these tool definitions are not this witness. Direct hosted-search operation is also distinct, as are other providers. The finding requires the selected OpenAI client actually to serve the tool request, not merely to participate in planning.

The rejected tool request cannot reach network execution through this path. That does not establish zero model calls or zero cost for the entire turn: classification, JIT work, or earlier generation may already have occurred. Nor is this an unauthorized-execution finding. The executor’s effective JIT allowlist remains an independent dispatch control. [Authorization boundary][authorization]

The predecessor’s tool path mapped definitions and posted Chat requests without this local guard. The Responses migration adds the rejection, but the earlier source does not prove that the old wire request was accepted by a provider. [Predecessor request path][predecessor]

Meta Responses offers a useful local comparison: its reserved browser-name checks accompany hosted-search attachment, while disabled attachment is a no-op. That difference warrants reviewing guard scope; it is not evidence about OpenAI’s upstream name rules. [Meta attachment policy][meta]

This note adds a concrete provider-adapter conflict to the existing request-visible tool contract work. It does not replace that work’s backend and channel projections, or the separate retry continuity distinction between grants and callability. [Existing contract scope][existing-contract] [Backend projection][existing-backend] [Retry continuity scope][retry-scope] Existing OpenAI guidance on flat function definitions and enabled hosted tools does not itself resolve the stock local name collision. [OpenAI architecture][openai-architecture]

## Preserve canonical meaning while validating the final offer

The contract should be: **the final offered catalog is serializable and valid for the configured provider without one canonical tool accidentally blocking unrelated tools**. Local DuckDuckGo `web_search` and provider-hosted search must retain distinct identities and behavior.

Three approaches merit an explicit decision:

- **Narrow the guard only after verification.** If provider documentation or a controlled compatibility fixture establishes that the local name is valid without hosted search, reserve it only when hosted search is actually present. This still needs a decision for the enabled-hosted collision case.
- **Prefer a reversible provider name map.** Project canonical names into an injective wire namespace, using a provider-valid alias when necessary. Choose aliases against verified provider naming rules rather than assuming a particular spelling is accepted. Keep canonical names stable for policy, capabilities, and local APIs.
- **Expose deliberate unavailability where mapping is impossible.** Return a structured disposition early enough for policy-owned degradation. Do not silently remove required search, automatically enable paid hosted search, switch providers, or rename the global canonical API.

Do not blanket-remove the guard before validating the upstream contract. OpenAI and Meta policies should remain separately specified.

## Mapping must survive authorization and conversation replay

Validate that a returned wire name was actually offered, reject unknown or ambiguous aliases, and reverse a valid mapping **before** the canonical allowlist check and backend dispatch. Translation must not confer permission. A denied canonical tool must remain denied; hosted-search output must never fall through into local function dispatch.

The same mapping must cover subsequent calls and replayed history. Preserve call IDs and their tool-result pairing. Stored conversations need a mapping version or equivalent replay context, including migration and rollback behavior when aliases change. A map reconstructed differently on a later turn can corrupt otherwise valid tool history.

JIT-visible names and rendered function instructions must agree with the emitted schema. Capability `requires_tools` relationships should stay canonical, with translation at the provider boundary. Reuse the existing stable manifest or cache identity for the projected catalog and mapping; hash the relevant representation after projection. Do not create per-turn uniqueness merely to distinguish aliases, because that would defeat reuse.

Explicitly handle collisions with user-defined names, stale policy projections, partial registration, unknown returned aliases, and accidentally interpreting a hosted marker as a local call. Log safe metadata and error categories rather than prompts, tool arguments, or payloads.

## Costs and comparison criteria

Building an alias map is linear in catalog size, with bounded per-tool storage and constant-time round-trip lookup. Conversation replay adds retained mapping state. Schema spelling, alias metadata, or history changes can alter prompt tokens and cache identity; an initial cold miss is possible. The design need not add model calls, but no performance or savings percentage is established here.

Compare unchanged provider, model, context limits, permissions, and task inputs. Record call counts; input, output, reasoning, and cached tokens; projection CPU and allocations; scheduler wait; time to first generation; and end-to-end latency. Include failed-turn repair cost and earlier classification or JIT work. Granting extra tools or silently substituting hosted search would invalidate a like-for-like quality comparison.

## Verification plan not executed

Use fake HTTP transport and authorized fake handlers before any provider compatibility check.

- Exercise both OpenAI tool methods with the stock registered research catalog, hosted search off and on separately, and a catalog without `web_search`. The current-source prediction is zero sends for an admitted conflicting request, not a measured result.
- Derive a real research catalog through the production kernel and reach `generateResponse` with captured transport. Check structural-first retention, commit and explicit-restriction omissions, precompiled catalogs, successful hydration, and unavailable-backend controls.
- Inspect serialized function and hosted-tool types. Inject alias collisions and unknown returned names. Verify an allowed canonical function executes exactly once, a denied function never executes, and hosted-search output never dispatches the local handler.
- Replay multiple turns with aliases, unchanged call IDs, and paired results. Include persisted-history migration and rollback, ordinary `read_file` behavior, and Meta hosted-on and hosted-off regression cases.
- Resolve the exact upstream naming contract through an appropriately controlled, separately authorized compatibility check, without harmful prompts or unintended external actions.

Completion means no accidental whole-catalog dead end, while preserving local versus hosted semantics, canonical authorization, and replay correctness. It does not mean merely suppressing the current error.

[openai-request]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai.go#L346-L390
[openai-finalize]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai.go#L259-L267
[hosted-definition]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_openai_grounding.go#L20-L38
[config]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/config/openai.go#L16-L72
[local-tool]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/tools/research/web_search.go#L23-L61
[research-registration]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/tools/research/register.go#L8-L52
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/tools/catalog/catalog.go#L32-L54
[hydration]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/virtual_store_tools.go#L41-L81
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/system/factory.go#L1671-L1677
[session-registry]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor.go#L481-L489
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/defaults/policy/intent_routing_rules.mg#L375-L532
[turn-derivation]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/turn_catalog.go#L20-L35
[compilation]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor.go#L1686-L1762
[offers]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/tool_offer.go#L34-L122
[native-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor.go#L1793-L1847
[working-request]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/working_context.go#L1005-L1025
[loop-error]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor_tools.go#L108-L114
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor.go#L1448-L1452
[working-filters]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/working_context.go#L94-L122
[raw-search]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/tools/structural.go#L22-L32
[authorization]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor_tools.go#L2445-L2452
[meta]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/perception/client_meta_responses.go#L217-L264
[existing-contract]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md#L137-L145
[existing-backend]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md#L59-L60
[predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_openai.go#L354-L374
[retry-scope]: https://github.com/theRebelliousNerd/codenerd/blob/75240b6649e9e7fce0c117ac5b03c6f4f0a95840/Docs/audits/retry-jit-callability-continuity.md#L11-L38
[function-docs]: https://developers.openai.com/api/docs/guides/function-calling
[responses-docs]: https://developers.openai.com/api/reference/resources/responses/methods/retrieve
[openai-architecture]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/architecture/perception/14-OPENAI-RESPONSES.md


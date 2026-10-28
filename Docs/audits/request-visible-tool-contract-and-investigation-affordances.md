---
doc-class: deep-dive
subsystem: session-prompt-tools
implementation-status: planned
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Make the tool contract visible to the model that must use it

## Decision

Resolve a **request-visible tool contract**, not just a list of policy-granted names. Preserve Mangle's authority and the existing execution/feedback gates. Resolve which granted tools have a supported backend and usable call description before JIT claims they are available; carry that same contract through native and Piggyback request rendering; give every omitted grant a typed disposition. Do not solve missing discoverability by granting more tools, inventing schemas, weakening permission checks or restoring free-form shell access.

The strongest current source witness is precise: a tool in the separate Ouroboros registry can be explicitly granted and executable through the governed generated-tool bridge, yet disappear from both native requests and the normal working-loop Piggyback catalog. The existing Process-to-generated-binary test bypasses this discovery seam by scripting the call and ignoring offered definitions. A second, closely related seam discards argument schemas when rendering the Piggyback catalog. These are input-contract failures before useful investigation begins, not failures of result delivery or terminal synthesis.

**Evidence status:** static code and test-definition inspection only. No tests, binaries, provider inference, benchmark, training, failure injection or repository changes were performed. Proposed witnesses below have not been executed. The source establishes the omissions and available routes; it does not establish the frequency of user-visible failure or a measured efficiency improvement.

## 1. Freeze, guidance and non-duplication

Main resolved to [356f65e0e6bb639adaa90ab36212daa714114871][main]; `feat/conversation-brief` resolved to [c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c][feature]. These were checked at approximately 11:10 UTC on October 28, 2026. The research branch resolved at approximately 11:16 UTC to [c212ad29c9527abf5ed037472760a55f53175f60][research]. All three heads were re-resolved unchanged at approximately 11:24 UTC. Source copies and main/feature differences were inspected. The generated-definition builder, normal Piggyback routing and catalog intersection described here are the same across main and feature; the feature adds restriction continuity and control-admission changes around them. The working-context file is text-identical across the two pins.

The [root guidance][root] requires tools to condense search space, reduce turns and offload deterministic cognition; JIT must deliver relevant knowledge; executive decisions belong to the kernel. [Session guidance][session-guidance] explicitly separates capability, exact permission and execution, and requires native/Piggyback parity. [Prompt guidance][prompt-guidance] says the effective catalog is resolved before selection and must not be widened to satisfy an atom. The [architecture-document standard][standard] requires current facts, target design and checkable exits to remain distinct.

This note does not refile:

| Existing study | Boundary retained there |
|---|---|
| [TUI evidence turn ownership][tui-study] | Routing disposition, read acquisition, turn/result ownership and evidence reaching first articulation. Here the request has already selected an executor but its useful granted tool may not be described to that executor's model. |
| [Conversation brief semantic fidelity][brief-study] | Accepted conversational meaning, applicability and progressive disclosure of brief items. A correct task brief does not repair a missing tool contract. |
| [Terminal knowledge synthesis grounding][synthesis-study] | Evidence continuity into the final consultation answer. This note concerns discovery and call construction before observations exist. |
| [Delegated-turn failure settlement][settlement-study] | Settlement and ownership of failed delegated work. An executable handler does not prove its tool was offered. |
| [Tool effect receipts][effect-study] | Exact effects, validation and durable feedback after dispatch. Keep the now-working generated execution bridge; the gap is before the model can select that bridge's tool. |
| [MCP capability admission][mcp-study] | Remote protocol/schema review and exact authorization. This proposal does not turn a discovered remote operation into an admitted capability. |
| [JIT generation-aware optimization][jit-study] | Source snapshots/cache identity and compilation costs. Reuse that machinery; a fresh prompt can still contain the wrong availability projection. |

This is a refinement of a documented concern, not a claim of entirely new discovery. [Session OPEN-QUESTIONS Q4][prior-question] already identifies the modular-only native definition builder and asks whether native providers should advertise Ouroboros tools. [The internal architecture account][prior-architecture] says Piggyback lists both registries; that is true of the direct renderer but too broad for the current working-loop path. The new composed finding is that the **normal shared-loop catalog filters away tools the old merged Piggyback renderer can describe**, while downstream round-trip tests do not inspect that request boundary. The July tools wiring document also notes modular schema construction but contains obsolete allow-all claims, so it is not evidence of current safety behavior. Likewise the prompt corpus's old alternate ConfigAtom G5 issue is not this live request contract.

## 2. The normal journey and the five different sets

A natural-language workspace question reaches perception and routing; on the feature branch a delegated `/query` can start a background job, and read-only debug requests mapped to the coder are redirected to the reviewer. This is real functionality, not an unwired agent. The job invokes the task executor with its task and context. [Chat routing][chat-route], [background work][background], [task executor][task-executor]

The execution path then proceeds:

1. Prepare turn facts and derive its tool envelope.
2. Build `CompilationContext.AvailableTools`; subtract turn/restriction withholdings; compile JIT.
3. Compile the effective config and narrow it.
4. Start a working set for the declared workspace.
5. Build provider definitions; narrow those for structural-first/commit regimes; render/send the request.
6. Admit a proposed call through capability, current exact permission and the execution bridge. [Executor order][executor-order], [working set][working-set], [request narrowing][request-narrowing], [execution admission][execution]

These are not interchangeable sets:

- **Granted**: policy permits this name to be considered within the turn envelope.
- **Registered**: some backend currently owns a concrete implementation/identity.
- **Describable on this channel**: a usable, supported argument contract exists for native or Piggyback delivery.
- **Offered now**: the current regime actually puts the definition into this provider request.
- **Authorized call**: this exact action, target and canonical payload currently derives permission.

The current `AvailableTools`/`AllowedTools` pair primarily carries the first set. `DeriveTurnTools` queries policy and normalizes names; it does not consult the implementation registry or build channel schemas. [Derivation][derive]

The later native definition builder consults only `tools.Global()` and silently skips absent names apart from debug logging. The actual offering can therefore be a proper subset of the names JIT selected against. [Definition builder][definitions]

Temporary regime narrowing is intentional and should remain. A structural-first turn can retain a broader turn-eligible catalog while temporarily withholding raw search; a commit regime can close exploration. Do not confuse this declared phase transition with a tool that can never appear in any request because its backend was omitted. The target contract should name both the turn-eligible surface and the current request offer, rather than erasing the distinction. [Regimes][regimes]

## 3. Finding T1: a granted generated tool can vanish before the model sees it

### Source chain

Production boot hydrates modular tools and separately restores static/generated tools. It passes the VirtualStore's generated registry to the session executor and spawner. User-agent tool declarations can be admitted when `is_tool_registered` derives; policy then adds those names to the agent's tool envelope. Registration is not permission, but it is a real supported route to a granted name outside the modular registry: a workspace specialist explicitly declares a persisted generated tool, boot restores/registers it, registration admits the declared name, and the specialist's spawn resolves that policy grant. This does not automatically grant the generated tool to every built-in persona. [Boot][boot], [registry wiring][registry-wiring], [user-agent registration check][user-tools], [envelope rules][envelope], [spawn catalog][spawn-catalog]

`buildToolDefinitions` iterates `cfg.AllowedTools`, looks up each name only in `tools.Global()`, and skips names absent there. It never looks up `e.ouroborosRegistry`. [Definition builder][definitions]

The consequences differ by request path:

| Path | Source | Result for a tool registered only in the generated registry and explicitly granted |
|---|---|---|
| Native working request | `generateResponse` → `buildToolDefinitions` → `completeWithWorkingContext` | No definition is offered. |
| Normal Piggyback working request | `generateResponse` takes the active working-loop branch, also passes `buildToolDefinitions` | Same missing definition. |
| Piggyback rendering within that loop | `piggybackCatalog` intersects allowed names with the already-built offered definitions before calling the merged renderer | The generated name is removed before the renderer can recover it. |
| Single-shot/non-active-working-loop Piggyback | `generateResponseWithPiggybackTools` directly calls `buildToolCatalogForPiggyback` | The old merged renderer can include a granted generated tool. This alternative works at the presentation layer and must not be erased from the diagnosis. |

[Generation paths][generate], [intersection][piggyback-intersection], [merged renderer][merged]

The active working-loop case is not an exotic test-only mode: `workingLoopWorkspace` uses the declared workspace, and `beginWorkingLoop` installs the working set when that root is available. Ordinary production boot supplies a workspace. An invalid or absent root is a separate degraded/no-loop case, not the recommended route around this omission. [Working-set admission][working-set]

### What this establishes—and what it does not

A generated tool can be executable after a correctly admitted proposal but undiscoverable through the request's normal tool presentation. This does **not** prove that every generated tool is unusable: the single-shot Piggyback presentation exists, an earlier prompt may name it, and a scripted or knowledgeable model can propose a name that is in the execution allowlist. The exact bridge still checks the call. Nor is this a request to grant all generated tools merely because they are registered.

The key failure is that extension of the toolkit does not reliably extend what the ordinary working model can see and call. The tool's supposed benefit—condensing a repeated workspace investigation into one deterministic operation—cannot be evaluated fairly if the model must guess that it exists.

### Existing coverage stops after discovery

`TestExecutorPiggybackCatalogHonorsOuroborosCapabilities` calls the direct merged renderer and checks that the explicitly granted generated name appears while the ungranted name does not. Keep it: that is a valuable capability test. It does not invoke the active loop's earlier definition-builder/intersection chain. [Direct-renderer test][catalog-test]

The production boot generated-feedback round trip uses a real fixture binary, real request identity, permission controls and durable feedback after reopen. Those are substantive working capabilities. Its fake provider's `CompleteWithTools` ignores definitions, and `CompleteWithToolResults` explicitly names the definitions parameter `_`; both return the preconfigured generated call independently of the offered surface. The test therefore proves **Process → proposed call → real effect → feedback**, not **Process → visible contract → valid proposal → effect**. [Fake provider][generated-model], [round trip][generated-roundtrip]

The new acceptance should preserve this real binary test and add a request-aware provider that will propose the fixture only if the delivered contract contains it. A test that injects the name into model output is the wrong witness for discoverability.

## 4. Finding T2: Piggyback loses the schema needed to use tools precisely

Native definitions carry the modular tool's property map and required fields. In the **Piggyback catalog renderer**, those definitions are used only as a name filter; the working-request path separately uses definitions for token estimation. `buildToolCatalogForPiggyback` reads the registry again and prints the tool name, description and comma-separated required names; it does not serialize `Schema.Properties`, argument types, optional property names, enum/default semantics or nested item shape. Generated entries print description and capability labels, not a canonical argument schema. [Native schema construction][definitions], [Piggyback name intersection][piggyback-intersection], [renderer][merged]

Concrete workspace examples:

- `list_files` has `path`, optional Boolean `recursive`, and optional Boolean `include_hidden`, both default false. Its description is only “List files in a directory.” The Piggyback catalog therefore supplies `path` but not the names or types of the two controls that distinguish a shallow visible listing from recursive/hidden-file discovery. [Definition][list-files]
- `get_element` has no unconditional `Required` entries; its schema describes `ref`, a string-array `refs`, optional `path`, nested-part string `part`, and Boolean `full`. Its prose does mention ref/path/part/full, which helps and must be credited. It does not make the catalog's omission of the array shape and typed property contract lossless. [Definition][get-element]
- `read_file`'s description already names `start_line/end_line`, so it would be incorrect to claim those names are entirely undisclosed. The schema still owns their integer types and inclusive/1-indexed meanings. [Definition][read-file]

This is a source-level serialization loss, not proof that a model always sends invalid calls. A selected JIT atom, prior knowledge or a repair error can incidentally teach the missing contract. That reliance can add turns and provider/channel dependence to exactly the investigation affordances the harness is supposed to make explicit; the behavioral effect is unmeasured here.

The right comparison is not giant descriptions versus terse descriptions. Keep short tool summaries and JIT methodology. Supply an authoritative, bounded callable schema—or a discoverable typed schema expansion when it is genuinely large—rather than expecting methodology prose to duplicate API structure. A tool-generation capability label is not an argument schema.

## 5. Finding T3: runtime backend absence is not the same as policy eligibility

The researcher envelope includes `grounded_web_search`; `/verify` and `/validate` also add it. Actual registration is conditional: `RegisterGroundedWebSearchIfSupported` skips when the searcher is nil or unsupported. Boot derives the searcher from the configured LLM client's supported interface and passes it into hydration. On a fresh/isolated registry with no prior registration, a valid policy grant therefore need not have a corresponding registered tool. Skipping registration does not unregister an existing tool from the process-global registry; an unsupported later boot is not by itself proof of absence. This witness must start from a clean process or explicitly isolated/reset test registry. [Envelope][envelope], [conditional registration][grounded-registration], [boot selection][boot]

Before JIT, `resolveAvailableTools` reads the policy envelope or precompiled list. The `available_tools` template emits that list. After JIT, `buildToolDefinitions` skips missing registry entries. This is a concrete granted-versus-offered mismatch, not merely a hypothetical race. [Resolution][resolution], [template][available-template], [definitions][definitions]

The grounded-search atom also lacks `requires_tools`. Its prose explicitly says to use the tool only when the effective catalog includes it, so it is **not** an unconditional instruction to call an unavailable tool. However, its inclusion is not mechanically tied to the actual offered tool, and the earlier catalog field can still carry that name. The required repair is truthful capability state, not turning this conditional prose into a purported authorization bypass. [Atom][grounded-atom]

A compact disposition such as `granted, unavailable on this runtime/channel` would let the harness choose an existing permitted alternative or report a meaningful limitation. Generic missing-name debug logs are not the model-facing contract, and a successful alternative does not prove grounded search itself was available.

## 6. Proposed contract and ownership

### 6.1 One resolved base manifest

This is a proposed representation, not a new current type. Resolve a per-turn base manifest containing:

- Session/task/turn identity and the policy/catalog revision used.
- Each candidate name and policy disposition; applicable restriction/effect class.
- Backend kind and registration identity/generation.
- Canonical argument-schema identity and supported channels.
- Presentation status: eligible, unsupported channel, missing backend, missing schema, withheld by restriction, or invalid.
- A compact digest used by JIT and provider projection, plus redacted omission diagnostics.

Go measures registrations, schema availability and channel support. Mangle derives candidate eligibility, restriction, phase selection and whether unresolved capability obligations permit a degraded answer. Neither registration nor a model-authored tool request becomes a grant. Exact `permitted(Action,Target,Payload)` remains a fresh dispatch check, not a cached bit in this manifest.

Resolve the actual callable subset before capability-gated JIT selection. Keep a separate labeled list of unavailable grants when useful for diagnosis; do not include those in `AvailableTools` as though they were callable. The precompiled-spawn path must consume the same resolution contract rather than bypassing it with a string slice.

Keep a stable content digest for catalog/schema/policy-relevant inputs separate from the per-turn/request receipt identity. A fresh turn ID must not by itself invalidate an otherwise reusable schema projection or provider prefix; changes in the actual grant, restriction, channel, backend identity or schema must invalidate the affected projection. This reuses the existing JIT source-vector discipline rather than replacing it with an always-changing audit key.

### 6.2 Channel projection with a declared support matrix

Native and Piggyback receive projections of the same canonical contract. Preserve property types, requiredness, enum/default semantics and nested shapes. Stable field ordering and normalized schema digests make the prompt/cache identity deterministic. Short rendering is acceptable if it preserves the callable meaning; arbitrary byte clipping is not.

Generated [`core.Tool`][generated-type] currently has metadata, binary/protocol identity and capability labels, but no canonical argument-schema field. Do not manufacture one from its description and then claim native support. Choose explicitly:

1. Add an admitted schema/descriptor to generated-tool creation, persistence and restoration, then support both channels; or
2. Declare channel limitations and expose the tool only on a supported channel with an adequate typed contract; or
3. Fail that capability closed with a precise missing-contract disposition until supported.

The existing generated stdin envelope is transport, not proof of each tool's application argument types. Reuse the existing generated identity/bridge/feedback ownership; do not register executable wrappers in `tools.Global()` merely to evade review or make a count match.

Admission of a generated descriptor needs its own checkable contract: bind schema identity to the admitted binary/protocol identity, validate the supported schema vocabulary, bound bytes/depth/property counts, reject unsupported or ambiguous shapes, and preserve ownership through persistence/restoration. Descriptions and schemas are tool metadata, not system-level instructions; keep them in a data surface and do not let generated text change grants, restrictions or completion policy. An updated binary with an old unqualified descriptor is not a valid cached callable contract.

### 6.3 Per-request offer and regime changes

Derive a request offer from the base manifest and current working regime. Bind a receipt to the actual provider request: manifest digest, offered definition/schema IDs, temporary withholdings, channel and JIT-context identity. Record metadata, not an extra full prompt copy with source content and secrets.

Do not recompile the whole corpus on every round solely because raw search opens. Existing phase narrowing is useful. Give capability atoms an explicit semantic choice—turn-eligible versus immediately offered—and provide truthful phase guidance. Runtime/channel unavailability must not masquerade as a temporary phase withholding.

On registry replacement, missing schema, provider/channel change or restriction amendment, refuse a stale contract and resolve a new admissible request before asking the model to act. Do not replay an effect to repair catalog publication. An already completed tool result belongs to its original call identity even if a later request has a new manifest.

Budget the final channel projection once. The current working-request estimator runs before the Piggyback adapter appends its textual catalog; a refactor must account for the actual rendered Piggyback system text, not charge native schema definitions and then add a second full Piggyback catalog charge for the same surface. Conversely, counting only compact native definitions is not evidence that a richer final Piggyback rendering fits. Capture the final request's measured/estimated cost using the actual serving channel, and keep estimate versus provider-reported usage distinct. [Request preparation order][request-narrowing], [Piggyback append][piggyback-append]

### 6.4 Failure and recovery behavior

- **A required capability is unavailable:** report/derive the capability blocker before spending an inference call pretending the tool is on the menu, but only when an explicit evidence/capability obligation establishes that this task needs it. A missing grant name alone does not prove necessity or that it is the only useful tool. If a different admitted route can satisfy the same obligation, choose it explicitly; an unused optional grant must not block otherwise valid work.
- **Partial catalog:** keep supported tools usable, label omitted capabilities, and preserve task evidence obligations. A smaller offer is not proof the task can be completed.
- **Unknown generated schema:** unavailable, not an unconstrained object silently invented by the host.
- **Compile/config failure:** preserve empty-envelope safety, but distinguish it from an intentionally tool-free conversation. Do not diagnose every zero-tool response as model reluctance.
- **Same name in two registries:** require deterministic backend identity and collision disposition; never let presentation describe one handler while dispatch invokes another.
- **Cancellation/teardown:** cancel resolution/compilation under existing lifecycle ownership and release owned snapshots; no new background registry reader outliving its runtime.
- **Privacy:** descriptor logs may contain private command paths or remote metadata. Store IDs/digests and redacted reasons by default; retain full schemas only under existing access and retention policy.

## 7. Complete cost accounting and alternatives

No savings are measured here. The change can increase input size while reducing failed calls; either direction is possible. Judge efficiency only at equal evidence quality and truthful completion.

Track the per-task vector:

`(uncached input tokens, cache-read tokens, cache-write tokens where billed, output/reasoning tokens, embedding tokens, inference calls, tool attempts, successful useful observations, CPU time, peak memory, local/remote IO, storage writes, task wall time, human repair time)`.

Attribute costs to route/perception, tool resolution, JIT, schema projection, provider prefill/decode, tool execution and repair, and final synthesis. Keep billable token categories disjoint. Do not count cached input twice or add parallel component latency as task wall time. Price only with explicit contemporaneous provider rates and stated assumptions; no monetary estimate follows from source size alone.

Expected costs to measure:

- Registry joins and schema hashing: CPU, lock hold time, allocations and possible cold-start work.
- Persisted generated descriptors: migration, storage, validation, recovery and maintenance burden.
- Richer Piggyback schemas: extra system tokens, prefill and cache-write cost; potentially fewer guessed arguments and repair rounds.
- Immutable manifests: retained memory and snapshot lifetime; fewer duplicate registry reads and repeated policy derivations only if measured.
- Per-request receipts: serialization/IO/storage cost; use bounded metadata and sampling only where it preserves required evidence.
- More accurate unavailability: some tasks may stop earlier rather than falsely claim success; report that as changed outcome quality, not a throughput win.

Compare fair alternatives:

| Alternative | Strength | Main limit |
|---|---|---|
| Current runtime | No new machinery; already safe exact admission and working feedback | Request discoverability and argument contracts vary by backend/channel. |
| Minimal union in `buildToolDefinitions` | Repairs the generated-name omission if valid schemas exist | Does not create those missing schemas, reconcile actual availability, or make Piggyback serialization complete. |
| Serialize existing modular schemas for Piggyback | Small focused repair of T2 | Leaves generated support and pre-JIT availability mismatches unresolved. |
| Full base manifest plus per-request projections | Strongest identity and parity contract | Largest design/migration cost; must demonstrate value beyond fixing the two call sites. |
| Schema-on-demand | Lower recurring schema tokens | Requires a guaranteed, policy-governed discovery route and another round; cannot hide mandatory call semantics without a usable expansion handle. |
| Advertise every registered tool | Superficially easy discovery | Rejected: registration is not capability or exact authorization, and needless surface increases cost and mistakes. |

A sensible first increment is small: preserve the current policy envelope; resolve supported backend/schema presence; make missing capability observable; render existing modular schemas faithfully for Piggyback; add request-aware tests. Adopt durable manifest persistence only if recovery requirements or measured overhead justify it. The architecture should not add a second registry or policy engine merely to close a projection bug.

## 8. Evaluation plan: test the request before celebrating the effect

All experiments here are proposed, not executed.

### A. Deterministic composition witnesses

1. **Generated tool, normal native request.** Boot a normal workspace with one admitted generated read-only fixture and its governed bridge. The fake provider refuses to propose any tool not present in the actual definitions and asserts the argument schema. Verify one valid proposal, exact authorization, one execution and correct observed output. An unsupported channel must instead produce an explicit pre-generation disposition, not a scripted invisible call.
2. **Generated tool, normal Piggyback loop.** Same runtime but an envelope client, with active working set. Inspect the actual system catalog and only then let the fake choose the tool. This directly exercises `buildToolDefinitions → piggybackCatalog → merged renderer`.
3. **Single-shot Piggyback control.** Exercise the non-active-loop direct renderer. Preserve its existing granted/ungranted distinction. A passing direct renderer must not substitute for test 2.
4. **Native/Piggyback schema parity.** Use `list_files` recursion/hidden flags and `get_element.refs` array / `full` Boolean / `part` string. Compare normalized contract semantics, not exact Markdown bytes. Verify optional controls survive and valid typed calls reach the intended behavior. This is a presentation test, not evidence of full runtime JSON Schema enforcement: preserve existing admission checks, and evaluate enum/nested/unknown-property enforcement separately rather than claiming those refusals already exist.
5. **Conditional grounded search.** Supporting searcher, unsupported searcher and nil searcher, each in a clean process or explicitly isolated/reset registry. Compare policy grant, resolved availability, JIT eligibility, actual request, and typed omission. Separately inspect a supporting-then-unsupported same-process sequence without assuming the second boot unregisters prior global state. Keep ordinary researcher/browser tools usable under their existing grants.
6. **Restricted investigation.** Whole-turn read-only request; generated/remote tools with unknown or effectful classification remain withheld. A metadata repair must not widen the grant or bypass the feature's restriction gate.
7. **Registry mutation between phases.** Replace/remove a tool after resolution but before request/dispatch. Observe stale-manifest refusal or re-resolution, no cross-generation handler swap and no replay of a completed effect.
8. **Collision.** Same name in modular and generated registries with different schemas/backend identities. Verify an explicit deterministic policy disposition; no prompt/dispatch split.
9. **Zero tools and partial catalog.** Separate a genuine conversational response from an explicit workspace-evidence/capability obligation with no available admitted satisfying route. Verify truthful blocked/partial state and no misleading no-tool nudge. A paired control with an unavailable but unnecessary optional grant must remain usable; the host must not invent necessity from the missing name.
10. **Regime control.** Structural-first raw-search withholding and commit-regime narrowing continue to work; permitted tools are not silently reintroduced to satisfy schema parity.

Existing exact-payload, bridge-refusal, capability-boundary and durable-feedback tests must remain hard assertions. The discovery test is added before them, not used to relax them. Run the normal production route with world model and CodeDOM enabled when authorized; an artificially restricted microbenchmark cannot establish normal investigation quality.

### B. User-journey evaluation

Pair held-out tasks on the same repository/model/provider configurations: explain a file; locate a nested/hidden configuration; diagnose a quoted log message; inspect a large declaration by parts; investigate an unknown symbol; use an explicitly granted generated project-analysis tool; combine repository evidence with current external facts where supported.

Use the normal chat entry with actual perception/routing, including background read-only delegation on the feature. Capture the original request, selected lane, applicable restriction, manifest/offer, JIT selection, all actual calls/results, evidence coverage and final answer. Distinguish routing mistakes from discovery failures, malformed arguments, backend errors, missing evidence and answer errors. A corrected catalog does not absolve perception or final synthesis.

Randomize paired baseline/treatment order, include warm/cold caches, vary repository size and tool-schema size, and hold model/version/limits/task evidence fixed. Report uncertainty and failures, not just averages. Primary quality: grounded answer correctness, required-evidence coverage, false completion, prohibited effects and human repair. Secondary efficiency: the full resource vector above and time to first useful observation/final answer. A schema-rich treatment that costs more per call but finishes in fewer valid calls can win; a sparse treatment that silently refuses more tasks cannot.

### C. Checkable exit criteria

| Proposed gap | Exit |
|---|---|
| TOOL-OFFER-01: backend discoverability | For every admitted supported backend/channel, the normal request-aware test sees the granted callable definition before producing a call; unsupported cases have explicit disposition. |
| TOOL-OFFER-02: callable schema fidelity | Native and Piggyback projections preserve the chosen canonical schema's property types, requiredness, nesting and documented optional controls; no hidden dependence on model memory. |
| TOOL-OFFER-03: truthful JIT availability | Capability-gated selection and available-tool rendering consume resolved runtime/channel availability; conditional missing backends do not appear callable. |
| TOOL-OFFER-04: safety and lifecycle | Existing exact permission, restrictions, generated identity/feedback, cancellation, collision and stale-generation controls remain intact; no effect replay to repair delivery. |
| TOOL-OFFER-05: measured usefulness | Normal-entry paired journeys improve or preserve evidence quality and false-completion rate with fully reported total cost; any tradeoff is explicit. |

The task is complete only when the actual request, not just the handler or a synthetic model proposal, carries the usable tool contract. That is the missing link between “the harness has this capability” and “the model can use it to investigate.”

## Source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/commit/c212ad29c9527abf5ed037472760a55f53175f60
[root]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/agents.md
[prompt-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/agents.md
[standard]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/09-architecture-doc-standard.md
[tui-study]: tui-evidence-turn-ownership.md
[brief-study]: conversation-brief-semantic-fidelity-and-progressive-disclosure.md
[synthesis-study]: terminal-knowledge-synthesis-grounding.md
[settlement-study]: delegated-turn-failure-settlement.md
[effect-study]: tool-effect-receipts-and-validation-completeness.md
[mcp-study]: mcp-capability-admission.md
[jit-study]: jit-generation-aware-optimization.md
[chat-route]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L258-L350
[background]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/background.go#L135-L205
[task-executor]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L315-L409
[executor-order]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1225-L1332
[working-set]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L166-L235
[request-narrowing]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L922-L935
[execution]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/config_factory.go#L186-L261
[definitions]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L2073-L2109
[regimes]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L88-L145
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/system/factory.go#L1655-L1676
[registry-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/system/factory.go#L2561-L2569
[user-tools]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/system/factory.go#L2347-L2413
[envelope]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/intent_routing_rules.mg
[generate]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1710-L1785
[piggyback-intersection]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/piggyback_channel.go#L166-L186
[merged]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1875-L1980
[catalog-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_capability_test.go#L153-L180
[generated-model]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/generated_feedback_roundtrip_external_test.go#L116-L139
[generated-roundtrip]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/generated_feedback_roundtrip_external_test.go#L213-L320
[list-files]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/core/file_ops.go#L621-L650
[get-element]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/codedom/elements.go#L594-L618
[read-file]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/core/file_ops.go#L22-L55
[grounded-registration]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/research/grounded_web_search.go#L115-L137
[resolution]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1607-L1690
[available-template]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/assembler.go#L506-L525
[grounded-atom]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/prompt/atoms/capability/grounded_web_search.yaml
[prior-question]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/session/OPEN-QUESTIONS.md#q4--ouroboros-tools-in-native-completewithtools
[prior-architecture]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/session/05-INTERNAL-ARCHITECTURE.md#L93-L105
[spawn-catalog]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/spawner.go#L729-L784
[generated-type]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/tool_registry.go#L26-L41
[piggyback-append]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/piggyback_channel.go#L58-L75

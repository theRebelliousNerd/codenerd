---
doc-class: deep-dive
subsystem: deliberation
implementation-status: planned
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: 7ad7edf3ca82fc9c211b2173af41bfea9f7636c6
supersedes: []
---

# Deliberation read results lose their invocation identity

## Finding and evidence boundary

The proposer and requirements interrogator can execute a valid read, then receive its result on their next model call without the arguments that identify what was read. Their local evidence loop retains a call ID, tool name, output and error. It does not retain the requested path, search scope, filters or other arguments, and it starts the next provider call from a fresh user payload rather than replaying the preceding tool-call message. A successful result that does not independently echo its complete subject therefore reaches the next decision with incomplete provenance. [Role loop][role-loop] [Result shape][result-shape]

This is a narrow source-supported evidence-attribution gap. It does not establish an observed wrong answer, fabricated successful test, unsafe mutation or retained bad improvement. The deterministic claim is that distinct successful read invocations can produce identical next-request evidence. Whether a model guesses incorrectly, repeats the read or leaves the question unresolved is not measured.

The review is pinned to public main [7ad7edf3ca82fc9c211b2173af41bfea9f7636c6][main], re-resolved on November 3, 2026. All findings come from static source, authored-test and documentation inspection. No application, test, build, model call, benchmark or failure injection was executed. Only this research document was authored.

## Contract and production route

The [repository guidance][guidance] requires relevant, recoverable evidence in bounded model requests and assigns executive control to Mangle. The [session specification][session-spec] describes read-only proposer/challenger rounds, measured evidence, bounded context and durable round records. Its readiness judgment concerns a proposed reading, while implementation verification belongs to the execution path. Preserve that distinction.

Both ordinary chat deliberation and mission recurse use the same service and registered role implementation. The chat constructor marshals a `deliberation.Frame` and spawns the requested role. Mission construction does the same through the booted Cortex. `DeliberationShard.Execute` invokes `deliberation.Agent` with its configured model, prompt compiler, kernel and read adapter. This is a production path, not an isolated unused helper. [Chat adapter][chat] [Mission adapter][mission] [Registered role][shard]

The read catalog includes the core envelope, which admits `list_files` and `grep` among the read tools. `ReadTools.Execute` submits `call.Input` to the existing registry and returns the resulting text. The defect is downstream of that execution; this report does not claim the tool ran with the wrong arguments or that registry guards were absent. [Policy envelope][envelope] [Core entries][core-reads] [Read execution][execute]

## The exact handoff

`Agent` creates one local payload with the fixed frame and `[]ProbeResult` observations. Each iteration serializes that payload and calls:

```go
client.CompleteWithTools(ctx, system, string(input), offer)
```

After a tool response, it constructs the observation from `call.ID` and `call.Name`, executes the call, and fills output or error. It never copies `call.Input` into the observation or appends the original assistant tool-call message to a history. The response's accompanying text is also not carried forward when the response contains tool calls. The next iteration therefore contains the original frame and projected results, not the previous call descriptors. [Payload and dispatch][role-loop] [Result projection][projection]

`ProbeResult` has only `Key`, `Tool`, `Output` and `Error`. A provider-generated call ID remains an opaque label unless a corresponding call descriptor or another lookup mechanism accompanies it. There is no such descriptor in this role payload. A suggestive ID could coincidentally help a model infer a subject, but the host does not require or verify semantic IDs. [Result shape][result-shape]

### Provider and wrapper checks

The call ID is not rescued by an implicit conversational history in the inspected paths:

- `OpenAIClient.CompleteWithTools` calls `CompleteWithToolResults` with `userTurn(userPrompt)`, not the previous response. The request is built from that supplied history. [OpenAI adapter][openai]
- `OpenAICompatClient.CompleteWithTools` builds `systemAndUserMessages(systemPrompt, userPrompt)`. That helper contains only the supplied system and user messages. Its Meta Responses branch likewise passes one user message. The separate `CompleteWithToolResults` method accepts a caller-supplied history, but `Agent` does not use it. [Compatible adapter][compatible] [Message helper][messages]
- `AnthropicClient.CompleteWithTools` constructs a request with a single user message and passes `userTurn(user)` to its exchange. It separately supports explicit multi-turn history. [Anthropic adapter][anthropic]
- The broker forwards the supplied system/user strings to the underlying `CompleteWithTools`; tracing and scheduled wrappers likewise forward those arguments. They do not append the preceding role call descriptor. [Broker][broker] [Tracing][tracing] [Scheduling][scheduling]

These inspections establish the issue for the named source paths, not for every possible third-party or future provider. CLI-backed agents and other adapters may have their own session behavior and need separate qualification. Provider-side prompt caching is not evidence that omitted prior arguments become an accessible conversational message.

OpenAI's current [function-calling guide](https://developers.openai.com/api/docs/guides/function-calling) illustrates the relevant distinction: its continuation examples preserve the prior response items and add results linked by call ID. That is corroborating protocol guidance, not a requirement to replace codeNERD's deliberately compact JSON envelope with an unbounded transcript.

## Small source-derived witnesses

The following are proposed fixtures, not executed reproductions. Use an unchanged workspace, registered production tools, successful reads and ample configured budget. No truncation, malformed JSON, cancellation or race is required.

### A negative search loses its scope

Let the fixed frame ask for a comparison of two ordinary workspace directories, `alpha` and `beta`. Consider two admissible first responses, each with ID `call_1`, name `grep`, and pattern `SENTINEL`:

- History A requests `path: alpha`.
- History B requests `path: beta`.

Assume both directories exist and the requested pattern is absent from the selected search domain. `executeGrep` returns `No matches found for pattern: SENTINEL` for either successful zero-match case. The output does not include the selected path or file filter. `Agent` then supplies the same original frame and the same observation in both histories:

```json
{"key":"call_1","tool":"grep","output":"No matches found for pattern: SENTINEL"}
```

The next role payloads are identical although different subjects were searched. The result cannot establish which directory has been inspected, and cannot support a claim that both were inspected. This is a projection collision between distinct histories, not an assertion that provider IDs collide within one live run. Changing the arbitrary call ID does not itself encode the missing subject. [Negative search][grep] [Projection][projection]

The missing-root branch is a useful counterexample: it explicitly prints the nonexistent path. Positive grep results also print matched file paths. Neither repairs the successful zero-match case, and even positive output does not necessarily establish the complete search domain. The finding should not be generalized to say every grep result lacks all location information. [Search branches][grep]

### A directory listing loses its base

For a normal nonrecursive listing, `executeListFiles` returns entry names joined with newlines. The recursive branch computes entries relative to the requested directory. Two different directories containing identically named admitted entries can therefore produce the same text. Since `Agent` drops the requested base and display options, the returned filenames alone need not identify which directory was listed. [Listing input and output][listing]

This second witness does not depend on the wording of a negative-result string. It establishes why repairing only grep's text would leave the generic carrier contract incomplete.

## Evidence lifecycles that must remain separate

There are two different read mechanisms in deliberation:

1. **Reads requested inside a role's `Agent` loop.** Their observations remain in the local payload. The role returns its final text; `collectRound` decodes that text into a proposal or challenge. It does not receive a typed bundle of those internal read observations. [Role return][role-return] [Round collector][round]
2. **Critical-data probes requested in a completed challenge.** Their full `ToolCall` is retained on the challenge item. `runProbes` records the resulting output/error on that same round; `collectRound` carries the previous round into the next frame. For this route, the previous challenge can still supply the probe arguments even though its `ProbeResult` alone is sparse. [Challenge shape][item] [Probe execution][probes] [Previous round][round]

The primary finding is the first route. It would be incorrect to claim that every conductor probe loses its arguments or that all saved deliberation evidence lacks a request identity. Likewise, consultations have their own requested-question and round lifecycle and are not needed for either witness.

Live-progress and diagnostic logging can contain useful tool subjects or other observations. That is not the same as supplying a descriptor to the next model request. This report makes no claim that every trace or log everywhere permanently loses the arguments. Nor does it require that no other tool could reread a subject; repeated acquisition is a possible recovery cost, not a substitute for correctly attributing the observation already obtained. [Progress emission][execute]

The mission's before/after source checks, write restrictions, verification and ratchet remain separate controls. A correct scope/argument receipt does not establish source freshness, tool correctness, test coverage or mission satisfaction. Fixing this carrier cannot make a model-authored success sentence into an empirical witness. [Mission contract][mission-contract]

## Relationship to existing audits

The current audit inventory and targeted searches for deliberation, `ProbeResult`, arguments and invocation identity were screened. The relevant documents were read, not merely their titles. No exact predecessor was identified in that bounded review; this is not an exhaustive historical novelty claim.

- [Deliberation terminal budget][budget] and its [continuation][budget-continuation] concern encoded capacity for closing notes. The present witnesses remain well below the bound and preserve every output byte. Carrying more identity must compose with those controls rather than reintroduce their failure modes.
- [Portable recurse deliberation and crucible][portable] already calls for truthful evidence and citations. This report gives a concrete current producer/consumer gap in the existing read-only funnel. It does not propose another framework or repeat the broader typed-success-test proposal.
- [Recurse values continuation ownership][values] concerns ownership of saved human questions. Current source has owner fields and pending selectors; those improvements are not being reported absent. The read identity loss occurs within one active role call sequence.
- [Research observation identity and admission][research] concerns external research acquisition and downstream workspace-freshness attribution. This report instead uses local built-in reads and the deliberation-specific request projection.
- [Mission error-free witness receipts][mission-receipt] concerns the focused host verification result entering a mission receipt. Here there is no acceptance command or gate-deficit-support claim.

## Proposed bounded contract

Update the owning session/deliberation specification before implementation. Keep the compact evidence envelope, JIT roles, existing guarded registry, explicit errors and coverage markers, bounded calls/context, and Mangle-owned decisions.

Each retained role read needs a host-produced invocation descriptor that remains resolvable together with its result. At minimum, identify the role invocation, call sequence/ID, tool, and the effective non-secret subject and parameters needed to interpret the output: for example path, pattern, file filter, line/range selector, recursion and hidden-file setting. Preserve types and literal values rather than rendering a lossy natural-language paraphrase. Distinguish requested values from any normalized/effective execution values when that difference matters.

Do not indiscriminately echo arbitrary argument maps. The envelope also includes MCP-related tools, whose inputs may need redaction or a domain-specific safe projection. Keep secret-bearing fields out of model-visible and durable records. If redaction removes information needed to interpret a result, expose that limitation or use a guarded resolvable reference; never imply complete provenance.

For compact arguments, include the descriptor directly. For oversized descriptors, a bounded host-owned reference is possible only if the model-facing continuation can actually resolve it and the referenced bytes survive for the required lifetime. A digest without accessible content proves neither the subject nor recovery. If minimum identity plus protocol controls cannot fit, refuse or close evidence truthfully before executing further reads rather than silently severing the result from its subject.

The host should construct and preserve observation identity. Mangle should continue to govern admission, relevance, retention and stopping. Model prose must not repair missing provenance by inventing a path. Existing missing metadata should remain explicitly unknown rather than being backfilled from an arbitrary call ID.

W3C [PROV-DM](https://www.w3.org/TR/prov-dm/) distinguishes a produced entity from the activity and inputs that produced it. Applied here, the design inference is modest: a result string plus an opaque label is not an adequate self-contained record of a scoped read. This does not require RDF, a PROV implementation, or a new global evidence store.

## Proposed verification and costs

Start with deterministic request-construction checks through the real role adapter and registered tools, then use the normal chat or mission boot path. No validation below has run in this review.

Required controls:

- The same-ID/same-output grep histories with different paths produce distinguishable, correctly scoped next requests.
- Same-named directory entries retain their requested base and relevant options.
- Several calls in one response and sequential calls across responses retain exact argument/result associations, including errors and refused reads.
- Unicode, escaped paths, typed numbers/booleans and reordered map keys preserve semantics.
- Long or sensitive arguments produce an explicit bounded or redacted disposition, not raw secret exposure or silent provenance loss.
- OpenAI, compatible Chat/Meta and Anthropic request captures prove that the descriptor reaches the actual model request; test other configured providers separately rather than infer parity.
- Conductor critical-data probes retain their existing full challenge association. Ordinary single reads with already self-describing output remain valid.
- Existing tool/byte budgets, closing-note behavior, read restrictions, values decisions, mission source checks and verification gates remain unchanged in authority.
- If durable role receipts are added, restart and retention tests establish their lifetime and truthful missing/expired-reference behavior. Persistence alone does not prove delivery to a model.

Authored `Agent` tests already capture outgoing inputs and use a scripted client, which is a useful request-level seam. The inspected tests check budget closing, fitting and answer nudging, not preservation of different read arguments with identical output. Their presence is source evidence only; they were not rerun. [Existing tests][tests]

The minimal repair adds serialized metadata, copying and validation. That consumes real context room and may cause earlier evidence closure or more recovery reads. A reference design trades repeated inline bytes for storage, lookup latency, retention rules and failure modes. Full native history could preserve descriptors but also adds protocol coupling and more replayed content; it is an alternative to measure, not an automatic architectural improvement.

Compare baseline, compact inline descriptors and guarded references under the same tasks, permissions, provider, prompt corpus and total budgets. Measure attribution accuracy, unnecessary repeated reads, unresolved findings, valid completed designs, rejected requests, payload bytes, input/output tokens, host allocations, retention bytes, latency and end-to-end cost. Include failures and deferrals. No reliability gain, token reduction or performance improvement has been measured.

The exit criterion is that a role can identify exactly which admitted invocation produced each observation it receives, or is explicitly told that the identity is unavailable. A plausible guess, a call ID alone, or a later reread does not satisfy that contract.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/agents.md#L19-L27
[session-spec]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L873
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/cmd/nerd/chat/deliberation.go#L18-L50
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/cmd/nerd/cmd_campaign_mission.go#L42-L69
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/shards/deliberation.go#L40-L68
[envelope]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/core/defaults/policy/deliberation.mg#L60-L67
[core-reads]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/core/defaults/policy/intent_routing_rules.mg#L258-L270
[execute]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/read_tools.go#L56-L81
[role-loop]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/read_tools.go#L106-L143
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/read_tools.go#L146-L205
[result-shape]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/types.go#L54-L59
[openai]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/perception/client_openai.go#L366-L383
[compatible]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/perception/client_openai_compat.go#L957-L1005
[messages]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/perception/client_tool_helpers.go#L73-L80
[anthropic]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/perception/client_anthropic.go#L322-L356
[broker]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/broker/broker.go#L419-L436
[tracing]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/perception/tracing_client.go#L381-L422
[scheduling]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/core/scheduled_llm_client.go#L311-L367
[grep]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/tools/core/search.go#L551-L593
[listing]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/tools/core/file_ops.go#L718-L834
[role-return]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/read_tools.go#L146-L158
[round]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/round.go#L10-L73
[item]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/types.go#L25-L31
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/outcome.go#L138-L190
[mission-contract]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[budget]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/audits/deliberation-terminal-budget.md
[budget-continuation]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/audits/deliberation-terminal-budget-continuation.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/audits/portable-recurse-deliberation-crucible.md
[values]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/audits/recurse-values-continuation-ownership.md
[research]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/audits/research-observation-identity-and-admission.md
[mission-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/Docs/audits/mission-errorfree-witness-receipts.md
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/deliberation/agent_test.go#L17-L183

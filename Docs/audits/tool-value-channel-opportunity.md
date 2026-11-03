---
doc-class: research-proposal
subsystem: session-tool-value-accounting
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Tool value must distinguish unavailable grants from actual offers

## Finding and scope

The tool-value meter counts a granted generated tool as offered even when the serving native channel intentionally cannot offer it. The request and JIT availability paths already distinguish that channel limitation correctly. The accounting producer instead unconditionally combines modular and generated definitions, records an offer session, and charges a catalog estimate. Ten distinct such sessions with no calls satisfy the stock policy's `never_called` retirement criterion. This is a measurement-to-policy integration gap, not a request to invent native schemas or make every generated tool callable. [Meter][meter], [channel contract][offer], [policy][dead], [defaults][defaults].

The established consequence is inaccurate opportunity/cost evidence and a conditional `dead / never_called` classification with a retirement **proposal**. No tool is automatically deleted. Actual withdrawal still requires a separate request, a derived proposal, and human approval or the standing yolo instruction. The source does not prove that any affected tool is useful, that a particular model would have called it if offered, or that an operator has removed one because of this evidence. [Retirement admission][retire].

Main was rechecked at approximately 15:44 UTC on 2026-10-30 and remained `388205138ba6d47c1cff0c5e97c896dfa642a391`. Independent source challenge corroborated the mechanism and emphasized that each counted session must reach the agent's successful compilation and ledger publication; boot alone does not produce the witness. This review used static source and documentation reads only. No application, tests, inference, benchmarks, provider calls or repository changes were run. The witness and evaluations below are unexecuted.

## Prior work and current controls

This is a focused continuation of **request-visible tool contract and investigation affordances**, not a new claim that the channel split is broken. That audit explicitly distinguished grants, backend registration, channel compatibility, actual offers and authorized calls. Its reconciliation records the implemented decision: generated tools remain Piggyback-only because they have no admitted typed application schema; native requests and JIT render them as unavailable. It also proposed request-bound offer receipts. The new contribution is the concrete remaining accounting consumer that reconstructs the union and feeds it into the tool-value policy despite those repairs. [Earlier audit and reconciliation][prior].

The relevant root, session, system and autopoiesis guidance was read, together with main's audit inventory and research additions through `research-main-20261030-1517` (`eb50ab1d51fc131a4d9af49f76f5aea62c855832`). That branch adds four documents, not implementation changes. The recent **tool duplicate substitute retirement** audit concerns the continuing applicability of a persisted Jev comparison after its substitute is withdrawn. This finding needs no Jev verdict, substitute, retirement race or withdrawn tool; its incorrect evidence is an offer opportunity that never existed on the serving channel. The earlier generated-feedback accounting audit concerns repeated per-execution judgments on feedback retry, not offer measurement. [1517 branch][branch], [substitute audit][substitute], [feedback audit][feedback-audit].

The five active non-research heads were checked: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. Their inspected `tool_meter.go` files retain the generated-definition union. Some surrounding code differs; this is not a claim that every branch's runtime or all source files are identical.

Important safeguards already work:

- Registration alone grants nothing. A user agent's explicit tool declarations are validated against registered tools or existing persona envelopes, and kernel policy supplies its allowed tool list. Invalid declarations refuse that agent rather than partially granting it. [Grant producer][grants], [grant validation][grant-check], [policy grant][envelope].
- `generatedToolDefinitions` skips names shadowed by modular handlers, matching dispatch precedence. Missing backends yield no generated definition. The witness requires a real registered generated-only tool. [Definition builder][offer].
- `resolveAvailableTools` separates callable names from unavailable dispositions before compilation. `buildToolDefinitions` stays modular-only for native requests. This is the correct behavior to preserve. [Availability][availability], [native request][native].
- The meter uses session deduplication, not one “session” per compile; blank session IDs do not count. Repeating ten turns in one ordinary session is not the ten-session witness. [Session deduplication][dedup].
- The profile ledger has a process mutex, cross-process lock/read-merge-write, and atomic file replacement. The witness assumes successful writes and sequential processes. Reliable persistence preserves the wrong measurement; it does not cause the semantic mismatch. [Ledger publication][publication].
- Tool value is derived in Mangle; Go exports measurements and reads the result. Unknown scan consumption remains unknown. Built-ins are only flagged, and retirement proposals remove nothing by themselves. [Fact projection][facts], [classification][classification], [retirement][retire].

## Production trace

### A supported grant can be unavailable on the selected channel

An operator can explicitly declare registered generated tool G in a valid user agent's `.nerd/agents.json` definition. Boot records the user-agent tool fact; the routing rules include it in that agent's envelope. This does not automatically grant G to all built-in personas.

The ordinary `nerd spawn <agent> <task>` command boots a Cortex and calls `SpawnTask`. Non-system/non-image tasks enter the task executor; a bare user-agent name becomes its slash verb. Inline task execution clones the session executor and calls `ProcessWithIntent`; the clone inherits the generated registry, meter and session ID. The isolated subagent path also receives those dependencies. This is not a helper-only insertion into the ledger. [CLI entry][cli], [Cortex handoff][handoff], [task identity and execution][task], [clone][clone], [spawn propagation][spawn].

Factory wiring installs the profile-store meter and VirtualStore registry on the session executor. Normal boot mints a process-unique session ID unless the caller supplies one. Separate sequential CLI processes therefore provide a normal route to distinct measured sessions without fabricating session facts. [Factory wiring][wiring], [session identity][session].

For a native serving client, `toolUnavailability` recognizes G's registered backend but returns the reason that the channel cannot carry its absent typed schema. `resolveAvailableTools` places it in `UnavailableTools`, not `AvailableTools`. The execution config retains the **authority** allowlist: `compileConfig` returns a precompiled config or assigns the kernel-derived tools. Narrowing callability does not revoke the grant. [Channel disposition][offer], [availability and config][availability].

### The meter reconstructs a broader set

After configuration and restriction narrowing, `ProcessWithIntent` calls `noteToolOffers`, before entering the model/tool loop. The meter:

1. Adds `buildToolDefinitions(cfg)` to its origin/catalog maps.
2. Always adds `generatedToolDefinitions(cfg)` to those maps.
3. Splits included teaching-atom tokens over the names it treats as offered.
4. Emits one `ToolOffer` per name with tool, origin, session and token figures.

It receives neither the selected serving channel nor the actual request offer. `ToolOffer` has no channel, callability disposition or request/compile identity field. For G, the generated builder finds the real registry entry and returns a definition even though the native request will not use that builder. [Call site][meter-site], [meter][meter], [measurement type][measurement].

The later native path calls only `buildToolDefinitions` before the provider request. The working Piggyback path deliberately appends generated definitions; the native path does not. Thus this is not merely uncertainty about model attention: the producer charges a generated catalog entry absent from the native tool catalog. A truthful unavailability notice may still mention G in the system text and consume tokens. That notice is neither a callable offer nor the definition JSON whose estimated cost the meter records. [Native request][native], [working text-channel addition][working].

### Correct aggregation derives the wrong opportunity conclusion

`ProfileStore.NoteToolOffers` increments `Offers`, records generated origin, increments `OfferSessions` when the session key is new, and accumulates catalog/atom costs. `ToolLedgerFacts` exports these sums alongside zero calls. The stock `MinSessions` is 10. The policy derives `never_called` when calls equal zero and offer sessions meet the minimum; it does not consult channel availability. A generated origin then permits a `/retire` proposal. No duplicate or quality judgment is needed. [Aggregation][aggregate], [facts][facts], [defaults][defaults], [dead rule][dead], [proposal rule][classification].

Both live `/tool value` and the separate `nerd tools value` report consume the ledger. The latter loads it into a fresh kernel; the stored origin lets the generated row survive even though that reporting command constructs only a built-in catalog. Reopening therefore does not correct this evidence. [Live report][live-report], [standalone report][report], [origin fallback][facts].

## Bounded source-derived witness

Use a healthy workspace with an admitted generated-only tool G, a valid non-system user agent that explicitly declares G, normal tool-value metering, a native serving provider, and stock thresholds.

1. Begin with G's ledger absent or with zero calls and zero eligible offer sessions. Do not add a duplicate/harness verdict or independent dead reason.
2. Run that agent through normal spawn in ten distinct sequential process sessions. Boot, JIT/config derivation and persistence succeed. G remains registered and granted; no request restriction removes its grant before the meter.
3. Each task reaches compilation and an actual native model request. Let its normal response use only offered modular tools or answer without G. No failed provider call, cancellation, concurrency or model-generated malformed tool request is required.
4. Inspect each captured request: G is not a native definition. Inspect the compilation disposition: G is unavailable on that channel. Inspect the persisted ledger: G's offer/session figures nevertheless increase.
5. After the tenth distinct session, read the normal report. With calls still zero, the current rules derive `dead`, `never_called` and a generated retirement proposal.

If a prior actual G call exists, the zero-call predicate prevents this specific reason. If G is not granted, is absent from the generated registry, is shadowed by a modular tool, or is removed by restrictions before metering, this witness does not apply. A blank session ID or repeated work within one session does not meet the default threshold. A real Piggyback offer is a positive control, not this defect.

This does not prove that G deserved to be called on these tasks. It establishes that “the model was offered G in ten sessions but never selected it” is unsupported by the recorded route. A truthful unavailable/unobserved classification is different from declaring G valuable. An operator can still choose to retire G for legitimate reasons.

## Proposed contract

### Keep opportunity separate from authority and cost

Use the same channel-resolved offer artifact for request construction and opportunity measurement. Preserve the explicit unavailable disposition and native-schema limitation; do not repair the numbers by offering an untyped generated tool natively or widening any grant.

Define the unit deliberately. Current policy counts **compiles** and distinct sessions, not every provider retry. A narrow repair can retain that unit while recording an offer only when that compile supplies at least one compatible offered request for the tool. A more detailed per-request ledger can be useful, but silently switching counters from compiles to requests would change their meaning and historical comparability.

Distinguish:

- policy grant and registered backend;
- channel-compatible/temporarily withheld opportunity;
- prepared versus dispatched request;
- estimated catalog/teaching cost versus provider-reported billing;
- actual calls and their independent outcome evidence.

“Dispatched” means handed to the provider boundary, not proof that the model attended to a definition. State how canceled preparation and rejected requests count. Do not treat an unavailable notice as evidence of an opportunity to select the tool.

A shared immutable offer receipt can carry compile/request identity, session, serving channel, tool/backend identity, offered names, exclusions and measured projection metadata. Avoid a second full prompt copy. Capture the actual serving client after routing/profile selection rather than inferring the channel from a global default. Preserve modular shadowing, restrictions, current regime and fallback behavior.

### Preserve historical truth during rollout

Legacy totals cannot reliably distinguish a real text-channel offer from this native-only union: the persisted offer record has no channel provenance. Do not subtract every generated offer, infer exposure from zero calls, or erase genuine call/quality history.

Version opportunity evidence or introduce a new qualified counter while retaining old totals as historical/unqualified. Policy can require qualified opportunity sessions for new `never_called` proposals, with an explicit conservative migration rule. Preserve other independent retirement reasons and real call history. A reset or uncertainty label for the affected opportunity metric must not resurrect retired artifacts or negate a legitimate judgment.

Keep the existing session-level deduplication semantics. If receipts add idempotency, bind it to the chosen compile/request unit and tool identity so a retried publication neither adds phantom exposure nor loses a distinct valid opportunity. The existing file-lock/atomic-publication controls remain applicable; changing the producer must not regress their ordering.

### Separate a narrow repair from broader accounting redesign

The minimum fix aligns generated-offer inclusion with the serving channel and uses that qualified set for the existing compile meter. Exact Piggyback rendered-token accounting and per-request retransmission costs are broader issues already discussed in the prior audit; they should not be prerequisites for correcting the false opportunity. Likewise, counter scope by model/task relevance is a possible policy improvement, not necessary to establish this witness.

## Costs and fair evaluation

A resolved-offer artifact adds metadata, lifecycle ownership and schema/version migration. Reusing definitions avoids another registry pass, but concurrency and late routing changes need a clear snapshot contract. Per-request persistence could multiply full profile-file read/merge/rewrite costs; preserving compile units or batching compatible telemetry is cheaper, with durability/drop behavior made explicit. Session/receipt deduplication has memory and retention costs. No extra model call is inherently needed.

A conservative migration may postpone otherwise valid `never_called` proposals until enough qualified opportunities accumulate. More precise accounting can change reports without improving task outcomes. Avoid claiming saved tokens from merely lowering an estimate, or counting fewer proposals as inherently better.

Recommended evaluations, **not run**:

- Drive normal CLI/TUI spawn, boot-restored G, explicit user-agent grant, actual `ProcessWithIntent`, captured native requests, persisted/reopened ledger and real Mangle report. Use ten ordinary distinct sessions for the default-threshold case; a smaller configured threshold is supplemental.
- Pair the identical workspace and tasks with a Piggyback client that actually receives G's catalog entry. Genuine offers must remain counted; the model need not call G for that positive control.
- Test ungranted G, no backend, modular-name collision, restriction withholding, one session with many compiles, blank session identity and an existing actual G call. Preserve current refusals and deduplication.
- Exercise cloned inline and isolated subagent routes, routed serving profiles, retry/fallback channels and shutdown. A test that calls only `noteToolOffers` cannot prove request parity.
- Read the earlier offer tests and meter tests together: they separately pin native exclusion, Piggyback inclusion and shared-atom accounting. Add a combined test comparing the actual offered set to the corresponding ledger event; do not weaken the native exclusion assertion to make accounting pass. [Offer tests][offer-tests], [meter tests][meter-tests].
- Keep another real dead reason present in a control fixture. Correcting false opportunity must not erase valid redundancy, quality or cost evidence. A report or proposal remains non-destructive; refused/no-host approval must still leave G intact.
- Compare legacy migration, repeated publication, process reopen and successful cross-process telemetry writes. Reopened valid history must remain available; unqualified history must not be relabeled as verified opportunity.
- Hold task set, native/text-channel configuration, policy thresholds and source snapshot fixed. Measure opportunity precision, classification/proposal precision, actual tool selection, task quality, total provider spend, foreground latency, ledger I/O, lock waits and retained receipt size. Include tools genuinely never called despite compatible offers so the baseline's intended retirement pressure retains credit.

Acceptance is not “keep every unused tool.” It is that non-use policy rests on opportunities the host actually supplied, with cost and uncertainty labeled honestly and all existing authorization and approval boundaries intact.

[meter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/tool_meter.go#L57-L126
[offer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/tool_offer.go#L15-L118
[dead]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/tool_value.mg#L132-L170
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/tool_value.go#L16-L63
[retire]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L131-L222
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md#L303-L335
[branch]: https://github.com/theRebelliousNerd/codenerd/compare/388205138ba6d47c1cff0c5e97c896dfa642a391...eb50ab1d51fc131a4d9af49f76f5aea62c855832
[substitute]: https://github.com/theRebelliousNerd/codenerd/blob/eb50ab1d51fc131a4d9af49f76f5aea62c855832/Docs/audits/tool-duplicate-substitute-retirement.md
[feedback-audit]: https://github.com/theRebelliousNerd/codenerd/blob/30684dc18e934f12e50db05035febe7692174692/Docs/audits/generated-feedback-jev-accounting-idempotency.md
[grants]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2270-L2368
[grant-check]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2420-L2486
[envelope]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/intent_routing_rules.mg#L509-L522
[availability]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L1834-L1910
[native]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L1941-L2008
[dedup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger.go#L268-L285
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger.go#L104-L188
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_facts.go#L55-L108
[classification]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/tool_value.mg#L209-L267
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_spawn.go#L131-L183
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L538-L576
[task]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L358-L449
[clone]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L690-L746
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/spawner.go#L530-L548
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2629-L2652
[session]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/session_log_boot.go#L12-L26
[meter-site]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L1562-L1604
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/types/tool_value.go#L40-L56
[working]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L957-L964
[aggregate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger.go#L191-L216
[live-report]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L40-L74
[report]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_tools_value.go#L63-L100
[offer-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/tool_offer_test.go#L31-L116
[meter-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/tool_meter_test.go#L129-L207


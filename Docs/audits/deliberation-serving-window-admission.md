---
doc-class: deep-dive
subsystem: deliberation-context-admission
implementation-status: planned
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: ea49784163dadf2c0472ea20b143d21fdcfa8f54
supersedes: []
---

# Fit deliberation evidence to the serving client's input window

## Finding and decision

A successful read inside a deliberation role can fit its configured byte bound but make its next model request exceed the serving client's configured input-token cap. The role sizes only its encoded frame and observations. The client measures the complete request, including the system prompt, transcript framing and tool catalog. The latter correctly refuses the oversized request before sending it; the role returns that error and the conductor saves the thread as failed. It does not first make a smaller, truthful evidence view or close evidence within the client's available window. [Role sizing][agent] [Complete-request measure][measure] [Client refusal][window] [Failure transition][failure]

This is a **source-supported composition gap with an unrun conditional liveness consequence**, not a demonstrated provider context overrun, false ready decision, permission bypass or measured production failure. The client backstop is working as intended. The proposed correction is to make the existing role fitter use the same effective input-window contract as its serving client, while preserving the separate byte limit and all read, proof and execution boundaries.

The main witness stays inside one role invocation in the first round. It needs one successful role-local read, not a conductor probe, a large previous round, a human answer, a closing-note append or a malformed model reply. Both requests satisfy the deliberation payload-byte bound. Their complete measured request sizes differ enough that only the first fits the client's cap.

### Evidence boundary

Public main resolved to [ea49784163dadf2c0472ea20b143d21fdcfa8f54][pin] on November 3, 2026. Main was rechecked unchanged at approximately 19:18 UTC. Repository links use that immutable pin. The inspection covered root and scoped guidance, the requirements-interrogator skill and JIT atoms, the session specification, production role/mission wiring, sizing and client code, related audits and authored tests.

No repository program, build, test, model inference, benchmark, gate or failure injection was run. No repository file, implementation, credential, configuration, permission or setting was changed. Local files are research artifacts only. The fixtures, proposed contract and evaluations below are unexecuted.

## 1. Why this matters to Socratic requirements work

The repository assigns creative design to models and executive control to Mangle. It requires bounded, recoverable context and pushes domain evidence into the reasoning process when needed. The proposer is told to use workspace evidence; the challenger must test candidates against the complete request rather than fabricate success to fit a budget. A correctly admitted read has little value if its next consuming request is predictably refused by a different, already exposed bound. [Root guidance][guidance] [Architecture requirements][architecture] [Role contracts][proposer] [Interrogator contract][challenger]

The requirements-interrogator skill treats context budgeting, lifecycle failure, resource bounds and falsifying acceptance gates as relevant interrogation dimensions. That skill is guidance, not proof that its Markdown protocol runs inside the current shard. The shipped runtime uses the separately inspected JSON/JIT contract. [Socratic skill][skill] [Shard execution][shard]

Preserve the current protections:

- Per-role read-count and encoded-payload bounds.
- An offered, guarded read catalog rather than arbitrary shell access.
- Visible truncation and explicit unresolved outcomes.
- JIT-authored proposer/challenger behavior and policy-derived candidate decisions.
- Client-side complete-request admission.
- Design-only command behavior and separate mission/execution authorization.

The session's deliberation specification literally describes `deliberation_context_bytes` as a role-input and individual-output bound. This report does not redefine that existing setting as a promise about full provider tokenization. It identifies the missing composition with a second implemented limit, `max_input_tokens`, and proposes an additional contract. [Current specification][spec]

## 2. Exact boundary and production reachability

### 2.1 The role's fitter knows only the payload byte limit

`Agent` compiles a system prompt, collects tool definitions, then builds a payload containing the frame and role-local observations. Initial admission requires that encoded payload plus `terminalReserve` fit `ReadTools.ContextBytes`. The loop independently checks encoded payload length, then calls:

`client.CompleteWithTools(ctx, system, string(input), offer)`

The system prompt and offered definitions are arguments to the model call, but they are not inputs to those byte checks. Nor does this function read `InputWindowReporter`, call `broker.InputWindow`, or use `prompt.EstimateRequestTokens`. [Role implementation][agent]

For a returned read call, the same function calculates room from `ContextBytes`, current encoded payload length and framing allowances. After execution it shrinks output until the JSON-encoded payload fits that byte allowance. This is a real safeguard against output and escaping growth. It is not sizing against the complete request's token cap. [Local read fitting][fit]

The read adapter separately limits an individual successful output and passes the call through the registry's guards. That permission/byte boundary is preserved in the witness; the concern begins after a permitted read succeeds. [Read adapter][read]

### 2.2 The client has a different, explicit measurement contract

`types.InputWindowReporter` defines a cap in the units counted by the shared request estimator: system prompt, transcript and tool catalog. Decorators need not forward the interface; `broker.InputWindow` walks the client chain and selects its smallest positive reported window. Zero means no reported cap, not proof of unlimited provider capacity. [Interface][interface] [Chain lookup][chain]

`EstimateRequestTokens` sums the system estimate, JSON-encoded transcript estimate and JSON-encoded tool catalog estimate, with additional handling for native blocks. `admitInputWindow` uses that measure and returns `ErrInputWindowExceeded` before sending when it exceeds the configured window. This is the repository's approximate admission ruler, not an exact provider-tokenizer guarantee. [Estimator][measure] [Admission][window]

The inspected OpenAI, OpenAI-compatible and Anthropic `CompleteWithTools` paths perform this admission. Anthropic additionally charges configured server-tool declarations; the minimal witness does not need server tools. [OpenAI][openai] [Compatible client][compat] [Anthropic entry][anthropic-entry] [Anthropic admission][anthropic]

### 2.3 This is not repaired by the ordinary session fitter

The normal session executor obtains `broker.InputWindow(client)`, lowers its working budget to that cap where applicable, and measures system, messages and definitions together. It can replace oversized results with recoverable observation references. Those are useful existing mechanisms and a strong implementation reference. [Session window][session-init] [Budget intersection][session-budget] [Working request fitting][session-fit] [Shared measurement][session-measure]

Deliberation's registered ephemeral roles do not pass through that working-request function. Registration configures their byte limit; their `Execute` method directly invokes `deliberation.Agent`. The chat conductor marshals a frame and spawns the registered role. [Registration][registration] [Direct role execution][shard] [Chat factory][chat]

A broker wrapper also does not silently solve this input-fitting problem. Its `CompleteWithTools` forwards the same user payload and catalog. The inspected compression path retries **output truncation**, adding a system instruction; it does not shorten an over-window input. Broker admission may refuse earlier under its own policy. The witness requires that any such upstream admission permit the first request and not independently provide a fitting transform. An earlier truthful refusal remains a valid guard, not evidence that the role and client budgets are joined. [Broker forwarding][broker] [Compression scope][compression]

### 2.4 The mismatch becomes a failed design attempt

A nonnil client error returns immediately from `Agent`. The collector returns a role error instead of a completed round. `Service.Run` then sets `Thread.Status = "failed"` and attempts to save it. No smaller payload is constructed at this boundary and no candidate is declared ready. [Role error][agent] [Collection error][collect] [Conductor failure][failure]

The design-only `/deliberate` route is sufficient to expose this failure. The recurse-mission constructor also uses the same service and registered roles; mission preparation records and returns a deliberation error. Mission write scopes, source checks, gates and promotion remain separate. No bypass of them is alleged. [Design command][command] [Mission constructor][mission-factory] [Mission preparation][mission]

## 3. Minimal source-derived witness, not executed

Use a healthy, ordinary first-round proposer invocation with successful JIT compilation, available proof/storage infrastructure, a permitted read-only local tool, no consultation, no prior round and no human pause. Use the default 65,536-byte role bound and at least two remaining tool calls. Keep hosted tools and output-truncation retries out of this fixture.

Let:

- `B` be the role's encoded-payload byte limit.
- `S` be the actual compiled system string.
- `D` be the actual offered definitions.
- `P0` be the initial encoded payload.
- `E(S,P,D)` be the existing complete-request estimator applied to a single user message containing `P`.

Construct the fixture so that `len(P0) + terminalReserve < B` with ample room for an ordinary 8 KiB ASCII observation. Set the **fixture client's** positive input cap to `W = E(S,P0,D) + 1024`. This is an unrun test construction, not a proposed change to the user's settings or a claim about the current live slot.

The scripted first endpoint response asks for exactly one admitted local read. Its successful result contains at least 8 KiB of ordinary low-escaping ASCII content, and is small enough that the existing role-local byte fitter preserves it. A meaningful contract section plus a long example can supply this material; no malicious content is needed. The second model response, if reached, is scripted as a valid proposal.

| Step | Expected source-derived behavior |
|---|---|
| Initial role admission | `P0` fits the role's byte bound. |
| First complete request | `E(S,P0,D) < W`; the client permits sending it. |
| One read | Registry permits execution; the output fits the individual and remaining payload-byte allowances. |
| New payload `P1` | It remains below `B`, but adding at least 8 KiB of ordinary text increases the shared bytes-based estimate by more than 1024 units. |
| Second role boundary | Unspent read allowance keeps the same catalog `D` offered. The role's own byte check succeeds. |
| Client admission | `E(S,P1,D) > W`; the client refuses before a second inference send. |
| Service result | The first round is not completed; the error propagates and the thread is saved as failed. |

The assertion about the estimator is limited to this deliberately low-escaping ASCII construction and its shared local measure. A future fixture must calculate the actual serialized requests, including framing, rather than assume raw source bytes equal billed tokens. No exact fixture size or runtime measurement was computed in this audit. [Estimator][measure] [Token estimate formula][estimate-formula] [Role algorithm][fit] [Client contract][window]

A matched control gives the same client a sufficiently large cap while keeping the role byte bound, request, tool result and replies unchanged. Another control reduces the observation enough to fit `W`. Both should reach the scripted second response. These distinguish the cross-limit mismatch from malformed output, insufficient permission, missing tools or an intrinsically impossible baseline.

The first request and read have already occurred before the refusal. The rejected second request itself is not a second provider inference charge. In the simple fresh-thread schedule, successful storage performs the initial running save and the failed-thread save, with no completed-round append. Role-local observations are not a completed round; separate tracing may retain them, so this report does not claim all diagnostic evidence is lost. [Save order][service] [Round publication][collect] [Persistence][persistence]

## 4. Nonduplication

A bounded screen covered the pinned audit inventory/queue, neighboring reports and repository searches for deliberation with `max_input_tokens`, `InputWindowReporter` and `EstimateRequestTokens`. No exact predecessor for this join was identified. This is not a claim of exhaustive historical novelty. [Audit queue][queue]

| Related work | Distinct boundary here |
|---|---|
| [Probe frame capacity][prior-frame] | That failure combines conductor-probe results into the next round's initial frame, exceeding the **same** byte bound. Here the first frame fits, one role-local read fits the byte budget, and a **different complete-request cap** rejects the next call within the same role. |
| [Terminal budget][prior-terminal] and [terminal continuation][prior-continuation] | Those concern closing-note capacity and a copied slice-header sizing error. No closing note, empty reply or exhausted tool budget is needed here. |
| [Anthropic native context admission][prior-native] | That audit concerns native replay/server-tool representation and provider continuations. Its as-built safeguards are credited. This witness uses ordinary text and custom definitions, with the client measuring them correctly. |
| [Read invocation identity][prior-identity] | That concern is attribution of an observation to its arguments. Keeping exact arguments does not intersect byte and client-window admission. |
| [Portable recurse deliberation][prior-portable] | That broader proposal calls for bounded, evidence-bearing design work. This report supplies one narrower implemented integration failure, not another overall framework. |

The repair may reuse shared context infrastructure and should be coordinated with the frame-capacity work. Shared code is not a reason to collapse independently reproducible boundaries or claim one fix covers both without the corresponding fixtures.

## 5. Proposed contract, not implemented

Update the owning session/deliberation and broker/client contracts before implementation.

1. **Resolve the serving boundary.** Before compiling/admitting a role, obtain the actual serving client's applicable input cap through the established wrapper-aware contract. If routing can change the client, bind the measurement to the selected client or require readmission after routing. Do not assume the global model, an outer decorator or an earlier request's client is the current one.
2. **Retain both limits and their units.** The encoded role payload must fit `B`; the complete request must also fit its configured input window under the same ruler the client uses. Do not replace the independent byte guard with an unchecked four-bytes-per-token conversion.
3. **Count every required request component.** Charge JIT system text, serialized transcript framing, offered tool definitions and applicable provider additions. When provider additions cannot be known here, preserve a conservative, explicit limitation and the downstream backstop. Handle broker-added retry instructions and output reserve under their owning contracts.
4. **Admit an answer path, not just a read.** Reserve room for required evidence-closing and answer-required controls in the final encoded request shape. Before accepting another observation, establish that a truthful consuming request can still be formed. If withholding tools is used to reclaim catalog space, remeasure that exact tool-less request and disclose evidence closure.
5. **Select context without falsifying evidence.** Let host measurements supply available capacity, source identity and observation sizes; let Mangle own disclosure/retention policy. Use existing bounded archives or an explicit partial view with a genuinely resolvable reference. Never silently drop a critical requirement or an adverse observation while retaining its favorable conclusion.
6. **Separate impossible baselines from recoverable growth.** If mandatory instructions, request and minimal status cannot fit, return a named unresolved-capacity result before needless reads. If a read grows the request beyond capacity, preserve its truthful disposition and fit or stop explicitly. Do not relabel the tool as failed merely because its result is too large.
7. **Preserve authority.** No automatic cap increase, provider/model switch, paid expansion, permission relaxation or arbitrary tool-catalog widening is required. A narrower context view is not permission to omit the user's constraints.

No new public API or predicate name is prescribed here. Reuse the existing `InputWindowReporter`, client-chain traversal, complete-request measurement and context-policy mechanisms where their contracts apply. Reusing the session archive also requires verifying that its handles are actually reachable by a deliberation role; a string saying “recall” is not a working continuation.

### Failure modes a repair must address

- **Wrapper blindness:** a bare type assertion misses an inner capped client.
- **Different serving client:** a routed or fallback client has a smaller cap than the measured one.
- **Catalog drift:** fitting against a catalog other than the one sent invalidates admission.
- **Overhead growth:** escaping, notes, provider additions or broker instructions invalidate a raw-output estimate.
- **Overaggressive compaction:** an input fits because a binding constraint or critical counterexample disappeared.
- **Reference without recovery:** a compact placeholder points to evidence the role cannot retrieve.
- **Repeated refitting without progress:** the agent repeatedly executes or re-fetches the same read because its complete observation never fits.
- **Unavoidable small window:** even a minimal truthful frame cannot fit. This must remain a valid unresolved outcome, not fabricated readiness.
- **Estimator uncertainty:** passing the repository's local cap does not certify exact provider tokenization or effective model use of the context.

## 6. Costs and tradeoffs

The current failure path spends at least the first model request, one tool execution, one JIT compilation and catalog construction, plus kernel admission/progress and persistence work before the refused request. It can also strand already paid challenger or earlier-round work in other schedules. The actual frequency, usage, dollar cost and recovery behavior are unknown.

The proposed deterministic work adds client-chain traversal, complete-request measurement and possibly repeated serialization. A straightforward full remeasurement after each retained observation costs work proportional to the request sizes scanned, `sum_i O(|S_i| + |P_i| + |D_i|)`, not merely the final payload size. Incremental accounting can reduce this work only if it preserves exact representation identity and is verified against the final assembled request.

Archive-backed views add storage, hashing, synchronization, retrieval latency and lifecycle/migration obligations. Smaller views may require more targeted reads or model requests, and closing tools sooner can reduce evidence coverage. Conversely, they may avoid failed attempts and whole-role restarts. Neither net savings nor better requirements quality has been measured.

Full evaluation must include all proposer/challenger requests, prompt-cache usage, output/reasoning tokens where reported, tool work, consultations and nested requests when present, broker/provider retries, count endpoints if used, serialization CPU, peak memory, bytes written/read, proof work, latency, operator recovery and abandoned runs. Count rejected requests separately from actual inference sends and billable usage. “Two roles” is not a total-call bound, and the payload-byte cap is not a total-dollar budget. [Role loop][agent] [Broker retries][compression] [Service lifecycle][service]

## 7. Fair evaluation plan, all unrun

### Deterministic contract fixtures

Start with request capture and a synthetic transport, not live inference. Use the real registered role, JIT compiler, registry, client cap and conductor where possible.

- Reproduce the one-read, two-request witness and require the repair to produce either an admitted truthful continuation or a specific unresolved-capacity outcome.
- Cover exact cap, one local-estimator unit over, uncapped clients and a baseline that cannot fit.
- Vary system and catalog size independently of payload size. An unchanged payload must not hide growing overhead.
- Repeat through tracing, broker and credential-pool wrappers; exercise routed-client changes where supported.
- Include a tool-less closing request, error/large-metadata observations, JSON escaping, Unicode, multiple tool calls and output-retry instructions.
- Preserve read count, default-deny permissions, cancellation and design-only behavior.
- Capture the exact complete requests and assert both byte and effective client-cap contracts on every actual send.
- Require zero duplicate read effects for fitting-only recovery and truthful provenance for all omitted/archived content.
- Keep the existing next-round frame-capacity and closing-note fixtures as separate regression controls.

Current role tests use a scripted client that records the user string but discards the system argument; their byte assertions do not exercise a configured client window. The existing client tests verify refusal across request surfaces, while session tests exercise complete-request fitting. These are valuable definitions to compose, not proof that the deliberation composition has passed. No tests were run for this audit. [Role tests][tests-role] [Client tests][tests-client] [Session tests][tests-session]

### Requirements-quality and portability study

After deterministic admission passes, compare the pinned baseline and candidate repair on matched design tasks with identical source snapshots, original constraints, provider/model/effort, tool catalog, JIT sources and **full-cost** budgets. Include both small easy requests and evidence-heavy tasks, Go and foreign-language repositories, and several configured windows. Use the ordinary production path with CodeDOM/world-model context; restricted synthetic tools are only boundary evidence.

Judge whether the final design actually preserves the request, identifies critical blockers, and honestly distinguishes supported, unresolved and impossible cases. Assessors should inspect artifacts/evidence rather than reward polished prose or mere JSON completion. Track unsupported-ready rate, useful design completion, truthful unresolved rate, window refusals, needless repeated reads, total cost and latency. Include failed/abandoned runs and cluster uncertainty by task/repository family. Do not discard infeasible windows or make the treatment win by increasing them.

A larger context is not automatically a better design assessment. ReAct studies motivate evaluating whether acquired observations can support subsequent reasoning; they do not prove this particular fitter correct. Lost in the Middle reports position-sensitive evidence use on its evaluated tasks/models, motivating quality checks after compaction, not an inference that current codeNERD models have identical failure rates. [ReAct](https://arxiv.org/abs/2210.03629) [Lost in the Middle](https://arxiv.org/abs/2307.03172)

Provider documentation separately treats system prompts and tools as part of token-count inputs. That supports the complete-request boundary, but does not make codeNERD's local approximation an exact provider count or authorize using a counting endpoint in this audit. [Anthropic token-counting documentation](https://platform.claude.com/docs/en/build-with-claude/token-counting)

## 8. Checkable exit

Close this gap only when a named specification joins the role-byte and serving-client-window contracts, the production role honors both through evidence growth and closure, the capped one-read witness no longer becomes an avoidable generic failure, and tests preserve impossible-baseline refusal plus original constraints and tool authority. Record executed verification separately from this static report. Increasing the cap or citing the working client refusal is not the missing implementation.

## Pinned source references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/ea49784163dadf2c0472ea20b143d21fdcfa8f54
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/agents.md#L5-L32
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/architecture/AGENTS.md#L93-L102
[skill]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/.agents/skills/requirements-interrogator/SKILL.md
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/prompt/atoms/system/deliberation_proposer.yaml#L13-L52
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/prompt/atoms/system/requirements_interrogator.yaml#L13-L51
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L898
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/read_tools.go#L86-L208
[fit]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/read_tools.go#L164-L205
[read]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/read_tools.go#L57-L81
[interface]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/types/interfaces.go#L369-L382
[chain]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/broker/input_window.go#L5-L26
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/prompt/request_tokens.go#L10-L40
[window]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/perception/input_window.go#L11-L59
[openai]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/perception/client_openai.go#L368-L380
[compat]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/perception/client_openai_compat.go#L958-L985
[anthropic]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/perception/client_anthropic.go#L85-L105
[session-init]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/session/working_context.go#L217-L237
[session-budget]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/session/working_context.go#L661-L695
[session-fit]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/session/working_context.go#L577-L634
[session-measure]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/session/working_context.go#L964-L975
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/shards/registration.go#L837-L849
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/shards/deliberation.go#L40-L64
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/cmd/nerd/chat/deliberation.go#L18-L50
[command]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/cmd/nerd/chat/deliberation.go#L96-L119
[broker]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/broker/broker.go#L419-L437
[compression]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/broker/compression.go#L50-L111
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/round.go#L10-L72
[failure]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/service.go#L35-L53
[service]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/service.go#L20-L175
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/persistence.go#L23-L55
[mission-factory]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/cmd/nerd/cmd_campaign_mission.go#L40-L68
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/campaign/recurse_mission.go#L244-L264
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/audits/ASTRA_QUEUE.md
[prior-frame]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/audits/deliberation-probe-frame-capacity.md
[prior-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/audits/deliberation-terminal-budget.md
[prior-continuation]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/audits/deliberation-terminal-budget-continuation.md
[prior-native]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/audits/anthropic-native-context-admission.md
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/audits/deliberation-read-invocation-identity.md
[prior-portable]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/Docs/audits/portable-recurse-deliberation-crucible.md
[tests-role]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/deliberation/agent_test.go#L14-L105
[tests-client]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/perception/input_window_test.go#L98-L168
[tests-session]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/session/working_context_test.go#L163-L191


[anthropic-entry]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/perception/client_anthropic.go#L322-L334

[estimate-formula]: https://github.com/theRebelliousNerd/codenerd/blob/ea49784163dadf2c0472ea20b143d21fdcfa8f54/internal/prompt/atoms.go#L287-L296

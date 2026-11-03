---
doc-class: deep-dive
subsystem: deliberation
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: 608ac7bec36cf66ecf2718ced8a438ee30b9dae6
supersedes: []
---

# Successful probes can exceed their next deliberation frame

## Finding and decision

The conductor admits critical-data probes by call count and limits each successful result separately. It then saves every result in the round. The next round places that entire record into `Frame.Previous`, while the registered role applies the same configured byte limit to its encoded role payload. Two individually small, successful observations can therefore make the next proposer fail before its first `CompleteWithTools` request for that round, even with unused rounds and tool allowance. The evidence is retained on disk, but cannot enter its intended consumer through this path. [Probe admission][probes] [Read threshold][read] [Replay][round] [Consumer admission][agent-admission]

**Classification: a source-supported producer/consumer capacity mismatch, with an unrun conditional failure witness.** This is not a demonstrated production outage, an incorrect ready decision, lost durable data, or a permission bypass. Rejecting an oversized role frame is correct. The missing property is that the conductor's admitted evidence must compose into a usable bounded continuation, or receive a specific unresolved-capacity disposition before the ordinary refinement path is attempted.

Strengthen the existing Socratic loop, not the number of agents. Separate a complete durable round from its policy-selected model view. Preserve the request, critical objections and exact evidence identity; use an explicitly partial, recoverable view when the complete observations do not fit. If no truthful minimum view fits, stop with that named limitation. Do not increase the byte bound, silently discard requirements, or turn a successful read into evidence that a design passed.

### Source and execution boundary

The initial public main observation was `faa613df416495fa0f1188edf97d59ad4223e831`. The final check at approximately 14:47 UTC on November 3, 2026 resolved to **608ac7bec36cf66ecf2718ced8a438ee30b9dae6**, committed at 14:43:39 UTC. The complete one-commit comparison changes only `Docs/audits/campaign-reproduction-outcome-contract.md`, adding an unrelated implementation-status update; every source cited here is unchanged. Repository links below use the final immutable pin. Source snapshots were checked against the initial Git tree's blob identities, with the comparison establishing their applicability at the final pin. [Comparison][comparison] The review read public guidance, architecture, relevant implementation, authored tests, the audit inventory and neighboring reports. [Source pin][pin]

No repository program, build, test, model inference, benchmark, gate or failure injection ran. No repository document, implementation, credential, permission, configuration or setting changed. The synthetic directories and outputs below are proposed fixtures, not created workspaces or executed reproductions. Proposed mechanisms, tests, benefits and costs remain unmeasured.

## 1. Why this matters to requirements interrogation

codeNERD already has the intended creative structure: a proposer supplies distinct readings and success conditions; the requirements interrogator challenges them against the complete request and workspace evidence; Mangle determines the disposition. Critical data items name the exact offered read that could settle them. A successful empirical step is useful only if its observation can be considered in the next design assessment. [Role contracts][proposer] [Interrogator][interrogator] [Decision policy][policy]

The repository's architectural requirement is stronger than merely placing a cap on each object: working context must be bounded, revision-aware and recoverable, with executive context selection owned by the kernel. The current session specification says measured probes feed subsequent frames and that only the last completed round is replayed under a configured byte bound. A single round is a count bound, not a size bound. Its several observations, proposals, assessments and proof can exceed the recipient's capacity. [Root guidance][guidance] [Architecture guidance][architecture] [Current session contract][spec]

This matters when investigating a foreign repository with a larger namespace or more verbose permitted tools. The witness uses only ordinary ASCII filenames and the shared host protocol; it depends on no programming-language parser, package manager or test runner. That supports a language-independent finding, not a claim of verified portability across every operating system or adapter.

## 2. Implemented safeguards and the compositional gap

| Boundary | Implemented at the pin | Remaining limit |
|---|---|---|
| Read admission | Offered-tool catalog plus registry argument, allowlist, workspace and effect guards. | Authorized execution does not reserve input space for its result. |
| Individual result | `ReadTools.Execute` clips long successful text at `ContextBytes`, then appends a visible marker. | This is a per-result threshold, not aggregate encoded-frame admission; the marker itself is additional bytes. |
| Conductor probe loop | Checks the call counter, executes each selected critical data item, saves output/error and asserts a digest receipt. | It does not account for prior probe output or the next role's mandatory frame. |
| Durable round | Retains proposal, challenge, probe records and proof. | Useful audit detail is also copied wholesale into the next model frame. |
| Role admission | Rejects the encoded frame unless it leaves `terminalReserve` within the configured bound. | This correct check happens after conductor reads and persistence. |
| Role-local reads | Fit output against the remaining encoded payload and expose cuts. | That fitter applies to the role's own new `Observations`, not inherited `Frame.Previous.Probes`. |
| Failure | Service returns the error and saves the thread as failed. | No smaller previous-round view or evidence-capacity continuation is constructed here. |

Sources: [Guarded read][read], [registry][registry], [probe loop][probes], [record shape][types], [frame collector][round], [role admission and fitting][agent], [failure transition][failure]. These controls remain necessary; the proposed correction must compose them rather than remove a guard.

### 2.1 The production adapters share the bound

Both the chat deliberator and recurse-mission constructor use the routing context-byte value for their `ReadTools` probe adapter. Their `RunShard` callbacks JSON-marshal the complete frame and pass it to the shard manager. The deliberation shard invokes the shared `Agent` with its configured read adapter. The default routing value is 65,536 bytes, with four rounds and six calls for each budgeted counter. [Chat construction][chat] [Mission construction][mission-adapter] [Registered role][shard] [Defaults][config]

The factory configures both registered roles from the routing context bound. The shard manager and its queue preserve the task string through `agent.Execute`; the shortened task summary is for display, not a smaller input. [Registration][registration] [Dispatch][manager] [Queue retention][queue-store] [Queue dispatch][queue-run]

The claim is about this normal configuration, not a custom callback that deliberately changes the frame or a runtime configuration mismatch. A later implementation should still test mismatched and independently evolving producer/consumer limits rather than assume equality is sufficient.

### 2.2 The conductor does not fit a continuation view

`runProbes` starts a local call count at zero and checks `AllowCall` for each critical data item on the selected candidate. For an allowed call it invokes `s.Probe`, constructs a `ProbeResult`, appends that result to the saved round and writes the thread. It does not run the role-local fitting logic, remove older output, or replace the round with a bounded projection. The digest fact is additional evidence metadata, not a replacement for the inline output. [Probe loop][probes] [Call rule][policy]

The service finishes probing before computing its progress fingerprint and entering the next loop. With available round capacity and no stagnation stop, `collectRound` then assigns the complete preceding `Round` to `frame.Previous` and invokes the proposer. The collector also includes matching receipt facts. [Service order][service] [Collector][round]

`Frame.Previous` is a pointer to the ordinary JSON-serializable round type. `Round.Probes` is not omitted when nonempty. Consequently all saved output strings participate in the encoded consumer frame. [Record types][types]

### 2.3 Admission rejects before the role can narrow the evidence

`Agent` wraps the received task as its raw JSON frame, with an initially empty role-local observation list. It checks the serialized length plus `terminalReserve` before calling `CompleteWithTools`. An oversized inherited frame therefore cannot reach a model that might choose a narrower read or request a smaller view. JIT prompt compilation and catalog construction precede this check, so “no model request from this role loop” does not mean “no host preparation work.” [Consumer admission][agent-admission]

The output fitter and `addNote` helper work on the separate role-local observations slice. Even a repair of their previously audited behavior cannot shrink the inherited previous round. This audit does not propose making them mutate the original request or historical record. [Role-local fitting][agent-fitting]

## 3. Minimal unrun witness

Use a fresh design-only thread under the default routing settings, small request and brief, functioning JIT/kernel/proof support, a stable workspace, live context and successful persistence. No consultations, role-internal reads, values questions, malformed JSON or concurrent activity are needed.

### Permitted workspace observation

The hypothetical workspace has two admitted ordinary directories, `east` and `west`. Each contains 1,000 admitted nonhidden regular files. Every filename is 35 ASCII bytes: the prefix `item-`, 22 `x` characters, an index from `0000` through `0999`, and `.txt`. There are no other entries in those directories. These names are ordinary path components, not adversarial identifier sizes.

The actual nonrecursive `list_files` implementation collects admitted entry names and joins them with one newline between entries. Its result for each directory therefore contains **35,999 bytes**: 1,000 names of 35 bytes plus 999 separators. This is arithmetic from the source format, not an executed measurement. The registry passes the returned string through as `ToolResult.Result`. [Listing implementation][listing] [Registry return][registry]

The offered core read envelope includes `list_files`. Each result is below the 65,536-byte deliberation threshold, so `ReadTools.Execute` returns it without clipping or a truncation marker. Assume ordinary registry and workspace authorization succeeds; denied reads are not part of this witness. [Envelope][envelope] [Core catalog][catalog] [Read adapter][read]

### Round schedule

1. The first proposer emits two short, structurally valid candidates for a consolidation or namespace-design task. Candidate A is first. Its remaining factual questions require inventories of `east` and `west`; candidate B retains a critical design objection.
2. The challenger supplies one assessment per candidate and recommends A. A has exactly two critical data items with distinct stable keys and offered calls `list_files` for the two directories. It has no critical design or values item and no ready assessment. B remains design-blocked. All candidate briefs and success tests are nonblank.
3. Policy selects A as empirical. The service records the decision and proof, then dispatches the two probes. Both fall within the six-call allowance. Each output is 35,999 bytes, and both results are appended and saved. Neither has been truncated.
4. The first completed round has no repeated-transition stagnation history. Round two remains within the four-round limit, so the service attempts ordinary refinement.
5. The second proposer receives a frame containing the complete first round. The two output strings alone contribute **71,998 raw bytes**, before JSON field syntax, escaped newlines, request, candidates, objections, receipt facts, proof or closing-note reserve. Its encoded payload must therefore exceed 65,536 bytes.
6. `Agent` returns its frame-capacity error before issuing its first `CompleteWithTools` request for round two. `collectRound` propagates the error; `Service.Run` saves failed status. Both observations remain in the saved round. No post-probe assessment or ready design is produced by this schedule. [Validation][validation] [Eligibility][policy] [Dispatch and stop checks][service] [Admission][agent-admission] [Failure save][failure]

This shows a composition failure without any individually oversized read. Merely changing the clipping marker to fit inside the per-result threshold would not fix it. Neither would granting more rounds: the next frame is already too large.

### Additional boundaries, not additional proven incidents

- One output at the full threshold can also leave no room for frame overhead; a longer output is clipped to the threshold plus a marker. The two-subthreshold-result witness avoids relying on either edge case.
- Several shorter observations can overflow after JSON escaping, metadata or proof are included. Raw-byte totals alone are insufficient for precise admission. Go's JSON contract explicitly describes escaping and structural encoding. [JSON documentation][json]
- Large candidate text or consultation advice can increase the same frame. Those producers deserve compositional checks, but are not required to establish this finding and are not separately claimed as reproduced failures.
- Smaller probes, a deliberately narrowed read or a short complete previous round are valid counterexamples. The report does not claim every empirical round fails.
- A different failure, cancellation, no-progress stop or final-round stop can occur first. The witness deliberately excludes those cases and leaves review time available.

## 4. Distinctness from existing audits

The current audit inventory and relevant report bodies were screened. This is a bounded novelty check, not an assertion that nobody has discussed context composition elsewhere.

- **Terminal budget and terminal-budget continuation:** those reports concern closing-note capacity and a copied observation-slice header inside an already running role. The present producer is the conductor; the oversized object is the next role's immutable input frame. There is no next-role model response, `addNote` call or role-local read. The original terminal-budget report correctly treats an oversized initial frame as a refusal control. This audit supplies an upstream normal producer that can create that refused frame. [Terminal budget][prior-budget] [Continuation][prior-continuation]
- **Probe-consumption horizon:** that report already proposes the broader consumer-capacity contract. This is a concrete byte-capacity complement, not the first argument that probes need consumers. Its witness establishes collection on the final allowed round with no subsequent round available. Here round two is allowed and entered; the recipient rejects because its bytes do not fit. Reserving a future round alone cannot satisfy this capacity contract. [Horizon][prior-horizon]
- **Mixed-objection probe admission:** that report proposes investigation while a design objection remains. Here candidate A has only data blockers and its probes are dispatched by existing policy. Its recommendation to preserve complete encoded context limits is compatible with this narrower current producer/consumer defect. [Mixed objections][prior-mixed]
- **Read invocation identity and progress observation identity:** those reports concern subject attribution and stagnation equivalence. This witness preserves the calls in the challenge, obtains new results and does not stop for stagnation. Correct identities still need a deliverable view. [Read identity][prior-identity] [Progress identity][prior-progress]
- **Critical-item retirement, citation admission and selected-review handoff:** these govern stronger readiness evidence and what survives later adoption. This schedule fails before a new assessment, much less adoption. Bounded projection must retain their obligations rather than hide them to fit. [Retirement][prior-retirement] [Citation][prior-citation] [Handoff][prior-handoff]
- **Portable crucible:** the broader proposal already advocates bounded, recoverable evidence. This report makes one existing continuation boundary concrete; it does not claim a new general agent architecture. [Crucible][prior-crucible]

## 5. Proposed producer/consumer contract

Update the owning deliberation/session specification before implementation. The following states and interfaces are proposals, not existing APIs.

### Separate retention from presentation

Keep the complete round and its exact observations as the durable record. Build a separate model-facing view with:

- Original request, constraints, answer and candidate/success-condition identities.
- Explicit unresolved critical-item inventory and current disposition.
- Bound observation identity: owning thread/round/candidate/item, effective call, source validity and status.
- An encoded-size-admitted body or excerpt with truthful coverage.
- A usable reference for omitted content, if recovery is supported and authorized; otherwise an explicit unavailable/deferred limitation.

The next-role contract should guarantee one of three outcomes:

1. **Inline admitted:** the complete required view plus controls fits the actual consumer limit.
2. **Partial and recoverable:** the bounded view identifies what is missing and how to retrieve it within existing tool, context, time and mission allowances. Partial delivery does not count as complete assessment.
3. **Capacity unresolved:** even the minimum faithful view cannot fit or be recovered. Preserve the observations and blockers and report that result without attempting an inevitably invalid ordinary frame or manufacturing success.

This does not promise that every large observation can lead to a design under a finite budget. It makes the producer's disposition honest and the recoverable domain explicit.

### Ownership and admission

Go should measure encoded views, immutable source/observation identities, available retrieval mechanisms and actual resource use. Mangle should govern which evidence remains active, what can be deferred and whether refinement is admitted. JIT atoms should explain the resulting coverage state and require explicit unresolved items. Do not move semantic importance ranking or candidate selection into an unrecorded Go truncation heuristic.

Perform admission on the *actual assembled recipient role payload*, including the fixed request, previous-round view, current candidate list for the challenger, consultation material, receipts, JSON escaping and terminal controls. It is insufficient to divide the limit by the nominal number of probes or subtract only the raw output lengths. The next challenger's frame can be larger than the next proposer's; both need the contract.

Unknown output length need not prevent every read. Before executing, reserve a bounded result descriptor and an allowed overflow disposition. After execution, measure the resulting presentation and select inline, recoverable or unresolved handling. The host must not imply that it knew the precise output length beforehand. A tool-side bound or a count-only inventory can reduce work where it genuinely answers the admitted question, but cannot silently substitute an incomplete inventory for a complete one.

Avoid feeding the entire persisted proof tree back as compulsory model context merely because it shares the Round type. If proof details are omitted from the working view, retain the full proof and an exact reference for audit. This is a design option to evaluate, not a claim that proof size caused the primary witness or that every proof is dispensable to every downstream consumer.

### Recovery and safety constraints

- A digest alone is not a retrieval mechanism. A reference must resolve through an offered, scoped, read-only path and return enough context to interpret the result.
- Retrieval must remain possible when the original inline view is too large. Requiring the same oversized frame to request a smaller view would recreate the problem.
- Full storage is not full delivery, and delivery is not completed assessment. Record these states separately.
- Never drop the critical-item inventory, user's constraints, an error or a caveat to make the view appear complete. Never count an excerpt as an exhaustive absence result.
- Keep source validity distinct from capacity. A source reread after change is a new observation; it must not silently replace the historical evidence behind the prior challenge.
- Do not expose arbitrary snapshot paths, secrets or unrelated session content through a new lookup tool. Preserve owner/workspace boundaries and retention policy.
- A missing/expired reference, failed snapshot write or unavailable resolver produces an explicit unresolved disposition, not an empty successful observation.
- Retrying or resuming cannot reset spent mission allowances or extend authorization. The mission still owns source/vision admission, scope, gate and permitted effects. [Mission boundary][mission]

Bertrand Meyer's **Applying Design by Contract** distinguishes a supplier's postcondition from the precondition its caller must establish. Applied here, returning several locally admissible strings does not establish the next consumer's aggregate input precondition. This motivates a composed handoff contract, not a requirement to adopt Eiffel or add redundant checks everywhere. [Primary paper][meyer]

## 6. Full cost and failure accounting

The baseline already pays for the first proposer/challenger calls, their possible read loops, JIT, kernel queries/proofs, successful conductor reads, receipt hashing and several full-thread snapshot writes. It then may perform next-role prompt/catalog preparation before refusing. There is no next-role completion request in the primary schedule, so do not charge one as if it had run or claim savings by removing it.

A bounded view adds encoding/measurement work, metadata, projection logic and possibly retrieval. Inline reduction can lower subsequent prompt bytes, but a retrieval-based treatment may require more tool executions and model turns. It can also close evidence earlier, leave a legitimate unresolved design, or spend more to reach the same answer. No savings or quality improvement has been measured.

Measure at least:

- All provider requests, retries, input/output/reasoning usage where reported, cached/uncached pricing and unknown spend.
- Source reads, directory membership work, tool time, bytes consumed and bytes returned. Clipping after `Registry.Execute` does not avoid the already completed directory scan or construction of its result.
- JIT compilation, fact assertion/query, proof extraction, hashing, JSON passes, allocations and peak retained bytes.
- Every snapshot write, synchronization and retained artifact, plus reference lookup and retention/cleanup costs. `Save` serializes the complete thread; total persistence traffic is the sum of snapshot sizes at each save, not merely the final file size. [Save implementation][save]
- End-to-end elapsed time, owner effort, failed attempts, legitimate capacity deferrals, repeated acquisition and cost per independently acceptable design.

If K results each approach the per-result threshold B, the stored result bodies can approach K times B before framing and other round data. A one-round replay policy does not eliminate that aggregate. A reference scheme trades model-input bytes for storage, lookup and lifecycle complexity; those costs belong in the same comparison.

SEDA's original staged-service paper treats resource control at stage boundaries as an architectural concern. The limited analogy is that bounded components still need explicit treatment of traffic entering the next stage. Its web-server results do not predict codeNERD reliability, throughput or token savings, and this finding requires no event-queue framework. [Primary paper][seda]

## 7. Proposed evaluation, not run

### Deterministic composition checks

Start with scripted valid participant output, the real conductor/kernel/proof path and the actual registered Agent boundary. Capture probe calls, saved round contents, frames delivered to roles and provider-request counts. A test that substitutes a `RunShard` callback which ignores its frame would miss this defect.

| Fixture | Required observation |
|---|---|
| Two 35,999-byte successful listings, default limits | Baseline saves both; next proposer receives an over-bound frame and does not issue its first completion request. Treatment returns a truthful admitted view or named capacity disposition. |
| Same names with small inventories | Ordinary next-round review and all critical-item identities remain intact. |
| Exact encoded threshold, one byte below/above | Admission includes terminal controls and JSON structure, not just raw strings. |
| Quotes, backslashes, Unicode and escaped HTML characters | Actual encoded bytes govern; no invalid or silently altered excerpt. |
| One near-limit result and several shorter results | Single-result and aggregate cases have distinct receipts and truthful coverage. |
| Proposer view fits, challenger candidates/advice make its view larger | Both consumers enforce the same compositional contract. |
| Proof metadata large, evidence small | Working-view policy is explicit; durable proof remains available and authoritative boundaries unchanged. |
| Missing resolver, expired reference, changed source or denied retrieval | No stale/empty-success substitution; unresolved status names the actual cause. |
| Persistence failure or cancellation after one probe | Exact completed read retained where publication succeeded; no invented second result or unchecked retry. |
| Final allowed round | Preserve the separate review-horizon requirement; byte fit is not future-turn admission. |
| No-progress and repeated read | Bounded views do not mint fake progress or fresh budgets. |

The authored Agent tests cover role-local output fitting, spent budgets, refusal/nudge behavior and initial oversized-frame refusal. The inspected deliberation tests cover decoding, validation, choices, fingerprints, persistence and read schemas. These are useful controls, but their named fixtures do not establish conductor multi-probe composition through the next real Agent. They were read, not run; no repository-wide absence-of-tests claim is made. [Agent tests][agent-tests] [Deliberation tests][tests]

### Production-route qualification

After an authorized implementation exists, exercise both `/deliberate` or `/innovate` and bounded recurse-mission preparation through normal boot, with world-model and CodeDOM context enabled. Verify the exact routing values reach the conductor adapter and registered role; record the actual role payload rather than infer it from a saved snapshot. Include admission failure before provider dispatch and successful partial-view retrieval. No mission attempt should execute from an unresolved design.

The real directory fixture is a portability control, not a benchmark of language reasoning. Use ordinary ASCII names across supported filesystems; separately test platform path/encoding differences and different tool result shapes. Do not attribute a file-membership denial or a different tool's truncation behavior to this aggregate-frame finding.

### Fair outcome comparison

Compare pinned baseline, a conservative explicit-capacity-disposition treatment, and a bounded recoverable-view treatment. Hold task, source snapshot, model/provider, prompt corpus, permissions, relevant limits and independently reviewed success conditions constant. First use scripted outputs to isolate transport; only then conduct separately authorized live reasoning evaluation.

Report both quality at matched full-cost budgets and total cost to the same independently accepted design. Include small workspaces where the change should have little effect, large/verbose tool outputs, irreducibly large mandatory requirements, and cases where a partial inventory is insufficient. Keep failures, budget exhaustion and deliberate unresolved outcomes in the denominator. A lower error rate achieved by dropping objections or automatically declaring readiness is not an improvement.

Primary outcomes are complete/faithful evidence delivery, explicit unresolved coverage, supported design decisions and false readiness. Secondary outcomes include unnecessary rereads, late frame refusals, total work/cost and latency. Cluster uncertainty by repository/task family and report inconclusive samples. The diagnostic assertion is deterministic; the practical benefit and model behavior remain empirical questions.

## Acceptance boundary

The correction is complete only when normal conductor-produced evidence has an explicit bounded consumer disposition; every encoded role payload respects its configured bound; omitted material remains truthfully identified and recoverable where promised; critical requirements survive projection; and no added permission, budget or implementation-success claim is inferred from a read.

Until then, the supported conclusion is narrow: successful, individually subthreshold probes can make the existing next-round refinement frame inadmissible. The saved evidence and correct refusal are real protections, but they do not compose into the usable requirements interrogation the workflow intends.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/608ac7bec36cf66ecf2718ced8a438ee30b9dae6
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/agents.md#L19-L27
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/architecture/AGENTS.md#L49-L103
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L875
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/prompt/atoms/system/deliberation_proposer.yaml#L14-L54
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/prompt/atoms/system/requirements_interrogator.yaml#L14-L49
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/core/defaults/policy/deliberation.mg#L13-L60
[read]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/read_tools.go#L57-L81
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/tools/registry.go#L326-L465
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/outcome.go#L138-L201
[types]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/types.go#L55-L122
[round]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/round.go#L10-L73
[agent]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/read_tools.go#L84-L212
[agent-admission]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/read_tools.go#L86-L139
[agent-fitting]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/read_tools.go#L167-L259
[failure]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/service.go#L42-L52
[service]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/service.go#L100-L166
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/cmd/nerd/chat/deliberation.go#L18-L50
[mission-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/cmd/nerd/cmd_campaign_mission.go#L42-L69
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/shards/deliberation.go#L40-L65
[config]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/config/routing.go#L17-L61
[listing]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/tools/core/file_ops.go#L722-L841
[envelope]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/core/defaults/policy/deliberation.mg#L60-L68
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/core/defaults/policy/intent_routing_rules.mg#L258-L270
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/types.go#L144-L183
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/campaign/recurse_mission.go#L244-L315
[save]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/persistence.go#L23-L58
[agent-tests]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/agent_test.go#L61-L182
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/deliberation/deliberation_test.go
[prior-budget]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-terminal-budget.md
[prior-continuation]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-terminal-budget-continuation.md
[prior-horizon]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-probe-consumption-horizon.md
[prior-mixed]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-mixed-objection-probe-admission.md
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-read-invocation-identity.md
[prior-progress]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-progress-observation-identity.md
[prior-retirement]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-critical-item-retirement.md
[prior-citation]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-citation-admission.md
[prior-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/deliberation-selected-review-handoff.md
[prior-crucible]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/Docs/audits/portable-recurse-deliberation-crucible.md
[json]: https://pkg.go.dev/encoding/json#Marshal
[meyer]: https://se.inf.ethz.ch/~meyer/publications/computer/contract.pdf
[seda]: https://www.sosp.org/2001/papers/welsh.pdf

[comparison]: https://github.com/theRebelliousNerd/codenerd/compare/faa613df416495fa0f1188edf97d59ad4223e831...608ac7bec36cf66ecf2718ced8a438ee30b9dae6
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/shards/registration.go#L837-L849
[manager]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/core/shards/manager_spawn.go#L365-L469
[queue-store]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/core/shards/spawn_queue.go#L231-L254
[queue-run]: https://github.com/theRebelliousNerd/codenerd/blob/608ac7bec36cf66ecf2718ced8a438ee30b9dae6/internal/core/shards/spawn_queue.go#L365-L380

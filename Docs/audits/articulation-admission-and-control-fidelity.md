---
doc-class: deep-dive
subsystem: articulation-session-chat-context
implementation-status: partial
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection; as-built verified by the tests named in the As-built section
supersedes: []
---

# Articulation admission and control outcome fidelity

**Decision:** give every model response one explicit interpretation, and carry that interpretation and the actual disposition of its controls through the kernel, history and UI. A complete generation is not yet an unambiguous envelope; an accepted proposal is not yet an applied effect; a string extracted for display must never regain control authority because it happens to contain JSON.

**Scope and evidence:** source-inspected research on main at `356f65e0e6bb639adaa90ab36212daa714114871`, observed on 2026-10-28 around 07:38 UTC. No repository edits, implementation, tests, failure injection, benchmarks, live model requests or production incident reproduction were performed. Examples below are static counterexamples and proposed acceptance fixtures. The design names are proposed contracts, not existing APIs.

The immediate benefit is not another defensive parser flag. It is preventing meaning from changing between the text the model emitted, the proposal the harness considered, the state the kernel actually obtained, and the answer a person sees or a later turn remembers.

## Relationship to existing work

This is a refinement of the existing architecture, not a proposal to replace Piggyback or bypass Mangle. Root guidance assigns creativity to the model and executive truth to the kernel; articulation guidance already says parsed control is not authorization and tolerant fallback must remain surface-only. [Root guidance][root] [Scoped articulation guidance][guidance]

Several earlier gaps are already closed and must stay closed:

- The current chat path waits for a typed successful `StreamResult` before final response processing. Missing terminal evidence is not silently accepted there. The stream audit's as-built appendix records the provider/framing work. This document starts **after generation completion** and does not relabel that implementation as missing. [Chat terminal consumer][chat-stream] [Stream audit][stream-audit]
- Streaming already tracks structural depth, rejects several nested/non-string decoys, preserves split Unicode and key fragments, and has partition tests. The remaining issue is agreement on the language it accepts, not a return to substring-only extraction. [Streaming parser][stream-parser] [Partition tests][partition-tests]
- Model-authored Mangle updates are filtered through `ModelObservationPolicy`; host-owned test/completion evidence is protected. Chat filters observations, temporarily validates candidate actions, and records only admitted Mangle updates. Scoped memory operations and chat's no-replay compressor handoff are also present. [Observation policy][policy] [Chat application][chat-apply] [Memory operations][memory] [Applied-controls compressor][compressor]
- The existing tool-effect receipt audit concerns external tool effects and validation. This proposal concerns **response interpretation and non-tool control dispositions**. Reuse its eventual correlation model rather than inventing a competing end-to-end receipt system. The session TODO already leaves a joined execution receipt open. [Tool effect audit][tool-audit] [Session receipt work][session-todo]

The architecture wiring document deliberately declined blanket strict JSON because a valid plain-language answer should not become an error. Preserve that product decision. Distinguishing plain prose from a malformed or ambiguous attempted control envelope is a different, narrower requirement. [Current articulation decisions][wiring]

## Source traced findings

### One response can produce different live and final surfaces

`ResponseProcessor.parseJSON` creates a standard `encoding/json.Decoder`, calls `Decode` once and returns after a nonempty surface check. There is no second decode requiring EOF. It also searches for the first opening brace after a failed initial decode. `Process` returns immediately when that branch succeeds. Only after earlier branches fail does it invoke the embedded extractor that prefers later candidates. [Decoder][parse] [Dispatch order][dispatch]

That creates two distinct inconsistencies.

**Duplicate surface keys.** The incremental parser emits the first top-level `surface_response` and sets `completed` when that string closes; subsequent input is ignored. The final parser accepts duplicate keys and retains the later string. This is not merely a hypothetical interpretation of Go: existing articulation tests explicitly require last-wins duplicate behavior, including in strict mode. [Streaming state][stream-parser] [Duplicate test][duplicate-test] [Strict duplicate test][strict-duplicate]

Proposed fixture:

~~~json
{
  "control_packet": {},
  "surface_response": "The setting was saved.",
  "surface_response": "The setting was not saved."
}
~~~

Static expectation from the inspected branches: live text is the first statement; final text is the second. The final `assistantMsg` replaces the active stream, so this is not evidence that the first statement necessarily persists in history. It is evidence that the person can be shown a materially different assertion before replacement. The live UI has a cursor indicating liveness, but no semantic admission state. [UI stream rendering][ui] [UI final replacement][ui-update]

**Multiple envelope candidates.** An answer of the form `Example: {valid envelope A} Actual: {valid envelope B}` can be accepted by the early first-brace decoder as A. The last-match behavior of `extractEmbeddedJSON` does not govern that successful path. Existing decoy tests invoke the extractor directly, so their asserted behavior does not establish the behavior of `Process`. [First-object decoder][parse] [Direct extractor decoy test][decoy-test]

This also means a later qualification after a complete first object can disappear from the accepted answer. It need not be malicious: a model can emit an example, a correction, or an explanatory suffix. JSON framing tolerance and source selection are presently mixed.

**Equivalent or aliased key spellings.** The live scanner matches the literal bytes of `"surface_response"`. Final JSON decoding resolves escaped member names, and the production tolerant Go decoder also accepts case-insensitive matches to struct tags. Thus `"surface_respons\u0065"` can name the final surface without matching the live scanner; `"SURFACE_RESPONSE"` is another production-tolerant case. The contract must define identity after JSON string decoding and explicitly decide whether legacy case aliases are permitted. Comparing raw spellings is not sufficient for either surface detection or duplicate rejection. These are proposed grammar fixtures, not executed observations. [Literal stream match][stream-parser] [Final decoder][parse] [Go decoding semantics](https://pkg.go.dev/encoding/json)

**Boundary:** these findings do not establish that blocked Mangle predicates execute, or that task verification is bypassed. Independent policy and evidence gates remain relevant. The failure is interpretation divergence before those gates and in the user-visible projection.

### Plain surface text is not a durable type boundary

`promotePiggybackToolRequests` parses a response, applies control side effects through `processPiggybackControlPacket`, and replaces response text with the extracted surface. Its comment says a later parse will find no control packet and therefore be a no-op. That property is not guaranteed for an arbitrary surface string. A legitimate explanation of Piggyback may itself contain a syntactically valid envelope. The final task path calls `processPiggybackControlPacket(llmResponse.Text)` again. [Promotion][promotion] [Task tail][task-tail]

There is a concrete path to retain the promoted surface as final text: forced finalization promotes envelope requests, handles permitted/refused calls, clears tool calls and returns the response; the task tail then reparses its text. Whether verification replaces that response depends on the actual turn. This is a conditional production-path witness, not a claim that every ordinary continuation reparses an intermediate surface. [Forced finalization][forced-final]

The strongest initial witness needs no permitted write tool. Arrange a read-only turn with zero write-tool effects, reach forced finalization, and return an outer envelope containing a well-formed but **unoffered** tool request plus a surface that displays an inner example envelope with a harmless `note` memory operation. Promotion runs before the final offered-tool check: it processes the outer controls and rewrites Text. The offered-tool check then refuses the request; refusal does not undo promotion. `ClearToolCalls` returns the surface, and `nextPostEditRound` returns no round when `WriteToolEffects == 0`, leaving that surface available for the task-tail parse. The inner example must remain display data. Use a recording memory/kernel fake so the witness itself changes no production state. [Promotion before offer filtering][forced-final] [No-write post-edit path][no-write-round] [Task-tail parse][task-tail]

This does not claim the unoffered tool executes—it remains refused. The unintended operation is a separately parsed example in display text. It also does not require assuming a post-edit model will reproduce the same text.

The same problem exists more generally when a caller explicitly accepts plain prose but `ProcessLLMResponseAllowPlain` still looks for control-looking JSON inside it. “Allow plain” currently changes fallback expectations; it is not a declaration that embedded examples are non-executable. [Shared processing][processed]

Do not solve this by weakening the kernel allowlist. A second-stage interpretation of a permitted memory operation is still an unintended effect even if protected executive facts remain protected. Nor does string replacement establish exactly-once application. Only typed admission identity plus recorded application state can do that.

### Failed controls are not represented in the returned outcome

The parser removes oversized or malformed Mangle updates and caps Mangle/memory lists. It emits warnings rather than a per-item result. Chat carries articulation warnings forward, but the session's `processPiggybackControlPacket` consumes the parsed result without propagating `processed.Warnings`. Policy rejection produces a safety notice; subsequent `AssertBatch` or `Assert` failure only logs a warning. Therefore “rejected by policy,” “removed by parser,” and “admitted but not asserted” do not share one observable outcome path. [Parser filtering][caps] [Session control consumer][session-controls] [Session assertion][session-assert]

Memory has a sharper end-to-end example. `ApplyMemoryOperation` returns no result. Missing stores, storage errors, failed retractions and failed assertions are logged. Chat cannot determine success from this API; after invoking it, chat copies all parser-retained memory operations into the control packet passed to `ProcessTurnWithAppliedControls`. The compressor intentionally skips reapplication and preserves that metadata. The parser may already have capped the original list, so this is not a claim that every originally requested operation survives. [Memory operations][memory] [Chat handoff][chat-handoff] [Compressor metadata][compressor]

Proposed fixture: model surface says “I saved your preference,” while the memory store returns an error for that operation. Current source does not feed that store error back into the surface or typed control metadata. A separate debug-log watcher might later surface a problem; that is not an operation-bound result contract. The finding is not that the write error is completely unlogged, nor that compressed metadata is automatically replayed. The missing distinction is **requested versus applied versus failed**.

For `note`, replacement is two operations: retract the old key, then assert the new value. An assert failure after successful retraction is not “nothing happened”; a retraction failure followed by successful assertion is not necessarily a clean replacement. The current void API cannot express either partial outcome. Preserve scoped retraction—it is already an important fix—but expose the resulting state. [Note replacement][memory]

### Existing agreement tests establish names rather than interpretation

`TestEnvelopeShape_SchemaStructAndStrictKeysAgree` compares property names across schema, Go types and strict-key tables. It does not establish required-field semantics, duplicate handling, exact value ranges, compatibility coercions, response selection, consumer support, or stream/final equivalence. This is a useful test, not inadequate work to delete. Its assertion is narrower than the production contract now needed. [Shape test][shape-test]

A relevant nuance: the **full** schema already gives `surface_response` a minimum length of one; the simple schema does not. Do not report empty full-schema surfaces as a newly discovered conformance bug. A future role-specific tool-only envelope may legitimately choose to permit empty display text, but that would be a deliberate schema/product change and must not be smuggled in as a parser fix. [Schema variants][schemas]

The custom unmarshalers intentionally tolerate scalar Mangle updates, stringified confidence and several boolean representations. Preserve or retire these under an explicit compatibility profile. A successful coercion is not proof that the result satisfied the full advertised schema. [Wire coercions][protocol]

## Proposed admission contract

The smallest useful design is one admission object, not another independent “strict mode.”

~~~text
ResponseAdmission
  host_response_id
  workspace_id, session_id, turn_id, logical_call_id, attempt_id
  input_mode: plain | envelope | declared_legacy_envelope
  protocol_version, compatibility_profile, consumer_capabilities
  raw_artifact_ref, raw_digest, raw_length
  terminal_result_ref
  outcome: admitted | plain_only | incomplete | malformed |
           ambiguous | unsupported | rejected_resource_limit
  selected_span
  normalized_envelope
  normalization_events[]
  controls[]
  provisional_surface_digest, final_surface_digest

ControlDisposition
  control_id = host_response_id + channel + original_index
  kind, normalized_payload_digest, proposed_payload_ref
  stage: proposed | rejected_parse | rejected_policy | unsupported |
         admitted | applied | failed | partially_applied | unknown
  policy_revision, reason_code, effect_or_store_receipt_ref
  actual_result_ref
~~~

These are host-owned records. The model proposes fields but cannot write an “applied” disposition, supply a trustworthy response identity, or assert its own verification result. IDs must distinguish two equal operations at different response positions and a retry of one operation. Hash equality alone is not operation identity.

### Required invariants

1. **One interpretation.** An admitted envelope has exactly one selected root and one value per protocol key. Define key identity on decoded JSON names and explicitly declared aliases, not raw escape spellings. Duplicate detection must happen before decoding into a map loses multiplicity. Require EOF after the declared envelope/fence, except explicitly specified legacy framing.
2. **One admitted surface.** For every accepted wire encoding and chunk partition, provisional semantic text agrees with the selected normalized surface. A late invalidation marks prior content unaccepted and cannot silently turn it into final history.
3. **Display data remains data.** `SurfaceText` cannot be passed to a control parser as if it were raw model transport. Parsing entry points accept a raw-response type or explicit admission request, not arbitrary strings.
4. **Every proposed control has a disposition.** No parser cap, invalid item, unsupported consumer field, policy denial, assertion failure or storage failure disappears as an unstructured side effect.
5. **No status promotion by serialization.** A proposed or failed operation cannot become “applied” because compression, persistence, replay or a model paraphrase flattened the status.
6. **Generation, admission, application and task verification are separate.** Completing one stage does not imply the others succeeded.
7. **Authority remains derived.** Go parses and records observations; Mangle decides permission, obligations and task acceptance. An admission record is not a grant.
8. **Retries preserve effect identity.** A response repair before execution may replace an unadmitted proposal. After any effect occurs, repair must consume the prior dispositions rather than replay the packet.

A global transaction across kernel, vector storage and long-term storage is not required to adopt this. Honest partial/unknown outcomes are better than a fictitious all-or-nothing contract. If an operation needs atomic replacement, its storage implementation must provide it or return the measured partial state.

## How the pipeline should behave

### Recognize before interpreting

Choose the input contract when making the model request. Plain mode permits ordinary explanations, including envelope examples, without promoting embedded JSON. Envelope mode expects one root object with the declared version; optionally unwrap one declared Markdown fence. Legacy mixed-text mode must record that compatibility path and reject multiple plausible control envelopes as ambiguous rather than guessing that first or last means authoritative.

Use one schema/profile for final parsing and incremental recognition. Keep explicit scalar/string compatibility rules if empirical output requires them, but record the normalization. Unknown control keys should be rejected or marked unsupported according to version negotiation; do not silently erase a potential obligation. Arbitrary tool arguments remain the individual tool schema's job, not a reason to allow unknown control-channel fields.

This does not require waiting for the full reply before showing all text. Incremental display may remain provisional. It does require an explicit UI state and an admission result that can confirm or invalidate what was shown. Valid streams should not incur another model call.

### Apply controls without reparsing

Pass the admitted typed envelope from generation through promotion, tool continuation, finalization and the task tail. Each consumer either applies the controls it owns once, forwards their typed identity, or records “unsupported by this consumer.” Promotion changes the execution representation, not the admission status. `ClearToolCalls` must not clear evidence that other controls have already been processed.

A small response sidecar is sufficient initially; rebuilding all provider block APIs is unnecessary. It must nevertheless survive `Rewrite`, tool-call clearing, repair rounds and history projection. If those operations return a new response, the sidecar's ownership and copy semantics must be explicit.

### Record actual state before claiming it

Make memory and Mangle application return typed results. Record policy-filtered-but-not-applied separately from applied. Preserve parser removals before filtering can hide their existence. For list limits, either reject the control batch before effects or produce item dispositions and a declared continuation policy; silently applying a prefix is not an intact plan.

The UI can render small host-owned outcome statements for important effects—“preference was not saved,” “note replacement partially failed”—without asking a model to reinterpret a log. If a narrative claims success in conflict with those outcomes, do not simply append an easy-to-miss generic warning. Mark the relevant action as not completed and give the model the actual disposition for a corrected narrative when that is worth the cost.

This contract does **not** prove the truth of arbitrary prose. Claim-to-evidence validation for every natural-language sentence is a separate, larger problem. The first release can cover finite host-observable claims about controls and task status while leaving ordinary explanation quality to existing evaluation.

### Preserve the outcome in history

The compressed representation should retain operation identity and disposition, not just the original operation request. “Asked to remember X but persistence failed” is useful future context; it must not become “X remembered.” Keep rejected/raw payloads under bounded, redacted references when needed for diagnosis, not indiscriminately injected into future prompts.

Carry constraints as first-class metadata through related projections. The chat articulation payload currently includes `Intent.Constraint`, while the reconstructed compression control includes only category, verb and target. Original user prose is still provided at turn construction, so this is not a claim that the entire constraint disappears. It is a concrete structured-projection review item for the perception/context workstream. [Payload construction][payload] [Compression handoff][chat-handoff]

## Costs and tradeoffs

There is no measured speedup or token reduction claim. The design can reduce avoidable retries and mistaken-success recovery while adding validation, metadata and sometimes repair cost.

For a request family, measure:

~~~text
Expected total cost
  = model input and output across all attempts
  + cached input at its actual provider price
  + tool and storage work
  + parser and policy CPU
  + peak memory and retained artifact storage
  + admission/disposition persistence and export
  + repair and replay work
  + human diagnosis and recovery
~~~

Latency must separately report time to first provisional text, provider completion, admission, control application, final visible result, and verified task completion. A lower time to first token is not a lower time to a trustworthy result.

**Parsing.** A token scanner with duplicate tracking is linear in input length; per-object key sets require memory proportional to encountered keys, with depth and byte budgets set at the request boundary. The current caller and incremental parser already retain response text, so adding another whole raw copy should be avoided. Sharing immutable buffers or storing one artifact reference is preferable.

**Disposition storage.** Size is proportional to the number of controls plus references, not to every duplicate rendering of their full values. Sensitive preferences and reasoning traces should not become perpetual debug data. Declare redaction, retention and access policy; retain digests only when their privacy consequences are acceptable.

**Kernel cost.** Do not assert every wire token or parser diagnostic into Mangle. Pass compact host observations sufficient for executive decisions, and retain detailed receipts outside the hot fixpoint. Measure query/closure work for any new disposition predicates.

**Compatibility.** Requiring unambiguous framing may increase rejection on providers that currently emit helpful prose around JSON. A declared one-fence compatibility profile is cheap; a second model call for reformatted output is not. Measure which cases need recovery. Do not retry a disallowed operation merely because its control packet was rejected.

**Application latency.** In-memory admission does not require a distributed commit. Durable exactly-once effects are more expensive and need backend-specific idempotency/transactions. If durable disposition recording fails after a write, mark durability unknown and avoid replaying the write to repair the receipt.

**Repair.** If baseline defect probability is p0, candidate defect probability p1, expected downstream loss per defect R, additional deterministic cost D, and expected new repair cost F, adoption is economically attractive only when `(p0 - p1) × R > D + F`, while capability and safety constraints hold. These quantities must be measured by task class. Averages can hide rare severe losses; report tails and failure modes separately.

**Maintenance.** One normalization/admission implementation replaces scattered interpretation decisions, but maintaining version profiles, negative fixtures and adapters is real work. Adding a receipt type without removing consumer reparsing would increase complexity without closing the seam.

## Evaluation that establishes end to end fidelity

No evaluation below was executed. The first stage is deterministic and does not require paid model calls.

| Proposed witness | Required observable result |
|---|---|
| Duplicate surface/control/tool-argument keys | Protocol-specific policy is explicit; no stream/final disagreement or map-based loss of duplicate evidence. |
| Escaped and case-variant keys, including alias duplicates | Equivalent accepted keys share decoded/canonical identity; strict-profile aliases are rejected consistently rather than accepted only by final decoding. |
| A valid example followed by a different real envelope | Production `Process` does not silently choose the example; mixed-mode ambiguity has a typed outcome. |
| One root plus trailing text or a second root | Framing policy determines admission; trailing qualifications are not silently discarded. |
| Envelope example inside an admitted surface | Promotion, forced finalization, clearing and final tail never execute the example. |
| Plain answer containing JSON examples | Plain mode preserves the answer and creates zero control effects. |
| Known coercions and unsupported fields | Declared compatible coercions are recorded; unsupported obligations are not dropped without disposition. |
| Malformed, oversized or excess Mangle updates | Every original index has a result; accepted prefix is not reported as an intact batch. |
| Policy rejection versus assertion failure | Distinct outcomes reach UI/history; protected predicates remain blocked. |
| Failed preference/vector write | No “saved” host status; failed operation is retained as failed, not as applied metadata. |
| Note retract succeeds then assert fails | Partial state is represented; any recovery is scoped and policy-mediated. |
| Note retract fails then assert succeeds | Outcome does not falsely claim clean replacement. |
| Control applied before later response repair | Retry does not duplicate the effect or erase its earlier result. |
| Late terminal/admission failure after streamed text | Provisional state is visibly invalidated; failed content is not persisted as an accepted answer. |
| Valid ordinary response | No unnecessary repair; stable surface bytes and unchanged permitted effects. |

Use the normal chat and session entry paths with CodeDOM/world-model context enabled. Cross the actual client factory/broker, Piggyback promotion, native fallback, forced finalization, post-edit repair, policy filters, memory stores and UI persistence. Unit tests of `extractEmbeddedJSON` alone are specifically insufficient for the first-object finding.

For accepted inputs, compare normalized field identity and the ordered surface across every split point for short fixtures and randomized partitions for larger ones. Differentially compare streaming and final parsing; also compare semantic effect traces across chat/session routes that claim the same capability. “Both returned nil error” is not an equivalence oracle.

Then run paired, fixed-budget task trials on the same provider/model configuration and repository revision. Include protocol explanations with JSON examples, memory requests, reviews with long findings, multi-round tool use, valid plain answers, and injected downstream failures. Preserve existing safety, verification and tool permissions. Record:

- interpretation agreement and accepted-answer fidelity;
- lost/duplicated/wrongly applied control count;
- constraint and evidence preservation through history;
- false success and false rejection;
- recovery success without replayed effects;
- verified task success and task-specific answer quality;
- complete resource accounting from the prior section.

Do not select on parse success alone. A permissive parser can score perfectly while choosing the wrong object. Report uncertainty and subgroup results. Zero observed failures is an empirical bound for the sampled population, not proof of universal correctness; use deterministic properties for the grammar boundary and held-out tasks for behavioral claims.

## Primary foundations and their limits

- [RFC 8259 sections 2 and 4](https://www.rfc-editor.org/rfc/rfc8259): JSON framing is one value with surrounding whitespace, and duplicate member names create interoperability problems. This supports choosing a stricter envelope profile; it does not by itself decide CodeNERD's compatibility policy.
- [Go encoding/json documentation](https://pkg.go.dev/encoding/json): the v1 decoder's permissive behaviors include duplicate-name acceptance and case-insensitive field matching. A single `Decode` reads a next value, not an application-level one-message contract. This explains why application admission must establish additional invariants; it is not a recommendation to change the repository's Go version.
- [Möller et al 2024 Cross-Language Differential Testing of JSON Parsers](https://www.mlsec.org/docs/2024b-asiaccs.pdf): semantic disagreement between parsers is a distinct target from testing each parser for crashes. The useful transfer is differential semantic fixtures across CodeNERD's streaming/final/compatibility paths, not the paper's parser-count results as a forecast for this project.
- [Samuel and Erlingsson 2012 Let's Parse to Prevent Pwnage](https://research.google/pubs/lets-parse-to-prevent-pwnage/): normalized representations and explicit interpretation boundaries address data confusion. The proposed `SurfaceText` boundary adapts that principle to model output; the paper does not establish that this particular implementation is exploitable.

No new scientific method, universal LLM correctness guarantee or measured efficiency gain is claimed.

## Refinement queue and completion criteria

These are proposed local research IDs to fold into the owning articulation/session/context/CLI specifications.

| ID | Deliverable | Evidence required to close |
|---|---|---|
| ART-ADMIT-01 | Declared wire grammar and compatibility profile | One-root, duplicate, unknown-field and wrapper policy shared by full processing and streaming. Existing duplicate tests deliberately reconciled, not silently removed. |
| ART-ADMIT-02 | Typed raw/admitted/surface boundary | Ordinary and forced-final production routes never reinterpret display examples as controls; no untracked string-only reparse remains in those routes. |
| ART-ADMIT-03 | Per-control disposition | Parser, policy and application outcomes are distinguishable for every original operation; memory and assertion failures reach returned records. |
| ART-ADMIT-04 | UI and compressed-history projection | User-visible effect status matches actual disposition; compression preserves failed/partial/unknown outcomes without reapplication. |
| ART-ADMIT-05 | Capability and cost evaluation | Paired normal/faulted trials show fidelity and task capability with full costs, including false rejection and recovery; no weakening of gates. |

The first useful increment is narrow: make `Process` and the live stream agree on which envelope exists, and stop treating extracted surface strings as raw control input. Then make control application return the outcome that the UI and history need. A giant receipt framework is not a prerequisite; an unambiguous boundary and honest results are.

## As-built (2026-10-28)

Verified against branch `feat/conversation-brief` after it gained the conversation brief, work-mode admission and background shards. None of those touch this seam: the response processor, the stream parser, the promotion path and the memory-operation applier are as the audit describes them, so every finding below was still real when built. Test names are in `internal/articulation/admission_test.go` unless a path is given.

### What was real, and what was built

| Finding | Status | Where it is fixed | Tests that would have caught it |
|---|---|---|---|
| Duplicate `surface_response`: stream keeps the first, `Process` kept the last | Real | `scanEnvelope` (`internal/articulation/admission.go`) rejects a repeated member name, after decoding and case folding, in every object of the envelope (tool arguments compare exact names). `Process` stops on it and returns `AdmissionAmbiguous`: no control, and a host-written surface that says nothing was accepted and any streamed text is not part of the answer (`rejectAmbiguous`, `emitter.go:393`). Strict mode returns an error. | `TestAdmission_RepeatedMemberNamesAreAmbiguous` (seven spellings: plain, escaped, case alias, duplicated `control_packet`, duplicate inside control, duplicate tool argument, duplicate inside an array item); `TestResponseProcessor_Boundary_DuplicateKeys`, `TestResponseProcessor_Process_DuplicateKeys` (the old last-wins assertions, reconciled, not deleted) |
| Escaped and case-variant key spellings read differently by stream and final parse | Real | The stream parser now holds at the opening quote of a top-level key until the closing quote arrives, decodes it, and compares case-insensitively (`scanStreamKey`, `stream_parser.go:243`), the rule the final decoder already applied. Held bytes are bounded by the longest escaped spelling of the name (98 bytes). | `TestAdmission_StreamAndFinalAgreeOnAcceptedSpellings` (7 encodings, every split point, 50 random partitions each); `TestStreamParser_OnlyTheSurfaceKeyCapturesTheStream` |
| `parseJSON` accepted the first object and ignored what followed; the first-brace branch let `Example: {A} Actual: {B}` select A | Real | `parseJSON` (`emitter.go:619`) admits exactly one object with nothing after it. The first-brace branch is deleted. Text around an envelope is the declared legacy framing owned by `extractEmbeddedJSON`, which now counts complete envelopes: one is admitted as `json_extracted` with a warning that outside text was dropped; two or more are `AdmissionAmbiguous`. The old last-match-wins rule is gone, because a quoted example could win it as easily as it beat a decoy. | `TestAdmission_TrailingTextIsReportedNotSilentlyDropped`, `TestAdmission_SecondEnvelopeIsAmbiguousButOtherObjectsAreNot`, `TestExtractEmbeddedJSON_DecoyInjection`, `TestResponseProcessor_ExtractEmbeddedJSON_DecoyInjection` (both reconciled to the ambiguity contract) |
| Display text regains control authority (promotion, forced finalization, task tail) | Real | `types.LLMToolResponse.ControlsApplied` (`internal/types/interfaces.go:162`) is the host's mark that a response's envelope was read and applied. It is set by promotion (`piggyback_promotion.go:71`), by the single-shot Piggyback path (`executor.go:1871`) and by `Executor.admitResponseControls` (`executor.go:2453`), which the task tail and the work-step note now call. It survives `Rewrite` and `ClearToolCalls`. A marked response is never promoted and never parsed again. Promotion parses once and applies from that parse (it used to parse, then parse again inside `processPiggybackControlPacket`, which no longer exists). Work steps used to apply a step's envelope and then let the tail apply the same raw text a second time; the mark ends that too. | `TestAdmitResponseControls_SurfaceTextIsNeverReadAsControlInput` (the audit's forced-final witness, with a recording kernel: outer envelope promoted, call cleared, tail run, inner example not applied); `TestAdmitResponseControls_AppliesAnEnvelopeExactlyOnce` |
| The single-shot Piggyback path applied mangle updates and dropped every other control | Real, found while tracing | `generateResponseWithPiggybackTools` now applies the whole packet once through `applyPiggybackControls` (`executor.go:2490`) and marks the response. | covered by the two tests above (same entry point) |
| Parser removals and caps were warnings only | Real | `applyCaps` records a `ControlDisposition` for every update it removes and for each item past a list limit, at the position in the list the model sent (`ArticulationResult.Dispositions`, `MangleOrigins`; carried on `ProcessedLLMResponse`). The limit policy is declared: the prefix is applied and each item of the tail is `rejected_parse`. | `TestAdmission_ParserRemovalsKeepDispositions`, `TestAdmission_ListLimitsRecordTheRejectedTail` |
| Policy rejection and assertion failure had no shared outcome; session assertion failure was a log line | Real | `core.MangleUpdateBlock` carries the position (`Index`); `articulation.MangleUpdateDispositions` accounts for every position (blocked, blank, failed write, applied) numbered in the model's own list. `processMangleUpdatesFromEnvelope` returns them. | `TestFilterMangleUpdates_BlocksCarryTheirPosition` (core); `TestMangleUpdateDispositions_OneDispositionPerPositionInTheModelsList`; `TestProcessMangleUpdates_RecordsPolicyRefusalAndAppliedByPosition`, `TestProcessMangleUpdates_NoKernelIsUnsupportedNotSilent`, `TestApplyPiggybackControls_PositionsAreTheModelsOwn` (session) |
| `ApplyMemoryOperation` returned nothing; a failed store was invisible, and chat copied every parsed operation into compressed history | Real | `ApplyMemoryOperation` returns a disposition (applied, failed with the store's reason, partially applied, unsupported, rejected) and `ApplyMemoryOperations` indexes a list (`internal/context/memory_ops.go:59,118`). A note is two writes and reports which half landed. Session: failures reach the user as host-written `[Control] ...` notices on the turn (`carryControlNotices`, `executor.go:2473`). Chat: `applyArticulationControlPacket` returns `appliedControls` (`cmd/nerd/chat/control_outcome.go`); only operations that landed go into the compressor's packet, the rest ride as `Turn.ControlOutcomes` and appear as warnings in the reply. | `TestApplyMemoryOperation_ReportsAStoreFailureInsteadOfDroppingIt`, `..._AppliedAndUnsupported`, `..._NoteReplacementReportsPartialState`, `TestApplyMemoryOperations_IndexesByPositionInTheModelsList` (`internal/context/control_outcomes_test.go`); `TestAdmitResponseControls_FailedMemoryWriteReachesTheUser`, `..._ParserRemovalsReachTheUser` (session); `TestApplyArticulationControlPacket_FailedMemoryWriteIsNotRecordedAsRemembered`, `..._CarriesParserRemovals` (chat) |
| Compressed history flattened status ("asked to remember" became "remembered") | Real | `CompressedTurn.ControlOutcomes` and `Turn.ControlOutcomes`; the compressor's own application path sorts operations the same way; outcomes are written into the rolling summary and the recent-turn rendering, are never masked, and are deep-copied in snapshots (`writeControlOutcomes`, `compressor_turns.go`, `serializer.go`). | `TestProcessTurn_FailedMemoryOperationIsRecordedAsFailedNotRemembered`, `TestProcessTurnWithAppliedControls_RetainsOutcomesInBothRenderings`, `TestChatHandoff_FailedControlReachesCompressedHistory` |
| Chat compression handoff dropped `Intent.Constraint` | Real | `compressionControlPacket` (`control_outcome.go:60`) carries the constraint into the `user_intent` atom history keeps. | `TestCompressionControlPacket_CarriesTheConstraintAndOnlyAppliedControls` |
| Hand-written memory ops from perception (`processMemoryOperations`) ignored the result | Real, small | the "forget" branch now warns when the op did not take effect (`process_helpers.go`). | covered by `TestApplyMemoryOperation_*` |

### Already fixed or moot

- Terminal evidence before final processing: `types.AwaitStreamResult` gates `Process` in chat (`cmd/nerd/chat/helpers_articulation.go:365`). The audit already said so; unchanged.
- Empty full-schema surface: the full schema has `minLength: 1` (`internal/articulation/schema.go:216`), as the audit noted. Not a defect.
- `TestEnvelopeShape_SchemaStructAndStrictKeysAgree` is kept; it checks names, and the new tests check interpretation. The existing duplicate and decoy tests were changed because they pinned the behavior this document calls a defect.
- Scoped `forget` and the model observation policy: present and unchanged.

### Deliberately left, with reasons

- **A declared input mode on the request (plain vs envelope).** A raw reply that is prose around exactly one envelope is still admitted as `json_extracted` (with a warning), because providers do emit that and a rejection would show people raw JSON. What is closed is the case the audit proved: an already-admitted surface is never read again, and a reply with two envelopes selects neither. A plain-mode request that never extracts needs the mode threaded from the request builders in every shard; no shard needs it today.
- **A provisional UI state.** The stream is still shown as it arrives, the final reply replaces it, and for an ambiguous reply the replacement says that the streamed text was not accepted. A distinct "unaccepted" rendering state in `view.go`/`model_update.go` needs a stream-reset message the channel does not carry; those files are being edited elsewhere.
- **`host_response_id`, `attempt_id`, durable receipts.** A control is identified by kind and position within one reply; a response identity needs the broker's call identity, which is the effect-receipt audit's join. Nothing here persists dispositions beyond compressed history.
- **Candidate actions in chat** (the stratified-trust path) still report through warnings only; they are not in `NotApplied`.
- **Compatibility coercions as a named profile** (scalar mangle updates, stringified booleans) stay as they are and are not recorded as normalization events.
- **Retries after an applied control.** Repair rounds produce new responses; no repair path replays an envelope today, so no replay guard was added.
- **Display-only re-extraction.** `formatDelegatedResponse` and its sibling run `ProcessLLMResponseAllowPlain` over shard results that start like an envelope, to show the inner surface. They apply nothing, so they are not control re-parses.
- **Strict profile vs the stream.** Strict mode rejects a case-variant key; the live stream (used only by tolerant chat) accepts it, as the final tolerant parse does. They agree wherever chat runs.
- **Cost measurements and paired model trials** were not run; the new checks are deterministic.

### Mangle and prompt surface

No Mangle declaration, policy rule or prompt atom changed. `MangleUpdateBlock` gained a Go field only.

[root]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/agents.md
[chat-stream]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L286-L367
[stream-audit]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/stream-completion-contract.md#as-built-2026-10-27
[stream-parser]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/stream_parser.go#L47-L225
[partition-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/articulation_uplift_test.go#L26-L108
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/mangle_updates.go#L25-L165
[chat-apply]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L980-L1151
[memory]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/memory_ops.go#L25-L136
[compressor]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/compressor_turns.go#L33-L137
[tool-audit]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/tool-effect-receipts-and-validation-completeness.md
[session-todo]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/session/TODO.md#L215-L250
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/articulation/WIRING-AND-NOT-BUILT.md#L69-L108
[parse]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/emitter.go#L527-L596
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/emitter.go#L203-L282
[duplicate-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/emitter_test.go#L371-L385
[strict-duplicate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/emitter_boundary_test.go#L169-L197
[ui]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/view.go#L135-L162
[ui-update]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_update.go#L84-L150
[decoy-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/emitter_test.go#L400-L418
[promotion]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/piggyback_promotion.go#L10-L78
[task-tail]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1299-L1347
[forced-final]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L796-L874
[no-write-round]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L510-L576
[processed]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/emitter.go#L1003-L1062
[caps]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/emitter.go#L360-L438
[session-controls]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L2385-L2494
[session-assert]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1981-L2015
[chat-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L906-L941
[shape-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/envelope_shape_agreement_test.go#L10-L111
[schemas]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/schema.go#L214-L240
[protocol]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/articulation/protocol_types.go#L49-L215
[payload]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L67-L79


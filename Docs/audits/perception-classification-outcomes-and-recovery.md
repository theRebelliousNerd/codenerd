---
doc-class: deep-dive
subsystem: perception
implementation-status: partially-implemented (see As-built status)
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Perception classification outcomes and recovery parity

A failed classification should remain distinguishable from a successful interpretation all the way to the interactive disposition. At the inspected main revision, the provider taxonomy distinguishes authentication, cancellation and other causes, but the compatibility intent retains only a transient bit as structured failure state. On the ordinary fresh full-turn path, a degraded `/explain` intent routes toward articulation, not the clarification lane; neither its safe failure response nor its typed cause is directly projected into that articulation request. A separate method-promotion boundary bypasses the specialized schema path at the perception firewall.

**Recommendation:** preserve a typed classification outcome from the selected transport through parsing, normalization and the session disposition; make all supported perception entry points use the same capability-aware classification operation. Keep the existing graceful-degradation compatibility contract while callers migrate. Do not restore a Go vocabulary validator, reconnect regex mutation fallback as a shortcut, or switch providers/models without explicit configuration.

This is source-based diagnosis and proposed design, not a reproduced account of the user's preliminary-investigation failure. No implementation, repository edits, tests, application probes, model calls or benchmarks were run. Main resolved to [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), committed 2026-10-28 06:24:06 UTC. Source inspection occurred on 2026-10-28; main and research-branch heads were reconfirmed at approximately 08:40 UTC, and the feature branch was refreshed again at approximately 08:55 UTC to the separately pinned head below. Main implementation links are pinned to that revision; the separately labeled branch comparison uses each branch's inspected commit. Proposed fixtures and acceptance criteria are not passed results.

## Scope and existing contracts

This work answers a narrow question: **when perception cannot produce an admitted interpretation, does the actual caller preserve why and choose an appropriate next step?** It does not redesign action permissions, source-evidence delivery or articulation parsing.

The research branch was inspected alongside main. Its [perception restriction continuity proposal][restriction] addresses recognized constraints lost after successful understanding. Its [articulation admission proposal][articulation] addresses response/control parsing, side-effect dispositions and display fidelity. Its [TUI evidence ownership proposal][evidence] addresses turn-correlated action outcomes and whether observed source evidence reaches the answering request. This proposal supplies an earlier input to those contracts: whether an interpretation was actually obtained, whether the problem belongs to the user or infrastructure, and which recovery route is valid. The [perception capability recovery audit][capability] already proposes a broader ownership/decision receipt; the compact outcome below should be part of that receipt, not a competing subsystem.

Several important capabilities are already built and must not be rediscovered as absent:

- `ProviderFailureClass` distinguishes transient service failure, rate limiting, authentication, invalid request, cancellation and permanent failure. `ClassifyProviderFailure` preserves cancellation precedence and recognizes wrapped rate-limit errors; `SafeProviderFailureMessage` avoids exposing the provider body. [Shared contract][provider]
- The degraded adapter intentionally returns an `/explain` `/query` intent with a nil error. A transient/retryable failure sets `TransientFailure`, and the firewall converts it to `/llm_unavailable`; an existing firewall test checks the corresponding honest kernel question. [Adapter][degraded], [firewall regression][transient-test]
- The architecture already records the error-contract choice as [OPEN-QUESTIONS Q5][questions]. Its [provider contract feature card][provider-card] is explicitly `in_progress`, not verified or closed, and records the shared taxonomy and conformance work. That card's older remaining-work notes must be checked against current tests rather than treated as current absence: the inspected suite includes CLI paths and cancellation cases. The broader session-recovery obligation remains open. [Conformance suite][conformance]
- Routing-vocabulary decisions belong to Mangle. Scoped guidance expressly forbids reintroducing a Go vocabulary validator. Structural wire validity and transport failure are different questions from whether an otherwise well-formed action name is in the kernel vocabulary. [Perception guidance][guidance]

Root guidance also requires JIT-first model-facing behavior, spec-first changes, deterministic executive decisions in Mangle, and validation through normal production paths with world-model and CodeDOM context. [Repository contract][root]

## Other branch code checked

**As-of freeze, 08:57 UTC:** the feature head advanced once more to [`facfe55107dfacd45745075f536f12ade86d0255`](https://github.com/theRebelliousNerd/codenerd/commit/facfe55107dfacd45745075f536f12ade86d0255), eighteen commits ahead of main. Comparing it to the code snapshot `777e93c` returns exactly one added documentation file and no code changes. Source links below intentionally retain the inspected `777e93c` code pin. The new [conversation-first architecture description](https://github.com/theRebelliousNerd/codenerd/blob/facfe55107dfacd45745075f536f12ade86d0255/Docs/architecture/conversation-first-agent.md) describes the already-inspected brief, work modes, background work, side turns and Jev integration. It does not introduce a classification-failure outcome contract or alter the corrected source trace. Its named tests are repository documentation, not tests run for this proposal.

The earlier 08:20 UTC branch listing contained `main`, `codenerd-research-docs` and `feat/conversation-brief`. Fresh head checks at approximately 08:38–08:55 UTC on 2026-10-28 found main and the research branch unchanged, but the feature branch advanced seven commits from the earlier `31e27a9` comparison, through `5846386`, `78103e`, `f37b855` and `e033fc4` to the pin below. The inspected feature code head was fetched and compared directly to main, followed by the docs-only freeze check above; do not treat the earlier `31e27a9` comparison as current.

| Branch and inspected code head | Relevant inspected changes | Status of this proposal's findings |
|---|---|---|
| `feat/conversation-brief` at [`777e93c42f3b9ef57212bfac7be0114f3cc4805e`](https://github.com/theRebelliousNerd/codenerd/commit/777e93c42f3b9ef57212bfac7be0114f3cc4805e) | Seventeen commits ahead of main. Adds the running brief, approval/hold work modes, background read-only delegation, self-diagnosis, Jev turn/relevance judgments, review-finding checks and defect-versus-surroundings triage; latest batch also adds native-context accounting, embedding identity, MCP wire fidelity, browser ownership and documentation evidence work; the latest commit adds side turns, message queuing and active-turn cancellation. | Active adjacent implementation, not a fix for the classification-cause or GCD dispatch boundaries. Preserve and integrate it rather than proposing a second conversation/work-mode system. |
| `codenerd-research-docs` at [`b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b`](https://github.com/theRebelliousNerd/codenerd/commit/b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b) | Eleven documentation commits ahead of its merge base and 89 commits behind main. Relevant old-code differences include the earlier input truncation and stream wrapper, not failure-outcome repair. | Research proposals are dependencies/context, not evidence that this implementation is fixed on a code branch. |

On the feature branch, `provider_failure.go`, `transducer_gemini.go`, the firewall implementation, `clarification.mg` and the session executor were byte-identical to main. The adapter still returns nil-error degradation and only a transient bit; its GCD method still calls the base context method. The parser adds brief transfer on the successful envelope path but retains the envelope-error-to-bare fallthrough. [Feature adapter](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/perception/understanding_adapter.go#L321-L351), [feature GCD](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/perception/understanding_adapter.go#L730-L742), [feature parser](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/perception/transducer_llm.go#L439-L477).

The feature's chat ingress now supplies brief context and can reuse an approved pending intent. Its routing adds hold/approval/background behavior. Those are real code changes relevant to future acceptance, not verified runtime results from this review. They do not add a nontransient classification outcome, but they mean the exact final UI must be tested under the new work modes rather than assumed identical to main. This document's detailed normal-TUI witness is therefore explicitly a **main-revision** trace. [Feature ingress](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/cmd/nerd/chat/process.go#L166-L217), [feature routing](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/core/defaults/policy/routing_arbitration.mg#L179-L286).

### The latest Jev integration changes the recovery matrix, not the transport diagnosis

The subsequent `f37b855` → `e033fc4` commit is broad but does not change the cited classification parser, adapter, Gemini specialization, firewall, routing rules, chat ingress or articulation builder; fresh blob hashes for all seven matched the preceding snapshot. Its factory change owns the research-browser runtime, not classification dispatch. Native-block request accounting changes the cost baseline and should be retained in evaluation; it does not restore failure-cause transport. The new documentation-evidence standard explicitly treats planned attachment-point citations as seams, not implementation evidence—the same distinction used throughout this proposal. [Latest batch](https://github.com/theRebelliousNerd/codenerd/commit/e033fc454e20482c5d008212991b09e5b997b9d7), [request accounting](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/prompt/request_tokens.go#L16-L78), [planned-seam standard](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/Docs/journeys/09-architecture-doc-standard.md#L139-L152).

The preceding `5846386` → `f37b855` comparison contains two further commits: reviewer findings are checked against cited code, and self-diagnosis separates probable code defects from environmental/provider/configuration failures. Neither modifies the primary perception parser, adapter, Gemini specialization or firewall; fresh file hashes matched the preceding feature snapshot. The triage operates on an existing problems log when constructing a diagnosis job, not on the original classifier outcome. It is valuable adjacent work, but not a substitute for preserving transport cause before the initial UI disposition. [Diagnosis-task triage](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/cmd/nerd/chat/self_diagnosis.go#L80-L103), [triage input and split](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/turnjudge/triage.go#L11-L63).

The refreshed feature branch adds an independent turn-judgment operation before perception and joins it before routing. Successful judgments can influence clarification, held-work approval, brief readiness, self-diagnosis and cancellation of background work. They do not carry the cause of a failed primary `Understand` call, and the adapter/parser/GCD implementation is unchanged from `31e27a9`. [Actual parallel start and join](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/cmd/nerd/chat/process.go#L177-L217), [judgment rules](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/core/defaults/policy/jev_evidence.mg#L32-L76).

This is not a reason to add another judge or conversation memory. Preserve the existing integration and make its provenance independent from primary classification validity. In particular, a successful judgment is not evidence that the primary classification succeeded; a primary failure is not evidence that a separate stop request failed. Recovery policy must specify which narrow control lanes can use independently admitted evidence and which require an admitted interpretation. Approval of held work already has precedence over other lanes, so the fresh-turn direct-answer trace below must not be generalized to every feature-branch held-work case. This is a design interaction to evaluate, not an executed unsafe-approval finding. [Approval precedence](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/core/defaults/policy/routing_arbitration.mg#L179-L195).

The feature documentation reports live judgment/retrieval probes; those are the repository author's observations, not results reproduced here. The source also waits for the judgment channel or context cancellation before routing, so parallel initiation alone does not prove zero additional latency. A slower judgment can remain on the critical path after perception finishes. Include that join time and independently successful/failed judgment attempts in the cost baseline. [Wait and failure behavior](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/cmd/nerd/chat/jev_turn.go#L50-L85), [reported probes](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/Docs/architecture/jev-turn-judgments.md#L57-L112).

### New side turns preserve the safe response, but not a typed cause

At `777e93c`, messages typed during delegated or multistep work can use a new side-turn path. It calls the direct transducer, carries `intent.Response` into `sideTurnMsg.Reply`, and displays that reply. Therefore a nil-error degraded classification's existing safe diagnostic **does survive this particular UI route**. The ordinary fresh-turn articulation trace below does not describe side turns. The new policy projects category/verb plus independent Jev judgments into `/stop`, `/later` or `/converse`; it does not transport a typed primary-classification failure. Preserve this useful response fidelity rather than adding an unnecessary articulation call to make paths uniform. [Side-turn classification and reply](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/cmd/nerd/chat/side_turn.go#L156-L175), [display and queue handling](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/cmd/nerd/chat/side_turn.go#L224-L242), [side-turn policy](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/internal/core/defaults/policy/side_turns.mg#L10-L35).

The five additions to ordinary `process.go` register/retire the running turn's cancel and mark multistep/delegated work active; they do not alter ordinary failure classification or the direct-answer routing conditions. The new branch is a reason to test side-turn versus full-turn disposition, not evidence that the parser or GCD boundary is repaired. [Latest integration commit](https://github.com/theRebelliousNerd/codenerd/commit/777e93c42f3b9ef57212bfac7be0114f3cc4805e).

## Failure causes are projected away before an explicit interactive disposition

The adapter receives a failed `Understand` call and returns `degradedClassificationIntent(err), nil`. The degraded helper retains a safe explanation in `Intent.Response` but carries only a `TransientFailure` boolean as machine-readable cause. It leaves confidence at zero, verb `/explain`, category `/query`, and scope/tool fields empty. [Error conversion][adapter-error], [degraded helper][degraded]

The firewall's independent error branch is based on `parseErr != nil`. That branch can use fallback parsing and later publish `/llm_failed`. It does not run for the canonical adapter's nil-error degraded result. For an exhausted retryable provider error, the explicit transient bit correctly selects `/llm_unavailable`. For authentication, invalid request, ordinary parse failure or cancellation, the bit is false; with the normal positive ambiguity threshold, the degraded `/explain` result selects `/heuristic_low`. This is a loss of structured cause at the policy-fact boundary, not yet a claim about the visible response. [Firewall failure selection][firewall-failure]

### Corrected source-derived fresh-turn trace

**Review correction:** an earlier draft inferred that the canned question derived from `/heuristic_low` would be shown on a normal fresh turn. That inference was wrong: it omitted direct-answer precedence. The claim that the TUI deterministically asks the user to clarify this failure is withdrawn.

For a fresh TUI turn such as “Investigate why the parser ignores this file,” with a classification client returning a wrapped HTTP 401:

1. The shared failure classifier identifies authentication failure; the adapter's safe `Response` advises checking the configured key.
2. The adapter creates `/query /explain`, confidence zero, `TransientFailure=false`, empty scope/tools, and nil error.
3. The firewall publishes `/heuristic_low`. In non-yolo mode a generic clarification question can derive, but existence of that question does not imply selection of the clarification route.
4. `/explain` is a conversational verb. With no scope/tool evidence signals, `wants_direct_answer()` derives; `clarify_lane()` requires its negation. Empty-target `/explain` is not one of the perception-answer fast-lane cases. Absent another applicable early lane, the route is `/respond_directly`, not `/clarify` or `/perception_answer`. [Signal projection][route-signals], [direct-answer rules][route-direct], [fast-lane cases][route-fast], [clarification exclusions][route-clarify]
5. Chat therefore does not return the degraded `Intent.Response` through the perception-answer branch and does not enter the canned-clarification branch. Its ordinary path proceeds toward articulation. `payloadForArticulation` initializes an empty surface and copies classification fields, not the safe response or transient/failure identity; the request builder renders the intent's verb/target but not `Intent.Response`. [Chat lane handling][chat-lanes], [articulation invocation][chat-articulate], [payload projection][art-payload], [request builder][art-request]

This is source-derived control flow, not an executed reproduction. Later failures, explicit overrides, feature-branch control evidence and other runtime conditions can change the outcome. A later model may recover useful work, or fail in turn. **The final visible wording and the user's reported incident are not established by this trace.** What is established is narrower: normal routing can continue after failed classification without a typed cause-specific disposition, and the safe cause held in the compatibility response is not directly carried by the ordinary articulation request builder. A generic low-confidence fact is not equivalent to an authentication/cancellation/invalid-response result.

The transient path is a lower-boundary positive control: its bit, `/llm_unavailable` fact and honest question derivation already exist. The firewall regression does not prove the assembled TUI shows that question; the same fresh `/explain` direct-answer precedence must be checked. A successful but uncertain interpretation is another control: it should still receive semantic clarification when its actual routing policy requires one. Do not report either control as an end-to-end pass from a helper or firewall-only test. [Firewall regression][transient-test], [clarification messages][clarification]

### Direct callers do not get equivalent outcome handling

Chat prefers the firewall but calls `m.transducer.ParseIntentWithContext` directly when the system path is unavailable or returns an error. Its immediate error gate only checks the returned error. The direct path asserts the resulting intent without constructing the firewall's unknown-reason facts. A normal session executor similarly accepts the observation when the error is nil, records the intent and proceeds into retrieval, catalog selection and JIT compilation. [Chat ingress][chat-ingress], [session observation][session-observe]

This establishes **continuation after an untyped degraded observation**, not that the eventual answer must be wrong or that an unsafe action necessarily executes. The original input still reaches downstream processing; a later model may recover, or another guard may stop the turn. But no common classification outcome determines whether that continuation is an authorized recovery attempt, a configuration blocker, a canceled turn or a user clarification. The source does not support interpreting “nil error” as “classification succeeded.”

## Structural failures can become successful default interpretations

`parseResponse` deliberately supports both an `UnderstandingEnvelope` and a bare `Understanding`, and normalizes case and confidence. Preserve compatible bare responses and useful deterministic normalization. The problem is when **an invalid instance of one shape is silently reinterpreted as a different shape**. [Parser][parse]

The function attempts full envelope unmarshalling first. Its key-presence probe runs only if that unmarshal succeeds. If the envelope has a field type error, it falls through to unmarshalling the entire object as a bare `Understanding`. The nested `understanding` key does not name a field in the bare type and is ignored by the default Go JSON decoder. The documented decoder behavior is that unknown object fields are ignored and a mismatched field type produces an unmarshal type error. [Go JSON documentation](https://pkg.go.dev/encoding/json#Unmarshal)

For example, this is a proposed fixture, not a test result:

```json
{
  "understanding": {
    "primary_intent": "investigate",
    "semantic_type": "location",
    "action_type": ["investigate"],
    "domain": "general",
    "confidence": 0.9
  },
  "surface_response": "I will investigate."
}
```

The envelope's array cannot populate `ActionType string`, so envelope decoding fails. The bare decode ignores the nested object, can retain the top-level surface, and returns an otherwise zero-valued understanding with nil error. Its empty action subsequently maps to `/explain`; the category defaults to `/query`. A syntactically valid unrelated object or empty object likewise need not be an admitted classification. [Parser shape fallthrough][parse], [conversion][conversion], [default mapping][mapping]

This does not justify hard-failing all unknown domain/action vocabulary, making `primary_intent` prose mandatory when it is unused, or mechanically calling the existing `Validate` method everywhere. That helper performs basic presence/range checks but cannot distinguish an omitted numeric field from an explicitly supplied zero once both have been decoded into a `float64`. The required contract needs presence-aware wire validation and an explicit compatibility profile. [Current validator][validate]

The actionable distinction is:

- **Structural invalidity:** the selected response is not a supported classification shape or a required routing field has the wrong wire type.
- **Incomplete interpretation:** a supported shape lacks information required for this routing decision.
- **Vocabulary miss:** a well-formed value is outside the kernel's current vocabulary.
- **Semantic uncertainty:** a structurally admitted interpretation is uncertain about what the user meant.

Only the last category is automatically evidence that the user's request may need clarification. The current parser and compatibility projection can collapse the first two into it. The existing malformed-JSON adapter test deliberately requires nil-error degradation; the wrong-type parser test uses bare fields. Neither is an end-to-end oracle for the malformed-envelope witness above. [Existing adapter test][malformed-test], [bare type test][type-test]

## The firewall bypasses the specialized schema path

The firewall calls `transducer.ParseIntentWithGCD` with a validation-retry limit. The base `UnderstandingTransducer.ParseIntentWithGCD` is intentionally a compatibility wrapper: it calls its own `ParseIntentWithContext`, returns no Mangle updates, and does not use `maxRetries`. The generic `Understand` operation itself calls `CompleteWithSystem`, not the specialized schema operation. [Firewall call][firewall-failure], [base GCD method][gcd], [generic model call][understand]

`NewUnderstandingTransducer` can return a `GeminiThinkingTransducer`. That wrapper embeds `*UnderstandingTransducer` and explicitly overrides `ParseIntent` and `ParseIntentWithContext`. The latter tries `CompleteWithSchema` when the client supports it. There is no wrapper override for `ParseIntentWithGCD`. The promoted GCD method therefore has a base receiver and calls the base context method; it does not dynamically dispatch to the wrapper's context method. The wrapper's existing `ParseIntent` comment already explains this Go embedding constraint. [Constructor][constructor], [specialized entry points][gemini], [Go promoted-method rules](https://go.dev/ref/spec#Struct_types)

| Actual entry point | Inspected classification behavior |
|---|---|
| Generic transducer context method | Generic system completion and shared parse path |
| Specialized wrapper context method | Schema attempt when supported, then free-form path on failure |
| Specialized wrapper `ParseIntent` | Explicitly calls the specialized context method |
| Specialized wrapper `ParseIntentWithGCD` | Promoted base method calls base context method; schema-first specialization is bypassed |
| Normal firewall | Calls GCD; therefore does not gain that specialization merely because construction selected the wrapper |

The specialized schema-routing test calls `ParseIntentWithContext` on a deliberately constructed wrapper and checks schema/free-call counts. It proves the targeted method's intended behavior, not parity through the interface method used by the firewall. [Schema test][gemini-test]

This finding is conditional on a client that exposes the relevant thinking/schema capability. The firewall's fallback guarded client exposes a smaller method set; its dedicated classification-client path can preserve the original client capabilities. Do not promise that every current provider supports schema generation, or remove the cost guard to expose it. Any capability forwarding must retain the wrapper's cost, cancellation, metering and configuration semantics. [Firewall client construction][construction]

### Fixing parity must not activate indiscriminate retry

The specialized context path currently falls back from **any** schema error to another free-form call, without classifying why the schema attempt failed. That includes a permanent authentication/configuration failure as well as a potentially recoverable output-shape problem. The code does not first select a failure-specific transition. [Schema fallback][gemini]

This is not proof of a second network send after cancellation: a compliant underlying client may stop before sending. It is proof that the transducer invokes its next operation without distinguishing that terminal cause. Reconnecting the schema path at GCD without specifying this transition could make the existing behavior more common. Pair entry-point parity with cause-aware recovery and a single accounting owner.

## Primary research: useful mechanisms and limits

These papers motivate evaluation choices; none establishes an improvement in CodeNERD or reproduces the source-derived failures above.

| Primary work | What transfers | What does not follow |
|---|---|---|
| [PICARD — Scholak, Schucher and Bahdanau, EMNLP 2021](https://aclanthology.org/2021.emnlp-main.779/) | Incremental parsing rejects inadmissible decoder tokens; formal validity can be enforced during generation rather than only repaired afterward. This motivates exercising the actual schema-capable entry path. | Its text-to-SQL results are not an accuracy or latency estimate for CodeNERD's provider API, user-intent classification or approval semantics. A method named GCD is not evidence that these constraints ran. |
| [JSONSchemaBench — Geng et al., 2025, v3](https://arxiv.org/abs/2501.10868v3) | The benchmark separates constraint coverage, generation efficiency and output quality across 10K real-world schemas. Measure all three for the configured classifier/schema combination. | Schema compliance is not task correctness. A provider's schema capability does not eliminate host shape admission, prove every schema keyword is enforced, or distinguish a legitimate empty interpretation from an omitted required field. |
| [Large Language Models Cannot Self-Correct Reasoning Yet — Huang et al., ICLR 2024](https://proceedings.iclr.cc/paper_files/paper/2024/hash/8b4add8b0aa8749d80a34ca5d941c355-Abstract-Conference.html) | Their experiments warn that intrinsic reasoning self-correction without external feedback can fail or degrade answers. Do not justify unconditional retry by assuming another model pass is automatically more reliable. | This is not a theorem that current models cannot repair JSON, nor evidence that deterministic type/path feedback is useless. The task and model populations differ. |
| [CRITIC — Gou et al., ICLR 2024](https://proceedings.iclr.cc/paper_files/paper/2024/hash/fef126561bbf9d4467dbb8d27334b8fe-Abstract-Conference.html) | Tool-derived feedback can support revision on the paper's evaluated tasks. A bounded repair should receive a precise parser/schema diagnostic and be checked again by the same deterministic admission boundary. | Its QA, mathematical-program and toxicity results do not establish that a repaired CodeNERD interpretation preserves user restrictions or that extra repair cost is worthwhile. Those require held-out end-to-end evidence. |

**Derived proposal:** spend no extra model calls on transport causes that another unchanged request cannot fix. For repairable wire faults, use feedback that identifies the violated structural contract without instructing the model which execution decision to invent. Admit repaired output through the same parser, restriction and kernel policies as the first attempt. Compare against a cheaper explicit-blocker baseline as well as the actual current continuation path; do not assume the baseline invariably asks for clarification.

## Proposed outcome contract

The following names are design sketches, not current types or registered predicates. A narrow result can extend the existing adapter and receipt work without a new persistence platform:

```text
ClassificationOutcome
  request or turn identity
  disposition: admitted | ambiguous | invalid_response | provider_failed | canceled
  failure stage: request | transport | output | decode | required_fields
  provider failure class and safe status when relevant
  effective method: generic | schema | compatibility
  attempt references and bounded repair disposition
  normalized interpretation and provenance only when admitted
  safe diagnostic code and compatibility projection reason
```

Keep model confidence separate from outcome validity. A zero confidence can mean a genuine uncertain interpretation; it must not also be the sole identifier for authentication failure, missing JSON or canceled work. A high confidence cannot admit an invalid payload. Preserve transport causes with typed error wrapping or a typed outcome rather than parsing display text back into control state.

Store compact codes, attempt identities and digests by default, under the existing session/receipt retention policy. Do not add full prompts, provider response bodies, credentials or hidden reasoning to a second log merely to make recovery observable. Safe user wording and restricted debugging artifacts are separate outputs.

### Shape admission and normalization

Choose envelope versus bare format from the top-level wire shape before decoding the selected variant. If an explicit envelope is malformed, report that envelope's error; do not reinterpret it as a bare object to obtain a nil error. Permit the bare form only under its documented compatibility profile. Record missing required fields separately from valid defaults, and decide which optional omissions are harmless. Maintain finite numeric bounds and distinguish an explicitly documented percent conversion from ordinary clamping; do not silently interpret a confidence change as stronger evidence.

The shared parser should return an admitted understanding or a typed structural diagnostic. It should not choose the execution lane. Vocabulary observations remain kernel-owned; do not duplicate routing tables or broad natural-language matching in Go. The new work also does not need to solve articulation's entire duplicate-key, streaming and control-admission grammar. Those belong to the linked articulation proposal; reuse its eventual parser-profile primitives where appropriate.

### One classification operation behind every public entry point

Make `ParseIntent`, the history-aware method and GCD adaptation call one underlying operation with explicit capability and policy inputs. A small forwarding repair may be the best first implementation; it still needs the same contract tests as a larger refactor. Do not use the legacy name “GCD” as evidence that a grammar, schema or repair loop actually ran.

Selection must respect the configured client and wrapper capabilities. Schema support, schema rejection, output invalidity and provider unavailability are different observations. The outcome should state which method actually ran. Existing interfaces can continue returning `(legacyIntent, nil)` for compatibility where required, but migrated callers must receive the corresponding typed outcome. Missing outcome provenance in a legacy/preset producer is an explicit compatibility state, not silently an admitted model interpretation.

A specifically recognized unsupported-schema feature may justify a configured same-client free-form compatibility path. A generic HTTP 400 is not sufficient evidence for that transition, and authentication, cancellation or a policy denial cannot be relabeled as a capability miss to reach another path.

Trusted preset intents remain useful: delegated tasks should not reclassify synthetic task text just to satisfy a new receipt. Carry their existing validated/provided provenance and originating outcome reference where relevant. Preserve original request ownership and restrictions rather than treating the outcome as a new authorization grant.

### Mangle chooses the recovery disposition

Go reports transport/parse observations and performs mechanical operations. Mangle derives whether the current request may continue through normal execution, receive clarification, attempt a bounded repair, wait for retry or return a blocker. This is not a second planner in Go.

| Observation | Proposed permissible next step |
|---|---|
| Admitted interpretation with sufficient routing information | Normal policy route under unchanged permissions |
| Admitted but genuinely ambiguous interpretation | Targeted semantic clarification when required |
| Recoverable response-format failure | Documented deterministic normalization or a bounded format-repair attempt |
| Authentication or permanent request/configuration rejection | Safe actionable blocker; no unchanged free-form retry |
| Retryable outage or rate limit | Existing provider retry ownership, then explicit retry/wait outcome under the request's limits |
| Cancellation or elapsed caller deadline | Canceled outcome; no semantic clarification or fresh repair attempt |
| Unknown or unsupported outcome | Explicit conservative disposition, not an invented successful interpretation |

The same outcome must reach the normal TUI, the direct TUI fallback, side turns and session execution. On the feature branch, retain distinct references for primary classification and Jev judgments; do not erase a failed classification because a different observation succeeded, or impose an indiscriminate classifier-success gate on independently admitted stop/cancel control. Different presentations are acceptable; materially different cause identity and continuation authority are not. A repeated input after an outage is a retry attempt, not evidence that a semantic clarification was answered. Preserve the existing anti-clarification-loop behavior for actual ambiguity without letting it erase a provider blocker.

### Bounded repair without hidden retry multiplication

Do not build an unconditional “try three more times” loop around provider retries. Existing adapters already retry selected transport failures. A classification controller should account for all logical attempts and inherit the caller's context, model configuration, input/output caps and cost policy. Its stopping conditions are successful admission, nonrecoverable cause, cancellation, exhausted authorized budget or repeated identical failure without progress.

Any model-facing repair instruction must be a JIT atom/contract, not an ad hoc system prompt. A repair can correct wire format or missing structural fields using the original authorized request; it cannot manufacture user consent, broaden the task or reinterpret quoted repository text as instructions. Retrying authentication, changing credentials, choosing a different provider or increasing configured limits requires the appropriate user/configuration change rather than automatic recovery.

Do not re-enable regex fallback merely by making errors non-nil. The existing compatibility error return also controls whether chat invokes a second transducer. Migration must make ownership explicit before changing those values, or a typed failure could accidentally trigger an extra classification/fallback path. Structural repair should occur before any action is authorized; once work has effects, use the separate action/terminal ownership contracts rather than replaying work to repair perception metadata. The new side-turn error branch also needs a migration check: it says the input is queued when `msg.Err != nil`, but returns without the `enqueue` used by its `/later` branch. The canonical degraded nil-error path does not currently enter that branch; changing the error contract must not turn a preserved diagnostic into a dropped request with a queue claim. That branch also interpolates `msg.Err.Error()` directly, so a migration must preserve safe diagnostic rendering rather than newly exposing raw provider bodies. Verify the queue itself, not just its display text. [Side-turn error and queue branches](https://github.com/theRebelliousNerd/codenerd/blob/777e93c42f3b9ef57212bfac7be0114f3cc4805e/cmd/nerd/chat/side_turn.go#L224-L242).

## Verification plan not executed

Exercise the actual factories, wrappers and method sets before optimizing prompts. Use deterministic clients and recording kernels to isolate contract behavior, then held-out normal-entry journeys to measure whether investigations become more reliable. No live credentials or production workspace effects are necessary for the contract fixtures.

| Fixture | Required observation |
|---|---|
| Wrapped 401/403 on a fresh normal TUI turn | First pin current direct-answer routing and request contents; proposed typed disposition surfaces the safe authentication blocker without an unowned ordinary continuation or unchanged repair call |
| 400/413/422 | Request rejection remains distinct from authentication, ambiguity and outage; configured caps remain enforced |
| 429 or 503 after provider retries | Preserve existing transient reason/question derivation; separately verify the actual route, visible response and all subsequent attempts |
| Successful low-confidence understanding | Still classified as semantic uncertainty, not infrastructure failure |
| Canceled or expired context before/during classification | Canceled terminal outcome; no new schema/free-form repair operation after cancellation is observed |
| Bare wrong type and envelope wrong type | Both produce structural diagnostics; a malformed explicit envelope cannot be redecoded as successful bare understanding |
| Empty/unrelated object and missing routing fields | No default `/explain` treated as an admitted interpretation solely because JSON decoding succeeded |
| Valid bare and valid envelope compatibility cases | Same normalized understanding and documented optional-field behavior |
| Unknown but well-formed vocabulary | Kernel-owned vocabulary handling remains intact; no new Go allowlist rejects it |
| Schema-capable thinking client through every public interface | Actual schema/free-call counts and normalized outcome agree across context, ParseIntent and GCD routes |
| Guarded client versus dedicated classification client | Cost, cancellation and metering remain enforced; method receipt reflects actual capabilities |
| Schema rejection by cause | Authentication/cancellation do not invoke free-form recovery; documented unsupported-schema/format cases follow only the configured recovery path |
| Firewall enabled, disabled and unavailable | Cause/disposition parity across ordinary TUI and direct fallback; no accidental second ownership of repair |
| Session processing without a preset | Failed observation cannot silently enter ordinary work as admitted; an explicit recovery continuation is distinguishable |
| Side turn with authentication/parse failure; Jev disabled or independently successful | Existing safe response stays visible; typed primary failure remains distinct from stop/queue evidence; no unnecessary articulation call |
| Side-turn non-nil error compatibility migration | Queue state and ownership match the displayed promise; the diagnostic stays safe, and failed classification cannot silently drop the original input |
| Trusted preset/delegated task | No redundant model classification; origin and restrictions remain attributable |
| Feature branch: primary classification × Jev success/failure, including delayed completion | Independent causes remain attributable; defined policy precedence and actual join latency are checked |
| Feature branch: held work, failed primary classification, affirmative/negative/unsettled approval judgments | Only the explicitly specified held-work admission path may proceed; failed primary classification is not relabeled successful |
| Feature branch: stop request during primary-provider failure | Independently admitted cancellation can settle owned work without inventing a valid work interpretation |
| Repeat same request after outage, then valid response | Retry remains distinct from semantic clarification; recovered turn does not inherit stale failure state |
| Preliminary read-only investigation and authorized follow-on task | Correct evidence route after recovered admission, with original objective intact and no authority expansion |

Tests must inspect the user-facing TUI result, not only the degraded intent's `Response`; inspect the actual interface call counts, not only a direct helper call. Keep the existing provider conformance, transient-firewall and valid-ambiguity tests as positive controls. Cover yolo and repeated-input variants separately, documenting their policy semantics rather than relying on the normal-mode fixture to claim universality.

## Cost and acceptance criteria

This proposal makes no latency, accuracy or token-saving claim. Typed cause transport is inexpensive relative to a model call in principle, but actual costs include schema/repair prompt tokens, additional model attempts, provider retry delays, kernel derivations, logging/receipt storage, and human time spent diagnosing an omitted cause or taking unnecessary recovery turns.

Measure a resource vector, not a single tool-count proxy: classification logical calls; provider sends and retries; schema and repair calls; input/output/cache tokens; cancellation settlement; total time to first useful source evidence; time to a correct actionable answer; recovery-turn count; configuration-blocker clarity; and invalid interpretation acceptance. Include failures and retries, not only successful turns. Use the existing configured models and equal authority/context ceilings when comparing designs. Keep Jev enabled/disabled and its thresholds matched within each feature-branch comparison; report judgment and Tier 2 relevance costs separately from classification repair. Parallel work consumes tokens and can extend tail latency at its join even when it often overlaps the main call.

Compare the current main baseline against three increasingly broad changes: cause transport and explicit UI/recovery disposition only; entry-point/schema parity with cause-aware fallback; then bounded structural repair. The first two can be justified by deterministic contract fixtures. Run the same ablations against the current feature branch with its brief/work-mode/Jev integrations intact, not an obsolete stripped-down baseline. The repair layer needs held-out evidence that added calls recover more valid tasks than they introduce delay, misleading confidence or permission risk. Label transport/decode/admission disposition independently of intent correctness; stratify by cause and entry path, retain successful and genuinely ambiguous controls, and report paired differences with uncertainty rather than hiding rare false-admission regressions in one mean. A higher parse-success percentage is not sufficient if invalid envelopes are accepted as empty interpretations.

## Implementation queue

| Research ID | Deliverable | Checkable exit |
|---|---|---|
| PERCEPTION-OUTCOME-01 | End-to-end nontransient cause transport | Auth/request/cancellation/schema causes survive normal TUI and direct session paths; transient and genuine ambiguity controls retain their behavior |
| PERCEPTION-OUTCOME-02 | Shape-aware classification admission | Explicit malformed envelopes are not reinterpreted as bare successes; compatible valid responses retain intended semantics |
| PERCEPTION-OUTCOME-03 | Public entry-point capability parity | Schema-capable fixtures exercise the same underlying operation through GCD/context/ParseIntent without bypassing guards |
| PERCEPTION-OUTCOME-04 | Cause-aware bounded recovery | No repair for terminal causes; all attempts owned/accounted; authorized format repair ends in admitted, blocked or canceled outcome |
| PERCEPTION-OUTCOME-05 | Preliminary-investigation value | Matched normal-entry journeys show fewer wasted continuation/recovery turns without weaker authority, evidence or answer correctness |

Start by reconciling the perception, CLI/session and core-policy specs with the chosen outcome contract and the existing Q5 compatibility question. Do not mark the broader perception receipt work or the provider conformance feature unimplemented just because this downstream boundary is incomplete. The first user-visible guarantee is narrow: **a classification failure reaches an explicit, cause-specific disposition—actionable blocker, canceled result or policy-owned recovery—rather than being mistaken for an admitted interpretation or left for a later model to rediscover.**

[root]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/agents.md
[restriction]: https://github.com/theRebelliousNerd/codenerd/blob/b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b/Docs/audits/perception-restriction-continuity.md
[articulation]: https://github.com/theRebelliousNerd/codenerd/blob/b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b/Docs/audits/articulation-admission-and-control-fidelity.md
[evidence]: https://github.com/theRebelliousNerd/codenerd/blob/b6a72db2b8aba54b69e47cb2f6dace9fc7145a0b/Docs/audits/tui-evidence-turn-ownership.md
[capability]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/perception-capability-recovery.md
[provider]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/provider_failure.go#L19-L125
[degraded]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L314-L345
[transient-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/perception_transient_test.go#L33-L87
[questions]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/perception/OPEN-QUESTIONS.md#q5--error-contract-for-parseintent
[provider-card]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/perception/TODO.md#L47-L104
[conformance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/provider_failure_conformance_test.go#L56-L261
[adapter-error]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L418-L431
[firewall-failure]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/perception.go#L498-L595
[clarification]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/clarification.mg#L46-L63
[route-clarify]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/routing_arbitration.mg#L247-L297
[chat-clarify]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L339-L397
[chat-ingress]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L166-L265
[session-observe]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1169-L1245
[parse]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L432-L491
[conversion]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L467-L525
[mapping]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L630-L665
[validate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding.go#L161-L182
[malformed-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter_test.go#L435-L453
[type-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm_test.go#L412-L430
[gcd]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L714-L727
[understand]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L125-L161
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L37-L54
[gemini]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_gemini.go#L70-L182
[gemini-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_gemini_uplift_test.go#L150-L180
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/perception.go#L202-L261

[route-signals]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_routing.go#L158-L188
[route-direct]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/routing_arbitration.mg#L54-L123
[route-fast]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/routing_arbitration.mg#L129-L189
[chat-lanes]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L278-L397
[chat-articulate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L783-L859
[art-payload]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L67-L80
[art-request]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L129-L283

## As-built status

Built 2026-10-28 in the `feat/conversation-brief` worktree (HEAD `ac2f1fd3` plus uncommitted work). The doc's inspected feature head was `777e93c`; the worktree is eleven commits past it, so every claim below was re-read against the current files, not carried over. The research-branch addenda ("Qualify audit against current feature branch") reported the adapter, parser, GCD method, firewall and session executor unchanged from main and the brief, work modes, side turns and Jev integration as adjacent work; all of that held at `ac2f1fd3`. The fixes are in the working tree, uncommitted.

### Claim ledger

| Claim | Verdict | Evidence in the worktree |
|---|---|---|
| `ProviderFailureClass` distinguishes transient, rate-limited, auth, invalid-request, canceled and permanent; cancellation keeps precedence; the safe message hides the provider body | Confirmed, kept and reused. `SafeProviderFailureMessage` now shares one table with the outcome (`safeFailureMessage`) | `internal/perception/provider_failure.go:19`, `:61`, `:99` |
| The degraded adapter returns `/explain` `/query` with a nil error and one transient bit as its only machine-readable cause | Real, fixed. The bit is gone; `Intent.Outcome` carries disposition, stage, provider class, status, method and attempts. The nil-error contract is unchanged | `internal/perception/classification_outcome.go`; `understanding_adapter.go:328` (`degradedClassificationIntent`); `transducer.go` (`Intent.Outcome`) |
| The firewall's `intent_unknown` branch gives authentication, invalid request, parse failure and cancellation `/heuristic_low`, blaming the user | Real, fixed. A failed outcome asserts no `intent_unknown` and no `ambiguity_flag`; it publishes `classification_outcome` | `internal/shards/system/perception.go:548`, `:575-600`, `:683` |
| Fresh full-turn `/explain` with a failed classification routes to `/respond_directly`, not the clarification lane | Confirmed unchanged in the policy (`/explain` is a conversational verb and `wants_direct_answer` derives with no evidence signals). Fixed by ending the turn before any lane acts, not by editing the lanes | `policy/routing_arbitration.mg:67`, `:118-133`; stop at `cmd/nerd/chat/process.go:333` |
| `payloadForArticulation` and the request builder carry neither the safe response nor the cause | Confirmed, and now moot for failures: a failed classification never reaches articulation. Left as is for admitted turns | `cmd/nerd/chat/helpers_articulation.go:67-81` |
| The direct-transducer fallback in chat and the session executor treat a nil error as success | Real, fixed. Chat publishes the outcome on the direct path; `observe` admits through the kernel and returns a typed `*perception.ClassificationStop` | `process.go:279`; `internal/session/executor.go:1388`; `perception.AdmitClassification` |
| The CLI `nerd instruction` has the same hole | Real (the doc did not list it), fixed. The `perception` test command now prints the outcome | `cmd/nerd/cmd_instruction.go:89`; `cmd/nerd/cmd_direct_actions.go` |
| An envelope that fails to decode is re-read as a bare `Understanding`, yielding an all-zero interpretation with a nil error | Real at the worktree head, fixed. The shape is chosen from the top-level keys; a malformed envelope is an error | `internal/perception/classification_admission.go:43` (`parseResponse`) |
| Empty, unrelated or routing-field-less objects decode as `/explain` successes | Real, fixed. `action_type` and `confidence` must be present (presence, not value, so an explicit `0` is a reading) | `classification_admission.go:28`, `:92` |
| `Understanding.Validate` cannot tell an omitted number from a supplied zero, so it must not be called everywhere | Confirmed, and it has no production caller (tests only); admission is presence-aware instead and `Validate` was left alone | `internal/perception/understanding.go:173` |
| Confidence above 1 clamps to 1 | Real and worse than the doc said: a percentage (85) became certainty (1.0). Fixed: above 1 up to 100 is converted as a percentage; anything outside 0 to 100 is refused | `classification_admission.go:92-110`; `transducer_llm.go` (`normalizeLLMFields`) |
| The firewall's `ParseIntentWithGCD` reaches the base context method and bypasses the schema path of the Gemini wrapper | Real, fixed by removing the cause rather than forwarding: `GeminiThinkingTransducer` (an embedding wrapper that could never dispatch virtually) is gone. One transducer type; the capability is read once by `LLMTransducer.methodPlan`, and `ParseIntent`, the history-aware method and GCD all end in `LLMTransducer.Understand` | `understanding_adapter.go:38`, `:734`; `transducer_llm.go:264` (`methodPlan`), `:275` (`send`) |
| The schema attempt falls back to free-form on any error | Real, fixed. Only `core.ErrSchemaNotSupported` (the repository's typed unsupported-schema signal) or a schema answer that is not a classification moves to the free-form attempt. Authentication, cancellation, outages, rate limits and a generic 400 are terminal, with no second send | `transducer_llm.go:159` |
| The guarded client hides capabilities from the transducer | Confirmed, unchanged on purpose: `guardedPerceptionClient` has no thinking or schema methods, so a firewall without a classification client classifies generically and says so in `Outcome.Method`. Forwarding capabilities would route around the cost guard | `internal/shards/system/perception.go:251` |
| Side-turn error branch says "queued" but does not enqueue, and prints the raw error | Real, fixed: it enqueues (except a canceled read) and prints only the safe message. The nil-error path is also fixed: a retryable failure queues the message (policy), a blocker does not | `cmd/nerd/chat/side_turn.go:243`; `policy/side_turns.mg` |
| Jev's success is not evidence the primary classification succeeded; held-work approval has precedence; a stop request needs no interpretation | Confirmed and made structural. `classification_stop` exempts `approve_lane()` (which Jev can derive); `stop_background_requested` reads Jev alone and is derived with a failed classification; a Jev "yes" never clears a failed outcome | `policy/perception_outcome.mg`; tests below |
| The side-turn path already shows the safe diagnostic | Confirmed, and kept: the reply is now the kernel's notice for the cause (same cause, better wording), with no articulation call | `side_turn.go:178` |
| Q5 (error contract) is open | Still open on purpose. The nil-error contract is unchanged; callers migrated to the typed outcome first, as the doc proposes | `Docs/architecture/perception/OPEN-QUESTIONS.md` Q5 |
| Feature-branch join latency, cost baselines, the held-out journeys | Not a code claim; not run (section "Deliberately left") | |

Claims about `777e93c` file identity with main (adapter, Gemini, firewall, clarification, session executor "byte-identical") are historical; those files are exactly the ones changed here.

### What was built

- **Typed outcome** (`internal/perception/classification_outcome.go`). `ClassificationOutcome` (disposition `admitted | invalid_response | provider_failed | canceled`, stage `transport | output | decode | required_fields`, provider failure class and status, method `generic | schema | free_form`, attempts, a fixed-vocabulary code that never holds model output) replaces `Intent.TransientFailure`. The zero value means no model classified the intent (a delegated task's preset, blank input) and projects no fact: not read is not an observation and not an admission. Model confidence stays a separate field. `ClassificationError` is the typed structural fault the parser returns.
- **Kernel disposition** (`internal/core/defaults/policy/perception_outcome.mg`, new). `classification_outcome/5` is the base fact. Derived: `classification_failed`, `classification_action` (`/retry` for an outage, a rate limit or an unreadable answer; `/blocker` for authentication, a rejected request or any other failure, and for any outcome the file does not know; `/canceled`), `classification_notice` (the wording), `classification_stop(Intent, Action, Text)` (the turn ends here; excludes `approve_lane()`), and `next_action(/interrogative_mode)` for retry and blocker. `clarification.mg` lost the `/llm_unavailable` and `/llm_failed` clarification rules: they described an infrastructure failure as a reason the intent was unknown, and the outcome now carries the cause. `intent_unknown` is again only about a request that was read.
- **Publishing and asking** (`classification_outcome.go`): `PublishClassificationOutcome` replaces only the intent's row (delegated tasks use task-scoped ids), `ClassificationStopFor` asks the kernel, `AdmitClassification` does both for callers without their own routing. A kernel that cannot be asked leaves a failure standing with the safe message.
- **Callers**: the firewall publishes the outcome and skips ambiguity and autopoiesis failure tracking for a failed classification (`perception.go`); chat publishes on the direct path and ends the turn with the kernel's notice (`cmd/nerd/chat/classification_stop.go`, two surgical hunks in `process.go`); the session executor, `nerd instruction` and the side-turn path use the same outcome. The routing policy and `delegation_routing.go` are untouched: the stop runs after `decideRoute` (so `work_proposal_pending` and Jev's judgments are in place) and before the hold and approve branches act.
- **Admission** (`classification_admission.go`): shape by top-level keys, strict envelope, presence-aware required fields, percentage conversion. Vocabulary stays the kernel's; no Go allowlist.
- **One operation** (`transducer_llm.go`, `understanding_adapter.go`): `GeminiThinkingTransducer` removed; the schema constant and `schemaCapableClient` live in `classification_schema.go`. `LLMTransducer.Understand` returns `(*Understanding, ClassificationOutcome)` and owns the bounded method plan: at most schema then free-form, at most two sends, counted in `Outcome.Attempts`.

Jev as a classification fallback: **not built, deliberately.** The judgments answer fixed yes/no questions about the message ("wants_to_stop", "approves_held_work"); none can produce a verb, target or restriction, so none can stand in for an interpretation, and the doc says the same (a successful judgment is not evidence the primary classification succeeded). What Jev can do without an interpretation it already does through the kernel, and the policy keeps both: held-work approval (`approve_lane`) and stopping background work (`stop_background_requested`). The side-turn `/stop` likewise reads Jev alone.

### Tests

- `internal/perception/classification_outcome_test.go`: `TestParseResponse_AdmitsOnlyClassifications` (the doc's mistyped-envelope witness, empty and unrelated objects, omitted versus explicit-zero confidence, percentage, out of range, unknown vocabulary still admitted), `TestParseResponse_BareAndEnvelopeAgree`, `TestFailedOutcome_ClassifiesEveryCause`, `TestDegradedIntent_CarriesTheTypedCauseAndAHonestResponse`, `TestEveryEntryPointRunsTheSameSchemaOperation` (ParseIntent, context method and GCD each make exactly one schema call), `TestSchemaFailureTransitionsAreCauseAware` (auth, cancellation, outage and a generic 400 make no second send; `ErrSchemaNotSupported` and an unreadable schema answer fall to free-form; both unreadable ends after exactly two attempts), `TestMalformedEnvelopeIsNotAnAdmittedInterpretation`, `TestKernelDerivesTheDispositionFromTheCause`, `TestKernelStopsAnOutcomeItDoesNotKnow`, `TestPublishReplacesOnlyTheIntentsRow`, `TestJevEvidenceIsIndependentOfAFailedClassification` (Jev yes does not clear the failure; Jev-read go-ahead on held work still wins; pending work alone does not; a Jev stop is derived beside a failed classification), `TestAdmitClassificationWithoutAKernelCannotClearAFailure`.
- `internal/shards/system/perception_transient_test.go`: the firewall positive control for the outage (outcome row, `/retry`, outage wording, no `/heuristic_low`, no `ambiguity_flag`), `TestPerception_NonTransientFailures_AreTypedAndDistinct` (auth, rejected request, rate limit, unreadable answer, mistyped envelope, canceled), and the low-confidence control (admitted, not stopped).
- `cmd/nerd/chat/classification_stop_test.go`, through the real `processInput`: auth, outage and unreadable answer end the turn with the cause's notice and call neither articulation nor a task executor; admitted and preset controls are not stopped; a Jev-read go-ahead during a failed classification still reaches the approve lane; pending work alone does not clear it; a retry after an outage is a fresh turn and is not recorded as a clarification; side turns (retryable failure queues and says so, a blocker does not, Jev's stop survives a failed classification, the error branch keeps its queue promise and leaks no body).
- `internal/session/observe_classification_test.go`: a failed classification ends `Process` with a typed `*ClassificationStop` (`/blocker`, `/retry`, `/canceled`); admitted and unclassified intents pass.
- `internal/core/yolo_policy_test.go` `TestYoloModePreservesErrorNotices` now pins that `classification_notice` and `classification_stop` hold under yolo. `cmd/nerd/chat/routing_performance_contract_e2e_test.go` carried a retired `control_packet` fixture that was never a classification and only passed because an answer with no routing fields decoded as a default `/explain`; it now carries the current envelope (a live instance of the doc's empty-object finding).
- Existing perception tests were updated where they pinned what this removes: the clamp test is now the percentage test, the null-understanding test is now the invalid-envelope test, the Gemini tests exercise the thinking path of the one transducer.

Run: `go test -p 4 ./internal/perception/ ./internal/shards/... ./internal/session/ ./cmd/nerd/chat/ ./internal/core/defaults/` (the literal gate is red for a reason outside this work, below), `go test ./internal/shards/ -run ShardJoin` (green; no sharing change was needed), and `go run ./cmd/tools/predicate_corpus_builder` (run; prompt atoms were not touched, so `prompt_builder -reconcile-embedded` was not).

### Deliberately left, with reasons

1. **Bounded structural repair (PERCEPTION-OUTCOME-04, the feedback loop): not built.** What is built is the cause-aware part: no repair or second send for terminal causes, every send counted, a bounded plan that ends in admitted, a typed failure or canceled. A model-facing repair instruction must be a JIT atom, and the doc itself says the repair layer needs held-out evidence that added calls recover more tasks than they delay or mislead; the first two layers are justified by deterministic fixtures and the third is not yet. The decision is cheap to revisit: `methodPlan` is the single place a repair step would join, and `Outcome.Attempts` already accounts for it.
2. **Value measurement (PERCEPTION-OUTCOME-05) and the cost baseline, including Jev join latency: not run.** They need matched live journeys with equal configured models and no model calls were made.
3. **`transducer_gemini.go` file name:** closed; renamed to `classification_schema.go` at commit time.
4. **Unsupported-schema detection for providers that report it as an HTTP 400** is not recognised: only the typed `core.ErrSchemaNotSupported` is. The doc says a generic 400 is not evidence; a provider-specific recogniser belongs with that provider's adapter and its conformance entry.
5. **`literal cmd/nerd/chat` 191 against 190:** closed. The extra literal was `len(steps) != 2` in `anchorVerbPairToIntent` (the live-journey commit); it is now the arity of the `verbPair` type, and the budget is back at 190.
6. **Q5** (should `ParseIntentWithContext` ever return an error) stays open; the typed outcome is what makes either answer safe to take later.

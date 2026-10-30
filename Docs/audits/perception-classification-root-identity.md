---
doc-class: deep-dive
subsystem: perception
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 8d373fb6ca05ef03fec59b923c5f1925066d623a
verification-method: static-source-inspection
supersedes: []
---

# Perception extraction must preserve the classification envelope

**Finding:** the repaired classification shape discriminator runs after an extractor that can replace a malformed or unfinished envelope with one of its complete nested objects. If that nested object contains `action_type` and `confidence`, it can be admitted as the supported bare classification profile. The subsequent typed failure gate then sees an admitted interpretation rather than the enclosing response's structural failure.

**Recommendation:** preserve root ownership and attempted-envelope failure through classification extraction. Do not reinterpret a nested fragment of a failed envelope as an independently emitted bare classification, and do not silently fall back to an earlier example when a later attempted classification is incomplete. Keep the valid bare profile, declared thinking/preamble compatibility, cause-aware method fallback, and kernel-owned routing.

This is a static source finding, not an observed production incident or executed reproduction. No build, test, application run, model inference, failure injection, benchmark, or implementation edit was performed. The examples are proposed fixtures and source-derived outcomes, not passed tests. Sources were initially inspected at `d490b08dc891d00087525f4d521ac7caa99a5c83` and revalidated on main at [`8d373fb6ca05ef03fec59b923c5f1925066d623a`][baseline] around 07:12 UTC on 2026-10-30. The relevant parser, transducer, adapter, typed-outcome, chat-stop, policy and test files were unchanged between those pins.

## Primary decoder reference

[Go’s JSON decoder documentation](https://pkg.go.dev/encoding/json#Unmarshal) describes how duplicate keys and struct fields are decoded. It informs the adjacent duplicate-member analysis below; it does not prove that any particular model output occurs in production.

## Relationship to completed work

The [classification outcome audit's as-built section][prior-built] records a real repair: when the selected JSON object explicitly contains `understanding`, `parseResponse` selects the envelope profile before decoding it. A wrong-type field in that envelope is no longer retried as a bare understanding that ignores the nested content. Presence-aware `action_type` and `confidence` checks also reject empty and unrelated objects; explicit zero confidence remains a valid uncertain reading, percentages are normalized rather than clamped, and vocabulary remains the kernel's responsibility. Preserve all of that.

This residual is earlier in the pipeline. It is an incompatibility between the older, deliberately tolerant extractor and the newer strict classification admission contract, not a newly discovered extraction behavior. The prior witness used syntactically valid JSON whose envelope had an invalid field type. This witness uses a malformed or unfinished enclosing object with a valid nested classification. The original root is discarded **before** the fixed shape discriminator sees it. The result need not be the old all-zero interpretation: a complete nested `modify` classification remains a nonzero mutation interpretation.

The separate [articulation admission audit][articulation-audit] and [implemented articulation scanner][articulation-scanner] concern display/control envelopes and unique decoded keys. They do not mediate perception's `ExtractCleanJSON` call. This report does not reopen the repaired streaming completion, no-control-reparse, or memory disposition work.

The inspected `research-main-20261030-0544`, `0614`, and `0642` branches were checked before main advanced. Their additional audits covered prompt screening, self-restart, verifier change evidence, policy reload, JIT strategy ownership, recurse contracts, deliberation budget, and knowledge-synthesis answer identity. At the final pin those audits had been merged and branch search returned no research branches. No inspected audit described this pre-discriminator nested-fragment admission.

## The source chain

### Extraction discards the enclosing structure

`parseResponse` begins with `jsonStr := ExtractCleanJSON(response)`. Only afterward does it decode `jsonStr` into a map and inspect its `understanding` key to choose envelope or bare mode. The required-field and confidence checks operate on that selected object, not on the original response. [Classification admission, lines 43–85][admission]

`ExtractCleanJSON` tracks strings, escapes, and container starts. On every closing brace or bracket it records a balanced candidate, **including nested candidates**. It then walks candidates backward and returns the first one for which `json.Valid` succeeds. The return type is a string: original depth, parent container, selected span, and the reason an enclosing candidate failed are not carried to admission. [Extractor, lines 570–655][extractor]

This behavior is intentional for the generic extraction helper. Existing `TestExtractJSON` fixtures require both `Valid inside Invalid` and `Valid followed by Invalid`; they also require selecting the last of multiple complete JSON objects. Those tests are positive evidence of the tolerant helper contract, not evidence that its output is a complete classification response. No tests were run for this review. [Extractor fixtures, lines 17–117][extract-tests]

### A malformed envelope can become a valid bare classification

Proposed fixture A is a syntactically invalid outer envelope with a trailing comma:

```text
{"understanding":{"action_type":"modify","confidence":0.9,"scope":{"target":"a.go"}},}
```

Source-derived steps:

1. The extractor records the complete `scope` object, the complete nested understanding, and the enclosing object.
2. The enclosing object fails `json.Valid` because of its trailing comma.
3. The next candidate is the nested understanding, which is valid JSON.
4. That candidate has no `understanding` wrapper key, so `parseResponse` selects the bare profile.
5. `action_type` and `confidence` are present and valid. The result is admitted, with target `a.go`.

Proposed fixture B omits only the final enclosing brace:

```text
{"understanding":{"action_type":"modify","confidence":0.9,"scope":{"target":"a.go"}}
```

The inner understanding is closed, but the envelope is not. No complete root candidate exists; the inner candidate can again win. This fixture does **not** claim that every partial JSON response is admitted. If no complete candidate supplies the required classification fields, admission rejects it. A cut that leaves only a `scope` object complete does not meet the required-field check.

Nor is this a claim that provider-declared truncation is accepted. The witness requires text to reach `parseResponse` after `t.send` returns nil error. A provider can successfully end a generation that contains malformed JSON, independently of network truncation or a reported output-limit finish. A transport error takes the existing typed failure route before parsing. [Send and parse ordering, lines 146–195][understand]

### Earlier examples can survive a failed final attempt

The same extraction rule can select an earlier complete object when a later attempted response is incomplete:

```text
Example: {"action_type":"explain","confidence":0.9,"scope":{"target":"example"}}
Actual: {"understanding":
```

The earlier bare-shaped example is valid and the later root contributes no complete classification. The extractor can return the example; shape admission has no provenance with which to distinguish it from the intended final answer. This is an adjacent witness of the same root-selection defect, not a reason to abolish legitimate thinking preambles. The current free-form thinking wrapper explicitly asks for the final JSON object after preceding thoughts. Compatibility should preserve that declared structure without turning a failed final object into permission to reuse an earlier example. [Thinking wrapper and method selection, lines 241–280][method-plan]

## What the admitted fragment changes

After successful parsing, `Understand` sets `admittedOutcome`, then derives routing from the selected understanding. `ParseIntentWithContext` takes its degraded-intent path only if `outcome.Failed()`; otherwise it converts the understanding and copies the admitted outcome into the intent. [Outcome assignment][understand] [Intent handoff, lines 422–447][adapter]

For fixture A or B, `modify` maps to `/fix`, and the mutation action set yields `/mutation`; the target comes from the nested scope. The existing classification stop returns immediately when `Intent.Outcome.Failed()` is false. It cannot recover a malformed root that extraction has already hidden. [Intent mapping][mapping] [Mutation category][category] [Classification stop, lines 250–260][stop]

**Established boundary:** malformed enclosing transport text can supply an admitted nonzero interpretation and continue to ordinary routing rather than produce the typed invalid-response disposition. **Not established:** that a real model emits this fixture, that delegation always occurs, that a write executes, or that a permission or verification gate is bypassed.

Actual effects still depend on the derived route, available candidate, confidence threshold, work mode, held work, task verifier, effective tools, and exact constitutional checks. For a controlled future mutation witness, use a named target and a recording executor; do not infer execution from `/fix` alone. A read-only witness can first establish the admission error without any effects.

## Safeguards to preserve

- Keep shape-aware field decoding. A valid root with a mistyped envelope field must remain an error rather than re-enter the bare decoder.
- Keep valid bare classifications, explicit zero confidence, percentage normalization, and kernel-owned vocabulary handling. The existing [classification admission cases][admission-tests] are useful controls.
- Preserve cause-aware transport handling. Authentication, cancellation, rate limits, and outages must not trigger a new representation as a speculative remedy.
- Preserve the existing method plan: a supported schema attempt may move to free-form for an unreadable answer; generic/free-form failure is terminal under that plan. Do not add an unbounded repair loop. [Method handling][understand]
- Keep typed invalid-response publication and the kernel's retry/blocker/cancellation disposition, including the independent held-work approval and stop semantics. [Outcome policy][policy]
- Keep any independently recognized Jev restriction and inherited request restriction. This finding is not evidence that those controls disappeared, or that the malformed fragment grants authority.
- Do not globally weaken or silently redefine `ExtractCleanJSON` for every consumer. Its tolerant extraction behavior is explicitly tested. Classification needs a stricter admission boundary or an explicit extraction profile, with callers inventoried before changing a shared helper.

## A focused repair contract

Update the perception specification first, then make classification extraction return enough structural information to enforce it. The implementation can be small; a universal receipt platform is not required.

1. Identify the response root under the selected input profile. A legitimate bare classification is an independently emitted root, not an arbitrary descendant of a failed envelope.
2. Preserve attempted-envelope identity through syntax failure. An incomplete or malformed selected envelope produces a typed structural diagnostic; its descendants are not alternative root candidates.
3. For thinking/preamble compatibility, explicitly specify how the final classification is selected. A later failed attempted classification must not silently expose an earlier example as the answer. Keep valid preambles and complete final roots working.
4. Apply existing envelope/bare decoding and required-field checks only after that root decision. Parsing observes structure; it does not choose an executive route.
5. Feed rejection through the existing `ClassificationError` and `ClassificationOutcome` flow. Reuse its bounded method recovery and truthful final notice rather than inventing a new Go routing fallback.

A strict whole-response JSON parser is an option for schema responses, but applying it indiscriminately to the documented free-form thinking profile would be a regression. Conversely, returning only the last valid JSON substring cannot establish that the final classification was complete. Make the compatibility boundary explicit and measurable.

Duplicate keys are an adjacent admission question, not this report's primary new defect. Current `break_test.go` already contains a logging-only duplicate-key probe. Later explicit `user_constraints: []` or `requires_confirmation: false` can replace earlier values, but Go's decoding of repeated object-valued struct fields can merge fields; a later empty object or null must not be described as generally clearing earlier booleans. A separate uniqueness change should define decoded-key/case identity and compatibility rather than being slipped into the extraction repair. [Existing duplicate probe][duplicates]

## Costs and fair evaluation

The expected trade is more honest rejection of structurally incomplete responses in exchange for losing some permissive salvage. There is no measured speed, reliability, token, or cost improvement here. A root-aware scan adds host parsing/provenance work and possibly an additional validation pass. Rejected schema answers may use the already-allowed free-form attempt; rejected generic answers may require a new user turn. Count those costs rather than calling a fast but misidentified interpretation efficient.

Evaluate unchanged baseline and the focused root-aware treatment on identical scripted responses through normal perception and chat entry points, with the real policy kernel and a recording/no-effect executor. First demonstrate classification and disposition; live-model incidence and task-quality trials would be separate authorized work.

| Fixture | Required observation |
|---|---|
| Valid full envelope | Same normalized understanding and admitted outcome |
| Valid independent bare profile | Compatibility retained |
| Complete inner understanding, missing outer brace | No nested-fragment admission; typed structural result |
| Outer trailing comma with complete inner understanding | Same protection as incomplete outer root |
| Only `scope` complete before truncation | Still rejected; no false claim that every fragment admits |
| Earlier valid example, incomplete final envelope | Earlier example is not silently promoted |
| Thinking preamble and complete final classification | Declared compatibility works without blanket rejection |
| Wrong-type field in valid envelope | Existing shape repair remains intact |
| Explicit zero and percentage confidence | Existing meanings remain intact |
| Schema invalid then free-form valid | Existing bounded fallback succeeds once; attempts recorded |
| Both allowed methods invalid | Invalid-response notice; no ordinary worker dispatch |
| Provider auth error, cancellation, or declared truncation | Existing transport disposition preserved, not reparsed |
| Held-work approval and independent stop | Existing policy precedence preserved |

Capture selected span/root provenance, structural outcome, method and attempt count, emitted intent, kernel disposition, actual downstream dispatch count, and user-visible response. Measure host parse time and allocations, input/output tokens, schema fallback calls, follow-up turns, latency, and useful task outcomes. Report rejection of valid compatibility responses alongside admission of invalid ones. Do not rely only on a direct `ExtractCleanJSON` assertion or treat two malformed cases as proof of general model robustness.

**Checkable exit:** a malformed or incomplete classification envelope cannot become an admitted bare interpretation by losing its enclosing structure; an earlier example cannot silently replace a failed final classification; valid supported profiles and existing policy/permission controls retain their behavior.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/8d373fb6ca05ef03fec59b923c5f1925066d623a
[prior-built]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/perception-classification-outcomes-and-recovery.md#as-built-status
[articulation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/articulation-admission-and-control-fidelity.md
[articulation-scanner]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/articulation/admission.go#L11-L26
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/classification_admission.go#L43-L85
[extractor]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/transducer_llm.go#L570-L655
[extract-tests]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/transducer_llm_test.go#L17-L117
[understand]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/transducer_llm.go#L146-L220
[method-plan]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/transducer_llm.go#L241-L280
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/understanding_adapter.go#L422-L447
[mapping]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/understanding_adapter.go#L494-L545
[category]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/understanding_adapter.go#L590-L682
[stop]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/classification_outcome.go#L250-L260
[admission-tests]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/classification_outcome_test.go#L19-L95
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/perception_outcome.mg
[duplicates]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/perception/break_test.go#L409-L425

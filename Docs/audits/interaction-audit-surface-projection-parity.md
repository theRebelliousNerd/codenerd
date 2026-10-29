# Interaction audit surface projection parity

*Scoped addendum to [articulation-admission-and-control-fidelity](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/articulation-admission-and-control-fidelity.md); source inspection only.*

**Baseline:** main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, unchanged at the 14:49 check. Latest research `f44ac08` adds residual-generation and retry-JIT documentation; no exact prior audit of this consumer was located. This assessment used source inspection only; no runtime execution, model inference, tests, or benchmarks were performed, and no observed production incident is claimed.

At 14:52, `visible_task_output.go`, `tracing_client.go`, `articulation/admission.go`, and `articulation/emitter.go` were byte-identical to main on active heads `a53cfc308e88`, `80af1863d3a3`, and `aa7988dec4c6`.

## Finding and ownership

The existing articulation audit owns stream/final framing consistency and plain/surface/control boundaries. This addendum concerns a different consumer: the durable, per-provider interaction record’s `Response` projection. It is not another proposal to repair display admission, nor a claim that every provider record should equal the final UI transcript.

Articulation accepts a single JSON object, optionally Markdown-fenced, with decoded, case-insensitive key identity; duplicate keys are ambiguous. Its processor permits Markdown wrapping, and inspected admission tests include uppercase and mixed-case surface keys. [Admission grammar](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/admission.go#L11-L27), [processor defaults](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/emitter.go#L181-L190), [case fixtures](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/admission_test.go#L87-L106).

The audit helper implements a narrower interpretation. `visibleTaskOutput` first unmarshals the entire input into `map[string]json.RawMessage`. On failure, exact textual control/reasoning markers cause an empty return; otherwise it returns the original input. On success, only the exact lowercase `control_packet` key selects `surface_response`; only exact top-level reasoning keys are removed. Unchanged input is returned raw. Thus accepted framing can erase the audit surface, while accepted key aliases can leave the serialized envelope in its place. [Complete helper](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/visible_task_output.go#L1-L47).

The defect is projection disagreement, not evidence of an action-permission bypass, actual secret disclosure, or incorrect task completion.

## Public synthetic witness

Use this complete, public synthetic terminal response, with no native tool calls or envelope tool requests:

````text
```json
{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[],"memory_operations":[]},"surface_response":"The comparison is complete."}
```
````

The witness concerns compatibility forms the host processor deliberately accepts, not a claim that a provider enforcing strict JSON schema will emit fences or aliases.

The control structure follows the existing admission fixture; the supplied category and verb also avoid dependence on permissive control validation. Use an informational task whose completion does not require tools, not an initial planning answer for a mutating task. [Fixture](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/admission_test.go#L11-L11), [JSON parsing and validation](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/emitter.go#L620-L671), [terminal tool guard](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor_tools.go#L134-L168).

Source predicts two useful comparisons:

1. **Canonical versus fenced:** the unfenced lowercase object projects to `The comparison is complete.` The fenced object is accepted by articulation’s wrapper parser, but the audit helper’s whole-input JSON decode fails and its exact `"control_packet"` check returns `""`. [Wrapper parser](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/emitter.go#L674-L698).
2. **Canonical versus case alias:** remove the fence and change only `surface_response` to `SURFACE_RESPONSE`. Articulation permits this tested alias; the audit helper finds lowercase `control_packet` but misses the surface, returning empty. Uppercasing both outer keys instead makes it miss `control_packet` and return the whole JSON object. Escaped spellings of canonical lowercase keys are a positive control: JSON map decoding already resolves those escapes.

These are deterministic fixture expectations, not predictions that a live provider will emit either spelling. The terminal example additionally assumes that this response survives subsequent processing and is not superseded by another provider round. It does not assert that global completion, conversation history, or the displayed answer disappears.

## Production route and limits

The factory opens workspace `.nerd/knowledge.db` and wraps the main client in `NewTracingLLMClient` when the store opens successfully. [Factory wiring](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L1106-L1113).

The working executor invokes `CompleteWithToolResults`. Its tracing wrapper captures `resp.Text` and constructs the trace before returning the response. `newReasoningTrace` assigns `Response = visibleTaskOutput(response)` and `Success = err == nil`. That success flag describes the provider-call error result, not admission or whole-turn completion. [Executor call](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1814-L1818), [wrapper](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/tracing_client.go#L411-L442), [trace construction](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/tracing_client.go#L320-L340).

The factory adapter forwards storage; the actual LocalStore reflection adapter copies `Response` and delegates to the raw insert, which persists that value. There is no corrective surface read on this route. [Factory adapter](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory_adapters.go#L27-L40), [LocalStore adapter](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_verification.go#L170-L226), [raw insert](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/trace_store.go#L170-L198).

Persistence is conditional: a nil store is a no-op, writes run asynchronously, and failures are logged. Database-open failure, write failure, or process shutdown may mean no durable row exists. The claim applies only when the traced route and successful write are established. [Async storage](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/tracing_client.go#L343-L353).

Independently, terminal handling admits controls and forms `Result.Response` from the surface plus possible safety notices and step reporting. Admission may already have happened through promotion; the repair must preserve one-response control application rather than assume the tail is always the first parser. [Terminal result](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1455-L1461), [admission guard](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L2392-L2406), [promotion](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/piggyback_promotion.go#L33-L74).

The production TUI caller invokes the articulation helper with `m.client`; shared boot obtains `cortex.LLMClient` and supplies it as the client. [Caller](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L965-L975), [client source](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L100-L110), [client assignment](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L302-L310). When this uses the traced streaming method, tracing constructs its record from accumulated raw content before publishing terminal completion; the helper independently waits for success and processes that raw response into a surface. This does not establish every alternate adapter’s behavior. [Streaming trace](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/tracing_client.go#L214-L246), [articulation helper](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_articulation.go#L303-L393).

Native thinking already has a distinct boundary, covered by an existing inspected audit test. Standard taxonomy learning uses final conversation history; manual `/learn` uses UI history. The reflector’s selected fields and descriptor omit `Response`. Accordingly, this addendum does not claim routine learning poisoning or downstream bad decisions. [Native-thinking fixture](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/audit_privacy_test.go#L23-L50), [taxonomy queue](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor_learning.go#L247-L294), [manual learning](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers.go#L587-L604), [reflector selection](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/trace_reflection.go#L15-L31), [descriptor](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/trace_reflection.go#L288-L326).

## Required contract and bounded repair

Give the provider interaction record a semantically typed `ResponseView`, not an unqualified claim to be a UI transcript. For a declared envelope role, projection must agree with accepted framing and decoded key identity. A valid envelope yields its admitted surface; malformed or ambiguous input yields an explicit admission/projection status, not an unexplained empty field or raw control-bearing fallback.

Keep transport success separate from statuses such as `unknown`, `pre-admission`, `surface`, and `admission-failed`. This distinguishes a legitimate empty provider response from absent or rejected projection without redefining `Success`. Intermediate provider calls remain distinct from terminal responses, and host-added notices remain outside the provider surface unless separately identified.

Prefer a shared, pure, bounded, side-effect-free admission/projection primitive, or an immutable sidecar produced by the response’s existing admission owner. A sidecar requires a response-identity and late-admission commit handshake; it cannot simply attach the last admitted answer to whichever asynchronous trace writes next. Resolve dependency cycles in a shared layer rather than introduce another parser with similar-looking rules.

Do not move control application into the tracer. Preserve `ControlsApplied`, tool permission checks, and native provider blocks and replay. Do not parse an already displayed surface a second time: ordinary structured JSON and prose containing envelope examples need role, mode, and provenance, not key-presence guessing. If transport diagnostics are needed, make them a separate, bounded, explicitly governed record rather than silently retaining raw responses as projection fallbacks.

Existing empty rows cannot reconstruct an erased surface. Classify legacy unknown or incompatible records without fabricating content or deleting legitimate plain payloads. Migration is optional and must follow the operator’s retention policy; no additional embedding or full raw-content receipt is required.

## Alternatives, costs, and risks

- **Keep the heuristic:** cheapest immediately, but leaves accepted syntax with different durable meanings. Human postmortem accuracy remains the relevant, unmeasured cost.
- **Patch fences and case locally:** a small repair for these witnesses, but continued duplicate-key, framing, and role drift remains likely enough to require explicit parity tests. It should not become a second admission policy.
- **Share typed projection:** broader integration work, but one explicit contract. Budget bounded linear parsing/allocation, status storage, and any sidecar synchronization. Preserve asynchronous failure semantics and avoid coupling provider latency to unnecessary storage acknowledgment.

No extra model or embedding call is needed; no latency or storage saving is measured. Correct fenced surfaces may increase answer bytes; avoiding raw aliased envelopes may reduce control bytes. Compare meaningful content and total bytes, not merely smaller logs.

Principal risks are role misclassification, double-parsing legitimate examples, losing intentionally retained partial diagnostics, import cycles, retry identity mistakes, and expanding raw retention without an owner. Preserve useful diagnostics through explicit status and authorized transport records, not by weakening surface semantics.

## Unrun validation plan and acceptance boundary

Use a scripted provider and synthetic payloads, not a live-model emission bet:

1. Feed canonical, fenced, mixed-case, and escaped-key equivalents of the same valid envelope. Require the same surface and status wherever admission treats them as equivalent.
2. Cover duplicate keys, malformed JSON, trailing objects, and partial envelopes. Tracing must neither manufacture an admitted surface nor apply controls. Preserve legitimate plain JSON and prose examples under their actual roles.
3. Exercise the current trace-to-LocalStore route and a no-tool terminal executor path, checking both the persisted projection and independently admitted result. Cover the production factory wiring as well as an injected client; distinguish intermediate calls from retained terminal responses.
4. Keep native plain text plus thinking unchanged, including provider replay signatures. Verify ordinary reads and a valid tool call retain their existing permission and control behavior.
5. Exercise multiple calls and retries with distinct response identities, nil stores, failed writes, and deterministic async draining. Report absent writes honestly rather than interpreting them as empty successful audit records.
6. Test input bounds and UTF-8 handling; record latency, allocations, record bytes, and artifact correctness without claiming benchmark improvements in advance.

Existing canonical-envelope and partial-control learning fixtures are useful regressions but do not establish fence or case parity. [Inspected fixtures](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/audit_privacy_test.go#L83-L107).

Acceptance is narrow: a successfully stored, role-identified provider response must accurately represent its admitted surface or explicitly state why that projection is unavailable. It need not become the final UI transcript, change control authority, or establish broader privacy or learning claims.

---
doc-class: deep-dive
subsystem: deliberation
implementation-status: implemented
evidence-status: source-only
last-verified: 2026-10-30
verified-against: fa6877b932322bb6ca75826289366836d442c051
verification-method: static-source-and-test-definition-inspection
supersedes: []
---

# Deliberation terminal observations must fit the admitted context

## Decision and scope

Preserve the new evidence-closing behavior, but make terminal-control capacity part of deliberation context admission. The role currently accepts an encoded frame up to its full configured byte bound, then can append an evidence-closed refusal or an empty-answer nudge without checking that the new record fits. On the next iteration the role fails its byte-bound check instead of receiving the promised tool-less opportunity to answer.

This is a residual in the recently repaired path, not a claim that the original budget fix is absent. It has a simple conditional source witness: an admitted frame with less room than one terminal observation, followed by one normal offered read request or one empty response. No huge tool batch, oversized identifier, failing tool, concurrency, malicious workspace content, or changed provider behavior is needed.

The source pin is public main [`fa6877b932322bb6ca75826289366836d442c051`][baseline], revalidated at approximately 06:39 UTC on October 30, 2026. This review read public source and existing test definitions only. It executed no repository program, tests, model inference, benchmark or reproduction. The witness and evaluation below are proposed, not observed runtime results. The configured bound is an encoded role-input bound in this implementation; this report does not establish a bound on the entire provider request including system prompt and tool definitions.

## Primary contract reference

[Go’s JSON encoder documentation](https://pkg.go.dev/encoding/json#Marshal) specifies encoding behavior including escaped string representations. The practical implication here is to count the actual serialized payload, including control metadata, rather than assuming raw character counts equal encoded bytes.

## Credit the repairs already on main

Three recent commits address separate live failures:

- [`77feafc69`][schema-fix] centralizes native tool schema rendering so empty required lists do not become provider-invalid nulls.
- [`d805e64d1`][budget-fix] counts evidence executions, closes evidence instead of immediately failing a spent tool budget, withholds tools on the following request, and visibly fits successful read output to the remaining encoded input capacity.
- [`4a7b7e654`][nudge-fix] gives an empty role reply one tool-less nudge before declaring that the role returned no answer.

The [live-exercise ledger][ledger] records four live runs surfacing defects and a fifth run converging in two rounds. That is relevant positive evidence reported by the repository owner, not an independently rerun evaluation here. Generic older runtime-validation-pending prose must not erase these documented runs. None of these changes should be reverted. They cover useful ordinary cases; the remaining admission gap concerns frames that have already consumed nearly all of the same capacity.

The current [agent tests][agent-tests] cover a five-call response against a budget of three; JSON-escaping growth in a large read; four reads after a full read; and an empty reply with a tiny request. Those definitions do not combine a near-bound initial frame with either terminal-control branch.

## Production route

The chat adapter constructs the deliberation service, marshals the complete frame and passes it to the shard manager. The specialized shard invokes `deliberation.Agent` with the configured `DeliberationContextBytes`. There is no control-space reservation in these adapters. [Chat handoff][chat-handoff], [role execution][shard]

The conductor constructs a frame from the original request, intent, brief and answer, includes the previous completed round when present, and appends existing probe receipts. Before the challenger it adds the proposed candidates and specialist consultation advice. It bounds candidate and consultation counts, not the encoded byte size of their strings. These fields can therefore consume different amounts of the role's input budget. This observation explains reachability; it is not an assertion that every ordinary frame approaches the default bound. [Frame assembly][round]

Routing configuration defaults the context bound to 64 KiB and validates it as a positive integer. The narrow finding does not require a change to this default or an invalid configuration. A proposed deterministic test can choose a valid frame and an admitted bound just above its encoded size; a default-bound fixture can instead pad an ordinary frame string until its serialized size falls in the same near-bound interval. Padding must be in a JSON string field, not insignificant raw JSON whitespace. [Configuration][routing]

`Agent` wraps the frame and accumulated observations, serializes that payload, and rejects only when its encoded length exceeds the configured bound. It then offers the read tools unless evidence was previously closed or the execution budget is spent. The offer decision does not inspect whether a refusal or nudge can still fit. [Admission and offer][admission]

An error from the role propagates through round collection. The service then marks and saves the thread as failed. The established impact is a conditional failed deliberation before a closing answer, not an edit escaping safety controls or an incorrectly approved design. [Round errors][round], [Failure persistence][failure]

## Conditional witness A

Let `B` be the configured role-input byte bound and `L` the length of the actual encoded payload at the start of an iteration. Use a valid, admitted frame with `L <= B`, but with less remaining capacity than the encoded addition for one evidence-closed observation. For example, `L = B - 1` is sufficient. Assume the role prompt, execution scope and read-tool registry are available, and the tool budget permits the first call.

1. The admission check accepts the input because `L <= B`.
2. Tools are offered. The provider returns one ordinary call to an offered read such as `read_file`, with a short identifier and name.
3. With one call there are no later calls to reserve. The code calculates `room = B - L - observationOverhead`, which is below `observationOverhead` for the chosen frame.
4. The low-room branch sets `closed = true`, appends an observation containing `evidenceClosed`, and continues without serializing, fitting, replacing or otherwise making room for that record. No tool execution is needed.
5. At the top of the next iteration, serializing the enlarged payload produces more than `B` bytes. The function returns the context-bound error before it can send a tool-less request.

The source establishing steps 3–4 is [the refusal branch][refusal]; step 5 follows from [the next-iteration check][admission]. Thus the closure mechanism itself makes an admitted input fail. The reserve expression for later calls does not solve this case: there are no later calls, and the existing frame already lacks capacity for the current refusal.

This is not a proposal to accept an initially oversized frame. A frame already above `B` correctly fails the existing admission check and is a distinct negative control.

## Conditional witness B

Start with the same admitted near-bound payload, but let the provider return a response with no tool calls and empty or whitespace-only text. The new nudge branch sets `closed` and `nudged`, appends an `answer-required` observation with the nudge text, and continues. It does not reserve or fit the addition. The next serialization exceeds `B`, so the role fails before the intended second request. [Nudge branch][nudge]

This witness does not require a tool call or exhausted tool budget. It targets the exact class of empty responses that the new nudge is meant to recover. It also does not establish that every full read followed by an empty answer fails: ordinary read fitting can leave enough spare bytes, depending on actual serialization. The necessary condition is inadequate remaining encoded capacity at the nudge boundary.

## Actual encoded overhead

The fixed `observationOverhead = len(evidenceClosed) + 128` is a budgeting estimate, not an encoded-length guarantee. A `ProbeResult` always serializes `key`, `tool` and `output`; only `error` has `omitempty`. Adding the first observation also introduces the `observations` property and array syntax. Later records add separators. Key and tool strings can require JSON escaping. The nudge uses the same record type, so even empty tool and output strings still have their field syntax. [Record shape][record], [Overhead constant][overhead]

For the concrete ASCII witness with no prior observations, `key: "a"`, `tool: "read_file"`, empty output and the current 108-byte refusal text produce a 161-byte record. Introducing `,"observations":[...]` adds another 18 bytes around that record: the complete payload grows by **179 bytes**. The `answer-required` record with empty tool/output and the current 105-byte nudge text grows the initial payload by **181 bytes**. These are source-derived serialization arithmetic for the stated strings, not a Go execution or a reproduced model run. The fixed estimate is 236 bytes; even an estimate larger than these particular records cannot manufacture capacity in a frame admitted at `B - 1`.

The successful-execution branch does reserialize the payload and shrink `Output` to account for escaping. That is a useful protection. But it is not called by either refusal or nudge append. Moreover, its shrink loop adjusts only output, not arbitrarily large error text or identifying metadata, and stops when the output allowance is nonpositive. This reinforces why a correction should measure complete encoded records, although neither near-bound witness needs large error text or unusual metadata. [Successful-output fitting][fit]

The contract to test is the final encoded request size, including structural overhead and control records, rather than a sum of raw text lengths or a fixed per-call estimate.

## Bounded correction

Update the deliberation capability and validation contracts before implementation, preserving the repository's [spec-first and bounded-context guidance][guidance]. Retain the current tool budget, configured byte bound, JIT role prompts, read-only tool restrictions, visible evidence cuts, candidate validation and policy-owned decisions.

The repair needs an explicit rule for capacity that cannot accommodate both the admitted frame and mandatory terminal control. There are several reasonable designs:

- Reserve a measured allowance for the required terminal-control records before admitting discretionary evidence, and close tools before requesting more evidence when that reserve is all that remains.
- Keep a bounded host-owned terminal-control field whose encoded cost is included in admission and whose state replaces an earlier control value instead of appending indefinitely.
- Fit a terminal record by reducing recoverable, discretionary observations under an explicit coverage marker. Do not silently trim the user's request, constraints, success conditions or required provenance to make it fit.
- If the original frame cannot preserve its semantic requirements and fit the minimum control protocol, report that condition explicitly before taking an additional model turn. Such an admission outcome must be distinguished from promising a nudge and then failing while constructing it.

Choosing a design requires deciding what content is recoverable and how to expose missing coverage. Merely setting `closed = true` while leaving no interpretable closing instruction may not be equivalent to the intended recovery contract. Conversely, one bounded host-owned summary can represent a refused batch without appending one full note per requested call, provided call-result coverage and the reason are explicit.

Use actual encoded size, including escaped metadata, when admitting the resulting payload. Define an explicit result for impossible minimum frames and excessively large control identifiers or errors; do not rely on a byte estimate silently covering them. The change should not increase `DeliberationContextBytes`, remove the hard bound, raise the tool budget, grant new tool permissions, fabricate tool observations, or accept an empty response as a valid design.

## Evaluation not run

First use deterministic production-kernel tests with the existing scripted client and registry seams. These can prove request construction and control flow without a live model. The payload reaching each `CompleteWithTools` call, offered catalog, tool execution count, role result, exact reason and serialization length are the observables.

| Fixture | Required distinction |
|---|---|
| Encoded initial payload at `B - 1`, one ordinary offered read | Either an explicit pre-call admission disposition or a bounded tool-less closing request under the chosen contract; no late overflow caused by the refusal |
| Same near-bound payload, one empty reply | The chosen bounded nudge path is observable, or the earlier admission disposition explicitly rules it out |
| One byte below, at and above the terminal-control threshold | Boundary is based on encoded size; above-bound original input is not accepted |
| Small frame and valid immediate answer | Existing success path remains unchanged |
| Small frame, empty reply then valid answer | Exactly one nudge; tools withheld |
| Two empty replies with enough space | Existing named no-answer failure remains |
| Spent execution budget and several requested reads | No extra tools run; refusal coverage remains truthful and bounded |
| Full output followed by a same-response batch and then an empty answer | Evidence fitting and nudge compose, rather than passing only in isolation |
| Quotes, backslashes, multibyte content and HTML-escaped characters | Check the Go encoder's actual bytes, not character counts |
| Long identifiers and errors | Bounded metadata/error handling, explicit coverage and no hidden increase in budget |
| Cancellation, invalid tool arguments and unavailable reads | Preserve actual failures and cancellation without inventing successful evidence |

Then exercise the chat service, shard adapter and role request through normal boot, with the ordinary world-model and CodeDOM context enabled. Use valid candidate/assessment fixtures so failure is not confused with malformed output. Observe both proposer and challenger frames, previous-round replay, specialist evidence and saved thread status. Restricted mocks are supplementary evidence, not a substitute for this production-route coverage.

A separately authorized live evaluation should hold model, prompt corpus, task, permissions and byte/tool budgets constant across baseline and treatment. Compare completed valid deliberations, legitimate stopped/admission outcomes, late context-overflow failures, evidence coverage, refusal truthfulness and human questions. Record which fixtures actually produced an empty reply or near-bound frame; lack of that event is not evidence the boundary was tested.

## Costs and acceptance

No speedup, savings or frequency has been measured. Reserving control bytes reduces the space available for discretionary evidence and can close reads earlier. A fit that drops recoverable observations may require later retrieval and must expose the missing coverage. These are real tradeoffs, not free improvements.

Exact serialization adds CPU and allocations proportional to the encoded payload size; repeatedly shrinking and remarshal-checking it may add multiple passes. Measure peak retained bytes, marshal passes, allocation volume and host wall time alongside model input/output tokens, tool runs, retries, failed rounds, subsequent retrieval and end-to-end cost per valid completed deliberation. Do not claim efficiency from suppressing evidence that changes task quality or from counting a failed/admission-rejected task as completed.

Acceptance is a bounded closing protocol whose guarantee is true for its admitted input domain, one source-matched regression for each terminal append path, preserved ordinary-case tests, and a production-route receipt showing the configured input cap was never enlarged. This audit does not claim the implementation or validation has been performed.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/fa6877b932322bb6ca75826289366836d442c051
[schema-fix]: https://github.com/theRebelliousNerd/codenerd/commit/77feafc69299b5807d8aa0436da35170505b8500
[budget-fix]: https://github.com/theRebelliousNerd/codenerd/commit/d805e64d12b5b75449c27a69f2d4027130eb937f
[nudge-fix]: https://github.com/theRebelliousNerd/codenerd/commit/4a7b7e654d023deafa134ba9b39a8b5ba09ebfc7
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/.claude/skills/codenerd-dogfood/references/component-ledger.md#L6257-L6266
[agent-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/agent_test.go#L61-L151
[chat-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/cmd/nerd/chat/deliberation.go#L18-L37
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/shards/deliberation.go#L40-L65
[round]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/round.go#L10-L72
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/config/routing.go#L17-L110
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/read_tools.go#L106-L132
[failure]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/service.go#L44-L51
[refusal]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/read_tools.go#L155-L170
[nudge]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/read_tools.go#L139-L150
[record]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/types.go#L54-L59
[overhead]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/read_tools.go#L196-L218
[fit]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/internal/deliberation/read_tools.go#L172-L191
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fa6877b932322bb6ca75826289366836d442c051/agents.md


## As built (2026-10-30)

`internal/deliberation/read_tools.go`: a role's frame must leave `terminalReserve` (room for one closing note and
one answer-required note) inside the context bound or it is refused before any model call. Control notes go
through `addNote`, which cuts the largest earlier observation, visibly, until the note fits; only a bound with no
room for a note at all is an error. Test: `TestAgent_ClosingNotesAlwaysFitTheBound` (a near-bound read followed by
a refused read and an empty reply still reaches the answer; an oversized frame is never sent).

---
doc-class: deep-dive
subsystem: deliberation
implementation-status: proposed-continuation
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 8d373fb6ca05ef03fec59b923c5f1925066d623a
continues: Docs/audits/deliberation-terminal-budget.md
---

# Deliberation terminal-budget continuation: account for the appended note

## Decision and evidence boundary

Continue the existing [deliberation terminal-budget audit][audit]; do not open a duplicate finding or discard its landed repair. Commit [95477e1ce][fix] adds initial terminal reserve, visible evidence trimming and a closing-note helper. Its remaining issue is narrower: the helper measures a copied payload whose observations slice still has its pre-append length. It can therefore return success without accounting for the control note it just appended.

Public main was re-resolved at approximately 07:15 UTC on 2026-10-30 to [8d373fb6][main]. The relevant implementation is unchanged from the repair. This continuation uses direct source inspection and an independent source challenge only. No repository program, Go test, model inference, benchmark or failure injection was run. The witness below is an unexecuted source-derived schedule, not an observed production failure. No implementation change is included.

## Primary language reference

[The Go specification on slice types](https://go.dev/ref/spec#Slice_types) and [function calls](https://go.dev/ref/spec#Calls) describes slice values and argument passing. The source-level question is which slice header the helper serializes after mutation, not whether the backing array is shared.

## Preserve what landed

The repaired `Agent` refuses an initial frame unless it leaves `terminalReserve`, with room for an evidence-closing note and an answer-required note. It also routes both notes through `addNote`, whose intended behavior is to trim earlier observation output visibly until the complete request fits. The existing per-request byte check remains a real fail-closed safeguard. [Initial admission][initial] [Note call sites][calls]

This is not a claim that the old repair did nothing, that oversized requests are sent, or that every near-bound turn fails. The newly added test supplies two tool calls in the same response. That shape benefits from the separate per-response reservation for remaining calls and does not exhaust the residual described here. [Added test][test] [Per-response reservation][calls]

## Source mechanism

`payload` is a local struct containing `Frame` and `Observations []ProbeResult`. Both call sites invoke:

```go
addNote(&payload.Observations, note, payload, read.ContextBytes)
```

The third argument copies the struct into an `any` value. In `addNote`, `*obs = append(*obs, note)` updates the caller's observations slice header. The copied struct still holds the old slice length. `json.Marshal(payload)` inside the helper therefore excludes the newly appended element, even if both slices share an underlying array. Its `over <= 0` branch can return without trimming anything. [Payload definition][initial] [Helper][helper]

The next iteration of `Agent` marshals the actual caller payload and checks its complete length. If that real payload exceeds `ContextBytes`, the function returns `deliberation role context exceeds configured byte bound` before the intended answering call. The bound is preserved by refusal; the closing protocol's promised answer opportunity is not. [Actual outgoing check][initial]

An append that reallocates also separates the copied payload's array from the array subsequently edited through `obs`. That reinforces why the helper must inspect the live payload, but the primary witness needs only the stale slice length and does not depend on a particular capacity or allocator behavior.

## Unexecuted continuation witness

Use the real `Agent`, its registered read path, a scripted client, `ContextBytes = 2000`, a short ordinary frame and short tool IDs. Use plain ASCII read output so JSON escaping is not the cause. Keep the configured tool-call allowance open through the second response; it is context room, not an already-exhausted call allowance, that will refuse the second read. Return this sequence:

1. A response containing **one** read call. Supply enough ASCII output for the normal fitter to consume most available context. There is no second call in this response for the per-response reservation to protect.
2. A later response requests another read while tools are still offered. Available context room is below `observationOverhead`, so the call is refused and an evidence-closing observation is appended. Select the first output length so this actual payload still fits, but leaves less room than the next control note requires.
3. The next tool-less response is empty. The host appends its answer-required note. `addNote` measures the copied pre-append payload, which fits, and returns without shrinking retained evidence.
4. The subsequent loop marshals the actual payload, now over 2000 bytes, and returns the context-bound error before the scripted final answer can be requested.

The independent source challenge identified this clean sequential read/refusal/empty-nudge schedule. A regression fixture should calculate and assert its serialized sizes rather than relying on a guessed amount of output. It should establish `beforeNote <= bound < afterNote` while showing that trimming earlier output would make the full request fit. This separates helper accounting failure from an intrinsically impossible frame.

## Bounded correction and acceptance

Measure the same live, post-append payload that `Agent` will serialize. A typed payload pointer or a serialization callback can preserve that identity; choose the implementation within the existing architecture. Do not remove the outgoing bound check, increase budgets to hide the defect, silently omit control notes, or broaden tool access. Keep visible evidence trimming and the initial reserve.

Proposed checks, all unexecuted here:

- The sequential witness above reaches the scripted final answer, with both control notes present and every actual serialized request within the bound.
- Appends with spare capacity and appends requiring reallocation yield the same accounting outcome.
- Trimming is visible; frame and control instructions are preserved.
- Existing two-call-in-one-response coverage, small frames and immediate answers remain valid.
- Frames that cannot fit the required protocol are still refused before model calls; repeated empty replies still fail explicitly.
- Escaped output and long identifiers are separately budgeted against complete serialization, without claiming they are needed for the primary witness.

The acceptance claim is limited to restoring a bounded closing opportunity. No success-rate, latency or token-cost improvement is measured. Evaluation should include all role calls, failures, serialized bytes, trimming work and retry cost, not merely the final successful reply.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/8d373fb6ca05ef03fec59b923c5f1925066d623a
[audit]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/deliberation-terminal-budget.md
[fix]: https://github.com/theRebelliousNerd/codenerd/commit/95477e1ced217f3b539b4789a3baa9d599485ba4
[initial]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/deliberation/read_tools.go#L106-L139
[calls]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/deliberation/read_tools.go#L146-L205
[helper]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/deliberation/read_tools.go#L218-L254
[test]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/deliberation/agent_test.go#L153-L183

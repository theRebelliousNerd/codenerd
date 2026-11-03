---
doc-class: deep-dive
subsystem: context-articulation-cli
implementation-status: proposed-continuation
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
supersedes: []
---

# Preserve control outcomes through the atom-only compression fallback

## Decision and scope

**The existing control-disposition repair reaches the summary generator, but not every representation the compressor can publish.** When a generated summary exceeds its target-ratio budget and serialized key atoms fit, `compress` replaces the entire summary with those atoms. `ControlOutcomes` are not key atoms. A failed memory operation can therefore be reported correctly to the user, recorded correctly in the recent compressed turn, and preserved through observation masking, then disappear from the published history segment. This is a narrow continuation of **ART-ADMIT-04**, not a new claim that CodeNERD lacks control outcomes. [Ratio fallback][fallback], [key-atom collection][atoms], [prior contract and as-built repair][prior]

Source inspected on 2026-10-30, with `main` revalidated at approximately 16:16 UTC at the pin above. Root/scoped guidance, the current audit inventory, and research additions through `research-main-20261030-1548` were reviewed. No application, test, inference, fault-injection or benchmark execution was performed; examples are source-derived conditions and proposed acceptance fixtures, not reproduced incidents.

The relevant `compressor_turns.go` is also byte-identical at the five inspected active code/QA heads: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. This is a comparison of that file, not certification of whole-branch behavior.

## What is already right

The prior audit's as-built appendix credits concrete repairs that remain present:

- The memory applier returns host-owned outcomes. For example, `promote_to_long_term` returns `failed` when no store exists or `RememberUserPreference` returns an error; it does not claim persistence. Notes can report partial application. [Memory application][memory]
- Chat separates operations that took effect from those that did not. The latter supply user-visible notices and `NotApplied`; only applied controls enter the sanitized packet. The classification's constraint is retained. [Chat disposition adapter][adapter], [application result][applied-result]
- Normal completed articulation appends system warnings to its response and passes `applied.NotApplied` as `Turn.ControlOutcomes` to `ProcessTurnWithAppliedControls`. That trusted-host path records effects without applying them again. [Production handoff][handoff], [no-replay path][no-replay]
- `CompressedTurn.ControlOutcomes` survive snapshots and recent-turn serialization. The observation-masked summary explicitly emits them before deciding whether observations should be masked. This is a deliberate protection, not a missing first-stage implementation. [Outcome-aware summary][summary], [existing tests][tests]

The proposed repair must retain these controls. In particular, it must not restore failed operations to `MemoryOperations`, turn notices into executable model-authored facts, or retry a memory effect merely to rebuild its history.

## The later consumer that loses the distinction

### 1. Normal production can reach the branch

The full-articulation chat route applies the packet, builds a sanitized compression packet, and asynchronously records the completed turn. A nonempty classification category yields a `user_intent` atom, independently of whether the memory write succeeded. The compression witness therefore need not fabricate an otherwise absent key atom. This is conditional on taking the full-articulation route and receiving an admitted memory operation; a short conversational fast path is not a substitute for that entry condition. [Production handoff][handoff], [packet projection][adapter], [intent extraction][extract]

`processTurn` stores the outcomes and checks compression. Budget pressure is one trigger; window overflow is another: more than twice the recent window calls `compress`, even if ordinary utilization has not crossed the compression threshold. Production defaults are a recent window of five, threshold `0.60`, and target compression ratio `100.0`, threaded into the session compressor. Thus, in an otherwise sequential session with no earlier compression, the eleventh completed recorded turn causes the overflow path to compress the oldest six and retain five. Asynchronous scheduling and earlier budget-triggered compression can change the exact timing. [Turn processing][turn-process], [overflow trigger][overflow], [defaults][defaults], [bootstrap wiring][bootstrap]

### 2. The initially correct summary is replaced

`compress` collects key atoms from the old turns, generates the outcome-bearing summary, and computes the ratio budget. Define the values as the actual configured token estimator computes them:

- `O`: original token estimate for the turns being compressed;
- `R`: positive configured target compression ratio;
- `B = max(1, floor(O / R))`;
- `S`: token estimate of the generated summary, including control notices;
- `A`: token estimate of `SerializeFacts(keyAtoms)`.

The atom-only replacement is taken exactly when **`0 < A <= B < S`**. If `S <= B`, this replacement is not taken. If the atom rendering is empty or too large, the other branch trims the summary instead; that is not the primary witness here. Setting a nonpositive ratio disables this ratio-budget branch. [Budget calculation and replacement][fallback]

`collectKeyAtoms` takes the intent, focus, a capped prefix of result atoms, and action atoms. It never reads `ControlOutcomes`: the latter were rendered as host comment lines, not facts. The key-atom serializer therefore has no failed-operation status to serialize. The replacement assigns `summary = serializedAtoms`, with no merge of the protected notices and no notice that control outcomes were omitted. [Key-atom collection][atoms], [outcome comments][summary]

The published `HistorySegment` contains `Summary`, `KeyAtoms`, metrics, masking and dropped-atom counts, but no typed outcome collection. The compressed source turns are then removed from the recent-turn slice. Rendering the segment adds its summary and key atoms; `DroppedAtoms` reports atom-cap/reserve omissions, not these discarded dispositions. Retaining every atom can consequently coexist with losing every control notice from that segment. [Segment type][segment], [publication and source removal][publish], [segment rendering][render]

### 3. Later articulation can depend on that representation

Once a segment exists, `IsCompressionActive` stays true. The normal articulation route then uses a bounded raw-history suffix plus `GetContextString`; a successful build includes the rolling summary. The prompt renderer labels that block as long-range compressed session context. Once the original response is outside the raw suffix, this route does not automatically resend the entire saved conversation to recover the missing notice. [Active-state decision][active], [later chat selection][selection], [context construction][build], [prompt injection][injection]

This establishes a reachable producer-to-consumer loss under the stated branch conditions. It does **not** establish that every later model answer will be wrong, that all copies of the original notice disappear, or that a failed memory effect becomes an applied effect in the kernel.

## Bounded witness and consequence

Use an ordinary completed articulation whose admitted packet requests `promote_to_long_term` for a preference and whose store returns an error. The memory applier supplies the failure; chat shows it and records it. Continue enough completed articulation turns to move that turn into a compression batch. Choose a fixture whose real counts satisfy `0 < A <= B < S`, with a small atom set below the 64-atom segment limit, no turn exceeding the five-result-atom cap, and enough history reserve that the segment is not subsequently truncated. The original input/surface lengths affect `O` without being retained as arbitrary prose in the key atoms, so a useful fixture can exercise this branch without disabling compression or manufacturing successful memory state. Its exact text and counts must be checked in evaluation, not guessed here. [Failure producer][memory], [original-token accounting][original], [fallback][fallback]

A useful observational-equivalence check is to hold ordinary key atoms fixed while varying a host failure disposition. If both variants satisfy the inequality, the resulting `Summary` and `KeyAtoms` can be identical despite differing failure facts. Metrics or timestamps may differ; they do not identify which operation failed or its disposition. This is stronger and narrower than simply observing that summaries are lossy.

A later request asking what was saved, why a preference was not retained, or which part of a replacement succeeded may then lack the explicit negative evidence that the host previously had. Possible effects are a less accurate explanation, an unnecessary repeat request, or an unsupported inference from the retained intent. Those are conditional quality and recovery costs. The sanitized packet contains no successful record of the failed operation, so **this source trace does not prove a false “saved” assertion or a permission escalation**.

## Limits, alternate recovery and novelty

- **Immediate truthfulness remains:** the original response receives the host warning. Recent raw history and recent compressed-turn serialization can still carry it.
- **Raw history is not globally erased:** session persistence serializes the full `m.history` messages, subject to normal save success. A user, diagnostic route or later retrieval may recover the earlier warning. This audit concerns the automatic compressed-history projection, not deletion of the archive. [Raw-history persistence][persistence]
- **The store/kernel remains authoritative:** a later check of memory may establish current state. Absence of a preference alone does not reconstruct the exact historical failure or a partially applied operation.
- **Other evidence may rescue an answer:** repeated user explanations, later successful controls, relevant observations, or independent retrieval can carry the needed distinction. The witness assumes no equivalent admitted evidence in the later call.
- **Existing disclosure remains useful:** observation-mask and atom-drop disclosures, marked text trimming, and the explicit notice when a compressed-context build fails are not being denied. The atom-only substitution can occur on a successful build with zero dropped atoms, so those notices do not describe this loss. [Rendering][render], [marked trimming][trim], [failed-view disclosure][view]
- **Prior work already owns the broad requirement:** ART-ADMIT-04 requires outcome fidelity through compression and its as-built appendix correctly describes the initial repair. This document identifies a subsequent budget consumer that bypasses it. The conversation-history audit's open H3 concerns routes not recorded and intentional loss of arbitrary answer prose; here the route is recorded and the typed disposition is successfully rendered before replacement. The latest calibration-model-binding audit concerns token-estimator identity, not which evidence an alternative summary preserves. [Prior repair][prior], [history H3 and as-built][history], [calibration audit][calibration]

## Proposed contract and repair options

These are proposed changes, not existing APIs.

1. **Make control-outcome coverage independent of summary strategy.** Every non-fully-applied disposition admitted into a turn must remain represented in its segment as a typed outcome, or as an explicit omission/recovery record. A prose-to-atoms optimization must not silently change that coverage.
2. **Render from host-owned evidence after choosing the summary form.** Preserve turn association, control kind/index and actual stage; retain bounded, redacted detail and reason or a genuinely retrievable evidence reference. Do not infer success from intent and do not reapply any effect. Historical failure is historical evidence, not a permanent claim that a later retry failed.
3. **Budget the combined representation.** Prefer trimming optional prose/duplicate atom renderings before removing outcome identity and stage. If the minimum status representation cannot fit, render explicit missing-outcome coverage and a bounded recovery route; if recovery is unavailable, say so. Do not append unlimited notices outside the budget or claim an unresolvable reference preserves evidence.
4. **Conserve coverage through the entire lifecycle.** Ratio fallback, reserve trimming, segment merging, state snapshots/persistence, rehydration and final context serialization must preserve outcomes or their explicit coverage records. Merely appending notices at the first fallback would leave later trimming able to undo the repair. Existing whole-block bounds must remain effective. [Later reserve/merge logic][reserve], [final serializer bound][final-bound]
5. **Keep the first increment small.** A shared renderer of “selected summary + bounded control-status section,” with correct accounting and outcome-drop metadata, can close the immediate silent substitution. Typed segment outcomes offer a cleaner merge/persistence contract. A durable receipt lookup is an optional larger design and must reuse the earlier audit's identity work rather than invent a second receipt system.

## Costs and fair evaluation

Preservation is not free. Repeatedly admitting outcomes increases prompt tokens and may lower the achieved compression ratio or displace other useful history. Typed segment metadata increases retained state, snapshot/copy and persistence work; an unbounded error history would defeat the window and reserve controls. Reasons can be large or sensitive, so detail redaction, bounded payloads, retention policy and accurate omission counts matter. A recovery store adds indexing, lookup latency, lifecycle/migration and privacy obligations; it should not be mandatory if a compact status suffices. Provider cost may rise even if summarization remains deterministic and needs no new model call. Correcting the relevant accounting must include the final rendered notices, not just fact counts; current `CountTurn` counts atoms and updates rather than the control-outcome text. [Current turn accounting][count]

Suggested evaluation, **not performed here**:

| Case | Required observation |
|---|---|
| Normal chat handoff, real memory-applier error, default configuration | Original warning shown; failed operation absent from applied controls; completed recorded turns trigger actual compression; capture and assert `A`, `B`, `S` before claiming the fallback fixture was exercised. |
| `A = B`, `S > B`; then `S = B`; empty or oversized atoms; nonpositive ratio | First case exercises atom replacement; controls cover the non-replacement, trim and disabled-budget branches without relabeling them. |
| Same atoms, different failed/partial dispositions | Published context distinguishes the statuses or explicitly reports unavailable outcome evidence; metrics alone do not count as distinction. |
| Recent turns, masked old turns, merged segments, tight reserves, save/reload, final block cap | The same coverage contract survives every representation, with no effect replay and no unbounded context growth. |
| Successful-only and no-control turns | No false failure notices, unjustified blocking, redundant retry or material regression from carrying an empty status section. |
| Later ordinary user question after the raw suffix no longer contains the notice | Inspect the actual admitted articulation request and score the answer against host outcomes; do not stop at a helper-generated summary or manually injected final context. |

The existing tests are valuable controls, not proof of this branch: the context test checks the recent snapshot plus a direct `generateObservationMaskedSummary` call; the chat handoff test checks one recent turn. Neither cited fixture advances through `compress` and verifies the final atom-only segment. Extend them rather than replacing their assertions. [Context test][tests], [chat handoff test][chat-test]

Compare baseline and repair with matched ordinary and faulted sessions, unchanged providers and permission gates. Report final admitted token counts, provider usage/cost, achieved ratio, serialization and persistence overhead, latency, retained-state growth, outcome coverage, correct later answers, recovery attempts and displaced useful history. Include cases where raw-history or current-memory recovery already succeeds. A win is better bounded evidence fidelity at an explicit total cost—not maximum retention, guaranteed model compliance, or a stronger-looking ratio computed after silently discarding failed effects.

[prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/articulation-admission-and-control-fidelity.md#L255-L295
[memory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/memory_ops.go#L63-L114
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/control_outcome.go#L9-L79
[applied-result]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L1305-L1315
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L1070-L1098
[no-replay]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L33-L43
[summary]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L449-L504
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/control_outcomes_test.go#L136-L196
[extract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/serializer.go#L332-L366
[turn-process]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L103-L194
[overflow]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L690-L723
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/memory.go#L143-L166
[bootstrap]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L341-L361
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L338-L387
[atoms]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_metrics.go#L359-L407
[segment]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/types.go#L358-L389
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L392-L428
[render]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L660-L687
[active]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_metrics.go#L105-L125
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L975-L999
[build]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor.go#L758-L784
[injection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_articulation.go#L232-L242
[original]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_metrics.go#L347-L356
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_persistence.go#L75-L90
[trim]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_metrics.go#L410-L430
[view]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/history_view.go#L9-L25
[history]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/conversation-history-coverage-handoff.md#L258-L286
[calibration]: https://github.com/theRebelliousNerd/codenerd/blob/e69f9f921f1b4a7318fafbf5c82834e75ecebc7f/Docs/audits/context-calibration-model-binding.md
[reserve]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L543-L657
[final-bound]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/serializer.go#L290-L325
[count]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/tokens.go#L118-L143
[chat-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/control_outcome_test.go#L56-L79

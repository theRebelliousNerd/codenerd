# Admitted observation values change during chat compression

## Finding

The chat host can successfully admit and assert an observation containing a newline or tab, then record a different value in compressed conversation history. The host uses the actual Mangle parser, serializes the accepted fact, and passes that sanitized text to the compressor. The compressor reparses it with a separate helper that removes a backslash without interpreting the escape. Consequently, a newline becomes the letter `n`, and a tab becomes `t`. A later articulation request can receive that altered observation as logical context. [Host application][host] [Compressor decoder][decoder]

This is a **source-established value-fidelity gap**, not evidence that a particular answer was wrong. The correct kernel fact and original sanitized update remain available, and the recording path deliberately does not replay effects. The bounded consequence is an inaccurate additional context representation; whether it misleads a later answer depends on the request, other evidence, and model behavior.

**Baseline:** public main `6cbe3962fbdb24a1639b26e71f8096e4f4330048`, inspected 2026-10-30. Guidance, the main audit inventory, active branch inventory, and research audit scopes through `research-main-20261030-1722` were reviewed. That research tip is `490b0d5511da8bb41326917c70737f331548ed1b`. All conclusions below are static; no application, tests, inference, or benchmarks were run.

## Relationship to earlier work

The articulation admission audit already established the need for one interpretation, typed handoffs, per-control outcomes, and truthful compressed history. Its appended fix record credits strict envelope handling, host-owned application, retained failure dispositions, and the sanitized chat handoff. This report preserves those repairs and identifies a later consumer that changes an **already admitted value**. It is a continuation of that fidelity contract, not another claim that the same admission fixes are absent. [Earlier admission audit][admission-audit]

The 16:19 report concerns a ratio fallback replacing outcome-bearing summaries with key atoms. Here one small recent result atom is sufficient: no summary fallback, result cap, or budget truncation is required. The 16:51 report concerns missing shard grounding in a second synthesis request; here the relevant fact reaches the compressor but its string value changes. The new 17:22 reports concern campaign learning receipts, generated-tool quality profiles, and Go test dependencies, rather than this decoder. [Outcome fallback audit][fallback-audit] [Synthesis grounding audit][grounding-audit] [17:22 inventory][latest-inventory]

This also differs from JSON escaping of protocol keys: the outer response can be unambiguous, valid JSON, and correctly admitted. The disagreement occurs inside the Mangle fact string after host application.

## Reachable production path

1. **An ordinary admitted observation is a supported producer.** The embedded Piggyback protocol permits `observation("key", "what you saw").`; the model-observation policy includes that predicate, and the schema declares two string arguments. The constitution marks observations as prose, so their punctuation is not treated as shell-command syntax. This does not make observations executable authority. [Protocol][protocol] [Policy][policy] [Schema][schema] [Prose classification][prose]

2. **The host obtains the actual value before recording it.** `FilterMangleUpdates` calls `ParseFactString`, enforces the predicate policy and declaration/arity checks, and applies the prose-aware shell-content check. `ParseFactString` delegates to the Mangle parser and converts string constants using their decoded `Symbol`. In `applyArticulationControlPacket`, only a successful `m.kernel.Assert(fact)` reaches `recordUpdate`, which retains `fact.String()`. String serialization uses Go's quoted representation. [Policy][policy] [Authoritative parser][parser] [String conversion][value] [Host application][host] [Fact serialization][fact-string]

3. **The handoff is explicit, not an assumed world-shard bridge.** `compressionControlPacket` takes `applied.RecordedUpdates`. After successful articulation, chat passes that packet to `ProcessTurnWithAppliedControls` in its background recording operation. That method extracts atoms for history but skips assertion and memory reapplication. The normal path therefore needs no assumption that world-owned facts happen to appear in the catch-all kernel. [Sanitized packet][packet] [Production recording][recording] [No replay][no-replay]

4. **The second decoder changes escape meaning.** `ExtractAtomsFromControlPacket` sends each recorded update through `ParseMangleAtom`. Its quoted-argument path calls `unescapeQuoted`, which advances over any backslash and writes the following rune unchanged. It returns no error for `\n` or `\t`. These facts are retained as `CompressedTurn.ResultAtoms`; the original text is separately retained in `MangleUpdates`. [Extraction][extraction] [Compressor decoder][decoder] [Turn storage][turn-storage]

5. **The altered value can reach a later model request.** `BuildContext` includes the recent compressed-turn window; serialization renders `ResultAtoms` rather than the original `MangleUpdates`. Chat uses this context once `IsCompressionActive()` is true and context construction succeeds. Activation occurs when summary segments exist or the budget says compression is due. The articulation helper includes `CompressedCtx` in the answering request. For the witness, the small result remains inside the recent window and the full context block's character clamp. [Context construction][build] [Recent rendering][render] [Activation][activation] [Chat selection][selection] [Request inclusion][inclusion]

## Exact small witness

Consider an accepted update whose Mangle source is:

```mangle
observation("layout", "alpha\nbeta").
```

The displayed `\n` is one Mangle escape, not a literal backslash followed by an intended `n`. The outer JSON string must escape that backslash in its own encoding. CodeNERD pins `codeberg.org/TauCeti/mangle-go v0.7.0`. The accessible GitHub mirror's annotated v0.7.0 tag resolves to `7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`, whose module declaration matches that dependency. Its string visitor calls `ast.Unescape`; the decoder maps `n` to LF and `t` to tab. This verifies the grammar against the pinned release rather than assuming a generic unquoting rule. [Dependency][dependency] [Release tag][upstream-tag] [Mirrored module][upstream-module] [String visitor][upstream-parse] [Canonical escapes][upstream-unescape]

The source-level transformations are:

| Boundary | Value represented |
|---|---|
| Mangle parser and admitted kernel fact | `alpha`, LF, `beta` |
| Host's sanitized fact text | `observation("layout", "alpha\nbeta").` |
| Compressor's decoded second argument | `alphanbeta` |
| Rendered recent result atom | `observation("layout", "alphanbeta").` |

Similarly, a tab-separated observation `"key\tvalue"` becomes `"keytvalue"`. Plain strings, escaped quotes, and escaped backslashes do not demonstrate this particular mismatch.

A useful distinguishing pair is the newline-bearing observation above versus an observation whose true value is the literal string `alphanbeta`. Their admitted values differ, but their recent **result-atom projection** can be identical. Their complete system state and complete requests need not be identical: kernel facts, retained source updates, and other history can still distinguish them.

Use a normal chat session whose compression is already active, complete the observation-bearing articulation, allow background recording to finish, then inspect the next articulation request while this turn remains recent. One short observation is enough. A session without active compression does not exercise this direct articulation consumer, even though recording may already have produced the altered atom.

## Safeguards and limits that remain real

- **Admission and authority are not bypassed.** Predicate restrictions, declaration checks, separate candidate-action handling, and successful host assertion precede this record. Rejected updates are not transformed into accepted effects by the compressor. The observation is data, not permission to act. [Host application][host] [Policy][policy]
- **Correct state is not globally erased.** The asserted kernel fact remains correct. The compressed turn also retains the correct canonical update string. Other selected context facts or raw conversation content may supply the correct information. The report does not claim these alternatives always rescue an answer or are always absent. [Turn storage][turn-storage] [Context construction][build] [Chat selection][selection]
- **Effects are not replayed on the audited route.** `ProcessTurnWithAppliedControls` preserves the host's application ownership. Existing tests explicitly check retained metadata and the absence of duplicate kernel/memory effects; their sample observation has no newline or tab. [No replay][no-replay] [Applied-control tests][applied-tests]
- **The custom parser has useful existing repairs.** Whole-argument numeric parsing avoids numeric-prefix loss; quote handling, quoted numeric strings, malformed inputs, and nested argument splitting have tests. Those controls should survive a repair. The inspected parser test does not check admitted newline/tab values through host application and later request rendering. This is not a claim that every relevant test in the repository was exhaustively reviewed. [Compressor decoder][decoder] [Parser tests][parser-tests]
- **Prompt bounds still apply.** Recent-window selection and the whole-block clamp can omit the result. The witness deliberately remains below those bounds. Loss from omission is a separate issue; the finding here is changing the meaning of included text. [Context construction][build] [Recent rendering][render]
- **No measured harm is established.** A later model may disregard the observation, infer the intended value, or recover from another source. Exact examples, delimiters, diagnostic snippets, or formatting-sensitive observations are plausible affected uses, not observed failures.

## Repair contract

Prefer carrying an immutable copy of the **already admitted typed facts** into the trusted recording path. The host already has those facts before converting them to text, and `Turn.ExtractedAtoms` already provides a typed input route. Integrating that route must avoid recording both the string-derived and typed copy, preserve intent and outcome handling, and keep untrusted model output behind the existing admission and application gates. This is a proposal, not a claim that the present typed input alone is a complete fix. [Host application][host] [Typed extraction input][no-replay]

An alternative is to use the canonical Mangle parser for the compressor's string input, with an explicit ground-fact contract and bounded error handling. Replacing the helper with a generic Go unquoter without checking language compatibility is insufficient: Mangle's complete literal grammar and the repository's supported values are the contract.

Whichever approach is chosen:

1. A successfully recorded observation has the same string bytes as the host-admitted fact.
2. Recording does not assert or apply the effect again.
3. Unsupported or invalid recording data produces a visible fidelity disposition, not a silently changed value.
4. Correct retained source strings remain available for recovery of existing history. Migration should use those per-turn strings, not silently substitute a possibly newer current kernel fact.
5. Snapshot ownership, copying, restoration, and version compatibility are explicit. If older records cannot be repaired unambiguously, report that limitation.
6. Context admission and existing bounds remain in force. Preserving a value does not authorize unlimited prompt expansion.

## Costs and fair evaluation

This repair should require no additional model call. It does have costs. Typed forwarding adds copying, ownership, and state-format work; shared canonical parsing adds deterministic CPU, allocations, and possible parser synchronization overhead. Rebuilding historical atoms can add restore latency. Both approaches require maintenance across supported fact types, duplicate handling, and versioned snapshots. Correct escape rendering may change byte/token counts, so measure the entire request rather than treating unchanged row counts as unchanged cost.

A fair evaluation should first establish deterministic fidelity, then assess task utility:

- Drive the **ordinary chat admission, host application, recording, and next-request construction path**, with the normal domain Cortex/world-model setup. Do not inject a finished `CompressedTurn` or use a monolithic test kernel as the only reachability evidence.
- Admit valid JSON envelopes containing small newline/tab observations and the literal-`n`/`t` contrast cases. Compare the decoded kernel value, sanitized update, recorded result, persisted/restored result, and actual next request.
- Reach compression activation through supported budgeting or existing summary state and wait for owned background recording to settle. Include a non-active-compression control. Do not attribute an asynchronous timing miss to decoding.
- Preserve positive controls for ordinary strings, quotes, backslashes, numeric arguments, malformed updates, blocked predicates, failed assertions, no repeated effects, and existing failure dispositions.
- Test request admission with enough room for the witness, then separately test windows and limits. Explicitly account for correct duplicate evidence from active facts or raw history.
- For downstream usefulness, use paired formatting-sensitive follow-up tasks with the same provider/model, available tools, and context budget. Report correct answers, contradictions, recovery turns, false rejection, prompt tokens, deterministic latency, allocations, and restore cost. Do not disable normal recovery mechanisms merely to make the baseline fail.

Acceptance should require byte-faithful recording of supported admitted values and unchanged safety/application ownership. A claimed improvement in answers, latency, or cost requires actual end-to-end evidence; this static audit supplies none.

[host]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/process.go#L1140-L1183
[decoder]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/serializer.go#L484-L538
[admission-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/articulation-admission-and-control-fidelity.md#L254-L280
[fallback-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3ef34bc061b497b9cfc471f4cc46f5df595b299d/Docs/audits/compressed-control-outcome-fallback.md
[grounding-audit]: https://github.com/theRebelliousNerd/codenerd/blob/90c84648725aa263c8a1ba2458390b572402e4de/Docs/audits/knowledge-synthesis-shard-grounding.md
[latest-inventory]: https://github.com/theRebelliousNerd/codenerd/tree/490b0d5511da8bb41326917c70737f331548ed1b/Docs/audits
[protocol]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/prompt/atoms/protocol/piggyback.yaml#L75-L89
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/mangle_updates.go#L51-L174
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/schemas_misc.mg#L193-L197
[prose]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/constitution.mg#L402-L418
[parser]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/kernel_query.go#L476-L496
[value]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/kernel_query.go#L426-L445
[fact-string]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/types/types.go#L135-L151
[packet]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/control_outcome.go#L65-L79
[recording]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/process.go#L1070-L1100
[no-replay]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/compressor_turns.go#L33-L124
[extraction]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/serializer.go#L333-L402
[turn-storage]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/compressor_turns.go#L127-L158
[build]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/compressor.go#L734-L784
[render]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/serializer.go#L219-L309
[activation]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/compressor_metrics.go#L104-L125
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/process.go#L975-L999
[inclusion]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/helpers_articulation.go#L232-L241
[dependency]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/go.mod#L1-L7
[upstream-tag]: https://api.github.com/repos/google/mangle/git/tags/eb51f1c2e772a30db0f4a2e2f878c22c31283a87
[upstream-module]: https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/go.mod#L1-L3
[upstream-parse]: https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/parse/parse.go#L504-L514
[upstream-unescape]: https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/ast/serde.go#L144-L158
[applied-tests]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/compressor_applied_controls_test.go#L12-L95
[parser-tests]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/context/context_unit3_uplift_test.go#L14-L75


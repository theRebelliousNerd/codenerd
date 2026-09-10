# Gap analysis

Gaps are numbered `B*`. Each states the gap, the consequence, and its disposition
in this corpus.

## B1 — No single accounting boundary — **CLOSED**

**Gap.** 145 call sites reached provider clients directly. There was no point in
the process where "an inference is about to happen" could be observed, priced, or
refused.

**Consequence.** Cost per verified task was not computable. Any efficiency claim
was unfalsifiable, because the denominator did not exist and some numerator terms
were invisible.

**Disposition.** `broker.Client` decorates `types.LLMClient` and is installed
inside the three factory functions. No caller changed; no caller can opt out.
Verified by `TestEveryClientConstructionPathIsBrokered`.

## B2 — Token counts were estimated when exact counts were available — **CLOSED**

**Gap.** Every provider returned exact usage. Every budgeting decision used
`charsPerToken = 4.0` instead.

**Consequence.** Roughly ±20% error on every budget, worse on code. The
conservative constants scattered through the codebase were the premium being paid
for that uncertainty.

**Disposition.** Accounting is now provider-reported and exact. Admission uses
the Anthropic `count_tokens` endpoint where available, and a calibrating
estimator elsewhere that corrects itself against reported actuals. Confidence is
labelled on every count.

## B3 — Multiple budget authorities over one window — **CLOSED**

**Gap.** The compressor and the JIT compiler each budgeted the full configured
window; four more components carried hard-coded literals.

**Consequence.** Joint overspend was possible in principle and masked in practice
by conservatism nobody could justify.

**Disposition.** One `Ledger`, segment-attributed, charged at the outbound
boundary. The competing counters and orphan constants are deleted, not deprecated.

## B4 — Counting happened before assembly finished — **CLOSED**

**Gap.** Components counted what they intended to send. Text appended afterward —
the persona concatenation, template expansion, tool schema serialization — was
never charged.

**Disposition.** The broker counts the assembled outbound request: system prompt,
every message, every tool definition. `TestCountedAtOutboundBoundary` asserts
that content appended after a subsystem's own count still appears in the receipt.

## B5 — No confidence signal on any number — **CLOSED**

**Gap.** An estimate and a measurement were indistinguishable downstream.

**Disposition.** `Confidence` is one of `Exact`, `Calibrated`, `Seeded`, carried
on every `Count` and every `Receipt`. `RequireExact` makes a caller fail rather
than proceed on an estimate.

## B6 — `ActivatedFacts` is declared, unpopulated, and trapped — **OPEN**

**Gap.** The activation-to-JIT handshake is documented in a comment and absent
from the code, and the two obvious ways to wire it each introduce a defect (stale
cache identity; shared-map data race).

**Consequence.** The premise that prompt compilation should be driven by what is
semantically hot is untested. It is also the cheapest available test of that
premise.

**Disposition.** Out of scope for the broker, which is a meter and not a
selector. Recorded with the full three-part fix in [TODO.md](TODO.md) so the trap
is not walked into.

## B7 — Truncation cut mid-line — **CLOSED** (and the original claim withdrawn)

**Original claim, withdrawn.** This gap previously read "`ClampText`/`ClampHead`
truncate from the front". That was false: `ClampText` has always been head+tail,
and `ClampHead` is head-only by design on content whose tail cannot matter. See
the correction in [02-CURRENT-STATE.md](02-CURRENT-STATE.md).

**The real gap.** `ClampText` cut at byte offsets, so both surviving ends
routinely started or ended mid-line. A half-line reads as a whole record, and
the failure mode is silent: a truncated package path names a package that does
not exist, and `10 tests failed` cut to `0 tests failed` inverts the verdict.

**Disposition.** Closed. Cuts snap to line boundaries when the snap costs under
an eighth of the budget, with a raw-cut fallback so a single enormous line is
not discarded. Four regression tests over real `go test` output shapes.

## B8 — Pruning happens after generation — **OPEN**

**Gap.** Tool results are generated in full, appended, then blanked by
`boundToolLoopHistory`. Pruning an output after generating it does not refund its
generation cost.

**Disposition.** Phase 1 with B7. The order matters: request less, then project
what returns, then compact what was already exposed — in decreasing order of
value and increasing order of risk.

## B9 — Subagent context is a prose blob with a character cap — **OPEN**

**Gap.** A delegated turn receives 6 messages and 24,000 characters from a ring
buffer that has no knowledge of the compressor. codeNERD has real fan-out
(spawner, subagents, campaign orchestration), which makes this plausibly the
largest single token sink in the system.

**Disposition.** Cannot be fixed by budget arithmetic; requires shared task state
so a spawn can be handed structured evidence instead of a truncated transcript.
Blocked on Goal B5 (typed graph). Recorded in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).

## B10 — Latency is unpriced — **OPEN**

**Gap.** The broker meters tokens. It does not meter seconds. Every economic
decision in the proposed roadmap prices input, output, cache reads and writes,
and prices time not at all.

**Consequence.** An optimizer that is not charged for time will trade fifteen
seconds for two hundred tokens, indefinitely, and will look excellent on the cost
dashboard while users abandon the tool. This is a TUI; a human is watching a
cursor.

**Disposition.** The broker records wall-clock duration per call today, which
makes the histogram available. Making latency a *term in the decision function*
rather than a *field in the record* is required before any routing or rebasing
work begins. Recorded as a hard precondition in
[13-ROADMAP-AND-GATES.md](13-ROADMAP-AND-GATES.md).

## B11 — No lossless native round-trip — **OPEN**

**Gap.** `types.Message` carries `Role`, `Text`, `ToolCalls`, `ToolResults`.
It cannot express arbitrary ordered provider-native content blocks, and the tool
loop rebuilds assistant turns from text plus tool calls.

**Consequence.** Any future reordering, epoch rebasing, or lane work is unsafe
until native continuation objects survive a round trip unchanged. Provider
reasoning state is opaque and can be bound to its preceding prefix; reconstructing
a turn from visible text is not equivalent to replaying it.

**Disposition.** Prerequisite for Goal B7 and B8. Explicitly *not* attempted here:
it is a provider-adapter change across seven clients and would have made this
change set unreviewable. Phase 3 in the roadmap.

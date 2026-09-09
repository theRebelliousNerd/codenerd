# Failure modes

## Failures this package handles

| Mode | Symptom | Handling |
|---|---|---|
| Counting endpoint 5xx / 401 / unreachable | none visible | degrade to the estimator; `Confidence` drops; debug log |
| Counting endpoint slow | none visible | 5s timeout, then degrade; the hot path stays bounded |
| Counting endpoint returns 0 tokens | none visible | rejected as nonsense; degrade |
| Counter errors outright | the turn fails with an `AdmissionError` | fail closed |
| Request exceeds the window | the turn fails with `window_exceeded` and a headroom figure | refusing locally is cheaper than being rejected after transmission and billed |
| Purpose budget exhausted | the turn fails with `budget_exhausted` | other purposes unaffected |
| Caller demanded exactness, only an estimate available | `confidence_too_low` | explicit opt-in only |
| Provider reports no usage | receipt shows zero actual, ledger unchanged | never invent a number |
| Provider reports absurd usage | ratio unchanged | clamp + band rejection |
| Consumer abandons a stream | none | forwarders drain and exit; the receipt still settles |
| Decorator chain cycles | none | traversal bounded at 16 hops |

## Failures this package makes visible for the first time

**Un-metered inference.** It used to be invisible by definition. Now
`PurposeUnattributed` accumulates and the wiring audit fails the build.

**A wrong window.** A misconfigured `ContextWindow.MaxTokens` used to produce
mysterious provider rejections. It now produces a local refusal naming the
window, the count, and the headroom.

**A drifting meter.** `EstimateErrorPct` on every receipt. Previously the error
existed, was roughly ±20%, and nothing measured it.

**Boot ordering.** A meter configured after clients are built reports a zero
window on every receipt from before configuration, rather than passing silently.

## Failures inherited from elsewhere, not fixed here

**Head truncation discards the decisive evidence.** `prompt.ClampText` and
`ClampHead` truncate from the front. Go test output puts the decisive failure at
the end, so the model reasons confidently from the surviving top half. This is
live today. The broker does not fix it; it makes the cost of the material being
truncated visible for the first time. Fixing it needs observation codecs —
deterministic, domain-specific projections applied *before* the model sees output
— which is Phase 1 in [13-ROADMAP-AND-GATES.md](13-ROADMAP-AND-GATES.md).

**Pruning after generation.** Tool results are generated in full, appended, then
blanked by `boundToolLoopHistory`. Pruning an output after generating it does not
refund its generation cost. Same phase, same fix.

**Subagent context starvation.** A delegated turn gets 6 messages and 24,000
characters from a ring buffer that has never heard of the compressor. Given how
much codeNERD fans out, this is plausibly the largest single token sink in the
system. It cannot be fixed with budget arithmetic — it needs shared task state so
a spawn can be handed structured evidence instead of a truncated transcript.
Blocked on the typed graph.

**No lossless native round-trip.** `types.Message` cannot express arbitrary
ordered provider-native content blocks, and the tool loop rebuilds assistant
turns from text plus tool calls. Provider reasoning state is opaque and can be
bound to its preceding prefix; reconstructing a turn from visible text is not
equivalent to replaying it. This blocks every reordering idea in the roadmap and
is deliberately untouched here — it is a change across seven provider adapters
and would have made this change set unreviewable.

## Ways this design could be wrong

**The framing constants could be systematically wrong in a way the calibrator
hides.** The learned ratio absorbs bias from `measure()`, which is the intent —
but it means a genuinely broken `measure()` produces a plausible ratio and
plausible counts that are wrong in a correlated way. The guard is
`EstimateErrorPct` on Anthropic, where an exact count is available to compare
against. If that number is not near zero, `measure()` is broken.

**Cache accounting could skew calibration on a provider not yet exercised.**
Cached tokens are added back before observing, and out-of-band ratios are
rejected. A provider that reports cache usage in some third shape would be
silently rejected by the band check rather than corrupting the ratio — the safe
failure, but it would show up as a calibrator that never trains, not as an error.

**Process-scoped is the wrong scope if codeNERD ever runs multi-workspace in one
process.** The ledger and the calibrator would then merge unrelated sessions.
Nothing today does this; if something does, the meter must move to a workspace
key.

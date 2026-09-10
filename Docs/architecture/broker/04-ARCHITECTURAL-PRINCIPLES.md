# Architectural principles

These are the rules the implementation is held to. Each has a test that fails
when it is broken; the test name is given so the rule is enforceable rather than
aspirational.

## P1 — Instrument before you optimize

An optimizer built on a broken meter does not merely fail to improve things. It
discovers the measurement bug and exploits it, then reports excellent numbers
while doing so.

This is why the meter shipped first and why the economic controller in
[13-ROADMAP-AND-GATES.md](13-ROADMAP-AND-GATES.md) is deliberately last. The
comp-plan analogy holds: your reps will find the loophole, not because they are
crooked, but because that is what optimization does — and afterwards you cannot
distinguish the good reps from the loophole reps, because the number is the same.

## P2 — Never report an estimate as a measurement

Every `Count` carries a `Confidence` of `Exact`, `Calibrated`, or `Seeded`, and
every `Receipt` carries it forward. A caller that must not act on a guess sets
`WithRequireExact` and is refused rather than served an estimate.

The one unforgivable bug in this package is labelling a guess `Exact`; every
admission decision downstream trusts that label.

*Enforced by:* `TestAnthropicCounterDegradesOnServerError`,
`TestAnthropicCounterRejectsNonsenseTokenCounts`, `TestLedgerHonoursRequireExact`.

## P3 — One implementation per concept

Two token counters is worse than either counter alone, because now nobody knows
which is authoritative. The heuristic counter was deleted rather than deprecated;
the orphan budget constants were removed rather than left "for compatibility".

*Enforced by:* `TestNoCompetingTokenCounters`, `TestOrphanBudgetConstantsAreGone`.

## P4 — Count at the outbound boundary

Whatever a subsystem believed it was sending is not evidence about what was sent.
The chat loop appends a persona after the kernel produces `final_system_prompt`;
the assembler expands templates after budget fitting; tool schemas serialize last.

The broker counts the assembled request at the point of dispatch, so all of that
is charged.

*Enforced by:* `TestCountedAtOutboundBoundary`, `TestToolSchemasAreCharged`,
`TestToolResultPayloadsAreCharged`, `TestSchemaIsChargedOnStructuredCalls`.

## P5 — Fail closed

A budget that cannot be checked is a budget that is not enforced. A request whose
size cannot be determined is refused, not admitted on optimism.

The single exception is an unconfigured window, which is an explicit "the limit
is unknown" rather than a failure to measure. It admits and reports zero headroom
on every receipt, so it is visible rather than mistaken for a passed check.

*Enforced by:* `TestFailsClosedWhenCounterErrors`,
`TestLedgerFailsClosedOnUncountableRequest`,
`TestLedgerAdmitsWhenWindowUnknownButReportsNoHeadroom`.

## P6 — A decorator must preserve the capability surface exactly

Several call sites choose a control flow by probing the client. A wrapper that
advertises a capability the client lacks routes requests into a path that cannot
serve them; one that drops a capability silently downgrades the provider.

`Wrap` composes one of eight shapes so the probe answers identically before and
after wrapping. Capabilities that only gate enrichment — accessors and setters —
are forwarded unconditionally, because a probe that succeeds and forwards to
nothing is indistinguishable from a probe that failed.

*Enforced by:* `TestWrapPreservesCapabilityMatrix`,
`TestWrapAlwaysExposesUnconditionalForwarders`, `TestGroundingForwardsToUnderlying`.

## P7 — Spend is recorded once, from the provider

Only provider-reported numbers reach the ledger. Estimates are never recorded as
spend: a statement about money assembled from guesses is worse than no statement.

`settle` is the only place spend is written, which makes double counting a
structural impossibility rather than a convention.

*Enforced by:* `TestNoDoubleCounting`, `TestRepeatedProviderReportsForOneCallAccumulate`,
`TestRefusedRequestNeverReachesTheProvider`.

## P8 — The receipt is produced by the pass that produced the request

An audit artifact generated separately from the thing it audits always drifts.
`Receipt` is emitted by `settle`, in the same call that admitted and dispatched,
so it cannot describe a request that was not sent.

## P9 — Correctness outranks economy

Nothing in this package trades a correctness property for a cheaper request. When
the economic controller lands, a stale precondition or a changed requirement will
force the expensive path regardless of what the arithmetic says.

## P10 — Receipts observe, they never authorize

Telemetry explains a decision. It never becomes executive truth. The Mangle
kernel's `permitted/3` remains the sole authority over what the agent may do; the
broker only decides whether a request fits and records what it spent.

## P11 — Latency is a first-class cost, and is not yet priced

The broker records wall-clock duration on every receipt, so the distribution is
measurable. It is deliberately **not** a term in any decision the broker makes,
because there is no decision to make yet.

This is called out as a principle rather than a gap because of what happens when
the routing work lands: an optimizer that is not charged for time will trade
fifteen seconds for two hundred tokens, forever, and will look excellent on the
cost dashboard while users quietly stop using the tool. This is a TUI. A human is
watching a cursor.

Pricing latency is a hard precondition on Phase 4, not a follow-up.

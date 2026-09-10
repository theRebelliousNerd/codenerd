# TODO

Ordered by dependency. Items marked **[gate]** must be answered with data before
the work they gate begins.

## Now

- **[gate] Calls-per-epoch histogram** over the receipt ring. Gates Phase 4
  entirely. ~1 day. See Q1.
- **[gate] Atom co-use analysis** over successful turns. Gates the Phase 5 lane
  taxonomy. See Q2.
- **`ActivatedFacts` — populate it.** Parts 2 and 3 are **done**: it is in
  `Hash()` (sorted, so map iteration order cannot destabilise it) and
  deep-copied by `Clone()`, with six tests including a concurrent one under
  `-race`. Only part 1 remains — populate from
  `Compressor.GetActivationScores()` and read it in the selector — and it is now
  a one-line change with no trap behind it.
- ~~Reconciliation alarm~~ — **done.** `internal/broker/reconcile.go` accumulates
  per-model estimate-vs-billed drift and warns once per model past 20 samples
  and 10% mean absolute error. Mean absolute error is the headline rather than
  net bias, so an estimator wrong by 30% on every call is not scored as perfect
  because its errors happened to balance.
- ~~Runtime sentinel-client test~~ — **done.**
  `internal/perception/broker_sentinel_test.go` builds a client through the real
  exported constructor for all ten providers and both CLI engines and asserts
  each is metered, then drives a refusal end to end to prove a receipt is
  emitted — no API key, no network, no fixture server.

## Next — Phase 1, observation codecs

- ~~Tail-aware test-output codec~~ — **withdrawn.** The claim behind it was
  wrong: `ClampText` was already head+tail. The genuine narrower defect
  (byte-offset cuts landing mid-line) is fixed on this branch. A structural
  test-output codec is still worth building, but as structure, not as rescue.
- Code-search codec: symbols and dependency edges, not matching lines.
- File-read codec: exact source around the edit plus a precondition hash.
- Subagent-return codec: findings, evidence refs, changed artifacts, verification
  status, remaining uncertainty — not the transcript.
- Generalize the MCP elision/handle mechanism rather than building a second one.
- **Hydration must read the retained artifact, never re-run the tool.** Re-running
  gives a different answer from the one the reasoning was built on.

## Then — Phase 2, the typed graph

- Declare `supports`, `contradicts`, `supersedes`, `depends_on`, `satisfies`,
  `invalidates`.
- Obligation-driven selection rules extending `context_compilation.mg`.
- Result reuse keyed on dependency-version validity — this skips whole
  inferences, which is worth more than every prefix optimization combined.
- `independent_support` as a derived predicate: count distinct evidence roots,
  not distinct agents.
- **Carry the activation caps forward.** The current caps and the 105 threshold
  are an injection defence; a rewrite could quietly lose it.

## Then — Phase 3, provider fidelity

- `types.Message` must carry ordered native content blocks, signatures, ids and
  continuation references losslessly, across all seven adapters.
- Provider profile: supported continuation modes, compaction, reminder
  placement, **and cache economics** (write penalty ÷ read discount), so Phase 4's
  break-even is derived per provider rather than hard-coded.

## Later — gated

- Phase 4 economic rebasing. Preconditions: Gate A passed, Phase 3 landed,
  latency a term in the objective, hysteresis, correctness interlock, manual
  override.
- Phase 5 lanes. Preconditions: Gate A passed, Phase 2 landed, single-writer or
  worktree isolation for concurrent mutation, one global admission controller.

## Housekeeping

- Raise broker coverage from 76.5%; the gaps are in `default.go` reconfiguration
  paths and the rarely-taken degradation branches.
- Configure a compression purpose budget once Q6 has a number.
- Consider surfacing `Meter.Receipts()` through a `nerd` subcommand — the data is
  there and nothing displays it.

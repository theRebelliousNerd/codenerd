# TODO

Ordered by dependency. Items marked **[gate]** must be answered with data before
the work they gate begins.

## Now

- ~~**[gate] Calls-per-epoch histogram**~~ — **built.** `internal/broker/epoch.go`
  fingerprints each request's cacheable head (tool definitions then system
  prompt, in wire order) onto the receipt, segments receipts into epochs per
  (scope, provider, model), and reports the distribution with a per-provider
  break-even derived from published cache economics rather than hard-coded.
  Read it with `nerd meter epochs`. **The gate is now a matter of running
  sessions and looking**, not of building anything.
- ~~**[gate] Atom co-use analysis**~~ — **built.** `internal/prompt/couse.go`
  records which atoms are selected together per compilation, settled against the
  turn's outcome, and reports lift, Jaccard, clusters and category alignment.
  Read it with `nerd meter atoms`. Same status: the gate needs data, not code.
- **`ActivatedFacts` — do not populate it yet.** The earlier note here claimed
  this was "a one-line change with no trap behind it". That was wrong, and the
  two real blockers are now recorded on the field itself:

  1. **There is no relation between facts and atoms.** Every selector dimension
     on `PromptAtom` is a context dimension — mode, phase, verb, shard,
     language, framework, model, provider, world state. None is fact-shaped.
     "Boost atoms related to hot facts" needs a definition of *related* that
     does not exist in the corpus schema. Populating the map hands the selector
     data it has no rule to act on.
  2. **It is in `Hash()`, and `Hash()` is the prompt cache key.** Activation
     scores move every turn, so live scores give every compilation a unique key
     and switch the prompt cache off — every turn becomes a full compile.
     `TestActivatedFactsWouldDefeatThePromptCache` pins this, and demonstrates
     the shape of the fix (quantize to coarse buckets, or carry the set of hot
     facts rather than exact scores).

  `Compressor.GetActivationScores()` is likewise called by nothing. The producer
  and the consumer both exist and neither is connected, because the thing
  between them was never designed.
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
- ~~Configure a compression purpose budget~~ — **moot; see Q6.** The compressor
  makes no LLM calls: `Compressor.generateSummary` is its only call site and is
  dead, replaced by kernel-driven observation masking. Compression costs zero
  tokens today, so `PurposeCompression` is on the exemption list with that
  reason and the test fails if it starts being tagged again without the
  exemption being removed.
- ~~Surface `Meter.Receipts()` through a `nerd` subcommand~~ — **done.**
  `nerd meter`, `nerd meter epochs`, `nerd meter atoms`, each with `--json`.
  Both measurement streams persist to rotating JSONL logs under `.nerd/meter/`
  via `internal/jsonl`, because every question they answer spans processes and
  the readout is itself a different process from the agent that spent the
  tokens.

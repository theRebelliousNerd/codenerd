# TODO

Ordered by dependency. Items marked **[gate]** must be answered with data before
the work they gate begins.

## Now

- **[gate] Calls-per-epoch histogram** over the receipt ring. Gates Phase 4
  entirely. ~1 day. See Q1.
- **[gate] Atom co-use analysis** over successful turns. Gates the Phase 5 lane
  taxonomy. See Q2.
- **`ActivatedFacts`, all three parts together** — the cheap test of the
  meaning-driven-compilation premise. Any subset introduces a new defect:
  1. populate from `Compressor.GetActivationScores()`
  2. add to `CompilationContext.Hash()` — currently absent, so different hot
     facts collide on one cache entry and serve a stale prompt with no error
  3. deep-copy in `Clone()` — `clone := *cc` shares the map, and compilation runs
     under `singleflight` with an `errgroup`
- **Reconciliation alarm**: accumulated estimate vs. accumulated actual per
  model; alarm on divergence. Closes the blind spot in Q4 for providers with no
  counting endpoint.
- **Runtime sentinel-client test** alongside the static audit. Install a sentinel
  client and assert every live path emits a request manifest. The current audit
  parses `client_factory.go`; a sentinel catches paths the parser cannot see —
  dynamic construction, injected overrides, and anything added outside that file.
  Raised by the Evidence-First Context Compiler report, Ch. 11 Phase 0.

## Next — Phase 1, observation codecs

- **Tail-aware test-output codec.** `ClampText`/`ClampHead` truncate from the
  front; Go test output puts the decisive failure at the end. This is a live
  defect.
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

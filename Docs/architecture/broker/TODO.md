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

  **Half of Q1 turns out to be answerable from the code, and it sharpens the
  other half.** An epoch in this architecture is not a session, it is one
  turn's tool loop. The system prompt handed to a provider is
  `compileResult.Prompt` plus `withFileContext` for the current target, so it
  changes from turn to turn by construction — that is what JIT context
  management *means*, and no cache strategy will talk it out of it. What holds
  still is the system prompt inside one turn's native loop, where
  `runToolLoop` passes the same string into every round while only the message
  history grows, and messages are deliberately outside the fingerprint.

  So the ceiling on epoch length is the tool-loop round count, capped by
  `budget.iterationLimit` — and the Piggyback structured-output path runs a
  single iteration by design, so epochs are 1 there whatever else is true.
  That makes the aggregate median ambiguous in a way that decides Phase 4 the
  wrong way round: a p50 of 1 means either turns that used no tools, where
  there is no loop to lengthen, or a tool loop that is not reusing its prefix,
  which is exactly what Phase 4 fixes. `nerd meter epochs` now splits by call
  shape for that reason. **Read the by-shape table, not the headline.**

  It also names the Phase 4 design input. If the prefix is going to move every
  turn anyway, the lever is not a better rebuild controller but request
  *ordering*: a stable skeleton ahead of the volatile JIT selection ahead of
  the file context, so the cacheable head is the part that does not move. That
  is a change to how the prompt is assembled, not to how it is cached, and it
  should be settled before any controller is built on top of it.
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
- ~~Code-search codec: symbols and dependency edges, not matching lines.~~ —
  **built.** `internal/observation` composes `internal/retain` with a
  code-search projection: each match resolves to the innermost element
  containing it, and only dependency-bearing edges are kept (`references`,
  `imports`). Wired into the two live producers — the `search_code` tool and
  the VirtualStore action — plus a `search_expand` verb to redeem a handle,
  registered through the tool registry, the effect table, `safe_action` and
  `coreTools`, because a handle the model cannot redeem is a promise it is
  structurally unable to keep.

  **Hydration cannot re-run, structurally rather than by discipline.**
  `CodeSearch` holds a `*retain.Store` and nothing else; the source reader is
  an argument to `Encode`, so `Hydrate` has no reader to reach for. A
  reflection test pins the field list.

  Measured on this repo, codec vs raw grep output: `logging.Tools` 2374 vs
  9246 bytes, `ActionResult` 3653 vs 9985, `WorkspaceRoot` 4428 vs 8687,
  `ClampText` 1653 vs 2957, and the honest worst case `retain.` at 2094 vs
  2019 — a wash on a search with one hit per symbol, while still carrying more
  structure. The first rendering was *larger* than grep on exactly that shape,
  which is why `TestResultText_ShouldNameEachSymbolOnce` exists.
- File-read codec: exact source around the edit plus a precondition hash.
- Subagent-return codec: findings, evidence refs, changed artifacts, verification
  status, remaining uncertainty — not the transcript.
- ~~Generalize the MCP elision/handle mechanism rather than building a second
  one~~ — **done.** Retention now lives in `internal/retain` and MCP composes
  it with its JSON projection. The seam is retention versus projection:
  retaining bytes under a content-addressed id, with TTL / byte / entry
  ceilings and an eviction hook, is not MCP-specific, while shaping a JSON
  payload through a pointer and a view is. Each codec below brings its own
  projection and reuses the retention.
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
  continuation references losslessly, across all seven adapters. **Not started,
  and this is the whole of what Phase 4 is blocked on now** — see below.
- Provider profile: supported continuation modes, compaction, reminder
  placement, **and cache economics** (write penalty ÷ read discount), so Phase 4's
  break-even is derived per provider rather than hard-coded.

  **The cache-economics half of this is done**, and the roadmap did not say so.
  `internal/broker/epoch.go` carries `CacheEconomics` per provider — write
  multiplier, read multiplier — and `BreakEvenCalls()` derives the threshold as
  `(write−read)/(1−read)`, which is a call count independent of prefix size and
  `+Inf` where reads are not discounted. An unknown provider gets `noCaching`
  rather than a plausible-looking guess. So Phase 4's break-even is already
  derived per provider, which is exactly what this line asked for.

  It also carries a **TTL** per provider, which this line did not ask for and
  which turned out to matter more than expected: an epoch whose wall-clock span
  outruns the provider's cache window was evicted between calls, so it looks
  long enough to pay and cannot. Anthropic and OpenAI hold an entry ~5 minutes,
  Gemini and DeepSeek an hour — and a single high-reasoning call can run 2-5
  minutes, which eats most of the shorter window. `nerd meter epochs` scores
  those epochs separately in its EXPIRED column. Counting calls without
  measuring time is how a cache strategy gets approved on paper and loses money
  in production.

  Still missing from the profile: continuation modes, compaction, reminder
  placement. Those are Phase 4 inputs but not break-even inputs.

## Later — gated

- Phase 4 economic rebasing. Preconditions: Gate A passed, Phase 3 landed,
  latency a term in the objective, hysteresis, correctness interlock, manual
  override.

  Two of those preconditions have moved and the list should say so. Gate A now
  needs sessions run rather than code written, and the break-even half of
  Phase 3 has landed — so what actually blocks Phase 4 is the ordered native
  content blocks, plus the design question Q1 surfaced: **the prefix moves
  every turn by construction**, because the system prompt is the JIT
  compilation plus the current target's file context. A rebuild controller
  built on a head that never holds still is optimising the wrong layer. Request
  ORDERING — a stable skeleton ahead of the volatile selection ahead of the
  file context — has to be settled first, and that is a change to how the
  prompt is assembled rather than to how it is cached.

  On "latency a term in the objective": worth being precise, because it is easy
  to read as an efficiency goal and it is not one. Latency is recorded on every
  receipt and priced into no decision. The single place wall-clock enters is
  cache TTL, above, which is a token question wearing a clock.
- Phase 5 lanes. Preconditions: Gate A passed, Phase 2 landed, single-writer or
  worktree isolation for concurrent mutation, one global admission controller.

## Housekeeping

- ~~Raise broker coverage from 76.5%~~ — **done: 94.1%.** The gaps that mattered
  were not the arithmetic. `passthrough.go` — the capability-preservation
  surface the eight-shape design exists to protect — was at 0%, and every
  forwarder is now checked in both directions, because the whole design rests on
  a forwarder producing exactly what the unwrapped client would have. The eight
  shapes were tested for what they *advertise* and never for whether the
  advertised method works; a shape delegating to the wrong core method would
  pass the capability matrix and fail at runtime looking like a provider
  problem. `Configure` is now exercised for real rather than through a replica
  of its logic, including the cap-change path that must carry accumulated spend
  across a ledger rebuild.
- ~~Speculative broker surface~~ — **removed.** `Meter.Calibrator`,
  `Meter.Receipts`, `Meter.Reconciler`, `Meter.Drift`, `Meter.PrimaryModel` and
  `ReceiptFunc` had no production caller: the readout reads the workspace log,
  because a `nerd meter` invocation is a different process from the agent that
  spent the tokens, so the in-process getters had nothing to serve. The repo's
  own `scripts/deadcode-budget.sh` is what surfaced them.
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

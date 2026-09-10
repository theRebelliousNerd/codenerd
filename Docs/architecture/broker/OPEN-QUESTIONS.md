# Open questions

Questions whose answers would change the design. Each records what is unknown,
why it matters, and what would settle it.

## Q1 — What is the real distribution of calls per epoch?

**Matters because** the rebuild controller in Phase 4 breaks even after a
computable number of calls against a warm prefix. If the median epoch is shorter
than that, the whole controller is dead weight.

**Settled by** a histogram over the receipt log across real sessions.
**The instrument is built**: `nerd meter epochs`. Receipts carry a fingerprint
of the cacheable head and a session scope; `broker.Segment` cuts them into
epochs and `broker.Histogram` reports the distribution with a per-provider
break-even derived from published cache economics — `(write-read)/(1-read)`, a
call count independent of prefix size, and `+Inf` where reads are not
discounted, so an unknown provider gets no credit for a cache it may not have.

Two ways an epoch can look profitable and not be are scored separately: one
whose span outran the cache TTL was evicted between calls, and one with no
cacheable head was never a candidate.

What remains is running real sessions and reading the number.

## Q2 — Do atoms actually cluster the way the lane taxonomy assumes?

**Matters because** architecture / implementation / verification is how *humans*
organize software teams. That is not evidence about how *information* clusters.
The natural cut might be "code under active edit vs. everything else", or
per-package, or something nobody would guess.

**Settled by** co-use analysis: which atoms are selected together in
compilations belonging to turns that succeeded. **The instrument is built**:
`nerd meter atoms`.

The sample unit is one selection, not one turn: a turn can compile several
prompts (its own, plus one per shard), and merging them would report atoms as
co-used when they were never in the same prompt — the exact false conclusion the
analysis exists to test for.

The statistic is lift rather than co-occurrence. A skeleton atom in every prompt
co-occurs with everything more than any real pair does, so a count-ranked list
puts the least informative atom at the top of every row; lift scores it at
exactly 1, which is independence. The headline is CATEGORY ALIGNMENT: high means
atoms are already used along the existing taxonomy, low means they are not — and
that a lane taxonomy copied from an org chart would fit the data even worse.

What remains is running real sessions and reading the number.

## Q3 — What fraction of parallel consultations get invalidated in flight?

**Matters because** lane B runs while lane A applies an edit; B's answer arrives
citing a hash the world has moved past. If that fraction is high, the parallelism
being paid for evaporates and Phase 5's premise fails on operational grounds
rather than architectural ones.

**Settled by** instrumenting version staleness once any concurrent lane exists.
Nothing measures it today.

## Q4 — How far does `measure()` drift from what each client actually sends?

**Matters because** the calibrator absorbs systematic bias, which is the intent —
but it means a genuinely broken `measure()` produces a plausible ratio and
correlated wrong counts.

**Partially settled**: on Anthropic, `EstimateErrorPct` compares against an exact
count, so a near-zero value proves `measure()` is sound there. For providers with
no counting endpoint there is no independent check, and drift would be invisible.

**Settled by** `internal/broker/reconcile.go`, which accumulates per-model
estimate-versus-billed drift and warns past 20 samples and 10% mean absolute
error. `nerd meter` reports the same per model, headlined on mean absolute error
rather than net bias: an estimator wrong by 30% on every call in alternating
directions has a bias near zero and is not remotely trustworthy.

## Q5 — Is a process-scoped ledger the right scope?

Correct today: one workspace per process. Wrong the moment codeNERD serves
multiple workspaces in one process, at which point unrelated sessions would share
a budget. Recorded so the assumption is visible rather than discovered.

## Q6 — Should compression spend be capped by policy?

The ledger supports per-purpose budgets and none are configured. Compression is
inference spent to reduce inference; an unbounded compressor can cost more than
the history it shrinks.

**Partially settled**: compression is now tagged `PurposeCompression` at
`Compressor.BuildContext`, so what it *costs* is measurable — `nerd meter`
reports it as its own row. What it *saves* is still unmeasured, and the right cap
is a fraction of the saving, so the number is still one measurement away.

## Q7 — Who owns a subagent's obligations?

Unaddressed across the entire idea set. Does the parent's task graph flow down?
Does the child's evidence flow back as typed atoms or as prose? Today a delegated
turn gets 6 messages and 24,000 characters from a ring buffer, which is why the
existing multi-agent structure degenerated into context-starved one-shots.

Phase 5 cannot be designed honestly until this is answered.

## Q8 — What is the right latency budget for a turn?

P11 says latency must be priced before any routing work. Pricing requires a
target: what does a user tolerate before abandoning? Receipts now record
duration, so the distribution is finally observable — but the acceptable
threshold is a product decision nobody has made.

## Q9 — Should a refusal degrade rather than fail?

Today a refused request returns an `AdmissionError` and the turn fails. The
alternative is to shed context and retry automatically — which is friendlier and
also a silent quality change the user never sees.

Current position: fail loudly. Shedding context invisibly is how a system stops
being trustworthy. Revisit when the task graph can tell the difference between
evidence that is safe to drop and evidence that is load-bearing.

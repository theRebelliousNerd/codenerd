# Open questions

Questions whose answers would change the design. Each records what is unknown,
why it matters, and what would settle it.

## Q1 — What is the real distribution of calls per epoch?

**Matters because** the rebuild controller in Phase 4 breaks even after a
computable number of calls against a warm prefix. If the median epoch is shorter
than that, the whole controller is dead weight.

**Settled by** a histogram over the receipt ring across real sessions. About a
day's work. It gates months of build and should be the next thing anyone does.

## Q2 — Do atoms actually cluster the way the lane taxonomy assumes?

**Matters because** architecture / implementation / verification is how *humans*
organize software teams. That is not evidence about how *information* clusters.
The natural cut might be "code under active edit vs. everything else", or
per-package, or something nobody would guess.

**Settled by** co-use analysis: which atoms are selected together in turns that
succeeded. Measure before hard-coding an org chart.

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

**Would be settled by** a periodic reconciliation: compare accumulated estimates
against accumulated provider actuals per model and alarm on divergence.

## Q5 — Is a process-scoped ledger the right scope?

Correct today: one workspace per process. Wrong the moment codeNERD serves
multiple workspaces in one process, at which point unrelated sessions would share
a budget. Recorded so the assumption is visible rather than discovered.

## Q6 — Should compression spend be capped by policy?

The ledger supports per-purpose budgets and none are configured. Compression is
inference spent to reduce inference; an unbounded compressor can cost more than
the history it shrinks.

**Unsettled**: the right cap is a fraction of what compression saves, and nothing
measures the saving yet.

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

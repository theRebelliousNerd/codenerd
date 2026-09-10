# Roadmap and gates

The idea set behind this corpus is substantially larger than what shipped. This
document records the rest, in dependency order, with the evidence that would
justify each step — because the most expensive parts are also the ones most
likely to be waste.

Three independent arguments converge on this ordering:

1. An optimizer built on a broken meter exploits the meter. Instrument first.
2. Protocol and integrity properties must be proven before performance is
   measured, or you optimize against a bug.
3. Compare against a *strong* single-agent baseline, not a bloated one, or the
   headline number is measured against a strawman you were already planning to
   kill.

## Phase 0 — Measurement — **DONE**

One accounting boundary, provider-backed counting, one ledger, receipts.
This corpus.

## Gate A — Two histograms

Both computable from receipts. Both cheap. Both gate expensive work.

| Histogram | Gates | Kill condition |
|---|---|---|
| Calls per epoch | the rebuild controller (Phase 4) | median epoch shorter than the break-even → the rebuild never pays; do not build it |
| Atom co-use in successful turns | the lane taxonomy (Phase 5) | the natural clusters are not architecture / implementation / verification → do not hard-code that org chart |

Neither exists yet. Both are roughly a day's work against the receipt ring, and
between them they decide months of build.

## Phase 1 — Observation codecs — highest value, lowest risk

Prune **before** the model sees output, not after. Pruning an output after
generating it does not refund its generation cost.

**Measured as the single largest lever available: −33.0% modeled cost** against
raw append, in the executed simulations accompanying the Evidence-First Context
Compiler report. Every other layout intervention in that suite was second-order
by comparison. Until this lands, nothing else in this roadmap should be started.

Three opportunities, in decreasing value and increasing risk:

1. **Before execution** — request the fields, symbols, ranges, or diagnostics
   needed, rather than everything.
2. **After execution, before exposure** — project raw output into a
   task-relevant form and retain the raw artifact behind a handle.
3. **After exposure** — compact, through a provider-valid transition only.

Codecs are domain-specific and deterministic, not LLM summarization:

- test result → failing assertion, distinguishing frames, command, revision.
  Note that the framing here was previously wrong: truncation is already
  head+tail and, since this branch, line-aware, so the verdict line survives.
  What a codec adds over a bounded excerpt is *structure* — naming the failing
  assertion and its frames rather than keeping some bytes from each end
- code search → matching symbols and dependency edges, not every line
- file edit → exact current source around the edit plus a precondition hash; a
  paraphrase is not an adequate substitute for code about to be modified
- subagent return → findings, evidence references, changed artifacts,
  verification status, remaining uncertainty — not the exploratory transcript

Hydration reads the retained artifact. It must never re-run the tool: re-running
gives a different answer from the one the reasoning was built on, silently
rebasing evidence under conclusions already drawn.

codeNERD already has the seed of this in the MCP layer, which shapes results,
records elisions, and retains withheld output behind an expandable handle.
Generalize that rather than building a second mechanism.

## Phase 1.5 — The `ActivatedFacts` fix — the cheap experiment

`prompt.CompilationContext` declares `ActivatedFacts`, documents it as populated
by the compressor, and nothing populates or reads it. The premise that prompt
compilation should be driven by what is semantically hot is therefore untested —
and this is the cheapest available test of it.

**Three coordinated changes. Any subset introduces a new defect:**

1. Populate the map from `Compressor.GetActivationScores()`.
2. Add it to `CompilationContext.Hash()`. It is currently absent, so two turns
   with entirely different hot facts collide on one cache entry and the model is
   served a stale prompt, with no error.
3. Deep-copy it in `Clone()`. `clone := *cc` copies the map by reference, and
   compilation runs under `singleflight` with an `errgroup` — a data race, not a
   hypothetical.

Do this before Phase 2. If boosting prompt atoms by hot facts moves nothing
measurable, the meaning-driven-compilation premise is in question, and that is
worth knowing for a week of work rather than six months.

## Phase 2 — The typed task graph — the actual moat

Add the edges the fact store does not carry: `supports`, `contradicts`,
`supersedes`, `depends_on`, `satisfies`, `invalidates`. In a Datalog kernel these
are one-line declarations, which is why codeNERD is unusually well placed here
and has not yet done it.

What they buy:

**Obligation-driven selection.** Walk backward from live obligations through
typed dependencies, rather than forward from cosine distance. A constraint like
"the public API must not change" has near-zero lexical overlap with a discussion
full of channel and goroutine terminology, and a similarity ranker drops it at
exactly the moment it matters most.

**Result reuse that skips the inference entirely.** Everything in the economics
below discounts tokens you still send. This deletes the call — input, generation,
reasoning. Generation is where the money is.

The simulations put a judgement on this directly: **evidence-result memoization
was more consequential than agent splitting.** That promotes this above every
part of Phase 4 and Phase 5, and it is the strongest available argument for
doing the typed graph before anything multi-agent. Validity becomes a derivation over
dependency versions rather than a TTL guess: if any source a conclusion rests on
has moved, the conclusion is invalid because the rule says so.

**The distinction agreement cannot make.** Three specialists reaching the same
conclusion from the same source span is one piece of evidence, not three. Every
voting and ensemble system in production counts it as three, which is how
confidence gets laundered. With provenance edges, independent corroboration is a
derived predicate: count distinct evidence roots, not distinct agents. This is
the one capability in the whole idea set that a similarity pipeline structurally
cannot replicate.

Carry the activation caps forward. The current dependency/campaign/issue/back-ref
caps and the 105 threshold are an injection defence — they stop one crafted fact
monopolizing the atom reserve. A rewrite could quietly lose that.

## Phase 3 — Lossless native round-trip — prerequisite, not polish

`types.Message` must carry arbitrary ordered provider-native content blocks,
signatures, ids, and continuation references losslessly. Provider reasoning state
is opaque and may be bound to its preceding prefix; a turn reconstructed from
visible text is not the same turn.

Nothing in Phase 4 or 5 is safe until this lands. It is boring plumbing across
seven adapters and it is a hard blocker.

## Gate B — Protocol and integrity tests

Cheap, deterministic, and they must pass before any performance measurement:

- native blocks survive a round trip unchanged
- tool results stay paired with their calls
- stale source cannot authorize an edit
- contradictory evidence is never silently erased
- every emitted request is budgeted at the real outbound boundary *(shipped)*

## Phase 4 — Economic layout transitions — only if Gate A says yes

Update meaning continuously; change physical layout selectively. Between
reorganizations, preserve the request prefix and append.

The arithmetic is real. With a cache-write penalty and a cached-read discount,
replacing a warm 30,000-token region with a 12,000-token one saves ~22% of that
input component over twelve calls and breaks even at eight.

Two things make it dangerous to build early:

**The break-even is per-provider, and the sign flips.** It depends entirely on
the ratio between write penalty and read discount, and providers price this
differently with different TTLs. It must live in a provider profile — the same
structure that already carries feature support — never a constant in Go.

This is no longer a prediction. Executed simulations in the Evidence-First
Context Compiler report measured a periodic epoch policy at **−11.66%** against
projected append at a 0.10 cached-read multiplier, and **+2.6%** — a loss — at
0.025. Same policy, same traces, opposite conclusion. A hard-coded rebuild
schedule is therefore a correctness risk rather than a tuning risk. The
report's own twelve-decision schedule is explicitly arbitrary and must not be
copied. See `Docs/architecture/broker/reference/2026-09-09-report-vs-shipped.md`.

That work also measured the cost of churn directly: an every-step reordering
policy cost **61.5% more** than projected append, and the cheaper epoch policy
had a *lower* cache fraction than the more expensive one (91.1% vs 96.0%).
More cache hits was not the cheaper plan.

**Cache cost is not additive.** Think of the request as a train: appending cars
is nearly free, but inserting one in the middle re-couples everything behind it.
A ranker scoring each atom with an independent cached-token discount treats every
car as if it were at the back, and will confidently make things worse.

Required properties: hysteresis (or the system thrashes its own cache when two
scores swap by a fraction of a percent), a correctness interlock (a stale
precondition or changed requirement forces the expensive path regardless of the
arithmetic), a manual override, and **latency as a term in the objective, not a
field in the record** — see P11.

Cache-hit rate is the wrong objective. A 100% hit rate is a very efficient way to
keep being wrong.

## Phase 5 — Selective lanes — only if Gate A says yes

Several persistent contexts, each activated only when it can contribute something
necessary. Not a committee around every tool call.

**The honest control, which must be stated in any evaluation:** if the
single-agent compiler already selects only the relevant material, partitioning
provides no token advantage at all. Lanes must then earn their keep through
continuity, reduced re-investigation, parallelism, or independent verification —
each of which needs its own measurement. The tempting 90k→30k arithmetic is
measured against a bloated baseline this roadmap is already planning to
eliminate, and it does not belong in any external claim.

The evidence base is thinner than it looks: aggregating multiple outputs from one
strong model has been found to beat multi-model mixtures, and multi-agent
benefits vary sharply by workload — with sequential planning deteriorating.
Coding is substantially sequential. So: lanes for the decomposable minority, one
strong loop for the sequential majority, and the router's **default is one lane**.

The executed simulations sharpen this considerably. Against a *lean* single agent
with equivalent result memoization, sparse lanes produced mean paired changes
from **−8.06% to +0.37%** across four synthetic workloads — a range that includes
being worse. Always invoking all three lanes was "much more expensive". That is a
weak case, and it makes Gate A load-bearing rather than merely prudent.

Before building any of it, answer the question the idea set never asks: codeNERD
already has subagents, a spawner, and campaign orchestration. **Why did the
multi-agent structure that already exists degenerate into context-starved
one-shots?** The apparent answer is that with no shared task state every spawn had
to be handed a prose blob, and prose blobs get capped. Lanes without Phase 2 are
subagents with better branding.

Hard constraints when it lands:

- **Parallel reasoning does not justify concurrent mutation.** Single authorized
  writer, or isolated worktrees with an explicit merge boundary. Read-parallelism
  and write-parallelism are different risk classes that share a word.
- One global admission controller, one total budget, bounded depth. A three-way
  tree three levels deep is forty agents.
- Merge findings, never opaque reasoning state.
- A presenter can be cheap; an arbiter cannot. Choose the integrator's tier from
  the *shape* of what came back, not statically.
- Lane dormancy has economics nobody has computed: a dormant lane wakes cold, and
  there is a crossover past which rebuilding from the graph is cheaper than
  keeping it warm.

## What the whole thing is for

Not "we compile context" — everyone will claim that within a year, and half will
mean "we call a model to summarize".

The defensible claim is: *we can show you why every token is in the window, why
this reasoning happened here, on this evidence, at this cost — and prove the
constraint you set on turn 3 is still enforced on turn 43.*

Phase 0 shipped the cost half of that sentence. Phase 2 is the rest.

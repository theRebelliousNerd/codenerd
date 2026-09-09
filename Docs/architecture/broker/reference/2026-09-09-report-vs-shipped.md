# The Evidence-First Context Compiler report vs. what shipped

> Compares [`2026-09-09-evidence-first-context-compiler.md`](2026-09-09-evidence-first-context-compiler.md)
> (external report, repo snapshot `7fd2762d`) against `internal/broker` as merged
> on branch `claude/jit-context-management-2mv7tq`.
>
> The two were produced independently and without knowledge of each other.

## Verdict

**They converge on the same first move, for the same reason, and the report's
executed simulations largely confirm the built design — while also weakening the
case for the most expensive things neither of us built.**

The report's Chapter 11 Phase 0 is: *"a universal inference boundary … normalized
usage … a provenance manifest … in production, network-capable model clients
should only be reachable through the broker … no downstream helper may append
unbudgeted text after final rendering."*

That is a description of what shipped, written by someone who had not seen it. It
even names the interface `InferenceBroker`; the package is `internal/broker`.

The report's closing line — *"the first goal is not more reasoning threads. It is
fewer unnecessary inferences and better information in the inferences that
remain"* — is the same argument as the shipped corpus's P1 (*instrument before you
optimize*), reached from economics rather than from measurement discipline.

## Independent confirmations

Findings each side reached separately. Agreement here is worth more than
agreement on architecture, because these are checkable facts about the code.

| Finding | Report | Shipped work |
|---|---|---|
| `ActivatedFacts` is absent from `CompilationContext.Hash()` | Ch. 11 Phase 2: *"the existing context hash does not currently include that map"* | `02-CURRENT-STATE.md`, `TODO.md` item 2 |
| `ActivatedFacts` is not deep-copied by `Clone()` | Ch. 2: *"activation fields are not fully represented in identity/copy behavior"*; Ch. 11: *"deep-copy or otherwise freeze its contents"* | `TODO.md` item 3 — a data race under `singleflight` |
| Blanking exposed tool payloads ≠ selecting a smaller first exposure | Ch. 2: *"materially different … must distinguish these operations instead of calling both 'compression'"* | gap B8, Phase 1 |
| `types.Message` cannot express ordered native blocks | Ch. 2: *"a representation limitation, not proof that every adapter currently mishandles reasoning"* | gap B11, Phase 3 |
| Summary-plus-recent-turns is unsafe for bound reasoning | Ch. 2 on subagent memory | `12-FAILURE-MODES.md` |
| The MCP elide/handle contract is the right seed to generalize | Ch. 11 Phase 1 | Phase 1 in `13-ROADMAP-AND-GATES.md` |
| Cache-hit rate is a diagnostic, not an objective | Ch. 1 | `11-OBSERVABILITY.md`, `13-ROADMAP-AND-GATES.md` |
| No untracked text after final rendering | Ch. 11 Phase 0 exit gate | P4 + `TestCountedAtOutboundBoundary` |

Both also independently identified that the report's Chapter 2 table row —
*"Shared LLM interfaces … Required change: add a lossless provider envelope **and
a common accounting/admission boundary**"* — is two separable pieces of work.
Only one of them shipped, deliberately (see *Disagreements*).

## What the report has that the shipped work does not

**Executed simulations.** This is the substantive new content and it is the
reason the report is worth acting on rather than merely agreeing with. 6,144
primary runs, 8,960 sensitivity runs, 50,000 restart scenarios, 100,000 ordering
trials, seeded and reproducible.

| Result | Number | Why it matters |
|---|---|---|
| Early projection vs. raw append | **−33.0%** modeled cost | the single biggest lever, and it is Phase 1 |
| Periodic epochs vs. projected append | **−11.66%** (95% CI 11.55–11.78) | real, but second-order |
| Same epochs at a 0.025 cached-read multiplier | **+2.6%** — *epochs lose* | break-even is provider-specific, not universal |
| Cached fraction, epochs vs. projected append | 91.1% vs. 96.0% | *more cache hits was the cheaper plan's loser* |
| Volatile every-step reordering | **+61.5%** | layout churn is expensive under discounted reads |
| Sparse lanes vs. lean single agent with equal memoization | **−8.06% to +0.37%** | a weak case, workload-dependent |
| Always invoking all three lanes | "much more expensive" | conditional admission, never a universal topology |

And the headline judgement: **"evidence-result memoization was more consequential
than agent splitting."**

**Other things it has that the built work does not:**

- A **shadow-mode migration** strategy for Phase 0.
- A **runtime sentinel-client test** — install a sentinel and assert every path
  emits a manifest. The shipped audit is static AST analysis; a runtime sentinel
  catches paths the parser cannot see. This is a genuine gap and worth adding.
- An explicit **`ProviderProfile`** interface carrying `ValidateContinuation`,
  `Render`, `Count`, and `NormalizeUsage`.
- **Ordering mathematics** (Appendix A): expected surviving cached prefix under
  independent block change probabilities, with all 720 permutations checked.
- The **three-plane naming** — event journal / evidence graph / materialized view
  — which is cleaner than the shipped corpus's phrasing.

## What shipped that the report only specifies

- **The boundary itself**, working, wired, and tested: 78 packages green, race
  clean, 76.5% statement coverage.
- **Provider-backed exact counting** via `POST /v1/messages/count_tokens`,
  LRU-cached and latency-bounded. The report says *"count the fully serialized
  request where supported"*; the shipped work does it.
- **A self-calibrating estimator.** The report says *"or use a conservative
  calibrated estimate with a stated margin"* and stops there. The shipped
  estimator closes the loop: it compares its own prediction against the
  provider's reported actual on every response and corrects the ratio, so error
  shrinks as a session runs.
- **The capability-preserving decorator.** The report does not address this
  problem at all, and it is the one that would have broken things quietly.
  Wrapping every client naively tells `types.ToolResultsProvider` probes "yes"
  for Gemini, which uses a synthesized Piggyback envelope and cannot serve the
  native tool loop. Eight wrapper shapes; `TestWrapPreservesCapabilityMatrix`.
- **Deletion of the competing rulers**, enforced by static audit. The report
  specifies the new boundary but does not require removing the old ones, and a
  replacement that leaves the thing it replaces in place is not a replacement.
- **The finding the audit produced**: `NewImageClientFromUserConfig` built a
  Gemini client directly and was spending entirely off the books.

## Disagreements

**Shadow mode.** The report recommends running the new path in shadow while
preserving existing adapter behaviour. For a *meter* that is wrong: two
accounting systems running concurrently is precisely the disease being treated,
and a shadow meter cannot enforce anything. A hard cutover was taken instead.
The report is right that shadow mode de-risks the **lossless envelope** work,
where behaviour genuinely changes — that is Phase 3 and should use it.

**Phase 0 scope.** The report bundles the lossless native envelope with the
accounting boundary. Splitting them was deliberate: the envelope is a change
across seven provider adapters and bundling it would have made a 65-file change
set unreviewable. The report is right that the envelope must land before any
reordering, which is why it is Phase 3 and a hard blocker on Phases 4 and 5.

**Latency.** The report mentions deadline constraints and assigns them "an
explicit penalty", but latency does not appear in its cost model or its
simulations — CPU and wall-clock time are explicitly not simulated. That is the
same gap flagged as P11 in the shipped corpus. An optimizer not charged for time
will trade fifteen seconds for two hundred tokens forever, and look excellent on
the cost dashboard while users leave. This is a TUI.

## What the simulations change about the roadmap

Four concrete revisions, in order of how much they move things:

1. **Phase 1 (observation codecs) is confirmed as the largest single lever at
   −33%.** It was already next. It should now be treated as the only thing that
   matters until it lands.

2. **Epoch rebasing must be provider-parameterized, not merely "should be".**
   The shipped roadmap argued break-even depends on the write-penalty ÷
   read-discount ratio. The simulations demonstrate the sign actually flips:
   −11.66% at a 0.10 read multiplier, **+2.6% at 0.025**. A hard-coded schedule
   is not a tuning risk, it is a correctness risk. `13-ROADMAP-AND-GATES.md`
   should be strengthened accordingly, and the twelve-decision schedule in the
   simulation is explicitly arbitrary — do not copy it.

3. **Result memoization outranks lane splitting, and by a wide margin.** This
   promotes Phase 2's dependency-valid result reuse — the mechanism that skips a
   whole inference rather than discounting one — above everything except Phase 1.
   It also further demotes Phase 5.

4. **The case for lanes is weaker than the earlier discussion suggested.** A
   range of −8.06% to +0.37% against a *lean* single agent with equal
   memoization, with all-three-lanes "much more expensive", supports the shipped
   position (decomposable minority only, router defaults to one lane) and makes
   Gate A load-bearing rather than prudent.

Two additions to `TODO.md` follow directly:

- Add the **runtime sentinel-client test** alongside the static audit.
- Carry **cache economics in the provider profile** — the shipped roadmap already
  says this; the sign flip makes it mandatory.

## The honest caveat, which the report states itself

> *"Do not interpret these numbers as predicted codeNERD savings. The simulations
> assume that the required information survives projection and that the abstract
> information services produce equivalent useful results. They do not measure
> reasoning quality, hallucinations, task success, or time to finish a real
> repository change."*

These are accounting and scheduling experiments. They are excellent at exposing
economic failure modes — the sign flip at 0.025, the cost of churn, the weakness
of lanes — and they say nothing about whether a projected observation still
contains the decisive error. That remains the empirical question, and it is why
the shipped work built the meter first: none of it is measurable otherwise.

---
doc-class: north-star
subsystem: context
implementation-status: planned
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# 01 — Vision: Mangle-managed working context for `internal/context`

> This file answers question 1 only: what the finished package would exhibit.
> It describes nothing as shipped. Every behavior below would hold in the
> finished package; where today's code is named, it is named only as the seam
> where the planned capability would attach. For what runs today, see
> `02-CURRENT-STATE.md` and `IMPLEMENTED_SPEC.md`. For the distance, see
> `03-GAP-ANALYSIS.md`.

## North Star trace

This vision would serve three sentences from `agents.md`:

1. `agents.md:20` — "Mangle manages the active working context throughout
   execution: relevance, retention, eviction, retrieval, and ordering. A
   growing tool transcript with occasional summarization does not fulfill this
   design. Evicted context must remain recoverable; stale evidence must not
   survive a source change as current truth."
2. `agents.md:40-42` — "Quality and capability first. Tokens are the
   constraint, not the goal. Lost-in-the-middle is eliminated by context
   compression, pruning and ordering — where a fact sits in the window is a
   decision."
3. `agents.md:47-50` — "'Clean fixpoint,' not 'clean loop.' The executive
   decisions — what a turn is, whether it is done, what enters the window,
   what the verdict is — are the fixpoint of the kernel over the facts."

Supporting traces: harness decides, not model discretion (`agents.md:27-29`);
knowledge pushed when the harness decides it is needed (`agents.md:30-33`);
the SQLite / vector / CodeDOM / history blend injected at the right time for
the right reason (`agents.md:35-39`); the north star as a final state that
pressure moves every turn toward (`agents.md:55-59`).

The drift this vision would eliminate is the one named in
`agents.md:47-51`: decisions computed in Go that would instead be derived in
the kernel.

## Finished behavior (planned)

### 1. Relevance — what would enter the window would be a derived decision

In the finished package, no Go heuristic would decide what is relevant. The
kernel would derive `should_include_context/2` and `context_score/2` over the
facts asserted for the turn, the current intent, focus resolution, and
dependency closure. Go would retain only mechanical duties: stable fact keys
and stable sort of kernel-scored facts.

- What today is a nine-component Go sum in `computeScore`
  (`internal/context/activation_scoring.go:39`) — base
  (`internal/context/activation_scoring.go:59`), recency
  (`internal/context/activation_scoring.go:75`), relevance
  (`internal/context/activation_scoring.go:102`), dependency
  (`internal/context/activation_scoring.go:225`), campaign
  (`internal/context/activation_scoring.go:268`), session
  (`internal/context/activation_scoring.go:333`), issue
  (`internal/context/activation_scoring.go:347`), feedback
  (`internal/context/activation_scoring.go:454`), back-reference
  (`internal/context/activation_scoring.go:479`) — would become kernel rules
  whose premises are inspectable facts.
- The substitution point would remain `ScoreFactsWithKernelOverride`
  (`internal/context/activation_scoring.go:653`): on a kernel hit, the kernel
  score would take precedence
  (`internal/context/activation_scoring.go:668`); on a miss, the Go fallback
  with component breakdown would apply
  (`internal/context/activation_scoring.go:676`); ordering would stay under
  the same-sort contract (`internal/context/activation_scoring.go:693`).
- The threshold-vs-budget split observed today — `FilterByThreshold`
  (`internal/context/activation.go:415`) pruning at a fixed threshold while
  `SelectWithinBudgetPreFiltered` (`internal/context/activation.go:469`)
  fills without one — would be resolved by derivation: the kernel would
  decide the cutoff per turn, and Go would enforce it.

This would serve `agents.md:20` (relevance managed by Mangle) and
`agents.md:47-50` (what enters the window is a fixpoint, not a Go sum).

### 2. Retention — what would stay would be policy, not token math

In the finished package, retention would be a derived policy over safety,
task continuity, and budget — not bare token accounting. Token math would
remain as the constraint the policy reasons within, never as the policy
itself.

- Today's retention window — `recalcBudget`
  (`internal/context/compressor_turns.go:247`) filling `AtomReserve` after a
  recent window, over `TokenBudget`
  (`internal/context/tokens.go:184`) with its `used{core,atoms,history,
  recent,working}` accounts and the `SetUsage` setter
  (`internal/context/tokens.go:424`) — would be gated by a kernel
  derivation: only facts the kernel retains would be eligible for the fill.
- Today's hardcoded safety-retention list in `getCoreFacts`
  (`internal/context/compressor.go:745`), which queries `permitted,
  dangerous_action, admin_override, security_violation, block_commit`
  (`internal/context/compressor.go:759`) and never returns a silent empty
  safety block, would become a `must_retain/1` derivation: the same safety
  set would be retained because the kernel derives it, not because a Go
  literal lists it.
- Today's uncalled turn shape — `ProcessTurn`
  (`internal/context/compressor_turns.go:30`) with its ten steps
  (`internal/context/compressor_turns.go:42`), sliding window
  (`internal/context/compressor_turns.go:119`), and best-effort persist
  (`internal/context/compressor_turns.go:168`) — would either be rewired as
  the live turn path or be superseded by the working-loop path
  (`beginWorkingLoop` → `WorkingSet` + recall tool), and this vision would
  not presume which until `03-GAP-ANALYSIS.md` resolves it.

This would serve `agents.md:20` (retention managed by Mangle) and
`agents.md:40-42` (tokens as constraint, not goal).

### 3. Eviction — what would leave would stay recoverable, and stale truth would not survive

In the finished package, eviction would be kernel-gated and recoverable. No
evicted observation would become unrecoverable at the prompt layer, and no
stale evidence would survive a source change as current truth.

- The trigger-plus-compressor chain — `shouldCompress`
  (`internal/context/compressor_turns.go:241`) firing on
  `budget.ShouldCompress()`, then `compress`
  (`internal/context/compressor_turns.go:297`) with cutoff
  (`internal/context/compressor_turns.go:305`), key-atom collection
  (`internal/context/compressor_metrics.go:360`), age categories
  (`internal/context/compressor_metrics.go:671`), masked summaries, ratio
  enforcement (`internal/context/compressor_turns.go:336`), segmentation
  (`internal/context/compressor_turns.go:359`), and `DecayRecency`
  (`internal/context/compressor_turns.go:398`) — would fire and fold under
  kernel age/mask derivations rather than on token pressure alone.
- The window safety net — `pruneRecentTurns`
  (`internal/context/compressor_turns.go:715`) with its `2*window` bound
  (`internal/context/compressor_turns.go:716`) — would remain as the last
  resort, and any drop there would warn and would remain recoverable through
  the history manager rather than being lossy.
- The existing kernel-gated masking exemplar — `assertTurnAgeCategories`
  (`internal/context/compressor_metrics.go:671`) writing age categories,
  `turnMaskID` (`internal/context/compressor_metrics.go:704`) naming
  `turn_<n>`, and `maskedObservationTurns`
  (`internal/context/compressor_metrics.go:717`) reading
  `should_mask_observation` and `should_preserve_reasoning` with
  refuse-to-mask on drift — would be the pattern the other verbs converge
  toward.
- Per-round working-set eviction — `internal/context/working_set.go:480`,
  where over-budget bodies become `Omitted` with a `[body outside active
  budget; recover with recall_context]` reference
  (`internal/context/working_set.go:496`) — would be proven recoverable: the
  `recall_context` reference would resolve to the evicted body in every case
  it names.
- History rendering under reserve — `rebuildRollingSummaryText`
  (`internal/context/compressor_turns.go:556`), `mergeOldestSegments`
  (`internal/context/compressor_turns.go:611`), `renderRollingSummaryText`
  (`internal/context/compressor_turns.go:674`), and `collectKeyAtoms`
  (`internal/context/compressor_metrics.go:360`) — would account every shed
  atom in `DroppedAtoms` and would preserve turn coverage, so a reader of the
  rolling summary could tell what was folded and where to recover it.

This would serve `agents.md:20` sentence 2 directly: a growing transcript
with occasional summarization would not count as fulfillment; recoverability
and staleness-invalidation would be derived properties with tests.

### 4. Retrieval — context would arrive as the harness-decided blend

In the finished package, each compile would receive the blend the Vision
names — domain knowledge from SQLite, vector search, CodeDOM, and history —
injected at the right time, for the right reason, to the right agent
(`agents.md:35-39`), pushed because the harness decides it is needed
(`agents.md:30-33`), in as few turns as possible without double-checking
waste.

- The live funnel — `Select` (`internal/context/working_set.go:308`) with
  control facts (`internal/context/working_set.go:323`), two-hop
  `dependency_link` traversal capped at 64 entities
  (`internal/context/working_set.go:325`), per-entity `working_revision`
  (`internal/context/working_set.go:361`), `Candidates` at 256
  (`internal/context/working_set.go:373`), recent-as-working-memory, and
  `working_observation/digest/span` asserts
  (`internal/context/working_set.go:408`) — would be fed by kernel-derived
  needs rather than by Go-chosen queries alone.
- Vector-vs-Mangle combination, `DerivedNeeds` population at every compile
  boundary, and CodeDOM/SQLite/history wiring beyond the current
  selector/assembler stubs would be specified per capability in `05-…` specs
  and would be proven by tests that fail today (for example, a retrieval
  test where the kernel-derived need changes the selected set while Go
  inputs are held constant).
- Long-term retention operations — `promote_to_long_term`, `forget`,
  `store_vector` in `processMemoryOperation`
  (`internal/context/compressor_turns.go:202`), where `note` is today
  accepted-by-schema but Warn-dropped
  (`internal/context/compressor_turns.go:226`) — would be closed: `note`
  would persist and would be recallable, or the schema would narrow to the
  implemented operations.

This would serve `agents.md:30-39` (harness-forced, harness-timed blend).

### 5. Ordering — where a fact sits would be a decision

In the finished package, ordering would be derived: the position of every
fact in the window would reflect a kernel decision about prefix-cache
stability, lost-in-the-middle avoidance, and task priority — never insertion
order or an unsorted merge.

- Within this package, the same-sort contract in `sortScoredFactsDesc`
  (`internal/context/activation_scoring.go:693`) and the priority-then-step
  sort in `working_selected`
  (`internal/context/working_set.go:433`) would order by kernel-derived
  scores and priorities.
- Across the prompt boundary, the Head/Tail category order
  (`internal/prompt/assembler.go:53`), dependency-respecting topological
  order (`internal/prompt/resolver.go:48`), and Fit's priority → category →
  score allocation order would consume this package's derived scores rather
  than Go-local ones, so the decision made here would survive to the window.

This would serve `agents.md:40-42` (lost-in-the-middle eliminated by
ordering; placement is a decision).

## What this vision would not do (non-goals)

- It would not move fuzzy matching or large natural-language pattern banks
  into Mangle; embeddings and retrieval would still precede structured
  assertion, per the Mangle guardrails (`agents.md:180-187`).
- It would not give models free-form shell access to bypass missing wiring
  (`agents.md:21`); missing seams would be built as typed,
  policy-mediated operations.
- It would not relitigate the turn-path question inside this file; whether
  `ProcessTurn` or the working loop owns the live path would be decided in
  `03-GAP-ANALYSIS.md` and witnessed in `WIRING-AND-NOT-BUILT.md`.

## Buildable endpoints (gap IDs this vision would close)

These IDs will be owned by `03-GAP-ANALYSIS.md`; the exits below are the
machine-checkable criteria that vision places on them:

| Gap ID | Capability | Exit criterion (machine-checkable) |
|---|---|---|
| GAP-CTX-01 | Relevance derived | `ScoreFactsWithKernelOverride` with a non-empty kernel map on a fixture would derive the expected order; Go fallback would apply only on miss |
| GAP-CTX-02 | Retention policy derived | `must_retain/1` would derive today's safety set; a test asserting retention of `security_violation` under budget pressure would pass |
| GAP-CTX-03 | Eviction recoverable | Every `Omitted` body reference from `internal/context/working_set.go:496` would resolve via `recall_context`; kernel would mark `/old`+`/ancient` for masking while `preserve` holds, with `MaskedTurns` counted |
| GAP-CTX-04 | Retrieval blend harnessed | A compile-boundary test with fixed Go inputs and changed kernel `DerivedNeeds` would change the selected set as derived |
| GAP-CTX-05 | Ordering derived | Window order on a fixture would match kernel-derived scores under the same-sort contract; a shuffle of inputs would not change the derived order |
| GAP-CTX-06 | `note` closed | `note` would persist and would recall, or the schema would narrow; a round-trip test would prove whichever holds |

Until those exits are met, this file would remain `planned` and no reader
would mistake it for a description of the code.

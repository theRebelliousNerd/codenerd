---
doc-class: deep-dive
subsystem: deliberation
implementation-status: proposed-refinement
evidence-status: static-source-inspection
audit-date: 2026-11-03
verified-against: d1b6a95ca5863b247c10fa578d45b8db5e65c556
---

# Deliberation probes need an admitted review opportunity

## Finding and decision

The conductor can execute and persist new critical-data probes in its last permitted round even though its next action will stop before either participant can review those results. The round's assessment is produced before the probes run. Their normal consumer is the next round, but the round-cap check runs before that next round is collected. Probe admission checks a separate tool-call count, not whether a review opportunity remains. [Round order][round] [Service order][service] [Probe admission][probes] [Stop policy][policy]

This is a **bounded deliberation completion gap**, not a false-acceptance or permission-bypass claim. The host correctly returns a stopped outcome and retains the observations in its durable thread. The narrower problem is that it admits evidence collection beyond the current run's review horizon, then returns only the stop status on the ordinary design-command surface. A useful observation may exist without a current design verdict that incorporates it. [Stopped note][note] [Persistence][persistence] [Command][command]

Require an explicit consumer contract before spending the remaining probe allowance: either a review phase fits inside the already authorized resource envelope, or the probe is deferred with its unresolved requirement preserved. If collection is deliberately for a later authorized continuation, record that purpose and expose the result as **collected but not reviewed**, not as design success.

The smallest conservative correction is to stop dispatching conductor probes that cannot be reviewed in the present envelope and report the remaining work. A fuller design may reserve review capacity, but must not silently add a fifth round, extra model calls or new execution authority. No mechanism proposed here is implemented or evaluated by this audit.

### Source boundary

The initial public main read resolved to `86ec71ddeab546141b2def8610418aa6a662ee5f`. A later read at approximately 13:14 UTC resolved to **`d1b6a95ca5863b247c10fa578d45b8db5e65c556`**. The complete two-commit comparison changes unrelated learning, session/context paths and campaign/chat regression files; none of the cited deliberation, routing, mission, guidance or audit sources changed. Links below use the later pin. [Commit][pin] [Comparison][comparison]

This review inspected public guidance, architecture, implementation, relevant prior audits and authored tests. No repository file was changed. No application, test, build, model inference, benchmark, gate or fault injection was run. Only a scratch Markdown report and source records were written. Source-predicted schedules below are not runtime reproductions.

## 1 Why this matters to Socratic requirements work

codeNERD already has the requested ideation and interrogation structure. Its proposer supplies materially different designs, success conditions, assumptions and evidence. Its challenger identifies critical data, design and values blockers. The conductor and Mangle policy decide whether to refine, investigate, ask a bounded values question, proceed or stop. This audit strengthens that path rather than proposing another agent framework. [Proposer][proposer] [Challenger][challenger] [Session specification][spec]

A Socratic data question has two separate obligations:

1. Obtain the observation needed to challenge a design.
2. Reassess the design using that observation.

Finishing the first is not finishing the second. For example, reading a configuration may establish that a compatibility condition holds, contradict the proposed design, or reveal another unresolved dependency. The result needs interpretation under the candidate's original success condition. A receipt digest and a successful read are not substitutes for that assessment.

The root guidance assigns orchestration and completion obligations to the kernel, keeps model requests bounded, and requires genuine progress rather than arbitrary tool-count completion. The architecture guidance distinguishes desired guarantees from current implementation. Those principles support a consumer-aware resource contract; they do not authorize unbounded retries or weakening stop policy. [Root guidance][guidance] [Architecture guidance][architecture]

The same issue applies to foreign repositories without depending on Go, Rust, Python or JavaScript gate detection. Any admitted repository read that is requested as a conductor-level data probe has the same ordering. This is protocol portability, not a claim that all tool adapters or languages are supported.

## 2 Implemented behavior and its exact limit

### The assessment precedes its requested probes

`collectRound` constructs a frame containing the previous completed round, runs the proposer, relays any permitted specialist consultations, and then runs the challenger. It validates the returned assessment before returning the new round. The current round's critical-data probes have not yet executed at that point. [Collector][round]

The service then asserts the round, queries its policy decision, proves that decision, appends and saves the round. For an empirical verdict it invokes `runProbes`; only afterward does it update the progress fingerprint and continue the loop. [Service][service]

Thus a round's empirical verdict is a decision to investigate an unresolved candidate. It is not a post-investigation verdict. This is an important and sensible distinction to retain.

### Probe admission has no consumer-capacity condition

`runProbes` visits the selected candidate's critical data items, uses a round-scoped actor and local call counter, and checks `AllowCall` before each dispatch. That helper asserts a measured count and queries `deliberation_agent_continue`. Its policy condition is `N < Cap` for `deliberation_tool_calls`. The predicate does not join the conductor round, round limit, remaining review phase or future consumer. [Probe execution][probes] [Call rule][policy]

Each result is appended to `Round.Probes` and saved. A receipt fact then records thread, round, item key, tool and result digest. These are useful records. The policy file does not derive readiness or blocker retirement from their contents; the semantic reassessment occurs through a subsequent participant round. [Receipt publication][probes] [Policy][policy] [Next frame][round]

This report does not ask the kernel to infer semantic success from the mere existence of those receipts. Such a shortcut would manufacture evidence rather than consume it.

### The next-round gate is decisive

The next iteration begins at `len(t.Rounds) + 1`. It checks `deliberation_stop` before calling `collectRound`. The round-cap rule stops when `R > Cap`. Therefore an empirical round numbered `Cap` can execute probes, but its normal successor `Cap + 1` cannot run under the same configuration. A newly changed fingerprint cannot override that rule. [Pre-round stop][service] [Round-cap rule][policy]

Defaults are four rounds, a six-call limit for each budgeted counter, and stopping after two consecutive equal-fingerprint transitions. The conductor probe counter starts at zero each round. A round cap of one is also a valid positive configuration, and the existing authored stop test explicitly uses one. The issue is not an invalid configuration. [Configuration][config] [Bounds test][tests]

### Persistence is not a resumed consumer

The stopped outcome contains the saved thread but no chosen ready candidate. `Outcome.Note` emits only `Deliberation: <status>.` for a non-ready thread, and the design command returns that note. The outputs have not disappeared from disk; they have not been incorporated into a new assessment or summarized as pending review on this surface. [Note][note] [Command][command]

Calling `Run` again with the same saved rounds and unchanged cap starts at the already exhausted next-round number and stops again. The special replay path is for an exact answer to an `awaiting_values` thread, not an empirical `round_cap` result. The inspected pending selectors restore values questions. None of these paths supplies an automatic review continuation for this schedule. A later manual inspection or newly authorized invocation can still use the record; this audit makes no claim that the evidence is permanently inaccessible. [Run and replay][replay] [Pending selection][persistence]

### Recurse inherits the design boundary

The recurse-mission path invokes the same service with a design-only thread. It stores the thread identity and path, maps a proceed action to ready and other non-question outcomes to stopped, and continues through separate mission admission and source checks. Its specification assigns deliberation bounds to routing configuration and finite mission limits to the owning mission. [Mission preparation][mission] [Mission bounds][mission-spec]

Consequently this gap can block a useful mission design without producing an execution-admitted candidate. It does **not** show that a write escaped mission admission, a gate was weakened, a failed test was accepted, or an improvement was incorrectly retained.

## 3 Minimal conditional witness

This is an unrun source-derived schedule. It deliberately avoids the previously reported progress-identity and role-context defects.

Use a valid deliberation configuration with round cap one, a positive probe allowance, a non-exhausted no-progress threshold, short frames, a live context, a functioning guarded read adapter and working kernel proof support. Script no specialist consultations or role-internal tool calls so the conductor probe is the only read in this witness.

1. The proposer returns two valid candidates. Candidate A is first. Both have nonempty briefs and success tests.
2. The challenge supplies exactly one assessment per candidate and names A as its valid Recommendation. The challenger gives B a critical design issue and A a critical data item with a stable key and a valid read-only call. A has no other critical kind. It leaves success unresolved. B remains design-blocked. Neither candidate is ready.
3. Policy selects A as empirical for round one. The service records and proves that pre-probe decision.
4. The read succeeds and returns a short, novel observation. The service appends it to round one's probes, saves it and records its digest. All byte and call limits are respected.
5. The next loop starts at round two. Since two exceeds the cap of one, it returns `round_cap` before either role receives the new observation.
6. The ordinary design-command result is the stopped note. The saved record contains the fresh observation but still only the assessment that requested it.

**Source-predicted consequence:** one successful conductor probe is collected with no subsequent participant consumption in this run. There are no role calls after that probe. Whether the observation would support a ready design is intentionally a separate, unmeasured question.

For a completion-sensitive fixture, script an independently reviewed next assessment that accepts or rejects A only after seeing this observation. That response establishes a counterfactual test oracle; it is not evidence that a live model would reliably converge. For the scheduling claim itself, no successful hypothetical assessment is necessary.

The same construction works at the default cap: arrange three permitted nonterminal rounds without satisfying the no-progress threshold, then make round four empirical. The new result can change the fingerprint, yet round five remains prohibited. Do not use three identical repeated rounds for this witness, because that would trigger an earlier, different stop.

## 4 Distinctness from existing work

The current audit inventory and relevant bodies were screened. This is a bounded novelty claim, not an exhaustive claim about every historical discussion.

- **Deliberation terminal budget and its continuation** concern role-local closure notes, encoded context capacity and a copied observation slice. Here both roles finish valid JSON, all payloads are comfortably within bounds, and the gap occurs after the challenger returns. Fixing those helpers alone does not admit a consumer for a final-round conductor probe. [Budget audit][prior-budget] [Budget continuation][prior-continuation]
- **Deliberation progress observation identity** concerns different executed reads collapsing to the same stagnation fingerprint. Here the observation is new, the fingerprint changes and no-progress is not the stopping reason. Even a perfect progress identity still cannot admit round `Cap + 1`. [Progress audit][prior-progress]
- **Critical item retirement and citation admission** concern what justifies a ready verdict or removal of an unresolved requirement. This witness never reaches a post-probe verdict. Any future terminal reviewer must satisfy those separate evidence standards. [Retirement audit][prior-retirement] [Citation audit][prior-citation]
- **Read invocation identity** concerns preserving a role read's requested subject in its next model payload. This witness uses the conductor's challenge-bound call and never reaches the next payload at all. [Invocation audit][prior-invocation]
- **The portable crucible proposal** establishes the broader bounded design and acceptance direction. This report isolates one presently executable scheduling mismatch and gives a minimal resource-preserving correction. It does not present the entire design funnel or reviewed behavioral contracts as new. [Crucible][prior-crucible]

## 5 Proposed contracts and ownership

All states, relations and interfaces in this section are design proposals, not existing APIs.

### Consumer admission

Before dispatching a conductor probe, require one of:

- A policy-admitted review opportunity within the current resource envelope.
- An explicitly owned, authorized continuation that accepts an unreviewed result.
- Otherwise, a deferred probe with the reason `no_review_capacity`; no tool dispatch.

The first implementation should support the conservative defer branch without inventing a continuation or increasing a limit. Do not label a deferred probe as executed, failed, empty or negative evidence.

Go measures the current phase, available resource envelope and observation lifecycle. Mangle derives execute, defer, review or stop. JIT roles receive these dispositions as data and may propose investigation, but cannot expand their own budget or turn a missing consumer into permission to continue.

A possible policy vocabulary would distinguish `probe_requested`, `probe_admitted`, `probe_observed`, `probe_reviewed` and `probe_deferred`. Exact names are a specification decision. A bare observation count is insufficient: the record must bind the thread, candidate/version, blocker, exact effective read, result status and source identity to the review that actually received it.

### Bounded review

There are two reasonable variants to evaluate:

1. **Defer at the boundary.** Preserve current round semantics and refuse a conductor probe when no successor can review it. Return unresolved items and a concrete continuation requirement. This saves that dispatch but does not improve design completion by itself.
2. **Reserve review within the envelope.** Allocate a closing review phase before discretionary investigation begins. Close new reads, consultations and candidate expansion for that phase. It may accept, reject or preserve uncertainty based on admitted evidence, but cannot request another paid cycle. Its calls, context and persistence must be included in the configured envelope, not added after exhaustion.

Do not silently redefine a round to hide extra participant calls. If a new review phase changes the meaning of existing settings, specify and migrate that meaning explicitly. Preserve a strict-cap option and compare total work, not only the number printed as rounds.

For semantic rigor, a terminal review must not introduce a materially new candidate and certify it without its required challenge. If only the challenger is invoked, bind it to the already proposed candidate and existing success condition. If revision is necessary and cannot fit, return unresolved.

### Durable result and stopping

A result completed after a cancellation or budget transition can legitimately remain unreviewed. Retain a typed disposition and reason; do not grant an exception to cancellation merely because an observation was collected.

At a normal stop, provide a bounded summary identifying unresolved items, completed but unreviewed observations, deferred probes and the exact next authorized action needed. The complete source-bound record remains recoverable. Do not expose arbitrary tool output or private data just to make the summary detailed.

An explicit later continuation must revalidate source freshness, candidate identity, owner, permissions and its own finite budget. Reuse valid evidence where possible, but do not treat a restart, replay or new thread ID as an automatic budget reset or approval.

## 6 Failure modes the correction must not introduce

- **Hidden extra work:** a closing review runs beyond the owner's round, time or spend limit. Reserve it beforehand or defer.
- **Synthetic completion:** successful read status is treated as a passing requirements test. Require the same candidate-bound assessment and proof admission as ordinary readiness.
- **Budget laundering through resume:** reopening an exhausted thread silently creates a fresh allowance. Require an explicit owner envelope and retain prior cost.
- **Stale review:** a saved observation is reviewed after its source or intended design changes. Revalidate applicability before use.
- **Truncation loss:** adding lifecycle metadata crowds out requirements or results. Measure full encoded payloads and visibly defer or retain recoverable detail.
- **Overconservative behavior:** no probe can run near a boundary even when a valid low-cost consumer is available. Compare reserve and strict-defer variants under equal total resources.
- **Infinite terminal refinement:** the reviewer asks for more reads or creates new candidates. Make the terminal phase's capabilities and exits explicit.
- **False lifecycle reporting:** passing a result in a frame is reported as reviewed although the invocation failed or no assessment returned. Distinguish delivered, assessed and admitted.

## 7 Full costs and evaluation design

No savings or reliability improvement has been measured.

The baseline already pays for proposer and challenger invocations, their internal read loops, optional specialist consultations and nested inference, JIT compilation, registry calls, kernel assertions/queries/proofs, JSON encoding and atomic complete-thread snapshots. Final-round conductor reads can add tool latency, output processing, persistence and receipt work without enabling the current run's next assessment. Some outputs can still have later diagnostic value; do not count every retained unreviewed byte as worthless.

With round cap `R` and conductor call allowance `K`, source control flow permits up to `K` selected critical-data probe dispatches in an empirical final round, subject to errors, cancellation and item count. That is a control-flow bound, not an observed workload or a complete financial bound. Consultations and role-internal reads have separate costs. The configured values do not establish a universal dollar, token or wall-time ceiling.

The conservative variant avoids some dispatches but adds admission facts, lifecycle bookkeeping, output metadata and possible later continuation cost. The reserved-review variant allocates model work and context that otherwise could have been used for exploration. It can reduce investigation breadth or cause earlier deferral. Reusing saved results adds storage, lookup, provenance, freshness and migration costs. A full rerun may cost more than simply keeping the baseline. These trade-offs must be measured.

### Deterministic checks proposed but not run

| Case | Required observation |
|---|---|
| Cap one, empirical candidate, short successful read | Baseline reproduces the ordering; corrected strict-defer variant does not dispatch without a consumer |
| Empirical round before the cap | Result reaches the ordinary next-round assessment; valid existing behavior remains |
| Final round already ready | No unnecessary review or probe is introduced |
| Default cap, novel final observation | Consumer disposition is correct even when the fingerprint changes |
| Probe failure, empty result and partial result | Status is retained; none becomes proof of success |
| Cancellation before and after dispatch | Stop is honored; a completed result is labeled unreviewed when applicable |
| Resume with unchanged exhausted envelope | No automatic paid work occurs |
| Explicit new envelope with changed source | Stale evidence is refreshed or rejected rather than silently reused |
| Review failure or malformed output | No reviewed/admitted marker is manufactured |
| Values fork and design-only command | Existing question and permission rules remain intact |

The existing empirical integration fixture asserts a two-round read-and-continue path. The configured-bounds fixture uses a design-stalled case, not an empirical final round. Their composition is the missing targeted check identified here. Role tests cover evidence closure and byte bounds; the boot test covers policy reachability. These are authored tests inspected in source, not executed receipts. [Empirical and stop tests][tests] [Role tests][role-tests] [Boot test][boot-test]

After focused checks, exercise the normal chat and recurse-mission entry paths with real policy loading, registered JIT roles, ordinary world-model/CodeDOM context and their existing guarded tools. A scripted provider isolates ordering but cannot establish live convergence. Keep that distinction in the result.

### Fair comparative evaluation

Compare current behavior, strict deferral and pre-reserved review with identical repository snapshots, requirements, permissions, providers, prompts, tool catalogs and **total** resource ceilings. Include both readily resolvable and genuinely unresolved data questions, unfamiliar repositories, long evidence, source drift, cancellation and tool failure. Hold out tasks and fixture variations rather than tuning on the single cap-one witness.

Report:

- Every dispatched probe's consumer disposition, and the fraction left unreviewed at stop.
- Correctly admitted designs, justified rejections and accurate unresolved results.
- False-ready rate and preservation of the original requirement set.
- Number of unnecessary repeats, successful evidence reuse and manual continuation burden.
- All model calls and input/output tokens, tool executions, JIT work, proof/query work, serialization and snapshot bytes, wall time, provider cost estimates and follow-up cost.
- Completion-quality/cost curves, including failures, deferrals and cancellations in the denominator.

A lower call count is not success if the design quality or requirement coverage worsens. A higher completion rate bought with extra hidden review calls is not an equal-budget improvement.

## 8 Primary research and limits of transfer

Hay, Russell, Tolpin and Shimony's **Selecting Computations: Theory and Applications** models computation selection in terms of its effect on the eventual decision and its cost. That motivates asking whether an admitted observation has a remaining decision consumer. Its analysis is about specified statistical selection problems, not a proof of this conductor's optimal policy; importing a bandit scheduler or estimated utility model is unnecessary for the concrete boundary check proposed here. [Primary paper][hay]

Yao and colleagues' **ReAct** studies interleaving language-model reasoning and external actions so observations can inform later reasoning. The relevant design lesson is the observation-to-reassessment link. Its published task results do not predict codeNERD completion rates or establish how to allocate this system's round budget. No benchmark gain is transferred into this report. [Primary paper][react]

The direct source establishes the scheduling order. The literature motivates why collection and consumption should be accounted for together. Neither substitutes for the unrun implementation and production-route evaluations.

## Acceptance boundary

Close this finding only when the owning specification names the probe-consumer contract, the host and kernel enforce it through the actual service path, each boundary observation has a truthful disposition, and equal-budget evaluation confirms preserved safety and requirement quality. Retain the finite limits and unresolved outcomes. Do not close it merely because probes are saved, a progress hash changes, a role receives a larger context window or the round limit is raised.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/d1b6a95ca5863b247c10fa578d45b8db5e65c556
[comparison]: https://github.com/theRebelliousNerd/codenerd/compare/86ec71ddeab546141b2def8610418aa6a662ee5f...d1b6a95ca5863b247c10fa578d45b8db5e65c556
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/agents.md#L1-L68
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/architecture/AGENTS.md
[spec]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L898
[round]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/deliberation/round.go#L10-L72
[service]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/deliberation/service.go#L100-L166
[replay]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/deliberation/service.go#L79-L120
[probes]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/deliberation/outcome.go#L138-L201
[note]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/deliberation/outcome.go#L89-L115
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/core/defaults/policy/deliberation.mg#L18-L60
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/deliberation/persistence.go#L23-L173
[command]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/cmd/nerd/chat/deliberation.go#L97-L120
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/campaign/recurse_mission.go#L240-L330
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L113-L128
[config]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/config/routing.go#L18-L97
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/prompt/atoms/system/deliberation_proposer.yaml
[challenger]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/prompt/atoms/system/requirements_interrogator.yaml
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/cmd/nerd/chat/deliberation_test.go#L230-L267
[role-tests]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/deliberation/agent_test.go
[boot-test]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/internal/system/deliberation_boot_test.go
[prior-budget]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/audits/deliberation-terminal-budget.md
[prior-continuation]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/audits/deliberation-terminal-budget-continuation.md
[prior-progress]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/audits/deliberation-progress-observation-identity.md
[prior-retirement]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/audits/deliberation-critical-item-retirement.md
[prior-citation]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/audits/deliberation-citation-admission.md
[prior-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/audits/deliberation-read-invocation-identity.md
[prior-crucible]: https://github.com/theRebelliousNerd/codenerd/blob/d1b6a95ca5863b247c10fa578d45b8db5e65c556/Docs/audits/portable-recurse-deliberation-crucible.md
[hay]: https://people.eecs.berkeley.edu/~russell/papers/uai12-meta.pdf
[react]: https://arxiv.org/abs/2210.03629


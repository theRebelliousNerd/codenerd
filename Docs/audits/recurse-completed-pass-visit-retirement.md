---
doc-class: deep-dive
subsystem: campaign-recurse-pass-lifecycle
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 73a592eb1aad65c571ecfb1f9547e8b85926d943
verification-method: static-source-inspection
supersedes: []
---

# Completed recurse passes must retire their live visit facts

## Finding and decision

An unchanged workspace can consume the recurse kernel's fact capacity merely by completing more passes. Each successful visit asserts `recurse_node_visited(Pass, ID)`. The next pass changes `Pass`, so ordinary fact deduplication does not replace the previous row. The implementation explicitly retains every pass's visits, although the scheduling rules consume only visits joined to `recurse_current_pass`. With a fixed nonempty graph of N visited nodes, P successful passes leave P × N distinct real-pass visit facts in the same kernel. [Retention][retention] [Driver][passes] [Consumers][rules]

This is a **live-state lifetime and eventual admission-failure problem**, not a claim of unlimited memory allocation. The kernel has a finite EDB ceiling and rejects assertions at that ceiling. A sufficiently long run with no retirement therefore cannot continue admitting required pass facts. The error propagates out of recurse. Graph rebuilding, probe visits or other bookkeeping can be the first rejected assertion; the failure need not occur at the last real visit of a pass. No particular time-to-failure, memory footprint or latency increase has been measured. [Admission][admission] [Assertion error][assert-error] [Run return][run-return]

The proposed correction is to give pass scheduling facts a completion-aware lifetime. Preserve the current unfinished pass's valid progress, and preserve the existing recovery and audit records under their own retention contract. Retire completed generations from the live scheduling relation once completion has been recorded and no active scheduling consumer can need them. Do not weaken the fact ceiling, truncate attempt evidence indiscriminately, or replace Mangle's scheduling decisions with a host-side traversal.

**Evidence boundary.** Public main was resolved to [`73a592eb1aad65c571ecfb1f9547e8b85926d943`][pin] on November 4, 2026, and rechecked unchanged at 23:51 UTC. Root and campaign guidance, architecture guidance and campaign documents, the current audit inventory and relevant reports, implementation, and test definitions were inspected. This is static source analysis. No tests, builds, gate commands, application runs, model inference, training, failure injections or benchmarks were executed. The witnesses and evaluations below are proposed and unrun; no implementation repair is claimed.

## Why the lifetime matters to the architecture

The project assigns ordering and completion to the logic executive. Its recurse journey calls for ongoing passes and describes bounded kernel memory, retirement of cycle inputs, and release of attempt campaigns. Those are compatible requirements only if the live executive state distinguishes information needed now from records retained for inspection. The architecture documentation explicitly separates intended capability from implemented and behaviorally verified capability. [Root guidance][root] [Architecture guidance][architecture] [Recurse journey][journey]

The current implementation already makes that distinction in neighboring lifetimes:

- **Graph generation:** `subsystem_node`, `subsystem_depends` and `subsystem_node_ord` are replaced each pass. Historical visit rows are deliberately excluded from that replacement set. [Graph replacement][graph]
- **Current pass and invocation budget:** the current-pass row and the budget/finished-count rows are replaced rather than accumulated. [Current pass][pass-facts]
- **Soundness probe:** pass `-1` is reserved for a graph-order probe, and a successful probe retracts its own visit rows before real traversal. [Probe][probe]
- **Attempt cycle:** `endCycle` retires ratchet inputs after judgment. Its predicate set does not include `recurse_node_visited`. [Cycle retirement][cycles]
- **Recovery ledger:** `visit_done` and `pass_end` are journal records. Recovery reconstructs the unfinished pass's completed nodes, or starts the next pass after a recorded end. It does not need to rehydrate every completed pass's visit facts. This describes the existing replay algorithm, not a claim that every historical journal byte is retained forever. [Ledger][ledger] [Resume][resume]

**Implemented:** generation-qualified visits prevent an old pass's completion from directly completing a new pass; the graph probe is isolated; mutation admission errors are reported; cycle inputs have retirement.

**Partial:** the long-horizon state-lifetime design covers cycle inputs and some replaced relations, but not completed real-pass visits.

**Proposed:** a retirement boundary and a pass-count-independent bound on this live relation, while keeping current-pass scheduling and recovery correct.

**Unverified:** occurrence in a real unattended run, practical time or bytes to exhaustion, performance impact, and the correctness and cost of any repair.

## The causal path

### One kernel outlives all passes

`RunRecurseCycles` installs `cfg.Kernel` in the run's policy object once. Its pass loop increments the pass number, calls `r.pass`, increments the finished-pass count, and continues until policy or a boundary stops it. A zero pass budget does not impose a pass-count stopping condition. Neither the loop nor the successful pass-close path replaces the kernel. [Run ownership][run-owner] [Pass loop][passes] [Stop rule][rules]

Each pass derives its graph, checks its soundness, optionally narrows the graph, asserts the real pass number and restores that unfinished pass's recovered visits. It then repeatedly queries completion and the next node. After `r.visit` succeeds, it asserts that node's visit. On successful completion, it appends `pass_end`; it does not retract the pass's visits. [Pass setup][pass-setup] [Visit and close][pass-close]

### Different generations are different EDB rows

`assertNodeVisited` places both the pass number and node identity in the fact. The core admission path canonicalizes a fact and checks its exact identity before inserting it into the fact slice and index. Reusing the same node string does not make `(0, "a")` equal to `(1, "a")`. String interning shares repeated string storage; it does not remove distinct tuples. [Visit assertion][visit-fact] [Fact representation][fact-storage]

The source states that the run keeps every pass's visits. This is not a claim inferred merely from a missing call near one loop. The full predicate's producers, consumers and cleanup sites were inspected: the planning helper clears visits in its separate planning kernel; the real-run probe retracts only pass `-1`; the cycle retirement list is different. No completed-real-pass retirement was found in the inspected production path. [Retention][retention] [Planning kernel][planner] [Probe][probe] [Cycle retirement][cycles]

### Historical visits do not serve current scheduling

`recurse_visited_id` joins visits to `recurse_current_pass`. Readiness excludes only those projected IDs. Completion likewise counts `recurse_node_visited_now(Pass, ID)`, which joins the current pass and current graph. Changing the current pass correctly makes old visits inapplicable to these decisions, but it leaves their underlying EDB rows allocated and counted against admission. Semantic inapplicability is not physical retirement. [Scheduling rules][rules]

### The capacity limit is effective, and the run sees the error

The kernel resolves a per-kernel limit, otherwise a configured process-wide limit, otherwise its default. The default at this revision is **2,000,000 EDB facts**, described in source as an out-of-memory backstop rather than a working budget. Do not substitute the older 250,000 value or assume the default is the actual configured limit. [Limits][limits]

At capacity, `addFactIfNewLockedErr` returns an `EDB fact limit reached` error. `Assert` returns that admission error; `assertNodeVisited` wraps it; a failed visit assertion returns from the pass, then the pass loop, then `RunRecurseCycles`. This is a visible failed run rather than a silently omitted visit. Batch graph/budget assertions can also fail earlier; the exact first failure depends on remaining headroom and other facts. [Cap][admission] [Assert][assert-error] [Visit assertion][visit-fact] [Propagation][pass-close] [Run return][run-return]

## A fixed-workspace witness with no model attempts

The following is an unrun production-driver fixture, not a claim that a live user run was observed:

1. Start with a fresh kernel carrying the normal recurse policy, an empty writable recurse journal, sufficient initial fact headroom and a clean valid Git workspace. Include two ordinary JavaScript source files, `a/a.js` and `b/b.js`, tracked and admitted to membership, with no relative imports. Exclude package/toolchain markers and declared gates. Ignore the recurse metadata directory in Git. Use the ordinary driver with `Passes: 0`, no mission, no external gate-measurement callback and no concurrent source edits or external cleanup.
2. Source discovery creates one JS/TS node per source directory without requiring `package.json`. The workspace graph adds the wiring, review and benchmark nodes, making five visited nodes. Gate detection reads markers and declarations; this fixture has none. [Discovery][discovery] [JS/TS nodes][js-nodes] [Cross-cutting nodes][crosscutting] [Detection][detect]
3. Supply the required executor callback with an assertion that it must never be invoked. With no gate findings, no fix is selected. Cross-cutting nodes return from improvement immediately. Ordinary nodes either have no angle metric or skip improvement because no runnable test gate can verify the change. Visits still append `visit_done` and are asserted as completed. [Visit][visit] [Improvement skip][skip]
4. Let ordinary policy-controlled passes proceed on the same unchanged graph. After pass 0, the real-visit relation has five rows. After pass 1 it has ten. After P successful passes it has 5P, assuming no initially present real visits. At each next-pass start, old rows cease to qualify as current visits but remain in the EDB.
5. Under a finite cap M, this cannot continue for arbitrarily many successful passes: even ignoring every other fact, 5P cannot exceed M. In practice the baseline, current graph, temporary full-graph probe and bookkeeping need headroom too. A required assertion eventually fails unless another stopping condition or failure ends the run earlier.

This fixture isolates **pass history** from failed-attempt history, model behavior, growing source graphs, source-revision staleness, prompt length and evidence capture. It requires neither a successful model patch nor an ever-failing gate. An adequate finite pass budget or an earlier cancellation can prevent the failure; that limits exposure, but does not provide the required lifetime rule for an authorized ongoing run.

The witness's exact no-executor property and cardinalities must still be checked by a future executable fixture. It should fail on any unexpected executor call or newly detected gate rather than silently changing the workload.

## What existing fixtures establish, and what they do not

`TestSweepPass_CompletesWhenEveryNodeWasVisitedAndSkipsTheRest` supplies visits for one pass, verifies completion, advances the pass and verifies that the new pass is incomplete. That is useful evidence of intended generation isolation, but its assertions do not bound retained historical visits. The graph-probe fixture inspects visit cleanup for the probe. These test definitions were read, not run. [Sweep fixtures][sweep-tests]

The existing long-run fixture `TestRecurseCycles_ALongRunLeavesBoundedState` cancels by attempted-cycle count. It checks attempt-history agreement, bounded kept-node and per-visit attempt facts, removal of ratchet inputs, cleared in-flight state and a journal byte cap. It does **not** count `recurse_node_visited`, require a bound independent of completed passes, or use a no-attempt/many-pass workload. Its name and the journey's cycle-soak description should not be interpreted as proof of this missing property. Nor does this audit invalidate the narrower properties that fixture actually checks. [Soak fixture][soak] [Journey validation][journey-tests]

## Proposed contract and repair boundary

Specify these requirements before selecting the implementation:

1. **Explicit owner and generation.** Pass scheduling state belongs to one recurse run's active scheduling generation. Identify which generation is active, probing, completed or being retired. Do not make a shared-kernel predicate-wide purge an implicit cross-run ownership mechanism.
2. **Keep active progress.** While a real pass is unfinished, retain the visits required to choose the next node and prove completion. Never clear visits before each node or replay the same pass as if nothing had completed. Imported visits must still satisfy the separate resumed-visit applicability contract.
3. **Completion before retirement.** First obtain the policy completion result and record the pass end under the required journal persistence contract. Then retire that completed generation at a serialized boundary before it can become historical overhead. The current append function reports append failures; this audit does not equate its success with a separately verified crash-durability guarantee. Preserve or explicitly reconcile an uncertain completion rather than erasing the only usable progress evidence. [Journal append][journal-append]
4. **Bound this relation independently of elapsed passes.** After successful retirement, no completed generation remains live solely for historical inspection. During transitions, allow only explicitly bounded active, probe and retiring sets. For a focused sweep, distinguish its active-node count from the full graph used by the probe. The bound must not contain total completed-pass count.
5. **Keep recovery and audit semantics separate.** Retiring live visit rows is not permission to delete journal records, attempt outcomes, stalls, refusals or retained code-change evidence. If historical visit queries are a desired feature, provide them through an explicit bounded history/read path instead of an indefinitely growing executive relation. Resume must retain its incomplete-pass behavior and next-pass behavior after a recorded end. [Resume][resume]
6. **Fail visibly on incomplete cleanup.** A retirement error must not be reported as successful bounded-state maintenance. Define retry/reconciliation for a recorded completion whose retirement failed. Keep the fact cap as an admission safety backstop, and leave room for graph probes and the control facts needed to settle or stop a run.
7. **Preserve the executive boundary.** The host supplies lifecycle observations and performs owned fact mutations. Mangle still determines readiness, next-node selection and pass completion. A Go topological walk is not an equivalent fix.

Possible implementations include retiring exactly the completed generation in a batch, or replacing a run-owned scheduling snapshot at the transition. Neither is implemented or endorsed as proven here. Reusing the existing probe helper would need correct error naming, lifecycle ownership and failure handling; its current name and call sites do not establish a completed-pass protocol. Broadly resetting the entire kernel can discard unrelated state. Raising the cap merely changes the finite headroom. Restarting periodically can mask accumulation while introducing replay and startup costs, and is not the same contract.

## Research basis and costs

Naiad's timely-dataflow model separates an iteration's identity from knowledge that no more input for that logical time can arrive. Its example `DistinctCount` vertex removes the timestamp's count state on a completion notification. The useful connection is **completion-aware reclamation**, not a proposal to replace Mangle with Naiad or to import a distributed protocol into a serial driver. A pass boundary is a simpler local place to establish the corresponding lifecycle condition. This analogy motivates the proposed contract; CodeNERD's defect follows from its own source. [Murray et al., SOSP 2013, §2.2 and Figure 4][naiad]

The official Go GC guide distinguishes live memory determined by the program from reclaimable garbage. Keeping rows reachable through kernel state cannot be repaired by GC tuning alone. Its GC cost model is not evidence of a measured recurse slowdown, and does not justify inferring total CPU growth from retained-row count alone. [Go GC guide][go-gc]

Account for the whole system rather than only a smaller relation:

- **Current retained state:** P × N distinct real-visit rows for the fixed-graph witness, plus baseline and transient rows. Facts are also represented in indexing/atom structures. Interned node strings can reduce duplication, but tuple count and pass values remain. Exact bytes depend on representations and allocation behavior and were not measured. [Storage][fact-storage]
- **Current cleanup work:** graph, current-pass, visit and metric replacement already invoke retraction APIs. The core compactor walks the fact slice, so retained history can enlarge the input to those scans even though it cannot affect current scheduling. Query-cone and index behavior can reduce other work; no uniform query complexity or wall-clock curve is asserted. [Compaction][compact] [Visit replacement][visit-replace]
- **Retirement cost:** an exact batch builds a key set and filters the live facts under the kernel lock, then rebuilds when rows were removed. It is not automatically an O(N) operation independent of total EDB size F. Budget selection/compaction work, index maintenance, cache invalidation and later evaluation, and lock hold time. Prefer one owned batch at an established boundary over N separate full scans. [Exact batch][retract]
- **Durable records:** the journal still incurs visit/pass I/O and has its own rotation/recovery limits. Removing in-memory visit history does not reduce those writes, solve journal replay coverage, or establish an unlimited audit archive.
- **Gate and model costs:** the isolating fixture should have no gate subprocesses or model attempts. Real workloads still pay discovery, measurements, model attempts, ratchets and recovery costs. Retirement does not prove fewer tokens, fewer attempts or better patches.
- **Other lifetime domains:** attempt history is deliberately used by stall rules and is outside this claim. Fixing visit retention alone does not establish a global O(N) bound for all recurse state. [Attempt history][attempt-history]

## Qualification plan, all unrun

Compare the pinned implementation with a completion-aware retirement candidate under the same policy, source graph, gate declarations and fact limit. Include a cap-increase-only variant as a diagnostic comparator: it should delay a capacity failure without changing historical-row growth. Do not call that variant a successful lifetime repair.

1. **Direct cardinality control.** In a real kernel, complete many fixed-graph passes with ordinary current-pass facts. At each completed boundary record real/probe visits separately, total EDB count, next-node result and completion result. The pinned version should accumulate historical rows; the candidate must keep the documented transient bound while producing the same per-pass node order.
2. **Production driver, no attempts.** Run the fixed five-node fixture through `RunRecurseCycles`. Fail on any executor call. Use a test-local finite cap sized after measuring baseline and probe headroom, not a production configuration change. Choose pass counts that cross the pinned implementation's predicted admission boundary. Record the first rejected predicate and wrapped error; do not require it always to be `recurse_node_visited`.
3. **Finite and ongoing modes.** Test one pass, multiple passes and cancellation of an ongoing run. A finite invocation must still report the requested number of completed passes. Cancellation mid-pass must preserve the documented restart behavior; completed-history cleanup must not repeat already completed work within an unchanged unfinished pass.
4. **Boundary failures and resume.** Inject failure before pass-end append, on append, after acknowledged completion but before retirement, during retirement, and after retirement but before the next pass. Restart with both a fresh kernel and a reused session kernel where supported. Verify explicit errors, no fabricated pass completion, correct reconstruction, and no deletion of unrelated state. Treat source-changed recovery separately under its existing applicability contract.
5. **Graph and scope changes.** Add/remove nodes between passes, exercise full and focused sweeps, and include a cycle rejected by the soundness probe. Current ordering and cycle rejection must be unchanged. Probe counts must be charged against the full graph and must not be mistaken for historical real visits.
6. **Actual-cost measurements.** For several graph sizes and baseline EDB sizes, report fact counts by lifetime, heap/allocations, retirement and replacement lock times, evaluation counts, pass latency and journal bytes. Compare equal completed work and identical source states, with warmup/repetitions and raw observations. A repair passes the semantic and cardinality contract first; any throughput claim requires these measurements.

Exit evidence should include executable regression coverage for the no-attempt path, a pass-count-independent bound on completed-history rows, recovery equivalence at each boundary, unchanged policy decisions, admission failures still surfaced, and measured overhead of the selected cleanup implementation. A short soak that simply remains below the default cap is insufficient.

## Overlap and limits

The current tree contains 391 Markdown audit entries. The inventory and queue were reviewed, relevant bodies and a substantial cached audit corpus were inspected, and repository search for the visit predicate located the implementation, scheduling fixtures and resumed-visit report. This is a bounded novelty screen, not a claim to have read every historical revision or private discussion.

- [Recurse resumed-visit applicability][prior-resume] concerns old completion evidence skipping changed source after recovery. This witness stays in one run with a fixed graph and unchanged source. Old passes correctly do not complete the current one; they consume capacity instead.
- [Recurse journal replay coverage][prior-journal] concerns unavailable or incomplete replay evidence. This witness does not require a restart, malformed record, uncertain commit or missing history. Better replay does not retire live completed-pass rows.
- [Mangle kernel performance][prior-kernel] covers fact indexes, clone/query work, retraction scans and other kernel performance boundaries. Those mechanisms affect cost, but do not supply a semantic lifetime for completed recurse passes. This report does not claim their general performance observations as new.
- [Recurse prior-reason budget][prior-reasons] concerns context rendering and omission, not this executive relation. The no-attempt witness has no growing model prompt or prior-attempt list.

The conclusion is intentionally narrow: **completed-pass visits remain live even after current scheduling stops consuming them, and their distinct generations can exhaust the kernel's finite fact budget.** It does not show an incorrect keep verdict, prove a production outage, or establish that every other kind of recurse history should be deleted. The missing contract is ownership and retirement of completed scheduling generations.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/73a592eb1aad65c571ecfb1f9547e8b85926d943
[root]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/agents.md#L3-L23
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/Docs/architecture/AGENTS.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/Docs/journeys/10-forever-loop.md#L181-L200
[journey-tests]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/Docs/journeys/10-forever-loop.md#L204-L215
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_plan.go#L69-L80
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_plan.go#L136-L150
[pass-facts]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_plan.go#L153-L186
[visit-fact]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_plan.go#L163-L167
[planner]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_plan.go#L38-L55
[probe]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_plan.go#L256-L349
[run-owner]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L353-L390
[run-return]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L422-L450
[passes]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L489-L512
[pass-setup]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L515-L550
[pass-close]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L565-L614
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L1052-L1106
[skip]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L1181-L1205
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_cycle.go#L1840-L1883
[rules]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/core/defaults/policy/recurse.mg#L417-L485
[cycles]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_policy.go#L283-L337
[visit-replace]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_policy.go#L54-L89
[attempt-history]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_policy.go#L13-L23
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_journal.go#L11-L40
[journal-append]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_journal.go#L66-L99
[limits]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/core/kernel_init.go#L202-L268
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/core/kernel_facts.go#L491-L498
[assert-error]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/core/kernel_facts.go#L566-L597
[fact-storage]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/core/kernel_facts.go#L500-L539
[compact]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/core/kernel_facts.go#L839-L877
[retract]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/core/kernel_facts.go#L999-L1069
[discovery]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_workspace.go#L64-L100
[js-nodes]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_workspace.go#L550-L558
[crosscutting]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_workspace.go#L237-L257
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/gates/detect.go#L120-L153
[sweep-tests]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_sweep_test.go#L81-L225
[soak]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/internal/campaign/recurse_soak_test.go#L60-L178
[prior-resume]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/Docs/audits/recurse-resumed-visit-applicability.md
[prior-journal]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/Docs/audits/recurse-journal-replay-coverage.md
[prior-kernel]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/Docs/audits/mangle-kernel-performance.md
[prior-reasons]: https://github.com/theRebelliousNerd/codenerd/blob/73a592eb1aad65c571ecfb1f9547e8b85926d943/Docs/audits/recurse-prior-reason-budget.md
[naiad]: https://sigops.org/s/conferences/sosp/2013/papers/p439-murray.pdf
[go-gc]: https://go.dev/doc/gc-guide

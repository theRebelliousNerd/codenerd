# Recurse recovery: a local cycle number is not a commit identity

## Finding and decision

**Ordinary recurse recovery joins a workspace-local in-flight attempt to repository-wide Git history using only its integer cycle number.** Two legitimate, sequential recurse runs in different subdirectories of one repository can both have cycle 1. A real commit from the first workspace can then be recorded as the second workspace's recovered kept attempt, even if the second executor never started.

This is a recovery-attribution defect, not a claim that the first commit was bad or that the second finding necessarily disappears. The recovered record reaches the workspace ledger and policy's kept-node observation. Later visits still remeasure findings and may perform the missing work.

**Recommendation:** preserve the cycle as a local display sequence, but bind recovery to a durable, unique attempt identity and its admitted repository/workspace/base and adoption evidence. Ambiguous legacy recovery must remain explicitly unresolved rather than manufacture a kept receipt.

Source pin: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), reconfirmed as main on 2026-10-30. This is a conditional static-source finding. No application, Git command, test, model inference or training was executed; the example below is a source-derived schedule, not a reproduced incident.

## Prior work and the narrower residual

The forever-loop journey already promises restart reconciliation and commits that name their finding and cycle. The positive recovery test deliberately covers a commit that landed before its local journal acknowledgement. That behavior is valuable and must survive a repair. [Journey durability contract][journey], [existing missed-acknowledgement test][positive-test].

The mission-improvement proposal already calls for immutable attempt IDs, base revisions, effect receipts, verification identity and durable commit identity. This document does not claim that broad design as new. It identifies a concrete remaining ordinary-recurse consumer that still equates a local sequence number with commit identity. Current main also adds an important protection: an unsettled mission-marked attempt is refused by ordinary recovery. **The witness excludes mission mode.** [Prior recovery proposal][prior-proposal], [current mission refusal][settlement].

The recent index-adoption audit concerns which bytes a new commit includes. Here the earlier workspace's commit can be fully legitimate, with no foreign staged bytes or external writer race. The campaign/recurse scheduler audit concerns simultaneous owners and shared kernel relations; this witness is sequential and needs neither. The campaign-resume return-evidence audit concerns missing completed-task reports, not the identity of a recovered commit. [Index-adoption audit][prior-index], [scheduler-ownership audit][prior-scheduling], [return-evidence audit][prior-return].

## Production route and the identity mismatch

### 1. Workspace state is local; the Git history is shared

The CLI chooses the explicit workspace or current directory and passes that directory to `RunRecurseCycles`. Canonicalization resolves filesystem identity, not the containing Git repository root. The recurse journal and lock live beneath that chosen workspace. A source test expressly supports a workspace below the Git root and checks that change collection and reversal stay within its tree. [CLI workspace and dispatch][cli-workspace], [CLI invocation][cli-run], [canonicalization][canonical-root], [journal path][journal], [workspace lock][lock], [subdirectory test][subdir-test].

Consequently, two sibling workspace directories can have separate journals, locks and cycle counters while Git commands executed in either directory see the same repository history. On resume, the local counter advances to the maximum cycle in that workspace's own journal; it is not allocated from a repository-wide namespace. [Resume][resume].

### 2. The durable intent carries a cycle, but no unique attempt ID

An ordinary attempt increments `r.cycle`, writes an in-flight record, and appends its attempt journal row before invoking `cfg.Execute`. The in-flight record contains mission marker, cycle, pass, node, finding, angle and prior untracked paths. It contains no unique attempt ID, admitted base revision or workspace/repository identity against which a recovered commit is checked. [In-flight schema][flight], [attempt publication and dispatch][attempt].

A hard interruption after successful publication but before `Execute` is therefore a concrete crash boundary. It is not the normal Ctrl+C path: CLI SIGINT/SIGTERM handling cancels the shared context, and ordinary cancellation has its own cleanup behavior. [CLI cancellation][cli-cancel].

### 3. The commit producer names more than the recovery reader checks

The Git helper writes both `Recurse-Cycle` and `Recurse-Finding` trailers. Recovery's `keptCycles` asks Git only for commit hash and `Recurse-Cycle`, parses the cycle into an integer, and returns `map[int]string`. It does not compare the finding trailer, workspace, admitted base, candidate tree or attempt identity. [Commit producer][commit], [recovery lookup][lookup].

The lookup is equivalent to `git log -50` with the custom format: no revision range, workspace path filter or attempt-specific ref is supplied. Git documents that an omitted revision range starts at `HEAD` and includes its reachable history, and that path restriction is an explicit additional argument. Merely running Git from a subdirectory does not add such an argument to this call. This supports the history scope, not a claim about any particular installed Git version. [Git log documentation](https://git-scm.com/docs/git-log).

### 4. Recovery promotes the match before normal branch preparation

Startup reads the local journal and calls `settleInFlight` **before** `git.prepare` checks the checkout and switches to the configured branch. Settlement first honors a matching local ratchet row and refuses an unsettled mission marker. Otherwise, it scans the latest 50 current-HEAD commits and looks up `kept[f.Cycle]`. On a match, it copies the local in-flight node/finding into a ratchet record, attaches the matched hash, labels it kept, appends it and removes the in-flight file. [Startup ordering][startup], [settlement][settlement].

The strongest witness keeps both runs on the same default `nerd/recurse` branch, with the foreign commit at `HEAD`. It does not rely on a branch-switch race. The before-prepare ordering nevertheless matters when specifying what history recovery actually consulted.

## A bounded sequential witness

Use an otherwise ordinary Git repository with two distinct, canonical subworkspaces, called **A** and **B** here. Each has the project inputs, usable kernel/executor and actual measured finding needed to admit an ordinary recurse attempt. Their runtime `.nerd` paths are ignored or otherwise do not create tracked dirt. These names are synthetic, not project-specific information.

1. **B genuinely completes its first ordinary recurse attempt.** Its real ratchet permits keeping the change, Git commits it as **K** with cycle 1, and B completes its journal/cleanup normally. Assume no other matching cycle-1 commit within the lookup window, to avoid irrelevant duplicate-selection behavior.
2. **B has stopped and the checkout is clean.** Leave K at `HEAD` on `nerd/recurse`. There is no concurrent recurse loop, external edit or staged foreign version.
3. **Fresh A starts ordinary recurse.** Its own journal has no previous attempts, so its first attempt is also cycle 1. A successfully writes its in-flight file and attempt row. A is then hard-interrupted before its executor is called. A has made no task effect and has no completed ratchet row.
4. **A restarts on the same checkout.** Its in-flight record has no mission marker. The current-HEAD log contains K within the first 50 commits. The local ratchet short-circuit does not apply. Git lookup and journal operations succeed.
5. **The integer join matches K.** Settlement writes a kept row for A's node/finding with B's commit hash, then removes A's in-flight file. B's commit did land; it was simply not A's attempt.

Every step follows the source route above. No model misclassification, fabricated commit trailer, simultaneous run or bypass of the normal keep decision for B is required. The defect occurs in reconstructing A's outcome, not in B's original admission.

## Actual consumers and limits on the consequence

**Durable status is affected.** `ReadRecurseStatus` reads the local journal, counts each kept ratchet, and renders the associated node and commit hash. Both the CLI status command and the TUI recurse-status handler consume that result. Thus A can display a kept cycle attributed to K without having executed that attempt. The status's kept total is not the same as the current invocation's `RecurseCycleResult.Kept`: the recovery path does not increment that result counter. [Status aggregation and rendering][status], [CLI status][cli-status], [TUI status][tui-status].

**The record also becomes policy input.** Startup rereads the journal after settlement. `resume` sends a kept outcome through `policy.attempt`, which calls `nodeKept`, or directly calls `nodeKept` for a kept improvement without a finding. That asserts `recurse_node_kept` for A's node and local cycle. [Startup reread][startup], [resume][resume], [kept-node projection][kept-node].

The stall policy uses a newer kept-node cycle as evidence that earlier failures may be worth trying again. A similar collision at a later local cycle could therefore affect stall classification if the required older same-node failure facts exist. **That is not a consequence of the minimal fresh-A cycle-1 witness**, which has no such older failures. [Stall policy][stall].

**No automatic permanent skip is established.** Resume marks a visit done from a `visit_done` row, not merely from this recovered ratchet. Subsequent visits remeasure and can rediscover A's unresolved finding. This document does not claim all findings appear resolved, current-run kept statistics increase, an unsafe patch is committed, or data is lost in the no-effect witness. The supported outcome is a false recovery receipt, its status representation and its kept-node policy projection. [Resume control flow][resume], [visit measurement][visit-measurement].

## Protections that remain real

- **Mission ownership:** an unsettled `MissionState` refuses this ordinary recovery route. Do not weaken that guard or silently apply the proposal to mission reconciliation.
- **Existing local completion:** a same-cycle ratchet already present in A's journal causes only in-flight cleanup before Git lookup. The witness requires its absence.
- **Normal counters:** retained single-workspace history advances the cycle counter. The witness does not assume routine counter reset or journal corruption.
- **Admission:** usable services, canonical workspace, exclusive workspace lock, valid project inputs and subsequent Git preparation remain required. Dirty tracked files still cause the normal preparation refusal; the witness keeps the checkout clean.
- **Bounded lookup:** K must be reachable from the current `HEAD` and inside the 50-commit window. An unrelated commit on an unreachable branch does not satisfy this witness.
- **Crash boundary:** complete intent/journal publication and a hard interruption are assumed. Unreadable in-flight JSON or Git/journal errors return errors, rather than producing the described kept record.
- **Original verification:** B's actual commit still needed its real ratchet. A's later measurements remain independent checks; they cannot retroactively make the recovery attribution truthful.

These controls are visible in [startup][startup], [Git preparation][prepare], [settlement][settlement] and [existing killed-attempt tests][interrupted-test]. This is not a permission-bypass or hostile-repository-authentication claim.

## Proposed recovery contract

### Separate display sequence from durable identity

Before any effect, assign a unique attempt ID and durably bind it to a versioned ledger/workspace identity, repository/worktree identity, local cycle, node/finding or improvement angle, admitted base revision, and the evidence/scope required by the adopted recovery contract. Keep cycle numbers for readable progress. Prefer opaque identifiers in commit trailers; avoid publishing absolute local paths or sensitive task descriptions merely to create a namespace.

Carry that same identity through the commit intent, commit trailer or durable adoption reference, journal verdict and recovery lookup. A candidate commit must be attributable to this attempt, not just have the same integer or mention a similarly named finding. A UUID trailer alone is not proof of verification: it links to the admitted attempt and adoption evidence, which must still be checked.

### Reconcile an adoption state, not a guessed outcome

Distinguish at least prepared, execution/effects observed, adoption intended, commit identified, and verdict published. Persist enough evidence before each irreversible boundary to recover the exact commit after an acknowledgement loss. Validate repository/workspace identity and the admitted parent/candidate relationship before publishing kept. If the branch moved or multiple candidates disagree, retain uncertainty for an explicit reconciliation decision.

Do not reconstruct lost effect ownership from an arbitrary difference against HEAD. Preserve the existing owned-effect and mission protections, and do not rerun a possibly committed side effect merely because its acknowledgement is absent. Recovery should append one idempotent verdict for the unique attempt; a repeated restart should only finish missing publication/cleanup.

### Compatibility and smaller alternatives

1. **Cheap containment:** also compare finding, workspace namespace and an admitted base/range. Recording the base would reject the primary witness because B's K is already A's base, not a later adoption. This is useful but not a complete unique-attempt contract: repeated findings, missing legacy fields, shared paths and moved history need defined handling.
2. **Namespaced durable attempt receipts:** recommended. Recover by exact attempt and expected adoption evidence, optionally using a durable local ref or object reference rather than an arbitrary recent-history window. Define reference retention and branch-rewrite behavior explicitly.
3. **Repository-wide serialization or Git path filtering alone:** insufficient for the primary sequential witness. Serial runs still reuse local cycle numbers; a path match is not proof of a particular attempt, and overlap/shared files complicate path ownership.
4. **Disable all missed-ack recovery:** avoids this false positive but regresses a deliberately supported crash case. Prefer explicit unresolved recovery over false success while preserving genuine exact matches.

For existing in-flight records without the new identity, do not silently synthesize provenance or erase historical rows. Use a conservative legacy reconciliation path with explicit uncertainty and retained evidence. A migration may offer owner review or independently established base/finding/scope evidence, but it must not label a bare cycle match sufficient. Keep existing successful historical records readable; append a correction when a known attribution is revised rather than rewriting the ledger without trace.

## Costs and failure modes to budget

- **Metadata:** constant-sized IDs and base/adoption references per attempt, plus versioned journal and in-flight records. Effect receipts or candidate manifests add storage proportional to the effects retained; reuse existing evidence rather than duplicate full patches by default.
- **I/O and crash durability:** additional publication stages, synchronization and error paths can add latency. Specify atomic replacement and flush guarantees for intent separately from append-only verdict durability; do not call multiple files a transaction without a protocol.
- **Git lookup:** current recovery scans at most 50 commits. A naive unbounded attempt-ID search costs history traversal; direct known-object/ref lookup can bound normal recovery but adds ref lifecycle and missing-object handling. Measure cold-cache recovery, not only warm in-process lookup.
- **Migration:** old journals, rotated generations, copied workspaces, cloned repositories, changed roots and worktree relocation need explicit identity rules. Do not accidentally make a legitimate moved workspace permanently unrecoverable or conflate a copied ledger with the original live owner.
- **Operations:** unresolved states require understandable diagnostics and an owner reconciliation path. Excessive conservatism can block harmless restarts; permissive fallback recreates false attribution.
- **Retention:** recovery references and receipts need a justified lifetime. Premature pruning loses evidence; indefinite retention grows local state. Preserve minimal provenance after larger artifacts expire.
- **Security and privacy:** IDs should not expose local absolute paths or private findings. Git trailers remain user-editable metadata, not an authentication boundary against a malicious repository owner.
- **Compute and model cost:** this is deterministic identity/recovery work. It requires no additional model inference in the normal path. Measure any extra gate reruns or manual recovery workload separately; do not claim latency or cost savings without data.

## Fair evaluation plan — proposed, not run

Compare current behavior, a documented minimal base/namespace containment, and full attempt-bound recovery using the same deterministic executors, repository topology, gates, commits and crash points. Do not improve only the new implementation's fixture or loosen ratchet requirements.

| Case | Required observation |
| --- | --- |
| Single workspace, own commit landed before verdict append | Recover the exact commit once; preserve the existing positive test's behavior without redispatching completed work. |
| Sequential sibling workspaces, each cycle 1; B's K at HEAD; A crashes before Execute | A must not publish kept for K or assert A's kept-node fact. Preserve K and all user data. |
| Same integer, different finding; same integer and coincidentally equal finding | Neither alone establishes attempt identity; verify the actual ownership contract. |
| Existing local ratchet plus leftover in-flight record | Finish cleanup idempotently without duplicate status counts. |
| Mission-marked unsettled intent | Preserve the current refusal/reconciliation boundary. |
| Hard interruptions at every publication boundary | Distinguish no execution, owned effects, adoption uncertainty and a known committed result; no invented verdict or duplicate effect. |
| Foreign commit outside latest 50, unreachable branch, or moved HEAD | Report evidence availability and identity accurately; a bounded-search miss is not proof of no commit. |
| Multiple candidate commits or rewritten history | Avoid arbitrary selection; retain ambiguity or apply an explicit validated policy. |
| Graceful cancellation, dirty startup and second same-workspace admission | Preserve current cleanup and admission controls. |
| Genuine unresolved A finding after recovery | Remeasure normally; measure remediation separately from ledger truthfulness. |
| Legacy record, rotated journal, copied or relocated workspace | Exercise the migration/identity policy without silently treating copied sequence numbers as ownership. |
| Replay of a corrected recovery result | Status, durable record and policy observation agree; correction does not erase original evidence. |

Inspect the actual journal, in-flight lifecycle, CLI/TUI status, policy facts, current-run result counters, commit graph and workspace/index contents. Primary metrics are false kept attribution, exact recovery rate, duplicate effects/verdicts, unresolved-state rate and user-data preservation. Secondary metrics include cold restart latency, Git processes/history scanned, bytes written/retained, synchronization cost and operator effort. Keep verification failures distinct from missing recovery evidence. No result or benchmark for this proposal is claimed.

## Branch and evidence scope

The research inventory was reviewed through `research-main-20261030-1246` (`58c84997ba39254e22c590f6cb3bab43a58509c1`), including the index-adoption predecessor. Targeted source/doc searches found no earlier audit of this exact local-cycle-to-repository-history join; this is not a claim that all possible historical branches were exhaustively reviewed.

Five visible active non-research heads were also checked: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908` and `767a3291d6287b238417f2466592a9002810d05a`. Their `recurse_git.go` lookup blob matches main. The two distinct older `recurse_cycle.go` variants retained the same cycle-only settlement join. This comparison supplies no competing fix for the scoped ordinary-recovery issue; it does not substitute their older mission behavior for main's current guard. [Active variant one][branch-one], [active variant two][branch-two].

[journey]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/journeys/10-forever-loop.md#L161-L175
[positive-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle_test.go#L317-L351
[prior-proposal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/innovations/mission-driven-forever-improvement.md#L167-L173
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/58c84997ba39254e22c590f6cb3bab43a58509c1/Docs/audits/recurse-index-adoption-identity.md
[prior-scheduling]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/campaign-recurse-scheduling-ownership.md
[prior-return]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-resume-return-evidence.md
[cli-workspace]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L120-L125
[cli-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L245-L260
[canonical-root]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/workspace_guard.go#L58-L88
[journal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_journal.go#L34-L64
[lock]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_control.go#L12-L55
[subdir-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git_test.go#L154-L175
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1527-L1570
[visit-measurement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L818-L837
[flight]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L876-L888
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L967-L1028
[cli-cancel]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L151-L179
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L125-L147
[lookup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L225-L243
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L298-L395
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1470-L1524
[status]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_control.go#L91-L178
[cli-status]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L95-L104
[tui-status]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_recurse.go#L331-L337
[kept-node]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_policy.go#L103-L128
[stall]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/recurse.mg#L188-L229
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L50-L76
[interrupted-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle_test.go#L258-L315
[branch-one]: https://github.com/theRebelliousNerd/codenerd/blob/51cfd42649552866d4aa54c53b68acf5740eb09b/internal/campaign/recurse_cycle.go#L1201-L1249
[branch-two]: https://github.com/theRebelliousNerd/codenerd/blob/767a3291d6287b238417f2466592a9002810d05a/internal/campaign/recurse_cycle.go#L1306-L1354

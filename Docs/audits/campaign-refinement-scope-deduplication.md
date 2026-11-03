# Campaign refinement: similar task wording is not shared scope

## Decision

**Rolling-wave refinement can discard a genuinely new, explicitly targeted task because another task has the same description after its final path is removed.** The near-duplicate guard compares only this shortened text. It does not compare task type, declared artifacts, write set or the obligation each task serves before dropping the proposed addition.

The existing optimization is intentional and useful: a planner can restate the same work with a path appended, wasting turns or leaving a redundant task blocked. The proposed correction is therefore **scope-aware duplicate admission**, not removal of duplicate protection. Similar wording should identify candidates for comparison; it should not alone establish that one task covers another deliverable.

Source pin: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), reconfirmed as main on 2026-10-30. This is a conditional static-source finding, not an executed reproduction or observed incident. No application, test, Git command, model inference, training or implementation was run.

The supported consequence is narrower than an incorrect overall success verdict: the new task and its target do not enter the refined executable plan. A warning is logged; the campaign goal remains available; checkpoints, an independent worker or a configured final acceptance check can still detect or repair the omission.

## Intentional behavior and prior work

`replanDedupeKey` explicitly documents the duplicate-restatement problem it was designed to prevent. Its positive test expects a path-suffixed research restatement to be dropped without increasing the phase or campaign task count. A second test preserves a task whose **base wording differs**. Neither fixture distinguishes equal base wording with two separate explicit scopes. These tests were read, not run. [Intent and key][key], [restatement and distinct-base tests][tests].

The ordinary `normalizedTaskKey` deliberately preserves `/`, `.`, `_` and `-`, with a comment explaining that paths should remain distinguishable. The additional refiner key then removes a trailing single-token ` in <path>` suffix. The precision loss here is specifically that second transformation; it is not an unavoidable consequence of whitespace normalization. [Ordinary key][ordinary-key], [refiner key][key].

This extends existing conservation work at a different producer/consumer seam:

- The planned-work audit covers a session edit-plan cap and an omitted suffix, not campaign task equivalence. Its principle that omitted obligations need an explicit disposition remains relevant. [Planned-work conservation][prior-work].
- The consultation-literal audit covers case/whitespace folding of main-chat knowledge requests, not the campaign's path-stripping and target-bearing task records. This report does not claim the broad observation that approximate identity can lose work as new. [Consultation identity][prior-consultation].
- The immediately preceding refinement-dependency audit concerns a task that survives but loses the kind of a prerequisite edge. Here the proposed task is dropped before target/effect admission. Fixing dependency projection would not restore it. [Dependency-kind audit][prior-dependencies].

## Production route

### From TUI campaign to a published refined plan

The TUI builds an orchestrator, decomposes and admits the campaign, then runs it. The constructor installs a replanner. After a phase completes, the normal task loop triggers rolling-wave refinement for the next phase. Successful refinement is followed by campaign-fact reload and a revision event. [TUI admission][tui-admission], [TUI execution][tui-run], [constructor][constructor], [phase transition][transition], [rolling-wave consumer][rolling-wave].

`RefineNextPhase` finds the phase with the next order, clones the campaign, and shows the planning client completed results plus upcoming IDs and write sets. The prompt permits adding concrete tasks, preserving execution effects and declaring `write_set`; the decoder also accepts artifact targets. The witness uses a normal `action: add` response with a distinct ID and explicit valid targets. It does not rely on an unsupported action or the unmatched-update fallback. [Setup and context][setup], [prompt contract][prompt], [decoder][decoder].

### A text-only guard runs before target admission

The refiner builds two maps from tasks already in the next phase: one ordinary normalized-description map and one suffix-insensitive map. New tasks accepted earlier in the same response also populate these maps. [Map construction][maps].

In the add branch:

1. Empty descriptions and exact normalized duplicates are skipped.
2. A new task ID is canonicalized or allocated, with an existing-ID collision check.
3. `replanDedupeKey(description)` is looked up in the suffix-insensitive map.
4. On a match, a warning identifies the proposed ID and existing ID, then `continue` discards the addition.
5. Only a survivor becomes a typed `Task`; only afterward does `applyRefinedTargets` copy artifacts and normalize the write set, followed by task-type defenses.

Thus target metadata that could distinguish two tasks is available in the parsed response but is not consulted at the drop. The existing task is not widened to include the omitted target, and no skipped-task or covered-obligation record is created in this branch. [Add path][add], [target adapter][targets].

After processing the response, the refiner validates effects for surviving tasks, adds its verification-order edges, recomputes totals from surviving tasks, increments the revision and syncs facts. The live campaign then adopts that refined clone. Validating the survivors cannot detect that a separately proposed target was discarded unless some independent contract names that obligation. [Publication][publication], [snapshot admission][admission].

## Conditional witness: two independently required guides

Use an admitted campaign whose goal calls for **two separate deployment guides**, with distinct content and locations. The names below are synthetic. The current phase completes normally, and the immediately following phase already contains:

| Field | Existing task A |
| --- | --- |
| ID | `/guide_alpha` |
| Type/status | `/document`, pending |
| Description | `Write deployment guide in docs/alpha.md` |
| Declared artifact/write set | `docs/alpha.md` only |

The planner is refining detail already within the campaign goal, not adding an unauthorized new requirement. Condition on a successfully returned, parseable response containing this proposed addition:

```json
{
  "tasks": [
    {
      "task_id": "/guide_beta",
      "description": "Write deployment guide in docs/beta.md",
      "type": "/document",
      "priority": "/normal",
      "action": "add",
      "artifacts": ["docs/beta.md"],
      "write_set": ["docs/beta.md"]
    }
  ],
  "summary": "Add the second required deployment guide"
}
```

Assume both target spellings are valid workspace-relative paths, the two files are genuinely separate deliverables rather than aliases, the existing plan is valid, and fact synchronization succeeds. No other task declares beta's output; no additional change in this response compensates for the omission.

The source-derived sequence is:

1. The ordinary normalized descriptions differ: one retains `docs/alpha.md`, the other `docs/beta.md`. The exact-key guard does not match them.
2. Each suffix-insensitive key becomes `write deployment guide`: the last suffix is a single token containing both a slash and a dot.
3. A's key is already in `seenReplanKeys`, so the add branch warns and drops B before inspecting B's distinct targets.
4. A remains scoped to alpha. There is no B task to validate, project, schedule or count. Assuming the unchanged survivors are valid and publication succeeds, refinement returns success and publishes a revision without the newly proposed beta obligation.

This is deterministic given the accepted response and stated preconditions; no erroneous model judgment about equivalence is necessary. The model can have supplied the correct two-target distinction, only for the host's equivalence rule to remove it. It is still not a claim that a model will emit this particular response in practice.

The same filter also applies between two new additions in one response: after the first survives, its shortened key can suppress the second. That is a useful order-reversal control, not a separate finding. [Shared map updates][add].

## Actual consequence and competing protections

**The new execution obligation is absent, not marked completed.** Campaign facts and totals are built from the retained task objects. The stock eligibility rule can schedule only published tasks in a current phase. B receives neither a pending task row nor a dropped/covered disposition from this branch. The revision number still advances when fact publication succeeds. [Totals/publication][publication], [task fact projection][task-facts], [eligibility][eligibility].

**The warning is real.** It names both IDs and includes the proposed description, bounded to 80 characters. Do not call this wholly silent or claim every trace of the request vanished. The raw planner response, logging and unchanged campaign goal can supply evidence to a human or another component. The missing artifact is a typed executable obligation or explicit equivalence receipt in the accepted plan. [Drop warning][add], [goal retained in planning input][setup].

**Existing target/effect validation is not bypassed for a surviving invalid task.** `validateTaskEffect` requires targets for document and other file-effect tasks; the refiner applies it before publishing survivors. The witness supplies a valid target for B anyway. The issue is deciding B need not exist before that target participates in identity, not admitting a pathless task as completed research. [Effect validation][targets], [publication][publication].

**Matched updates are different.** A legitimate retargeting of A should use its existing ID with `action: update`; that branch applies targets rather than going through this add filter. Initial tasks that already include both A and B are not deleted merely when the maps are built. The witness requires a genuinely new addition; it is not an instruction to create duplicates instead of updating an existing obligation. [Matched update][update], [prompt retargeting guidance][prompt].

**Scope is per phase.** A matching task in another phase alone does not seed these maps. Nonmatching base wording, or a suffix that does not meet the parser's single-token/path-character condition, also avoids this specific near-duplicate match. Empty/punctuation-only normalization has a conservative pass-through behavior. These are relevant negative controls, not reasons to generalize the defect to all similar task descriptions. [Maps][maps], [key][key], [ordinary key][ordinary-key].

**Verification can rescue the overall campaign.** A phase objective/checkpoint that checks both required documents can reject incomplete work. A configured deterministic acceptance command runs after the phases and can append remediation or block completion. The acceptance path is optional; without a declared command it does not supply that extra witness. A worker may also recognize the original requirement and produce or request the missing deliverable independently. This audit does not establish any particular final verdict, claim the worker cannot recover, or weaken those checks. [Phase checkpoint contract][phase-checkpoints], [final acceptance][acceptance].

**Failure and authority controls still apply.** Planning errors, effect errors on survivors, inclusion/publication failure, cancellation, permissions, risk admission, write-set coordination and later verification can stop work. Dropping B does not grant a new write or bypass a safety gate. The bounded concern is lost work admission and potentially misleading plan completeness, not unauthorized execution.

## Proposed contract: preserve scope before deciding equivalence

### 1. Preserve the proposed operation before filtering

Build a bounded typed candidate from each proposed add/update: original ID/description, action, task type, declared artifacts, canonical write targets, prerequisite/context relationships, and any stable requirement or obligation identity available. Retain the original spellings for explanation. Resolve targets through the same workspace rules used elsewhere; do not create a second path-normalization dialect just for deduplication.

Separate the action sequence from the duplicate index. An index should describe the effective proposed revision, not independently decide what a removed or updated object still covers. This is a design requirement and evaluation case; the primary finding remains the distinct-scope add above.

### 2. Similarity proposes a match; evidence admits a merge

Use a shortened-description match as a candidate-pair signal, not final proof. Distinct explicit target sets, incompatible effects or different obligation identities should prevent automatic suppression unless an explicit coverage relationship accounts for both. Matching write sets alone is also insufficient: successive edits or separate investigations of one file can be legitimate different tasks.

For a genuine restatement, retain an auditable relationship from the proposed obligation to the surviving task: what effect and scope it covers, at what revision, and why a second dispatch is unnecessary. A broad existing task may cover a narrower proposal, but that coverage needs a declared scope/obligation witness rather than inference solely from a generic description. Unknown equivalence should lead to retained work or explicit bounded refinement, not automatic omission.

This preserves the host/executive split: host adapters measure canonical descriptors and exact relationships; Mangle can derive admitted duplicate/coverage dispositions from those facts. Do not move a large fuzzy natural-language rule bank into Mangle, and do not let a model's unsupported assertion of equivalence waive a required deliverable.

### 3. Keep the economics of genuine deduplication

Retain the existing ID-based update repair and strong exact-repeat controls where identity is actually established. The current positive restatement test is evidence of an intended cost-saving behavior, not proof that every path suffix is semantically redundant. Add an explicit common-scope witness to the positive control rather than merely deleting the test or forcing all approximate matches to execute twice.

For a conservative first correction, disjoint explicit target sets can veto this near-duplicate drop. That directly protects the primary example, but is not a complete semantic identity system. Cases without declared targets, overlapping directory scopes, aliases, repeated operations on one file and changed dependency/evidence needs need explicit treatment. Avoid promising that one extra path comparison solves all task equivalence.

### 4. Publish a disposition, not just a warning

After admission, every proposed obligation should be retained, explicitly removed by the plan operation, covered by a named survivor, or rejected/deferred with a reason. Expose that projection in the revision receipt and useful UI diagnostics. Recompute counts only after this conservation check; counts may legitimately be lower than raw proposal count when multiple proposals truly share one admitted task.

Preserve validation of the cloned plan before replacing the live model. Use the existing transactional fact-publication path where supported; the nontransactional fallback should not be described as an atomic multi-step transaction. Do not restore an omitted task by guessing from old warnings in a later process. Historical plans may lack enough metadata to reconstruct the intended distinction; label that uncertainty and use owner-authorized refinement instead of inventing scope.

## Costs and failure modes

The current key work is broadly linear in the descriptions it processes, with hash-map storage proportional to retained keys. A typed identity layer adds target normalization, descriptor storage, reference resolution, provenance and policy evaluation. Indexed candidate pairing can retain near-linear normal behavior; comparing every candidate with every task would create O(NM) work for N existing and M proposed tasks. Measure the chosen implementation rather than claiming the richer contract is free.

Path identity can require filesystem work for aliases or canonicalization. Use bounded, versioned descriptors and existing canonicalization services where possible. Platform case rules, symlink aliases, missing targets, directory scopes and renamed outputs require tests; lexical inequality is not a universal proof of distinct files, and lexical equality is not proof of identical work.

Restoring a legitimate omitted task increases useful model/tool work: possibly another document-writing turn, research, gate execution and checkpoint pass. That is not automatically a cost regression, nor should the old omission be counted as token efficiency. Conversely, conservative retention may admit genuine repeats and increase write conflicts, latency or review work. Measure both false suppression and false duplication under the same obligation coverage and quality standard.

Include planner/worker tokens, model latency, tool CPU and I/O, descriptor construction, kernel reevaluation, persistence bytes, task/phase retries, verification, retained receipts and operator effort. Schema migration, UI explanation and maintenance across all planning adapters are real engineering costs. Normal duplicate admission should not require an extra model call; semantic uncertainty can be surfaced for bounded refinement instead of an unconditional expensive equivalence judge.

## Fair evaluation — proposed, not run

Use deterministic responses through the actual refinement decoder, target adapter, clone/publication boundary and production scheduler. Capture proposed obligations, retained task objects, published facts, revision dispositions and any worker dispatches. Test the real TUI/orchestrator adapter in integration, while retaining permission and verification controls; a string-key unit test alone cannot establish end-to-end work conservation.

| Fixture | Required distinction |
| --- | --- |
| Existing alpha guide; add beta guide with equal base wording and disjoint explicit targets | Both obligations remain represented, or an explicit admitted task covers both; beta is not dropped by suffix equality alone. |
| Same two additions in one response, then reverse their order | Correct coverage does not depend on which path is mentioned first. |
| Genuine repeated obligation, same effect and canonical scope | Avoid redundant work and record the covering survivor. |
| Generic task plus a scoped restatement | Require a grounded coverage relationship; distinguish true containment from unknown scope. |
| Different base wording, same or different paths | Preserve the existing distinct-task control without assuming different words always mean different work. |
| Same target but distinct sequential effect or verification requirement | Do not merge merely because files coincide. |
| Existing ID update/retarget; add with distinct ID | Preserve update semantics while retaining genuinely additional work. |
| Remove/update/add combinations in one revision | Build duplicate indexes against the effective plan; no stale representative silently claims coverage. |
| Same description in different phases | Preserve the existing per-phase boundary unless an explicit cross-phase reuse contract is introduced. |
| Case-sensitive names, aliases, quoted spaces, directory targets and missing files | Normalize only supported equivalences; retain original evidence and meaningful scope distinctions. |
| Invalid target/effect, failed inclusion or fact publication | Keep the previous live plan; do not hide a rejection as a successful duplicate merge. |
| Independent acceptance detects beta missing | Credit the mitigation; distinguish repaired completion from correct first-pass admission. |
| No independent beta-specific check | Inspect the plan omission directly without claiming that any eventual prose verdict proves full completion. |

Compare current filtering, a minimal distinct-target veto, and typed obligation/coverage admission using identical task goals, planner outputs, source snapshots, budgets and verification contracts. Report false suppression, duplicate dispatch, represented/resolved obligations, rejection/ambiguity rate, final independently verified deliverables and full cost. Retain failed and unresolved runs. No measured reliability or efficiency gain is claimed here.

## Inventory and active branches

The review read current root/campaign/public-repository guidance and screened the research inventory through `research-main-20261030-1346` (`0b9eab6cc9f406a17bec847a84c3259e4e0cbf41`). Its added documents cover refinement dependency kind, generated-tool retirement, review severity and semantic expansion. Targeted searches included `replanDedupeKey`, suffix/near-duplicate task suppression, scope, and the existing tests. No earlier exact campaign target-suffix collision report was identified; this is a bounded screen, not a claim to exhaust every historical discussion.

The current `replan.go` was fetched at all five visible active non-research heads and is byte-identical to main: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908` and `767a3291d6287b238417f2466592a9002810d05a`. No competing implementation of this filter was found on those heads. [Representative active-branch source][branch].

[key]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L1220-L1253
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan_retype_test.go#L83-L154
[ordinary-key]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L698-L735
[prior-work]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/planned-work-conservation.md
[prior-consultation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-consultation-literal-identity.md
[prior-dependencies]: https://github.com/theRebelliousNerd/codenerd/blob/0b9eab6cc9f406a17bec847a84c3259e4e0cbf41/Docs/audits/campaign-refinement-dependency-kind.md
[tui-admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L56-L84
[tui-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L88-L140
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_init.go#L67-L75
[transition]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L235-L247
[rolling-wave]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L354-L412
[setup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L440-L529
[prompt]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms/campaign/replanning.yaml#L54-L84
[decoder]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L536-L575
[maps]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L578-L599
[add]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L612-L670
[targets]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_effect_contract.go#L16-L101
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L755-L791
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/campaign_fact_sync.go#L34-L98
[task-facts]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/types.go#L691-L727
[eligibility]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_tasks.mg#L99-L111
[update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L671-L691
[phase-checkpoints]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_phases.mg#L63-L90
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_acceptance.go#L21-L126
[branch]: https://github.com/theRebelliousNerd/codenerd/blob/51cfd42649552866d4aa54c53b68acf5740eb09b/internal/campaign/replan.go#L578-L670

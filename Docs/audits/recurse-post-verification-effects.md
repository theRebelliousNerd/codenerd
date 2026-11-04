---
doc-class: deep-dive
subsystem: campaign-recurse-verification-effects
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 5732e75982a55a8b4a8a142cbbe2c17baf761d87
verification-method: static-source-inspection
supersedes: []
---

# Recurse must reconcile effects produced by its verification commands

## Finding and decision

**The recurse driver closes its changed-path census before running its own post-attempt gates, then reuses that census for protection checks and adoption.** A gate that subsequently changes a different tracked file can leave that effect outside the ratchet's forbidden-path and mission-ownership checks, outside the path-limited commit, and outside the `OnKept` notification. The candidate's measured workspace and retained commit can consequently have different source/configuration contents without an external writer, untracked-file exception or Git index race. [Census and post-checks][census] [Protection consumers][protection] [Keep and notification][keep]

This is an **effect-set closure defect**, not a claim that every check must be side-effect-free or that every generated file belongs in Git. The required distinction is between admitted disposable build output, explicitly owned generated source, protected inputs and unexplained changes. The current result type and driver handoff do not express that distinction for effects first produced by an outer check. [Gate result and execution][runner] [Command construction][command]

The bounded correction is to make verification part of the attempt's effect lifecycle. Before settlement, reconcile the final candidate with the observed and authorized effects of both executor and host checks. A source-changing check must either be refused/contained or enter an explicit generation stage followed by verification of the admitted final candidate. Merely moving the existing Git census later would detect additional filenames but would not establish ownership, safe compensation, or verification of their final contents.

**Evidence limit:** this is a conditional source-derived finding, not a reproduced incident. No tests, builds, gates, repository programs, model inference, benchmarks or failure injections were executed. No repository file, credential or setting was changed. Public source copies and this standalone report are the local work products. The fixture and evaluation below are proposed and unrun.

## 1. Source boundary and architectural obligation

Public main was resolved to [`5732e75982a55a8b4a8a142cbbe2c17baf761d87`][pin] on November 4, 2026. Source reads, a complete recursive tree inventory and the audit queue were taken at that immutable commit, with main rechecked unchanged at 01:46 UTC. Locally retained source bytes were checked against their Git blob identities. Root and campaign guidance, the campaign internals, mission contract and recurse journey were read before selecting the finding. [Root guidance][root] [Campaign guidance][campaign-guidance] [Architecture guidance][architecture-guidance] [Campaign internals][architecture] [Mission contract][mission-spec] [Recurse journey][journey]

The intended architecture separates host observations from Mangle decisions. The host must supply a complete, correctly attributed account of effects; the kernel cannot reject a changed path that was never presented. Root guidance also requires revision-aware evidence. The recurse journey names forbidden-write refusal and a ratchet that commits or reverts the attempt's paths. The acceptance-evidence guidance independently requires stable before/after verification snapshots and verification after the last edit. Those obligations motivate an explicit boundary here; they do not establish that every existing subprocess already satisfies it. [Root guidance][root] [Journey safety contract][journey] [Evidence guidance][evidence-guidance]

The primary witness concerns ordinary recurse with declared workspace gates and no optional `MeasureGates` callback. Mission behavior is analyzed separately. It does not rely on unsupported-language detection, a fabricated model success sentence, an intentionally weakened test or missing executor write receipts.

## 2. Production trace

### 2.1 Executor ownership is returned before outer verification starts

CLI and chat both build a `RecurseAttemptCampaign`, run an orchestrator and return its `KeptWrites()` to `RunRecurseCycles`. The driver receives `owned, execErr := r.cfg.Execute(...)`. In mission mode it persists those returned writes before collecting paths or running the subsequent gates. [CLI adapter][cli] [Chat adapter][chat] [Returned writes][returned-writes] [Write sidecar][write-sidecar]

Those are meaningful receipts. They should not be discarded or treated as globally absent. Their temporal scope, however, ends with the executor's work: a subprocess the outer driver starts afterward cannot already be represented by that completed return.

The attempt campaign also does not unconditionally run every outer ratchet gate. A finding carries its own command as the campaign acceptance witness; its phase checkpoint is `VerifyNone`. Thus a separate declared build/lint gate is not guaranteed to have performed all its later effects before `Execute` returns. A worker may voluntarily run it earlier, but that is a material control case, not the contract that closes the outer handoff. [Attempt acceptance and phase][attempt-campaign]

### 2.2 The path census precedes the commands that can change it

At `recurse_cycle.go:1324`, `changedByAttempt` collects tracked working-tree differences from HEAD and newly untracked filenames relative to the attempt's initial name set, filtering the recurse ledger. The driver then measures own-node gates and selected workspace gates. Shipped policy includes workspace build and lint in every applicable ratchet; the target gate is also rerun. [Census order][census] [Collector][collector] [Ledger filter][filter] [Ratchet kinds][ratchet-policy]

No subsequent successful-keep census replaces `changed`. After measurements, the same slice supplies:

- `Changed` and `r.forbidden(changed)`;
- the mission loop checking ownership, known pre/postimages, current owned postimages, pinned verification inputs, protected paths and admitted scope;
- optional touched-package measurement scope;
- `r.git.commit(ctx, changed, ...)`;
- `OnKept(ctx, changed)`.

The existing predicates therefore assess a set observed **before** the driver's verification commands, rather than the complete effects present at settlement. [Protection consumers][protection] [Optional measurement][extra-checks] [Keep and notification][keep]

### 2.3 A gate is a real workspace process, not a read-only observation primitive

Declared commands and extra gates become argument vectors; node/workspace scope and an environment are retained. `gates.Run` calls `Command`, which uses `build.WorkspaceCommand`. That helper constructs `exec.CommandContext` and sets its directory to the actual workspace root. It applies an environment allowlist, not a filesystem write-isolation layer. `processutil.Run` manages process lifetime, pipe waits and cancellation; the inspected path does not return filesystem effects or wrap the command in an attempt write recorder. [Declaration adapter][declarations] [Runner][runner] [Workspace command][command] [Process lifecycle][process]

`gates.Result` carries argv, node, exit, output, truncation, pass/error and duration. It has no changed-file set, owned pre/postimages, generated-output contract or compensation receipt. The outer `measure` retains these results and parses findings. [Result schema][runner] [Measurement][measure]

This does not make the commands inherently malicious. Build generators, autofixing lint wrappers and test fixtures can legitimately write files. Native JS detection also invokes package scripts through a package manager; npm documents matching pre/post lifecycle scripts around named commands. That is one ordinary reason a command name is not a proof of read-only behavior. No particular npm project or package script was run or alleged faulty here. [JS command detection][js] [npm scripts](https://docs.npmjs.com/cli/v11/using-npm/scripts/)

Go's `os/exec` documentation describes `Dir` as the child working directory. This review's live-workspace conclusion comes from the actual command construction, not an assumption that selecting a directory constrains what its process can write. [Go command contract](https://pkg.go.dev/os/exec#Cmd)

### 2.4 The final commit and cleanup still use the earlier ownership domain

On a keep, `recurseGit.commit` stages and commits the supplied paths. The current implementation is path-limited, an important containment added after the earlier index audit. Git documents that explicit path arguments select their current working-tree contents rather than unrelated staged changes. A newly changed path not in the supplied list is consequently not adopted merely because the later checks used it. [Commit helper][commit] [Git path-limited commit](https://git-scm.com/docs/git-commit)

The driver copies the after-measurements into its state, increments kept, persists the ratchet and removes the in-flight marker. The notification still gets the original paths. It does not compare the committed tree with the final checked workspace before reporting that keep. [Keep and notification][keep]

On refusal/reversion, `undoOwned` restores recorded executor writes, then performs a fresh census to describe changes left behind. An unrecorded check-produced path is preserved and can be described as not this attempt's write. Preserving ambiguous files is safer than resetting them; it is not complete ownership or compensation of effects produced by the driver's own check. [Refusal settlement][settlement] [Owned compensation][undo]

## 3. Conditional witness: an outer generator introduces a tracked path

This is a proposed deterministic fixture specification, not an executed reproduction. It uses ordinary tracked paths and one executor write.

### Preconditions

Use a clean temporary Git repository on its already selected recurse branch. All relevant files are tracked, writable regular files; A and B are included by workspace membership and by mission evidence snapshots. There are no foreign writers, hooks, filters, symbolic links, preexisting untracked source files or uncertain Git operations.

- **A: `src/value.py`.** An admitted Python node contains a one-line value, initially 1. The intended repair changes only that literal to 2, leaving source-line and test-count metrics unchanged.
- **B: `generated/value.txt`.** A tracked non-source artifact initially contains the projection of A's old value. It is outside the selected node; an optional valid `nerd.md` forbid rule protects this exact path.
- **G: declared workspace build.** A deterministic generator/check reads A, writes B only when its projected content differs, checks its ordinary postcondition and exits zero. On the initial tree it has no net effect. It does not change A, tests or `nerd.md`.
- **T: declared workspace test.** A read-only executable check genuinely fails when A is 1 and passes when A is 2. Its failing output identifies `src/value.py` with a supported file/line diagnostic. [Diagnostic parser][parser] It does not need B to pass.
- Use valid `nerd/v1` declarations for both commands and ordinary available executables. Their declared kinds replace detected gates of those kinds. Keep any remaining gates read-only and passing. Focus on the `src` node if needed to exclude unrelated script nodes.
- No errors, cancellations, budget stops, failed journal writes or independent vetoes occur. All executor permission, lease and acceptance prerequisites are satisfied.

For the first boundary test, use a deterministic executor that changes A, returns its accurate known pre/postimage receipt, and does not run G. In a later full-route test, require the normal campaign to execute its own target acceptance T but not G before return. This last condition is explicit: if a worker has already generated B and a correct census/receipt includes it, that run does not exhibit this post-return witness.

### Expected source-derived sequence

1. Baseline G sees B already synchronized and leaves it unchanged. T reports the real A finding, which can be attributed to `src`. This initial measurement does not dirty the checkout.
2. The executor changes A from 1 to 2. Its returned owned set contains only A. T can verify the fix inside campaign acceptance without invoking the independent build G.
3. `changedByAttempt` returns `[A]`; B is still its HEAD version.
4. The outer driver runs the build G because build is a ratchet kind. G now changes B from B0 to B1 and exits zero. The target T is rerun and passes. With command IDs `nerd.md:build` and `nerd.md:test`, the sorted workspace-gate iteration places build before test. [Gate ordering][gate-order] [After scheduling][census]
5. The driver constructs protection facts by iterating only `[A]`. It never asks the retained project matcher about B. In mission mode it likewise never looks up B in the pinned-input, protected-path, scope or receipt checks.
6. The target finding is absent from the successful T result. With unchanged metrics and no other veto, the shipped policy can derive keep. This uses a genuine executable target oracle, not an always-green stub. [Target projection][target] [Keep policy][ratchet-policy]
7. Given successful Git operations, the path-limited commit records A1 but retains B0 in its tree. The live workspace still has B1. The driver records kept and notifies only A.

The minimal established mismatch is **final live effect set `{A,B}` versus ratchet/adoption set `{A}`**. The witness does not need the committed program to be broken: a generated artifact left dirty and excluded from the settlement contract is already the missing disposition. It also does not claim a remote push, irreversible loss or any actual production incident.

With the optional forbid on B, this becomes a precise missed protection observation: a fresh `r.forbidden` call on the final complete path set would identify B, but the current caller does not provide it. Whether a particular project intends a generator exemption is a product-policy decision. The current ratchet has no explicit exemption provenance that explains this omission, so a general claim that its changed protected paths were assessed would be unsupported. [Forbidden matcher][forbidden]

### Mission qualifications

Mission measurement brackets each gate with `evidence.Snapshot`. If G changes included workspace bytes, G's `run.snapshot` remains empty. This is real evidence of instability and should be preserved. The gate's pass/verdict is nevertheless determined from process result/error; empty snapshot is not projected as an ordinary ratchet refusal. [Snapshot bracketing][measure] [Verdict projection][verdict] [Ratchet bridge][bridge]

If T runs **after** G and is stable, T's after-snapshot can equal the final mission snapshot. The before-target snapshot can also match the pre-attempt mission source. Under the stated successful admissions, the current `missionWitness` condition can consequently mark the target resolved even though G had no stable snapshot and B never entered the effect checks. That supports only the measured gate deficit under mission policy, not satisfaction of an arbitrary mission or North Star. [Mission witness][mission-witness] [Mission evidence policy][mission-policy]

If a later mutator runs **after** T, the target's after-snapshot instead differs from the final mission snapshot and the mission witness remains unknown. This is a useful defense, but it still does not retroactively add B to the earlier `changed` slice or make unknown mission support an automatic keep veto. `finish` records the already chosen outcome and evidence disposition. [Mission witness][mission-witness] [Mission finish][mission-finish]

The primary report does not depend on either mission evidence outcome. Its ordinary driver witness precedes these additional requirements. Missing or failed mission preparation, stale owned postimages on A, changed protected paths already in the original census, and independent ratchet regressions remain valid blockers.

## 4. Distinctness from prior work

The complete current audit inventory and queue were screened, and the following neighboring reports were read. Targeted public-code searches for the collector and gate-write/verification-mutation terms found the production path and related reports, not this specific post-executor verification producer. This is a bounded main-branch novelty screen, not an assertion about every private discussion or historical branch. [Audit queue][queue]

| Earlier report | Already established boundary | Distinct residual here |
|---|---|---|
| [Compensation write ownership][prior-compensation] | A workspace diff does not establish authorship; preserve foreign writes and use observed receipts. | There is no foreign writer. The outer driver starts the process after its executor receipts are complete. Its own check effects lack an owner/settlement path. |
| [Index adoption identity][prior-index] | The old pathless commit could acquire foreign staged content; exact candidate trees are the stronger goal. | Current path-limited adoption is retained. The omitted B is a live tracked working-tree change first created by verification, not an unrelated staged version. |
| [Preexisting untracked owned effects][prior-untracked] | The initial untracked-name exclusion can hide an executor-owned change. | B is tracked and unchanged when the first census runs. Even a perfect census at that moment cannot see its future gate-produced version. |
| [Verification applicability][prior-applicability] | Verification judgments must remain bound to the source they assessed. | The host's own command is the later producer, and the exact path set drives protection, compensation, adoption and notification, not only judgment freshness. |
| [Portable recurse/deliberation proposal][portable] | Calls for source-bound authority, behavioral oracles and a versioned candidate/crucible. | Supplies a concrete existing-driver handoff and a small deterministic observation that the broader design must cover. |

The existing forbidden-write fixture edits the protected file **inside the executor callback**, before collection, and requires refusal/restoration. The guard-source bypass fixture likewise records writes made before the callback returns. These are useful positive controls, but do not compose a later outer gate write with the already-closed census. The inspected cancellation fixture also cancels from the executor callback. These fixtures were read, not run; this is not a claim that no other test anywhere covers related behavior. [Forbidden fixture][forbidden-test] [Guard-source fixture][guard-test] [Cancellation fixture][cancel-test]

## 5. Proposed contract and design choices

Update the owning campaign/gates specifications before implementation. Keep Mangle responsible for admission and settlement. Host code should report typed source/effect observations with enough identity to support the decision, rather than infer that a zero exit means a command was read-only.

### Separate three stages

1. **Candidate construction:** executor effects and any explicitly authorized generator effects have owners, allowed destinations and recoverable pre/postimages. A generation command is a mutation stage even if its executable also validates output.
2. **Verification:** required checks observe one admitted candidate identity. Source/configuration/oracle mutation during a purported read-only check invalidates that observation or yields an explicit generation transition, never a silent extension of the candidate.
3. **Settlement:** the final candidate tree, owned retained effects, permission/protection evaluation, commit and notification agree on what is being adopted. Effects not adopted have an explicit preserved, compensated, discarded-artifact or unresolved disposition.

Do not automatically call every newly discovered file the attempt's write. A final census detects divergence; it does not prove origin, give permission to stage it, or authorize overwriting its preimage. Preserve the existing independent-writer protections and uncertain-adoption handling.

### Start with a conservative stop, then support legitimate generation

For source and verification inputs, bracketing checks with a complete snapshot and stopping on unexpected mutation is a useful initial containment. Retain an exact diagnostic and the in-flight effect uncertainty; do not label the result a clean revert if the host cannot safely restore its check's writes.

Define disposable outputs separately. Compilers and tests need caches, reports and temporary files. A blanket whole-tree-write prohibition can make legitimate checks unusable, while blanket exclusion of `generated/`, ignored files or all build output can conceal source or oracle changes. The contract needs an admitted output domain rather than a directory-name heuristic.

For legitimate tracked generated outputs, either run generation before sealing the candidate or execute it in an isolated candidate environment, capture its owned changes and obtain the required admission before promotion. Then rerun relevant behavior checks against that final admitted content. Isolation without correct workspace/environment/tool routing and output promotion is not a solution by itself.

### Preserve recovery and evidence

The lifecycle owner must cover success, failing exit, cancellation, crash and uncertain adoption. A gate can mutate before it returns an error. Save sufficient bounded evidence to distinguish a known no-effect check from an incomplete effect census. Do not keep raw secrets or an unlimited subprocess transcript merely to prove ownership.

Per-gate receipts should bind command/environment identity, workspace/candidate identity, process completion, source-before/source-after, output domain and effect disposition. A source digest alone cannot safely restore an overwritten file; compensation needs preserved preimages or an isolated transactional boundary. A fresh name list cannot detect a same-path version change after the last check.

For mission runs, apply protected-input, protected-path, scope and owned-postimage checks to the admitted final effect set, not only executor-era names. Preserve the distinction between ratchet adoption and mission evidence support. Do not weaken an existing protection merely because a project command produced the conflicting effect.

## 6. Fair proposed evaluation, not run

Begin with deterministic temporary repositories and real shipped ratchet policy, gate execution and Git adoption. Replace only model behavior with a fixed, correctly attributed executor. Record the original census, complete post-check diff, gate snapshots, owned receipts, ratchet facts, actual commit tree, residual working-tree changes and notification paths. Inspect content identities and modes, not only kept counts or exit codes.

| Control | Required outcome |
|---|---|
| Primary A-only executor, G changes tracked B later | B is explicitly admitted and verified, or settlement holds/refuses without claiming complete effect adoption. |
| Same fixture, B forbidden/protected/out of scope | B cannot disappear from the restriction decision; no implicit generator exemption. |
| Read-only G, ordinary valid A repair | Normal keep remains possible; a repair that blocks everything is not success. |
| G produces identical B bytes | No false source-change finding from unchanged content; metadata semantics are explicit. |
| G executes before executor return and B is properly recorded | Distinguish this control from the post-return residual; retain correct ownership handling. |
| G mutates an already-listed A instead of new B | Detect final-version/receipt mismatch; a second filename census alone is insufficient. |
| G changes a pinned test/configuration file | Preserve oracle protection even when a later target check passes. |
| G fails, cannot start, or is canceled after an effect | No false pass or false clean rollback; bounded cleanup and explicit unresolved effects. |
| Allowed disposable cache/report output | Checks stay usable without allowing source or oracle files into an unreviewed exclusion. |
| Authorized tracked generation | Correctly capture, review/admit, verify and adopt generated content rather than silently dropping it. |
| Mission G-before-T and T-before-G orderings | Preserve accurate per-gate snapshots and mission support while enforcing the same complete effect boundary. |
| Foreign edit occurs during a check | Do not relabel or overwrite it as gate-owned merely because it appears in a post-check diff. |
| Later error-free measurement or focused witness changes files | The final lifecycle boundary includes every post-executor producer, not only ordinary `measure`. |
| Crash before/after effect recording, before/after commit | Recover exact state without repeating ambiguous mutation or deleting unrelated work. |
| POSIX and Windows, spaces and non-ASCII paths, modes | Equivalent identity/ownership contract and honest unsupported cases. |

Compare the pinned baseline, a post-check recensus-only candidate, and the selected full effect-lifecycle design on identical fixtures and permissions. The middle arm matters: it may find B and trigger a veto, yet still lack ownership/compensation or a stable final candidate. Include clean positive controls and legitimate generator workflows to measure unnecessary holds and usability.

Then exercise normal CLI and chat adapters with production campaign acceptance, permission/lease checks, source/world-model context, real host gate processes and final notification behavior. The deterministic injected-executor phase establishes the mechanical boundary, not complete session qualification. Any later live-model study should be separately authorized and hold model/version, task population and budgets comparable.

Measure unexplained residual effects, missing restriction facts, candidate/commit mismatches, false holds, unsafe compensation attempts, recovery ambiguity and lost notification paths separately. Count extra process launches, bytes hashed/copied, peak memory/disk, verification reruns, latency and any additional model work caused by holds. No frequency, token reduction, latency improvement or success rate is claimed by this report.

## 7. Costs, risks and closure

A complete source census or snapshot adds filesystem traversal and hashing proportional to the admitted files/content. Repeating it around every gate can dominate short checks in a large repository. Incremental observation needs a trustworthy freshness boundary; a watcher alone can miss races or unsupported writes. Preimage capture, isolation and generation promotion add storage, lifecycle and recovery costs. No new inference or embedding layer is intrinsically required for the narrow host boundary.

Principal repair failures are treating every diff as owned, suppressing legitimate caches through a too-broad write ban, allowing source through a too-broad artifact exclusion, forgetting late witness producers, dropping receipts on failed commands, rechecking only filenames, changing the candidate after verification, and clearing in-flight state before unresolved host effects have a disposition. Parallel workspace baseline gates also need deliberate effect semantics; they are outside the single post-attempt witness, not evidence that all source changes there are already safe.

The closure criterion is bounded: **after all checks that can affect the candidate, every net retained source/configuration/oracle effect is either admitted with owner and verification evidence, explicitly preserved outside the adoption with an honest hold, or safely compensated. The ratchet, adopted tree and notification must describe that same disposition.** The A/B witness must no longer record an ordinary successful keep while silently omitting B from its effect accounting. This does not establish a universal sandbox against arbitrary external processes or the entire portable recurse vision.

## Pinned references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/5732e75982a55a8b4a8a142cbbe2c17baf761d87
[root]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/architecture/AGENTS.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/architecture/campaign/INTERNALS.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/journeys/10-forever-loop.md
[evidence-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/evidence/agents.md
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/cmd/nerd/cmd_campaign_recurse.go#L226-L272
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/cmd/nerd/chat/campaign_recurse.go#L223-L279
[returned-writes]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1284-L1303
[write-sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_mission_storage.go#L64-L77
[attempt-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_attempt.go#L34-L145
[census]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1324-L1355
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_git.go#L96-L127
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1663-L1676
[protection]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1352-L1391
[extra-checks]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1392-L1414
[declarations]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/gates/detect.go#L15-L91
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/gates/run.go#L15-L100
[command]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/build/workspace_command.go#L30-L65
[process]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/processutil/command.go#L23-L52
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L759-L839
[js]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/gates/detect.go#L175-L214
[gate-order]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/gates/gates.go#L171-L196
[target]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1421-L1445
[verdict]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L258-L267
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_policy.go#L205-L270
[ratchet-policy]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/core/defaults/policy/recurse.mg#L259-L335
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_git.go#L125-L147
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1481-L1571
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1508-L1532
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1891-L1920
[mission-witness]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1446-L1459
[mission-policy]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/core/defaults/policy/mission.mg#L26-L32
[mission-finish]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_mission.go#L326-L376
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/audits/ASTRA_QUEUE.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/audits/recurse-compensation-write-ownership.md
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/audits/recurse-index-adoption-identity.md
[prior-untracked]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/audits/recurse-untracked-owned-effect-adoption.md
[prior-applicability]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/audits/verification-applicability-and-progressive-evidence.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/Docs/audits/portable-recurse-deliberation-crucible.md
[forbidden-test]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle_test.go#L200-L227
[guard-test]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_guard_source_test.go#L65-L99
[cancel-test]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle_test.go#L229-L253

[parser]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/gates/findings.go#L340-L379
[forbidden]: https://github.com/theRebelliousNerd/codenerd/blob/5732e75982a55a8b4a8a142cbbe2c17baf761d87/internal/campaign/recurse_cycle.go#L1740-L1754

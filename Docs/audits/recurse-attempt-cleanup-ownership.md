---
doc-class: deep-dive
subsystem: campaign-recurse-cleanup
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 7ec32947fcd6b372c1153851a4455a9296699bda
verification-method: static-source-inspection
supersedes: []
---

# Recurse cleanup needs an acquired, literal artifact owner

## Finding and decision

**Recurse installs destructive campaign cleanup before acquiring the campaign it intends to run.** Both production callbacks defer `ReleaseRecurseAttempt` immediately after constructing a candidate campaign. `SetCampaign` can then correctly reject that candidate because its ID already has a durable journal or is held by another orchestrator. The deferred cleanup nevertheless retracts facts keyed by the candidate's ID and removes every filesystem entry matching that ID's prefix. Its arguments contain no evidence that this invocation acquired or created those resources. [CLI callback][cli] [Chat callback][chat] [Admission][admission] [Cleanup][cleanup]

The primary conditional witness is a fresh recurse campaign ID that equals an existing campaign ID. Recurse uses the first eight hexadecimal characters of a new UUID; there is no uniqueness reservation in its constructor. This review does **not** claim such a collision is frequent, has occurred, or is controllable by the model. The significant contract is that a rejected acquisition must be harmless even when names coincide. The existing durable-journal and owner-lock checks explicitly protect that case, but cleanup can undo their protection. [Constructor][constructor] [Ownership lock][owner]

The same cleanup boundary also turns literal workspace names into pattern syntax. A bracket-containing workspace can leave its own attempt artifacts behind, or, if a matching sibling directory contains a matching campaign basename, select that sibling's artifacts for deletion. **This cleanup/path observation is already documented:** §5 of the mission-receipt census audit expressly names `ReleaseRecurseAttempt`, both production callers, missed intended cleanup and the matching-sibling-artifact precondition for unrelated deletion. It is retained here as a credited regression requirement for the proposed ownership repair, not a second novel finding. The new join is **failed acquisition → unconditional retirement**. [Cleanup][cleanup] [Prior census §5][prior-census-cleanup]

**Recommended contract:** retirement must consume an exact acquisition/creation receipt, bind it to the literal workspace and campaign generation, and remove only the resources that receipt owns. A candidate name, successful string sanitization, or entry into an executor callback is not such a receipt. Preserve bounded long-running state, existing owner locks, source-write compensation and Mangle's executive role.

**Evidence limit:** source composition and documented Go semantics, not an executed reproduction. No repository program, test, build, gate, model inference, benchmark or fault injection was run. No implementation, repository file, credential, permission or setting was changed. This standalone report and public-source copies are the work products. All evaluations below are proposed and unrun.

## 1. Scope, architecture and prior work

The inspected public main is [`7ec32947fcd6b372c1153851a4455a9296699bda`][pin], rechecked unchanged at 02:48 UTC on November 4, 2026. Root, public-repository, campaign and architecture guidance were read first, followed by the mission specification, relevant production paths, authored cleanup fixture and neighboring audits. Local copies of referenced repository files were matched to the pinned Git blob identities. This is a bounded screen of current public main, not a claim about every historical branch or unpublished change. [Root][root] [Public repository][public] [Campaign guidance][campaign-guidance] [Architecture guidance][architecture] [Mission contract][mission-contract]

The north star assigns creative proposal and challenge to models while the host and kernel supply execution, evidence and safety. The Socratic mission uses ordinary recurse attempts rather than a separate scheduler. Preserving another campaign's durable state is therefore an ordinary host lifecycle obligation. A better proposer, requirements interview or final review cannot repair deletion that occurs after admission has already refused the attempt.

Relevant prior boundaries are distinct:

- **Campaign generation admission and recovery** documents the owner lock, refusal to overwrite a durable generation, admitted resume, journal uncertainty and recovery. Those protections are present and deserve credit. This report traces an outer cleanup caller that runs even when admission refuses; it does not reopen the earlier missing-lock finding. [Prior generation audit][prior-generation]
- **Campaign/recurse scheduling ownership** studies two admitted campaigns sharing scheduler facts, including global readers and broad refresh. The primary witness here needs only one retained, non-running campaign and a candidate that is never admitted. Its disk effect does not require a shared live kernel. [Prior scheduling audit][prior-scheduling]
- **Recurse compensation and index adoption** concern ownership of source effects and Git publication. This report concerns deletion of campaign bookkeeping by the host's callback cleanup, before the outer ratchet. The source-write receipt mechanism is not its producer. [Prior compensation][prior-compensation] [Prior index adoption][prior-index]
- **Mission recovery receipt census** owns literal-root enumeration and incomplete-read admission. Its §5 also expressly identifies this cleanup helper, both production callers, missed cleanup and the necessary matching-sibling-artifact condition for unrelated deletion. This report does not claim that destructive consumer or conditional path witness as new. Its new contribution composes rejected campaign acquisition with the unconditional cleanup defer; the known literal-root case constrains a complete repair. Fixing either the mission scanner or cleanup alone would still leave the other's boundary unresolved. [Prior receipt census][prior-census] [Prior census §5][prior-census-cleanup]

The audit inventory and adjacent reports above were inspected. A code-search result is not sufficient evidence of absent prior prose: the census audit already discusses this helper. The bounded novelty claim is only the joined **failed acquisition → unconditional retirement** path, not the known literal-root witness or the general principle that cleanup needs ownership.

## 2. Production trace

### 2.1 Construction allocates a candidate identity, not ownership

`RecurseAttemptCampaign` creates `/campaign_` plus the first eight characters of `uuid.New().String()`. It derives phase/task IDs and a knowledge-database path from that identity, and returns a campaign structure. It does not check the existing journal, reserve the namespace, acquire the campaign owner lock or return a created-artifact inventory. [Constructor][constructor]

`sanitizeCampaignID` removes the leading slash and maps invalid character runs to underscores. That is useful for a filename component; it neither proves allocation nor literalizes the separate workspace argument. For an ordinary generated ID such as `/campaign_1234abcd`, the slug is `campaign_1234abcd`. [Sanitizer][sanitize]

### 2.2 Both normal entry points install cleanup too early

The CLI callback creates `camp`, sets its context budget and immediately defers `ReleaseRecurseAttempt(cwd, kernel, camp)`. It then builds the orchestrator, defers `orchestrator.Close`, calls `executeCampaignPlan`, and returns kept writes plus any error. `executeCampaignPlan` first calls `SetCampaign` and returns immediately if admission fails. [CLI callback][cli] [CLI admission consumer][cli-set]

The chat callback has the same important order. Its cleanup defer is installed before constructing the orchestrator. After construction it defers `orch.Close`, calls `orch.SetCampaign(camp)` and returns `nil` writes plus an error on refusal. `orch.Run` is reached only after successful admission. [Chat callback][chat]

Go executes deferred calls on function return, including an early error return, in reverse registration order. Consequently `Close` runs before the earlier cleanup defer when an orchestrator exists. This is language behavior, not a conjectured error handler. [Go defer contract][defer]

### 2.3 Admission correctly refuses preexisting ownership

`SetCampaign` first calls `acquireOwnershipLocked`. The latter opens the campaign's `.owner` file and takes the nonblocking OS lock; a held lock returns `ErrCampaignOwned`. Successful acquisition can truncate the `.owner` file and write the acquiring process's PID before `SetCampaign` checks the journal path. If a journal already exists, admission returns an error instructing the caller to resume the recorded generation. Only afterward does it reset journal state, assign `o.campaign`, load facts and save a new campaign. The PID update is legitimate lock metadata, not the destructive retirement under review. [Admission][admission] [Ownership lock][owner]

Thus the existing-journal rejection can happen **before the candidate has been installed or published**. A retained campaign does not become the new recurse attempt's resource merely because the same ID was proposed or its owner lock could temporarily be acquired.

`Close` releases any owner lock it holds. It does not call `ReleaseRecurseAttempt` and does not confer artifact-deletion authority on its caller. Its documented role is releasing lifetime resources. [Close][close]

### 2.4 Retirement receives only names

`ReleaseRecurseAttempt` checks only for a nil campaign before beginning retirement. It calls `retractCampaignFacts(kernel, c)`, computes the sanitized slug, then evaluates:

```text
filepath.Glob(filepath.Join(workspace, ".nerd", "campaigns", slug + "*"))
```

Every returned path goes directly to `os.RemoveAll`. There is no admitted-generation token, successful-publication flag, created-path inventory, equality check with the configured root, owner-lock acquisition, or per-artifact ownership validation. A kernel-retraction error is retained as `firstErr`, but it does not prevent the filesystem loop. [Cleanup][cleanup]

The fact helper retracts campaign-wide families by the supplied campaign ID, then profiles, phases and tasks derived from the supplied structure. The kernel's `RetractFact` matches predicate and first argument, not a publication-generation receipt. Neither operation tests whether this invocation published those rows. The filesystem witness does not depend on any particular kernel result: the helper proceeds to disk cleanup even if fact retirement returned an error. [Fact retirement][facts] [Kernel retraction][kernel-retract] [Cleanup][cleanup]

The prefix matches the ordinary campaign snapshot, journal and knowledge-directory names. It can also match an `.owner` entry and unrelated prefix-sharing entries. Not every such entry is necessarily foreign; some may be intentional campaign artifacts. The missing contract is that matching the prefix alone is accepted as sufficient ownership evidence. [Snapshot layout][snapshot-layout] [Journal layout][journal-layout] [Knowledge path construction][constructor] [Owner path][owner]

### 2.5 Outer verification does not govern this deletion

The cleanup defer runs inside `Execute`, before the outer driver receives the callback's writes and error. The outer driver then records mission writes when applicable, checks cancellation, collects source differences, remeasures and judges its ratchet. Its compensation uses the returned `FileWrite` sequence, not a receipt for `ReleaseRecurseAttempt`'s direct `RemoveAll` calls. [Outer executor boundary][outer-execute] [Owned compensation][undo]

For the primary witness, use the normal case where `.nerd/` is ignored and the rejected worker has returned no source writes. The cleanup target is not a source edit that later test failure or an outer `/revert` can restore. No false-keep claim is needed: another campaign's artifacts can be removed even while the candidate execution is correctly refused.

## 3. Primary conditional witness: refusal deletes a retained campaign

This is an unrun fixture specification, not a claim about an observed user workspace.

### Preconditions

1. Use a normal literal workspace with no glob metacharacters, clean tracked Git state, the normal recurse lock available and no unresolved recurse state blocking admission.
2. A different, previously created campaign **B** has valid retained state under `.nerd/campaigns`: its snapshot, durable journal and knowledge directory. It is not currently running and holds no owner lock. Retaining a completed or paused campaign is sufficient.
3. A fresh recurse attempt **A** is selected and reaches the production executor callback. Its newly generated eight-hex ID happens to equal B's ID. The candidate and historical campaign are different lifetimes despite that string equality.
4. The orchestrator factory succeeds. Kernel operations and filesystem access have ordinary successful behavior; the witness needs no malicious model output, damaged journal, source edit, concurrent writer or failed filesystem read.

The equal-ID condition must be explicit. There is no claim that an arbitrary recurse invocation deletes an arbitrary campaign. A deterministic later test should supply that identity at the construction boundary rather than waiting for random generation to collide.

### Source-composed sequence

| Boundary | Observed source behavior under the stated conditions |
|---|---|
| Candidate construction | A has B's string ID; it has not acquired B's generation. |
| Callback entry | A's cleanup defer is installed unconditionally. |
| `SetCampaign` | The owner lock can be acquired and its PID metadata refreshed, but B's existing journal causes a refusal before `o.campaign = A`, fact publication or new snapshot save. |
| Callback return | No A task runs. Chat returns nil writes; CLI returns the new orchestrator's kept-write slice and the admission error. |
| `Close` defer | The temporary owner lock is released. B's durable artifacts still exist at this point. |
| Cleanup defer | A's name selects B's snapshot, journal and knowledge directory. Successful `RemoveAll` removes them. |
| Outer ratchet | Its later decision cannot undo these unreceipted campaign-artifact removals. |

The directly established failure is destructive retirement of preexisting state after a successful admission refusal. It does not require claiming that the old campaign resumes, a worker runs without a lock, or a bad source change is committed.

A second, separately qualified case uses B holding its owner lock. `SetCampaign` then refuses earlier, at lock acquisition. The cleanup defer still reaches the same file selection. Platform-specific open-file deletion behavior can change which removals succeed, so the retained/non-running case above is the portable primary witness. If B's facts are also in the supplied kernel, keyed fact retirement is an additional interference risk; disk deletion does not require it.

## 4. Previously documented literal-root regression

This section makes the already documented census-audit §5 cleanup case explicit for repair evaluation. It is independent of failed acquisition but is not an independent novelty claim. [Prior census §5][prior-census-cleanup]

Use an admitted workspace with the literal path `/work/project[ab]`, and let `s` be the current attempt's sanitized campaign ID. Its expected cleanup directory is literally `/work/project[ab]/.nerd/campaigns`.

The helper instead submits the pattern `/work/project[ab]/.nerd/campaigns/s*`. Go's documented matching syntax treats `[ab]` as a character class. A directory `/work/projecta` or `/work/projectb` can match, while the literal directory `project[ab]` does not match that component. [Go pattern contract][match]

There are two bounded outcomes:

- **No matching sibling:** ordinary cleanup can return success with no matches, leaving the actual attempt's snapshot, journal and knowledge directory behind. Repeated attempts then defeat this helper's stated purpose of keeping disposable campaign state bounded.
- **Matching sibling with a matching basename:** if `/work/projecta/.nerd/campaigns/` contains an entry beginning with `s`, cleanup selects that entry and recursively removes it if access permits. The entry may be an independently retained artifact or a saved copy; its same-name/prefix condition is necessary and must not be omitted. The cleanup does not check whether it belongs to the selected workspace or invocation. The original workspace's own artifact still is not selected.

This witness needs no failed campaign admission. A legitimately admitted, otherwise successful attempt can reach it. It also needs no symlink or directory swap. File-system access failures can prevent some deletions, and absent sibling matches prevent the cross-root outcome. Neither circumstance repairs literal cleanup.

`CanonicalWorkspaceRoot` resolves filesystem spelling and symlinks; it does not escape a later pattern language. Additionally, the entry callbacks pass their captured workspace string to cleanup, not a root handle issued by the shared driver. Canonicalizing that string alone would therefore not repair bracket interpretation. [Canonical root][canonical] [Shared driver root][run-root] [Callbacks][cli]

Go also documents that glob enumeration suppresses directory-read errors. At this cleanup boundary that can produce incomplete retirement without an error, independently of the cross-root witness. The prior mission census report already owns the broader complete-enumeration reasoning. This report's recommendation is to avoid pattern expansion for owned artifact identity, not to treat a successful glob as a deletion authorization. [Go glob contract][glob] [Prior census][prior-census]

## 5. Required cleanup contract

Update the campaign lifecycle and recurse capability specifications before implementation. The following is proposed design, not a shipped API.

### Ownership and lifecycle

- **Candidate state is non-owning.** Constructing a campaign may produce names and requested configuration. It must not enable fact or file retirement for that name.
- **Acquisition reports exact rights.** The host should return an opaque owner/generation receipt that identifies the canonical workspace, campaign ID, allocation generation, resources newly created by this invocation and resources merely opened or found preexisting.
- **Refusal preserves existing state.** An existing journal, held lock, generation mismatch or failed inclusion check must not authorize deletion of another generation. Preserve its snapshot, journal, knowledge data and stable lock identity; this does not prohibit legitimate PID metadata updates during successful lock acquisition. Partial initialization may clean up only resources proven newly created by that initialization.
- **Retirement follows the owner state machine.** Stop/join any owned activity and preserve the required durable handoff before retiring its campaign. A successful cleanup of disposable state must be distinct from an uncertain or refused retirement.
- **Facts carry publication ownership.** Retract only the admitted generation's published rows. Reconstructing candidate phase/task IDs is not evidence those rows were published by this invocation.
- **Keep lock identity stable.** Do not fix this merely by deleting `.owner` while holding its current handle, or by releasing it and assuming no new owner can appear before deletion. Specify a stable lock namespace or equivalent retirement/tombstone protocol that excludes a newly admitted owner throughout artifact retirement. Qualify the actual platform semantics.

The existing per-campaign owner lock and rejected-new-generation behavior remain necessary. Making IDs longer lowers collision risk but cannot establish retirement ownership. Moving a cleanup defer after a successful `SetCampaign` is useful containment for the primary refusal witness, but is not a complete answer for partial publication failures, lock-release gaps or literal target selection.

### Literal and exact artifact identity

Prefer a single attempt-owned artifact directory plus explicit sibling metadata identities, or an exact inventory returned by creation. Operate relative to the admitted literal root. Enumerate a literal directory when discovery is necessary, propagate enumeration errors, and validate exact names and ownership rather than matching a free prefix across the whole path.

Existing layout spans snapshot, journal, knowledge directory and owner metadata, so a migration must account for those producers and readers. Preserve records needed by admitted recovery; do not delete unknown legacy files just to satisfy a cleanup-count assertion. A conservative retained orphan with a clear owner-visible reason is preferable to removing unproven state.

If root-relative APIs are selected, verify their minimum supported Go version and Windows/Unix behavior. A lexical containment check alone is not a universal defense against concurrent path replacement. The minimal witnesses use stable directories; broader race resistance requires its own contract and tests. [Go removal API][remove]

### Outcome propagation

Both callers currently print a cleanup error without adding it to the callback's returned error. Define whether incomplete retirement stops new attempts, records bounded cleanup debt, or allows continuation under an explicit retention budget. No policy should silently equate “no matched entries” with verified retirement when the enumeration identity is uncertain.

The model should not choose whether to ignore cleanup debt, widen the deletion root or erase a conflicting campaign. Host observations should identify the owned resources, publication status and retirement outcome; any executive continuation/retention policy belongs at the existing kernel boundary. This repair requires no additional inference call.

## 6. Existing controls and important nonclaims

- `SetCampaign`'s existing-journal rejection and owner-lock refusal work in the inspected source. The finding is their composition with unconditional cleanup.
- The primary witness requires equal generated and retained campaign IDs. No random-collision incidence, probability estimate or exploitability rate was measured.
- The literal-root witness requires pattern-significant path spelling; cross-root deletion additionally requires a matching foreign basename and sufficient filesystem permissions. Ordinary punctuation-free roots and unrelated names are negative controls.
- `os.RemoveAll` attempts recursive removal and can partly succeed before returning an error. An error is not proof that every artifact was preserved. [Removal contract][remove]
- Campaign artifacts are the affected resources in the presented cases. This is not a claim that arbitrary workspace source paths are directly matched by this helper.
- The report does not assert that ordinary campaign journals contain the exact process-local `FileWrite` preimages. The outer mission sidecar has its own publication boundary; that broader crash-recovery question is not established here.
- There is no new requirement to retain every attempt forever. Bounded disposal is desirable when the resources actually belong to the retiring attempt.
- A per-ID mutex, UUID choice, source test pass or valid generated plan is not a substitute for cleanup ownership. Conversely, a correct retirement protocol must not disable useful mission work just to achieve zero deletions.

## 7. Authored evidence and fair unrun evaluation

`TestReleaseRecurseAttempt_DropsTheAttemptsCampaign` creates two separately generated recurse campaigns under one ordinary temporary directory. It writes each campaign's snapshot, journal and knowledge database, loads both fact sets, releases one, and expects the other's three filesystem entries and campaign fact to remain. This is a valuable basic isolation control. It does not exercise rejected `SetCampaign`, equal candidate IDs, a live ownership refusal, bracket-containing roots, prefix-sharing foreign artifacts, partial cleanup or acquisition-to-retirement races. The fixture was inspected, not run. [Existing fixture][fixture]

Proposed evaluation should compare the current implementation, the narrow successful-admission defer containment, and the full acquisition-receipt/literal-target design. Keep the campaign set, lifecycle outcomes, kernel, filesystem permissions and cleanup obligations identical.

| Case | Required observation |
|---|---|
| Retained valid campaign journal; same-ID new recurse candidate | Admission refuses before execution; the preexisting snapshot, journal and knowledge data remain byte-identical, facts remain unchanged where present, and existing lock identity stays stable. A legitimate `.owner` PID metadata refresh is allowed. |
| Same-ID campaign holds its owner lock | Refused candidate never retires the active owner's facts, snapshot, journal, knowledge directory or stable lock identity. |
| Distinct IDs in ordinary literal root | Only the admitted attempt's disposable resources are removed; existing positive control remains valid. |
| Factory fails before ownership | Candidate-only names confer no deletion right. |
| Failure after lock acquisition but before publication; partial snapshot/journal failure | Newly created resources are distinguished from preexisting state; uncertainty is retained and recoverable. |
| Literal bracket root with no matching sibling | The intended attempt artifacts are cleaned or an explicit retirement failure is returned; no silent accumulating orphan. |
| Literal bracket root plus matching sibling and matching basename | Sibling content survives unchanged; the intended literal root is the only target. |
| Prefix-sharing entry owned by a different generation or unknown owner | Prefix similarity alone does not authorize deletion. |
| Cancellation, executor error and successful run | Same ownership rules at all callback exits; source compensation and mission receipts retain their contracts. |
| Owner handoff attempted during cleanup | No admitted new owner loses resources or lock identity to the old owner's retirement. |
| Permission/read/remove failure, partial removal, repeat cleanup | Outcome names what remains and what was removed; retry is bounded to the same owned resource set and does not broaden targeting. |
| Linux and Windows; symlinked root and supported alias spelling | Literal identity and owner lifetime remain correct under each qualified platform's semantics. |

First use deterministic, offline production-composition tests. A helper test alone is insufficient for the primary issue because it can supply a campaign assumed to be owned. Drive the actual callback ordering through a constructor seam with a fixed ID, real `SetCampaign` refusal and preserved peer fixtures. Also inspect actual kernel rows and artifacts, not just the returned error or an expected log string. Do not wait for a random collision, use live model work to produce one, or run destructive fixtures against a user's real campaign directory.

For the literal-root case, keep two wholly disposable trees and assert both the intended removals and the untouched sibling. A mock glob that simply returns a chosen foreign path would test a consumer but not establish the actual filesystem-name composition.

Then measure long-running resource behavior with equal successful, refused and canceled attempts. Record orphan count/bytes, deletion count and identity, retained recovery records, kernel fact counts, cleanup latency, acquisition/retirement wait, directory operations, bytes read/written, host CPU, allocations and human intervention. Compare useful completed work as well as preservation: a solution that never starts or never cleans anything must not appear successful merely because it avoids foreign deletion.

## 8. Costs and exit criteria

A successful-admission cleanup flag is a small containment, with low metadata overhead, but needs precise partial-initialization semantics and still leaves target/root and lock-lifetime problems. An owned artifact inventory adds bookkeeping proportional to artifact count, a lifecycle API change and versioned recovery state. A single owned directory simplifies membership but requires compatibility for existing snapshot, journal and knowledge consumers. Stable retirement synchronization can add wait time and reduce overlap. Conservative failure handling retains extra disk state; bounded retry or garbage collection then needs its own proven ownership rules.

Root-relative literal operations avoid repeated pattern interpretation but are not free: directory enumeration costs scale with entries when an inventory is unavailable, and stronger identity checks may add metadata reads and platform-specific behavior. Persisting a cleanup receipt adds I/O and crash-consistency work. Benefits such as fewer orphan campaigns, preserved diagnostics, fewer failed resumes or less operator work remain hypotheses, not measured gains.

No model, embedding, judge or training request is intrinsically needed. If the repair preserves a formerly lost campaign and permits its authorized work to continue, total future model spend may increase while useful completion improves. Report those effects separately rather than claiming automatic token savings.

The exit is specific: **a recurse candidate that never acquired a campaign cannot retire it; a retiring attempt can affect only its exact owned generation under its literal workspace; incomplete retirement stays explicit without discarding recovery evidence.** Existing normal cleanup stays bounded, and proposer/challenger work continues through the same mission, campaign and ratchet architecture.

## Immutable source references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/7ec32947fcd6b372c1153851a4455a9296699bda
[root]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/agents.md
[public]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/.claude/rules/public-repo.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/architecture/AGENTS.md
[mission-contract]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L100-L140
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/recurse_attempt.go#L55-L139
[sanitize]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/utils.go#L51-L86
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/cmd/nerd/cmd_campaign_recurse.go#L225-L246
[cli-set]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/cmd/nerd/cmd_campaign.go#L407-L437
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/cmd/nerd/chat/campaign_recurse.go#L218-L265
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/orchestrator_lifecycle.go#L13-L59
[owner]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/campaign_owner.go#L12-L76
[close]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/orchestrator_control.go#L78-L102
[cleanup]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/recurse_attempt.go#L256-L285
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/campaign_fact_sync.go#L101-L170
[kernel-retract]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/core/kernel_facts.go#L912-L960
[snapshot-layout]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/orchestrator_lifecycle.go#L68-L96
[journal-layout]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/orchestrator_journal.go#L31-L33
[outer-execute]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/recurse_cycle.go#L1284-L1326
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/recurse_cycle.go#L1886-L1919
[canonical]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/tools/workspace_guard.go#L58-L88
[run-root]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/recurse_cycle.go#L356-L389
[fixture]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/internal/campaign/recurse_soak_test.go#L181-L226
[prior-generation]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/audits/campaign-generation-admission-and-recovery.md
[prior-scheduling]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/audits/campaign-recurse-scheduling-ownership.md
[prior-compensation]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/audits/recurse-compensation-write-ownership.md
[prior-index]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/audits/recurse-index-adoption-identity.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/audits/recurse-mission-recovery-receipt-census.md
[prior-census-cleanup]: https://github.com/theRebelliousNerd/codenerd/blob/7ec32947fcd6b372c1153851a4455a9296699bda/Docs/audits/recurse-mission-recovery-receipt-census.md#L115-L130
[defer]: https://go.dev/ref/spec#Defer_statements
[glob]: https://pkg.go.dev/path/filepath#Glob
[match]: https://pkg.go.dev/path/filepath#Match
[remove]: https://pkg.go.dev/os#RemoveAll

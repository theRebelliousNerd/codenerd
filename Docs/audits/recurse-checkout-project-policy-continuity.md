---
doc-class: deep-dive
subsystem: campaign-recurse-project-policy
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 0aa0012e5c64080386e3ea47f8ed4ace236c8297
verification-method: static-source-inspection
supersedes: []
---

# Recurse must bind project policy after selecting its checkout

## Finding and decision

**Recurse can start its work on one committed branch while retaining another branch's project protection document.** `RunRecurseCycles` loads `nerd.md` into `r.doc` before `recurseGit.prepare` can switch to an existing `nerd/recurse` branch. The driver then derives the graph and detects gates from the selected checkout, but its outer forbidden-path check continues using the earlier document. The captured pointer is never rebound during that run. A clean, ordinary branch switch is enough; no concurrent writer, corrupt journal, failed read or model decision is needed to create the mismatch. [Startup][startup] [Branch selection][prepare] [Fresh pass inputs][pass] [Forbidden-path consumer][forbidden]

The directly established defect is **mixed project-policy provenance at the production driver boundary**. In the conditional witness below, the selected branch forbids a file, yet the outer ratchet's producer supplies no forbidden-path fact for it because the entry branch did not contain that rule. This does not establish that every complete model/campaign path will perform the edit or adopt it. Independent permission, lease, review and verification controls remain authoritative. Their outcomes were not executed here.

The correction is an explicit checkout-to-project-policy admission boundary before attempts begin. Bind the driver's document, shared policy facts, gate declarations and worker context to one admitted checkout/project-document generation, or stop with a clear policy-generation mismatch. Do not repair only the local pointer while silently leaving the executor on earlier facts, and do not treat a branch switch as authorization to weaken an owner's active restrictions.

This is a source-supported, conditional finding, not a reproduced incident. No test, build, gate, repository program, model call, inference run, benchmark or failure injection was performed. No repository file, credential or setting was changed. Only public sources, existing test definitions and primary Git documentation were inspected; source copies and this standalone report are the local work products.

## Evidence boundary and architectural obligation

The inspected public main is [`0aa0012e5c64080386e3ea47f8ed4ace236c8297`][pin], resolved on November 4, 2026 and rechecked unchanged at 01:17 UTC. Relevant source artifacts were read at that commit and matched to its Git blob identities. All CodeNERD references below use the immutable commit. Root and campaign guidance, the campaign architecture, mission specification, recurse journey, current audit inventory and relevant earlier reports were inspected. [Root guidance][root] [Campaign guidance][campaign-guidance] [Architecture guidance][architecture-guidance] [Campaign architecture][campaign-architecture] [Mission specification][mission-spec] [Journey][journey]

The root contract requires revision-aware context and says stale evidence must not survive a source change as current truth. Project-document code treats forbidden paths as enforced machine facts, rather than optional prose. The recurse policy explicitly defines `recurse_ratchet_forbidden` as an observation that the attempt changed a path forbidden by `nerd.md`. These obligations require more than a valid parse at an earlier checkout. [Root guidance][root] [Project fact contract][project-facts] [Ratchet observation contract][policy-contract]

This review does not decide that every restriction in an entry branch should disappear when another branch is selected. Preserving an invocation-level owner restriction may be correct. What is missing is the decision and provenance: the current code neither admits a deliberate composite policy nor identifies the selected branch's additional restrictions before continuing. A repair must preserve explicit owner constraints while ensuring that the selected project document is not silently treated as current for commands and irrelevant for protection.

## 1. The production source trace

### 1.1 The document is captured before branch preparation

`RunRecurseCycles` canonicalizes the requested workspace, chooses the configured branch or default, then calls `projectdoc.Load(root)`. It stores the returned document in `recurseRun.doc`. Only afterward does it acquire the recurse lock, settle any in-flight record, and call `r.git.prepare(ctx, branch)`. [Startup][startup]

`Load` reads bytes from the document selected by `Find`, parses them and returns a new `Document`. It is not a live filesystem view. `Find` prefers root `nerd.md` over the supported `.nerd/nerd.md` fallback. A missing document returns nil. Neither the pointer nor its parsed `Spec.Forbid` changes merely because Git replaces the file later. [Load contract][load]

The lock is useful for keeping a second recurse run on the same workspace from interfering. It does not move the earlier document read after branch selection or attach source identity to the document. The primary witness is single-process and needs no lock race. [Run lock][startup] [Lock implementation][lock]

### 1.2 A successful clean startup can change tracked project configuration

`recurseGit.prepare` verifies that Git recognizes the checkout and rejects tracked uncommitted changes. It returns when the current branch already equals the chosen branch. Otherwise it checks whether the branch exists; if it does, it executes `git switch --quiet <branch>`. Only the absent-branch case creates a new branch from the current checkout. [Branch selection][prepare]

The existing-branch case is the relevant one. Git documents that switching branches updates the working tree and index to the target branch. A clean status establishes no tracked differences from the current `HEAD`; it does not establish equality with the target branch's committed files. A valid `nerd.md` can therefore be replaced by a different valid `nerd.md` during a successful, non-racing startup. This is normal Git behavior, not a Git error. [Git switch](https://git-scm.com/docs/git-switch) [Git status](https://git-scm.com/docs/git-status)

The witness does not need a stale in-flight attempt. With no in-flight file, settlement returns immediately and the ordinary branch preparation path remains reachable. It also does not need a custom branch configuration: the default `nerd/recurse` branch can already exist from earlier owner-controlled work. [No-flight return][settlement] [Default branch][default-branch]

### 1.3 Graph and gates observe the selected checkout

After preparation and journal replay, `passes` enters `pass`. The pass derives the workspace DAG and calls `gates.Detect`. Graph derivation itself calls `projectdoc.Load(abs)` before its source scans; `Detect` separately calls `projectdoc.Load(root)` before composing declared and detected gates. Those are later file reads, so under the stable-after-switch witness they consume the selected branch's document. [Pass sequence][pass] [Graph document read][graph-load] [Gate document read][gate-load]

The North Star helper similarly calls `LoadAll` when composing an attempt's direction. It does not update `r.doc`; it returns text. Thus a successful fresh document read by another consumer is not evidence that the ratchet's retained protection object was refreshed. [North Star consumer][northstar]

The driver now has two different sources of project meaning in one run:

| Consumer | Document acquisition in the inspected path | Conditional source |
|---|---|---|
| Outer changed-path protection | `r.doc` captured before preparation | Entry branch A |
| DAG forbidden-path filtering | Fresh `Load` during graph derivation | Selected branch B |
| Declared gate commands | Fresh `Load` in `gates.Detect` | Selected branch B |
| Attempt North Star text | Fresh `LoadAll` in `northStar` | Selected branch B |

This table describes an ordinary stable two-branch scenario. It does not assume that every field must be reloaded on every helper call, nor that repeated parsing is a sound long-term consistency design.

### 1.4 The retained document controls an actual ratchet veto

After an executor returns changes, `run` constructs `ratchetInput` using `Forbidden: r.forbidden(changed)`. `forbidden` independently protects `.git` and `.nerd`; for all other paths it consults only `r.doc.ForbidsPath`. It does not reload the document or check that its source belongs to the current checkout. [Outer input][outer-input] [Forbidden-path consumer][forbidden]

`ForbidsPath` checks the parsed document's rules. The policy bridge emits `recurse_ratchet_forbidden(Cycle, Path)` for the returned paths. Mangle derives the refusal from those facts. If the host omits a selected-branch rule before this projection, the refusal rule cannot infer the missing policy source on its own. This is an observation/admission defect, not an argument for moving the final decision out of Mangle. [Rule matcher][matcher] [Fact bridge][bridge] [Refusal and keep rules][policy]

The exact source-level result is therefore checkable without making a model produce a bad patch: after A's document is captured and B is selected, querying the retained matcher for a path protected only by B returns no match. A fresh B document would match it. The additional consequence of a keep requires the independent conditions in the witness below.

### 1.5 The normal entrypoint can also have earlier shared project facts

The CLI obtains or boots its cortex before it calls `RunRecurseCycles`. The factory's project-document loader reads `LoadAll`, projects each document's facts, loads them into the kernel, and saves a root project-document pointer for worker context. Project facts include canonical commands and forbidden-path rules. The supplied kernel is then passed into the recurse driver. [CLI order][cli] [Factory publication][factory] [Fact projection][project-facts]

This is not proof that every pre-existing cortex has exactly A's facts. It establishes a concrete cold-start arrangement in which the ordinary boot happens on A, followed by the driver's branch switch to B. The inspected recurse startup does not republish those project facts after the switch. Chat likewise supplies its existing kernel to the shared driver, so simply changing the driver's local pointer would not establish coherent project authority across that route. [CLI order][cli] [Chat entry][chat] [Startup][startup]

The earlier incomplete-discovery audit already warns that a subsequent successful document scan is not the same as republishing project policy. That general concern applies to the correction here, but the present trigger needs no discovery failure: both A and B are fully readable and valid. [Prior discovery audit][prior-discovery]

## 2. Conditional witness

### Preconditions

Prepare a future, isolated owner-controlled Git fixture with two already committed branches:

- **A:** the clean checkout at command entry. It has a valid root `nerd.md`; its rules do not forbid `pkg/locked.go`.
- **B:** the existing default `nerd/recurse` branch. It has a valid root `nerd.md` that adds `forbid: [{match: pkg/locked.go, reason: owner-maintained implementation}]`.
- Both branches contain an admitted ordinary package `pkg`, including `pkg/locked.go` and another source file. B's rule is file-specific, not a ban on the whole package directory.
- No document in another scope independently supplies the same protection on A. No external file edit, branch change or competing recurse process occurs after startup.
- The checkout is clean, no in-flight record exists, the journal does not introduce another blocker, the required toolchain is present, and startup otherwise succeeds.
- Use ordinary non-mission recurse for the primary boundary. Mission protections are additional controls, not assumed absent or defeated.

The YAML fragment is illustrative fixture data, not a file created or parsed by this review. A full future fixture must satisfy the actual `nerd/v1` schema. No private repository content is needed.

### Primary observation: mixed source and missing current restriction

1. At entry, `projectdoc.Load` reads A and `r.doc` retains A's parsed document.
2. `prepare` sees a clean checkout on A, finds B and successfully switches to B.
3. Graph and gate discovery read B. The package `pkg` can remain a node because the rule protects a particular file rather than the package directory. In the Go path, graph admission checks the package's directory against forbidden paths, not each member source file. [Go graph admission][go-graph]
4. The on-disk document now protects `pkg/locked.go`.
5. `r.forbidden([]string{"pkg/locked.go"})` supplies no project-document prohibition because it reads A's retained rule set. The independent `.git`/`.nerd` protections do not apply to this ordinary source path.
6. A fresh `projectdoc.Load(root).ForbidsPath("pkg/locked.go")` would find B's rule under the stated stable/readable conditions.

Steps 1–6 are the narrow deterministic boundary to establish first. No output capture, model call, failing test or adopted commit is necessary to distinguish the current and intended observations.

### Conditional outer-cycle consequence

For a later authorized fault-injection evaluation, let a deterministic executor return an owned candidate that repairs its selected ordinary finding and also changes `pkg/locked.go`. The incidental protected-file edit is deliberately invalid under B's policy. It is an erroneous candidate for the outer boundary to reject, not a recommendation to make such an edit in a real workspace.

Stipulate that the candidate reaches the outer ratchet, its target is genuinely resolved, checked gates do not regress, guarded metrics do not regress, and all independent execution/adoption prerequisites succeed. Under those conditions, the missing forbidden fact removes this particular refusal reason; the remaining ratchet rules can derive keep. The adoption branch then attempts its normal Git commit. [Outer input][outer-input] [Target evaluation][target] [Policy][policy] [Adoption][adoption]

This report does **not** establish that the real session worker would make the edit, that all independent guards would allow it, that an actual commit would succeed, or that a mission would falsely certify its North Star. The guaranteed source inconsistency precedes those questions. A shared policy that is correctly rebound to B could block the candidate earlier; that is a desired defense, not evidence that the stale outer observation is correct.

### Directional controls

- Already on B: the startup load reads B, so this branch-switch mismatch is absent.
- B does not exist: creating it from the current commit does not by itself change document bytes.
- A and B have identical relevant project policy: the pointer is earlier, but the stated differing-rule witness does not apply.
- A lacks a document and B introduces one: the retained nil pointer makes the same missing-rule issue particularly direct.
- A forbids a path that B permits: the outer pointer preserves the earlier restriction. That may be appropriate if the owner intended invocation-level protection; it must not be silently reclassified as an error or automatically removed. The product needs an explicit conflict/continuity contract.
- B's whole package directory is forbidden: fresh graph filtering may remove the node, preventing the specific file-level candidate used here. Do not use that case to claim reachability.
- B is malformed or unreadable: graph construction discards the `projectdoc.Load` error (`doc, _`), while later gate discovery can fail on that error. These are separate behaviors; the primary witness intentionally uses two valid documents. [Graph load][graph-load] [Gate load][gate-load]

## 3. Nonduplication screen

The current audit inventory and queue, relevant recurse reports, and project-document reports were screened. Targeted searches for `git.prepare`, `r.doc`, `ForbidsPath`, and branch-switch/project-document combinations found the production path and neighboring reports, not this two-valid-branch policy capture. This is a bounded public-main screen, not a claim to have exhausted every historical branch, private discussion or sentence.

| Prior work | Existing boundary | Distinction here |
|---|---|---|
| [Project-document logical root identity][prior-root] | A supported fallback document is given the wrong logical role by selected consumers. It notes that `r.doc` is loaded directly. | Both documents here use ordinary root `nerd.md`; the direct load succeeds but occurs before the checkout changes. |
| [Incomplete discovery policy][prior-discovery] | A failed module walk can publish incomplete policy; later scanning does not prove policy recovery. | Neither parse nor discovery fails. The driver itself switches the committed source after taking a valid policy snapshot. |
| [Resumed visit applicability][prior-visit] | A completed-visit ledger entry remains eligible for reuse after committed source changes. | No recovered completed visit, changed graph, stale skip or interrupted pass is needed. The mismatch exists in a fresh startup. |
| [Recovery attempt identity][prior-recovery] | A historical commit trailer is matched to the wrong uncertain attempt. | No in-flight record, reused cycle, foreign trailer or adoption acknowledgement exists. |
| [Finding owner graph transition][prior-graph] | Retained findings name the prior graph generation. | The first graph and first gate measurements may be entirely fresh. The older object is project protection. |
| [Gate contract continuity][prior-contract] | Selected command environments and accepted exits must survive the attempt handoff. | B's gate descriptor can be copied perfectly; the prohibition consumer independently remains on A. |

The [portable recurse/deliberation proposal][portable] already motivates source-bound authority, evidence and coherent contracts. This report contributes the concrete startup transition and an observable mismatch; it does not claim to invent that general architecture.

The existing Git preparation test checks branch creation, dirty-checkout refusal and non-repository refusal. The existing outer-loop forbidden-write fixture verifies refusal when the loaded document already contains the rule. These are relevant controls, but neither inspected fixture composes an existing divergent branch switch with a new project restriction. They were read, not run; no repository-wide claim about absent coverage is made. [Preparation fixture][prepare-test] [Forbidden-write fixture][forbidden-test]

## 4. Proposed contract

Update the campaign/project-document specifications before implementation. The admission contract should distinguish:

1. **Invocation constraints:** explicit owner restrictions, authorization, configured workspace and any intentionally pinned policy that must survive a checkout change.
2. **Selected checkout:** canonical workspace and the actual selected source/branch state after safe recovery and preparation.
3. **Project-document inventory:** root/fallback/module roles, content identities, completeness and the selected checkout to which they apply.
4. **Published runtime:** the kernel facts, driver matchers, gate descriptors and worker context derived from that admitted inventory, plus any separately identified invocation constraints.

A run must not enter mutable attempts when those identities conflict or when the relevant project inventory is incomplete. Host code should observe the identities, read/parse outcomes and publication results; Mangle should own the admission, hold and refusal decisions.

### Preserve recovery and authorization ordering

The existing settlement-before-cleanliness order has a reason: a killed attempt can leave changes the loop must reconcile. Do not move branch switching ahead of recovery merely to make the document read later. Define the whole startup transition and establish checkout/policy coherence **after** required settlement and selection, before fresh attempts. A failed or ambiguous recovery still holds the run.

For a cold CLI start, establish how branch selection and policy-dependent boot are coordinated under one lifecycle owner. For an already-running chat cortex, either safely rebind the project-owned runtime at a quiescent boundary or explicitly require a fresh admitted runtime. Do not globally retract all project facts while another turn is using them, mix old/new facts during publication, or treat a new orchestrator as proof that the shared kernel was refreshed.

If a selected branch weakens a previously active restriction, do not silently drop an owner constraint. Carry it as an explicit invocation constraint when that is the product's intended semantics, or request the necessary decision. Conversely, do not omit additional selected-project protections merely because the earlier invocation did not know them. A conservative mismatch hold is preferable to inventing authority from either document alone.

### Bind one coherent policy snapshot, not many unrelated rereads

Moving `projectdoc.Load` below `prepare` is a useful local correction to investigate, but insufficient as an end-to-end contract. Shared boot facts and worker context also need admitted provenance. Repeatedly reopening the file in `forbidden` would still permit mid-run policy changes to be accepted without an explicit transition.

Prefer a versioned, complete project-policy snapshot supplied to participating consumers. Validate that its source/configuration remains applicable at the relevant boundary. Keep independent `.git`/`.nerd` protections and all existing permission/lease/check rules. Source mutation can invalidate the snapshot; it must not silently authorize new writes.

Record only the identities and decisions needed for diagnosis: entry and selected source identity, admitted document inventory/digest, transition disposition, and the reason for a hold. Do not log raw environment values or private document contents merely to prove equality. Durable records must distinguish old historical policy from currently admitted policy; no migration should manufacture a missing generation identity.

## 5. Fair unrun evaluation

### First establish the source boundary without model behavior

Create a temporary repository with ordinary branches A and B and two valid differing root documents. Use the normal preparation and driver construction, with fixed stubs only where necessary to observe admission without running a model. Capture the branch/document identities at entry, after selection, before graph/gate observation and before the first executor dispatch.

The minimal failing-before assertion is: **a run must not dispatch mutable work with B's selected checkout and gate declarations while its outer protection source remains A without an explicit admitted composite-policy record.** A helper-only `ForbidsPath` test is not sufficient; it must cross the actual branch transition.

Then use a deterministic fault-injection executor and real shipped policy to expose the outer refusal difference. Supply recorded owned writes, a correctly resolved target and controlled genuine check observations. Verify that the B-protected edit is refused/compensated or the run is held before execution. Keep a permitted repair as a positive control. Do not call these injected-executor observations full session qualification.

### Then exercise production adapters and independent controls

Run the normal CLI and chat paths, including real project-fact loading, worker/tool permissions, campaign write observation, acceptance checks and the outer ratchet. Use a benign project with fixed behavior oracles; the checker must actually distinguish a correct repair from an incorrect one, rather than always exiting zero. With ordinary world-model/CodeDOM context enabled, verify the active document identity and protection facts seen by each layer. Optional model studies should come only after deterministic host behavior is established and under separate execution authorization.

Required controls, all unrun:

| Scenario | Required outcome |
|---|---|
| Existing B adds file-level protection | B's protection is effective or startup holds before effects. |
| A has no root/fallback document; B adds one | No nil-as-current-policy continuation. |
| A and B have different valid command declarations | Executed gate and worker/runtime policy belong to the admitted generation. |
| Already on B; identical documents; newly created B | No unnecessary hold or behavior change. |
| B removes an earlier owner restriction | No silent weakening; explicit approved continuity semantics. |
| Root/fallback precedence changes; module policy changes | Logical roles and full inventory stay coherent, not only one root file. |
| Read/parse/publication failure after switch | No partial-policy continuation or false claim that the new policy is active. |
| No in-flight work versus recoverable/ambiguous in-flight work | Preserve safe recovery ordering and hold on unresolved ownership. |
| A real permitted repair | Normal checks can succeed and adoption remains possible. |
| Protected candidate, forbidden reserved path, failed/unverified check | Existing independent refusals remain intact. |
| Chat has another active consumer of shared project facts | No partially replaced policy or interference with the other turn. |
| Cancellation before, during or after publication | Honest terminal state; no leaked partially admitted runtime. |
| Source changes again after admission | Detect invalidation rather than reusing the old identity as current. |
| Windows and POSIX; supported fallback paths | Equivalent semantic policy despite path spelling differences. |

Compare the pinned baseline, a local-pointer-only adjustment, and the proposed coordinated admission design on identical branch contents, checks and permission settings. The pointer-only arm is important: it may close the outer symptom while leaving earlier runtime consumers unchanged. Include unchanged-policy projects so the evaluation measures unnecessary holds and startup costs, not just favorable divergent fixtures.

Report mixed-generation dispatches, missed selected-project prohibitions, unjustified loss of owner restrictions, false holds, protected/allowed write outcomes, recovery regressions and publication failures separately. For later model studies, distinguish fewer invalid attempts from merely blocking all work; retain model/version, budgets and task populations across arms. No success rate, latency improvement, token saving or incident frequency is claimed by this review.

## 6. Costs, failure modes and stopping condition

Reading and hashing an admitted document inventory costs work proportional to its bytes, plus discovery and membership work. Reusing a snapshot requires an explicit validity key. Coordinating shared policy publication may add a startup or transition pause; rebuilding a full cortex can be materially more expensive than replacing one local pointer. Measure that cost rather than describing reinitialization as free. No extra inference, embedding or new agent layer is intrinsically required for the narrow check.

Principal repair risks are publishing only part of the new inventory, treating a read error as document absence, retaining old forbidden facts beside new commands without a declared merge rule, dropping prior owner restrictions, updating shared facts beneath another task, and fixing CLI while leaving chat on its earlier runtime. A stale `HEAD` value alone also does not identify uncommitted document bytes. Keep checkout identity and content identity distinct.

The bounded closure criterion is: after a supported branch transition, every policy-dependent recurse consumer either uses the admitted project-policy generation and explicit retained owner constraints, or refuses to proceed with a precise mismatch. The two-branch witness must no longer omit a selected-project prohibition, permitted controls must remain usable, and recovery/cancellation must remain sound. Passing that boundary does not establish the entire portable recurse design or a general guarantee against all concurrent repository mutation.

## Pinned references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/0aa0012e5c64080386e3ea47f8ed4ace236c8297
[root]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/architecture/AGENTS.md
[campaign-architecture]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/architecture/campaign/INTERNALS.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/journeys/10-forever-loop.md
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L356-L447
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_git.go#L50-L77
[default-branch]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_git.go#L24-L25
[load]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/projectdoc/nerdmd.go#L255-L290
[lock]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_control.go#L15-L63
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L1786-L1809
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L515-L572
[graph-load]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_workspace.go#L64-L103
[go-graph]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_workspace.go#L410-L436
[gate-load]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/gates/detect.go#L15-L43
[northstar]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L1626-L1663
[outer-input]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L1324-L1361
[forbidden]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L1738-L1754
[matcher]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/projectdoc/nerdmd.go#L623-L645
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_policy.go#L205-L255
[policy-contract]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/core/defaults/policy/recurse.mg#L27-L40
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/core/defaults/policy/recurse.mg#L296-L329
[target]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L1421-L1446
[adoption]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle.go#L1487-L1514
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/cmd/nerd/cmd_campaign_recurse.go#L178-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/cmd/nerd/chat/campaign_recurse.go#L194-L279
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/system/factory.go#L1551-L1607
[project-facts]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/projectdoc/facts.go#L26-L107
[prepare-test]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_git_test.go#L60-L78
[forbidden-test]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/internal/campaign/recurse_cycle_test.go#L200-L227
[prior-root]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/audits/projectdoc-logical-root-identity.md
[prior-discovery]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/audits/projectdoc-incomplete-discovery-policy.md
[prior-visit]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/audits/recurse-resumed-visit-applicability.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/audits/recurse-recovery-attempt-identity.md
[prior-graph]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/audits/recurse-finding-owner-graph-transition.md
[prior-contract]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/audits/recurse-gate-contract-continuity.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/0aa0012e5c64080386e3ea47f8ed4ace236c8297/Docs/audits/portable-recurse-deliberation-crucible.md

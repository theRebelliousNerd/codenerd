---
doc-class: deep-dive
subsystem: campaign-mission-recovery
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 8fa638876aa474ef4d0111f7f9dc1634c609f430
verification-method: static-source-inspection
supersedes: []
---

# Mission recovery admission needs a literal, complete receipt census

## Finding and decision

**Mission startup treats a successful `filepath.Glob` result as a complete search for unsettled mission receipts. It is not that contract.** The pattern embeds the canonical workspace directory without separating literal directory names from pattern syntax, and Go's glob operation suppresses directory-read errors. A valid bracket-containing workspace can consequently miss its own pending receipt; an unreadable historical session directory can produce the same false absence. Both become a successful return from `checkInterrupted`. [Receipt discovery][discovery] [Directory construction][layout] [Go Glob contract][glob]

This is a recovery-admission correctness and portability issue. It is not a new requirement for automatic replay, another mission scheduler, or a claim that every filesystem error is ignored. Errors opening or decoding a receipt **after it was found** already stop admission. The missing guarantee is that all relevant receipts were found in the right workspace first.

There is a source-composed restart witness in which the second in-flight guard does not rescue the omission. A hard interruption after a ratchet record has been appended, but before `Mission.finish`, leaves the mission's pending receipt unsettled. On a later start, ordinary settlement removes the matching in-flight marker before its mission-specific refusal because the ratchet row exists. With failed receipt discovery, the new mission can proceed without reconciling the older mission receipt. [Publication order][publication] [In-flight settlement][settlement] [Startup order][startup]

**Recommended bounded change:** specify and implement literal-root, error-reporting enumeration of the owned session/mission namespace. Admission must distinguish complete-and-empty, complete-with-pending, incomplete and invalid. Preserve owner-led reconciliation and the current protections against replaying uncertain effects. Fixing filename quoting alone does not repair suppressed I/O failures.

**Evidence limit:** conditional deductions from source, not a reproduced incident. No tests, builds, application runs, gates, model inference, benchmarks, fault injections, credential accesses, settings changes or repository writes were performed. Only public-source copies and this standalone report were produced. All evaluations below are proposed and unrun.

## 1. Source boundary and intended behavior

Public `main` resolved to [`8fa638876aa474ef4d0111f7f9dc1634c609f430`][pin] on November 4, 2026, and was rechecked unchanged at 02:17 UTC. The recursive repository tree, root guidance, architecture guidance, campaign guidance, campaign internals, mission contract and recurse journey were inspected before selecting the finding. Local copies of the inspected repository files were checked against their Git blob hashes; source links below are immutable. [Root guidance][root] [Architecture guidance][arch-guidance] [Campaign guidance][campaign-guidance] [Campaign internals][internals] [Recurse journey][journey]

The current mission specification makes three distinct commitments:

- Retain exact mission intent, source/evidence identity, observed outcome and uncertainty durably.
- Do not automatically replay or report an interrupted mission accepted.
- Reconcile a pending receipt against the recurse journal before additional effects; inspect ambiguity rather than infer success.

It explicitly says pending intents are checked across session folders before a new mission starts. The implemented scanner is therefore an admission boundary, not a best-effort history browser. [Mission recovery contract][contract]

The broad architecture remains appropriate: models propose designs, while host observations and the kernel govern execution. Better deliberation cannot reveal a receipt omitted before the new mission is created. This repair belongs at durable-state discovery and its admission consumer, not in a larger prompt or a model-generated assurance sentence.

## 2. Implemented production path

### 2.1 Both mission doors reach the same admission scanner

The CLI constructs a `RecurseMission` and supplies it to `RunRecurseCycles`; chat does the same with its configured workspace. The shared driver canonicalizes that workspace, loads the project document, acquires the workspace recurse lock, and calls `Mission.begin` before reading/reconciling the recurse journal. [CLI][cli] [Chat][chat] [Startup][startup]

`begin` checks the deliberator and workspace, configuration, kernel parameters and session ID. It then invokes `checkInterrupted`. Only after that succeeds does it acquire the background-quiescence lease, allocate a fresh mission ID, save the new state, and obtain admission under its finite resource envelope. These are real guards, not missing machinery. [Mission begin][begin]

`CanonicalWorkspaceRoot` resolves an absolute filesystem identity through symlink resolution. It does not encode directory names for a later pattern language. `SessionsDir` then joins the actual root with `.nerd/sessions`; it does not move receipts into a punctuation-free global cache. [Canonical root][canonical] [Session layout][layout]

### 2.2 Discovery places literal root bytes inside a glob

The scanner constructs:

```text
<workspace>/.nerd/sessions/*/missions/*.json
```

with `filepath.Join`, then sends the whole string to `filepath.Glob`. It returns an error only if that call returns an error, otherwise iterates the returned paths. If no path is returned, the method returns nil. [Receipt discovery][discovery]

For paths that *are* returned, it opens the file, reads it, checks close/read errors, decodes JSON, validates a nonempty session and filename-compatible mission ID, and refuses any nonnil `Pending`. Those protections are useful and must remain. They are downstream of enumeration and cannot assess a receipt absent from its output.

Go documents brackets as character-class pattern syntax and states that Glob ignores filesystem errors such as failed directory reads; its returned error is for malformed patterns. Thus checking `err` from Glob is not evidence that the required directories were successfully enumerated. Canonicalization and path joining do not change that API meaning. [Go matching rules][match] [Go glob contract][glob] [Go glob implementation][glob-source]

### 2.3 Mission completion spans separate durable records

Preparation saves a pending receipt before design execution, and later saves its outcome as `executing` before returning the admitted attempt. Owned-write and check sidecars are attached through further mission saves. [Pending publication][pending] [Executing publication][executing] [Write sidecar save][sidecars] [Check sidecar save][check-sidecars]

On a kept attempt, the driver commits and sets a kept ratchet record. It then retires temporary policy facts, records the cycle observation and appends the ratchet journal row. Only afterward does it call `Mission.finish`. That method obtains the evidence disposition, appends the completed receipt, clears `Pending` and saves the mission snapshot. The in-flight file is removed still later. [Keep][keep] [Publication order][publication] [Mission finish][finish]

This arrangement creates a legitimate interruption cut: journal acknowledgement can exist while the mission snapshot still has its earlier pending intent. The specification's cross-session discovery is necessary even when Git adoption itself is known.

### 2.4 The in-flight guard has a different responsibility

The driver persists the mission snapshot path in the in-flight marker before recording the attempt. [Mission-marked in-flight publication][flight-publication] `settleInFlight` first scans journal records. A same-cycle ratchet row causes it to remove the in-flight file and return immediately. Its mission-marker refusal is checked only after that branch. It does not open, finish or reconcile the referenced mission snapshot in the ratchet-present branch. [Settlement][settlement]

That is not, by itself, evidence that the original commit is wrong. The ratchet journal may accurately establish adoption. It is evidence that this helper is **not an independent full mission-receipt census**. A missing discovery result cannot be excused by asserting that every mission marker always blocks restart.

## 3. Minimal conditional witness: bracket-containing workspace

The following is a hypothetical fixture, not an executed reproduction.

Use an owner-controlled, otherwise valid Git workspace whose **physical canonical directory** ends in `repo[1]`, with no sibling directory named `repo1`. Keep its runtime `.nerd` artifacts outside tracked source changes in the ordinary supported way. Stipulate successful project loading, kernel setup, quiescence and existing admission checks. No symlink, hostile contents, changed permission, external writer or ambiguous commit is required.

1. **Mission A reaches a real kept ratchet.** Its admitted repair, executable verification, ownership checks and Git commit succeed. Its mission snapshot is still pending with outcome `executing`, and its write/check evidence has been saved.
2. **A's ratchet row becomes readable on disk.** The process is then hard-interrupted before `Mission.finish` updates the mission snapshot. This is a process interruption, not the graceful cancellation path whose deferred cleanup runs. Require the row and pending snapshot to be intact; torn journal recovery is not part of this witness.
3. **A new explicit mission B starts in the same workspace.** It acquires the normal workspace lock. The original process is gone, and the kept commit leaves a clean tracked checkout.
4. **The scanner looks in a pattern-derived directory.** In the composed glob, `repo[1]` selects the spelling `repo1`, not the literal directory `repo[1]`. With no such sibling, the result is empty and no pattern error occurs. A's valid pending receipt is never opened.
5. **B passes this recovery check.** Its own fresh mission state is saved and its finite envelope admitted, conditional on the other existing checks succeeding.
6. **Ordinary in-flight settlement finds A's genuine ratchet row.** It removes the old in-flight marker via its first branch, before the mission-specific refusal, and does not reconcile A's mission snapshot.
7. **The driver can continue into B's normal sweep.** A's old pending receipt remains unresolved on disk. B does not automatically replay A, inherit A's evidence, or become exempt from its own permissions and verification.

The precisely supported defect is **new mission admission despite an existing unreconciled mission receipt that the documented startup check should have found**. The example does not claim a duplicate write, an incorrect original commit, a shared-budget overrun, a false successful mission, data loss or a bypass of B's independent write guards.

### Useful controls

- A normal literal root without pattern metacharacters discovers the same pending receipt and refuses admission.
- A crash **before** the ratchet row is available retains the separate mission-marker refusal, even if the census misses A. That is a real defense and is outside the primary cut.
- A completed mission with `Pending == nil` does not block a fresh mission merely because historical receipts exist.
- A malformed bracket sequence in the physical root can produce a pattern error and an unnecessary startup refusal rather than false absence. That is a portability failure with a different outcome.
- If a matching sibling `repo1` exists, its receipts can be inspected instead of the intended root's receipts. A pending sibling record can impose a false hold. This needs the stated sibling and valid record, not merely brackets.
- A symlink spelling containing brackets whose canonical target does not contain them is not the physical-root witness.

These controls keep the claim tied to exact path identity and durable state rather than treating all unusual filenames as equivalent.

## 4. Second witness: incomplete enumeration with ordinary names

The same admission contract fails without metacharacters. Consider a normally named root with two session directories. Historical session S contains A's pending mission; a new writable session T is available for B. Directory enumeration of S or its `missions` directory fails with an I/O or access error, while the root, current session, journal and new-state path remain usable.

Glob may return only the other accessible matches, with nil error. If they contain no pending receipt, `checkInterrupted` returns nil. The returned-file read checks do not fire because S's record never entered the result set. The same ratchet-present interruption state avoids the later in-flight refusal. [Discovery][discovery] [Settlement][settlement]

This is a conditional filesystem-failure witness, not a claim that every access error allows a complete mission to run. If the same failure also prevents project/kernel loading, lock acquisition, journal reading or saving B, those independent failures stop the route. Likewise, an already enumerated JSON file whose `Open` fails currently produces the correct error. The missing observation is a failed **directory census**, not generic exception handling.

A future permission fixture must actually deny access under its executing identity; a root-privileged test that still reads a chmod-restricted directory would not establish this case. A deterministic filesystem seam can separately test I/O error propagation without relying on local privilege behavior.

## 5. Distinctness and existing evidence

The complete current audit-path inventory and queue were screened. Targeted repository searches for `checkInterrupted`, `mission filepath.Glob`, and `unsettled cycle` identified the scanner, its caller and related material, not a dedicated report of this admission census. This is a bounded current-main novelty claim, not an assertion about every private discussion or historical branch. [Audit queue][queue]

| Neighboring work | Existing boundary | Distinct issue here |
|---|---|---|
| [Recurse recovery attempt identity][prior-recovery] | Ordinary recovery equates local cycle numbers with repository-wide commit identity; its primary witness explicitly excludes mission mode. | A's ratchet and commit are genuine and correctly attributed. The pending mission is missed before reconciliation. |
| [Recurse Git literal-path adoption][prior-literal] | Exact changed filenames become Git pathspecs during staging/commit. | The literal object here is the workspace root, the interpreter is Go Glob, and the consumer authorizes mission startup. Git literal-mode changes do not repair it. |
| [Recurse journal replay coverage][prior-journal] | Incomplete journal replay can manufacture an incomplete view of history. | The journal is complete and readable. A separately stored mission snapshot is never enumerated. |
| [Project-document incomplete discovery][prior-project] | A partial project-document walk can lose live policy. | The protected data here is durable pending mission intent. The specified error suppression occurs in Glob before receipt parsing. |
| [Recurse values continuation ownership][prior-values] | Pending questions need a durable answer owner and a correct selector. | This is execution/recovery intent, not an awaiting-values question or conversational selector. |
| [Portable recurse/deliberation proposal][prior-portable] | Requires evidence-bound, portable execution and durable uncertainty. | This report gives that broader requirement a concrete existing producer, consumer, crash cut and acceptance oracle. |

The inspected `TestMissionInterruptedIntentCannotBeReplayed` is meaningful positive evidence: it persists an executing pending receipt, confirms another `begin` refuses it, and confirms a mission-marked in-flight record without a ratchet refuses ordinary recovery. Its fixture uses an ordinary `t.TempDir`; it does not compose bracket-root discovery or directory enumeration failure with a journaled ratchet. These are authored assertions that were read, not test results obtained in this review. [Fixture setup][fixture] [Existing interruption test][test]

An adjacent helper, `ReleaseRecurseAttempt`, also constructs a glob beneath an unencoded workspace root and deletes returned matches. Its normal production callers defer it for each attempt. The bracket-root shape can miss intended cleanup; any unrelated deletion claim additionally requires a matching sibling artifact with that generated campaign prefix. That stronger coincidence was not established or executed here, so it is **not** the primary finding. It is a focused follow-through site for the literal-root inventory, not justification for a speculative universal deletion claim. [Release helper][release] [CLI caller][cli] [Chat caller][chat]

## 6. Proposed contract and design choices

### Required discovery outcomes

Before creating a new mission, the host must establish one of:

1. **Complete, no pending intent:** all relevant owned session/mission entries were enumerated and admitted records were read successfully; new mission admission may continue under existing rules.
2. **Complete, pending intent found:** identify the owning mission/session and hold for the existing reconciliation path.
3. **Incomplete:** retain the exact failed directory or operation and hold; do not reinterpret absence from the returned subset as no pending work.
4. **Invalid state:** retain the invalid identity/record and report a bounded repair requirement. Do not delete or silently skip it to make admission pass.

Known absence needs its own rule. An absent sessions directory for a truly new workspace can be an empty history. An existing child directory that cannot be read is not empty. A session with no `missions` child can be legitimate; disappearance during traversal must be handled deliberately instead of being conflated with every I/O error.

### Prefer literal traversal over interpolated globbing

Read the literal sessions directory, enumerate its immediate session entries, then inspect each literal `missions` child and filter filenames there. Propagate directory read and close failures. Sort when deterministic diagnosis is useful. Reuse the existing mission-ID and JSON checks; add path/owner consistency as required by the chosen namespace contract.

The report does not prescribe a full filesystem library or recursive walk over every project file. The namespace has fixed depth, so a small, testable, error-returning helper can cover the contract. Keep symlink behavior explicit: accepting a symlinked session directory, refusing it or resolving it under an approved root are different choices, none should accidentally widen the search.

Escaping the root inside Glob is a partial alternative only. Platform differences and filesystem-error suppression remain. A platform-specific string replacement is not sufficient evidence of portable literal semantics.

### Keep discovery and reconciliation separate

This repair should not automatically mark pending missions completed merely because any ratchet row exists. The existing journal is authoritative for adoption, but mission receipts also own evidence, accounting and uncertainty. If automatic reconciliation is separately designed, require an exact owned attempt relationship, verify the relevant receipts and publish the terminal mission state idempotently before admitting new effects.

A narrower fail-closed discovery repair can preserve today's owner-inspection behavior. It need not redesign commit identity, retry old writes, remove records, extend budgets, change protected paths or relax quiescence. Those actions require their own reviewed contracts.

### Bound work without hiding history

The present scanner reads every selected JSON file in full. A replacement should expose cancellation and explicit scan/read limits if added, rather than silently admitting from a prefix. An auxiliary pending-intent index may eventually reduce startup cost, but only with transactional publication/reconciliation and a defined rebuild path. A stale empty index would recreate the same defect in another representation.

No hard scan cap, new retention policy, deletion of old records or persistent service is established as necessary by this report.

## 7. Costs and failure modes

Let S be session-directory entries, M be mission records in the required namespace, and B be their total decoded bytes. A literal census remains roughly O(S + M + B) work, comparable to the logical work already intended. It adds explicit directory-error handling and may inspect entries that the old glob silently missed. Deterministic sorting adds ordering cost; streaming/bounded reads may reduce peak memory, but need an honest incomplete outcome.

There is no inherent additional model call, token cost, embedding request, remote service or gate execution. A correct hold can prevent a new paid mission from starting while older evidence is unresolved. That is enforcement, not a measured cost saving. Operationally, the changed behavior can surface previously hidden unreadable history and require owner repair; that friction should be reported and measured rather than disguised as a performance regression.

Measure:

- startup latency, directories opened, entries examined, bytes read and peak memory;
- successful clean admissions, correct pending holds and incomplete-discovery holds;
- false holds caused by irrelevant or unsupported entries;
- time and steps to diagnose/recover a blocked receipt;
- extra mission attempts or inference caused by a misleading admission/hold, if a later authorized live study is conducted.

Preserve cancellation, no duplicate receipt settlement, no automatic replay, valid same-workspace locking, correctly attributed owned effects and normal successful mission admission. Test missing directories separately from EACCES/EIO, invalid JSON separately from incomplete enumeration, and pattern-derived sibling matches separately from literal-root history. Do not claim a faster scan by dropping old sessions or ignoring unreadable ones.

## 8. Fair proposed evaluation, not run

Use deterministic temporary workspaces and real mission/recurse state paths and policy. Replace model behavior with a fixed executor only where needed; retain a genuine behavioral gate for the prior successful repair rather than treating a generated success string as proof. Do not use providers, credentials or production user state.

### Layer A: discovery contract

| Case | Required result |
|---|---|
| Ordinary root, no session history | Clean empty history, subject to normal admission. |
| Ordinary root, valid pending receipt in an older session | Pending hold before new mission state/effects. |
| Same bytes under physical `repo[1]`, no `repo1` sibling | Same pending hold as the ordinary root. |
| `repo[1]` and `repo1` coexist with different receipts | Inspect only the literal owned root; sibling history cannot admit or block it. |
| Unmatched bracket in an otherwise supported root | Literal filesystem handling, not glob-syntax refusal. |
| Spaces and Unicode | Equivalent outcomes with exact source identity. |
| Read error enumerating old session / missions directory | Explicit incomplete hold, not clean absence. |
| Enumerated receipt cannot be opened or decoded | Preserve current fail-closed behavior. |
| Settled historical receipts | No false pending hold. |
| Missing sessions or missions child | Follow the explicit known-absence contract. |
| Cancellation partway through large history | No partial-history admission; report interruption. |
| Session-directory symlink / entry changes during traversal | Follow the declared containment and consistency rule; no silent cross-root read. |

### Layer B: real restart composition

Arrange a valid prior mission and synchronize a controlled interruption **after its ratchet record is durably observable and before mission finish**. Do not race an arbitrary sleep. Capture original mission JSON, sidecar references, journal rows, actual commit identity and in-flight marker. Restart through `RunRecurseCycles` with the same root and a fresh explicit mission.

For each root/enumeration case above, capture whether B reaches fresh-state creation, quiescence, sweep and executor; whether A remains pending; whether its in-flight marker was removed; and which exact records were consulted. A returned nil from a helper is not the end-to-end oracle.

Controls must include interruption before ratchet, normal full completion, known completed ratchet plus fully finished mission, unrelated sibling receipt, no provider/worker admission on a pending hold, and a legitimate subsequent mission after explicit reconciliation. Repeat restart to verify idempotence and no duplicate outcome publication.

Compare three arms on the same inputs: pinned baseline, literal-root-only repair, and literal complete-enumeration repair. The middle arm shows why quoting alone cannot satisfy the I/O-failure requirement. Run POSIX and Windows versions with platform-valid names. For access-error checks, verify the actual process identity sees the denial or use an injected error seam; do not count ineffective permission setup as coverage.

A solution passes only if pending/incomplete cases stop correctly **and clean ordinary missions continue**. Runtime reproduction, failure frequency, platform qualification, performance improvement and live-model impact remain unestablished.

## 9. Decision boundary

The source establishes a mismatch between a mission-recovery admission contract and its enumeration API. It supports the conditional root-spelling and directory-error witnesses, including why a journaled-ratchet restart can pass the separate in-flight guard. It does not establish that a real user experienced the issue or that any code change has fixed it.

The next authorized implementation should update the owning recovery/discovery specification, add the deterministic positive and negative controls, repair the narrow host boundary, and validate through the shared production driver. Keep model reasoning, Mangle verdicts, write permission and durable adoption as separate authorities.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/8fa638876aa474ef4d0111f7f9dc1634c609f430
[root]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/agents.md
[arch-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/architecture/AGENTS.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/agents.md
[internals]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/architecture/campaign/INTERNALS.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/journeys/10-forever-loop.md
[contract]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L185-L215
[discovery]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission_storage.go#L17-L49
[layout]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/sessionlog/layout.go#L23-L46
[canonical]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/tools/workspace_guard.go#L59-L89
[begin]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission.go#L99-L174
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_cycle.go#L356-L447
[pending]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission.go#L212-L247
[executing]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission.go#L304-L323
[sidecars]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission_storage.go#L51-L77
[check-sidecars]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission_storage.go#L103-L151
[flight-publication]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_cycle.go#L1243-L1251
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_cycle.go#L1480-L1507
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_cycle.go#L1536-L1570
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission.go#L326-L376
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_cycle.go#L1783-L1806
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/cmd/nerd/cmd_campaign_recurse.go#L211-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/cmd/nerd/chat/campaign_recurse.go#L194-L234
[fixture]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission_policy_test.go#L21-L36
[test]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_mission_policy_test.go#L160-L178
[release]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/internal/campaign/recurse_attempt.go#L256-L284
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/audits/ASTRA_QUEUE.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/audits/recurse-recovery-attempt-identity.md
[prior-literal]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/audits/recurse-git-literal-path-adoption.md
[prior-journal]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/audits/recurse-journal-replay-coverage.md
[prior-project]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/audits/projectdoc-incomplete-discovery-policy.md
[prior-values]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/audits/recurse-values-continuation-ownership.md
[prior-portable]: https://github.com/theRebelliousNerd/codenerd/blob/8fa638876aa474ef4d0111f7f9dc1634c609f430/Docs/audits/portable-recurse-deliberation-crucible.md
[glob]: https://pkg.go.dev/path/filepath#Glob
[match]: https://pkg.go.dev/path/filepath#Match
[glob-source]: https://go.dev/src/path/filepath/match.go#L223


# Tool-duplicate evidence must retain its live substitute

## Finding and decision

**A generated tool can remain classified as redundant after the only tool that justified that classification has been deliberately withdrawn.** The durable Jev verdict records the chosen substitute's name, but its replay into Mangle discards that dependency. Retirement removes the substitute and marks its own ledger retired; it does not invalidate another tool's duplicate verdict. Even asking for fresh tool-value judgments with only one generated tool remaining skips the now-empty duplicate comparison and then restores the old positive verdict from disk-backed state.

The proposed repair is **substitute-bound applicability**, not deletion of historical judgments or a new automatic retirement mechanism. Preserve the original observation, identify which current tool/capability it depended on, and derive current redundancy only while that dependency is still valid. An empty current comparison cohort is not a new model answer, but it cannot support replay of a prior claim that one of those candidates is available.

Source pin: main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), rechecked October 30, 2026 at approximately 15:13 UTC. An independent source challenge corroborated the subject-only retirement filter and the empty-cohort skip followed by positive replay. This is a conditional static-source finding, not an executed reproduction or production incident. No application, test, model inference, benchmark, filesystem failure injection or repository mutation was performed.

The directly established consequence is stale **evidence, classification and retirement proposal**. A is not automatically removed. The user must separately request retirement of A and pass the existing approval path; retained source can later support regeneration.

## Current status and nonduplication

Main, guidance and the queue remained at the same immutable pin. Active non-research code/QA tips were unchanged. The latest inspected branch, `research-main-20261030-1450`, tip `ac3013965dd2ce27d66f04ab833638ac8f4db788`, adds four documents: campaign intake pause ownership, learning-index zero-backlog migration, Northstar unchanged projection recovery, and reviewer-fix citation scope. It contains no implementation repair for this dependency. [Research tip](https://github.com/theRebelliousNerd/codenerd/commit/ac3013965dd2ce27d66f04ab833638ac8f4db788).

The existing tool-value specification deliberately persists Jev evidence, classifies tools in Mangle, and exposes retirement only as a proposal requiring approval. Keeping useful readings across restarts is intentional. The report should not reset all measurements or erase a judgment merely because a process restarted. [Tool-value specification](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/tools/IMPLEMENTED_SPEC.md#L464-L577).

Two recent generated-tool audits cover adjacent but different contracts:

- [Feedback Jev accounting idempotency](https://github.com/theRebelliousNerd/codenerd/blob/30684dc18e934f12e50db05035febe7692174692/Docs/audits/generated-feedback-jev-accounting-idempotency.md) concerns one execution being counted repeatedly during feedback repair. Here every call tally and persistence write may be correct; only a tool-level duplicate dependency becomes obsolete.
- [Retirement version admission](https://github.com/theRebelliousNerd/codenerd/blob/0b9eab6cc9f406a17bec847a84c3259e4e0cbf41/Docs/audits/generated-tool-retirement-version-admission.md) concerns a tool replaced while approval is pending. This witness has no replacement, race, or stale approval subject. B is correctly and intentionally retired; A's old justification should then cease to represent current availability.

The source check covered current autopoiesis guidance, tool-value/retirement implementation and tests, relevant specifications, main audit inventory and research additions through 1450. It does not assert exhaustive novelty across all historical research. The existing cross-process profile lock and atomic file writer are credited below; this is not another lost-update claim.

## Exact producer, persistence and consumer path

### 1. The question is explicitly about currently registered alternatives

`JudgeToolValue` asks the Ouroboros registry for its current tools and invokes `judgeEachTool`. That method converts the current list into name/description records. For each tool it asks whether the harness covers the operation, then compares the tool's description against every other listed tool, subject to the existing option limit. [Judge entry and cohort](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_value.go#L130-L190).

The duplicate request asks whether one of `existing_tools`, run as it is, supplies the requested operation. It also asks which listed option is closest. `CheckDuplicate` returns the yes/no probability and the selected name. This is not a timeless assertion that similar source code once existed somewhere. [Question contract and response](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/toolcheck.go#L36-L116).

The host asserts `tool_duplicate_judgment(Tool, Other, Percent)`, asks the kernel for the resulting evidence, then persists the kernel's yes/no reading along with the selected `Other`. A successful low-probability later reading can replace an earlier positive. [Observation and persistence](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_value.go#L190-L222).

### 2. The durable record has the name, but its replay does not

`ToolJevVerdict` stores `Yes`, `Percent`, `Other` and timestamp. `NoteJevVerdicts` replaces only the question kinds included in the incoming writes. An update to `harness_covers` does not clear `duplicate`. [Record shape](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_jev.go#L46-L72), [selective replacement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_jev.go#L104-L127).

`jevFacts` replays each positive verdict as only:

`tool_jev_verdict(Tool, Kind, Percent)`

It does not include `Other`, the source tool's version, the substitute's version, or the comparison inventory. `JevFacts` skips the ledger of a tool that is itself retired, but cannot evaluate the omitted dependency of another tool. [Replay and retirement filtering](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_jev.go#L138-L185).

Boot intentionally loads that evidence again when attaching the kernel. A fresh process is therefore not a reset. [Kernel attachment](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_kernel.go#L17-L35).

The ProfileStore's mutex, file lock, read/merge/write and atomic replacement are substantial controls. Under the witness's successful lock and persistence conditions they correctly retain the old verdict. They cannot determine that its omitted comparison dependency is no longer applicable. [Ledger publication](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger.go#L97-L188).

### 3. Current policy admits the replay without an available substitute

The persisted-verdict rule derives `tool_value_evidence(Tool, /duplicate)` directly from the replayed triple. Its live-judgment counterpart also ignores `Other` after accepting the threshold. Neither rule joins the selected substitute to a currently callable registry identity. [Evidence derivation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/jev_learning.mg#L92-L146).

The value policy turns duplicate evidence into a `/redundant` dead reason; dead outranks earning. A generated origin then permits a `/retire` proposal. This redundant path does not require the minimum call/session bars used by other value rules. [Redundant reason](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/tool_value.mg#L135-L172), [class precedence and retirement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/tool_value.mg#L208-L267).

The report's fact adapter does consult the current generated catalog and suppress a retired tool's own facts, but it invokes the same `jevFacts` for each surviving tool. Removing B's own row therefore does not remove A's replayed verdict. [Catalog and evidence projection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_facts.go#L35-L112).

### 4. The real interfaces exercise both sides

The CLI `nerd autopoiesis tools` boots the Cortex, invokes `JudgeToolValue`, prints derived evidence and closes the Cortex. The TUI `/tool value` obtains `ToolValueReport`; `/tool retire <name>` calls `RetireGeneratedTool` under the session operation context and approval broker. These are live consumers, not only test helpers. [CLI judgment](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_systems.go#L432-L486), [slash-command dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L889-L908), [TUI report and retirement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/tool_value.go#L12-L56).

Retirement first requires a current policy-derived proposal, builds a version-bearing approval subject and bounded preview, waits for approval and reserves dispatch. Successful revocation removes the compiled artifact/identity and runtime registration, unregisters the session tool, and marks **that name's** ledger retired. It does not scan or retire other ledgers' duplicate dependencies. [Retirement admission and settlement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L131-L222), [actual registry withdrawal](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L82-L126), [retirement marker](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger.go#L316-L318).

The source is deliberately retained. On restart, restoration enumerates compiled binaries; keeping B's source alone does not restore the removed compiled registration. [Restore entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/runtime_registry.go#L227-L264).

## Sequential normal-entry witness

Assume one workspace with two valid, already registered generated tools A and B that genuinely provide the same operation. Their admitted origins need not defeat duplicate prevention: they could have been generated while optional Jev tool checks were disabled, then evaluated after checks were enabled. The current generation path explicitly supports an unconfigured/disabled check rather than treating it as denial. This is a setup condition, not a claim that duplicate creation is guaranteed. [Generation-check availability behavior](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_jev.go#L15-L35).

Use healthy persistence and kernel operations, configured Jev with `ToolChecks` enabled for the judgments, and no concurrent publisher. Keep unrelated dead reasons absent for A; its harness judgment is below the positive threshold, and its ordinary measurements do not independently make it dead.

1. Run the actual CLI tool-value judgment with A and B present. Condition on valid reciprocal duplicate readings: A's selected alternative is B, and B's is A; both yes/no probabilities meet the configured yes threshold. Harness readings do not derive `harness_covers`. These readings can be correct in this initial two-tool state.
2. Start the TUI with the persisted profile state. Ask `/tool retire B` and approve that exact request. The proposal is legitimate while A remains as the substitute. All revocation, registry withdrawal and profile persistence operations succeed.
3. Ask `/tool value`. A remains registered and unretired. Its stored duplicate verdict still says `Other=B`, but replay drops B's identity. The kernel still derives A's duplicate evidence, redundant reason, dead class and retirement proposal.
4. No action removes A merely because the report contains that proposal. A remains available unless a separate authorized retirement is requested and succeeds.

A fresh-judgment extension shows why “ask Jev again” is not sufficient:

5. Close the TUI cleanly and run `nerd autopoiesis tools` again. Only A remains in the generated registry; no overlapping process or stale in-memory registry is needed.
6. The method clears the temporary judgment/replay predicates. A's harness question returns a valid non-positive answer.
7. `others` is empty, so the duplicate branch executes `continue`. It creates no new duplicate write, rather than replacing the old duplicate verdict.
8. `NoteJevVerdicts` updates only A's harness record. The deferred `replayJevEvidence` then reloads A's still-positive duplicate record. The CLI can again print A's duplicate evidence despite having no current generated alternative to compare it with.

The exact zero-alternative branch is at [`tool_value.go:186–188`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_value.go#L153-L222). It differs from a provider timeout or an oversized comparison cohort: the registry snapshot is complete and contains no alternate candidate. No second duplicate model answer is needed for steps 5–8.

This establishes a false **current applicability** of an old observation, not that the old observation was false. It also does not establish that no conceivable built-in or shell operation could replace A. Those are governed by the separate harness question, conditioned negative in this witness. Nor does it claim that every redundant classification has this cause; other valid evidence can independently justify it.

## Safeguards and intended behavior to preserve

- Keep Mangle as the classifier. Go should publish current registry/dependency observations and identity-bound judgment receipts, not replace policy with a discretionary “keep A” rule.
- Keep optional judgment availability and explicit uncertainty. An unanswered question is not a model “no.” A removed witness makes an old positive inapplicable; it does not certify that the tool has unique capability.
- Preserve whole-history call tallies, costs and their existing policy semantics. Do not reset legitimate execution measurements merely to invalidate a tool-level duplicate comparison.
- Preserve valid replay when the subject and substitute still match, and preserve the existing behavior where an actual later negative reading withdraws a prior positive. [Replacement regression](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_jev_test.go#L152-L166).
- Preserve generated artifact containment, registry withdrawal, version-bound human approval, cancellation and explicit mode behavior. This repair must not automatically retire either A or B.
- An invalidated duplicate reason does not suppress independent `harness_covers`, cost, non-use or miss evidence. The desired result is not that A must always become earning.

## Proposed contract and migration

### Keep a dependency-bearing historical receipt

A tool-level duplicate receipt should identify the subject, chosen substitute and the identity/description or capability contract actually judged, plus the judgment's scope, time, model/policy interpretation and relevant inventory generation. The precise identity may differ for generated tools versus reviewed built-ins; do not assume a name is an immutable capability.

The existing `Other` is a useful start and should reach the policy boundary rather than disappear during replay. Current callable registration must be observed independently. A persisted ledger origin alone is insufficient proof that a substitute remains available.

Derive applicable duplicate evidence only when its dependencies match the current supported serving scope. At minimum, successful B withdrawal must make a receipt depending on B unavailable for A's current redundancy claim. Preserve the historical receipt and disclose why it is inactive. Regeneration under B's name requires a deliberate identity/applicability decision, not automatic resurrection of the old relation.

### Treat refresh dispositions explicitly

A refresh needs more than “some questions answered”:

- positive/negative answered for the current cohort;
- no alternatives in a complete current cohort;
- unavailable judgment;
- unsupported/oversized cohort;
- stale dependency while the request was pending.

For a complete empty cohort, host-observed absence can retire the duplicate applicability without inventing a Jev probability. For timeout or unsupported cohort, keep the historical observation but distinguish unverified current applicability. Do not make uncertainty a new permanent positive or a fabricated negative.

A small implementation can filter dependency-bearing receipts against the current registry snapshot during every report/admission and update them on successful retirement. A generation-bound receipt/index is more general but costs more. Either approach must cover restart replay and `ToolValueReport`, not just the CLI refresh branch. Retirement should recompute the relevant current proposal under the existing approval/version contract; a previously rendered report is not fresh admission evidence.

### Avoid races introduced by the repair

If judgment runs concurrently with registration or withdrawal, publish its result only against the subject/substitute snapshot it assessed. A late response must not revive evidence about a removed substitute. Define identity and lock/transaction boundaries without holding a lifecycle lock across network inference or human approval.

This concurrency requirement is an evaluation obligation for the repair, not a race required by the witness. Preserve the existing file lock/read-merge-write and atomic file publication. If adding per-receipt replacement, do not let one writer erase unrelated tool statistics.

Older records have `Other` but no version/cohort identity. Migrate conservatively: retain the historical record, validate what can be validated, and mark insufficient applicability unknown or request bounded reassessment. Do not assert missing version provenance from today's registration merely because its name matches. Keep rollout changes to schema, fact declarations, rules, CLI/TUI output and tests synchronized.

## Costs and fair unrun evaluation

Dependency checking can be inexpensive relative to inference: build one current registry identity map and validate each retained verdict against it, rather than repeatedly scanning all tools. For T tools and D retained dependency receipts, the straightforward host-side inventory/filter storage and traversal are O(T + D), excluding kernel materialization and payload sizes. This is an algorithmic estimate, not measured performance.

Receipt metadata, migration, reverse-dependency indexes and current/historical display add disk, memory and maintenance costs. Rejudging every pair after every retirement would add avoidable inference and can approach quadratic comparison input. Prefer invalidating affected dependencies first; any reassessment must respect existing budgets, option limits, cancellation and provider availability. No token or latency savings are claimed.

Proposed checks, all unrun:

1. **Normal producer-to-consumer sequence:** persist reciprocal A/B readings through `JudgeToolValue`, retire B through actual approved retirement, then inspect `/tool value` and the derived proposal. Compare registry, durable receipt and Mangle evidence separately.
2. **Fresh-process singleton refresh:** reopen the remaining A and call the real CLI judgment route. Require the old duplicate to remain historical/inapplicable, including when the harness query succeeds negatively. Count questions to show no invented duplicate request for zero options.
3. **Live substitute control:** retain B unchanged and verify that legitimate replay survives restart. Do not “fix” the witness by discarding all persisted evidence.
4. **Explicit later negative control:** preserve the existing test's replacement of an earlier positive.
5. **Other reasons:** make A legitimately covered by the harness or dead for independent measurements; withdrawing B must not erase those reasons. Preserve whole-history counters exactly.
6. **Refused retirement:** rejection, expiry, absent approval host, no policy proposal or revocation failure must not be reported as a successful dependency withdrawal.
7. **Name reuse/version change:** withdraw or replace B, then register a different B. Require identity-aware treatment, not name-based reactivation.
8. **Refresh failure and cohort bounds:** no Jev, disabled tool checks, timeout, empty cohort and too-many-options must have distinct honest dispositions. Avoid treating “could not ask” as “no duplicate exists.”
9. **Late response:** pause a judgment, successfully withdraw its substitute, then deliver the old response. It must not restore current redundancy without revalidation.
10. **End-to-end costs:** measure complete report/retirement/reopen cycles, profile read/write volume, allocations, kernel query work, lock time, question counts/tokens, latency and false invalidations, not only a lookup helper.

Existing definitions verify persistence across reboot, explicit replacement by a later reading, and successful approved withdrawal. The missing contract connects those properties across **substitute retirement**. [Judgment and reboot regression](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_value_test.go#L66-L98), [approval withdrawal regression](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement_test.go#L72-L86).

**Completion criterion:** an old correct duplicate judgment cannot, by replay alone, continue to justify current redundancy after its only identified substitute is withdrawn. History remains inspectable, other valid evidence remains effective, and any actual removal still requires its own current proposal and authorization.


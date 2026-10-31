# 03 — Gap Analysis: session

## GAP-SESSION-FOCUS-VIEW-DEPENDENCY-FRESHNESS

A focus file's digest is not the identity of its rendered context: sibling
signatures, graph facts, related tests and render budgets can change while that
file stays byte-identical. At each working-request boundary the session must
locally obtain the provider's rendered view and compare its content alongside
the focus identity and the working policy's observed file revision. Append a
new snapshot when that identity changes; byte-identical renders with unchanged
focus/revision do not duplicate context. A disappearance of an available view
must be represented as unavailable rather than silently leaving the previous
snapshot presented as current. A later available render must be admitted even
when the focus-file digest never changed.

Keep previous request messages and their harness text byte-identical within an
epoch. New snapshots and availability notices belong on the current round;
compaction retains its existing policy ownership and clears the epoch's view
identity. Prior snapshots remain historical evidence, not current authority.
Refreshing rendered bytes does not prove that every underlying graph fact or
arbitrary carried observation is fresh, coherent or safe: those retain their
own producer, revision, permission and acceptance contracts.

Status: implemented for working-request focus-view reuse. `updateLedger`
measures a new render and fingerprints its bytes with the focus identity and
policy-observed file revision (`internal/session/working_context.go:834`). It
appends an unavailability notice when a previously available render disappears
(`:842`), and compaction still clears view identity with the epoch (`:809`).
The native tool-loop regression and unchanged/reverted dependency controls are
`TestFocusViewFreshness_NativeRequestsTrackRenderedDependencies`
(`internal/session/focus_view_freshness_test.go:40`); availability transitions
are covered in the same file at line 164. Existing append-only, restatement,
budget and compaction tests retain their independent responsibilities.
Exit: the native production tool loop with the real holographic renderer and
working policy delivers changed sibling signatures, graph callers and test
presence to its offline provider double while the focus digest stays unchanged.
Unchanged renders, unrendered dependency edits, reversions, availability
transitions, append-only prefixes, budgets and compaction controls pass.

This bounded repair re-renders locally per request and deduplicates what is
sent. It does not establish an atomic workspace/world snapshot, repair stale
facts in the provider, or introduce billable calls for freshness checks.

## GAP-SESSIONLOG-TRANSCRIPT-CRASH-CONSISTENCY

Each successful transcript append must publish a complete `codenerd.transcript/1`
JSON snapshot while preserving the previous committed snapshot until replacement
is ready. Use the existing atomic-file protocol: a unique sibling temporary
file, sync before replacement and best-effort containing-directory sync. Keep
the schema, ordered header/extensions, line arrays, event order, sequence
continuation and existing file mode. A failed append must not consume a sequence
number or destroy earlier committed events.

Legacy in-place appends can leave a recognized transcript's final event or
closing tail incomplete. Recover only fully decoded events under an intact
matching schema, text encoding, session and agent header. Preserve raw history
through the last complete event; do not invent a missing event or rewrite
unknown identity, malformed earlier content or unrelated JSON. Reopening must
atomically publish recovered history before appending. Search may read that
recoverable prefix without modifying the file and must ignore unfinished
temporary snapshots. Unrecoverable files remain untouched and visibly counted
by the existing recording-failure path.

Status: implemented for transcript snapshot publication and recognized torn-tail
recovery. `Transcript.open` repairs recognized legacy tails before sequence
continuation (`internal/sessionlog/transcript.go:217`); `Transcript.write`
publishes complete ordered snapshots through atomic replacement (`:281`).
`recoverTranscriptTail` validates the recovery boundary (`:323`), and Search
uses that same read-only recovery (`internal/sessionlog/search.go:142`).
Deterministic interruption, old-reader, reopen/search, identity refusal, retry,
initial-mode and concurrent-reader controls live in
`internal/sessionlog/transcript_durability_test.go`.
Exit: deterministic interrupted legacy writes, including early residual-tail
bytes and a fully encoded event missing its tail, remain searchable and resume
recording after reopening. An old open reader retains its complete prior
snapshot across a new append. Negative identity/corruption, unknown-field,
concurrent append and incomplete-temporary-file controls pass under race.

Snapshot publication reads and rewrites the current transcript per event, with
memory, temporary space and I/O proportional to its size. This bounded repair
retains the existing single-Store writer ownership; it does not add cross-process
append coordination or claim physical power-loss guarantees on every filesystem.

## GAP-SESSION-CRITIC-DIAGNOSTIC-OWNER-STOP

Optional critic grounding has its own positive `session.lsp_timeout` bound, but
must obey explicit cancellation and `WithOwnerStop`, including an owner stop
after the caller's soft deadline. A stopped owner must not start later language
diagnostics, file-open stages or critic requests. Keep the soft-deadline
completion allowance, absent-server silence, partial findings and unchecked-file
notices. A canceled or timed-out LSP request must close its transport so blocked
protocol writes end; cleanup must not detach the owner again. Normal completion
retains bounded polite shutdown. The critic stays advisory; mandatory proof,
permission gates and the review's existing request bounds remain unchanged.

Exit: offline gopls and real in-memory LSP fixtures prove pre-cancellation,
mid-request cancellation and owner stop after a soft deadline terminate owned
work and suppress later stages. Controls prove ordinary completion, bounded
timeout, partial findings, absent-server and soft-deadline-only behavior.
Status: implemented for optional critic diagnostics and stage admission.
`diagnosticOwnerStopped` and `diagnosticRequestContext` retain the explicit owner
channel while allowing the existing soft-deadline completion window
(`internal/session/build_verify.go:1244`, `:1262`). Transport closure is attached
to the request and joined during cleanup (`:1297`). Go and stdio diagnostics use
that lifecycle (`internal/session/lsp_diagnostics.go:137`, `:275`), and the critic
gate checks stop before/between diagnostics and before model dispatch
(`internal/session/build_verify.go:941`). Offline fail-before and regressions
exercise owner stop, partial findings, transport closure and later-stage refusal
(`internal/session/diagnostic_owner_test.go:33`, `:68`, `:133`), plus pre-cancel,
soft-deadline-only completion and a blocked initialization write
(`internal/session/diagnostic_owner_controls_test.go:17`, `:43`, `:71`). This is
request/transport ownership, not a new critic verdict, permission or inference
budget policy.

## Required contract: explicit admission for configured live TypeSafe tests

A configured TypeSafe section or nonempty key is not consent for a test to call
an external provider. The session report-reader fixture must skip before loading
configuration unless `CODENERD_LIVE_TYPESAFE` is exactly `1`. Approved live runs
retain their existing configured client, judgments and assertions. Ordinary
production inference does not read this test-only switch. Workspace gate
environment allowlists stay unchanged: an ambient switch is not automatically
forwarded; an owner can explicitly forward it with existing configuration.

| Gap ID | Current deficiency | Target | Status | Exit criteria |
|---|---|---|---|---|
| GAP-SESSION-LIVE-TEST-ADMISSION | The former credential-only test admission is replaced by an exact-one switch before configuration loading. | Explicit default-off test admission above. | Closed for this bounded test-admission contract | `internal/session/finish_round_test.go:34` and `internal/session/typesafe_live_admission_test.go:89`: default and malformed admission dispatch zero HTTP requests; exact admission, missing configuration/key, ordinary inference and gate forwarding controls pass. The paired chat regression and build-environment regression cover the other helper and Go/non-Go command construction. |

## Required contract: Consultation evidence freshness

Completed specialist answers must not be reused across execution results without
receipts identifying the evidence that produced them. Filesystem samples before
and after a call cannot establish that identity, including for CodeDOM and
knowledge context or an A→B→A transition. The session routing ladder must therefore
ask again rather than advertise an old answer as unchanged. Within-turn scoped
cards and `internal/consultation.Manager` in-flight coalescing retain their
existing responsibilities.

Failure history may drive escalation or a visible unresolved outcome only for a
nonempty file scope with complete successful reads before the call, immediately
after that call, and at the later routing boundary, all with matching digests.
Go retains every read outcome; Mangle judges coverage and stability. Fileless or
unreadable scopes cannot suppress a later consultation. A changed scope starts
a new escalation sequence.

| Gap ID | Capability | Current deficiency | Target | Phase | Exit criteria |
|---|---|---|---|---|---|
| GAP-SESSION-CONSULT-EVIDENCE | Consultation evidence freshness | Completed-answer reuse is removed; failure history now carries complete before/after observations into the kernel. See `consult_route.go`, `expert_spawn.go`, and `policy/shard_experts.mg`. | Required contract above | Authored; verification pending | Execute the real-kernel `TestConsultSnapshot_*` cases, `TestConsultRoute_*`, and full-boot `TestBootConsultSnapshot_DoesNotDeliverOldAdviceToTheNextModelRequest`; establish fail-before/pass-after behavior and shard/baseline checks before closing. No tests have been run for this change. |

## Accepted generated-tool feedback target (2026-10-02)

**PARTIAL — GAP-SESSION-GENERATED-FEEDBACK:** `internal/session/executor_tools.go#Executor.executeToolCall` directly calls the generated registry at line 2597 after safety preflight. Root inspected this route and `internal/system/factory_tool_executor.go#orchestratorToolExecutor.ExecuteTool` at line 47: the factory's tool-quality feedback adapter is installed on VirtualStore, not this direct path. Raw execution counters are not automatic learning. Existing turn feedback through `#Executor.persistTurn` is distinct. Source audit: artifact:.corpus-build/runs/all-features-20261002/02-execution/handoff-3.json.

**PROPOSED UPLIFT:** normal generated calls execute exactly once through the existing executive/feedback path, preserving the executive-issued action identity, exact canonical payload, JIT allowlist, default-deny permission, preflight, caller cancellation, binary protocol, true output/errors, validation and counters. Missing bridge or matching authorization must fail closed, never fall back to raw execution. Real successful and failed executions reach tool learning exactly once; refusals produce no effect or fabricated execution feedback. Root proves Process-to-effect-to-learning behavior with disconnected-route/feedback and refusal controls. Accepted, not yet wired or verified; no permission widening or shell access is implied. Companion GAP-SYSTEM-GENERATED-FEEDBACK owns factory/adapter lifecycle.

> Wave-2 reconciliation 2026-09-25: see the table at the end of [TODO.md](TODO.md) for each item's current classification and evidence. Last full verification of the text below: 2026-08-09 — true-up for the 2026-08-09 task-integrity incident (prompt/03-GAP-ANALYSIS G9/G10, prompt/12-FAILURE-MODES FM17/FM18, world/03-GAP-ANALYSIS, session/09-SAFETY-AND-INVARIANTS). Section 7 records current reality as OPEN; do not mark G9/G10 closed until the canonical-precedence and temp-repo negative exams exist and pass.

## 1. Spec vs reality matrix

| Desired behavior | Reality | Gap severity |
|------------------|---------|--------------|
| Universal clean loop | `Process` / `ProcessWithIntent` live | **None** |
| JIT specialization | Compiler + ConfigFactory wired | **None** |
| Constitutional default deny | `checkSafety` fail-closed | **None** |
| Interactive executive gate | Type-assert seam on VirtualStore | **None** (depends on VS implementing interface) |
| Multi-turn tools all providers | Native and Piggyback share one loop (`piggybackChannel`, commit 799c5a3) | **Closed 2026-09-25** |
| No planning-only false success | `intent_requires_tool_call` + nudge | **Low** residual (kernel policy must exist) |
| Task isolation | CloneForTask + task intent IDs | **None** |
| Cross-session memory | Persister optional; atomsJSON populated (`compilationAtomsJSON`) | **Low** (atomsJSON stale claim) |
| Piggyback memory → cold store | `context.ApplyMemoryOperation` (commit e0df714) | **Closed 2026-09-25** |
| Ouroboros auto-generation from missing_tool_for | Detect + log; generation elsewhere | **Low** (wiring external) |
| Spawn auto-start consistency | Spawn does not start; SpawnSpecialist/async do | **Low** |
| Completion signaling | Poll 100ms | **Low** |
| Empty AllowedTools unrestricted | Nil/empty now denies every tool | **Closed 2026-07-13** |
| Package README accuracy | Stale slogans | **Low** (docs) |
| Pre-task ownership baseline + shell-effect safety (2026-08-09) | Mutating/ambiguous shell denied before any handler (`checkShellEffect`, `TestExecuteToolCall_ShellEffectGateStopsIncidentBeforeExecution`) | **Closed by fail-closed denial** (see TODO.md wave-2 table) |
| World refresh after shell effects (2026-08-09) | No shell mutation is accepted, so none needs a refresh | **Moot while shell mutation is denied** |

## 2. Priority backlog (engineering)

### P0 — correctness / safety — 2026-08-09 task-integrity incident (OPEN, do first)

1. **Capture an immutable pre-task ownership baseline at task start.** Snapshot (tracked vs untracked) × (dirty vs clean) × (owned by this task vs pre-existing) and freeze it for the task. Every later revert/delete decision must consult this baseline, not a later mutated view.
2. **Detect, attribute, scope-check, and fail closed before success on every run_command/bash effect.** Treat every shell invocation as a potential workspace mutation; detect filesystem effects, attribute them to the invoking task, scope-check against the task’s permitted mutation set, and deny success on any out-of-scope or unattributable effect.
3. **Forbid revert of pre-existing dirty tracked work and delete of pre-existing untracked paths.** `git checkout`/`restore` of a dirty tracked file that was dirty in the baseline and `rm -rf` of an untracked directory that was untracked in the baseline are violations, even if “cleanup” would otherwise be convenient.
4. **Wire accepted mutations to incremental world retraction and reassertion.** When a shell effect is accepted, incrementally retract and reassert the affected world facts before any later policy/world query or success verdict, so the kernel view reflects post-shell reality. One-shot pre-delegation scan alone is insufficient.
5. **Add both negative acceptance exams in a temporary repo** — (a) git checkout of dirty tracked work must not lose the dirty content; (b) recursive deletion of an untracked directory must not delete it. Both must currently fail and pass only when 1–4 are implemented.
6. **Enforce canonical prompt-atom precedence (coupled with prompt/03-GAP-ANALYSIS G9):** boot reconciliation must replace 878 stale corpus.db built-in copies with the 888-ID embedded canonical atoms while preserving project-only atoms. Add the seeded temp-corpus.db exam.

### P0 — correctness / safety (pre-existing)

7. Ensure production VirtualStore always implements `InteractiveExecutiveGate` when destructive tools are enabled (wiring audit, not session-only).
8. Keep fail-closed tests green; never reintroduce nil-kernel allow under gate-on.

### P1 — tool protocol completeness

3. Multi-iteration Piggyback tool feedback (re-issue envelope with tool results).  
4. Preserve empty AllowedTools as “no tools”; any bootstrap capability must be an explicit validated envelope.

### P2 — memory & persistence

5. Populate `atomsJSON` on `StoreSessionTurn`.  
6. Wire memory operations to cold storage / session compressed state.  
7. Use `StoreCompressedState` when SubAgent compresses.

### P3 — lifecycle polish

8. Unify Spawn vs SpawnSpecialist start semantics (or document contract firmly).  
9. Replace Wait polling with event/channel if contention becomes hot.  
10. Align package README with Spawner/TaskExecutor reality.

## 3. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| No local `.mg` | Correct: session is runtime, policy is global |
| Spawner exists despite “no spawn” slogan | Slogan is anti-domain-shard; Spawner is intentional |
| Baseline prompt on JIT failure | Deliberate degrade, not silent success with wrong tools |
| `safe_action/1` classification | Correctly insufficient; only exact `permitted/3` authorizes |
| Shared kernel across SubAgents | Intent IDs + retract; required for shared policy world |

## 4. Consumer asymmetry

| Path | Assembly |
|------|----------|
| Normal Cortex boot | `system.factory` `initFinalExecutors` |
| Campaign cobra | Rebuilds Executor/Spawner/JITExecutor in `cmd_campaign.go` |

**Gap:** dual assembly can drift (timeouts, token budgets, persister, ouroboros registry). Prefer shared factory helper long-term.

## 5. Test gaps

See [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).

Notable: Piggyback multi-turn (once built) needs e2e; real InteractiveExecutiveGate blocking path has unit coverage mainly via mocks/type-assert patterns.

## 6. Repaired gaps and remaining truth

The missing/empty capability fail-open, Ouroboros registration bypass, specialist
config validation bypass, and `safe_action/1` permission fallback are closed with
focused tests. The remaining priorities are protocol-neutral multi-turn tools,
one owned stack composition, durable state/receipts, and policy-reference registry
parity. The authoritative implementation contracts are in [TODO](TODO.md).

## 7. 2026-08-09 task-integrity incident — true-up (OPEN gaps)

> Records current reality, not aspirational completion. All items in this
> section are OPEN until the contracts and negative acceptance exams listed
> below exist and pass. This section is coupled to prompt/03-GAP-ANALYSIS.md
> G9/G10, prompt/12-FAILURE-MODES.md FM17/FM18, world/03-GAP-ANALYSIS.md, and
> session/09-SAFETY-AND-INVARIANTS.md for the same incident.

### 7.1 Session reality on 2026-08-09 — OPEN

- `run_command` and `bash` can mutate the workspace while being classified as
  non-write tools. That classification lets shell effects bypass the write-intent
  and world-refresh gates that `write_file`/`edit_*` paths go through.
- The task saw fresh dirty state but incorrectly cleaned pre-existing tracked
  and untracked work. In the incident this meant `git checkout -- <tracked>`
  reverting dirty tracked files to HEAD and `rm -rf <untracked-dir>` deleting
  an untracked directory that the task did not own.
- The one-shot world scan runs before delegation but does not refresh after
  shell effects. The pre-delegation snapshot was fresh; after an unreported
  mutation, later policy/world queries can authorize or “succeed” on a stale
  view.
- No immutable pre-task ownership baseline is snapshotted at task start. Without
  a frozen record of (tracked vs untracked) × (dirty vs clean) × (owned by
  this task vs pre-existing), later mutations cannot be attributed or
  scope-checked.

### 7.2 Required session contracts (all OPEN)

1. **Immutable pre-task ownership baseline** — at task start, snapshot the
   workspace ownership map (tracked vs untracked, dirty vs clean, task-owned
   vs pre-existing) and freeze it for the duration of the task. The baseline
   is the authority for revert/delete decisions; it must not be recomputed
   from a later mutated view.
2. **Shell effects must be detected, attributed, scope-checked, and fail
   closed before success** — every `run_command`/`bash` invocation is a
   potential workspace mutation. The executor must detect filesystem effects,
   attribute them to the invoking task/shell call, scope-check them against
   the task’s permitted mutation set, and fail closed (deny success) on any
   out-of-scope or unattributable effect before returning success.
3. **Pre-existing dirty or untracked paths must never be reverted or
   deleted** — reverting a dirty tracked file (`git checkout`/`restore`) or
   recursively deleting an untracked directory is forbidden when the path was
   dirty/untracked in the baseline. Only mutations owned by the task may be
   cleaned; pre-existing work is inviolate.
4. **Accepted mutations must trigger incremental world retraction and
   reassertion** — when a shell effect is scope-checked and accepted, the
   world must incrementally retract and reassert the affected facts before
   any later policy/world query or success verdict, so the kernel view
   reflects post-shell reality.

### 7.3 Negative acceptance exams (both OPEN)

Both must be implemented as tests that reproduce the violation without losing
the artifact and that pass only when the contracts above are implemented:

- **Exam A — git checkout of dirty tracked work:** in a temporary repo,
  commit a file, mutate it to dirty, run the task/shell path that previously
  issued `git checkout -- <file>`, and assert the dirty content is preserved
  (no revert to HEAD). The test must fail on the current code and pass only
  when baseline preservation + shell-effect scope-check are enforced.
- **Exam B — recursive deletion of an untracked directory:** in a temporary
  repo, create an untracked directory tree, run the task/shell path that
  previously issued `rm -rf <untracked-dir>` (via run_command/bash), and
  assert the directory survives intact. Must fail today and pass only when
  baseline preservation + fail-closed shell handling are enforced.

### 7.4 Coupling note

Prompt/03-GAP-ANALYSIS G9 (canonical atom precedence) and G10
(prompt/world/session coupling) and world/03-GAP-ANALYSIS and
session/09-SAFETY-AND-INVARIANTS describe the same incident from their
subsystem perspectives. Closing any one requires closing the shared contracts
and both negative exams listed here.


## Required contract: Generated execution acceptance

A normal Executor.Process call must execute a real fixture generated tool exactly once through the authorized bridge and preserve its real output, error, and durable learning acknowledgement. Missing bridge or exact authorization must produce no effect and no fabricated execution feedback. This strengthens acceptance for GAP-SESSION-GENERATED-FEEDBACK without changing its target.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-SESSION-GENERATED-FEEDBACK-ROUNDTRIP | Generated execution acceptance | Normal `ProcessWithIntent` reaches the generated-tool loop and exact generated bridge (`internal/session/executor.go:1058`, `internal/session/executor_tools.go:2356`, `:2649`); the production fixture exercises durable acknowledgment after reopen (`generated_feedback_roundtrip_external_test.go:213`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Normal Process-to-generated-binary-to-durable-feedback test plus missing-bridge and refusal controls proves one effect and one real feedback record. |


## Required contract: JIT degradation and cancellation

Compiler cancellation/deadline must terminate the turn before configuration or provider calls. Other JIT failures and absent compiler must use the embedded mandatory identity/safety/protocol baseline or return a visible assembly error. Mandatory-budget refusal must not become a one-sentence ungoverned turn; empty capabilities remain no tools.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-SESSION-JIT-FAILURE-CONSTITUTION | JIT degradation and cancellation | Absent or failed JIT compilation assembles the embedded mandatory baseline and surfaces cancellation/assembly/budget failures before provider work (`internal/session/executor.go:997-1040`, `:1233-1240`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Process tests force cancellation and ordinary compile failure; canceled compilation invokes no LLM/config, degraded requests retain mandatory atoms, and denied/empty capability remains enforced. |

**Verified current:** baseline, compile-failure, cancellation, and budget-refusal cases (`internal/session/executor_process_test.go:64`, `:114`, `:154`, `:207`) pass. `TestExecutor_Process_RealGeneratedBinaryFeedbackRoundTrip` (`internal/session/generated_feedback_roundtrip_external_test.go:213`) passes through a normal Cortex `ProcessWithIntent`, exact permission and disconnected-bridge controls, one real binary effect/output, and durable learning after reopen. The focused session gate passed; aggregate session/corpus verification remains pending.


## Required contract: Planned-step admission completeness

A plan with more valid distinct steps than `session.step_plan_max_steps` runs the first `max` and keeps the rest owed. The parser returns every step, marking the ones past the bound `Beyond`; the executor asserts `step_plan_remaining(Turn, Step, File)` for each before any step runs; `turn_has_unresolved_step` joins them, so the bounded prefix is never `/complete`. The anchor tells each pass how many steps lie beyond, and the report and the `ErrStepsIncomplete` message name every remaining step. Nothing past the bound is run, and no completed effect is replayed.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-SESSION-STEP-PLAN-OVERFLOW | Planned-step admission completeness | `parseWorkSteps` marks steps past the bound `Beyond` instead of dropping them; `runPlannedSteps` asserts `step_plan_remaining` and `derivedStepOutcome` names them; `turn_steps.mg` joins them into `turn_has_unresolved_step` (`internal/session/work_steps.go`, `internal/core/defaults/policy/turn_steps.mg`). | Required contract above | P1 | Implemented; focused tests pass | Existing turn-steps rules | `TestRunToolLoop_PlannedSteps_StepsBeyondTheBoundStayOwed`: with a bound of 2 and three planned steps, two run and edit, the third does not run, the verdict is `ErrStepsIncomplete` naming `[3] a.go (not run: beyond the step bound)`, and the report counts 3 planned, 2 run, 1 remaining. `TestParseWorkSteps` keeps the 5 steps past the default bound. Retained windows across tasks are not built. |

## Required contract: Final per-step file state

Planned-step outcome must be derived from the final file state after every post-edit repair and formatting pass. `work_steps.go` currently records `step_execution` pass counts (`internal/session/work_steps.go:349`), while policy declares `step_file_net/4` as `/changed` or `/unchanged` and derives `step_reverted` from `/unchanged` (`internal/core/defaults/policy/turn_steps.mg:50-71`, `:112-123`); the host does not currently assert that final-state fact. For each file's first planned write, retain its true preimage (existence and bytes, distinguishing absent from empty), then measure only after all repairs. Emit `/changed` or `/unchanged` for matching executed steps and let the existing Mangle rules determine status. Exact restoration must produce `step_reverted`/incomplete, never executed/complete success. Unknown baselines or unreadable final state must surface a measurement error and fail closed; they must not be represented as unchanged. Add no Go completion policy and no new `/unknown` literal.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-SESSION-STEP-NET-STATE | Planned-step final file state | `recordStepPass` records `step_execution`, but no host path asserts the existing `step_file_net/4` predicate after final repair (`internal/session/work_steps.go:346-366`; policy contract `internal/core/defaults/policy/turn_steps.mg:50-71`). | Required contract above | P1 | Accepted; implementation pending | Work-step host measurement and existing turn-steps rules | A real planned write followed by repair that exactly restores the preimage derives `step_reverted`/incomplete; changed, absent-versus-empty, and unknown-baseline/unreadable-final-state controls distinguish correctly and fail closed. Existing policy remains the sole completion decider. |


## Required contract: Evidence toolchain executable identity

Change evidence must pin the resolved Go executable artifact identity at transaction Begin and bind baseline/final witnesses to it. Replacement at the same path must make evidence unverified even if version and environment outputs match. Detect artifact changes around execution so a replaced tool cannot manufacture a passing receipt.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-EVIDENCE-GO-ARTIFACT-IDENTITY | Evidence toolchain executable identity | `Begin` pins the resolved executable identity and `Transaction.Verify` compares witness identities (`internal/evidence/change.go:249`, `:326`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Replacing a pinned Go shim with identical version/env and forged passing JSON is rejected; genuine unchanged executable produces revision-bound evidence. |

**Verified current:** `TestChangeCarriesExecutedRevisionBoundEvidence` and `TestVerify_RejectsReplacedGoExecutableWithSameVersionAndEnvironment` (`internal/evidence/change_test.go:32`, `:82`) pass, along with the full `internal/evidence` package suite.

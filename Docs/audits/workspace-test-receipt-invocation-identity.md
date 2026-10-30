---
doc-class: deep-dive
subsystem: session-verification
implementation-status: proposed-refinement
last-verified: 2026-10-30
verified-against: 4cb0b3650de09085351014d522beb440c7e97ae3
verification-method: static-source-inspection
supersedes: []
---

# Workspace test receipts: identical arguments are not identical checks

## Decision

Keep reuse of genuinely equivalent test executions, but require the execution identity the obligation actually depends on. The current non-Go workspace test-plan adapter compares a prior process's argument vector and post-write sequence. Its receipt does not contain the working directory. Consequently, a successful `cargo test` in an independent nested project can discharge the root project's `cargo test` obligation without running the root tests.

This is a focused follow-on to [test-evidence-capability](test-evidence-capability.md), **not a new discovery of the need for scoped receipts**. That report already proposes canonical working directory and scope, and its as-built section explicitly acknowledges argv-only reuse. This document supplies a concrete production producer-to-consumer witness for that remaining limitation, its safeguards, and a bounded repair/evaluation contract. It does not repeat the earlier missing-obligation-plan finding: the plan now exists and is used; evidence is admitted to the wrong planned obligation.

All source references below are pinned to main `4cb0b3650de09085351014d522beb440c7e97ae3`. This is static source analysis, independently challenged, not an executed reproduction. No implementation, tests, scanner, build, application-model inference, training, or benchmark was run. No end-to-end false-completion or measured performance claim follows from this report.

## 1. Intended contract and as-built status

The [repository guidance][guidance] assigns executive decisions to Mangle and distinguishes current evidence from narrative. The [session guidance][session-guidance] requires typed, policy-mediated operations, exact capability and permission checks, and revision-bound completion. These requirements support reuse only when the retained measurement actually answers the current obligation.

| Boundary | Observed implementation | Status in this inspection |
|---|---|---|
| Select required non-Go checks | Workspace declarations/detection produce gates; written languages and directories select a plan | Implemented |
| Preserve unavailable checks | An unavailable applicable check remains an obligation | Implemented |
| Admit an actual process result | Tool layer records argv and nonnegative exit code; session sequences it | Implemented |
| Reuse a post-write process | Equal argv plus receipt sequence after last write | Implemented, incomplete identity |
| Bind receipt to actual directory | Actual tool result JSON has directory, structured TestRun does not | Missing from reuse contract |
| Bind receipt to gate environment | Gate runner and typed non-Go runner construct environments differently; receipt has neither | Missing from reuse contract |
| Establish measured harm, cost, or frequency | No runtime study performed | Unverified |

This is distinct from [test-gate-memoization-identity](test-gate-memoization-identity.md), which audits the fingerprint-based Go gate cache, and [net-change-test-plan-applicability](net-change-test-plan-applicability.md), which audits historical versus remaining written paths. It also precedes the downstream judgment freshness discussed in [verification-applicability-and-progressive-evidence](verification-applicability-and-progressive-evidence.md).

## 2. The production route

### The process runs in a real, contained directory

`executeTypedVerification` reads `working_dir`, resolves it against the authorized workspace, and detects the runner in that directory. `resolveWorkingDir` permits valid contained directories; it does not force every test to run at the root. A nested directory containing `Cargo.toml` selects the Cargo framework and then `cargo test`. [Typed selection][typed-selection] [Containment][containment] [Framework selection][framework]

For a non-Go test without a filter, the tool runs that argv with `cmd.Dir = dir`. Once the process has an exit state, it calls `RecordTestRun` with argv and exit code. The returned JSON separately includes `Directory`, so a human or model reading that tool result can see the correct directory. The issue is not a falsified textual result. [Execution and result][typed-execution]

`TestRun` has only `Argv` and `ExitCode`. `RecordTestRun` refuses negative exit codes, preserving the existing distinction between an exited test process and a process stopped by a signal. The receipt never gains a directory field later. Session's tool boundary collects these receipts, retains the last test-run pointer and calls `noteTestRun` for each. The latter assigns the turn sequence and appends the receipt. [Receipt definition][receipt] [Tool ingestion][ingestion] [Sequence publication][sequence]

### The planned check is for the workspace root

The detector recognizes root `Cargo.toml` and declares `rust:test` with `ScopeAll` and argv `cargo test`. `workspaceTestRun` detects the workspace's gates, builds the applicable plan and measures each obligation. Had the planned check run, `gates.Run` would execute it with `cmd.Dir = root`. [Cargo gate][cargo-gate] [Plan entry][plan-entry] [Planned runner][gate-runner]

Instead, `runPlannedCheck` asks `matchingReceipt` first. That helper searches newest-first and accepts a receipt when its sequence is after `lastWriteSeq` and its argv is slice-equal to the planned argv. No directory, manifest or environment comparison is possible with the current receipt. The caller classifies the receipt's exit under the gate, sets `Reused = true`, and returns before invoking the intended runner. [Matcher][matcher] [Reuse branch][reuse]

`noteObligation` publishes that outcome under the planned obligation ID. Mangle's `turn_ob_state` selects the latest applicable outcome, `/passed` meets an obligation, and the plan can yield a passing `/test_run` when no obligation remains unmet and no selected failing run blocks it. The kernel is correctly applying the facts it receives; the host has supplied an overbroad equivalence claim. [Outcome publication][outcome] [Policy][policy]

## 3. Conditional directory witness

Use an ordinary authorized write-oriented session, with its kernel, native continuation, typed test capability, and workspace verification path available. Assume:

1. The workspace root is a Rust project with `Cargo.toml`; Cargo is available.
2. The turn makes a real net edit to root `src/lib.rs`. A root test would fail if the planned root check were executed.
3. `fixtures/independent/` is a contained, genuinely independent Cargo project with passing tests. It is excluded from the enclosing Cargo workspace or otherwise declares a valid independent workspace; it is not accidentally selecting the same root package.
4. After the last write, the model calls `run_tests` with `working_dir = "fixtures/independent"`, without a test filter. The process exits zero. No later write invalidates its sequence eligibility.
5. There is no overriding command that changes the planned root argv, no unavailable gate, and no cancellation before admission.

The nested tool produces the receipt `Argv=["cargo","test"], ExitCode=0`. The root plan produces `rust:test@.` with the same argv. The matcher accepts the nested receipt, the root check is not run, and its obligation is recorded as passed. With no other unmet test-plan obligations or selected failing process, the local `/test_run` can pass.

The official [Cargo test documentation](https://doc.rust-lang.org/cargo/commands/cargo-test.html#package-selection) supports the key distinction: without explicit manifest selection, the selected manifest depends on the invocation directory, and package selection follows that manifest and its workspace rules. Identical `cargo test` tokens therefore do not identify identical selected tests.

This is not an assumption that the model is malicious. A model can legitimately run a nested component's tests during investigation or while dealing with another concern. Containment is necessary for authorized execution but does not prove the result covers another component.

### What this witness does not establish

- It does not show that the entire turn necessarily completes or is accepted. Independent acceptance, effect settlement, finalization and other evidence gates remain relevant.
- It does not bypass tool capability or permission checks or escape the workspace.
- It does not claim the root failure was observed and suppressed. The mistaken reuse prevents this planned measurement from observing it.
- It does not require stale source bytes, a missing sequence check, or a failed process being reclassified as success.
- It does not apply automatically to default JavaScript detection: the typed mapping uses `npm test`, while detected gates use `npm run test`. These unequal vectors do not satisfy this matcher.
- It is not a claim that all nested Cargo invocations differ from root invocation. The independent-project condition is essential.

## 4. Counterevidence and nearby protections

The adapter handles an unavailable check and a canceled context before it attempts reuse. It rejects argument-vector differences, and receipts before the last write are ineligible. The negative-exit filter rejects signal-stopped processes. Those protections should remain unchanged. [Reuse branch][reuse] [Matcher][matcher] [Receipt definition][receipt]

The existing `TestWorkspaceTestRun_AnIdenticalRunIsReusedNotRepeated` checks that a matching post-write receipt suppresses a second gate execution and satisfies the plan. Its fixture would fail if actually executed, making this a useful test of intended reuse. It is not evidence of directory qualification: its handcrafted receipt has no directory to compare. The test was inspected, not run. [Existing test][reuse-test]

The forcing round calls `workspaceTestRun` before checking the derived gate. If that gate passes, it returns without a repair episode. If later repair does happen, the recheck calls the planner again, but that still uses the same receipt matcher. Replanning alone cannot repair a missing identity field. A later genuine write can invalidate the earlier receipt by sequence, while independent later measurements can still reveal the issue. [Forcing and recheck][forcing]

## 5. Environment is a second input to the same contract

The source also demonstrates why adding directory alone is not a complete equivalence rule. Declared gates receive `commands.env`; `gates.Run` applies gate-specific values on top of its constructed environment. Its non-Go command builder uses an allowlisted environment. Typed non-Go verification leaves `cmd.Env` unset and inherits the ambient environment instead. The current receipt identifies neither environment. [Declaration][declaration] [Gate environment][gate-environment] [Typed execution][typed-execution]

A conditional same-directory variant is therefore possible: a declared `cargo test` check requires an environment setting, the typed tool's ambient setting differs, and tests behave differently under those settings. Equal argv and cwd would still not prove that the required check passed. This is a source-derived contract concern, not an executed environment-sensitive fixture. The directory witness above remains the primary reproduction target because it isolates one concrete missing field without depending on a particular test's environment behavior.

No fix should serialize secrets or expose raw environment values in public evidence. Prefer a governed effective-environment identity inside the trusted host and a safe public explanation such as `environment identity differs`. A digest without a specified input set is not a completeness guarantee, and unkeyed digests of low-entropy secret values are not a safe disclosure mechanism.

## 6. Proposed admission contract

The following is a proposed shape, not an existing schema or executable patch:

```text
CheckRequirement:
  obligation_id, turn_id, required_source_state
  canonical_workspace, canonical_execution_directory
  expanded_argv, execution_profile_identity
  required_scope, identity_completeness

TestExecutionReceipt:
  receipt_id, turn_id, measured_after_write_sequence
  canonical_execution_directory, actual_argv
  execution_profile_identity, measured_source_state
  outcome, output_handle, identity_completeness

Admission:
  obligation_id, receipt_id
  equivalent | different | unknown
  host_measured_reason
```

`execution_profile_identity` represents the governed effective environment, executable/toolchain selection and applicable configuration. Source-state identity must state its coverage: turn sequence alone does not witness external changes. A complete dependency manifest is a future optimization, not something to infer from equal command strings.

Rules for the narrow repair:

1. Capture actual invocation identity at the process boundary, not from model prose or re-parsed JSON.
2. Construct the required invocation through the same resolver that would execute it. Avoid separate planning and execution implementations of environment or path normalization.
3. Admit only equivalence established by the host. Missing fields on old or unsupported receipts mean unknown and require a real planned check; an empty directory must never mean implicitly root.
4. Preserve the original receipt as historical evidence even when it does not satisfy this obligation. Do not rewrite it with the required directory.
5. Publish the receipt-to-obligation admission relation and reason, then let Mangle derive coverage and gate state. A passing execution and a satisfied obligation remain different facts.
6. Keep last-write sequence checks, cancellation/unavailable handling, and all execution permissions. Reuse does not authorize any new process or source access.
7. Represent uncertainty in external inputs honestly. Stronger identity reduces erroneous reuse but does not make arbitrary project tests hermetic or deterministic.

This extends the earlier report's canonical-cwd proposal. It does not require a new LLM call, expert consultation, or a new context manager. The [Bazel hermeticity documentation](https://bazel.build/basics/hermeticity) is relevant primary engineering guidance: source identity and controlled tools/environment are foundations for safe reuse. The proposal does not claim CodeNERD gains Bazel's isolation guarantees by storing a richer receipt.

## 7. Options and full costs

| Option | Behavior | Benefit and cost |
|---|---|---|
| A: disable receipt substitution for planned checks | Always execute the applicable planned check | Simple reference behavior; can duplicate expensive tests and increase latency/CPU |
| B: qualify exact invocations | Reuse only host-established invocation and state equivalence | Retains sound exact reuse; adds metadata, canonicalization, resolver unification and invalidation work |
| C: dependency-aware coverage reuse | Admit narrower executions proven to cover the obligation | Potentially fewer tests; requires a trustworthy dependency/scope proof and much more invalidation/evaluation machinery |

Start with B if its minimum identity can be supplied consistently; use A for receipt producers that cannot qualify their invocation. Do not launch C merely to avoid a small metadata change. A caller's claim that two checks are equivalent is not the proof C needs.

Measure costs separately: gate CPU and wall time, toolchain startup, memory, bytes read or hashed for identity, output/receipt storage, concurrent writer coordination, downstream repair, and model/tool tokens. Directory canonicalization may require filesystem work; profile comparison costs scale with the governed inputs; source hashing can dominate a small test. Repeated unmatched checks can add substantial computation even when model tokens stay unchanged. Conversely, a falsely reused pass has hidden debugging and confidence costs not captured by low test duration.

For a current receipt list, newest-first matching is linear in the number of retained receipts and compares argv. A richer descriptor does not by itself require scanning all source bytes on every candidate: compute a qualified invocation identity once per requirement and receipt where valid, then compare. Do not optimize away remeasurement when the identity's inputs can change. No numeric savings are asserted.

## 8. Evaluation and acceptance

Run these only in a separately authorized implementation/evaluation phase. Compare A, B and current behavior on the same source states, toolchains and task traces. Include both correct reuse controls and mismatch cases; count all retries and failed tasks.

| Fixture | Required result |
|---|---|
| Root Rust test fails; independent nested `cargo test` passes after edit | Nested receipt retained but not admitted; root planned check executes and records failure |
| Same required root invocation genuinely passed after final write | Reuse permitted when all required identity fields are known and equivalent |
| Equal cwd/argv but required environment differs | No unqualified reuse; required invocation executes or remains explicitly unverified |
| Legacy receipt missing directory or profile | Unknown identity, not implicit root/equality |
| Pre-write matching receipt, then a real edit | Not reusable; preserve existing sequence rule |
| Different argv or test filter | No exact equivalence; broader coverage inference remains out of scope |
| Valid directory aliases resolving to same canonical location | Defined equivalence, tested on supported platforms; no blind lowercasing on case-sensitive filesystems |
| Cancellation or unavailable required runner | Existing explicit outcome preserved; no old success substituted |
| Process killed by signal | No passing receipt; preserve negative-exit rejection |
| Concurrent external source change | Receipt becomes historical/unknown unless the admission contract measures the change |
| Multiple planned checks with identical argv but distinct profiles | Each obligation gets only compatible evidence |

Acceptance requires an end-to-end typed-tool producer → session receipt → plan matcher → Mangle outcome fixture, not just a test of a new comparator. Verify that the textual directory remains truthful and that stored receipt identity does not leak secrets. Also verify that final acceptance guards remain independent: a passing local test gate is not automatically task completion.

A fair performance report must state the false-admission count alongside processes avoided, p50/p95 latency, CPU, identity overhead, total tokens and repair work. The target is fewer redundant checks **subject to correct obligation coverage**, not the lowest number of test executions.

## 9. Bounded next step

Resolve the existing report's remaining exact-reuse limitation by giving every reusable receipt a qualified process identity and making the planned-check admission fail closed on missing or mismatched identity. Keep the first change narrow enough to compare against always-running planned checks. Only then consider semantic scope subsumption or dependency-aware reuse.

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/agents.md
[typed-selection]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/tools/shell/verification.go#L38-L55
[containment]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/tools/shell/execute.go#L43-L68
[framework]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/tools/framework.go#L86-L152
[typed-execution]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/tools/shell/verification.go#L186-L218
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/tools/test_runs.go#L9-L58
[ingestion]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/executor_tools.go#L1041-L1068
[sequence]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/gate_facts.go#L657-L682
[cargo-gate]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/gates/detect.go#L146-L151
[plan-entry]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/workspace_test_run.go#L39-L62
[gate-runner]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/gates/run.go#L63-L103
[matcher]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/workspace_test_plan.go#L159-L171
[reuse]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/workspace_test_run.go#L112-L134
[outcome]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/workspace_test_run.go#L136-L153
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/defaults/policy/coder_safety.mg#L610-L635
[reuse-test]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/workspace_test_plan_test.go#L112-L131
[forcing]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/session/test_run_round.go#L35-L95
[declaration]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/gates/detect.go#L46-L91
[gate-environment]: https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/gates/run.go#L79-L143

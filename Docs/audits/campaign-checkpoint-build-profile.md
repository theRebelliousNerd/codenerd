---
doc-class: deep-dive
subsystem: campaign-verification
implementation-status: proposed-refinement
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: source-inspection-only
supersedes: []
---

# Campaign checkpoints must preserve workspace build configuration

## Finding and decision

The production campaign `/tests_pass` checkpoint does not explicitly apply workspace `build.go_flags` or `build.env_vars`. It constructs a detected test command and calls `GetBuildEnv(nil, workspace)`, bypassing the shared `GoInvocation` path that reads `.nerd/config.json` and applies those settings. Consequently, a successful checkpoint can describe a genuinely passing default test selection while omitting tests required by the workspace build profile. This is a residual profile-parity defect, not an absence of all build environment support or a hidden permission bypass. [Checkpoint construction][checkpoint] [Shared invocation][invocation]

The proposed refinement is to reuse the shared Go invocation contract for Go checkpoints and bind checkpoint evidence to the effective profile. Mangle should own the required verification obligation; Go should resolve configuration, construct typed commands, and execute them. A passing result should discharge only the obligation whose selection and invocation it actually tested. This strengthens the logic-executive boundary without asking a model to invent tags, interpret process success as broader proof, or bypass existing execution controls.

No repository code, tests, model evaluations, benchmarks, or campaign runs were executed for this audit. The witness below is a source-derived conditional construction, not a reproduced incident.

## Where the profile is lost

`BuildConfig` defines `EnvVars` and `GoFlags` as additional settings for Go build and test commands. `GoInvocation` loads `WorkspaceUserConfig`, selects `GetBuildEnvForTest` for tests or `GetBuildEnv` otherwise, and calls `AppendGoFlags`. The configuration reader uses the workspace's `.nerd/config.json`; missing configuration produces an empty/default result, while unreadable or invalid configuration is logged and falls back to nil. The existing helper is therefore not fail-closed. [Configuration fields][config] [Invocation and loading][invocation]

Environment support alone does not imply configuration loading. `GetBuildEnv` merges filtered ambient and configured environment, but `loadBuildConfig` copies explicit `EnvVars` and `GoFlags` only when its configuration argument is non-nil. It does not read the configuration file itself. With nil configuration, workspace detection can still supply SQLite header settings and `sqlite_vec` defaults when applicable and absent from ambient settings. Those defaults are useful, but they are not the user's arbitrary build profile. [Environment merge][env-merge] [Configuration and automatic defaults][env-load]

For `/tests_pass`, the checkpoint detects its command, adds `-json`, applies `TestTagsForWorkspace`, and supplies `build.GetBuildEnv(nil, cr.workspace)`. It does not load `WorkspaceUserConfig` or call `AppendGoFlags`. A `go.mod` selects the Go framework, whose test command is `go test ./...`. The extra tag helper is explicitly CodeNERD-specific: it requires a matching module and SQLite header file, rather than interpreting general workspace build settings. [Checkpoint construction][checkpoint] [Framework selection][framework] [Automatic tags][tags]

There is a concrete typed-tool contrast: the shell verification implementation calls `GoInvocation` and later executes the resulting command with its environment. That establishes an existing consumer of the shared construction, not a claim that every task or session gate necessarily uses it. [Verification invocation][verification-invocation] [Verification execution][verification-execution]

The shared flag helper also embodies precedence, not merely concatenation: configured flags follow the Go subcommand, while an explicit argument with the same flag name wins. Repairing the checkpoint by blindly appending one tag would duplicate only part of this behavior and could change precedence. Likewise, adopting `GetBuildEnvForTest` brings test-specific behavior, including `GOTRACEBACK`, `-count=1`, and relevant ambient settings; the cache and timing implications deserve an explicit decision. [Flag precedence][flags] [Test environment][test-env]

The `/builds` checkpoint has the same construction concern: it supplies detected build arguments without injecting workspace configuration and relies on the base environment. That is a secondary affected surface, not a separate fully developed witness here. [Build checkpoint][build-checkpoint]

## Why the production executor does not restore the profile

The boot path preserves the audited executor rather than replacing it with an ungoverned runner. The system factory assigns `BaseEnvironment = build.GetBuildEnv(nil, workingDir)`; application configuration supplies execution settings there, but its `Build` configuration is not passed into this environment construction. The factory creates the direct executor and virtual store. The virtual store preserves its injected configuration in the audited composite, its accessor returns that composite, and the factory exposes it as `Cortex.Executor`. [Executor configuration][executor-config] [Factory construction][factory-create] [Audited composite][virtual-store] [Accessor][audited-accessor] [Cortex exposure][cortex-executor]

Chat initialization takes `Cortex.Executor`; campaign planning passes the executor into the orchestrator; orchestrator initialization passes it into `NewCheckpointRunner`. The tactile factory routes the command unchanged. Direct execution uses the supplied executable and argument array, with environment assembled in order from whitelisted ambient values, base environment, and per-command environment; later layers win. None of those layers independently reads the missing workspace build profile. [Chat boot][chat-boot] [Campaign wiring][campaign-wire] [Checkpoint initialization][checkpoint-init] [Routing][routing] [Direct process][direct-process] [Environment precedence][direct-env]

This path matters in two directions. It establishes that the checkpoint is production-reachable, and it rules out an assumed configuration rescue by the audited wrapper. The checkpoint calls the audited executor directly; this is not a claim that it traverses `VirtualStore.RouteAction`. Existing executor validation, auditing, limits, cancellation, and campaign admission remain. The defect is which valid command runs and what its success proves, not whether execution secretly escapes those protections.

## A conditional witness with real passing tests

Consider an ordinary Go module with these conditions:

1. Its valid `.nerd/config.json` sets `build.go_flags` to `["-tags=featurecheck"]`.
2. An ordinary, always-selected `TestOrdinary` passes. An existing test file guarded by `//go:build featurecheck` contains `TestFeature`, which fails when selected.
3. No ambient `GOFLAGS`, persistent Go environment setting, or injected argument enables `featurecheck`. The module does not meet the CodeNERD-specific automatic-tag conditions.
4. The toolchain and ordinary test execution are healthy. An admitted phase has completed read-only tasks and contains a `/tests_pass` objective.

Go's documented build constraints make custom tags part of file selection; the `-tags` flag supplies additional satisfied tags. Accordingly, the configured invocation includes the failing test, while the default invocation excludes it. This is a selection distinction, not a speculative compiler behavior. [Official Go build constraints][go-constraints]

The source accepts normalized verification methods during decomposition, and `/tests_pass` is a recognized method. The witness therefore needs only valid admitted production data, not a prediction that a particular planner or model will generate it. Once no workers remain, phase tasks are done, and quality-remediation conditions permit progression, the orchestrator runs the phase checkpoint. [Planning method][planning-method] [Verification methods][verification-methods] [Phase checkpoint gate][phase-gate]

Under the stated conditions, the default tests genuinely pass. The checkpoint rejects execution errors, nonzero exit status, and failed JSON test events; otherwise it returns success with its passing-test count. Its evidence need not be fabricated to be insufficient: the omitted configured test was never selected. The phase checkpoint loop records type, pass/fail, details, timestamp, and a `phase_checkpoint` fact, but no effective profile field. If the remaining objectives pass, the success branch can compress and complete the phase. [Result handling][checkpoint-result] [Recorded checkpoint evidence][checkpoint-record] [Phase completion][phase-completion]

This construction changes no tests or build headers, requires no preceding mutation, and does not depend on Boolean constraint flattening, named-test no-match, cache reuse, or zero-test success. It proves a wrong-profile checkpoint, not unconditional campaign success. Independent task or session checks, review, and user-declared acceptance can still expose the defect. Failure replan/recheck safeguards and acceptance checks remain relevant. [Bounded failure handling][phase-failures] [Acceptance checks][acceptance]

## Prior work and the bounded novelty claim

The earlier subsystem review deserves explicit credit. Its Item 38 identified missing CGO headers in phase checkpoints; it subsequently documented the tactile `BaseEnvironment` fix using `GetBuildEnv` and reported a successful build checkpoint. The present source still contains that repair. Reopening the old claim that checkpoints lack all build environment support would be inaccurate. The remaining omission concerns explicit workspace profile settings, while CodeNERD header defaults can work correctly. [Earlier finding and repair][predecessor] [Current executor configuration][executor-config]

Two inspected tests help distinguish these contracts. The factory base-environment test exercises header and automatic `GOFLAGS` behavior, not an explicit workspace `Build` configuration. The shared invocation test already checks configured `-mod=mod` propagation into test and build arguments. These are read-only observations about particular tests, not claims about global coverage or passing test execution. [Base-environment test][base-env-test] [Invocation test][invocation-test]

The targeted screen covered root and scoped campaign guidance and related audits: [lint receipt precedence][prior-lint], [workspace test invocation identity][prior-reuse], [tagged-test constraint witnesses][prior-tagged], [verification evidence and judge contracts][prior-evidence], and [campaign generation admission][prior-recovery]. Those works already address receipt precedence, invocation identity, build-constraint witnesses, compatible evidence, and campaign admission. The refinement here is their concrete application to a production checkpoint that bypasses workspace profile loading; it is not a claim to originate the general evidence-identity principle.

The screened research heads were `493e65a2892d7c5262de95999b8c2e256f6a3c4f`, `136e02f0a38897a40931747b1e56a6f0c14feb19`, and `77069960b69aba14a67dbd682d6f65435e8ada50`; relevant checkpoint/factory files matched. Code heads `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908` also matched those files, although other helpers differed. QA heads `51cfd42649552866d4aa54c53b68acf5740eb09b` and `767a3291d6287b238417f2466592a9002810d05a` matched the checkpoint. This bounded comparison supports the residual finding, not universal novelty or whole-tree identity.

## Proposed invocation and evidence contract

Use one immutable effective invocation contract for the checkpoint's execution and receipt. It should identify the workspace/configuration snapshot, working directory, executable, typed argument array, relevant environment identity, selection/profile, and completion outcome. The receipt must distinguish what was required, what was resolved, and what actually ran; a generic successful exit cannot substitute for that relationship.

For Go checkpoints, prefer the existing `GoInvocation` construction rather than another local loader. Preserve argument boundaries, applicable permission and scope boundaries, auditing, cancellation, and execution limits. Non-Go commands should retain their existing behavior. Automatic CodeNERD tags, explicit argument precedence, and configured environment must be reconciled deliberately rather than accumulated through unrelated patches.

Mangle should admit the required profile and determine whether evidence satisfies the obligation. Go should resolve configuration and execute the resulting command. An unknown required profile should remain unresolved, not silently become the default. Legitimately missing optional configuration can still use defaults. Malformed or unreadable configuration needs a stated policy: the existing helper logs and falls back, so simply adopting it does not establish fail-closed verification.

Choose configuration timing explicitly. A checkpoint can use a pinned configuration snapshot or refresh configuration and identify the new version; it must not silently claim a stale obligation was verified under changed inputs. Define compatibility for legacy receipts that lack profile identity, including when they cannot discharge a configured obligation. Avoid recording raw environment values indiscriminately: use secret-safe identities and variable names, with any necessary raw diagnostics access-controlled. Credentials should never be dumped into ordinary Mangle facts or receipts.

## Alternatives and costs

The smallest useful repair is checkpoint-only reuse of shared Go invocation construction, paired with enough profile identity to avoid overclaiming. An explicitly documented default-only checkpoint is another coherent contract, but it cannot discharge a configured-profile obligation. A richer common invocation packet spanning multiple consumers may improve consistency later; it should not delay the narrow parity correction without a demonstrated need.

Costs include configuration reads and parsing, identity generation and storage, and any extra receipt or prompt bytes. Selecting previously excluded tests can legitimately increase wall time or trigger failure and repair. Adopting the test environment's `-count=1` can change caching, so measure that choice separately rather than labeling all runtime differences as overhead. There is no intrinsic need for an additional model call and no measured token, latency, or cost saving in this audit.

Review failure semantics for malformed configuration, unknown profiles, changing configuration, nested modules, command detection, flag precedence, and automatic tags. Do not solve this by mutating the ambient environment globally or widening every non-Go command's configuration surface.

## Authorized evaluation before adoption

A future authorized evaluation should compare the existing and repaired paths using matched workspace contents, tools, and budgets. Exercise boot configuration through the audited executor into a real phase checkpoint and its recorded Mangle/state evidence; a helper-only test cannot establish production wiring.

Include configured custom-tag fail and pass cases; absent configuration; ambient-tag rescue; configured environment `GOFLAGS`; competing explicit tag arguments; CodeNERD automatic headers; malformed and unreadable configuration; nested-module/detection boundaries; and a separate no-tests case. Cover command failure, timeout, cancellation, unchanged non-Go behavior, and independent task or acceptance checks that stop progression.

Capture actual executable/arguments, secret-safe environment identity, selected files and named-test execution markers, required/effective profile identities, checkpoint and phase verdicts, process counts, latency, CPU, I/O, prompt bytes, and total model tokens/retries. A successful repair should include the required test selection, preserve legitimate defaults and execution protections, and refuse to treat incompatible evidence as sufficient. None of this matrix has been run.

## Pinned source references

All CodeNERD links below use revision `388205138ba6d47c1cff0c5e97c896dfa642a391`. Line ranges identify inspected evidence, not executed coverage. The official Go documentation is pinned at `2aae1000ea1d4b28923b96db28acf4e3b99d6aff`; it is not a claim about an installed Go binary.

[config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/build.go#L3-L18
[invocation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/build/invocation.go#L31-L68
[verification-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/shell/verification.go#L105-L112
[verification-execution]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/shell/verification.go#L188-L195
[env-merge]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/build/env.go#L131-L169
[env-load]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/build/env.go#L493-L536
[checkpoint]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint.go#L158-L191
[framework]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/framework.go#L86-L152
[tags]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/build/tags.go#L9-L22
[flags]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/build/env.go#L307-L344
[test-env]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/build/env.go#L235-L280
[build-checkpoint]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint.go#L240-L272
[executor-config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_execution.go#L30-L40
[factory-create]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1652-L1657
[virtual-store]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store.go#L329-L347
[audited-accessor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store.go#L441-L458
[cortex-executor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2998-L3002
[chat-boot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L107-L121
[campaign-wire]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign_planning.go#L112-L133
[checkpoint-init]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_init.go#L69-L73
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tactile/factory.go#L151-L167
[direct-process]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tactile/direct.go#L172-L176
[direct-env]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tactile/direct.go#L300-L317
[go-constraints]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/alldocs.go#L2102-L2148
[planning-method]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L416-L424
[verification-methods]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/types.go#L1046-L1049
[phase-gate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L152-L167
[checkpoint-result]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/checkpoint.go#L196-L215
[checkpoint-record]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_utils.go#L49-L116
[phase-completion]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L230-L245
[phase-failures]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L173-L228
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_acceptance.go#L96-L183
[predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/.claude/skills/codenerd-dogfood/references/subsystem-pass-2026-09-04.md#L75-L95
[base-env-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_execution_base_env_test.go#L12-L55
[invocation-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/build/invocation_test.go#L14-L37
[prior-lint]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/run-lint-effective-build-tag-receipt.md
[prior-reuse]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/workspace-test-receipt-invocation-identity.md
[prior-tagged]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/tagged-test-constraint-witness.md
[prior-evidence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/verification-evidence-and-judge-contracts.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-generation-admission-and-recovery.md

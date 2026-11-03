# Campaign capability resolution and filesystem checkpoint continuity

## Finding

**The campaign tool-creation handler can correctly resolve an already provided capability, then have that success rejected by the enclosing filesystem checkpoint.** For `/tool_create`, the handler explicitly interprets `Artifacts[0].Path` as a capability name. Normal campaign planning and settlement interpret the same value as a workspace file path. An admitted request for `go_vet` can therefore resolve to the existing `run_lint` provider without generating anything, then fail because `<workspace>/go_vet` is not a file on disk.

This is a targeted continuation of the repaired tool-creation path, not a claim that all capability requests fail or that generation safety is bypassed. It is supported by a conditional production source trace and an existing handler-level test, not by a runtime reproduction. No application, test, command, model inference, or training was executed for this review.

Source pin: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), reconfirmed as main on 2026-10-30. The useful improvement is to preserve the capability subject and its resolution evidence through task settlement, without treating capability metadata as permission to execute a tool or exempting genuine file effects from verification.

## Existing repairs and the narrower residual

The September journey's L5 finding describes a different failure: tool creation timed out but returned a pending result with no error, allowing false completion; it also identifies missing headless generation dispatch. Current code repairs the timeout outcome and reads the kernel's reason before waiting. A provided capability resolves immediately, a dangerous one is refused, and unfinished generation now returns an error. Those improvements are real and should remain intact. This audit does not rely on the historical headless-generation issue. [L5 predecessor](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/journeys/07-external-audit-2026-09-19.md#L81-L92), [current handler contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tool_create.go#L14-L58), [timeout and failure handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tool_create.go#L129-L195).

The current campaign architecture describes `/provided` as resolving the task to its registered provider. The residual is that this handler result does not survive the normal task wrapper for one supported representation of the capability request. [Current architecture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/campaign/INTERNALS.md#L132-L136).

The recent [deletion microcheckpoint audit](https://github.com/theRebelliousNerd/codenerd/blob/9ac1e81ed2b43d6e8f78af8ee55e2c384aa3de08/Docs/audits/campaign-deletion-microcheckpoint.md) already owns the general observation that a surviving-path check does not cover every legitimate effect. Its concrete subject is an observed, authorized file removal. Here there need be no file effect at all: a capability name was converted into a file obligation. A removal witness would not fix this case. The new [finalization artifact audit](https://github.com/theRebelliousNerd/codenerd/blob/c3a596c8e9444a523b3368a25843cb3bad2de112/Docs/audits/campaign-finalization-artifact-scope.md) concerns cleanup after successful acceptance, not this earlier rejection.

## Minimal production conditions

Consider an admitted campaign task with:

- `Type: /tool_create` and an empty `Shard` field;
- a description asking to make the vet capability available;
- `Artifacts: [{Path: "go_vet"}]` and no explicit write set in the planner response;
- a workspace in which the exact path `go_vet` is absent throughout the attempt;
- a healthy kernel with the shipped provider rules, successful assertions and queries, and no competing dangerous classification for this capability;
- successful plan, risk, lease, snapshot, and cancellation admission, with no unrelated writes recorded into the attempt.

The empty shard is essential. `executeTask` checks explicit shard routing before the task-type switch. A nonempty shard takes a different route and is not evidence for this handler composition. In normal plan construction, the raw shard value is copied directly; an omitted optional shard remains empty and reaches type-based routing. No prediction that a model will choose this representation is required, and no claim is made about how frequently it does. [Plan construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L460-L475), [planner schema](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_requirements.go#L553-L567), [dispatch precedence](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L208-L275).

This example uses the existing handler test's exact `go_vet` representation. It does not invent a missing generator, require generated code to compile, or depend on an LLM returning a success verdict. Resolution here means the provider relationship that this handler recognizes; it does not prove that every underlying executable is installed, every environment can run it, or a later invocation is authorized.

## End to end mechanism

### Planning creates a file-shaped obligation

The planner schema permits `/tool_create` and an array of artifact strings. Decomposition sanitizes each artifact as a workspace path and, for a mutating task without an explicit write set, derives the write set from those artifacts. `go_vet` is a valid relative path, so the task receives an absolute write-set entry ending in `/go_vet`. Tool creation is explicitly classified as mutating. The create-to-modify defense is irrelevant because it applies only to `/file_create`, not `/tool_create`. [Artifact conversion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L539-L578), [mutation classification](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_mutation_types.go#L3-L21), [retyping boundary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L770-L793).

Even outside that inference step, `resolveTaskWriteSet` falls back to artifact paths when the task has no nonempty write set. The overload therefore exists at both planning and execution boundaries. The initial snapshot does not reject a missing exact path: it records that path with `Exists: false`. [Write-set fallback](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L569-L595), [missing-path snapshot](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L300-L319).

### The specialized handler correctly resolves the capability

`executeToolCreateTask` treats the first artifact path as the capability, normalizes `go_vet` to `/go_vet`, asserts its requirement, and asks for the generation decision. Shipped policy declares `tool_capabilities(/run_lint, /go_vet)`, derives `has_capability`, and derives the `/provided` block reason. The handler reads the actual provider and returns `status: complete`, `tool_name: run_lint`, explicitly reporting that nothing was generated. [Capability extraction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tool_create.go#L25-L74), [provider facts](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/delegation.mg#L110-L122), [derived availability and block reason](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/autopoiesis.mg#L15-L77), [provider resolution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tool_create.go#L79-L126).

This branch returns directly. It does not call `spawnTask`, create a placeholder file, or obtain an observed file write. `beginAttempt` starts a fresh empty record, so there is no surviving attempt-write path to supplement the erroneous file obligation in the stated witness. [Attempt records](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_attempt_writes.go#L39-L72).

### Settlement rejects the resolved task

`runSingleTask` invokes `executeTaskWithRollback`. After the handler returns without error, that wrapper runs `runTaskMicroCheckpoint` before committing the attempt. The checkpoint sees a mutating task and a nonempty resolved write set, appends recorded write paths, and requires an `os.Stat` success. With no `<workspace>/go_vet`, it returns the missing-path error. The transaction treats that as failure, and `runSingleTask` takes the failure branch rather than `completeTask`. [Production wrapper](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L415-L478), [transaction ordering](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_transaction.go#L52-L107), [checkpoint](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/micro_checkpoint.go#L23-L56).

The failure handler records a failed attempt and asks policy whether to retry, replan, or fail. Those paths can recover; an endless loop or inevitable campaign failure is not established. The immediate supported consequence is a false rejection of the handler's capability-resolution contract and possible unnecessary recovery work. The built-in provider is not unregistered by this file-check failure. [Failure policy handoff](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_failure.go#L24-L147).

## Existing tests and important controls

`TestToolCreate_AProvidedCapabilityResolvesToItsTool` uses a real kernel and the artifact `go_vet`, expecting `run_lint`. That test calls `executeToolCreateTask` directly and does not set the task's type. It verifies the useful local repair but does not exercise classification, the transaction wrapper, or the microcheckpoint. The test was read, not run. [Existing regression test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tool_create_test.go#L18-L39).

The scope has several meaningful boundaries:

- An existing exact file named `go_vet`, or another surviving declared path, can satisfy the current checkpoint. That is an accidental filesystem control, not capability evidence or a recommended workaround.
- A description-only request such as `go_vet`, with no artifacts, explicit write set, or extractable filename, can resolve the same capability while leaving the resolved write set empty; the checkpoint then skips its file gate. This representation difference is an evaluation control, not advice to hide actual file effects.
- Explicit shard routing follows a different consumer. Earlier risk, plan, lease, snapshot, permission, or cancellation failures can prevent the disputed point.
- Dangerous capability refusals, missing-provider/query errors, generation failures, and timeout errors remain failures. The proposed repair must not turn any of them into success.
- Registered capability metadata is not permission to execute the provider, and is not evidence that a requested lint run or test already happened. A capability-acquisition task and a tool-execution obligation must remain different.

## Proposed contract

**Task settlement should evaluate the declared kind of result, not require a capability identifier to exist as a file.** Preserve capability identity separately from file targets in planning, refinement, persistence, and execution. An explicit capability field or typed task subject can coexist with actual source, binary, or metadata artifacts when generation really produces them.

For the narrow existing-provider path, carry a typed resolution result identifying the requested capability, selected provider, and the current trusted registry or policy evidence used to resolve it. Let the kernel derive satisfaction of the capability obligation from that evidence, with missing or contradictory evidence remaining unresolved. Do not accept a prose `status: complete` or arbitrary map alone. This should settle only acquisition/reuse, not a later invocation, test result, or the entire campaign goal.

Keep filesystem checkpoints for actual declared file effects. Where tool generation owns generated files or registration changes, attach their separate effect and lifecycle evidence rather than manufacturing a workspace file named after the capability. Preserve rollback and uncertainty handling for real effects. Do not solve the witness with a blanket checkpoint skip for every `/tool_create`, a token sentinel file, or a widened directory write set.

A compatibility transition should recognize the existing capability-in-artifact convention deliberately, without guessing that every historical artifact string is a capability. Ambiguous legacy records need explicit refinement or reconciliation. On resume, stale provider evidence should be revalidated before reuse; an old resolution must not trigger uncontrolled regeneration or authorize execution. No change to dangerous-capability policy, provider invocation permissions, or the intentional `/file_modify` contract is needed.

## Cost and failure tradeoffs

The narrow provided-capability path needs structured local evidence and settlement plumbing, not another model call or a generated tool. It may add registry/policy queries, fact lifecycle management, and diagnostics. A broader typed subject requires schema, prompt, decomposition, refinement, checkpoint, and migration work. Freshness checks can introduce contention or additional reads; overstrict invalidation can create needless retries. Conversely, accepting stale metadata can falsely promise a provider that is no longer available.

Potential benefits are avoided false failure, rollback bookkeeping, retries, replanning calls, and operator diagnosis. These are hypotheses, not measured savings. Track local CPU, filesystem work, kernel queries, durable state, recovery attempts, latency, and any model usage separately. The principal built-in witness requires no generation inference, so it cannot by itself establish savings in generated-tool compilation or model tokens. Preserve the cheap existing-provider route rather than replacing it with a costly availability probe that executes tools without authorization.

## Fair evaluation not yet run

Start with a deterministic normal-path case using the real shipped capability rules: normalize a valid plan with `/tool_create`, empty shard, artifact `go_vet`, and no workspace file of that name; drive `runSingleTask`, not only the handler. Observe the capability result, checkpoint decision, failed or completed task status, attempt records, provider state, and downstream eligibility. A helper test passing alone is insufficient.

Compare current behavior, a narrowly typed resolution result, and a broader typed-subject design under identical plans, facts, permissions, and budgets. Include:

| Control | Required distinction |
| --- | --- |
| Existing provider with capability artifact | Completion should not require a same-named dummy file. |
| Equivalent description-only request | Subject representation alone should not change acquisition semantics. |
| Explicit shard | Exercise its actual route separately; do not count it as testing this handler. |
| Dangerous, absent, conflicting, or unreadable provider evidence | No completion inferred from missing data or file existence. |
| Same-named file without a valid provider | A surviving path cannot substitute for capability evidence. |
| Genuine generation with success, refusal, failure, timeout, or cancellation | Preserve lifecycle and effect checks; no pending-to-success regression. |
| Task with independent required file artifacts | Capability resolution does not discharge those separate file obligations. |
| Retry, resume, provider change, and legacy representation | No stale replay, duplicate generation, or silent reinterpretation. |

Measure false rejection and false acceptance independently, attempts and replans, resulting task and phase states, unnecessary file creation, provider revalidation, and end-goal correctness. Keep the repaired L5 behavior, C2 observed-path support, L4 removal of redundant builds, and ordinary file-task checks unchanged. No results or performance improvements are claimed here.

## Review coverage

Root and campaign guidance, current audit inventory, and research branches through `research-main-20261030-1117` were reviewed for overlap. Targeted searches did not identify this capability-as-path settlement case; that is bounded screening, not a claim of global novelty.

The active cartographer head `767a3291d6287b238417f2466592a9002810d05a` has identical tool-creation, microcheckpoint, mutation-classification, and planning files for this trace. The older QA, store, session, and OAuth heads retain the same checkpoint, classification, and planning code but lack the split repaired handler file. Their earlier inline implementation is not asserted to have the same `/provided` route. The principal finding and proposed evaluation remain pinned to main.

---
doc-class: deep-dive
subsystem: autopoiesis-session
implementation-status: proposed
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Generated-tool observation fidelity: keep partial returns and diagnostics available to the solver

## Decision

Preserve useful output independently of execution success, and carry generated-process diagnostics into the working observation the solving model can actually consume. Reuse the generated execution receipt, the existing working-result archive and Mangle's disclosure/continuation policy. Do not add a second execution backend or repair missing evidence by replaying the effect.

Two separate, source-established losses make this a substantive seam:

1. The normal generated-function wrapper discards a nonempty return value whenever the function also returns an error.
2. The generated-stdin runtime captures stderr in its receipt, but the session projects only decoded stdout and the error's text. Stderr is retained for durable learning, not delivered in that tool observation or its recall body.

This is an observation-delivery contract, not a claim that failed processes are accepted as successful or that every generated tool is currently offered to every model request.

**Evidence status:** static source inspection only. No implementation, repository write, tests, executable fixture, model inference, training, benchmark or user-computer task was performed. Proposed witnesses below are conditional traces, not reproduced incidents. No repair-speed or quality improvement has been measured. Local files are research material only.

## Source freeze and scope

Main resolved to [356f65e](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871); active feature `feat/conversation-brief` resolved to [c2d39e5](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c); research inventory was read at [b802db9](https://github.com/theRebelliousNerd/codenerd/commit/b802db97f53b913a9995370b9d44d2784fd3d918), around 12:39–12:44 UTC on October 28, 2026. All three heads were rechecked unchanged at 12:49 UTC.

The wrapper, runtime registry, generated receipt type, generated profile/feedback adapter, working-context recorder and production generated-pipeline test are byte-identical across the two implementation refs. `executor_tools.go` and `piggyback_channel.go` differ, so their relevant projections were inspected separately: both still reduce a generated receipt to `receipt.Output` plus error before creating model-facing results. Citations below generally use feature; main-specific anchors are identified where helpful.

The [root North Star](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md#north-star) assigns problem-solving to the model and memory, orchestration and completion decisions to Mangle. [Generated-tool guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/autopoiesis/agents.md) explicitly separates process, validation and durable feedback and forbids effect replay for publication repair. [Session guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/agents.md) requires whole, recoverable tool observations and common native/Piggyback contracts. This proposal closes a delivery gap under those existing requirements.

The July tools questions and September autopoiesis overview contain historical paths and semantics; they were treated as orientation, not proof of current execution. The [architecture documentation standard](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/09-architecture-doc-standard.md) requires proposed behavior to remain distinguishable from current code.

### Relationship to current research

| Existing work | Boundary not repeated here |
|---|---|
| [Request-visible tool contract](https://github.com/theRebelliousNerd/codenerd/blob/b802db97f53b913a9995370b9d44d2784fd3d918/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md) | Whether a tool and its real schema reach a particular model request. This report starts after a legitimately offered, authorized generated call reaches the backend. |
| [Tool-effect receipts](https://github.com/theRebelliousNerd/codenerd/blob/b802db97f53b913a9995370b9d44d2784fd3d918/Docs/audits/tool-effect-receipts-and-validation-completeness.md) | Whether changed state survives post-validation rejection. Current main already settles recognized write-mutation tool effects before returning the error; that ledger is not proof of complete effect coverage for arbitrary generated subprocesses. This note does not report the old recognized-write defect as absent work. [Current accounting](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L1036-L1045), [recognized-write boundary](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L226-L229). |
| [Delegated-turn failure settlement](https://github.com/theRebelliousNerd/codenerd/blob/b802db97f53b913a9995370b9d44d2784fd3d918/Docs/audits/delegated-turn-failure-settlement.md) | Settlement of the whole failed worker turn. The present loss occurs earlier, while that worker is deciding how to respond to a tool call. |
| [TUI evidence turns](https://github.com/theRebelliousNerd/codenerd/blob/b802db97f53b913a9995370b9d44d2784fd3d918/Docs/audits/tui-evidence-turn-ownership.md) | Residual system-result ownership, read acquisition and first-articulation delivery. The present path is the delegated session tool loop, not the global execution-result count-difference path. |
| Background outcome and verification studies | Background job ownership, acceptance applicability, terminal verdicts and independent proof remain separate. More diagnostics are not verification and do not justify a success claim. |

## What already works

The generated path is not a bare ungoverned process launch. Its [typed request](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/types/generated_tool.go#L22-L108) carries host binary/protocol/workspace identity, scope, call, authorization, action, target and canonical arguments. [VirtualStore](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/virtual_store_generated.go#L12-L64) verifies identity, exact authorization and preflight before invoking the backend.

The [factory adapter](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/system/factory_tool_executor.go#L66-L164) retains live-call identity, separately validates, and synchronously records durable learning. Its [feedback-only retry](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/system/factory_tool_executor.go#L184-L228) does not execute again.

The runtime captures bounded stdout and stderr separately; a capture overflow becomes an explicit backend error. Nonzero exits, malformed envelopes and envelope error fields also fail. [Runtime](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/autopoiesis/runtime_registry.go#L410-L503). This report is therefore **not** a silent-overflow finding.

The [durable feedback adapter](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/autopoiesis/autopoiesis_profiles.go#L133-L163) carries stdout, stderr, exit code, partial/truncation flags and validation outcomes. Existing receipt fidelity is valuable and should remain intact.

## Finding A: ordinary partial returns disappear before a receipt can retain them

`ToolCompiler.Compile` creates a wrapper for non-`package main` generated functions; directly supplied main programs take a separate path. [Dispatch](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/autopoiesis/tool_compiler.go#L71-L103).

The wrapper calls the selected `func(context.Context, string) (string, error)`. It sets `output.Error` when the error is nonnil; serialization of `res` occurs only in the else branch. It then writes the envelope and exits nonzero. [Wrapper](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/autopoiesis/tool_compiler.go#L397-L417).

**Conditional source trace:** a wrapped function returns a useful partial inventory, identifiers for completed work or a repair diagnostic together with an error. The emitted JSON contains `output: null` and the error. Runtime `decodeToolOutput` converts null to an empty string. The error is preserved and the process fails, but the nonempty partial result never reaches the host receipt, working archive, learning record or model. The host cannot recall bytes it never received.

This limitation is already acknowledged inside an existing test, so it should not be presented as newly unknown to every contributor. The production fixture's `partial-fail` case writes its own JSON envelope and exits directly, with an explicit comment that returning an error would let the wrapper discard partial output. [Fixture](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/tests/e2e/production_generated_pipeline_test.go#L77-L88). Its [assertion](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/tests/e2e/production_generated_pipeline_test.go#L1000-L1015) covers the manually emitted envelope path, not ordinary partial-return conservation. These test definitions were read, not run; their presence is not a reported passing result.

Preserve returned data as **partial, untrusted observation**, not as an authoritative description of effects. Independently observed host state is needed to establish what actually changed; the recognized write-mutation ledger must not be assumed to enumerate arbitrary generated-process effects.

## Finding B: stderr exists in the receipt but not in the solver's observation

For `generated-stdin-v1`, the runtime initializes Output from stdout, then decodes the JSON output field. Stderr is concatenated only for the separate legacy-argv branch. [Runtime projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/autopoiesis/runtime_registry.go#L475-L502).

The receipt type preserves Stderr. Its `Err()` joins only backend, validation and feedback errors; `GeneratedExecutionError.Error()` adds call identity and that joined error, not the diagnostic stream. [Receipt/error types](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/types/generated_tool.go#L119-L150).

Session execution then returns `receipt.Output` with the error, or just Output after checking complete validation and feedback. [Feature projection](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L2703-L2721), [main projection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2693-L2711).

**Conditional source trace:** a generated process supplies a valid envelope with a partial result and generic error, and writes a more specific diagnostic to stderr. The typed receipt and durable learning have the diagnostic; the model receives the partial result and generic error only. For a process that writes only stderr and exits, it can instead receive an envelope-decode error and exit status without the useful stderr explanation. Stderr on a successful process is similarly absent from the tool observation. None of these cases proves that the ultimate answer must be wrong; they establish that a useful evidence channel is unavailable to the solver through this route.

The normal batch path deliberately carries error text **and returned output** to the model; that recent repair cannot recover a channel already omitted by the generated adapter. [Main result framing](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L943-L969).

Nor does ordinary recall repair it: `recordWorkingResult` appends the error's text to the passed body and saves that WorkingRecord. [Recorder](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/working_context.go#L409-L431). Its body is the projected Output, not the complete generated receipt. Claim only this path is lossy; the raw stderr is not globally destroyed because durable feedback retains it.

### Connection to the TUI

The TUI [delegation helper](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation.go#L48-L74) calls the observed task executor when available. The inline executor path [runs ProcessWithIntent](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L352-L409); tool results enter subsequent solver requests, and the returned output later feeds the chat's disposition. [TUI result consumption](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L606-L656).

The inline task clone explicitly defers `RetireWorkingScopes`: its working archives end with that task. [Lifetime](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L360-L367). Existing worker recall is therefore an **in-task** facility, not a promise that the parent TUI can recall its bodies after return. This proposal targets diagnostic availability before the worker settles. A later cross-task diagnostic handoff would need explicit ownership, retention and authorization; durable learning storage alone does not supply that recall contract.

Native results carry the projected body plus IsError. Feature Piggyback [renders those same result blocks](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/piggyback_channel.go#L184-L221); changing that renderer alone cannot recover an absent diagnostic. The intervention belongs before channel rendering and before working observation storage. TUI terminal error preservation remains the separate delegated-failure study, not a new claim here.

## Proposed contract and implementation boundary

The following is a design, not an existing API:

1. **Producer conservation.** Encode a returned result independently of the error. A nonempty partial return survives while the error and nonzero exit remain errors. Preserve JSON-valued results without turning them into a string or accepting incomplete JSON. Direct-main and legacy protocols retain explicit identities; never retry a failed call under a different protocol.
2. **Raw receipt remains distinct from the model projection.** Keep stdout envelope, decoded result, stderr, parse outcome, process outcome, capture completeness, validation and feedback independently inspectable. Do not concatenate stderr into protocol stdout before JSON parsing, nor overwrite decoded Output used by existing validators.
3. **Task-owned observation.** At the session boundary, retain an authorized observation of those receipt components under the existing task/call identity for the owning task's lifetime. Small relevant diagnostics can be inline; larger bodies have existing-style recall handles and exact component labels. Store once and reference, rather than duplicating the entire receipt into every request. Do not publish expiring worker handles as durable parent-TUI evidence; post-task access is outside this proposal unless a separately specified, authorized handoff owns the body.
4. **Logic-owned disclosure.** Go measures stream presence, size, parse/exit/capture facts and produces immutable projections. Mangle derives disclosure and continuation obligations. A present stderr stream is not automatically a failed operation; empty stderr is not proof of success. Introduce any model-facing interpretation guidance as JIT atoms, not hardcoded recovery speeches.
5. **Failure stays failure.** Partial output, receipt durability, a clear explanation and successful diagnostic recall do not satisfy the task. Existing effect, verification, deadline and terminal gates remain mandatory. New diagnostics do not grant tools or permission.
6. **No effect replay for disclosure.** A diagnostic request reads the retained observation within its authorized lifetime. Feedback retries retain their existing no-reexecution behavior. If the body is missing after restart or task retirement, report it unavailable; do not synthesize it, downgrade the failure or rerun the tool merely to populate a UI.
7. **Untrusted data boundary.** Stderr and returned text are tool-authored data, not executive instructions. Redact prohibited secrets before model-facing disclosure, show a redaction/omission indicator, preserve access control on recall, and do not expose unauthorized or sensitive host paths or credentials through “raw details.” Legitimate workspace paths needed to explain a diagnostic should remain available. Authorized sensitive raw retention and model-visible projection may differ.
8. **Migration honesty.** Already compiled binaries still contain their previous wrapper. Track the producer behavior/version or rebuild and re-register through the normal authorized generation path; changing host source alone does not recover old partial returns. Keep binary hashes and protocol identity truthful.

Start with wrapper conservation plus a narrow generated-receipt-to-observation adapter, not a universal replacement for every tool result. Update the autopoiesis, session and affected CLI specs first with checkable exits. Existing general effect-receipt and observation infrastructure should carry this refinement where possible.

## Alternatives and costs

| Option | Benefit | Limitation / cost |
|---|---|---|
| Keep current behavior; inspect learning JSON manually | No runtime change or request growth. Raw stderr can sometimes be recovered by a human. | Worker cannot use it through its normal observation; partial returns were discarded before learning. Adds human diagnosis and possible repeated work. |
| Prompt generated tools to include every diagnostic in their returned error | Low host implementation cost. | Convention cannot repair existing binaries, panic/abrupt-exit stderr or wrapper loss. Depends on the model following a convention under failure. |
| Preserve partial returns only | Small producer fix; no new stderr disclosure. | Repairs Finding A, not B. Still worthwhile independently; requires rebuilding affected wrappers. |
| Append every stderr byte to Output | Simple and makes some diagnostics visible. | Pollutes JSON/format contracts, loses component provenance, inflates every request and can trigger validators on unrelated warnings. Does not restore dropped partial returns. |
| Typed receipt projection plus existing recall, recommended | Keeps result/diagnostic/status distinct; supports bounded progressive disclosure and no-effect diagnostic recovery. | Adapter, policy facts, archive identity/redaction and cross-channel tests; additional storage, serialization and request bytes. |
| New global logging/telemetry system or extra diagnostic agent | Broad observability. | Duplicates retention, adds latency and model cost, and still cannot reconstruct a producer-discarded value. Not necessary for this seam. |

The goal is better **correct repair/disposition per total resource spend**, not fewer visible tokens. Account separately for:

- Model input, output, cache-read/cache-write and reasoning usage where reported; do not assume a byte-to-token ratio or zero cost for cached diagnostics.
- Inline diagnostic bytes repeated across requests; recalled bytes and any lost cache prefix from unstable projection. Immutable append-only observations should preserve existing prefix behavior.
- Host CPU for redaction/projection/serialization, archive I/O, peak memory and retained disk. The existing capture cap is **10 MiB per stream**, not a desirable prompt payload. A receipt may hold raw stdout plus decoded Output as well as stderr; avoid another unconditional full copy.
- Generation/recompilation and validation cost when upgrading existing wrappers.
- Time-to-first-useful-diagnostic, p50/p95 task latency, retries, repeated effects, model/tool calls, human diagnosis minutes, review effort and failure-recovery time.

A useful prospective accounting model is:

`total_cost = provider_spend + host/storage_cost + amortized_regeneration + priced_human_rework`

Report physical quantities alongside any dollar conversion. Compare paired tasks under the same provider/config/acceptance constraints, count failed and blocked tasks in the denominator, and distinguish tool execution success from verified task completion. There is no claimed numeric saving or production incidence estimate here.

## Evaluation plan and checkable exits

These are proposed evaluations requiring separate execution authorization; none were run in this review.

### Deterministic conservation matrix

Use an ordinary generated function compiled through the real wrapper, not a hand-emitted envelope, for the first two rows:

| Fixture | Required observation |
|---|---|
| Nonempty plain-text result plus error | Exact returned text remains accessible, error remains an error, process runs once. |
| Valid JSON object/array result plus error | JSON content survives without becoming malformed or being mistaken for success. |
| Success with stderr warning | Result remains successful if existing validators pass; diagnostic presence is represented without treating stderr as an automatic failure. |
| Valid envelope plus generic failure plus specific stderr | Both partial result and specific stderr are available, independently labelled. |
| Stderr-only nonzero exit; malformed stdout with stderr | Protocol/process failure remains explicit; captured diagnostic remains available. |
| Stream at and beyond capture limits, including multibyte boundary | No false completeness; capture-bound error survives; safely rendered prefix is labelled; omitted tail is not promised as recallable. |
| Empty result, null result, empty diagnostics, no process start | No invented body, no fabricated partial result, no executed-success claim. |
| Feedback publication failure and feedback-only retry | Same effect count and observation identity; publication repair does not execute or duplicate the observation. |
| Existing legacy executable and old generated wrapper | Explicit old semantics/version; no automatic alternate-protocol retry or fictitious recovered return. |

Inspect native bytes, host receipt, persisted feedback after reopen, working archive, actual model request and TUI disposition. A fake provider may make deterministic request assertions but must not propose tools absent from actual definitions: satisfy catalog/authorization prerequisites through normal legitimate configuration, and report an unavailable tool as an admission blocker rather than declaring this observation path tested.

### End-to-end solver and TUI checks

- Route a normal TUI task through the real task executor with CodeDOM/world context enabled. Use a harmless generated fixture whose repair-relevant detail occurs only in stderr or in a nonempty error return.
- For native and Piggyback transports, assert the next model request contains the bounded diagnostic or a valid, authorized recall handle. Exercise recall after compaction within the owning task and verify exact component bytes plus unchanged call identity. After inline-clone retirement, require an explicit unavailable disposition rather than an implied post-task recall guarantee.
- After diagnostic delivery, ensure the worker can choose a correct repair or truthful blocker. Assert final TUI claims against independent host state and verification evidence; do not grade merely on whether it mentions the diagnostic.
- Include long diagnostics, warnings on success, failures after actual temporary-file effects, duplicated result delivery, cancellation, unavailable archive and user-request restrictions. Preserve existing gates and prove zero disclosure-driven reexecution.
- Keep the existing manual-envelope partial-output fixture as a runtime test. Add normal-return coverage instead of replacing it or treating it as redundant.

### Comparative experiment

Compare current baseline; wrapper-only repair; indiscriminate stderr append; and typed bounded projection. Use paired deterministic cases first, then representative authorized coding tasks with held-out fault cases. Hold model/config/permissions/task acceptance constant, randomize variant order where practical, and retain both successful and failed traces. Report semantic diagnostic availability separately from solver accuracy.

Release exits: all required result/diagnostic bytes are accessible or explicitly marked unavailable; native and Piggyback projections agree on outcome and provenance; ordinary wrapper partial results survive; final TUI does not promote partial data into verified completion; no authority bypass, secret disclosure or disclosure-induced effect replay; established result/validation behavior is unchanged for no-diagnostic success; operational costs are measured and acceptable against the user-visible repair benefit. A deterministic transport win alone is not a measured coding-quality win.

## Bottom line

The host already captures more generated-tool evidence than the solver receives, while the normal wrapper discards another class of useful evidence before capture. Close those two seams before adding prompts, retries or another diagnostic agent. The small architectural unit is a truthful, recoverable generated-call observation—not a new planner, broader execution permission or a stronger claim of success.


## As-built status

Verified and built 2026-10-28 on `main` (after `50cb9c64`). The wrapper, runtime and session projection were unchanged from the audited revision.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| A: the wrapper encodes the result only in the no-error branch, so a partial result returned with an error becomes `output: null` | Real, fixed | `tool_compiler.go:403` encodes a non-empty result beside the error; exit stays 1 |
| A: the runtime decodes an envelope carrying both output and error into `Output` plus `BackendError` | Confirmed; unchanged and now reached | `runtime_registry.go:488-490` |
| A: the production fixture hand-writes its partial envelope because the wrapper dropped it | Confirmed; the fixture was kept. A real-wrapper, real-build test was added | `tool_compiler_partial_test.go` |
| B: generated-stdin-v1 keeps stderr only in the receipt; the session returns `receipt.Output` and the error | Real, fixed | `executor_tools.go:2716`, `:2719` (and the success return) now return `generatedObservation(receipt)` (`generated_observation.go`) |
| B: legacy-argv appends stderr to Output already | Confirmed; not appended twice | `generated_observation.go` returns legacy Output as is |
| Capture overflow is an explicit backend error (not a silent cut) | Confirmed; the observation also labels the stderr as a prefix | `runtime_registry.go` `OutputTruncated` |
| `recordWorkingResult` archives the projected body, so recall cannot recover what projection dropped | Confirmed; recall now archives the body with stderr in it | `working_context.go` `recordWorkingResult` |
| Inline-clone working archives end with the task | Confirmed; not changed (cross-task diagnostic handoff is out of scope) | `task_executor.go` `RetireWorkingScopes` |

### What was built

- Ruling (A): the wrapper encodes a non-empty result regardless of the error; a JSON result stays JSON. The error still fails the call. Already compiled binaries keep their old wrapper until regenerated; nothing pretends otherwise.
- Ruling (B): the model's tool result is the decoded output followed by the stderr under `[generated tool stderr]` (or `...; the capture bound was reached, so this is a prefix]`), with `logging.RedactSecrets` applied. `receipt.Output` is untouched, so validators, learning and the durable receipt are unchanged. No extra Mangle predicate: whether the call failed is still the receipt's error, and stderr presence decides nothing. The whole diagnostic is carried; the working context archives a large result behind a recall handle.
- Spec: `Docs/architecture/session/IMPLEMENTED_SPEC.md` §5.5, `Docs/architecture/autopoiesis/INTERNALS.md` (Compilation).

### Tests

`internal/autopoiesis/tool_compiler_partial_test.go` compiles a real generated function through the real wrapper and checks a text and a JSON partial result survive beside the error with exit 1 (fails with the old condition). `internal/session/generated_observation_test.go`: stderr beside a result, stderr-only failure, whitespace-only stderr and legacy protocol leave the result alone, credentials redacted, a cut capture marked.

### Left open

- Separate structured components (result / stderr / parse / exit) as Mangle facts with logic-owned disclosure (§ proposed contract 3-4): not built; one labelled observation was enough to put the diagnostic in front of the solver.
- Post-task (parent TUI) recall of a worker's diagnostic: out of scope, as the document says.
- Regenerating existing generated binaries; the comparative experiment and cost measurements: not run.

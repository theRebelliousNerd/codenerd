# Advisory repair should survive its first read

## Scope and evidence

This is a static-source audit and design proposal against CodeNERD main at `4d984f9c9b719b013a6400f0c88deb48d2e5eb82`. No runtime experiment, test, inference, benchmark, or implementation was performed. The observation concerns the native tool-result continuation path; a provider that performs its own internal tool loop can behave differently.

The existing architecture deliberately offers a single advisory response to a review. That is not an accidental unbounded loop or missing mandatory gate. The proposed change is to distinguish **one advisory episode** from **one model response**: a useful episode may require reading before accepting or rejecting advice. Advisory findings must remain non-authoritative opinions, not mandates.

## Executive finding

Expert critique, the generic critic uplift, and survivor advice each invoke the coding model once, execute any returned tool batch, and return without supplying that batch's results to a subsequent coding-model response in the same episode. This works when the first response edits directly. It does not complete a read-first sequence when the model needs to inspect the source before deciding whether the advice is sound.

There is a second cost consequence: after a nonempty read-only batch, `recheckUplift` uses the turn's cumulative Go write set to decide whether to build and test. On a turn with an earlier Go edit, the advisory read can therefore buy another build/test recheck even though that advisory batch changed no bytes. This is an opportunity to avoid unnecessary uplift-triggered verification under a stable revision, not permission to discard final revision-bound evidence or concurrent-change detection.

## What is implemented

| Mechanism | Source status |
|---|---|
| Expert review dispatch through consultation, on-change filtering, severity triage | Implemented in `reviewWithShardExperts` |
| Advisory repair prompt allowing either edit or reasoned rejection | Implemented |
| One model response followed by one execution batch | Implemented in three advisory channels |
| Tool-result recording in the session transcript | Implemented; results are not globally erased |
| Tool-result continuation inside the same advisory episode | Not present in the inspected caller paths |
| Recheck/rollback when advisory edits break Go build/tests | Implemented |
| Final revision-bound evidence closure after post-edit rounds | Implemented in the inspected scheduler |
| Proposed bounded advisory episode and disposition receipt | Proposed, not implemented or measured |

## Producer-to-consumer trace

### Expert critique

The repair prompt asks the coding model to either fix each finding with edit tools or state why the finding is wrong. `reviewWithShardExperts` appends this message to its local history and calls `completeWithWorkingContext`. If the answer contains tool calls, it snapshots turn files, calls `executeToolBatch`, ignores the returned tool-result slice, and invokes `recheckUplift` immediately. If the answer contains no tool calls, it returns without exposing the answer through this function's return type. This preserves the existing decision not to replace the user's final answer with review narration, but supplies no structured disposition here either. [Expert round][expert]

The scheduler's `/expert_critique` driver returns `nil` for the model response. On a successful driver return it marks the round as run. Other rounds can still execute and later writes can rearm obligations; the narrow claim is that the current expert repair invocation does not continue after its first batch. [Scheduler][schedule]

### Generic critic and survivor advice

`verifyAndUpliftWithCritic` has the same shape: one working-context completion, execution with its result slice ignored, then recheck. Its comments explicitly explain that review words should not replace the answer produced by the main loop and that fallible opinions should never fail a turn. This proposal preserves both choices. [Critic uplift][critic]

`adviseOnSurvivors` also uses the same shape. Its prompt asks the model to distinguish real behavioral choices from guards whose other branch changes nothing observable, then write a test for real decisions. A first read is a reasonable prerequisite to that judgment. [Survivor advice][survivors]

### Tool results exist, but no reply consumes them here

`executeToolBatch` records individual tool results into the session transcript and returns them. Thus it would be inaccurate to say the read disappeared from all logs or all state. What is missing is its delivery as the next assistant/tool-result pair in the advisory episode. The native loop normally owns this pairing. [Tool execution][tools]

## Conditional witness

1. A writing turn has changed a Go file and reaches expert critique.
2. A consultant returns an on-change, actionable finding; a tool-results provider is wired.
3. The coding model chooses to read the cited function or related contract before accepting the finding.
4. Its first response contains only a successful read call.
5. The executor executes the call and records its output, but this advisory function makes no second completion containing that output.
6. Because the turn's cumulative write list contains Go, it invokes build/test rechecking through `recheckUplift`.
7. The driver returns and the round is marked run, without this episode producing an informed edit/rejection response.

The code directly supports this control-flow witness. It does not establish how often real providers choose the read-first response, whether a specific finding was correct, or whether a later round rescues the underlying defect.

## Existing rescue paths and limits

- Direct edits in the first response work; the existing expert round test exercises that path.
- The consultant can independently inspect source through its consultation tools, potentially making the returned finding sufficiently detailed for immediate action.
- The coding model may already have enough context to act without another read.
- Later coverage, vet, finish, final closure, or another outer attempt may independently find and resolve the defect. They do not necessarily preserve this finding's decision trail.
- Tool results are transcripted and working-result recording may retain them in the working state. This is not a storage-loss finding; the narrow claim is no same-episode model continuation consuming the returned result.
- A provider-internal tool loop is not equivalent to the native external continuation path and must be evaluated separately.
- Advisory opinions do not constitute hard failure evidence. This is not a demonstrated gate bypass or false test pass.
- An unchanged advisory batch does not establish that the whole workspace stayed unchanged: concurrent writers and external effects must still invalidate relevant evidence.
- Cancellation and indeterminate rechecks are not success receipts. Existing recheck and final-closure handling must remain intact; an optimization may omit redundant work only when its revision/effect premise actually holds.

## Reuse the working attempt, not a second executive

`workingAttempt` already performs the central mechanics: it appends the demand, invokes the model, executes calls, records results, detects writes, and feeds assistant/tool-result pairs back. When no write occurs, it asks the working policy whether to continue, nudge, change regime, or stop. It restores the previous regime on exit. `repairRound` wraps this with a repair write guard; the generic helper is also used by finish work. [Existing attempt][attempt]

This is the preferred design seam, not proof that substituting the function is sufficient. Advisory episodes have different semantics from mandatory repair attempts:

1. Preserve the advisory phase for accounting and avoid accidentally tagging all work as mandatory repair.
2. Preserve current write authority. The generic critic explicitly treats its current uplift as unguarded by the repair-episode write-set guard; importing `repairRound` would change that contract. Decide and document advisory write scope separately rather than silently broadening or narrowing it.
3. Keep a reasoned rejection as a valid terminal outcome. A model must not be forced to edit good code merely to satisfy a critic.
4. Keep substantive review narration separate from the final user answer while recording its disposition.
5. Maintain policy-derived progress and repetition limits. Productive source acquisition can continue; repeated reads without progress must not create an infinite advisory tax.
6. Handle the absence of a working loop explicitly. The helper currently requires one; changing all advisory paths to fail the turn when it is missing would violate the advisory contract.
7. On actual edits, retain rollback and fresh final verification. An episode's successful edit is not itself a proof the advice was correct.

## Proposed contract

An advisory episode should carry immutable identifiers for the turn, reviewer, finding, source revision and policy version, plus the scope of the finding. Its result should distinguish at least:

- `rejected_with_reason`: the coding model explains why the opinion should not be acted on;
- `edited_pending_verification`: tools changed relevant state;
- `edit_verified`: the required post-edit measurements passed for the current revision;
- `edit_rolled_back`: a damaging uplift was reverted;
- `unresolved`: productive continuation ended without disposition;
- `unavailable`: no continuation channel or working context;
- `canceled` or `failed`: execution did not finish normally.

These are proposed semantic outcomes, not existing schema names. `edit_verified` means verification requirements passed; it must not claim the critic's semantic concern was mathematically disproven. A reasoned rejection is attributable reasoning, not automatically true evidence.

Mangle should decide continuation, applicable verification obligations and escalation from typed progress/effect facts. Go should execute permitted tools and publish observed outcomes. Advisory content remains untrusted model opinion. Neither a finding nor a model-generated disposition may mint permission, suppress a mandatory gate or assert a successful verification result.

## Avoid needless rechecks without weakening closure

Current `recheckUplift` first checks `wroteGo(result.WrittenPaths)` and then builds and measures tests. That list describes the entire turn, not just the advisory batch. [Recheck][recheck]

A proposed optimization should compare the relevant before/after revision or effect receipt, not merely `ToolCalls > 0` or a tool-name heuristic. A successful read or refused write should not manufacture an uplift-edit obligation. An actual edit—even one hidden behind an execution tool—must invalidate relevant evidence. Preserve the final closure's independent check of what the workspace became. Content-hash comparison has its own I/O cost; reuse existing revision/effect mechanisms where sound rather than adding a full-tree scan per response.

## Full cost model

The present path pays for consultant/reviewer inference, the coding-model advisory response, its selected reads, and potentially unchanged-workspace build/tests. The proposal can add extra model calls, additional context tokens and storage of dispositions, plus policy evaluation and revision/effect checks. It can reduce repeated outer attempts, abandoned readings and unnecessary verification, but neither net savings nor improved repair rates are measured here.

Evaluate total cost per resolved task, including:

- reviewer and coding-model input/output/reasoning tokens, cached and uncached;
- source acquisition, deserialization, paging and repeated reads;
- Mangle evaluation and typed-state maintenance;
- filesystem/revision hashing and conflict checks;
- build/test subprocesses, CPU time, wall time and cache state;
- retries, rollbacks, human review, reopened defects and repeated outer attempts;
- persisted transcript/disposition bytes and retention costs.

The shortest advisory episode is not necessarily the cheapest completed task. Conversely, permitting continuation can magnify a bad review into expensive churn. Report the cost-quality frontier instead of optimizing call count in isolation.

## Fair evaluation plan — not executed

Use the same model, repository revision, tool policy, warm/cold cache conditions, finding set and verification requirements for baseline and candidate. Include no-finding turns so the denominator does not hide review overhead. Separate native continuation from providers with internal tool execution.

| Case | Required observation |
|---|---|
| Immediate correct edit | No unnecessary extra response; fresh verification retained |
| Read then correct edit | Read output appears in next paired response; edit verified |
| Read then reasoned rejection | Rejection preserved; no forced edit or synthetic hard failure |
| Incorrect reviewer finding | No incentive to modify correct code to clear advice |
| Failed read then corrected read | Error reaches model; bounded recovery rather than silent exit |
| Repeated unproductive read | Policy ends episode with unresolved disposition |
| Read-only batch on previously edited Go turn | No extra uplift-caused build/test when revision and effects remain valid |
| Actual write or concurrent change | Relevant verification cannot be reused stale |
| Invalid tool arguments | Error pairing retained; no infinite retry |
| Cancel during batch | Completed effects recorded, pending work terminally canceled |
| Uplift breaks tests | Compare-and-swap rollback and truthful current-state evidence |
| Missing working loop/provider | Typed advisory unavailability, not accidental turn failure |

Measure finding disposition completion, correct fixes, harmful edits, accepted false findings, retained pairings, calls, wall/CPU time and cost per successfully resolved task. Report uncertainty and stratify by issue class and project size. Require held-out review cases and repeated trials before making superiority claims.

## Relationship to existing research

This is not the planned-step loss of expert advice, review-domain packet coverage, critique-parser ambiguity, or final-verdict freshness problem. It concerns the next boundary: after useful advice has already reached the coding model, can the model consume the evidence it asks for before deciding what to do?

Existing architecture explicitly describes the one-call advisory shape. Treat that as an intentional bounded-cost design to improve, not as proof that developers forgot a loop. The recommended evolution preserves one advisory episode while making its necessary read/decision chain explicit, observable and governed.

[expert]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/expert_critique_round.go#L155-L175
[schedule]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/executor_tools.go#L495-L580
[critic]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/build_verify.go#L906-L1067
[survivors]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/survivors_round.go#L13-L60
[tools]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/executor_tools.go#L931-L1019
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/build_verify.go#L757-L903
[recheck]: https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/build_verify.go#L1084-L1118

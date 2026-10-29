# Slash-command target continuity into expert selection

## Scope and decision

Preserve the target already parsed by the slash-command handler as structured execution data. At the inspected revision, `/fix` and `/refactor` correctly separate a path-like first argument from the remaining requirements for task formatting, but still give the whole request to specialist target resolution. Adding a requirement after an existing filename can therefore remove the initial expert assistance that the same bare filename would receive.

This is a source-derived conditional finding at main [`33260224758a13a243acdc9949eeda947866c69c`](https://github.com/theRebelliousNerd/codenerd/commit/33260224758a13a243acdc9949eeda947866c69c), inspected October 29, 2026. No tests, model calls, execution, training or benchmarks were performed for this report. It establishes a routing discontinuity under stated conditions, not a measured task failure rate or token saving.

The target parser, task formatter, expert selector, JIT compiler and later step-specific expert selection are implemented. The proposed change is to carry their shared input contract across the existing boundaries. This is not a proposal to replace the expert policy with a Go heuristic or to invoke every expert for every task.

## Revalidation after main advanced

This report was first traced at `8decae5763cb88bbd842795ca81d3e2559ec2b20` and revalidated at the pinned `33260224` revision after main advanced. The handler, parser/resolver, request-builder and preset handoffs still exhibit the same field loss. The changed executor and expert helper still select from `intent.Target`, preserve the full input for semantic retrieval, and reselect experts at planned-step compilation. Those relevant paths were re-read; this is not a claim to have audited every intervening change.

The complete pinned tree contains 7,174 entries and was not truncated. Its documentation inventory and adjacent slash-payload, specialist-roster and journey material were checked for overlap. No separate current-main document was found covering this residual specialist-routing contract. The prior slash-payload audit concerns bytes supplied to direct file-writing commands, rather than this target-to-expert handoff.

## Relationship to earlier findings

The repository's [journey map](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/Docs/journeys/00-journey-map.md) already discusses the historical problem of putting an entire slash description in the target field. Current source has a repair: `splitSlashTarget` separates a path-like first token, and `formatShardTask` keeps the remaining text as a constraint. That improvement should be retained.

The remaining issue is downstream of that repair. The handler uses the separated values to build prose, then sends the unsplit request to specialist matching. The fallback execution request also omits the structured target. The newly wired expert pipeline makes the consequence concrete: both the chat-side selection and the executor's initial target-based selection can lack the file that was successfully parsed earlier. This document is a refinement of the existing target-fidelity work, not a claim that the original issue was unknown.

## Inspected path

### 1. Parsing knows more than the routing argument carries

Both handlers construct `request` by joining their arguments, call `splitSlashTarget`, and format the task using `target` and `constraint`. Their final call is nevertheless `spawnShardWithSpecialists(..., task, request)`. The last parameter is interpreted as a target by the callee. [Handlers, lines 19–32 and 49–62](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/cmd/nerd/chat/commands_handlers_evolution.go#L19-L62).

`splitSlashTarget` recognizes a path-like first token and returns the rest as the constraint. If the first token is not path-like, it retains the whole text as target instead; that compatibility behavior is not the witness here. `/fix auth.go preserve compatibility` has a parsed target of `auth.go`. The formatter supplies `fix file:auth.go` and carries the constraint in the task. [Parser and formatter](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/cmd/nerd/chat/delegation.go#L436-L529).

### 2. The specialist resolver receives a different value

`spawnShardWithSpecialists` calls `resolveReviewTarget(target)`. The resolver joins a relative target to the workspace and calls `os.Stat` on that full value; an error returns no files. The spawning helper's fallback attempts another stat on the same target rather than using the parsed path. Thus it asks for a path equivalent to `auth.go preserve compatibility`, not `auth.go`. [Resolution](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/cmd/nerd/chat/review_aggregator.go#L947-L976), [specialist routing](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/cmd/nerd/chat/delegation.go#L838-L866).

This is a literal filesystem lookup, not a request to infer a filename from natural language. Go's primary API documentation defines `Stat` as operating on the named file and returning an error on failure. It does not parse task syntax. [Go `os.Stat`](https://pkg.go.dev/os#Stat).

When selection returns no specialists, the helper plain-spawns. A missing expert is not treated as an execution failure, so generic work still proceeds.

### 3. The fallback cannot automatically recover the field

`spawnSimpleShard` calls `spawnTask`, which builds a request through `shardTypeToTaskRequest`. That builder supplies only `IntentVerb` and `Task`. In contrast, the existing `intentTaskRequest` helper for perceived-intent routing explicitly preserves `Target`, `Constraint` and `Request`. This is evidence that the system already has a structural contract capable of representing the information. [Plain spawn](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/cmd/nerd/chat/delegation.go#L901-L915), [task entry](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/cmd/nerd/chat/delegation.go#L35-L54), [request builders](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/cmd/nerd/chat/delegation_routing.go#L354-L372).

The executor intentionally uses a preclassified intent instead of re-perceiving synthetic task text. Its preset takes `req.Target`, and it copies `req.Constraint`. A missing field therefore remains missing at that boundary. Reintroducing an LLM perception call simply to reconstruct fields the handler already knew would add cost and uncertainty. [Preset construction](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/internal/session/task_executor.go#L234-L252), [preset use](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/internal/session/task_executor.go#L427-L441).

### 4. Initial expert selection depends on the missing field

The executor calls `selectExperts(intent.Verb, intent.Target)` before initial prompt compilation. `selectExperts` returns an empty selection when the target is blank. The resulting selection controls the expert keys supplied to compilation and the following consultation stage. [Selection guard](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/internal/session/expert_context.go#L19-L35), [initial compilation and consultations](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/internal/session/executor.go#L1373-L1455).

Two real compensations limit the claim. First, the executor replaces the semantic retrieval query with the actual input text, up to its documented 4,096-rune limit. Generic task-based retrieval is therefore still available. Second, planned-step prompt compilation calls `neededExperts` using the step's target, so a later valid plan can restore target-specific expert knowledge. That later compilation is not proof that the initial expert-led route or initial consultations happened. [Semantic query preservation](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/internal/session/executor.go#L1381-L1391), [step-specific expert selection](https://github.com/theRebelliousNerd/codenerd/blob/33260224758a13a243acdc9949eeda947866c69c/internal/session/work_steps.go#L794-L819).

## Conditional witness and counterexamples

Use a workspace with an existing file, a registered expert whose evidence and policy actually select it for that file, and a functioning kernel. Compare the same request first as `/fix auth.go`, then as `/fix auth.go preserve the exported API`. Assume no real filename contains the entire second request. In the second case, the parser still knows `auth.go`, but specialist file resolution sees the longer string. If that returns no files and the resulting selection is empty, the generic fallback has no structured target for initial executor expert selection.

The contrast is strongest when a matching expert is known to be selected for the bare file; a language extension alone need not imply the same execution or advice mode as stronger evidence. Record the actual derived selection rather than assuming that any `.go` file activates an expert. The `/refactor` handler has the same argument handoff.

This does not establish that requirements disappear from the prompt, that every slash request loses specialists, that all JIT knowledge is absent, that tools cannot read the file, or that the task cannot succeed. A bare path can resolve; an unusual real filename matching the full request can resolve; no registered matching expert means there was no expert benefit to lose; later planned steps can select experts; generic reasoning can still finish the work. The defect is non-preservation of already-known routing structure, with a conditional loss of initial assistance.

## Proposed contract: one parsed task, several projections

Carry one structured request from the handler, with separate fields for original request text, explicit verb, target specification, constraint text and formatted task. Resolution should consume only the target specification. User-facing status can display the original request. The model can receive formatted task and original wording. Policy should receive observed resolved targets and selection evidence. None of those consumers needs to reinterpret another consumer's display string.

The first repair can use the existing `TaskRequest` fields and a typed specialist-spawn input rather than inventing another parallel task format. Merely changing the resolver argument from `request` to `target` fixes one seam but leaves the plain fallback's structured-field loss. Conversely, fixing only fallback requests does not restore chat-side expert-led or advisory routing. Acceptance must cover both.

Proposed invariants:

1. If parsing identifies target T and constraint C, every downstream typed execution request retains T and C unless an explicit, recorded transformation changes them.
2. Filesystem resolution receives T, never display prose assembled from T and C.
3. A resolution failure is distinguishable from a resolved target with no matching expert. Generic fallback may remain legitimate, but its reason should be observable.
4. No-match fallback retains the target so the executor can use its normal target-dependent context. It does not fabricate a specialist selection.
5. Expert policy, consultation limits, tool restrictions and verification gates remain authoritative. A target field is context, not permission to modify that target.
6. Original text remains available independently of normalized target identity. Do not use lowercased or lossy-normalized requirements as the only task record.

For a path-plus-prose command, parse once and validate the selected target against the normal workspace membership contract. For free-form issue descriptions or genuinely ambiguous targets, retain an explicit unresolved target state rather than statting prose and silently conflating failure with no expert. Handling quoting, spaces, globs, new files and multiple targets needs an explicit command grammar; this finding does not establish that the present whitespace parser can represent all of them. Do not repair one case by selecting the whole repository whenever resolution fails.

## Algorithm and observability proposal

1. Parse the slash input into a command record. Keep the raw text and parsing provenance.
2. Resolve the target through the workspace-scoped resolver. Record outcome as resolved, unresolved, invalid or intentionally broad. Deduplicate files without changing semantic scope.
3. Ask the existing expert policy about the resolved file set and verb. Record selected agents and derived roles, not an optimistic message claiming that matching happened.
4. Construct every execution mode from the same command record. Expert-led, advisory and generic fallback paths receive the same target and constraints. Consultation requests may be specialized, but should retain a parent-task reference.
5. Record an inexpensive boundary receipt containing request identity, resolution status, target count, selection result and fallback reason. Retain detailed paths only in appropriately scoped local logs; do not expose user source paths to external telemetry by default.
6. At executor admission, compare the expected explicit fields with the received fields. A disagreement should produce a visible diagnostic or a bounded refusal to claim target-aware assistance, rather than silently treating an empty target as a successful transfer.
7. Preserve later step-specific reselection. It adapts to the plan's actual files and is useful even when the initial target is carried correctly.

The tracing proposal draws on the W3C Trace Context principle that request identity should propagate across intermediaries. That standard concerns distributed trace headers, not CodeNERD task semantics; the application here is an engineering analogy, not a claim that adopting HTTP headers solves field loss. A stable request identity lets logs connect parse, resolution, selection and execution receipts without making the identifier itself a correctness proof. [W3C Trace Context](https://www.w3.org/TR/trace-context/).

## Full cost accounting and failure modes

Passing already-known fields costs storage and copying proportional to the request and target representation. A single-file resolution needs the existing membership and filesystem checks; broad directories may incur traversal. Selection still pays kernel fact preparation, derivation, query and cleanup costs. Restored expert assistance can increase prompt tokens and consultation model calls even if it reduces later investigation. No unconditional token saving follows from making the route correct.

Measure total request cost as parsing, resolution I/O, kernel work, JIT compilation, retrieved context tokens, expert consultation input/output and reasoning, main model calls, tool calls, repeated file reads, verification processes, retries, persistence and user rework. Include unsuccessful tasks and fallback attempts. Charge cold initialization and cache effects separately from warm steady state. Latency should include serial consultation time, not just parallel work's longest model call.

Risks include stale target state when a command record is reused; accidentally sending a broad directory to an expert intended for one file; confusing an absent path with a new-file request; path canonicalization that crosses workspace boundaries; duplicate consultations at chat and executor layers; constraints becoming inconsistent between prose and fields; and overdiagnosing intentional generic execution as failure. Avoid automatic target recovery from arbitrary prose after the user-facing parse has already succeeded. If such recovery is needed for legacy callers, make its provenance and uncertainty explicit.

An accurate route can still choose unhelpful experts or produce poor code. This repair only restores the input contract needed for the existing policy to make its intended decision. It should not change expert confidence thresholds, force expert execution, weaken admission checks, or treat consultation output as verified task completion.

## Fair evaluation plan

These are proposed evaluations, not results.

At the routing socket, hold workspace facts, expert registrations, policy, configuration and task text constant. Compare current behavior, resolver-argument-only repair, fallback-field-only repair and the complete structured handoff. Capture the parsed target, resolver input, resolved files, derived expert selection, constructed `TaskRequest`, preset intent, initial compilation expert keys and consultation requests. A direct call to the specialist helper with a clean target is a useful unit control but cannot replace an entrypoint test through the actual slash handler.

Fixtures should include:

- Bare existing file versus the same file with appended requirements, for both `/fix` and `/refactor`.
- Strongly matched expert, weakly matched expert, no expert, unavailable kernel and selection failure.
- Relative and absolute paths within the permitted workspace, directories, nonexistent targets and new-file intentions.
- Free-form issue text without a path-like first token; malformed quoting and spaces in filenames as explicitly specified grammar cases.
- Generic fallback followed by a valid planned step that restores expert knowledge, and a single-step execution with no such rescue.
- Constraints preserved in both fields and model-visible text; no extra perception model call required merely to reconstruct them.
- Cancellation during resolution or consultation and unchanged restrictions on effectful tools.

At the complete-system level, run the actual TUI command entry through task execution in a separately authorized evaluation environment. Use fixed tasks, model/version/configuration, workspace revision and expert material across variants. Repeat stochastic tasks and report uncertainty. Compare verified completion, first-pass correctness, constraint compliance, human correction burden, false or unnecessary expert activation, total tokens, compute, wall time and peak memory. Include a generic-only control and a manually correct structured-target control to distinguish routing benefit from expert quality.

Do not count fewer consultations as an efficiency win if they result from dropping the intended context. Equally, do not call extra expert calls a win merely because they now occur. The target outcome is lower total cost at equal or better verified task quality, or a justified quality improvement with its full cost shown.

## Handoff criteria

The change is ready for acceptance only when the real slash-handler path conserves target and constraints through specialist selection and generic fallback, reports resolution distinctly from no expert match, and preserves the existing generic retrieval and step-level rescue paths. The minimum regression witness is that appending a requirement to a valid file target no longer silently changes target-aware routing merely by changing the string passed to `Stat`.

The broader architectural lesson is practical: preserve intent structure across adapters rather than repeatedly asking a model or string heuristic to reconstruct it. CodeNERD already has most of the necessary components. This seam is about connecting their contracts end to end.

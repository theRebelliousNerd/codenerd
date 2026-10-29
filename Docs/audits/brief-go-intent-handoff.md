---
doc-class: deep-dive
subsystem: chat-perception-routing
implementation-status: planned
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Brief goal approval needs an interpreted work request

## Decision

Separate the user's go-ahead from the availability of a saved interpretation. A held proposal can reuse the intent already read from its request. A conversation brief that has never become a held proposal must first pass through ordinary perception, with the brief as context, before the kernel can route its work. Keep both cases on the existing policy, capability, permission and verification paths.

The current feature branch conflates these states. When `/go` finds a brief goal but no held proposal, it constructs a work carrier containing only the goal text. `processInput` treats the carrier's existence as proof that it contains a saved intent, skips perception, and reads its zero-value intent. This breaks the documented conversation-to-work handoff even when the brief is correct, current and owned by the selected session. The repair does not require another planner, summary model or permission system.

**Evidence status:** static source and test-definition inspection only. No codeNERD builds, tests, runtime journeys, model inference, failure injection or performance measurements were performed. No implementation was modified. This document is a proposed repair and evaluation plan, not an executed reproduction. The source establishes the missing interpretation; it does not establish a universal blank response, an unauthorized effect or the frequency of user-visible failure.

## Source freeze and prior work

The branch collection was checked at approximately 17:37 UTC and rechecked unchanged at approximately 17:40 UTC on October 28, 2026:

| Branch | Inspected head | Relevance |
|---|---|---|
| `main` | [356f65e0e6bb639adaa90ab36212daa714114871][main] | Comparison baseline; does not contain the new brief-only `/go` path. |
| `feat/conversation-brief` | [c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c][feature] | Contains the defect and the intended conversation-first workflow. |
| `research-main-20261028` | [cb195322f15418ce026f8f28f9ce3fd87a0b9f52][research] | Current research inventory and the query-to-persona continuity refinement. |
| `codenerd-research-docs` | [c592ff223cd207fca4c29ae793d5eba57aa4781d][historical] | Historical related research used to distinguish this finding. |

Read guidance included the root [architecture contract][guidance] and scoped [chat][chat-guidance], [perception][perception-guidance], [core][core-guidance] and [session][session-guidance] instructions. The root requires spec-first changes, JIT-first model behavior, Mangle-owned executive decisions and validation through normal production paths. The [conversation-first architecture document][architecture] supplies the intended product contract; the implementation citations below establish the actual path.

The closest prior observation is in [Chat session memory ownership and coherent recovery, lines 85–91][session-prior]. That document already traces `/go` preferring held work and otherwise selecting a brief goal. It also explicitly notes that **held work** reuses its intent without fresh perception. Its finding is that switching sessions can retain another session's work or brief. This report does not claim discovery of that entry path. Its additional finding is that the **brief-only variant has no saved intent but takes the same skip-perception branch anyway**. No session switch, stale memory or concurrent message is needed.

Other inspected boundaries remain distinct:

- [Conversation brief semantic fidelity and progressive disclosure][brief-prior] concerns accepted meaning, item retirement, task applicability and clipped disclosure. Here even a complete brief never receives the promised fresh interpretation.
- [Perception restriction continuity][restriction-prior] covers typed obligations and detached background/continuation losses. The present carrier error precedes those execution boundaries; this report does not re-report them.
- [Read-only routing and child persona continuity][persona-prior] concerns an interpreted query becoming a write-oriented child. Here the ordinary interpretation is absent.
- [Side-turn ordering and cancellation ownership][side-prior] and [background outcome ownership][background-prior] concern asynchronous identities and settlement. This witness is sequential.
- [Perception classification outcomes and recovery][classification-prior] concerns failed or malformed classification. Here the classifier is not called.
- [Planned-work conservation][plan-prior] concerns retention of admitted plan steps. This defect occurs before the intended request receives an admitted work route.

## The intended handoff and what already works

The feature keeps a structured brief as kernel facts, supplies it to perception through `briefContext`, and provides work modes that hold or admit mutation work. The architecture explicitly says that `/go` with nothing held starts the brief's goal. The carrier's own comment says that a brief-goal run has no saved intent and that its goal passes through perception. [Architecture, lines 9–36][architecture-brief], [carrier comment and fields, lines 18–27][carrier], [brief context, lines 100–105][brief-context]

For an actual held proposal, reusing the existing intent is useful. The later word “go” is not the requested implementation task; reclassifying that word would discard the frozen request. `RouteHold` retains the input and full intent, and the go-ahead re-enters the normal turn flow with that carrier. The saved intent includes the feature's restriction descriptor, which the turn subsequently reattaches. Preserve that behavior. [Held work creation, lines 322–330][hold-create], [reuse and restriction attachment, lines 168–219][reuse]

The defect is therefore not that approval should always invoke perception. The two request sources need different interpretation behavior while sharing the same downstream executive policy.

## The source chain

### The command creates an input without a saved intent

`handleCommand` routes `/go` directly to `approvePendingWork`. That function first takes `m.pendingWork`. If none exists and the current brief has a nonempty goal, it constructs:

```go
work = &pendingWork{Input: brief.Goal, At: time.Now()}
```

It then sets `m.goAheadWork = work` and calls `m.processInput(work.Input)`. `Intent` is a value field and is not initialized by this literal. [Command entry, lines 61–66][command], [fallback and dispatch, lines 76–98][fallback], [carrier definition][carrier]

The source has no missing-goal crash in this branch: when neither held work nor a nonempty goal exists, it returns the existing “nothing waiting” message. That is a useful negative control, not the failure case.

### The consumer uses carrier presence as an interpretation discriminator

Inside the returned command, `processInput` declares a zero-value local intent and then does:

```go
reusedIntent := m.goAheadWork != nil
if reusedIntent {
    intent = m.goAheadWork.Intent
}
```

Both the PerceptionFirewall call and the fallback `ParseIntentWithContext` call require `!reusedIntent`. Starting Jev turn judgments and replacing the brief from a newly returned intent have the same guard. A brief-origin carrier is nonnil, so none of those interpretation operations run. [Intent selection and both perception routes, lines 168–211][reuse]

Resetting the caller's `m.goAheadWork` immediately after creating the command does not avoid the error. `processInput` has a value receiver and returns a closure over that receiver. The invocation receives the model value while the pointer is set; resetting the separate caller value afterward does not remove it from the command's receiver. This is ordinary Go value/closure behavior, not a race prerequisite. [Receiver and closure, lines 47–49][process-entry], [caller reset, lines 92–97][fallback]

The available Jev integration does not repair this branch. No new judgment starts, and `recordTurnJudgments` retracts its prior judgment predicates before returning on the nil channel. [Judgment replacement, lines 54–65][jev-clear]

### Routing receives empty interpretation fields and a go ahead signal

The resulting intent has empty category, verb and target, zero confidence, no suggested shard and no workspace-evidence signals. `resolveShardTypeForIntent` checks the verb corpus and then the intent's `shard=` suggestions; neither yields a candidate for this empty value. `decideRoute` therefore constructs `/none` with confidence zero. It nevertheless emits `/user_go_ahead` because `m.goAheadWork` is nonnil. [Resolver, lines 314–335][resolver], [candidate construction, lines 127–158][candidate], [go-ahead signal, lines 210–218][signal]

`/user_go_ahead` admits otherwise runnable work; it does not manufacture the missing verb, category, candidate or plan. Ordinary delegation requires a non-`/none` candidate and adequate confidence. Multistep work requires a `/mutation` intent and a runnable plan. [Admission and work lanes, lines 218–278][admission], [delegation confidence rule, lines 327–343][confidence]

Under a clean routing fixture with no independently applicable clarification or early-lane facts, these empty routing inputs produce no lane, and `decideRoute` returns `RouteNone`. The normal code path then proceeds toward articulation rather than the intended work delegation. This is not a schema-rejection claim: empty strings in the relevant name positions are not assumed to be refused. It is the absence of actionable interpretation facts that matters. [No-route handling, lines 229–240][no-route], [foreground delegation guard, lines 455–483][delegate-guard]

## Reachable witness and outcome limits

A focused source-derived witness is:

1. Start an initialized, idle feature TUI with the normal kernel, client and transducer.
2. Conversation or investigation establishes a current brief with a concrete goal such as fixing a specified typo in `README.md`, but no turn has installed `pendingWork`. For the first fixture, make the brief ready and give it no open questions to avoid conflating readiness with this bug.
3. Enter `/go`.
4. Observe that the command is created and loading starts, but neither perception entry point receives the goal. Routing receives the empty intent and `/none` candidate instead.

This does not depend on Jev being disabled, a provider returning malformed JSON, a thread switch, cancellation, a missing tool or a stale brief. Those conditions belong in later regression coverage, not in the minimal cause.

The final answer is not determined by this trace. The existing history and JIT-injected brief can still reach articulation. Other kernel state, reflection, autopoiesis suggestions, system actions or subsequent model behavior can change what appears. A model may produce useful prose about the goal. None of that demonstrates that the goal went through the promised perception-to-work path. Do not describe the defect as a universally blank response, total no-op or permission bypass. The precise lost guarantee is a classified request and its normally derived execution route.

Important counterexamples:

- **Real held proposal:** a populated saved intent is reused intentionally; the proposed repair must not re-perceive it just to fix the brief case.
- **No held proposal and no goal:** the existing explanatory response remains correct.
- **Plain natural-language “go”:** it goes through normal perception and the approval-lane rules; it is not the same command path. The current tests deliberately distinguish an approve verb with and without held work.
- **Main:** this feature-specific carrier and command are absent; do not classify this as a mainline regression.
- **Unready brief:** whether an explicit go-ahead overrides readiness is a separate policy question. This report preserves the current policy and does not use that question to explain the missing interpretation.

## Why the current test definitions do not establish this handoff

`TestWorkAdmission_GoAheadRunsHeldWork` creates a populated `Intent`, installs it in `goAheadWork`, asserts the route intent and calls `decideRoute` directly. It tests admission of already interpreted held work, not brief-origin command execution. [Test, lines 72–80][test-held]

`TestWorkAdmission_ApproveVerbTakesThePendingLane` tests the perceived approve verb and explicitly expects no pending-approval lane when nothing is held. It does not run the slash command's separate brief fallback. [Test, lines 82–97][test-approve]

`TestApprovePendingWork_ClearsTheTypedGo` checks input-box clearing and discards the returned command. The test can satisfy its stated UI contract without exercising perception or routing. [Test, lines 10–19][test-input]

The brief round-trip test verifies facts and injected text. That is useful evidence of storage and context availability, but cannot establish that `/go` invokes the classifier. [Test, lines 99–139][test-brief]

These are test-definition observations, not claims about executed test results or an exhaustive absence of every possible indirect test.

## Repair options and recommended contract

### Option A Explicit interpretation presence in the existing carrier

Keep the carrier but make saved-intent availability explicit, for example through an optional intent or a discriminator with checked constructors. Retain a separate host-issued go-ahead indicator. A brief-origin carrier has approval context but no saved interpretation, so the ordinary perception path runs. A held-origin carrier contains an admitted intent and preserves reuse.

This is the smallest coherent repair. A `Verb != ""` condition could close the immediate witness but infers state from one payload field; it does not state why an intent is reusable, and it makes malformed held work look like fresh work. Prefer an explicit representation over a magic-field test.

### Option B Tagged request sources with validated construction

Represent `held_proposal` and `brief_goal` as separate request variants or a tagged envelope, created only through narrow constructors. Define these invariants:

- Held proposal: source input and saved intent exist; interpretation is reused; its accepted restrictions remain attached.
- Brief goal: source goal and brief context exist; no saved intent is claimed; perception must run before work routing.
- Invalid combinations return an explicit internal/admission failure rather than silently substituting an empty intent or guessing a task.
- A go-ahead controls work admission only. It does not authorize arbitrary tools, waive verification or attest that perception succeeded.

This makes the state distinction easier to inspect and test than a boolean alone. It adds a small type/construction surface, not a new execution subsystem. Owner and brief-version fields can align with the existing ownership research, but repairing the zero-intent witness should not be held hostage to implementing the entire memory/session redesign.

### Option C Separate brief execution entry point

A dedicated entry function could always classify the goal and then enter the same shared turn pipeline. It is acceptable only if the downstream pipeline remains shared. Duplicating routing, restriction, cancellation or verification logic would create a larger drift risk than this defect warrants.

**Recommendation:** implement Option A with explicit, validated request-source semantics from Option B. Keep `processInput`'s normal firewall preference and direct fallback, and keep approval separate from whether an interpretation can be reused. The new interpretation must see the full brief context, not just a clipped or reconstructed goal string. Existing brief-fidelity and restriction designs remain dependencies for their own guarantees; this repair does not solve them by itself.

Update the conversation-first specification and the applicable CLI/perception capability rows before code. Record a checkable gap such as “brief-origin go-ahead receives an admitted interpretation before routing”; the label is proposed here, not an assertion that such a row already exists. No new model-facing prose should be hardcoded outside the existing JIT contract.

## Primary foundations and their limits

The [Go language specification on keyed struct literals](https://go.dev/ref/spec#Composite_literals) defines omitted fields as zero-valued, and its [function-literal section](https://go.dev/ref/spec#Function_literals) defines closure capture. The [Go FAQ on value receivers](https://go.dev/doc/faq#methods_on_values_or_pointers) explains the receiver-copy behavior. These establish the mechanics of the source trace. They do not establish the resulting user-visible wording or the semantic quality of a future perception response.

Strom and Yemini's [Typestate A Programming Language Concept for Enhancing Software Reliability](https://research.ibm.com/publications/typestate-a-programming-language-concept-for-enhancing-software-reliability), IEEE TSE 1986, distinguishes an object's ordinary type from the operations valid in its current state. The useful design inference here is modest: a structurally valid work carrier can still be in the wrong state for “reuse interpretation.” Representing that precondition explicitly prevents one pointer-presence check from standing in for it. This proposal does not add a typestate compiler, prove natural-language correctness or import the paper's stronger language guarantees into Go. It uses the principle to design a small checked boundary.

## Evaluation program not executed

### Layer one Deterministic entry path contracts

Use the real embedded routing policy and recording interpretation/execution adapters. Invoke `handleCommand("/go")`, execute the returned command, and feed returned messages through the relevant Update path. Do not merely populate a valid intent and call `decideRoute`; that bypasses the defect. Capture the goal passed to perception, session brief context, call count by entry point, resulting intent facts, route, actual task request and terminal disposition.

| Fixture | Required observation |
|---|---|
| Ready brief goal and no held proposal | Normal interpretation receives the goal and brief; no zero intent enters ordinary work routing. |
| Firewall present | Firewall preference is honored; a successful interpretation is not needlessly repeated by the fallback. |
| Firewall absent | Direct transducer receives the same goal and brief contract. |
| Valid mutation goal in propose, explicit and yolo modes | Host go-ahead survives fresh interpretation; the work is routed under existing verification and permission obligations, not held again solely because interpretation was fresh. |
| Read-only goal | Resulting query operation and restrictions survive admission; no writer is selected solely because the carrier was approved. |
| Held proposal with a complete saved intent | No fresh primary perception; the saved request and restrictions are retained. |
| No held proposal and absent or whitespace-only goal | Existing explanatory response; no task dispatch. |
| Invalid held carrier | Explicit rejection or recovery according to its contract; never silent zero-intent reuse. |
| Primary perception returns a hard error | No work dispatch; report the cause through the normal safe error path. |
| Primary perception returns its existing degraded classification | Observe the actual existing recovery policy; do not mislabel degradation as successful interpretation or claim this patch repairs classification-recovery gaps. |
| Jev enabled, disabled or failing | Correct source classification in every case; optional judgment availability cannot change whether a saved intent exists. |
| Command created, caller model subsequently changed | Captured request source is stable; clearing UI state cannot turn a fresh request into a reused intent or vice versa. |

For positive execution controls, use a bounded fixture workspace and registered handlers with invocation counts. Keep normal world-model, CodeDOM, JIT and verification wiring enabled where the layer exercises them. A route assertion alone is not end-to-end evidence that the intended task reached a worker.

### Layer two Conversation to investigation to work

Run normal boot and TUI entry journeys with deterministic provider fixtures first, then representative real-provider trials under explicit authorization:

1. Discuss a change, perform a read-only investigation, retain a ready brief without holding a proposal, inspect `/brief`, then issue `/go`.
2. Hold an actual proposal and approve it, confirming that the repair does not reclassify the approval or replace the accepted task.
3. Build a brief containing constraints and decisions not repeated in its goal. Confirm those details reach the perception request and downstream task context.
4. Exercise a genuinely ambiguous goal and an unresolved brief question. Verify policy-selected clarification or admission, not forced execution merely to make the happy path pass.
5. Include cancellation and session-switch regression cases using the ownership specifications. Keep failures from the existing ownership gaps separately labeled rather than attributing them all to this narrow repair.

Measure source-path correctness separately from semantic interpretation, route correctness, actual task progress and verified completion. A passing fixture for call count cannot prove that a model understood the user's constraints. Conversely, an eloquent articulation about the goal cannot compensate for an omitted work handoff.

### Layer three Regression and acceptance

Once execution is authorized, run focused chat routing/work-admission/brief tests, relevant perception and session integration tests, and the repository-required aggregate checks. Include race checks if the chosen repair changes shared carrier or publication ownership. The current investigation ran none of these stages.

The mechanical exit is exact: **brief-origin go-ahead invokes the selected normal interpretation route before work routing; held-origin go-ahead reuses its valid saved intent; neither source can be mistaken for the other by pointer presence.** Every fixture must retain capability, exact permission, cancellation and verification behavior. Any new action outside the original brief's scope remains unauthorized.

## Cost accounting and decision criteria

The correction restores work that the baseline omits. Comparing raw tokens or latency alone would reward the broken path for not doing the task. Report both full end-to-end cost and cost per correctly admitted, verified outcome.

For each paired journey, account for:

- Primary perception requests, provider retries, input/output/reasoning tokens and cached-token classes under the provider's actual billing fields.
- Optional Jev judgments: requests, tokens, latency and failed attempts. Fresh brief work may now exercise the configured judgment path that the defective branch skipped; held work should not gain an unnecessary second judgment.
- JIT compilation, kernel queries/derivations, world-model and CodeDOM work, allocations and any additional carrier snapshot/storage bytes.
- Downstream execution, tool calls, verification and recovery attempts. Do not exclude these because the defective baseline never reached the task.
- Time to first useful status, time to work dispatch, time to verified terminal result, and extra user messages or manual correction time.
- Invalid or missing interpretation incidents, wrong-source reuse, duplicate interpretation, re-held approved requests, lost restrictions and unauthorized-dispatch attempts.

Keep a request ledger so retry and repair costs are not counted once as model calls and again as an unrelated repair subtotal. Compare identical provider/model settings, work modes, fixture state and context contents. Stratify results by held versus brief origin and by firewall versus fallback. For stochastic trials, report paired results and uncertainty rather than a single blended average that can hide a regression in held-work reuse.

Option A adds little mechanical state and can avoid new persistence work; it still needs careful construction discipline. Option B makes the invariant clearer and may simplify review, at the cost of changing more callers and test fixtures. Option C risks duplicate turn machinery and should be rejected if it bypasses the shared gates. No latency, token-saving or success-rate improvement has been measured here.

The repair is justified first by the deterministic contract: the documented brief-to-work path must not skip the operation that gives the kernel an actionable intent. Whether a broader typed handoff also lowers support burden or improves completion rate is a subsequent measurement, not a prerequisite for acknowledging the source defect.

## Implementation sequence

1. Amend the conversation-first and affected capability specs with the two request-source states and the checkable interpretation-before-routing exit.
2. Add a failing entry-path fixture that runs the command returned by `/go` for a brief-only request; retain populated held-work and missing-goal controls.
3. Separate saved-interpretation presence from host go-ahead, validate construction, and reuse the normal perception and downstream execution pipeline.
4. Verify request context, restrictions, task dispatch and terminal behavior through the real wiring, then perform the authorized regression suite.
5. Reconcile current-state/spec evidence with the actual results. Link this note from the prior session-memory audit as a distinct sequential handoff finding; do not replace or mark its ownership issues repaired.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/commit/cb195322f15418ce026f8f28f9ce3fd87a0b9f52
[historical]: https://github.com/theRebelliousNerd/codenerd/commit/c592ff223cd207fca4c29ae793d5eba57aa4781d
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md
[chat-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/agents.md
[perception-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/agents.md
[core-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/conversation-first-agent.md
[architecture-brief]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/conversation-first-agent.md#L9-L36
[session-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/chat-session-memory-ownership-and-recovery.md#L85-L91
[brief-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/conversation-brief-semantic-fidelity-and-progressive-disclosure.md
[restriction-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/perception-restriction-continuity.md
[persona-prior]: https://github.com/theRebelliousNerd/codenerd/blob/cb195322f15418ce026f8f28f9ce3fd87a0b9f52/Docs/audits/perception-query-persona-continuity.md
[side-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/side-turn-ordering-and-cancellation-ownership.md
[background-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/background-outcome-and-continuation-ownership.md
[classification-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/perception-classification-outcomes-and-recovery.md
[plan-prior]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/planned-work-conservation.md
[carrier]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_hold.go#L18-L27
[command]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands.go#L61-L66
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_hold.go#L76-L98
[brief-context]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_hold.go#L100-L105
[hold-create]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L322-L330
[reuse]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L168-L219
[process-entry]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L47-L49
[jev-clear]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/jev_turn.go#L54-L65
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L314-L335
[candidate]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L127-L158
[signal]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L210-L218
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/routing_arbitration.mg#L218-L278
[confidence]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/delegation.mg#L327-L343
[no-route]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L229-L240
[delegate-guard]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L455-L483
[test-held]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_admission_test.go#L72-L80
[test-approve]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_admission_test.go#L82-L97
[test-input]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_hold_input_test.go#L10-L19
[test-brief]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_admission_test.go#L99-L139

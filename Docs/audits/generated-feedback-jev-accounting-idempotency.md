# Feedback repair must not count one generated execution as several Jev observations

## Decision and bounded finding

**The generated-tool recovery path preserves the native effect but can repeat a learning side effect.** At main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), feedback-only retry reconstructs feedback from the retained receipt and evaluates quality again. A successful Jev output judgment immediately increments the separate tool-value ledger, before the durable LearningStore publication that the retry is repairing. That ledger increment has no execution identity or deduplication parameter.

Consequently, if the first Jev tally write succeeds but LearningStore publication fails, a later admitted repair of the same receipt can count that same execution again. The native process still runs once, and the eventual durable LearningStore aggregate can correctly count it once. The inconsistency is specifically in Jev's persisted landed/missed/unsettled evidence and consumers of that evidence.

**Required contract:** retries of one correlated execution must preserve one accounted tool-value observation. Separate obtaining an assessment, publishing its learning record, and applying its ledger contribution. If reassessment is intentionally allowed, model it as a revision of that execution's judgment—not as another independent tool execution.

This is static source analysis, not a reproduced incident. Main, guidance, the queue, active code branches and audit additions through `research-main-20261030-1246` were checked on 2026-10-30. Main and code/QA tips were unchanged; 1246 was [`58c84997ba39254e22c590f6cb3bab43a58509c1`](https://github.com/theRebelliousNerd/codenerd/commit/58c84997ba39254e22c590f6cb3bab43a58509c1), four documentation commits ahead. Independent source challenge corroborated the mechanism and the conditional classification example below. No application, tests, model inference, failure injection, benchmarks or repository changes were performed.

## Scope and prerequisites

The witness requires all of the following:

- A generated process actually started, returned without a backend error, and passed the actual named-result validator. Failed process or failed validation receipts receive the evaluator's early 0.1 failure assessment and do **not** take this output-judgment route.
- Jev is configured and `AutopoiesisChecks` is enabled. This per-output path uses that switch, not the separate `ToolChecks` switch used by other tool judgments.
- Jev returns a valid output judgment; the kernel can classify its call-level probability; the ProfileStore tally write succeeds.
- The later LearningStore publication fails, leaving a matching live receipt with a feedback error.
- The same request identity/fingerprint reaches the supported feedback-repair transition while the adapter is open, its receipt is retained, and applicable authorization/identity checks admit the request. The retry gets another valid judgment and another successful tally write.

Not every failed call or ordinary user retry meets these conditions. A new execution identity is new work, not this replay. A successfully acknowledged duplicate returns its retained receipt without reevaluation. A changed fingerprint, missing live receipt, canceled context, closed adapter, or changed registered identity can prevent repair. This report does not claim that every frontend automatically schedules the exported retry method, or that all provider retries reuse a tool-call identity.

## Exact source chain

### 1. The repair path correctly avoids a second process

The factory's duplicate-request branch checks the fingerprint, waits for the retained call, and, when a completed started call has a feedback error, calls `RetryGeneratedFeedback`. The repair method locks that call's receipt, returns immediately for an already durable successful publication, otherwise clears the old feedback error and invokes `recordGenerated` again. It never dispatches the backend. [`factory_tool_executor.go:90–116`][duplicate], [`factory_tool_executor.go:173–224`][repair].

VirtualStore's explicit feedback bridge separately validates the request, current registry identity, and exact authorization before calling the retrier. Preserve these checks; a retained historical outcome is not ambient permission. [`virtual_store_generated.go:107–134`][bridge].

### 2. Evaluation is repeated before durable deduplication

`RecordGeneratedExecution` creates a fresh `ExecutionFeedback` from the receipt, including `ExecutionID = request.Key()` and `RequestFingerprint = request.Fingerprint()`. It then unconditionally calls `EvaluateWithProfile` before `RecordExecutionDurable`. The quality assessment is not recovered from a retained completed assessment at this boundary. [`autopoiesis_profiles.go:133–163`][record].

Both ordinary and profile-aware evaluators return early for unsuccessful feedback. For successful feedback they call `outputIssues`, which invokes the attached output judge. The constructor attaches that judge from the configured Jev settings. [`quality.go:221–263,269–318`][quality], [`autopoiesis_orchestrator.go:329–332`][attachment], [`autopoiesis_jev.go:29–48,63–85`][judge].

`toolOutputJudge` asks `JudgeToolOutput` under `TurnJudgmentTimeout`. On a valid response it immediately calls `recordCallHit` using the `OutputGivesWhatWasAsked` probability, then returns the output judgments to the quality evaluator. Thus evaluation is not side-effect-free. [`autopoiesis_jev.go:72–84`][judge-call].

### 3. Identity is used for classification, then lost at accounting

`recordCallHit` does carry the execution ID into `tool_call_hit(Tool, Call, Percent)`, and reads back the kernel's landed/missed classification for that exact pair. It removes the temporary observation and calls `ledger.NoteJevCall(f.ToolName, outcome)`. The accounting call receives neither execution ID nor request fingerprint. [`tool_value.go:39–101`][call-hit].

`NoteJevCall` increments `Landed`, `Missed`, or `Unsettled` on every invocation. Its stored evidence consists of aggregate counts and recency, not a set of already-accounted execution identities. The file itself states that each call should be counted once; this is an existing semantic contract, not a new preference for universal exactly-once systems. [`tool_ledger_jev.go:13–29,46–95`][ledger].

The ProfileStore mutex, cross-process file lock, read/merge/write, and atomic replacement are real protections. They prevent ordinary competing writers from dropping independent counts when the lock/write path succeeds. They cannot deduplicate two serialized increments that lack identity. This witness needs only one process and successful profile persistence; it is not a claim that cross-process coordination is generally absent. [`tool_ledger.go:97–147`][ledger-write].

### 4. The LearningStore's working guard occurs later

Only after evaluation does `RecordExecutionDurable` call `RecordLearningDurable`. That store checks `ExecutionID` and fingerprint under its lock, stages updates, and swaps its in-memory state after successful disk publication. Its identity guard is real, but it cannot undo a Jev tally already written to another file. The PatternDetector also has its own replay guard after the durable acknowledgment. [`autopoiesis_feedback.go:54–70`][durable-wrapper], [`feedback.go:373–383,461–497`][durable-store].

The two files have independent locations and publication boundaries: learning under `.nerd/tools/.learnings`, profiles/ledger under `.nerd/tools/.profiles`. A failure confined to the learning destination can leave the earlier profile write intact. A whole-disk failure that prevents both writes is not the required witness. [`autopoiesis_orchestrator.go:277–300`][stores], [`tool_retirement.go:27–29`][profile-dir].

## Minimal sequential witness

Assume a valid execution E satisfying the prerequisites above, with no prior learning acknowledgment for E:

| Step | Native effect | LearningStore contribution for E | Jev ledger contribution for E |
|---|---:|---:|---:|
| E executes and passes validation | 1 | 0 | 0 |
| First successful judgment is classified and profile write succeeds | 1 | 0 | 1 |
| Learning publication fails | 1 | 0 | 1 |
| Storage is repaired; same-identity feedback retry evaluates again | 1 | 0 | 2, if the second judgment and profile write succeed |
| Learning publication succeeds | 1 | 1 | 2 |
| A further duplicate returns the now-durable receipt | 1 | 1 | still 2 |

Both judgments may return the same probability; model nondeterminism is unnecessary. If they differ, one native execution can instead contribute to both sides of the tally. A cache returning the same judgment would not cure the missing accounting identity. Reentering the judgment seam can also add request work, but this review does not claim a particular number of provider-billed requests or measured cost.

The first failure can be an ordinary publication error after a healthy assessment, not corrupted source or a deliberately fabricated receipt. Existing tests already model a learning-destination failure separately from execution; the missing case is the successful process/validation branch with Jev enabled.

## Why the extra count matters to a live consumer

This ledger is not merely a debug counter. `ToolLedgerFacts` projects its Jev tallies beside measured call counts, and `EvaluateToolValue` reads the kernel's value classes, reasons and retirement proposals. `/tool value` renders that report. [`tool_ledger_facts.go:55–111`][facts], [`tool_ledger_report.go:90–178`][report], [`cmd/nerd/chat/tool_value.go:12–34`][surface].

Stock policy treats landed greater than missed as `/hits_mark`, missed greater than landed as `/misses_mark`, and a tie as neither. The tool-value policy requires both enough judged observations and enough metered calls before its judged-evidence classification. Default `MinJudgedCalls` is 10 and `MinHitPercent` is 80. These floors matter: counting one call twice is **not by itself** enough to classify a new unused tool as dead. [`jev_learning.mg:118–136`][judgment-policy], [`tool_value.mg:107–174`][value-policy], [`config/tool_value.go:34–63`][thresholds].

A source-derived classification example, independently challenged:

- A still-registered generated tool has at least ten genuinely metered calls already, no unique-capability declaration, and prior settled Jev counts of five landed/four missed. It has no other dead reason relevant to the comparison.
- E succeeds mechanically but Jev reads its output as not giving what its input asked for. That is a valid distinction: semantic usefulness need not equal process/validator success.
- Its first missed observation makes the genuine history five/five: no majority evidence.
- Learning publication fails; the same receipt's admitted repair yields the same missed judgment and counts it again. The ledger becomes five/six, satisfying the missed-majority and sample premises.
- A subsequent report can therefore derive `/misses_results`, `/dead`, and a generated-tool retirement **proposal** because of the duplicated observation.

This example does not fabricate a sixth independent miss or lower the default floors. It is conditional on actual prior calls, legitimate settled judgments, current registration, absence of a unique declaration, and healthy policy publication. It does not assert that the tool was objectively useful, that Jev was right, or that every double count crosses a decision threshold.

**Nothing here removes a tool automatically.** Retirement still requires explicit invocation, a current proposal, an approval broker/grant under the selected work mode, reservation/dispatch, and registry/artifact checks. Source is retained. The finding is evidence integrity at a decision input, not a demonstrated approval bypass or observed unwanted deletion. [`tool_retirement.go:131–222`][retirement].

## Repair contract and alternatives

Update the affected autopoiesis/tool-value capability specification before implementation. Preserve process, validation, assessment and publication as distinct outcomes.

### A. Retain the assessment; make its contribution idempotent

Retain the completed quality assessment and typed Jev observation with the joined execution receipt before attempting learning publication. Feedback-only retry can then resume publication instead of purchasing and applying the assessment again. Carry the execution identity, request fingerprint, registered tool identity, judgment outcome/probability, and judgment configuration/provenance needed to distinguish replay from intentional reassessment.

At the durable ledger boundary, apply the contribution under an execution-scoped idempotency key and persist that key atomically with the tally mutation. Same key/same observation is a replay; conflicting evidence needs an explicit revision or conflict outcome, not another increment. Existing file locking/read-merge-write remains necessary. An in-memory flag alone handles only its live owner, not process failure between durable increment and flag update.

### B. Durable assessment plus an idempotent projection/outbox

Alternatively, record the assessment and pending tool-value contribution with the durable execution learning, then apply it to the ProfileStore with acknowledgment and replay-safe identity. Publication failure leaves a retryable projection, not a new judged call. A durable LearningStore acknowledgment and a successfully applied tool-value projection remain separately observable; do not silently redefine the existing acknowledgment to promise both files committed atomically.

Simply moving `NoteJevCall` after the store write is insufficient: a failure between the ledger mutation and its acknowledgment can still double-count on replay, while skipping all replayed learnings can strand an unapplied contribution. The recovery protocol must close both directions without rerunning the native process.

### Deliberate reassessment is not prohibited

There may be a valid reason to judge an old execution again after changing the model, rubric or thresholds. Give that operation an explicit assessment revision and accounting rule. For example, select one current qualified judgment per execution and replace its contribution, while retaining older assessments for audit. Do not represent two assessments of one execution as two independent executions. Missing, failed and unsettled judgments must retain their existing distinct semantics.

### Migration and retention

Current aggregate Jev tallies do not retain the identities necessary to prove which historic increments were retries. Do not “repair” them by dividing counts, clamping them to metered calls, or guessing missing IDs. Preserve legacy totals with explicit qualification, start identity-qualified accounting at an announced epoch, or rebuild only where independently retained evidence actually proves each contribution. Define how legacy evidence may influence classification during migration.

New idempotency metadata has a retention cost. Its lifetime must cover the supported replay/reconciliation window; deleting keys merely because an arbitrary cache is full can make an old receipt count again. Keep compact IDs/digests and outcomes rather than another unbounded copy of all inputs/outputs, and coordinate retirement with the existing receipt/evidence lifecycle. Do not evict an unacknowledged receipt to hide the retry problem.

## Costs, controls and fair evaluation

### Costs and protections

- Reusing an assessment can avoid repeated judgment setup and requests during feedback repair. The current judgment already shares the output questions and is bounded by input/output truncation and `TurnJudgmentTimeout`; retain those limits. No additional model is needed to identify replay. [`learnjudge/autopoiesis.go:129–195`][question], [timeout boundary][judge-call].
- Execution-keyed accounting adds metadata, serialization and recovery operations. A persistent uniqueness check or outbox consumes storage and introduces migration/cleanup work; a volatile cache is cheaper but has a narrower guarantee.
- Preserve the ProfileStore's healthy cross-process lock/read-merge-write and atomic file replacement. A lock alone orders duplicate writes but cannot identify them. A cross-file outbox adds intermediate states and failure handling; do not describe two independent JSON writes as one transaction.
- Keep the existing native execution rendezvous, conflict detection, retained terminal output, no-effect feedback repair, exact authorization, cancellation/join, corruption handling and restart reconciliation refusal. No recommendation here authorizes a second backend run or automatic retirement.
- The per-execution LearningStore count and PatternDetector replay guard are working controls, not absent features. The repair should extend correlation to Jev accounting rather than weakening those controls or suppressing legitimate new executions.

### Proposed evaluation — not run

1. **Successful native effect, failed learning publication.** Through the generated factory/VirtualStore route, execute a real admitted fixture that passes the actual named validator. Enable a deterministic Jev response through the real judgment seam. Allow profile persistence but fail only the subsequent learning publication. Reopen the ProfileStore to establish the first tally before repair; do not infer persistence from an in-memory counter.
2. **Same-identity repair.** Restore learning publication and exercise the actual duplicate-to-repair transition, retaining current authority and registration identity. Observe one native effect, one eventual LearningStore execution, one accounted Jev observation, and the expected assessment-request count. Reopen both stores. A stubbed successful receipt is not the primary test.
3. **Controls that distinguish the branch.** Cover failed backend, failed validation, pre-start refusal, Jev disabled, failed/unsettled judgments, successful acknowledged replay, new execution ID, conflicting fingerprint, missing receipt, changed registered identity, canceled retry and shutdown. Failed process tests alone do not exercise the successful-output judgment side effect.
4. **Classification at stock thresholds.** Produce the five/four history and at least ten metered calls through legitimate measurement paths, then the failed-publication/repaired missed outcome. Compare the real derived evidence, class, reason and proposal with a control that counts E once. Also cover repeated landed observations and a valid unique capability. Do not judge correctness only from final rendered text.
5. **Crash/publication interleavings for the chosen design.** Use deterministic barriers around assessment retention, learning commit, ledger uniqueness/tally commit and projection acknowledgment. A completed ledger write with a lost acknowledgment must replay without another increment; a missing ledger application must remain repairable. Include concurrent same-key replay and independent executions, without global model-call serialization.
6. **Fair cost accounting.** Compare identical successful effects, failure schedules and admitted judgments. Report native starts, assessment attempts and actual provider requests/tokens, ledger writes, duplicate-suppression counts, projection lag, repair latency, metadata bytes, lock contention and report/classification accuracy. Do not count suppressed legitimate assessments or abandoned failed-publication cases as savings.

Existing tests deserve credit. `TestGeneratedFactoryDuplicateFailureAndFeedbackRetry` proves retained-effect repair and restart refusal, but the request remains configured for backend failure, which takes the early quality path rather than output Jev. `TestToolValue_PerCallTruthTable` uses distinct IDs and checks policy evidence. Jev ledger tests establish persistence and merge of counts, not replay idempotence for an execution identity. Extend these boundaries rather than claiming they prove nothing. [Factory recovery test][recovery-test], [per-call truth table][truth-test], [ledger reload test][reload-test].

## Prior coverage and nonduplication

The [effect-receipt audit][effect-audit] already credits generated live-call deduplication and feedback-only recovery. This report preserves that repair and identifies the additional side effect inside its recovery callee. The [receipt-retention audit][retention] owns coordinated metadata retirement; the [multiwriter publication audit][multiwriter] owns stale learning snapshots across owners; the [aggregate-supersession audit][aggregate-audit] owns obsolete `tool_learning` rows; and the [refinement proposal-owner audit][proposal-audit] owns source/candidate continuity. This witness requires none of those failures and does not claim a system-wide exactly-once guarantee.

**Acceptance boundary:** one correlated generated execution contributes at most one selected Jev tool-value outcome, even when its durable feedback publication is repaired repeatedly. Legitimate new executions and explicit reassessments remain observable, and policy/approval—not retry count—continues to decide what a tool earns and whether it may be retired.

[duplicate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L90-L116
[repair]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L173-L224
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/virtual_store_generated.go#L107-L134
[record]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L133-L163
[quality]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L221-L318
[attachment]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_orchestrator.go#L329-L332
[judge]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_jev.go#L29-L85
[judge-call]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_jev.go#L72-L84
[call-hit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_value.go#L39-L101
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_jev.go#L13-L95
[ledger-write]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger.go#L97-L147
[durable-wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_feedback.go#L54-L70
[durable-store]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L373-L497
[stores]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_orchestrator.go#L277-L300
[profile-dir]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L27-L29
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_facts.go#L55-L111
[report]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_report.go#L90-L178
[surface]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/tool_value.go#L12-L34
[judgment-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/jev_learning.mg#L118-L136
[value-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/tool_value.mg#L107-L174
[thresholds]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/tool_value.go#L34-L63
[retirement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_retirement.go#L131-L222
[question]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/autopoiesis.go#L129-L195
[recovery-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_generated_feedback_test.go#L140-L250
[truth-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_value_test.go#L33-L64
[reload-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_ledger_jev_test.go#L118-L149
[effect-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/tool-effect-receipts-and-validation-completeness.md
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/c3a596c8e9444a523b3368a25843cb3bad2de112/Docs/audits/generated-call-receipt-retention.md
[multiwriter]: https://github.com/theRebelliousNerd/codenerd/blob/921628ea7b1af635cb9c99d4055f6feed2a5efeb/Docs/audits/generated-learning-multiwriter-publication.md
[aggregate-audit]: https://github.com/theRebelliousNerd/codenerd/blob/58c84997ba39254e22c590f6cb3bab43a58509c1/Docs/audits/generated-learning-aggregate-supersession.md
[proposal-audit]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/generated-tool-refinement-candidate-continuity.md

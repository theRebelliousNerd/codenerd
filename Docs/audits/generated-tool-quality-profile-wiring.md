---
doc-class: deep-dive
subsystem: autopoiesis-system-cli
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 6cbe3962fbdb24a1639b26e71f8096e4f4330048
verification-method: static-source-inspection
supersedes: []
---

# Connect generated tool quality expectations to normal creation

## Finding and decision

Normal generated-tool creation publishes a tool without producing its tool-specific quality profile. Execution does consult the profile store, but a fresh name has no expectations, so the live durable-feedback path falls back to generic output-size, name-pattern and duration scoring. The profile generator, its atom-compiled role, persistence, evaluator and convenience wrapper exist; the missing connection is from ordinary successful generation to that producer. This is an integration finding, not a claim that all tools execute without validation or that every generic assessment is wrong. [Normal generation][generation], [profile producer][producer], [live consumer][receipt].

Define an explicit lifecycle decision for expectations: a validated profile for the committed version, or a visible generic/pending/unavailable mode with its reason. If purpose-aware assessment is a product requirement, connect its bounded producer to normal generation and preserve fallback when optional work cannot run. Do not silently add an unlimited model call, make a model-authored profile an execution permission, or block truthful terminal feedback on profile availability.

Initially inspected at `388205138ba6d47c1cff0c5e97c896dfa642a391`, this finding was revalidated against new main `6cbe3962fbdb24a1639b26e71f8096e4f4330048` at approximately 17:22 UTC. Every cited main source was refetched; the relevant source files and root/autopoiesis/system guidance are unchanged, so their line anchors remain valid. Repository guidance, main's audit queue and inventory, research through `research-main-20261030-1651` at `90c84648725aa263c8a1ba2458390b572402e4de`, and active branch heads were checked. This is static inspection only: no application, tests, compilation, model inference, benchmark or fault injection was run. The witness is source-derived, not an observed incident. Independent source challenge corroborated the missing producer edge and emphasized the existing-profile, ledger-only and optional-Jev controls retained below.

## Main advancement revalidation

The intervening main delta contains three commits and 52 changed files. `47994bfef396246aa7b27a7a73636b9c3ef74f01` scopes failed self-restart suppression to the executable identity and stages development policy programs, evaluation, routing and shared-fact hydration before group publication. `5efbc6782a44a45e19ad4d643dec2016ea7df3c9` preserves recurse check environment, allowed exits and workspace through campaign/task execution, adds invocation-bound receipt pass rules, and removes unsupported kind/language subsumption. `6cbe3962fbdb24a1639b26e71f8096e4f4330048` updates the queue and rebuilds the predicate corpus. All text patches, including new tests and as-built documentation, were reviewed; none adds a profile producer call. The queue now marks those four older findings fixed rather than queued. Commit messages report package suites passing, while author-stage appendices retain their earlier source-only qualifications; this review independently ran none. [Full delta][delta], [current queue][queue].

The changed binary `internal/core/defaults/predicate_corpus.db` was inventoried but not decoded: the read connector does not support binary contents and runtime execution is outside this task. This limits verification of that rebuilt artifact, not the source-level profile call-path conclusion. Historical audit citations below intentionally retain their original branch pins; current source citations use the new main pin.

## Intended behavior and existing implementation

The profile type's design comment explicitly distinguishes a slow background indexer from a slow calculator and says expectations are defined during generation. The actual profiler role asks for purpose-specific tool type, timing, output sizes and content requirements. This is more than an unused field or a speculative future idea. However, the existence of that role does not prove that every product route is intended to buy a profiling call; the first implementation decision should make that requirement or intentional opt-out explicit. [Profile contract][profile-contract], [profiler atoms][atoms].

The normal orchestrator creates a ProfileStore, passes it to QualityEvaluator and retains it. Existing profiles are loaded. `GetProfile` deliberately returns nil for an absent entry or one with an empty ToolType. Tool-value measurement can create just such a ledger-only entry; that is intentional separation between cost/use evidence and quality expectations, not a profile-generation mechanism. The proposed repair must not turn a built-in's ledger into invented expectations or discard its measured history. [Construction][construction], [store lookup][lookup], [ledger mutation][ledger].

`GenerateToolProfile` compiles the `tool_profiler` role and sends the name, purpose and bounded source excerpt to the client. A parsed profile is stored. Missing role compilation returns an error; model or parse failure returns a data-fetch default. `GenerateToolWithProfile` invokes the audited tracing wrapper, then the profiler, uses a nonfatal default if needed, stores the result, and annotates the trace. These APIs are functional but are not called by the normal generation routes identified below. Note a fallback asymmetry: the direct profiler returns its default without storing it, whereas the convenience wrapper stores it. Reusing either API requires understanding that behavior rather than assuming every successful return means durable profile adoption. [Producer and wrapper][producer].

## The missing production connection

1. **Ordinary facade.** `Orchestrator.GenerateTool`, documented as the production chat/corrective-action entry, checks admission and calls `ExecuteOuroborosLoop`. That method refreshes learnings, executes the audited loop, publishes registration/hot-reload facts on success, records generation learning and returns. It never calls the profile producer. [Facade][generation], [loop facade][loop].
2. **Explicit operator routes.** CLI generation calls `ExecuteOuroborosLoop` directly. TUI `generateTool` detects the need and does the same. Neither switches to `GenerateToolWithProfile` after a successful result. [CLI][cli], [TUI generation][tui-generate].
3. **Registration callback.** The orchestrator's registered-tool callback asserts registration facts; it does not supply a hidden profile producer. [Callback][callback].
4. **Producer reachability search.** At the main pin, symbol search found `GenerateToolWithProfile` only at its definition and a comment. `GenerateToolProfile` is called by that wrapper and tests. This negative search supports the positive entry-point trace; it is not a mathematical proof against every external library consumer.
5. **Boot coverage does not close this gap.** `TestBootCortex_AutopoiesisCompilesRolePromptsFromAtoms` boots a real Cortex and then manually calls `GenerateToolProfile`. It demonstrates role-compiler injection and atom selection. It does not generate a tool through the ordinary route and observe automatic expectations production. Keep that test and add the missing root-to-consumer test. It was read, not run. [Boot test][boot-test].
6. **Execution consumes the absence.** `RecordGeneratedExecution` builds feedback from the actual typed receipt, evaluates with `EvaluateWithProfile`, then records it durably. The profile lookup selects generic evaluation on nil. Recording updates the learning aggregate, patterns, kernel projection and generation learning context. This consumer is live even when the expected producer was never called. [Typed receipt][receipt], [selection][selection], [recording][recording].

Fresh state is important. An external library caller can explicitly set or generate a profile; a preexisting persisted profile is loaded and used. Those are valid negative controls, not evidence that normal creation supplies one.

## Bounded source-derived witness

Use a fresh workspace/name, with generation admitted and all normal safety, compilation and registration gates successfully completed. No imported or manually set quality profile exists. The generated tool performs a small pure calculation, such as summing its input, and returns the scalar text `42`. A later admitted invocation really starts, completes without backend error and passes the actual validator. The runtime unwraps the protocol envelope and unquotes a string result, so a valid wrapper output can become exactly those two feedback bytes. [Output decoding][decode].

For a name `sum_values`, the generic name hints do not match. With a two-byte output, duration below one second and no output issues, the current code assigns:

- completeness `0.3`, because an unmatched output below 100 bytes is considered suspiciously small;
- efficiency `1.0`;
- overall score `0.3 × 0.4 + 1.0 × 0.2 + 0.4 = 0.72`.

For an otherwise equivalent purpose named `find_max`, the name matches the search hint's 1,000-byte minimum; the completeness ratio is clamped to `0.1`, and the corresponding issue-free score is `0.64`. These calculations are direct consequences of the code under the stated conditions, not measured task quality. They show that the missing producer leaves the live assessment dependent on generic size/name assumptions rather than the tool's actual purpose. [Hints][hints], [generic dimensions][generic], [formula][formula].

The small scalar does not match the built-in empty/minimal pattern. If Jev output judgment is enabled, condition the example on its reporting no relevant issue; if disabled or unavailable, the pattern fallback has no matching issue for `42`. This distinction matters: current output judgment is a real mitigation, not an absent subsystem. It contributes issue evidence but does not replace the generic completeness or duration calculation. [Issue patterns][patterns], [Jev selection][jev].

A manually supplied quick-calculation profile is a useful consumer control. Its default expected output interval includes two bytes, and the profile evaluator uses that interval rather than the 100-byte/name heuristic. This does **not** establish that a live profiler will choose correct expectations, nor that its composite score will necessarily be higher or better calibrated. The generic and profile branches use different weights; the profile branch also includes an Accuracy component that this method does not populate. Automatically wiring the existing producer is not by itself proof of improved score semantics. [Quick-calculation default][quick], [profile scoring][profile-scoring].

## Consequences and limits

The durable runtime feedback receives the generic assessment, and later aggregate, issue and prompt-learning consumers can use it. TUI `/tool run` independently uses the same lookup/fallback distinction and displays the numeric score and completeness without displaying the evaluator provenance in that result. The assessment structure itself does retain `EvaluatedBy`; this is not a claim that all provenance is absent. [Recording][recording], [TUI result][tui-run], [generic assessment][formula].

Purpose-specific required-field, required-content, pagination and timing expectations are likewise not consulted when the profile is absent. Those existing checks are heuristic/string-based, not a full schema proof. The established gap is missing expectation coverage, not proof that a particular malformed result will be accepted by every validator or judgment route. [Profile checks][profile-checks].

The stock policy has minimum-sample and average-quality bands, plus separate known-incomplete/pagination triggers. Generic assessment can therefore matter to later policy, but the short-output example alone does **not** establish a poor-quality classification or automatic refinement: scores 0.72 and 0.64 are above the poor threshold, and low completeness alone does not add an incomplete issue here. Generation/runtime population mixing and stale aggregate projection are separate existing audits. Do not borrow those findings to exaggerate this one. [Policy][policy], [event-domain audit][event-domain], [projection audit][projection-audit].

Even a genuine refinement-eligible state needs lifetime admission, usable JIT and a working client before purchasing a completion. The current factory discards the returned proposal and does not automatically replace a binary. That separate proposal-owner gap is unchanged. Missing profile production does not bypass permission, process validation, rollback, durable acknowledgment or explicit retirement approval. [Factory trigger][trigger], [bounded refinement][refinement], [proposal-owner audit][proposal-audit].

No measured degradation, extra provider spend, wrong user answer or improvement from repair is asserted. Generic scoring may be adequate for some tasks, and a bad generated profile may be worse. A correct outcome may be to retain generic mode deliberately while clearly labeling its limits.

## Bounded repair contract

### Give expectations an explicit lifecycle owner

Place the decision at a shared successful-generation lifecycle boundary, rather than fixing only one CLI command or the TUI. Preserve the complete audited generation route. Calling `GenerateToolWithProfile` from inside a function that its tracing wrapper calls could recurse into generation; the producer-only operation must be separated cleanly from orchestration.

Associate a proposed profile with the committed tool identity and source used to infer it. Current profiles are keyed only by name. A delayed profile for an older version must not overwrite expectations for a newer same-name publication. Snapshot source/identity under appropriate coordination, release before model work, then compare the identity on publication; do not hold a global or per-tool execution lock through a model completion. Exact representation and compatibility rules are a design choice, not an existing capability.

The result should distinguish at least purpose-specific, explicitly selected default, generic pending, generic unavailable and superseded. Record producer version/provenance and failure reason with bounded metadata. A data-fetch default is not a purpose-specific inference for every calculator or background task. No new state should falsely certify correctness.

### Keep optional work within existing authority and budgets

Use the existing atom-compiled role, scheduled client/context and explicit per-operation budget. Bound source and response sizes, profile fields, regex complexity, duration/size ranges, retries and concurrency. Avoid a profile model call on every execution. Coalesce equivalent requests by exact tool version; profile failure must not regenerate the binary or turn a completed effect into a retry.

Choose synchronous versus owned background profiling explicitly. Synchronous work adds generation latency. Background work needs admission, cancellation, join, bounded queueing and a defined pending interval during which calls still record truthful generic feedback. Optional profile failure must not prevent the started execution's terminal durable acknowledgment. If product policy requires expectations before use, implement a separate visible readiness state rather than weakening the acknowledgment contract or reporting generation failure after an already published tool without saying what happened.

Model-supplied caching, timeout or side-effect declarations remain advisory assessment metadata unless a separate authorized contract promotes them. They must not grant network/process access, change approved execution bounds, enable unsafe caching or expand runtime authority.

### Preserve publication and measurement contracts

Profile publication needs an observable success/failure boundary if the lifecycle promises retained expectations. The current `SetProfile` is void and calls `save`; merely receiving a generated profile is not proof a reopened store contains it. Existing save already logs errors and uses atomic file replacement, which protects against publishing a partial file; preserve those controls rather than claiming all persistence is absent. Preserve existing measured Value when replacing expectations. Coordinate with the existing cross-process ledger read/merge/write behavior instead of claiming the repository has no such locking or introducing a second writer that loses counters. [Profile replacement][lookup], [save behavior][save], [existing ledger synchronization][ledger].

Do not overwrite old feedback to make historic scores look purpose-aware. New assessments should name the expectation revision/mode they actually used. Reassessment, if desired, is a separate bounded operation over retained evidence and must not replay the native tool or double-count an execution. Keep the event-domain and current-projection repairs independent.

### Roll out without pretending old profiles are current

Start by exposing mode/provenance and measuring how many ordinary successful generations produce usable expectations. Add opt-in or policy-selected production only after the normal path is covered. Legacy name-only profiles need an explicit compatibility policy: retain as legacy/advisory, validate against the current version, or mark unknown. Do not silently attach them to every future same-name binary.

Backfill must be explicit and budgeted, not an unbounded startup sweep. Limit by admitted tool/version, concurrency and total spend; expose pending/failure without suppressing ordinary feedback. Keep built-in and ledger-only records separate. A rollback should return to explicit generic mode while preserving raw outcomes, existing profiles and value ledgers.

## Costs and fair evaluation

A connected producer adds a role compilation and model completion per admitted new version, source disclosure to the configured provider, latency, tokens and possible retries. Even with the existing 12,000-character source clamp, prompt plus reply cost is nonzero and truncation may omit important implementation context. Deterministic purpose categories or explicit operator profiles are alternative producers worth comparing; universal automatic profiling is not the only valid design.

Identity/provenance adds metadata, migration and invalidation work. Background ownership adds lifecycle complexity. Synchronous persistence and coordination add I/O and contention; avoid turning profile work into a global execution bottleneck. Better expectation coverage can increase justified issue detection/refinement work, while poorly calibrated expectations can create false alarms. No net savings or quality percentage is known.

Proposed evaluation, **not performed here**:

| Case | Required observation |
|---|---|
| Normal fresh creation through real CLI/session facade | Profile mode is produced by the actual shared lifecycle, not by a test calling SetToolProfile or GenerateToolProfile directly. Capture the profiler request and exact committed source/version association. |
| Started execution through factory and durable recorder | Observe selected evaluator/mode, assessment dimensions, reopened feedback and actual kernel/prompt consumer state separately. The binary runs once. |
| Small calculator, search with genuinely short complete result, background task, paginated fetch | Evaluate purpose-specific coverage against independently known task results; compare generic mode and correct/wrong/default profiles without assuming all profiles are better. |
| Producer disabled, absent role, malformed reply, timeout, canceled request, persistence failure | Visible fallback/pending/unavailable state; bounded work; terminal execution feedback still records truthfully and is not retried by rerunning the tool. |
| Failed generation, reused existing tool, same-name replacement and delayed old profile | No profile for an uncommitted candidate masquerades as current; no unnecessary generation/profile duplication; stale publication cannot overwrite the new version. |
| Existing imported profile and ledger-only built-in | Positive control for existing profile consumption; correct nil fallback and preserved measurements for ledger-only records. |
| Concurrent sessions and store reopen | Profile identity/provenance remains coherent, measured ledgers survive, persistence failures are observable and no instance silently certifies an unretained result. |
| Policy and proposal controls | Preserve sample thresholds, independent issues, lifetime/JIT admission and proposal-only adoption. Do not infer replacement from a changed quality number. |

Compare matched tasks, provider/model, tool implementation, authority, initial history and budgets. Report profile coverage and correctness, fallback frequency/reasons, score calibration by tool purpose, false issue rates, downstream decision accuracy, final task quality, all added model calls/tokens, end-to-end latency, I/O, concurrency impact and backfill cost. Separate a better-informed assessment from an improved tool or final answer. Include tasks where generic mode already works and where profile inference fails.

## Novelty and acceptance boundary

The older wiring document correctly established the profile-wrapper-to-tracing relationship but did not establish an ordinary external caller; this report traces the missing root edge and the live feedback consumer. The historical audit already reports profile persistence error handling; this is not a new claim about ignored writes. The recent event-domain audit owns mixing generation outcomes into execution statistics, the aggregate-supersession audit owns stale current projections, and the proposal-owner audit owns unconsumed refinement proposals. None supplies expectations during normal generation. [Wiring review][wiring], [historical audit][old-audit], [event-domain audit][event-domain], [projection audit][projection-audit], [proposal-owner audit][proposal-audit].

Acceptance is not “every tool has a JSON file” or “the score rises.” An ordinary newly committed tool must reach the declared expectations lifecycle, and its later assessment must use compatible purpose-specific evidence or honestly labeled fallback under bounded cost. Execution authority, truthful failures, durable feedback, measured ledgers and governed adoption remain unchanged.

[generation]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_tools.go#L38-L75
[loop]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_tools.go#L128-L147
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_profiles.go#L166-L231
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_profiles.go#L133-L163
[profile-contract]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/profiles.go#L13-L46
[atoms]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/prompt/atoms/autopoiesis/tool_roles.yaml#L42-L133
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_orchestrator.go#L277-L305
[lookup]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/profiles.go#L148-L182
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/tool_ledger.go#L97-L145
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/cmd_advanced.go#L1025-L1056
[tui-generate]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/helpers_tools.go#L171-L240
[callback]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_orchestrator.go#L323-L327
[boot-test]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/system/role_prompts_autopoiesis_test.go#L156-L195
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_profiles.go#L18-L40
[recording]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/autopoiesis_feedback.go#L37-L95
[hints]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality.go#L193-L217
[generic]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality.go#L563-L615
[formula]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality.go#L221-L265
[patterns]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality.go#L152-L190
[jev]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality_jev.go#L30-L75
[quick]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/profiles.go#L184-L216
[profile-scoring]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality.go#L269-L397
[profile-checks]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/quality.go#L399-L463
[tui-run]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/helpers_tools.go#L119-L166
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/autopoiesis.mg#L101-L137
[trigger]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/system/factory_tool_executor.go#L148-L160
[refinement]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/system/factory_tool_executor.go#L227-L265
[event-domain]: https://github.com/theRebelliousNerd/codenerd/blob/3ef34bc061b497b9cfc471f4cc46f5df595b299d/Docs/audits/generated-learning-event-domain.md
[projection-audit]: https://github.com/theRebelliousNerd/codenerd/blob/58c84997ba39254e22c590f6cb3bab43a58509c1/Docs/audits/generated-learning-aggregate-supersession.md
[proposal-audit]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/generated-tool-refinement-candidate-continuity.md
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/architecture/autopoiesis/WIRING-AND-NOT-BUILT.md#L48-L152
[old-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/AUDIT.md

[decode]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/runtime_registry.go#L478-L520
[save]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/autopoiesis/profiles.go#L364-L389
[delta]: https://github.com/theRebelliousNerd/codenerd/compare/388205138ba6d47c1cff0c5e97c896dfa642a391...6cbe3962fbdb24a1639b26e71f8096e4f4330048
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/ASTRA_QUEUE.md#L131-L144


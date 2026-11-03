---
doc-class: research-proposal
subsystem: autopoiesis-learning-evidence
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Generation outcomes must remain distinct from runtime tool quality

## Finding and decision

Generation feedback and production execution feedback enter the same per-tool learning aggregate without an event-kind boundary. A successful generation contributes a quality score of 1.0; a specification-stage generation failure contributes 0.2. Both increment `TotalExecutions`, affect `SuccessRate` and `AverageQuality`, and become `tool_learning` facts. The stock policy interprets those totals as tool-quality evidence and uses them to derive refinement eligibility. Consequently, unsuccessful attempts to produce a replacement can label the still-unchanged installed tool poor, even when its only production invocation succeeds. [Generation producer][generation], [aggregate][aggregate], [quality policy][policy].

This is not a proposal to stop learning from generation. That learning is intentional, useful, and explicitly tested. The missing contract is a distinction between **how well the generator produced a candidate** and **how the admitted tool performed when invoked**. Preserve both histories and their appropriate consumers; do not make a failed candidate or generation-service failure an execution sample for the incumbent tool.

The established consequences are mixed-domain counts, quality bands and conditional refinement eligibility. A later real generated call can reach the automatic refinement requester after successful durable feedback. An actual model completion additionally requires live lifetime admission, usable JIT compilation and a successful client path. The returned refinement remains a proposal; this finding does not establish automatic replacement, removal, permission bypass, bad generated code, or measured excess spend. [Runtime trigger][trigger], [refinement request][request], [prior proposal-owner audit][proposal-audit].

## Review status and novelty

Main was rechecked at approximately 16:14 UTC on October 30, 2026 and remained `388205138ba6d47c1cff0c5e97c896dfa642a391`. Root and scoped system, session and autopoiesis guidance, main's audit inventory and queue, and the additions through `research-main-20261030-1548` were inspected. The latter tip is `e69f9f921f1b4a7318fafbf5c82834e75ecebc7f`, four documentation-only commits ahead of main: checkpoint report freshness, expert-role projection, context calibration model binding, and tool-value channel opportunity. No new implementation merge was found. [Main][main], [latest research comparison][research].

The five active non-research heads remained `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908` and `767a3291d6287b238417f2466592a9002810d05a`. Their inspected `autopoiesis_tools.go` files retain generation-to-`RecordExecution` publication and the reuse exclusion; four surrounding file bodies differ from main. This is not whole-branch equivalence.

This report is distinct from:

- **Generated learning aggregate supersession:** that audit establishes a correct durable running mean but stale earlier kernel snapshots. Here the *current durable mean itself* combines different kinds of evidence. Replacing all old facts with exactly one current row does not fix the witness. Preserve that audit's truth-maintenance repair. [1246 audit][aggregate-audit]
- **Generated tool refinement candidate continuity:** that audit starts with eligible feedback and follows missing source and discarded proposal disposition. This one concerns the evidence establishing eligibility. Neither repair supplies the other's missing contract. [1227 audit][proposal-audit]
- **Generated feedback Jev accounting idempotency:** that audit concerns repeated output judgments during repair of one execution's feedback. This witness uses distinct generation events and one durably recorded production invocation, with output Jev disabled; no retry is required. [1316 audit][jev-audit]
- **Tool-value channel opportunity:** that audit concerns offer evidence in `quality_profiles.json` and retirement proposals. This finding concerns `LearningStore`'s generation/runtime aggregate and refinement. It does not establish incorrect offer counts, Jev duplicate evidence, or a tool-value retirement proposal. [1548 audit][offer-audit]

Independent source challenge corroborated the population mixing and emphasized both the intentional generation-learning contract and the absence of automatic binary replacement. Main was revalidated unchanged at approximately 16:17 UTC. Static source and documentation inspection only: no application, tests, compilation, provider inference, benchmarks or failure injection were run. Only this research Markdown was written. The scenarios and evaluations below are unexecuted.

## Intentional learning that must be preserved

`recordGenerationLearning` explicitly says it captures generation outcomes, including safety and Thunderdome results, for future learning. `RecordExecutionDurable` refreshes generation learning context after recording. `AggregateLearningsForPrompt` deliberately extracts common issues and anti-patterns for subsequent tool generation, applies top-K selection, and caps its final text. Removing generation feedback wholesale would break an intended integration. [Generation producer][generation], [publication][publish], [generation context][context].

An existing test, `TestExecuteOuroborosLoop_RefreshesLearningsAfterGeneration`, supplies a rejected generation and requires both generator/Ouroboros learning-context updates and a `tool_learning` assertion. Credit this evidence of deliberate behavior; the proposal changes an overloaded evidence contract, not an accidentally uncalled function. The test does not distinguish generation evidence from the runtime-quality relation or check a still-installed incumbent after failed replacement attempts. It was read, not run. [Existing test][test].

Generation has real quality evidence: safety checks, adversarial testing, simulation and successful compilation matter. Some stages execute test code. These are not interchangeable with production tool invocations for user requests, nor is a generation-client failure proof that the incumbent binary produced bad output. A repair should retain stage-specific evidence and let policy decide what it justifies.

## Production path

### Normal entry reaches the generation recorder

The ordinary `nerd tool generate <description>` command boots a Cortex, obtains a tool need, and calls the orchestrator's `ExecuteOuroborosLoop`. A detected need supplies its name; the explicit-request fallback derives a stable name. The witness conditions on a canonical name G being selected consistently rather than predicting a model's naming choice. The CLI properly returns an error when the loop fails and verifies registry presence before reporting successful generation. [CLI entry][cli].

The orchestrator calls the loop, publishes registration facts on success, and calls `recordGenerationLearning` for its result. That recorder skips a reused tool, but otherwise constructs `ExecutionFeedback` keyed by `need.Name`. Its ID is `gen_<name>_<Unix seconds>`; its input is the requested purpose, duration is generation duration, and success is generation success. It supplies its own stage score before calling `RecordExecution`. [Wrapper and recorder][generation].

The supplied `QualityAssessment` means the shared recorder does not replace the stage score with ordinary output evaluation. The record's lack of a started production process is not a rejection condition in the generic store. The typed generated-runtime adapter has a stricter, appropriate started-process requirement; generation uses a different caller. [Generic recorder][publish], [feedback shape and admission][shape], [actual runtime adapter][runtime].

### Candidate failure does not require damage to the incumbent

The duplicate checker intentionally excludes the same name: same-name regeneration represents a deliberate new version, not reuse. If the new specification call fails, `ExecuteWithConfig` returns at `StageSpecification`, before the later write/compile/register commit stage. A successfully installed G can therefore remain unchanged while attempts to replace G contribute low generation scores. This witness needs no publication race, rollback failure or unsafe candidate adoption. [Same-name behavior][same-name], [early return][spec-failure], [commit boundary][commit].

Use fresh sequential command processes if necessary to keep each attempt's transient generation engine independent. Successful learning publication persists across those processes. There is no simultaneous writer in the witness.

### One accumulator erases the distinction

`ExecutionFeedback` has runtime receipt fields but no explicit generation-versus-runtime kind. `RecordLearningDurable` selects the aggregate solely by tool name, increments `TotalExecutions`, updates success rate, and incorporates any finite quality score into the same running mean. It does not filter by `ProcessStarted`, generation stage or producing subsystem. The durable receipt retains the original record; the aggregate and its kernel projection do not retain the domain distinction. [Feedback shape][shape], [aggregate][aggregate].

The shared path asserts that aggregate as `tool_learning` and publishes its known issues. The stock rules derive poor quality when executions are at least three and average quality is below 50 on the integer percentage scale. Poor quality derives `tool_needs_refinement` and `next_action(/refine_tool)`. No generation-kind or registered-version premise appears in that branch. [Projection][publish], [policy][policy].

The automatic requester is not launched merely because generation recorded a fact. In the factory's typed execution route, a real process must start, settle validation, and obtain a durable feedback acknowledgment. Only then does the adapter call `startRefinement`; that worker checks the kernel eligibility, obtains its lifetime-bounded context and invokes `RefineTool`. Preserve this distinction between a derived action and a demonstrated requester. [Factory wiring][factory], [runtime trigger][trigger], [requester][request].

## Conditional witness with current rather than stale evidence

Use a healthy workspace, a canonical generated-only tool name G, successful storage and kernel publication, and no independent pagination/incompleteness trigger.

1. Successfully generate and register G through the ordinary audited path. Its generation record receives score 1.0. There have been no production invocations of G.
2. Make four explicitly authorized generation attempts for the same canonical name. Each reaches specification and returns a generation error before commit, while its parent context remains live long enough to record feedback. The installed G stays unchanged. The existing duplicate checker must not select another name for reuse.
3. Space the generation records into distinct Unix seconds. This avoids the recorder's existing second-resolution ID colliding and being treated as replay; ID redesign is not the finding here.
4. Invoke the still-installed G once through a legitimately granted, authorized generated execution route. The real process starts, succeeds and passes actual named-result validation. Its feedback is durably acknowledged.
5. For a concrete score, use no tool quality profile, Jev output judgment off/unconfigured, a name such as `echo_rows` matching no completeness hint, clean output of at least 100 bytes matching no issue rule, and duration below one second. Ordinary evaluation then assigns completeness 0.8, efficiency 1.0 and score `0.8 × 0.4 + 1.0 × 0.2 + 0.4 = 0.92`. These are fixture conditions, not observed provider behavior. [Profile fallback][fallback], [quality formula][quality], [generic scores][scores], [output fallback][output].

The source-derived aggregate is:

| Included event | Count | Score contribution |
|---|---:|---:|
| Successful generation | 1 | 1.00 |
| Specification failures of uncommitted replacements | 4 | 0.80 |
| Successful production invocation of unchanged G | 1 | 0.92 |
| Current combined aggregate | 6 | 2.72 / 6 = 0.4533… |

The current projection is therefore approximately `tool_learning(G, 6, 33, 45)`. The policy derives poor quality and refinement eligibility. The production-only evidence consists of **one successful invocation**, not six executions with four runtime failures. It neither satisfies the three-invocation quality gate nor establishes that the incumbent is poor.

This witness does not depend on the obsolete-snapshot defect: even deleting every earlier aggregate leaves the current 45% row eligible. It does not assert that a single successful call proves G generally good. The correct runtime disposition may simply be insufficient samples, while a separate generation policy legitimately records repeated specification failures.

A smaller diagnostic control is three distinct specification failures for a never-created name. They alone produce count three and quality 20%, despite no production invocation or installed tool. That establishes the aggregate/policy mismatch, but is **not** evidence that the factory automatically buys a refinement call for the nonexistent tool.

## Controls and limitations

- **Generation feedback is not fabricated:** the attempts really failed, and the successful candidate really passed its admission path. The error is using the evidence for a different population, not denying those outcomes.
- **Reuse is excluded correctly.** `ReusedTool` causes no generation learning; retain that protection.
- **Canceled or failed publication can prevent the witness.** The generic store checks context and refuses corrupt loaded state; it stages and atomically publishes before memory acknowledgment. The scenario assumes these operations succeed. It does not rely on the separate multiwriter durability issue. [Store admission and persistence][durable]
- **Real runtime gates remain intact.** Capability, permission, binary/protocol identity, process-start, validation and durable acknowledgment are not bypassed by a low quality score. A quality assessment is not task acceptance.
- **No automatic adoption or retirement follows.** Current refinement is proposal-only; the existing proposal-owner audit governs its empty source and dropped disposition. This report must not be used to silently apply candidate code.
- **Other issue paths remain legitimate.** Correcting this denominator must not clear independently supported runtime pagination, incomplete-output or failure evidence.
- **Versioning is related but not required for this witness.** All production behavior here belongs to the unchanged incumbent. Failed replacements never publish a new binary. More general cross-version aggregation needs its own explicit policy.
- **No universal cost claim follows.** A generation failure can be useful evidence for improving the generator; an extra review could occasionally help G. The report establishes attribution, not that every eligible proposal wastes money or lowers quality.

## Proposed evidence contract

### Preserve two populations and permit explicit learning across them

Record an explicit event kind and producing operation: generation attempt, candidate test/admission result, or production invocation. Include generation attempt identity and stage, proposed name/candidate identity when available, and the incumbent/version relationship where applicable. Runtime events retain the exact admitted binary/protocol/request identity and process/validation outcome.

Keep separate sufficient statistics for generation and runtime quality. This need not require separate files or duplicate output bodies; typed records and separate projections in the existing durable store can suffice. Runtime quality and its minimum-sample gate should count admitted runtime observations according to a documented contract, not generation-client outcomes.

Retain generation feedback in prompt learning and candidate-repair policy. Mangle can derive a distinct obligation such as investigate generation failures or revise a rejected candidate. If product policy intentionally wants a combined lifecycle score, give it an explicit name, weights, denominator and consumer; do not present it as production execution quality. Keep failure reason ownership clear so provider/JIT failures are not silently equated with defects in the incumbent.

### Roll out without inventing historical provenance

Version the persisted schema and kernel projection together. Historical full feedback records may permit some classification using trustworthy producer provenance and receipt fields. Do not classify every `ProcessStarted=false` legacy row as generation: older runtime wrappers also omitted that field. A string prefix alone is not a universal proof of event kind.

Where evidence is incomplete, preserve the historical mixed aggregate as unqualified rather than relabeling it as runtime-only, guessing a split, or deleting history. Define a conservative runtime eligibility policy until enough qualified observations exist. A backfill must be replay-safe, survive restart and not count already acknowledged events again.

Coordinate current-fact supersession with the separate 1246 repair. Rebuilding the right aggregate while leaving old mixed `tool_learning` facts active would not close the live path. Preserve independent known-issue provenance rather than clearing all issues to make a test pass.

### Maintain existing durability and lifecycle ownership

Keep acknowledged execution identities and feedback-repair semantics; an accounting migration must never rerun a generated effect. Use a coherent atomic migration/publication boundary and an explicit schema version. Cross-process ownership and receipt-retention work remain separate obligations; do not replace synchronous acknowledgment with unproved asynchronous durability.

Bound retained migration metadata and reuse existing bodies. Do not retain every body forever merely to support a new counter; coordinate any later compaction with established replay and unacknowledged-receipt protections.

## Costs and fair evaluation

The narrow separation requires schema and constructor plumbing, projections, policy declarations, generation-context rendering, diagnostics and migration tests. Typed event metadata adds small per-record fields, but full-history reaggregation can require reading and classifying retained records. For N retained records, a straightforward migration is an O(N) pass plus decoding/storage costs; no performance was measured. Incremental counters avoid repeating that pass on each call.

No extra model or embedding call is inherently needed. More careful policy may suppress unjustified incumbent refinements but also delay valid refinement until enough runtime evidence exists. Separate generation recovery may add useful work. Report total host/model cost and verified outcomes, not just a lower count of refinement requests.

Recommended evaluations, **not run**:

1. Exercise normal CLI generation, actual registration, four pre-commit failures, reopen, and an actual authorized generated invocation through factory/VirtualStore. Inspect unchanged binary identity, durable typed feedback, current aggregates, kernel facts and automatic requester. Helper-only fabricated aggregates are supplemental.
2. Isolate from stale-fact behavior by evaluating a fresh kernel containing only the newest projected aggregate. The mixed-domain classification must remain demonstrable before the repair and disappear from the runtime-only relation afterward.
3. Keep generation learning active: a rejected safety candidate must still inform subsequent generation context and any explicit generation-repair rule. Update the existing refresh test to assert the correct domain rather than remove its coverage.
4. Cover pure runtime failure/recovery, pure generation success/failure, reuse, pre-start refusal, genuine validation failure, parent cancellation, storage failure, duplicate identity and distinct identities. Real runtime faults must retain their intended pressure.
5. Cover the ordinary successful-generation baseline and repeated same-name replacement. Do not mistake adversarial fixture execution during generation for a production invocation; preserve its own evidence.
6. Exercise legacy mixed records, absent bodies, ambiguous old wrappers, rollback/restart during migration and feedback retries. Neither migration nor replay may execute a tool or manufacture a successful receipt.
7. Preserve JIT failure, shutdown cancellation/join, per-call refinement timeout, proposal-only behavior and explicit governed adoption. Count attempted eligibility checks separately from actual completions and adopted changes.
8. Pair representative tasks at fixed model, source, authority, thresholds and initial histories. Measure evidence-kind accuracy, runtime quality classification, appropriate generation-repair/refinement decisions, useful candidate retention, final task quality, all model calls/tokens, latency, migration I/O and retained metadata. Include no-effect cases and cases where separate evidence causes more justified work.

**Acceptance:** generation remains learnable, but an unsuccessful uncommitted candidate cannot satisfy the incumbent tool's runtime sample count or lower its runtime-quality mean. Every policy consequence states which kind of evidence supports it, while existing execution, durability and adoption controls remain intact.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[research]: https://github.com/theRebelliousNerd/codenerd/compare/388205138ba6d47c1cff0c5e97c896dfa642a391...e69f9f921f1b4a7318fafbf5c82834e75ecebc7f
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_tools.go#L126-L218
[aggregate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L394-L422
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/autopoiesis.mg#L101-L137
[trigger]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L148-L160
[request]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_tool_executor.go#L227-L265
[proposal-audit]: https://github.com/theRebelliousNerd/codenerd/blob/4585e7fa4732597bd37e0bfe36357f0a3ba71031/Docs/audits/generated-tool-refinement-candidate-continuity.md
[aggregate-audit]: https://github.com/theRebelliousNerd/codenerd/blob/58c84997ba39254e22c590f6cb3bab43a58509c1/Docs/audits/generated-learning-aggregate-supersession.md
[jev-audit]: https://github.com/theRebelliousNerd/codenerd/blob/30684dc18e934f12e50db05035febe7692174692/Docs/audits/generated-feedback-jev-accounting-idempotency.md
[offer-audit]: https://github.com/theRebelliousNerd/codenerd/blob/e69f9f921f1b4a7318fafbf5c82834e75ecebc7f/Docs/audits/tool-value-channel-opportunity.md
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_feedback.go#L19-L95
[context]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_feedback.go#L211-L327
[test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/orchestrator_test.go#L157-L201
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_advanced.go#L1025-L1120
[shape]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L30-L75
[runtime]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L133-L163
[same-name]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/tool_jev.go#L101-L120
[spec-failure]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/ouroboros.go#L458-L479
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/ouroboros.go#L692-L704
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1990-L2018
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/autopoiesis_profiles.go#L18-L40
[quality]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L193-L260
[scores]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality.go#L563-L615
[output]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/quality_jev.go#L30-L75
[durable]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/feedback.go#L353-L497


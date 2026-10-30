# Campaign analysis artifact role continuity

**Status:** source-derived integration finding and proposed contract, not an observed incident or an executed regression. Inspected main `388205138ba6d47c1cff0c5e97c896dfa642a391`, reconfirmed on 2026-10-30. No application execution, tests, model inference, or implementation were performed.

## Conclusion

A campaign research task can treat the test source it inspected as its durable analysis output. The planner classifies paths containing `_test` or `test_` as `/test_file`; the output-persistence helper treats every `/test_file` as an output. When that input already exists, the helper does not persist the returned findings, and the subsequent deliverable check accepts the input's nonzero size.

The bounded defect is **artifact role confusion at the analysis persistence and local deliverable boundary**. It does not prove a false session verdict, a completed campaign, or the disappearance of every copy of the findings. In-memory return handoff, audit records, phase review, and other verification remain separate channels and controls.

The recommended repair is to distinguish an artifact's kind from its role and production provenance. A file containing tests can be either an input or an output. Its filename and existence alone cannot establish which.

## Existing repair and what remains

The current research handler deliberately persists returned findings so an audit produces a campaign-owned document rather than only an in-memory response. Its comments identify the earlier F-DURABLE-1 failure. The explicit-shard handler has the same persistence call. The current F-HOLLOW/F12 repair also avoids judging answers by length or opening phrases: a terse real finding is eligible, and a turn with nothing to persist fails the local analysis gate. These are useful repairs to preserve. [Research handling][research] [Explicit-shard handling][explicit] [Existing tests][artifact-tests]

The helper correctly excludes `/source_file` and `/knowledge_base` as inputs, but includes `/test_file` among output kinds. That distinction is incompatible with the planner's filename-based type assignment. The result is not that all analysis tasks fail, nor that every existing output is invalid. It is that an ordinary test input falls on the wrong side of a boundary the implementation already intends to enforce. [Artifact classifiers and gate][artifact-gate] [Planner classification][planner]

The prior [campaign resume return evidence audit][resume-audit] owns completed tasks losing requested return evidence after cold resume. It expressly credits analysis artifact persistence as a competing protection. This document concerns a preceding classification seam that prevents that protection from producing a report in the first place. It does not reopen the full resume design or assert that persisting a visible projection alone would solve long-return handle recovery.

## Source derived witness

Use an otherwise admitted campaign with a research task whose purpose is to inspect an existing regular, nonempty test file, for example:

- task type `/research`;
- description asking for findings about `pkg/widget_test.go`;
- artifact list containing that path as the material to inspect;
- no genuine report artifact already associated with the task;
- no intervening deletion or modification of the input.

The planner's JSON schema accepts string artifact paths and does not carry an input/output role. The description and task type remain research; this witness does not downgrade a planned mutation or test execution obligation. [Raw task and schema][schema] [Effect validation][effect-validation]

The remainder is conditional on the real worker successfully returning nonempty findings and an admitted `/done` outcome without writing a report. It does not predict what a particular model will emit or claim that a preflight, permission check, cancellation, or worker error must permit progress.

| Stage | Current source behavior | Consequence for the witness |
|---|---|---|
| Plan conversion | Any artifact path containing `_test` or `test_` becomes `/test_file`; safe paths are normalized and retained. | The inspected test source is typed as a kind the consumer treats as output. |
| Worker dispatch | The campaign requires an observed executor and uses `RunUntilVerified`; only successful return reaches the persistence step. | The witness retains the existing verdict and obstacle safeguards. |
| Persistence | A successful `os.Stat` for any recognized output artifact immediately suppresses fallback persistence. | The existing test input prevents creation of the findings document. |
| Local deliverable check | A recognized output artifact with `Size() > 0` suffices. | The unchanged test source satisfies this local gate. |
| Task completion | Nonmutating research skips the microcheckpoint; a successful wrapped task reaches `completeTask`. | The task can be recorded complete without a new analysis report artifact. |

Sources: [plan conversion][planner], [path normalization][normalization], [observed dispatch][dispatch], [persistence][persistence], [local gate and research handler][research], [microcheckpoint][microcheckpoint], [completion caller][completion-caller].

The same persistence and gate combination is reachable through an explicitly routed research task. No special shard implementation is needed, but the explicit route still has its own input assembly and execution requirements. The ordinary research route is sufficient for the finding. [Explicit route][explicit]

## Returned findings are not the same as a report artifact

The local omission should not be exaggerated into end-to-end starvation.

On successful task completion, the orchestrator creates a bounded kernel audit summary and stores a return projection for dependents. The result cache protects entries still named by pending, active, or blocked tasks' `ContextFrom` edges. The planner also inherits a context edge from an earlier direct research dependency when appropriate. These are real mitigations. [Completion records][completion-records] [Return projection and cache][return-cache] [Research dependency inheritance][inheritance]

The evidence policy keeps a requested return projection unless the producer's durable `/doc` is already inlined. In this witness there is no such report, so a retained context edge can still deliver the findings from the live cache. The task context builder combines that return channel with selected artifacts and code preload. [Projection policy][projection-policy] [Actual context assembly][context-assembly]

The artifact channel is different. Its host measurement requires regular files, and its prose-evidence rules select `/doc` outputs or document-class extensions. The existing `.go` test input is not transformed into an analysis report by those rules. Thus the verified consequence is the absent campaign-owned findings document and a misleading basis for the local deliverable admission, not a guarantee that the next task lacks the findings. Other logs or retained responses may also contain them; this review does not establish their universal availability or loss. [Artifact measurement][measurement] [Prose evidence policy][prose-policy]

A later phase reviewer can reject missing or inadequate reports. Campaign acceptance can independently fail. The source does not establish how those model-mediated reviews would judge this particular case, and the proposal must not weaken them.

## Related eligibility controls

The two helpers also disagree about what counts as a usable existing output:

- **Existing empty file:** persistence returns on successful stat, but the deliverable check rejects zero size. A nonempty returned finding can therefore remain unpersisted while the handler reports that there were no findings to persist.
- **Directory:** persistence also returns on successful stat. The size-only gate can accept it on a filesystem where its reported size is positive; downstream artifact measurement rejects it because it is not a regular file. Directory size is platform-dependent, so this is a conditional control, not the primary witness.
- **Genuine existing output:** the implementation intentionally avoids duplicating a report the task already produced. The existing test explicitly checks that behavior. A repair that always overwrites or duplicates every existing output would discard this useful contract.
- **File-producing tasks:** they are deliberately exempt from analysis fallback and have their own write and verification paths. This proposal does not require them to manufacture prose reports.

These cases are visible in the [persistence helper][persistence], [local gate][artifact-gate], [regular-file measurement][measurement], and [existing dedup and exemption tests][artifact-tests].

## Proposed contract

The minimal invariant is:

> An input reference cannot suppress persistence of an analysis return or satisfy the analysis-output gate merely because its kind is a test file and it exists.

Separate three dimensions:

1. **Kind:** source, test, document, configuration, or another artifact representation.
2. **Role:** input, intended output, or explicit reusable output.
3. **Production evidence:** producer task, attempt or admitted prior attempt, path, and the version actually observed.

Go should measure existence, regular-file status, readable content, size, and any required version identity. It should report absence and read errors distinctly. A shared eligibility decision should govern both persistence suppression and local deliverable admission; otherwise fixing only one helper leaves contradictory states.

The policy should permit reuse of a genuine task output without demanding a new write on every retry. Conversely, an old file with no output relationship should not acquire that relationship from its filename. Where the current analysis return has no eligible durable representation, persist it under the existing campaign artifact directory and register the resulting report. Preserve the current rule that empty returns do not become invented findings and that short findings are not rejected for being short. Preserve existing workspace bounds and mediated repository writes. [Current bounded persistence][persistence]

Do not overwrite the inspected test file to satisfy this contract. Do not reinterpret a research task as `/test_write`, `/file_modify`, or `/document` merely to force a write. The campaign can materialize the returned report within its own artifact area without asking the worker to change the evidence it inspected.

### Implementation choices

A focused first step can make legacy research references contextual and persist their returned findings unless an explicitly eligible report is already present. This has a small implementation surface but needs an explicit compatibility rule for research tasks that intentionally produce non-document outputs.

A fuller design adds role and producer/version fields to the plan and journal representation, with observations binding actual outputs to attempts. It supports legitimate reuse more precisely but requires migration, validation, and consistent handling by decomposition, refinement, resume, and downstream evidence selection. Model-authored role declarations express intent; they do not themselves prove that a file was produced.

Another option is a campaign-owned return record with a stable document projection. This can share infrastructure with the prior resume proposal, but should not silently turn a bounded role fix into a new replay subsystem. Compare these choices before committing to the larger migration.

Legacy snapshots must be handled conservatively: preserve their bytes and task history, mark uncertain output provenance, and avoid rerunning completed side effects simply to reconstruct a report. A version mismatch or missing artifact should lead to an explicit policy disposition, not an empty successful replacement.

## Costs and failure modes

No additional LLM call is inherently necessary for the focused repair. Incremental costs include writing and retaining reports currently skipped, extra artifact metadata, a shared eligibility check, and potentially content hashes. Hashing is proportional to bytes read; task/attempt records add journal size and migration complexity.

Newly materialized reports can affect upstream prompt selection. Existing `/doc` deduplication can avoid repeating the live return when the full report is inlined, but digest and handle paths still need evaluation. Track prompt tokens rather than assuming a saving.

Storage exhaustion, permission errors, cancellation, partial publication, repeated attempts, and concurrent replacement can still prevent a valid report. Define whether task completion waits for durable publication and how a failed write is surfaced. If publication must survive a crash, use a coherent file-and-metadata receipt rather than relying on a successful stat observed earlier. Keep retention and access controls appropriate for potentially sensitive findings.

Overstrict freshness checks can reject legitimate reuse; permissive legacy inference can retain the original confusion. Always-persist behavior can create duplicates. Overwriting an existing file can destroy user content. These tradeoffs are reasons to preserve input bytes and make ownership explicit, not to relax verification.

## Fair evaluation plan

These are proposed evaluations; none were run.

Use deterministic observed executors returning the same findings, same admitted outcome, and no writes. Exercise actual plan conversion and the research handler rather than only constructing a convenient `TaskArtifact` in a unit test.

| Control | Required observation |
|---|---|
| Equivalent existing `widget.go` and `widget_test.go` inputs | Both retain input bytes and obtain a usable findings report; spelling alone does not change output eligibility. |
| `test_` in a directory or unrelated basename | Kind inference does not create output provenance. |
| Genuine already-written report | Reuse remains possible without unnecessary duplication or overwriting. |
| Empty return and no eligible output | Existing honest no-deliverable behavior remains. |
| Short nonempty finding | It is preserved without a prose-length threshold. |
| Empty file, directory, unreadable path, missing path | Persistence and admission agree on typed eligibility; no directory-as-report success. |
| Mixed inputs and genuine outputs | Input presence neither grants completion nor blocks legitimate reuse. |
| Worker error, unverified outcome, obstacle, cancellation | Earlier execution safeguards remain authoritative. |
| Same-process dependent with `ContextFrom` | Returned findings remain available; report/return dedup does not erase distinct evidence. |
| Retry, legacy snapshot, missing or changed prior output | Preserve bytes and history; require explicit reuse or recovery disposition without repeating side effects. |

Compare current behavior, the focused role correction, and a typed producer/version design under identical fixtures and budgets. Record report creation rate, false no-deliverable errors, duplicate reports, input-byte integrity, completed-task records, downstream evidence availability, artifact bytes, stat/hash work, journal writes, prompt tokens, and wall time. Verify phase review and final acceptance independently rather than counting local gate admission as campaign success.

## Review scope

Root and campaign guidance, current campaign wiring documentation, the main audit inventory, and research additions through branches 0715, 0747, 0817, 0850, and 0919 were screened. The closest predecessor is the resume-return audit discussed above. Searches for the helper names and durable/test-artifact terms found no exact published treatment of this producer/consumer role mismatch; that is a bounded screen, not proof that no differently worded discussion exists.

The inspected session lane `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, store lane `a53cfc308e8836c72370adaa46deba3f7b00180d`, and OAuth lane `aa7988dec4c62116656ea722ff73b687a8509908` retain the same planner and artifact-measurement files and the same relevant classifier, persistence, local-gate, and research-handler bodies. Their larger handler files are not wholly identical to main. No runtime result is inferred from those source comparisons.

[planner]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L539-L553
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_requirements.go#L484-L565
[effect-validation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_effect_contract.go#L16-L38
[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/normalization.go#L123-L162
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L45-L112
[research]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L374-L422
[explicit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L282-L342
[artifact-gate]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L350-L385
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L425-L475
[microcheckpoint]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/micro_checkpoint.go#L11-L26
[completion-caller]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L450-L475
[completion-records]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L653-L691
[return-cache]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_results.go#L18-L153
[inheritance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L505-L533
[projection-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_evidence.mg#L214-L235
[context-assembly]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_evidence.go#L426-L467
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_evidence.go#L94-L121
[prose-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_evidence.mg#L50-L65
[artifact-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_durable_artifact_test.go#L39-L166
[resume-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-resume-return-evidence.md


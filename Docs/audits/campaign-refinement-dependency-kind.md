# Campaign refinement must preserve dependency kind

## Decision and evidence boundary

**The rolling-wave refiner accepts a task's `depends_on` field but projects it only as a context edge, not a scheduling dependency.** A newly added higher-priority consumer can consequently become eligible before its pending producer, even with a single worker. Its context renderer then has no producer return to inject.

The issue is not that the scheduler ignores a stored hard dependency. The scheduler correctly follows the facts it receives; the hard edge was lost before those facts were built. Nor does every context edge need to become a hard dependency. Ordering and optional return reuse are different contracts and should remain distinguishable.

**Recommendation:** admit typed, resolved dependency edits before publishing a refined plan. Preserve `depends_on` as a hard edge; keep `context_from` as an evidence edge unless an explicit contract also requires ordering. Define the legacy `dependencies` alias rather than silently merging all three fields.

Inspected main: [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), rechecked on 2026-10-30. This is static source analysis and a conditional accepted-output witness, not an observed production incident. No application execution, tests, Git commands, model inference, training or implementation accompanied this review.

**Important producer qualification:** the current refinement prompt's JSON example does not advertise `depends_on`. The decoder and a repository test nevertheless deliberately accept it. The witness conditions on such an accepted response; it does not predict that a model will emit the field, or describe it as a field the current sample reliably teaches. [Prompt example][prompt], [accepted fields][parse], [existing mapping test][mapping-test].

## Existing work and what this adds

The campaign return-evidence audit already distinguishes completion from evidence availability: a cold-resumed dependent may lose a completed producer's return. Its witness has both a real hard dependency and a context edge. This report addresses an earlier, different boundary: a live refined plan can lose the hard edge while the producer is still pending. Persisting all completed returns would not produce a return from work that has not run. [Prior return-evidence audit][prior-return].

Current code has important protections that must receive credit:

- Initial decomposition separately populates `Task.DependsOn`, and can additionally inherit research context from that dependency. It does not make the same projection mistake. [Initial dependency wiring][decomposition], [research-context inheritance][inheritance].
- Campaign snapshot inclusion validation rejects dangling task/phase/context references before publication. Both nodes exist in this witness, so the surviving context edge is valid; inclusion cannot reconstruct an omitted edge kind. [Snapshot admission][admission].
- The scheduler no longer falls back to a permissive in-memory scan when Mangle returns no eligible work. It honors the kernel, including dependencies, priorities, backoff and conflicts. This report is not a recurrence of that old bypass. [Kernel-authoritative selection][selection].
- Effect validation and the edit-before-verification helper preserve important execution obligations. Their coverage is narrower than arbitrary producer/consumer ordering. [Effect and verification-order contract][effect-contract].

The root/campaign split still applies: host code must accurately adapt the plan's declared structure; Mangle decides execution. No additional agent authority is needed to retain an already declared hard edge. [Campaign guidance][guidance].

## Live producer-to-consumer trace

### 1. Ordinary campaign execution reaches rolling-wave refinement

The TUI creates an orchestrator, decomposes and admits a campaign, and later invokes its `Run` method. The normal constructor installs a replanner. Once a phase completes its completion path, the task loop calls `triggerRollingWave`, which invokes `RefineNextPhase` with the live campaign and stored completed-task results. A successful refinement republishes campaign facts and produces a revision event. [TUI admission][tui-admission], [TUI execution][tui-run], [constructor][constructor], [phase transition][transition], [rolling-wave call][rolling-wave].

The refiner selects the next phase by adjacent order and clones the campaign before editing it. It shows the planner completed-task returns and upcoming task IDs/write targets. This is a real improvement over the earlier target-retargeting gap; the witness uses an exact existing producer ID, not an unknown name or a missing-ID fallback. [Refinement setup][setup].

### 2. Three accepted fields become one context list

The response decoder accepts separate `depends_on`, `context_from` and `dependencies` fields. In the `action: add` path, the refiner concatenates all three, resolves the names, and initializes only `ContextFrom: ctxFrom`. There is no `DependsOn` assignment. The unmatched-update-as-add path performs the same conversion. [Decoder][parse], [add projection][add], [update fallback][fallback].

The resolver can resolve exact IDs already present in the next phase, numeric indices or description matches. The primary witness uses an exact ID, avoiding ambiguity, forward-reference resolution, numeric coercion and description normalization. [Resolver][resolver].

Afterward, validation checks retained task effects and applies `orderVerificationAfterEdits`. Totals and revision metadata are updated; `syncCampaignFacts` validates and publishes the resulting plan; the live campaign adopts that clone. Nothing in this sequence converts a context edge back into the lost hard edge. [Publication][publication], [effect contract][effect-contract].

The separate new-requirement adapter also merges the three fields into `ContextFrom` alone. It is a related consumer to align in a repair, not needed to establish the rolling-wave witness. A matched update of an existing task, meanwhile, processes description/targets/type/priority but does not apply these parsed edge fields. **It does not strip the task's previously stored hard or context edges: those survive the clone unless another operation removes the task.** The omission here concerns newly requested edge changes. Keep those branches distinct when specifying replacement versus preservation semantics. [Requirement adapter][requirement], [matched update][update], [cloned working plan][setup].

### 3. Facts and policy preserve the distinction the refiner erased

`Task.ToFacts` emits `task_dependency` from `DependsOn` and `task_context_from` from `ContextFrom`. The stock `has_blocking_task_dep` rule examines only hard dependencies whose producer is neither completed nor skipped. The context relation is used by evidence projection, not that readiness rule. [Fact projection][facts], [hard-dependency rule][readiness], [context projection][context-policy].

The priority rule is significant: a pending higher-priority task with no blocking hard dependency holds a lower-priority task back. `eligible_task` also checks current phase, storage uncertainty, retry backoff and active write conflicts. Therefore a higher-priority consumer whose hard edge disappeared can outrank its still-pending producer; a single-worker configuration is not a general rescue. [Priority and eligibility][eligibility].

The host queries `eligible_task`, intersects it with the current phase's actual tasks, acquires a write-set lease and starts admitted workers up to the concurrency limit. There is no later generic `ContextFrom` readiness check in that route. [Selection][selection], [dispatch][dispatch].

### 4. Missing producer output is not an implicit ordering barrier

The document handler delegates to the file-task path, which builds its input through `buildTaskInput`. That includes policy-selected context edges, selected durable evidence and source preloads. `renderContextFrom` explicitly skips a source task that has no stored return. An edge can therefore remain visible in the plan while contributing no return to the worker's input. [Document handler][document], [file-task input and spawn][file-input], [input assembly and absent-result handling][render], [context selection][context-selection].

This does not prove the worker produces a bad document. It might discover the necessary information independently, ask for clarification, refuse, fail verification, or later be corrected. The supported discrepancy is that the accepted hard dependency does not control dispatch and its pending producer's return is unavailable at that moment.

## Concrete conditional witness

Begin with a valid, admitted campaign whose current phase finishes normally and whose immediately following phase contains:

- **P:** ID `/producer`, pending `/research`, normal priority, no blocking dependencies or backoff, and no completed return or durable findings yet.
- No other task that would block the example through higher priority, shared write targets, active locks, storage hold or custom policy. Keep the context alive and ordinary permissions/admission satisfied.

Condition on the planning client returning this accepted refinement object:

```json
{
  "tasks": [
    {
      "task_id": "/consumer",
      "description": "Write a synthesis of the producer's findings",
      "type": "/document",
      "priority": "/high",
      "action": "add",
      "artifacts": ["docs/summary.md"],
      "write_set": ["docs/summary.md"],
      "depends_on": ["/producer"]
    }
  ],
  "summary": "Add the report that consumes the pending investigation"
}
```

Use a valid writable workspace target distinct from P's outputs, and a description distinct from existing tasks so deduplication is not involved. The explicit target satisfies the document task's plan-time file-target requirement. This is an accepted decoder fixture, not a claim that the current prompt sample advertises every field in it.

1. Resolution finds `/producer`. The new C retains `ContextFrom: ["/producer"]` but no `DependsOn`.
2. Snapshot inclusion succeeds because both task identities exist. Effect validation succeeds, assuming the stated valid target; the verification-order helper does not add an edge for document-after-research.
3. When this next phase is current, C has no hard blocker. Under the stock priority rules C is eligible; P is held by the higher-priority pending C. This is independent of concurrency greater than one or result ordering.
4. With a positive concurrency admission and a successful write-set lease/ordinary execution gates, C is dispatched before P.
5. With no earlier cached return or independently available equivalent evidence, the explicit context edge supplies no P result to C's input. C's actual work still passes through normal worker, permission, transaction and verification controls.

An otherwise equivalent plan carrying `DependsOn: ["/producer"]` behaves differently: C is blocked; the priority helper excludes that blocked C as a reason to hold P; P can run first. This is an especially useful paired control because it tests dependency preservation without changing the model, task text, priority or concurrency policy. [Hard blockers and priority interaction][readiness], [eligibility][eligibility].

## Safeguards, counterexamples and nonclaims

**Already completed producer:** the existing mapping test uses a completed research producer and checks only the context list. In that state the missing hard edge does not produce the demonstrated premature scheduling. Preserve its successful context-reuse behavior while adding a pending-producer control. The test was read, not run. [Existing test][mapping-test].

**Initial decomposition:** it explicitly populates hard edges and inherits research context separately. This is not a claim that every campaign dependency is broken. [Decomposition][decomposition], [inheritance][inheritance].

**Verifier ordering:** a later `/test_run` or `/verify` task receives hard dependencies on preceding mutating tasks through `orderVerificationAfterEdits`. That can rescue those particular pairs. It does not cover a document consuming pending research, and the proposal must preserve intentional reproducer-before-fix ordering. [Effect contract][effect-contract].

**Priority/concurrency controls:** equal priorities plus a single worker may happen to execute an earlier producer first because host selection preserves phase order; that is a schedule-dependent mitigation, not preservation of the declared edge. The primary higher-priority-consumer witness avoids it. Multiworker overlap is an additional evaluation case, not required here. [Phase-order intersection][selection], [bounded dispatch][dispatch].

**Admission failures:** malformed JSON, unavailable planning, failed effect validation, inclusion failure, fact publication errors, cancellation, risk refusal, missing tools or write-lock failure can prevent the described dispatch. The witness requires successful relevant admissions. A valid context reference is not a dangling-reference example.

**Other evidence:** completed earlier research, reusable source artifacts, source preloading or the worker's own investigation may supply enough information. Require the desired P result to be genuinely not yet produced and not duplicated elsewhere when assessing the input consequence. Do not claim total context starvation.

**Later verification:** task micro-checkpoints, file checks, phase checkpoints and final acceptance can reject or repair incomplete work. No overall false-success verdict, permission bypass, lost file or fabricated finding is established. The concern is incorrect work ordering and potentially avoidable execution/retry caused by a missing prerequisite.

## Proposed typed refinement contract

### Plan admission

1. Specify dependency kinds in the refinement schema and prompt: hard execution prerequisite, optional/required evidence source, and any supported soft-order preference. Give `depends_on` and `context_from` the same meanings as the campaign model and initial decomposer.
2. Allocate/resolve task identities against the complete proposed revision before resolving edges. Preserve stable IDs for updates; distinguish omitted fields from explicit clearing. A missing producer, ambiguous description or unsupported forward reference should produce a bounded plan error, not silently become an empty edge list.
3. Populate `DependsOn` from admitted hard references. Separately populate `ContextFrom`; it is reasonable to retain the current useful research-context inheritance, but context must supplement rather than replace a hard dependency.
4. Define the `dependencies` compatibility alias explicitly. If its historical meaning is context-only, migrate/deprecate it visibly rather than silently upgrading all old hints into hard blockers. Never treat every context edge as a hard prerequisite: optional historical reuse is legitimate.
5. Validate reference inclusion, self/cyclic hard dependencies, phase compatibility, execution effects and declared output targets on the resulting clone. Keep the previous live plan unchanged if admission fails. Reuse existing snapshot publication rather than invent a second scheduler in Go.

### Runtime and evidence

Have the host project admitted edge kinds faithfully; let the existing Mangle readiness rules own dispatch. Where evidence is required in addition to producer completion, represent availability/freshness separately rather than inferring it from `completed` or turning a missing string into successful empty context. That latter requirement continues the prior return-evidence contract; fixing scheduling alone does not resolve cold-resume evidence loss.

Retain the raw proposed fields, their resolved IDs, typed edges and any rejected or downgraded disposition in a compact revision receipt. This distinguishes a planner omission from an adapter omission and allows the UI to say what changed. Do not permit a free-text explanation or model assertion to waive a required dependency.

### Implementation choices and limits

- **Minimal mapping repair:** preserve `depends_on` in `DependsOn` while retaining useful context inheritance. This directly addresses the primary witness but still needs clear update/clear semantics and validation before newly materialized cycles can reach scheduling.
- **Shared typed plan adapter:** use one admission representation across initial decomposition, failure replanning, requirement intake and rolling-wave refinement. More migration work, but fewer divergent field contracts. Avoid forcing all producers to use the same numeric reference space without an explicit version change.
- **Make all context edges blocking:** not recommended. It conflates optional evidence reuse with hard order and can deadlock or serialize previously valid work.
- **Rely on one worker, list order or prompt wording:** insufficient. The priority witness remains with one worker; prompts cannot repair a correct accepted hard field after the adapter drops its kind.
- **Infer dependencies from prose:** not necessary for this correction. Preserve explicit typed input first; do not add expensive semantic inference to solve deterministic transduction.

## Costs, risks and operational burden

For N tasks and E references, an ID-indexed resolution and graph validation pass can be O(N + E); description aliases and canonicalization add work and require ambiguity rules. Current per-task resolver rebuilding/scanning should be measured rather than assumed cheaper. Typed edge storage is O(E), with added provenance bytes if raw/resolved forms are retained.

Costs include schema/prompt alignment, parser and migration maintenance, cycle diagnostics, campaign fact replacement, Mangle reevaluation, persistence and revision rendering. Restoring real prerequisites can reduce unsafe parallelism and increase elapsed time. It can also avoid premature worker calls and retries, but no saving is measured. Count both changes, including planning/worker tokens, tool/check CPU and I/O, failed attempts, lock waits, checkpoint reruns and owner interventions.

An overstrict resolver can reject legitimate legacy aliases; a permissive fallback can erase an obligation again. Turning ambiguous aliases into hard edges can create new cycles. Persisted plans without original refinement fields may not support retrospective reconstruction: preserve their historical state and qualify unknown provenance rather than inventing edges. A corrected edge graph still needs the existing concurrency, pause, ownership, permissions and verification mechanisms.

## Fair evaluation — proposed, not run

Use deterministic planner responses through the real refinement decoder, clone/admission/publication path, production policy and scheduler. Capture the published plan/facts, actual dispatch order and built consumer input. A unit assertion on `ContextFrom` alone is insufficient. Do not require real model calls to establish this mapping defect; any later model-based evaluation requires separate authorization.

| Fixture | Required distinction |
| --- | --- |
| Pending normal-priority P; higher-priority C with exact `depends_on:[P]`; one worker | C remains blocked; P is eligible first; original hard-edge type survives publication. |
| Same pair and priorities, directly decomposed rather than refined | Equivalent admitted dependency semantics across both producers. |
| P already completed with a retained return | Preserve existing context mapping and avoid unnecessary redispatch. |
| Context-only optional source, no hard dependency | Do not introduce an unintended readiness block. |
| Both hard and context edges | Preserve ordering and selected evidence without duplicate context injection. |
| Later test/verify after edits, and an intentional reproducer before a fix | Preserve current automatic ordering and the intentional reverse order. |
| Equal priorities with one worker, then multiple workers | Distinguish accidental favorable order from enforced dependency. |
| Add, matched update, unmatched-update fallback and requirement intake | Apply documented edge-kind and omitted-versus-empty semantics consistently. |
| Unknown/ambiguous producer, forward reference, self-edge and hard cycle | Reject or explicitly resolve; no silent edge disappearance or partial publication. |
| Fact-sync failure, storage hold, risk refusal, pause/cancel, lock failure | Preserve existing no-dispatch and lifecycle guarantees. |
| Producer completes but its return is unavailable after restart | Ordering repair must not misreport evidence availability; retain the earlier audit's explicit unresolved state. |
| Consumer fails verification or discovers the missing prerequisite itself | Report that mitigation honestly; do not count a blocked/failed task as accepted incomplete work. |

Compare current behavior, minimal typed mapping, and a shared adapter under identical source/configuration, task graph, priorities, worker limits and verification obligations. Report premature dispatch, dropped/misclassified edges, unnecessary blocking, dependency cycles, producer-return availability at dispatch, final independently verified task quality and full cost. Include failures and unresolved plans; a lower task/token count caused by dropping work is not an efficiency improvement.

## Inventory and active-branch scope

Current guidance and audit inventory were screened through `research-main-20261030-1316` (`30684dc18e934f12e50db05035febe7692174692`). That branch's additions cover generated-feedback accounting, recurse commit recovery identity, relevance-head acquisition and review-scope remainder; none is this refinement edge-kind boundary. Targeted repository searches covered `ContextFrom`, `DependsOn`, `RefineNextPhase`, dependency/refinement and the existing mapping tests. This is a bounded duplication screen, not an exhaustive claim about every historical discussion.

All five visible active non-research heads retain the same `replan.go` and effect-contract blobs as main: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908` and `767a3291d6287b238417f2466592a9002810d05a`. The differing older scheduling-policy blob still blocks on `task_dependency`, not context edges. No competing fix for this specific projection was identified. [Active-branch refiner][branch-refiner], [older scheduling variant][branch-policy].

[prompt]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms/campaign/replanning.yaml#L138-L155
[parse]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L536-L575
[mapping-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replanner_fix_test.go#L295-L331
[prior-return]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-resume-return-evidence.md
[decomposition]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L467-L484
[inheritance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/decomposer_planning.go#L505-L533
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/campaign_fact_sync.go#L34-L98
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_phases.go#L68-L164
[effect-contract]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_effect_contract.go#L16-L101
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/agents.md
[tui-admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L56-L84
[tui-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/campaign.go#L88-L140
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_init.go#L67-L75
[transition]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L235-L247
[rolling-wave]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L354-L412
[setup]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L440-L529
[add]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L612-L670
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L692-L750
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L1255-L1321
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L755-L791
[requirement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L297-L374
[update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/replan.go#L671-L691
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/types.go#L712-L727
[readiness]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_tasks.mg#L19-L62
[eligibility]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_tasks.mg#L55-L107
[context-policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/campaign_evidence.mg#L214-L235
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_tasks.go#L250-L293
[document]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L999-L1002
[file-input]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L529-L603
[render]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_results.go#L166-L208
[context-selection]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/task_evidence.go#L426-L455
[branch-refiner]: https://github.com/theRebelliousNerd/codenerd/blob/51cfd42649552866d4aa54c53b68acf5740eb09b/internal/campaign/replan.go#L649-L664
[branch-policy]: https://github.com/theRebelliousNerd/codenerd/blob/51cfd42649552866d4aa54c53b68acf5740eb09b/internal/core/defaults/policy/campaign_tasks.mg#L19-L107

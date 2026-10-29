---
doc-class: deep-dive
subsystem: autopoiesis
implementation-status: partial
last-verified: 2026-10-27
verified-against: fb1d74d0f13b7d0d647c968ae3c1d830eae9b555
supersedes: []
---

# Prompt evolution: separate candidate learning from production instruction deployment

## Decision in brief

CodeNERD already has an end-to-end learning path. The next useful improvement is not to add another learner: it is to give the existing path explicit admission, experiment, publication, and shutdown contracts. A particularly consequential mismatch is that **pending evolved atoms are loaded into the ordinary JIT candidate pool even though automatic promotion defaults off**. Pending is a storage category, not an isolation boundary in the inspected code.

This document proposes a controlled learning lifecycle, not a claim that self-improvement has been demonstrated. All findings below are static source observations at the pinned commit; no model calls, implementations, tests, imports, or runtime reproductions were performed. Actual inclusion of a specific pending atom still depends on context matching, source precedence, selection, conflict resolution, and token budget.

Scope is distinct from the existing research branch audits of usage, consultation, retrieval freshness, verification judging, and tool-effect receipts. This concerns the feedback-to-prompt deployment loop itself. It serves the repository's long-running, bounded, kernel-governed self-improvement vision without allowing Go-side heuristics to replace executive policy.

## 1. What is already wired

The shared factory runs the learning initialization stage (`initLearningLoop`, [internal/system/factory.go:2792](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/system/factory.go#L2792)). That initializer constructs the evolver, registers the evolved-atom reader and strategy provider with JIT, and installs the session recorder ([internal/system/factory_learning.go:85–146](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/system/factory_learning.go#L85-L146), `initLearningLoop`). This is not a chat-only learner awaiting basic factory integration.

The recorder translates a kernel `/done` into a prefilled PASS without purchasing another judge call; failed/hollow turns remain candidates for diagnosis, while unverified turns are dropped ([internal/system/factory_learning.go:233–290](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/system/factory_learning.go#L233-L290), `executionRecordFor`). That is a valuable cost and evidence distinction to preserve. Provider/model provenance and atom IDs are carried forward; process-wide sequence IDs disambiguate delegated turns ([internal/system/factory_learning.go:64–69](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/system/factory_learning.go#L64-L69), `mintRecordIDs`).

Automatic evolution is opt-in through the feature flag, and automatic cycles have a two-minute context budget ([internal/system/factory_learning.go:317–363](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/system/factory_learning.go#L317-L363), `Cortex.runEvolutionCycle`; constant `evolutionCycleTimeout` at line 76). Neither fact establishes that all candidate-serving paths enforce review.

The current architecture corpus was inspected, particularly its “Wave 2 reconciliation” and generated-tool lifecycle sections in [the autopoiesis wiring document](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/Docs/architecture/autopoiesis/WIRING-AND-NOT-BUILT.md). Those generated-tool improvements must not be conflated with prompt-learning lifecycle guarantees. This proposal does not reopen generated-tool publication or alter protected safety logic.

## 2. Source-grounded seams

### 2.1 Pending candidates can reach normal production selection

`DefaultEvolverConfig` sets `AutoPromote: false`; its comment describes the pending queue as the human review surface ([internal/autopoiesis/prompt_evolution/evolver.go:35–80](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/evolver.go#L35-L80)). But `EvolvedAtomManager.Reload` loads both pending and promoted directories ([internal/prompt/evolved_atoms.go:48–65](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/prompt/evolved_atoms.go#L48-L65)).

The loader reads only `wrapper.Atom`, validates and normalizes that atom, and does not retain lifecycle/promotion metadata; `GetMatching` applies ordinary context matching ([internal/prompt/evolved_atoms.go:97–156](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/prompt/evolved_atoms.go#L97-L156), `loadAtomFromFile`, `GetMatching`). `collectAtomsWithStats` appends the evolved source after embedded/project/shard sources, with earlier IDs taking precedence ([internal/prompt/compiler.go:1379–1465](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/prompt/compiler.go#L1379-L1465)). Those candidates enter ordinary selection, dependency/conflict resolution and token fitting ([internal/prompt/compiler.go:815–918](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/prompt/compiler.go#L815-L918), `JITPromptCompiler.compile`).

**Classification:** implemented candidate ingestion; partially specified review/deployment boundary. This is not evidence that every pending atom is served, nor that any harmful instruction ran. It is evidence that “not promoted” alone does not exclude it from consideration. Existing validation, selector, tool-availability and budget protections remain relevant and must remain intact.

### 2.2 Repeated cycles can regenerate from unchanged evidence

`GetFailuresByProblemType` reads all FAIL records, groups by problem/shard/provider/model and applies a minimum count, with no consumed-evidence marker or cycle cursor in its query ([internal/autopoiesis/prompt_evolution/feedback_collector.go:381–418](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/feedback_collector.go#L381-L418)). `RunEvolutionCycle` invokes generation for every qualifying group, truncating generated atoms **within each group**, not across the entire cycle ([internal/autopoiesis/prompt_evolution/evolver.go:332–398](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/evolver.go#L332-L398)). Thus `MaxAtomsPerEvolution=3` does not by itself mean at most three atoms globally when multiple groups qualify.

Generated records preserve source failure IDs, and absent model-supplied IDs receive a random suffix ([internal/autopoiesis/prompt_evolution/atom_generator.go:170–246](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/atom_generator.go#L170-L246), `parseGeneratedAtoms`, `convertToPromptAtom`). Source provenance exists; a consumption/idempotency policy is the missing refinement. The static path permits repeated generation spend with no new evidence; frequency and realized spend are unmeasured.

### 2.3 Background recording has no owned drain in the inspected adapter

`turnEvolutionRecorder.RecordTurn` starts a goroutine per admitted turn, then calls `RecordExecution` and strategy attribution ([internal/system/factory_learning.go:190–209](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/system/factory_learning.go#L190-L209)). `RecordExecution` acquires the evolver mutex, while `RunEvolutionCycle` holds that mutex through judge calls, generation and strategy refinement ([internal/autopoiesis/prompt_evolution/evolver.go:262–284,293–415](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/evolver.go#L262-L415)). Asynchronous return is therefore not bounded outstanding work: arrivals can wait behind a slow cycle.

`Cortex.Close` stops maintenance, then later invokes evolver and context-feedback Close through timed wrappers; `runCloseStep` returns on timeout without canceling or joining the function goroutine ([internal/system/cortex_close.go:26–51,154–168,258–276](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/system/cortex_close.go#L26-L276)). `PromptEvolver.Close` serializes on its mutex and closes stores, but it has no recorder-admission state or recorder wait group ([internal/autopoiesis/prompt_evolution/evolver.go:852–873](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/evolver.go#L852-L873)). This supports a missing lifecycle guarantee, not a measured shutdown-loss incident. A record goroutine may acquire the mutex after Close and encounter a closed store. There is no guaranteed order merely because both use the same mutex.

### 2.4 Attribution is association, and replay is not harmless

`applyExecutionOutcomeLocked` credits every listed evolved atom with the verdict outcome; `recordAtomUsageLocked` increments usage/success and persists them ([internal/autopoiesis/prompt_evolution/evolver.go:640–666,824–836](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/evolver.go#L640-L836)). `ShouldPromote` requires at least three uses and success rate at or above threshold ([internal/autopoiesis/prompt_evolution/types.go:314–324](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/types.go#L314-L324)). These statistics are not estimates of causal incremental utility.

`RecordExecution` applies attribution before database recording. `FeedbackCollector.Record` uses `INSERT OR REPLACE` but increments in-memory total counters on each successful call; `UpdateVerdict` increments failure counts whenever passed FAIL, without comparing the old outcome ([internal/autopoiesis/prompt_evolution/feedback_collector.go:217–270,449–478](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/autopoiesis/prompt_evolution/feedback_collector.go#L217-L478)). A future retry layer cannot simply replay this API and assume attribution/statistics are idempotent. Normal IDs are already collision-resistant in the inspected process; this concern is repeated submission or failure recovery, not an assertion of routine ID collisions.

## 3. Proposed lifecycle and contracts

These are proposed data shapes, not current importable configuration or Mangle declarations.

### Candidate state machine

`generated -> quarantined -> experiment-approved -> trialed -> reviewed -> promoted -> retired`

Every transition has candidate content hash, source-evidence set hash, model/policy/corpus revision, decision issuer, timestamp, and reason. Rejected/retired states never become live merely because a file is found. Existing pending files migrate to quarantined unless the owner explicitly authorizes legacy treatment; never silently promote them during migration.

Normal production compilation admits only promoted versions. Trial compilation requires a kernel-derived experiment admission tied to task cohort, candidate hash, expiry, and cost budget. Go validates and materializes the immutable envelope; Mangle decides eligibility from declared facts. A trial does not gain new tools, permissions, or authority over safety gates. Declaring and validating any proposed predicates is implementation work outside this document.

An admission receipt records: request/turn identity, candidate version, experiment assignment, eligible scope, exact included-atom manifest, and actual inclusion/exclusion reason. **Eligible**, **selected**, **served**, **exposed**, and **credited** are separate events. A zero-token or excluded candidate gets no exposure credit. Authorization must be checked against the current policy epoch rather than cached directory membership.

### Bounded collection and cycle transaction

1. Admit immutable turn records under a lifecycle-owned queue with capacity Q and explicit overflow outcome. Never claim durable learning merely because enqueue returned.
2. Use stable event identity and payload hash. Repeated identical event returns its prior receipt; a conflicting payload for the same identity is a reconciliation error.
3. Persist raw observations before applying credits. Make the unique attribution key `(event_id, verdict_version, candidate_version)` durable. Revised verdicts reverse or supersede old contribution rather than counting twice.
4. Select a bounded evidence snapshot outside long-held shared locks. Derive a generation key from sorted evidence IDs/versions, group, generator version, and requested mechanism. Claim that key transactionally so concurrent processes do not independently consume the same work.
5. Release metadata locks before any LLM call. A lease/fencing generation prevents a timed-out worker from publishing over a replacement worker. Publish candidate metadata and completed-generation state together, or retain explicit recoverable ambiguity.
6. Budget globally across groups: maximum judge calls, generation calls, candidate count, prompt/output tokens, wall time, and retained bytes. Select groups by kernel-derived policy, with age/fairness to prevent newest-record starvation.
7. Graceful close stops admission, cancels generation, drains accepted records and reconciles unresolved publication before stores close. A timeout is an incomplete-shutdown result, not proof of cleanup. Hard crash durability requires a journal/outbox and restart reconciliation; a wait group alone cannot provide it.

The official [Go pipeline guidance](https://go.dev/blog/pipelines) motivates explicit cancellation and joined worker lifetimes. SQLite's [REPLACE semantics](https://www.sqlite.org/lang_conflict.html) replace a conflicting row, not external counters or previously written atom statistics; transaction design must include those effects. Neither source proves this proposed design correct.

## 4. Training/learning evaluation contract

This is prompt adaptation, not weight training. Log task lineage, repository revision, model/provider, complete prompt manifest, candidate assignment, verification policy, true exposure, and all cost categories. Failure diagnosis may use a model, but a diagnostic PASS must not erase the kernel's failed/hollow observation. Keep raw outcome, explanatory label, and promotion decision separate.

For candidate a, estimate incremental utility rather than interpreting success among its users as benefit:

`Delta(a) = E[verified_success | randomized trial a] - E[verified_success | randomized control]`.

Hold task distribution, tools, available evidence, verification requirements, model versions and intervention budget fixed. Randomize at task/repository-family level when adjacent turns share context; do not contaminate control by retaining a trial atom in history. Use intention-to-treat as primary analysis and exposure-aware secondary analysis. Report intervals, failure subtypes, safety regressions, and subgroup effects. Promotion needs explicit review plus predeclared minimum evidence and regression bounds, not a universal numeric threshold invented here.

[Automatic Prompt Optimization](https://arxiv.org/abs/2305.03495) supplies precedent for deriving candidate edits from failure feedback and evaluating alternatives with search/selection. It does not establish CodeNERD improvements or justify uncontrolled live deployment. Keep candidate-generation examples separate from held-out evaluation repositories and temporal future tasks; otherwise the feedback loop can memorize its own failures and report misleading progress.

## 5. Full cost accounting

For a horizon H, use:

`C_total = C_task + C_record + C_judge + C_generate + C_trial + C_review + C_refresh + C_storage + C_recovery`.

Use a disjoint ledger: each underlying operation belongs to exactly one category (for example, trial-task model and verifier work belong to C_trial, not also C_task). Report a resource vector of token classes, CPU/GPU time, I/O, latency, storage, and human minutes; sum categories only after converting to an explicitly stated common unit such as dollars under fixed rates. Do not add raw tokens to human minutes. Count input, cache read/write, output and reasoning tokens separately where reported; unknown usage remains unknown. Include failed/canceled generation, discarded candidates, additional task turns, verifier/tool execution, JIT reload/cache eviction, human review time, and background CPU/I/O. Quality per task token alone hides adaptation cost.

With G qualifying groups and per-group atom cap K, the inspected loop can store up to G*K atoms per cycle, assuming each group supplies at least K valid atoms and storage succeeds. This is a structural upper count conditional on groups, not a measured rate. If F historical failures remain and C cycles repeatedly consider them, evidence processing can revisit O(CF) records; a consumed-version index targets O(F_initial + sum(F_new)) record-ingestion work across the horizon. This bound excludes index maintenance, rescans, generation, deliberate reanalysis, and evidence-retention work; account for those separately. Deliberate reevaluation under a changed generator/version remains possible. Do not suppress intentional reanalysis; require a new rationale/version and charge it explicitly.

Queue backlog during a cycle of duration T at arrival rate lambda can reach roughly lambda*T records before service catches up; this is an arrival approximation, not a measured memory bound. A bounded queue caps memory at O(Q*record_size), but imposes backpressure, rejection, or disk-spool costs. Specify which tradeoff the kernel authorizes and expose loss counters. Trial storage grows with exposure events, not only candidates; retention must preserve enough provenance for rollback and honest aggregate statistics.

Report break-even honestly: cumulative avoided task cost must exceed adaptation and review cost at noninferior verified quality. One successful trial is not amortization, and a shorter prompt that causes more repairs is not efficiency.

## 6. Proposed evaluation matrix — not executed

| Scenario | Required observation | Failure caught |
|---|---|---|
| Unique matching pending atom, AutoPromote false, normal production entry | Candidate absent from served manifest; quarantine reason visible | Directory lifecycle confused with review |
| Same atom under authorized experiment envelope | Exactly scoped exposure, unchanged tool authority | Trials either impossible or globally leaked |
| Two cycles with identical failure snapshot | No duplicate generation charge unless explicit reanalysis version | Unchanged evidence repeatedly purchased |
| Many groups | Global budget respected; aged groups eventually admitted | Per-group cap misrepresented as cycle cap |
| LLM cycle blocked while turns finish | Bounded outstanding records and visible admission results | Unbounded detached goroutines |
| Graceful close during accepted record write | Reopened store contains each acknowledged event exactly once | Shutdown races record persistence |
| Failure after credit computation, before record persistence | Recovery applies no duplicate credit | Split durability corrupts promotion statistics |
| Verdict revised FAIL to PASS and replayed | Aggregate reconstructed from unique current contributions | Counters drift from persisted evidence |
| Two processes claim same generation key | One admitted generation or explicitly reconciled ambiguity | Duplicate spend and publication races |
| Held-out task families with/without candidate | Verified task outcomes, complete costs, calibrated intervals | Correlation mistaken for causal benefit |

Compare current behavior, lifecycle-only changes, lifecycle plus evidence cursor, and full controlled trials. At socket level feed identical immutable records and frozen candidate responses to compare durability and accounting. At complete-system level include normal boot, shared compiler, task execution, validation, feedback, restart and shutdown. Increase workloads across 10, 100, 1,000 and 10,000 recorded turns and independently vary active groups/candidates/concurrency; these are proposed fixture scales, not capability or benchmark results.

## 7. Failure modes and build queue

Quarantine without an experiment path starves candidates of evidence. Over-aggressive deduplication blocks useful reanalysis. A single global lock avoids races but converts model latency into record backlog. Per-process uniqueness does not establish cross-process ownership. An outbox without privacy/retention controls preserves sensitive task material indefinitely. A trial statistically improved on easy tasks can harm rare hard tasks. A successful kernel gate proves that gate's evidence contract, not universal correctness. Preserve these distinctions in operator output.

| Proposed gap | Priority | Dependency | Checkable exit |
|---|---|---|---|
| GAP-PE-RESEARCH-01: lifecycle-aware prompt admission | P1 | Explicit owner-approved trial contract | Normal compiled manifests exclude quarantined candidates; authorized trials include only the pinned version/scope |
| GAP-PE-RESEARCH-02: consumed-version generation claims | P1 | Durable event/generation identity | Same evidence/version cannot repurchase generation after success or restart; authorized reanalysis is separately identified |
| GAP-PE-RESEARCH-03: owned bounded recording/drain | P1 | Admission/receipt model and queue policy | Accepted records drain before dependency close; overload and incomplete shutdown are explicit |
| GAP-PE-RESEARCH-04: idempotent attribution and complete costs | P1 | Unique observation ledger | Replays/revisions reproduce the same aggregate and candidate credits; full spend includes failed adaptations |
| GAP-PE-RESEARCH-05: controlled incremental-utility trials | P2 | 01–04 plus held-out workload | Preregistered quality/regression/cost analysis supports a reviewed deployment decision, including a no-promote result |

These identifiers are local research proposals, not existing implementation commitments. No source or production policy is changed by this document.

## As built (2026-10-27)

All four P1 seams were confirmed against current source before building (pending files were served; cycles re-read every FAIL; recording was a goroutine per turn behind a mutex held across model calls; `INSERT OR REPLACE` plus unconditional counters).

**Implemented**

- GAP-PE-RESEARCH-01, lifecycle-aware admission. `internal/prompt/evolved_atoms.go`: `EvolvedAtomManager` keeps `EvolvedCandidate` (state, `Version`, provenance) per file; `pending/` loads as `EvolvedQuarantined`, `promoted/` as `EvolvedPromoted`. `GetMatching` serves promoted candidates on context match and a quarantined one only when `prompt_trial_admitted(AtomID, Version)` names its exact `EvolvedCandidateHash` (content + selectors + composition fields), as a non-mandatory copy; `collectEvolvedAtoms` reads admission from the compile's private kernel scope (`cc.Kernel`). Kernel side: `prompt_trial_envelope/4`, `prompt_trial_exposures/2`, `prompt_trial_admitted/2` in `schemas_prompts.mg`; rule in `policy/prompt_trials.mg` (unexpired via `current_time`, exposures below budget; every literal fails closed). Go materializes envelopes: `PromptEvolver.ApproveTrial/RevokeTrial/Trials/SetTrialFactSink` (`evolver_candidates.go`), `trialFactSink` in `internal/system/factory_learning.go`. Operator surface: `/trial-atom <id> [hours] [max-exposures] | revoke <id> | list` (`cmd/nerd/chat/commands_evolution.go`, parser `trial_command.go`). Promotion and rejection end a trial.
- GAP-PE-RESEARCH-02, consumed-version claims and global budget. `evidence_ledger.go`: `generation_claims` (lease, owner fence, statuses claimed/completed/released/publish_failed) and `consumed_evidence (task_id, verdict_version)`; key = `generationClaimKey(pin scope|group, atomGeneratorVersion, sorted task@verdict_version)`. `GetFailuresByProblemType` returns only unconsumed failures. `evolver_cycle.go`: `RunEvolutionCycle` judges at most `MaxCycleJudgeCalls`, serves groups oldest-evidence-first, and caps generation+refine calls and stored atoms per cycle across groups; the rest is reported as deferred.
- GAP-PE-RESEARCH-03, owned recording and drain. `evolver_recording.go`: `recordQueue` (bounded channel + one worker), `SubmitExecution` returns `accepted | rejected_full | rejected_closed`; `PromptEvolver.Close` stops admission, cancels the cycle (`lifeCtx`), drains within `RecordDrainTimeout` (abandoned records counted and returned as an incomplete-shutdown error), joins cycles and synchronous `RecordExecution`, then closes stores. No model call runs under `pe.mu`; publication completes the fenced claim under the lock. `turnEvolutionRecorder.RecordTurn` and chat `recordShardExecution` use the queue; `cortex_close.go` gives the step drain + step timeout.
- GAP-PE-RESEARCH-04, idempotent attribution and cost. `FeedbackCollector.Record` now returns `RecordOutcome`: identity = task id, `payload_hash` decides duplicate vs `ErrConflictingEvent`; `UpdateVerdict` is a no-op for the same `verdict_version` and moves the failure counter by the difference. Ledger tables `candidate_exposures` and `candidate_attributions (event, candidate, candidate_version, verdict_version, superseded)`; credit is applied after the observation persists, a revised verdict supersedes, and candidate counts are reproduced from rows (`refreshCountsLocked`). `RecordAtomUsage` (the non-idempotent increment) was deleted. Spend: `adaptation_spend` filled by `meteredLLM` around the judge/generator client (judge/generate/refine; ok/error/unusable/discarded/publish_failed), surfaced in `GetStats().Spend`, `/evolution-stats` and `nerd` systems output.
- Tunables: `prompt_evolution` section (`internal/config/prompt_evolution.go`: queue capacity, drain timeout, cycle judge/generation/atom caps, claim lease, trial exposures/TTL), defaulted, validated in `Check`, mapped by `EvolverConfigFrom`. `EvolutionStats.SuccessfulCycles/AvgCycleDuration` are now filled.

**Ruled differently**

- Trial scope is the candidate's own selectors pinned by the version hash, not a separate envelope scope column: widening scope changes the version and ends admission.
- Expiry is enforced twice (Go publishes only live envelopes; the rule requires `current_time`), so a stale or missing clock fails closed rather than extending a trial.
- A generation that returns but yields no candidate still consumes its evidence (recorded as `unusable`); only a failed model call releases the claim. Re-trying identical evidence with the same generator would otherwise repurchase nothing every cycle.
- Exposure = served in a recorded turn's atom list. Eligible/selected events are not recorded separately (the turn record carries only included atoms).
- Legacy file counts are not evidence: all candidate counts are re-derived from the ledger at load, so pre-ledger pending atoms start at zero and cannot auto-promote from unreviewed exposure.
- Token usage stays unknown: the evolver's `LLMClient` returns text only, so the ledger records calls, bytes and duration, not estimated tokens.

**Deferred**

- GAP-PE-RESEARCH-05 controlled incremental-utility trials (randomized cohorts, held-out repositories, intervals): needs live models and workloads. The mechanism pieces exist (pinned-version envelope, exposure and credit rows kept apart, spend ledger).
- Crash-durable outbox for accepted-but-unpersisted records: a hard crash still loses the in-memory queue; graceful close drains it.
- Cross-process judging of the same unevaluated record is not claimed (generation is).
- `StrategyStore.RecordOutcome` remains non-idempotent itself; the queue calls it only on first persistence of an event.

## Appendix Preserving served candidate identity through exposure accounting

**Recommendation:** extend the implemented lifecycle ledger with compilation-owned candidate receipts. Exposure and credit must identify the evolved candidate version actually included in the model request, not whichever candidate currently has the same string ID when the recording worker runs. Keep the existing quarantine, trial admission, private-kernel compilation, source precedence, bounded recording queue, and idempotent ledger. The gap is the identity handoff between them.

**Evidence boundary:** static source inspection on 2026-10-28 at main `356f65e0e6bb639adaa90ab36212daa714114871`. The relevant compiler source collection, evolved manager, section expansion, generator, candidate lifecycle, recording, ledger, and factory-learning files were individually compared with `feat/conversation-brief` at `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c` and had identical contents. No implementation, tests, model inference, profiling, or production experiments were run. Counterexamples below are proposed source-derived sequences, not reproduced incidents.

The current audit's [as-built section][lineage-as-built] already claims exact-version trial admission and attribution. Its counterparts on [the current research branch][lineage-current-research] and [the older research branch][lineage-historical-research] were also inspected. The March QA report already identifies duplicate IDs overwriting the candidate file/map and losing the old mutable counters; that overwrite is not a new discovery here. The new trace follows source precedence and the subsequently implemented versioned ledger: an unserved candidate can now receive a different source's exposure, or a served version can disappear/change identity at the recording boundary. [Historical collision report][lineage-prior-collision]

This appendix identifies concrete conformance gaps in the implemented handoff; it does not present versioned evidence accounting as a new design. The generation-aware JIT audit's snapshot, source-vector, vector-scope, and degradation findings concern different boundaries. Likewise, the historical section-admission finding concerns post-selection eligibility, not ownership of exposure evidence.

### The working protections and the missing handoff

When a compiler and judge client are available and evolver initialization succeeds, the normal production factory registers an `EvolvedAtomManager`, installs the trial fact sink when a kernel is present, and connects the turn recorder when the session executor is present. A quarantined candidate is returned by `GetMatching` only when the private kernel admits its exact hash; the trial copy is made nonmandatory. These protections are real and must remain. Recording is not evidence that automatic generation is enabled; the witnesses require an existing/generated candidate, not an assertion that every default session generates one. [Factory wiring][lineage-factory], [candidate admission][lineage-serving]

The downstream path loses information:

| Boundary | Inspected behavior |
|---|---|
| Canonical candidate collection | Embedded atoms win, then project and selected-expert atoms; later duplicate IDs, including evolved atoms, are discarded. This correctly prevents a generated duplicate from overriding a built-in. [Source][lineage-precedence] |
| Compilation to turn observation | `turnAtomIDs` projects `CompilationResult.IncludedAtoms` to bare IDs. It does not retain candidate origin or the admitted candidate hash. [Source][lineage-turn-ids] |
| Turn observation to asynchronous recording | `executionRecordFor` copies those IDs, and `RecordTurn` enqueues the record. The ordinary session path does not fill the optional `ExecutionRecord.PromptManifest` here. [Conversion][lineage-record-conversion], [queue admission][lineage-queue] |
| Exposure reconstruction | After persisting the observation, `recordExecution` calls `candidateRefs(exec.AtomIDs)`. That helper looks up each ID in the current `pe.evolvedAtoms` map and computes the hash of its current body. It neither checks which source won compilation nor consumes an immutable served-version receipt. [Recording][lineage-recording], [lookup][lineage-current-lookup] |
| Durable attribution | `RecordExposures` inserts the supplied version, and `ApplyCredit` credits versions from those exposure rows. The ledger faithfully preserves the identity it is given; it cannot repair a wrong identity upstream. [Ledger][lineage-ledger] |

`ExecutionRecord` does have an optional prompt manifest, so this is not a claim that no manifest type exists. The current `AtomManifestEntry` contains ID, category, source, scores and render mode, but no exact evolved-candidate version, and `candidateRefs` does not read it. A context/cache hash identifies a compilation context; it is not a per-candidate provenance receipt. [Record type][lineage-record-type], [manifest type][lineage-manifest]

### Counterexample one A candidate receives credit for a different source with the same ID

The generator accepts an explicit YAML atom ID unchanged; it generates an `evolved/...` UUID only when the ID is empty. Publication persists the candidate and assigns it into `pe.evolvedAtoms[ID]` without a collision check at these sites. [Generator][lineage-generator-id], [publication][lineage-generation-publication], [storage][lineage-candidate-storage]

Proposed fixture:

1. Create an ordinary built-in or project atom with ID `x` that is actually selected and emitted for a qualifying turn.
2. Generate and store a distinct evolved candidate also named `x`. Keep it quarantined with no approved trial. An explicit generator-returned ID takes this path; no same-ID mutation during the turn is required.
3. Compile and finish the turn. Quarantine correctly excludes the evolved candidate. Even if it were trial-admitted or promoted, canonical source precedence would still retain the earlier atom rather than the duplicate evolved body.
4. Let the normal recorder process the resulting `AtomIDs`, which contain `x` because the earlier source was served.
5. `candidateRefs` finds the quarantined entry at `pe.evolvedAtoms[x]` and records its version as exposed. If the persisted event receives a qualifying verdict, the ledger can credit that version despite its body never reaching the model.

The conditions matter: the colliding earlier-source atom must actually survive selection and fitting; the turn must enter the learning recorder; the evolved entry must still be present when lookup occurs; and the relevant persistence operations must succeed. A unique evolved ID is the natural negative control. An omitted earlier-source atom produces no ID and therefore no collision exposure.

**Bounded consequence:** false exposure, incorrect success/failure evidence, and potentially misleading version-specific trial counts. This does not show that the quarantined instruction executes or that execution permissions widen. The existing precedence and quarantine checks still protect prompt content. `AutoPromote` defaults to false, and the normal JSON configuration mapping does not expose an `AutoPromote` field; false credit must not be described as automatic deployment through ordinary settings. Any consequence involving operator promotion or a nondefault API configuration needs its own evidence. [Default and mapping][lineage-default-promotion]

Independent review confirmed this collision-lineage distinction and the automatic-promotion qualification. The relevant repair is not to weaken canonical precedence so the credited candidate becomes the one served.

### Counterexample two Rejection can erase an exposure that already happened

`RejectAtom` moves a pending candidate into the rejected directory and deletes its current map entry before publishing the reduced trial set. That correctly prevents future ordinary admission. It does not undo a model request that already carried the candidate. [Rejection][lineage-rejection]

Proposed fixture:

1. Approve a trial of candidate `x` at version `v`; compile a request that actually includes it under the existing admission and budget rules.
2. Before the finished-turn observation is processed, reject `x`. The turn can still be in flight or its record can already be waiting in the bounded recording queue.
3. Process the successful or failed observation.

The current map no longer contains `x`, so `candidateRefs` emits no reference. `RecordExposures` returns successfully for an empty reference list. The historical exposure is absent even though the request carried the instruction. This is an accounting loss, not a claim that rejection should permit another exposure or that an in-flight request is guaranteed to be canceled. [Current lookup][lineage-current-lookup], [empty exposure behavior][lineage-ledger]

A related conditional case is same-ID replacement: a later generation may supply an existing explicit ID, and `storeEvolvedAtom` replaces the map entry. If the earlier version was served but recording happens after replacement, the current lookup can attribute that old exposure to the new version. The generator permits the necessary ID shape; occurrence frequency is not established. There is no inspected evolved-atom `RefineAtom` path, and strategy refinement must not be substituted as evidence of one.

The ledger's first-write rule makes source identity especially important. `candidate_exposures` is keyed by event and candidate ID, and insertion is `INSERT OR IGNORE`: a mistaken first version is not corrected by replay with another version. This is a useful idempotency property for matching receipts, but it needs a separate conflicting-receipt result rather than silently treating different lineage as the same observation. [Ledger schema and insertion][lineage-ledger-schema]

### Related exact-version gap Section metadata is not in the candidate digest

`EvolvedCandidateHash` includes content, selectors, priority, mandatory/exclusive status, dependencies, conflicts and tool requirements. It omits `SectionOf` and `SectionOrder`, although `expandSections` uses them to bring sections into the selected set and calculate their scores relative to the selected head. `ApproveTrial` pins this hash, and `GetMatching` compares the loaded candidate against it. [Hash][lineage-hash], [section fields][lineage-section-fields], [expansion][lineage-expansion], [trial approval][lineage-approval]

A section-order-only edit therefore keeps the same hash while potentially changing ranking and tight-budget inclusion. An adequate fixture uses a valid persisted evolved wrapper, holds body/selectors/dependencies constant, changes only its positive `SectionOrder`, and places another optional atom at a score/budget boundary where that order affects which instruction fits. The old trial envelope still names the unchanged hash. A source reload moves the evolved-source generation and prevents stale-cache reuse, but it does not revoke the old trial authorization because the candidate digest is unchanged.

Preserve the reviewer qualifications:

- The current atom generator does not populate section fields. This witness requires supplied or edited persisted-wrapper metadata, or another explicitly established producer; it is not evidence that ordinary generation already emits such candidates.
- Canonical YAML/database handling can add `SectionOf` to `DependsOn`. Since dependencies are hashed, changing a head and also changing that implied dependency can change the digest. The independent `SectionOrder` omission remains the clean witness.
- The evolved-wrapper loader unmarshals `PromptAtom` directly and applies its own validation/normalization; do not assume it uses the canonical YAML section-enrichment path.
- A section-free candidate, an unchanged order, or an ample budget that retains every affected atom is a negative control for output differences. Equal digest despite changed serving metadata is still the narrower identity discrepancy.

The existing [hash test][lineage-hash-test] correctly checks content and selector changes and intentionally excludes derived bookkeeping fields. Extend that contract rather than invalidating all harmless metadata changes. This is a digest-completeness refinement adjacent to receipt lineage, not a repeat of the earlier post-Mangle section-gating audit.

### Proposed contract and bounded repair

Names below are design notation, not existing APIs.

```text
ServedCandidateReceipt:
  request or exposure event identity
  canonical source kind and source identity
  candidate ID and admitted candidate version
  selected rendering mode and emitted-fragment identity
  inclusion disposition
```

1. Preserve evolved origin and the approved candidate hash when an admitted candidate enters compilation. Carry that metadata through precedence, selection, fitting, cache reuse and private-result copying. Create a served receipt only for the candidate that actually survives into the outgoing request.
2. Keep the admitted hash distinct from rendered-fragment identity. Trial admission makes an optional copy, and budget fitting may choose concise/min content. Rehashing such a presentation copy can produce a different value from the authorized candidate. Carry the original admitted version and separately record which rendering was emitted.
3. Hand immutable receipts to the session/delegated execution record and bounded queue. Recording must not resolve historical identity through a mutable current-candidate map. Rejection changes future eligibility, not past exposure. Unknown or deleted current candidates retain their historical receipt rather than losing the observation.
4. Distinguish ordinary-source atoms from evolved candidates even if string IDs collide. Rejecting generator collisions or enforcing a namespace is useful defense in depth, but is not a replacement for receipt provenance: a later rename/reload or legacy collision must not corrupt history.
5. Persist the receipt with the observation or with an explicitly recoverable idempotent handoff. Duplicate identical receipts remain no-ops; the same event/candidate carrying conflicting source/version identity must produce an observable reconciliation result. Decide explicitly whether one event may contain multiple actual versions; do not silently force that case into the current one-version primary key.
6. Complete the semantic candidate digest with fields that affect admission or composition, including section order. Version the digest contract and specify treatment of outstanding trial envelopes during migration. Do not silently bless a changed digest under an old approval.
7. Continue deriving trial eligibility in Mangle. Go's role is faithful identity capture and durable transport, not deciding that a candidate deserves exposure, success credit or promotion.

The minimum useful change is a per-included-candidate identity sidecar, not a large general execution-receipt platform. Existing ledger tables and private compilation results provide much of the infrastructure. A later full provenance receipt can extend this seam without delaying the concrete invariants.

The [End-to-End Arguments in System Design](https://web.mit.edu/saltzer/www/publications/endtoend/endtoend.pdf) motivates checking application-specific delivery at the endpoint that can identify the actual served candidate. This is a design inference, not proof of measured outcomes in CodeNERD.

### Costs and proposed evaluation

For `k` actually included evolved candidates, a compact receipt adds O(k) records per relevant request or exposure event. Avoid copying whole candidate bodies into every event: retain an immutable content-addressed candidate record and small source/version references, with a declared retention policy. Hashing already occurs at candidate loading/approval; propagate the captured hash rather than recomputing all bodies on the turn's recording path.

The costs are schema/serialization migration, cache-result sidecar ownership, session and delegated producer wiring, conflict diagnostics, and retention of rejected or superseded candidate identities. Queue memory increases with receipt size; persistence may need one additional transactional write or a widened existing write. Digest changes can invalidate outstanding approvals and require renewed operator review. None of these costs is justified by a claimed token saving: the primary benefit is trustworthy learning evidence and avoiding decisions based on exposure that did not occur.

**Proposed checks, all unrun:**

| Fixture | Required observation |
|---|---|
| Quarantined evolved ID collides with an emitted embedded/project atom; no trial exists | The ordinary atom's receipt is retained; evolved candidate receives neither exposure nor credit. |
| Trial-admitted or promoted duplicate loses canonical precedence | No evolved exposure is inferred merely from the winning source's identical ID. |
| Unique admitted candidate actually emitted | One correctly versioned exposure; existing verdict supersession and duplicate-event behavior preserved. |
| Eligible candidate omitted by budget, conflict or missing prerequisite | No served exposure; eligibility is not credited as use. |
| Request includes trial candidate, then operator rejects before queue drain | Historical exposure retains the exact served version; future admission remains denied. |
| Explicit same-ID generation replaces the candidate before recording | Old request credits old version only; new version begins with its own evidence. |
| Same event replay with identical versus conflicting receipt | Identical replay is idempotent; conflicting lineage is observable, not silently ignored. |
| Cache hit, shared compile, optional trial copy and concise/min rendering | Source and admitted version remain intact; rendered identity describes the actual fragment. |
| Persisted wrapper changes only positive section order | Candidate semantic digest changes; old trial envelope cannot authorize changed composition. |
| Canonical section-head change that also changes DependsOn | Already-hashed dependency behavior remains effective; do not misdiagnose it as an unchanged-version case. |
| Content-free bookkeeping or normalized selector spelling changes | Intentional hash stability from existing tests remains. |
| No evolved candidates, normal disabled-auto-promotion configuration | Negligible extra payload; no invented promotion or authority behavior. |

Start with deterministic real-kernel compilation and SQLite ledger fixtures, including the normal factory/session recorder and delegated producer paths. Direct hash or ledger tests alone cannot establish that the version served was the version recorded. Use controllable worker barriers for the rejection/replacement sequence rather than timing sleeps. Inspect existing test definitions as controls, not as evidence these new sequences pass.

Measure wrong-source/wrong-version credits, missing actual exposures, conflicting receipts, queue admission/drain outcomes, bytes per event, write latency, allocations, retained candidate metadata and end-to-end recording delay. Later model studies should hold task/model/tools/budget/verdict criteria fixed and evaluate whether corrected evidence changes candidate decisions and verified task quality. Include failed and omitted candidates; a cleaner-looking success rate after dropping difficult evidence is not an improvement.

**Checkable exit:** every exposure row names the actual evolved source and admitted version in the outgoing request; an ID collision cannot credit an unserved candidate; rejection or replacement cannot rewrite historical lineage; and composition metadata cannot change under the same exact-version approval.

[lineage-as-built]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md#as-built-2026-10-27
[lineage-current-research]: https://github.com/theRebelliousNerd/codenerd/blob/0c38d4fc987213a17f6570cc7d8dae34c87d8ace/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[lineage-historical-research]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[lineage-factory]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_learning.go#L85-L151
[lineage-serving]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/evolved_atoms.go#L278-L310
[lineage-precedence]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_sources.go#L33-L104
[lineage-turn-ids]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_learning.go#L174-L190
[lineage-record-conversion]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_learning.go#L268-L317
[lineage-queue]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_learning.go#L226-L244
[lineage-recording]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evolver_recording.go#L190-L222
[lineage-current-lookup]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evolver_recording.go#L279-L313
[lineage-ledger]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L150-L221
[lineage-record-type]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/types.go#L118-L156
[lineage-manifest]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/manifest.go#L7-L30
[lineage-generator-id]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/atom_generator.go#L226-L282
[lineage-generation-publication]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L244-L277
[lineage-candidate-storage]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L43-L70
[lineage-rejection]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L151-L167
[lineage-ledger-schema]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L46-L65
[lineage-hash]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/evolved_atoms.go#L57-L112
[lineage-section-fields]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms.go#L215-L223
[lineage-expansion]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/sections.go#L33-L116
[lineage-approval]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evolver_candidates.go#L277-L315
[lineage-hash-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/evolved_lifecycle_test.go#L108-L130
[lineage-prior-collision]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/.quality_assurance/autopoiesis/2026-03-28_10-00-EST_prompt_evolver_boundary_analysis.md#L360-L369
[lineage-default-promotion]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/evolver.go#L80-L128

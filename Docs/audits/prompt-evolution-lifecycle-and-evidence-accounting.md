---
doc-class: deep-dive
subsystem: autopoiesis
implementation-status: planned
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

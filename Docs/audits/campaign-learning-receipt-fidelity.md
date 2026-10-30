---
doc-class: research-proposal
subsystem: campaign-learning-receipts
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 6cbe3962fbdb24a1639b26e71f8096e4f4330048
verification-method: static-source-inspection
---

# Campaign learning receipts must preserve promotion values and observation identity

## Finding and decision

A campaign can report more acquired or applied learnings without receiving a new learning conclusion. After each successful task, `applyLearnings` queries every currently derived `promote_to_long_term` tuple. It appends a new record for each returned tuple, sets every record's type to `/success_pattern`, copies the tuple's **kind** into `Pattern`, discards its **value**, and uses the just-completed task's description and result as `Fact`. The declared relation is `(FactType, FactValue)`; the adapter does not preserve that contract. [Schema][schema], [adapter][adapter].

The stock campaign policy provides a concrete source: a completed phase with an objective and a passed checkpoint derives `promote_to_long_term(/phase_success, V)`, where V is the string representation of the objective type. While that conclusion remains true, later successful tasks can each append it again, now accompanied by their own unrelated task context. One standing conclusion can therefore become several campaign records, none containing its value. [Producer][producer], [objective projection][objectives], [successful task path][task-success].

**Recommended decision:** make campaign learning records faithful, idempotent receipts for typed evidence, and distinguish seeing a conclusion from applying it. The demonstrated effect is inaccurate campaign records, counts and presentation—not proven prompt poisoning, automatic training, unsafe execution or a false campaign-completion verdict. No current path from these particular records into the LearningStore or JIT prompt compiler was established.

## Review status and prior work

Main initially remained `388205138ba6d47c1cff0c5e97c896dfa642a391`; it then advanced during review to `6cbe3962fbdb24a1639b26e71f8096e4f4330048`, rechecked at approximately 17:18 UTC on October 30, 2026. Root, campaign and core guidance, the main audit inventory, targeted prior-audit and QA searches, and recent research scopes through `research-main-20261030-1651` were inspected. That research branch ends at `90c84648725aa263c8a1ba2458390b572402e4de`, four documentation-only commits ahead of the earlier main: completion storage holds, generated-test authority, knowledge-synthesis grounding, and recurse prior-reason budgets. [Main][main], [guidance][guidance], [campaign guidance][campaign-guidance], [research comparison from its original base][research].

The new main comparison contains three commits and 52 changed files. It fixes restart recovery, constitution adoption and recurse gate contracts/subsumption. The promotion adapter and both campaign promotion producers are unchanged. The inspected query change adds a policy-adoption read lock and preserves per-query tuple deduplication; the campaign ownership addition concerns task-check receipts, not these promotion supports. New campaign guidance requires measured recurse runtime contracts to retain their declared command, environment and accepted exits, and refuses restored recurse runs without them. This witness uses an admitted ordinary campaign, not a restored recurse run, and still requires every task/checkpoint gate to pass. No concurrent policy reload or source withdrawal is needed. The finding therefore remains supported at the new pin; relevant moved citation ranges below were updated. [Main delta][main-delta], [new campaign contract guidance][new-guidance], [runtime admission guard][runtime-guard].

The complete `applyLearnings` function was also compared with the five active non-research heads: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. Its body is identical at those heads. This is a comparison of the affected adapter, not a claim that entire branches or all surrounding behavior are equivalent.

This is distinct from the recent **generated learning event domain** audit: that document follows generation and invocation feedback into a tool-quality aggregate and refinement eligibility. Here the object is `campaign.Campaign.Learnings`, and the confirmed consumers are persistence and reporting. Neither fix substitutes for the other. It is also distinct from **campaign completion storage hold**: this witness uses successful persistence, not a lost snapshot. [Generated learning audit][generated-audit], [storage-hold audit][storage-audit].

The older campaign declaration inventory already records the two-argument promotion schema. A QA journal also names `applyLearnings` in an outage discussion, but does not establish this projection or repetition problem; its suggestion that this function itself calls the LLM does not describe the current body. Credit those prior inspections without treating them as evidence of model calls here. [Prior inventory][inventory], [older QA][qa].

Independent source review corroborated the adapter mismatch and standing-row scenario. This report is static inspection only. No application, tests, compiler, benchmark, model inference or failure injection was run. Only this research Markdown was written. All scenarios and evaluation results below are proposed or source-predicted.

## Production mechanism

### The producer is a current policy conclusion rather than a consumed event

The kernel loads the constitution's policy directory, including `campaign_autopoiesis.mg`. Its rule joins a completed `campaign_phase`, a `phase_objective`, and a `phase_checkpoint` whose passed slot is `/true`. The objective's second argument comes from `Objective.Type`, not the phase's context profile. The rule then projects away phase and campaign identity, producing only the promotion kind and value. [Constitution loading][loading], [producer][producer], [objective projection][objectives].

A healthy normal phase can supply these facts. The phase loop waits for its tasks to finish, checks quality remediation, runs its checkpoint and keeps a failed checkpoint open or eventually unverified. A successful checkpoint is asserted into the kernel; `completePhase` then publishes the completed phase row. These are genuine safeguards: an arbitrary successful task alone does not establish the phase promotion used here. [Phase checkpoint gate][phase-gate], [checkpoint assertion][checkpoint], [phase completion][phase-completion].

The completed phase does not disappear merely because its context is compressed. The pager retracts that phase's `phase_context_atom` rows and publishes a compression summary and activation changes; it does not consume the phase, objective or passed-checkpoint facts supporting this promotion. The witness requires those supports to remain present when subsequent tasks finish; it does not rely on recovery after a restart or a refinement that removes the objective. [Pager retirement][pager].

Cortex ownership does not inherently make this a dead join: the three producer families are assigned to the campaign shard. Queries for unowned derived predicates use the derivation map and merge results, deduplicating identical tuples within one query. That deduplication prevents counting the same tuple multiple times merely because multiple shards returned it; it is not a durable acknowledgement across successive task completions. [Campaign ownership][ownership], [query routing and deduplication][query].

### Task completion polls that standing relation and manufactures a different record

The task execution path calls `executeTaskWithRollback`; on error it handles failure and returns before `applyLearnings`. On success it completes the task, emits its completion event, calls the adapter, emits progress and saves the campaign. Therefore the proposed witness deliberately requires genuinely successful later tasks. No fabricated success, unusual model output or concurrent task interleaving is necessary. [Successful task path][task-success].

The adapter has cancellation, query-error and empty-result exits. For a nonempty successful query, it optionally serializes and truncates the current task result, locks the campaign, and appends once per returned tuple. There is no comparison with earlier campaign learning records and no acknowledgement or removal of the source conclusion. Its mapping is:

| Source or current context | Stored field |
|---|---|
| Promotion argument 0, the fact kind | `Pattern` |
| Promotion argument 1, the fact value | Not stored |
| Constant `/success_pattern` | `Type` |
| Current task description and optional result text | `Fact` |
| Time this polling call appends the record | `AppliedAt` |

The current task context can be useful **observation context**. The defect is not retaining it; it is using it as the only fact text while omitting the actual promoted value, with no field separating observer from source. The name `AppliedAt` also records this append time, not an independently evidenced downstream application. [Adapter][adapter], [record type][record-type].

### The records reach real persistence and reporting surfaces

`Learnings` is a JSON field of the campaign. `saveCampaign` marshals the campaign, appends journal events and writes the snapshot atomically. The durable part of this finding requires those operations to succeed. Existing journaling and atomic snapshot machinery help preserve what was recorded; they do not reconstruct the missing promotion value. [Campaign field][campaign-field], [snapshot writer][snapshot].

`GetProgress` reports `len(o.campaign.Learnings)`. The TUI start path supplies the session kernel to the orchestrator, runs it, and feeds the active campaign into its campaign page on progress updates. That page counts the entries, lists their `Type`, and labels the total as acquired learnings. The CLI status command loads persisted campaign JSON and labels the same length as “Learnings applied.” These are actual reporting consumers, conditioned on delivery/opening the view or selecting that campaign as the latest readable record. [TUI construction][tui-build], [TUI run and update][tui-update], [progress][progress], [page][page], [CLI status][cli].

There is important counterevidence against a broader learning-effect claim. The intelligence gatherer's historical-pattern path loads a separate LearningStore. Its previous-campaign path reads these JSON files but extracts campaign ID, goal, task count, success rate and creation time—not `Campaign.Learnings`. This review did not establish reinjection of these entries into a task prompt or a learning database. [Historical learning source][historical], [previous-campaign projection][previous].

## Conditional normal-path witness

Use an admitted, continuously running two-phase campaign with no cancellation, healthy kernel operations and successful snapshot persistence. Track the `/phase_success` tuple below separately from other returned kinds:

1. Phase P has one objective type, such as `/modify`, and a working verification method. Its tasks genuinely succeed, its checkpoint passes, and its completed status is published.
2. The stock rule now supplies `promote_to_long_term(/phase_success, V)`. V denotes the rule's actual string conversion of the objective type; the witness does not depend on guessing a different string normalization.
3. Phase Q has two ordinary tasks, Q1 and Q2, which both genuinely succeed after P. Neither needs to generate a new promotion. P's objective and checkpoint supports remain in the kernel.
4. Q1's adapter call sees that tuple and appends a record with `Type=/success_pattern`, `Pattern=/phase_success`, Q1's description/result as `Fact`, and the current timestamp.
5. Q2's adapter call sees the same tuple and appends another record with Q2's context. Neither record preserves V.
6. A successful save retains both entries attributable to this same tuple. A subsequent relevant CLI status read or campaign-page refresh includes both in its applied/acquired total, alongside any entries for other returned tuples.

The stock corpus also has a distinct `/phase_pattern` producer for a completed phase without a currently failed task. For the clean P above, that rule can supply an additional tuple for the same objective value. It is not the `/phase_success` rule and must not be collapsed into it: when both tuples are returned for Q1 and Q2, the adapter appends four records, two per kind. The primary witness requires only that the identified `/phase_success` tuple is returned twice; it does not require the total query result to contain one row. [Separate zero-failure producer][phase-pattern].

This is a source prediction, not a runtime reproduction. More generally the appended count is **one record per returned tuple per successful adapter invocation**. The source does not warrant multiplying by all historical phases: equal type/value conclusions can collapse into the same relation tuple. Nor does it warrant claiming every campaign duplicates learning; a one-phase campaign may have no later successful task at which this phase-completion promotion is sampled.

A variant with two distinct objective values in the returned relation is a useful precision control: both records receive the same stored `Pattern=/phase_success`, and both values are omitted. This distinguishes type/value loss from repeated sampling of one tuple.

## Safeguards and limits to preserve

- **Task and phase gates remain authoritative.** Failed task execution returns before this adapter. Failed phase checkpoints do not become the completed verified phase used by the witness. Do not weaken those gates to exercise learning.
- **Cancellation and query failures can suppress recording.** The witness needs the adapter's admission conditions and a successful query. Kernel assertion failures are logged; absent supporting facts can prevent the promotion.
- **The campaign mutex protects the append.** This is not a data-race claim. Sequential Q1 and Q2 suffice.
- **Kernel set semantics and query deduplication are useful.** They bound one query's returned tuples, but do not turn a later read into a new evidence event.
- **Phase completion itself is idempotent.** `completePhase` refuses to count an already-completed phase again. Repeated learning entries arise from later task callbacks, not from defeating that guard.
- **Durability is conditional.** An in-memory record is not proof of successful disk persistence. This proposal should compose with storage-hold work rather than masking a failed save.
- **Reporting is the proven consumer.** No permission bypass, automatic prompt mutation, completion-gate bypass, model-quality degradation or measured monetary cost is claimed. A future consumer must not inherit stronger semantics from the name “learning.”

## Proposed evidence and receipt contract

### Preserve meaning before adding new behavior

Decode the declared relation into explicitly typed kind and value fields. Keep the value's original content; do not lower-case, trim, summarize or replace it with the current task's output for identity purposes. Unknown kinds should remain representable without silently becoming success patterns. An invalid shape should receive a diagnostic disposition rather than corrupting a valid record.

Store the current task ID and optional bounded result reference as **observed during**, separately from the source that established the promotion. The present two-argument relation has already discarded source phase and campaign IDs; the adapter cannot honestly recover them from the task it happened to be executing.

If maintainers want only a lightweight campaign activity log, an acceptable minimal alternative is to retain each observation, faithfully include its kind/value, and label counts as **promotion observations**, not distinct learnings or applications. This preserves deliberate repetition without overstating what it means.

### Choose an explicit identity and lifecycle

For a small truthful receipt layer, deduplicate captured propositions by campaign plus canonical typed kind/value, with a schema version. Additional sightings can update first/last-observed metadata or a separately named observation count. This answers “which propositions were observed,” not “how many independent successful phases support them.”

For stronger causal evidence, add a source-owned relation or receipt that retains campaign ID, phase/task identity, evidence version and the kind/value. Keep unique proposition identity separate from independent supporting-event identity. Two different phases supporting the same value should be two support events for one proposition, not either two new propositions or an indistinguishable duplicate.

Do not retract shared policy conclusions merely to mark them consumed: they may remain valid and have other readers. A per-consumer acknowledgement/index is safer. Conversely, if source support is withdrawn or revised, retain historical evidence with an explicit status; do not present it as a still-current claim.

The W3C PROV model separates producing an entity from later using it, and cautions that co-occurring use and production do not by themselves establish derivation. That distinction is useful here as a design reference, not a claim that CodeNERD must implement PROV or violates a required standard. [PROV model][prov].

### Make capture and application different receipts

Use statuses such as observed, captured durably, application pending, applied, rejected or superseded only where their corresponding evidence exists. An actual application would need a named sink, operation identity and success receipt. Do not add an automatic LearningStore write, embedding request or JIT injection solely to justify the existing UI label.

Persist the capture identity with the record so resume/replay does not duplicate it. If storage fails, retain an explicit pending state and use the existing recovery policy; do not mark a nonexistent durable receipt as acknowledged. Avoid retrying completed user work just to retry its recording.

Legacy records cannot generally recover the discarded value. Preserve them as legacy observation records with unknown value/provenance, or migrate only where independent source evidence proves the mapping. Never invent a value from the old `Pattern` field or treat every old `AppliedAt` as an application receipt.

## Costs and implementation choices

| Choice | Benefit | Cost and failure risk |
|---|---|---|
| Faithful observation log with truthful labels | Smallest change; retains every historical sighting and current-task context | Still grows per sample; does not establish distinct acquisition or causal source |
| Typed proposition receipts plus persisted deduplication | Preserves values and makes unique-count semantics deterministic without new model calls | Additional schema/index and migration; content deduplication alone cannot count independent support events |
| Source-owned evidence receipts plus proposition aggregation | Supports accurate attribution, replay, revision and independent evidence counts | More policy/adapter fields, lifecycle handling and compatibility work; must avoid changing existing promotion consumers |
| Automatic durable learning or prompt application | Could create a new capability if separately desired | New authority, data-retention, retrieval, token and provider costs; unjustified as a repair for this reporting issue alone |

Current local work is a query plus appends. For T successful sampling calls returning P_i tuples, the recorder appends the sum of P_i records; each repeats that task's description and optional bounded result text. Those records are included in later whole-campaign snapshots. This is a structural storage and serialization cost, not a measured latency or memory result. A unique-proposition index requires space proportional to retained unique propositions; preserving every support event or observation still requires an explicit retention policy.

A minimal repair needs no additional LLM request, embedding, network transfer or training. The main engineering costs are schema migration, exact identity encoding, replay-safe persistence, concurrency handling, display compatibility and tests. If source-owned evidence is selected, budget for policy ownership and truth-maintenance review as well. Do not introduce a timer or arbitrary evidence cap that silently discards useful support; make compaction preserve counts and provenance.

## Fair evaluation not yet run

Compare the current adapter, the faithful-observation alternative, and the typed/source-owned receipt design under the same policy, task/checkpoint outcomes, filesystem reliability and query sequence. Start with deterministic production-path fixtures and a real kernel; use scripted task/model seams only where needed to admit a valid plan. No live provider is required to establish this projection.

| Case | Required observation |
|---|---|
| Verified P followed by successful Q1 and Q2 | Current code records the same tuple twice and omits V; repaired unique-count mode retains V once, with two sightings if requested |
| Two objective values with the same promotion kind | Distinct values survive; neither is merged merely because argument 0 matches |
| Independent phases supporting the same value | Proposition count and support-event count remain distinct according to the declared contract |
| No promotion, query error or cancellation | No invented receipt or applied count; diagnostics reflect the actual reason |
| Failed task or failed phase checkpoint | Existing execution and verification gates stay unchanged; no manufactured positive source |
| Duplicate tuple returned from different shards | Existing per-query deduplication remains; cross-call identity is independently checked |
| Repeated callback, resume and replay | Durable identity prevents duplicate capture without suppressing a genuinely new support event |
| Source withdrawal or revised evidence | Historical records remain inspectable but are not mislabeled current; unrelated consumers retain their facts |
| Snapshot or journal failure | No false durable acknowledgement; recovery does not rerun already-completed user work |
| Legacy campaign JSON | Reader compatibility remains; missing values stay explicitly unknown rather than guessed |
| CLI and TUI refresh | Counts, kinds, values and wording agree with durable receipt semantics; observed is never displayed as applied without a sink receipt |
| Future actual application sink | Only its confirmed success increments applied count; failed or absent sinks do not |

The existing golden campaign fixture loads the autopoiesis policy, but its input contains pending phases/tasks and its golden output concerns scheduling/delegation. It does not exercise the completed-phase promotion followed by successive Go adapter calls. Preserve that scheduling coverage and add the missing cross-boundary fixture rather than claiming the existing suite was run or failed. [Golden scenario][golden-test], [fixture][golden-input], [expected facts][golden-output].

Measure exact value retention, unique proposition count, support-event count, observation count, attribution accuracy, JSON size, snapshot bytes/time, kernel query time, lock contention and provider-call count. The repair should not improve a count by deleting legitimate evidence, nor improve storage by merging values that only look similar. Acceptance requires the end-to-end receipt contract and unchanged task/phase safety—not merely a lower number in the UI.

## Source links

[main]: https://github.com/theRebelliousNerd/codenerd/commit/6cbe3962fbdb24a1639b26e71f8096e4f4330048
[main-delta]: https://github.com/theRebelliousNerd/codenerd/compare/388205138ba6d47c1cff0c5e97c896dfa642a391...6cbe3962fbdb24a1639b26e71f8096e4f4330048
[new-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/architecture/campaign/INTERNALS.md#L152-L178
[runtime-guard]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/acceptance_runtime.go#L19-L44
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/agents.md#L1-L23
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/agents.md
[research]: https://github.com/theRebelliousNerd/codenerd/compare/388205138ba6d47c1cff0c5e97c896dfa642a391...90c84648725aa263c8a1ba2458390b572402e4de
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/schemas_analysis.mg#L93-L99
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_tasks.go#L699-L748
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/campaign_autopoiesis.mg#L12-L20
[objectives]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/types.go#L617-L645
[phase-pattern]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/campaign_rules.mg#L393-L405
[loading]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/constitution_source.go#L75-L113
[phase-gate]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_tasks.go#L152-L247
[checkpoint]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_utils.go#L48-L117
[phase-completion]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_phases.go#L517-L607
[pager]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/context_pager.go#L287-L312
[ownership]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/shards/registration.go#L415-L459
[query]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/cortex_kernel.go#L634-L735
[task-success]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_tasks.go#L450-L477
[record-type]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/types.go#L455-L460
[campaign-field]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/types.go#L239-L244
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_lifecycle.go#L62-L110
[tui-build]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/campaign_planning.go#L79-L122
[tui-update]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/chat/model_update.go#L325-L373
[progress]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_control.go#L149-L164
[page]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/ui/campaign_page.go#L476-L523
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/cmd_campaign.go#L530-L592
[historical]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/intelligence_gathering_methods.go#L198-L228
[previous]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/intelligence_gathering_methods.go#L438-L509
[generated-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3ef34bc061b497b9cfc471f4cc46f5df595b299d/Docs/audits/generated-learning-event-domain.md
[storage-audit]: https://github.com/theRebelliousNerd/codenerd/blob/90c84648725aa263c8a1ba2458390b572402e4de/Docs/audits/campaign-completion-storage-hold.md
[inventory]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/2026-08-11-self-audit-campaign/decl_inventory.md
[qa]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/.quality_assurance/qa_journal_2026-08-01.txt#L156-L161
[golden-test]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/logic_test.go#L32-L39
[golden-input]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/testdata/campaign.edb
[golden-output]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/testdata/campaign.golden
[prov]: https://www.w3.org/TR/2013/REC-prov-dm-20130430/#section-entity-activity


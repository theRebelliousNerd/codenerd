---
doc-class: shipped-with-future
subsystem: orient
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472
supersedes: []
---

# Timeline, lineage and informative attention

Orientation serves deterministic context selection: Go measures a foreign
repository and Mangle decides what deserves a full read. Target directory and
document names never determine quality. Recency alone does not establish it.

## C3 capability contract

This specifies the C3 signal-quality repair. Source observations below refer to
uncompiled working-tree additions; evaluation and tests remain unverified in
this author-only lane.

1. **Partial similarity survives.** Skip whitespace-only chunks. Documents without
   embeddable text emit `doc_embedding_omitted(Path, /empty, Detail)`. Pool chunks
   into `EmbedBatch` calls bounded by `orient.embedding_batch_size`, with
   `orient.embedding_concurrency` workers. Retry requests up to
   `orient.embedding_retry_attempts` total attempts, retaining the embedding
   engine's existing per-request timeout. Split exhausted batches to isolate
   failures; retain every complete successful document vector. Facts, stderr and
   reports name exclusions and reasons. Never cache a partial centroid.
2. **Prevalence reduces weight.** Count distinct documents for each read or vision
   reason. Its rarity factor is the complement of its permille share, bounded by
   `orient.reason_rarity_floor_permille`. Multiply configured base weights by
   this factor. Equal-weight reasons contribute independently.
3. **One slot per unit.** Equal normalized body digests and similarities at or
   above `orient.near_duplicate_permille` connect duplicates. Cohorts connect
   their members too. Policy elects a representative, transfers the unit's reasons
   to it, and lists suppressed members explicitly. Cohort representatives use
   internal inbound links, then centrality and a deterministic ordinal.
4. **Origin requires evidence.** Opening-era/span births are `/origin` only with
   descendants, link or embedding centrality, or repository/subtree instruction
   roots. Otherwise they are `/early`. Shallow history derives no generations.
5. **Cohorts are small and cohesive.** Group births in configured short UTC day
   windows anchored to actual births. Overlapping windows prefer larger acts,
   then earlier starts. Qualified directory acts take priority over broad
   similarity connections. Shared subtrees, reciprocal links or similarity provide
   cohesion. Require `orient.cohort_min_docs` and cap connected-group share at
   `orient.cohort_share_ceiling_permille`. Distinct `repo_file_day` measurements
   provide active days; aggregate file commit touches measure work volume. A
   sparse per-file landing can therefore be a burst. Window-local components
   prevent birth chains spanning an unbounded interval. Embedding centrality
   requires the configured `orient.embedding_hub_top_k_multiple` of the
   per-document top-k allowance, so ordinary neighbours are not all hubs.
6. **Lulls are relative.** Derive median monthly commits including empty calendar
   months. Below either `orient.era_lull_commits` or the median multiplied by
   `orient.era_lull_median_permille` is a lull. Scaled integer comparison avoids
   rounding away a small relative threshold.

Every tunable has OrientConfig Default, WithDefaults, Check and Params coverage.
Unit conversions and ordering encodings are algorithmic constants; thresholds and
base weights are configuration.

## Measurement and recovery boundaries

The measurement contract consumes committer timestamps and distinct days
without another git pass; the source witness is recorded below. Measure normalized digests and directory ancestry
before embedding. Preserve the visible similarity top-k/floor boundary and
content/model/chunk-size cache identity. Exclusion affects only a failed document.
Cache write failure retains the vector; parent cancellation returns an error.
Report partial similarity prominently and retain every excluded path.

## Acceptance exits (authored, not executed)

Keep the C3 gap open until a verification lane runs
`go test -count=1 ./internal/orient/... ./internal/config/...`,
`go test -count=1 ./internal/core/defaults/...` and
`go build -tags sqlite_vec ./...` on stable source. Required witnesses:

- Whitespace chunks are never embedded; ordinary similarity survives exclusions.
  Cover retries, isolation, batch/concurrency bounds, cancellation and reporting.
- A 90-percent-shared reason loses to a rare structural reason. Transitive
  duplicate components consume one slot.
- A twelve-document one-day subtree with one touch per document forms a burst
  cohort and elects the internally linked member; list all others. Reject
  repository-wide imports and temporally unrelated groups.
- Early peripheral documents stay early; descendants, hubs and instruction roots
  qualify origin. Cover relative lulls, even-sized medians and zero activity.

## Implementation evidence

The C3 working tree above `4dded472` contains the following implementation.
These are source witnesses and authored regression witnesses, not runtime passes.

| Capability | Source witness | Authored regression witness |
|---|---|---|
| Batched, isolated embedding failures | `embedDocs` at `internal/orient/docs.go:443`; `embedBatch` at `internal/orient/docs.go:576`; `validVector` at `internal/orient/docs.go:620` | `TestCollectDocs_WhitespaceChunksAndPartialBatch` at `internal/orient/docs_test.go:290` |
| Distinct reason prevalence and independent contributions | `reason_rarity/2` at `internal/orient/timeline.mg:625`; `read_contribution/3` at `internal/orient/timeline.mg:663` | `TestPolicy_RareReasonsOutrankNinetyPercentSharedReasons` at `internal/orient/policy_test.go:620`; `TestInspect_RareInstructionBeatsSharedBurstInFixtureRepository` at `internal/orient/docs_test.go:555` |
| Bounded cohesive acts with one representative | `doc_birth_window/2` at `internal/orient/timeline.mg:253`; `cohort_burst/1` at `internal/orient/timeline.mg:342`; `cohort_rep/2` at `internal/orient/timeline.mg:377` | `TestInspect_SparseOneDayCohortElectsLinkedMember` at `internal/orient/docs_test.go:475`; `TestPolicy_CohortSpansCalendarBoundaryWithoutChainingWindows` at `internal/orient/policy_test.go:764`; `TestPolicy_DirectoryCohortSurvivesBroadSimilarity` at `internal/orient/policy_test.go:818` |
| Qualified origin and early generations | `doc_origin_evidence/1` at `internal/orient/lineage.mg:203` | `TestPolicy_OriginsRequireStructuralWitnesses` at `internal/orient/policy_test.go:697`; `TestInspect_EarlyPeripheralDocumentsAreNotOrigins` at `internal/orient/docs_test.go:586` |
| Relative monthly lulls | `month_median_twice/1` at `internal/orient/timeline.mg:57` | `TestInspect_QuietMonthIsRelativeToFixtureHistory` at `internal/orient/docs_test.go:615`; `TestPolicy_LullsUseMonthlyMedianAndAbsoluteFloor` at `internal/orient/policy_test.go:733` |
| Duplicate read units and explicit members | `read_slot_rep/1` at `internal/orient/lineage.mg:254`; `orient_read_member/2` at `internal/orient/lineage.mg:263` | `TestPolicy_DuplicateComponentsShareOneSlot` at `internal/orient/policy_test.go:658` |
| Prominent, complete diagnostics | `buildReport` at `internal/orient/report.go:172`; `Report.Text` at `internal/orient/report.go:452` | `TestCollectDocs_WhitespaceChunksAndPartialBatch` at `internal/orient/docs_test.go:290` |
| Config contract | `DefaultOrientConfig` at `internal/config/orient.go:132`; `WithDefaults` at `internal/config/orient.go:182`; `Check` at `internal/config/orient.go:317`; `Params` at `internal/config/orient.go:400` | `TestOrientConfig_C3ValuesRoundTripAndReachPolicy` at `internal/config/orient_test.go:106`; `TestOrientConfig_C3InvalidValues` at `internal/config/orient_test.go:151` |

The sensor reuses `ScanHistory` (`internal/orient/history.go:33`) and its existing
daily witnesses; C3 adds no git pass. Policy remains in the existing embedded
glob (`internal/orient/engine.go:22`, `policySource` at
`internal/orient/engine.go:111`). C3 adds new declarations beginning with
`doc_body_digest/2` at `internal/orient/schema.mg:185`; shared declarations keep
their shapes.

This work is tracked as [`GAP-ORIENT-24`](03-GAP-ANALYSIS.md). Its package tests
pass; the exit that remains is the large-repository census.

Projection must carry the new diagnostic/member facts, and refresh without an
embedder must preserve or remeasure unchanged embedding diagnostics. C3 cannot
claim those integration gates from source authoring. Ollama's existing batch
interface also remains the backend owner's responsibility; C3 bounds interface
calls and does not claim a measured native GPU batch throughput.

# First-recall legacy repair records an attempt, not completion

## Decision and evidence boundary

Separate automatic legacy-vector repair's **running, incomplete, and complete** states. Preserve the existing identity gate and explicit resumable migration. This is a scoped recovery and request-budget refinement, not a reopened incompatible-vector-ranking finding.

**Source freeze:** CodeNERD main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, checked 2026-10-29. Evidence is static source inspection only: no tests, provider calls, inference, runtime reproduction, or repository modifications were performed. The outcomes below are source-derived, not observed incidents.

## 1. Relationship to the existing migration audit

The existing audit documents durable, resumable **explicit force migration**, with measured coverage certifying completion. Its as-built statement that unstamped vectors never rank now needs qualification: first-recall repair can generate fresh attributable vectors and make those rows eligible. Foreign-stamped vectors remain excluded until separately reembedded. [Prior audit](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/embedding-representation-and-migration.md#L39-L54).

The new automatic path has a different lifecycle. `maybeHealUnattributed` inserts the space into `healedSpaces` **before** attempting repair. No success check clears or changes that entry. Subsequent calls on the same store object and space return immediately, including after cancellation, SQL failure, partial provider refusal, or an empty initial inventory. [One-shot latch](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L345-L372).

Thus "attempted once" is the implemented contract—not "all legacy rows healed." Retry suppression is intentional in the source comments; this proposal challenges its recovery contract rather than claiming the latch is an accidental omission. An existing test explicitly expects provider-refused rows not to be retried automatically on a second recall; that policy prevents repeated provider work and must be considered in any refinement. The original audit's durable-migration design and cost cautions already anticipate interrupted work generally; the new contribution is their concrete application to this subsequently added automatic recall path. [Intentional no-retry test](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity_test.go#L821-L870).

## 2. Reachability and work scope

Semantic recall invokes repair synchronously after successfully embedding its query and before ranking. Path-filtered and metadata-filtered recalls invoke the same repair. Its inventory is **store-wide**, not restricted by the query's path, metadata filter, or result limit. This establishes work amplification relative to a narrow recall, not an established authorization violation. [Ordinary recall](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L462-L513), [filtered recall](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L583-L646), [inventory](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L193-L235).

The automatic inventory is narrowly defined: nonblank content, an existing non-NULL embedding, and NULL or empty `embedding_model`. It does not include foreign-stamped rows, blank content, or keyword-only rows whose embedding is NULL.

## 3. Source-pinned success and failure outcomes

| Condition | Source-derived outcome | Qualification |
|---|---|---|
| Query embedding fails before repair | Recall returns an error before claiming the automatic-repair latch. | This failure does not consume the attempt. [Recall ordering](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L483-L505). |
| Repair returns vectors and conditional publication succeeds | Canonical rows receive newly computed vectors and the producing space; applicable index insertion is attempted. | No identity is guessed from old vector dimensions. Ranking still depends on index availability and ordinary read eligibility. [Publication](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L303-L342). |
| Context is canceled after the space is claimed | Repair can return with partial or zero progress; the claimed space remains in the in-memory set. | A later healthy recall on that object skips automatic retry. Do not claim the canceled caller necessarily receives successful recall. [Cancellation](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L223-L235), [latch/error handling](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L352-L372). |
| Individual provider items fail or produce empty results | Failed rows can remain unstamped while other rows publish; the helper can finish without an aggregate completeness error. | Failure is logged, not proof that all rows were repaired. [Provider handling](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L250-L302), [empty-result skip](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L309-L317). |
| SQL update fails | Repair returns an error after any earlier durable progress; the caller logs it and keeps the latch. | No rollback of all prior batches is promised. [Update error](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L318-L322). |
| Content or attribution changes during provider work | The conditional update affects no row; that result is not published. | This stale-publication defense must remain. [Conditional update](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L318-L325). |
| Index insertion fails after canonical update | The row may be stamped while index drift remains; a warning is emitted. | Do not equate the helper's healed count with either canonical coverage or index completeness. [Index path](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L327-L335). |
| Another recall arrives while repair runs | It sees the claimed space and does not join or wait for the first repair. | The latch is not a completion barrier. [Lock release before work](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L356-L366). |
| Initial inventory is empty; eligible legacy rows appear later | The first call still consumes the space's attempt, so later recalls on that object do not rescan. | This requires later introduction of qualifying unstamped rows; no routine producer incidence is claimed. [Empty inventory](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L202-L225). |

## 4. Counterevidence and recovery that already exists

No incompatible-vector resurrection is established. Failed legacy rows retain their missing stamps and remain outside semantic ranking. Foreign-stamped rows are never selected for automatic repair. Producing-engine identity is captured, and a changed attached engine does not lend its identity or index to old-engine bytes. [Identity contract](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L15-L33), [publication controls](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L303-L325).

The latch belongs to the store object, not a durable completion record; a newly opened object has a fresh latch. Explicit force migration is independent: it resumes incomplete work and certifies measured target coverage. Do not describe this gap as permanent data loss or absence of recovery. [In-memory fields](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_core.go#L76-L81), [durable migration](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_migration.go#L95-L168).

## 5. Proposed contract

Retain cheap repeated recalls, but name the state accurately. Track in-flight and incomplete attempts separately; expose remaining coverage and a bounded retry or maintenance route. For cancellation or transient infrastructure errors, consider an explicitly bounded retry with backoff or a user-initiated maintenance path. Preserve the deliberate non-retry policy for provider-refused items unless a separate policy explicitly authorizes it. Specify whether concurrent readers wait, proceed with disclosed partial coverage, or use lexical fallback. If durable automatic resumption is desired, reuse the existing migration mechanisms where appropriate rather than inventing a second incompatible state machine.

Do not expand automatic repair to foreign spaces, substitute providers, or authorize additional external spending merely because repair is incomplete. The configured provider and explicit maintenance scope remain authoritative. An automatic-repair policy should state its row/input/time budget before executing store-wide work on a narrow recall.

## 6. Cost accounting

The current first recall materializes the complete pending inventory, then embeds batches of 32. Peak inventory memory scales with pending content and metadata; provider work, request latency, and SQL/index writes scale with the legacy backlog—not the requested result count. Batch failure can additionally trigger per-item calls. [Inventory and batching](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L193-L302).

Report separately: inventory rows and bytes; provider requests and reported input units; local CPU/GPU time; first-recall and later-recall latency; successful canonical publications; index drift; remaining exclusions; retries and duplicate work; database/WAL bytes; operator recovery time. Bounded traversal would reduce peak inventory memory but adds cursor/checkpoint bookkeeping. Persistent retry state adds writes and retention obligations. Waiting readers trade stronger coverage for latency; partial serving avoids waiting but requires honest coverage disclosure.

No dollar saving or performance improvement is claimed. Monetary estimates require actual provider pricing and measured input units. Do not count a failed batch plus per-item fallback as one request, or treat successful canonical updates as proof of indexed availability. These cost principles extend the existing audit rather than replace it. [Existing cost requirements](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/embedding-representation-and-migration.md#L273-L292).

## 7. Comparative evaluation — proposed, not run

Compare three policies on the same non-sensitive corpus, configured engine, failure schedule, and retrieval tasks:

1. Current one-attempt-per-object/space automatic repair.
2. Existing explicit resumable migration, with automatic repair disabled as an evaluation control.
3. Proposed bounded automatic repair with explicit running/incomplete/complete state and retry policy.

Measure coverage recovered over time, first and subsequent recall latency, provider work, retry amplification, peak memory, index parity, retrieval quality, and operator intervention. Keep foreign-space exclusions identical across arms; do not manufacture a speedup by ranking unattributed or incompatible vectors.

Required deterministic cases: cancellation after query embedding; transient provider refusal followed by recovery; SQL failure; partial completion; concurrent recalls; empty pass followed by qualifying legacy insertion; close/reopen; engine swap during work; content replacement during work; canonical success with index failure; explicit force recovery; foreign-stamped, blank-content, and NULL-embedding controls. Exercise indexed and brute-force recall independently.

Success means remaining legacy coverage and retry state are accurately reported, transient failure can recover under the selected bounded policy, successful progress is retained, stale publication is rejected, and foreign-stamped rows never enter automatic repair. Failure means an attempt is represented as complete, retries are unbounded, narrow recall silently exceeds its declared work budget, or compatibility protections regress. Runtime and end-to-end benefit remain unverified until authorized evaluation establishes them.

**Exit condition:** incomplete automatic work remains safely excluded, visibly incomplete, and recoverable under a bounded policy; an attempt marker is never presented as a completion certificate.

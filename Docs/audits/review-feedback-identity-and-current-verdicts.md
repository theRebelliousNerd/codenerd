---
doc-class: deep-dive
subsystem: chat-review-feedback
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection
---

# Review feedback needs canonical finding identity and explicit judgment semantics

The September F10 repair made review feedback produce facts again. This focused addendum addresses what those facts identify and what the report counts. The present report labels timestamped feedback records as distinct “finding(s) judged,” while command handlers ignore an available canonical aggregate review ID. A repeated judgment can therefore increase reported finding coverage without another finding being judged.

Preserve feedback history, carry canonical review and finding identities, and explicitly choose whether current disposition means latest verdict or a contested state. Automatic supersession is a design proposal, not an existing policy promise. No training damage, unauthorized action, automatic retry, or universal review starvation is established here.

## Scope and the repair already present

[September F10][f10] documented a nil-provider no-op and feedback-loop starvation. The current manager gives an installed provider precedence and otherwise asserts feedback through the kernel, refreshing accuracy on success. This is a real repair. The [feedback fixtures][tests] cover producer behavior, distinct-location suspicion, four distinct findings, scoped refresh, nil-kernel handling, and provider precedence; they were inspected, not run.

This finding concerns `/accept-finding`, `/reject-finding`, and `/review-accuracy`, not `/learn` transcript pairing or candidate confirmation. It is separate from [manual learning evidence conservation][manual], [system execution outcome projection][outcome], and [document extraction coverage resumption][documents].

## The live path loses an available identity

The [router][router] reaches the [feedback handlers][handlers]. Both derive `review-<turn>-<seconds>` from a reviewer `lastShardResult`, or use `unknown` when the result is absent or is not a reviewer result. They ignore `ExtraData["review_id"]` and leave `lastShardResult` unchanged: sequential feedback retains this alias until another result or session transition replaces the context. Location parsing uses the last colon and an unchecked `fmt.Sscanf` result, without resolving an actual finding; stronger admission is a proposal, not current behavior.

In the concrete multi-shard route, [`/review` selects aggregation][reviewroute] when the registry has an agent. The [aggregator][aggregate] creates `review-<nanoseconds>`. [Successful completion][update] reaches `storeAggregatedReviewResult`; the [chat result][lifecycle] contains the findings and canonical ID in `ExtraData["review_id"]`, then becomes `lastShardResult`. The canonical and command-generated formats differ deterministically. The identity is available in memory, and [conditional Markdown export][export] includes it; the per-finding database persistence does not establish equivalent ID retention.

Important counterevidence: [`/review-accuracy`][reportcommand] computes the same synthetic alias. Its report is not universally empty. The defect is divergent correlation with the completed aggregate and misleading counts, not complete disconnection among these commands.

The [manager][manager] emits acceptance and rejection facts with integer-second timestamps. Its [counting function][counts] scans both predicates and increments for every matching review row; it neither groups by finding nor supersedes an opposite verdict. Exact repeated acceptance within one second may set-deduplicate, whereas the same acceptance in different seconds creates distinct tuples. Rejection reasons can also distinguish records. The [report][report] repeats these counts and presents their sum as “finding(s) judged,” with an accepted percentage. Its [scoped retract-and-replace of `review_accuracy`][refresh] is a useful safeguard: derived accuracy rows are refreshed, not endlessly appended.

## Two controlled source witnesses

These are source-derived scenarios, not executed reproductions. Assume an initialized, idle ChatView, a genuinely completed review, an attached kernel, successful assertions and queries, and no intervening replacement of `lastShardResult`.

| Completed review and commands | Present report | What the report actually counts |
| --- | --- | --- |
| One unique finding at `billing.go:12`; accept twice in different Unix seconds, then reject it | 3 judged, 2 accepted, 1 rejected, 67 percent accepted | Three feedback records about one finding |
| Two findings at `billing.go:12` and `billing.go:20`; reject both, then accept line 12 | 3 judged, 1 accepted, 2 rejected, 33 percent accepted | Three records about two findings, including opposite verdicts on line 12 |

The second witness needs no timing assumption: different locations and opposite predicates already distinguish its facts. Under a latest-verdict design, only line 20 remains rejected. Under a historical-evidence design, the two rejection records may intentionally remain relevant until explicit revalidation. Either design can preserve history; neither justifies presenting three feedback records as three distinct findings.

## Suspicion is related but not an automatic consequence

[Reviewer policy][policy] counts rejection records, but `multiple_rejections` specifically requires different files or lines. Repeating one location alone does not trigger that rule. In the second witness, both historical rejected locations remain present, so this reason still derives. Calling that retention wrong would require a policy decision about correction and revalidation; the source does not promise latest-verdict semantics.

The high-rejection-rate rule additionally requires `review_rejection_rate_high`. Static search found only its declaration, reader, and [starved-predicate baseline][starved], not a live producer. Inflated counts do not establish that this threshold fires. Human acceptance must not automatically erase independent structural and Jev-derived validation reasons.

[Validation and reason lookup][validation] use the supplied exact review ID. [Ordinary delegation][process] checks validation after review facts and Jev work, before subsequent user feedback. Repairing identity alone cannot make that finished check react retroactively or establish future-review learning. The misleading live report is sufficient evidence.

## Proposed identity and disposition contract

Keep immutable feedback events with explicit retention, plus a separate current-disposition projection. Carry a typed canonical review ID through completion, display, admission, facts, persistence, and reporting. Resolve a review-owned finding ID with source revision and diagnostic identity; preserve source anchors rather than fuzzy historical matching. The [aggregator currently deduplicates by file and line][dedup], keeping the highest severity. Preserve that policy; same-line ambiguity below is a future identity-design control, not a second demonstrated defect.

Reject or mark unresolved feedback when identity cannot be established. Check owner and generation so late commands cannot silently target a newer review or session. Valid coordinates do not establish finding membership.

Choose disposition semantics. With latest-verdict semantics, repetition is idempotent and replacement references the superseded event. With contested-state semantics, opposing judgments retain visible conflict until resolution. Neither destroys original evidence. Go performs IO and admission; Mangle derives declared state. Add no model, judge, or embedding calls for deterministic bookkeeping.

Show events separately from distinct findings, current verdicts, and coverage. The accepted fraction among judged findings is not objective accuracy or total review quality. Apply declared semantics to feedback-based suspicion while retaining independent evidence.

Return receipts such as admitted, repeated, superseded, unresolved, or failed. Current manager methods return no result and handlers print success unconditionally; [nil kernels and assertion failures][assertion] need explicit handling. Preserve provider precedence and compatibility without double-writing through provider and fallback.

Stage identity repair separately from disposition projection. Version the schema and define old-alias handling before changing keys. Retain unresolved historical aliases; never blindly merge them by similar location or time.

## Alternatives and implementation cost

Relabeling the existing report as historical feedback events is a small, honest correction, but leaves distinct-finding status and canonical correlation unresolved. Deduplicating present tuples is insufficient: timestamps, opposite predicates, and multiple diagnostics per coordinate still require identity and semantics. Canonical IDs plus separate event and disposition views address both issues without another critic model.

Current updates and reports scan retained feedback, approximately O(F) in feedback facts, and refresh scoped accuracy rows. An indexed current view could offer O(1) or O(log F) lookup while retaining O(E) event history plus O(distinct findings) disposition storage. These are design estimates, not measured savings. Costs include API and schema migration, source fingerprints, stable ordering, transactional failure handling, retention, alias reconciliation, and occasional user disambiguation. No latency, dollar, or model-behavior improvement is claimed before measurement.

## Fair evaluation before implementation claims

The following evaluation is proposed and has not been run. Compare baseline, relabel-only, and full identity-plus-projection variants on identical fixtures, beginning with an actual idle `/review` registry completion, then feedback commands and the report rather than only direct manager calls.

| Fixture | Required observation |
| --- | --- |
| One finding repeated within and across seconds; opposite verdicts within and across seconds | Separate event counts from unique findings; declared disposition is time-boundary independent |
| Two locations followed by reversal; one genuine acceptance and one genuine rejection | Correct distinct coverage and policy-specific suspicion; preserve the positive control |
| Future identity control with two diagnostics on one line | Preserve declared aggregate deduplication; disambiguate any separately exposed identities |
| Aggregate in-memory and exported IDs; separate single-review path | Correlate with the actual completed review in each supported route |
| Intervening review, nonreview result, or new session | Explicit unresolved or stale-context receipt, not `unknown` aggregation |
| Nil kernel and assertion, query, or retract failures | Honest failure receipt and last known coherent projection |
| Provider override; independent Jev suspicion; old aliases | No double-write, no erased independent evidence, no fabricated migration |

Measure event and distinct-finding counts, current-verdict consistency, correlation, retained evidence, latency, allocations, fact counts, retention bytes, and request counts. Expect zero additional model requests. A successful bookkeeping test is not evidence of improved reviewer judgment.

## Verification boundary

The October 29 static snapshot pins main at `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, resolved at 18:09 UTC and again unchanged at 18:15 UTC. Complete-tree comparisons covered code lanes `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and QA `51cfd42649552866d4aa54c53b68acf5740eb09b`. Feedback/report handlers, lifecycle, aggregator, and manager were unchanged; reviewer Section 11 and `handleReviewCommand` were unchanged despite other file differences. Research context was checked at `0fa6fd9f52cc6338445a498895a2052699bcc3b9`. No tests, runtime reproduction, or private-content inspection were performed.

[f10]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/2026-09-09-system-hardening/00-FINDINGS.md#L233-L256
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/review_feedback_test.go
[router]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands.go#L229-L234
[handlers]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_misc.go#L179-L276
[reviewroute]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers.go#L961-L1009
[aggregate]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/review_aggregator.go#L639-L680
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_lifecycle.go#L300-L335
[reportcommand]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_evolution.go#L241-L257
[manager]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager.go#L485-L525
[counts]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager.go#L542-L561
[report]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager.go#L608-L633
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager.go#L575-L601
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager.go#L424-L470
[assertion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager.go#L527-L540
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/reviewer.mg#L327-L379
[update]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L213-L235
[export]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/review_aggregator.go#L321-L377
[starved]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/testdata/starved_predicates.txt#L58
[process]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L674-L711
[dedup]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/review_aggregator.go#L837-L873
[manual]: https://github.com/theRebelliousNerd/codenerd/blob/0fa6fd9f52cc6338445a498895a2052699bcc3b9/Docs/audits/manual-learning-evidence-conservation.md
[outcome]: https://github.com/theRebelliousNerd/codenerd/blob/0fa6fd9f52cc6338445a498895a2052699bcc3b9/Docs/audits/system-execution-outcome-projection.md
[documents]: https://github.com/theRebelliousNerd/codenerd/blob/0fa6fd9f52cc6338445a498895a2052699bcc3b9/Docs/audits/document-extraction-coverage-resumption.md

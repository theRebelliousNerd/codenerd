# Multi-shard review must admit the severity vocabulary it asks the reviewer to use

## Finding and decision

A single finding in the documented reviewer format can become **“No findings.”** at the multi-shard review boundary. At main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), the reviewer identity atom specifies `CRITICAL`, `HIGH`, `MEDIUM`, and `LOW`. The aggregate parser instead searches each line for the substrings `CRITICAL`, `ERROR`, `WARNING`, and `INFO`. A canonical `HIGH` finding without an incidental legacy substring produces no parsed record. The per-reviewer report then explicitly says “No findings.”

This is a focused continuation of the [finding-location audit](https://github.com/theRebelliousNerd/codenerd/blob/58c84997ba39254e22c590f6cb3bab43a58509c1/Docs/audits/review-finding-location-deduplication.md), not a new general parser design. That audit acknowledged the narrower vocabulary, deliberately used `CRITICAL` to isolate location collapse, and already proposed a shared parser with severity-consumer parity. This continuation establishes the independent admission failure and its false-empty display: **one finding is sufficient, before deduplication, and restoring coordinates alone does not repair it.**

The recommended change is to finish that existing shared-contract repair across the actual aggregate route. Preserve canonical severity, legacy compatibility, reviewer provenance, location uncertainty, and report evidence. Do not turn a parser's empty projection into evidence that the worker reported no issues.

This report is based on static source inspection on October 30, 2026. Main, scoped guidance, active branches, and audit inventories through `research-main-20261030-1316` were reviewed. The witness was independently source-challenged, not executed. No application execution, tests, inference, benchmarks, or repository writes were performed. Actual model output frequency, missed defects, repair outcomes, and efficiency changes remain unmeasured.

## The contract already exists, and works on another path

The mandatory reviewer constraints atom requires each finding on one line, with a bracketed severity, workspace-relative path, optional line number, and explanation. It explicitly lists the four canonical severities and describes their meanings. The witness below follows that contract rather than relying on malformed output. [Reviewer atom](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/atoms/identity/reviewer.yaml#L24-L60).

The ordinary chat result parser has already been repaired: `extractFindings` recognizes that shape, extracts location and message, and normalizes canonical and legacy severity aliases onto `critical/high/medium/low`. It also retains severity-marked text when the location cannot be parsed. The ordinary storage path calls this parser. Existing fixtures explicitly require all four canonical severities and legacy aliases to survive. These are real positive controls, not missing system-wide capabilities. [Ordinary parser](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L42-L129), [ordinary storage](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L264-L289), [canonical and compatibility fixtures](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/findings_test.go#L66-L100).

The shared observation layer independently recognizes the same canonical ladder, and Mangle's delegation ranking assigns ranks to `critical/high/medium/low`, not the aggregate's log-level vocabulary. Reuse and align these owners rather than introduce another unrelated mapping. Parsing a reported finding does not establish that it is true or authorize a repair. [Observation parsing](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/observation/subagent.go#L824-L890), [policy severity ranks](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/delegation.mg#L648-L656).

## Reachable production boundary

### 1. A normal review can take the aggregate route

`handleReviewCommand` selects multi-shard review when the loaded registry exists and contains at least one agent; otherwise it uses the single-reviewer fallback. A registry agent need not match the requested files for this branch to be taken. The aggregate always dispatches the generic reviewer, plus any matching specialists and enabled Nemesis review. [Command branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L1028-L1070), [generic and optional workers](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L512-L579).

The generic call uses `spawnTask("reviewer", ...)`, which routes ordinary personas to `TaskExecutor.Execute`. The kernel maps reviewer to `/review`. The JIT executor's string API runs `RunUntilVerified` before returning the observed output, so the aggregate is not necessarily consuming the first model response or an arbitrary intermediate draft. Preserve that lifecycle safeguard. This finding is conditional on the final eligible worker return containing the stated finding; it does not predict that a live model always returns it. [Chat dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L35-L80), [persona mapping](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/delegation.mg#L517-L521), [string API lifecycle](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L317-L337).

### 2. The aggregate reparses with a different vocabulary

The worker wrapper returns its output on nil error. `aggregateReviewResults` skips error-bearing returns, invokes `parseShardOutput` on the remaining strings, stores the resulting slices in `FindingsByShard`, and increments `TotalFindings` by their lengths. Only then does deduplication run. [Worker collection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L469-L485), [aggregate producer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L638-L677).

`parseShardOutput` does not call the repaired parser. It uppercases each trimmed line and searches the entire line for one of four legacy words. `HIGH`, `MEDIUM`, and `LOW` are not in that list. A matching word elsewhere in the path or message can incidentally make a line survive, possibly with the severity of that word rather than the bracketed marker. The clean witness excludes that complication. [Exact admission loop](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L264-L287).

### 3. Empty parsing becomes an affirmative empty report

For an empty slice, `formatShardSection` emits “No findings.” The aggregate display calls it for each `FindingsByShard` entry. The optional narrative input receives `Total Findings: 0` and `None` in its detailed findings section. Its eventual prose is model-dependent; the deterministic report section and supplied narrative evidence are not. [Empty section](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L359-L380), [display consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L1123-L1173), [narrative projection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L715-L759).

Conditional persistence stores the parsed per-shard findings and count. The aggregate type does not carry the original worker responses. Later, `storeAggregatedReviewResult` saves `RawOutput: rendered`: that is the rendered aggregate, **not the source worker response**, and its structured findings come from the already-empty deduplicated list. Other execution logs or independent observations may retain original evidence; this audit does not claim global destruction. [Aggregate representation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L395-L421), [persistence](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L603-L630), [continuation storage](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L299-L337), [executor logging boundary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L446-L456).

## Single-finding source witness

Consider an idle, initialized TUI reviewing the admitted file `src/cache.py`, with a nonempty registry but no matching specialist. Use no root `go.mod` and unset `NEMESIS_REVIEW`, so the independent Nemesis route is not selected. Keep enhancement mode off. These conditions isolate the generic reviewer; they are not required for the parser mismatch itself. [Nemesis gating](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L1182-L1202).

Assume the final generic worker return has nil error and consists of this public synthetic example, not a claim about an actual repository defect:

```text
- [HIGH] src/cache.py:12: Shared state is mutated without a lock; guard it with a mutex.
```

No `CRITICAL`, `ERROR`, `WARNING`, or `INFO` substring occurs in that line. Therefore:

| Boundary | Source-derived result |
| --- | --- |
| Ordinary repaired `extractFindings`, as a comparison | One high-severity finding with the stated path and line |
| Aggregate `parseShardOutput` | Zero records |
| Aggregate count and deduplicated list | Zero and empty; no collision is needed |
| Generic reviewer's displayed section | `No findings.` |
| Narrative evidence | `Total Findings: 0` and detailed findings `None` |
| Aggregate-backed continuation | Empty structured findings; raw field contains the rendered aggregate |

All workers in this isolated example returned without error, so the existing error-only incompleteness check does not mark the review incomplete. A `Complete` header is an execution-status control, not proof of a defect-free workspace; the independently incorrect assertion is “No findings.” [Failure disposition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L583-L593), [header](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L349-L356).

The consequence is conditional underreporting and loss of available structured follow-up evidence. The fix/refactor task formatter consumes prior reviewer findings, so an empty slice supplies none through that route. The user, model, another specialist, Nemesis, source tools, history, or a fresh review may recover the issue. No deterministic missed repair, unauthorized write, or demonstrated task failure is established. [Follow-up consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L112-L173).

## Repair contract, safeguards, and costs

1. **One admitted finding contract.** Share a typed parser/decoder across ordinary, aggregate, and observation paths. Preserve canonical severity and explicit legacy aliases; retain original marker/text and reviewer source. Unknown or malformed evidence needs an unresolved disposition, not silent proof of absence. Prefer existing parsing owners over another prompt or model call.
2. **Complete consumer parity.** Updating only recognition is insufficient. Aggregate narrative sorting, deduplication ranking, summaries, and cross-shard insights still name legacy log levels. Align them with the canonical ladder and policy, while preserving legitimate highest-severity duplicate handling. This is the consumer migration already called for by the location audit. [Rank consumers](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L757-L842), [summary and insights](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L845-L935).
3. **Separate reported-none from parsed-none.** Preserve an owned, bounded or redeemable worker-response reference and parsing disposition through aggregation. A human-readable response with unavailable structured extraction must remain available as unstructured evidence, with honest presentation. Do not classify every sentence as a finding to avoid false emptiness; plain clean-review prose is a necessary negative control.
4. **Retain independent controls.** Preserve worker failure reporting, `RunUntilVerified`, narrative timeout/fallback, reviewer attribution, optional persistence, workspace membership, and normal tool/permission/verification gates. An admitted finding remains a worker claim, not verified truth or execution authority. The existing complete-list narrator projection must not regain an arbitrary finding cap.

Costs are compatibility and schema decisions, shared package ownership, consumer migration, provenance/retention work, and joined-path tests. Retaining actual findings can increase report size, narrator tokens, and legitimate follow-up work. Persisting redeemable raw evidence requires lifecycle and storage accounting. A shared deterministic parser needs no extra LLM judgment, but no net latency or cost saving is established. Relabeling an empty section as “No parsed findings” is a useful small honesty patch, not a substitute for admitting the documented format.

## Fair evaluation, not run

Begin with normal `/review` entry, a real populated registry, the ordinary JIT/world-model/CodeDOM configuration, and captured final worker returns. Capture aggregate state, narrator input, rendered report, conditional export/persistence, and subsequent contextual fix/refactor task. A deterministic worker-return fixture isolates the parser defect; it must not be presented as measured live-model prevalence or end-to-end correctness.

Cover each canonical severity with a single well-formed finding, then multiple findings and the earlier distinct-location witness. Include legacy aliases; optional line numbers; malformed location; unknown severity; clean prose; and incidental legacy words in otherwise canonical messages and paths. Confirm that the bracketed marker, not unrelated message text, determines severity. Check true same-location duplicates, error-bearing workers, mixed-worker results, independent Nemesis findings, enhancement text, disabled narrative generation, and narrator failure. Preserve source attribution and ordinary-parser behavior.

Existing canonical-vocabulary fixtures call `extractFindings`, not the aggregate producer. The aggregate source-attribution fixture calls the real aggregate parser with one `CRITICAL` finding; it is a useful positive control but cannot establish admission of `HIGH`, `MEDIUM`, or `LOW`. Retain both and add the joined production route. [Canonical fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/findings_test.go#L66-L100), [aggregate attribution fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/wizard_test.go#L713-L745).

Measure admitted and unresolved records, severity/location/source fidelity, false-empty and false-positive reports, downstream evidence availability, duplicate-policy behavior, retention bytes, narrator tokens, provider requests, total wall time, repair turns, and cost per independently verified successful task. Hold task scope, authority, models, and verification obligations fixed. Less output achieved by discarding findings is not an efficiency improvement; more findings is not itself better review accuracy.

**Acceptance:** the documented one-line `HIGH` finding survives the aggregate route with its correct semantics, cannot produce an unsupported “No findings.” section, and remains available to the intended downstream consumers without weakening evidence or authority controls.

## Verification boundary

Main remained `388205138` when rechecked for this pass. The `parseShardOutput` function body was also identical on the observed three lane heads (`a53cfc30`, `80af1863`, `aa7988de`) and two QA heads (`51cfd426`, `767a3291`). This is a scoped parser comparison, not a claim of whole-branch equivalence. Prior inventories through research 1316 were screened, including the location, review-scope-remainder, feedback-identity, and shared parser proposals. This continuation should be implemented and evaluated with that existing work, not counted as another unrelated architecture.

# Multi-shard review findings lose their location before deduplication

## Decision and scope

Preserve finding identity at the review parser boundary, and never treat missing coordinates as proof that two findings are duplicates. At main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), the multi-shard review parser creates findings with severity, full message line, and reviewer source, but does not populate `File` or `Line`. Its immediate deduplication consumer keys solely on those two fields. Every accepted finding therefore has the same key, `:0`, and any nonempty set of these parser-produced findings deduplicates to only one record.

That list supplies the detailed findings section of the narrative-generation input and the structured findings saved for later conversation. The system still displays and optionally persists the per-shard parsed findings, including the original location text inside each accepted message line. This is **loss in selected downstream projections**, not disappearance of every finding everywhere. Actual narrative omissions, missed follow-up work, model recovery, and costs have not been measured.

This source-only review was conducted October 30, 2026 against the pinned main revision, after reviewing guidance, current audit inventories, and research branches through `research-main-20261030-1227`. No codeNERD execution, tests, model calls, training, or benchmarks were performed. This document is a proposed continuation, not an implementation or reproduced incident.

## Relationship to the existing repairs

The [review-feedback identity audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/review-feedback-identity-and-current-verdicts.md) already addresses review-ID correlation, repeated feedback records, and current-verdict semantics. It acknowledges the aggregator's file-and-line deduplication policy and deliberately does not redefine same-location duplicates. This finding precedes that policy: different locations never reach its key fields. No feedback command, repeated judgment, or timestamp collision is necessary.

Several implemented improvements matter:

- The reviewer source is now carried consistently through `ParsedFinding.Source` and `reviewFindingMaps`. The code and regression explicitly repair the previous `Source` versus `ShardSource` disagreement. Preserve that provenance. [Consumer and rationale](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L339-L360), [producer-to-renderer test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/wizard_test.go#L713-L745).
- The ordinary shard-result path already uses a richer `extractFindings`: it recognizes the documented bullet format, extracts location and message, normalizes severity aliases, and retains an unparsed severity-bearing line instead of dropping it. This is an available repaired path, not a missing system-wide capability. [Parser](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L42-L108), [ordinary result storage](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L264-L289).
- Narrative projection no longer takes only twelve findings or a short enhancement prefix. It intentionally asks for the entire deduplicated list. The present residual occurs before that repaired projection receives its input. [Full-list narrative projection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L742-L752).

The proposed repair should compose these improvements, not replace them with another independently maintained prose parser or weaken actual duplicate handling.

## Reachable production chain

### 1. A normal `/review` can select aggregation

`handleReviewCommand` takes the multi-shard route when the loaded registry is nonnil and contains at least one agent. Otherwise it falls back to the single reviewer path. A populated registry is the necessary branch condition; the witness does not require every registry agent to be selected as a specialist. The aggregator always spawns the generic reviewer and additionally runs matching specialists and, when enabled, Nemesis. [Command branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers.go#L1028-L1070), [worker dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L512-L579).

After workers return, `aggregateReviewResults` skips error-bearing results, parses every remaining result, stores the parsed slice under its shard name, adds its length to `TotalFindings`, and deduplicates the collected slices. Nil execution error means eligible output here; it is not independent evidence that the review is correct or verified. [Aggregation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L638-L677).

### 2. The parser does not produce the deduplicator's identity

`parseShardOutput` iterates nonempty output lines, looks for one of `CRITICAL`, `ERROR`, `WARNING`, or `INFO`, and appends a `ParsedFinding` with only `Severity`, `Message`, and `Source` initialized. Even a line that explicitly names a file and line number leaves the typed location blank and zero. The message contains the complete trimmed line, so its textual citation survives. [Parser](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L264-L287).

`deduplicateFindings` constructs `fmt.Sprintf("%s:%d", f.File, f.Line)`. It stores the first record for a key, replacing it only when a later record has strictly higher severity. There is no unresolved-location branch or fallback identity. All parser-produced records consequently collide at `:0`. [Deduplication](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L805-L842).

For equal severities within one shard's slice, the first encountered finding survives. Across different shard entries, the map iteration does not establish a stable global winner. Higher severity replaces lower severity according to the existing rank map. The conclusion is cardinality collapse, not a promise that a particular specialist always wins.

### 3. Two real consumers trust the collapsed list

`formatReviewNarrativeContext` includes total count and summary, then renders only `agg.DeduplicatedList` as its detailed findings. `generateReviewNarrative` passes that constructed context to `interpretShardOutput`. The translator builds its prompts and makes a real `CompleteWithSystem` call; this is not a dead reporting helper. If no client is present or narration fails, no narrative is produced, but the structural finding list is still collapsed. [Narrative construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L696-L754), [articulation call](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_dream_delegation.go#L400-L432).

On completion, `model_update.go` displays the report and invokes `storeAggregatedReviewResult`. That method converts only `review.DeduplicatedList` into `lastShardResult.Findings`, while `RawOutput` holds the rendered report. It also keeps the canonical review ID and aggregate metrics. This leaves a genuine discrepancy: the rendered per-shard report can expose multiple accepted findings while the structured continuation has only one, still without typed coordinates. [Update](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L237-L259), [follow-up storage](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L299-L360).

The structured follow-up is operationally useful: `formatShardTaskWithContext` consumes prior reviewer findings when constructing fix or refactor tasks. Its fix path can inject only findings present in that slice. A model may recover other concerns from rendered history, the current request, source inspection, or additional context; static inspection does not establish deterministic omission of the actual repair. [Follow-up task construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L112-L173).

## Minimal source-derived witness

Use normal initialized `/review` aggregation and a successful worker result containing two distinct findings in the ordinary documented finding shape:

```text
- [CRITICAL] a.go:10: authorization check is absent
- [CRITICAL] b.go:20: credential material reaches the log
```

These are illustrative fixture findings, not claims about the repository. Both use `CRITICAL`, which this parser accepts, so the witness does not depend on its narrower severity vocabulary or arbitrary prose mentioning a severity word.

| Stage | Expected from the inspected implementation |
|---|---|
| Parser output | Two records; both have `File == ""`, `Line == 0`; each retains its own full message line |
| Per-shard slice / total count | Both records remain; count contribution is two |
| Deduplicated list | One record, because both keys are `:0` |
| Detailed narrator findings | Only the surviving record from the deduplicated list; total/summary can still indicate more findings |
| Rendered per-shard section | Both accepted lines remain visible |
| Structured follow-up findings | Only the surviving record, with unresolved typed location |

If these are the only two contributing findings from one worker, the first survives the equal-severity comparison. Additional workers can change the survivor, but do not restore distinct keys. No particular model output frequency or production incidence is asserted.

## Protections and bounded consequences

The report is not globally censored. `formatMultiShardResponse` iterates `FindingsByShard`, and `formatShardSection` displays each message, using `(general)` when the structured file is empty. Conditional database persistence and Markdown export likewise consume the per-shard findings. Their message lines retain textual citations. Do not describe this as preservation of every original worker transcript: aggregation does not retain every raw result body, and these paths preserve the accepted parsed lines. [Visible report](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L1123-L1173), [section rendering](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L360-L380), [persistence and export](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L603-L631).

Worker failures are separately marked incomplete. Narrative generation has a configured articulation timeout, and failure returns an empty narrative rather than erasing the remaining report. A single accepted finding has no cardinality loss, although its typed location is still absent. Manually constructed findings with distinct valid coordinates deduplicate correctly. The ordinary nonaggregate result parser is a meaningful alternative path. [Failure and narrative handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L583-L601), [timeout and failure](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator.go#L680-L711).

The supported consequence is reduced structured detail and a misleading equivalence class at the producer-consumer boundary. Narrative quality and follow-up completeness may suffer, but remain model- and context-dependent. This does not demonstrate unauthorized access, unsafe edits, false verification, automatic execution of every finding, or corruption of every persisted review.

## Proposed identity and projection contract

1. **One finding representation:** share a parser or typed decoder across ordinary and aggregate review paths. Preserve raw accepted text, normalized severity, source reviewer, location parse status, and any parsed location. Do not merely copy the ordinary parser's severity strings into consumers whose rank vocabulary still differs; update the whole producer-consumer contract together.
2. **Unknown is not equal:** findings with absent or invalid coordinates receive distinct source-bound identities, such as review, participant, and finding ordinal, until equivalence is established. A common zero value cannot justify deletion. Retain unresolved records visibly rather than inventing a file or dropping them.
3. **Preserve existing duplicate policy:** for valid same-location records, keep the intended highest-severity behavior unless a separate policy change is approved. Distinguishing different diagnostics at the same line is worthwhile but not required to fix this witness. Keep provenance of contributing records where practical.
4. **Location is evidence, not authority:** parsing a path and number does not prove the concern, establish that the file belongs to the workspace, or authorize a change. Continue source validation, membership, normal tool gates, and existing human/work-mode requirements.
5. **Conserve detail through projections:** narrative input and continuation state should account for every admitted distinct finding or explicitly expose a bounded, recoverable projection with coverage metadata. Counts must distinguish parsed records, unresolved records, and deduplicated findings. A count of two alongside one unmarked detailed record is not sufficient coverage.
6. **Keep display and recovery:** retain per-shard sections, original accepted lines, source attribution, canonical review ID, incomplete reasons, and the rendered report. Reusing those as recovery evidence is preferable to rerunning specialists solely to reconstruct dropped coordinates.

Implement transport and parsing in Go while keeping any policy choice about grouping, unresolved evidence, or required follow-up explicit to the executive. Update relevant architectural contracts before implementation, as repository guidance requires. No additional model call is inherently necessary for deterministic location extraction and collision avoidance.

## Costs and evaluation

A shared representation requires parser compatibility, severity normalization, unresolved-state handling, migration of consumers, and regression coverage. Retaining N real findings instead of one increases memory and potentially narrator tokens; it may increase legitimate follow-up work. Deduplication can remain approximately linear in admitted records with a map, while sorting and any source-validation IO add their own cost. These are design expectations, not measurements. Lower tokens achieved by losing findings are not an efficiency win.

Existing tests establish useful parts but not this composition. Deduplication fixtures manually populate distinct file/line values. The source-attribution regression runs the real parser and renderer with one finding, but does not pass several parser outputs through deduplication. Preserve these tests while adding the missing joined path. [Deduplication fixtures](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/review_aggregator_test.go#L9-L75), [source-attribution fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/wizard_test.go#L713-L745).

An authorized future evaluation should first enter a normal `/review` with a real populated registry and capture actual worker returns, aggregate state, narrative request, displayed report, and saved continuation. A deterministic parser→aggregate fixture can isolate the two-line witness, but must not be presented as evidence of normal model behavior. Test two locations in one worker, two workers, equal and unequal severities, genuine same-location duplicates, general findings without a location, malformed citations, Windows paths, optional line numbers, and supported severity aliases. Confirm that missing coordinates never merge unrelated records and that legitimate duplicates still follow the declared rule.

Then evaluate a follow-up request through ordinary perception and delegation. Measure structured finding recall, citation fidelity, narrative omission, requested repair coverage, recovery from retained report text, and false merging. Keep world-model/CodeDOM context, normal tools, gates, model settings, and output-generation conditions unchanged. Include failed workers, no narrative client, narrative failure, missing persistence, and the single-reviewer fallback as controls. Charge additional tokens, latency, IO, storage, and any necessary follow-up work to the treatment. No rerun or implementation performance claim is established by this document.

The narrow acceptance condition is that two accepted findings with different source locations cannot become one solely because the producing parser left both typed locations at their defaults.

# Budget original task terms in project knowledge lexical fallback

CodeNERD should protect selected original task and target terms before spending its lexical budget on generated synonyms. Project knowledge lookup can discard every distinguishing task term before SQL matching, even with one healthy, relevant row. This concerns candidate coverage, not ranking after limiting or index freshness. Preserve retrieval bounds and semantic-first behavior; do not incidentally reorder the shared embedding query.

## Scope and evidence

This source-only addendum freezes public sources at 2026-10-29, approximately 17:14 UTC, on main commit `9dcb01b9736bf4763aa3ac391d29f133d73c6611`. No reproduction, test, model inference, or training was executed. The example below is a source-derived expectation, not a production incident or a measured recall improvement.

The seven central files (`internal/prompt/{query_expansion,compiler_db}.go`; `internal/store/{knowledge_lexical_db,local_knowledge}.go`; `internal/session/{executor,task_executor}.go`; `internal/system/factory.go`) were byte-identical at the inspected Mangle/store head `a53cfc308e8836c72370adaa46deba3f7b00180d` and session head `80af1863d3a3aa46a722ac8b0c8a0985582d4995`. OAuth head `aa7988dec4c62116656ea722ff73b687a8509908` matched the five query, store, and task-executor files, but its executor and factory differed; full production-route parity is not established there.

Existing audits cover bounded score populations, retrieval coverage, and the separate LearningStore candidate window. The learning audit credits knowledge retrieval with ranking before its SQL limit. This addendum concerns the earlier term-selection decision that determines row eligibility. Bounded guidance searches found no exact treatment of this seam, not proof of global novelty. [Bounded score populations][bounded] [Retrieval coverage][coverage] [Learning candidate window][learning]

## Where the distinguishing terms disappear

`buildExpandedQuery` produces ordered, unique, lowercased terms and removes stopwords. It adds the verb and its synonyms, each target token and its synonyms, then shard tokens and `SemanticQuery`; language and framework terms can follow. The full string contains later task terms. They disappear downstream, not in the builder. [Expansion ordering and tokenization][expansion]

The knowledge collector sends that expanded string to project lookup. For selected experts, it instead prepends `cc.SemanticQuery` to the expanded string. Project lookup first requests five semantic results and returns a successful nonempty result without lexical retrieval. Otherwise it attempts lexical retrieval using the same expanded query. An absent embedding engine returns `nil, nil` and is not itself a degraded-health condition; actual errors are handled separately. [Project and expert query construction][collector] [Semantic fallback control flow][fallback] [No engine behavior][noengine]

`knowledgeLexicalKeywords(query, 8)` retains only the first eight distinct whitespace-separated terms of length at least two, after lowercasing and trimming boundary punctuation. SQL uses escaped `LIKE` matches over concept or content and orders by keyword-count relevance before applying its fetch limit. For the current five-result JIT request, at most fifteen rows are fetched. Ranking before limiting is already protective, but cannot restore a term removed before SQL construction. [Keyword selection][keywords] [Bounded lexical query and ranking][lexical]

## A supported production route

This does not require an invented classifier output. Structured task execution preserves canonical verb and target in a preset intent and calls `ProcessWithIntent`. The executor copies that preset rather than re-perceiving it, derives `IntentVerb` and `IntentTarget`, and supplies nonempty input as `SemanticQuery`, capped at 4096 runes, before compilation. [Preset construction][presetconstruction] [Structured task execution][taskroute] [Preset preservation][preset] [Context construction][context] [Intent fields][intent]

The factory can continue booting after optional embedding initialization or health-check failure while the context remains live. It attaches an engine only when one exists and attaches the local store to JIT independently. A healthy project store without embeddings is therefore supported; no explicit disable switch is asserted. [Optional engine setup][factory] [Local store attachment][storeattachment]

## A one row source derived counterexample

Consider this unrun compilation context:

```text
IntentVerb: /fix
IntentTarget: database/checkpoint.go
ShardID: coder
SemanticQuery: wal checkpoint stalls
```

Assume a healthy project store without an embedding engine, no selected expert containing the row, and one synthetic knowledge atom:

```text
Concept: wal checkpoint
Content: Passive checkpoints can stall behind long-lived readers.
Confidence: 0.9
```

The storage path preserves concept, content, and confidence. The expansion tables and ordering yield these first eight lexical terms:

```text
fix debug error issue resolve bug database db
```

Neither stored field contains any of those terms. `checkpoint` from the target and `wal`, `checkpoint`, and `stalls` from the task occur later in the expanded string and do not reach SQL. Project lexical lookup therefore has no matching row under the source-defined predicates. Direct lexical lookup with the raw task text would match the sole row through `wal` or `checkpoint`. A real structured task with that task text and an empty request produces `fix wal checkpoint stalls` as semantic text; the project-side omission remains. [Expansion tables][tables] [Expansion ordering][expansion] [Keyword selection][keywords] [Knowledge insertion][insert] [Structured task execution][taskroute]

This needs no large corpus, timestamp ordering, stale state, database error, or custom policy. Other project atoms, kernel content, experts, or later tools may compensate. A selected expert with the same row and short task-prefixed query provides a useful control, not project-retrieval equivalence.

The established consequence is candidate omission. Final-prompt loss also requires that the atom would otherwise pass context, conflict, capability, dependency, and whole-atom budget gates and lack an alternative source. Knowledge enters before selection and budgeting as optional retrieved context with source and confidence tags. Recovery confers neither authority nor guaranteed inclusion. [Knowledge conversion and merged cap][conversion] [Selection and budget pipeline][pipeline] [Retrieved knowledge guidance][guidance]

## A bounded contract and implementation choices

Specify that chosen original task and target terms cannot all be displaced by generated synonyms. Keep an explicit eight-term budget, or document a deliberately changed quota; no finite budget promises to retain every word of a long request. Preserve five returned results, fifteen fetched rows for that request, and ten merged project-plus-expert atoms, along with SQL escaping, cancellation, confidence, provenance, and normal Mangle admission gates.

Compare three policies rather than assuming a universal improvement:

1. Give project lexical fallback a separate query with short original task prose first, following the selected-expert precedent. This is a small change but can still waste slots on stopwords or path punctuation.
2. Build an explicit lexical token plan: selected task and target terms first, then a declared routing and synonym quota. This makes normalization, deduplication, and overflow decisions reviewable.
3. Retain and document the existing prefix policy if evidence shows its generic terms are preferable for the intended corpus.

Keep the semantic query unchanged while evaluating lexical policies: the consumers have different semantics. If new runtime settings control quotas, include their revision in relevant configuration hashing; this finding does not require a cache redesign. The existing zero-result health treatment is appropriate for the actual truncated query and does not prove freshness or adequate task coverage.

## Costs and tradeoffs

A local term planner adds normalization work, allocations, compatibility decisions, and tests. With unchanged term and result caps, it need not add a database query or embedding call. That does not establish equal database time: different predicates change selectivity and sorting work, potentially in either direction. No efficiency savings or asymptotic improvement is claimed.

Retrieval distribution changes are the main product risk. Generic repair vocabulary may retrieve a useful playbook that a task-specific policy misses; task terms may retrieve irrelevant literal matches. Additional admitted knowledge consumes prompt tokens and can displace better atoms. Changing the shared semantic query would also broaden the experiment to embedding-cache identity, provider work, and semantic ranking, obscuring the lexical question. Keep diagnostics bounded and avoid logging full private task text merely to explain term selection.

## Evaluation before changing behavior

Inspected tests verify fix synonyms and semantic text appearing somewhere in the expanded string. The project-and-expert coexistence fixture lacks verb and target expansion. These checks do not establish what survives the eight-term boundary. None was executed here. [Expansion assertions][expansiontests] [Semantic text assertion][semantictest] [Coexistence fixture][coexistence]

Add a composed test from actual context construction through expansion, lexical reading, and compilation. Assert final lexical terms and candidate source, not only a substring in the expanded string. Use the direct raw-task query as the one-row control and selected-expert retrieval as the asymmetry control. Cover matching early-term rows, shorter expansion prefixes, different verbs and paths, short and long tasks, Unicode, punctuation, stopwords, wildcard escaping, duplicates, and empty queries.

Exercise no-engine, healthy empty-semantic, relevant nonempty-semantic, actual-error, and canceled-deadline cases. Preserve the intentional nonempty-semantic short circuit rather than turning this into an always-hybrid retrieval proposal. Include absent selected experts and sibling isolation, distracting generic rows, and tight prompt budgets; confirm no authority or whole-atom-fit regression.

Compare the current, prose-prefix, and structured-quota policies on the same corpus with fixed eight-term, five-result, fifteen-fetch, ten-merged, and full-prompt budgets. Measure SQL time and plans, planner CPU and allocations, rows fetched, candidate usefulness, irrelevant matches, and final inclusion or omission. Include cases favoring generic synonyms. Only later authorized model trials can assess verified task outcomes and total provider, token, tool, retry, and human-repair costs. Until then, the warranted conclusion is narrow: protect original intent deliberately at the lexical budget boundary, and evaluate the tradeoff without relaxing existing safeguards.

[bounded]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/jit-generation-aware-optimization.md#L105-L117
[coverage]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/retrieval-freshness-coverage-and-context-value.md#L99-L125
[learning]: https://github.com/theRebelliousNerd/codenerd/blob/c89392f38fe86f0cad4c82caab9d7d832b82ea43/Docs/audits/learning-lexical-candidate-window.md#L7-L31
[tables]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/query_expansion.go#L18-L36
[expansion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/query_expansion.go#L54-L177
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_db.go#L329-L383
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_db.go#L385-L439
[noengine]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_knowledge.go#L422-L447
[keywords]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/knowledge_lexical_db.go#L143-L161
[lexical]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/knowledge_lexical_db.go#L17-L121
[presetconstruction]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/task_executor.go#L237-L249
[taskroute]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/task_executor.go#L415-L441
[preset]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1258-L1268
[context]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1361-L1397
[intent]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1491-L1502
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L1702-L1742
[storeattachment]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L1856-L1857
[insert]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_knowledge.go#L43-L68
[conversion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_db.go#L449-L478
[pipeline]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_pipeline.go#L282-L337
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/agents.md#L47-L58
[expansiontests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/query_expansion_test.go#L24-L30
[semantictest]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/query_expansion_semantic_test.go#L50-L66
[coexistence]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_expert_knowledge_test.go#L154-L177

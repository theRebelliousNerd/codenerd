---
doc-class: deep-dive
subsystem: campaign-prompt-store
implementation-status: proposed-refinement
last-verified: 2026-10-29
verified-against: 6d8477940cbeaa795e1917330b79853048189ca2
verification-method: static-source-inspection
supersedes: []
---

# Campaign specialist knowledge needs a live retrieval contract

## Decision and evidence limit

The default campaign specialist-knowledge provider is installed, but its semantic branch has no embedding engine. Its fallback treats a natural-language task sentence as an exact knowledge concept. Consequently an admitted explicit-specialist task can receive no campaign `SPECIALIST KNOWLEDGE` section despite an existing, relevant specialist KB—even if the new boot backfill has successfully embedded it.

This is a bounded retrieval integration and ownership finding, **not** a claim that every campaign lacks knowledge, that final task output must be wrong, or that a safety gate is bypassed. The downstream JIT expert bridge can independently recover knowledge. Fix or retire the ineffective duplicate route rather than declaring stored vectors sufficient or constructing a new embedding engine per task.

Source pin: main `6d8477940cbeaa795e1917330b79853048189ca2`. Inspection used source connectors only: no tests, code execution, inference, training, implementation or computer tasks. The conditional witness below is a static deduction, not a reproduced incident or a prediction of planner output.

## Reachable production path

| Boundary | Source evidence | What it establishes |
|---|---|---|
| TUI entry | [command handler 640–658][command], [campaign start 32–87][start] | `/campaign start <goal>` builds the chat orchestrator, decomposes and installs a campaign. |
| Dependency construction | [chat builder 112–141][builder], [constructor 45–66][constructor] | The standard constructor installs `NewLocalSpecialistKnowledgeProvider(cfg.Workspace)`; the provider receives only a workspace. |
| Task admission to this lane | [dispatch 187–207 and explicit-shard path 262–278][dispatch] | A valid task with nonempty `Task.Shard` invokes specialist enrichment before dispatch. `TaskTypeTestRun` takes its host path first. This is not every campaign task. |
| Local retrieval | [provider 37–121][provider] | It opens the named expert DB with `NewLocalStore`, immediately attempts semantic search, then falls back to `GetKnowledgeAtoms(extractKeyConcept(task))`. No engine is attached. |
| Engine ownership | [constructor 79–142][store-open], [semantic guard 426–439][semantic] | A new store initializes its DB and supporting structures, not an embedding engine. Semantic knowledge search returns `nil, nil` when that handle has no engine. A vector's persisted bytes do not attach one. |
| Exact fallback | [concept query 184–197][exact], [task reduction 124–142][reduction] | SQL uses `WHERE concept = ?`. The candidate concept is the first sentence or at most 50 bytes of the task, preferring a word boundary—not topic extraction or lexical retrieval. |
| Missing-section projection | [enrichment 178–197][enrichment] | Only nonempty returned atoms append the specialist section. An empty result is not reported as failed semantic capability. |

The new [boot synchronization][sync] and [backfill adapter][backfill] can attach the configured engine, create vectors and close their own repair store successfully. The campaign provider later opens a **different** store without that engine. This does not undo repair or corrupt vectors; it means that repairing storage alone cannot activate this consumer.

## A minimal, conditional witness

1. A workspace has a registered specialist `goexpert` and a real research atom with concept `go concurrency:section_0`, whose content explains goroutine cancellation. This follows the [research parser's concept form][research-concept]. The KB is readable; optionally it already has a correct vector in the configured space. This is genuine source knowledge, not the fallback templates discussed in the prior audit.
2. An otherwise admitted campaign contains a valid non-test-run task with `Shard: "goexpert"` and description `Review goroutine cancellation for the worker`. Assume required evidence assembly succeeds and ordinary campaign gates admit execution; no particular planner response is asserted.
3. The provider opens that expert KB with no engine. Its semantic call returns no atoms.
4. The exact fallback asks for concept `Review goroutine cancellation for the worker`, not `go concurrency:section_0`. If no atom has that exact sentence as its concept, the result is empty and the campaign's specialist section is omitted.

This mechanism requires neither provider failure nor a model-space change. It is older than the latest backfill commit; the new repair makes the producer/consumer distinction especially important, not a newly introduced regression.

## Counterevidence and scope

- The provider correctly checks for a missing KB before opening it and closes its owned store. Preserve the no-creation behavior and ownership.
- Exact concept requests can succeed. A task literally naming a stored concept is a positive control, not evidence that natural task descriptions work.
- The [task executor normalizes the specialist intent][normalize]; the serving executor sets the [same expert's ShardID][expert-id] and uses [actual task input for retrieval][query]. The [JIT expert collector][jit] already performs bounded, cancellable lexical retrieval from that selected expert's shared handle. Isolated execution also preserves the intent and enters the [same serving executor][subagent]. A healthy normal TUI path can therefore compensate, subject to successful compile and atom admission. Project knowledge, upstream evidence, source preloading, consultation and ordinary tools can also supply the information. No guaranteed final-answer omission or task failure follows.
- `/test_run` and tasks without explicit `Shard` are outside this provider witness. Existing risk, write-set, tool-authority, checkpoint and verdict gates must remain unchanged.
- Generic vector readers correctly exclude foreign/unstamped spaces. The [embedding migration audit][embedding-audit] already owns that contract and deliberately avoids automatic external spend on a mismatch. This proposal neither reopens its fixed reader defects nor mandates auto-migration.
- [Fallback research coverage][fallback-audit] owns templates being counted as researched topics; [terminal knowledge synthesis][terminal-audit] owns consultation evidence reaching the final chat answer. Neither is this stored-expert retrieval boundary. All 90 main audit files and the recent research branches `0345`, `0316` and `0248` were screened; no exact campaign provider/engine/fallback finding was identified.

The existing [dependency-injection test][di-test] checks provider type/workspace, and the [provider test][provider-test] checks missing-DB noncreation. Those are useful safeguards but do not establish successful retrieval of a populated KB through a normal task sentence. They were inspected, not run.

## Proposed bounded contract

Specify one route from **selected expert identity → owned source handle → bounded retrieval → policy-admitted context → actual worker request**.

Resolve duplication with JIT first. Prefer one JIT-owned expert collector when it covers this campaign lane: retain the intended expert identity and retire the redundant provider only after a production-path sentinel proves that the same expert's relevant content reaches the actual request. A missing section alone does not justify deleting an integration point, and adding another retrieval/model call without measuring JIT's existing delivery is not a repair.

If campaign-specific enrichment remains necessary, reuse the existing [lexical knowledge query][lexical] for natural task text instead of exact-concept fallback. It already uses `QueryContext`, limits candidate/result counts, searches concept and content and escapes SQL wildcard input. Keep a bounded retrieval receipt and deduplicate it with JIT before prompt assembly. Reuse a registered read handle when available; never close a borrowed handle or open a write-capable store merely to search. Do not indiscriminately scan every expert.

Semantic retrieval is an optional further capability, not the necessary first fix. If retained, borrow the already-resolved workspace engine through an explicit lifetime contract; do not select a provider/model or trigger migration within each task lookup. Preserve space eligibility and expose semantic-unavailable separately from no matching knowledge. Missing KB, empty KB, no match, cancelled lookup and failed lookup deserve distinct measured dispositions; Mangle decides whether the task proceeds qualified, retrieves more or waits.

Keep source identity/confidence and whole-atom or usable-handle delivery under existing context budgets. Retrieval is evidence, not instruction authority or proof of correctness. Do not fix a quiet miss by dumping every KB row into the task or adding unconditional model judgment.

## Costs and fair evaluation

The lexical option needs no new model request or embedding call. It adds bounded local query/merge work and can increase useful prompt tokens; removing repeated DB construction may offset some IO, but no saving is established. Semantic use adds query-embedding latency/cost and engine-lifetime complexity. Reuse must key workspace, expert and source revision; caching by task text alone risks stale or cross-expert context.

Future checks should compose the real constructor and explicit-task enrichment with a deterministic observed executor, then separately inspect the actual model request with normal JIT enabled:

| Case | Required observation |
|---|---|
| Relevant populated KB; natural sentence differs from concept | Correct expert marker reaches admitted task context, or an explicit justified disposition; a provider object or row count is insufficient. |
| Exact concept, unrelated query, sibling expert | Preserve exact success; no irrelevant or cross-expert inclusion. |
| Missing/empty KB, cancelled/failed read | No DB creation, leak or late work; absence and failure remain distinguishable. |
| Healthy vectors, missing engine, incompatible vectors | Lexical path remains available; no incompatible comparison, guessed stamp or implicit migration spend. |
| Large atoms and overlapping JIT hits | No silent truncation, duplicate injection or budget overflow; usable omission/handle semantics. |
| Explicit specialist, implicit route, host test run | Assert the intended lane boundaries and unchanged authority/verdict behavior. |

Use matched tasks, engines, source snapshots and budgets. Compare current behavior, JIT-only consolidated retrieval, shared lexical retrieval, and explicitly wired semantic-plus-lexical retrieval. Measure correct-source inclusion and irrelevant-source rate, independently checked task quality, retries, tool calls, full token/provider cost, startup and per-task latency, DB opens, CPU/memory and human correction time. Include misses, cancellations and failed campaigns in denominators. Separate the provider-level repair from downstream JIT compensation; otherwise a passing end-to-end answer can hide the dead branch, or a local miss can exaggerate user impact.

Exit criterion: a normal admitted campaign task can use its selected expert's relevant saved knowledge through a demonstrated, bounded consumer—not merely through a populated database or a successfully constructed provider.

[command]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/cmd/nerd/chat/commands_handlers.go#L640-L658
[start]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/cmd/nerd/chat/campaign.go#L32-L87
[builder]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/cmd/nerd/chat/campaign_planning.go#L112-L141
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/campaign/orchestrator_init.go#L45-L66
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/campaign/orchestrator_task_handlers.go#L187-L278
[provider]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/campaign/specialist_knowledge.go#L37-L121
[store-open]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/store/local_core.go#L79-L142
[semantic]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/store/local_knowledge.go#L426-L439
[exact]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/store/local_knowledge.go#L184-L197
[reduction]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/campaign/specialist_knowledge.go#L124-L142
[enrichment]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/campaign/specialist_knowledge.go#L178-L197
[sync]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/prompt/sync/synchronizer.go#L143-L175
[backfill]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/prompt/sync/knowledge_backfill.go#L17-L35
[jit]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/prompt/compiler_db.go#L329-L439
[normalize]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/session/task_executor.go#L359-L372
[expert-id]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/session/executor.go#L1564-L1594
[query]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/session/executor.go#L1338-L1345
[subagent]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/session/subagent.go#L337-L356
[embedding-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/Docs/audits/embedding-representation-and-migration.md
[fallback-audit]: https://github.com/theRebelliousNerd/codenerd/blob/76e1b31d784a4e4340e189e1bf514c86b4a24537/Docs/audits/fallback-knowledge-research-coverage.md
[terminal-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/Docs/audits/terminal-knowledge-synthesis-grounding.md
[di-test]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/campaign/orchestrator_di_test.go#L10-L47
[provider-test]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/campaign/specialist_knowledge_test.go#L12-L28
[lexical]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/store/knowledge_lexical_db.go#L10-L103
[research-concept]: https://github.com/theRebelliousNerd/codenerd/blob/6d8477940cbeaa795e1917330b79853048189ca2/internal/init/agents_knowledge.go#L224-L245


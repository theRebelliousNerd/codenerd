---
doc-class: deep-dive
subsystem: research-context7
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Preserve Context7 omission notices across the public tool boundary

**Decision:** apply the document cap to document bodies, not to coverage metadata. A helper's explicit statement that it left linked documents unfetched must survive the wrapper that presents its result.

**Finding:** `parseLlmsTxt` correctly appends an omitted-document notice after its capped document strings. `executeContext7` then caps the combined string slice again, treating the notice as another document. Whenever the helper has filled the document cap and observed additional links, the wrapper drops the notice. The public tool returns the bounded documents without the omission count or the argument that would request more.

**Evidence status:** static source inspection of public main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked on 2026-10-30. No application, browser read, test, provider call, database experiment or model inference was executed. The result below follows from the source-defined composition; it is not an observed production incident or a measured answer-quality regression.

## Scope and neighboring work

This is narrower than the [Context7 documentation-ref audit](https://github.com/theRebelliousNerd/codenerd/blob/b42816320d9164c824aba7da0c54bc27df2b4279/Docs/audits/context7-documentation-ref-continuity.md). That audit concerns selecting the correct branch and retaining the manifest's base through relative-document acquisition. Here every selected URL can be correct, main-based and successfully read. The lost information is the helper's own acquisition-coverage notice, not the source URL or branch.

It also does not reopen generic web-fetch link resolution, snapshot-bound paging, browser ownership, or the byte guard. Nor does it argue that every linked document must be fetched. Bounded acquisition is appropriate; the defect is silently erasing an already measured limit. Root guidance favors bounded model context with recoverable omitted material, rather than presenting a bounded view without its applicable limitations. [Root guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md).

The audit inventory through 1018 was screened. The older argument-coercion report concerns numeric input acceptance, not this post-acquisition projection. This is bounded deduplication, not a claim to have proved global novelty.

## The producer has the intended safeguard

`Context7Tool` exposes `max_docs` as a maximum document count, defaulting to ten. `executeContext7` obtains that positive integer, locates the repository, and asks the llms manifest reader for documents. [Tool schema and effective count](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L15-L53), [manifest dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L74-L89).

The parser tracks successful document count separately from links skipped after the cap. Once `docCount >= maxDocs`, another parsed document link increments `skipped` without being fetched. At the end, if any links were skipped, it appends a separate string naming how many linked documents were not fetched and identifying `max_docs` as the argument that raises the bound. [Counter and notice](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L252-L304).

This is working local behavior and should be credited. The helper's comment explicitly describes a non-silent cap, and its focused test asserts both the omitted count and the recovery argument. The proposed change should preserve that information, not remove the cap or characterize the helper as lacking coverage awareness. [Helper contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L245-L251), [helper test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/research_coverage_test.go#L114-L134).

## The wrapper reapplies the cap to the wrong collection

The public wrapper combines the returned strings into a documentation response. It iterates `results` by index and stops when `i >= maxDocs`. It does not distinguish a document body from the helper's appended status string. [Public projection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L99-L110).

When the helper reaches its cap, the first `maxDocs` entries are documents and the notice is exactly the next entry. The second cap therefore discards the notice deterministically.

A small witness makes the boundary explicit:

| Stage | Value with three valid links and max_docs equal to one |
|---|---|
| Acquisition | Document A is fetched; links B and C are skipped |
| Helper result | One document string followed by a notice about two unfetched links |
| Wrapper result | One document string; no omission notice |
| Information lost | Observed omitted count and the named argument for requesting more |

Assume a readable main manifest, three valid links, a first document above the parser's 50-character threshold, healthy browser admission and transport, enough caller lifetime, and no source mutation. No failed read, malformed URL, branch fallback, race or oversized body is required. The helper need not fetch B or C to report that it deliberately left those links unfetched; the witness stipulates valid documents to avoid conflating omission with broken links.

The default-cap analogue needs eleven such links. Ten document strings occupy indices zero through nine; the notice is at index ten and is rejected by the wrapper's ten-entry cutoff.

## The public and knowledge paths both consume the clipped result

The wrapper is the actual registered tool's Execute function, not an unused presentation helper. Direct tool consumers receive its string. The normal background knowledge refresher also invokes `context7_fetch`, passing only a topic and returning `result.Result`. It supplies no `max_docs` override, so the default ten-document case reaches that production path. [Tool entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L15-L21), [background adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh_boot.go#L26-L47).

`Context7Source` consumes that returned body, splits sufficiently long sections into knowledge atoms and marks their source Context7. The acquisition outcome records source, subject, status, atom count, details, concepts and version; there is no separate document-coverage result in that interface. Thus the omitted-link notice is already gone before knowledge persistence. [Knowledge adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L124-L143), [outcome and source interface](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L91-L123).

Successful sections remain useful. The acquirer's `acquired` status means material was obtained; it is not an explicit declaration that every upstream document was read. This audit does not reinterpret that status as an exhaustive-coverage guarantee. The narrower problem is that neither the tool string nor the current knowledge adapter preserves the helper's actual observed omission.

Later expert retrieval and JIT admission may omit some stored sections for their own reasons. Those are separate, legitimate context limits, not evidence that the earlier loss is harmless or that any particular missing document caused a bad answer. [Expert knowledge consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L349-L386).

## Counterevidence and test boundary

The document-count cap works: no additional successful document body is emitted merely because the helper appended metadata. The response heading does not expressly claim exhaustive coverage. A model that knows it requested a small cap can remain cautious or request more without the notice. Other sources can also supply the omitted material.

Those qualifications limit the consequence. The demonstrated defect is loss of the distinction between an exactly full result and a known capped result, not an inevitable false-completion claim or a safety-gate bypass. The existing notice would communicate a stronger fact than a caller guessing that more documents might exist.

The tests cover the two pieces separately:

- The helper test calls `parseLlmsTxt` with three links and cap one, joins all returned strings, and requires the omitted-count notice and `max_docs`.
- The wrapper test also uses three documents and cap one, but asserts only that the output contains at most one `Source` block. It does not check that the notice survives.

Both tests can satisfy their local assertions while the public composition drops metadata. This is a test-coverage observation from source, not a report that the tests were run. [Helper assertions](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/research_coverage_test.go#L122-L134), [wrapper fixture and assertion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/research_coverage_test.go#L204-L225).

Preserve the independent safeguards: absent documentation produces an error, the browser path enforces its read contract, and large individual documents carry a byte-guard marker. None supplies the omitted linked-document count after the wrapper removes it. [No-document error](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L92-L97), [browser guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/agents.md), [per-document guard](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L334-L373).

## Repair the representation rather than raising the cap

The narrow public-boundary repair should preserve the helper's document cap and render coverage metadata outside that count. Do not raise `max_docs`, fetch everything or append another document merely to make room for the notice.

A typed internal acquisition result is preferable to representing both documents and control metadata as interchangeable strings. It can carry document bodies/source locators and coverage fields such as successful documents, observed links left unfetched and the supported recovery argument. The public tool can render a concise notice from those fields without counting it as a document.

Coverage must remain accurately scoped. The skipped count describes links the parser observed and declined to fetch; it is not proof of every document in the repository or of the availability of each omitted target. An unreadable or guard-limited manifest leaves additional unknown coverage. Failed attempted links and deliberately unfetched links are different dispositions. This focused fix need not solve every acquisition-coverage question, but it must not turn a known partial result into a new unsupported completeness claim.

The knowledge adapter deserves an explicit choice. If a text-only repair simply preserves the notice, its paragraph may be long enough for the current section splitter to store it as a Context7 knowledge atom. That is a repair-design risk: acquisition status is not library documentation. Prefer a typed path or explicit metadata channel that lets the knowledge layer retain coverage without embedding the notice as domain advice. A minimal public-tool fix can ship separately, provided its narrower scope is stated and it does not claim durable coverage propagation is already solved.

Mangle may use measured coverage to choose whether more acquisition is worth its cost; the wrapper should not silently decide that the limit is irrelevant. Preserve the user's and harness's existing budgets and permission boundaries. Knowing that documents were omitted is not authorization to fetch them automatically.

## Compatibility and costs

The smallest rendering correction adds a bounded notice to the returned text without adding browser requests. It therefore has modest output-byte/token cost and no inherent need to change the document count. It does not establish a measured reduction in total model turns or total spend.

Typed propagation costs more: helper return types, wrappers, mocks, knowledge adapters, outcome/reporting fields and potentially durable metadata must agree. Keep document counts distinct from status fields in serialization and budgeting. Metadata should have an explicit bounded rendering cost; fixing a silent cap by making an uncharged status channel arbitrarily large is not a sound trade.

Legacy knowledge rows have no reliable acquisition-coverage receipt. Treat that as unknown, not as evidence that the source was complete or definitely capped. Do not delete useful historical documentation or automatically refetch every agent's corpus to populate a new field. Targeted reacquisition can follow ordinary availability, priority and budget controls.

If downstream policy starts requesting more because it can now see omissions, extra browser reads, provider work, embeddings, database writes and prompt candidates are real costs. Compare those costs against better task evidence, not against the smaller but less informative baseline output alone. The fix should preserve information first; increased acquisition is a separate governed decision.

## Fair evaluation

All proposed checks are unrun. Extend the existing wrapper fixture rather than relying only on another direct parser test. Assert both that exactly the permitted number of document bodies is returned and that the actual omitted count and recovery argument survive.

Cover:

1. Fewer than the cap, exactly the cap, and more than the cap. An exactly full result must not be falsely reported as known-truncated.
2. Cap one with three valid links and the production default ten with eleven valid links.
3. Main-relative and absolute links, keeping branch resolution correct so the ref-continuity issue cannot mask this one.
4. Failed and short document responses before the successful cap, invalid/empty parsed entries and a guard-limited manifest. Do not count failed attempts as successfully acquired documents.
5. No-manifest common-document fallback, no documents anywhere, cancellation and browser failure. Preserve truthful errors and the existing cap.
6. A registered-tool call through the same adapter used by the refresher, then the knowledge ingestion path. Verify that coverage is retained in its intended channel and not embedded as domain advice.
7. Stable repeated acquisition and legacy metadata absence; no automatic unlimited fetch or migration storm.

Measure attempted reads separately from retained documents; output bytes/tokens separately from source bytes; and acquisition metadata separately from knowledge atoms. Include browser work, source latency, cleanup, storage/embedding work and actual JIT inclusion where applicable. For any later answer-quality study, hold tasks, models, document snapshots and budgets matched, and count unsupported completeness assertions as well as useful evidence delivered.

The acceptance condition is not merely fewer dropped strings. It is that the public result preserves the actual acquisition limit without exceeding the document budget, falsely claiming completeness, confusing metadata with source content, or widening authority.

## Branch check

At `767a3291d6287b238417f2466592a9002810d05a`, Context7 source is identical to main. The four older lane/QA heads retain the same two relevant operations despite older helper signatures: an appended omitted-documents notice and a wrapper cutoff at `i >= maxDocs`. Research head `b42816320d9164c824aba7da0c54bc27df2b4279` contains the separate ref-continuity audit. No competing coverage-notice repair was found in this bounded inspection.


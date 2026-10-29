# Document extraction coverage and unchanged document resumption

## Finding and scope

The production `/refresh-docs` path can persist atoms from one document chunk, warn that another failed, and nevertheless record the document as `Stored` at its current hash. A later non-forced refresh treats that receipt as extraction-complete and issues no extraction requests for the unchanged document, even after provider recovery. The defect is a producer/state outcome contract: partial acquisition becomes durable skip eligibility. It does not establish wrong prompts, semantic completeness guarantees, or impossible recovery. [extraction], [processing], [admission]

Snapshot: main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, 2026-10-29 at approximately 17:47 UTC; refs rechecked unchanged at 17:54 UTC. Seven witness files were byte-identical across Mangle/store `a53cfc308e8836c72370adaa46deba3f7b00180d`, session `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and OAuth `aa7988dec4c62116656ea722ff73b687a8509908`: `internal/init/strategic_documents.go`, `internal/init/orientation_documents.go`, `cmd/nerd/chat/helpers_scan.go`, `cmd/nerd/chat/model_update.go`, `cmd/nerd/chat/commands_handlers_analysis.go`, `internal/store/local_knowledge.go`, and `internal/prompt/compiler_db.go`. Research head `89329ac439acb7a8576f0a6189c1122ec9e6bd04` adds streaming grounding/source ownership and lexical-term-budget audits, not this case. This review is source-only: no runtime execution, tests, model calls, or measurements; the witness and evaluation remain unrun.

## The durable boundary

`DocProcessingEntry` records path, hash, status, atom count, and error, but no chunk-coverage receipt. `extracted()` accepts `Stored` and `Synthesized`. State is loaded from and atomically written to `.nerd/doc_ingestion_state.json`; unchanged-hash admission preserves extracted status unless forced. Atomic file publication protects the checkpoint file, not its completeness interpretation. [state], [admission]

The extraction function first retires previous path-owned atoms and the legacy exact-concept record, then chunks the current content with `chunkDocument(doc.Content, 8000)`. Each chunk reaches the main-slot `doc_extraction` request. A final request error or malformed JSON array is accumulated as a failure and processing continues; malformed output is not stored as raw knowledge. Valid atoms are persisted individually, with store errors also accumulated. Its aggregate-error condition requires failure count equal to chunk count, zero persisted atoms, and nonzero chunks. Other accumulated failures produce a partial-extraction warning and return the successful atom count with `nil`. [extraction]

Two chunks with one successful nonempty result and one request failure therefore return success to the document loop. It maps errors to `Failed`, but `nil` to `Stored`, saving the atom count, timestamp, and totals. Subsequent processing skips extracted entries. `Stored` means “Atoms persisted,” not every semantic fact discovered; the problem is using that partial collection as an unchanged-document skip receipt. [processing], [state]

This route receives full bodies: orientation retains file content, and `OrientationDocuments` transfers admitted snapshot bodies into `DocumentInfo.Content`. The chunker splits on newline boundaries when accumulated size crosses its threshold; sufficiently long bodies with suitably sized lines produce multiple chunks. This is not a hard 8,000-byte maximum—oversized single lines are separate. [orientation-body], [orientation-admission], [chunking]

The production TUI's `/refresh-docs` and `/scan-docs` handler defaults force to false. `runDocRefresh` obtains orientation documents and calls `ProcessDocumentsWithTracking` with the workspace store, kernel, clients, and force flag. Completion reports counts and duration without a chunk-partial outcome. Existing extraction warnings and request metrics can expose failures; not all diagnostics are silent. [dispatch], [handler], [refresh], [ui]

Optional synthesis reads stored `doc/` atoms and promotes `Stored` to `Synthesized` only after success. Both statuses qualify for reuse, so synthesis failure leaves the latch intact. With no other newly stored document, the next unchanged refresh has zero newly stored documents and skips optional synthesis. The checkpoint precedes synthesis. [synthesis], [refresh]

## Source-predicted witness, not a measured result

Use a newly admitted, valid-UTF-8 document with exactly two chunks, avoiding any dependence on old atoms or private documents. Keep the outer context live and make database and state writes succeed.

1. On the first manual, non-forced refresh, the first chunk returns a well-formed nonempty atom array and its row write succeeds.
2. The second chunk's `withJITPrompt` call finishes with a provider/request error, after any internal retries. A malformed-array result is a separate variant.
3. With one failure, two chunks, and at least one persisted atom, extraction warns and returns `nil`. Processing persists `Stored` and the current hash. Successful optional synthesis changes the status but not skip eligibility.
4. Restore provider health. Run `/refresh-docs` again with the same body hash, continuing orientation admission, an intact state file, and force false. Admission retains the extracted status; processing makes no extraction requests for this document. The failed chunk is not retried. [extraction], [processing], [admission]

That is a stable incomplete-coverage receipt, not guaranteed task failure. A deterministic fixture should place a unique marker only in the failed chunk and specify its mock extraction response. Request and row absence are observable; what a live model would have extracted is not established. Force, content-hash change, or unreadable state can reopen extraction. When each chunk ends in one request or parse failure and no atoms are stored, the document is marked `Failed` and remains retryable; that status does not guarantee a top-level TUI error. A valid empty array is a legitimate zero-result control, not automatically failure. [admission], [extraction], [processing], [refresh]

## What this does—and does not—own

Canonical row persistence differs from embedding availability. `StoreKnowledgeAtomWithEmbedding` writes the row first; optional embedding failure warns and returns success because the row exists. That differs from extraction that never produced its rows. [store] Normal JIT searches project-local knowledge semantically, falls back lexically, and applies bounded selection and prompt budgets. [consumer], [selection] It cannot retrieve an absent row here, but orientation bodies, other atoms, direct source access, or tools may compensate.

The finding survives those independent challenges without becoming a query-cache, learned lexical-window, query-term-cap, index-freshness, or model-truth claim. Correct source-generation tracking exposes existing rows; it cannot reconstruct an unacquired chunk. This is not ordinary initialization: current init documentation distinguishes the production refresh entry and credits its engine, force/skip, retirement, and counter wiring. [init-readme], [wiring]

Prior retrieval-saturation work owns complete/partial acquisition outcomes and valid positive hits. This addendum concerns a serial, durable document checkpoint, not concurrent keyword-cache publication fencing. Bounded audit searches found no exact checkpoint case, not proof of global novelty. Existing regression definitions cover successful incremental extraction, unchanged reuse, and forced malformed-response retirement, using successful one-atom replies rather than this mixed-chunk case. Tests were not run. [prior-outcomes], [prior-controls], [regression]

The proof depends on JSON state, not an assumed Mangle consumer. `assertDocFact` attempts an assertion and logs failure; this review does not establish that a document-ingestion fact is declared, accepted, or consumed. Any future fact-based coverage design needs actual declarations and readers, not merely another assertion site. [facts]

## Proposed outcome contract and repair choices

Represent expected chunks, completed valid chunks, failed or pending chunks, successfully persisted atoms, source hash, extractor-contract version, and failure reason separately. “Complete” should mean the agreed transport, validation, and persistence coverage completed for that generation—not that the model found every true fact. Only complete coverage at the current hash should earn unchanged skip eligibility. Useful current-generation rows may remain available during a partial result, but partial state, UI counts, and any synthesis qualification must remain explicit. [guidance]

Existing `Stored`/`Synthesized` receipts lack chunk coverage; migration cannot retroactively certify them from atom counts. Choose an explicit legacy-unknown/revalidation policy and disclose its provider expense, rather than silently re-extracting every unchanged document. [state]

The smallest repair is a typed partial outcome plus a durable partial receipt excluded from `extracted()` reuse. The next admitted refresh retries the whole owned generation. Keep source-owned retirement: do not present old-document claims as current merely because replacement extraction failed. If retained for availability, last-known content needs an explicit noncurrent identity.

A more selective alternative is a chunk manifest keyed by source hash, stable chunk identity, and extraction contract. Retry failed or pending chunks with bounded work and idempotent successful-chunk publication. Checkpoint after row publication, define reconciliation for a crash between SQLite and JSON, and invalidate receipts when the source or extraction contract changes. Atomic JSON replacement is not cross-store atomicity. If future parallelism permits late results, reject publication for outdated generations; do not hold a store mutex across LLM calls.

Neither option should automatically make optional enrichment fatal to whole initialization. Existing guidance calls for visible degradation, no filler knowledge, correct JIT routing, per-request bounds, and incremental work. Backoff may defer a retry, but must not label incomplete coverage healthy and complete. Preserve normal cross-source and prompt budgets; an elaborate outbox is not a prerequisite for fixing this boundary. [guidance]

## Costs and an authorized evaluation plan

Whole-document retry repeats successful LLM and embedding work, warnings, and latency. Chunk resumption adds manifest records, hashes, retained bytes, schema/metadata migration, lookup complexity, and ambiguity when chunk boundaries or extraction versions change. Staging generations adds storage and retirement cost. Qualified partial synthesis consumes additional prompt tokens and may yield less confident outputs. No net savings or quality improvement is measured here.

An authorized, initially scripted evaluation should exercise actual orientation admission, processing, and refresh-message forwarding: first-chunk success, second-chunk failure, then unchanged-source provider recovery. Inspect JSON hash/status/coverage, canonical rows, request counts, partial UI reporting, synthesis inputs, and missing-versus-restored JIT rows under explicit admission and sufficient budget. Also test first, last, interior, and multiple failures; malformed JSON; store errors; valid empty arrays; all-success reuse; all-failure retry; force; source change; demotion/deletion; cancellation; state-write failure; and reopening after rows publish but before checkpointing.

Compare baseline, whole-document retry, and chunk resumption with the same source, scripted failure schedule, model/provider/task conditions, and full prompt budget. Count repeated successful work, provider attempts including unsuccessful or refused calls, tokens and cached tokens, embedding calls, database writes, retained bytes, CPU/allocation cost, and latency. Assess useful knowledge and verified task outcomes only in later authorized trials, distinguishing coverage repair from semantic truth and compensation through full-body orientation. These are proposed observations, not results.

The exit condition is checkable: a failed chunk cannot grant complete unchanged-document reuse, while healthy unchanged reuse, useful positive rows, visible partial outcomes, and source-owned retirement remain intact.

[state]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/strategic_documents.go#L38-L141
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/strategic_documents.go#L151-L235
[processing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/strategic_documents.go#L270-L317
[extraction]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/strategic_documents.go#L329-L402
[chunking]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/strategic_documents.go#L405-L430
[orientation-body]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/orient/docs.go#L70-L94
[orientation-admission]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/orientation_documents.go#L15-L82
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands.go#L123-L124
[handler]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_analysis.go#L147-L166
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_scan.go#L247-L331
[ui]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L506-L529
[synthesis]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/strategic_documents.go#L459-L515
[store]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_knowledge.go#L381-L419
[consumer]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_db.go#L329-L458
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_pipeline.go#L282-L337
[init-readme]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/architecture/init/README.md#L1-L14
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/architecture/init/WIRING-AND-NOT-BUILT.md#L18-L21
[prior-outcomes]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/retrieval-saturation-outcomes-and-producer-lifecycle.md#L50-L60
[prior-controls]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/retrieval-saturation-outcomes-and-producer-lifecycle.md#L130-L146
[regression]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/init_regressions_test.go#L192-L268
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/strategic_documents.go#L86-L96
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/init/agents.md#L5-L21

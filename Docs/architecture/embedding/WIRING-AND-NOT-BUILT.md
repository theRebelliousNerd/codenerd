# embedding — WIRING-AND-NOT-BUILT

> Verified 2026-09-21 against branch `main` working tree; the last two
> sections re-verified 2026-09-25 against `fc857b3`. Anchors in "Wired and
> reachable" are from the 2026-09-21 pass.

What is wired and reachable, what exists but has no verified caller, and what
the design assumes that the code does not do.

## Wired and reachable

- `NewEngine` (`internal/embedding/engine.go:93`) is the construction entry
  point, called from chat ingest (`cmd/nerd/chat/ingest.go:64`), reembed
  (`cmd/nerd/chat/reembed.go:57`), init scan (`cmd/nerd/cmd_init_scan.go:328`),
  retrieve (`cmd/nerd/cmd_retrieve.go:226`), the embedding command
  (`cmd/nerd/embedding_cmd.go:109`, `:154`), both corpus tools
  (`cmd/tools/corpus_builder/main.go:164`,
  `cmd/tools/prompt_builder/main.go:237`), the campaign document ingestor
  (`internal/campaign/document_ingestor.go:27`), the initializer
  (`internal/init/initializer.go:426`), the semantic classifier
  (`internal/perception/semantic_classifier.go:210`), and the system factory
  (`internal/system/factory.go:1485`).
- `HealthChecker` (`internal/embedding/engine.go:50`) is asserted by retrieve
  (`cmd/nerd/cmd_retrieve.go:239`) and the factory
  (`internal/system/factory.go:1488`); only `OllamaEngine` implements it
  (`internal/embedding/ollama.go:296`), and its healthy-path test pins the
  `/api/tags` contract (`internal/embedding/ollama_coverage_test.go:447`).
- `TaskTypeAwareEngine` (`internal/embedding/engine.go:34`) and
  `TaskTypeBatchAwareEngine` (`internal/embedding/engine.go:41`) are asserted
  across the tree: reflection (`cmd/nerd/chat/reflection.go:209`), both corpus
  tools (`cmd/tools/corpus_builder/main.go:703`,
  `cmd/tools/prompt_builder/main.go:583`), the MCP analyzer and compiler
  (`internal/mcp/analyzer.go:66`, `internal/mcp/compiler.go:144`), the
  semantic classifier (`internal/perception/semantic_classifier.go:340`,
  `:604`, `:921`), the prompt loader (`internal/prompt/loader.go:272`,
  `internal/prompt/loader_embedding.go:130`,
  `internal/prompt/vector_searcher.go:49`), and the store
  (`internal/store/vector_store.go:139`, `:231`, `:450`, `:508`, `:566`,
  `:1131`, `internal/store/learned_store.go:223`,
  `internal/store/prompt_reembed.go:72`,
  `internal/store/reflection_worker.go:243`). Compile-time conformance is
  pinned by `internal/embedding/genai_coverage_test.go:71` and
  `internal/embedding/ollama_coverage_test.go:508`.
- `GetOptimalTaskType` (`internal/embedding/task_selector.go:183`) is the
  store's write- and query-path selector
  (`internal/store/vector_store.go:136`, `:222`, `:447`, `:505`, `:563`,
  `internal/store/vector_store_reembed.go:297`); `SelectTaskType`
  (`internal/embedding/task_selector.go:36`) is used directly where the
  content kind is already known (`cmd/nerd/chat/reflection.go:72`,
  `internal/prompt/loader.go:269`, `internal/store/learned_store.go:220`,
  among others).

## Exists but nothing verified calls

Re-verified 2026-09-25 (lane B build-out, against `fc857b3`). Each entry
now carries its decision.

- `GenAIEngine.EmbedBatchJob` (`internal/embedding/genai.go:403`): still no
  caller. **Declined, not wired.** It is the submission half only: it
  returns a job handle, and nothing in the tree polls the job or reads its
  results, so wiring it into reembed means building the other half against
  an SDK surface the code itself marks experimental — unverifiable here
  without a Gemini key. The synchronous `EmbedBatch` (parallel 100-text
  chunks, now retried) covers every caller today. Keep as a library entry
  point; wiring it is a maintainer decision.
- `FindTopK`: **removed 2026-09-25 (lane B wave 3).** It was test-only.
  The production rankers break ties deterministically
  (`internal/retrieval/semantic.go` sorts by score, then path) and apply their
  own filters; `FindTopK` was a partial selection sort with no tie-break, so
  routing them through it would have made results order-unstable. With that
  decision made there was nothing left for it to serve, and a ranker that
  must not be used is cruft. `SimilarityResult` went with it, and so did
  `int32Ptr` in `genai.go`, a `//go:fix inline` shim for `new(v)` with no
  caller.
- `DefaultConfig` (`internal/embedding/engine.go:83`): **wired 2026-09-25.**
  `config.DefaultEmbeddingConfig` (`internal/config/memory.go`) was a second
  hand-written copy of the same defaults; it now derives from this one, pinned
  by `TestDefaultEmbeddingConfig_ShouldBeTheEmbeddingPackageDefaults`.
- `DetectContentType` (`internal/embedding/task_selector.go:124`): its entry
  point is `GetOptimalTaskType` (`:230`) by design; no direct caller is
  needed.

## Assumed but not done — status 2026-09-25

- GenAI had no retry — **done, `fc857b3`.** `embedContent`
  (`internal/embedding/genai.go:63`) retries a 429, a 5xx or a transport
  error up to `genaiMaxAttempts` (3, `:56`), the wait observing the caller's
  context; a 4xx other than 429 is not retried (`retryableGenAIError`,
  `:89`). Both the single embed and each batch chunk go through it. The SDK
  retries uploads only. Tests: `TestGenAIEngine_RetriesATransientFailure`,
  `_DoesNotRetryABadRequest`, `_GivesUpAfterItsAttempts`,
  `_RetryWaitObservesTheContext` (`internal/embedding/guards_test.go`).
- Ollama batch is sequential (`internal/embedding/ollama.go:267`) — **open,
  declined for now.** Throughput from concurrent requests depends on the
  server's `OLLAMA_NUM_PARALLEL`, and the real fix, the batched `/api/embed`
  endpoint, changes the wire protocol; neither can be measured or verified
  without an Ollama server. Reopen with a measurement.
- Unknown task types reached the API — **done, `fc857b3`.** `checkTaskType`
  (`internal/embedding/task_selector.go:53`) refuses a type outside
  `knownTaskTypes` (`:38`, the set `config.EmbeddingConfig.TaskType`
  documents): at construction for `embedding.task_type`, and at the call for
  a caller's. `TestGenAIEngine_RefusesAnUnknownTaskType`.
- Metadata was trusted verbatim — **done, `fc857b3`.** `DetectContentType`
  normalizes a `content_type` and accepts it only if it is in
  `knownContentTypes` (`:67`); otherwise it warns and detects from the type
  field and the text.
  `TestDetectContentType_WhenMetadataNamesNoKnownType_ShouldDetectFromTheText`.
- Empty text was sent — **done, `fc857b3`.** Both engines return
  `ErrEmptyText` (`internal/embedding/engine.go:229`) before any request,
  naming the index in a batch. `TestEngines_RejectEmptyTextBeforeTheProvider`;
  the coverage test that pinned the old behaviour is now
  `TestOllamaEngine_Embed_WhenEmptyText_ShouldRefuseBeforeTheServer`.

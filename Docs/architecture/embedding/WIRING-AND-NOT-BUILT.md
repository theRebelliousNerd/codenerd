# embedding — WIRING-AND-NOT-BUILT

> Verified 2026-09-21 against branch `main` working tree.

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

- `GenAIEngine.EmbedBatchJob` (`internal/embedding/genai.go:330`): full async
  submission implementation, no caller found in the package or in search
  output. Treat as unwired until a caller is produced.
- `FindTopK` (`internal/embedding/engine.go:147`): the only verified callers
  are its own tests (`internal/embedding/engine_coverage_test.go:250` and
  following). No production call site was verified — the store ranks elsewhere.
- `DetectContentType` (`internal/embedding/task_selector.go:84`): verified
  callers are `GetOptimalTaskType` (`task_selector.go:186`) and its tests; no
  direct external caller was verified.

## Assumed but not done

- Gemini single-embed has no retry: `embedWithTask`
  (`internal/embedding/genai.go:98`) makes one API call and wraps failure
  (`genai.go:129`). Any retry story for the GenAI backend lives outside this
  package, if anywhere.
- Ollama batch is sequential (`internal/embedding/ollama.go:231`); a caller
  embedding N texts pays N round trips with no parallelism.
- `normalizeTaskType` (`internal/embedding/task_selector.go:30`) passes
  unknown task strings to the API untouched — an invalid task type fails
  server-side, not here.
- Metadata is trusted verbatim: `DetectContentType` returns any
  `content_type` metadata string unchecked (`task_selector.go:91`), so a
  misspelled kind silently becomes an unknown `ContentType` and selects
  `SEMANTIC_SIMILARITY` (`task_selector.go:74`).
- Neither engine rejects empty text client-side: `embedWithTask`
  (`internal/embedding/genai.go:98`) and `OllamaEngine.Embed`
  (`internal/embedding/ollama.go:81`) send what they are given.

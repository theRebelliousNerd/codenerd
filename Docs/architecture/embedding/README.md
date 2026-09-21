# embedding — README

> Verified 2026-09-21 against branch `main` working tree.
> `go build ./...` and `go test ./internal/embedding/` pass (see report).

`internal/embedding` is the package that turns text into vectors. It owns the
engine abstraction, two engine implementations (Google Gemini, local Ollama),
the content→task-type mapping for the Gemini API, and the cosine-similarity
helpers used to rank vectors.

## Source files (5 non-test files)

| File | Owns |
|---|---|
| `internal/embedding/engine.go:19` | `EmbeddingEngine` interface, `Config`, `DefaultConfig`, `NewEngine`, `FindTopK`, validators |
| `internal/embedding/genai.go:35` | `GenAIEngine` — Gemini API backend |
| `internal/embedding/ollama.go:33` | `OllamaEngine` — local Ollama backend |
| `internal/embedding/task_selector.go:13` | `ContentType`, `SelectTaskType`, `DetectContentType`, `GetOptimalTaskType` |
| `internal/embedding/math_generic.go:14`, `internal/embedding/math_amd64.go:15` | `CosineSimilarity`, defined once per file |

## Public surface

All constructors and methods below were checked against the cited lines.

| Symbol | Where | Notes |
|---|---|---|
| `Config` | `internal/embedding/engine.go:61` | Engine selection knobs |
| `DefaultConfig` | `internal/embedding/engine.go:78` | Defaults constructor |
| `NewEngine` | `internal/embedding/engine.go:93` | Builds the Ollama (`engine.go:108`) and GenAI (`engine.go:125`) engines |
| `EmbeddingEngine` | `internal/embedding/engine.go:19` | `Embed`, `EmbedBatch`, `Dimensions`, `Name`, `Close` |
| `TaskTypeAwareEngine` | `internal/embedding/engine.go:34` | Adds `EmbedWithTask` (`engine.go:37`) |
| `TaskTypeBatchAwareEngine` | `internal/embedding/engine.go:41` | Adds `EmbedBatchWithTask` (`engine.go:44`) |
| `HealthChecker` | `internal/embedding/engine.go:50` | Optional `HealthCheck`; only `OllamaEngine` implements it (`ollama.go:296`) |
| `NewGenAIEngine` | `internal/embedding/genai.go:42` | Requires API key (`genai.go:48`); defaults model to `gemini-embedding-001` (`genai.go:55`), task type to `SEMANTIC_SIMILARITY` (`genai.go:61`) |
| `GenAIEngine.Embed` | `internal/embedding/genai.go:89` | Delegates to `embedWithTask` (`genai.go:98`); one `EmbedContent` call, no retry loop in this function |
| `GenAIEngine.EmbedBatch` | `internal/embedding/genai.go:155` | Slices over `maxBatchSize = 100` (`genai.go:20`) are chunked and run under `errgroup` with `SetLimit(batchParallelism)` = 6 (`genai.go:200`, `genai.go:27`); chunk order is preserved via `chunkResults` (`genai.go:198`, `genai.go:226`) |
| `GenAIEngine.EmbedBatchJob` | `internal/embedding/genai.go:330` | Async batch submission; rejects empty input (`genai.go:335`) |
| `NewOllamaEngine` | `internal/embedding/ollama.go:44` | Local endpoint + model |
| `OllamaEngine.Embed` | `internal/embedding/ollama.go:81` | POSTs `ollamaEmbedRequest` to `/api/embeddings` (`ollama.go:119`); up to 3 attempts with 300ms doubling backoff (`ollama.go:95`, `ollama.go:96`) |
| `OllamaEngine.EmbedBatch` | `internal/embedding/ollama.go:233` | No native batch: calls `Embed` sequentially (`ollama.go:231`) |
| `OllamaEngine.EnsureModel` | `internal/embedding/ollama.go:329` | Resolve → prefer installed → `POST /api/pull` once per engine (four-step chain documented at `ollama.go:28`) |
| `OllamaEngine.HealthCheck` | `internal/embedding/ollama.go:296` | GETs `/api/tags` (`ollama.go:304`) |
| `SelectTaskType` | `internal/embedding/task_selector.go:36` | Pure `ContentType` × query-flag → task string; unknown input falls to `SEMANTIC_SIMILARITY` (`task_selector.go:74`) |
| `DetectContentType` | `internal/embedding/task_selector.go:84` | Metadata first (`task_selector.go:91`, `task_selector.go:97`), then heuristics, defaulting to conversation (`task_selector.go:178`) |
| `GetOptimalTaskType` | `internal/embedding/task_selector.go:183` | `DetectContentType` (`task_selector.go:186`) + query-side override (`task_selector.go:187`) + `SelectTaskType` (`task_selector.go:194`) |
| `FindTopK` | `internal/embedding/engine.go:147` | Cosine-rank top K; `k <= 0` means 10 (`engine.go:151`); dimension mismatches are skipped, not errors (`engine.go:161`) |
| `CosineSimilarity` | `internal/embedding/math_generic.go:14`, `internal/embedding/math_amd64.go:15` | One definition per file; exactly one compiles per target (the build is green, so the separation holds — check the build tags atop those files before editing) |

## Task-type strings actually emitted

These are the only strings `SelectTaskType` (`task_selector.go:36`) can return:
`CODE_RETRIEVAL_QUERY` (`:44`), `RETRIEVAL_DOCUMENT` (`:46`, `:56`, `:68`),
`RETRIEVAL_QUERY` (`:50`), `QUESTION_ANSWERING` (`:53`), `FACT_VERIFICATION`
(`:59`), `CLASSIFICATION` (`:62`), `CLUSTERING` (`:65`), `SEMANTIC_SIMILARITY`
(`:71`, `:74`). `normalizeTaskType` (`task_selector.go:30`) only uppercases
and trims — it does not validate, so anything outside this set passes through
to the API untouched.

## Where this is explained further

- `Docs/architecture/embedding/INTERNALS.md` — how each engine works.
- `Docs/architecture/embedding/WIRING-AND-NOT-BUILT.md` — what calls this
  package, what nothing calls, and what the code assumes but does not do.

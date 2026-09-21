# embedding — INTERNALS

> Verified 2026-09-21 against branch `main` working tree.

How the five source files of `internal/embedding` work. For who calls them,
see `WIRING-AND-NOT-BUILT.md`.

## GenAI engine (`internal/embedding/genai.go:35`)

- Single embed: `Embed` (`genai.go:89`) → `embedWithTask` (`genai.go:98`),
  one `Models.EmbedContent` call (`genai.go:124`) with `OutputDimensionality`
  forced to 3072 (`genai.go:115`). There is no retry loop in this path — a
  failed call returns a wrapped error (`genai.go:129`). The result vector is
  validated (`genai.go:142`) by `validateEmbeddingVector` (`engine.go:219`),
  which rejects empty and non-finite vectors.
- Batch: `embedBatchWithTask` (`genai.go:164`) sends ≤100 texts in one
  request (`genai.go:188`); larger slices are chunked (`genai.go:195`) and run
  in parallel under `errgroup` limited to 6 in flight (`genai.go:200`), with
  order preserved through `chunkResults` (`genai.go:198`, `genai.go:226`).
  Each chunk goes through `embedBatchChunk` (`genai.go:253`), which also pins
  3072 dimensions (`genai.go:260`) and validates the whole batch
  (`genai.go:290`) via `validateEmbeddingBatchResponse` (`engine.go:231`).
- `Dimensions` (`genai.go:300`) returns 3072; `Close` (`genai.go:310`) is a
  no-op.

## Ollama engine (`internal/embedding/ollama.go:33`)

- Single embed: `Embed` (`ollama.go:81`) first runs an `EnsureModel` preflight
  whose failure is non-fatal (`ollama.go:87`), then POSTs to `/api/embeddings`
  (`ollama.go:119`) with at most 3 attempts and 300ms doubling backoff
  (`ollama.go:95`, `ollama.go:96`). A 404 naming a missing model triggers
  `invalidateModel` + `EnsureModel` + one retry with the attempt budget reset
  (`ollama.go:162`); 5xx responses are retried (`ollama.go:177`); the decoded
  body is validated (`ollama.go:209`) before return.
- Batch (`ollama.go:233`) is sequential `Embed` calls — Ollama has no native
  batch endpoint (`ollama.go:231`).
- Model management: the four-step chain (resolve bare names → prefer an
  installed match → `POST /api/pull` once → ensure-and-retry on 404) is
  documented on the struct (`ollama.go:28`) and implemented by
  `resolveInstalledModel` (`ollama.go:514`),
  `preferInstalledEmbeddingModel` (`ollama.go:572`), `pullTargetFor`
  (`ollama.go:597`), `pullModel` (`ollama.go:442`), and `isModelNotFoundStatus`
  (`ollama.go:499`); once-per-engine state lives in `ensureMu`, `modelReady`,
  `pullAttempted` (`ollama.go:38`). The fallback default is
  `embeddinggemma:300m` (`ollama.go:23`).

## Task selection (`internal/embedding/task_selector.go`)

- `SelectTaskType` (`task_selector.go:36`) maps code→`CODE_RETRIEVAL_QUERY` /
  `RETRIEVAL_DOCUMENT` (`:44`, `:46`), queries→`RETRIEVAL_QUERY` (`:50`),
  questions→`QUESTION_ANSWERING` (`:53`), answers and docs→`RETRIEVAL_DOCUMENT`
  (`:56`), facts→`FACT_VERIFICATION` (`:59`), classification→`CLASSIFICATION`
  (`:62`), clustering→`CLUSTERING` (`:65`), conversation and anything
  unknown→`SEMANTIC_SIMILARITY` (`:71`, `:74`).
- `DetectContentType` (`task_selector.go:84`) trusts metadata first: a
  `content_type` key is returned verbatim with no validation (`:91`), then a
  `type` key is mapped through a fixed table (`:97`). Heuristics apply only
  when metadata says nothing: code on indicator score ≥ 3 (`:137`, `:149`),
  questions on prefix/suffix (`:155`), short informal text as conversation
  (`:163`), doc markers (`:169`), otherwise conversation (`:178`).
- `GetOptimalTaskType` (`task_selector.go:183`) detects (`:186`), then forces
  query-side content to `ContentTypeQuery` except code, classification, and
  clustering (`:187`), then selects (`:194`).

## Ranking and validation (`internal/embedding/engine.go:145`)

- `FindTopK` (`engine.go:147`) scores every corpus vector with
  `CosineSimilarity` (`engine.go:162`), skips mismatches with a warning
  (`engine.go:161`, `:175`), partially orders the head (`engine.go:181` — the
  comment at `:179` calls it a bubble sort; the loop is a head-selection), and
  truncates to K (`engine.go:191`).
- `validateEmbeddingVector` (`engine.go:219`) and
  `validateEmbeddingBatchResponse` (`engine.go:231`, also enforcing uniform
  dimensionality at `:244`) are the shared gates both engines call.

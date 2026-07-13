# 10 — Testing Alignment: embedding

> Last verified against codebase: 2026-07-13  
> Existing tests, gaps, commands

## 1. Commands

```powershell
# Unit + coverage tests (no live network required for main suite)
go test ./internal/embedding/...

# Verbose
go test -v ./internal/embedding/...

# Race detector (Ollama ensure mutex, GenAI errgroup)
go test -race ./internal/embedding/...

# Optional live GenAI benchmark (needs credentials + network)
# go test -bench=BenchmarkEmbedBatchParallel -benchtime=1x ./internal/embedding/

# SIMD cosine path (if toolchain supports)
# go test -tags simd ./internal/embedding/...
```

## 2. Test inventory

| File | ≈Lines | What it covers |
|------|-------:|----------------|
| `engine_coverage_test.go` | 448 | DefaultConfig fields; NewEngine unsupported/empty/ollama/genai-no-key; CosineSimilarity happy/mismatch/zero; FindTopK k defaults/order/skips |
| `ollama_coverage_test.go` | 486 | Mock HTTP for Embed success/fail/retry/404-pull; EmbedBatch; HealthCheck; Name/Dimensions |
| `ollama_ensure_test.go` | 176 | resolveInstalledModel, preferInstalledEmbeddingModel, pullTargetFor, modelBase, known families non-remap |
| `task_selector_coverage_test.go` | 409 | SelectTaskType matrix; DetectContentType metadata + heuristics; GetOptimalTaskType query remap |
| `task_selector_test.go` | 60 | Focused smaller cases |
| `genai_coverage_test.go` | 124 | Construct validation, Name/Dimensions, limited method surface without full live Embed |
| `genai_bench_test.go` | 44 | Parallel batch bench scaffold |

**Ratio:** test lines ≳ production lines — good for HTTP edge cases.

## 3. Alignment vs risk hotspots

| Hotspot | Covered? | Notes |
|---------|----------|-------|
| NewEngine provider switch | **Yes** | engine_coverage |
| Ollama retries / 404 pull | **Yes** | ollama_coverage + ensure |
| Known-family non-remap of test-model | **Yes** | ensure tests (protects custom names) |
| Cosine / FindTopK | **Yes** | engine_coverage |
| Task selector | **Yes** | large matrix |
| GenAI parallel errgroup order | **Partial** | bench + limited coverage; hard without mock SDK |
| GenAI EmbedBatchJob | **Weak / none** | experimental path |
| Live Ollama EnsureModel pull | **No CI** | would need daemon + network |
| SIMD cosine numerical parity | **No default** | tag-gated file |
| Integration with store reembed | **Outside package** | store tests use mocks |

## 4. Testing principles used

1. **No network by default** — Ollama uses `httptest` mock servers.
2. **Protect remaps** — arbitrary model names must not morph.
3. **Interface implementability** — store mocks prove base interface is sufficient.
4. **Error string stability** — unsupported provider message asserted exactly in at least one test.

## 5. Gaps (testing)

| Gap | Impact | Suggested approach |
|-----|--------|--------------------|
| No fake GenAI HTTP layer | Parallel batch regressions possible | Interface-wrap Models.EmbedContent or record/replay |
| EmbedBatchJob untested | Tools may break on SDK changes | Unit with stub Batches client if SDK allows |
| No integration test job | Real Ollama regressions slip | Optional Makefile target `test-embedding-live` |
| SIMD vs generic parity | Numeric drift if both diverge | Shared table-driven vectors under both tags |
| Factory health discard path | Boot asymmetry | Test in `internal/system` with mock HealthChecker |

## 6. Downstream test coupling

When changing interfaces:

1. Run `go test ./internal/embedding/...`
2. Run `go test ./internal/store/...` (mocks + reembed)
3. Run `go test ./internal/prompt/...` if AtomLoader signatures shift
4. Grep for `mock.*Engine` / `EmbeddingEngine` implementers

## 7. What “done” means for a change

| Change type | Minimum bar |
|-------------|-------------|
| Pure math | engine_coverage cosine cases green |
| Ollama ensure/pull logic | ollama_ensure + coverage green |
| Task types | task_selector matrices updated |
| GenAI batch | coverage or bench local verification + store batch tests |
| New provider | NewEngine tests + mock in store |

## 8. Non-goals for this package’s tests

- Full sqlite-vec E2E (belongs to store).
- TUI embedding command E2E (belongs to CLI).
- Constitutional policy tests (belongs to core).

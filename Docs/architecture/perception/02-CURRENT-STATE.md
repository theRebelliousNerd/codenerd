# 02 — Current State (perception)

> Last verified: **2026-07-13**  
> Source of truth: `internal/perception/` (+ `xaioauth/`)

## Inventory summary

| Metric | Approx value |
|--------|--------------|
| Package root non-test `.go` | ~50 |
| Package root `*_test.go` | ~48 |
| `xaioauth` sources | 13 |
| `xaioauth` tests | several (`chat_test`, `store_test`, `token_test`, `errors_test`) |
| Local `.mg` | 0 (consumes embedded core + policy) |
| Package README | `internal/perception/README.md` (operator notes; some defaults stale vs code) |
| Shared globals | `SharedTaxonomy`, `SharedSemanticClassifier` |

## Functional areas (status)

| Area | Hot files | Status |
|------|-----------|--------|
| LLM-first transduction | `understanding_adapter.go`, `transducer_llm.go` | Live, primary |
| Gemini thinking specialization | `transducer_gemini.go` | Live when thinking enabled |
| Verb corpus / regex | `transducer.go` | Live dual path |
| Semantic vector classify | `semantic_classifier.go` | Live if embed engine boots |
| Taxonomy Mangle | `taxonomy.go`, `taxonomy_persistence.go` | Live SharedTaxonomy |
| Learning / sleep cycle | `learning.go`, `consolidation.go` | Live async worker |
| Client factory | `client_factory.go` | Live |
| Provider clients | `client_*.go` | Live |
| CLI engines | `claude_cli_client.go`, `codex_*.go` | Live |
| SuperGrok OAuth | `xaioauth/` | Live |
| Tracing / metrics | `tracing_client.go`, `metrics.go` | Live |
| Transport pooling | `transport.go`, `scanner_pool.go` | Live |

## Exported / package-visible hotspots

### Construction

- `NewUnderstandingTransducer`, `NewLLMTransducer`, `NewGeminiThinkingTransducer`
- `NewSemanticClassifier`, `NewSemanticClassifierFromConfig`, `InitSharedSemanticClassifier`
- `NewTaxonomyEngine`, package `init` → `SharedTaxonomy`
- `NewClientFromEnv`, `NewClientFromConfig`, `NewClassificationClientFromConfig`
- `NewWorkerClientFromUserConfig`, `NewImageClientFromUserConfig`
- `NewTracingLLMClient`, `NewClaudeCodeCLIClient`, `NewCodexExecClient`
- `xaioauth.NewClientFromUserConfig`

### Kernel-facing

- `Intent.ToFact`, `FocusResolution.ToFact`
- `sanitizeFactArg`
- Semantic inject path inside `SemanticClassifier.Classify`
- `LLMTransducer.assertRoutingFacts` via `KernelAsserter`

### Types of record

- `Intent`, `Understanding`, `UnderstandingEnvelope`, `Routing`, `Signals`, `Scope`
- `SemanticMatch`, `SemanticConfig`, `CorpusEntry`
- `ProviderConfig`, provider configs, request/response DTOs
- `ReasoningTrace`, `LLMMetrics`
- `RateLimitError` (Claude CLI)

## Test surface (package)

| Kind | Examples |
|------|----------|
| Unit / factory | `client_factory_test.go`, `*_test.go` per client |
| Classifier / taxonomy | `semantic_classifier_test.go`, `taxonomy_test.go` |
| Transducer | `transducer_*_test.go`, `understanding_adapter_*_test.go` |
| Adversarial / break | `break_test.go` (JSON extract, injection, UTF-8, races) |
| Transient LLM | `understanding_adapter_transient_test.go` |
| Benchmarks | `benchmark_test.go` |
| Assault verb | `assault_verb_test.go` |
| Live (gated) | `*_live_test.go`, `xai_torture_test.go`, `zai_live_test.go` |

## Known behavioral realities (not bugs by default)

1. `ParseIntentWithContext` returns **nil error** with degraded Intent on LLM failure.  
2. Semantic path is **optional enhancement** when classifier/engine nil.  
3. Classification client may be **nil** → callers use main client.  
4. `validate()` on Understanding vocabulary is **not** on the success hot path (comments: dead work removed).  
5. Package README default models may lag factory code.

## Line-count leaders

See [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) §3.2.

## How this sits in the monorepo

Heavy importers: `cmd/nerd/chat`, `cmd/nerd` commands, `internal/session`, e2e tests.  
Heavy callees: `config`, `core`, `embedding`, `logging`, `types`, `articulation` (aliases/schema), `store`, `mangle`, `sqlpragmas`.

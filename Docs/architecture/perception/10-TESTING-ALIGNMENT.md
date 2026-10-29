# 10 — Testing Alignment (perception)

> Last verified: **2026-07-13**

## Commands

```powershell
# Unit + package tests
go test ./internal/perception/...

# Subpackage
go test ./internal/perception/xaioauth/...

# Adversarial / break suite (subset of package tests)
go test ./internal/perception/ -run Break -count=1

# Benchmarks
go test ./internal/perception/ -bench=. -benchmem -run=^$

# E2E contracts (module root)
go test ./tests/e2e/ -run Perception -count=1
```

Live provider tests (`*_live_test.go`, torture tests) typically require API keys and are environment-gated—do not assume CI runs them.

## Existing coverage map

| Area | Tests (examples) | Strength |
|------|------------------|----------|
| Factory / engines | `client_factory_test.go`, `client_factory_extra_test.go` | Strong |
| Ollama wiring | `client_ollama_test.go` | Strong |
| Gemini HTTP/retry/schema | `client_gemini_*_test.go` | Strong |
| ZAI complete/retry | `client_zai_test.go`, `client_zai_retry_test.go` | Strong |
| OpenAI HTTP | `client_openai_http_test.go` | Medium |
| Claude CLI parse | `claude_cli_client_test.go` | Strong (no live CLI required for unit) |
| Codex probe | `codex_cli_probe_test.go` | Strong |
| Transducer LLM parse | `transducer_llm_test.go`, extras | Strong |
| Understanding adapter | `understanding_adapter_*_test.go` | Strong |
| Transient ErrLLMUnavailable | `understanding_adapter_transient_test.go` | Strong |
| Semantic classifier | `semantic_classifier_test.go` | Medium–strong |
| Taxonomy | `taxonomy_test.go`, `taxonomy_extra_test.go`, persistence tests | Strong |
| Break / adversarial | `break_test.go` | **Excellent** |
| Assault verb | `assault_verb_test.go` | Focused |
| Tracing | `tracing_client_*_test.go` | Strong |
| Transport pool | `transport_pool_test.go` | Present |
| xaioauth | store/token/errors/chat tests | Medium–strong |
| E2E perception | `tests/e2e/perception_*` | Contract-level |

## Contract tests that matter

From e2e packages (names):

- `perception_contract_e2e_test.go` — Intent shape / stability contracts  
- `perception_stateful_e2e_test.go` — multi-turn / history  
- `perception_adversarial_e2e_test.go` — hostile inputs  

These use `NewUnderstandingTransducer` with mock clients—verify **adapter mapping**, not provider networks.

## Gaps

| Gap | Risk | Suggested test |
|-----|------|----------------|
| Non-Gemini clients wrapping `ErrLLMUnavailable` | Wrong clarification UX | Unit inject 503 → errors.Is sentinel |
| Boot wiring: classification client selection | Latency regressions | Integration boot assertion |
| SharedSemanticClassifier nil path in chat | Silent quality drop | Boot smoke with embed disabled |
| Full multi-provider tools parity | Runtime type assert panics | Interface compliance table test |
| Live schema strictness drift vs articulation | Tool call failures | Golden schema equality test (partially via parse of Piggyback constant) |
| Race under parallel Classify + learn | Flaky production | Stress go test -race package |

## Alignment with north star

- Break tests protect **logic executive** from poisoned fact args.  
- Transient failure tests protect **honest signalling**.  
- Contract e2e protects **transduction interface** stability for session/kernel.

## Recommended pre-handoff bar (for perception code changes)

1. `go test ./internal/perception/...`  
2. If touching JSON parse / sanitize: `-run Break`  
3. If touching factory: factory + ollama tests  
4. If changing Intent fields: e2e perception contracts  

## Coverage philosophy

Prefer:

- deterministic HTTP test servers over live keys  
- property/break tests for parsers  
- mock `LLMClient` for transducer logic  

Avoid:

- asserting exact surface prose of models  
- requiring network for default `go test`

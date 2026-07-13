# perception — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/perception/` (complete internal coverage)
> **Implementation: `internal/perception/` — 50 non-test .go, 48 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/perception/` (exists; 50 non-test Go files)
- 1:1 mapping: `Docs/architecture/perception/` ↔ `internal/perception/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/perception/semantic_classifier.go` | 1254 | source |
| `internal/perception/client_zai.go` | 1041 | source |
| `internal/perception/transducer_llm.go` | 949 | source |
| `internal/perception/tracing_client.go` | 946 | source |
| `internal/perception/client_gemini.go` | 944 | source |
| `internal/perception/taxonomy.go` | 799 | source |
| `internal/perception/understanding_adapter.go` | 725 | source |
| `internal/perception/client_anthropic.go` | 686 | source |
| `internal/perception/client_types.go` | 624 | source |
| `internal/perception/transducer.go` | 616 | source |
| `internal/perception/codex_cli_client.go` | 596 | source |
| `internal/perception/claude_cli_client.go` | 576 | source |
| `internal/perception/client_openai.go` | 505 | source |
| `internal/perception/client_openrouter.go` | 441 | source |
| `internal/perception/client_factory.go` | 425 | source |
| `internal/perception/client_gemini_files.go` | 403 | source |
| `internal/perception/client_gemini_tools.go` | 400 | source |
| `internal/perception/learning.go` | 400 | source |
| `internal/perception/client_gemini_streaming.go` | 354 | source |
| `internal/perception/client_zai_streaming.go` | 308 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/perception/claude_cli_client.go` | 576 |
| `internal/perception/client.go` | 17 |
| `internal/perception/client_anthropic.go` | 686 |
| `internal/perception/client_factory.go` | 425 |
| `internal/perception/client_gemini.go` | 944 |
| `internal/perception/client_gemini_files.go` | 403 |
| `internal/perception/client_gemini_streaming.go` | 354 |
| `internal/perception/client_gemini_tools.go` | 400 |
| `internal/perception/client_ollama.go` | 134 |
| `internal/perception/client_openai.go` | 505 |
| `internal/perception/client_openrouter.go` | 441 |
| `internal/perception/client_schema.go` | 87 |
| `internal/perception/client_tool_helpers.go` | 202 |
| `internal/perception/client_types.go` | 624 |
| `internal/perception/client_xai.go` | 242 |
| `internal/perception/client_zai.go` | 1041 |
| `internal/perception/client_zai_retry.go` | 135 |
| `internal/perception/client_zai_streaming.go` | 308 |
| `internal/perception/codex_cli_client.go` | 596 |
| `internal/perception/codex_cli_probe.go` | 212 |
| `internal/perception/codex_exec_client.go` | 15 |
| `internal/perception/consolidation.go` | 99 |
| `internal/perception/debug.go` | 16 |
| `internal/perception/learning.go` | 400 |
| `internal/perception/metrics.go` | 52 |
| `internal/perception/scanner_pool.go` | 56 |
| `internal/perception/semantic_classifier.go` | 1254 |
| `internal/perception/taxonomy.go` | 799 |
| `internal/perception/taxonomy_persistence.go` | 125 |
| `internal/perception/tracing_client.go` | 946 |
| `internal/perception/transducer.go` | 616 |
| `internal/perception/transducer_gemini.go` | 187 |
| `internal/perception/transducer_llm.go` | 949 |
| `internal/perception/transport.go` | 33 |
| `internal/perception/understanding.go` | 230 |
| `internal/perception/understanding_adapter.go` | 725 |
| `internal/perception/utils.go` | 23 |
| `internal/perception/xaioauth/auth_device.go` | 236 |
| `internal/perception/xaioauth/chat.go` | 171 |
| `internal/perception/xaioauth/client.go` | 107 |
| `internal/perception/xaioauth/config.go` | 154 |
| `internal/perception/xaioauth/doc.go` | 36 |
| `internal/perception/xaioauth/errors.go` | 86 |
| `internal/perception/xaioauth/grok_auth_import.go` | 109 |
| `internal/perception/xaioauth/probe.go` | 105 |
| `internal/perception/xaioauth/store.go` | 73 |
| `internal/perception/xaioauth/streaming.go` | 27 |
| `internal/perception/xaioauth/token.go` | 221 |
| `internal/perception/xaioauth/tools.go` | 124 |
| `internal/perception/xaioauth/transport.go` | 84 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/perception/transducer_coverage_test.go` | 2080 |
| `internal/perception/xai_torture_test.go` | 1617 |
| `internal/perception/semantic_classifier_test.go` | 876 |
| `internal/perception/tracing_client_test.go` | 837 |
| `internal/perception/break_test.go` | 820 |
| `internal/perception/transducer_unit_test.go` | 820 |
| `internal/perception/transducer_llm_test.go` | 662 |
| `internal/perception/gemini_live_test.go` | 491 |
| `internal/perception/gemini_structured_test.go` | 412 |
| `internal/perception/understanding_adapter_test.go` | 395 |

## 5. Behavior summary

Package **perception** is a living codeNERD subsystem: NL→atoms transduction, semantic classification, LLM clients.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.

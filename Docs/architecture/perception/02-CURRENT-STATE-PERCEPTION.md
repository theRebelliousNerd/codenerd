# perception — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/perception/` (49 non-test .go, 47 tests, 0 .mg)**


## 1. Source location

- Primary package: `internal/perception/` (**exists** with 49 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

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
| `internal/perception/client_types.go` | 623 | source |
| `internal/perception/transducer.go` | 616 | source |
| `internal/perception/codex_cli_client.go` | 596 | source |
| `internal/perception/claude_cli_client.go` | 576 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/perception/transducer_coverage_test.go` | 2080 |
| `internal/perception/xai_torture_test.go` | 1617 |
| `internal/perception/semantic_classifier_test.go` | 876 |
| `internal/perception/tracing_client_test.go` | 837 |
| `internal/perception/break_test.go` | 820 |
| `internal/perception/transducer_unit_test.go` | 820 |

## 4. Current behavior (summary)

Package **perception** is a living codeNERD subsystem: NL→atoms transduction, semantic classifier, LLM clients.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (85%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.

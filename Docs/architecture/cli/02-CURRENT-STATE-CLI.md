# cli — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `cmd/nerd/` (113 non-test .go, 55 tests, 2 .mg)**


## 1. Source location

- Primary package: `cmd/nerd/` (**exists** with 113 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `cmd/nerd/chat/process.go` | 1195 | source |
| `cmd/nerd/chat/session_boot.go` | 1101 | source |
| `cmd/nerd/cmd_campaign.go` | 1101 | source |
| `cmd/nerd/chat/review_aggregator.go` | 1070 | source |
| `cmd/nerd/chat/model_update.go` | 1048 | source |
| `cmd/nerd/ui/splitpane.go` | 1009 | source |
| `cmd/nerd/chat/multistep_corpus.go` | 992 | source |
| `cmd/nerd/chat/model_session_context.go` | 945 | source |
| `cmd/nerd/chat/commands_handlers.go` | 904 | source |
| `cmd/nerd/chat/northstar_wizard.go` | 868 | source |
| `cmd/nerd/chat/model_handlers.go` | 810 | source |
| `cmd/nerd/chat/multistep_decomposer.go` | 801 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `cmd/nerd/chat/helpers_test.go` | 1760 |
| `cmd/nerd/chat/live_integration_test.go` | 1673 |
| `cmd/nerd/chat/tui_frame_contract_e2e_test.go` | 992 |
| `cmd/nerd/chat/testutil_test.go` | 952 |
| `cmd/nerd/chat/session_adapters_test.go` | 932 |
| `cmd/nerd/chat/commands_test.go` | 885 |

## 4. Current behavior (summary)

Package **cli** is a living codeNERD subsystem: CLI entrypoints, chat TUI, campaign and system commands.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (70%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.

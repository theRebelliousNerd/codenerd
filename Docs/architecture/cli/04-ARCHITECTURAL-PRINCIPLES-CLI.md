# 04 — Architectural Principles: CLI

> Binding design principles for `cmd/nerd`. Violations should be called out in review.

## P1 — Logic owns execution; CLI owns presentation

The CLI may **request** actions and display results. Permission, planning, and sequencing belong to Mangle-derived `next_action` / `permitted` and VirtualStore dispatch, not to ad-hoc `os/exec` sprinkled through views.

## P2 — Two doors, one house

Cobra subcommands and chat slash commands are two UX doors into the **same** Cortex. Prefer shared runners over duplicated business logic. When parity is intentionally broken, document it in `05-COMMAND-ARCHITECTURE.md`.

## P3 — Quiescent multi-turn boot

Interactive sessions boot Cortex once (`session_boot` / `GetOrBootCortex` patterns) and reuse it. Do not re-init sqlite, embeddings, and system shards per keystroke.

## P4 — Fail closed on workspace and timeouts

- Honor `--workspace` (including chat `Chdir`).
- Honor `--timeout` / config execution timeout.
- Surface permission / missing-directory errors with actionable messages (`main.go` RunE).

## P5 — Panic isolation in TUI goroutines

Chat input processing runs in Bubble Tea cmds/goroutines. Recover panics to `errorMsg` and keep the UI alive (`chat/process.go` documented recover). Never let a model bug hard-kill the process if avoidable.

## P6 — Observability is product, not afterthought

Operators get:

- categorized file logs (`.nerd/logs/`)
- glass box / transparency / reflection commands
- activity pulse in the TUI

Adding a major CLI feature without a log category or glass-box event is incomplete.

## P7 — JIT-first LLM text

New LLM-facing behavior should land as prompt atoms + selection, not by growing monolithic strings inside chat handlers. Align with `internal/prompt` and `Docs/architecture/prompt/`.

## P8 — CGO/sqlite honesty

Release builds that need vector features must document CGO flags. CLI features depending on sqlite-vec must degrade or error clearly when headers/build tags are missing.

## P9 — Interactive meta-loop discipline

Direct-action `--interactive` mode uses explicit meta-commands (`refine`, `redo`, `approve`, `quit`, `help` in `cmd_interactive.go`). Do not invent silent auto-loops without user control.

## P10 — No time/cost estimates in architecture docs

Roadmaps use dependency gates and ordering only (project-wide rule shared with arch-propose).

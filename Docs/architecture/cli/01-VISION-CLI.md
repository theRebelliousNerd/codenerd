# 01 — Vision: codeNERD CLI / TUI

> Last verified: 2026-07-13  
> Target state for the CLI product surface (not every line is fully realized — see gap analysis).

## 1. Product identity

`nerd` is a **logic-first coding agent CLI**:

- **Interactive by default**: bare `nerd` opens the Bubble Tea chat (primary UX).
- **Scriptable second**: Cobra subcommands for CI, automation, and focused verbs (`run`, `scan`, `query`, `campaign`, …).
- **Portable**: one binary, per-project `.nerd/` state, sqlite-vec enabled builds for embedding stores.

Tagline from root command long help (`cmd/nerd/main.go`):

> Architecture: Logic determines Reality; the Model merely describes it.

## 2. Experience principles

1. **Quiescent boot** — assemble Cortex once; keep it warm across multi-turn chat.
2. **Glass box by default when debugging** — operators can see tool/shard/kernel activity without leaving the TUI.
3. **Two doors, one house** — Cobra verbs and slash commands should map to the same intents and shard/kernel pathways.
4. **Campaigns as long horizon** — multi-phase work (including assault/soak) is a first-class CLI concern, not a side script.
5. **Auth is multi-engine** — Claude CLI, Codex CLI, Grok/API paths coexist under `nerd auth`.

## 3. Target architecture (logical)

```
┌──────────────────────────────────────────────────────────────┐
│                     nerd binary (Cobra)                      │
│  global: --verbose --api-key --workspace --timeout           │
└───────────────┬──────────────────────────────┬───────────────┘
                │ default RunE                 │ subcommands
                ▼                              ▼
     ┌─────────────────────┐        ┌─────────────────────┐
     │ chat.RunInteractive │        │ run / scan / query  │
     │ Bubble Tea Model    │        │ campaign / auth …   │
     └──────────┬──────────┘        └──────────┬──────────┘
                │                              │
                └──────────────┬───────────────┘
                               ▼
                    ┌────────────────────┐
                    │ Cortex / system    │
                    │ boot (session_*)   │
                    └─────────┬──────────┘
          perception │ kernel │ shards │ VirtualStore │ articulation
```

## 4. Non-goals for the CLI package

- Hosting the Mangle engine implementation (that is `internal/mangle` + `internal/core`).
- Being the long-term home for domain shard business logic (comments in `session_boot.go` document migration toward JIT prompt atoms).
- Replacing `Docs/Spec` product templates.
- Implementing every Vectryx-style dual flag/Cobra client SDK pattern — codeNERD is agent-native, not primarily a remote DB client.

## 5. Success metrics (measurable)

| Metric | Direction | How measured |
|--------|-----------|--------------|
| Boot success rate | ↑ | Boot logs / chat start without errorMsg |
| Time-to-first-token/action | ↓ | Activity pulse / glass box timestamps |
| Panic recovery rate | 100% of chat goroutine panics → errorMsg | `process.go` recover path |
| Command discovery | Users find verbs via `--help` and `/help` | Help surfaces complete |
| Test line ratio on chat hot paths | ↑ | `go test` coverage on process/session_boot |

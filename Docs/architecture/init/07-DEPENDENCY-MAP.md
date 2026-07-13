# init — Dependency Map

> Last verified: 2026-07-13

## Upstream (init imports)

| Package | Usage in init |
|---------|----------------|
| `internal/config` | Default/user config, Context7 auto-detect, embedding settings, `UserConfig` defaults on first create |
| `internal/core` | `NewRealKernel`, `LoadFacts`, `Assert` (doc facts), workspace set |
| `internal/core/shards` | `ShardManager`, `StartSystemShards`, `DefineProfile` |
| `internal/embedding` | Engine for sqlite-vec KBs |
| `internal/logging` | Boot/store categories, timers |
| `internal/northstar` | Create northstar knowledge store schema at init |
| `internal/perception` | `LLMClient` interface only (via InitConfig) |
| `internal/prompt` | JIT compiler, corpus DB, `ReloadAllPrompts` |
| `internal/shards` | Import alias in initializer for system shard registration path |
| `internal/sqlpragmas` | Validation DB opens |
| `internal/store` | LocalStore, migrations, knowledge/prompt atoms |
| `internal/tools` | Registry for research tools |
| `internal/tools/research` | Context7 + grounding helper registration |
| `internal/types` | `ShardConfig`, permissions, model capability |
| `internal/world` | Scanner, ScanResult, facts conversion |
| `github.com/mattn/go-sqlite3` | Validation opens |

**Does not import:** `cmd/nerd` (CLI depends on init), domain shards (`researcher`, `tool_generator` removed), `session` executor (opposite direction for deep research vision).

## Downstream (who imports init)

Evidence from grep `codenerd/internal/init`:

| Consumer | Usage |
|----------|--------|
| `cmd/nerd/cmd_init_scan.go` | Primary: `DefaultInitConfig`, `NewInitializer`, `Initialize`, `IsInitialized`, `CleanupBackups` |
| `cmd/nerd/cmd_sessions.go` | Session helpers |
| `cmd/nerd/cmd_query.go` | `IsInitialized` gate |
| `cmd/nerd/chat/session_persistence.go` | `SessionState`, `Load/SaveSessionState`, history |
| `cmd/nerd/chat/session_boot_helpers.go` | Init package types/helpers during boot |
| `cmd/nerd/chat/helpers.go` / `helpers_scan.go` | Workspace/init utilities |
| `cmd/nerd/chat/model_types.go` | Type references |
| `cmd/nerd/chat/commands_tools.go` | Status: initialized flag |
| `cmd/nerd/chat/commands_handlers_analysis.go` | Init/force style gates |
| `cmd/nerd/chat/commands_handlers_files.go` | File/workspace helpers |
| `cmd/nerd/chat/session_functions_test.go` | Tests |

**No other `internal/*` packages** currently import init as a library dependency (init is CLI/chat edge + persistence helpers).

## Dependency direction diagram

```
perception.LLMClient ──┐
config / embedding ────┤
core + core/shards ────┼──► internal/init ──► cmd/nerd (init/scan)
store / prompt ────────┤                 └──► cmd/nerd/chat (session I/O)
world.Scanner ─────────┤
tools/research ────────┘
```

## Coupling risks

1. **Heavy package**: init pulls core kernel + shards + embedding + prompt — expensive for light `IsInitialized` callers (acceptable; function is file-stat only and does not construct Initializer).
2. **Chat dual-use of session types**: session persistence lives in init historically; long-term may belong in `session` package (open question).
3. **Tool catalog vs tools runtime**: JSON catalog is descriptive; actual tool execution is VirtualStore/tools packages.

## Transitive operator requirements

| Capability | Required for |
|------------|--------------|
| CGO sqlite + sqlite-vec headers | Full KB create with embeddings |
| LLM API key (`ZAI_API_KEY` etc.) | Strategic knowledge, some enrichment |
| Context7 key | Doc research quality |
| Ollama/GenAI per config.json | Embedding engine |

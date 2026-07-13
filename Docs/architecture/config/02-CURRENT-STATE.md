# 02 — Current State: `internal/config`

> Last verified: 2026-07-13  
> Counts are approximate from source inspection; re-run `Get-ChildItem` / `wc` if exact CI metrics needed.

## 1. Package metrics

| Metric | Value |
|--------|------:|
| Non-test `.go` files | 17 |
| Test `.go` files | 5 |
| Mangle `.mg` | 0 |
| Approx. non-test LOC | ~3,100 |
| Approx. test LOC | ~1,600 |
| Primary on-disk format | JSON (UserConfig) |
| Secondary format | YAML (Config) |

## 2. File roles

| File | Role | Hotspot? |
|------|------|----------|
| `user_config.go` | UserConfig, workspace root, all Get*, engines, features install | **Yes** — density center |
| `config.go` | YAML Config, DefaultConfig, Load/Save, env overrides, Validate | **Yes** — dual path |
| `llm.go` | Engine/provider structs (Claude/Codex/xAI OAuth/Gemini) | Medium |
| `llm_timeouts.go` | Global timeout singleton + presets | Medium |
| `limits.go` | CoreLimits, APISchedulerPolicy | Medium |
| `memory.go` | Context window, embedding, memory shard config | Medium |
| `jit.go` | JIT compiler knobs + bool-set tracking | Medium |
| `shard.go` | Per-shard profiles | Low |
| `execution.go` | Tactile allowlists | Low |
| `integrations.go` | MCP server map bridge | Medium |
| `reflection.go` | System-2 reflection recall | Low |
| `world.go` | Scan workers / ignore | Low |
| `logging.go` | Category-gated logging | Low |
| `ux.go` | Onboarding, transparency, guidance | Medium |
| `build.go` | CGO/build env | Low |
| `tool_generation.go` | Ouroboros OS/arch | Low |
| `mangle.go` | Schema/policy paths on YAML Config | Low |

## 3. Hotspots (behavioral)

### 3.1 GetActiveProvider

Explicit provider vs priority-key fallback; ollama sentinel key. Most auth/boot bugs surface here.

### 3.2 FindWorkspaceRoot

go.mod-first walk. Session and DefaultUserConfigPath depend on this; wrong root ⇒ wrong DB/config.

### 3.3 GetEffectiveAPISchedulerPolicy

Merges core concurrency, engine caps, subscription defaults, and pointer overrides. Campaign + session boot depend on it.

### 3.4 Default drift pairs

| Concept | UserConfig Get* default | YAML DefaultConfig default |
|---------|-------------------------|----------------------------|
| MaxConcurrentShards | 12 | 4 |
| Execution DefaultTimeout | 30s | 10m |
| Context MaxTokens | 200000 | 128000 (Memory.ContextWindow in defaultMemoryConfig) |

## 4. What is solid today

- Multi-engine model (api / claude-cli / codex-cli / xai-oauth) with sensible CLI sandbox defaults.
- Worker vs image isolation for Nano Banana / Gemini image.
- Comprehensive tests for load/save/validate/provider/engine/scheduler/workspace.
- Feature flag install path without forcing core → config import cycles.
- Embedding model canonicalization (`embeddinggemma:300m`).

## 5. What is partial

- Full UserConfig schema validation (engine validated; many nested structs only “defaulted”).
- Env override parity on JSON path.
- Deprecation story for YAML Config.
- UIConfig not fully folded into UserConfig.
- No package-level agents.md (guidance lives in this corpus + root AGENTS.md).

## 6. Inventory of exported type families

See [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) for the full export catalog.

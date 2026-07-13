# 05 — Command Architecture (Cobra)

> Source of truth: `cmd/nerd/main.go` `init()` + per-file `cobra.Command` definitions.  
> Verified: 2026-07-13

## 1. Root command

| Field | Value |
|-------|-------|
| Use | `nerd` |
| Short | codeNERD - Logic-First CLI Agent (Cortex 1.5.0) |
| Default RunE | Interactive chat |
| PersistentPreRunE | Zap logger + `logging.Initialize` (skipped pure interactive path nuances) |
| PersistentPostRun | logger.Sync + `logging.CloseAll` |

## 2. Command tree (registered)

### Core lifecycle

| Command | File | Purpose |
|---------|------|---------|
| `nerd` (default) | `main.go` | Launch TUI |
| `init` | `cmd_init_scan.go` | Create/migrate `.nerd/` |
| `scan` | `cmd_init_scan.go` | Refresh codebase index / facts |
| `run [instruction]` | `cmd_instruction.go` | Single-shot OODA |
| `status` | `cmd_query.go` | System status |
| `query [predicate]` | `cmd_query.go` | Query kernel facts |
| `why [predicate]` | `cmd_query.go` | Derivation explanation |
| `logs` | `cmd_logs.go` | Aggregate errors/warnings |
| `test-context` | `cmd_test_context.go` | Context system validation |

### Agents & spawn

| Command | File |
|---------|------|
| `define-agent` | `cmd_spawn.go` |
| `spawn [shard-type] [task]` | `cmd_spawn.go` |
| `agents` | `cmd_advanced.go` |

### Direct action verbs (TUI mirrors)

| Command | File | Interactive `-i` |
|---------|------|------------------|
| `review <target>` | `cmd_direct_actions.go` | yes |
| `fix <target>` | same | yes |
| `test <target>` | same | yes |
| `explain <target>` | same | yes |
| `create <description>` | same | yes |
| `refactor <target>` | same | yes |
| `security <target>` | same | |
| `analyze <target>` | same | |
| `perception <input>` | same | |
| `push` / `commit` | same | git helpers |

### Advanced / speculative

| Command | File | Purpose |
|---------|------|---------|
| `dream <scenario>` | `cmd_advanced.go` | Multi-agent consultation without execute |
| `shadow <action>` | same | Simulate without side effects |
| `whatif <change>` | same | Counterfactuals |
| `logic [predicate]` | same | Dump kernel facts |
| `tool <list\|run\|info\|generate>` | same | Ouroboros tools |
| `jit` | same | JIT compiler status |

### Campaign

| Command | File |
|---------|------|
| `campaign start\|status\|pause\|resume\|list` | `cmd_campaign.go` |

Flags on start: `--docs`, `--type` (`greenfield|feature|audit|migration|remediation`).

### Auth

| Command | File |
|---------|------|
| `auth claude\|codex\|grok\|status` | `cmd_auth.go` |

### Browser

| Command | File |
|---------|------|
| `browser launch\|session\|snapshot` | `cmd_browser.go` |

### Mangle tooling

| Command | File |
|---------|------|
| `check-mangle [files...]` | `cmd_mangle_check.go` |
| `mangle-lsp` | `cmd_mangle_lsp.go` |

### Systems visibility

| Command | File |
|---------|------|
| `mcp list\|tools\|status` | `cmd_systems.go` |
| `autopoiesis status\|learning\|tools` | `cmd_systems.go` |
| `memory status` | `cmd_systems.go` |

### Transparency

| Command | File |
|---------|------|
| `glassbox` | `cmd_transparency.go` |
| `transparency` | `cmd_transparency.go` |
| `reflection` | `cmd_transparency.go` |

### Sessions & knowledge

| Command | File |
|---------|------|
| `sessions list\|load` | `cmd_sessions.go` |
| `knowledge list\|search` | `cmd_knowledge.go` |

### North star

| Command | File |
|---------|------|
| `northstar show\|summary\|query\|facts\|export\|stats` | `cmd_northstar.go` |

### CodeDOM

| Command | File |
|---------|------|
| `dom demo\|inspect\|get\|edit\|apply\|replace` | `dom_*.go` |

### Embeddings

| Command | File |
|---------|------|
| `embedding set\|stats\|reembed` | `embedding_cmd.go` |

## 3. Dispatch pattern

Most non-chat commands:

1. Parse flags / args via Cobra.
2. Resolve workspace + API key.
3. Boot Cortex (`internal/system` helpers) or open lightweight stores.
4. Invoke internal packages.
5. Print human-readable output (and sometimes structured data).

Interactive direct actions may enter `runInteractiveAction` (`cmd_interactive.go`) for refine/redo/approve loops with a 30m session timeout and signal handling.

## 4. Known intentional asymmetries

| Feature | Cobra | Chat slash | Notes |
|---------|-------|------------|-------|
| Multistep corpus UX | limited | deep | chat-owned |
| Config wizard | partial | `/config` wizard | TUI-first |
| Assault campaigns | campaign cmd | `/campaign assault …` | chat docs emphasize assault |
| File ops patch/edit | limited | `/patch` `/edit` `/write` | chat handlers |

## 5. Extension guide

Adding a Cobra command:

1. Define `var fooCmd = &cobra.Command{...}` in the appropriate `cmd_*.go`.
2. Implement `RunE` with context + timeouts.
3. `rootCmd.AddCommand` (or parent.AddCommand) in `main.go` or file `init`.
4. Mirror slash command if it is a primary user verb (`chat/commands.go` + handlers).
5. Add tests under `cli_test.go` or dedicated `*_test.go`.
6. Update this document’s tree table.

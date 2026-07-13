# Workflow: Full CLI Surface Matrix

## What It Stresses

One-shot CLI entry points end-to-end (not interactive TUI). Use a dedicated workspace so tool writes stay isolated.

## Command Catalog (live 2026-07)

### Diagnostics / glass-box

| Command | Notes |
|---------|-------|
| `nerd status` | Boot smoke |
| `nerd agents` | Shard registry |
| `nerd jit` | JIT compiler status |
| `nerd glassbox` | Kernel transparency |
| `nerd transparency` | Explainability |
| `nerd reflection` | System 2 memory |
| `nerd embedding stats` | Embedding engine |
| `nerd sessions` | Session list |
| `nerd logs` | Aggregate errors |
| `nerd mcp` / `nerd mcp list` | MCP servers |
| `nerd memory` | Memory tiers |
| `nerd logic` | Kernel facts dump |
| `nerd query <pred>` | Query facts (predicate only, not rule text) |
| `nerd knowledge` | Knowledge base |
| `nerd why` | Why last action |
| `nerd autopoiesis` | Self-mod status |
| `nerd campaign list` | Campaign inventory |

### Workspace / world

| Command | Notes |
|---------|-------|
| `nerd init` | Initialize `.nerd/` |
| `nerd scan` | Codebase index → `scan.mg` |
| `nerd northstar` | Vision/requirements |

### Coding / shards

| Command | Notes |
|---------|-------|
| `nerd create <desc>` | Coder shard write path |
| `nerd spawn <type> <task>` | coder/tester/reviewer/researcher/… |
| `nerd run <instruction>` | Full OODA |
| `nerd fix <desc>` | Fix path |
| `nerd refactor <target>` | Refactor |
| `nerd test <target>` | Tester path |
| `nerd analyze <path>` | Researcher analysis |
| `nerd explain <path>` | Explain code |
| `nerd review <path>` | Reviewer |
| `nerd security <path>` | Security analysis |

### Simulation / advanced

| Command | Notes |
|---------|-------|
| `nerd shadow <action>` | Simulate without side effects |
| `nerd whatif <change>` | Counterfactual |
| `nerd dream <question>` | Multi-agent consult |
| `nerd campaign start <goal>` | Long-running campaign |
| `nerd browser …` | Rod browser automation |
| `nerd tool list\|info\|generate\|run` | Ouroboros tools (**subcommand required**) |
| `nerd check-mangle <file.mg>` | Mangle validate (**file arg required**) |
| `nerd define-agent <name> <desc>` | Custom specialist |
| `nerd test-context` | Context system (may fail CortexKernel type assert) |
| `nerd perception <text>` | Perception diagnostic |

### Image (Nano Banana 2)

| Path | Notes |
|------|-------|
| `nerd spawn image_generator …` | Must use Gemini image model, **not** Ollama worker |
| Config `image.model` | `gemini-3.1-flash-image` |

## Known Failure Modes (live matrix)

| Symptom | Cause | Fix / Mitigate |
|---------|-------|----------------|
| Files land in monorepo not `-w` | Missing `CODENERD_WORKSPACE_ROOT` | Fixed: `resolveWorkspaceRoot` sets env |
| Process hangs after `Result:` | Maintenance loop not cancelled on `Close` | Fixed: `maintenanceCancel` in `Cortex.Close` |
| SuperGrok `invalid_grant` | Revoked OAuth refresh | `nerd auth grok` or `engine=api` + `xai_api_key` |
| `tool` / `check-mangle` exit 1 | Missing required args | Pass subcommand / file |
| `test-context` type assert | Expects `*core.RealKernel`, got CortexKernel | Code bug — track in panic catalog |
| Concurrent `nerd.exe` hangs | SQLite multi-process locks | Serial matrix only |

## Harness Notes

- Prefer **serial** runs; kill prior `nerd.exe` between steps.
- After hang fix, process should exit 0 shortly after Result; if not, capture stack.
- Evidence dir pattern: `%TEMP%\codenerd-live-matrix\` with `MATRIX.md` + per-step `.out/.err`.
- Local orchestrator: `scripts/live_feature_matrix.ps1` (may be gitignored under `scripts/`).

## Pass Criteria

- [ ] Each catalog command invoked at least once on a dedicated workspace
- [ ] Coding commands leave artifacts **only** under workspace
- [ ] No panic / `debug_program_ERROR.mg`
- [ ] Post-Result exit latency &lt; 15s for one-shot create/spawn

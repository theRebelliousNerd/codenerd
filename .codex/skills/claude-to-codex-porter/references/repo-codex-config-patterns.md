# Repo Codex Config Patterns

This repository uses `.codex/config.toml` as an active part of the custom-agent
surface, not just as a personal preference file.

## Top-Level Model Surface

The current local config includes:

- `model`
- `model_context_window`
- `model_auto_compact_token_limit`

Migration rule:

- Do not change these during a skill or agent port unless the user explicitly asks.
- Read them so you understand the repo's default model and context assumptions.
- Preserve explicit codeNERD pins. Otherwise consult current official model and
  reasoning guidance; do not infer a permanent model mapping from Claude labels.

## UI and Feature Surface

The current local config also includes:

- `[tui]`
- `[features]`

The important feature flag for migration work is:

- `[features].multi_agent = true`
- `[features].hooks = true`

Migration rule:

- Treat this as repo context.
- Do not toggle feature flags while porting a skill unless the task is specifically
  about Codex configuration.
- Use `hooks` as the canonical hook feature key; `codex_hooks` is a deprecated
  alias in current docs.
- Do not assume the feature flag alone proves activation. Project hooks and
  config require trust, and codeNERD currently owns repo hooks in
  `.codex/hooks.json`.

## Global Agent Runtime Controls

The current local config uses an `[agents]` table for global limits:

- `max_threads`
- `max_depth`
- `job_max_runtime_seconds`

Migration rule:

- Read these so you understand how much concurrency the repo expects.
- Do not change them during a surface port unless the user explicitly asks for a
  config tuning change.

## MCP Server Surface

This repo also declares MCP servers in `.codex/config.toml`, for example:

- `[mcp_servers."jules-mcp"]`

Migration implication:

- If a migrated custom agent relies on a specific MCP server and the repo already
  declares it centrally, reuse that existing server name.
- Do not add or edit MCP server definitions unless the port actually requires it.

## Governed Custom-Agent Registration

This repository registers project agents explicitly with blocks like:

```toml
[agents.arch_monitor]
description = "Monitoring lane for long-running architecture waves."
config_file = "agents/arch-monitor.toml"
```

Official Codex can discover standalone `.codex/agents/*.toml` files without a
duplicate registry block. codeNERD additionally maintains a curated governed fleet
in `.codex/config.toml`.

Migration rule:

- When adding a new `.codex/agents/<name>.toml` file, also add or update the
  matching `[agents."<name>"]` block in `.codex/config.toml`.
- Keep `config_file` relative using the repo's existing `"agents/<name>.toml"`
  pattern.
- Use a concise Codex-facing `description`, even if the TOML file itself preserves
  a much longer source description in comments or fields.
- Validate registrations through each block's `config_file`, then compare the
  parsed TOML `name` to the registry key. Do not compare registry keys directly
  to kebab-case filenames.
- Do not make full-repository registry parity a scoped migration gate; report
  unrelated ambient drift separately.

## Configuration Ownership

Project-local `.codex/config.toml`, hooks, and rules are ignored until the project
is trusted. Provider, authentication, notification, telemetry routing, and other
machine/user-owned settings may not belong in checked-in project config.

Classify each Claude setting as one of:

- valid project-scoped Codex config
- user/admin-scoped config that must remain outside the repository
- unsupported gap

## Validation

After editing `.codex/config.toml`, parse it:

```powershell
python -c "import pathlib, tomllib; tomllib.loads(pathlib.Path('.codex/config.toml').read_text(encoding='utf-8')); print('config ok')"
```

Also verify every new agent file has a registration block when this repo pattern
applies.

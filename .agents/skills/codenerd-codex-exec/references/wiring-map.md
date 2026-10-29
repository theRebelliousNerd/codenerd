# Wiring Map

Use this reference to find the real seams for the Codex exec backend.

## Primary runtime files

- `internal/perception/codex_cli_client.go`
- `internal/perception/codex_cli_probe.go`
- `internal/config/llm.go`
- `internal/config/user_config.go`
- `cmd/nerd/cmd_auth.go`
- `cmd/nerd/chat/config_wizard.go`

## Scheduler touchpoints

The global scheduler still owns the hard ceiling for concurrent LLM calls.

The Codex-specific concurrency cap is a lower, engine-specific override layered on top of that ceiling.

When changing Codex concurrency behavior, check the boot/config wiring in:

- `internal/system/factory.go`
- `cmd/nerd/chat/session.go`
- `cmd/nerd/cmd_campaign.go`

## Runtime invariants

- `codex-cli` remains the user-facing engine name
- the transport is `codex exec`
- repo-skill injection is explicit, not implicit
- missing repo skill warns at runtime and degrades to legacy prompt behavior
- setup/auth probing treats missing repo skill as a failure
- schema-capable paths must work through `SchemaCapable()` and `CompleteWithSchema(...)`

# Concurrency And Rate Limits

Codex subscription-backed `codex exec` usage should start conservatively.

## Default posture

- Codex-specific default: `2`
- Global scheduler ceiling still applies
- Effective Codex concurrency: `min(global_ceiling, codex_cli.max_concurrent_calls)`

## Tuning guidance

Start at `2`.

Lower to `1` if noninteractive probes or real shard traffic show repeated subscription rate limits.

Do not raise Codex-specific concurrency above the global scheduler ceiling.

If the rate-limit path fires, verify:

- current model
- fallback model
- effective concurrency
- whether the failures are immediate auth/subscription failures versus genuine throughput limits

## What not to do

Do not hide rate limits by silently disabling schema validation.

Do not move concurrency handling into skill prose alone. The skill documents the tuning path, but the effective ceiling must be enforced in Go config/scheduler wiring.

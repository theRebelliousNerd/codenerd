# Auth And Probes

The Codex backend should be validated with real noninteractive execution, not with `codex --version`.

## First-line checks

- `nerd auth codex`
- `nerd auth status`

These commands should distinguish:

- CLI missing
- login/subscription unavailable
- repo skill missing
- schema-constrained probe failure
- rate limiting
- fallback model exhaustion

## Raw probe shape

The readiness probe should use a tiny schema-constrained `codex exec` call that asks for a JSON object like:

```json
{"status":"ok","mode":"codex-exec-health","skill":"...","schema_valid":true}
```

This keeps auth, schema, and repo-skill validation tightly scoped and easy to classify.

## Failure interpretation

- Login/subscription failures: tell the user to run `codex login`
- Skill-missing failures: point to `.agents/skills/codenerd-codex-exec`
- Schema failures: treat as a broken structured-output path, not a generic auth problem
- Rate-limit failures: keep current settings visible and suggest lowering Codex-specific concurrency first

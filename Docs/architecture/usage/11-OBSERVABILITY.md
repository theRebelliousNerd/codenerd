# usage — Observability

> Last verified: **2026-07-13**

## Dual identity

`internal/usage` **is** an observability subsystem for **token economy**. It is not a logging framework and does not currently integrate with `internal/logging`.

## What operators can observe

| Channel | Content | Latency |
|---------|---------|---------|
| `.nerd/usage.json` | Version + aggregate maps (and optional events if ever filled) | Debounced ~5s after first dirty Track; immediate after `Save()` |
| TUI Usage page | Totals + tables for provider/model/shard type/operation | On view refresh (`UpdateContent`) |
| In-process `Stats()` | Same as JSON aggregates | Immediate snapshot |

## What is not logged today

| Event | Reality |
|-------|---------|
| Track called | No log line |
| Save success/failure | No log line |
| Load failure at NewTracker | Comment says “would log”; **silent** continue |
| Unknown attribution spike | Only visible as `"unknown"` map keys in Stats |

## Interaction with other observability systems

| System | Relationship |
|--------|----------------|
| `internal/logging` StructuredLog / Audit | ZAI client logs prompt/completion token counts on success **independently** of usage tracker |
| `logging.Audit().LLMCall` | Audit trail of calls; not the durable project aggregate |
| Glass box / transparency | Separate UX; not driven by usage.Tracker |
| Campaign assault artifacts | Campaign code attaches tracker for metering but assault dumps are their own artifact tree |

## Debug hooks

| Hook | How |
|------|-----|
| Inspect file | Open `<workspace>/.nerd/usage.json` |
| Force flush | Call `tracker.Save()` from a debugger or future CLI |
| Disable autosave in tests | Set unexported `dirty=true` before Track (tests only, same package) |
| Attribution dump | Read `Stats().ByShardType` / `BySession` |

## Metrics that *should* exist (future)

If usage grows into a first-class platform meter:

- `usage_track_total` counter by provider/op  
- `usage_save_errors_total`  
- `usage_unknown_attribution_ratio`  
- histogram of tokens per Track  

Today: **none** of these are implemented in-package.

## Privacy

`usage.json` stores model/provider names and session ids, not prompt text. Keep it that way. Do not add raw message bodies to Events without a privacy review.

## Operator playbook

1. After a long session, open Usage page or `usage.json`.  
2. If totals look low: check engine (non-ZAI?) and whether contexts used `NewContext`.  
3. If mostly `"unknown"`: shard spawn path may have skipped `WithShardContext` or LLM used a detached context.  
4. If file empty after work: process killed before debounce Save; call Save on shutdown if available.

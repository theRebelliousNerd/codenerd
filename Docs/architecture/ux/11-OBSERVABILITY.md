# ux — Observability

> Last verified: **2026-07-13**  
> Honest inventory: this package is intentionally quiet.

## Logging

**No** calls to `codenerd/internal/logging` inside `internal/ux`.

Related logs live in **consumers**:

| Location | Category | Event |
|----------|----------|-------|
| `cmd/nerd/chat/session_boot.go` | `logging.CategoryBoot` | Preferences load failure warn |
| `cmd/nerd/chat/session_shared_boot.go` | `logging.CategoryBoot` | Same |
| `cmd/nerd/chat/session.go` | stdout-style warn | Load failure during model construct |

## Metrics (product-local, not ops)

`UserMetrics` in preferences.json are **product analytics for journey**, not Prometheus-style ops metrics:

| Counter | Intended meaning |
|---------|------------------|
| sessions_count | Sessions started |
| commands_executed | Commands run |
| clarifications_needed | Clarification friction |
| help_requests | Help usage |
| successful_tasks | Completions |
| errors_encountered | Errors |
| last_session | Timestamp string |

There is no scrape endpoint, no export format, and no aggregation service in-package.

## Telemetry prefs

`TelemetryPrefs` exposes:

- `enabled`  
- `anonymous_usage`  

Defaults false. **No exporter implementation** in `internal/ux`. Fields are forward-compatible schema.

## Debug hooks

| Hook | Presence |
|------|----------|
| Debug log of prefs path | No |
| Dump preferences to glass box | No |
| `/debug` slash integration | No (CLI may have other debug surfaces unrelated) |
| MigrationResult logging | Not logged by migrate callers (`_, _ = MigratePreferences`) |

## Glass box / transparency

Independent system (`internal/transparency`). UX does not emit glass-box events when journey changes.

## Recommended observability (future; not implemented)

1. Log migration results at Info once per boot (`WasMigrated`, versions).  
2. Log journey transitions when `CheckJourneyTransition` fires.  
3. Optional debug: redact-safe summary of metrics on `/status` if product wants.  
4. Never log full intent correction strings at Info (PII/workspace content).  

## Correlation with sessions

`LastSession` is an RFC3339 string only — not a session ID foreign key into `internal/session` or usage tracker. Cross-system correlation requires consumers to join by workspace + wall clock.

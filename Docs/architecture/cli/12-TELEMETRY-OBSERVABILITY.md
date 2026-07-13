# 12 — Telemetry & Observability (CLI)

> Last verified: 2026-07-13

## 1. Logging stacks

| Stack | Init | Output |
|-------|------|--------|
| Uber zap | `main.go` PersistentPreRunE | stderr structured logs for non-interactive |
| Categorized file logs | `logging.Initialize(workspace)` | `.nerd/logs/*` |
| Boot step prints | `session_boot.go` logStep | TTY status line during boot |

## 2. Operator-facing visibility

| Feature | Location | Use |
|---------|----------|-----|
| Glass box | `chat/glass_box.go`, `glassbox` cmd | Live tool/shard timeline |
| Transparency toggle | `/transparency`, `transparency` cmd | Show/hide ops |
| Activity pulse | chat activity helpers + tests | Elapsed/activity line |
| Reflection | `/reflection`, `reflection` cmd | Introspection |
| `logs` command | `cmd_logs.go` | Aggregate warnings/errors |
| Verbose / trace flags | `cmd_debug.go` | API/shard/kernel traces |
| JIT page | `ui/jit_page.go`, `jit` cmd | Prompt compilation inspection |

## 3. Metrics / flight recorder

Runtime metrics and flight recorder live under `internal/observability` (imported from main). CLI should not reimplement counters; emit through shared observability when adding hot paths.

## 4. Gaps

- Not all slash handlers emit structured events.
- Boot timing is printf-based; could be richer structured spans.
- Assault campaigns produce filesystem artifacts; ensure operators know path conventions.

## 5. Guidance for new features

When adding a CLI feature that performs multi-step work:

1. Log start/end at Info with duration.
2. Emit glass-box events for user-visible stages.
3. Never log secrets.
4. Prefer existing categories in `internal/logging` over new ad-hoc files.

# 11 — Observability: Northstar

> Last verified against codebase: 2026-07-13  
> Package: `internal/northstar`

## 1. Logging category

| Constant | Value | Declared in |
|----------|-------|-------------|
| `logging.CategoryNorthstar` | `"northstar"` | `internal/logging/logger.go` |

Boot may also log registration under `CategoryBoot` (`session_boot.go`).

## 2. Log events (package)

| Level | When | Message pattern |
|-------|------|-----------------|
| **Info** | Init with vision | Mission truncated to 50 chars |
| **Warn** | Init without vision | Points at store path; checks will skip |
| **Info** | Vision updated | Truncated mission |
| **Info** | Alignment check complete | subject, result, score |
| **Info** | Campaign observer start/end | campaign ID, success |
| **Debug** | Kernel assert fail | predicate + error |
| **Debug** | Persist check/drift/state fail | error |
| **Debug** | Campaign start check fail | error |
| **Debug** | Task/file observation fail | error |
| **Debug** | Task start (TaskObserver) | type + truncated desc |
| **Debug** | Background observation fail | error |

## 3. Durable observability (SQLite)

Better than logs for forensics:

| Table | Query intent |
|-------|--------------|
| `alignment_checks` | Historical scores/results by time |
| `drift_events` | Open vs resolved drift |
| `observations` | Session activity trail |
| `guardian_state` | Current rollup (overall_alignment, tasks_since_check) |

Access today: Store APIs (`GetAlignmentHistory`, `GetActiveDriftEvents`, `GetRecentObservations`, `GetState`). **No first-class CLI** dumps these tables.

## 4. Runtime UX signals

| Surface | Signal |
|---------|--------|
| `/alignment` | Formatted score bar + suggestions (`formatAlignmentCheckResult`) |
| BackgroundObserver | Assessment Level proceed/note/clarify/block |
| Campaign risk audit | `risk_gate_result` / blocked emissions in campaign package |

## 5. Gaps

- No metrics counters (checks/sec, blocked rate).
- No structured logging fields (key-value) — printf style.
- No glass-box export of Northstar facts specifically.
- Dual store means “show vision” CLI may not match Guardian state used for checks.

## 6. Operator debug recipe

1. Ensure `.nerd/northstar_knowledge.db` exists after boot.
2. Enable verbose / northstar category logs.
3. Run `/alignment <subject>` and inspect score.
4. Query store via test harness or sqlite3 CLI if needed:
   - `SELECT result, score, subject FROM alignment_checks ORDER BY timestamp DESC LIMIT 20;`
5. Confirm kernel (if wired): query `northstar_defined` / `northstar_mission` through `nerd query` if available.

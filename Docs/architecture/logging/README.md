# logging — `internal/logging/`

> Verified 2026-09-21 against commit `3463477`
> (`34634770970153e78c1e250fdab7abd888dcce6f`).

File-action log sinks, an NDJSON audit trail, and fresh-run log hygiene for the
nerd binary: 9 non-test Go files (~4,200 lines), 12 test files, and one
lifecycle note (`internal/logging/agents.md`). No Go was changed for this
rewrite; `go build` and `go test ./...` still pass.

## Map

| Subsystem | Files | Reaches disk through |
| Category sinks | `logger.go`, `logger_convenience.go` | `Initialize` (`logger.go:174`), `Get` (`logger.go:454`), `CloseAll` (`logger.go:805`) |
| Audit trail | `audit.go`, `audit_facts.go`, `audit_reader.go` | `InitAudit` (`audit.go:138`), `Audit` (`audit.go:184`), `ExportAuditFacts` (`audit_facts.go:37`), `ReadRecentAuditEvents` (`audit_reader.go:69`) |
| Fresh-run hygiene | `fresh_run.go` | `generateRunPrefix` (`fresh_run.go:85`), `clearOrdinaryLogs` (`fresh_run.go:277`) |
| Size rotation | `rotate.go` | `openRotatingFile` (`rotate.go:55`), `rotationPolicy` (`rotate.go:214`) |
| LLM trace + redaction | `llm_io_logger.go`, `redact.go` | `LogLLMRequest` (`llm_io_logger.go:115`), `RedactSecrets` (`redact.go:52`) |

Lifecycle rule (`agents.md:5-6`): every audit entry goes through `auditMu`, and
the audit file opens only via `InitAudit` — lazily, through any `Audit*()`
constructor (`Audit`, `AuditWithSession`, `AuditWithShard`, `AuditWithContext`
at `audit.go:184-210`).

## Where the questions are answered

- What it is: this file.
- How each subsystem works: `INTERNALS.md` — one section per subsystem above.
- Who calls it, what exists but nothing calls, and what the design assumes the
  code does not do: `WIRING-AND-NOT-BUILT.md`.

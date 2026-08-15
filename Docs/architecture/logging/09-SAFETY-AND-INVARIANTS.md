# 09 — Safety and Invariants (`internal/logging`)

> Last verified: **2026-08-15**

## 1. Role in constitutional safety

| Layer | Responsibility |
|-------|----------------|
| Mangle policy `permitted(...)` | **Executive gate** (default deny) |
| VirtualStore / tools | Enforcement of denied actions |
| **logging** | **Witness** — may record allow/block after the fact |

`AuditLogger.SafetyCheck` is evidence, not a control. Callers must never skip policy because an audit line was written.

## 2. Invariants

### I1 — Disabled means no files

If `!IsDebugMode()`, category `Get` returns nil-backed logger; `InitAudit` no-ops; LLM I/O disabled unless somehow separately… (LLM I/O also requires `TraceLLMIO` and non-empty `logsDir`, which is only set when init proceeds in debug mode).

### I2 — No panics from logging APIs

Public log methods must tolerate nil internal logger, missing files, disabled mode. Tests encode this with extensive “ShouldNotPanic” cases.

### I3 — Product path fails open

Failure to initialize logging must not abort CLI product operations (`main` PersistentPreRun warns and continues).

### I4 — One binding at a time

`Initialize` runs setup once per workspace. Repeating it with the same absolute
path is a no-op; a different path **rebinds** (all sinks closed and reopened under
the new workspace, LLM I/O `sync.Once` rearmed, config reloaded, move recorded in
the new boot log). `initMu` serializes the whole operation, so a rebind cannot
interleave with a concurrent first init.

### I5 — Mutex coverage

Config, logger map, audit file, and LLM I/O file must not be written without their mutexes.

### I6 — Error logs not level-filtered away

`Logger.Error` ignores `logLevel` threshold (still requires non-nil logger). Debug/Info/Warn respect level.

### I7 — Mangle string escaping and literal validity

**Every** string interpolated into a generated fact goes through `mangleString`
(quote + `escapeString` for `" \ \n \r \t`), not just error and message text —
targets are file paths and shell commands, and one embedded quote used to corrupt
the fact and everything after it. Booleans render as the `/true` / `/false` name
constants; a bare `true` is not a Mangle literal, so facts carrying success flags
never parsed. `cmd/nerd/cmd_audit_test.go` runs the real parser over an export of
every event family.

### I8 — Performance self-skip

`logPerformance` does not re-enter when category is already `CategoryPerformance` (prevents recursion).

### I9 — Opt-in for full prompts, redacted by default

`debug_mode` alone does **not** enable the LLM I/O dump; `trace_llm_io` is
required. When it is on, everything written passes through `RedactSecrets` first.
Disabling that requires a second, explicitly named opt-in (`trace_llm_io_raw`),
and the boot log states which mode is active.

### I10 — Bounded files

No sink grows without bound. Across runs, old run prefixes are swept at startup
(`fresh_run.go`); within a run, every sink is a `rotatingFile` that closes its
segment on size or age and keeps a bounded number of archived segments. Archived
segments keep the `.log` suffix and the run prefix so the startup sweep expires
them with their run.

## 3. Concurrency hazards (known / accepted)

| Hazard | Mitigation / residual risk |
|--------|----------------------------|
| `ReloadConfig` vs active loggers | Config changes apply to enablement/level; open files keep writing |
| `RequestLogger.WithField` mutates shared map | Document as not concurrent-safe across goroutines sharing one instance |
| `rand` sampling without mutex | Acceptable for sampling; not crypto |
| Midnight date boundary | Filenames use a per-run prefix, not a date, so the boundary is irrelevant |
| Once + wrong workspace | Resolved: a later `Initialize` with a different absolute workspace rebinds every sink |
| Rotation mid-write | `rotatingFile.mu` covers the size check, rename and reopen; `log.Logger` issues one `Write` per record, so a boundary never splits a line |

## 4. Privacy & secrets

| Surface | Risk | Guidance |
|---------|------|----------|
| Category Info/Debug | Medium | Never log API keys, tokens, full env |
| Audit Error fields | Medium | Prefer error codes over secret bodies |
| LLM I/O | **High** | Entire system/user prompts on disk — redacted, but still full content |
| History truncation 2000 | Partial | Only history truncated; system/user full |
| `trace_llm_io_raw` | **Critical** | No redaction at all; single-run debugging only, delete the file afterwards |

**Invariant (I9, implemented):** no LLM I/O write bypasses `RedactSecrets` unless
`trace_llm_io_raw` is set. Patterns cover credential-ish key names,
`Authorization`/bearer, `sk-`/`ghp_`/`AKIA`/`xox` prefixes, PEM blocks and URL
credentials — the same shapes as `internal/mcp`'s redactor, duplicated rather than
imported because `internal/mcp` depends on this package.

## 5. Filesystem safety

- Paths confined under `<workspace>/.nerd/logs`  
- No user-controlled path join for category names (categories are typed constants)  
- Audit/LLM filenames follow a fixed pattern built from the process run prefix; rotated segments derive their name from the live one and stay in the same directory  
- File mode `0600`; the logs directory is `0700` and a symlinked logs directory (or symlinked `.nerd`) is refused outright

## 6. Mangle / Decl relevance

This package declares no predicates in the live kernel. `ExportAuditFacts`
(`nerd audit facts`) writes an **offline** `.mg` that owns its own `Decl`
statements, derived from the arity actually present in the log, and is labelled
"Do not load into the live kernel" — telemetry that re-entered the executive would
let the record of what happened change what happens next.

## 7. Security non-goals

- Encryption at rest for log files  
- Multi-tenant isolation beyond OS permissions  
- Anti-tamper / signed audit chains  

## 8. Verification gates

```powershell
go test ./internal/logging/...
go test -race ./internal/logging/...
```

Manual: with `debug_mode:false`, assert `.nerd/logs` is not created after a short CLI command.

# 09 — Safety and Invariants (`internal/logging`)

> Last verified: **2026-07-13**

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

### I4 — Once init

Successful process init path runs setup at most once. Concurrent `Initialize` is safe.

### I5 — Mutex coverage

Config, logger map, audit file, and LLM I/O file must not be written without their mutexes.

### I6 — Error logs not level-filtered away

`Logger.Error` ignores `logLevel` threshold (still requires non-nil logger). Debug/Info/Warn respect level.

### I7 — Mangle string escaping

User-controlled error/message fragments in facts go through `escapeString` for `" \ \n \r \t`.

### I8 — Performance self-skip

`logPerformance` does not re-enter when category is already `CategoryPerformance` (prevents recursion).

### I9 — Opt-in for full prompts

`debug_mode` alone does **not** enable LLM I/O dump; `trace_llm_io` is required.

## 3. Concurrency hazards (known / accepted)

| Hazard | Mitigation / residual risk |
|--------|----------------------------|
| `ReloadConfig` vs active loggers | Config changes apply to enablement/level; open files keep writing |
| `RequestLogger.WithField` mutates shared map | Document as not concurrent-safe across goroutines sharing one instance |
| `rand` sampling without mutex | Acceptable for sampling; not crypto |
| Midnight date boundary | Filename uses open time; long process may keep writing to yesterday’s file |
| Once + wrong workspace | Residual: first Initialize wins |

## 4. Privacy & secrets

| Surface | Risk | Guidance |
|---------|------|----------|
| Category Info/Debug | Medium | Never log API keys, tokens, full env |
| Audit Error fields | Medium | Prefer error codes over secret bodies |
| LLM I/O | **High** | Entire system/user prompts on disk |
| History truncation 2000 | Partial | Only history truncated; system/user full |

**Invariant candidate (future):** redaction filter before any LLM I/O write.

## 5. Filesystem safety

- Paths confined under `<workspace>/.nerd/logs`  
- No user-controlled path join for category names (categories are typed constants)  
- Audit/LLM filenames fixed pattern with date  
- File mode `0644` — readable by local users on shared machines (operator responsibility)

## 6. Mangle / Decl relevance

This package does **not** declare Mangle predicates. Generated fact *names* (`shard_lifecycle`, `action_event`, …) are **informal conventions**. If a future offline loader asserts them, that loader must own `Decl` statements per `internal/mangle/agents.md` rules.

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

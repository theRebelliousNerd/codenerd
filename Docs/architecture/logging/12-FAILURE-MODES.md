# 12 — Failure Modes (`internal/logging`)

> Last verified: **2026-07-13**

## FM1 — Config missing or unreadable

| | |
|--|--|
| **Symptom** | No `.nerd/logs`; diagnostics “don’t work” |
| **Cause** | Missing `.nerd/config.json` or read error → `DebugMode=false` |
| **Mitigation** | Write explicit `logging.debug_mode: true`; stderr shows parse warnings |
| **Product impact** | None (silent production) |

## FM2 — Invalid JSON in config

| | |
|--|--|
| **Symptom** | Logging disabled; stderr warning from `initializeInternal` |
| **Cause** | `json.Unmarshal` failure in `loadConfig` |
| **Mitigation** | Fix JSON; `ReloadConfig` after edit (level/flags), restart for full re-init |

## FM3 — Cannot create logs directory

| | |
|--|--|
| **Symptom** | `Initialize` returns error |
| **Cause** | Permissions / full disk on workspace |
| **Mitigation** | CLI warns and continues without file telemetry |
| **Residual** | No diagnostics until fixed |

## FM4 — Cannot open category log file

| | |
|--|--|
| **Symptom** | stderr warning; that category no-ops |
| **Cause** | Permissions / path issues in `Get` |
| **Mitigation** | Other categories still work |

## FM5 — Audit init failure

| | |
|--|--|
| **Symptom** | Boot warn “Failed to initialize audit logging”; no audit file |
| **Cause** | `InitAudit` open error |
| **Mitigation** | Category logs may still work |
| **Residual** | No Mangle-fact stream |

## FM6 — Wrong workspace due to sync.Once

| | |
|--|--|
| **Symptom** | Logs appear under unexpected path; or empty when expected |
| **Cause** | `main` Initialize(cwd) before `Chdir(--workspace)` / second Initialize ignored |
| **Mitigation** | Start process with correct cwd; or fix boot order (code change) |
| **Severity** | Medium for multi-workspace users |

## FM7 — Disk fill from debug / LLM I/O

| | |
|--|--|
| **Symptom** | Large `*_llm_io.log` or many category files |
| **Cause** | `trace_llm_io` or broad category map at debug level |
| **Mitigation** | Narrow categories; disable LLM I/O; `max_log_file_mb` / `max_log_file_minutes` / `max_rotated_files` bound each sink within a run, and old run prefixes are swept at startup |

## FM8 — Secret leakage to disk

| | |
|--|--|
| **Symptom** | API keys / private code in log files |
| **Cause** | Callers logging secrets; or `trace_llm_io` with secret-bearing prompts |
| **Mitigation** | Redaction is on by default for the LLM I/O trace (`redact.go`); category/audit lines are still the caller's responsibility, so do not pass secrets into log messages. `trace_llm_io_raw` disables redaction — treat any file produced under it as a credential |
| **Severity** | High on shared machines |

## FM9 — Incomplete shutdown

| | |
|--|--|
| **Symptom** | Rare truncated last lines; handles held until exit |
| **Cause** | Historic: `CloseAll` closed only the category loggers |
| **Mitigation** | Resolved — `CloseAll` closes categories, problems, audit and LLM I/O. `CloseAudit` / `CloseLLMIOLogger` remain for partial teardown; all are idempotent |

## FM10 — Level hides errors that are actually Debug

| | |
|--|--|
| **Symptom** | “Missing logs” for failures logged only at Debug |
| **Cause** | `level: info` with important paths using Debug |
| **Mitigation** | Temporarily set `level: debug` for investigation; prefer Error for real failures |

## FM11 — Performance sampling drops useful samples

| | |
|--|--|
| **Symptom** | Sparse performance log |
| **Cause** | Low `performance_sampling` and ops under threshold |
| **Mitigation** | Raise sampling; lower thresholds; use `StopWithThreshold` for critical paths |

## FM12 — JSON vs text confusion

| | |
|--|--|
| **Symptom** | Parsers fail; greps for `[INFO]` miss |
| **Cause** | `json_format: true` changes line shape |
| **Mitigation** | Align tooling with format. `format: "json"` and the legacy `json_format: true` both drive this package |

## FM13 — Context/Request logger ignore JSON mode (resolved)

| | |
|--|--|
| **Symptom** | Mixed formats in the same category file; a JSONL consumer silently dropped exactly the lines carrying correlation IDs |
| **Cause** | Historic: Context/Request paths always emitted text |
| **Mitigation** | Resolved — both emit `StructuredLogEntry` when the format is JSON (`req` + `fields`) |

## FM14 — Race on shared RequestLogger fields

| | |
|--|--|
| **Symptom** | Interleaved field maps / data race under `-race` |
| **Cause** | Concurrent `WithField` on same instance |
| **Mitigation** | One RequestLogger per goroutine or external sync |

## FM15 — Stale filename across midnight (resolved)

| | |
|--|--|
| **Symptom** | Historic: all events after midnight still in yesterday's file |
| **Cause** | Filenames were date-prefixed and fixed at first `Get` |
| **Mitigation** | Resolved — filenames use a per-run prefix, so the date boundary is meaningless; `Initialize` with a different workspace also rebinds cleanly |

## Failure mode summary table

| ID | Severity | User-visible? | Auto-recover? |
|----|----------|---------------|---------------|
| FM1 | Low | No logs | After config fix + restart |
| FM2 | Low | No logs | After config fix |
| FM3 | Medium | Telemetry off | After FS fix |
| FM4 | Low | Partial | Per-category |
| FM5 | Medium | No audit | After FS fix + restart |
| FM6 | Medium | Wrong logs | Restart correct cwd |
| FM7 | High | Disk pressure | Disable flags |
| FM8 | High | Security | Manual scrub |
| FM9 | Low | Rare | Process exit |
| FM10 | Low | Missing detail | Raise level |
| FM11 | Low | Sparse metrics | Config tweak |
| FM12 | Low | Tooling break | Align format |
| FM13 | Low | Mixed lines | Use StructuredLog |
| FM14 | Medium | Race | Coding fix |
| FM15 | Low | File name | Restart |

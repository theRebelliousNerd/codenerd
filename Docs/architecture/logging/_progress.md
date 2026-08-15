# _progress — logging architecture corpus

## 2026-07-13 — Full rebuild (SUBAGENT_INSTRUCTIONS)

- **Mode:** Docs only under `Docs/architecture/logging/`  
- **Source researched:** `internal/logging/` (4 non-test Go, 5 tests, 0 `.mg`)  
- **Reverse deps:** `cmd/nerd`, chat boot, and many `internal/*` consumers  
- **Quality bar:** `Docs/architecture/cli/` depth  
- **Produced full set:**

| File | Notes |
|------|-------|
| `README.md` | Scope, map, verify |
| `IMPLEMENTED_SPEC.md` | Flagship deep-dive |
| `00-ALIGNMENT-VISION-REVIEW.md` | Scored dimensions |
| `01-VISION.md` | Target vision |
| `02-CURRENT-STATE.md` | Inventory |
| `03-GAP-ANALYSIS.md` | Gaps + non-gaps |
| `04-ARCHITECTURAL-PRINCIPLES.md` | 12 principles |
| `05-INTERNAL-ARCHITECTURE.md` | Components + flows |
| `06-PUBLIC-API-AND-TYPES.md` | API surface |
| `07-DEPENDENCY-MAP.md` | Import graph |
| `08-WIRING-AND-INTEGRATION.md` | Boot + call patterns |
| `09-SAFETY-AND-INVARIANTS.md` | Safety / privacy |
| `10-TESTING-ALIGNMENT.md` | Tests |
| `11-OBSERVABILITY.md` | Operator guide |
| `12-FAILURE-MODES.md` | FM1–FM15 |
| `TODO.md` | Backlog |
| `OPEN-QUESTIONS.md` | Design questions |
| `_progress.md` | This file |

- **Earlier thin stubs** (DOMAIN-MODEL, CROSS-SYSTEM-WIRING naming, etc.) may still exist as orphans; authoritative set is the table above linked from `README.md`.  
- **No code changes.**

## 2026-08-15 — Backlog pass (code + docs)

Implemented the open TODO items; corpus reconciled against the code, not the other
way round.

| Item | Where |
|------|-------|
| Config schema alignment (`format` canonical, `json_format` alias) | `logger.go`, `config_schema_test.go` |
| Injectable config from boot (`ApplyConfig`) | `logger.go`, `init_rebind_test.go` |
| LLM I/O secret redaction + `trace_llm_io_raw` | `redact.go`, `llm_io_logger.go`, `llm_io_trace_test.go` |
| `CloseAll` closes all four sinks | `logger.go`, `sink_lifecycle_test.go` |
| Workspace rebind (`sync.Once` + `--workspace` race) | `logger.go`, `init_rebind_test.go` |
| Enabled-path `trace_llm_io` marker tests | `llm_io_trace_test.go` |
| Structured `ContextLogger` / `RequestLogger` | `logger.go`, `structured_decorators_test.go` |
| Operator playbook | `nerd audit playbook`, README |
| Audit JSONL → `.mg` | `audit_facts.go`, `cmd/nerd/cmd_audit.go` |
| Size/age rotation | `rotate.go`, `rotate_test.go` |
| Northstar/Regression convenience wrappers | `logger_convenience.go` |
| `runtime.Caller` file/line on JSON entries | `logger.go` |
| SafetyCheck call-site audit as a ratchet test | `safety_callsite_audit_test.go` |

Incidental defect found by the new parser-backed test: generated Mangle facts used
`%v` booleans (bare `true`, which Mangle has no literal for) and interpolated
targets unescaped, so no audit fact was ever loadable. Fixed with `mangleBool` /
`mangleString`.

Still open: wiring `internal/config` to call `ApplyConfig`, and the unaudited
constitutional gate in `internal/shards/system/constitution.go`.

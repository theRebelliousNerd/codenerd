# 03 — Gap Analysis (`internal/logging`)

> Last verified: **2026-07-13**

## Matrix: vision vs reality

| Vision item | Reality | Status | Priority |
|-------------|---------|--------|----------|
| Default silent production | Missing/`debug_mode:false` ⇒ no logs | **Met** | — |
| Category isolation | Map + defaults-true for unspecified | **Met** | — |
| Three streams | Category + audit + LLM I/O | **Met** | — |
| Correlation IDs | RequestLogger + audit fields | **Met** (opt-in) | P3 adopt more widely |
| Performance thresholds/sampling | Implemented + tests | **Met** | P3 document in operator guides |
| Witness of safety | `SafetyCheck` helper | **Met API** / partial caller use | P2 audit call sites |
| Config single source of truth | Dual structs; `json_format` vs `Format` | **Gap** | **P1** |
| Read config.yaml as well as JSON | Only `config.json` | **Gap** | **P1** |
| Safe LLM redaction | Full dump, no filters | **Gap** | **P1** |
| Unified shutdown | `CloseAll` ≠ audit/LLM I/O | **Gap** | **P2** |
| Workspace rebind after init | `sync.Once` forever | **Gap** (tests fight it) | P2 |
| Kernel ingest of audit facts | Strings only | **Intentional non-goal** or P3 tool | P3 |
| JSON path for Context/Request loggers | Text-only formatting | **Gap** | P3 |
| Northstar convenience wrappers | Category only | **Minor gap** | P4 |
| Log rotation / retention | Date filename only | **Gap** | P3 |
| Structured source file/line | Fields exist; not auto-filled | **Gap** | P4 |

## Non-gaps (do not “fix” by expanding scope)

| Temptation | Why it’s a non-gap |
|------------|--------------------|
| Move `permitted` into logger | Breaks inversion of control |
| Replace glass-box UX | Different product surface |
| Force debug on in CI always | Would thrash disks and hide real enablement bugs |
| Import `internal/config` for shared types | Risk of import cycles; mirror is deliberate — fix via shared neutral package if needed |
| Auto-assert every log to kernel | Couples executive to I/O; volume explosion |

## Prioritized remediation

### P1 — correctness / safety for operators

1. **Unify format config** — map `format: "json"|"text"` ↔ `json_format` or document that only `json_format` in `config.json` controls this package.  
2. **Document path** — `logging` reads **only** `.nerd/config.json`; YAML loaders must sync.  
3. **LLM I/O redaction hooks** — at minimum strip common `Bearer` / `api_key=` patterns when dumping.

### P2 — lifecycle

1. `CloseAll` should call `CloseAudit` + `CloseLLMIOLogger` (or document required order).  
2. Test helper / optional `ResetForTest` exporting Once rebind (tests currently poke unexported state).  
3. Wire chat shutdown path to close audit/LLM I/O explicitly if long-lived sessions matter.

### P3 — ergonomics

1. Context/Request loggers honor `json_format` via `StructuredLog`.  
2. Operator doc: enable set for common investigations (kernel hang, JIT bloat, campaign fail).  
3. Optional offline `nerd logs audit-to-mg` consumer (CLI, not package).

### P4 — polish

1. Northstar convenience funcs.  
2. Auto `runtime.Caller` for file/line on structured entries.  
3. Size-based rotation if multi-day debug sessions are common.

## Spec vs implementation completeness (heuristic)

| Subsystem | Completeness |
|-----------|-------------:|
| Category logger core | 95% |
| Convenience API | 90% (northstar missing) |
| Audit API | 90% |
| Audit→kernel pipeline | 0% (out of package) |
| LLM I/O | 85% (no redaction) |
| Config integration | 70% |
| Shutdown completeness | 65% |

**Package overall (as diagnostic substrate):** ~**88%**.

# 03 — Gap Analysis (`internal/logging`)

> Last verified: **2026-08-15**

## Matrix: vision vs reality

| Vision item | Reality | Status | Priority |
|-------------|---------|--------|----------|
| Default silent production | Missing/`debug_mode:false` ⇒ no logs | **Met** | — |
| Category isolation | Map + defaults-true for unspecified | **Met** | — |
| Three streams | Category + audit + LLM I/O | **Met** | — |
| Correlation IDs | RequestLogger + audit fields | **Met** (opt-in) | P3 adopt more widely |
| Performance thresholds/sampling | Implemented + tests | **Met** | P3 document in operator guides |
| Witness of safety | `SafetyCheck` wired at the VirtualStore and tool-executor gates; call sites pinned by `safety_callsite_audit_test.go` | **Met**, one known gap (`internal/shards/system/constitution.go`) | P2 for `internal/shards` |
| Config single source of truth | One file, both key spellings accepted; `ApplyConfig` for injection | **Met** | — |
| Read config.yaml as well as JSON | Only `config.json` — deliberate: it is the file the app treats as truth | **Met (by decision)** | — |
| Safe LLM redaction | Shape-based redaction on by default; `trace_llm_io_raw` opts out | **Met** | — |
| Unified shutdown | `CloseAll` closes categories + problems + audit + LLM I/O | **Met** | — |
| Workspace rebind after init | Rebind on a different absolute workspace | **Met** | — |
| Kernel ingest of audit facts | `nerd audit facts` exports an offline `.mg` | **Intentional non-goal** (Q1 resolved) | — |
| JSON path for Context/Request loggers | Structured entries with `req` + `fields` | **Met** | — |
| Northstar convenience wrappers | Info/Debug/Warn/Error (plus Regression, PersistError) | **Met** | — |
| Log rotation / retention | Cross-run retention + in-run size/age rotation | **Met** | — |
| Structured source file/line | `callerSite()` fills file/line on the JSON path | **Met** | — |

## Non-gaps (do not “fix” by expanding scope)

| Temptation | Why it’s a non-gap |
|------------|--------------------|
| Move `permitted` into logger | Breaks inversion of control |
| Replace glass-box UX | Different product surface |
| Force debug on in CI always | Would thrash disks and hide real enablement bugs |
| Import `internal/config` for shared types | Risk of import cycles; mirror is deliberate — fix via shared neutral package if needed |
| Auto-assert every log to kernel | Couples executive to I/O; volume explosion |

## Prioritized remediation

All P1–P4 items above are implemented (see TODO.md for the test that keeps each
one honest). What remains:

1. `internal/config` should call `logging.ApplyConfig` so `.nerd/config.json` is
   parsed once instead of twice (same file, so no divergence — just a wasted read).
2. `ConstitutionGateShard.CheckAction` (`internal/shards/system/constitution.go`)
   decides allow/deny without recording a verdict. Classified as a known gap in
   `safety_callsite_audit_test.go`; the fix belongs to `internal/shards`.
3. Tests still poke unexported state to reset the package; an exported
   `ResetForTest` would be cleaner but would also be a public API whose only
   caller is a test.

## Spec vs implementation completeness (heuristic)

| Subsystem | Completeness |
|-----------|-------------:|
| Category logger core | 98% |
| Convenience API | 98% |
| Audit API | 95% (facts now parse as Mangle) |
| Audit→kernel pipeline | intentionally 0%; offline `.mg` export instead |
| LLM I/O | 95% (redacted by default) |
| Config integration | 90% (injection available, caller not wired) |
| Shutdown completeness | 100% |

**Package overall (as diagnostic substrate):** ~**96%**.

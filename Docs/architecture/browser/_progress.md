# Progress — Docs/architecture/browser

| Date | Event |
|------|-------|
| 2026-07-13 | Thin auto-inventory stubs (tier 2) generated |
| 2026-07-13 | **Full corpus rebuild** to SUBAGENT_INSTRUCTIONS quality bar (CLI-depth): research of `internal/browser/` + reverse wiring (CLI, chat boot, tactile router, research tools, schemas/policy, VirtualStore); wrote full required document set including dense `IMPLEMENTED_SPEC.md` |
| 2026-08-09 | Pinned BrowserNERD source at CrossThread `0610dacd0`; reconciled the newer system-factory manager wiring; added `BROWSERNERD-PARITY.md` with 26 capability gates and five bounded delivery milestones |
| 2026-08-09 | BPAR-1 runtime slice: live-kernel sink and shared research route; shared/isolated tabs; multi-browser package lifecycle; focus/close; limits/reaper; manager-owned event streams; redaction and symlink-aware private output policy. Focused package, live Chrome, config, CLI, research, and system wiring tests pass; progressive/CLI lifecycle exposure remains open. |
| 2026-08-09 | BPAR-2 progressive slice: generation-bound opaque refs with fingerprint fallback; bounded state/nav/interactive/grid/hidden/screenshot/DOM/React observations; closed sequential act/fill/key/history/lifecycle operations; exact constitutional permission; JIT atoms and intent routing. Unit, race, policy, and live modular-registry Chrome proof pass. |
| 2026-08-09 | BPAR-3 reasoning slice: session-scoped typed event facts; live-kernel failure/root-cause/blocker rules; read-only bounded `browser_mangle`; fresh action-watermark fact/condition/stability waits; bounded `browser_reason`; JIT reasoning atoms. Unit, policy, race, and live same-Cortex query/diagnosis/session-isolation proof pass. |
| 2026-08-09 | BPAR-4 evidence slice: default-on per-session redacted JSONL recorder; size/file rotation and read/export ceilings; confined exports; native `browser_evidence` tool and JIT atoms; platform current-user-only ACL enforcement. Focused, prompt, and production-shaped live Chrome read/export/session-isolation proof pass; specs and declarative tests remain. |

## Rebuild notes

- Source: 13 checked-in non-test Go files across `internal/browser` and `internal/browser/security` (12 per platform), 13 tests, 0 package-local `.mg`
- Companion: `schemas_browser.mg`, `policy/browser.mg`, `policy/browser_honeypot.mg`
- The 2026-08-09 BPAR-1 slice modifies Go/config/CLI wiring; the 2026-07-13 rebuild was docs-only.
- The BPAR-2 live tool test writes screenshot evidence only under `t.TempDir`; no rendered artifact is checked in.
- The BPAR-3 live proof queries the same live kernel that receives and authorizes browser facts.
- The BPAR-4 evidence live proof writes only below `t.TempDir`, verifies current-user-only export permissions, and checks cross-session exclusion; no trace artifact is checked in.
- Canonical doc names per rebuild contract (see README); older stub filenames superseded

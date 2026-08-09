# Progress — Docs/architecture/browser

| Date | Event |
|------|-------|
| 2026-07-13 | Thin auto-inventory stubs (tier 2) generated |
| 2026-07-13 | **Full corpus rebuild** to SUBAGENT_INSTRUCTIONS quality bar (CLI-depth): research of `internal/browser/` + reverse wiring (CLI, chat boot, tactile router, research tools, schemas/policy, VirtualStore); wrote full required document set including dense `IMPLEMENTED_SPEC.md` |
| 2026-08-09 | Pinned BrowserNERD source at CrossThread `0610dacd0`; reconciled the newer system-factory manager wiring; added `BROWSERNERD-PARITY.md` with 26 capability gates and five bounded delivery milestones |
| 2026-08-09 | BPAR-1 runtime slice: live-kernel sink and shared research route; shared/isolated tabs; multi-browser package lifecycle; focus/close; limits/reaper; manager-owned event streams; redaction and symlink-aware private output policy. Focused package, live Chrome, config, CLI, research, and system wiring tests pass; progressive/CLI lifecycle exposure remains open. |
| 2026-08-09 | BPAR-2 progressive slice: generation-bound opaque refs with fingerprint fallback; bounded state/nav/interactive/grid/hidden/screenshot/DOM/React observations; closed sequential act/fill/key/history/lifecycle operations; exact constitutional permission; JIT atoms and intent routing. Unit, race, policy, and live modular-registry Chrome proof pass. |

## Rebuild notes

- Source: 10 non-test Go files across `internal/browser` and `internal/browser/security`, 12 tests, 0 package-local `.mg`
- Companion: `schemas_browser.mg`, `policy/browser.mg`, `policy/browser_honeypot.mg`
- The 2026-08-09 BPAR-1 slice modifies Go/config/CLI wiring; the 2026-07-13 rebuild was docs-only.
- The BPAR-2 live tool test writes screenshot evidence only under `t.TempDir`; no rendered artifact is checked in.
- Canonical doc names per rebuild contract (see README); older stub filenames superseded

# Stress-tester skill expansion REPORT (2026-07 live matrix gaps)

## Summary

Expanded category **09-cli-workspace-matrix** from 3 to **6** workflows (total skill workflows **32 to 35**). Additive only; no old workflows deleted. Panic catalog refined for **e18d6818** Close timeouts; subsystem CLI table updated for dual-LLM / define-agent flags / one-shot lifecycle.

## New workflows (mirrored to `.agents`, `.claude`, `.codex`)

| File | Purpose |
|------|---------|
| `references/workflows/09-cli-workspace-matrix/one-shot-cli-exit.md` | create/spawn clean exit; maintenanceCancel + Close 8s bounds |
| `references/workflows/09-cli-workspace-matrix/dual-llm-routing.md` | main vs optional worker Ollama vs image Gemini Nano Banana 2 (never Ollama) |
| `references/workflows/09-cli-workspace-matrix/define-agent-flags.md` | `--name` / `--topic` required flags; invalid names |

## Updated files

| File | Change |
|------|--------|
| `references/workflows/09-cli-workspace-matrix/full-cli-surface.md` | Correct define-agent flags; sibling workflow links; Close timeout / image failure modes |
| `references/panic-catalog.md` | P0 refined (e18d6818 maintenance + runCloseStep); new **P0c** Close-step timeout storm |
| `references/subsystem-stress-points.md` | CLI table + 2026-07 surfaces table with workflow links; define-agent failure modes |
| `.agents/.../SKILL.md` + `.claude/.../SKILL.md` | Category 09 count 6; total 35; three new rows |
| `.codex/.../SKILL.md` | Additive note only (control-plane SKILL v3 kept; workflows/refs mirrored) |

## Roots kept in sync

- `C:\CodeProjects\codeNERD\.agents\skills\stress-tester\`
- `C:\CodeProjects\codeNERD\.claude\skills\stress-tester\`
- `C:\CodeProjects\codeNERD\.codex\skills\stress-tester\`

## Absolute paths touched

### New
- `C:\CodeProjects\codeNERD\.agents\skills\stress-tester\references\workflows\09-cli-workspace-matrix\one-shot-cli-exit.md`
- `C:\CodeProjects\codeNERD\.agents\skills\stress-tester\references\workflows\09-cli-workspace-matrix\dual-llm-routing.md`
- `C:\CodeProjects\codeNERD\.agents\skills\stress-tester\references\workflows\09-cli-workspace-matrix\define-agent-flags.md`
- (same three under `.claude\skills\stress-tester\...` and `.codex\skills\stress-tester\...`)

### Updated
- `...\references\workflows\09-cli-workspace-matrix\full-cli-surface.md` (all three roots)
- `...\references\panic-catalog.md` (all three roots)
- `...\references\subsystem-stress-points.md` (all three roots)
- `C:\CodeProjects\codeNERD\.agents\skills\stress-tester\SKILL.md`
- `C:\CodeProjects\codeNERD\.claude\skills\stress-tester\SKILL.md`
- `C:\CodeProjects\codeNERD\.codex\skills\stress-tester\SKILL.md` (additive 09 note only)

## Verification

- Each root `09-cli-workspace-matrix/` has **6** workflow files
- Existing preserved: workspace-isolation, full-cli-surface, polyglot-app-vehicle

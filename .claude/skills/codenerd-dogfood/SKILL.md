---
name: codenerd-dogfood
description: Systematically dogfood codeNERD through its own CLI to exercise and then upgrade every component. Use when driving nerd run / nerd campaign / chat mode against codeNERD's own source to surface real defects, when deciding how to exercise a specific subsystem via the CLI, or when recording what a self-audit found and how it was fixed. Built incrementally as understanding of codeNERD deepens.
---

# codeNERD Dogfood

Drive codeNERD against its own codebase, through its own CLI, to surface real
defects and upgrade every component. This is a **development methodology**, not
just a test: pointing a `nerd` campaign at codeNERD's source turns its own
checkpoint reviewer into a defect-finder aimed at its own executor — and the
critiques are correct.

## The core principle — the self-sharpening loop

**Each fix upgrades the instrument that finds the next bug.** Observed live
(runs 12→13): while the phase-checkpoint reviewer was blind (every phase failed
"no durable outputs"), failures were opaque. The moment research/audit tasks
persisted their findings to disk, the reviewer could *read* them and started
high-fidelity per-task critique — which immediately exposed two deeper bugs
(empty-response hollow success, `/verify` running only `go build`) that had been
invisible. Fix the thing it can't see yet, and it will see the next thing.

So: **read a failing checkpoint verdict as a genuine code review of the executor.**
It names the failing task and why. Fix the executor gap it points at — not the
reviewer.

## The per-component loop (two phases, per the standing goal)

For every component: **exercise it through the CLI, then upgrade it.**

1. **Exercise** — run a real `nerd` CLI invocation that puts the component on the
   live path (see [references/cli-surface.md](references/cli-surface.md)). Clear
   logs first; watch logs live as they populate (Monitor on `.nerd/logs/*.log`).
2. **Observe** — read the checkpoint verdicts and the shard/session/tools logs.
   Distinguish infra failures (tool-not-found, timeouts) from substantive verdicts
   (the reviewer completed and judged the work).
3. **Diagnose** — find the *executor* gap the failure points at, not a surface
   symptom. Trace the fact/task flow to the handler that misbehaved.
4. **Upgrade** — make the smallest correct fix. Add a unit test. `go build` +
   `go test ./internal/<pkg>/`. Conventional commit + push.
5. **Re-verify** — rebuild `nerd.exe`, clear logs, re-run the same CLI exercise.
   Confirm the specific verdict improved on merit. Expect a *deeper* gap to surface.

Update [references/component-ledger.md](references/component-ledger.md) after each
loop: what was exercised, what was found, what was fixed, what's still open.

## Build / run cheatsheet

```powershell
# Build with sqlite-vec (PowerShell)
if (Test-Path .\nerd.exe) { Remove-Item .\nerd.exe -Force }
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

- **Exercise a subsystem via a self-audit campaign:**
  `nerd campaign start "Audit internal/<pkg> for correctness, safety, and resource-lifecycle issues, then produce a short ranked risk report." --type audit --timeout 30m`
- **Exercise the OODA loop / coder path:** `nerd run "<instruction>"`
- **Raw git commit path (no LLM):** `nerd commit "<msg>"`
- Rebuild is required after editing `.mg` policy (go:embed) and after any Go change.
- `nerd.exe` is locked while running — never `Remove-Item` it mid-run; wait for exit.

## Standing constraints (non-negotiable)

- **Incremental, not sweeping.** Build on what works. Small surgical fixes with
  tests; no big refactors. (Steve, 2026-07-14.)
- **Live tests only.** No stubs, no test-mode flags, no dual paths. Whatever the
  live config produces is what the test runs against.
- **NEVER `Write` `.nerd/config.json`.** Use Edit anchored on a unique string; no
  backup-and-restore. See `.claude/rules/never-overwrite-nerd-config.md`.
- **Clear logs between runs; watch logs live** as the new run populates. If the
  log-clear glob is guard-blocked, archive by moving files instead.
- **Conventional commits; push to GitHub regularly.** Branch off `main` first.
- The stack is Gemini/Grok per the live `.nerd/config.json` — do not assume.

## Grading rubric — the 10 campaign contracts (A+ target)

Used to grade the campaign orchestrator specifically; a useful shape for any
long-horizon component. (1) decomposition valid + kernel-validated; (2) persisted
state JSON per schema; (3) phases execute in dependency order; (4) checkpoints
created + verified; (5) context paging respected; (6) event/progress stream;
(7) durable journal; (8) coherent terminal state; (9) no panic / crash-dump /
clean exit; (10) intelligence + risk gating logged.

## See also

- [references/component-ledger.md](references/component-ledger.md) — the living
  per-component status matrix (exercise command, findings, fixes, open gaps).
- [references/cli-surface.md](references/cli-surface.md) — how to exercise each
  component through the CLI.
- `.claude/skills/stress-tester/` — the adversarial/chaos workflows (sibling skill).
- `.claude/skills/log-analyzer/` — parse `.nerd/logs/*` into Mangle facts for query.

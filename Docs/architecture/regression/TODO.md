# regression — TODO

> Last verified against codebase: **2026-08-15**  
> Prioritized backlog for `internal/regression` and its hosts. **Docs-only rebuild did not implement these.**

---

## P0 — Honesty & wiring

- [x] **Wire one real consumer** — prefer `nerd regression run` or a campaign assault optional stage that calls `LoadBattery`/`RunBattery`.
- [x] **Reconcile package comment** — until wired, change “can be run as part of Nemesis gauntlets” to “intended for” or implement the hook.
- [x] **Decide empty-suite policy** for any host (vacuous pass vs config error).

---

## P1 — Operator usefulness

- [x] Example `battery.yaml` for codeNERD workspace (build + `go test ./internal/regression/...` smoke).
- [~] Optional seed from `nerd init` under `.nerd/regression/`. `regression.Seed` is implemented, tested and used by `nerd regression init`; the one-line call from `internal/init.runPhase1DirectorySetup` is still open (see 08-WIRING §2.5).
- [x] Print-friendly summary helper or CLI table (pass/fail/duration).
- [x] Persist last run under `.nerd/regression/runs/` (host-side OK).

---

## P2 — Tests & robustness

- [x] Unit: missing file, bad YAML, timeout, empty command, workdir, multi-task success.
- [x] Document runtime dependency on `powershell` / `bash` — 07-DEPENDENCY-MAP §1.4 and the package doc, enforced by `RequiredShell`/`CheckShell` and a preflight in `RunBatteryWithOptions`.
- [x] Consider `bash --noprofile --norc` (or equivalent) for more deterministic Unix runs — **behavior change**, needs decision.

---

## P3 — Safety (only if agent-facing)

- [x] VirtualStore action `run_regression_battery` — **decided NO**. An
  allowlisted battery action launders every `blocked_pattern` past
  `dangerous_content/2`, which only reads an action's target and payload while
  a battery's commands live in a file. Rationale recorded at the decision point
  in `internal/core/defaults/policy/regression_battery.mg` and in
  09-SAFETY §4.1. No handler routes the action.
- [x] Mangle `Decl` + `permitted(...)` rules — `schemas_safety.mg` SECTION 24
  declares the seven predicates; `policy/regression_battery.mg` registers the
  action as `requires_permission` (→ `dangerous_action`, default deny) and
  derives `regression_battery_permitted` / `regression_battery_refused` from
  the battery's projected task commands.
- [x] Never expose unrestricted shell battery run without policy — enforced by
  `TestBatteryPolicy_RunBatteryAction_ShouldBeDangerousAndNotAllowlisted`,
  which fails if the action is ever added to `safe_action`.

---

## P4 — Richness (optional)

- [x] `RunOptions{FailFast bool}` default true.
- [x] Optional `expect_contains` / `expect_exit` on `Task`.
- [x] Honor or validate `Version`.
- [x] Structured logging category `regression`.
- [x] Result JSON tags for easy serialization.

---

## Explicit non-goals (near term)

- [ ] ~~Parallel task scheduler~~  
- [ ] ~~LLM failure interpretation inside package~~  
- [ ] ~~Merging Nemesis armory into this package~~  
- [ ] ~~Deleting package solely for zero importers~~ (wire first)

---

## Done (as of this corpus)

- [x] Library: load YAML, run shell, fail-fast, timeouts, default path  
- [x] Unit tests for core public behaviors  
- [x] Architecture corpus rebuild (2026-07-13)

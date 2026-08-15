# regression — TODO

> Last verified against codebase: **2026-07-13**  
> Prioritized backlog for `internal/regression` and its hosts. **Docs-only rebuild did not implement these.**

---

## P0 — Honesty & wiring

- [x] **Wire one real consumer** — prefer `nerd regression run` or a campaign assault optional stage that calls `LoadBattery`/`RunBattery`.
- [x] **Reconcile package comment** — until wired, change “can be run as part of Nemesis gauntlets” to “intended for” or implement the hook.
- [x] **Decide empty-suite policy** for any host (vacuous pass vs config error).

---

## P1 — Operator usefulness

- [x] Example `battery.yaml` for codeNERD workspace (build + `go test ./internal/regression/...` smoke).
- [ ] Optional seed from `nerd init` under `.nerd/regression/`.
- [x] Print-friendly summary helper or CLI table (pass/fail/duration).
- [x] Persist last run under `.nerd/regression/runs/` (host-side OK).

---

## P2 — Tests & robustness

- [x] Unit: missing file, bad YAML, timeout, empty command, workdir, multi-task success.
- [ ] Document runtime dependency on `powershell` / `bash`.
- [x] Consider `bash --noprofile --norc` (or equivalent) for more deterministic Unix runs — **behavior change**, needs decision.

---

## P3 — Safety (only if agent-facing)

- [ ] VirtualStore action `run_regression_battery`.
- [ ] Mangle `Decl` + `permitted(...)` rules.
- [ ] Never expose unrestricted shell battery run without policy.

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

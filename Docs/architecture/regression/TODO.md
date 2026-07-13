# regression — TODO

> Last verified against codebase: **2026-07-13**  
> Prioritized backlog for `internal/regression` and its hosts. **Docs-only rebuild did not implement these.**

---

## P0 — Honesty & wiring

- [ ] **Wire one real consumer** — prefer `nerd regression run` or a campaign assault optional stage that calls `LoadBattery`/`RunBattery`.
- [ ] **Reconcile package comment** — until wired, change “can be run as part of Nemesis gauntlets” to “intended for” or implement the hook.
- [ ] **Decide empty-suite policy** for any host (vacuous pass vs config error).

---

## P1 — Operator usefulness

- [ ] Example `battery.yaml` for codeNERD workspace (build + `go test ./internal/regression/...` smoke).
- [ ] Optional seed from `nerd init` under `.nerd/regression/`.
- [ ] Print-friendly summary helper or CLI table (pass/fail/duration).
- [ ] Persist last run under `.nerd/regression/runs/` (host-side OK).

---

## P2 — Tests & robustness

- [ ] Unit: missing file, bad YAML, timeout, empty command, workdir, multi-task success.
- [ ] Document runtime dependency on `powershell` / `bash`.
- [ ] Consider `bash --noprofile --norc` (or equivalent) for more deterministic Unix runs — **behavior change**, needs decision.

---

## P3 — Safety (only if agent-facing)

- [ ] VirtualStore action `run_regression_battery`.
- [ ] Mangle `Decl` + `permitted(...)` rules.
- [ ] Never expose unrestricted shell battery run without policy.

---

## P4 — Richness (optional)

- [ ] `RunOptions{FailFast bool}` default true.
- [ ] Optional `expect_contains` / `expect_exit` on `Task`.
- [ ] Honor or validate `Version`.
- [ ] Structured logging category `regression`.
- [ ] Result JSON tags for easy serialization.

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

# regression — Architectural Principles

> Last verified against codebase: **2026-07-13**  
> Binding for `internal/regression` changes. Package-specific; not a restatement of repo-wide AGENTS.md alone.

---

## P1 — Library purity

**Statement:** The package loads configuration and runs tasks. It does not boot Cortex, import shards, call LLMs, or register global hooks.

**Evidence:** Imports are stdlib + `yaml.v3` only (`battery.go`).

**Rule:** New features that need kernel/VS/prompt belong in hosts, not here.

---

## P2 — YAML is the contract

**Statement:** Operator-visible suite definition is YAML on disk, not Go-hardcoded task lists inside the library.

**Evidence:** `Battery`/`Task` yaml tags; `LoadBattery`.

**Rule:** Prefer extending the YAML schema (versioned) over embedding task suites in Go.

---

## P3 — Fail-fast by default

**Statement:** First hard failure stops the suite to bound latency (gauntlet-friendly).

**Evidence:** break on `!res.Success` with explicit comment.

**Rule:** Changing default to “run all” requires an explicit option and docs; do not flip silently.

---

## P4 — Timeouts are mandatory in practice

**Statement:** Every shell task runs under a deadline (task default 5 minutes, nested under parent `ctx`).

**Evidence:** `context.WithTimeout` + `CommandContext`.

**Rule:** Never spawn unbounded shell without a context. New task types must honor `ctx`.

---

## P5 — Results, not panics; task errors are data

**Statement:** Task failures populate `Result` and do not necessarily surface as `RunBattery`’s `error` return.

**Evidence:** Current `RunBattery` returns `nil` error after partial failure; `Result.Success`/`Error` carry outcomes. Load/parse failures use the error return.

**Rule:** Hosts must check `[]Result`. Do not assume `err != nil` means “a task failed.” If dual-channel error is confusing, document or introduce `ErrFailedTasks` carefully.

---

## P6 — Shell is the universal escape hatch

**Statement:** v1 task model is `shell`. Unknown types fail closed (task failure), not skipped as success.

**Evidence:** default switch branch sets `Success=false`.

**Rule:** Unknown type ⇒ failure. Empty type ⇒ shell (documented default). New types need tests.

---

## P7 — Workspace path convention under `.nerd`

**Statement:** Canonical battery lives at `{workspace}/.nerd/regression/battery.yaml`.

**Evidence:** `DefaultBatteryPath`.

**Rule:** Do not invent a second default path without migration. Optional alternate paths are call-site parameters to `LoadBattery`, not competing globals.

---

## P8 — Cross-platform shell selection is explicit

**Statement:** Windows uses PowerShell without profile; Unix uses login bash. Command is delivered via stdin.

**Evidence:** `runShell` `runtime.GOOS` branch.

**Rule:** Document OS differences. Changes to shell flags are behavioral and need tests on both families when feasible.

---

## P9 — Safety is host-owned until an agent surface exists

**Statement:** This package will run whatever command is in the YAML. Trust boundary is “who can write the battery and who can call RunBattery.”

**Evidence:** No allowlist, no sandbox.

**Rule:** Agent-exposed execution **must** go through VirtualStore + constitutional `permitted(...)`. Do not add a secret backdoor CLI that bypasses policy for untrusted input.

---

## P10 — Prefer wiring over deletion

**Statement:** Absence of importers is a **wiring gap**, not proof of dead code.

**Evidence:** Package comment + zero reverse imports; repo contract in AGENTS.md / harness rules.

**Rule:** Before removing the package, wire a consumer or record an explicit deprecation decision in this corpus.

---

## P11 — No prompt atoms here

**Statement:** Failure interpretation for humans/agents belongs in articulation/prompt layers, not in shell harness code.

**Rule:** Do not grow LLM strings inside `internal/regression`.

---

## P12 — Keep the public surface small

**Statement:** Three types + three functions is a feature.

**Rule:** Resist framework growth (plugins, DSLs, parallel schedulers) unless a real host demands it.

# regression — Wiring and Integration

> Last verified against codebase: **2026-08-15**  
> Honest status: **library implemented; CLI wired; agent path deliberately closed**

---

## 1. Registration surfaces

| Surface | Wired? | Evidence |
|---------|--------|----------|
| `go` package build | Yes | `internal/regression` compiles |
| CLI Cobra command | **Yes** | `cmd/nerd/cmd_regression.go` — `nerd regression run\|init\|list` |
| Chat slash command | No | — |
| Session boot / Cortex | No | Not in boot assembly |
| Shard registration | No | Not in `internal/shards/registration.go` |
| VirtualStore route | **No, by decision** | See [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) §4.1 |
| Mangle policy corpus | **Yes** | `defaults/policy/regression_battery.mg`, `defaults/schemas_safety.mg` SECTION 24 |
| Prompt atoms | No | N/A |
| Campaign assault stages | No | Own runner in `cmd/nerd/chat/campaign_assault.go` |
| Nemesis gauntlet | No | Comment-only intent in package docs |
| `nerd init` template | **Partial** | `regression.Seed` implemented and tested; `internal/init` call site pending — §2.5 |

---

## 2. How a host would wire it (design sketch)

This section is **integration design**, not current code.

### 2.1 CLI (simplest product wire)

```
nerd regression run [--workspace W] [--battery path] [--workdir D]
  → DefaultBatteryPath or flag
  → LoadBattery
  → RunBattery(ctx, b, workdir)
  → print table of Results; exit 1 if any !Success
```

No kernel required.

### 2.2 Campaign assault stage

Assault already runs staged shell/`go test` work. A stage could:

1. If `DefaultBatteryPath` exists, `LoadBattery` + `RunBattery`.  
2. Persist `[]Result` under `.nerd/campaigns/<id>/assault/`.  
3. Fail the stage on first battery failure (aligns with fail-fast).

### 2.3 Agent action (constitutional path) — CLOSED

This path is **not** taken, and the reasoning is recorded in
[09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) §4.1 and at the
decision point in `internal/core/defaults/policy/regression_battery.mg`. In
short: the constitution's content gate reads an action's target and payload, a
battery's target is a path, and writing files is already permitted — so an
allowlisted battery action is a laundering channel around every
`blocked_pattern`.

`/run_regression_battery` is registered `requires_permission` (→
`dangerous_action`, default deny) and no handler routes it. A host that ever
adds one must satisfy **both** gates:

```
host loads battery
  → regression.PolicyFacts(path, battery)     # tasks first, declaration last
  → kernel.Assert(...)
  → query regression_battery_permitted(Path)  # content gate
  → AND permitted(/run_regression_battery, …) # constitutional gate
  → only then regression.RunBatteryWithOptions
```

### 2.5 `nerd init` seed

`regression.Seed(workspace)` writes the starter battery to
`.nerd/regression/battery.yaml` when the workspace has none, returns
`(path, created, err)`, and never overwrites an existing file — so it is safe to
call unconditionally, including under `nerd init --force`. `nerd regression init`
already uses it (and turns `created == false` into an explicit error, which is
right for a command the operator typed by name).

Remaining wire, one call in `internal/init`'s
`runPhase1DirectorySetup` after `createDirectoryStructure`:

```go
if path, created, err := regression.Seed(i.config.Workspace); err != nil {
    result.Failures = append(result.Failures, fmt.Sprintf("seed regression battery: %v", err))
} else if created {
    result.FilesCreated = append(result.FilesCreated, path)
}
```

### 2.4 Nemesis gauntlet preflight

Package comment’s intended use:

1. Before expensive attack generation, run workspace battery.  
2. Fail fast if environment is already broken.  
3. Optionally append armory-derived shell checks into a dynamic `Battery` in memory (no YAML required if host constructs `[]Task` directly — types are exported).

---

## 3. Fact-flow integration (current)

```
user_intent → kernel → next_action → VirtualStore → articulation
                         ▲
                         │  /run_regression_battery is dangerous_action;
                         │  no handler routes it. Closed by decision.
                         │
             regression.PolicyFacts ──► regression_battery_task/declared
                                            │
                                            ▼
                             regression_battery_permitted / _refused
                             (a gate for a future host, not a route)
```

The operator path is the live one:

```
nerd regression run → LoadBattery → RunBatteryWithOptions → SaveRun → FormatSummary
```

---

## 4. Workspace filesystem contract

| Path | Role | Created by package? |
|------|------|---------------------|
| `{ws}/.nerd/regression/battery.yaml` | Canonical suite | **Yes** — `Seed` writes it when absent |
| `{ws}/.nerd/regression/runs/` | Run records (`<UTC timestamp>.json`) | **Yes** — `SaveRun` / `RunsDir` |

---

## 5. Wiring gap classification

Per repo discipline (“wiring before deletion”):

| Classification | Rationale |
|----------------|-----------|
| **Wired** | `cmd/nerd/cmd_regression.go` is a real production caller |
| Residual risk | Only one host; the assault-stage and `nerd init` edges remain open |

`AUDIT.md` marks `internal/regression` as **clean** (no known defects in isolation).

---

## 6. Integration test coverage

| Test | What it proves |
|------|----------------|
| `internal/regression/policy_test.go` | The constitutional gate, against the real corpus files: clean battery permitted, laundered battery refused, empty/undeclared battery denied, action is `dangerous_action` and absent from `safe_action`, seeded template passes its own policy |
| `internal/regression/seed_test.go` | Seed writes a loadable battery, never overwrites, and the shell preflight fails the run rather than every task |
| `cmd/nerd/cmd_regression_test.go` | The CLI leaf commands end to end in a temp workspace |

Still absent: an assault stage with a battery, and a VirtualStore routing test
(there is nothing to route — see §2.3).

---

## 7. Checklist for the first wire

1. ~~Choose host (CLI vs assault).~~ CLI.
2. ~~Define exit/pass semantics for empty suite and missing file.~~ Config error; `run` points at `init`.
3. ~~Log results~~ — `logging.CategoryRegression`, see [11-OBSERVABILITY.md](11-OBSERVABILITY.md).
4. ~~If agent-facing: Mangle `Decl` + `permitted` rules.~~ Declared and gated; the action itself stays closed (§2.3).
5. ~~Update package comment to match reality.~~
6. ~~Add integration test with temp `battery.yaml`.~~
7. ~~Refresh this document's wiring table.~~

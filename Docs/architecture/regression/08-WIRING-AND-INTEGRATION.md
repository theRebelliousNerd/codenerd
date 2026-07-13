# regression — Wiring and Integration

> Last verified against codebase: **2026-07-13**  
> Honest status: **library implemented; product wiring absent**

---

## 1. Registration surfaces

| Surface | Wired? | Evidence |
|---------|--------|----------|
| `go` package build | Yes | `internal/regression` compiles |
| CLI Cobra command | **No** | No `cmd/nerd` references |
| Chat slash command | **No** | — |
| Session boot / Cortex | **No** | Not in boot assembly |
| Shard registration | **No** | Not in `internal/shards/registration.go` |
| VirtualStore route | **No** | No action handler |
| Mangle policy corpus | **No** | No predicates |
| Prompt atoms | **No** | N/A |
| Campaign assault stages | **No** | Own runner in `cmd/nerd/chat/campaign_assault.go` |
| Nemesis gauntlet | **No** | Comment-only intent in package docs |
| `nerd init` template | **No** | No `.nerd/regression` seed |

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

### 2.3 Agent action (constitutional path)

```
user asks to run battery
  → perception → user_intent
  → kernel derives next_action(run_regression_battery, …)
  → permitted(run_regression_battery, …) required
  → VirtualStore executes → regression.RunBattery
  → assert structured results / feed articulation
```

**Do not** let the model shell out by writing YAML and calling RunBattery without policy.

### 2.4 Nemesis gauntlet preflight

Package comment’s intended use:

1. Before expensive attack generation, run workspace battery.  
2. Fail fast if environment is already broken.  
3. Optionally append armory-derived shell checks into a dynamic `Battery` in memory (no YAML required if host constructs `[]Task` directly — types are exported).

---

## 3. Fact-flow non-integration (current)

```
user_intent → kernel → next_action → VirtualStore → articulation
                         ▲
                         │
              internal/regression does NOT enter here
```

---

## 4. Workspace filesystem contract

| Path | Role | Created by package? |
|------|------|---------------------|
| `{ws}/.nerd/regression/battery.yaml` | Canonical suite | **No** — path helper only |
| `{ws}/.nerd/regression/runs/` | Potential artifacts | **Not defined in code** |

Hosts must create directories if they want persistence.

---

## 5. Wiring gap classification

Per repo discipline (“wiring before deletion”):

| Classification | Rationale |
|----------------|-----------|
| **Dormant integration** | Implementation complete enough to call; no callers |
| Risk | Bit-rot, comment drift, false “feature exists” signals |
| Preferred fix | Add one caller (CLI preferred for observability) |
| Acceptable alt | Deprecate + delete after explicit decision |

`AUDIT.md` marks `internal/regression` as **clean** (no known defects in isolation). Clean ≠ wired.

---

## 6. Integration test gap

There is no cross-package test that:

- boots CLI and runs a battery, or  
- runs assault with a battery stage, or  
- asserts VirtualStore routing.

Only package-local unit tests exist.

---

## 7. Checklist for the first wire

1. Choose host (CLI vs assault).  
2. Define exit/pass semantics for empty suite and missing file.  
3. Log results (see [11-OBSERVABILITY.md](11-OBSERVABILITY.md)).  
4. If agent-facing: Mangle `Decl` + `permitted` rules.  
5. Update package comment to match reality.  
6. Add integration test with temp `battery.yaml`.  
7. Refresh this document’s wiring table.

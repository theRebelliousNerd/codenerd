# regression — Vision

> Last verified against codebase: **2026-07-13**  
> Status: Target architecture vision for `internal/regression`  
> Grounded in: existing library + package comment intent + codeNERD north star

---

## 1. Product role

**Workspace regression batteries** are the operator’s cheap, declarative “is the agent’s environment still healthy?” check. They sit between:

- **Unit tests** (`go test`) — too package-local, too slow to always run as a full tree  
- **Campaign assault / Nemesis** — heavy, multi-stage, adversarial  

A battery is a **short, ordered, YAML-defined gate** that can run:

- after agent patches (pre-merge smoke)
- at gauntlet entry/exit
- on a schedule or manual CLI
- as a campaign stage without inventing a new runner each time

---

## 2. Target experience

### 2.1 Operator

```text
.nerd/regression/battery.yaml   # checked into repo or workspace template
nerd regression run             # future CLI — not implemented
# or: campaign stage "regression_battery"
# or: Nemesis gauntlet preflight
```

Results printed and optionally persisted under `.nerd/regression/runs/…`.

### 2.2 Agent (executive path)

If the model *suggests* “run the regression battery,” the **kernel** must derive a `next_action` that is `permitted(...)`. VirtualStore executes `RunBattery`. Results become facts or structured observations — the model narrates; logic decides.

### 2.3 Human-only path

`LoadBattery` + `RunBattery` remain usable as a library for scripts and tests without booting Cortex.

---

## 3. Target architecture

```
                    ┌─────────────────────────┐
                    │  battery.yaml (workspace)│
                    └───────────┬─────────────┘
                                │ LoadBattery
                                ▼
┌──────────────┐      ┌─────────────────────┐      ┌──────────────────┐
│ CLI / campaign│─────►│ internal/regression  │─────►│ []Result         │
│ Nemesis stage │      │ RunBattery / shell   │      │ (+ optional sink)│
│ VS action     │      └─────────────────────┘      └────────┬─────────┘
└──────────────┘                                             │
                                                             ▼
                                              optional: facts / logs / artifacts
```

### Principles for growth

1. **Keep the core dumb** — load YAML, run tasks, return results.  
2. **Hosts own policy** — permissions, when to run, what to do on fail.  
3. **Extend task types sparingly** — prefer shell for generality; add typed tasks only when shell is error-prone (e.g. structured `go_test` with package list).  
4. **Fail-fast default, report-all option** — gauntlet latency vs CI completeness.  
5. **No LLM inside the package** — ever.

---

## 4. Success criteria (vision)

| Criterion | Measure |
|-----------|---------|
| Adopted | ≥1 in-tree caller (CLI or campaign or Nemesis) |
| Safe | Agent path goes through `permitted` |
| Useful | Default battery template for new workspaces (`nerd init`) |
| Honest | Package comment matches wiring |
| Observable | Results land in a log category or run artifact |
| Stable | API remains small; version field becomes real if schema changes |

---

## 5. Non-goals

- Replacing `go test ./…` as the primary verification bar.  
- Becoming a general workflow engine (no DAGs, no parallel stages in v1 vision).  
- Embedding Nemesis attack generation.  
- Fuzzy natural-language task specs (use structured YAML only).  
- Remote/distributed runners.

---

## 6. Relationship to adjacent visions

| System | Boundary |
|--------|----------|
| Campaign assault | Long-horizon multi-stage stress; **may call** regression as one stage |
| Nemesis | Adversarial patch breaking; **may call** regression as regression suite of known weak spots |
| `internal/testing` | Go test helpers; not YAML batteries |
| Prompt JIT | Unrelated unless a future atom documents “how to interpret battery failures” for articulation |

---

## 7. Near-term vision (pragmatic)

Given current code maturity, the **near-term** win is not a large redesign:

1. Keep `Battery`/`Task`/`Result` stable.  
2. Add one thin host (CLI subcommand or assault stage).  
3. Ship an example `battery.yaml` for the codeNERD workspace (build + package smoke).  
4. Soften package comment until wiring lands, or land wiring and keep the comment.

That path honors “logic as executive” without overbuilding.

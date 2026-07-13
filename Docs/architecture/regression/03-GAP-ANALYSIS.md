# regression — Gap Analysis

> Last verified against codebase: **2026-07-13**  
> Compared: package comment + [01-VISION.md](01-VISION.md) vs `internal/regression/`

---

## 1. Spec vs reality matrix

| Capability | Vision / comment | Reality | Gap? |
|------------|------------------|---------|------|
| YAML load | Required | `LoadBattery` | No |
| Shell tasks | Required | `type=shell` | No |
| Ordered execution | Required | for-loop | No |
| Fail-fast | Gauntlet latency | Implemented | No |
| Timeouts | Bounded | Default 5m + `timeout_sec` | No |
| Default path under `.nerd` | Convention | `DefaultBatteryPath` | No |
| Unit tests | Quality bar | 5 tests | Partial (edge gaps) |
| Manual CLI | Operator UX | **Missing** | **Yes** |
| Nemesis gauntlet hook | Package comment | **Missing** | **Yes** |
| Campaign assault stage | Adjacent system | **Missing** | **Yes** |
| VirtualStore action | Agent path | **Missing** | **Yes** |
| `permitted(...)` gate | North star | **Missing** | **Yes** (if agent-facing) |
| Result persistence | Operator UX | **Missing** | **Yes** |
| Expected output / exit code config | Richer gates | Exit 0 only | **Yes** (optional) |
| Task env / cwd overrides | Flexibility | Suite-level cwd only | Partial |
| Schema versioning | `version` field | Loaded, unused | Partial |
| Logging / metrics | Glass-box | **Missing** | **Yes** |
| Workspace template battery | Onboarding | No `.nerd/regression` | **Yes** |
| Task types beyond shell | Extensibility | Rejected with error | Intentional for now |

---

## 2. Priority ranking

### P0 — Integrity

| Gap | Why | Suggested direction |
|-----|-----|---------------------|
| Comment claims Nemesis integration | Doc/code drift; confuses architects | Wire a caller **or** reword comment to “intended for” |
| Zero importers | Dead code risk; feature invisible | Thin CLI or campaign stage |

### P1 — Product usefulness

| Gap | Why | Suggested direction |
|-----|-----|---------------------|
| No CLI | Operators cannot run without custom Go | `nerd regression run [--path]` |
| No example battery | Convention unproven in-tree | Example under docs or `.nerd/regression/battery.example.yaml` |
| No result summary helper | Hosts reimplement pass/fail fold | `Summarize([]Result) (passed, failed int)` optional |

### P2 — Safety & executive integration

| Gap | Why | Suggested direction |
|-----|-----|---------------------|
| Unrestricted shell | Agent path would be dangerous | VS action + constitutional policy |
| No fact emission | Kernel cannot reason about suite outcomes | Host asserts `regression_task_result(...)` if needed |

### P3 — Richness

| Gap | Why | Suggested direction |
|-----|-----|---------------------|
| No golden stdout | Exit 0 alone misses wrong-but-successful cmds | Optional `expect_contains` |
| Fail-fast only | CI wants full report | `RunOptions{FailFast bool}` |
| Login bash / profile variance | Non-determinism on Unix | Document; consider `bash --noprofile --norc` |
| Timeout tests | Untested path | Unit test with `sleep` + short timeout |

---

## 3. Non-gaps (do not thrash)

| Item | Why it is not a defect |
|------|------------------------|
| Outside OODA fact-flow | Correct for a library harness |
| Single file | Matches “lightweight” |
| No Mangle in package | Hosts assert facts if needed |
| No parallel task execution | Fail-fast sequential is simpler and safer for shell |
| Version unused | Fine until breaking schema change |
| Not campaign assault | Different product; may compose later |

---

## 4. Risk of wrong fixes

| Wrong fix | Problem |
|-----------|---------|
| Delete package as unused | Violates wiring-audit rule; useful primitive |
| Embed full Nemesis inside regression | Scope explosion; wrong layer |
| Add LLM “interpret failure” inside package | Breaks executive/creative split |
| Silent continue-on-error default | Breaks gauntlet latency contract |

---

## 5. Completeness heuristic

| Layer | Estimate |
|-------|----------|
| Library core | ~95% |
| Tests | ~70% of desirable edges |
| Product wiring | ~0% |
| Safety envelope for agents | ~0% |
| **Blended “shipping subsystem”** | **~35–45%** |

Earlier auto-inventory “90%” scores measured **file presence**, not **product integration**. This corpus treats wiring as first-class.

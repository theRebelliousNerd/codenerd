# regression — Alignment & Vision Review

> Last verified against codebase: **2026-07-13**  
> Source of truth: `internal/regression/battery.go`, `battery_test.go`  
> Method: score dimensions against codeNERD north star with **file evidence**, not aspiration.

Scoring: **0** = absent/contrary · **1** = thin/partial · **2** = solid for package scope · **3** = exemplary

---

## 1. Dimension scores

| # | Dimension | Score | Evidence |
|---|-----------|------:|----------|
| 1 | Inversion of control (LLM creative / logic executive) | **3** | Pure deterministic load+run; zero model calls (`battery.go` imports: stdlib + yaml only). |
| 2 | Constitutional safety (`permitted`, default deny) | **0** | No Mangle, no policy hook. Shell runs if `RunBattery` is called. Safety is caller-owned. |
| 3 | JIT prompt / atoms discipline | **n/a** | No LLM-facing behavior. Correct non-participation. Score as **2** for “did not invent ad-hoc prompts.” |
| 4 | Fact-flow honesty (user_intent → kernel → VS) | **2** | Correctly **outside** fact-flow; does not fake kernel involvement. Documented as side harness. |
| 5 | Wiring integrity | **0** | Package comment claims Nemesis gauntlet use; **zero** Go importers of `codenerd/internal/regression`. Classic dormant library. |
| 6 | Declarative configuration | **2** | YAML battery with explicit schema tags; path convention under `.nerd/`. Version field unused. |
| 7 | Fail-closed / bounded latency | **2** | Fail-fast on first failure (`battery.go` loop break); per-task context timeout (default 5m). |
| 8 | Observability / glass-box | **0** | No logging category, metrics, or result persistence. |
| 9 | Test grounding | **2** | Five focused unit tests covering load, success, fail-fast, empty, path. Gaps on timeout/IO errors. |
| 10 | Minimal surface / non-bloat | **3** | Single file, three types, three exports + one helper path. Matches “lightweight, optional.” |
| 11 | Cross-platform clarity | **2** | Explicit Windows PowerShell vs Unix bash branches in `runShell`. |
| 12 | Security of effectful execution | **1** | stdin script to real shell is powerful and honest; no sandbox/allowlist — acceptable only if gated upstream. |

**Arithmetic mean of scored dims (excluding pure n/a as 2):** roughly **1.6 / 3** — strong as a **library primitive**, weak as a **productized subsystem**.

---

## 2. North-star narrative

### What aligns

- **Determinism:** Same battery + same environment → same shell invocations. No stochastic LLM step inside the harness.
- **Separation of concerns:** Does not smuggle executive decisions into prompt text.
- **Optional weight:** Can sit unused without affecting boot/OODA hot path (and currently does).

### What misaligns or is incomplete

- **“Gauntlet” product story without wiring** violates the repo’s “look for wiring gaps before calling code unused / deleting” discipline *and* risks doc drift (comment overpromises).
- **Unrestricted shell** is antithetical to constitutional default-deny **if** the harness is ever exposed as an agent-selected action without a `permitted(...)` gate.
- **No structured outcome → kernel facts** means even a successful battery run cannot participate in logic executive memory unless a host asserts facts.

### Verdict

**Keep as a focused library.** Highest-value alignment work is **wiring** (campaign/Nemesis/CLI) plus **optional policy envelope** when agent-driven — not growing a mini test framework inside this package.

---

## 3. Score justification snippets

### Wiring (0)

```
// Package regression ... can be run as part of Nemesis gauntlets or manually
```

Grep of production Go for `codenerd/internal/regression`: no matches outside the package. Campaign assault (`cmd/nerd/chat/campaign_assault.go`) implements its own stages without this import.

### Fail-fast / timeout (2)

```go
// Fail-fast on first hard failure to keep gauntlet latency bounded.
if !res.Success {
    break
}
```

```go
timeout := time.Duration(task.TimeoutSec) * time.Second
if timeout <= 0 {
    timeout = 5 * time.Minute
}
```

### Minimal surface (3)

Only exports: `Battery`, `Task`, `Result`, `LoadBattery`, `RunBattery`, `DefaultBatteryPath`.

---

## 4. Alignment actions (docs → backlog)

Mapped to [TODO.md](TODO.md):

| Priority | Action | North-star win |
|----------|--------|----------------|
| P0 | Wire one real consumer (CLI or assault stage) **or** soft-retire comment | Honesty / wiring |
| P1 | If agent-callable: VirtualStore action + `permitted` | Constitutional safety |
| P2 | Result → optional structured report / facts | Executive memory |
| P3 | Logging category `regression` | Glass-box |

---

## 5. What this review is not

- Not a claim that unit-test “regression guards” elsewhere (`*_test.go` comments saying “regression”) belong to this package — they do not.
- Not a request to fold Nemesis armory into `internal/regression`.

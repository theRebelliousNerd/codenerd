# usage — Alignment & Vision Review

> Last verified: **2026-07-13**  
> Source of truth: `internal/usage/`, reverse-deps under `cmd/nerd/`, `internal/system`, `internal/perception`, `internal/core/shards`

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully aligned, wired, tested |
| 4 | Aligned with minor gaps |
| 3 | Partial — structure present, incomplete coverage |
| 2 | Thin / aspirational in code |
| 1 | Missing or actively conflicting |

## Dimensions

### 1. Inversion of control (LLM creative / logic executive)

**Score: 5**

Usage correctly **does not** try to be executive. It meters LLM work after the fact. No Mangle rules, no action routing, no intent parsing. Evidence: package is pure Go persistence + aggregates (`usage_tracker.go`, `usage_types.go`); zero `.mg` files.

### 2. Constitutional safety (`permitted`, default deny)

**Score: 5 (N/A-as-pass)**

Token accounting is not a privileged effect surface. Reading/writing `.nerd/usage.json` is local workspace telemetry. There is no path from usage data into VirtualStore actions. Safety is **orthogonal**: misuse of token data cannot open file/network actions without other systems.

### 3. Fact-flow placement

**Score: 4**

```
user_intent → kernel → next_action → VirtualStore → articulation
                              ↘ (parallel, not on the critical path)
                         LLM call → Track usage
```

Wiring attaches the tracker to the **same context** that flows into perception and shard execution (`usage.NewContext` in CLI/chat; `WithShardContext` in `manager_spawn.go`). That is correct placement for ambient telemetry. Gap: only the ZAI HTTP client actually calls `Track`; other engines can run without metering (score not lower because the design allows nil-safe no-op).

### 4. JIT / prompt-atom discipline

**Score: 5 (N/A-as-pass)**

No prompts, no atoms, no shard prose. Package stays free of LLM-facing strings.

### 5. Observability & operator transparency

**Score: 3**

Aggregates + TUI page (`cmd/nerd/ui/usage_page.go`) give operators totals by provider/model/shard-type/operation. Gaps:

- `UsageEvent` raw history is defined but **not appended** in `Track`
- `TokenCounts.Cost` is never computed
- UI does not show `BySession`
- No structured log on Track/Save failure (Load errors swallowed in `NewTracker`)

### 6. Wiring completeness

**Score: 3**

| Wire | Status |
|------|--------|
| Boot: `usage.NewTracker(workspace)` in `internal/system/factory.go` | **Live** |
| Cortex field `UsageTracker` | **Live** |
| CLI contexts (`cmd_*`, chat process/campaign) | **Live** `NewContext` |
| Shard spawn metadata | **Live** `WithShardContext` |
| Perception record path | **Partial** — ZAI only |
| Chat session local tracker | **Live** (second construction path in `chat/session.go`) |
| TUI usage page | **Live** |

### 7. Persistence durability

**Score: 3**

JSON file under `.nerd/usage.json` with debounced auto-save. Works for operator dashboards. Not durable enough for billing, multi-process, or crash-consistent accounting (see [12-FAILURE-MODES.md](12-FAILURE-MODES.md)).

### 8. Test alignment

**Score: 4**

Strong unit coverage for aggregates, load edge cases, context helpers, copy safety, corrupt file, non-string context values. Missing: concurrent Track race stress; debounce/timer integration; cross-package “ZAI Track with real context” test inside usage (that lives or should live at perception).

### 9. North-star “logic determines reality”

**Score: 5**

Usage never claims to determine agent behavior. Reality for permissions and plans remains Mangle. Usage reports **what happened**, not what is allowed.

## Composite

| Dimension | Score |
|-----------|------:|
| Inversion of control | 5 |
| Constitutional safety | 5 |
| Fact-flow placement | 4 |
| JIT discipline | 5 |
| Observability | 3 |
| Wiring | 3 |
| Durability | 3 |
| Tests | 4 |
| North-star purity | 5 |
| **Average** | **~4.1** |

## Verdict

**Aligned package with partial metering coverage.** Keep it small and ambient. Priority work is **broadening Track call sites** (all perception engines) and **honest gaps** (events, cost, save races) — not inventing kernel predicates for token budgets unless product requirements demand it.

## Evidence anchors

| Claim | Path |
|-------|------|
| Tracker + persist | `internal/usage/usage_tracker.go` |
| Types + optional Cost/Events | `internal/usage/usage_types.go` |
| Boot construct | `internal/system/factory.go` (~434) |
| ZAI Track | `internal/perception/client_zai.go` (~517–525) |
| Shard context | `internal/core/shards/manager_spawn.go` (~342) |
| UI | `cmd/nerd/ui/usage_page.go` |

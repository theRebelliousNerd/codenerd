# 02 — Current State: `internal/northstar`

> Last verified against codebase: 2026-07-13  
> Status: Living inventory — code-grounded

## 1. Package snapshot

| Metric | Value |
|--------|------:|
| Non-test `.go` files | 4 |
| Test `.go` files | 6 |
| Package-local `.mg` | 0 |
| Approx. non-test lines | ~2,196 |
| Approx. test lines | ~3,135 |
| Exported types (major) | ~18 |
| Test functions (`func Test`) | ~114 |

Package comment (`types.go`): Northstar is a **core system component**, not a user specialist under `.nerd/agents/`; knowledge DB is `.nerd/northstar_knowledge.db`; prompt atoms live under `internal/prompt/atoms/northstar/`.

## 2. File inventory (non-test)

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/northstar/store.go` | 732 | SQLite store: schema, vision CRUD, observations, alignment history, drift, guardian_state |
| `internal/northstar/guardian.go` | 677 | `Guardian` runtime: init, LLM alignment, observe helpers, thresholds, kernel refresh |
| `internal/northstar/observer.go` | 482 | `CampaignObserver`, `TaskObserver`, `BackgroundEventHandler` (+ cycle-safe mirror types) |
| `internal/northstar/types.go` | 305 | Domain model, `Vision.ToFacts()`, defaults, enums |

## 3. File inventory (tests)

| Path | ≈Lines | Focus |
|------|-------:|-------|
| `internal/northstar/guardian_test.go` | 1103 | Init, alignment parse, thresholds, observe, concurrency |
| `internal/northstar/store_test.go` | 710 | Schema ops, vision timestamps, drift resolve, state rollups |
| `internal/northstar/types_test.go` | 623 | JSON round-trips, enum values, default config |
| `internal/northstar/observer_test.go` | 514 | Campaign/task/background handlers |
| `internal/northstar/types_facts_test.go` | 114 | `ToFacts` / priority / impact parsing |
| `internal/northstar/guardian_warn_test.go` | 71 | No-vision warning + idempotency |

## 4. Component map (as implemented)

```
types.go          Vision, Alignment*, Drift*, GuardianConfig/State
     │
store.go          NewStore(nerdDir) → northstar_knowledge.db
     │
guardian.go       NewGuardian(store, config)
     │              SetLLMClient / SetParentKernel / Initialize
     │              CheckAlignment / Observe* / ShouldCheckNow / OnTaskComplete
     │
observer.go       NewCampaignObserver(guardian)
                  NewTaskObserver(guardian, sessionID)
                  NewBackgroundEventHandler(guardian, sessionID)
```

## 5. SQLite schema (Store)

Tables created in `initSchema()`:

| Table | Purpose |
|-------|---------|
| `vision` | Singleton (`id=1`): mission, problem, vision_statement, JSON blobs for lists |
| `observations` | Session-scoped events with relevance + tags/metadata JSON |
| `alignment_checks` | Historical checks with score, result, duration_ms |
| `drift_events` | FK to checks; resolved flag; severity/category |
| `guardian_state` | Singleton rollups: vision_defined, tasks_since_check, overall_alignment, … |
| `ingested_docs` | Path/title/content/summary/relevance/embedding BLOB — **schema only, no package API** |

Pragmas: `sqlpragmas.ProfileHot` + `PRAGMA foreign_keys = ON`.

## 6. Runtime behaviors that exist today

| Behavior | Location | Status |
|----------|----------|--------|
| Load/save vision | `Store` + `Guardian.UpdateVision` | **Implemented** |
| LLM alignment check | `Guardian.CheckAlignment` | **Implemented** (structured parse) |
| No-vision skip | `CheckAlignment` → `AlignmentSkipped` | **Implemented** |
| No-LLM soft pass | score 0.8, `AlignmentPassed` | **Implemented** |
| Periodic check by task count | `OnTaskComplete` + `ShouldCheckNow` | **Implemented** |
| High-impact path glob match | `matchesHighImpactPath` | **Implemented** |
| Keyword relevance | `calculateRelevance` | **Implemented** (not embeddings) |
| Drift on failed/blocked | `persistAlignmentOutcome` | **Implemented** |
| Kernel fact refresh | `refreshKernelFacts` | **Implemented** if kernel set |
| Campaign observer lifecycle | `observer.go` | **Implemented** |
| Background event → assessment | `BackgroundEventHandler.HandleEvent` | **Implemented** |
| TaskObserver | `observer.go` | **Implemented** (library; external use sparse) |
| Doc ingestion API | — | **Not implemented** (table only) |
| Single source with northstar.json | — | **Not implemented** |

## 7. Hotspots

1. **`CheckAlignment`** — central policy of soft vs hard outcomes; all observers funnel here.
2. **`Vision.ToFacts`** — contract with Mangle Decls; incomplete relative to policy corpus (`northstar_serves`, `northstar_supports`, … not emitted).
3. **Dual persistence** with chat wizard / `cmd_northstar.go` (outside package but operator-visible).
4. **Boot asymmetry** — shared boot sets parent kernel; primary boot path may not.

## 8. What is *not* in this package

- Wizard state machine (`cmd/nerd/chat/northstar_wizard.go` et al.)
- Cobra `nerd northstar` tree (`cmd/nerd/cmd_northstar.go`) — JSON/MG oriented
- Prompt atom YAML under `internal/prompt/atoms/northstar/`
- Campaign risk toggle resolution (`internal/campaign/risk_scoring.go`)

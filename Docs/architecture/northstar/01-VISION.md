# 01 — Vision: Northstar Guardian

> Last verified against codebase: 2026-07-13  
> Status: Target architecture (product + package)

## 1. Why Northstar exists

Long-horizon agent work (campaigns, multi-session coding) drifts. Without an explicit project vision:

- The LLM invents priorities from the last prompt.
- Campaign phases optimize locally and miss the mission.
- Policy has no durable “what we are building” facts.

Northstar is the **strategic memory and guardian** for that vision: define once, check continuously, inject into the logic kernel, record drift.

## 2. Target product behavior

### 2.1 For users

1. **Define** mission, problem, vision, personas, capabilities, risks, requirements, constraints (wizard or import).
2. **See** vision in CLI (`nerd northstar show`) and in-session context.
3. **Check** alignment on demand (`/alignment`) and automatically at campaign/phase/high-impact boundaries.
4. **Trust** that blocked campaigns mean real conflict with stated vision, not a silent skip.
5. **Review** history of checks, observations, and open drift.

### 2.2 For the platform

1. Vision is **data**, not only chat history.
2. Vision becomes **Mangle facts** consumable by policy, JIT selectors (`northstar_phase`, injectable context), and campaign risk scoring.
3. Alignment uses the LLM as **judge**, not executor — results are structured (`SCORE`/`RESULT`/…) and persisted.
4. Enforcement strength is **configurable**: advisory vs hard gate (campaign already has toggles; general OODA path remains advisory today).

## 3. Architectural target shape

```
                 ┌─────────────────────────────┐
                 │  Vision definition surface  │
                 │  wizard / import / API      │
                 └──────────────┬──────────────┘
                                │ single write path
                                ▼
                 ┌─────────────────────────────┐
                 │  Northstar Store (SQLite)   │
                 │  vision + obs + checks +    │
                 │  drift + guardian_state     │
                 └──────────────┬──────────────┘
                                │
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                 ▼
        Guardian          Observers           Kernel
        CheckAlignment    Campaign/Task/BG    northstar_* facts
              │                 │
              └────────┬────────┘
                       ▼
              Drift + history (audit)
                       │
                       ▼
              Campaign risk gate / UX
```

## 4. Non-goals (package boundary)

- Not a general RAG document store (table `ingested_docs` is reserved; not a product feature until API exists).
- Not constitutional action policy (`permitted(...)` remains `internal/core` / policy corpus).
- Not campaign orchestration — only observation + gate inputs.
- Not the TUI wizard implementation — package provides domain types and guardian runtime.

## 5. Success criteria

| Criterion | How we would know |
|-----------|-------------------|
| Single vision source of truth | One write path updates both operator view and Guardian |
| Kernel always consistent | After boot and every `UpdateVision`, `northstar_defined` matches store |
| Campaign safety | Protected campaigns cannot start without observer + non-blocked alignment |
| Debuggability | Alignment history queryable; CategoryNorthstar shows check outcomes |
| No dual-file surprise | JSON/MG either generated from DB or removed as authority |

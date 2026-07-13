# store — Gap Analysis

> Last verified: **2026-07-13**  
> Method: vision / north-star claims vs `internal/store` reality.

## Spec vs reality matrix

| Claim | Reality | Gap? |
|-------|---------|------|
| Multi-tier SQLite memory | Implemented in LocalStore schema | No |
| Semantic vector search | Engine + ANN/brute/keyword ladder | No |
| sqlite-vec optional/required by tag | `vec_support_*.go`, `init_vec.go` | No |
| Cold + archival lifecycle | Full API + maintenance | No |
| Knowledge graph + hydrate | Implemented | No |
| World fast/deep cache | Implemented | No |
| Session + compressed state | Implemented | No |
| Prompt atoms on disk | Implemented | No |
| Reasoning traces + reflection | Implemented + worker | No |
| Autopoiesis learnings | LearningStore separate DBs | No |
| Tool journal isolation | ToolStore + cleanup | No |
| Embedded intent corpus | Implemented when baked | Partial in dev without corpus build |
| Unified Store interface | Concrete types only | **Yes (soft)** |
| Auto-heal ANN drift | Warn only; backfill on engine set | **Yes** |
| Complete GetStats | Subset of tables | **Yes (minor)** |
| Cross-DB transactions | Not present | Intentional non-goal |
| Constitutional checks in store | Not present | Correct non-goal |
| Multi-process concurrent writers | Single conn | Intentional limit |
| Remote/blob cold tier | Local SQLite only | Horizon |

## Prioritized gaps

### P1 — Operational correctness

1. **ANN drift reconcile**  
   After warned vec_index failures, no continuous job rebuilds missing rowids. Background `backfillVecIndex` on engine attach helps but is not a periodic healer.

2. **Reflection backlog under heavy load**  
   45s / batch 32 may lag large campaign trace volumes; ops need force re-embed path (exists) but productized scheduling is thin.

### P2 — API ergonomics

3. **No single interface** for LocalStore + LearningStore + ToolStore — complicates tests/mocks outside package (package has mocks for embedding only).

4. **GetStats incomplete** — omit `reasoning_traces`, `prompt_atoms`, `review_findings`, `task_verifications`, etc.

### P3 — Documentation / naming

5. Comments still say “Shards B/C/D” while many more tables exist — mitigated by this corpus, not code rename.

6. Dual ownership of prompt atoms (store vs prompt package) can look like duplication without wiring docs.

## Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| No `.mg` in store | Store is not executive logic |
| Separate tools.db | Explicit design to avoid bloating KB |
| MaxOpenConns=1 | SQLite safety for this process model |
| Keyword fallback without engine | Graceful degrade |
| Embedded corpus missing in dev | Requires build pipeline; fails closed with error |

## Recommended sequencing

1. ANN drift audit command or startup reconcile (P1)  
2. Expand `GetStats` (P2)  
3. Optional thin interfaces at consumer edges only if tests demand (P2)  
4. Keep horizon remote cold store off the critical path

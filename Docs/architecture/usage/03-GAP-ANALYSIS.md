# usage — Gap Analysis

> Last verified: **2026-07-13**; rows marked † re-verified **2026-10-27** against source (see Docs/audits/detailed-usage-wiring-and-verified-efficiency.md, As built).

## Spec vs reality matrix

| Capability | Vision / type surface | Reality in code | Priority |
|------------|----------------------|-----------------|----------|
| Aggregate token totals | Required | **Implemented** (`TotalProject`, `By*`) | — |
| Persist across process restarts | Required | **Implemented** (`Load`/`Save`/`usage.json`) | — |
| Context ambient attach | Required | **Implemented** (`NewContext`/`FromContext`) | — |
| Shard/session attribution | Required | **Implemented** (`WithShardContext` + Track) | — |
| Meter all LLM engines † | Required for trustworthy totals | **Implemented** — every provider adapter reports through `perception.reportUsage` → `usage.RecordFromContext` | — |
| Meter streaming completions † | Vision | **Implemented** — each stream reports once per attempt, before its terminal result | — |
| Raw event history † | Type `UsageEvent` + `UsageData.Events` | **Implemented** — bounded ring behind `WithEventLog` | — |
| Cost estimation † | `TokenCounts.Cost` | **Implemented** — `EstimateCostDetailed` with cache read/write rates; unpriced spend counted in `UnpricedTokens` | — |
| By-shard-name map † | Comment in Track | **Implemented** — `ByShardName` | — |
| UI shows BySession | Stats has map | **UI omits** session table | P2 |
| Atomic file write † | Durability vision | **Implemented** — temp file, fsync, `atomicfile.Replace` | — |
| Debounce correctness | Vision: no lost dirty | Dirty clear race possible | P1 |
| Logging on Load/Save fail | Comment “would log” | **No logging package use** | P2 |
| Cache/reasoning sub-counts with presence † | Accurate token economics | **Implemented** — `types.UsageMetadata` (`Known`, `Partial`), carried to tracker and broker receipt alike; cross-process merge keeps them | — |
| Mangle budget facts | Optional future | **None** (correct non-gap if non-goal) | — |
| Multi-process lock | Fleet vision | **None** | P3 |
| Export CLI verb | Operator vision | No dedicated cobra dump in usage package (UI only) | P3 |

## Priority narratives

### P0 — Incomplete producer coverage (resolved †)

The original finding — one production `Track` site, totals biased toward Z.AI — no longer holds. Every adapter now builds one normalized report per billed attempt and delivers it to both the tracker and the in-flight broker observer. The rationale stands: a producer that skips the report makes operator totals silently wrong, which `TestStreamConformance` and the usage-tracking tests now catch per provider.

### P1 — Persistence races / durability

1. **Dirty re-arm**: while `dirty==true`, additional `Track` calls do not schedule a new timer. After `Save` completes, `dirty` is cleared. A `Track` that lands after the marshal snapshot but before `dirty=false` can leave newer aggregates unsaved until a later Track when dirty is false.  
2. **Non-atomic write**: `os.WriteFile` can leave truncated JSON on crash.  
3. **Unused `autoSaveTimer`**: cannot cancel on process exit; no explicit flush API beyond `Save()`.

### P2 — Incomplete product surface on existing types

Types already advertise Events and Cost. Leaving them empty forever confuses readers and invites “delete unused” mistakes. Either implement bounded event ring + price table **or** document as reserved and keep wiring audit notes.

### P3 — Attribution polish

Shard **name** is read from context but not aggregated. Session table is tracked but not rendered in the TUI page.

## Non-gaps (do not “fix” these)

| Observation | Why it is not a bug |
|-------------|---------------------|
| No Mangle Decl / policy | Package is a sensor, not executive |
| Nil `FromContext` no-op | Intentional ambient pattern |
| Corrupt JSON at boot still returns tracker | Soft-fail keeps agent usable |
| Negative tokens mathematically allowed in `Add` | Documented by tests as mathematical completeness; providers should not send negatives |
| String context keys for shard metadata | Fragile but intentionally shared contract between shards and Track |

## Gap → document pointers

| Gap | See |
|-----|-----|
| Wiring producers | [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) |
| Failure modes | [12-FAILURE-MODES.md](12-FAILURE-MODES.md) |
| Backlog | [TODO.md](TODO.md) |
| Design forks | [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) |

# usage — Gap Analysis

> Last verified: **2026-07-13**

## Spec vs reality matrix

| Capability | Vision / type surface | Reality in code | Priority |
|------------|----------------------|-----------------|----------|
| Aggregate token totals | Required | **Implemented** (`TotalProject`, `By*`) | — |
| Persist across process restarts | Required | **Implemented** (`Load`/`Save`/`usage.json`) | — |
| Context ambient attach | Required | **Implemented** (`NewContext`/`FromContext`) | — |
| Shard/session attribution | Required | **Implemented** (`WithShardContext` + Track) | — |
| Meter all LLM engines | Required for trustworthy totals | **Partial** — only `client_zai.go` calls Track | **P0** |
| Meter streaming completions | Vision | **Unknown / likely missing** — process stream path attaches context but ZAI non-stream path is the only Track site found | **P0** |
| Raw event history | Type `UsageEvent` + `UsageData.Events` | **Not written** | P2 |
| Cost estimation | `TokenCounts.Cost` | **Never set** | P2 |
| By-shard-name map | Comment in Track | **Not implemented** | P3 |
| UI shows BySession | Stats has map | **UI omits** session table | P2 |
| Atomic file write | Durability vision | Direct `WriteFile` | P1 |
| Debounce correctness | Vision: no lost dirty | Dirty clear race possible | P1 |
| Logging on Load/Save fail | Comment “would log” | **No logging package use** | P2 |
| Mangle budget facts | Optional future | **None** (correct non-gap if non-goal) | — |
| Multi-process lock | Fleet vision | **None** | P3 |
| Export CLI verb | Operator vision | No dedicated cobra dump in usage package (UI only) | P3 |

## Priority narratives

### P0 — Incomplete producer coverage

Reverse-dep grep shows **one** production `Track` call site outside tests: `internal/perception/client_zai.go`. Other providers and CLI backends can burn tokens without updating aggregates. Operator totals are **biased toward ZAI HTTP usage**.

Mitigation path (docs only here): every `LLMClient` that receives usage metadata should call `FromContext` + `Track` with a stable `operation` string.

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

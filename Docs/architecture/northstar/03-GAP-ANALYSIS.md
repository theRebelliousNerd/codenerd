# 03 — Gap Analysis: Northstar

> Last verified against codebase: 2026-07-13  
> Status: Living — vision (01) vs current state (02)

## 1. Spec vs reality matrix

| Target capability | Reality | Gap severity | Notes |
|-------------------|---------|--------------|-------|
| Durable vision storage | SQLite `vision` table | **None** | `Store.SaveVision` / `LoadVision` |
| Operator-visible vision | CLI reads `.nerd/northstar.json` | **High** | Dual authority; Guardian may not see wizard vision |
| Wizard → Guardian sync | Not in package | **High** | Chat wizard persists JSON/MG; must separately call `UpdateVision` or share store |
| Mangle fact injection | `refreshKernelFacts` | **Medium** | Only if `SetParentKernel` called; boot paths inconsistent |
| Full predicate fan-out | Partial `ToFacts` | **Medium** | Missing `northstar_serves`, `northstar_supports`, `northstar_addresses`; mitigation loses free text |
| LLM alignment judge | Implemented | **Low** | Inline prompt (not JIT atoms) |
| Embeddings relevance | Keyword match only | **Low–Med** | Comments admit production would use embeddings; table has embedding BLOB unused |
| Campaign hard gate | Observer + risk_scoring | **Low** | Works when observer configured; protected campaigns block if missing |
| General OODA hard gate | Soft only | **Medium** | Background assessments map to proceed/note/clarify/block but do not own `permitted` |
| Alignment history UX | Store APIs only | **Medium** | No CLI `nerd northstar history` over SQLite |
| Doc ingestion | Schema stub | **Low** | `ingested_docs` without writers/readers in package |
| Config model selection | `AlignmentModel` field | **Low** | Unused by Guardian (client is injected) |
| TaskObserver in session loop | Type exists | **Medium** | Campaign path is primary; standard tasks may only hit BackgroundEventHandler if events fire |
| Concurrent safety | Mutex + clone getters | **None** | Tests cover concurrent init/access |

## 2. Non-gaps (do not “fix”)

| Item | Why not a gap |
|------|----------------|
| No package-local `.mg` | Decls live in core defaults / workspace programs; package emits `types.Fact` |
| Soft pass without LLM | Deliberate availability bias so offline/dev boots still run |
| Not implementing wizard UI | Correct layering; chat owns UX |
| Import-cycle mirror types (`ObserverAssessment`) | Intentional; adapter in `session_boot_helpers.go` |

## 3. Priority backlog (doc-level)

### P0 — correctness / trust

1. **Unify vision authority**: document and then implement one write path (JSON→Store or Store→JSON export). Today operators can believe CLI show while Guardian checks empty vision.
2. **Kernel wiring parity**: every boot path that constructs a Guardian should `SetParentKernel` when a kernel exists (shared boot does; primary boot should match).

### P1 — product completeness

3. Align wizard save with `Guardian.UpdateVision` / `Store.SaveVision`.
4. Emit relational facts (`serves`/`supports`/`addresses`) or stop Declaring them as load-bearing.
5. Surface alignment history and active drift via CLI or chat.

### P2 — quality

6. Move alignment system prompt to prompt atoms (JIT-first).
7. Wire or remove `ingested_docs` + embeddings path.
8. Honor or delete `GuardianConfig.AlignmentModel`.

## 4. Honest completion heuristic

| Slice | Approx. |
|-------|---------|
| Core library (store + guardian + observers) | **~90%** of intended library surface |
| Platform integration (single vision, kernel always) | **~55%** |
| Operator UX over SQLite history | **~30%** |
| Full Mangle relational model | **~40%** of Decl surface |

**Do not treat package as pre-implementation.** Code is living and tested; gaps are integration and dual-store product issues.

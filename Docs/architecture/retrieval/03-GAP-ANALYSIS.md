# retrieval — Gap Analysis

> Last verified: **2026-07-13**  
> Method: vision (§01) × current state (§02) × wiring evidence

## 1. Spec vs reality matrix

| Capability | Vision | Reality | Gap severity |
|------------|--------|---------|--------------|
| Keyword extraction from issues | Required | Implemented + chat-seeded | **None** (quality can improve) |
| Sparse FS keyword search | Required | Implemented | **Wiring** (not called in agent loop) |
| Ranked candidates → EDB | Required | Decls exist; no producer from search | **High** |
| Tiered context T1 | Required | Builder + partial seed of mentioned files | **Medium** (seed skips resolve-under-workDir) |
| Tiered context T2–T4 → EDB | Required | Builder only in tests | **High** |
| Multi-lang import expand | Target | Python only | **Medium** |
| Embedding T4 | Target | Placeholder heuristic | **High** (for hybrid RAG) |
| Boot integration | Required | Constructs retriever only | **High** |
| Observability metrics | Target | Printf-style Context logs | **Low–Medium** |
| Index / incremental scan | Scale target | Full walk each keyword | **Medium** |
| Binary/large-file skip | Safety | Not implemented | **Medium** |
| Comment accuracy (ripgrep) | Hygiene | Stale | **Low** |
| Dead T2 empty FindRelevantFiles | Hygiene | Present | **Low** |
| Use of `SparseRetriever.mu` | Internal consistency | Field unused | **Low** |
| Assert `issue_context` summary | Schema | Not produced | **Low–Medium** |

## 2. Prioritized remediation

### P0 — Make retrieval executive-visible

1. **Wire search into seed or session executor**  
   After `ExtractKeywords`, run `FindRelevantFiles` or `BuildContext` under timeout; assert:
   - `keyword_hit` / `candidate_file`
   - `tiered_context_file` for T2–T4 (with real resolved paths)
   - optional `issue_context` summary  
   Call sites: `cmd/nerd/chat/process_seed.go` and/or clean-loop session path.

2. **Resolve dormant `Model.Retriever`**  
   Either (a) use it for the wire above, or (b) remove field + boot construction with wiring audit note. Prefer (a).

### P1 — Quality & hybrid

3. Fix `searchKeywordFiles` to drop empty `FindRelevantFiles(ctx, "", …)`.  
4. Implement embedding-backed T4 when `embedding.Engine` available (inject, don’t hard-import if possible).  
5. Add Go import graph expander for T3.  
6. Cap max file size / skip binary sniff.

### P2 — Scale & polish

7. Optional: real `rg` subprocess backend behind interface (justify or delete `parseRipgrepOutput`).  
8. Per-workDir inverted index / mtime cache for multi-turn sessions.  
9. Structured metrics (cache hit rate, files walked, elapsed).  
10. Align comments/tests naming with native scan.

## 3. Non-gaps (do not “fix” these)

| Observation | Why not a gap |
|-------------|----------------|
| No Mangle files inside package | By design — pure transduction library |
| No VirtualStore routes | Retrieval is not an action executor |
| No prompt atoms in package | JIT atoms live under `internal/prompt/` |
| Default scanner is non-SIMD | Correct default; SIMD optional |
| Integration tests behind build tag | Intentional cost control |

## 4. Wiring-audit caution

Before deleting any “unused” type (`TieredContextBuilder`, `parseRipgrepOutput`, `Model.Retriever`):

1. Grep chat boot, process_seed, schemas, context compressor, campaign/SWE paths.
2. Prefer completing the producer pipeline over deletion.
3. Schemas already expect richer facts than producers emit — that is a **wire gap**, not dead schema.

## 5. Dependency gaps (cross-package)

| Peer | Gap |
|------|-----|
| `internal/context` | Consumes facts that sparse search never asserts |
| `internal/embedding` | No interface handoff for T4 |
| `internal/session` | Clean loop does not call retrieval package |
| `cmd/nerd/chat` | Seed uses extract only |

## 6. Risk if gaps remain

- Agent “knows” mentioned files from text but **does not evidence** other relevant files via search.
- Operators believe sparse retriever is active (boot log line) when it is idle.
- Activation engine underfed → weaker issue-aware context selection.

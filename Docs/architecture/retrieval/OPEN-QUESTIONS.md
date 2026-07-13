# retrieval — Open Questions

> Last verified: **2026-07-13**

Real open questions (not rhetorical). Resolve with code + product decisions.

## Q1 — Who owns the sparse search call?

Should search run in:

- (a) `seedIssueFacts` (chat-only),  
- (b) `session.Executor` observe phase (clean loop),  
- (c) both with dedupe, or  
- (d) VirtualStore tool invoked by policy?

**Trade-offs:** (a) faster to wire; (b) aligns with clean loop / non-TUI entrypoints; (d) most constitutional but higher latency to first useful facts.

## Q2 — Native scan vs real ripgrep?

Comments and tests still speak of `rg`. Options:

1. Commit to native Go and rewrite docs/names.  
2. Add optional `rg` backend for monorepo speed.  
3. Hybrid: native for small trees, `rg` above threshold.

## Q3 — How much EDB is enough per turn?

Full `BuildContext` can assert dozens of files. Risk: EDB bloat and activation noise. Need policy for:

- max facts per issue  
- issue ID lifecycle (currently nanoid-ish each seed)  
- retraction of previous issue facts  

## Q4 — Verb gating for seed

Only `/fix|/debug|/review|/security` seed issues. Should `/refactor`, natural-language intents, and campaign tasks also seed? Who decides — perception, Mangle rule, or chat switch?

## Q5 — Embedding boundary

Should T4 depend on `internal/embedding` via interface injected at boot, or should embedding own “semantic file retrieval” and retrieval only do lexical stages?

## Q6 — Parallelism defaults for 50k-file trees

Is `Parallelism=4` and full walk-per-keyword acceptable, or is an on-disk index mandatory for SWE-bench claims?

## Q7 — Should `TieredContext.LoadContent` ever feed prompts directly?

Or must all content loading go through `internal/context` compression for token budgets and feedback loops?

## Q8 — Issue text truncation at 4000

Is 4000 chars enough for long stack traces? Should truncation prefer head+tail or structured sections (stack vs description)?

## Q9 — Multi-root workspaces

Single `workDir` only. Do multi-module monorepos need multiple SparseRetrievers or a path list?

## Q10 — SIMD tag productization

Is `simd` tag ever used in release builds? If not, is `scanner_amd64.go` worth keeping?

---

When a question is decided, record the decision in `IMPLEMENTED_SPEC.md` and strike it here with date + choice.

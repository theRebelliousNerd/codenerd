# prompt — TODO

> Last verified: **2026-07-13**  
> Prioritized backlog for `internal/prompt/` (docs-tracked; do not treat as committed engineering plan without user approval).

## P0 — Correctness

- [ ] Expand `CompilationContext.Hash()` to cover prompt-affecting fields currently omitted (`PreviousAttemptNoToolCall`, `AvailableTools`, activation-related inputs as applicable).  
- [ ] Unify or generate-sync **ConfigAtom** catalogs (`DefaultConfigAtomProvider` vs `RegisterDefaultConfigAtoms`) so tool/policy lists cannot drift.  
- [ ] Add regression test: tool_nudge world state must change compile result or explicitly clear cache.

## P1 — Operability

- [ ] Document all `PredicateSelector` call sites; wire or mark dormant via integration audit.  
- [ ] Clarify `CacheTTLSeconds` (enforce TTL eviction or remove field).  
- [ ] Add `internal/prompt/agents.md` if root Working Map continues to cite it (or fix root map to README).  
- [ ] Ensure all production boots attach vector searcher when embeddings available.

## P2 — Scale / polish

- [ ] Embedding quantization (TODO in `atoms.go`) for large corpora.  
- [ ] Per-request vector weight via CompilationContext (TODO in selector).  
- [ ] Streaming FactBuilder for `ToContextFacts` (TODO in context.go).  
- [ ] Offline atom dependency cycle lint in CI.  
- [ ] Benchmark Fit/Select against full embedded corpus size growth.

## P3 — Content

- [ ] Keep mangle encyclopedia atoms selector-tight so legislator budgets stay healthy.  
- [ ] Review dual concise/min coverage on high-priority language atoms.

## Done recently (do not re-open without evidence)

- Skeleton/flesh bifurcation  
- Mandatory absolute budget caps  
- compile_context defer retract  
- Structured-output protocol filter  
- prompt_atom fact arg order fix vs schema  

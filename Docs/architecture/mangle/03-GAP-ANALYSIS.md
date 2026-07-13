# 03 — Gap Analysis: mangle

> Last verified: 2026-07-13  
> Compares vision (`01-VISION.md`) and north star to **implemented** code.

## Matrix

| Target capability | Reality | Gap severity | Notes |
|-------------------|---------|--------------|-------|
| Durable Engine API | **Met** | — | Load, fact, query, limits, stats |
| Gas-limited inference | **Mostly met** | Low | Engine has limits; kernel full path has limits; **diff path does not forward** `WithCreatedFactLimit` |
| Differential as default hot path | **Partial** | Medium | Flag defaults via features; fallbacks often take full path; external predicates force full |
| Snapshot isolation | **Met (copy)** | Low | Full copy, not structural COW — memory cost under large EDB |
| Virtual/lazy predicates | **Met** | Low | FactStoreProxy; loader shape is string→string |
| Closed-loop generation | **Met** | Low | FeedbackLoop production-used |
| Structured synth as default | **Partial** | Medium | Modes exist; not mandatory for all writers |
| GCD atom validation | **Met** | Low | AtomValidator + RepairLoop |
| Schema drift + forbidden heads | **Met** | Low | Regex Decl limits edge cases |
| Parse process lock | **Mostly met** | Medium | Engine/core locked; sanitizer/synth use `parse.Unit` |
| Unified proof/provenance story | **Partial** | Medium | ProofTreeTracer ≠ DerivationRecorder |
| LSP product polish | **Partial** | Low | Server code exists; editor integration quality varies |
| True delta propagation | **Not met** | Medium–High | Explicit future work in differential comments |
| intent_routing.mg as executive | **Unclear** | Medium | File present; load path needs wiring audit |

## Priority backlog (dependency-ordered)

### P0 — Correctness under concurrency / eval options

1. Route `transpiler.Sanitizer` and `synth.Compile` parse through `mangle.ParseUnit`.
2. Forward `EvalOption`s (externals, created-fact limit, recorder) on DifferentialEngine eval calls — unlocks more kernel time on diff path safely.

### P1 — Differential completeness

3. Document and test kernel fallback matrix (externals, proof, policy dirty, retract).
4. Design real delta propagation (beyond unified re-eval) only after (2).
5. Snapshot memory strategy if large EDB snapshots become common.

### P2 — Generation consistency

6. Require synth mode for autopoiesis / legislator / mangle_repair (or VirtualStore `mangle_synth_tool`).
7. Prefer ProgramInfo-based Decl maps over regex for SchemaValidator when ProgramInfo available.

### P3 — Observability & routing corpus

8. Unify glass-box derivation (proof tree + provenance).
9. Wiring audit for `intent_routing.mg` and `IntersectSIMD` call sites.

## Non-gaps (do not “fix”)

| Item | Why it is not a gap |
|------|---------------------|
| Package does not implement `permitted` | Owned by core policy by design |
| Package does not own OODA loop | Session/kernel ownership |
| 2-bucket stratification | Intentional performance choice with measurements |
| Feedback session budgets | Safety feature, not incomplete work |
| Auto-eval default true | Configurable via env |

## Spec vs stub claims

Older thin corpus files claimed generic “90%” without behavioral narrative. **This rebuild** treats the package as production-integrated. Gaps above are incremental engineering, not missing skeleton.

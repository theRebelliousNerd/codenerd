# world — Gap Analysis

> Last verified: **2026-07-13**  
> Spec/vision vs `internal/world` reality.

## Matrix

| Area | Vision / schema intent | Reality | Gap? |
|------|------------------------|---------|------|
| Portable `file_topology` identity | Workspace-relative everywhere | Full scan uses `canonicalScanPath`; incremental often stores walk absolute paths | **Yes — High** |
| Deep holographic for all supported langs | Cartographer multi-lang | `MapFile` only deep-maps `.go`; multi-lang dataflow exists off main entry | **Yes — High** |
| `WorldPredicates` complete replace | Safe full replace of world EDB | List omits `entry_point`, `project_language`, CodeDOM preds, git, scope preds | **Yes — Medium** |
| `dependency_link` | Schema Decl + WorldPredicates | Sparse emission in scan path | **Yes — Medium** |
| Multi-language LSP | README extensibility | Only Mangle LSP manager | **Yes — Medium** |
| Dual ingestor (shard vs chat scan) | Single world truth | Two systems can both write facts | **Yes — Medium** |
| Incremental entry_point/project_language | Detected on full | Mostly full-scan path | **Yes — Low** |
| Holographic as JIT atoms | North-star JIT for LLM text | Go formatters build prompt strings | **Yes — Low** (design choice) |
| Full CFG dataflow | Sound analysis | Explicit scope-range heuristics | **No** — intentional |
| Constitutional gating of scan | Default deny actions | Scan is local read; not an “action” in policy sense | **No** |
| CodeDOM polyglot factory | Unified elements | DefaultParserFactory registers go/py/ts/rs/mg | **No** |
| Nano mtime invalidation | No same-second stale cache | Implemented FileCache + fingerprints | **No** |
| Deep fact cache in LocalStore | Reuse Cartographer results | `EnsureDeepFacts` depth=`deep` | **No** |
| Core import cycle avoidance | types + system bridge | `types` aliases + `HolographicCodeScope` | **No** |

## Priority backlog (gaps only)

### P0 — correctness

1. **Canonicalize paths in incremental scan** to match full scan (`canonicalScanPath` or shared helper).  
   Risk: duplicate `file_topology` rows for same file; session restore breaks.

2. **Document and/or extend `WorldPredicates`** so `ApplyIncrementalResult` Full mode cannot leave orphans or delete too little.

### P1 — depth parity

3. **Cartographer multi-lang `MapFile`** (or explicit secondary API wired from deep scan) for python/ts/js/rust `code_defines`/`code_calls`.

4. **Emit or retire `dependency_link`** — either implement import edges on scan or remove from replace-set / docs claims.

### P2 — architecture hygiene

5. **Clarify ownership**: chat incremental vs `WorldModelIngestorShard` (when each runs; who wins).

6. **LSP gopls adapter** behind `lsp.Manager` (already sketched in `lsp/README.md`).

7. **Optional:** move stable holographic sections toward prompt atoms if campaign prompts drift.

## Non-gaps (do not “fix” without redesign)

| Item | Why not a gap |
|------|----------------|
| Scope-range dataflow (not CFG) | Source comments state intentional simplicity for Mangle |
| Skip test files in fast AST | Reduces noise; topology still marks `/true` tests |
| Max AST bytes / 5MB dataflow skip | Performance guardrails |
| Soft-fail git when not a repo | Correct product behavior |
| Holographic package file cap 100 | Resource protection |

## Spec debt in older docs

Earlier auto stubs undercounted multi-lang CodeDOM and dataflow depth. This rebuild treats them as **implemented partials**, not zero.

## Exit criteria for “gap closed”

| Gap | Done when |
|-----|-----------|
| Path identity | Property test: full vs incremental same Path arg for same file |
| Replace-set | Every emitted predicate either listed or explicitly “ephemeral/scope-only” |
| Multi-lang deep | `EnsureDeepFacts` accepts non-Go or dedicated path with tests |
| dependency_link | Grep shows real emitters + tests **or** removed from WorldPredicates |

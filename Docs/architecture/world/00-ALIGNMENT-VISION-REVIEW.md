# world — Alignment & Vision Review

> Last verified: **2026-07-13**  
> Against: codeNERD north star (LLM creative / Mangle executive / transduction interface)

## Scoring legend

| Score | Meaning |
|------:|---------|
| 5 | Strong alignment with evidence in code |
| 4 | Aligned with minor gaps |
| 3 | Partial / dual paths |
| 2 | Weak or aspirational |
| 1 | Misaligned or absent |

## Dimensions

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **Transduction (NL/code → atoms)** | **5** | Scanner, Cartographer, CodeDOM, LSP all emit `core.Fact` / `types.Fact` for EDB |
| **Logic as executive** | **4** | World never decides `permitted`; schemas derive `file_exists` etc. Gap: dual scan paths can race facts |
| **LLM as creative center** | **5** | `HolographicProvider` + impact callers feed agents; world does not “reason” for them |
| **Constitutional safety default-deny** | **3** | World is mostly read-only scan; no local `permitted`. Safety is boundary (caps, ignores), not constitutional |
| **JIT prompt atoms** | **2** | Holographic formatting is Go string assembly, not prompt atoms under `internal/prompt/atoms/` |
| **Wiring before “unused”** | **4** | Deep scan wired via `HolographicCodeScope`; CodeDOM/TestDependency partially consumer-driven |
| **Long-horizon fidelity** | **4** | Incremental + LocalStore + nano mtime; absolute/relative path hazard remains |
| **Polyglot seriousness** | **3** | Tree-sitter + multi-lang parsers present; Cartographer deep map Go-only |
| **Mangle Decl discipline** | **4** | Core `schemas_world.mg`; package lists `WorldPredicates` (incomplete vs emitters) |
| **Observability** | **4** | `CategoryWorld` timers and structured walk logs |

**Weighted read:** World is a **strong transducer** and correctly stays out of executive control. Soft spots are dual pipelines, path identity, and LLM-facing holographic text not being JIT atoms.

## What “good” looks like for this package

1. Every workspace change that matters becomes **retractable, Decl’d EDB**.
2. Fast path answers “what files exist / rough symbols”; deep path answers “what calls what / guards”.
3. Agent context is **query-selected**, not whole-repo paste.
4. Scanners never become policy oracles.

## Alignment risks

| Risk | Severity | Mitigation today |
|------|----------|------------------|
| Stale EDB after silent edit | High | Incremental scan on chat sync; FileCache nano mtime |
| Absolute path facts break restore | High | `canonicalScanPath` on full scan; incremental partial |
| Deep facts missing for non-Go | Medium | Fast `symbol_graph` still available |
| Predicate replace misses some atoms | Medium | Manual retract by consumers |
| Prompt bloat from holographic bodies | Medium | max 10 callers / 50 lines |

## Verdict

**Aligned as the workspace perception / world-model substrate.** Not a north-star violation. Highest-value alignment work: unify path identity, complete replace-set, deepen multi-lang deep map, optionally JIT-ify holographic prompt fragments.

# tools — Alignment & Vision Review

> Last verified: **2026-07-13**  
> Source of truth: `internal/tools/**`

## Scoring rubric

Scores are evidence-based against codeNERD north star (LLM creative; Mangle executive; constitutional safety; JIT-first).

| Dimension | Score | Evidence |
|-----------|------:|----------|
| Inversion of control (tools ≠ executive) | **9/10** | Tools only return strings; session asserts `pending_action` / queries `permitted`; VS executive gate on interactive path |
| Modular JIT tool surface | **9/10** | Standalone `Tool` defs + RegisterAll; replaces deleted domain-shard embeds (`types.go` package comment) |
| Constitutional safety coupling | **7/10** | Safety lives in session/core, not tools; workspace guard on file ops only; empty AllowedTools allows all |
| Intent-driven selection | **7/10** | Mangle `modular_tool_allowed` + ConfigFactory; package `FilterByIntent` is soft fallback |
| Workspace / blast-radius control | **6/10** | Strong for core file ops; weak for glob/grep/codedom/shell cwd |
| Observability | **6/10** | ToolsDebug + VirtualStore/Researcher/Browser channels; no metrics |
| Test density | **8/10** | Solid unit + boundary + shell mock + some integration; e2e in tests/e2e |
| Category completeness | **5/10** | `/review` and `/attack` categories empty; doc.go lists unregistered codedom tools |
| Dual-registry coherence | **7/10** | Hydrate to VS + Global; Has() skip duplicates; process singleton risk in tests |
| North-star narrative fit | **8/10** | Clear effect layer under logic executive |

**Weighted overall: ~7.5/10** — production-grade effect library with known containment and catalog-sync gaps.

## What aligns well

1. **Separation of concerns:** registry is dumb-ish plumbing; policy is external.  
2. **Schema-first args:** LLM tool calling supported with required + type checks.  
3. **Research extraction:** tools pulled out of deleted ResearcherShard into reusable package.  
4. **DI for test impact:** interfaces avoid import cycles with world/core.  
5. **Idempotent RegisterAll:** `registry.Has` skip supports re-hydrate.

## What misaligns

1. **Open-default allowlist** when `AllowedTools` empty (`session.isToolAllowed`).  
2. **Inconsistent path containment** across tool families.  
3. **Mangle tool catalog lag** (git tools, some cache tools not in intent_routing).  
4. **Process globals** (registry, browser, cache, test provider) complicate multi-session purity.  
5. **Package FilterByIntent** can return entire toolbox on unknown intent — opposite of default deny.

## Verdict

Keep architecture: modular tools under kernel policy. Prioritize **containment parity** and **single allowlist source of truth** (ConfigFactory fed by Mangle, never silent open).

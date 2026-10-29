# prompt — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Source: `internal/prompt/` vs codeNERD north star (Agents.md / Claude.md)

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully realized in package + wired consumers |
| 4 | Realized in package; minor consumer gaps |
| 3 | Partial — important paths incomplete or dual |
| 2 | Stub / aspirational with thin code |
| 1 | Contradicts north star or absent |

## Dimension scores

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **LLM = creative center** | **5** | Package only assembles *prompts and tool surfaces*; it does not decide `next_action`. Session/shards call LLM with compiled prompt. |
| **Logic = executive** | **5** | Skeleton selection requires `KernelQuerier` + Mangle predicates (`selected_result`, `mandatory_selection`, `blocked_by_context` in `jit_compiler.mg`). Context asserted as `compile_context/2`. |
| **JIT prompt atoms for LLM behavior** | **5** | Canonical library under `internal/prompt/atoms/`; `go:embed`; project/agent DBs; evolved atoms path. README + Agents.md mandate atoms-first. |
| **Constitutional safety default deny** | **4** | Safety atoms are skeleton/mandatory budget; ConfigFactory sets `RequirePolicyEnforcement: true` and ships `base.mg` (+ persona policies). Actual `permitted(...)` enforcement lives in kernel/VirtualStore — prompt supplies policy *list*, not the deny engine. |
| **Transduction NL → formal atoms** | **4** | `CompilationContext.ToContextFacts()` → Mangle; `PromptAtom.ToFact` / selector facts; kernel-injected `injectable_context` / `specialist_knowledge` → ephemeral atoms. Intent parsing itself is perception’s job. |
| **No ad-hoc shard prompt growth** | **4** | System shards (router, planner, legislator, mangle_repair, world_model, perception) document JIT atoms. Baseline path still exists for non-JIT (`baseline.go`). Some dual wiring via articulation assembler. |
| **Wiring auditability** | **4** | Session interfaces (`JITCompiler`, `ConfigFactory`); functional options; DB registrars for shards. Cache/singleflight well instrumented. PredicateSelector is a parallel surface with less obvious session wiring density. |
| **Observability of selection** | **5** | `CompilationStats`, `PromptManifest`, `CategoryJIT` logs, `lastResult` atomic, DebugMode flight recorder. |
| **Budget honesty under limits** | **5** | Polymorphic render modes; mandatory absolute budget cap; headroom; category allocations in `budget.go`. |
| **Long-horizon / campaign fitness** | **4** | Context tiers: campaign phase, northstar, ouroboros, init, build_layer. Campaign atoms under `atoms/campaign/`. Semantic query expansion optional. |

**Weighted overall: ~4.5 / 5** — package is a production JIT compiler aligned with the north star, not a stub.

## North-star quotes (binding for this package)

1. *“New prompt behavior becomes prompt atoms first, not ad-hoc shard prompt text.”*  
2. *“Internal prompt atoms live under `example:internal/prompt/atoms/<category>/`.”*
3. *“Logic determines reality; the model merely describes it.”* — prompts never replace `permitted(...)`.

## Alignment risks (not fails)

| Risk | Severity | Note |
|------|----------|------|
| Skeleton hard-fails without kernel | Medium | Correct for production; unit tests must inject mocks |
| Dual ConfigAtom registries | Low–Med | `DefaultConfigAtomProvider` vs `RegisterDefaultConfigAtoms` / `SimpleRegistry` tool name lists can drift |
| External kernel adapter contract | Medium | Production uses cloned compilation scopes; third-party adapters that expose neither `KernelScopeProvider` nor `KernelRetracter` cannot guarantee fact isolation |
| Flesh degrades silently | Low | By design; skeleton must remain complete |

## Verdict

**Ship-grade alignment.** The package is the reference implementation of the JIT half of inversion-of-control. Cache identity, production fact isolation, and atom-contract parity are verified; continue with ConfigAtom consistency, external-adapter fail-closed policy, decision receipts, and wiring audits before deletion of “unused” helpers.

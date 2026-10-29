# world — Vision

> Last verified: **2026-07-13**  
> Status: Target architecture for `internal/world` (as implied by code structure + schemas, not aspirational product fiction)

## Product role

codeNERD must *know the workspace* without asking the LLM to re-walk the tree every turn.

**Vision:** The world package is the **always-on sensory cortex** of the agent:

- Topology is cheap, continuous, and portable.
- Structure is layered (fast symbols → deep graph → CodeDOM edit surface).
- Intelligence (impact, tests, diagnostics) is **projected into Mangle**, so policy and shards query facts instead of re-parsing ad hoc.

## Target architecture (layers)

```
┌─────────────────────────────────────────────────────────┐
│ L0  Topology     file_topology, directory, entry_point  │
│ L1  Fast AST     symbol_graph (tree-sitter / mangle)    │
│ L2  Deep graph   code_defines, code_calls, dataflow     │
│ L3  CodeDOM      code_element + lang Stratum-0 + scope  │
│ L4  Intelligence LSP symbols/diags, git_history, tests  │
│ L5  Agent view   HolographicContext (selected, capped)  │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
              Kernel EDB + LocalStore cache
                         │
                         ▼
         policy / spreading activation / shards / JIT
```

## Design goals

1. **Portable identity** — paths and hashes that survive machine move and repo relocate.
2. **Incremental by default** — full scan is bootstrap; steady state is deltas.
3. **Fail soft** — parse errors skip symbols, not topology; dataflow enhances, doesn’t block defines.
4. **Polyglot CodeDOM** — one `CodeElement` model, many parsers, bridge rules normalize semantics.
5. **Import-cycle hygiene** — `types` aliases; `HolographicCodeScope` in `system` bridges core↔world.
6. **Agent-safe context** — holographic and impact views are bounded and priority-sorted.

## Non-goals

- Replacing language servers for full IDE fidelity in all languages (LSP path is Mangle-first).
- Full interprocedural static analysis / sound dataflow (scope-range heuristics are intentional).
- Owning constitutional policy or tool execution.
- Embedding product-specific “foreign-product-surface” or app client features into world.

## Success criteria

| Criterion | Observable |
|-----------|------------|
| Boot freshness | Chat boot + sync leave kernel with current topology |
| Edit latency | Single-file edit → delta retract/assert, not full rescan |
| Review quality | Impact-prioritized callers present for Go review targets |
| Scope edits | FileScope exposes interactable `code_element` refs |
| Policy usable | `file_exists`, `code_defines` rules fire without Go-side hardcoding |

## Evolution direction (from code comments + partials)

1. Multi-lang deep Cartographer (Python/TS/RS `code_defines` parity).
2. gopls (and other LSPs) behind `lsp.Manager`.
3. Complete `WorldPredicates` / replace discipline for all emitters.
4. Single path-canonicalization helper used by full **and** incremental scans.
5. Optional JIT atoms for holographic sections if LLM surface stabilizes.

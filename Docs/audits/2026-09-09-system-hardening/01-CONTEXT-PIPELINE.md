# What the model sees, and why

Date: 2026-09-09
Companion to [00-FINDINGS.md](00-FINDINGS.md).

This is the reference for the per-file context pipeline after the hardening
pass: what reaches the model on a file-targeted turn, where each piece comes
from, what bounds it, and what was deliberately left out.

The governing rule is the project owner's: **the right context at the right
time, and nothing more.** "Nothing more" is the half that is easy to lose. Every
section below has to justify its tokens against the alternative of the model
simply reading the file.

---

## The path

```
Executor.ProcessWithIntent                       internal/session/executor.go
  └─ JIT compile                                 internal/prompt/compiler.go
  └─ withProjectInstructions(prompt)             executor_tools.go   (nerd.md)
  └─ withFileContext(prompt, intent.Target)      executor_tools.go
       └─ HolographicProvider.PromptSection      internal/world/holographic.go
            ├─ getContextInternal
            │    ├─ buildGoContextWithContext    ← package-parse cache
            │    ├─ analyzeArchitecture
            │    ├─ queryRelationshipsWithContext ← kernel: code_defines/code_calls
            │    ├─ applyImpactPriorities        ← kernel: context_priority_file
            │    ├─ applyImportDimensions
            │    └─ applyDirectImporters         ← kernel: dependency_link
            └─ render, bounded
```

`withFileContext` is a no-op when the target is empty, the provider is nil, or
the rendered section is empty. The provider is installed in
`internal/system/factory.go` only when the kernel is a `CortexKernel` with a
live `RealKernel`; without one, no holographic context appears at all.

## What is rendered, and what earns it

| Section | Cap | Why it earns its tokens |
|---|---|---|
| Package · Layer · Module · Role | one line | Orients the model in the system before it reads anything. Facets are joined, not chained — a missing package clause used to emit a dangling ` · `. |
| Tests: yes/no (+ coverage) | one line | Changes what a careful change looks like. |
| Exported signatures | 8 + pool count | See **Ranking** below. |
| Type definitions | 8 + pool count | Field/method counts only, never bodies. |
| **Imported by** | 6 + count | The only dimension here a model cannot get by reading the file. First thing worth knowing before changing an exported symbol. |
| Callers (impact-prioritized) | 8 + count | Only present after an edit; ranked by the kernel's impact walk. |

**Deliberately not rendered**, though populated on the struct for programmatic
consumers: `DirectImports`, `ExternalDeps` (the model can read the file's own
import block) and `TODOCount`.

**Deliberately removed**: `RelatedEntities` had no producer, no consumer and no
stated semantics. `ComplexityHints` had no possible source —
`cyclomatic_complexity` is declared, queried by `edge_case_detector`, and
asserted by nothing.

**A section with no substance is not emitted.** Architecture facets are inferred
from path patterns, so a file that does not exist still produces a Role. Without
a substance check, a missing target rendered a header, an inferred Role and
"Tests: no" — tokens spent to say nothing, on the turn where the model is
already looking at a path that is not there.

## Ranking: three tiers, then centrality

Eight slots is a hard budget, so what fills them is the whole question.

1. **Defined in the target file** — what this file offers.
2. **Referenced by the target file** — what it depends on.
3. **Everything else**, ordered by how many other files in the package
   reference it.

Tier 3 is what fixes the case tiers 1 and 2 cannot help with: a file that
defines nothing and references nothing. On `internal/core/kernel.go` — a
16-line package marker whose implementation lives in `kernel_*.go` — the old
directory-order fallback produced eight symbols from `action_validator.go` and
"… and 592 more". Roughly 400 tokens, zero signal, because that filename sorts
first.

Same file now:

```
- `func NewRealKernel() (*RealKernel, error)`     — kernel_init.go
- `type RealKernel`  — struct (40 fields)         — kernel_types.go
- `type Fact` — alias                             — kernel_types.go
- `type VirtualStore` — struct (31 fields)        — virtual_store.go
  …
- … and 593 more exported in package `core`
```

Reference edges come from the parse the cache already performs: every identifier
is collected per file during the AST walk, then narrowed once the whole package
is known to just the package-level names each file uses but does not define.
Narrowing is what keeps it bounded by the package's symbol count rather than by
every identifier in every file.

**Every ordering step is deterministic.** A section that reshuffles between
turns for no reason throws away the provider's prompt-cache hit on every turn,
which costs more than the ranking saves.

## Cost

`PromptSection` runs once per LLM turn with a file target. Measured on
`internal/core` (150+ files):

| | Before | After |
|---|---|---|
| Time | 49.4 ms | **1.3 ms** |
| Allocated | 12.6 MB | **426 KB** |
| Allocations | 296,445 | **1,163** |
| Output | ~1.6 KB | ~1.4 KB, ranked |

The cache holds only the filesystem-derived half of the context, keyed on a
`(name, size, mtime)` fingerprint over the directory's non-test `.go` files.
Anything the kernel answers — call graph, impact priorities, importers — is
recomputed every call: kernel state changes without any file changing, and a
stale answer there is a correctness bug, not a slow path.

`localRefs` and `refCount` are shared rather than deep-copied on a hit. Both are
written once at the end of `parsePackage` and only read afterwards; copying
`internal/core`'s ~600-entry table on every hit tripled the cost of the hit.
`TestPackageParse_FrozenMapsAreNotMutated` pins the invariant.

## The impact chain

```
edit_element / edit_lines / insert_lines / delete_lines
  └─ modified_function(<pkg>.<Name>, File)        internal/core/codedom_modified_symbols.go
       └─ impact_caller(Target, Caller)           impact.mg:29
            └─ impact_graph(Target, Caller, ≤3)   impact.mg:41-53
                 └─ context_priority_file(...)    impact.mg:68-78
                      └─ PrioritizedCallers       holographic_impact.go
                           └─ "Callers (impact-prioritized)"
```

Two things about this chain are worth stating plainly, because both were
invisible until an end-to-end test existed:

**The identifier shape must match the call graph exactly.** `Cartographer` keys
`code_calls` and `code_defines` by `<pkg>.<Name>` (`<pkg>.<Receiver>.<Name>` for
a method). Emitting a bare `Target` where the graph holds `impactdemo.Target`
produces correct-looking facts on both sides and a join that never fires. No
unit test can catch this — each side is internally consistent.

**The line tools resolve overlap *before* the edit.** Afterwards the line
numbers have moved, and for `delete_lines` the element is gone entirely — which
is precisely the case whose callers matter most.

`TestImpactChain_EndToEndThroughVirtualStore` (`internal/system`) drives a real
`edit_element` through a real kernel and asserts every link.

## Priority scale

`impact.mg` emits `Priority = 4 - Depth`, so 3 / 2 / 1. Every Go renderer
buckets on 80 / 50 / 25. Unconverted, a direct caller rendered as `MINIMAL` —
below the default 50 given to facts carrying no priority at all.
`impactPriorityToScale` maps 3→100, 2→70, 1→40 and recovers the depth the
priority encodes, instead of the hardcoded 1.

## Where the bounds live

| Bound | Value | Location |
|---|---|---|
| Signatures / types / callers rendered | 8 each | `holographic.go` |
| Importers rendered | 6 | `holographic_dependencies.go` |
| Prioritized callers held | 10 | `holographic_impact.go` |
| Caller body lines (non-prompt path) | 50 | `holographic_impact.go` |
| Sibling files parsed | 100 | `holographic.go` |
| Sibling file size | 5 MB | `holographic.go` |
| Call-graph edges | 100 | `holographic.go` |
| Cached package directories | 64 | `holographic_cache.go` |
| Holographic sections per campaign report | 5 | `intelligence_gathering_methods.go` |
| Free-text fact argument | 4 KiB | `internal/types/fact_text.go` |
| Tool result fed back to the model | 16 KiB | `internal/session/executor_tools.go` |

Every truncation is visible to the model. A silently truncated result is one the
model will reason about as though it were complete.

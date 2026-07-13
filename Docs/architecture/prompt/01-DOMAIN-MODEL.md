# prompt — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/prompt/` (25 non-test .go, 32 tests, 0 .mg)**


## Source package

`internal/prompt/`

## Exported / primary types (sampled)

| Type | Location |
|------|----------|
| `FinalAssembler` | `internal/prompt/assembler.go:21` |
| `TemplateEngine` | `internal/prompt/assembler.go:298` |
| `TemplateFunc` | `internal/prompt/assembler.go:305` |
| `AssemblyOptions` | `internal/prompt/assembler.go:457` |
| `PromptStats` | `internal/prompt/assembler.go:592` |
| `AtomCategory` | `internal/prompt/atoms.go:27` |
| `PromptAtom` | `internal/prompt/atoms.go:135` |
| `EmbeddedCorpus` | `internal/prompt/atoms.go:603` |
| `AtomStore` | `internal/prompt/atoms.go:663` |
| `BudgetPriority` | `internal/prompt/budget.go:18` |
| `CategoryBudget` | `internal/prompt/budget.go:56` |
| `TokenBudgetManager` | `internal/prompt/budget.go:82` |
| `AllocationStrategy` | `internal/prompt/budget.go:95` |
| `BudgetReport` | `internal/prompt/budget.go:812` |
| `CategoryUsage` | `internal/prompt/budget.go:823` |
| `Fact` | `internal/prompt/compiler.go:24` |
| `KernelQuerier` | `internal/prompt/compiler.go:31` |
| `KernelRetracter` | `internal/prompt/compiler.go:41` |
| `VectorSearcher` | `internal/prompt/compiler.go:46` |
| `SearchResult` | `internal/prompt/compiler.go:55` |
| `CompilationStats` | `internal/prompt/compiler.go:62` |
| `CompilationResult` | `internal/prompt/compiler.go:207` |
| `JITPromptCompiler` | `internal/prompt/compiler.go:249` |
| `CompilerConfig` | `internal/prompt/compiler.go:306` |
| `CompilerStats` | `internal/prompt/compiler.go:1117` |

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 0 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| — | 0 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **JIT prompt compiler, atoms, selector, budget**

## Data & control concepts

- Primary language surface: Go under `internal/prompt/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package

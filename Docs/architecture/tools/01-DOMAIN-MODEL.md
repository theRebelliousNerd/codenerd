# tools — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/tools/` (25 non-test .go, 21 tests, 0 .mg)**


## Source package

`internal/tools/`

## Exported / primary types (sampled)

| Type | Location |
|------|----------|
| `CodeElement` | `internal/tools/codedom/elements.go:17` |
| `TestDependencyAnalyzer` | `internal/tools/codedom/run_impacted_tests.go:19` |
| `KernelQuerier` | `internal/tools/codedom/run_impacted_tests.go:35` |
| `FactData` | `internal/tools/codedom/run_impacted_tests.go:41` |
| `ImpactedTestInfo` | `internal/tools/codedom/run_impacted_tests.go:47` |
| `TestImpactProvider` | `internal/tools/codedom/run_impacted_tests.go:56` |
| `GrepMatch` | `internal/tools/core/search.go:184` |
| `Registry` | `internal/tools/registry.go:16` |
| `CacheEntry` | `internal/tools/research/cache.go:17` |
| `ResearchCache` | `internal/tools/research/cache.go:27` |
| `GroundingHelper` | `internal/tools/research/grounding.go:23` |
| `GroundingStats` | `internal/tools/research/grounding.go:174` |
| `GroundedResearchResult` | `internal/tools/research/grounding.go:244` |
| `ThinkingHelper` | `internal/tools/research/thinking.go:21` |
| `ThinkingMetadata` | `internal/tools/research/thinking.go:148` |
| `ThinkingStats` | `internal/tools/research/thinking.go:166` |
| `MultiTurnContext` | `internal/tools/research/thinking.go:180` |
| `SearchResult` | `internal/tools/research/web_search.go:20` |
| `ToolCategory` | `internal/tools/types.go:18` |
| `Property` | `internal/tools/types.go:41` |
| `PropertyItems` | `internal/tools/types.go:51` |
| `ToolSchema` | `internal/tools/types.go:57` |
| `ExecuteFunc` | `internal/tools/types.go:67` |
| `Tool` | `internal/tools/types.go:72` |
| `ToolResult` | `internal/tools/types.go:117` |

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

This package's position: **Tool registry and research/tool integrations**

## Data & control concepts

- Primary language surface: Go under `internal/tools/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package

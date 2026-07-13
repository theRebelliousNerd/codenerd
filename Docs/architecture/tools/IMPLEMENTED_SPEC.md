# tools — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/tools/` (25 non-test .go, 21 tests, 0 .mg)**


## 1. Purpose

Tool registry and research/tool integrations

## 2. Source paths

| Path | Role |
|------|------|
| `internal/tools/` | Primary implementation |
| `Docs/architecture/tools/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global | **n/a** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 85% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

| Path | Lines |
|------|------:|
| `internal/tools/shell/execute.go` | 772 | source |
| `internal/tools/core/file_ops.go` | 465 | source |
| `internal/tools/codedom/run_impacted_tests.go` | 433 | source |
| `internal/tools/core/search.go` | 362 | source |
| `internal/tools/research/browser.go` | 355 | source |
| `internal/tools/registry.go` | 341 | source |
| `internal/tools/research/grounding.go` | 334 | source |
| `internal/tools/research/cache.go` | 303 | source |
| `internal/tools/research/context7.go` | 297 | source |
| `internal/tools/codedom/lines.go` | 279 | source |
| `internal/tools/research/web_fetch.go` | 263 | source |
| `internal/tools/research/web_search.go` | 246 | source |

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)

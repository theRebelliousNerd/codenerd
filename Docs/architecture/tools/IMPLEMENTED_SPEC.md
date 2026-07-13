# tools — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tools/` (complete internal coverage)
> **Implementation: `internal/tools/` — 25 non-test .go, 21 tests, 0 .mg**


## 1. Purpose

Tool registry and research/tool integrations

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/tools/` | Primary implementation |
| `Docs/architecture/tools/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 85%** as living package (25 src / 21 tests)

## 4. Public surface inventory

### Largest files

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
| `internal/tools/codedom/elements.go` | 239 | source |
| `internal/tools/research/thinking.go` | 211 | source |
| `internal/tools/types.go` | 134 | source |
| `internal/tools/core/workspace_guard.go` | 102 | source |
| `internal/tools/research/register.go` | 44 | source |
| `internal/tools/codedom/register.go` | 34 | source |
| `internal/tools/core/register.go` | 33 | source |
| `internal/tools/shell/register.go` | 29 | source |

### Types (sampled)

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

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `GetElementsTool` | `internal/tools/codedom/elements.go:74` |
| `GetElementTool` | `internal/tools/codedom/elements.go:190` |
| `EditLinesTool` | `internal/tools/codedom/lines.go:14` |
| `InsertLinesTool` | `internal/tools/codedom/lines.go:114` |
| `DeleteLinesTool` | `internal/tools/codedom/lines.go:193` |
| `RegisterAll` | `internal/tools/codedom/register.go:8` |
| `RegisterTestImpactProvider` | `internal/tools/codedom/run_impacted_tests.go:67` |
| `RunImpactedTestsTool` | `internal/tools/codedom/run_impacted_tests.go:72` |
| `GetImpactedTestsTool` | `internal/tools/codedom/run_impacted_tests.go:113` |
| `ReadFileTool` | `internal/tools/core/file_ops.go:15` |
| `WriteFileTool` | `internal/tools/core/file_ops.go:123` |
| `EditFileTool` | `internal/tools/core/file_ops.go:200` |
| `DeleteFileTool` | `internal/tools/core/file_ops.go:291` |
| `ListFilesTool` | `internal/tools/core/file_ops.go:346` |
| `RegisterAll` | `internal/tools/core/register.go:8` |
| `GlobTool` | `internal/tools/core/search.go:17` |
| `GrepTool` | `internal/tools/core/search.go:141` |
| `SearchCodeTool` | `internal/tools/core/search.go:357` |
| `NewRegistry` | `internal/tools/registry.go:25` |
| `Register` | `internal/tools/registry.go:34` |
| `MustRegister` | `internal/tools/registry.go:63` |
| `Get` | `internal/tools/registry.go:73` |
| `Has` | `internal/tools/registry.go:80` |
| `GetByCategory` | `internal/tools/registry.go:88` |
| `GetMultiple` | `internal/tools/registry.go:108` |
| `All` | `internal/tools/registry.go:122` |
| `Names` | `internal/tools/registry.go:134` |
| `Count` | `internal/tools/registry.go:147` |
| `Execute` | `internal/tools/registry.go:155` |
| `ExecuteTool` | `internal/tools/registry.go:171` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)

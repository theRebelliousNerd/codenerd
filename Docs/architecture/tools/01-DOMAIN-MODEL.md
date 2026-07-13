# tools — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tools/` (complete internal coverage)
> **Implementation: `internal/tools/` — 25 non-test .go, 21 tests, 0 .mg**


## Package

`internal/tools/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Tool registry and research/tool integrations**

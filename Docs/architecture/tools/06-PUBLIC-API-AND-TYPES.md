# tools — Public API and Types

> Last verified: **2026-07-13**  
> Exported symbols that matter for integrators.

## Root package `codenerd/internal/tools`

### Types

| Symbol | File | Role |
|--------|------|------|
| `ToolCategory` | `types.go` | String enum for categories |
| `Property` | `types.go` | Schema property |
| `PropertyItems` | `types.go` | Array items type |
| `ToolSchema` | `types.go` | Required + Properties |
| `ExecuteFunc` | `types.go` | Handler signature |
| `Tool` | `types.go` | Full tool definition |
| `ToolResult` | `types.go` | Execution wrapper |
| `Registry` | `registry.go` | Thread-safe registry |

### Constants

`CategoryResearch`, `CategoryCode`, `CategoryTest`, `CategoryReview`, `CategoryAttack`, `CategoryGeneral`

### Errors

`ErrToolNotFound`, `ErrToolNameEmpty`, `ErrToolExecuteNil`, `ErrToolAlreadyRegistered`, `ErrMissingRequiredArg`, `ErrInvalidArgType`, `ErrToolNil`

### Methods — Tool

- `(*Tool) Validate() error`  
- `(*Tool) WithPriority(int) *Tool`  
- `(*ToolResult) IsSuccess() bool`

### Methods — Registry

- `NewRegistry() *Registry`  
- `Register(*Tool) error`  
- `MustRegister(*Tool)`  
- `Get(name) *Tool`  
- `Has(name) bool`  
- `GetByCategory(ToolCategory) []*Tool`  
- `GetMultiple([]string) []*Tool`  
- `All() []*Tool`  
- `Names() []string`  
- `Count() int`  
- `Execute(ctx, name, args) (*ToolResult, error)`  
- `ExecuteTool(ctx, tool, args) (*ToolResult, error)`  
- `FilterByIntent(intent string) []*Tool`

### Package-level global API

- `Global() *Registry`  
- `Register(*Tool) error`  
- `MustRegisterGlobal(*Tool)`  
- `Get(name) *Tool`  
- `Execute(ctx, name, args) (*ToolResult, error)`

---

## Subpackage `core`

| Symbol | Role |
|--------|------|
| `RegisterAll(*tools.Registry) error` | Register 8 filesystem/search tools |
| `ReadFileTool`, `WriteFileTool`, `EditFileTool`, `DeleteFileTool`, `ListFilesTool` | Constructors |
| `GlobTool`, `GrepTool`, `SearchCodeTool` | Constructors |
| `GrepMatch` | Match struct |
| `ErrPathOutsideWorkspace` | Containment error |

Unexported but critical: `workspaceRoot`, `resolveWorkspacePath`.

---

## Subpackage `shell`

| Symbol | Role |
|--------|------|
| `RegisterAll` | 7 tools |
| `RunCommandTool`, `BashTool`, `RunBuildTool`, `RunTestsTool` | |
| `GitDiffTool`, `GitLogTool`, `GitOperationTool` | |

---

## Subpackage `codedom`

| Symbol | Role |
|--------|------|
| `RegisterAll` | 7 tools |
| `CodeElement` | JSON element record |
| `GetElementsTool`, `GetElementTool` | |
| `EditLinesTool`, `InsertLinesTool`, `DeleteLinesTool` | |
| `RunImpactedTestsTool`, `GetImpactedTestsTool` | |
| `TestDependencyAnalyzer` | Interface |
| `KernelQuerier` | Interface |
| `FactData` | Kernel fact DTO |
| `ImpactedTestInfo` | Impact row |
| `TestImpactProvider` | Boot DI interface |
| `RegisterTestImpactProvider` | Set global provider |

---

## Subpackage `research`

### Tools / registration

`RegisterAll`, `Context7Tool`, `WebSearchTool`, `WebFetchTool`, browser tool constructors, cache tool constructors.

### Types

| Symbol | Role |
|--------|------|
| `SearchResult` | Web hit |
| `SearchResultsToJSON` | Helper |
| `CacheEntry`, `ResearchCache` | Cache |
| `NewResearchCache` | Constructor |
| `GroundingHelper`, `NewGroundingHelper` | Gemini grounding |
| `GroundingStats`, `GroundedResearchResult` | (grounding.go) |
| `ThinkingHelper`, `NewThinkingHelper` | Thinking mode |
| `ThinkingMetadata`, `ThinkingStats` | Thinking DTOs |

Integrators outside tools (init, campaign) typically import helpers, not RegisterAll.

---

## Integration API owned elsewhere (not re-exported)

| API | Package |
|-----|---------|
| `(*VirtualStore).HydrateModularTools` | core |
| `(*VirtualStore).GetModularTools` | core |
| `(*VirtualStore).RegisterModularTool` | core |
| `(*Executor).executeToolCall` | session (unexported) |
| `EffectiveAgentRuntimeConfig.AllowedTools` | jit/config |

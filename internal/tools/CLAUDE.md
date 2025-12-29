# internal/tools - Modular Tool Registry

This package provides modular tool definitions for the JIT Clean Loop architecture. Tools are standalone executables that any agent can use based on intent-driven JIT selection.

**Related Packages:**
- [internal/session](../session/CLAUDE.md) - Session executor consuming tools
- [internal/prompt](../prompt/CLAUDE.md) - ConfigFactory selecting tools
- [internal/core](../core/CLAUDE.md) - VirtualStore routing tool calls

## Architecture

Tools replace embedded implementations deleted with domain shards. Each tool is self-contained and selected via:

```
Intent → ConfigFactory → AllowedTools[] → Registry.Get() → Tool.Execute()
```

## File Index

| File | Description |
|------|-------------|
| `types.go` | Core type definitions including Tool, ToolCategory, ToolSchema, and ExecuteFunc. Exports Tool struct with Name, Description, Category, Execute, Schema, Priority fields. |
| `registry.go` | Thread-safe tool registry with category-based lookup. Exports Registry with Register(), Get(), GetByCategory(), AllNames(), and Execute() methods. |
| `errors.go` | Error definitions for tool operations. Exports ErrToolNameEmpty, ErrToolExecuteNil, ErrToolAlreadyRegistered, ErrToolNotFound. |

### Subdirectories

| Subdirectory | Description |
|--------------|-------------|
| `core/` | Filesystem tools: read_file, write_file, glob, grep, list_files |
| `shell/` | Execution tools: run_command, bash, run_build, run_tests |
| `codedom/` | Semantic code tools: get_elements, get_element, edit_lines |
| `research/` | Research tools: context7_fetch, web_search, web_fetch, browser, grounding (Gemini) |

## Key Types

### Tool
```go
type Tool struct {
    Name            string        // Unique identifier
    Description     string        // For LLM tool calling
    Category        ToolCategory  // /research, /code, /test, /review, /attack, /general
    Execute         ExecuteFunc   // func(ctx, args) (string, error)
    Schema          ToolSchema    // JSON schema for args
    Priority        int           // Higher = preferred (default 50)
    RequiresContext bool          // Needs session context
}
```

### ToolCategory
```go
const (
    CategoryResearch ToolCategory = "/research"  // Context7, web search
    CategoryCode     ToolCategory = "/code"      // File ops, build
    CategoryTest     ToolCategory = "/test"      // Test execution
    CategoryReview   ToolCategory = "/review"    // Static analysis
    CategoryAttack   ToolCategory = "/attack"    // Adversarial
    CategoryGeneral  ToolCategory = "/general"   // Universal
)
```

### Registry
```go
type Registry struct {
    tools      map[string]*Tool
    byCategory map[ToolCategory][]*Tool
}

func (r *Registry) Register(tool *Tool) error
func (r *Registry) Get(name string) *Tool
func (r *Registry) GetByCategory(cat ToolCategory) []*Tool
func (r *Registry) Execute(ctx, name string, args map[string]any) (*ToolResult, error)
```

## Tool Categories and Mangle Routing

Tools are routed via intent in `internal/mangle/intent_routing.mg`:

```mangle
# Core tools available to all intents
modular_tool_allowed(/read_file, Intent) :- user_intent(_, _, Intent, _, _).

# Write tools - available for code intents
modular_tool_allowed(/write_file, Intent) :- intent_category(Intent, /code).

# Research tools - available for /research intent
modular_tool_allowed(/web_search, Intent) :- intent_category(Intent, /research).
```

## Usage

### Registering Tools
```go
import "codenerd/internal/tools"

registry := tools.NewRegistry()

// Register from subdirectories
core.RegisterAll(registry)
shell.RegisterAll(registry)
codedom.RegisterAll(registry)
research.RegisterAll(registry)
```

### Executing Tools
```go
result, err := registry.Execute(ctx, "read_file", map[string]any{
    "path": "/path/to/file.go",
})
```

## Gemini Grounding Helper

The `research/grounding.go` file provides a reusable helper for Gemini's grounding features (Google Search, URL Context). This is **only active when Gemini is the LLM provider**.

### Usage
```go
import "codenerd/internal/tools/research"

// Create helper from any LLM client
helper := research.NewGroundingHelper(llmClient)

// Check if grounding is available (only for Gemini)
if helper.IsGroundingAvailable() {
    helper.EnableGoogleSearch()
    helper.EnableURLContext([]string{"https://docs.example.com"})

    response, sources, err := helper.CompleteWithGrounding(ctx, prompt)
    // sources contains URLs used for grounding
}
```

### Key Types
- `GroundingHelper` - Wraps any LLM client, activates grounding only for Gemini
- `GroundingStats` - Usage statistics (searches, URLs used)
- `GroundedResearchResult` - Query results with sources

### Documentation URLs
The helper includes `CommonDocURLs` map with well-known documentation URLs:
```go
research.GetDocURLsForTech("go")     // go.dev/doc, pkg.go.dev
research.GetDocURLsForTech("python") // docs.python.org
research.GetDocURLsForTech("react")  // react.dev
```

## Testing

```bash
go test ./internal/tools/...
```

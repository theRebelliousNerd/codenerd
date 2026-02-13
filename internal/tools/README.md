# internal/tools/

Modular Tool Registry for JIT-driven agent execution.

**Architecture Version:** 2.0.0 (December 2025 - JIT-Driven)

## Overview

This package provides modular tool definitions for the JIT Clean Loop architecture. Tools are standalone executables that any agent can use based on intent-driven JIT selection.

```text
Intent → ConfigFactory → AllowedTools[] → Registry.Get() → Tool.Execute()
```

## Structure

```text
tools/
├── types.go      # Tool, ToolCategory, ToolSchema types
├── registry.go   # Thread-safe tool registry
├── errors.go     # Error definitions
├── core/         # Filesystem tools
├── shell/        # Execution tools
├── codedom/      # Semantic code tools
└── research/     # Research tools
```

## Tool Categories

| Category | Directory | Tools |
|----------|-----------|-------|
| `/code` | `core/` | read_file, write_file, glob, grep, list_files |
| `/shell` | `shell/` | run_command, bash, run_build, run_tests |
| `/codedom` | `codedom/` | get_elements, get_element, edit_lines |
| `/research` | `research/` | context7_fetch, web_search, web_fetch, browser |

## Key Types

### Tool

```go
type Tool struct {
    Name            string
    Description     string
    Category        ToolCategory
    Execute         ExecuteFunc
    Schema          ToolSchema
    Priority        int
    RequiresContext bool
}
```

### Registry

```go
registry := tools.NewRegistry()

// Register tools
core.RegisterAll(registry)
shell.RegisterAll(registry)
codedom.RegisterAll(registry)
research.RegisterAll(registry)

// Execute
result, err := registry.Execute(ctx, "read_file", map[string]any{
    "path": "/path/to/file.go",
})
```

## Mangle Routing

Tools are routed via intent in `internal/mangle/intent_routing.mg`:

```mangle
modular_tool_allowed(/read_file, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/write_file, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/web_search, Intent) :- verb_category(Intent, /research).
```

## Research Tools

### Gemini Grounding Helper

For Gemini providers, the `research/grounding.go` helper enables Google Search and URL Context grounding:

```go
helper := research.NewGroundingHelper(llmClient)
if helper.IsGroundingAvailable() {
    helper.EnableGoogleSearch()
    response, sources, err := helper.CompleteWithGrounding(ctx, prompt)
}
```

## Testing

```bash
go test ./internal/tools/...
```

---

**Last Updated:** December 2024

> *[Archived & Reviewed by The Librarian on 2026-01-25]*


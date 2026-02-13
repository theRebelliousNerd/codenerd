# context/ - Dynamic Context Injection Atoms

Templates for injecting runtime context into prompts.

## Files

| File | Context Type | Injected Data |
|------|--------------|---------------|
| `file_context.yaml` | File Context | Current file content, path, language |
| `symbol_context.yaml` | Symbol Context | Functions, classes, dependencies |
| `error_context.yaml` | Error Context | Compiler errors, test failures, stack traces |
| `session_context.yaml` | Session Context | Conversation history, prior shard outputs |
| `encyclopedia.yaml` | Context Encyclopedia | All context types reference |

## Usage

Context atoms are **flesh** atoms dynamically populated at compile time with actual values from:
- VirtualStore queries
- SessionContext blackboard
- World model facts

## Template Pattern

```yaml
content: |
  ## Current File
  Path: {{.FilePath}}
  Language: {{.Language}}

  ```{{.Language}}
  {{.FileContent}}
```text
```


> *[Archived & Reviewed by The Librarian on 2026-01-25]*
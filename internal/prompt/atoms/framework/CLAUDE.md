# framework/ - Framework-Specific Atoms

Guidance for specific libraries and frameworks.

## Files

| File | Framework | Purpose |
|------|-----------|---------|
| `bubbletea.yaml` | Bubbletea | TUI framework (Elm architecture) |
| `bubbles.yaml` | Bubbles | Bubbletea component library |
| `lipgloss.yaml` | Lipgloss | Terminal styling |
| `glamour.yaml` | Glamour | Markdown rendering |
| `cobra.yaml` | Cobra | CLI framework |
| `rod.yaml` | Rod | Browser automation |
| `react.yaml` | React | Frontend framework |
| `django.yaml` | Django | Python web framework |
| `gin.yaml` | Gin | Go web framework |
| `sqlc.yaml` | sqlc | SQL code generation |
| `sqlite.yaml` | SQLite | Database patterns |
| `sqlite_vec.yaml` | sqlite-vec | Vector extension |
| `testify.yaml` | Testify | Go testing framework |
| `tree_sitter.yaml` | Tree-sitter | AST parsing |
| `yaegi.yaml` | Yaegi | Go interpreter |
| `zap.yaml` | Zap | Structured logging |
| `genai.yaml` | GenAI | Google AI SDK |

## Selection

Framework atoms are selected via `frameworks` selector:

```yaml
frameworks: ["/bubbletea", "/lipgloss"]
```

# internal/mangle - Datalog Inference Engine

This package implements Google Mangle, a Datalog-based inference engine that powers codeNERD's logical reasoning capabilities.

## Architecture

Mangle provides deductive database programming with:
- Fact storage and querying
- Rule-based inference
- Negation-as-failure
- Aggregation functions

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `engine.go` | ~880 | Main Mangle engine |
| `lsp.go` | ~920 | LSP integration for Mangle files |
| `grammar.go` | ~700 | Parser/lexer for .gl files |
| `policy.gl` | ~800 | Core policy rules |

## Key Types

### Engine
```go
type Engine struct {
    facts    map[string][]Fact
    rules    []Rule
    builtins map[string]BuiltinFunc
}

func (e *Engine) LoadFacts(facts []Fact) error
func (e *Engine) Query(predicate string) ([]Fact, error)
func (e *Engine) Eval(rule Rule) ([]Fact, error)
```

### Fact
```go
type Fact struct {
    Predicate string
    Args      []interface{}
}
```

### Rule
```go
type Rule struct {
    Head    Atom
    Body    []Atom
    Negated []Atom
}
```

## Policy Language (.gl files)

### Facts
```datalog
file_exists(/src/main.go).
user_intent(/session123, /mutation, /fix, "auth.go", "none").
```

### Rules
```datalog
needs_review(File) :-
    file_modified(File),
    not file_tested(File).

delegate_task(/reviewer, Target, /pending) :-
    user_intent(_, _, /review, Target, _).
```

### Aggregation
```datalog
total_lines(Sum) :-
    Sum = fn:sum(Lines),
    file_lines(_, Lines).
```

## Core Policy Sections

1. **Intent Routing** - Map verbs to actions
2. **Context Selection** - Spreading activation
3. **Delegation Rules** - Shard routing
4. **Strategy Selection** - Campaign triggers
5. **Abductive Reasoning** - Hypothesis generation

## Built-in Functions

| Function | Description |
|----------|-------------|
| `fn:sum` | Sum aggregation |
| `fn:count` | Count aggregation |
| `fn:min`/`fn:max` | Min/max values |
| `fn:contains` | String containment |
| `fn:matches` | Regex matching |

## LSP Support

The package includes LSP server for .gl files:
- Syntax highlighting
- Diagnostics
- Completion
- Hover documentation

## Dependencies

- Standard library only (no external deps)

## Testing

```bash
go test ./internal/mangle/...
```

## Adding Rules

1. Edit `policy.gl` with new rules
2. Rules are hot-reloaded on `/init`
3. Test with `/query <predicate>` command

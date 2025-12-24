---
name: go-architect
description: Write production-ready, idiomatic Go code following Uber Go Style Guide patterns. This skill prevents common AI coding agent failures including goroutine leaks, race conditions, improper error handling, context mismanagement, and memory leaks. Includes Mangle integration patterns and JIT compiler patterns (go:embed, callback wiring, multi-source compilation, SQLite table extension). Use when writing, reviewing, or refactoring any Go code.
license: Apache-2.0
version: 1.2.0
go_version: 1.21+
last_updated: 2025-12-23
---

# Go Architect: Production-Ready Idiomatic Go

This skill ensures Claude writes safe, idiomatic, production-ready Go code. It addresses documented failure modes where AI coding agents generate code that compiles but harbors latent defects.

## CRITICAL: Before Writing Go Code

AI agents consistently fail at Go due to training bias toward Python/JavaScript/Java. Go's explicit error handling, CSP concurrency model, and strict type system require different mental models.

**The Competence-Confidence Gap**: AI agents generate syntactically correct Go that often:

- Compiles successfully but deadlocks at runtime
- Leaks goroutines that accumulate until OOM
- Ignores errors that cause silent data corruption
- Breaks context cancellation chains
- Creates race conditions on shared state

**Rule**: Every piece of generated Go code must pass the validation checklists in this skill.

## Critical Failure Modes (Quick Reference)

| Failure | Severity | Wrong Pattern | Correct Pattern |
|---------|----------|---------------|-----------------|
| Goroutine Leak | CRITICAL | `ch := make(chan T)` unbuffered | `ch := make(chan T, 1)` buffered |
| WaitGroup Race | HIGH | `wg.Add(1)` inside goroutine | `wg.Add(1)` before `go func()` |
| Map Race | CRITICAL | Concurrent map access | `sync.RWMutex` or `sync.Map` |
| Context Severance | MEDIUM | `context.Background()` in handler | Derive from parent: `context.WithTimeout(ctx, ...)` |
| Error Suppression | HIGH | `_, _ := f()` | Handle every error |
| Panic Abuse | HIGH | `panic(err)` for control flow | Return errors |
| Slice Leak | MEDIUM | Return sub-slice of large data | Copy to new slice |
| Nil Channel | CRITICAL | `var ch chan T` | `ch := make(chan T)` |

For detailed examples and anti-patterns, see [100-AI_FAILURE_MODES](references/100-AI_FAILURE_MODES.md).

## Essential Patterns

### Goroutine with Guaranteed Exit

```go
func fetchWithTimeout(ctx context.Context) (string, error) {
    ch := make(chan string, 1)  // Buffered! Sender never blocks

    go func() {
        result := slowOperation()
        ch <- result  // Completes even if no receiver
    }()

    select {
    case result := <-ch:
        return result, nil
    case <-ctx.Done():
        return "", ctx.Err()
    }
}
```

### Mutex-Protected Map

```go
type SafeCache struct {
    mu    sync.RWMutex
    cache map[string]string
}

func (c *SafeCache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.cache[key]
    return val, ok
}
```

### Error Wrapping with Context

```go
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config file %s: %w", path, err)
    }
    // ... use data
}
```

For complete patterns, see [200-CONCURRENCY_PATTERNS](references/200-CONCURRENCY_PATTERNS.md) and [300-ERROR_HANDLING](references/300-ERROR_HANDLING.md).

## Idiomatic Go Quick Reference

### Interface Design

```go
// Accept interfaces, return structs
func ProcessReader(r io.Reader) error { ... }
func NewServer(addr string) *Server { ... }

// Compile-time interface verification
var _ Handler = (*MyType)(nil)
```

### Functional Options

```go
type ServerOption func(*Server)

func WithTimeout(d time.Duration) ServerOption {
    return func(s *Server) { s.timeout = d }
}

srv := NewServer(":8080", WithTimeout(time.Minute))
```

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 1, 2, 3},
        {"negative", -1, -1, -2},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Add(tt.a, tt.b); got != tt.expected {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

## Mangle Integration

codeNERD uses Mangle (Datalog) as its logic kernel. Critical understanding:

### What Mangle CAN'T Do

**The "Mangle as HashMap" Anti-Pattern**: Don't expect fuzzy matching from Mangle.

```go
// WRONG - expecting semantic matching
facts := []Fact{
    {Predicate: "intent_definition", Args: []interface{}{"review my code", "/review"}},
}
// User says "audit my code" → NO MATCH (exact string matching only!)
```

**Mangle has NO string functions**: `fn:string_contains`, `fn:substring`, `fn:regex` DO NOT EXIST.

### Neuro-Symbolic Solution

```go
// 1. Vector search for semantic matching
matches := semanticClassifier.Search(ctx, userInput)

// 2. Inject as Mangle facts
for _, m := range matches {
    kernel.Assert(Fact{
        Predicate: "semantic_match",
        Args: []interface{}{userInput, m.Sentence, m.Verb, m.Similarity},
    })
}

// 3. Mangle derives final selection using deductive rules
```

For complete Mangle integration patterns, see [400-MANGLE_INTEGRATION](references/400-MANGLE_INTEGRATION.md).

## JIT Compiler Patterns

For codeNERD's JIT Prompt Compiler development:

- **go:embed** - Compile-time asset embedding
- **Callback Wiring** - Dependency injection without import cycles
- **SQLite Table Extension** - Idempotent schema creation
- **Multi-Source Compilation** - Tiered atom loading with graceful degradation
- **FeedbackLoop Adapter** - LLM client interface adapters

See [600-JIT_PATTERNS](references/600-JIT_PATTERNS.md) for complete examples.

## Validation Checklist

Before submitting any Go code, verify:

### Concurrency

- [ ] Every `go func()` has guaranteed termination
- [ ] Channels are buffered appropriately (no forgotten senders)
- [ ] `wg.Add(1)` is called BEFORE `go func()`
- [ ] Maps accessed from multiple goroutines have mutex protection
- [ ] Context is propagated (no `context.Background()` in handlers)

### Error Handling

- [ ] No ignored errors (no `_, _` except in documented cases)
- [ ] Errors are wrapped with context (`fmt.Errorf("...: %w", err)`)
- [ ] No `panic` for recoverable errors
- [ ] Sentinel errors defined for common cases

### Memory

- [ ] Large slice sub-slices are copied
- [ ] Channels are initialized with `make()`
- [ ] Resources are closed (using `defer`)
- [ ] Temp files from `go:embed` are cleaned up

### Security

- [ ] Using `crypto/rand` for security-sensitive randomness
- [ ] SQL queries are parameterized
- [ ] Dependencies are verified to exist

### Mangle Integration

- [ ] Using `engine.Atom()` for constants, `engine.String()` for text
- [ ] Running `analysis.Analyze()` before engine creation
- [ ] Proper error handling for parse/analysis failures
- [ ] Context propagation in queries

### JIT Compiler Patterns

- [ ] Embedded assets checked for zero-length before use
- [ ] Callbacks are nil-checked before invocation
- [ ] SQLite table creation uses `CREATE TABLE IF NOT EXISTS`
- [ ] Multi-source loading continues on error (graceful degradation)
- [ ] Mutex protection when accessing shared DB maps

## Reference Library

| Reference | Contents |
|-----------|----------|
| [100-AI_FAILURE_MODES](references/100-AI_FAILURE_MODES.md) | Complete AI failure taxonomy with examples |
| [200-CONCURRENCY_PATTERNS](references/200-CONCURRENCY_PATTERNS.md) | Worker pools, graceful shutdown, channels |
| [300-ERROR_HANDLING](references/300-ERROR_HANDLING.md) | Error wrapping, sentinel errors, custom types |
| [400-MANGLE_INTEGRATION](references/400-MANGLE_INTEGRATION.md) | Complete Mangle/Go integration guide |
| [500-UBER_STYLE_GUIDE](references/500-UBER_STYLE_GUIDE.md) | Uber Go Style Guide summary |
| [600-JIT_PATTERNS](references/600-JIT_PATTERNS.md) | go:embed, callbacks, SQLite, multi-source |

## Resources

- [Uber Go Style Guide](https://github.com/uber-go/guide)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Go Memory Model](https://go.dev/ref/mem)
- [Google Mangle](https://github.com/google/mangle)

---

**Next step**: For Mangle-specific patterns, see the [mangle-programming skill](../mangle-programming/SKILL.md).

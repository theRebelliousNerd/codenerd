# Uber Go Style Guide Summary

This reference summarizes key patterns from the [Uber Go Style Guide](https://github.com/uber-go/guide), the industry-standard reference for idiomatic Go code.

## Style Principles

### Reduce Nesting

```go
// BAD - Deep nesting
func process(items []Item) error {
    if items != nil {
        for _, item := range items {
            if item.Valid {
                if err := process(item); err != nil {
                    return err
                }
            }
        }
    }
    return nil
}

// GOOD - Early returns
func process(items []Item) error {
    if items == nil {
        return nil
    }

    for _, item := range items {
        if !item.Valid {
            continue
        }
        if err := process(item); err != nil {
            return err
        }
    }
    return nil
}
```

### Reduce Scope

```go
// BAD - Variable scope too wide
err := os.WriteFile(path, data, 0644)
if err != nil {
    return err
}
// err still in scope

// GOOD - Minimal scope
if err := os.WriteFile(path, data, 0644); err != nil {
    return err
}
```

### Avoid Naked Parameters

```go
// BAD - Unclear what parameters mean
printInfo("foo", true, true)

// GOOD - Use named struct or clear variables
printInfo("foo", PrintOptions{
    Recursive: true,
    Verbose:   true,
})
```

## Naming Conventions

### Packages

```go
// BAD
package common    // Too generic
package utils     // Too generic
package helpers   // Too generic

// GOOD
package http      // Clear purpose
package json      // Clear purpose
package user      // Clear domain
```

### Variables

```go
// Short names for short scopes
for i := 0; i < len(items); i++ { }

// Longer names for longer scopes
userPermissions := fetchPermissions(userID)
```

### Receivers

```go
// Use consistent short names (1-2 chars)
func (s *Server) Start() { }
func (s *Server) Stop() { }
func (s *Server) Handle(req Request) { }
```

### Interface Names

```go
// Single method interfaces use -er suffix
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }

// Multi-method interfaces describe behavior
type ReadWriter interface {
    Reader
    Writer
}
```

## Error Handling

### Error Wrapping

```go
// BAD - Lose original error
return errors.New("failed to connect")

// GOOD - Wrap with context
return fmt.Errorf("connect to %s: %w", addr, err)
```

### Error Types

```go
// Define custom error types when needed
type NotFoundError struct {
    Resource string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s not found", e.Resource)
}

// Use errors.Is/As for checking
var notFoundErr *NotFoundError
if errors.As(err, &notFoundErr) {
    // Handle not found
}
```

### Error Naming

```go
// Error types end with Error
type PathError struct { }
type SyntaxError struct { }

// Error variables start with Err
var ErrNotFound = errors.New("not found")
var ErrInvalidInput = errors.New("invalid input")
```

## Functions

### Accept Interfaces, Return Structs

```go
// GOOD - Accept interface
func Process(r io.Reader) error { }

// GOOD - Return concrete type
func NewServer() *Server { }

// BAD - Return interface (hides implementation)
func NewServer() Server { }
```

### Functional Options

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func WithLogger(l Logger) Option {
    return func(s *Server) {
        s.logger = l
    }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{
        addr:    addr,
        timeout: defaultTimeout,
        logger:  defaultLogger,
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

## Concurrency

### Start Goroutines in Functions

```go
// BAD - Goroutine started in init
func init() {
    go monitor()
}

// GOOD - Explicit startup with lifecycle control
func (s *Server) Start() {
    s.wg.Add(1)
    go s.monitor()
}

func (s *Server) Stop() {
    close(s.done)
    s.wg.Wait()
}
```

### No Fire-and-Forget Goroutines

```go
// BAD
go func() {
    // No way to wait or stop
}()

// GOOD
s.wg.Add(1)
go func() {
    defer s.wg.Done()
    // Work
}()
// Later: s.wg.Wait()
```

### Channel Size

```go
// Prefer unbuffered or size 1
ch := make(chan int)    // Unbuffered - synchronous
ch := make(chan int, 1) // Size 1 - async but bounded

// Large buffers need justification
ch := make(chan int, 100) // Why 100? Document!
```

## Structs

### Use Field Names in Literals

```go
// BAD - Positional
k := User{"John", "john@example.com", 25}

// GOOD - Named fields
k := User{
    Name:  "John",
    Email: "john@example.com",
    Age:   25,
}
```

### Embed for Behavior, Not State

```go
// GOOD - Embed for behavior
type Client struct {
    http.Client  // Gets all http.Client methods
    baseURL string
}

// BAD - Embed for state sharing
type Server struct {
    sync.Mutex  // Exposes Lock/Unlock publicly!
}

// GOOD - Private mutex
type Server struct {
    mu sync.Mutex
}
```

### Zero Values

```go
// Use zero values when meaningful
type Buffer struct {
    data []byte
    // Zero value is valid empty buffer
}

// Document when zero value is NOT valid
type Server struct {
    addr string
    // Use NewServer() - zero value is not usable
}
```

## Slices and Maps

### Nil vs Empty Slices

```go
// Nil slice is valid for most operations
var s []int
len(s)         // 0
cap(s)         // 0
for _, v := range s { } // Works fine
s = append(s, 1) // Works

// But JSON marshaling differs
json.Marshal([]int(nil)) // null
json.Marshal([]int{})    // []
```

### Reduce Scope of Slices

```go
// BAD - Creates sub-slice that references large array
func getHeader(data []byte) []byte {
    return data[:64]  // Keeps entire array in memory
}

// GOOD - Copy to new slice
func getHeader(data []byte) []byte {
    header := make([]byte, 64)
    copy(header, data[:64])
    return header
}
```

### Pre-allocate When Possible

```go
// BAD - Repeated allocation
var items []Item
for _, raw := range rawItems {
    items = append(items, process(raw))
}

// GOOD - Pre-allocate
items := make([]Item, 0, len(rawItems))
for _, raw := range rawItems {
    items = append(items, process(raw))
}
```

### Map Initialization

```go
// Empty map - use make
m := make(map[string]int)

// Map with initial values - use literal
m := map[string]int{
    "a": 1,
    "b": 2,
}
```

## Performance

### strconv Over fmt

```go
// BAD
s := fmt.Sprintf("%d", n)

// GOOD
s := strconv.Itoa(n)
```

### Avoid String Conversion in Loops

```go
// BAD
for i := 0; i < b.N; i++ {
    process([]byte(s)) // Allocates each iteration
}

// GOOD
data := []byte(s)
for i := 0; i < b.N; i++ {
    process(data)
}
```

### Prefer Specifying Map Capacity

```go
// BAD - May reallocate multiple times
m := make(map[string]int)

// GOOD - Single allocation
m := make(map[string]int, len(keys))
```

## Testing

### Table Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 1, 2, 3},
        {"negative", -1, -1, -2},
        {"zero", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.expected {
                t.Errorf("Add(%d, %d) = %d, want %d",
                    tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

### Subtests for Cleanup

```go
func TestDB(t *testing.T) {
    db := setupTestDB(t)
    t.Cleanup(func() {
        db.Close()
    })

    t.Run("insert", func(t *testing.T) {
        // Test insert
    })

    t.Run("query", func(t *testing.T) {
        // Test query
    })
}
```

## Linting

### Recommended Linters

Use [golangci-lint](https://golangci-lint.run/) with these linters:

```yaml
linters:
  enable:
    - errcheck      # Check error returns
    - gosimple      # Simplify code
    - govet         # Vet issues
    - ineffassign   # Unused assignments
    - staticcheck   # Static analysis
    - unused        # Unused code
    - gofmt         # Format check
    - goimports     # Import check
```

### Running Linters

```bash
# Install
go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest

# Run
golangci-lint run ./...
```

## Documentation

### Package Comments

```go
// Package user provides user management functionality.
//
// It handles user creation, authentication, and authorization.
package user
```

### Function Comments

```go
// ProcessItems processes all items in the queue.
// It returns an error if any item fails validation.
//
// Items are processed concurrently with a maximum of 10 workers.
func ProcessItems(items []Item) error { }
```

### Examples

```go
func ExampleAdd() {
    result := Add(1, 2)
    fmt.Println(result)
    // Output: 3
}
```

## Import Organization

```go
import (
    // Standard library
    "context"
    "fmt"
    "net/http"

    // External packages
    "github.com/gorilla/mux"
    "go.uber.org/zap"

    // Internal packages
    "mycompany.com/myproject/internal/user"
    "mycompany.com/myproject/pkg/auth"
)
```

Use `goimports` to automatically organize imports.

## References

- [Uber Go Style Guide](https://github.com/uber-go/guide)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Go Proverbs](https://go-proverbs.github.io/)

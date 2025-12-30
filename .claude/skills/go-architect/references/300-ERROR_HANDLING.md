# Go Error Handling Patterns

This reference covers idiomatic error handling in Go, with emphasis on patterns that AI agents commonly get wrong.

## Core Principles

### Errors Are Values

In Go, errors are not exceptional control flow (like exceptions). They are values returned from functions that should be checked and handled.

```go
// Go philosophy: Explicit error handling
result, err := operation()
if err != nil {
    // Handle error explicitly
    return fmt.Errorf("operation failed: %w", err)
}
// Use result
```

### Handle Once, Log Once

An error should be handled exactly once. Logging is a form of handling.

```go
// WRONG - Handles error multiple times
func process() error {
    result, err := fetch()
    if err != nil {
        log.Printf("fetch failed: %v", err) // Handle 1: Log
        return err // Handle 2: Return
    }
    return nil
}

// Caller also logs
func caller() {
    if err := process(); err != nil {
        log.Printf("process failed: %v", err) // Handle 3: Log again!
    }
}
```

```go
// CORRECT - Handle once at boundary
func process() error {
    result, err := fetch()
    if err != nil {
        return fmt.Errorf("fetch: %w", err) // Add context, don't log
    }
    return nil
}

// Handle at system boundary
func handler(w http.ResponseWriter, r *http.Request) {
    if err := process(); err != nil {
        log.Printf("request failed: %v", err) // Log once at boundary
        http.Error(w, "internal error", 500)
    }
}
```

## Error Wrapping

### Adding Context with %w

```go
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        // Wrap with context - preserves original error
        return nil, fmt.Errorf("read config %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config JSON: %w", err)
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }

    return &cfg, nil
}
```

### Error Chain Inspection

```go
// Check if error is (or wraps) a specific error
if errors.Is(err, os.ErrNotExist) {
    // File not found
}

// Check if error is (or wraps) a specific type
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    fmt.Printf("operation %s failed on path %s\n", pathErr.Op, pathErr.Path)
}
```

### When NOT to Wrap

Don't wrap when:
1. It doesn't add useful context
2. You're at a package boundary and want to hide implementation details
3. The wrapped error creates confusing chains

```go
// TOO MUCH WRAPPING - Noisy chain
return fmt.Errorf("process: %w",
    fmt.Errorf("handle: %w",
        fmt.Errorf("fetch: %w", err)))

// Result: "process: handle: fetch: connection refused"

// BETTER - Add context only where meaningful
return fmt.Errorf("process user %s: %w", userID, err)
```

## Sentinel Errors

### Defining Sentinel Errors

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrConflict     = errors.New("conflict")
    ErrInvalidInput = errors.New("invalid input")
)
```

### Using Sentinel Errors

```go
func GetUser(id string) (*User, error) {
    user, ok := users[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}

// Caller checks with errors.Is
user, err := GetUser(id)
if errors.Is(err, ErrNotFound) {
    // Handle not found case
}
```

### Wrapping Sentinel Errors

```go
func GetUserProfile(id string) (*Profile, error) {
    user, err := GetUser(id)
    if err != nil {
        return nil, fmt.Errorf("get user %s: %w", id, err)
    }
    // ...
}

// Still works with errors.Is
if errors.Is(err, ErrNotFound) {
    // Matches even through wrapping
}
```

## Custom Error Types

### Simple Custom Error

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// Usage
func ValidateUser(u *User) error {
    if u.Name == "" {
        return &ValidationError{Field: "name", Message: "required"}
    }
    if len(u.Email) < 5 {
        return &ValidationError{Field: "email", Message: "too short"}
    }
    return nil
}
```

### Error with Unwrap

```go
type QueryError struct {
    Query string
    Err   error
}

func (e *QueryError) Error() string {
    return fmt.Sprintf("query %q failed: %v", e.Query, e.Err)
}

func (e *QueryError) Unwrap() error {
    return e.Err
}

// Now errors.Is and errors.As work through QueryError
```

### Multiple Wrapped Errors (Go 1.20+)

```go
func ValidateAll(items []Item) error {
    var errs []error
    for _, item := range items {
        if err := Validate(item); err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return errors.Join(errs...) // Joins multiple errors
    }
    return nil
}
```

## Error Handling Patterns

### Early Return

```go
func process(input string) (Result, error) {
    // Validate early
    if input == "" {
        return Result{}, ErrInvalidInput
    }

    // Each step can fail - return early
    data, err := fetch(input)
    if err != nil {
        return Result{}, fmt.Errorf("fetch: %w", err)
    }

    parsed, err := parse(data)
    if err != nil {
        return Result{}, fmt.Errorf("parse: %w", err)
    }

    result, err := transform(parsed)
    if err != nil {
        return Result{}, fmt.Errorf("transform: %w", err)
    }

    return result, nil
}
```

### Defer for Cleanup

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("open: %w", err)
    }
    defer f.Close() // Always closes, even on error

    // Process file...
    return nil
}
```

### Defer with Error Handling

```go
func processFile(path string) (err error) {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("open: %w", err)
    }
    defer func() {
        if cerr := f.Close(); cerr != nil && err == nil {
            err = fmt.Errorf("close: %w", cerr)
        }
    }()

    // Process file...
    return nil
}
```

### Error Aggregation

```go
type MultiError struct {
    Errors []error
}

func (m *MultiError) Error() string {
    var msgs []string
    for _, e := range m.Errors {
        msgs = append(msgs, e.Error())
    }
    return strings.Join(msgs, "; ")
}

func (m *MultiError) Add(err error) {
    if err != nil {
        m.Errors = append(m.Errors, err)
    }
}

func (m *MultiError) Err() error {
    if len(m.Errors) == 0 {
        return nil
    }
    return m
}

// Usage
func validateAll(items []Item) error {
    var errs MultiError
    for _, item := range items {
        errs.Add(validate(item))
    }
    return errs.Err()
}
```

## Panic vs Error

### When to Panic

Panic is for unrecoverable programmer errors:

```go
// OK to panic - Programmer error
func MustParse(s string) Value {
    v, err := Parse(s)
    if err != nil {
        panic(fmt.Sprintf("MustParse(%q): %v", s, err))
    }
    return v
}

// OK to panic - Invalid internal state
func (s *Server) handler() {
    if s.db == nil {
        panic("server: db is nil") // Should never happen
    }
}
```

### When NOT to Panic

```go
// WRONG - Don't panic on user input
func ParseUserInput(input string) Config {
    var cfg Config
    if err := json.Unmarshal([]byte(input), &cfg); err != nil {
        panic(err) // WRONG - crashes server on bad input
    }
    return cfg
}

// CORRECT - Return error
func ParseUserInput(input string) (Config, error) {
    var cfg Config
    if err := json.Unmarshal([]byte(input), &cfg); err != nil {
        return Config{}, fmt.Errorf("parse input: %w", err)
    }
    return cfg, nil
}
```

### Recovery

```go
// Recover from panics in handlers
func recoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("panic recovered: %v\n%s", err, debug.Stack())
                http.Error(w, "internal error", 500)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## HTTP Error Handling

### Standard Patterns

```go
// Error response helper
func respondError(w http.ResponseWriter, code int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Handler with error handling
func getUser(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    user, err := userService.Get(r.Context(), id)
    if errors.Is(err, ErrNotFound) {
        respondError(w, http.StatusNotFound, "user not found")
        return
    }
    if err != nil {
        log.Printf("get user %s: %v", id, err)
        respondError(w, http.StatusInternalServerError, "internal error")
        return
    }

    json.NewEncoder(w).Encode(user)
}
```

### Error Handler Pattern

```go
type AppHandler func(http.ResponseWriter, *http.Request) error

func (fn AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if err := fn(w, r); err != nil {
        var appErr *AppError
        if errors.As(err, &appErr) {
            respondError(w, appErr.Code, appErr.Message)
            return
        }
        log.Printf("unhandled error: %v", err)
        respondError(w, 500, "internal error")
    }
}

// Handler returns error
func getUser(w http.ResponseWriter, r *http.Request) error {
    id := chi.URLParam(r, "id")

    user, err := userService.Get(r.Context(), id)
    if errors.Is(err, ErrNotFound) {
        return &AppError{Code: 404, Message: "user not found"}
    }
    if err != nil {
        return err // Let middleware handle
    }

    return json.NewEncoder(w).Encode(user)
}

// Usage
mux.Handle("/users/{id}", AppHandler(getUser))
```

## Database Error Handling

### Transaction Error Handling

```go
func UpdateUserInTx(ctx context.Context, db *sql.DB, user *User) error {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback() // Rollback if not committed

    if err := updateUser(ctx, tx, user); err != nil {
        return fmt.Errorf("update user: %w", err)
    }

    if err := updateAuditLog(ctx, tx, user.ID); err != nil {
        return fmt.Errorf("update audit: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    return nil
}
```

### sql.ErrNoRows Handling

```go
func GetUser(ctx context.Context, db *sql.DB, id string) (*User, error) {
    var user User
    err := db.QueryRowContext(ctx,
        "SELECT id, name, email FROM users WHERE id = ?", id,
    ).Scan(&user.ID, &user.Name, &user.Email)

    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrNotFound // Convert to domain error
    }
    if err != nil {
        return nil, fmt.Errorf("query user: %w", err)
    }

    return &user, nil
}
```

## Concurrent Error Handling

### Error Channel Pattern

```go
func processAll(ctx context.Context, items []Item) error {
    errCh := make(chan error, len(items))
    var wg sync.WaitGroup

    for _, item := range items {
        wg.Add(1)
        go func(item Item) {
            defer wg.Done()
            if err := process(ctx, item); err != nil {
                errCh <- fmt.Errorf("item %s: %w", item.ID, err)
            }
        }(item)
    }

    go func() {
        wg.Wait()
        close(errCh)
    }()

    var errs []error
    for err := range errCh {
        errs = append(errs, err)
    }

    return errors.Join(errs...)
}
```

### errgroup Pattern

```go
import "golang.org/x/sync/errgroup"

func processAll(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item
        g.Go(func() error {
            return process(ctx, item)
        })
    }

    return g.Wait() // Returns first error
}
```

## Testing Errors

### Testing Error Conditions

```go
func TestGetUser_NotFound(t *testing.T) {
    user, err := GetUser("nonexistent")

    if !errors.Is(err, ErrNotFound) {
        t.Errorf("expected ErrNotFound, got %v", err)
    }
    if user != nil {
        t.Errorf("expected nil user, got %v", user)
    }
}
```

### Testing Error Types

```go
func TestValidate_ValidationError(t *testing.T) {
    err := Validate(&User{Name: ""})

    var validErr *ValidationError
    if !errors.As(err, &validErr) {
        t.Fatalf("expected ValidationError, got %T", err)
    }
    if validErr.Field != "name" {
        t.Errorf("expected field 'name', got %q", validErr.Field)
    }
}
```

### Testing Error Messages

```go
func TestLoadConfig_FileNotFound(t *testing.T) {
    _, err := LoadConfig("/nonexistent/path")

    if err == nil {
        t.Fatal("expected error")
    }

    // Check error chain
    if !errors.Is(err, os.ErrNotExist) {
        t.Errorf("expected os.ErrNotExist in chain, got %v", err)
    }

    // Check context was added
    if !strings.Contains(err.Error(), "read config") {
        t.Errorf("error should contain 'read config': %v", err)
    }
}
```

## Anti-Patterns

### 1. Ignoring Errors

```go
// WRONG
result, _ := mayFail()

// CORRECT
result, err := mayFail()
if err != nil {
    return err
}
```

### 2. Empty Error Messages

```go
// WRONG
return errors.New("")

// CORRECT
return errors.New("operation failed: specific reason")
```

### 3. Error String Matching

```go
// WRONG - Fragile
if err.Error() == "not found" {
    // ...
}

// CORRECT - Use sentinel or type
if errors.Is(err, ErrNotFound) {
    // ...
}
```

### 4. Logging Then Returning

```go
// WRONG - Double handling
log.Printf("error: %v", err)
return err

// CORRECT - One or the other
return fmt.Errorf("operation: %w", err)  // Return with context
// OR
log.Printf("error: %v", err)  // Handle completely here
return nil  // Don't propagate
```

### 5. Using panic for Control Flow

```go
// WRONG
func Find(items []Item, id string) Item {
    for _, item := range items {
        if item.ID == id {
            return item
        }
    }
    panic("not found") // WRONG
}

// CORRECT
func Find(items []Item, id string) (Item, error) {
    for _, item := range items {
        if item.ID == id {
            return item, nil
        }
    }
    return Item{}, ErrNotFound
}
```

## Summary

| Pattern | Use When |
|---------|----------|
| `fmt.Errorf("...: %w", err)` | Adding context to errors |
| `errors.Is(err, target)` | Checking for sentinel errors |
| `errors.As(err, &target)` | Checking for error types |
| `errors.Join(errs...)` | Combining multiple errors |
| Custom error type | Need structured error data |
| Sentinel error | Caller needs to distinguish error kinds |
| Early return | Multiple failure points |
| defer for cleanup | Resources need closing |

# Stability Patterns for Bubbletea Applications

Best practices and patterns for building reliable, crash-resistant TUI applications.

## Table of Contents

- [Critical Rules](#critical-rules)
- [Command Safety](#command-safety)
- [Resource Management](#resource-management)
- [Error Handling](#error-handling)
- [Goroutine Safety](#goroutine-safety)
- [State Management](#state-management)
- [Terminal Recovery](#terminal-recovery)
- [Testing Strategies](#testing-strategies)
- [Performance Patterns](#performance-patterns)
- [Common Pitfalls](#common-pitfalls)

---

## Critical Rules

### Rule 1: Never Block in Update()

The Update function runs in the main event loop. Blocking it freezes the entire UI.

```go
// BAD - Blocks the event loop
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "f" {
            data, _ := http.Get("https://api.example.com")  // BLOCKS!
            m.data = data
        }
    }
    return m, nil
}

// GOOD - Use commands for I/O
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "f" {
            m.loading = true
            return m, fetchDataCmd  // Returns immediately
        }
    case dataMsg:
        m.loading = false
        m.data = msg.data
    case errMsg:
        m.loading = false
        m.err = msg.err
    }
    return m, nil
}

func fetchDataCmd() tea.Msg {
    resp, err := http.Get("https://api.example.com")
    if err != nil {
        return errMsg{err}
    }
    defer resp.Body.Close()
    // Process response...
    return dataMsg{data}
}
```

### Rule 2: Always Handle Window Resize

Not handling resize leads to broken layouts and panics on dimension-dependent components.

```go
type model struct {
    width    int
    height   int
    viewport viewport.Model
    ready    bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

        headerHeight := 3
        footerHeight := 2
        contentHeight := msg.Height - headerHeight - footerHeight

        if !m.ready {
            // Initial setup
            m.viewport = viewport.New(msg.Width, contentHeight)
            m.viewport.SetContent(m.content)
            m.ready = true
        } else {
            // Resize
            m.viewport.Width = msg.Width
            m.viewport.Height = contentHeight
        }
    }
    return m, nil
}

func (m model) View() string {
    if !m.ready {
        return "Initializing..."
    }
    // Safe to render now
    return m.viewport.View()
}
```

### Rule 3: Always Provide Quit Mechanism

Users must always be able to exit the application.

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        // Multiple quit options for reliability
        case "ctrl+c":
            return m, tea.Quit
        case "q":
            if !m.editing {  // Only if not in text input mode
                return m, tea.Quit
            }
        case "esc":
            if m.editing {
                m.editing = false  // Exit edit mode first
            } else {
                return m, tea.Quit  // Then quit
            }
        }
    }
    return m, nil
}
```

### Rule 4: Use Value Receivers

Bubbletea expects value semantics for model updates.

```go
// BAD - Pointer receiver can cause race conditions
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    m.count++  // Mutating in place
    return m, nil
}

// GOOD - Value receiver, returns modified copy
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    m.count++  // Modifying local copy
    return m, nil  // Return the modified copy
}
```

### Rule 5: Clean Up Before Quitting

Release resources before the program exits.

```go
type model struct {
    cancel context.CancelFunc
    db     *sql.DB
    file   *os.File
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            // Clean up before quitting
            if m.cancel != nil {
                m.cancel()
            }
            if m.db != nil {
                m.db.Close()
            }
            if m.file != nil {
                m.file.Close()
            }
            return m, tea.Quit
        }
    }
    return m, nil
}
```

---

## Command Safety

### Timeout Protection

Prevent commands from hanging indefinitely.

```go
func fetchWithTimeout(url string, timeout time.Duration) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), timeout)
        defer cancel()

        req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
        if err != nil {
            return errMsg{err}
        }

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            if ctx.Err() == context.DeadlineExceeded {
                return errMsg{fmt.Errorf("request timed out after %v", timeout)}
            }
            return errMsg{err}
        }
        defer resp.Body.Close()

        // Process response...
        return dataMsg{data}
    }
}

// Usage
return m, fetchWithTimeout("https://api.example.com", 10*time.Second)
```

### Cancellable Commands

Support cancellation for long-running operations.

```go
type model struct {
    ctx    context.Context
    cancel context.CancelFunc
}

func initialModel() model {
    ctx, cancel := context.WithCancel(context.Background())
    return model{ctx: ctx, cancel: cancel}
}

func (m model) startLongTask() tea.Cmd {
    ctx := m.ctx  // Capture context
    return func() tea.Msg {
        result := make(chan dataMsg)
        errCh := make(chan error)

        go func() {
            data, err := longRunningOperation()
            if err != nil {
                errCh <- err
                return
            }
            result <- dataMsg{data}
        }()

        select {
        case <-ctx.Done():
            return cancelledMsg{}
        case err := <-errCh:
            return errMsg{err}
        case data := <-result:
            return data
        }
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "c" {
            m.cancel()  // Cancel ongoing operations
            // Create new context for future operations
            m.ctx, m.cancel = context.WithCancel(context.Background())
        }
    }
    return m, nil
}
```

### Command Error Handling

Always handle errors in commands.

```go
type errMsg struct {
    err     error
    source  string  // Where the error came from
    retryable bool
}

func (e errMsg) Error() string {
    return fmt.Sprintf("%s: %v", e.source, e.err)
}

func fetchDataCmd(url string) tea.Cmd {
    return func() tea.Msg {
        resp, err := http.Get(url)
        if err != nil {
            // Network errors are usually retryable
            return errMsg{
                err:       err,
                source:    "fetch",
                retryable: true,
            }
        }
        defer resp.Body.Close()

        if resp.StatusCode != 200 {
            return errMsg{
                err:       fmt.Errorf("HTTP %d", resp.StatusCode),
                source:    "fetch",
                retryable: resp.StatusCode >= 500,
            }
        }

        var data Data
        if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
            return errMsg{
                err:       err,
                source:    "parse",
                retryable: false,
            }
        }

        return dataMsg{data}
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case errMsg:
        m.err = msg.err
        m.errSource = msg.source
        if msg.retryable && m.retryCount < 3 {
            m.retryCount++
            return m, tea.Tick(time.Second*time.Duration(m.retryCount),
                func(time.Time) tea.Msg {
                    return retryMsg{}
                })
        }
    case retryMsg:
        return m, fetchDataCmd(m.url)
    }
    return m, nil
}
```

---

## Resource Management

### File Handle Safety

```go
type model struct {
    logFile *os.File
}

func initialModel() model {
    f, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        // Handle gracefully - log to stderr instead
        return model{}
    }
    return model{logFile: f}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            if m.logFile != nil {
                m.logFile.Sync()   // Flush pending writes
                m.logFile.Close()
            }
            return m, tea.Quit
        }
    }
    return m, nil
}
```

### Database Connections

```go
type model struct {
    db *sql.DB
}

func initialModel() model {
    db, err := sql.Open("sqlite3", "app.db")
    if err != nil {
        return model{}
    }

    // Configure connection pool
    db.SetMaxOpenConns(5)
    db.SetMaxIdleConns(2)
    db.SetConnMaxLifetime(time.Minute * 5)

    return model{db: db}
}

// Query command with proper cleanup
func queryCmd(db *sql.DB, query string) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        rows, err := db.QueryContext(ctx, query)
        if err != nil {
            return errMsg{err}
        }
        defer rows.Close()  // Always close rows

        var results []Row
        for rows.Next() {
            var r Row
            if err := rows.Scan(&r.Field1, &r.Field2); err != nil {
                return errMsg{err}
            }
            results = append(results, r)
        }

        if err := rows.Err(); err != nil {
            return errMsg{err}
        }

        return queryResultMsg{results}
    }
}
```

---

## Error Handling

### Graceful Error Display

```go
type model struct {
    err       error
    errExpiry time.Time
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case errMsg:
        m.err = msg.err
        m.errExpiry = time.Now().Add(5 * time.Second)
        return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
            return clearErrMsg{}
        })

    case clearErrMsg:
        if time.Now().After(m.errExpiry) {
            m.err = nil
        }
    }
    return m, nil
}

func (m model) View() string {
    var s strings.Builder

    if m.err != nil {
        errStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Bold(true)
        s.WriteString(errStyle.Render("Error: " + m.err.Error()) + "\n\n")
    }

    // Rest of view...
    return s.String()
}
```

### Recovery from Panics

```go
func safeCommand(fn func() tea.Msg) tea.Cmd {
    return func() tea.Msg {
        defer func() {
            if r := recover(); r != nil {
                // Log panic for debugging
                log.Printf("Panic in command: %v\n%s", r, debug.Stack())
            }
        }()
        return fn()
    }
}

// Usage
return m, safeCommand(func() tea.Msg {
    // Potentially panicking code
    return doRiskyOperation()
})
```

---

## Goroutine Safety

### Safe External Message Sending

```go
type model struct {
    program *tea.Program
}

func main() {
    m := model{}
    p := tea.NewProgram(m)
    m.program = p  // Store reference

    // Background goroutine
    go func() {
        for update := range externalUpdates {
            // Safe to call from any goroutine
            p.Send(externalUpdateMsg{update})
        }
    }()

    p.Run()
}
```

### Avoiding Race Conditions

```go
// BAD - Shared mutable state
type model struct {
    data []string  // Modified by both Update and commands
}

func (m model) someCmd() tea.Cmd {
    return func() tea.Msg {
        m.data = append(m.data, "new")  // RACE CONDITION!
        return nil
    }
}

// GOOD - Commands return new data via messages
type model struct {
    data []string
}

type appendDataMsg struct {
    item string
}

func appendCmd(item string) tea.Cmd {
    return func() tea.Msg {
        // Process item...
        return appendDataMsg{item}
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case appendDataMsg:
        m.data = append(m.data, msg.item)  // Safe - single goroutine
    }
    return m, nil
}
```

### Mutex for Shared Resources

When you must share state (e.g., with SSH sessions):

```go
type sharedState struct {
    mu      sync.RWMutex
    clients map[string]*clientInfo
}

func (s *sharedState) addClient(id string, info *clientInfo) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.clients[id] = info
}

func (s *sharedState) getClient(id string) *clientInfo {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.clients[id]
}

// Commands read from shared state, messages update model
func (m model) refreshCmd() tea.Cmd {
    return func() tea.Msg {
        client := m.shared.getClient(m.clientID)  // Thread-safe read
        return clientInfoMsg{client}
    }
}
```

---

## State Management

### Immutable Updates

```go
// BAD - Mutating slices in place
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case addItemMsg:
        m.items[len(m.items)-1] = msg.item  // May affect other views!
    }
    return m, nil
}

// GOOD - Create new slices
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case addItemMsg:
        // Create new slice
        newItems := make([]Item, len(m.items)+1)
        copy(newItems, m.items)
        newItems[len(newItems)-1] = msg.item
        m.items = newItems
    }
    return m, nil
}
```

### State Validation

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up":
            if m.cursor > 0 {  // Validate bounds
                m.cursor--
            }
        case "down":
            if m.cursor < len(m.items)-1 {  // Validate bounds
                m.cursor++
            }
        }
    }

    // Defensive validation after any update
    m = m.validate()
    return m, nil
}

func (m model) validate() model {
    // Ensure cursor is in bounds
    if m.cursor < 0 {
        m.cursor = 0
    }
    if len(m.items) > 0 && m.cursor >= len(m.items) {
        m.cursor = len(m.items) - 1
    }

    // Ensure page is valid
    if m.page < 0 {
        m.page = 0
    }
    maxPage := (len(m.items) - 1) / m.pageSize
    if m.page > maxPage {
        m.page = maxPage
    }

    return m
}
```

---

## Terminal Recovery

### Handling Subprocess Failure

```go
type editorFinishedMsg struct {
    err error
}

func editFileCmd(filename string) tea.Cmd {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vi"
    }

    c := exec.Command(editor, filename)
    return tea.ExecProcess(c, func(err error) tea.Msg {
        return editorFinishedMsg{err}
    })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case editorFinishedMsg:
        if msg.err != nil {
            // Editor failed, but terminal should be restored
            m.err = fmt.Errorf("editor failed: %w", msg.err)
        } else {
            // Reload file content
            return m, loadFileCmd(m.filename)
        }
    }
    return m, nil
}
```

### Signal Handling

```go
// Default: Bubbletea handles SIGINT, SIGTERM
// Use WithoutSignalHandler for custom handling

func main() {
    p := tea.NewProgram(model{})

    // Custom signal handling
    c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-c
        // Custom cleanup
        cleanup()
        p.Quit()
    }()

    p.Run()
}
```

---

## Testing Strategies

### Unit Testing Models

```go
func TestModel_Update(t *testing.T) {
    m := initialModel()

    // Test key handling
    newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
    m = newModel.(model)

    if m.cursor != 1 {
        t.Errorf("expected cursor=1, got %d", m.cursor)
    }
    if cmd != nil {
        t.Error("expected no command")
    }
}

func TestModel_Update_Bounds(t *testing.T) {
    m := initialModel()
    m.items = []string{"a", "b", "c"}
    m.cursor = 2  // Last item

    // Try to go past end
    newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
    m = newModel.(model)

    if m.cursor != 2 {
        t.Errorf("cursor should stay at 2, got %d", m.cursor)
    }
}

func TestModel_View_NoError(t *testing.T) {
    m := initialModel()

    // View should never panic
    defer func() {
        if r := recover(); r != nil {
            t.Errorf("View panicked: %v", r)
        }
    }()

    view := m.View()
    if view == "" {
        t.Error("view should not be empty")
    }
}
```

### Testing Commands

```go
func TestFetchCmd(t *testing.T) {
    // Mock server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(testData)
    }))
    defer server.Close()

    cmd := fetchDataCmd(server.URL)
    msg := cmd()

    dataMsg, ok := msg.(dataMsg)
    if !ok {
        t.Fatalf("expected dataMsg, got %T", msg)
    }

    if dataMsg.data != testData {
        t.Error("data mismatch")
    }
}

func TestFetchCmd_Error(t *testing.T) {
    cmd := fetchDataCmd("http://invalid-url-that-does-not-exist.local")
    msg := cmd()

    errMsg, ok := msg.(errMsg)
    if !ok {
        t.Fatalf("expected errMsg, got %T", msg)
    }

    if errMsg.err == nil {
        t.Error("expected error")
    }
}
```

---

## Performance Patterns

### Debouncing Input

```go
type model struct {
    input       string
    searchTimer *time.Timer
}

type searchMsg struct {
    query string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.Type == tea.KeyRunes {
            m.input += string(msg.Runes)

            // Debounce: wait 300ms after last keystroke
            return m, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
                return searchMsg{m.input}
            })
        }

    case searchMsg:
        // Only search if input hasn't changed
        if msg.query == m.input {
            return m, performSearchCmd(msg.query)
        }
    }
    return m, nil
}
```

### Lazy Loading

```go
type model struct {
    items       []Item
    loaded      int
    loading     bool
    hasMore     bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "end" && m.hasMore && !m.loading {
            m.loading = true
            return m, loadMoreCmd(m.loaded)
        }

    case itemsLoadedMsg:
        m.items = append(m.items, msg.items...)
        m.loaded += len(msg.items)
        m.hasMore = msg.hasMore
        m.loading = false
    }
    return m, nil
}
```

### Efficient View Rendering

```go
func (m model) View() string {
    var b strings.Builder
    b.Grow(1024)  // Pre-allocate for common case

    // Render only visible items
    start := m.offset
    end := min(m.offset+m.pageSize, len(m.items))

    for i := start; i < end; i++ {
        if i == m.cursor {
            b.WriteString(m.selectedStyle.Render(m.items[i]))
        } else {
            b.WriteString(m.normalStyle.Render(m.items[i]))
        }
        b.WriteString("\n")
    }

    return b.String()
}
```

---

## Common Pitfalls

### Pitfall 1: Forgetting to Return Commands

```go
// BAD - Lost command
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "r" {
            m.loading = true
            refreshCmd()  // Command created but not returned!
        }
    }
    return m, nil
}

// GOOD
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "r" {
            m.loading = true
            return m, refreshCmd()  // Return the command
        }
    }
    return m, nil
}
```

### Pitfall 2: Not Updating Child Components

```go
// BAD - Child never receives messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Only handling our keys, forgetting children
    }
    return m, nil
}

// GOOD - Always update children
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Handle our keys first
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            return m, tea.Quit
        }
    }

    // Always update children
    var cmd tea.Cmd
    m.textInput, cmd = m.textInput.Update(msg)
    cmds = append(cmds, cmd)

    m.spinner, cmd = m.spinner.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}
```

### Pitfall 3: View Depending on Uninitialized State

```go
// BAD - May panic on nil/zero values
func (m model) View() string {
    return m.viewport.View()  // Panics if viewport not initialized
}

// GOOD - Guard against uninitialized state
func (m model) View() string {
    if !m.ready {
        return "Loading..."
    }
    return m.viewport.View()
}
```

### Pitfall 4: Infinite Command Loops

```go
// BAD - Infinite loop
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case dataMsg:
        m.data = msg.data
        return m, fetchDataCmd()  // Immediately fetches again!
    }
    return m, nil
}

// GOOD - Controlled refresh
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case dataMsg:
        m.data = msg.data
        m.lastFetch = time.Now()
        // Don't auto-refresh; let user trigger next fetch
    case tea.KeyMsg:
        if msg.String() == "r" {
            return m, fetchDataCmd()
        }
    }
    return m, nil
}
```

### Pitfall 5: Heavy Work in View()

```go
// BAD - Expensive computation in View
func (m model) View() string {
    sorted := sortItems(m.items)  // Called 60 times per second!
    return renderItems(sorted)
}

// GOOD - Compute in Update, cache result
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case itemsLoadedMsg:
        m.items = msg.items
        m.sortedItems = sortItems(m.items)  // Compute once
    }
    return m, nil
}

func (m model) View() string {
    return renderItems(m.sortedItems)  // Use cached result
}
```

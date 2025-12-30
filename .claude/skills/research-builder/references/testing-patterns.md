# Browser Automation Testing Patterns

Comprehensive testing strategies for BrowserNERD implementation.

## Overview

Testing browser automation requires multiple layers:
1. **Unit Tests** - Isolated component logic
2. **Integration Tests** - Chrome connectivity, event handling
3. **End-to-End Tests** - Full workflows with real browser
4. **Mangle Tests** - Query evaluation and rule logic

## Test Structure

### Directory Organization

```
tests/
├── unit/
│   ├── mangle_test.go          # Fact transformation, query logic
│   ├── session_manager_test.go # Session creation, persistence
│   └── tools_test.go            # MCP tool validation
├── integration/
│   ├── chrome_test.go           # Chrome connection, CDP events
│   ├── react_test.go            # Fiber extraction
│   └── causal_test.go           # RCA rules with scenarios
├── e2e/
│   ├── workflow_test.go         # Complete user workflows
│   └── performance_test.go      # Load testing, benchmarks
└── fixtures/
    ├── test_app/                # Simple React app for testing
    ├── scenarios/               # Test scenarios (HTML files)
    └── expected/                # Expected outputs
```

## Unit Testing

### Testing Mangle Engine

```go
// tests/unit/mangle_test.go
package unit

import (
    "context"
    "testing"
    "time"

    "browsernerd-mcp-server/internal/config"
    "browsernerd-mcp-server/internal/mangle"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEngineLoadSchema(t *testing.T) {
    cfg := config.MangleConfig{
        Enable:          true,
        SchemaPath:      "../fixtures/test_schema.mg",
        FactBufferLimit: 1000,
    }

    engine, err := mangle.NewEngine(cfg)
    require.NoError(t, err)

    // Verify schema loaded
    assert.True(t, engine.IsSchemaLoaded())
}

func TestEngineAddFacts(t *testing.T) {
    engine := newTestEngine(t)

    facts := []mangle.Fact{
        {
            Predicate: "net_request",
            Args: []interface{}{
                "req_1",
                "GET",
                "https://api.example.com",
                "script",
                int64(1700000000),
            },
            Timestamp: time.Now(),
        },
    }

    err := engine.AddFacts(context.Background(), facts)
    require.NoError(t, err)

    // Verify fact stored
    results, err := engine.Query(context.Background(),
        `net_request("req_1", Method, Url, _, _)`)
    require.NoError(t, err)
    assert.Len(t, results, 1)
    assert.Equal(t, "GET", results[0]["Method"])
}

func TestEngineQuery(t *testing.T) {
    engine := newTestEngine(t)

    // Add test data
    addTestFacts(engine,
        netRequest("req_1", "GET", "https://api.example.com"),
        netResponse("req_1", 200, 50, 150),
        netRequest("req_2", "POST", "https://api.example.com"),
        netResponse("req_2", 500, 100, 200),
    )

    // Query: find failed requests
    results, err := engine.Query(context.Background(),
        `net_response(ReqId, Status, _, _), Status >= 400`)
    require.NoError(t, err)

    assert.Len(t, results, 1)
    assert.Equal(t, "req_2", results[0]["ReqId"])
    assert.Equal(t, int64(500), results[0]["Status"])
}

func TestEngineTemporalQuery(t *testing.T) {
    engine := newTestEngine(t)

    // Add facts with timestamps
    now := time.Now()
    facts := []mangle.Fact{
        {
            Predicate: "console_event",
            Args:      []interface{}{"error", "Message 1", now.Add(-10 * time.Second).Unix()},
            Timestamp: now.Add(-10 * time.Second),
        },
        {
            Predicate: "console_event",
            Args:      []interface{}{"error", "Message 2", now.Add(-2 * time.Second).Unix()},
            Timestamp: now.Add(-2 * time.Second),
        },
    }

    engine.AddFacts(context.Background(), facts)

    // Query: events in last 5 seconds
    results, err := engine.QueryTemporal(context.Background(),
        `console_event("error", Msg, T)`, 5*time.Second)
    require.NoError(t, err)

    // Should only return Message 2
    assert.Len(t, results, 1)
    assert.Equal(t, "Message 2", results[0]["Msg"])
}

func TestEngineAddRule(t *testing.T) {
    engine := newTestEngine(t)

    // Add base facts
    addTestFacts(engine,
        netRequest("req_1", "GET", "https://slow-api.com"),
        netResponse("req_1", 200, 50, 2000),
    )

    // Add dynamic rule
    ruleSource := `slow_api(ReqId, Url, Duration) :-
        net_request(ReqId, _, Url, _, _),
        net_response(ReqId, _, _, Duration),
        Duration > 1000.`

    err := engine.AddRule(ruleSource)
    require.NoError(t, err)

    // Query derived facts
    results, err := engine.Query(context.Background(), `slow_api(ReqId, Url, Duration)`)
    require.NoError(t, err)

    assert.Len(t, results, 1)
    assert.Equal(t, "req_1", results[0]["ReqId"])
    assert.Equal(t, int64(2000), results[0]["Duration"])
}

// Test helpers
func newTestEngine(t *testing.T) *mangle.Engine {
    t.Helper()

    cfg := config.MangleConfig{
        Enable:          true,
        SchemaPath:      "../../schemas/browser.mg",
        FactBufferLimit: 1000,
    }

    engine, err := mangle.NewEngine(cfg)
    require.NoError(t, err)

    return engine
}

func netRequest(id, method, url string) mangle.Fact {
    return mangle.Fact{
        Predicate: "net_request",
        Args:      []interface{}{id, method, url, "script", time.Now().Unix()},
        Timestamp: time.Now(),
    }
}

func netResponse(id string, status int, latency, duration int64) mangle.Fact {
    return mangle.Fact{
        Predicate: "net_response",
        Args:      []interface{}{id, int64(status), latency, duration},
        Timestamp: time.Now(),
    }
}

func addTestFacts(engine *mangle.Engine, facts ...mangle.Fact) {
    engine.AddFacts(context.Background(), facts)
}
```

### Testing Session Manager

```go
// tests/unit/session_manager_test.go
package unit

import (
    "context"
    "testing"

    "browsernerd-mcp-server/internal/browser"
    "browsernerd-mcp-server/internal/config"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSessionManagerStart(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping Chrome connection test in short mode")
    }

    cfg := config.BrowserConfig{
        DebuggerURL: "ws://localhost:9222",
    }

    mockEngine := &mockEngineSink{}
    manager := browser.NewSessionManager(cfg, mockEngine)

    err := manager.Start(context.Background())
    require.NoError(t, err)

    // Verify no sessions initially
    sessions := manager.List()
    assert.Empty(t, sessions)
}

func TestCreateSession(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping Chrome interaction test in short mode")
    }

    manager := newTestSessionManager(t)

    session, err := manager.CreateSession(context.Background(), "https://example.com")
    require.NoError(t, err)

    assert.NotEmpty(t, session.ID)
    assert.Equal(t, "https://example.com", session.URL)
    assert.Equal(t, "active", session.Status)

    // Verify session in list
    sessions := manager.List()
    assert.Len(t, sessions, 1)
    assert.Equal(t, session.ID, sessions[0].ID)
}

func TestSessionPersistence(t *testing.T) {
    // Create session
    manager1 := newTestSessionManager(t)
    session, err := manager1.CreateSession(context.Background(), "https://example.com")
    require.NoError(t, err)

    // Restart manager (should reload sessions)
    manager2 := newTestSessionManager(t)
    err = manager2.Start(context.Background())
    require.NoError(t, err)

    // Verify session restored
    sessions := manager2.List()
    assert.Len(t, sessions, 1)
    assert.Equal(t, session.ID, sessions[0].ID)
}

// Mock engine for testing
type mockEngineSink struct {
    facts []mangle.Fact
}

func (m *mockEngineSink) AddFacts(ctx context.Context, facts []mangle.Fact) error {
    m.facts = append(m.facts, facts...)
    return nil
}

func newTestSessionManager(t *testing.T) *browser.SessionManager {
    t.Helper()

    cfg := config.BrowserConfig{
        DebuggerURL: "ws://localhost:9222",
    }

    mockEngine := &mockEngineSink{}
    manager := browser.NewSessionManager(cfg, mockEngine)

    err := manager.Start(context.Background())
    require.NoError(t, err)

    return manager
}
```

## Integration Testing

### Testing Chrome Connection

```go
// tests/integration/chrome_test.go
package integration

import (
    "context"
    "testing"
    "time"

    "browsernerd-mcp-server/internal/browser"
    "browsernerd-mcp-server/internal/config"
    "browsernerd-mcp-server/tests/testutil"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestChromeConnectionAttach(t *testing.T) {
    // Requires Chrome running with --remote-debugging-port=9222
    cfg := config.BrowserConfig{
        DebuggerURL: "ws://localhost:9222",
    }

    mockEngine := &testutil.MockEngine{}
    manager := browser.NewSessionManager(cfg, mockEngine)

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    err := manager.Start(ctx)
    require.NoError(t, err, "Failed to connect to Chrome. Ensure Chrome is running with debugging enabled.")
}

func TestChromeConnectionLaunch(t *testing.T) {
    cfg := config.BrowserConfig{
        Launch: []string{
            "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
            "--headless",
            "--no-sandbox",
        },
    }

    mockEngine := &testutil.MockEngine{}
    manager := browser.NewSessionManager(cfg, mockEngine)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    err := manager.Start(ctx)
    require.NoError(t, err, "Failed to launch Chrome programmatically")
}
```

### Testing CDP Event Ingestion

```go
// tests/integration/events_test.go
package integration

import (
    "context"
    "testing"
    "time"

    "browsernerd-mcp-server/tests/testutil"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNetworkEventIngestion(t *testing.T) {
    manager := testutil.NewTestManager(t)
    mockEngine := manager.Engine.(*testutil.MockEngine)

    // Create session
    session, err := manager.CreateSession(context.Background(), "https://httpbin.org/get")
    require.NoError(t, err)

    // Wait for events to be captured
    time.Sleep(2 * time.Second)

    // Verify network facts captured
    facts := mockEngine.GetFacts()

    // Should have net_request facts
    requestFacts := filterFacts(facts, "net_request")
    assert.NotEmpty(t, requestFacts, "No network request facts captured")

    // Should have net_response facts
    responseFacts := filterFacts(facts, "net_response")
    assert.NotEmpty(t, responseFacts, "No network response facts captured")

    // Verify request details
    reqFact := requestFacts[0]
    assert.Equal(t, "GET", reqFact.Args[1])
    assert.Contains(t, reqFact.Args[2], "httpbin.org")
}

func TestConsoleEventIngestion(t *testing.T) {
    manager := testutil.NewTestManager(t)
    mockEngine := manager.Engine.(*testutil.MockEngine)

    // Navigate to page with console logs
    session, err := manager.CreateSession(context.Background(), "about:blank")
    require.NoError(t, err)

    // Execute console.log
    page := manager.GetPage(session.ID)
    _, err = page.Evaluate(`
        console.log("Test message");
        console.error("Test error");
        console.warn("Test warning");
    `)
    require.NoError(t, err)

    // Wait for events
    time.Sleep(500 * time.Millisecond)

    // Verify console facts captured
    facts := mockEngine.GetFacts()
    consoleFacts := filterFacts(facts, "console_event")

    assert.Len(t, consoleFacts, 3)

    // Check levels
    levels := make(map[string]bool)
    for _, fact := range consoleFacts {
        levels[fact.Args[0].(string)] = true
    }

    assert.True(t, levels["log"])
    assert.True(t, levels["error"])
    assert.True(t, levels["warning"])
}

func TestNavigationEventIngestion(t *testing.T) {
    manager := testutil.NewTestManager(t)
    mockEngine := manager.Engine.(*testutil.MockEngine)

    // Create session and navigate
    session, err := manager.CreateSession(context.Background(), "https://example.com")
    require.NoError(t, err)

    // Navigate to another page
    page := manager.GetPage(session.ID)
    err = page.Navigate("https://example.org")
    require.NoError(t, err)

    time.Sleep(1 * time.Second)

    // Verify navigation facts
    facts := mockEngine.GetFacts()
    navFacts := filterFacts(facts, "navigation_event")

    assert.NotEmpty(t, navFacts)

    // Should contain navigation from example.com to example.org
    found := false
    for _, fact := range navFacts {
        if fact.Args[1] == "https://example.org" {
            found = true
            break
        }
    }
    assert.True(t, found, "Navigation to example.org not captured")
}

func filterFacts(facts []mangle.Fact, predicate string) []mangle.Fact {
    var filtered []mangle.Fact
    for _, fact := range facts {
        if fact.Predicate == predicate {
            filtered = append(filtered, fact)
        }
    }
    return filtered
}
```

### Testing React Fiber Extraction

```go
// tests/integration/react_test.go
package integration

import (
    "context"
    "testing"

    "browsernerd-mcp-server/tests/testutil"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestReactFiberExtraction(t *testing.T) {
    manager := testutil.NewTestManager(t)
    mockEngine := manager.Engine.(*testutil.MockEngine)

    // Navigate to test React app
    session, err := manager.CreateSession(context.Background(),
        "file://"+testutil.TestAppPath("fixtures/test_app/index.html"))
    require.NoError(t, err)

    // Run reify-react
    err = manager.ReifyReact(context.Background(), session.ID)
    require.NoError(t, err)

    // Verify React facts captured
    facts := mockEngine.GetFacts()

    // Should have react_component facts
    componentFacts := filterFacts(facts, "react_component")
    assert.NotEmpty(t, componentFacts, "No React component facts extracted")

    // Should have react_prop facts
    propFacts := filterFacts(facts, "react_prop")
    assert.NotEmpty(t, propFacts, "No React prop facts extracted")

    // Verify component structure
    // Test app has: App -> Header, Counter, Footer
    componentNames := make(map[string]bool)
    for _, fact := range componentFacts {
        componentNames[fact.Args[1].(string)] = true
    }

    assert.True(t, componentNames["App"])
    assert.True(t, componentNames["Header"])
    assert.True(t, componentNames["Counter"])
    assert.True(t, componentNames["Footer"])
}

func TestReactFiberNonReactPage(t *testing.T) {
    manager := testutil.NewTestManager(t)

    // Navigate to non-React page
    session, err := manager.CreateSession(context.Background(), "https://example.com")
    require.NoError(t, err)

    // Run reify-react (should handle gracefully)
    err = manager.ReifyReact(context.Background(), session.ID)

    // Should not error, just return empty results
    assert.NoError(t, err)
}
```

### Testing Causal Reasoning Rules

```go
// tests/integration/causal_test.go
package integration

import (
    "context"
    "testing"
    "time"

    "browsernerd-mcp-server/internal/mangle"
    "browsernerd-mcp-server/tests/testutil"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCausedByRule(t *testing.T) {
    engine := testutil.NewTestEngine(t)

    // Scenario: Failed HTTP request followed by console error
    now := time.Now()

    facts := []mangle.Fact{
        // Request fails
        {
            Predicate: "net_request",
            Args:      []interface{}{"req_1", "GET", "https://api.example.com", "script", now.Unix()},
            Timestamp: now,
        },
        {
            Predicate: "net_response",
            Args:      []interface{}{"req_1", int64(500), int64(50), int64(100)},
            Timestamp: now,
        },
        // Console error 50ms later
        {
            Predicate: "console_event",
            Args:      []interface{}{"error", "Network request failed", now.Add(50 * time.Millisecond).Unix()},
            Timestamp: now.Add(50 * time.Millisecond),
        },
    }

    err := engine.AddFacts(context.Background(), facts)
    require.NoError(t, err)

    // Query caused_by rule
    results, err := engine.Query(context.Background(), `caused_by(ErrorMsg, ReqId)`)
    require.NoError(t, err)

    // Should detect causality
    assert.Len(t, results, 1)
    assert.Equal(t, "Network request failed", results[0]["ErrorMsg"])
    assert.Equal(t, "req_1", results[0]["ReqId"])
}

func TestSlowAPIRule(t *testing.T) {
    engine := testutil.NewTestEngine(t)

    facts := []mangle.Fact{
        {
            Predicate: "net_request",
            Args:      []interface{}{"req_1", "GET", "https://slow-api.com", "script", time.Now().Unix()},
            Timestamp: time.Now(),
        },
        {
            Predicate: "net_response",
            Args:      []interface{}{"req_1", int64(200), int64(100), int64(2500)}, // 2.5 seconds
            Timestamp: time.Now(),
        },
    }

    err := engine.AddFacts(context.Background(), facts)
    require.NoError(t, err)

    // Query slow_api rule
    results, err := engine.Query(context.Background(), `slow_api(ReqId, Url, Duration)`)
    require.NoError(t, err)

    assert.Len(t, results, 1)
    assert.Equal(t, "req_1", results[0]["ReqId"])
    assert.Equal(t, int64(2500), results[0]["Duration"])
}

func TestCascadingFailureRule(t *testing.T) {
    engine := testutil.NewTestEngine(t)

    facts := []mangle.Fact{
        // Parent request fails
        {
            Predicate: "net_request",
            Args:      []interface{}{"parent", "GET", "https://api.example.com/auth", "script", time.Now().Unix()},
            Timestamp: time.Now(),
        },
        {
            Predicate: "net_response",
            Args:      []interface{}{"parent", int64(401), int64(50), int64(100)},
            Timestamp: time.Now(),
        },
        // Child request (initiated by parent) also fails
        {
            Predicate: "net_request",
            Args:      []interface{}{"child", "GET", "https://api.example.com/data", "parent", time.Now().Unix()},
            Timestamp: time.Now(),
        },
        {
            Predicate: "net_response",
            Args:      []interface{}{"child", int64(403), int64(50), int64(100)},
            Timestamp: time.Now(),
        },
    }

    err := engine.AddFacts(context.Background(), facts)
    require.NoError(t, err)

    // Query cascading_failure rule
    results, err := engine.Query(context.Background(), `cascading_failure(ChildId, ParentId)`)
    require.NoError(t, err)

    assert.Len(t, results, 1)
    assert.Equal(t, "child", results[0]["ChildId"])
    assert.Equal(t, "parent", results[0]["ParentId"])
}
```

## End-to-End Testing

### Full Workflow Test

```go
// tests/e2e/workflow_test.go
package e2e

import (
    "context"
    "testing"

    "browsernerd-mcp-server/tests/testutil"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCompleteWorkflow(t *testing.T) {
    // 1. Start server
    server := testutil.NewTestServer(t)
    defer server.Shutdown()

    // 2. Create session via MCP tool
    result, err := server.CallTool("create-session", map[string]interface{}{
        "url": "https://httpbin.org/html",
    })
    require.NoError(t, err)

    sessionID := result["session"].(map[string]interface{})["id"].(string)

    // 3. Wait for page load and events
    time.Sleep(2 * time.Second)

    // 4. Query facts via MCP tool
    result, err = server.CallTool("query-facts", map[string]interface{}{
        "query": `net_request(ReqId, "GET", Url, _, _)`,
    })
    require.NoError(t, err)

    results := result["results"].([]interface{})
    assert.NotEmpty(t, results)

    // 5. Fork session for testing
    result, err = server.CallTool("fork-session", map[string]interface{}{
        "session_id": sessionID,
    })
    require.NoError(t, err)

    forkedSessionID := result["session"].(map[string]interface{})["id"].(string)
    assert.NotEqual(t, sessionID, forkedSessionID)

    // 6. Verify events in both sessions
    result, err = server.CallTool("list-sessions", nil)
    require.NoError(t, err)

    sessions := result["sessions"].([]interface{})
    assert.Len(t, sessions, 2)
}
```

## Test Utilities

### Test Fixtures

```go
// tests/testutil/fixtures.go
package testutil

import (
    "os"
    "path/filepath"
)

func TestAppPath(relative string) string {
    cwd, _ := os.Getwd()
    return filepath.Join(cwd, "..", relative)
}

// Simple React test app
const TestAppHTML = `
<!DOCTYPE html>
<html>
<head>
    <script crossorigin src="https://unpkg.com/react@18/umd/react.development.js"></script>
    <script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.development.js"></script>
</head>
<body>
    <div id="root"></div>
    <script>
        const { useState } = React;

        function Counter() {
            const [count, setCount] = useState(0);
            return React.createElement('div', null,
                React.createElement('p', null, 'Count: ' + count),
                React.createElement('button', { onClick: () => setCount(count + 1) }, 'Increment')
            );
        }

        function App() {
            return React.createElement('div', null,
                React.createElement('h1', null, 'Test App'),
                React.createElement(Counter)
            );
        }

        ReactDOM.render(React.createElement(App), document.getElementById('root'));
    </script>
</body>
</html>
`
```

### Mock Engine

```go
// tests/testutil/mock_engine.go
package testutil

import (
    "context"
    "sync"

    "browsernerd-mcp-server/internal/mangle"
)

type MockEngine struct {
    mu    sync.RWMutex
    facts []mangle.Fact
}

func (m *MockEngine) AddFacts(ctx context.Context, facts []mangle.Fact) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.facts = append(m.facts, facts...)
    return nil
}

func (m *MockEngine) GetFacts() []mangle.Fact {
    m.mu.RLock()
    defer m.mu.RUnlock()

    return append([]mangle.Fact{}, m.facts...)
}

func (m *MockEngine) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.facts = nil
}
```

## Running Tests

### Unit Tests Only

```bash
go test ./tests/unit/... -v
```

### Integration Tests (Requires Chrome)

```bash
# Start Chrome with debugging
start chrome --remote-debugging-port=9222 --user-data-dir=C:\temp\chrome-debug

# Run tests
go test ./tests/integration/... -v
```

### All Tests

```bash
go test ./... -v
```

### With Coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Benchmarks

```bash
go test ./tests/integration/... -bench=. -benchmem
```

## Continuous Integration

### GitHub Actions Example

```yaml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Install Chrome
        run: |
          wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add -
          sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list'
          apt-get update
          apt-get install -y google-chrome-stable

      - name: Run unit tests
        run: go test ./tests/unit/... -v

      - name: Start Chrome
        run: google-chrome --remote-debugging-port=9222 --headless --no-sandbox &

      - name: Run integration tests
        run: go test ./tests/integration/... -v

      - name: Generate coverage
        run: go test ./... -coverprofile=coverage.out

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

# Execution Wiring Patterns

This document describes patterns for detecting "code exists but doesn't execute" issues - where components are created but never actually run.

## The Problem

The most insidious bugs in codeNERD are **execution wiring gaps**:

```go
// This code COMPILES but DOESN'T WORK
orch := campaign.NewOrchestrator(config)
orch.SetCampaign(campaign)
return campaignStartedMsg(campaign)  // orch.Run() NEVER CALLED!
```

The orchestrator exists, is configured, but never runs. No compile error. No runtime error. Just silent failure.

---

## Pattern Categories

### 1. Object Creation Without Execution

**Symptom:** Object created with `New*()` but key methods never called.

**Detection:**
```text
MATCH: variable := package.New*()
MISSING: variable.Run() OR variable.Start() OR variable.Execute()
```

**Common Offenders:**
| Object Type | Creation | Required Call |
|-------------|----------|---------------|
| Orchestrator | `NewOrchestrator()` | `.Run(ctx)` |
| Server | `NewServer()` | `.Start()` or `.ListenAndServe()` |
| Watcher | `NewWatcher()` | `.Start()` or `.Watch()` |
| Ticker | `time.NewTicker()` | Read from `.C` channel |
| Timer | `time.NewTimer()` | Read from `.C` channel |

**Fix Pattern:**
```go
// WRONG
orch := campaign.NewOrchestrator(config)
orch.SetCampaign(campaign)
return result  // Orphaned orchestrator

// RIGHT
orch := campaign.NewOrchestrator(config)
orch.SetCampaign(campaign)
go orch.Run(ctx)  // Actually execute
return result
```

---

### 2. Local Variable Escaping Scope

**Symptom:** Object created as local variable, should be stored in struct field.

**Detection:**
```text
MATCH: localVar := New*() inside method
MISSING: m.fieldName = localVar OR return localVar
```

**Example:**
```go
// WRONG - orch goes out of scope, gets GC'd
func (m Model) startCampaign(goal string) tea.Cmd {
    orch := campaign.NewOrchestrator(config)  // LOCAL
    orch.SetCampaign(campaign)
    return campaignStartedMsg(campaign)
    // orch is garbage collected here!
}

// RIGHT - store reference for later use
func (m *Model) startCampaign(goal string) tea.Cmd {
    m.campaignOrch = campaign.NewOrchestrator(config)  // STORED
    m.campaignOrch.SetCampaign(campaign)
    return campaignStartedMsg(campaign)
}
```

---

### 3. Channel Creation Without Listeners

**Symptom:** Channel created but nothing reads from it.

**Detection:**
```text
MATCH: ch := make(chan X) OR ch := make(chan X, N)
MISSING: <-ch OR select { case x := <-ch: }
```

**Example:**
```go
// WRONG - channel created but never read
type OrchestratorConfig struct {
    ProgressChan chan Progress
    EventChan    chan OrchestratorEvent
}

orch := NewOrchestrator(OrchestratorConfig{
    ProgressChan: make(chan Progress, 10),  // Created
    EventChan:    make(chan OrchestratorEvent, 10),  // Created
})
// But nobody reads from these channels!

// RIGHT - spawn listener goroutines
progressChan := make(chan Progress, 10)
eventChan := make(chan OrchestratorEvent, 10)

go func() {
    for progress := range progressChan {
        m.updateProgress(progress)
    }
}()

go func() {
    for event := range eventChan {
        m.handleEvent(event)
    }
}()

orch := NewOrchestrator(OrchestratorConfig{
    ProgressChan: progressChan,
    EventChan:    eventChan,
})
```

---

### 4. Bubbletea Message Type Without Handler

**Symptom:** Message type defined but no case in Update().

**Detection:**
```text
MATCH: type fooMsg struct OR type fooMsg = X
MISSING: case fooMsg: in Update()
```

**Example:**
```go
// WRONG - message defined but not handled
type campaignProgressMsg struct {
    Progress campaign.Progress
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // handled
    case chatResponseMsg:
        // handled
    // campaignProgressMsg NOT handled - updates are lost!
    }
}

// RIGHT - add handler
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case campaignProgressMsg:
        m.campaignProgress = msg.Progress
        return m, nil
    // ... other cases
    }
}
```

---

### 5. Background Goroutine Not Spawned

**Symptom:** Code designed for background execution but `go` keyword missing.

**Detection:**
```text
MATCH: comment mentions "background" OR "async" OR "concurrent"
NEARBY: function call WITHOUT `go` prefix
```

**Example:**
```go
// WRONG - blocks the caller
func (m Model) startCampaign() tea.Cmd {
    return func() tea.Msg {
        // This runs synchronously and blocks TUI!
        err := m.campaignOrch.Run(context.Background())
        return campaignDoneMsg{err: err}
    }
}

// RIGHT - run in background, send progress
func (m Model) startCampaign() tea.Cmd {
    return func() tea.Msg {
        go func() {
            // Runs in background
            err := m.campaignOrch.Run(context.Background())
            // Progress sent via channel
        }()
        return campaignStartedMsg{}  // Return immediately
    }
}
```

---

### 6. Struct Field Never Assigned

**Symptom:** Struct field defined and checked, but never assigned.

**Detection:**
```text
MATCH: m.fieldName != nil (check) OR m.fieldName == nil (check)
MISSING: m.fieldName = value (assignment)
```

**Example:**
```go
type Model struct {
    campaignOrch *campaign.Orchestrator  // Field defined
}

// WRONG - field checked but never assigned
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case pauseCampaignMsg:
        if m.campaignOrch != nil {  // Checked!
            m.campaignOrch.Pause()
        }
        // But m.campaignOrch is NEVER assigned anywhere!
    }
}

// Search for: m.campaignOrch =
// Result: Only found "m.campaignOrch = nil" (clearing)
// Missing: m.campaignOrch = campaign.NewOrchestrator(...)
```

---

### 7. Return Value Ignored

**Symptom:** Function returns value that should be used, but caller ignores it.

**Detection:**
```text
MATCH: functionThatReturns() without assignment
WHERE: function signature shows non-error return value
```

**Example:**
```go
// WRONG - ignoring the orchestrator
campaign.NewOrchestrator(config)  // Returns *Orchestrator, ignored!

// RIGHT - capture and use
orch := campaign.NewOrchestrator(config)
m.campaignOrch = orch
```

---

### 8. Interface Method Not Called

**Symptom:** Interface satisfied but key methods never invoked.

**Detection:**
```text
MATCH: type implements interface (has all methods)
MISSING: calls to interface methods on instances of type
```

**Example:**
```go
// Orchestrator implements Runner interface
type Runner interface {
    Run(ctx context.Context) error
}

type Orchestrator struct { /* ... */ }
func (o *Orchestrator) Run(ctx context.Context) error { /* ... */ }

// WRONG - Orchestrator created, implements Runner, but Run() never called
orch := NewOrchestrator(config)
// orch.Run() should be called somewhere!
```

---

## Audit Checklist

For every new feature, verify:

| Check | Question | How to Verify |
|-------|----------|---------------|
| **Object Execution** | Is `New*()` followed by `Run()`/`Start()`? | Search: `grep -A 10 "New${Component}"` |
| **Reference Storage** | Are local objects stored if needed long-term? | Check: `m.field = localVar` |
| **Channel Listeners** | Are created channels read from? | Search: `<-channelName` |
| **Message Handlers** | Do all `*Msg` types have `case` handlers? | Check Update() switch |
| **Goroutine Spawn** | Are long-running ops wrapped in `go func()`? | Check for `go` keyword |
| **Field Assignment** | Are checked fields assigned somewhere? | Search: `m.field =` |
| **Return Usage** | Are non-error returns captured? | Check assignment to variable |

---

## Detection Script

The `audit_execution.py` script automates these checks:

```bash
# Full execution wiring audit
python audit_execution.py

# Focus on specific component
python audit_execution.py --component campaign

# Verbose with suggestions
python audit_execution.py --verbose
```

### What It Detects

1. **Orphaned Objects** - `New*()` without `Run()`/`Start()`
2. **Lost References** - Local vars that should be struct fields
3. **Dead Channels** - Created but never read
4. **Unhandled Messages** - Bubbletea `*Msg` without case handlers
5. **Missing Goroutines** - Blocking calls that should be async
6. **Unassigned Fields** - Fields checked but never set

---

## Real-World Examples

### Example 1: Campaign Orchestrator (The Bug We Found)

**File:** `cmd/nerd/chat/campaign.go:73-94`

**Problem:**
```go
func (m Model) startCampaign(goal string) tea.Cmd {
    orch := campaign.NewOrchestrator(config)  // Created
    orch.SetCampaign(result.Campaign)          // Configured
    return campaignStartedMsg(result.Campaign) // Returns... but Run() never called!
}
```

**Issues:**
1. `orch` is local variable (lost when function returns)
2. `orch.Run(ctx)` is never called
3. `m.campaignOrch` is never assigned
4. ProgressChan/EventChan are not created or listened to

**Fix:**
```go
func (m *Model) startCampaign(goal string) tea.Cmd {
    progressChan := make(chan campaign.Progress, 10)
    eventChan := make(chan campaign.OrchestratorEvent, 10)

    orch := campaign.NewOrchestrator(campaign.OrchestratorConfig{
        // ... config ...
        ProgressChan: progressChan,
        EventChan:    eventChan,
    })
    orch.SetCampaign(result.Campaign)

    // Store reference
    m.campaignOrch = orch

    // Spawn execution goroutine
    go func() {
        if err := orch.Run(context.Background()); err != nil {
            // Handle error via channel
        }
    }()

    // Spawn progress listener
    go func() {
        for progress := range progressChan {
            // Send to TUI via tea.Cmd
        }
    }()

    return campaignStartedMsg(result.Campaign)
}
```

### Example 2: Ticker Without Reader

**Problem:**
```go
ticker := time.NewTicker(5 * time.Second)
// ticker.C never read - tick events are lost
```

**Fix:**
```go
ticker := time.NewTicker(5 * time.Second)
defer ticker.Stop()

go func() {
    for range ticker.C {
        m.checkProgress()
    }
}()
```

---

## Integration with Other Audits

Execution wiring is **Phase 6** of the full audit:

| Phase | Audit | Focus |
|-------|-------|-------|
| 1 | Shard Registration | Factory, profiles, injection |
| 2 | Mangle Schema | Decl, rules, virtual predicates |
| 3 | Action Layer | CLI, transducer, handlers |
| 4 | Logging | Category coverage |
| 5 | Cross-System | Boot sequence, dependencies |
| **6** | **Execution Wiring** | **Objects run, channels listened, goroutines spawned** |

Run all phases:
```bash
python audit_wiring.py --verbose
```

---

## Prevention

### Code Review Checklist

Before approving any PR:

- [ ] Every `New*()` has corresponding `Run()`/`Start()`
- [ ] Long-lived objects stored in struct fields
- [ ] Channels have goroutines reading them
- [ ] Bubbletea `*Msg` types have case handlers
- [ ] Blocking operations wrapped in `go func()`
- [ ] Struct fields that are checked are also assigned

### IDE Patterns to Watch

Configure your IDE to highlight:
- `New*` function calls without assignment
- Channels created without `<-` nearby
- Struct fields that appear only in nil checks

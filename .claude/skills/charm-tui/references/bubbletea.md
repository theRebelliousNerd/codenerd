# Bubbletea Framework Reference

Complete reference for building terminal user interfaces with Bubbletea.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Model Interface](#model-interface)
- [Messages](#messages)
- [Commands](#commands)
- [Program Configuration](#program-configuration)
- [Keyboard Input](#keyboard-input)
- [Mouse Input](#mouse-input)
- [Window Management](#window-management)
- [External Process Execution](#external-process-execution)
- [Program Control](#program-control)
- [Debugging](#debugging)

---

## Architecture Overview

Bubbletea implements the **Model-View-Update (MVU)** pattern from Elm:

```
┌─────────────────────────────────────────────────────────┐
│                    Bubbletea Runtime                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐          │
│   │  Init   │────▶│  Model  │────▶│  View   │──┐       │
│   └─────────┘     └────┬────┘     └─────────┘  │       │
│                        │                        │       │
│                        ▼                        │       │
│   ┌─────────┐     ┌─────────┐                  │       │
│   │   Cmd   │◀────│ Update  │◀─────────────────┘       │
│   └────┬────┘     └─────────┘                          │
│        │               ▲                               │
│        │               │                               │
│        ▼               │                               │
│   ┌─────────┐     ┌─────────┐                          │
│   │   I/O   │────▶│   Msg   │                          │
│   └─────────┘     └─────────┘                          │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Flow

1. **Init()** - Called once at startup, returns initial command
2. **Update(msg)** - Receives messages, updates model, returns commands
3. **View()** - Renders model to string for display
4. **Commands** - Perform I/O asynchronously, return messages
5. **Messages** - Events that trigger Update()

---

## Model Interface

Every Bubbletea application must implement the `tea.Model` interface:

```go
type Model interface {
    Init() Cmd
    Update(Msg) (Model, Cmd)
    View() string
}
```

### Complete Example

```go
package main

import (
    "fmt"
    tea "github.com/charmbracelet/bubbletea"
)

// State
type model struct {
    count    int
    quitting bool
}

// Init returns an initial command
func (m model) Init() tea.Cmd {
    return nil // No initial command
}

// Update handles messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            m.quitting = true
            return m, tea.Quit
        case "up", "k":
            m.count++
        case "down", "j":
            m.count--
        }
    }
    return m, nil
}

// View renders the UI
func (m model) View() string {
    if m.quitting {
        return "Goodbye!\n"
    }
    return fmt.Sprintf("Count: %d\n\nPress up/down to change, q to quit.\n", m.count)
}

func main() {
    p := tea.NewProgram(model{})
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

---

## Messages

Messages are events that flow through the application.

### Built-in Message Types

```go
// Keyboard input
type KeyMsg Key

// Mouse events (when enabled)
type MouseMsg Mouse

// Terminal window resize
type WindowSizeMsg struct {
    Width  int
    Height int
}

// Focus gained/lost (when WithReportFocus enabled)
type FocusMsg struct{}
type BlurMsg struct{}

// Bracketed paste (when enabled)
type PasteMsg string
type PasteStartMsg struct{}
type PasteEndMsg struct{}

// Program quit
type QuitMsg struct{}
```

### Custom Messages

```go
// Define custom message types
type tickMsg time.Time
type statusMsg int
type errMsg struct{ err error }

type dataLoadedMsg struct {
    items []Item
    count int
}

// Type aliases work too
type responseMsg string
```

### Message Handling Pattern

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    // Keyboard
    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            return m, tea.Quit
        }

    // Mouse (if enabled)
    case tea.MouseMsg:
        if msg.Action == tea.MouseActionPress {
            m.clicked = true
        }

    // Window resize
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

    // Custom messages
    case tickMsg:
        m.currentTime = time.Time(msg)
        return m, tickCmd()

    case dataLoadedMsg:
        m.data = msg.items
        m.loading = false

    case errMsg:
        m.err = msg.err
        m.loading = false
    }

    return m, nil
}
```

---

## Commands

Commands are functions that perform I/O and return messages.

### Command Signature

```go
type Cmd func() Msg
```

### Built-in Commands

```go
// Quit the program
tea.Quit

// Clear the screen
tea.ClearScreen

// Enter alternate screen buffer (full-screen)
tea.EnterAltScreen

// Exit alternate screen buffer
tea.ExitAltScreen

// Show/hide cursor
tea.ShowCursor
tea.HideCursor

// Enable/disable mouse
tea.EnableMouseAllMotion
tea.EnableMouseCellMotion
tea.DisableMouse

// Enable/disable bracketed paste
tea.EnableBracketedPaste
tea.DisableBracketedPaste

// Query window size
tea.WindowSize()

// Print above program (inline mode only)
tea.Printf(format, args...)
tea.Println(args...)
```

### Timer Commands

```go
// One-shot tick (starts immediately)
tea.Tick(duration, func(t time.Time) tea.Msg {
    return tickMsg(t)
})

// Clock-synced tick (aligns to wall clock)
tea.Every(duration, func(t time.Time) tea.Msg {
    return tickMsg(t)
})

// Example: Repeating timer
type tickMsg time.Time

func tickCmd() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

func (m model) Init() tea.Cmd {
    return tickCmd() // Start timer
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg:
        m.elapsed++
        return m, tickCmd() // Continue timer
    }
    return m, nil
}
```

### Batching Commands

```go
// Run concurrently (no order guarantee)
tea.Batch(cmd1, cmd2, cmd3)

// Run sequentially (in order)
tea.Sequence(cmd1, cmd2, cmd3)

// Example: Init with multiple commands
func (m model) Init() tea.Cmd {
    return tea.Batch(
        textinput.Blink,     // Start cursor blink
        m.spinner.Tick,      // Start spinner
        loadDataCmd,         // Fetch initial data
    )
}

// Example: Component updates
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)
    cmds = append(cmds, cmd)

    m.spinner, cmd = m.spinner.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}
```

### Custom Commands

```go
// Simple command
func doSomethingCmd() tea.Msg {
    // Perform I/O (this runs in a goroutine)
    result := doSomething()
    return resultMsg{result}
}

// Command with parameters (use closure)
func fetchDataCmd(url string) tea.Cmd {
    return func() tea.Msg {
        resp, err := http.Get(url)
        if err != nil {
            return errMsg{err}
        }
        defer resp.Body.Close()

        var data Data
        if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
            return errMsg{err}
        }
        return dataMsg{data}
    }
}

// Usage
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "f" {
            return m, fetchDataCmd("https://api.example.com/data")
        }
    case dataMsg:
        m.data = msg.data
    case errMsg:
        m.err = msg.err
    }
    return m, nil
}
```

---

## Program Configuration

### NewProgram Options

```go
p := tea.NewProgram(
    model{},

    // Screen modes
    tea.WithAltScreen(),              // Full-screen mode

    // Mouse support
    tea.WithMouseAllMotion(),         // All mouse events including hover
    tea.WithMouseCellMotion(),        // Click, drag, wheel only

    // Rendering
    tea.WithFPS(120),                 // Custom frame rate (default 60)

    // Input/Output
    tea.WithInput(reader),            // Custom input source
    tea.WithOutput(writer),           // Custom output destination
    tea.WithInputTTY(),               // Use TTY for input

    // Behavior
    tea.WithContext(ctx),             // Cancellation context
    tea.WithFilter(filterFunc),       // Message filtering
    tea.WithoutSignalHandler(),       // Disable signal handling
    tea.WithoutCatchPanics(),         // Don't catch panics (debug)
    tea.WithoutBracketedPaste(),      // Disable bracketed paste
    tea.WithReportFocus(),            // Report focus changes

    // Environment
    tea.WithEnvironment(envVars),     // Custom environment (SSH)
)
```

### Running the Program

```go
// Basic run
if _, err := p.Run(); err != nil {
    log.Fatal(err)
}

// Get final model
finalModel, err := p.Run()
if err != nil {
    log.Fatal(err)
}
result := finalModel.(model)

// With context cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

p := tea.NewProgram(model{}, tea.WithContext(ctx))
if _, err := p.Run(); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("Timed out")
    }
}
```

---

## Keyboard Input

### KeyMsg Structure

```go
type Key struct {
    Type  KeyType
    Runes []rune   // For KeyRunes type
    Alt   bool     // Alt modifier
    Paste bool     // From paste operation
}

type KeyMsg Key
```

### Key Types

```go
const (
    KeyRunes KeyType = iota  // Regular characters
    KeyEnter
    KeyTab
    KeyBackspace
    KeyEscape
    KeySpace

    // Arrow keys
    KeyUp
    KeyDown
    KeyLeft
    KeyRight

    // Navigation
    KeyHome
    KeyEnd
    KeyPgUp
    KeyPgDown
    KeyDelete
    KeyInsert

    // Control keys
    KeyCtrlA  // through KeyCtrlZ
    KeyCtrlSpace
    KeyCtrlBackslash
    KeyCtrlCaret
    KeyCtrlUnderscore

    // Function keys
    KeyF1  // through KeyF20
)
```

### Handling Keys

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Method 1: String matching (concise)
        switch msg.String() {
        case "q", "Q":
            return m, tea.Quit
        case "ctrl+c":
            return m, tea.Quit
        case "enter":
            m.submit()
        case "up", "k":
            m.moveUp()
        case "down", "j":
            m.moveDown()
        case "ctrl+a":
            m.selectAll()
        case "alt+enter":
            m.submitAndContinue()
        }

        // Method 2: Type switching (more precise)
        switch msg.Type {
        case tea.KeyCtrlC:
            return m, tea.Quit
        case tea.KeyEnter:
            m.submit()
        case tea.KeyUp:
            m.moveUp()
        case tea.KeyRunes:
            // Handle typed characters
            m.input += string(msg.Runes)
        }

        // Check modifiers
        if msg.Alt {
            // Alt was held
        }
    }
    return m, nil
}
```

### Key String Representations

```go
// Single characters
"a", "A", "1", "!", "@"

// Special keys
"enter", "tab", "backspace", "esc", "space"
"up", "down", "left", "right"
"home", "end", "pgup", "pgdown"
"delete", "insert"

// Control combinations
"ctrl+a", "ctrl+c", "ctrl+z"
"ctrl+space", "ctrl+@"

// Alt combinations
"alt+a", "alt+enter"

// Function keys
"f1", "f2", ... "f20"
```

---

## Mouse Input

### Enabling Mouse

```go
// At program creation
p := tea.NewProgram(model{}, tea.WithMouseAllMotion())
// or
p := tea.NewProgram(model{}, tea.WithMouseCellMotion())

// Dynamically
return m, tea.EnableMouseAllMotion
return m, tea.DisableMouse
```

### MouseMsg Structure

```go
type Mouse struct {
    X, Y   int          // Position
    Button MouseButton  // Which button
    Action MouseAction  // Press/release/motion
    Ctrl   bool         // Modifiers
    Alt    bool
    Shift  bool
}

type MouseMsg Mouse
```

### Mouse Actions and Buttons

```go
// Actions
const (
    MouseActionPress MouseAction = iota
    MouseActionRelease
    MouseActionMotion
)

// Buttons
const (
    MouseButtonNone MouseButton = iota
    MouseButtonLeft
    MouseButtonMiddle
    MouseButtonRight
    MouseButtonWheelUp
    MouseButtonWheelDown
    MouseButtonWheelLeft
    MouseButtonWheelRight
    MouseButtonBackward
    MouseButtonForward
)
```

### Handling Mouse Events

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.MouseMsg:
        m.mouseX = msg.X
        m.mouseY = msg.Y

        switch msg.Action {
        case tea.MouseActionPress:
            switch msg.Button {
            case tea.MouseButtonLeft:
                m.handleClick(msg.X, msg.Y)
            case tea.MouseButtonRight:
                m.showContextMenu(msg.X, msg.Y)
            case tea.MouseButtonWheelUp:
                m.scrollUp()
            case tea.MouseButtonWheelDown:
                m.scrollDown()
            }

        case tea.MouseActionRelease:
            m.dragging = false

        case tea.MouseActionMotion:
            if msg.Button == tea.MouseButtonLeft {
                m.handleDrag(msg.X, msg.Y)
            } else {
                m.handleHover(msg.X, msg.Y)
            }
        }

        // Check modifiers
        if msg.Ctrl {
            // Ctrl+click
        }
    }
    return m, nil
}
```

---

## Window Management

### WindowSizeMsg

```go
type WindowSizeMsg struct {
    Width  int
    Height int
}
```

### Handling Resize

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

        if !m.ready {
            // First resize - initialize viewport
            m.viewport = viewport.New(msg.Width, msg.Height-4)
            m.viewport.SetContent(m.content)
            m.ready = true
        } else {
            // Subsequent resizes - update dimensions
            m.viewport.Width = msg.Width
            m.viewport.Height = msg.Height - 4
        }
    }
    return m, nil
}

func (m model) View() string {
    if !m.ready {
        return "Loading..."
    }
    return m.viewport.View()
}
```

### Alt Screen Management

```go
// Full-screen mode at start
p := tea.NewProgram(model{}, tea.WithAltScreen())

// Toggle dynamically
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "f" {
            m.fullscreen = !m.fullscreen
            if m.fullscreen {
                return m, tea.EnterAltScreen
            }
            return m, tea.ExitAltScreen
        }
    }
    return m, nil
}
```

---

## External Process Execution

### ExecProcess

Run external commands, temporarily yielding terminal control:

```go
type editorFinishedMsg struct{ err error }

func openEditorCmd(filename string) tea.Cmd {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vim"
    }
    c := exec.Command(editor, filename)
    return tea.ExecProcess(c, func(err error) tea.Msg {
        return editorFinishedMsg{err}
    })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "e" {
            return m, openEditorCmd(m.filename)
        }

    case editorFinishedMsg:
        if msg.err != nil {
            m.err = msg.err
        } else {
            // Reload file content
            return m, loadFileCmd(m.filename)
        }
    }
    return m, nil
}
```

### Shell Commands

```go
func runShellCmd(command string) tea.Cmd {
    c := exec.Command("sh", "-c", command)
    return tea.ExecProcess(c, func(err error) tea.Msg {
        return shellFinishedMsg{err}
    })
}
```

---

## Program Control

### External Control

```go
func main() {
    p := tea.NewProgram(model{})

    // Send messages from outside
    go func() {
        time.Sleep(5 * time.Second)
        p.Send(timeoutMsg{})
    }()

    // Quit from outside
    go func() {
        <-shutdownChan
        p.Quit()
    }()

    // Kill immediately (no cleanup)
    go func() {
        <-forceChan
        p.Kill()
    }()

    if _, err := p.Run(); err != nil {
        log.Fatal(err)
    }
}
```

### Terminal Control

```go
// Release terminal for subprocess
p.ReleaseTerminal()

// Run something that needs the terminal
runInteractiveProcess()

// Restore Bubbletea control
p.RestoreTerminal()
```

### Message Filtering

```go
func messageFilter(m tea.Model, msg tea.Msg) tea.Msg {
    model := m.(myModel)

    // Block quit if unsaved changes
    if _, ok := msg.(tea.QuitMsg); ok {
        if model.hasUnsavedChanges {
            return nil // Block the message
        }
    }

    // Transform messages
    if keyMsg, ok := msg.(tea.KeyMsg); ok {
        if keyMsg.String() == "ctrl+q" {
            return tea.QuitMsg{} // Transform to quit
        }
    }

    return msg // Pass through unchanged
}

p := tea.NewProgram(model{}, tea.WithFilter(messageFilter))
```

---

## Debugging

### Logging to File

```go
func main() {
    // Enable logging when DEBUG is set
    if os.Getenv("DEBUG") != "" {
        f, err := tea.LogToFile("debug.log", "myapp")
        if err != nil {
            log.Fatal(err)
        }
        defer f.Close()
    }

    // Use standard log package
    log.Println("Starting application")

    p := tea.NewProgram(model{})
    if _, err := p.Run(); err != nil {
        log.Printf("Error: %v", err)
    }
}

// In your code
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    log.Printf("Received message: %T %v", msg, msg)
    // ...
}
```

### Running with Debug

```bash
# Enable logging
DEBUG=1 go run main.go

# Watch logs in another terminal
tail -f debug.log
```

### Disable Panic Recovery

```go
// For debugging - panics won't be caught
p := tea.NewProgram(model{}, tea.WithoutCatchPanics())
```

### Using Delve Debugger

Since Bubbletea controls stdin/stdout, use headless mode:

```bash
# Terminal 1: Start debugger
dlv debug --headless --api-version=2 --listen=127.0.0.1:43000 .

# Terminal 2: Connect
dlv connect 127.0.0.1:43000
```

---

## Best Practices Summary

1. **Never block in Update()** - Use commands for I/O
2. **Handle WindowSizeMsg** - Respond to terminal resize
3. **Always handle quit keys** - ctrl+c, q, esc
4. **Use value receivers** - `func (m model)` not `func (m *model)`
5. **Return new model** - Don't rely on mutation
6. **Batch component updates** - Collect commands from child components
7. **Log to file** - Use tea.LogToFile for debugging
8. **Clean up resources** - Cancel contexts, close files before quit

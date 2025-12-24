---
name: charm-tui
description: Build production-ready terminal UIs in Go with the Charm ecosystem (Bubbletea, Lipgloss, Bubbles, Glamour). This skill should be used when the user asks for TUI or terminal UI work, interactive CLI apps, MVU architecture, terminal styling/layouts, or components like forms, lists, tables, spinners, progress bars, or markdown rendering. Covers stability patterns and goroutine safety.
---

# Charm TUI Builder

Build beautiful, stable terminal user interfaces in Go using the Charm ecosystem: Bubbletea (TUI framework), Lipgloss (styling), Bubbles (components), and Glamour (markdown rendering).

## When to Use This Skill

- Building terminal/TUI apps in Go with Bubbletea, Bubbles, Lipgloss, or Glamour
- Creating interactive CLI apps with keyboard/mouse input and MVU architecture
- Styling terminal output with layouts, theming, and composed views
- Implementing components like forms, lists, tables, spinners, or progress bars
- Designing multi-view terminal navigation flows and dashboards
- Rendering markdown or structured data in the terminal

## Quick Start

### Minimal Bubbletea Application

```go
package main

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
)

// Model holds application state
type model struct {
    cursor   int
    choices  []string
    selected map[int]struct{}
}

// Init returns initial command (nil = no initial I/O)
func (m model) Init() tea.Cmd {
    return nil
}

// Update handles messages and returns updated model + commands
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "j":
            if m.cursor < len(m.choices)-1 {
                m.cursor++
            }
        case "enter", " ":
            if _, ok := m.selected[m.cursor]; ok {
                delete(m.selected, m.cursor)
            } else {
                m.selected[m.cursor] = struct{}{}
            }
        }
    }
    return m, nil
}

// View renders the UI as a string
func (m model) View() string {
    s := "Select items:\n\n"
    for i, choice := range m.choices {
        cursor := " "
        if m.cursor == i {
            cursor = ">"
        }
        checked := " "
        if _, ok := m.selected[i]; ok {
            checked = "x"
        }
        s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
    }
    s += "\nPress q to quit.\n"
    return s
}

func main() {
    initial := model{
        choices:  []string{"Option 1", "Option 2", "Option 3"},
        selected: make(map[int]struct{}),
    }
    p := tea.NewProgram(initial)
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Adding Lipgloss Styling

```go
import "github.com/charmbracelet/lipgloss"

var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FAFAFA")).
        Background(lipgloss.Color("#7D56F4")).
        Padding(0, 1)

    selectedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("212")).
        Bold(true)

    normalStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("240"))
)

func (m model) View() string {
    s := titleStyle.Render("Select Items") + "\n\n"
    for i, choice := range m.choices {
        style := normalStyle
        cursor := "  "
        if m.cursor == i {
            style = selectedStyle
            cursor = "> "
        }
        s += style.Render(cursor + choice) + "\n"
    }
    return s
}
```

## Core Concepts

### The MVU Pattern

Bubbletea uses the Model-View-Update (Elm) architecture:

1. **Model**: Application state struct
2. **Init()**: Returns initial command to run
3. **Update(msg)**: Handles messages, returns new model + commands
4. **View()**: Renders model to string for display

### Message Types

```go
// Built-in messages
tea.KeyMsg      // Keyboard input
tea.MouseMsg    // Mouse events (if enabled)
tea.WindowSizeMsg // Terminal resize
tea.QuitMsg     // Program quit signal

// Custom messages
type tickMsg time.Time
type dataLoadedMsg struct { data []string }
type errMsg struct { err error }
```

### Commands

Commands are functions that perform I/O and return messages:

```go
// Return nil for no command
return m, nil

// Quit the program
return m, tea.Quit

// Run multiple commands concurrently
return m, tea.Batch(cmd1, cmd2, cmd3)

// Run commands sequentially
return m, tea.Sequence(cmd1, cmd2, cmd3)

// Timed tick (not synced to clock)
tea.Tick(time.Second, func(t time.Time) tea.Msg {
    return tickMsg(t)
})

// Clock-synced tick
tea.Every(time.Minute, func(t time.Time) tea.Msg {
    return tickMsg(t)
})
```

### Program Options

```go
p := tea.NewProgram(
    model{},
    tea.WithAltScreen(),           // Full-screen mode
    tea.WithMouseAllMotion(),      // All mouse events
    tea.WithMouseCellMotion(),     // Click/drag/wheel only
    tea.WithFPS(120),              // Custom frame rate
    tea.WithoutCatchPanics(),      // Debug mode
    tea.WithReportFocus(),         // Focus gain/loss events
    tea.WithContext(ctx),          // Cancellation context
)
```

## Reference Documentation

For detailed documentation on specific topics, see:

- **[references/bubbletea.md](references/bubbletea.md)** - Complete Bubbletea framework reference
- **[references/lipgloss.md](references/lipgloss.md)** - Comprehensive styling guide
- **[references/bubbles.md](references/bubbles.md)** - Pre-built component library
- **[references/stability.md](references/stability.md)** - Stability patterns and best practices
- **[references/patterns.md](references/patterns.md)** - Common recipes and integration patterns

## Critical Stability Rules

### 1. Never Block in Update()

```go
// BAD - blocks the event loop
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    data := fetchData() // BLOCKS!
    m.data = data
    return m, nil
}

// GOOD - use commands for I/O
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "f" {
            return m, fetchDataCmd // Returns immediately
        }
    case dataLoadedMsg:
        m.data = msg.data
    }
    return m, nil
}

func fetchDataCmd() tea.Msg {
    data, err := fetchData()
    if err != nil {
        return errMsg{err}
    }
    return dataLoadedMsg{data}
}
```

### 2. Handle Window Resize

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        // Update child components
        m.viewport.Width = msg.Width
        m.viewport.Height = msg.Height - 4
    }
    return m, nil
}
```

### 3. Always Handle Quit Keys

```go
case tea.KeyMsg:
    switch msg.String() {
    case "ctrl+c", "q", "esc":
        return m, tea.Quit
    }
```

### 4. Return New Model Copies

```go
// BAD - mutates pointer directly
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    m.count++
    return m, nil
}

// GOOD - value receiver, returns modified copy
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    m.count++
    return m, nil
}
```

### 5. Clean Up Resources

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            // Clean up before quitting
            if m.cancel != nil {
                m.cancel()
            }
            return m, tea.Quit
        }
    }
    return m, nil
}
```

## Common Patterns

### Multi-View Navigation

```go
type viewState int

const (
    viewList viewState = iota
    viewDetail
    viewEdit
)

type model struct {
    state     viewState
    listModel listModel
    detailModel detailModel
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m.state {
    case viewList:
        return m.updateList(msg)
    case viewDetail:
        return m.updateDetail(msg)
    }
    return m, nil
}

func (m model) View() string {
    switch m.state {
    case viewList:
        return m.listModel.View()
    case viewDetail:
        return m.detailModel.View()
    }
    return ""
}
```

### Component Composition

```go
type model struct {
    textInput textinput.Model
    spinner   spinner.Model
    list      list.Model
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        textinput.Blink,
        m.spinner.Tick,
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Update all components
    var cmd tea.Cmd
    m.textInput, cmd = m.textInput.Update(msg)
    cmds = append(cmds, cmd)

    m.spinner, cmd = m.spinner.Update(msg)
    cmds = append(cmds, cmd)

    m.list, cmd = m.list.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}
```

### External Message Injection

```go
func main() {
    p := tea.NewProgram(model{})

    // Send messages from outside
    go func() {
        time.Sleep(2 * time.Second)
        p.Send(customMsg{data: "external data"})
    }()

    if _, err := p.Run(); err != nil {
        log.Fatal(err)
    }
}
```

### Logging for Debug

```go
func main() {
    // Enable debug logging
    if os.Getenv("DEBUG") != "" {
        f, err := tea.LogToFile("debug.log", "debug")
        if err != nil {
            log.Fatal(err)
        }
        defer f.Close()
    }

    p := tea.NewProgram(model{})
    p.Run()
}

// Run with: DEBUG=1 go run main.go
// View logs: tail -f debug.log
```

## Lipgloss Quick Reference

### Colors

```go
// ANSI 16 colors (0-15)
lipgloss.Color("5")

// ANSI 256 colors (0-255)
lipgloss.Color("201")

// True color (hex)
lipgloss.Color("#FF00FF")

// Adaptive (light/dark backgrounds)
lipgloss.AdaptiveColor{Light: "236", Dark: "248"}
```

### Common Style Properties

```go
style := lipgloss.NewStyle().
    // Text formatting
    Bold(true).
    Italic(true).
    Underline(true).
    Strikethrough(true).

    // Colors
    Foreground(lipgloss.Color("212")).
    Background(lipgloss.Color("63")).

    // Spacing
    Padding(1, 2).          // vertical, horizontal
    Margin(1, 2, 1, 2).     // top, right, bottom, left

    // Size
    Width(40).
    Height(10).
    MaxWidth(80).

    // Alignment
    Align(lipgloss.Center).

    // Borders
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("63"))

output := style.Render("Hello, World!")
```

### Layout Utilities

```go
// Join horizontally
lipgloss.JoinHorizontal(lipgloss.Top, block1, block2)

// Join vertically
lipgloss.JoinVertical(lipgloss.Left, block1, block2)

// Place in whitespace
lipgloss.Place(80, 24, lipgloss.Center, lipgloss.Center, content)

// Measure dimensions
width := lipgloss.Width(rendered)
height := lipgloss.Height(rendered)
```

## Bubbles Components

Import components from `github.com/charmbracelet/bubbles`:

| Component | Import | Purpose |
|-----------|--------|---------|
| textinput | `bubbles/textinput` | Single-line text input |
| textarea | `bubbles/textarea` | Multi-line text editor |
| spinner | `bubbles/spinner` | Loading indicators |
| progress | `bubbles/progress` | Progress bars |
| table | `bubbles/table` | Tabular data display |
| list | `bubbles/list` | Filterable lists |
| viewport | `bubbles/viewport` | Scrollable content |
| paginator | `bubbles/paginator` | Page navigation |
| filepicker | `bubbles/filepicker` | File selection |
| timer | `bubbles/timer` | Countdown timer |
| stopwatch | `bubbles/stopwatch` | Count-up timer |
| help | `bubbles/help` | Key binding help |
| key | `bubbles/key` | Key binding definitions |

See [references/bubbles.md](references/bubbles.md) for detailed usage of each component.

## Testing

```go
func TestModel(t *testing.T) {
    m := initialModel()

    // Test initial state
    if m.cursor != 0 {
        t.Error("cursor should start at 0")
    }

    // Test key handling
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
    if m.(model).cursor != 1 {
        t.Error("cursor should move down")
    }

    // Test view doesn't panic
    view := m.View()
    if view == "" {
        t.Error("view should not be empty")
    }
}
```

## Search Patterns for References

When looking for specific information in the reference files:

- **Commands/async**: Search `references/bubbletea.md` for "Cmd", "Batch", "Sequence"
- **Styling**: Search `references/lipgloss.md` for property names
- **Components**: Search `references/bubbles.md` for component name
- **Stability issues**: Search `references/stability.md` for error symptoms
- **Recipes**: Search `references/patterns.md` for use case keywords

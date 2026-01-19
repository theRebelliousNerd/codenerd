# Bubbles Component Reference

Pre-built UI components for Bubbletea applications.

Import: `github.com/charmbracelet/bubbles/<component>`

## Table of Contents

- [Text Input](#text-input)
- [Text Area](#text-area)
- [Spinner](#spinner)
- [Progress Bar](#progress-bar)
- [Table](#table)
- [List](#list)
- [Viewport](#viewport)
- [Paginator](#paginator)
- [File Picker](#file-picker)
- [Timer](#timer)
- [Stopwatch](#stopwatch)
- [Help](#help)
- [Key Bindings](#key-bindings)

---

## Text Input

Single-line text input with cursor, selection, and clipboard support.

### Import

```go
import "github.com/charmbracelet/bubbles/textinput"
```

### Basic Usage

```go
type model struct {
    input textinput.Model
}

func initialModel() model {
    ti := textinput.New()
    ti.Placeholder = "Enter your name"
    ti.Focus()           // Start focused
    ti.CharLimit = 50    // Max characters
    ti.Width = 30        // Display width

    return model{input: ti}
}

func (m model) Init() tea.Cmd {
    return textinput.Blink  // Start cursor blinking
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "enter":
            value := m.input.Value()  // Get entered text
            return m, nil
        case "esc":
            return m, tea.Quit
        }
    }

    m.input, cmd = m.input.Update(msg)
    return m, cmd
}

func (m model) View() string {
    return fmt.Sprintf("Name:\n%s\n\n(esc to quit)", m.input.View())
}
```

### Configuration

```go
ti := textinput.New()

// Basic settings
ti.Placeholder = "Type here..."
ti.CharLimit = 100
ti.Width = 40
ti.Prompt = "> "           // Prompt prefix
ti.SetValue("initial")     // Set initial value
ti.SetCursor(5)            // Set cursor position

// Masking (for passwords)
ti.EchoMode = textinput.EchoPassword  // Show asterisks
ti.EchoMode = textinput.EchoNone      // Show nothing
ti.EchoCharacter = '*'

// Styling
ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
ti.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

// Suggestions/autocomplete
ti.SetSuggestions([]string{"apple", "apricot", "avocado"})
ti.ShowSuggestions = true
```

### Methods

```go
ti.Focus()                    // Enable input
ti.Blur()                     // Disable input
ti.Focused() bool             // Check if focused
ti.Value() string             // Get current value
ti.SetValue(s string)         // Set value
ti.Position() int             // Get cursor position
ti.SetCursor(pos int)         // Set cursor position
ti.Reset()                    // Clear value and reset cursor
```

---

## Text Area

Multi-line text editor with scrolling and line wrapping.

### Import

```go
import "github.com/charmbracelet/bubbles/textarea"
```

### Basic Usage

```go
type model struct {
    textarea textarea.Model
}

func initialModel() model {
    ta := textarea.New()
    ta.Placeholder = "Write your thoughts..."
    ta.Focus()
    ta.SetWidth(60)
    ta.SetHeight(10)

    return model{textarea: ta}
}

func (m model) Init() tea.Cmd {
    return textarea.Blink  // Start cursor blinking
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "ctrl+d" {
            value := m.textarea.Value()  // Get all text
            return m, tea.Quit
        }
    }

    m.textarea, cmd = m.textarea.Update(msg)
    return m, cmd
}

func (m model) View() string {
    return m.textarea.View()
}
```

### Configuration

```go
ta := textarea.New()

// Dimensions
ta.SetWidth(60)
ta.SetHeight(10)
ta.MaxHeight = 20
ta.MaxWidth = 80

// Content
ta.Placeholder = "Start typing..."
ta.SetValue("Initial content\nLine 2")
ta.CharLimit = 5000

// Behavior
ta.ShowLineNumbers = true
ta.LineNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// Styling
ta.FocusedStyle.Base = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
ta.BlurredStyle.Base = lipgloss.NewStyle().BorderStyle(lipgloss.HiddenBorder())
ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
```

### Key Bindings

Default key bindings:
- Arrow keys: Navigate
- Ctrl+E/Ctrl+A: End/start of line
- Ctrl+K: Delete to end of line
- Ctrl+U: Delete to start of line
- Ctrl+D: Delete character
- Ctrl+W: Delete word backward
- Alt+Backspace: Delete word backward
- Alt+D: Delete word forward

---

## Spinner

Animated loading indicator.

### Import

```go
import "github.com/charmbracelet/bubbles/spinner"
```

### Basic Usage

```go
type model struct {
    spinner spinner.Model
    loading bool
}

func initialModel() model {
    s := spinner.New()
    s.Spinner = spinner.Dot  // Spinner style
    s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

    return model{spinner: s, loading: true}
}

func (m model) Init() tea.Cmd {
    return m.spinner.Tick  // Start animation
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }

    case spinner.TickMsg:
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
    return m, nil
}

func (m model) View() string {
    if m.loading {
        return m.spinner.View() + " Loading..."
    }
    return "Done!"
}
```

### Spinner Styles

```go
spinner.Line      // |/-\
spinner.Dot       // ⣾⣽⣻⢿⡿⣟⣯⣷
spinner.MiniDot   // ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏
spinner.Jump      // ⢄⢂⢁⡁⡈⡐⡠
spinner.Pulse     // █▓▒░
spinner.Points    // ∙∙∙
spinner.Globe     // 🌍🌎🌏
spinner.Moon      // 🌑🌒🌓🌔🌕🌖🌗🌘
spinner.Monkey    // 🙈🙉🙊
spinner.Meter     // ▱▰
spinner.Hamburger // ☱☲☴

// Custom spinner
s.Spinner = spinner.Spinner{
    Frames: []string{"▹▹▹▹▹", "▸▹▹▹▹", "▹▸▹▹▹", "▹▹▸▹▹", "▹▹▹▸▹", "▹▹▹▹▸"},
    FPS:    time.Second / 10,
}
```

---

## Progress Bar

Visual progress indicator.

### Import

```go
import "github.com/charmbracelet/bubbles/progress"
```

### Basic Usage

```go
type model struct {
    progress progress.Model
    percent  float64
}

func initialModel() model {
    p := progress.New(
        progress.WithDefaultGradient(),  // Blue to pink gradient
        progress.WithWidth(40),
    )
    return model{progress: p}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg:
        if m.percent >= 1.0 {
            return m, tea.Quit
        }
        m.percent += 0.1

        // Animate to new percentage
        cmd := m.progress.SetPercent(m.percent)
        return m, cmd

    case progress.FrameMsg:
        // Animation frame update
        progressModel, cmd := m.progress.Update(msg)
        m.progress = progressModel.(progress.Model)
        return m, cmd
    }
    return m, nil
}

func (m model) View() string {
    return m.progress.View()
}
```

### Configuration

```go
// Solid color
p := progress.New(
    progress.WithSolidFill("#7D56F4"),
    progress.WithWidth(50),
)

// Gradient
p := progress.New(
    progress.WithGradient("#FF0000", "#00FF00"),
    progress.WithWidth(50),
)

// Default gradient (blue to pink)
p := progress.New(progress.WithDefaultGradient())

// Scaled gradient (repeating)
p := progress.New(
    progress.WithScaledGradient("#FF0000", "#00FF00"),
)

// No percentage display
p := progress.New(
    progress.WithoutPercentage(),
)

// Custom characters
p.Full = '█'
p.Empty = '░'
p.FullColor = "#7D56F4"
p.EmptyColor = "#3C3C3C"
```

### Methods

```go
p.SetPercent(0.5)            // Set to 50% (returns animation cmd)
p.IncrPercent(0.1)           // Increment by 10%
p.DecrPercent(0.1)           // Decrement by 10%
p.Percent() float64          // Get current percentage
p.ViewAs(0.75) string        // Render at specific percentage
```

---

## Table

Interactive data table with selection and scrolling.

### Import

```go
import "github.com/charmbracelet/bubbles/table"
```

### Basic Usage

```go
type model struct {
    table table.Model
}

func initialModel() model {
    columns := []table.Column{
        {Title: "ID", Width: 10},
        {Title: "Name", Width: 20},
        {Title: "Email", Width: 30},
    }

    rows := []table.Row{
        {"1", "Alice", "alice@example.com"},
        {"2", "Bob", "bob@example.com"},
        {"3", "Charlie", "charlie@example.com"},
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithRows(rows),
        table.WithFocused(true),
        table.WithHeight(10),
    )

    // Styling
    s := table.DefaultStyles()
    s.Header = s.Header.
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("240")).
        BorderBottom(true).
        Bold(true)
    s.Selected = s.Selected.
        Foreground(lipgloss.Color("229")).
        Background(lipgloss.Color("57")).
        Bold(false)
    t.SetStyles(s)

    return model{table: t}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "esc":
            return m, tea.Quit
        case "enter":
            row := m.table.SelectedRow()
            // Do something with selected row
            return m, nil
        }
    }

    m.table, cmd = m.table.Update(msg)
    return m, cmd
}

func (m model) View() string {
    return m.table.View()
}
```

### Methods

```go
t.SelectedRow() table.Row    // Get selected row
t.Cursor() int               // Get cursor position
t.SetCursor(i int)           // Set cursor position
t.SetRows(rows []table.Row)  // Replace all rows
t.SetColumns(cols)           // Replace columns
t.SetWidth(w int)            // Set table width
t.SetHeight(h int)           // Set table height
t.Focus()                    // Focus table
t.Blur()                     // Blur table
t.Focused() bool             // Check focus state
t.GotoTop()                  // Go to first row
t.GotoBottom()               // Go to last row
t.MoveUp(n int)              // Move cursor up
t.MoveDown(n int)            // Move cursor down
```

---

## List

Filterable list with fuzzy search.

### Import

```go
import "github.com/charmbracelet/bubbles/list"
```

### Item Interface

```go
// Implement list.Item interface
type item struct {
    title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }  // For filtering
```

### Basic Usage

```go
type model struct {
    list list.Model
}

func initialModel() model {
    items := []list.Item{
        item{title: "Go", desc: "A fast compiled language"},
        item{title: "Python", desc: "A versatile scripting language"},
        item{title: "Rust", desc: "A safe systems language"},
    }

    l := list.New(items, list.NewDefaultDelegate(), 0, 0)
    l.Title = "Programming Languages"
    l.SetFilteringEnabled(true)

    return model{list: l}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.list.SetWidth(msg.Width)
        m.list.SetHeight(msg.Height)
        return m, nil

    case tea.KeyMsg:
        if msg.String() == "enter" {
            if i, ok := m.list.SelectedItem().(item); ok {
                // Do something with selected item
                _ = i
            }
        }
    }

    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

func (m model) View() string {
    return m.list.View()
}
```

### Custom Delegate

```go
// Create custom delegate for item rendering
delegate := list.NewDefaultDelegate()

// Style normal items
delegate.Styles.NormalTitle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("252"))
delegate.Styles.NormalDesc = lipgloss.NewStyle().
    Foreground(lipgloss.Color("240"))

// Style selected items
delegate.Styles.SelectedTitle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("212")).
    Bold(true)
delegate.Styles.SelectedDesc = lipgloss.NewStyle().
    Foreground(lipgloss.Color("212"))

// Dimensions
delegate.SetHeight(2)      // Lines per item
delegate.SetSpacing(1)     // Lines between items

l := list.New(items, delegate, 0, 0)
```

### Configuration

```go
l.Title = "My List"
l.SetShowTitle(true)
l.SetShowStatusBar(true)
l.SetShowFilter(true)
l.SetShowHelp(true)
l.SetShowPagination(true)
l.SetFilteringEnabled(true)

// Status bar format
l.Styles.StatusBar = lipgloss.NewStyle().
    Foreground(lipgloss.Color("240"))

// Filter prompt
l.FilterInput.Prompt = "Filter: "
l.FilterInput.PromptStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("205"))
```

---

## Viewport

Scrollable content container.

### Import

```go
import "github.com/charmbracelet/bubbles/viewport"
```

### Basic Usage

```go
type model struct {
    viewport viewport.Model
    ready    bool
    content  string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        if !m.ready {
            // First time: create viewport
            m.viewport = viewport.New(msg.Width, msg.Height-4)
            m.viewport.SetContent(m.content)
            m.ready = true
        } else {
            // Resize
            m.viewport.Width = msg.Width
            m.viewport.Height = msg.Height - 4
        }

    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            return m, tea.Quit
        }
    }

    m.viewport, cmd = m.viewport.Update(msg)
    return m, cmd
}

func (m model) View() string {
    if !m.ready {
        return "Loading..."
    }

    headerHeight := 2
    header := fmt.Sprintf("Viewing document (%d%%)\n",
        int(m.viewport.ScrollPercent()*100))

    return header + m.viewport.View()
}
```

### Configuration

```go
vp := viewport.New(80, 24)

// Content
vp.SetContent(longText)

// Enable mouse wheel
vp.MouseWheelEnabled = true
vp.MouseWheelDelta = 3

// Styling
vp.Style = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("240"))

// ANSI-aware line counting (for styled content)
vp.SetContent(styledContent)
```

### Methods

```go
vp.SetContent(s string)       // Set content
vp.ScrollPercent() float64    // Get scroll position (0.0-1.0)
vp.AtTop() bool               // At top of content
vp.AtBottom() bool            // At bottom of content
vp.PastBottom() bool          // Scrolled past bottom
vp.LineUp(n int)              // Scroll up n lines
vp.LineDown(n int)            // Scroll down n lines
vp.HalfViewUp()               // Scroll up half viewport
vp.HalfViewDown()             // Scroll down half viewport
vp.ViewUp()                   // Scroll up full viewport
vp.ViewDown()                 // Scroll down full viewport
vp.GotoTop()                  // Scroll to top
vp.GotoBottom()               // Scroll to bottom
```

---

## Paginator

Page navigation for paginated content.

### Import

```go
import "github.com/charmbracelet/bubbles/paginator"
```

### Basic Usage

```go
type model struct {
    paginator paginator.Model
    items     []string
}

func initialModel() model {
    items := make([]string, 50)
    for i := range items {
        items[i] = fmt.Sprintf("Item %d", i+1)
    }

    p := paginator.New()
    p.Type = paginator.Dots  // or paginator.Arabic
    p.PerPage = 5
    p.SetTotalPages(len(items))

    return model{paginator: p, items: items}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    m.paginator, cmd = m.paginator.Update(msg)
    return m, cmd
}

func (m model) View() string {
    start, end := m.paginator.GetSliceBounds(len(m.items))

    var s strings.Builder
    for _, item := range m.items[start:end] {
        s.WriteString(item + "\n")
    }
    s.WriteString("\n" + m.paginator.View())

    return s.String()
}
```

### Configuration

```go
p := paginator.New()

p.Type = paginator.Dots     // ○ ● ○ ○
p.Type = paginator.Arabic   // 1/10

p.PerPage = 10              // Items per page
p.SetTotalPages(100)        // Total pages

p.ActiveDot = "●"           // Active page indicator
p.InactiveDot = "○"         // Inactive page indicator

p.ArabicFormat = "Page %d of %d"
```

### Methods

```go
p.Page                        // Current page (0-indexed)
p.TotalPages                  // Total pages
p.PerPage                     // Items per page
p.GetSliceBounds(total int)   // Get start, end indices
p.NextPage()                  // Go to next page
p.PrevPage()                  // Go to previous page
p.OnLastPage() bool           // Check if on last page
```

---

## File Picker

File system navigation and selection.

### Import

```go
import "github.com/charmbracelet/bubbles/filepicker"
```

### Basic Usage

```go
type model struct {
    filepicker filepicker.Model
    selected   string
}

func initialModel() model {
    fp := filepicker.New()
    fp.AllowedTypes = []string{".go", ".md", ".txt"}
    fp.CurrentDirectory, _ = os.Getwd()
    fp.Height = 20

    return model{filepicker: fp}
}

func (m model) Init() tea.Cmd {
    return m.filepicker.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
    }

    var cmd tea.Cmd
    m.filepicker, cmd = m.filepicker.Update(msg)

    // Check for file selection
    if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
        m.selected = path
    }

    // Check for directory selection
    if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
        // User selected a disabled file (filtered out)
        _ = path
    }

    return m, cmd
}

func (m model) View() string {
    if m.selected != "" {
        return fmt.Sprintf("Selected: %s", m.selected)
    }
    return m.filepicker.View()
}
```

### Configuration

```go
fp := filepicker.New()

fp.CurrentDirectory = "/home/user"
fp.AllowedTypes = []string{".go", ".txt"}  // Empty = all types
fp.ShowHidden = true
fp.ShowPermissions = true
fp.ShowSize = true
fp.Height = 20
fp.DirAllowed = true         // Allow selecting directories
fp.FileAllowed = true        // Allow selecting files

// Styling
fp.Styles.Cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
fp.Styles.Directory = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
fp.Styles.File = lipgloss.NewStyle()
fp.Styles.Selected = lipgloss.NewStyle().Bold(true)
fp.Styles.DisabledFile = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
```

---

## Timer

Countdown timer.

### Import

```go
import "github.com/charmbracelet/bubbles/timer"
```

### Basic Usage

```go
type model struct {
    timer timer.Model
}

func initialModel() model {
    t := timer.NewWithInterval(10*time.Second, time.Second)
    return model{timer: t}
}

func (m model) Init() tea.Cmd {
    return m.timer.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case timer.TickMsg:
        var cmd tea.Cmd
        m.timer, cmd = m.timer.Update(msg)
        return m, cmd

    case timer.TimeoutMsg:
        // Timer finished
        return m, tea.Quit

    case tea.KeyMsg:
        switch msg.String() {
        case "s":
            return m, m.timer.Toggle()  // Start/stop
        case "r":
            return m, m.timer.Reset()   // Reset to initial
        }
    }
    return m, nil
}

func (m model) View() string {
    return m.timer.View()
}
```

---

## Stopwatch

Count-up timer.

### Import

```go
import "github.com/charmbracelet/bubbles/stopwatch"
```

### Basic Usage

```go
type model struct {
    stopwatch stopwatch.Model
}

func initialModel() model {
    s := stopwatch.NewWithInterval(time.Second)
    return model{stopwatch: s}
}

func (m model) Init() tea.Cmd {
    return m.stopwatch.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case stopwatch.TickMsg:
        var cmd tea.Cmd
        m.stopwatch, cmd = m.stopwatch.Update(msg)
        return m, cmd

    case tea.KeyMsg:
        switch msg.String() {
        case "s":
            return m, m.stopwatch.Toggle()  // Start/stop
        case "r":
            return m, m.stopwatch.Reset()   // Reset to zero
        }
    }
    return m, nil
}

func (m model) View() string {
    return m.stopwatch.View()  // Format: 00:00:00
}
```

---

## Help

Keybinding help display.

### Import

```go
import (
    "github.com/charmbracelet/bubbles/help"
    "github.com/charmbracelet/bubbles/key"
)
```

### Basic Usage

```go
// Define key bindings
type keyMap struct {
    Up    key.Binding
    Down  key.Binding
    Help  key.Binding
    Quit  key.Binding
}

// Implement help.KeyMap interface
func (k keyMap) ShortHelp() []key.Binding {
    return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        {k.Up, k.Down},       // First column
        {k.Help, k.Quit},     // Second column
    }
}

type model struct {
    keys keyMap
    help help.Model
}

func initialModel() model {
    keys := keyMap{
        Up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
        Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
        Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
        Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
    }

    return model{
        keys: keys,
        help: help.New(),
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.help.Width = msg.Width

    case tea.KeyMsg:
        switch {
        case key.Matches(msg, m.keys.Help):
            m.help.ShowAll = !m.help.ShowAll
        case key.Matches(msg, m.keys.Quit):
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m model) View() string {
    helpView := m.help.View(m.keys)
    return "Content here\n\n" + helpView
}
```

---

## Key Bindings

Define and manage keybindings.

### Import

```go
import "github.com/charmbracelet/bubbles/key"
```

### Creating Bindings

```go
// Basic binding
quit := key.NewBinding(
    key.WithKeys("q", "ctrl+c"),      // Keys that trigger this
    key.WithHelp("q", "quit"),        // Help text
)

// Binding with disabled state
save := key.NewBinding(
    key.WithKeys("ctrl+s"),
    key.WithHelp("ctrl+s", "save"),
    key.WithDisabled(),               // Start disabled
)

// Check if key matches binding
switch msg := msg.(type) {
case tea.KeyMsg:
    if key.Matches(msg, quit) {
        return m, tea.Quit
    }
}
```

### Binding Methods

```go
b := key.NewBinding(key.WithKeys("q"))

b.Enabled() bool       // Check if enabled
b.SetEnabled(bool)     // Enable/disable
b.Enable()             // Enable
b.Disable()            // Disable

b.Keys() []string      // Get bound keys
b.SetKeys(...string)   // Set keys

b.Help() key.Help      // Get help info
b.SetHelp(key, desc)   // Set help text

// Help struct
type Help struct {
    Key  string  // e.g., "q"
    Desc string  // e.g., "quit"
}
```

### Typical KeyMap Pattern

```go
type keyMap struct {
    Up     key.Binding
    Down   key.Binding
    Left   key.Binding
    Right  key.Binding
    Enter  key.Binding
    Help   key.Binding
    Quit   key.Binding
}

func defaultKeyMap() keyMap {
    return keyMap{
        Up: key.NewBinding(
            key.WithKeys("up", "k"),
            key.WithHelp("↑/k", "move up"),
        ),
        Down: key.NewBinding(
            key.WithKeys("down", "j"),
            key.WithHelp("↓/j", "move down"),
        ),
        Left: key.NewBinding(
            key.WithKeys("left", "h"),
            key.WithHelp("←/h", "move left"),
        ),
        Right: key.NewBinding(
            key.WithKeys("right", "l"),
            key.WithHelp("→/l", "move right"),
        ),
        Enter: key.NewBinding(
            key.WithKeys("enter"),
            key.WithHelp("enter", "select"),
        ),
        Help: key.NewBinding(
            key.WithKeys("?"),
            key.WithHelp("?", "toggle help"),
        ),
        Quit: key.NewBinding(
            key.WithKeys("q", "esc", "ctrl+c"),
            key.WithHelp("q", "quit"),
        ),
    }
}
```

# Common Patterns and Recipes

Reusable patterns for building Bubbletea applications.

## Table of Contents

- [Application Architecture](#application-architecture)
- [Multi-View Navigation](#multi-view-navigation)
- [Forms and Input](#forms-and-input)
- [Loading States](#loading-states)
- [Confirmation Dialogs](#confirmation-dialogs)
- [Notifications and Toasts](#notifications-and-toasts)
- [Command Palette](#command-palette)
- [Split Layouts](#split-layouts)
- [Tab Navigation](#tab-navigation)
- [Markdown Rendering](#markdown-rendering)
- [Real-time Updates](#real-time-updates)
- [CLI Integration](#cli-integration)

---

## Application Architecture

### Standard Application Structure

```
myapp/
├── main.go           # Entry point
├── model.go          # Main model and Update
├── view.go           # View function
├── commands.go       # Command definitions
├── messages.go       # Message type definitions
├── keys.go           # Key bindings
├── styles.go         # Lipgloss styles
└── components/       # Reusable components
    ├── header.go
    ├── sidebar.go
    └── statusbar.go
```

### Main Model Pattern

```go
// model.go
type model struct {
    // State
    state     appState
    width     int
    height    int
    ready     bool

    // Components
    list      list.Model
    viewport  viewport.Model
    textInput textinput.Model
    help      help.Model

    // Data
    items     []Item
    selected  *Item
    err       error

    // Config
    keys      keyMap
    styles    styles
}

type appState int

const (
    stateList appState = iota
    stateDetail
    stateEdit
    stateConfirm
)
```

### Initialization Pattern

```go
// main.go
func main() {
    // Parse flags
    flag.Parse()

    // Initialize model
    m := initialModel()

    // Configure program
    p := tea.NewProgram(
        m,
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    // Run
    finalModel, err := p.Run()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Handle exit state
    if m, ok := finalModel.(model); ok {
        if m.selected != nil {
            fmt.Println("Selected:", m.selected.Name)
        }
    }
}

func initialModel() model {
    // Initialize components
    ti := textinput.New()
    ti.Placeholder = "Search..."
    ti.CharLimit = 100

    h := help.New()
    h.ShowAll = false

    return model{
        state:     stateList,
        textInput: ti,
        help:      h,
        keys:      defaultKeyMap(),
        styles:    defaultStyles(),
    }
}
```

---

## Multi-View Navigation

### State Machine Pattern

```go
type viewState int

const (
    viewMain viewState = iota
    viewDetail
    viewEdit
    viewHelp
)

type model struct {
    currentView viewState
    prevView    viewState  // For back navigation

    // View-specific models
    mainView   mainViewModel
    detailView detailViewModel
    editView   editViewModel
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Global keys work in all views
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c":
            return m, tea.Quit
        case "?":
            m.prevView = m.currentView
            m.currentView = viewHelp
            return m, nil
        case "esc":
            if m.currentView != viewMain {
                m.currentView = m.prevView
                return m, nil
            }
        }
    }

    // Delegate to current view
    var cmd tea.Cmd
    switch m.currentView {
    case viewMain:
        m.mainView, cmd = m.mainView.Update(msg)
        // Check for navigation
        if m.mainView.shouldNavigate {
            m.currentView = viewDetail
            m.mainView.shouldNavigate = false
        }
    case viewDetail:
        m.detailView, cmd = m.detailView.Update(msg)
    case viewEdit:
        m.editView, cmd = m.editView.Update(msg)
    }

    return m, cmd
}

func (m model) View() string {
    switch m.currentView {
    case viewMain:
        return m.mainView.View()
    case viewDetail:
        return m.detailView.View()
    case viewEdit:
        return m.editView.View()
    case viewHelp:
        return m.renderHelp()
    }
    return ""
}
```

### Stack-Based Navigation

```go
type model struct {
    viewStack []view
}

type view interface {
    Update(tea.Msg) (view, tea.Cmd, navigationAction)
    View() string
}

type navigationAction int

const (
    navNone navigationAction = iota
    navPush
    navPop
    navReplace
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if len(m.viewStack) == 0 {
        return m, tea.Quit
    }

    currentView := m.viewStack[len(m.viewStack)-1]
    newView, cmd, action := currentView.Update(msg)

    switch action {
    case navPush:
        m.viewStack = append(m.viewStack, newView)
    case navPop:
        if len(m.viewStack) > 1 {
            m.viewStack = m.viewStack[:len(m.viewStack)-1]
        } else {
            return m, tea.Quit
        }
    case navReplace:
        m.viewStack[len(m.viewStack)-1] = newView
    default:
        m.viewStack[len(m.viewStack)-1] = newView
    }

    return m, cmd
}
```

---

## Forms and Input

### Multi-Field Form

```go
type formModel struct {
    inputs    []textinput.Model
    focused   int
    submitted bool
}

func newFormModel() formModel {
    inputs := make([]textinput.Model, 3)

    inputs[0] = textinput.New()
    inputs[0].Placeholder = "Username"
    inputs[0].Focus()
    inputs[0].CharLimit = 20

    inputs[1] = textinput.New()
    inputs[1].Placeholder = "Email"
    inputs[1].CharLimit = 50

    inputs[2] = textinput.New()
    inputs[2].Placeholder = "Password"
    inputs[2].EchoMode = textinput.EchoPassword
    inputs[2].CharLimit = 50

    return formModel{inputs: inputs, focused: 0}
}

func (m formModel) Update(msg tea.Msg) (formModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "tab", "down":
            m.focused = (m.focused + 1) % len(m.inputs)
            return m, m.updateFocus()
        case "shift+tab", "up":
            m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
            return m, m.updateFocus()
        case "enter":
            if m.focused == len(m.inputs)-1 {
                m.submitted = true
                return m, nil
            }
            m.focused++
            return m, m.updateFocus()
        }
    }

    // Update focused input
    var cmd tea.Cmd
    m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
    return m, cmd
}

func (m formModel) updateFocus() tea.Cmd {
    cmds := make([]tea.Cmd, len(m.inputs))
    for i := range m.inputs {
        if i == m.focused {
            cmds[i] = m.inputs[i].Focus()
        } else {
            m.inputs[i].Blur()
        }
    }
    return tea.Batch(cmds...)
}

func (m formModel) View() string {
    var b strings.Builder

    for i, input := range m.inputs {
        b.WriteString(input.View())
        if i < len(m.inputs)-1 {
            b.WriteString("\n")
        }
    }

    b.WriteString("\n\n")
    b.WriteString("Tab/Shift+Tab to navigate, Enter to submit")

    return b.String()
}

func (m formModel) Values() map[string]string {
    return map[string]string{
        "username": m.inputs[0].Value(),
        "email":    m.inputs[1].Value(),
        "password": m.inputs[2].Value(),
    }
}
```

---

## Loading States

### Spinner with Status

```go
type model struct {
    spinner  spinner.Model
    loading  bool
    status   string
    progress float64
}

func initialModel() model {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

    return model{spinner: s}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case startLoadingMsg:
        m.loading = true
        m.status = "Starting..."
        return m, tea.Batch(m.spinner.Tick, performTask())

    case progressMsg:
        m.progress = msg.percent
        m.status = msg.status

    case doneMsg:
        m.loading = false
        m.status = "Complete!"

    case spinner.TickMsg:
        if m.loading {
            var cmd tea.Cmd
            m.spinner, cmd = m.spinner.Update(msg)
            return m, cmd
        }
    }
    return m, nil
}

func (m model) View() string {
    if m.loading {
        return fmt.Sprintf("%s %s (%.0f%%)",
            m.spinner.View(),
            m.status,
            m.progress*100)
    }
    return m.status
}
```

### Progress with Steps

```go
type step struct {
    name   string
    status stepStatus
}

type stepStatus int

const (
    stepPending stepStatus = iota
    stepRunning
    stepDone
    stepError
)

type model struct {
    steps   []step
    current int
    spinner spinner.Model
}

func (m model) View() string {
    var b strings.Builder

    for i, s := range m.steps {
        var icon string
        switch s.status {
        case stepPending:
            icon = "○"
        case stepRunning:
            icon = m.spinner.View()
        case stepDone:
            icon = "✓"
        case stepError:
            icon = "✗"
        }

        style := lipgloss.NewStyle()
        if s.status == stepDone {
            style = style.Foreground(lipgloss.Color("42"))
        } else if s.status == stepError {
            style = style.Foreground(lipgloss.Color("196"))
        } else if s.status == stepRunning {
            style = style.Bold(true)
        } else {
            style = style.Foreground(lipgloss.Color("240"))
        }

        b.WriteString(style.Render(fmt.Sprintf("%s %s", icon, s.name)))
        if i < len(m.steps)-1 {
            b.WriteString("\n")
        }
    }

    return b.String()
}
```

---

## Confirmation Dialogs

### Simple Confirm Dialog

```go
type confirmModel struct {
    message   string
    confirmed bool
    cancelled bool
    focused   int  // 0 = Yes, 1 = No
}

func newConfirmModel(message string) confirmModel {
    return confirmModel{message: message, focused: 1}  // Default to No
}

func (m confirmModel) Update(msg tea.Msg) (confirmModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "left", "h":
            m.focused = 0
        case "right", "l":
            m.focused = 1
        case "enter":
            if m.focused == 0 {
                m.confirmed = true
            } else {
                m.cancelled = true
            }
        case "y", "Y":
            m.confirmed = true
        case "n", "N", "esc":
            m.cancelled = true
        }
    }
    return m, nil
}

func (m confirmModel) View() string {
    yesStyle := lipgloss.NewStyle().Padding(0, 2)
    noStyle := lipgloss.NewStyle().Padding(0, 2)

    if m.focused == 0 {
        yesStyle = yesStyle.Background(lipgloss.Color("212")).Foreground(lipgloss.Color("0"))
    } else {
        noStyle = noStyle.Background(lipgloss.Color("212")).Foreground(lipgloss.Color("0"))
    }

    buttons := lipgloss.JoinHorizontal(lipgloss.Center,
        yesStyle.Render("Yes"),
        "  ",
        noStyle.Render("No"),
    )

    return lipgloss.JoinVertical(lipgloss.Center,
        m.message,
        "",
        buttons,
    )
}

func (m confirmModel) Done() bool {
    return m.confirmed || m.cancelled
}
```

### Modal Overlay Pattern

```go
type model struct {
    content    contentModel
    modal      tea.Model
    showModal  bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.showModal {
        // Modal captures all input
        switch msg := msg.(type) {
        case tea.KeyMsg:
            if msg.String() == "esc" {
                m.showModal = false
                return m, nil
            }
        }

        var cmd tea.Cmd
        m.modal, cmd = m.modal.Update(msg)

        // Check if modal is done
        if cm, ok := m.modal.(confirmModel); ok && cm.Done() {
            m.showModal = false
            if cm.confirmed {
                return m, performAction()
            }
        }

        return m, cmd
    }

    // Normal input handling
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "d" {
            m.modal = newConfirmModel("Delete this item?")
            m.showModal = true
            return m, nil
        }
    }

    var cmd tea.Cmd
    m.content, cmd = m.content.Update(msg)
    return m, cmd
}

func (m model) View() string {
    content := m.content.View()

    if m.showModal {
        // Center modal over content
        modalBox := lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("212")).
            Padding(1, 2).
            Render(m.modal.View())

        return lipgloss.Place(m.width, m.height,
            lipgloss.Center, lipgloss.Center,
            modalBox,
            lipgloss.WithWhitespaceChars(" "),
            lipgloss.WithWhitespaceBackground(lipgloss.Color("0")),
        )
    }

    return content
}
```

---

## Notifications and Toasts

### Toast Notification System

```go
type notification struct {
    message string
    level   notificationLevel
    expiry  time.Time
}

type notificationLevel int

const (
    levelInfo notificationLevel = iota
    levelSuccess
    levelWarning
    levelError
)

type model struct {
    notifications []notification
    // ... other fields
}

type addNotificationMsg struct {
    message string
    level   notificationLevel
}

type clearNotificationMsg struct{}

func (m model) notify(message string, level notificationLevel) tea.Cmd {
    return func() tea.Msg {
        return addNotificationMsg{message, level}
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case addNotificationMsg:
        n := notification{
            message: msg.message,
            level:   msg.level,
            expiry:  time.Now().Add(3 * time.Second),
        }
        m.notifications = append(m.notifications, n)

        // Schedule removal
        return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
            return clearNotificationMsg{}
        })

    case clearNotificationMsg:
        // Remove expired notifications
        now := time.Now()
        var active []notification
        for _, n := range m.notifications {
            if n.expiry.After(now) {
                active = append(active, n)
            }
        }
        m.notifications = active
    }
    return m, nil
}

func (m model) renderNotifications() string {
    if len(m.notifications) == 0 {
        return ""
    }

    var rendered []string
    for _, n := range m.notifications {
        style := lipgloss.NewStyle().Padding(0, 1)

        switch n.level {
        case levelSuccess:
            style = style.Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0"))
        case levelWarning:
            style = style.Background(lipgloss.Color("214")).Foreground(lipgloss.Color("0"))
        case levelError:
            style = style.Background(lipgloss.Color("196")).Foreground(lipgloss.Color("15"))
        default:
            style = style.Background(lipgloss.Color("63")).Foreground(lipgloss.Color("15"))
        }

        rendered = append(rendered, style.Render(n.message))
    }

    return lipgloss.JoinVertical(lipgloss.Right, rendered...)
}
```

---

## Command Palette

### Fuzzy Search Command Palette

```go
type command struct {
    name     string
    desc     string
    shortcut string
    action   func() tea.Msg
}

type paletteModel struct {
    input    textinput.Model
    commands []command
    filtered []command
    cursor   int
    visible  bool
}

func newPaletteModel(commands []command) paletteModel {
    ti := textinput.New()
    ti.Placeholder = "Type a command..."
    ti.Focus()

    return paletteModel{
        input:    ti,
        commands: commands,
        filtered: commands,
    }
}

func (m paletteModel) Update(msg tea.Msg) (paletteModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up", "ctrl+p":
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "ctrl+n":
            if m.cursor < len(m.filtered)-1 {
                m.cursor++
            }
        case "enter":
            if m.cursor < len(m.filtered) {
                m.visible = false
                return m, func() tea.Msg {
                    return m.filtered[m.cursor].action()
                }
            }
        case "esc":
            m.visible = false
            m.input.SetValue("")
            m.filtered = m.commands
            m.cursor = 0
        }
    }

    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)

    // Filter commands
    query := strings.ToLower(m.input.Value())
    if query == "" {
        m.filtered = m.commands
    } else {
        m.filtered = nil
        for _, c := range m.commands {
            if strings.Contains(strings.ToLower(c.name), query) ||
               strings.Contains(strings.ToLower(c.desc), query) {
                m.filtered = append(m.filtered, c)
            }
        }
    }

    // Clamp cursor
    if m.cursor >= len(m.filtered) {
        m.cursor = max(0, len(m.filtered)-1)
    }

    return m, cmd
}

func (m paletteModel) View() string {
    if !m.visible {
        return ""
    }

    var b strings.Builder
    b.WriteString(m.input.View() + "\n\n")

    for i, cmd := range m.filtered {
        cursor := "  "
        style := lipgloss.NewStyle()
        if i == m.cursor {
            cursor = "> "
            style = style.Bold(true).Foreground(lipgloss.Color("212"))
        }

        shortcut := ""
        if cmd.shortcut != "" {
            shortcut = lipgloss.NewStyle().
                Foreground(lipgloss.Color("240")).
                Render(" [" + cmd.shortcut + "]")
        }

        b.WriteString(style.Render(cursor + cmd.name + shortcut) + "\n")
    }

    return lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(1, 2).
        Width(50).
        Render(b.String())
}
```

---

## Split Layouts

### Horizontal Split

```go
type model struct {
    left       leftPanelModel
    right      rightPanelModel
    focused    int  // 0 = left, 1 = right
    splitRatio float64
    width      int
    height     int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.updatePanelSizes()

    case tea.KeyMsg:
        switch msg.String() {
        case "tab":
            m.focused = (m.focused + 1) % 2
        case "[":
            m.splitRatio = max(0.2, m.splitRatio-0.05)
            m.updatePanelSizes()
        case "]":
            m.splitRatio = min(0.8, m.splitRatio+0.05)
            m.updatePanelSizes()
        }
    }

    // Update focused panel
    var cmd tea.Cmd
    if m.focused == 0 {
        m.left, cmd = m.left.Update(msg)
    } else {
        m.right, cmd = m.right.Update(msg)
    }

    return m, cmd
}

func (m *model) updatePanelSizes() {
    leftWidth := int(float64(m.width) * m.splitRatio)
    rightWidth := m.width - leftWidth - 1  // -1 for divider

    m.left.SetSize(leftWidth, m.height)
    m.right.SetSize(rightWidth, m.height)
}

func (m model) View() string {
    divider := lipgloss.NewStyle().
        Foreground(lipgloss.Color("240")).
        Render(strings.Repeat("│\n", m.height))

    leftStyle := lipgloss.NewStyle()
    rightStyle := lipgloss.NewStyle()

    if m.focused == 0 {
        leftStyle = leftStyle.BorderForeground(lipgloss.Color("212"))
    } else {
        rightStyle = rightStyle.BorderForeground(lipgloss.Color("212"))
    }

    return lipgloss.JoinHorizontal(lipgloss.Top,
        leftStyle.Render(m.left.View()),
        divider,
        rightStyle.Render(m.right.View()),
    )
}
```

---

## Tab Navigation

### Tab Bar Component

```go
type tabModel struct {
    tabs    []string
    active  int
    content []tea.Model
}

func (m tabModel) Update(msg tea.Msg) (tabModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "1", "2", "3", "4", "5", "6", "7", "8", "9":
            idx := int(msg.Runes[0] - '1')
            if idx < len(m.tabs) {
                m.active = idx
            }
        case "shift+tab", "left":
            m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
        case "tab", "right":
            m.active = (m.active + 1) % len(m.tabs)
        }
    }

    // Update active tab content
    var cmd tea.Cmd
    m.content[m.active], cmd = m.content[m.active].Update(msg)
    return m, cmd
}

func (m tabModel) View() string {
    var tabs []string

    for i, t := range m.tabs {
        style := lipgloss.NewStyle().Padding(0, 2)
        if i == m.active {
            style = style.
                Bold(true).
                Background(lipgloss.Color("212")).
                Foreground(lipgloss.Color("0"))
        } else {
            style = style.
                Foreground(lipgloss.Color("240"))
        }
        tabs = append(tabs, style.Render(t))
    }

    tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
    separator := lipgloss.NewStyle().
        Foreground(lipgloss.Color("240")).
        Render(strings.Repeat("─", lipgloss.Width(tabBar)))

    content := m.content[m.active].View()

    return lipgloss.JoinVertical(lipgloss.Left,
        tabBar,
        separator,
        content,
    )
}
```

---

## Markdown Rendering

### Using Glamour

```go
import "github.com/charmbracelet/glamour"

type model struct {
    viewport viewport.Model
    renderer *glamour.TermRenderer
    content  string
}

func initialModel() model {
    renderer, _ := glamour.NewTermRenderer(
        glamour.WithAutoStyle(),
        glamour.WithWordWrap(80),
    )

    return model{renderer: renderer}
}

func (m model) loadMarkdown(content string) (model, tea.Cmd) {
    rendered, err := m.renderer.Render(content)
    if err != nil {
        return m, nil
    }

    m.viewport.SetContent(rendered)
    return m, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        // Recreate renderer with new width
        m.renderer, _ = glamour.NewTermRenderer(
            glamour.WithAutoStyle(),
            glamour.WithWordWrap(msg.Width-4),
        )

        // Re-render content
        if m.content != "" {
            rendered, _ := m.renderer.Render(m.content)
            m.viewport.SetContent(rendered)
        }

        m.viewport.Width = msg.Width
        m.viewport.Height = msg.Height
    }

    var cmd tea.Cmd
    m.viewport, cmd = m.viewport.Update(msg)
    return m, cmd
}
```

---

## Real-time Updates

### WebSocket Integration

```go
type model struct {
    messages []string
    conn     *websocket.Conn
    err      error
}

type wsMessageMsg struct {
    data string
}

type wsErrorMsg struct {
    err error
}

func connectCmd(url string) tea.Cmd {
    return func() tea.Msg {
        conn, _, err := websocket.DefaultDialer.Dial(url, nil)
        if err != nil {
            return wsErrorMsg{err}
        }
        return wsConnectedMsg{conn}
    }
}

func listenCmd(conn *websocket.Conn) tea.Cmd {
    return func() tea.Msg {
        _, message, err := conn.ReadMessage()
        if err != nil {
            return wsErrorMsg{err}
        }
        return wsMessageMsg{string(message)}
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case wsConnectedMsg:
        m.conn = msg.conn
        return m, listenCmd(m.conn)

    case wsMessageMsg:
        m.messages = append(m.messages, msg.data)
        // Keep listening
        return m, listenCmd(m.conn)

    case wsErrorMsg:
        m.err = msg.err
    }
    return m, nil
}
```

### Polling Updates

```go
type model struct {
    data      Data
    lastFetch time.Time
}

type tickMsg time.Time
type dataMsg Data

func pollCmd() tea.Cmd {
    return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

func fetchCmd() tea.Cmd {
    return func() tea.Msg {
        data, _ := fetchLatestData()
        return dataMsg(data)
    }
}

func (m model) Init() tea.Cmd {
    return tea.Batch(fetchCmd(), pollCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg:
        return m, tea.Batch(fetchCmd(), pollCmd())

    case dataMsg:
        m.data = Data(msg)
        m.lastFetch = time.Now()
    }
    return m, nil
}
```

---

## CLI Integration

### Flag Parsing and Configuration

```go
var (
    flagConfig  = flag.String("config", "", "Config file path")
    flagVerbose = flag.Bool("v", false, "Verbose output")
)

func main() {
    flag.Parse()

    // Load config
    cfg, err := loadConfig(*flagConfig)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Check if interactive
    if !isTerminal() {
        // Non-interactive mode
        runBatch(cfg)
        return
    }

    // Interactive TUI
    m := initialModel(cfg)
    p := tea.NewProgram(m, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func isTerminal() bool {
    fileInfo, _ := os.Stdin.Stat()
    return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
```

### Output After Exit

```go
type model struct {
    selected []string
    output   string
}

func main() {
    m := initialModel()
    p := tea.NewProgram(m)

    finalModel, err := p.Run()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Access final state
    if fm, ok := finalModel.(model); ok {
        if len(fm.selected) > 0 {
            fmt.Println("Selected items:")
            for _, s := range fm.selected {
                fmt.Println(" -", s)
            }
        }
        if fm.output != "" {
            fmt.Println(fm.output)
        }
    }
}
```

### Pipe-Friendly Mode

```go
func main() {
    // Read from stdin if piped
    stat, _ := os.Stdin.Stat()
    if (stat.Mode() & os.ModeCharDevice) == 0 {
        // Data piped in
        input, _ := io.ReadAll(os.Stdin)
        items := strings.Split(string(input), "\n")

        // Launch selector
        m := initialModel(items)
        p := tea.NewProgram(m, tea.WithInput(os.Stdin))
        final, _ := p.Run()

        // Output selection for piping
        if fm, ok := final.(model); ok {
            fmt.Println(fm.selected)
        }
        return
    }

    // Normal interactive mode
    // ...
}
```

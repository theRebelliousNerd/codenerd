package ui

import (
	"codenerd/internal/prompt"
	"fmt"
	"github.com/atotto/clipboard"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// clipboardWriteAll is a package-level variable to allow mocking in tests.
var clipboardWriteAll = clipboard.WriteAll

// JITPageModel defines the state of the JIT Prompt Inspector.
// TODO: Persist Mandatory/Optional toggle state (filter preference) across sessions.
type JITPageModel struct {
	width    int
	height   int
	list     list.Model
	viewport viewport.Model

	// Focus state
	focusViewport bool

	// Search state
	searchInput   textinput.Model
	searchFocused bool

	// Data
	lastResult *prompt.CompilationResult
	selected   *prompt.PromptAtom

	// Styles
	styles Styles
}

// atomItem adapts prompt.PromptAtom to list.Item
// TODO: IMPROVEMENT: Add support for custom icons based on atom category.
type atomItem struct {
	atom *prompt.PromptAtom
}

func (i atomItem) Title() string { return i.atom.ID }
func (i atomItem) Description() string {
	return fmt.Sprintf("[%s] Prio:%d Tokens:%d", i.atom.Category, i.atom.Priority, i.atom.TokenCount)
}
func (i atomItem) FilterValue() string {
	return i.atom.ID + " " + string(i.atom.Category) + " " + i.atom.Content
}

// NewJITPageModel creates a new JIT inspector page.
func NewJITPageModel() JITPageModel {
	vp := viewport.New(0, 0)
	vp.SetContent("Select an atom to view content.")

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Prompt Atoms"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "Filter atoms..."
	ti.Width = 40

	return JITPageModel{
		list:     l,
		viewport: vp,
		searchInput: ti,
		searchFocused: false,
		styles:   DefaultStyles(),
	}
}

// Init initializes the model.
func (m JITPageModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m JITPageModel) Update(msg tea.Msg) (JITPageModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			if !m.searchFocused {
				m.searchFocused = true
				cmd = m.searchInput.Focus()
				return m, cmd
			}
		case "esc", "enter":
			if m.searchFocused {
				m.searchFocused = false
				m.searchInput.Blur()
				return m, nil
			}
		case "tab":
			if !m.searchFocused {
				m.focusViewport = !m.focusViewport
				return m, nil
			}
		case "c", "y":
			if !m.searchFocused && !m.focusViewport {
				if m.selected != nil {
					if err := clipboardWriteAll(m.selected.Content); err != nil {
						cmd = m.list.NewStatusMessage(m.styles.Error.Render("Failed to copy atom content"))
					} else {
						cmd = m.list.NewStatusMessage(m.styles.Success.Render(fmt.Sprintf("Copied atom content for [%s] to clipboard", m.selected.ID)))
					}
					cmds = append(cmds, cmd)
				}
			}
		case "p":
			if !m.searchFocused && !m.focusViewport {
				if m.lastResult != nil {
					if err := clipboardWriteAll(m.lastResult.Prompt); err != nil {
						cmd = m.list.NewStatusMessage(m.styles.Error.Render("Failed to copy full prompt"))
					} else {
						cmd = m.list.NewStatusMessage(m.styles.Success.Render("Copied full prompt to clipboard"))
					}
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	// Determine where to route events
	_, isKey := msg.(tea.KeyMsg)

	if m.searchFocused {
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
		m.applySearch()
	}

	updateList := !isKey || (!m.searchFocused && !m.focusViewport)
	updateViewport := !isKey || (m.focusViewport && !m.searchFocused)

	if updateList {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	if updateViewport {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Check for selection change ALWAYS, not just conditionally
	if sel := m.list.SelectedItem(); sel != nil {
		item := sel.(atomItem)
		if m.selected == nil || m.selected.ID != item.atom.ID {
			m.selected = item.atom
			m.viewport.SetContent(m.renderAtomContent(item.atom))
		}
	} else {
		// Handle empty list case
		m.selected = nil
		m.viewport.SetContent("No atoms match the filter.")
	}

	return m, tea.Batch(cmds...)
}

// renderAtomContent formats the atom for display using strings.Builder
// TODO: IMPROVEMENT: Implement syntax highlighting for atom content based on file type (e.g., Markdown, Mangle, Go).
func (m JITPageModel) renderAtomContent(atom *prompt.PromptAtom) string {
	headerStyle := m.styles.Header
	infoStyle := m.styles.Info
	mutedStyle := m.styles.Muted

	header := headerStyle.Render(atom.ID)
	info := infoStyle.Render(fmt.Sprintf("Category: %s | Priority: %d | Tokens: %d", atom.Category, atom.Priority, atom.TokenCount))

	mandatoryStatus := ""
	if atom.IsMandatory {
		mandatoryStatus = m.styles.Error.Render("MANDATORY (Skeleton)")
	} else {
		mandatoryStatus = m.styles.Success.Render("OPTIONAL (Flesh)")
	}

	separator := mutedStyle.Render("--- Content ---")

	capacity := len(header) + len(info) + len(mandatoryStatus) + len(separator) + len(atom.Content) + 4 // 4 for newlines

	var b strings.Builder
	b.Grow(capacity)

	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(info)
	b.WriteString("\n")
	b.WriteString(mandatoryStatus)
	b.WriteString("\n")
	b.WriteString(separator)
	b.WriteString("\n")
	b.WriteString(atom.Content)

	return b.String()
}

// View renders the page.
// TODO: IMPROVEMENT: Abstract split view logic into a shared helper or component to ensure consistency across pages.
func (m JITPageModel) View() string {
	if m.lastResult == nil {
		return m.styles.Content.Render("No JIT compilation result available yet.")
	}

	// Filter bar
	var sb strings.Builder
	filterStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Outline).
		Padding(0, 1)

	if m.searchFocused {
		filterStyle = filterStyle.BorderForeground(m.styles.Theme.Primary)
	}

	sb.WriteString(filterStyle.Render(m.searchInput.View()))
	sb.WriteString("  ")
	sb.WriteString(m.styles.Muted.Render("[/] Filter  [Tab] Focus"))
	sb.WriteString("\n\n")

	// Split view: List (35%) | Viewport (65%)
	// Note: Widths are calculated in SetSize for the inner components.
	// But we need to render the containers here.

	// Re-calculate pane widths (outer widths)
	totalWidth := m.width
	listPaneWidth := int(float64(totalWidth) * 0.35)
	viewPaneWidth := totalWidth - listPaneWidth

	// Define base styles with border
	baseStyle := m.styles.Content.Copy().
		Padding(0, 1). // Reduced padding to accommodate border
		Border(lipgloss.RoundedBorder())

	// Focus styles
	focusedBorder := m.styles.Theme.Secondary
	blurredBorder := m.styles.Theme.OnSurfaceMuted

	var listStyle, viewStyle lipgloss.Style
	if !m.focusViewport && !m.searchFocused {
		listStyle = baseStyle.BorderForeground(focusedBorder)
		viewStyle = baseStyle.BorderForeground(blurredBorder)
	} else if m.focusViewport && !m.searchFocused {
		listStyle = baseStyle.BorderForeground(blurredBorder)
		viewStyle = baseStyle.BorderForeground(focusedBorder)
	} else {
		listStyle = baseStyle.BorderForeground(blurredBorder)
		viewStyle = baseStyle.BorderForeground(blurredBorder)
	}

	// Render panes
	// We force the width on the style to ensure layout consistency
	listView := listStyle.Width(listPaneWidth - 4).Render(m.list.View())
	contentView := viewStyle.Width(viewPaneWidth - 4).Render(m.viewport.View())

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)

	help := m.styles.Muted.Render(" • c/y: copy atom • p: copy full prompt • tab: focus switch • /: filter")

	sb.WriteString(mainView)
	sb.WriteString("\n")
	sb.WriteString(help)

	return sb.String()
}

// SetSize updates the size.
func (m *JITPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	// Chrome: Border(2) + Padding(2) = 4 width per pane
	chromeW := 4
	// Vertical: Border(2) + Padding(0) = 2 height
	chromeH := 2

	// Search bar height: Box(3) + Margin(2) = 5
	searchBarH := 5

	paneH := h - 3 - chromeH - searchBarH // Footer(1+margin) - VerticalChrome - SearchBar

	listPaneWidth := int(float64(w) * 0.35)
	viewPaneWidth := w - listPaneWidth

	// Inner sizes
	m.list.SetSize(listPaneWidth-chromeW, paneH)
	m.viewport.Width = viewPaneWidth - chromeW
	m.viewport.Height = paneH
}

// UpdateContent updates the data from the JIT compiler.
func (m *JITPageModel) UpdateContent(result *prompt.CompilationResult) {
	if result == nil {
		return
	}
	m.lastResult = result

	// Sort by priority desc
	sort.Slice(result.IncludedAtoms, func(i, j int) bool {
		return result.IncludedAtoms[i].Priority > result.IncludedAtoms[j].Priority
	})

	m.applySearch()

	// Set stats in title
	stats := fmt.Sprintf("JIT Inspector (%d atoms, %d tokens, %.0f%% budget)",
		len(result.IncludedAtoms), result.TotalTokens, result.BudgetUsed*100)
	m.list.Title = stats
}

// applySearch filters the included atoms based on the search term
func (m *JITPageModel) applySearch() {
	if m.lastResult == nil {
		return
	}

	term := strings.ToLower(m.searchInput.Value())
	var filteredItems []list.Item

	for _, atom := range m.lastResult.IncludedAtoms {
		id := strings.ToLower(atom.ID)
		cat := strings.ToLower(string(atom.Category))
		content := strings.ToLower(atom.Content)

		if term == "" || strings.Contains(id, term) || strings.Contains(cat, term) || strings.Contains(content, term) {
			filteredItems = append(filteredItems, atomItem{atom: atom})
		}
	}

	m.list.SetItems(filteredItems)
}

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
type JITPageModel struct {
	width         int
	height        int
	list          list.Model
	viewport      viewport.Model
	focusViewport bool

	// Search/Filter state
	filterInput   textinput.Model
	filterFocused bool
	filteredAtoms []*prompt.PromptAtom

	// Data
	lastResult *prompt.CompilationResult
	selected   *prompt.PromptAtom

	// Styles
	styles Styles
}

// atomItem adapts prompt.PromptAtom to list.Item
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
	l.SetFilteringEnabled(false) // Custom filter used instead
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	fi := textinput.New()
	fi.Placeholder = "Filter atoms by content..."
	fi.CharLimit = 100
	fi.Width = 40

	return JITPageModel{
		list:          l,
		viewport:      vp,
		filterInput:   fi,
		filterFocused: false,
		styles:        DefaultStyles(),
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
			if !m.filterFocused {
				m.filterFocused = true
				m.filterInput.Focus()
				return m, nil
			}
		case "esc":
			if m.filterFocused {
				m.filterFocused = false
				m.filterInput.Blur()
				m.filterInput.SetValue("")
				m.applyFilter()
				return m, nil
			}
		case "enter":
			if m.filterFocused {
				m.filterFocused = false
				m.filterInput.Blur()
				return m, nil
			}
		case "tab":
			if !m.filterFocused {
				m.focusViewport = !m.focusViewport
				return m, nil
			}
		}

		if !m.filterFocused {
			switch msg.String() {
			case "c", "y":
				if m.selected != nil {
					if err := clipboardWriteAll(m.selected.Content); err != nil {
						cmd = m.list.NewStatusMessage(m.styles.Status.Error.Render("Failed to copy atom content"))
					} else {
						cmd = m.list.NewStatusMessage(m.styles.Status.Success.Render(fmt.Sprintf("Copied atom content for [%s] to clipboard", m.selected.ID)))
					}
					cmds = append(cmds, cmd)
				}
			case "p":
				if m.lastResult != nil {
					if err := clipboardWriteAll(m.lastResult.Prompt); err != nil {
						cmd = m.list.NewStatusMessage(m.styles.Status.Error.Render("Failed to copy full prompt"))
					} else {
						cmd = m.list.NewStatusMessage(m.styles.Status.Success.Render("Copied full prompt to clipboard"))
					}
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	if m.filterFocused {
		m.filterInput, cmd = m.filterInput.Update(msg)
		cmds = append(cmds, cmd)
		m.applyFilter()
	} else {
		_, isKey := msg.(tea.KeyMsg)
		updateList := !isKey || !m.focusViewport
		updateViewport := !isKey || m.focusViewport

		if updateList {
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		}

		if updateViewport {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if sel := m.list.SelectedItem(); sel != nil {
		item := sel.(atomItem)
		if m.selected == nil || m.selected.ID != item.atom.ID {
			m.selected = item.atom
			m.viewport.SetContent(m.renderAtomContent(item.atom))
		}
	} else {
		m.selected = nil
		m.viewport.SetContent("No atoms match filter.")
	}

	return m, tea.Batch(cmds...)
}

func (m *JITPageModel) applyFilter() {
	if m.lastResult == nil {
		return
	}

	filterText := strings.ToLower(m.filterInput.Value())
	m.filteredAtoms = make([]*prompt.PromptAtom, 0)

	for _, atom := range m.lastResult.IncludedAtoms {
		if filterText == "" ||
			strings.Contains(strings.ToLower(atom.ID), filterText) ||
			strings.Contains(strings.ToLower(string(atom.Category)), filterText) ||
			strings.Contains(strings.ToLower(atom.Content), filterText) {
			m.filteredAtoms = append(m.filteredAtoms, atom)
		}
	}

	items := make([]list.Item, 0, len(m.filteredAtoms))
	for _, atom := range m.filteredAtoms {
		items = append(items, atomItem{atom: atom})
	}
	m.list.SetItems(items)
}

// renderAtomContent formats the atom for display using strings.Builder
func (m JITPageModel) renderAtomContent(atom *prompt.PromptAtom) string {
	headerStyle := m.styles.Layout.Header
	infoStyle := m.styles.Status.Info
	mutedStyle := m.styles.Text.Muted

	header := headerStyle.Render(atom.ID)
	info := infoStyle.Render(fmt.Sprintf("Category: %s | Priority: %d | Tokens: %d", atom.Category, atom.Priority, atom.TokenCount))

	mandatoryStatus := ""
	if atom.IsMandatory {
		mandatoryStatus = m.styles.Status.Error.Render("MANDATORY (Skeleton)")
	} else {
		mandatoryStatus = m.styles.Status.Success.Render("OPTIONAL (Flesh)")
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
func (m JITPageModel) View() string {
	if m.lastResult == nil {
		return m.styles.Layout.Content.Render("No JIT compilation result available yet.")
	}

	var sb strings.Builder

	// Render filter bar
	filterStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Outline()).
		Padding(0, 1).
		MarginBottom(1)

	if m.filterFocused {
		filterStyle = filterStyle.BorderForeground(m.styles.Theme.Primary())
	}
	sb.WriteString(filterStyle.Render(m.filterInput.View()))
	sb.WriteString("\n")

	totalWidth := m.width
	listPaneWidth := int(float64(totalWidth) * 0.35)
	viewPaneWidth := totalWidth - listPaneWidth

	baseStyle := m.styles.Layout.Content.Copy().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder())

	focusedBorder := m.styles.Theme.Secondary()
	blurredBorder := m.styles.Theme.OnSurfaceMuted()

	var listStyle, viewStyle lipgloss.Style
	if !m.focusViewport && !m.filterFocused {
		listStyle = baseStyle.BorderForeground(focusedBorder)
		viewStyle = baseStyle.BorderForeground(blurredBorder)
	} else if m.focusViewport && !m.filterFocused {
		listStyle = baseStyle.BorderForeground(blurredBorder)
		viewStyle = baseStyle.BorderForeground(focusedBorder)
	} else {
		listStyle = baseStyle.BorderForeground(blurredBorder)
		viewStyle = baseStyle.BorderForeground(blurredBorder)
	}

	listView := listStyle.Width(listPaneWidth - 4).Render(m.list.View())
	contentView := viewStyle.Width(viewPaneWidth - 4).Render(m.viewport.View())

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)
	sb.WriteString(mainView)

	help := m.styles.Text.Muted.Render("\n • c/y: copy atom • p: copy full prompt • tab: focus switch • /: filter • esc: clear filter")
	sb.WriteString(help)

	return sb.String()
}

// SetSize updates the size.
func (m *JITPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	chromeW := 4
	chromeH := 2

	filterBarHeight := 4
	paneH := max(h-3-chromeH-filterBarHeight, 5)

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
	// Sort by priority desc before storing
	sort.Slice(result.IncludedAtoms, func(i, j int) bool {
		return result.IncludedAtoms[i].Priority > result.IncludedAtoms[j].Priority
	})

	m.lastResult = result
	m.applyFilter()

	// Set stats in title
	stats := fmt.Sprintf("JIT Inspector (%d atoms, %d tokens, %.0f%% budget)",
		len(result.IncludedAtoms), result.TotalTokens, result.BudgetUsed*100)
	m.list.Title = stats
}

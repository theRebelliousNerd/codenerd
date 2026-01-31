package ui

import (
	"codenerd/internal/prompt"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// JITPageModel defines the state of the JIT Prompt Inspector.
type JITPageModel struct {
	width    int
	height   int
	list     list.Model
	viewport viewport.Model

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
func (i atomItem) FilterValue() string { return i.atom.ID + " " + string(i.atom.Category) }

// NewJITPageModel creates a new JIT inspector page.
func NewJITPageModel() JITPageModel {
	vp := viewport.New(0, 0)
	vp.SetContent("Select an atom to view content.")

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Prompt Atoms"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	return JITPageModel{
		list:     l,
		viewport: vp,
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
		// Viewport navigation if list is not filtering
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "tab":
				// Could toggle focus, but for now just let logic handle it or simple split
				// TODO: Implement focus switching between list and viewport
			}
		}
	}

	// Update List
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Update Viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Check for selection change
	if sel := m.list.SelectedItem(); sel != nil {
		item := sel.(atomItem)
		if m.selected == nil || m.selected.ID != item.atom.ID {
			m.selected = item.atom
			m.viewport.SetContent(m.renderAtomContent(item.atom))
		}
	}

	return m, tea.Batch(cmds...)
}

// renderAtomContent formats the atom for display
// TODO: IMPROVEMENT: Fix indentation and improve string building performance.
// TODO: IMPROVEMENT: Implement syntax highlighting for atom content based on file type (e.g., Markdown, Mangle, Go).
func (m JITPageModel) renderAtomContent(atom *prompt.PromptAtom) string {
	var sb strings.Builder

	headerStyle := m.styles.Header
	infoStyle := m.styles.Info
	mutedStyle := m.styles.Muted

	sb.WriteString(headerStyle.Render(atom.ID) + "\n")
	sb.WriteString(infoStyle.Render(fmt.Sprintf("Category: %s | Priority: %d | Tokens: %d", atom.Category, atom.Priority, atom.TokenCount)) + "\n")
	
		
		if atom.IsMandatory {
			sb.WriteString(m.styles.Error.Render("MANDATORY (Skeleton)") + "\n")
		} else {
			sb.WriteString(m.styles.Success.Render("OPTIONAL (Flesh)") + "\n")
		}
		
	sb.WriteString(mutedStyle.Render("--- Content ---") + "\n")
		sb.WriteString(atom.Content + "\n")
	
		return sb.String()}

// View renders the page.
// TODO: IMPROVEMENT: Abstract split view logic into a shared helper or component to ensure consistency across pages.
func (m JITPageModel) View() string {
	if m.lastResult == nil {
		return m.styles.Content.Render("No JIT compilation result available yet.")
	}

	// Split view: List (30%) | Viewport (70%)
	listWidth := int(float64(m.width) * 0.35)
	viewWidth := m.width - listWidth - 4

	listView := m.styles.Content.Copy().Width(listWidth).Render(m.list.View())
	contentView := m.styles.Content.Copy().Width(viewWidth).Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)
}

// SetSize updates the size.
func (m *JITPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	
	listWidth := int(float64(w) * 0.35)
	m.list.SetSize(listWidth, h-2)
	m.viewport.Width = w - listWidth - 4
	m.viewport.Height = h - 2
}

// UpdateContent updates the data from the JIT compiler.
func (m *JITPageModel) UpdateContent(result *prompt.CompilationResult) {
	if result == nil {
		return
	}
	m.lastResult = result

	// Convert atoms to items
	items := make([]list.Item, 0, len(result.IncludedAtoms))
	
	// Sort by priority desc
	sort.Slice(result.IncludedAtoms, func(i, j int) bool {
		return result.IncludedAtoms[i].Priority > result.IncludedAtoms[j].Priority
	})

	for _, atom := range result.IncludedAtoms {
		items = append(items, atomItem{atom: atom})
	}

	m.list.SetItems(items)
	
	// Set stats in title
	stats := fmt.Sprintf("JIT Inspector (%d atoms, %d tokens, %.0f%% budget)", 
		len(result.IncludedAtoms), result.TotalTokens, result.BudgetUsed*100)
	m.list.Title = stats
}

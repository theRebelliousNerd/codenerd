package ui

import (
	"codenerd/internal/prompt"
	"fmt"
	"sort"
	"github.com/atotto/clipboard"

	"github.com/charmbracelet/bubbles/list"
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
func (i atomItem) FilterValue() string { return i.atom.ID + " " + string(i.atom.Category) + " " + i.atom.Content }

// NewJITPageModel creates a new JIT inspector page.
func NewJITPageModel() JITPageModel {
	vp := viewport.New(0, 0)
	vp.SetContent("Select an atom to view content.")

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Prompt Atoms"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
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

	// TODO: Add search bar to filter atoms by content, not just ID/Category.
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
			case "c", "y":
				if m.selected != nil {
					if err := clipboardWriteAll(m.selected.Content); err != nil {
						cmd = m.list.NewStatusMessage(m.styles.Error.Render("Failed to copy atom content"))
					} else {
						cmd = m.list.NewStatusMessage(m.styles.Success.Render(fmt.Sprintf("Copied atom content for [%s] to clipboard", m.selected.ID)))
					}
					cmds = append(cmds, cmd)
				}
			case "p":
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

// renderAtomContent formats the atom for display using lipgloss.JoinVertical
// TODO: IMPROVEMENT: Implement syntax highlighting for atom content based on file type (e.g., Markdown, Mangle, Go).
func (m JITPageModel) renderAtomContent(atom *prompt.PromptAtom) string {
	// TODO: Consider using strings.Builder or a more efficient rendering method for large content.
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

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		info,
		mandatoryStatus,
		separator,
		atom.Content,
	)
}

// View renders the page.
// TODO: IMPROVEMENT: Abstract split view logic into a shared helper or component to ensure consistency across pages.
func (m JITPageModel) View() string {
	if m.lastResult == nil {
		return m.styles.Content.Render("No JIT compilation result available yet.")
	}

	// Split view: List (35%) | Viewport (65%)
	listWidth := int(float64(m.width) * 0.35)
	viewWidth := m.width - listWidth - 4

	listView := m.styles.Content.Copy().Width(listWidth).Render(m.list.View())
	contentView := m.styles.Content.Copy().Width(viewWidth).Render(m.viewport.View())

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)

	help := m.styles.Muted.Render(" • c/y: copy atom • p: copy full prompt • tab: focus viewport • /: filter")

	return lipgloss.JoinVertical(lipgloss.Left, mainView, help)
}

// SetSize updates the size.
func (m *JITPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	listWidth := int(float64(w) * 0.35)
	m.list.SetSize(listWidth, h-3) // Reserve space for footer
	m.viewport.Width = w - listWidth - 4
	m.viewport.Height = h - 3
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

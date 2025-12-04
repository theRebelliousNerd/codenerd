// Package ui provides the Interactive Diff Approval component.
// This allows users to review proposed code changes before they're applied.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// DiffLine represents a single line in the diff
type DiffLine struct {
	LineNum int
	Content string
	Type    DiffLineType
}

// DiffLineType represents the type of diff line
type DiffLineType int

const (
	DiffLineContext  DiffLineType = iota // Unchanged context line
	DiffLineAdded                        // Added line
	DiffLineRemoved                      // Removed line
	DiffLineHeader                       // Diff header line
)

// DiffHunk represents a group of changes
type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// FileDiff represents changes to a single file
type FileDiff struct {
	OldPath  string
	NewPath  string
	Hunks    []DiffHunk
	IsNew    bool
	IsDelete bool
	IsBinary bool
}

// PendingMutation represents a mutation awaiting approval
type PendingMutation struct {
	ID          string
	Description string
	FilePath    string
	Diff        *FileDiff
	Reason      string    // Why approval is needed
	Warnings    []string  // Safety warnings
	Approved    bool
	Rejected    bool
	Comment     string    // User's comment
}

// DiffApprovalView handles interactive diff approval
type DiffApprovalView struct {
	Styles         Styles
	Viewport       viewport.Model
	Mutations      []*PendingMutation
	CurrentIndex   int
	Width          int
	Height         int
	ShowWarnings   bool
	SelectedHunk   int
	ApprovalMode   ApprovalMode
}

// ApprovalMode represents the current approval state
type ApprovalMode int

const (
	ModeReview ApprovalMode = iota
	ModeApproved
	ModeRejected
	ModePending
)

// NewDiffApprovalView creates a new diff approval view
func NewDiffApprovalView(styles Styles, width, height int) DiffApprovalView {
	vp := viewport.New(width, height-6)
	vp.SetContent("")

	return DiffApprovalView{
		Styles:       styles,
		Viewport:     vp,
		Mutations:    make([]*PendingMutation, 0),
		CurrentIndex: 0,
		Width:        width,
		Height:       height,
		ShowWarnings: true,
		SelectedHunk: 0,
		ApprovalMode: ModeReview,
	}
}

// SetSize updates dimensions
func (d *DiffApprovalView) SetSize(width, height int) {
	d.Width = width
	d.Height = height
	d.Viewport.Width = width - 4
	d.Viewport.Height = height - 8
}

// AddMutation adds a pending mutation for approval
func (d *DiffApprovalView) AddMutation(m *PendingMutation) {
	d.Mutations = append(d.Mutations, m)
	d.updateContent()
}

// ClearMutations removes all pending mutations
func (d *DiffApprovalView) ClearMutations() {
	d.Mutations = make([]*PendingMutation, 0)
	d.CurrentIndex = 0
	d.updateContent()
}

// NextMutation moves to the next mutation
func (d *DiffApprovalView) NextMutation() {
	if d.CurrentIndex < len(d.Mutations)-1 {
		d.CurrentIndex++
		d.SelectedHunk = 0
		d.updateContent()
	}
}

// PrevMutation moves to the previous mutation
func (d *DiffApprovalView) PrevMutation() {
	if d.CurrentIndex > 0 {
		d.CurrentIndex--
		d.SelectedHunk = 0
		d.updateContent()
	}
}

// NextHunk moves to the next hunk in the current diff
func (d *DiffApprovalView) NextHunk() {
	if d.CurrentIndex < len(d.Mutations) {
		m := d.Mutations[d.CurrentIndex]
		if m.Diff != nil && d.SelectedHunk < len(m.Diff.Hunks)-1 {
			d.SelectedHunk++
			d.updateContent()
		}
	}
}

// PrevHunk moves to the previous hunk
func (d *DiffApprovalView) PrevHunk() {
	if d.SelectedHunk > 0 {
		d.SelectedHunk--
		d.updateContent()
	}
}

// ApproveCurrent approves the current mutation
func (d *DiffApprovalView) ApproveCurrent() bool {
	if d.CurrentIndex < len(d.Mutations) {
		d.Mutations[d.CurrentIndex].Approved = true
		d.Mutations[d.CurrentIndex].Rejected = false
		d.ApprovalMode = ModeApproved
		d.updateContent()
		return true
	}
	return false
}

// RejectCurrent rejects the current mutation
func (d *DiffApprovalView) RejectCurrent(comment string) bool {
	if d.CurrentIndex < len(d.Mutations) {
		d.Mutations[d.CurrentIndex].Approved = false
		d.Mutations[d.CurrentIndex].Rejected = true
		d.Mutations[d.CurrentIndex].Comment = comment
		d.ApprovalMode = ModeRejected
		d.updateContent()
		return true
	}
	return false
}

// ApproveAll approves all pending mutations
func (d *DiffApprovalView) ApproveAll() {
	for _, m := range d.Mutations {
		m.Approved = true
		m.Rejected = false
	}
	d.updateContent()
}

// GetApprovedMutations returns all approved mutations
func (d *DiffApprovalView) GetApprovedMutations() []*PendingMutation {
	approved := make([]*PendingMutation, 0)
	for _, m := range d.Mutations {
		if m.Approved {
			approved = append(approved, m)
		}
	}
	return approved
}

// GetPendingCount returns the number of unapproved mutations
func (d *DiffApprovalView) GetPendingCount() int {
	count := 0
	for _, m := range d.Mutations {
		if !m.Approved && !m.Rejected {
			count++
		}
	}
	return count
}

// HasPending returns true if there are mutations awaiting approval
func (d *DiffApprovalView) HasPending() bool {
	return d.GetPendingCount() > 0
}

// ToggleWarnings toggles warning display
func (d *DiffApprovalView) ToggleWarnings() {
	d.ShowWarnings = !d.ShowWarnings
	d.updateContent()
}

// updateContent refreshes the viewport content
func (d *DiffApprovalView) updateContent() {
	if len(d.Mutations) == 0 {
		d.Viewport.SetContent(d.renderEmpty())
		return
	}
	d.Viewport.SetContent(d.renderCurrentMutation())
}

// renderEmpty renders the empty state
func (d *DiffApprovalView) renderEmpty() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(d.Styles.Theme.Muted).
		Italic(true).
		Padding(2).
		Width(d.Width - 4).
		Align(lipgloss.Center)

	return emptyStyle.Render("No pending mutations to review.")
}

// renderCurrentMutation renders the current mutation diff
func (d *DiffApprovalView) renderCurrentMutation() string {
	if d.CurrentIndex >= len(d.Mutations) {
		return d.renderEmpty()
	}

	m := d.Mutations[d.CurrentIndex]
	var sb strings.Builder

	// Header
	sb.WriteString(d.renderHeader(m))
	sb.WriteString("\n\n")

	// Warnings (if any)
	if d.ShowWarnings && len(m.Warnings) > 0 {
		sb.WriteString(d.renderWarnings(m.Warnings))
		sb.WriteString("\n")
	}

	// Diff content
	if m.Diff != nil {
		sb.WriteString(d.renderDiff(m.Diff))
	} else {
		sb.WriteString(d.Styles.Muted.Render("(No diff available)"))
	}

	// Footer with controls
	sb.WriteString("\n\n")
	sb.WriteString(d.renderControls())

	return sb.String()
}

// renderHeader renders the mutation header
func (d *DiffApprovalView) renderHeader(m *PendingMutation) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(d.Styles.Theme.Primary).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(d.Styles.Theme.Border).
		Width(d.Width - 4).
		Padding(0, 1)

	// Status indicator
	status := "⏳ PENDING"
	statusColor := d.Styles.Theme.Muted
	if m.Approved {
		status = "✅ APPROVED"
		statusColor = Success
	} else if m.Rejected {
		status = "❌ REJECTED"
		statusColor = Destructive
	}

	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)

	header := fmt.Sprintf("📝 Mutation %d/%d: %s  %s",
		d.CurrentIndex+1,
		len(d.Mutations),
		m.Description,
		statusStyle.Render(status))

	subheader := fmt.Sprintf("File: %s\nReason: %s", m.FilePath, m.Reason)

	return headerStyle.Render(header) + "\n" + d.Styles.Muted.Render(subheader)
}

// renderWarnings renders safety warnings
func (d *DiffApprovalView) renderWarnings(warnings []string) string {
	warningStyle := lipgloss.NewStyle().
		Foreground(Warning).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Warning).
		Padding(0, 1).
		Width(d.Width - 8)

	var sb strings.Builder
	sb.WriteString("⚠️ Warnings:\n")
	for _, w := range warnings {
		sb.WriteString(fmt.Sprintf("  • %s\n", w))
	}

	return warningStyle.Render(sb.String())
}

// renderDiff renders the diff content with syntax highlighting
func (d *DiffApprovalView) renderDiff(diff *FileDiff) string {
	var sb strings.Builder

	// File header
	fileHeader := fmt.Sprintf("--- %s\n+++ %s", diff.OldPath, diff.NewPath)
	sb.WriteString(d.Styles.Muted.Render(fileHeader))
	sb.WriteString("\n\n")

	if diff.IsBinary {
		sb.WriteString(d.Styles.Warning.Render("Binary file - diff not shown"))
		return sb.String()
	}

	// Render each hunk
	for i, hunk := range diff.Hunks {
		// Hunk header
		hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
			hunk.OldStart, hunk.OldCount,
			hunk.NewStart, hunk.NewCount)

		hunkStyle := d.Styles.Muted
		if i == d.SelectedHunk {
			hunkStyle = lipgloss.NewStyle().
				Background(d.Styles.Theme.Secondary).
				Foreground(d.Styles.Theme.Primary)
		}
		sb.WriteString(hunkStyle.Render(hunkHeader))
		sb.WriteString("\n")

		// Render lines
		for _, line := range hunk.Lines {
			sb.WriteString(d.renderDiffLine(line))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderDiffLine renders a single diff line with appropriate styling
func (d *DiffApprovalView) renderDiffLine(line DiffLine) string {
	var style lipgloss.Style
	var prefix string

	switch line.Type {
	case DiffLineAdded:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22c55e")).
			Background(lipgloss.Color("#052e16"))
		prefix = "+ "
	case DiffLineRemoved:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ef4444")).
			Background(lipgloss.Color("#2d0a0a"))
		prefix = "- "
	case DiffLineContext:
		style = d.Styles.Body
		prefix = "  "
	case DiffLineHeader:
		style = d.Styles.Bold
		prefix = ""
	}

	return style.Render(fmt.Sprintf("%s%s", prefix, line.Content))
}

// renderControls renders the approval controls
func (d *DiffApprovalView) renderControls() string {
	controlStyle := lipgloss.NewStyle().
		Foreground(d.Styles.Theme.Muted).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.Styles.Theme.Border).
		Padding(0, 1).
		Width(d.Width - 4)

	controls := "Controls: [y] Approve  [n] Reject  [a] Approve All  [←/→] Prev/Next  [↑/↓] Prev/Next Hunk  [w] Toggle Warnings  [q] Close"

	return controlStyle.Render(controls)
}

// View returns the rendered view
func (d *DiffApprovalView) View() string {
	return d.Viewport.View()
}

// CreateDiffFromStrings creates a FileDiff from old and new content strings
func CreateDiffFromStrings(oldPath, newPath, oldContent, newContent string) *FileDiff {
	diff := &FileDiff{
		OldPath: oldPath,
		NewPath: newPath,
		Hunks:   make([]DiffHunk, 0),
	}

	if oldContent == "" {
		diff.IsNew = true
	}
	if newContent == "" {
		diff.IsDelete = true
	}

	// Simple line-by-line diff
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	hunk := DiffHunk{
		OldStart: 1,
		NewStart: 1,
		Lines:    make([]DiffLine, 0),
	}

	// Very simple diff algorithm - show all old as removed, all new as added
	// A real implementation would use LCS or Myers diff
	for i, line := range oldLines {
		hunk.Lines = append(hunk.Lines, DiffLine{
			LineNum: i + 1,
			Content: line,
			Type:    DiffLineRemoved,
		})
	}
	for i, line := range newLines {
		hunk.Lines = append(hunk.Lines, DiffLine{
			LineNum: i + 1,
			Content: line,
			Type:    DiffLineAdded,
		})
	}

	hunk.OldCount = len(oldLines)
	hunk.NewCount = len(newLines)

	diff.Hunks = append(diff.Hunks, hunk)

	return diff
}

// CreateMockMutation creates a sample mutation for testing
func CreateMockMutation(id, file, description string) *PendingMutation {
	return &PendingMutation{
		ID:          id,
		Description: description,
		FilePath:    file,
		Reason:      "Requires approval due to Chesterton's Fence warning",
		Warnings: []string{
			"File was recently modified by another developer",
			"5 other files depend on this code",
		},
		Diff: CreateDiffFromStrings(
			file, file,
			"func oldFunction() {\n    // Old implementation\n    return nil\n}",
			"func newFunction() {\n    // New implementation with improvements\n    return result\n}",
		),
	}
}

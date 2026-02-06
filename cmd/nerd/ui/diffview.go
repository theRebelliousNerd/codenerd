// Package ui provides the Interactive Diff Approval component.
// This allows users to review proposed code changes before they're applied.
package ui

import (
	"fmt"
	"strings"

	"codenerd/internal/diff"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// Type aliases for backward compatibility with UI code
type (
	DiffLine     = diff.Line
	DiffLineType = diff.LineType
	DiffHunk     = diff.Hunk
	FileDiff     = diff.FileDiff
)

// Constants for diff line types
const (
	DiffLineContext = diff.LineContext
	DiffLineAdded   = diff.LineAdded
	DiffLineRemoved = diff.LineRemoved
	DiffLineHeader  = diff.LineHeader
)

// PendingMutation represents a mutation awaiting approval
type PendingMutation struct {
	ID          string
	Description string
	FilePath    string
	Diff        *FileDiff
	Reason      string   // Why approval is needed
	Warnings    []string // Safety warnings
	Approved    bool
	Rejected    bool
	Comment     string // User's comment
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
	WordLevelDiff  bool // Enable word-level diffing for changed lines
	diffEngine     *diff.Engine
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
	vp := viewport.New(ViewportWidth(width), ViewportHeight(height))
	vp.SetContent("")

	return DiffApprovalView{
		Styles:        styles,
		Viewport:      vp,
		Mutations:     make([]*PendingMutation, 0),
		CurrentIndex:  0,
		Width:         width,
		Height:        height,
		ShowWarnings:  true,
		SelectedHunk:  0,
		ApprovalMode:  ModeReview,
		WordLevelDiff: true, // Enable word-level diffing by default
		diffEngine:    diff.NewEngine(),
	}
}

// SetSize updates dimensions using layout constants
func (d *DiffApprovalView) SetSize(width, height int) {
	d.Width = width
	d.Height = height
	d.Viewport.Width = ViewportWidth(width)
	d.Viewport.Height = ViewportHeight(height)
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
	if len(d.Mutations) == 0 || d.CurrentIndex >= len(d.Mutations) {
		return
	}
	m := d.Mutations[d.CurrentIndex]
	if m.Diff != nil && d.SelectedHunk < len(m.Diff.Hunks)-1 {
		d.SelectedHunk++
		d.updateContent()
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

// ToggleWordLevelDiff toggles word-level diffing display
func (d *DiffApprovalView) ToggleWordLevelDiff() {
	d.WordLevelDiff = !d.WordLevelDiff
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
		Width(ViewportWidth(d.Width)).
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
		Width(ViewportWidth(d.Width)).
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
		Width(WarningBoxWidth(d.Width))

	var sb strings.Builder
	sb.WriteString("⚠️ Warnings:\n")
	for _, w := range warnings {
		sb.WriteString(fmt.Sprintf("  • %s\n", w))
	}

	return warningStyle.Render(sb.String())
}

// renderDiff renders the diff content with word-level highlighting
// TODO: IMPROVEMENT: Optimize rendering by caching styles or using a renderer that doesn't recreate styles per line.
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

		// Render lines with word-level diffing for adjacent changed lines
		sb.WriteString(d.renderHunkLines(hunk.Lines))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderHunkLines renders hunk lines with word-level diffing support
func (d *DiffApprovalView) renderHunkLines(lines []DiffLine) string {
	var sb strings.Builder

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if word-level diff should be applied
		if d.WordLevelDiff && i+1 < len(lines) {
			nextLine := lines[i+1]

			// If we have a removed line followed by an added line, compute word diff
			if line.Type == DiffLineRemoved && nextLine.Type == DiffLineAdded {
				sb.WriteString(d.renderWordDiffPair(line, nextLine))
				sb.WriteString("\n")
				i++ // Skip the next line since we handled it
				continue
			}
		}

		// Regular line rendering
		sb.WriteString(d.renderDiffLine(line, nil))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderDiffLine renders a single diff line with appropriate styling
// wordDiffs is optional - if provided, word-level highlights will be applied
func (d *DiffApprovalView) renderDiffLine(line DiffLine, wordDiffs interface{}) string {
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

	// If word diffs provided, render with highlights (wordDiffs not used yet, placeholder)
	_ = wordDiffs
	return style.Render(fmt.Sprintf("%s%s", prefix, line.Content))
}

// renderWordDiffPair renders a removed/added line pair with word-level highlighting
func (d *DiffApprovalView) renderWordDiffPair(removed, added DiffLine) string {
	// Compute word-level diffs
	wordDiffs := d.diffEngine.ComputeWordLevelDiff(removed.Content, added.Content)

	var sb strings.Builder

	// Render removed line with highlights
	sb.WriteString(d.renderLineWithWordHighlights(removed, wordDiffs, true))
	sb.WriteString("\n")

	// Render added line with highlights
	sb.WriteString(d.renderLineWithWordHighlights(added, wordDiffs, false))

	return sb.String()
}

// renderLineWithWordHighlights renders a line with word-level change highlighting
func (d *DiffApprovalView) renderLineWithWordHighlights(line DiffLine, wordDiffs interface{}, isRemoved bool) string {
	// Import the diff package types
	// For now, we'll do basic rendering with full line styling
	// In a full implementation, we'd parse wordDiffs and apply different colors to changed words

	var baseStyle, highlightStyle lipgloss.Style
	var prefix string

	if isRemoved {
		// Removed line styles
		baseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ef4444")).
			Background(lipgloss.Color("#2d0a0a"))
		highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#991b1b")).
			Bold(true)
		prefix = "- "
	} else {
		// Added line styles
		baseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22c55e")).
			Background(lipgloss.Color("#052e16"))
		highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#166534")).
			Bold(true)
		prefix = "+ "
	}

	// For now, render with base style
	// Full word-diff highlighting would require parsing wordDiffs and applying highlightStyle
	// to specific character ranges
	_ = highlightStyle // Placeholder for future enhancement
	_ = wordDiffs

	return baseStyle.Render(fmt.Sprintf("%s%s", prefix, line.Content))
}

// renderControls renders the approval controls
// TODO: IMPROVEMENT: Use `key.Binding` for customizable keyboard controls instead of hardcoded strings.
// TODO: IMPROVEMENT: Use `bubbles/help` for the help view to ensure consistency.
func (d *DiffApprovalView) renderControls() string {
	controlStyle := lipgloss.NewStyle().
		Foreground(d.Styles.Theme.Muted).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.Styles.Theme.Border).
		Padding(0, 1).
		Width(ViewportWidth(d.Width))

	controls := "Controls: [y] Approve  [n] Reject  [a] Approve All  [←/→] Prev/Next  [↑/↓] Prev/Next Hunk  [w] Toggle Warnings  [d] Toggle Word Diff  [q] Close"

	return controlStyle.Render(controls)
}

// View returns the rendered view with horizontal scrolling support
func (d *DiffApprovalView) View() string {
	return d.Viewport.View()
}

// ScrollRight scrolls the viewport right for viewing long lines
func (d *DiffApprovalView) ScrollRight() {
	// TODO: FIX: bubbles/viewport v0.21.0 does not support LineRight/horizontal scrolling.
	// This code was causing build errors. Re-enable when bubbles is updated or alternative found.
	// d.Viewport.LineRight(3)
}

// ScrollLeft scrolls the viewport left
func (d *DiffApprovalView) ScrollLeft() {
	// TODO: FIX: bubbles/viewport v0.21.0 does not support LineLeft/horizontal scrolling.
	// d.Viewport.LineLeft(3)
}

// ScrollToStart scrolls to the beginning of lines
func (d *DiffApprovalView) ScrollToStart() {
	// TODO: FIX: bubbles/viewport v0.21.0 does not support GotoLeft.
	// d.Viewport.GotoLeft()
}

// CreateDiffFromStrings creates a FileDiff using the robust sergi/go-diff library
// with caching support for performance optimization.
func CreateDiffFromStrings(oldPath, newPath, oldContent, newContent string) *FileDiff {
	return diff.ComputeDiff(oldPath, newPath, oldContent, newContent)
}

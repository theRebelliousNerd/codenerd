// Package ui provides the Interactive Diff Approval component.
// This allows users to review proposed code changes before they're applied.
package ui

import (
	"fmt"
	"strings"

	"codenerd/internal/diff"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Type aliases for backward compatibility with UI code
var (
	diffAddedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22c55e")).
		Background(lipgloss.Color("#052e16"))

	diffRemovedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444")).
		Background(lipgloss.Color("#2d0a0a"))

	diffAddedHighlightStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#166534")).
		Bold(true)

	diffRemovedHighlightStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#991b1b")).
		Bold(true)
)

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

// DiffKeyMap defines the key bindings for the DiffApprovalView
type DiffKeyMap struct {
	Approve          key.Binding
	Reject           key.Binding
	ApproveAll       key.Binding
	NextMutation     key.Binding
	PrevMutation     key.Binding
	NextHunk         key.Binding
	PrevHunk         key.Binding
	ToggleWarnings   key.Binding
	ToggleWhitespace key.Binding
	ToggleWordDiff   key.Binding
	ScrollLeft       key.Binding
	ScrollRight      key.Binding
	ScrollToStart    key.Binding
	Quit             key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k DiffKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Approve, k.Reject, k.ApproveAll, k.NextMutation, k.PrevMutation, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
func (k DiffKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Approve, k.Reject, k.ApproveAll},
		{k.NextMutation, k.PrevMutation, k.NextHunk, k.PrevHunk},
		{k.ToggleWarnings, k.ToggleWhitespace, k.ToggleWordDiff},
		{k.ScrollLeft, k.ScrollRight, k.ScrollToStart, k.Quit},
	}
}

// DefaultDiffKeyMap returns the default keybindings.
func DefaultDiffKeyMap() DiffKeyMap {
	return DiffKeyMap{
		Approve:          key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "Approve")),
		Reject:           key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "Reject")),
		ApproveAll:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Approve All")),
		NextMutation:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "Next")),
		PrevMutation:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "Prev")),
		NextHunk:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "Next Hunk")),
		PrevHunk:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "Prev Hunk")),
		ToggleWarnings:   key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "Warnings")),
		ToggleWhitespace: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "Whitespace")),
		ToggleWordDiff:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "Word Diff")),
		ScrollLeft:       key.NewBinding(key.WithKeys("ctrl+left"), key.WithHelp("ctrl+←", "Scroll Left")),
		ScrollRight:      key.NewBinding(key.WithKeys("ctrl+right"), key.WithHelp("ctrl+→", "Scroll Right")),
		ScrollToStart:    key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "Scroll to Start")),
		Quit:             key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "Close")),
	}
}

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

type diffCachedStyles struct {
	selectedHunk lipgloss.Style
	warningBase  lipgloss.Style
	emptyBase    lipgloss.Style
	headerBase   lipgloss.Style
	controlBase  lipgloss.Style
}

// DiffApprovalView handles interactive diff approval
type DiffApprovalView struct {
	Styles       Styles
	Viewport     viewport.Model
	Mutations    []*PendingMutation
	CurrentIndex int
	Width        int
	Height       int
	ShowWarnings bool
	// IgnoreWhitespace hides whitespace-only diffs (useful for formatting-only changes).
	IgnoreWhitespace bool
	SelectedHunk     int
	ApprovalMode     ApprovalMode
	WordLevelDiff    bool // Enable word-level diffing for changed lines
	diffEngine       *diff.Engine
	XOffset          int // Horizontal scroll offset (columns)
	keys             DiffKeyMap
	help             help.Model
	cachedStyles     diffCachedStyles
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
// TODO: IMPROVEMENT: Implement a side-by-side diff view mode for better comparison of complex changes.
func NewDiffApprovalView(styles Styles, width, height int) DiffApprovalView {
	vp := viewport.New(ViewportWidth(width), ViewportHeight(height))
	vp.SetContent("")
	h := help.New()

	return DiffApprovalView{
		Styles:           styles,
		Viewport:         vp,
		Mutations:        make([]*PendingMutation, 0),
		CurrentIndex:     0,
		Width:            width,
		Height:           height,
		ShowWarnings:     true,
		IgnoreWhitespace: false,
		SelectedHunk:     0,
		ApprovalMode:     ModeReview,
		WordLevelDiff:    true, // Enable word-level diffing by default
		diffEngine:       uiDiffEngine,
		keys:             DefaultDiffKeyMap(),
		help:             h,
		cachedStyles: diffCachedStyles{
			selectedHunk: lipgloss.NewStyle().
				Background(styles.Theme.Container).
				Foreground(styles.Theme.OnContainer),
			warningBase: lipgloss.NewStyle().
				Foreground(styles.Theme.Warning).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(styles.Theme.Warning).
				Padding(0, 1),
			emptyBase: lipgloss.NewStyle().
				Foreground(styles.Theme.OnSurfaceMuted).
				Italic(true).
				Padding(2).
				Align(lipgloss.Center),
			headerBase: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.Theme.Primary).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(styles.Theme.Outline).
				Padding(0, 1),
			controlBase: lipgloss.NewStyle().
				Foreground(styles.Theme.OnSurfaceMuted).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(styles.Theme.Outline).
				Padding(0, 1),
		},
	}
}

// SetSize updates dimensions using layout constants
func (d *DiffApprovalView) SetSize(width, height int) {
	d.Width = width
	d.Height = height
	d.Viewport.Width = ViewportWidth(width)
	d.Viewport.Height = ViewportHeight(height)
}

// Init initializes the component
func (d *DiffApprovalView) Init() tea.Cmd {
	return nil
}

// Update handles messages and state transitions
func (d *DiffApprovalView) Update(msg tea.Msg) (DiffApprovalView, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, d.keys.Approve):
			d.ApproveCurrent()
		case key.Matches(msg, d.keys.Reject):
			d.RejectCurrent("")
		case key.Matches(msg, d.keys.ApproveAll):
			d.ApproveAll()
		case key.Matches(msg, d.keys.NextMutation):
			d.NextMutation()
		case key.Matches(msg, d.keys.PrevMutation):
			d.PrevMutation()
		case key.Matches(msg, d.keys.NextHunk):
			d.NextHunk()
		case key.Matches(msg, d.keys.PrevHunk):
			d.PrevHunk()
		case key.Matches(msg, d.keys.ToggleWarnings):
			d.ToggleWarnings()
		case key.Matches(msg, d.keys.ToggleWhitespace):
			d.ToggleIgnoreWhitespace()
		case key.Matches(msg, d.keys.ToggleWordDiff):
			d.ToggleWordLevelDiff()
		case key.Matches(msg, d.keys.ScrollLeft):
			d.ScrollLeft()
		case key.Matches(msg, d.keys.ScrollRight):
			d.ScrollRight()
		case key.Matches(msg, d.keys.ScrollToStart):
			d.ScrollToStart()
		}
	}

	d.Viewport, cmd = d.Viewport.Update(msg)
	cmds = append(cmds, cmd)

	return *d, tea.Batch(cmds...)
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

// ToggleIgnoreWhitespace toggles whitespace-only change filtering.
func (d *DiffApprovalView) ToggleIgnoreWhitespace() {
	d.IgnoreWhitespace = !d.IgnoreWhitespace
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
	emptyStyle := d.cachedStyles.emptyBase.Width(ViewportWidth(d.Width))

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
	headerStyle := d.cachedStyles.headerBase.Width(ViewportWidth(d.Width))

	// Status indicator
	status := "⏳ PENDING"
	statusColor := d.Styles.Theme.OnSurfaceMuted
	if m.Approved {
		status = "✅ APPROVED"
		statusColor = d.Styles.Theme.Success
	} else if m.Rejected {
		status = "❌ REJECTED"
		statusColor = d.Styles.Theme.Destructive
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
	warningStyle := d.cachedStyles.warningBase.Width(WarningBoxWidth(d.Width))

	var sb strings.Builder
	sb.WriteString("⚠️ Warnings:\n")
	for _, w := range warnings {
		sb.WriteString(fmt.Sprintf("  • %s\n", w))
	}

	return warningStyle.Render(sb.String())
}

// renderDiff renders the diff content with word-level highlighting
func (d *DiffApprovalView) renderDiff(diff *FileDiff) string {
	var sb strings.Builder

	// File header
	fileHeader := fmt.Sprintf("--- %s\n+++ %s", diff.OldPath, diff.NewPath)
	sb.WriteString(d.Styles.Muted.Render(fileHeader))
	sb.WriteString("\n\n")

	// Show whitespace mode indicator
	if d.IgnoreWhitespace {
		sb.WriteString(d.Styles.Info.Render("(Ignoring whitespace changes)"))
		sb.WriteString("\n\n")
	}

	if diff.IsBinary {
		sb.WriteString(d.Styles.Warning.Render("Binary file - diff not shown"))
		return sb.String()
	}

	// Render each hunk (with whitespace filtering if enabled)
	for i, hunk := range diff.Hunks {
		filteredLines := d.filterHunkLines(hunk.Lines)

		// Skip hunks that become empty after whitespace filtering
		if d.IgnoreWhitespace && len(filteredLines) == 0 {
			continue
		}

		// Hunk header
		hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
			hunk.OldStart, hunk.OldCount,
			hunk.NewStart, hunk.NewCount)

		hunkStyle := d.Styles.Muted
		if i == d.SelectedHunk {
			hunkStyle = d.cachedStyles.selectedHunk
		}
		sb.WriteString(hunkStyle.Render(hunkHeader))
		sb.WriteString("\n")

		// Render lines with word-level diffing for adjacent changed lines
		sb.WriteString(d.renderHunkLines(filteredLines))
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
		sb.WriteString(d.renderDiffLine(line))
		sb.WriteString("\n")
	}

	return sb.String()
}

// filterHunkLines filters lines based on whitespace settings
func (d *DiffApprovalView) filterHunkLines(lines []DiffLine) []DiffLine {
	if !d.IgnoreWhitespace {
		return lines
	}

	// When ignoring whitespace, we need to identify whitespace-only changes
	// and convert them to context lines or skip them
	filtered := make([]DiffLine, 0, len(lines))

	for i := range lines {
		line := lines[i]

		// Always keep context lines
		if line.Type == DiffLineContext || line.Type == DiffLineHeader {
			filtered = append(filtered, line)
			continue
		}

		// For add/remove lines, check if there's a corresponding change that's whitespace-only
		if line.Type == DiffLineRemoved {
			// Look ahead for a corresponding added line
			foundWhitespaceMatch := false
			for j := i + 1; j < len(lines) && j < i+5; j++ { // Look ahead up to 5 lines
				if lines[j].Type == DiffLineAdded {
					if isWhitespaceOnlyChange(line.Content, lines[j].Content) {
						// This is a whitespace-only change, convert to context
						contextLine := DiffLine{
							LineNum: line.LineNum,
							Content: line.Content,
							Type:    DiffLineContext,
						}
						filtered = append(filtered, contextLine)
						foundWhitespaceMatch = true
						break
					}
				} else if lines[j].Type == DiffLineRemoved {
					// Another removal, can't be a whitespace match
					break
				}
			}
			if !foundWhitespaceMatch {
				filtered = append(filtered, line)
			}
		} else if line.Type == DiffLineAdded {
			// Look back for a corresponding removed line that was whitespace-only
			foundWhitespaceMatch := false
			for j := i - 1; j >= 0 && j > i-5; j-- {
				if lines[j].Type == DiffLineRemoved {
					if isWhitespaceOnlyChange(lines[j].Content, line.Content) {
						// Already handled by the removed line logic, skip this added line
						foundWhitespaceMatch = true
						break
					}
				} else if lines[j].Type == DiffLineAdded {
					break
				}
			}
			if !foundWhitespaceMatch {
				filtered = append(filtered, line)
			}
		}
	}

	return filtered
}

func isWhitespaceOnlyChange(a, b string) bool {
	normalize := func(s string) string {
		var sb strings.Builder
		sb.Grow(len(s))
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case ' ', '\t', '\n', '\r':
				continue
			default:
				sb.WriteByte(s[i])
			}
		}
		return sb.String()
	}

	return normalize(a) == normalize(b)
}

// renderDiffLine renders a single diff line with appropriate styling.
// Word-level highlighting is handled by renderWordDiffPair, which is the only
// place a line has a counterpart to be compared against.
func (d *DiffApprovalView) renderDiffLine(line DiffLine) string {
	var style lipgloss.Style
	var prefix string

	switch line.Type {
	case DiffLineAdded:
		style = diffAddedStyle
		prefix = "+ "
	case DiffLineRemoved:
		style = diffRemovedStyle
		prefix = "- "
	case DiffLineContext:
		style = d.Styles.Body
		prefix = "  "
	case DiffLineHeader:
		style = d.Styles.Bold
		prefix = ""
	}

	fullLine := fmt.Sprintf("%s%s", prefix, line.Content)
	slicedLine := sliceString(fullLine, d.XOffset, d.Viewport.Width)
	return style.Render(slicedLine)
}

// renderWordDiffPair renders a removed/added line pair with word-level
// highlighting: the runs unique to each side are painted in a stronger colour
// so the eye lands on what actually changed rather than on two whole red/green
// lines that differ by one identifier.
func (d *DiffApprovalView) renderWordDiffPair(removed, added DiffLine) string {
	spans := d.diffEngine.ComputeWordLevelDiff(removed.Content, added.Content)

	var sb strings.Builder
	sb.WriteString(d.renderLineWithWordHighlights(removed, spans, true))
	sb.WriteString("\n")
	sb.WriteString(d.renderLineWithWordHighlights(added, spans, false))
	return sb.String()
}

// styledSegment is a run of a rendered line that shares one style.
type styledSegment struct {
	text      string
	highlight bool
}

// renderLineWithWordHighlights paints one side of a changed line pair.
//
// The span slice covers both sides at once, so the removed line consumes
// SpanEqual + SpanDelete and the added line consumes SpanEqual + SpanInsert.
// When the spans do not reconstruct this line's content — the pair was filtered
// or rewritten between compute and render — it falls back to whole-line styling
// rather than displaying text the file does not contain.
func (d *DiffApprovalView) renderLineWithWordHighlights(line DiffLine, spans []diff.WordSpan, isRemoved bool) string {
	var baseStyle, highlightStyle lipgloss.Style
	var prefix string

	if isRemoved {
		baseStyle = diffRemovedStyle
		highlightStyle = diffRemovedHighlightStyle
		prefix = "- "
	} else {
		baseStyle = diffAddedStyle
		highlightStyle = diffAddedHighlightStyle
		prefix = "+ "
	}

	segments := wordDiffSegments(prefix, line.Content, spans, isRemoved)
	segments = sliceSegments(segments, d.XOffset, d.Viewport.Width)

	var sb strings.Builder
	for _, seg := range segments {
		if seg.highlight {
			sb.WriteString(highlightStyle.Render(seg.text))
			continue
		}
		sb.WriteString(baseStyle.Render(seg.text))
	}
	return sb.String()
}

// wordDiffSegments turns the shared span slice into this side's segments,
// prefixed by the +/- gutter. It returns a single unhighlighted segment when
// the spans do not reconstruct content exactly.
func wordDiffSegments(prefix, content string, spans []diff.WordSpan, isRemoved bool) []styledSegment {
	unique := diff.SpanInsert
	if isRemoved {
		unique = diff.SpanDelete
	}

	segments := []styledSegment{{text: prefix}}
	var rebuilt strings.Builder
	for _, span := range spans {
		if span.Type != diff.SpanEqual && span.Type != unique {
			continue
		}
		rebuilt.WriteString(span.Text)
		segments = append(segments, styledSegment{
			text:      span.Text,
			highlight: span.Type == unique,
		})
	}

	if rebuilt.String() != content {
		return []styledSegment{{text: prefix + content}}
	}
	return segments
}

// sliceSegments applies the horizontal scroll window across styled segments,
// using the same column arithmetic as sliceString so a highlighted line scrolls
// in step with the plain ones around it.
func sliceSegments(segments []styledSegment, startCol, maxCols int) []styledSegment {
	if startCol < 0 {
		startCol = 0
	}
	if maxCols <= 0 {
		return nil
	}

	var currentWidth, outputWidth int
	out := make([]styledSegment, 0, len(segments))

	for _, seg := range segments {
		var sb strings.Builder
		for _, r := range seg.text {
			w := runewidth.RuneWidth(r)
			if currentWidth >= startCol {
				if outputWidth+w > maxCols {
					if sb.Len() > 0 {
						out = append(out, styledSegment{text: sb.String(), highlight: seg.highlight})
					}
					return out
				}
				sb.WriteRune(r)
				outputWidth += w
			}
			currentWidth += w
		}
		if sb.Len() > 0 {
			out = append(out, styledSegment{text: sb.String(), highlight: seg.highlight})
		}
	}
	return out
}

// renderControls renders the approval controls
func (d *DiffApprovalView) renderControls() string {
	controlStyle := d.cachedStyles.controlBase.Width(ViewportWidth(d.Width))

	wsStatus := "OFF"
	if d.IgnoreWhitespace {
		wsStatus = "ON"
	}

	// Prepend whitespace status to the help view dynamically, or you can just show it.
	helpView := d.help.View(d.keys)
	controls := fmt.Sprintf("Whitespace: %s | %s", wsStatus, helpView)

	return controlStyle.Render(controls)
}

// View returns the rendered view with horizontal scrolling support
func (d *DiffApprovalView) View() string {
	return d.Viewport.View()
}

// ScrollRight scrolls the viewport right for viewing long lines
func (d *DiffApprovalView) ScrollRight() {
	d.XOffset += 4
	y := d.Viewport.YPosition
	d.updateContent()
	d.Viewport.YPosition = y
}

// ScrollLeft scrolls the viewport left
func (d *DiffApprovalView) ScrollLeft() {
	if d.XOffset > 0 {
		d.XOffset -= 4
		if d.XOffset < 0 {
			d.XOffset = 0
		}
		y := d.Viewport.YPosition
		d.updateContent()
		d.Viewport.YPosition = y
	}
}

// ScrollToStart scrolls to the beginning of lines
func (d *DiffApprovalView) ScrollToStart() {
	d.XOffset = 0
	y := d.Viewport.YPosition
	d.updateContent()
	d.Viewport.YPosition = y
}

// sliceString returns a substring starting at startCol (column index) with maxCols width.
// It handles multi-byte characters and wide characters correctly using runewidth.
func sliceString(s string, startCol, maxCols int) string {
	if startCol < 0 {
		startCol = 0
	}
	if maxCols <= 0 {
		return ""
	}

	var currentWidth int
	var outputWidth int
	var sb strings.Builder
	runes := []rune(s)

	for _, r := range runes {
		w := runewidth.RuneWidth(r)

		if currentWidth >= startCol {
			if outputWidth+w > maxCols {
				break
			}
			sb.WriteRune(r)
			outputWidth += w
		}
		currentWidth += w
	}

	return sb.String()
}

// uiDiffEngine is the one diff engine the UI package uses.
//
// CreateDiffFromStrings used to call diff.ComputeDiff, which runs on the
// package-global DefaultEngine, while each DiffApprovalView built a private
// engine for its word-level path. The same file content was therefore diffed
// and cached twice, in two caches with different lifetimes: clearing one left
// the other stale, and the engine Stats a caller read described whichever
// engine they happened to hold rather than the work the UI actually did. One
// engine for the package removes the surprise; it is safe to share because
// Engine is mutex-guarded internally.
var uiDiffEngine = diff.NewEngine()

// CreateDiffFromStrings computes a file diff on the UI's engine.
func CreateDiffFromStrings(oldPath, newPath, oldContent, newContent string) *FileDiff {
	return uiDiffEngine.ComputeDiff(oldPath, newPath, oldContent, newContent)
}

// DiffEngineStats reports the UI diff engine's cumulative cache counters.
func DiffEngineStats() diff.Stats {
	return uiDiffEngine.Stats()
}

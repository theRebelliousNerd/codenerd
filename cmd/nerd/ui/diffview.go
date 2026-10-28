// Package ui provides the Interactive Diff Approval component.
//
// The view renders approval requests the host (internal/approval) froze and
// handed it, and turns the person's keys into decisions for the host to
// commit. It is never an authority: it holds immutable request snapshots, it
// never recomputes a decision preview, and it changes nothing by itself --
// every verdict leaves as an ApprovalDecisionMsg naming the request ID,
// version and digest that were on screen.
package ui

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/approval"
	"codenerd/internal/diff"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
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
	ToggleSideBySide key.Binding
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
		{k.ToggleWarnings, k.ToggleWhitespace, k.ToggleWordDiff, k.ToggleSideBySide},
		{k.ScrollLeft, k.ScrollRight, k.ScrollToStart, k.Quit},
	}
}

// DefaultDiffKeyMap returns the default keybindings.
func DefaultDiffKeyMap() DiffKeyMap {
	return DiffKeyMap{
		Approve:          key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "Approve")),
		Reject:           key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "Reject")),
		ApproveAll:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Approve all viewed")),
		NextMutation:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "Next")),
		PrevMutation:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "Prev")),
		NextHunk:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "Next Hunk")),
		PrevHunk:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "Prev Hunk")),
		ToggleWarnings:   key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "Warnings")),
		ToggleWhitespace: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "Whitespace")),
		ToggleWordDiff:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "Word Diff")),
		ToggleSideBySide: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "Side-by-Side")),
		ScrollLeft:       key.NewBinding(key.WithKeys("ctrl+left"), key.WithHelp("ctrl+←", "Scroll Left")),
		ScrollRight:      key.NewBinding(key.WithKeys("ctrl+right"), key.WithHelp("ctrl+→", "Scroll Right")),
		ScrollToStart:    key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "Scroll to Start")),
		Quit:             key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "Close")),
	}
}

// PendingMutation is one approval request as this view shows it. Request is
// the host's immutable snapshot; the rest is view-local state that decides
// nothing.
type PendingMutation struct {
	Request approval.Request
	// viewedVersion and viewedDigest are the request as it was the last time
	// it was the item on screen. Bulk approval covers only requests the
	// person has seen, at the version and digest they saw.
	viewedVersion int
	viewedDigest  string
	// Submitted is the verdict this view last sent for the request. It is a
	// display marker: the broker commits or refuses the decision.
	Submitted approval.Verdict
}

// NewPendingMutation wraps a host request snapshot for display.
func NewPendingMutation(r approval.Request) *PendingMutation {
	return &PendingMutation{Request: r}
}

// Viewed reports whether the person has seen the request at its current
// version and digest.
func (m *PendingMutation) Viewed() bool {
	return m.viewedDigest != "" && m.viewedDigest == m.Request.Digest && m.viewedVersion == m.Request.Version
}

// ApprovalDecisionMsg carries the person's decisions to the host, which
// alone commits them (approval.Host.Decide).
type ApprovalDecisionMsg struct {
	Decisions []approval.DecisionInput
}

func decisionCmd(decisions ...approval.DecisionInput) tea.Cmd {
	return func() tea.Msg { return ApprovalDecisionMsg{Decisions: decisions} }
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
	WordLevelDiff    bool // Enable word-level diffing for changed lines
	SideBySide       bool // Enable side-by-side diff view
	diffEngine       *diff.Engine
	XOffset          int // Horizontal scroll offset (columns)
	keys             DiffKeyMap
	help             help.Model
	cachedStyles     diffCachedStyles
}

// NewDiffApprovalView creates a new diff approval view
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
		WordLevelDiff:    true, // Enable word-level diffing by default
		diffEngine:       uiDiffEngine,
		keys:             DefaultDiffKeyMap(),
		help:             h,
		cachedStyles: diffCachedStyles{
			selectedHunk: lipgloss.NewStyle().
				Background(styles.Theme.Container()).
				Foreground(styles.Theme.OnContainer()),
			warningBase: lipgloss.NewStyle().
				Foreground(styles.Theme.Warning()).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(styles.Theme.Warning()).
				Padding(0, 1),
			emptyBase: lipgloss.NewStyle().
				Foreground(styles.Theme.OnSurfaceMuted()).
				Italic(true).
				Padding(2).
				Align(lipgloss.Center),
			headerBase: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.Theme.Primary()).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(styles.Theme.Outline()).
				Padding(0, 1),
			controlBase: lipgloss.NewStyle().
				Foreground(styles.Theme.OnSurfaceMuted()).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(styles.Theme.Outline()).
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
			if in, ok := d.ApproveCurrent(); ok {
				cmds = append(cmds, decisionCmd(in))
			}
		case key.Matches(msg, d.keys.Reject):
			if in, ok := d.RejectCurrent(""); ok {
				cmds = append(cmds, decisionCmd(in))
			}
		case key.Matches(msg, d.keys.ApproveAll):
			if ins := d.ApproveAll(); len(ins) > 0 {
				cmds = append(cmds, decisionCmd(ins...))
			}
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
		case key.Matches(msg, d.keys.ToggleSideBySide):
			d.ToggleSideBySide()
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

// AddMutation adds a request to the end of the list. It is not viewed until
// it becomes the item on screen.
func (d *DiffApprovalView) AddMutation(m *PendingMutation) {
	d.Mutations = append(d.Mutations, m)
	d.updateContent()
}

// SyncRequests replaces the list with the host's pending requests, in the
// host's order. A request that keeps its ID, version and digest keeps its
// viewed and submitted markers; a revised one is new to the person and is
// shown, and must be seen, again.
func (d *DiffApprovalView) SyncRequests(reqs []approval.Request) {
	var currentID string
	if d.CurrentIndex < len(d.Mutations) {
		currentID = d.Mutations[d.CurrentIndex].Request.ID
	}
	prior := make(map[string]*PendingMutation, len(d.Mutations))
	for _, m := range d.Mutations {
		prior[m.Request.ID] = m
	}
	next := make([]*PendingMutation, 0, len(reqs))
	d.CurrentIndex = 0
	for _, r := range reqs {
		m := NewPendingMutation(r)
		if old, ok := prior[r.ID]; ok && old.Request.Digest == r.Digest && old.Request.Version == r.Version {
			m.viewedVersion, m.viewedDigest, m.Submitted = old.viewedVersion, old.viewedDigest, old.Submitted
		}
		if r.ID == currentID {
			d.CurrentIndex = len(next)
		}
		next = append(next, m)
	}
	d.Mutations = next
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
	if fd := m.Request.Preview.Diff; fd != nil && d.SelectedHunk < len(fd.Hunks)-1 {
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

// decide records a verdict on m as displayed and returns the decision input
// for the host. A request already answered from this view is not answered
// again: a second key press is not a second decision.
func (d *DiffApprovalView) decide(m *PendingMutation, v approval.Verdict, reason string) (approval.DecisionInput, bool) {
	if m.Submitted != "" || !m.Viewed() {
		return approval.DecisionInput{}, false
	}
	m.Submitted = v
	return approval.DecisionInput{
		RequestID:      m.Request.ID,
		RequestVersion: m.viewedVersion,
		RequestDigest:  m.viewedDigest,
		EventID:        uuid.NewString(),
		Verdict:        v,
		Reason:         reason,
	}, true
}

// ApproveCurrent returns the decision approving the request on screen.
func (d *DiffApprovalView) ApproveCurrent() (approval.DecisionInput, bool) {
	if d.CurrentIndex >= len(d.Mutations) {
		return approval.DecisionInput{}, false
	}
	in, ok := d.decide(d.Mutations[d.CurrentIndex], approval.VerdictApprove, "")
	d.updateContent()
	return in, ok
}

// RejectCurrent returns the decision rejecting the request on screen.
func (d *DiffApprovalView) RejectCurrent(reason string) (approval.DecisionInput, bool) {
	if d.CurrentIndex >= len(d.Mutations) {
		return approval.DecisionInput{}, false
	}
	in, ok := d.decide(d.Mutations[d.CurrentIndex], approval.VerdictReject, reason)
	d.updateContent()
	return in, ok
}

// ApproveAll approves exactly the displayed set: every request the person has
// seen at its current version and has not answered yet. A rejected request
// stays rejected, and a request added after the person looked, or never put
// on screen, stays pending. It approves no future request and changes no
// policy.
func (d *DiffApprovalView) ApproveAll() []approval.DecisionInput {
	var out []approval.DecisionInput
	for _, m := range d.Mutations {
		if in, ok := d.decide(m, approval.VerdictApprove, "approved with the viewed batch"); ok {
			out = append(out, in)
		}
	}
	d.updateContent()
	return out
}

// PendingCount returns how many requests have no answer from this view yet.
func (d *DiffApprovalView) PendingCount() int {
	count := 0
	for _, m := range d.Mutations {
		if m.Submitted == "" {
			count++
		}
	}
	return count
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

// ToggleSideBySide toggles side-by-side view mode
func (d *DiffApprovalView) ToggleSideBySide() {
	d.SideBySide = !d.SideBySide
	d.updateContent()
}

// updateContent refreshes the viewport content. The item it puts on screen
// is marked viewed at its current version and digest.
func (d *DiffApprovalView) updateContent() {
	if len(d.Mutations) == 0 {
		d.Viewport.SetContent(d.renderEmpty())
		return
	}
	if d.CurrentIndex < len(d.Mutations) {
		cur := d.Mutations[d.CurrentIndex]
		cur.viewedVersion, cur.viewedDigest = cur.Request.Version, cur.Request.Digest
	}
	d.Viewport.SetContent(d.renderCurrentMutation())
}

// renderEmpty renders the empty state
func (d *DiffApprovalView) renderEmpty() string {
	emptyStyle := d.cachedStyles.emptyBase.Width(ViewportWidth(d.Width))

	return emptyStyle.Render("No approval requests to review.")
}

// renderCurrentMutation renders the current mutation diff
func (d *DiffApprovalView) renderCurrentMutation() string {
	if d.CurrentIndex >= len(d.Mutations) {
		return d.renderEmpty()
	}

	m := d.Mutations[d.CurrentIndex]
	p := m.Request.Preview
	var sb strings.Builder

	sb.WriteString(d.renderHeader(m))
	sb.WriteString("\n\n")

	// What the host could not model or did not show is never hidden behind
	// a toggle: approving without it would approve something unseen.
	var notice []string
	for _, u := range p.Unknowns {
		notice = append(notice, "Not modeled: "+u)
	}
	for _, w := range p.Incomplete {
		notice = append(notice, "Not shown: "+w)
	}
	if !p.Complete && len(p.Incomplete) == 0 {
		notice = append(notice, "Not shown: the preview is incomplete")
	}
	if len(notice) > 0 {
		sb.WriteString(d.renderWarnings("Review limits", notice))
		sb.WriteString("\n")
	}
	if d.ShowWarnings && len(p.Effects) > 0 {
		var effects strings.Builder
		effects.WriteString("Effects:\n")
		for _, e := range p.Effects {
			effects.WriteString("  • " + e + "\n")
		}
		sb.WriteString(d.Styles.Text.Body.Render(effects.String()))
		sb.WriteString("\n")
	}
	if p.Text != "" {
		sb.WriteString(d.Styles.Text.Muted.Render("Arguments: " + p.Text))
		sb.WriteString("\n\n")
	}

	// The diff is the host's, computed from the frozen inputs. The view
	// never recomputes a decision preview.
	if p.Diff != nil {
		sb.WriteString(d.renderDiff(p.Diff))
	} else {
		sb.WriteString(d.Styles.Text.Muted.Render("(No content diff for this operation)"))
	}

	sb.WriteString("\n\n")
	sb.WriteString(d.renderControls())

	return sb.String()
}

// renderHeader renders the request header: what, where, which request, until when.
func (d *DiffApprovalView) renderHeader(m *PendingMutation) string {
	headerStyle := d.cachedStyles.headerBase.Width(ViewportWidth(d.Width))
	r := m.Request

	status := "⏳ AWAITING YOUR DECISION"
	statusColor := d.Styles.Theme.Warning()
	switch m.Submitted {
	case approval.VerdictApprove:
		status = "→ APPROVAL SENT"
		statusColor = d.Styles.Theme.Success()
	case approval.VerdictReject:
		status = "→ REJECTION SENT"
		statusColor = d.Styles.Theme.Destructive()
	}
	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)

	header := fmt.Sprintf("🔐 Approval %d/%d: %s  %s",
		d.CurrentIndex+1, len(d.Mutations), r.Preview.Summary, statusStyle.Render(status))

	digest := r.Digest
	if len(digest) > 23 {
		digest = digest[:23] + "…"
	}
	expires := "no deadline"
	if !r.ExpiresAt.IsZero() {
		expires = fmt.Sprintf("expires %s (in %s)", r.ExpiresAt.Format("15:04:05"), time.Until(r.ExpiresAt).Round(time.Second))
	}
	subheader := fmt.Sprintf("Tool: %s  Action: %s  Target: %s  Agent: %s\nRequest: %s v%d  %s  %s",
		r.Subject.Tool, r.Subject.Action, r.Subject.Target, r.Subject.Agent,
		r.ID, r.Version, digest, expires)

	return headerStyle.Render(header) + "\n" + d.Styles.Text.Muted.Render(subheader)
}

// renderWarnings renders a titled warning box.
func (d *DiffApprovalView) renderWarnings(title string, warnings []string) string {
	warningStyle := d.cachedStyles.warningBase.Width(WarningBoxWidth(d.Width))

	var sb strings.Builder
	sb.WriteString("⚠️ " + title + ":\n")
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
	sb.WriteString(d.Styles.Text.Muted.Render(fileHeader))
	sb.WriteString("\n\n")

	// A hidden line is part of what is being approved, so the filter says
	// how many changed lines it is hiding.
	if d.IgnoreWhitespace {
		if hidden := d.hiddenWhitespaceLines(diff); hidden > 0 {
			sb.WriteString(d.Styles.Status.Warning.Render(fmt.Sprintf("⚠ %d whitespace-only changed line(s) hidden — press s to show them", hidden)))
		} else {
			sb.WriteString(d.Styles.Status.Info.Render("(Whitespace filter on: no changed line is hidden)"))
		}
		sb.WriteString("\n\n")
	}

	if diff.IsBinary {
		sb.WriteString(d.Styles.Status.Warning.Render("Binary file - diff not shown"))
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

		hunkStyle := d.Styles.Text.Muted
		if i == d.SelectedHunk {
			hunkStyle = d.cachedStyles.selectedHunk
		}
		sb.WriteString(hunkStyle.Render(hunkHeader))
		sb.WriteString("\n")

		// Render lines with word-level diffing for adjacent changed lines
		if d.SideBySide {
			sb.WriteString(d.renderSideBySideHunkLines(filteredLines))
		} else {
			sb.WriteString(d.renderHunkLines(filteredLines))
		}
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

// hiddenWhitespaceLines counts the changed lines the whitespace filter hides.
func (d *DiffApprovalView) hiddenWhitespaceLines(fd *FileDiff) int {
	changed := func(lines []DiffLine) int {
		n := 0
		for _, l := range lines {
			if l.Type == DiffLineAdded || l.Type == DiffLineRemoved {
				n++
			}
		}
		return n
	}
	hidden := 0
	for _, h := range fd.Hunks {
		hidden += changed(h.Lines) - changed(d.filterHunkLines(h.Lines))
	}
	return hidden
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
		style = d.Styles.Text.Body
		prefix = "  "
	case DiffLineHeader:
		style = d.Styles.Text.Bold
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

	sbsStatus := "OFF"
	if d.SideBySide {
		sbsStatus = "ON"
	}

	// Prepend status to the help view dynamically, or you can just show it.
	helpView := d.help.View(d.keys)
	controls := fmt.Sprintf("Whitespace: %s | SideBySide: %s | %s", wsStatus, sbsStatus, helpView)

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
	// Ranging the string directly yields the same runes without materializing
	// a []rune the size of the input.
	for _, r := range s {
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

// uiDiffEngine is the one diff engine the UI package uses, and it only ever
// computes word-level spans inside lines of a diff the host already computed.
// File diffs for a decision come from the request's sealed preview
// (approval.BuildPreview, uncached); CreateDiffFromStrings, which computed
// them here, had no production caller and was removed with the move to
// host-provided requests (2026-10-27). Shared because Engine is mutex-guarded.
var uiDiffEngine = diff.NewEngine()

// renderSideBySideHunkLines renders hunk lines in a side-by-side format
func (d *DiffApprovalView) renderSideBySideHunkLines(lines []DiffLine) string {
	var sb strings.Builder

	// Calculate panel width (half of the viewport width, minus the separator)
	// Make sure we have at least some minimum width
	totalWidth := d.Viewport.Width
	if totalWidth < 10 {
		totalWidth = 80 // fallback
	}
	panelWidth := (totalWidth - 3) / 2 // -3 for separator " | "
	if panelWidth < 10 {
		panelWidth = 10
	}

	// Temporarily override the XOffset calculation width for the inner rendering
	originalWidth := d.Viewport.Width
	d.Viewport.Width = panelWidth
	defer func() {
		d.Viewport.Width = originalWidth
	}()

	separator := lipgloss.NewStyle().Foreground(d.Styles.Theme.Outline()).Render(" | ")

	// Process lines mapping removes to the left side and adds to the right side
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		var left, right string

		switch line.Type {
		case DiffLineContext:
			// Context goes on both sides
			renderedLine := d.renderDiffLine(line)
			left = renderedLine
			right = renderedLine

		case DiffLineRemoved:
			// Removed goes on left side
			left = d.renderDiffLine(line)

			// Try to pair with an added line
			right = d.renderEmptyDiffLine()

			// Look ahead for an added line to pair with
			if i+1 < len(lines) && lines[i+1].Type == DiffLineAdded {
				// We have an added line next, pair them up!
				nextLine := lines[i+1]

				// Optional: Do word-level diffing if enabled
				if d.WordLevelDiff {
					spans := d.diffEngine.ComputeWordLevelDiff(line.Content, nextLine.Content)
					left = d.renderLineWithWordHighlights(line, spans, true)
					right = d.renderLineWithWordHighlights(nextLine, spans, false)
				} else {
					right = d.renderDiffLine(nextLine)
				}

				i++ // Skip the next line since we handled it
			}

		case DiffLineAdded:
			// Unpaired added line goes on right side
			left = d.renderEmptyDiffLine()
			right = d.renderDiffLine(line)

		case DiffLineHeader:
			left = d.renderDiffLine(line)
			right = "" // Or replicate to right side, but usually single line is fine
			// Let's span it across by not using side-by-side format for headers
			sb.WriteString(left)
			sb.WriteString("\n")
			continue
		}

		// Join the two panels horizontally
		row := lipgloss.JoinHorizontal(lipgloss.Top, left, separator, right)
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderEmptyDiffLine renders an empty line for side-by-side padding
func (d *DiffApprovalView) renderEmptyDiffLine() string {
	// Use lipgloss to ensure exact width padding
	style := d.Styles.Text.Body.Width(d.Viewport.Width)
	return style.Render("")
}

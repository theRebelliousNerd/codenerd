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
	DiffLineContext DiffLineType = iota // Unchanged context line
	DiffLineAdded                       // Added line
	DiffLineRemoved                     // Removed line
	DiffLineHeader                      // Diff header line
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
	Reason      string   // Why approval is needed
	Warnings    []string // Safety warnings
	Approved    bool
	Rejected    bool
	Comment     string // User's comment
}

// DiffApprovalView handles interactive diff approval
type DiffApprovalView struct {
	Styles           Styles
	Viewport         viewport.Model
	Mutations        []*PendingMutation
	CurrentIndex     int
	Width            int
	Height           int
	ShowWarnings     bool
	SelectedHunk     int
	ApprovalMode     ApprovalMode
	IgnoreWhitespace bool // When true, whitespace-only changes are hidden
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
// TODO: IMPROVEMENT: Replace magic numbers (e.g., `width - 4`, `height - 8`) with layout constants.
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

// ToggleIgnoreWhitespace toggles whitespace-only change filtering
func (d *DiffApprovalView) ToggleIgnoreWhitespace() {
	d.IgnoreWhitespace = !d.IgnoreWhitespace
	d.updateContent()
}

// isWhitespaceOnlyChange checks if a line change is whitespace-only
func isWhitespaceOnlyChange(oldContent, newContent string) bool {
	return strings.TrimSpace(oldContent) == strings.TrimSpace(newContent)
}

// normalizeWhitespace normalizes a string for whitespace comparison
func normalizeWhitespace(s string) string {
	// Replace all whitespace sequences with single space and trim
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
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
		Width(d.Width-4).
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
			hunkStyle = lipgloss.NewStyle().
				Background(d.Styles.Theme.Secondary).
				Foreground(d.Styles.Theme.Primary)
		}
		sb.WriteString(hunkStyle.Render(hunkHeader))
		sb.WriteString("\n")

		// Render lines
		for _, line := range filteredLines {
			sb.WriteString(d.renderDiffLine(line))
			sb.WriteString("\n")
		}
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

	for i := 0; i < len(lines); i++ {
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
// TODO: IMPROVEMENT: Use `key.Binding` for customizable keyboard controls instead of hardcoded strings.
func (d *DiffApprovalView) renderControls() string {
	controlStyle := lipgloss.NewStyle().
		Foreground(d.Styles.Theme.Muted).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.Styles.Theme.Border).
		Padding(0, 1).
		Width(d.Width - 4)

	// Build controls string with current toggle states
	wsStatus := "OFF"
	if d.IgnoreWhitespace {
		wsStatus = "ON"
	}

	controls := fmt.Sprintf("Controls: [y] Approve  [n] Reject  [a] Approve All  [←/→] Prev/Next  [↑/↓] Hunks\n"+
		"          [w] Warnings  [s] Whitespace(%s)  [q] Close", wsStatus)

	return controlStyle.Render(controls)
}

// View returns the rendered view
func (d *DiffApprovalView) View() string {
	return d.Viewport.View()
}

// computeLCS computes the Longest Common Subsequence table for two slices
// TODO: IMPROVEMENT: Consider replacing this manual LCS implementation with a robust diff library (e.g., `sergi/go-diff`) if performance or edge cases become an issue.
// TODO: IMPROVEMENT: Add unit tests specifically for the LCS and backtracking logic to ensure correctness of manual implementation.
func computeLCS(oldLines, newLines []string) [][]int {
	m, n := len(oldLines), len(newLines)
	// Create LCS table with (m+1) x (n+1) dimensions
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	// Fill the LCS table
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				if lcs[i-1][j] > lcs[i][j-1] {
					lcs[i][j] = lcs[i-1][j]
				} else {
					lcs[i][j] = lcs[i][j-1]
				}
			}
		}
	}
	return lcs
}

// diffOperation represents a single diff operation
type diffOperation struct {
	op      DiffLineType // DiffLineContext, DiffLineAdded, DiffLineRemoved
	oldIdx  int          // index in old lines (-1 if added)
	newIdx  int          // index in new lines (-1 if removed)
	content string
}

// backtrackLCS generates diff operations from the LCS table
func backtrackLCS(lcs [][]int, oldLines, newLines []string) []diffOperation {
	ops := make([]diffOperation, 0)
	i, j := len(oldLines), len(newLines)

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			// Lines match - context
			ops = append(ops, diffOperation{
				op:      DiffLineContext,
				oldIdx:  i - 1,
				newIdx:  j - 1,
				content: oldLines[i-1],
			})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			// Line added
			ops = append(ops, diffOperation{
				op:      DiffLineAdded,
				oldIdx:  -1,
				newIdx:  j - 1,
				content: newLines[j-1],
			})
			j--
		} else if i > 0 {
			// Line removed
			ops = append(ops, diffOperation{
				op:      DiffLineRemoved,
				oldIdx:  i - 1,
				newIdx:  -1,
				content: oldLines[i-1],
			})
			i--
		}
	}

	// Reverse to get correct order
	for left, right := 0, len(ops)-1; left < right; left, right = left+1, right-1 {
		ops[left], ops[right] = ops[right], ops[left]
	}
	return ops
}

// Large file threshold for optimized diff algorithm
const (
	LargeFileThreshold = 10000 // Lines threshold for switching to optimized algorithm
	MaxDiffLines       = 50000 // Maximum lines to process (safety limit)
	ChunkSize          = 1000  // Process files in chunks of this size
)

// CreateDiffFromStrings creates a FileDiff from old and new content strings using LCS algorithm
// For large files, it uses a chunked approach to avoid memory issues
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

	// Split into lines
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Handle empty content edge cases
	if oldContent == "" {
		oldLines = []string{}
	}
	if newContent == "" {
		newLines = []string{}
	}

	// Safety limit check
	if len(oldLines) > MaxDiffLines || len(newLines) > MaxDiffLines {
		// For extremely large files, create a summary diff instead
		return createLargeFileSummaryDiff(diff, oldLines, newLines)
	}

	// Use optimized chunked algorithm for large files
	if len(oldLines) > LargeFileThreshold || len(newLines) > LargeFileThreshold {
		return createChunkedDiff(diff, oldLines, newLines)
	}

	// Standard LCS algorithm for smaller files
	lcs := computeLCS(oldLines, newLines)
	ops := backtrackLCS(lcs, oldLines, newLines)

	// Convert operations to hunks with context grouping
	const contextLines = 3
	hunks := groupOpsIntoHunks(ops, contextLines)
	diff.Hunks = hunks

	return diff
}

// createLargeFileSummaryDiff creates a summary diff for extremely large files
func createLargeFileSummaryDiff(diff *FileDiff, oldLines, newLines []string) *FileDiff {
	// Count actual differences using a fast line-by-line comparison
	added, removed := 0, 0

	// Create line sets for fast lookup
	oldSet := make(map[string]int)
	for _, line := range oldLines {
		oldSet[line]++
	}

	newSet := make(map[string]int)
	for _, line := range newLines {
		newSet[line]++
	}

	// Count lines unique to each set
	for line, count := range oldSet {
		if newCount, exists := newSet[line]; exists {
			if count > newCount {
				removed += count - newCount
			}
		} else {
			removed += count
		}
	}

	for line, count := range newSet {
		if oldCount, exists := oldSet[line]; exists {
			if count > oldCount {
				added += count - oldCount
			}
		} else {
			added += count
		}
	}

	// Create a summary hunk
	summaryHunk := DiffHunk{
		OldStart: 1,
		OldCount: len(oldLines),
		NewStart: 1,
		NewCount: len(newLines),
		Lines: []DiffLine{
			{
				LineNum: 0,
				Content: fmt.Sprintf("File too large for detailed diff (%d lines old, %d lines new)", len(oldLines), len(newLines)),
				Type:    DiffLineHeader,
			},
			{
				LineNum: 0,
				Content: fmt.Sprintf("Summary: ~%d lines added, ~%d lines removed", added, removed),
				Type:    DiffLineHeader,
			},
		},
	}

	diff.Hunks = []DiffHunk{summaryHunk}
	return diff
}

// createChunkedDiff creates a diff using chunked processing for better memory efficiency
func createChunkedDiff(diff *FileDiff, oldLines, newLines []string) *FileDiff {
	// Find common prefix and suffix to reduce diff area
	prefixLen := findCommonPrefix(oldLines, newLines)
	suffixLen := findCommonSuffix(oldLines[prefixLen:], newLines[prefixLen:])

	// Extract the differing middle sections
	oldMiddle := oldLines[prefixLen : len(oldLines)-suffixLen]
	newMiddle := newLines[prefixLen : len(newLines)-suffixLen]

	// If middle sections are still large, process in chunks
	var allOps []diffOperation

	if len(oldMiddle) > ChunkSize || len(newMiddle) > ChunkSize {
		allOps = processInChunks(oldMiddle, newMiddle, prefixLen)
	} else {
		// Small enough for standard LCS
		if len(oldMiddle) > 0 || len(newMiddle) > 0 {
			lcs := computeLCS(oldMiddle, newMiddle)
			allOps = backtrackLCS(lcs, oldMiddle, newMiddle)
			// Adjust indices for prefix offset
			for i := range allOps {
				if allOps[i].oldIdx >= 0 {
					allOps[i].oldIdx += prefixLen
				}
				if allOps[i].newIdx >= 0 {
					allOps[i].newIdx += prefixLen
				}
			}
		}
	}

	// Convert operations to hunks
	const contextLines = 3
	hunks := groupOpsIntoHunks(allOps, contextLines)
	diff.Hunks = hunks

	return diff
}

// findCommonPrefix finds the number of common lines at the start
func findCommonPrefix(old, new []string) int {
	minLen := len(old)
	if len(new) < minLen {
		minLen = len(new)
	}

	for i := 0; i < minLen; i++ {
		if old[i] != new[i] {
			return i
		}
	}
	return minLen
}

// findCommonSuffix finds the number of common lines at the end
func findCommonSuffix(old, new []string) int {
	minLen := len(old)
	if len(new) < minLen {
		minLen = len(new)
	}

	for i := 0; i < minLen; i++ {
		if old[len(old)-1-i] != new[len(new)-1-i] {
			return i
		}
	}
	return minLen
}

// processInChunks processes large diffs in smaller chunks
func processInChunks(oldLines, newLines []string, baseOffset int) []diffOperation {
	allOps := make([]diffOperation, 0)

	// Simple chunked approach: process in fixed-size windows
	// This is a trade-off between accuracy and memory/speed
	oldIdx, newIdx := 0, 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		// Determine chunk boundaries
		oldEnd := oldIdx + ChunkSize
		if oldEnd > len(oldLines) {
			oldEnd = len(oldLines)
		}
		newEnd := newIdx + ChunkSize
		if newEnd > len(newLines) {
			newEnd = len(newLines)
		}

		oldChunk := oldLines[oldIdx:oldEnd]
		newChunk := newLines[newIdx:newEnd]

		// Find sync point at end of chunks (common line to align on)
		syncOldOffset, syncNewOffset := findSyncPoint(oldChunk, newChunk)

		if syncOldOffset > 0 && syncNewOffset > 0 {
			oldChunk = oldChunk[:syncOldOffset]
			newChunk = newChunk[:syncNewOffset]
		}

		// Process this chunk
		if len(oldChunk) > 0 || len(newChunk) > 0 {
			lcs := computeLCS(oldChunk, newChunk)
			chunkOps := backtrackLCS(lcs, oldChunk, newChunk)

			// Adjust indices for offset
			for i := range chunkOps {
				if chunkOps[i].oldIdx >= 0 {
					chunkOps[i].oldIdx += oldIdx + baseOffset
				}
				if chunkOps[i].newIdx >= 0 {
					chunkOps[i].newIdx += newIdx + baseOffset
				}
			}
			allOps = append(allOps, chunkOps...)
		}

		oldIdx += len(oldChunk)
		newIdx += len(newChunk)
	}

	return allOps
}

// findSyncPoint finds a common line near the end of both chunks to sync on
func findSyncPoint(oldChunk, newChunk []string) (int, int) {
	// Look for a common line in the last 20% of each chunk
	searchStart := len(oldChunk) * 4 / 5
	if searchStart < 0 {
		searchStart = 0
	}

	for i := len(oldChunk) - 1; i >= searchStart; i-- {
		for j := len(newChunk) - 1; j >= len(newChunk)*4/5 && j >= 0; j-- {
			if oldChunk[i] == newChunk[j] {
				return i, j
			}
		}
	}

	return len(oldChunk), len(newChunk) // No sync point found, use full chunks
}

// groupOpsIntoHunks groups diff operations into hunks with context
func groupOpsIntoHunks(ops []diffOperation, contextLines int) []DiffHunk {
	if len(ops) == 0 {
		return nil
	}

	hunks := make([]DiffHunk, 0)
	var currentHunk *DiffHunk
	lastChangeIdx := -1

	for i, op := range ops {
		isChange := op.op != DiffLineContext

		if isChange {
			// Start a new hunk if needed
			if currentHunk == nil {
				currentHunk = &DiffHunk{
					OldStart: 1,
					NewStart: 1,
					Lines:    make([]DiffLine, 0),
				}
				// Add leading context
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				for j := start; j < i; j++ {
					if ops[j].op == DiffLineContext {
						lineNum := ops[j].oldIdx + 1
						if lineNum == 0 {
							lineNum = ops[j].newIdx + 1
						}
						currentHunk.Lines = append(currentHunk.Lines, DiffLine{
							LineNum: lineNum,
							Content: ops[j].content,
							Type:    DiffLineContext,
						})
					}
				}
				// Set start positions
				if len(currentHunk.Lines) > 0 {
					currentHunk.OldStart = ops[start].oldIdx + 1
					currentHunk.NewStart = ops[start].newIdx + 1
				} else if op.oldIdx >= 0 {
					currentHunk.OldStart = op.oldIdx + 1
				} else {
					currentHunk.NewStart = op.newIdx + 1
				}
			}
			lastChangeIdx = i
		}

		// Add the current operation to the hunk
		if currentHunk != nil {
			lineNum := op.oldIdx + 1
			if op.op == DiffLineAdded {
				lineNum = op.newIdx + 1
			}
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				LineNum: lineNum,
				Content: op.content,
				Type:    op.op,
			})

			// Check if we should close the hunk (too much context after changes)
			if op.op == DiffLineContext && i-lastChangeIdx > contextLines {
				// Trim trailing context to contextLines
				trimTo := len(currentHunk.Lines) - (i - lastChangeIdx - contextLines)
				if trimTo > 0 && trimTo < len(currentHunk.Lines) {
					currentHunk.Lines = currentHunk.Lines[:trimTo]
				}
				// Count old and new lines
				for _, line := range currentHunk.Lines {
					if line.Type == DiffLineRemoved || line.Type == DiffLineContext {
						currentHunk.OldCount++
					}
					if line.Type == DiffLineAdded || line.Type == DiffLineContext {
						currentHunk.NewCount++
					}
				}
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		}
	}

	// Close final hunk
	if currentHunk != nil && len(currentHunk.Lines) > 0 {
		for _, line := range currentHunk.Lines {
			if line.Type == DiffLineRemoved || line.Type == DiffLineContext {
				currentHunk.OldCount++
			}
			if line.Type == DiffLineAdded || line.Type == DiffLineContext {
				currentHunk.NewCount++
			}
		}
		hunks = append(hunks, *currentHunk)
	}

	// If no hunks created but we have ops, create a single hunk (all context case)
	if len(hunks) == 0 && len(ops) > 0 {
		// Check if there were any actual changes
		hasChanges := false
		for _, op := range ops {
			if op.op != DiffLineContext {
				hasChanges = true
				break
			}
		}
		if !hasChanges {
			return nil // No changes, no hunks needed
		}
	}

	return hunks
}

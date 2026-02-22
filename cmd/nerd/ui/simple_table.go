package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SimpleTable is a simple table component for rendering static data.
type SimpleTable struct {
	Title   string
	Headers []string
	Rows    [][]string
}

// NewSimpleTable creates a new SimpleTable with the given title and headers.
func NewSimpleTable(title string, headers []string) *SimpleTable {
	return &SimpleTable{
		Title:   title,
		Headers: headers,
		Rows:    make([][]string, 0),
	}
}

// AddRow adds a row to the table.
func (t *SimpleTable) AddRow(row ...string) {
	t.Rows = append(t.Rows, row)
}

// View renders the table using the provided styles.
func (t *SimpleTable) View(styles Styles) string {
	if len(t.Rows) == 0 {
		return ""
	}

	var sb strings.Builder

	// Title
	if t.Title != "" {
		sb.WriteString(styles.Title.Render(t.Title))
		sb.WriteString("\n")
	}

	// Calculate column widths
	colWidths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		colWidths[i] = lipgloss.Width(h)
	}

	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(colWidths) {
				w := lipgloss.Width(cell)
				if w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}

	// Add padding to widths because lipgloss Width includes padding
	for i := range colWidths {
		colWidths[i] += 2
	}

	// Define styles
	headerStyle := styles.Bold.Copy().Padding(0, 1)
	rowStyle := styles.Body.Copy().Padding(0, 1)
	sepStyle := styles.Muted

	// Render Header
	for i, h := range t.Headers {
		if i < len(colWidths) {
			sb.WriteString(headerStyle.Width(colWidths[i]).Render(h))
			if i < len(t.Headers)-1 {
				sb.WriteString(sepStyle.Render("|"))
			}
		}
	}
	sb.WriteString("\n")

	// Render Divider
	// Calculate total width
	totalWidth := 0
	for _, w := range colWidths {
		totalWidth += w
	}
	totalWidth += len(t.Headers) - 1 // Separators

	sb.WriteString(sepStyle.Render(strings.Repeat("-", totalWidth)) + "\n")

	// Render Rows
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(colWidths) {
				sb.WriteString(rowStyle.Width(colWidths[i]).Render(cell))
				if i < len(row)-1 {
					sb.WriteString(sepStyle.Render("|"))
				}
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

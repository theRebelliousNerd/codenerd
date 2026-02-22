package ui

import (
	"strings"
	"testing"
)

func TestSimpleTable(t *testing.T) {
	table := NewSimpleTable("Test Table", []string{"Col1", "Col2"})
	table.AddRow("Row1Col1", "Row1Col2")

	styles := DefaultStyles()
	view := table.View(styles)

	t.Logf("View:\n%q", view)

	if !strings.Contains(view, "Test Table") {
		t.Error("View missing title")
	}
	if !strings.Contains(view, "Row1Col1") {
		t.Error("View missing cell content")
	}
}

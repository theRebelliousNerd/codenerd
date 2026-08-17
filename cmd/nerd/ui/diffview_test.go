package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffApprovalView_HorizontalOffset_Logic(t *testing.T) {
	// This test focuses on the state changes of XOffset
	view := NewDiffApprovalView(Styles{Theme: LightTheme()}, 10, 10)

	assert.Equal(t, 0, view.XOffset)

	// Scroll Right
	view.ScrollRight()
	assert.Equal(t, 4, view.XOffset)

	view.ScrollRight()
	assert.Equal(t, 8, view.XOffset)

	// Scroll Left
	view.ScrollLeft()
	assert.Equal(t, 4, view.XOffset)

	view.ScrollLeft()
	assert.Equal(t, 0, view.XOffset)

	// Scroll Left (Should not go negative)
	view.ScrollLeft()
	assert.Equal(t, 0, view.XOffset)

	// Scroll To Start
	view.ScrollRight() // 4
	view.ScrollRight() // 8
	view.ScrollToStart()
	assert.Equal(t, 0, view.XOffset)
}

func TestDiffApprovalView_Rendering_Truncation(t *testing.T) {
	view := NewDiffApprovalView(Styles{Theme: BasicTheme{}}, 20, 20)
	longLine := "0123456789abcdefghijklmnopqrstuvwxyz"
	line := DiffLine{LineNum: 1, Content: longLine, Type: DiffLineContext}
	initialRendered := view.renderDiffLine(line)

	// Scroll right and verify the visible slice changes.
	view.ScrollRight()
	assert.Equal(t, 4, view.XOffset)

	scrolledRendered := view.renderDiffLine(line)
	assert.NotEqual(t, initialRendered, scrolledRendered, "Rendered line should change after scrolling right")

	// Scroll back to start and verify the original rendering is restored.
	view.ScrollToStart()
	assert.Equal(t, 0, view.XOffset)
	assert.Equal(t, initialRendered, view.renderDiffLine(line), "Rendered line should match initial state after scrolling back")
}

func TestDiffApprovalViewHorizontalScrolling(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 10, 10)
	longLine := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	line := DiffLine{LineNum: 1, Content: longLine, Type: DiffLineContext}
	initialRendered := view.renderDiffLine(line)

	// Scroll right
	view.ScrollRight()

	if view.XOffset != 4 {
		t.Fatalf("expected XOffset 4 after ScrollRight, got %d", view.XOffset)
	}

	scrolledRendered := view.renderDiffLine(line)
	if scrolledRendered == initialRendered {
		t.Fatalf("expected rendered line to change after horizontal scrolling")
	}

	// Scroll left
	view.ScrollLeft()
	if view.XOffset != 0 {
		t.Fatalf("expected XOffset 0 after ScrollLeft, got %d", view.XOffset)
	}
	if rendered := view.renderDiffLine(line); rendered != initialRendered {
		t.Fatalf("expected rendered line to return to the initial slice after ScrollLeft")
	}

	// Scroll right again to test ScrollToStart
	view.ScrollRight()
	view.ScrollRight()
	if view.XOffset != 8 {
		t.Fatalf("expected XOffset 8 before ScrollToStart, got %d", view.XOffset)
	}

	// Scroll to start
	view.ScrollToStart()
	if view.XOffset != 0 {
		t.Fatalf("expected XOffset 0 after ScrollToStart, got %d", view.XOffset)
	}
	if rendered := view.renderDiffLine(line); rendered != initialRendered {
		t.Fatalf("expected rendered line to match initial state after ScrollToStart")
	}
}

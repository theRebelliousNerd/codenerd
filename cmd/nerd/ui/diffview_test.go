package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffApprovalView_HorizontalOffset_Logic(t *testing.T) {
	// This test focuses on the state changes of XOffset
	view := NewDiffApprovalView(Styles{}, 10, 10)

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
	// Setup styles to avoid nil pointer dereferences
	styles := Styles{
		Theme: Theme{}, // Zero value
	}

	view := NewDiffApprovalView(styles, 100, 20)

	// Add a mutation
	m := &PendingMutation{
		ID:          "1",
		Description: "Simple Description",
		FilePath:    "file.txt",
		Reason:      "Reason",
		Approved:    false,
		Rejected:    false,
		Diff:        nil, // Will render "(No diff available)"
	}
	view.AddMutation(m)

	initialView := view.View()
	assert.Contains(t, initialView, "Simple Description")
	assert.Contains(t, initialView, "Mutation")

	// Scroll Right by 3
	view.ScrollRight()

	scrolledView := view.View()

	// Let's check that the view content CHANGED.
	assert.NotEqual(t, initialView, scrolledView, "View content should change after scrolling right")

	// And if we scroll back to start, it should match initial view (mostly, assuming no other side effects)
	view.ScrollToStart()
	backToStartView := view.View()
	assert.Equal(t, initialView, backToStartView, "View content should match initial state after scrolling back")
}

func TestDiffApprovalViewHorizontalScrolling(t *testing.T) {
	// Create a view with small width
	view := NewDiffApprovalView(DefaultStyles(), 10, 10)

	// Create a diff with a very long line
	longLine := strings.Repeat("a", 50)
	diff := &FileDiff{
		OldPath: "old.txt",
		NewPath: "new.txt",
		Hunks: []DiffHunk{
			{
				OldStart: 1, OldCount: 1,
				NewStart: 1, NewCount: 1,
				Lines: []DiffLine{
					{LineNum: 1, Content: longLine, Type: DiffLineContext},
				},
			},
		},
	}

	view.AddMutation(&PendingMutation{
		ID:          "1",
		Description: "Test",
		FilePath:    "file.go",
		Diff:        diff,
	})

	if view.XOffset != 0 {
		t.Fatalf("expected initial XOffset 0, got %d", view.XOffset)
	}

	initialView := view.View()

	// Scroll right
	view.ScrollRight()

	if view.XOffset != 4 {
		t.Fatalf("expected XOffset 4 after ScrollRight, got %d", view.XOffset)
	}

	if scrolledView := view.View(); scrolledView == initialView {
		t.Fatalf("expected rendered view to change after horizontal scrolling")
	}

	// Scroll left
	view.ScrollLeft()
	if view.XOffset != 0 {
		t.Fatalf("expected XOffset 0 after ScrollLeft, got %d", view.XOffset)
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

	if backToStart := view.View(); backToStart != initialView {
		t.Fatalf("expected rendered view to match initial state after ScrollToStart")
	}
}

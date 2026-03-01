package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffApprovalView_HorizontalOffset_Logic(t *testing.T) {
	// This test focuses on the state changes of HorizontalOffset
	view := NewDiffApprovalView(Styles{}, 10, 10)

	assert.Equal(t, 0, view.HorizontalOffset)

	// Scroll Right
	view.ScrollRight()
	assert.Equal(t, 3, view.HorizontalOffset)

	view.ScrollRight()
	assert.Equal(t, 6, view.HorizontalOffset)

	// Scroll Left
	view.ScrollLeft()
	assert.Equal(t, 3, view.HorizontalOffset)

	view.ScrollLeft()
	assert.Equal(t, 0, view.HorizontalOffset)

	// Scroll Left (Should not go negative)
	view.ScrollLeft()
	assert.Equal(t, 0, view.HorizontalOffset)

	// Scroll To Start
	view.ScrollRight() // 3
	view.ScrollRight() // 6
	view.ScrollToStart()
	assert.Equal(t, 0, view.HorizontalOffset)
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

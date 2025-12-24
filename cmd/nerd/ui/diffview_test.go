package ui

import (
	"strings"
	"testing"
)

func TestCreateDiffFromStringsFlags(t *testing.T) {
	diff := CreateDiffFromStrings("old.txt", "new.txt", "", "line1\nline2")
	if !diff.IsNew {
		t.Fatalf("expected IsNew to be true")
	}
	if diff.IsDelete {
		t.Fatalf("expected IsDelete to be false")
	}

	diff = CreateDiffFromStrings("old.txt", "new.txt", "line1", "")
	if !diff.IsDelete {
		t.Fatalf("expected IsDelete to be true")
	}
	if diff.IsNew {
		t.Fatalf("expected IsNew to be false")
	}
}

func TestCreateDiffFromStringsLineTypes(t *testing.T) {
	diff := CreateDiffFromStrings("old.txt", "new.txt", "old1\nold2", "new1\nnew2")
	if len(diff.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(diff.Hunks))
	}
	lines := diff.Hunks[0].Lines
	if len(lines) != 4 {
		t.Fatalf("expected 4 diff lines, got %d", len(lines))
	}
	if lines[0].Type != DiffLineRemoved || lines[1].Type != DiffLineRemoved {
		t.Fatalf("expected first two lines to be removed")
	}
	if lines[2].Type != DiffLineAdded || lines[3].Type != DiffLineAdded {
		t.Fatalf("expected last two lines to be added")
	}
}

func TestDiffApprovalViewApproveReject(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 80, 20)
	view.AddMutation(&PendingMutation{
		ID:          "1",
		Description: "Test",
		FilePath:    "file.go",
		Diff:        makeSimpleDiff(),
		Reason:      "reason",
	})

	if view.GetPendingCount() != 1 {
		t.Fatalf("expected 1 pending mutation, got %d", view.GetPendingCount())
	}
	if !view.ApproveCurrent() {
		t.Fatalf("expected approve to succeed")
	}
	if !view.Mutations[0].Approved || view.Mutations[0].Rejected {
		t.Fatalf("expected mutation to be approved")
	}
	if view.GetPendingCount() != 0 || view.HasPending() {
		t.Fatalf("expected no pending mutations after approval")
	}

	view.ClearMutations()
	view.AddMutation(&PendingMutation{
		ID:          "2",
		Description: "Reject",
		FilePath:    "file.go",
		Diff:        makeSimpleDiff(),
		Reason:      "reason",
	})
	if !view.RejectCurrent("no") {
		t.Fatalf("expected reject to succeed")
	}
	if !view.Mutations[0].Rejected || view.Mutations[0].Approved {
		t.Fatalf("expected mutation to be rejected")
	}
	if view.Mutations[0].Comment != "no" {
		t.Fatalf("expected reject comment to be recorded")
	}
}

func TestDiffApprovalViewWarningsAndHunks(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 80, 20)
	view.AddMutation(&PendingMutation{
		ID:          "1",
		Description: "Test",
		FilePath:    "file.go",
		Diff:        makeTwoHunkDiff(),
		Warnings:    []string{"unsafe"},
		Reason:      "reason",
	})

	if view.SelectedHunk != 0 {
		t.Fatalf("expected selected hunk to start at 0")
	}
	view.NextHunk()
	if view.SelectedHunk != 1 {
		t.Fatalf("expected selected hunk to move to 1")
	}
	view.PrevHunk()
	if view.SelectedHunk != 0 {
		t.Fatalf("expected selected hunk to move back to 0")
	}

	content := view.renderCurrentMutation()
	if !strings.Contains(content, "Warnings:") {
		t.Fatalf("expected warnings to be rendered")
	}
	view.ToggleWarnings()
	content = view.renderCurrentMutation()
	if strings.Contains(content, "Warnings:") {
		t.Fatalf("expected warnings to be hidden")
	}
}

func TestDiffApprovalViewRenderDiffLine(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 80, 20)
	line := DiffLine{LineNum: 1, Content: "hello", Type: DiffLineAdded}
	rendered := view.renderDiffLine(line)
	if !strings.Contains(rendered, "+ ") || !strings.Contains(rendered, "hello") {
		t.Fatalf("expected added line to include prefix and content")
	}
}

func makeSimpleDiff() *FileDiff {
	return &FileDiff{
		OldPath: "old.txt",
		NewPath: "new.txt",
		Hunks: []DiffHunk{
			{
				OldStart: 1,
				OldCount: 1,
				NewStart: 1,
				NewCount: 1,
				Lines: []DiffLine{
					{LineNum: 1, Content: "line", Type: DiffLineAdded},
				},
			},
		},
	}
}

func makeTwoHunkDiff() *FileDiff {
	return &FileDiff{
		OldPath: "old.txt",
		NewPath: "new.txt",
		Hunks: []DiffHunk{
			{
				OldStart: 1,
				OldCount: 1,
				NewStart: 1,
				NewCount: 1,
				Lines: []DiffLine{
					{LineNum: 1, Content: "first", Type: DiffLineAdded},
				},
			},
			{
				OldStart: 3,
				OldCount: 1,
				NewStart: 3,
				NewCount: 1,
				Lines: []DiffLine{
					{LineNum: 3, Content: "second", Type: DiffLineAdded},
				},
			},
		},
	}
}

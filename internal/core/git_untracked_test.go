package core

import "testing"

// The untracked set was already being recognised and then thrown away: the "??"
// branch has always identified it, folded it into the unstaged count, and
// dropped the paths. Returning it separately is what makes the new_files world
// state reachable, and that state gates a MANDATORY atom -- so for the life of
// the field it was a mandatory atom that could never be selected.
//
// Untracked is not a subset of modified in any useful sense: a modified file is
// a change to something the project has, an untracked one is something that
// exists and is not yet part of the project. They want different advice.
func TestParseGitStatusSeparatesUntrackedFromModified(t *testing.T) {
	// Real `git status --porcelain` shapes, including a rename, which carries
	// its arrow form and must resolve to the destination path.
	status := " M internal/core/kernel_query.go\n" +
		"?? internal/core/git_untracked_test.go\n" +
		"A  internal/core/added.go\n" +
		"?? scratch/notes.md\n" +
		"R  old/path.go -> new/path.go\n"

	files, unstaged, untracked := parseGitStatus(status)

	wantUntracked := map[string]bool{
		"internal/core/git_untracked_test.go": true,
		"scratch/notes.md":                    true,
	}
	if len(untracked) != len(wantUntracked) {
		t.Fatalf("untracked = %v, want the two ?? entries", untracked)
	}
	for _, u := range untracked {
		if !wantUntracked[u] {
			t.Errorf("untracked contains %q, which is not an untracked path", u)
		}
	}

	// The existing contract must not shift underneath its callers: untracked
	// files still count as changed paths and still count as unstaged, which is
	// what the high_churn state and the modified-files list have always meant.
	var sawUntrackedInFiles, sawRenameDestination bool
	for _, f := range files {
		if f == "scratch/notes.md" {
			sawUntrackedInFiles = true
		}
		if f == "new/path.go" {
			sawRenameDestination = true
		}
	}
	if !sawUntrackedInFiles {
		t.Error("untracked paths dropped out of the changed-file list; that list is what " +
			"GitModifiedFiles is built from and its meaning has not changed")
	}
	if !sawRenameDestination {
		t.Error("a rename did not resolve to its destination path")
	}
	if unstaged != 3 {
		t.Errorf("unstaged = %d, want 3: the two ?? entries and the worktree-modified one. "+
			"A staged add and a staged rename are not unstaged.", unstaged)
	}
}

// A clean tree must report nothing rather than one empty string, or the world
// state fires on a repository with no untracked files at all.
func TestParseGitStatusOnACleanTree(t *testing.T) {
	files, unstaged, untracked := parseGitStatus("")
	if len(files) != 0 || unstaged != 0 || len(untracked) != 0 {
		t.Errorf("clean tree gave files=%v unstaged=%d untracked=%v, want all empty",
			files, unstaged, untracked)
	}
}

// The porcelain format is two columns, "XY path", and a space is a meaningful
// value in either: " M file" is modified in the worktree and NOT staged, while
// "M  file" is staged. The line was being trimmed before parsing, which shifts
// both columns left and turns the first into the second — and since the unstaged
// check reads the SECOND column, an ordinary edited-but-unstaged file was
// counted as staged.
//
// That is the state a working tree spends most of its life in, so the unstaged
// count only ever saw untracked files and files staged and then edited again.
// high_churn is gated on that count passing twenty and could not be reached by
// editing files at all.
func TestParseGitStatusReadsBothPorcelainColumns(t *testing.T) {
	cases := []struct {
		line         string
		wantUnstaged int
		why          string
	}{
		{" M file.go", 1, "modified in the worktree, not staged"},
		{"M  file.go", 0, "staged, worktree clean"},
		{"MM file.go", 1, "staged, then modified again"},
		{"A  file.go", 0, "added to the index, worktree clean"},
		{" D file.go", 1, "deleted in the worktree, not staged"},
		{"?? file.go", 1, "untracked"},
	}

	for _, c := range cases {
		_, unstaged, _ := parseGitStatus(c.line)
		if unstaged != c.wantUnstaged {
			t.Errorf("parseGitStatus(%q) unstaged = %d, want %d (%s)",
				c.line, unstaged, c.wantUnstaged, c.why)
		}
	}
}

// A Windows git ends its lines with \r. Trailing whitespace must still be
// removed or the path carries it, while leading whitespace must not be.
func TestParseGitStatusToleratesCarriageReturns(t *testing.T) {
	files, unstaged, untracked := parseGitStatus(" M file.go\r\n?? other.go\r\n")
	if unstaged != 2 {
		t.Errorf("unstaged = %d, want 2", unstaged)
	}
	if len(untracked) != 1 || untracked[0] != "other.go" {
		t.Errorf("untracked = %v, want [other.go] with no trailing carriage return", untracked)
	}
	for _, f := range files {
		if f != "file.go" && f != "other.go" {
			t.Errorf("file %q carries whitespace it should not", f)
		}
	}
}

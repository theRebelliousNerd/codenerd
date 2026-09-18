package chat

import (
	"strings"
	"testing"
)

// Regression for live session 2026-09-17: typing "/fix <file> <symptom>
// <constraint>" in the chat TUI delegated "fix issue in <file>" with the
// symptom/constraint dropped, so the coder shard guessed. The CLI path
// passes the brief verbatim. These tests pin the root fix in
// formatShardTask: the shard task must carry the target file explicitly
// (file: prefix, matching the reviewer-context path "fix file:%s" and the
// /review|/security|/analyze convention) together with the user's words.
//
// Each test fails against the pre-fix formatter:
//   "/fix"  -> "fix issue in <target>" (no file: label)
//   "/refactor" -> "refactor <target>" (no file: label)
// and passes after.

func TestFormatShardTask_FixPreservesFileAndBrief(t *testing.T) {
	task := formatShardTask("/fix", "auth.go", "login fails when token expires, must keep backwards compat", "")
	if !strings.Contains(task, "fix file:auth.go") {
		t.Fatalf("fix task must name the target file explicitly, got %q", task)
	}
	if !strings.Contains(task, "login fails when token expires") {
		t.Fatalf("fix task must carry the symptom description verbatim, got %q", task)
	}
	if !strings.Contains(task, "must keep backwards compat") {
		t.Fatalf("fix task must carry the constraint verbatim, got %q", task)
	}
}

func TestFormatShardTask_FixWithoutConstraintStillNamesFile(t *testing.T) {
	task := formatShardTask("/fix", "auth.go", "", "")
	if !strings.Contains(task, "fix file:auth.go") {
		t.Fatalf("fix task without constraint must still name the file, got %q", task)
	}
}

func TestFormatShardTask_RefactorPreservesFileAndBrief(t *testing.T) {
	task := formatShardTask("/refactor", "auth.go", "split login handler, keep exported API", "")
	if !strings.Contains(task, "refactor file:auth.go") {
		t.Fatalf("refactor task must name the target file explicitly, got %q", task)
	}
	if !strings.Contains(task, "split login handler") {
		t.Fatalf("refactor task must carry the user's brief verbatim, got %q", task)
	}
}

// The slash handlers used to join everything after "/fix" into the target, so
// "/fix auth.go login fails when token expires" reached the shard as one opaque
// string with no target and no constraint. splitSlashTarget is the join's
// replacement: a path-like first token is the target, the rest is the user's
// words; text with no path stays whole.
func TestSplitSlashTarget_PathThenWordsBecomeTargetAndConstraint(t *testing.T) {
	target, constraint := splitSlashTarget([]string{"auth.go", "login", "fails", "when", "token", "expires"})
	if target != "auth.go" {
		t.Fatalf("target = %q, want auth.go", target)
	}
	if constraint != "login fails when token expires" {
		t.Fatalf("constraint = %q, want the user's words verbatim", constraint)
	}
	task := formatShardTask("/fix", target, constraint, "")
	for _, want := range []string{"fix file:auth.go", "login fails when token expires"} {
		if !strings.Contains(task, want) {
			t.Fatalf("slash /fix task %q must contain %q", task, want)
		}
	}
}

func TestSplitSlashTarget_NoPathKeepsWholeTextAsTarget(t *testing.T) {
	target, constraint := splitSlashTarget([]string{"the", "login", "flow", "is", "broken"})
	if target != "the login flow is broken" || constraint != "" {
		t.Fatalf("got target=%q constraint=%q; text without a path must stay whole", target, constraint)
	}
}

func TestSplitSlashTarget_DirectoryAndNestedPathsCount(t *testing.T) {
	for _, first := range []string{"internal/session", `cmd\nerd\chat\process.go`, "./main.go"} {
		target, constraint := splitSlashTarget([]string{first, "tidy", "it"})
		if target != first || constraint != "tidy it" {
			t.Fatalf("%q: got target=%q constraint=%q", first, target, constraint)
		}
	}
	if looksLikePath("v1.") || looksLikePath("e.g.") || looksLikePath("login") {
		t.Fatal("trailing-dot and plain words must not be treated as paths")
	}
}

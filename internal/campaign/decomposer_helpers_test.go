package campaign

import (
	"slices"
	"testing"
)

func TestLimitString(t *testing.T) {
	if got := limitString("hello", 3); got != "hel" {
		t.Errorf("limitString(hello,3)=%q, want hel", got)
	}
	if got := limitString("hi", 10); got != "hi" {
		t.Errorf("limitString(hi,10)=%q, want hi", got)
	}
	if got := limitString("anything", 0); got != "" {
		t.Errorf("limitString(_,0)=%q, want empty", got)
	}
	// Rune-aware (multibyte not split mid-rune).
	if got := limitString("héllo", 2); got != "hé" {
		t.Errorf("limitString(héllo,2)=%q, want hé", got)
	}
}

func TestDeriveTagsFromPath(t *testing.T) {
	tags := deriveTagsFromPath("src/auth-service/login.go")
	for _, want := range []string{"login", "service", "auth"} {
		if !slices.Contains(tags, want) {
			t.Errorf("deriveTagsFromPath should include %q, got %v", want, tags)
		}
	}
	// Backslash paths are normalized.
	winTags := deriveTagsFromPath(`pkg\handlers\user.go`)
	if !slices.Contains(winTags, "user") || !slices.Contains(winTags, "handlers") {
		t.Errorf("backslash path tags missing expected segments: %v", winTags)
	}
}

func TestExtractActionsFromDescription(t *testing.T) {
	got := extractActionsFromDescription("Parse the file and validate the schema, then build it")
	for _, want := range []string{"parse", "validate", "build"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected action %q in %v", want, got)
		}
	}
	if slices.Contains(got, "deploy") {
		t.Error("deploy should not be extracted when absent")
	}
	if len(extractActionsFromDescription("just some prose with no verbs of note")) != 0 {
		t.Error("no action keywords should yield no actions")
	}
}

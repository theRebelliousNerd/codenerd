package campaign

import (
	"slices"
	"testing"
)

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

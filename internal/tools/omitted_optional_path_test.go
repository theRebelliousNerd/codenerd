package tools

import (
	"context"
	"strings"
	"testing"
)

// Null-like spellings mean the optional argument was not given.
func TestIsOmittedOptionalPath_SentinelsAreOmitted(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"", " ", "null", "NULL", "  null  ", "none", "NONE", "nil", "NIL", "undefined", "UNDEFINED", ".", "./"} {
		if !IsOmittedOptionalPath(p) {
			t.Errorf("IsOmittedOptionalPath(%q) = false, want true", p)
		}
	}
}

// Ordinary paths are not omitted, including ones that merely contain a
// sentinel word.
func TestIsOmittedOptionalPath_RealPathsAreKept(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"foo", "./internal/session", "nulls", "a/null/b", "0", "nonexistent-dir"} {
		if IsOmittedOptionalPath(p) {
			t.Errorf("IsOmittedOptionalPath(%q) = true, want false", p)
		}
	}
}

// ResolveWorkspaceDir with a sentinel lands on the workspace root instead of
// a literal "null" directory under it.
func TestResolveWorkspaceDir_SentinelResolvesToRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), root)
	want, err := CanonicalWorkspaceRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"null", "none", "nil", "undefined", "NULL", "  null  "} {
		got, err := ResolveWorkspaceDir(ctx, "", p)
		if err != nil || got != want {
			t.Errorf("ResolveWorkspaceDir(%q) = %q, %v; want %q", p, got, err, want)
		}
		if strings.HasSuffix(got, "null") && got != want {
			t.Errorf("ResolveWorkspaceDir(%q) joined a literal null dir: %q", p, got)
		}
	}
}

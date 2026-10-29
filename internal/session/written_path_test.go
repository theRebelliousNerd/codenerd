package session

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalizeWrittenPath(t *testing.T) {
	workspace := filepath.Clean(t.TempDir())

	if got := canonicalizeWrittenPath("", workspace); got != "" {
		t.Fatalf("empty: got %q want %q", got, "")
	}

	rel := filepath.Join("internal", "session", "example.go")
	wantRel := filepath.ToSlash(filepath.Clean(rel))
	if got := canonicalizeWrittenPath(rel, workspace); got != wantRel {
		t.Fatalf("relative: got %q want %q", got, wantRel)
	}

	containedAbs := filepath.Join(workspace, "internal", "session", "example.go")
	got := canonicalizeWrittenPath(containedAbs, workspace)
	wantContained := filepath.ToSlash(filepath.Join("internal", "session", "example.go"))
	if got != wantContained {
		t.Fatalf("contained: got %q want %q", got, wantContained)
	}
	pkgs := packagesForPaths([]string{got})
	if len(pkgs) != 1 {
		t.Fatalf("packagesForPaths len %d want 1 pkgs=%v", len(pkgs), pkgs)
	}
	if pkgs[0] != "./internal/session" {
		t.Fatalf("packagesForPaths[0] %q want %q", pkgs[0], "./internal/session")
	}
	if strings.Contains(pkgs[0], ":") {
		t.Fatalf("packagesForPaths[0] contains colon: %q", pkgs[0])
	}

	sibling := filepath.Clean(t.TempDir())
	outsideAbs := filepath.Join(sibling, "outside.go")
	got = canonicalizeWrittenPath(outsideAbs, workspace)
	wantOutside := filepath.ToSlash(filepath.Clean(outsideAbs))
	if got != wantOutside {
		t.Fatalf("outside: got %q want %q", got, wantOutside)
	}
}

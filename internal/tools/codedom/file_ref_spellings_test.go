package codedom

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

// A file's header is addressed however the model spells its file-scoped ref,
// with path passed as well: as get_elements prints it, with the package's
// dotted prefix a function's ref carries, or as its bare key. The dotted
// spelling missed three times in one session (2026-10-29), a round each.
func TestResolveElement_AFileScopedRefResolvesInEverySpelling(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	rel := filepath.Join("internal", "store", "vector_identity.go")
	if err := os.MkdirAll(filepath.Join(ws, filepath.Dir(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "// Package store keeps vectors.\npackage store\n\nfunc Adopt() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(ws, rel), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, ws)
	for _, ref := range []string{
		"internal/store/vector_identity.go:header",
		"internal/store.vector_identity.go:header",
		"header",
	} {
		re, err := resolveElement(ctx, ref, filepath.ToSlash(rel))
		if err != nil {
			t.Errorf("ref %q with path: %v", ref, err)
			continue
		}
		if re.elem.Key != codemodel.HeaderKey {
			t.Errorf("ref %q resolved to %q, want the header", ref, re.elem.Key)
		}
	}
	if _, err := resolveElement(ctx, "internal/store.other.go:header", filepath.ToSlash(rel)); err == nil {
		t.Error("a file-scoped ref naming another file resolved in this one")
	}
}

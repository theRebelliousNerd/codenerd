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

func TestSplitFileRef_PinsExtensionPriorityAndEarliestMatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		ref    string
		wantP  string
		wantK  string
		wantOK bool
	}{
		{"go", "internal/store/vector_identity.go:header", "internal/store/vector_identity.go", "header", true},
		{"earliest wins over later ext", "a.md:Edit main.go: the entry point", "a.md", "Edit main.go: the entry point", true},
		{"longer ext first mts over ts", "pkg/mod.mts:Helper", "pkg/mod.mts", "Helper", true},
		{"no ext", "justakey", "", "", false},
		{"empty key after colon ext still ok", "a.go:", "a.go", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotP, gotK, gotOK := splitFileRef(tc.ref)
			if gotOK != tc.wantOK || gotP != tc.wantP || gotK != tc.wantK {
				t.Errorf("splitFileRef(%q) = (%q,%q,%v), want (%q,%q,%v)", tc.ref, gotP, gotK, gotOK, tc.wantP, tc.wantK, tc.wantOK)
			}
		})
	}
}

func TestSameScriptExt_DiscriminatesBareNames(t *testing.T) {
	t.Parallel()
	if !sameScriptExt("hooks.tsx", "app/hooks.tsx") {
		t.Error("sameScriptExt(hooks.tsx, app/hooks.tsx) = false, want true")
	}
	if sameScriptExt("3fa2c1", "app/hooks.tsx") {
		t.Error("sameScriptExt(3fa2c1, app/hooks.tsx) = true, want false (no extension)")
	}
	if sameScriptExt("hooks.ts", "app/hooks.tsx") {
		t.Error("sameScriptExt(hooks.ts, app/hooks.tsx) = true, want false (different ext)")
	}
	if !sameScriptExt("HOOKS.TSX", "app/hooks.tsx") {
		t.Error("sameScriptExt is not case-insensitive for extensions")
	}
}

func TestDirRefFile_PinsGoAndScriptLangs(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"a.go", "a.py", "a.ts", "a.tsx", "a.js", "a.jsx"} {
		if !dirRefFile(rel) {
			t.Errorf("dirRefFile(%q) = false, want true", rel)
		}
	}
	if dirRefFile("doc.md") {
		t.Error("dirRefFile(doc.md) = true, want false")
	}
	if dirRefFile("notes.txt") {
		t.Error("dirRefFile(notes.txt) = true, want false")
	}
}

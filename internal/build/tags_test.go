package build

import (
	"os"
	"path/filepath"
	"testing"
)

// The tag helper fires only in codeNERD's own tree: go.mod module plus the
// vendored sqlite headers. Everywhere else it must be a no-op so generic
// workspaces keep their plain `go test ./...`.
func TestTestTagsForWorkspace_Detection(t *testing.T) {
	t.Run("codenerd tree gets sqlite_vec", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, "go.mod"), "module codenerd\n\ngo 1.24\n")
		write(t, filepath.Join(root, "sqlite_headers", "sqlite3.h"), "/* stub */")
		tags := TestTagsForWorkspace(root)
		if len(tags) != 2 || tags[0] != "-tags" || tags[1] != "sqlite_vec" {
			t.Fatalf("got %v, want [-tags sqlite_vec]", tags)
		}
	})
	t.Run("other module is untouched", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, "go.mod"), "module example.com/other\n")
		write(t, filepath.Join(root, "sqlite_headers", "sqlite3.h"), "/* stub */")
		if tags := TestTagsForWorkspace(root); tags != nil {
			t.Fatalf("got %v, want nil", tags)
		}
	})
	t.Run("codenerd module without headers is untouched", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, "go.mod"), "module codenerd\n")
		if tags := TestTagsForWorkspace(root); tags != nil {
			t.Fatalf("got %v, want nil", tags)
		}
	})
	t.Run("empty and missing dirs are untouched", func(t *testing.T) {
		if tags := TestTagsForWorkspace(""); tags != nil {
			t.Fatalf("got %v, want nil", tags)
		}
		if tags := TestTagsForWorkspace(filepath.Join(t.TempDir(), "nope")); tags != nil {
			t.Fatalf("got %v, want nil", tags)
		}
	})
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

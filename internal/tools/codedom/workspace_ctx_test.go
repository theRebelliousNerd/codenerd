package codedom

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/tools"
)

// wsCtxFor returns a context whose workspace root is the directory holding
// path.
//
// The codedom line tools now contain every path to the workspace, matching the
// file_ops family (see internal/tools/workspace_guard.go). These tests write
// into t.TempDir(), which is outside the repo, so without an explicit root
// WorkspaceRoot falls back to the process working directory and every fixture
// is correctly rejected as an escape. That rejection is the fix working; the
// tests simply have to declare their workspace, exactly as the file_ops tests
// already do (file_ops_lines_test.go:65).
func wsCtxFor(t *testing.T, path string) context.Context {
	t.Helper()
	return context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, filepath.Dir(path))
}

// TestCodedomContainsPathsToWorkspace is the security regression itself.
//
// codeNERD's own security review of internal/tools/core/file_ops.go raised this
// as its highest finding: edit_lines / insert_lines / delete_lines called raw
// os.ReadFile and os.WriteFile on the caller-supplied path at all six of their
// I/O sites, with no root, no symlink resolution and no escape check, while the
// file_ops family next door routed everything through the guard. An absolute
// path also defeats the constitution's path_traversal_protection rule, which
// tests for a literal ".." and finds none in an absolute path.
func TestCodedomContainsPathsToWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")

	// The victim file must EXIST and be writable. Pointing at a missing path
	// would let this test pass for the wrong reason — an unguarded tool would
	// fail on "no such file" and look indistinguishable from containment.
	// With the file present, an unguarded tool succeeds and rewrites it, which
	// is exactly the breach being asserted against.
	const original = "package outside\n\nfunc Untouched() {}\n"
	if err := os.WriteFile(outside, []byte(original), 0644); err != nil {
		t.Fatalf("seed victim file: %v", err)
	}

	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, workspace)

	cases := []struct {
		name string
		run  func() (string, error)
	}{
		{"edit_lines", func() (string, error) {
			return executeEditLines(ctx, map[string]any{
				"path": outside, "start_line": 1, "end_line": 1, "new_content": "pwned",
			})
		}},
		{"insert_lines", func() (string, error) {
			return executeInsertLines(ctx, map[string]any{
				"path": outside, "line_number": 1, "content": "pwned",
			})
		}},
		{"delete_lines", func() (string, error) {
			return executeDeleteLines(ctx, map[string]any{
				"path": outside, "start_line": 1, "end_line": 1,
			})
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.run()
			if err == nil {
				t.Fatalf("%s operated on a path outside the workspace", c.name)
			}
			if !errorMentionsEscape(err) {
				t.Errorf("%s failed for the wrong reason: %v", c.name, err)
			}

			// The decisive assertion: the victim file is byte-identical. An
			// error alone could still follow a partial write.
			after, readErr := os.ReadFile(outside)
			if readErr != nil {
				t.Fatalf("victim file unreadable after %s: %v", c.name, readErr)
			}
			if string(after) != original {
				t.Errorf("%s modified a file outside the workspace:\n got: %q\nwant: %q", c.name, string(after), original)
			}
		})
	}
}

func errorMentionsEscape(err error) bool {
	return err != nil && (contains(err.Error(), "escapes workspace root") || contains(err.Error(), "outside"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

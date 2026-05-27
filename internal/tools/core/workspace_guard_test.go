package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkspacePath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	// Pre-create a nested directory inside root so paths with ".." that
	// normalize back inside root can be exercised.
	nested := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "plain relative path inside root",
			input: filepath.Join(root, "file.txt"),
		},
		{
			name:    "parent traversal escaping root",
			input:   filepath.Join(root, "..", "escape.txt"),
			wantErr: true,
		},
		{
			name:    "absolute path outside root",
			input:   filepath.Join(outside, "evil.txt"),
			wantErr: true,
		},
		{
			name:  "dotdot that normalizes back inside root",
			input: filepath.Join(root, "sub", "..", "sub", "deep", "ok.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveWorkspacePath(root, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got resolved path %q", got)
				}
				if !errors.Is(err, ErrPathOutsideWorkspace) {
					t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// EvalSymlinks may rewrite root on macOS/Windows; compare via Rel.
			rel, relErr := filepath.Rel(root, got)
			if relErr != nil {
				// Fall back to symlink-resolved root before declaring failure.
				resolvedRoot, evalErr := filepath.EvalSymlinks(root)
				if evalErr != nil {
					t.Fatalf("rel(%q, %q): %v", root, got, relErr)
				}
				rel, relErr = filepath.Rel(resolvedRoot, got)
				if relErr != nil {
					t.Fatalf("rel(%q, %q): %v", resolvedRoot, got, relErr)
				}
			}
			if strings.HasPrefix(rel, "..") {
				t.Fatalf("resolved path %q is outside root %q (rel=%q)", got, root, rel)
			}
		})
	}
}

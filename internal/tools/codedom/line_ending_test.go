package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func TestLineMutationToolsPreserveCRLF(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, string) error
	}{
		{
			name: "edit lines",
			run: func(ctx context.Context, path string) error {
				_, err := executeEditLines(ctx, map[string]any{
					"path": path, "start_line": 2, "end_line": 2,
					"new_content": "two-a\ntwo-b",
				})
				return err
			},
		},
		{
			name: "insert lines",
			run: func(ctx context.Context, path string) error {
				_, err := executeInsertLines(ctx, map[string]any{
					"path": path, "after_line": 1, "content": "insert-a\ninsert-b",
				})
				return err
			},
		},
		{
			name: "delete lines",
			run: func(ctx context.Context, path string) error {
				_, err := executeDeleteLines(ctx, map[string]any{
					"path": path, "start_line": 2, "end_line": 2,
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "target.txt")
			if err := os.WriteFile(path, []byte("one\r\ntwo\r\nthree\r\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)
			if err := tt.run(ctx, filepath.Base(path)); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lfCount := strings.Count(string(data), "\n")
			crlfCount := strings.Count(string(data), "\r\n")
			if crlfCount == 0 || lfCount != crlfCount {
				t.Fatalf("expected CRLF-only file, got lf=%d crlf=%d", lfCount, crlfCount)
			}
		})
	}
}

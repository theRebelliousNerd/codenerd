package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// A read of AGENTS.md, which does not exist (the file is agents.md), was told
// "Did you mean one of these in the same directory: .env, .gitattributes,
// .gitignore, agents.md?". A dotfile's stem was empty and every name contains
// the empty string, so every dotfile matched every request -- and .env, the
// secrets file, was put in front of the model as a file to read.
func TestReadFile_NotFoundSuggestsOnlyPlausibleNonSecretFiles(t *testing.T) {
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	for _, n := range []string{".env", ".gitattributes", ".gitignore", "agents.md", "go.mod"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)

	_, err := executeReadFile(ctx, map[string]any{"path": "AGENTS.md"})
	if err == nil {
		t.Fatal("reading a missing file succeeded")
	}
	msg := err.Error()
	if !strings.Contains(msg, "agents.md") {
		t.Errorf("agents.md was not suggested for AGENTS.md:\n%s", msg)
	}
	for _, bad := range []string{".env", ".gitattributes", ".gitignore", "go.mod"} {
		if strings.Contains(msg, bad) {
			t.Errorf("%s was suggested for AGENTS.md:\n%s", bad, msg)
		}
	}

	// Nothing plausible: the whole listing, still without the secret file,
	// and saying one was left out.
	_, err = executeReadFile(ctx, map[string]any{"path": "zzz_unrelated.txt"})
	msg = err.Error()
	if strings.Contains(msg, ".env") || !strings.Contains(msg, "1 secret file") || !strings.Contains(msg, ".gitignore") {
		t.Errorf("the not-found listing offered the secret file or hid that it left one out:\n%s", msg)
	}

	// A directory read lists its contents the same way.
	_, err = executeReadFile(ctx, map[string]any{"path": "."})
	msg = err.Error()
	if strings.Contains(msg, ".env") || !strings.Contains(msg, "1 secret file") || !strings.Contains(msg, "Contents (4)") {
		t.Errorf("the directory listing offered the secret file:\n%s", msg)
	}
}

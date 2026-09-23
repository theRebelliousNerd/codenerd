package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A workspace-wide grep never returns a secret file's lines. The walk skipped
// hidden directories but not hidden files, so a pattern over the root read
// .env like any other file and sent its matches to the model.
func TestGrep_NeverReturnsASecretFilesLines(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".env", "XAI_API_KEY=sk-not-a-real-key\n")
	write("deploy/prod.env", "XAI_API_KEY=sk-also-not-real\n")
	write("internal/app/app.go", "package app\n\n// XAI_API_KEY is read from the environment.\n")

	out, err := executeGrep(wsCtx(dir), map[string]any{"pattern": "XAI_API_KEY", "path": "."})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if strings.Contains(out, "sk-not-a-real-key") || strings.Contains(out, "sk-also-not-real") {
		t.Fatalf("grep returned a secret file's contents:\n%s", out)
	}
	if !strings.Contains(out, "app.go") {
		t.Fatalf("grep must still search ordinary files; got:\n%s", out)
	}

	// Aimed straight at the secret, it refuses rather than answering.
	if out, err := executeGrep(wsCtx(dir), map[string]any{"pattern": "KEY", "path": ".env"}); err == nil && strings.Contains(out, "sk-not-a-real-key") {
		t.Fatalf("grep of .env returned its contents:\n%s", out)
	}
}

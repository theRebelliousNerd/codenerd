package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mixed: the first two lines end in LF, the rest in CRLF.
const mixedText = "alpha\nbeta\r\ngamma\r\ndelta\r\n"

func TestEditFile_MixedEndingsKeepUntouchedLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", dir)
	path := filepath.Join(dir, "mixed.txt")
	if err := os.WriteFile(path, []byte(mixedText), 0o644); err != nil {
		t.Fatal(err)
	}

	// An edit in the CRLF region, then one in the LF region.
	if _, err := executeEditFile(context.Background(), map[string]any{"path": path, "old_text": "gamma", "new_text": "GAMMA"}); err != nil {
		t.Fatal(err)
	}
	if _, err := executeEditFile(context.Background(), map[string]any{"path": path, "old_text": "alpha", "new_text": "ALPHA"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if want := "ALPHA\nbeta\r\nGAMMA\r\ndelta\r\n"; string(got) != want {
		t.Fatalf("edit_file moved bytes outside its lines: got %q want %q", got, want)
	}

	// A multi-line replacement across the boundary pairs lines in order.
	if _, err := executeEditFile(context.Background(), map[string]any{"path": path, "old_text": "ALPHA\nbeta\n", "new_text": "one\ntwo\nthree\n"}); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if want := "one\ntwo\r\nthree\r\nGAMMA\r\ndelta\r\n"; string(got) != want {
		t.Fatalf("edit_file across the boundary: got %q want %q", got, want)
	}
}

func TestWriteFile_OverMixedFileKeepsUnchangedLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", dir)
	path := filepath.Join(dir, "mixed.txt")
	if err := os.WriteFile(path, []byte(mixedText), 0o644); err != nil {
		t.Fatal(err)
	}

	content := strings.ReplaceAll(mixedText, "\r\n", "\n")
	content = strings.Replace(content, "delta", "DELTA", 1)
	if _, err := executeWriteFile(context.Background(), map[string]any{"path": path, "content": content}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if want := "alpha\nbeta\r\ngamma\r\nDELTA\r\n"; string(got) != want {
		t.Fatalf("write_file moved bytes outside its lines: got %q want %q", got, want)
	}

	// A new file keeps the content as given.
	fresh := filepath.Join(dir, "fresh.txt")
	if _, err := executeWriteFile(context.Background(), map[string]any{"path": fresh, "content": "x\ny\n"}); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(fresh)
	if string(got) != "x\ny\n" {
		t.Fatalf("new file: got %q", got)
	}
}

package session

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tactile"
)

func TestFormatWrittenGoFiles(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	uglyOriginal := "package a\nfunc F() {}\nfunc G() {}\n\n\n"
	fineSrc := "package a\n\nvar X = 1\n"
	fineFormatted, err := format.Source([]byte(fineSrc))
	if err != nil {
		t.Fatalf("failed to format fine source: %v", err)
	}
	brokenContent := "package a\nfunc (\n"
	notesContent := "func (\n"

	initial := map[string][]byte{
		"a/ugly.go":   []byte(uglyOriginal),
		"a/fine.go":   fineFormatted,
		"a/broken.go": []byte(brokenContent),
		"a/notes.txt": []byte(notesContent),
	}
	for rel, data := range initial {
		if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(rel)), data, 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	got := formatWrittenGoFiles(ws, []string{"a/ugly.go", "a/fine.go", "a/broken.go", "a/notes.txt", "a/missing.go"})

	if len(got) != 1 || got[0] != "a/ugly.go" {
		t.Fatalf("formatWrittenGoFiles() = %q, want %q", got, []string{"a/ugly.go"})
	}

	expectedUgly, err := format.Source([]byte(uglyOriginal))
	if err != nil {
		t.Fatalf("failed to format ugly source: %v", err)
	}
	uglyOnDisk, err := os.ReadFile(filepath.Join(ws, "a", "ugly.go"))
	if err != nil {
		t.Fatalf("failed to read ugly.go: %v", err)
	}
	if !bytes.Equal(uglyOnDisk, expectedUgly) {
		t.Errorf("ugly.go on disk = %q, want %q", uglyOnDisk, expectedUgly)
	}

	for _, rel := range []string{"a/fine.go", "a/broken.go", "a/notes.txt"} {
		onDisk, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("failed to read %s: %v", rel, err)
		}
		if !bytes.Equal(onDisk, initial[rel]) {
			t.Errorf("%s changed: got %q, want %q", rel, onDisk, initial[rel])
		}
	}
}

func TestFormatWrittenGoFiles_KeepsCRLF(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	fineCRLF := "package a\r\n\r\nvar X = 1\r\n"
	uglyCRLF := "package a\r\nfunc F() {}\r\nfunc G() {}\r\n"
	initial := map[string][]byte{
		"a/crlf_fine.go": []byte(fineCRLF),
		"a/crlf_ugly.go": []byte(uglyCRLF),
	}
	for rel, data := range initial {
		if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(rel)), data, 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	got := formatWrittenGoFiles(ws, []string{"a/crlf_fine.go", "a/crlf_ugly.go"})
	if len(got) != 1 || got[0] != "a/crlf_ugly.go" {
		t.Fatalf("formatWrittenGoFiles() = %q, want %q", got, []string{"a/crlf_ugly.go"})
	}

	fineOnDisk, err := os.ReadFile(filepath.Join(ws, "a", "crlf_fine.go"))
	if err != nil {
		t.Fatalf("failed to read crlf_fine.go: %v", err)
	}
	if !bytes.Equal(fineOnDisk, initial["a/crlf_fine.go"]) {
		t.Errorf("crlf_fine.go changed: got %q, want %q", fineOnDisk, initial["a/crlf_fine.go"])
	}

	uglyOnDisk, err := os.ReadFile(filepath.Join(ws, "a", "crlf_ugly.go"))
	if err != nil {
		t.Fatalf("failed to read crlf_ugly.go: %v", err)
	}
	if !strings.Contains(string(uglyOnDisk), "\r\n") {
		t.Errorf("crlf_ugly.go has no CRLF: %q", uglyOnDisk)
	}
	for i, b := range uglyOnDisk {
		if b == '\n' && (i == 0 || uglyOnDisk[i-1] != '\r') {
			t.Fatalf("crlf_ugly.go has lone LF at byte %d: %q", i, uglyOnDisk)
		}
	}
	uglyLF := "package a\nfunc F() {}\nfunc G() {}\n"
	formattedLF, err := format.Source([]byte(uglyLF))
	if err != nil {
		t.Fatalf("failed to format LF ugly source: %v", err)
	}
	expected := tactile.NormalizeLineEnding(string(formattedLF), "\r\n")
	if string(uglyOnDisk) != expected {
		t.Errorf("crlf_ugly.go = %q, want %q", uglyOnDisk, expected)
	}
}

package tactile

import (
	"os"
	"path/filepath"
	"testing"
)

func assertCRLFOnly(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lfCount := 0
	crlfCount := 0
	for i, b := range data {
		if b != '\n' {
			continue
		}
		lfCount++
		if i > 0 && data[i-1] == '\r' {
			crlfCount++
		}
	}
	if crlfCount == 0 || lfCount != crlfCount {
		t.Fatalf("expected CRLF-only file, got lf=%d crlf=%d", lfCount, crlfCount)
	}
}

func TestFileEditorMutationsPreserveCRLF(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FileEditor, string) error
	}{
		{
			name: "write",
			mutate: func(editor *FileEditor, path string) error {
				_, err := editor.WriteFile(path, []string{"package p", "", "func B() {}"})
				return err
			},
		},
		{
			name: "edit lines",
			mutate: func(editor *FileEditor, path string) error {
				_, err := editor.EditLines(path, 3, 3, []string{"func B() {}"})
				return err
			},
		},
		{
			name: "insert lines",
			mutate: func(editor *FileEditor, path string) error {
				_, err := editor.InsertLines(path, 2, []string{"// inserted"})
				return err
			},
		},
		{
			name: "delete lines",
			mutate: func(editor *FileEditor, path string) error {
				_, err := editor.DeleteLines(path, 2, 2)
				return err
			},
		},
		{
			name: "replace element",
			mutate: func(editor *FileEditor, path string) error {
				_, err := editor.ReplaceElement(path, 3, 3, "func B() {\n\treturn\n}")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "target.go")
			if err := os.WriteFile(path, []byte("package p\r\n\r\nfunc A() {}\r\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			editor := NewFileEditor()
			editor.SetWorkingDir(dir)
			if err := tt.mutate(editor, filepath.Base(path)); err != nil {
				t.Fatal(err)
			}
			assertCRLFOnly(t, path)
		})
	}
}

func TestExistingLineEndingDistinguishesMissingFromUnreadable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.go")
	if _, exists, err := ExistingLineEnding(missing); err != nil || exists {
		t.Fatalf("missing file: exists=%v err=%v", exists, err)
	}

	if _, exists, err := ExistingLineEnding(t.TempDir()); err == nil || exists {
		t.Fatalf("directory must be an existing-path read error: exists=%v err=%v", exists, err)
	}
}

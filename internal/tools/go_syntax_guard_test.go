package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectUnparseableGo(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		content   string
		wantErr   bool
		errPrefix string
	}{
		{name: "valid go ok", path: "foo.go", content: "package foo\n", wantErr: false},
		{name: "invalid go refused", path: "foo.go", content: "placeholder\n", wantErr: true, errPrefix: "refusing to write foo.go: Go syntax invalid: "},
		{name: "txt garbage ok", path: "notes.txt", content: "placeholder\nfunc (\n", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectUnparseableGo(tt.path, []byte(tt.content))
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("RejectUnparseableGo(%q) = %v, want nil", tt.path, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("RejectUnparseableGo(%q) = nil, want error", tt.path)
			}
			if !strings.HasPrefix(err.Error(), tt.errPrefix) {
				t.Errorf("RejectUnparseableGo(%q) error = %q, want prefix %q", tt.path, err.Error(), tt.errPrefix)
			}
		})
	}
}

func TestRejectGoSyntaxRegression(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		before    string
		after     string
		wantErr   bool
		errPrefix string
	}{
		{name: "valid to valid nil", path: "foo.go", before: "package foo\n", after: "package foo\n", wantErr: false},
		{name: "valid to invalid error", path: "foo.go", before: "package foo\n", after: "placeholder\n", wantErr: true, errPrefix: "refusing to write"},
		{name: "invalid to invalid nil", path: "foo.go", before: "placeholder\n", after: "still broken (((\n", wantErr: false},
		{name: "invalid to valid nil", path: "foo.go", before: "placeholder\n", after: "package foo\n", wantErr: false},
		{name: "txt garbage to garbage nil", path: "notes.txt", before: "placeholder\nfunc (\n", after: "more garbage (((\n", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectGoSyntaxRegression(tt.path, []byte(tt.before), []byte(tt.after))
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("RejectGoSyntaxRegression(%q) = %v, want nil", tt.path, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("RejectGoSyntaxRegression(%q) = nil, want error", tt.path)
			}
			if !strings.HasPrefix(err.Error(), tt.errPrefix) {
				t.Errorf("RejectGoSyntaxRegression(%q) error = %q, want prefix %q", tt.path, err.Error(), tt.errPrefix)
			}
		})
	}
}

// Lines.go calls RejectGoSyntaxRegression with no workspace. The check has
// to refuse an added ignore, spare a file that already had one, spare a
// mention after the package clause, and spare a sqlite_vec constraint in
// this module (the walk to go.mod is what supplies that tag).
func TestRejectGoSyntaxRegression_BuildExclusion(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "1")
	compiled := "package p\n\nfunc A() int { return 1 }\n"
	ignored := "//go:build ignore\n\n" + compiled
	mention := "package p\n\nfunc A() int {\n\t// Never add //go:build ignore here.\n\treturn 1\n}\n"

	t.Run("added ignore", func(t *testing.T) {
		err := RejectGoSyntaxRegression("foo.go", []byte(compiled), []byte(ignored))
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the added ignore refused", err)
		}
	})
	t.Run("already ignored", func(t *testing.T) {
		next := "//go:build ignore\n\npackage p\n\nfunc B() int { return 1 }\n"
		if err := RejectGoSyntaxRegression("foo.go", []byte(ignored), []byte(next)); err != nil {
			t.Fatalf("a file that already excluded was refused: %v", err)
		}
	})
	t.Run("mention", func(t *testing.T) {
		if err := RejectGoSyntaxRegression("foo.go", []byte(compiled), []byte(mention)); err != nil {
			t.Fatalf("a mention after the package clause was refused: %v", err)
		}
	})
	t.Run("sqlite_vec in this module", func(t *testing.T) {
		// path is relative, so the module walk starts at the package directory
		// go test runs in, which is inside this repo. sqlite_vec is a gate tag.
		next := "//go:build sqlite_vec\n\n" + compiled
		if err := RejectGoSyntaxRegression("foo.go", []byte(compiled), []byte(next)); err != nil {
			t.Fatalf("sqlite_vec in this module was refused: %v", err)
		}
	})
	t.Run("sqlite_vec outside this module", func(t *testing.T) {
		outside := filepath.Join(t.TempDir(), "x.go")
		next := "//go:build sqlite_vec\n\n" + compiled
		err := RejectGoSyntaxRegression(outside, []byte(compiled), []byte(next))
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want sqlite_vec refused where the module has no gate tag", err)
		}
	})
	t.Run("broken repair adds ignore", func(t *testing.T) {
		before := "package p\n\nfunc (\n"
		after := "//go:build ignore\n\npackage p\n\nfunc (\n"
		err := RejectGoSyntaxRegression("foo.go", []byte(before), []byte(after))
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want a repair that adds ignore refused", err)
		}
	})
}

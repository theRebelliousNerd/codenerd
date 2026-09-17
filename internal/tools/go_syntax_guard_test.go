package tools

import (
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

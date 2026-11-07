package build

import (
	"path/filepath"
	"slices"
	"testing"
)

// A run may exec a rebuilt binary of itself only in the tree that binary is
// built from: go.mod names the module codenerd and the tree holds
// cmd/nerd. A nerd.md line, a lookalike module or a missing main package do
// not qualify.
func TestIsCodeNERDModule(t *testing.T) {
	tree := func(t *testing.T, mod string, withMain bool) string {
		t.Helper()
		root := t.TempDir()
		write(t, filepath.Join(root, "go.mod"), "module "+mod+"\n\ngo 1.24\n")
		if withMain {
			write(t, filepath.Join(root, "cmd", "nerd", "main.go"), "package main\n\nfunc main() {}\n")
		}
		write(t, filepath.Join(root, "nerd.md"), "# self\n\nThis workspace is codeNERD itself.\n")
		return root
	}
	for _, tc := range []struct {
		name     string
		mod      string
		withMain bool
		want     bool
	}{
		{"the module itself", "codenerd", true, true},
		{"a fork under a host path", "example.org/someone/codenerd", true, true},
		{"quoted module path", `"codenerd"`, true, true},
		{"module without the main package", "codenerd", false, false},
		{"lookalike module", "example.com/mycodenerdtools", true, false},
		{"another module", "example.com/other", true, false},
	} {
		if got := IsCodeNERDModule(tree(t, tc.mod, tc.withMain)); got != tc.want {
			t.Errorf("%s: IsCodeNERDModule = %v, want %v", tc.name, got, tc.want)
		}
	}
	if IsCodeNERDModule("") || IsCodeNERDModule(filepath.Join(t.TempDir(), "nope")) {
		t.Error("an empty or missing workspace is not the codeNERD module")
	}
}

// BuildBinary used to wrap the tag flags TestTagsForWorkspace already returns
// in a second -tags, so go saw the tags "-tags" and "sqlite_vec". The argv
// passes the flags through once.
func TestBuildBinaryArgs_PassesTheWorkspaceTagFlagsThroughOnce(t *testing.T) {
	codenerd := t.TempDir()
	write(t, filepath.Join(codenerd, "go.mod"), "module codenerd\n")
	write(t, filepath.Join(codenerd, "sqlite_headers", "sqlite3.h"), "/* stub */")
	if got, want := BuildBinaryArgs(codenerd, "out/nerd", "./cmd/nerd"), []string{"build", "-tags", "sqlite_vec", "-o", "out/nerd", "./cmd/nerd"}; !slices.Equal(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
	other := t.TempDir()
	write(t, filepath.Join(other, "go.mod"), "module example.com/other\n")
	if got, want := BuildBinaryArgs(other, "out/app", "./cmd/app"), []string{"build", "-o", "out/app", "./cmd/app"}; !slices.Equal(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

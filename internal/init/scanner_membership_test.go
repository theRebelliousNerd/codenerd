package init

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestEntryPointsUseWorkspaceMembershipForSubmodule(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".nerd/config.json", `{"world":{"ignore_patterns":["module/main.go","module/app.py","module/cmd/ignored/","module/src/"]}}`)
	write("module/main.go", "package main\nfunc main() {}\n")
	write("module/app.py", `if __name__ == "__main__": pass`)
	write("module/cmd/ignored/main.go", "package main\nfunc main() {}\n")
	write("module/cmd/kept/main.go", "package main\nfunc main() {}\n")
	write("module/src/ignored.py", `if __name__ == "__main__": pass`)
	i := &Initializer{config: InitConfig{Workspace: root}}
	got := i.detectEntryPointsForRoot(filepath.Join(root, "module"))
	want := filepath.Join("cmd", "kept", "main.go")
	if !slices.Equal(got, []string{want}) {
		t.Fatalf("entry points = %v, want [%s]", got, want)
	}
}

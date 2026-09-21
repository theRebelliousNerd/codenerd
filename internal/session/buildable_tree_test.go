package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newBuildableTreeWorkspace(t *testing.T) (*Executor, string) {
	t.Helper()
	ws := t.TempDir()
	for name, content := range map[string]string{
		"go.mod": "module buildable\n\ngo 1.24\n",
		"a.go":   "package buildable\n\nfunc A() int { return 1 }\n",
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	e := &Executor{config: DefaultExecutorConfig()}
	e.config.WorkspaceRoot = ws
	return e, ws
}

// A turn that gives up with the tree uncompilable has its files put back and
// its attempt kept: observed 2026-09-21, a repair loop's third attempt left a
// call to a function it had deleted, and the repository did not build.
func TestLeaveBuildableTree_RestoresAnUncompilableAttemptAndKeepsIt(t *testing.T) {
	e, ws := newBuildableTreeWorkspace(t)
	original, _ := os.ReadFile(filepath.Join(ws, "a.go"))
	result := &ExecutionResult{
		WrittenPaths: []string{"a.go", "b.go"},
		PreWriteContents: map[string]PreImage{
			"a.go": {Existed: true, Content: string(original)},
			"b.go": {},
		},
	}
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package buildable\n\nfunc A() int { return missing() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "b.go"), []byte("package buildable\n\nvar B = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	note := e.leaveBuildableTree(context.Background(), result)
	if !strings.Contains(note, "restored as the turn found them") {
		t.Fatalf("note = %q, want it to say the files were restored", note)
	}
	got, err := os.ReadFile(filepath.Join(ws, "a.go"))
	if err != nil || string(got) != string(original) {
		t.Fatalf("a.go was not put back: %q (err=%v)", got, err)
	}
	if _, err := os.Stat(filepath.Join(ws, "b.go")); !os.IsNotExist(err) {
		t.Fatalf("b.go, which the turn created, is still there (err=%v)", err)
	}
	patches, _ := filepath.Glob(filepath.Join(ws, ".nerd", "attempts", "attempt_*.patch.md"))
	if len(patches) != 1 {
		t.Fatalf("the attempt was not kept: %d patch file(s)", len(patches))
	}
	saved, _ := os.ReadFile(patches[0])
	if !strings.Contains(string(saved), "missing()") {
		t.Fatalf("the saved attempt does not hold the edit it undid:\n%s", saved)
	}
	if v := verifyBuild(context.Background(), ws, nil); v.Verdict() == VerifyFailed {
		t.Fatalf("the workspace still does not build:\n%s", v.Output)
	}
}

// A turn that gives up with the tree still compiling keeps its edits: red
// tests are a starting point, an uncompilable tree is an outage.
func TestLeaveBuildableTree_LeavesACompilingAttemptAlone(t *testing.T) {
	e, ws := newBuildableTreeWorkspace(t)
	original, _ := os.ReadFile(filepath.Join(ws, "a.go"))
	edited := "package buildable\n\nfunc A() int { return 2 }\n"
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{
		WrittenPaths:     []string{"a.go"},
		PreWriteContents: map[string]PreImage{"a.go": {Existed: true, Content: string(original)}},
	}
	if note := e.leaveBuildableTree(context.Background(), result); note != "" {
		t.Fatalf("a compiling attempt was touched: %q", note)
	}
	got, _ := os.ReadFile(filepath.Join(ws, "a.go"))
	if string(got) != edited {
		t.Fatalf("a.go = %q, want the turn's edit left in place", got)
	}
}

// A workspace that is not a Go module has no build to break: `go build ./...`
// fails there whatever the turn did, and that is not evidence against it.
func TestLeaveBuildableTree_IgnoresAWorkspaceThatIsNotAGoModule(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Executor{config: DefaultExecutorConfig()}
	e.config.WorkspaceRoot = ws
	result := &ExecutionResult{WrittenPaths: []string{"a.txt"}, PreWriteContents: map[string]PreImage{"a.txt": {}}}
	if note := e.leaveBuildableTree(context.Background(), result); note != "" {
		t.Fatalf("a non-Go workspace was touched: %q", note)
	}
	if _, err := os.Stat(filepath.Join(ws, "a.txt")); err != nil {
		t.Fatalf("the turn's file was removed from a workspace with no build to break: %v", err)
	}
}

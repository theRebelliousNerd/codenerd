package codedom_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func readRel(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runErr(reg *tools.Registry, name string, args map[string]any) error {
	_, err := reg.Execute(context.Background(), name, args)
	return err
}

// The audit's failing test for G3: a package b that only aliases a.T, and a
// package c that uses b.T. Repointing b.T to a.T must leave c importing a,
// not b, every file parsing, and b.T with no uses left.
var aliasFixture = map[string]string{
	"go.mod":   "module fixture\n\ngo 1.24\n",
	"a/a.go":   "package a\n\n// T is the real type.\ntype T struct{ N int }\n",
	"b/b.go":   "package b\n\nimport \"fixture/a\"\n\n// T is an alias kept for old callers.\ntype T = a.T\n",
	"c/c.go":   "package c\n\nimport \"fixture/b\"\n\n// Use takes a T.\nfunc Use(t b.T) int { return t.N }\n\nfunc Make() b.T { return b.T{} }\n",
	"d/d.go":   "package d\n\nimport (\n\t\"fmt\"\n\n\t\"fixture/b\"\n)\n\nfunc Show(t b.T) string { return fmt.Sprint(t) }\n",
	"a/lib.go": "package a\n\n// Helper helps.\nfunc Helper() int { return 1 }\n",
}

func TestRepoint_MovesEveryUseAndDerivesImports(t *testing.T) {
	reg, root := structWorkspace(t, aliasFixture)
	if err := runErr(reg, "repoint", map[string]any{"from": "b.T", "to": "a.T", "paths": []any{"c/c.go"}}); err == nil || !strings.Contains(err.Error(), "d/d.go") {
		t.Fatalf("a use outside the declared write set must be refused naming its file: %v", err)
	}
	out := run(t, reg, "repoint", map[string]any{"from": "b.T", "to": "a.T", "paths": []any{"c/c.go", "d/d.go"}})
	if !strings.Contains(out, "4 uses rewritten in 2 files") {
		t.Fatalf("repoint answer:\n%s", out)
	}
	c := readRel(t, root, "c/c.go")
	if !strings.Contains(c, `import "fixture/a"`) || strings.Contains(c, "fixture/b") || strings.Contains(c, "b.T") || strings.Count(c, "a.T") != 3 {
		t.Fatalf("c must import a, not b:\n%s", c)
	}
	d := readRel(t, root, "d/d.go")
	if !strings.Contains(d, `"fixture/a"`) || strings.Contains(d, `"fixture/b"`) || !strings.Contains(d, `"fmt"`) {
		t.Fatalf("d keeps fmt, swaps b for a:\n%s", d)
	}
	// b.T has no uses left, so it deletes without a repoint.
	if err := runErr(reg, "delete_element", map[string]any{"path": "b/b.go", "ref": "b.T"}); err != nil {
		t.Fatalf("b.T is now unused: %v", err)
	}
	if b := readRel(t, root, "b/b.go"); strings.Contains(b, "type T") || strings.Contains(b, "fixture/a") {
		t.Fatalf("the alias and the import it needed are gone:\n%s", b)
	}
}

func TestDeleteElement_RefusesWhileUsedAndRepointsInTheSameCall(t *testing.T) {
	reg, root := structWorkspace(t, aliasFixture)
	err := runErr(reg, "delete_element", map[string]any{"path": "b/b.go", "ref": "b.T"})
	if err == nil || !strings.Contains(err.Error(), "uses remain") || !strings.Contains(err.Error(), "c/c.go:6") {
		t.Fatalf("a used element is not deleted, and the uses are listed: %v", err)
	}
	out := run(t, reg, "delete_element", map[string]any{
		"path": "b/b.go", "ref": "b.T", "replace_with": "a.T", "paths": []any{"c/c.go", "d/d.go"},
	})
	if !strings.Contains(out, "its uses repointed to a.T") {
		t.Fatalf("answer:\n%s", out)
	}
	if strings.Contains(readRel(t, root, "b/b.go"), "type T") || strings.Contains(readRel(t, root, "c/c.go"), "b.T") {
		t.Fatal("the element is deleted and its uses repointed in one call")
	}
}

func TestInsertElement_DerivesImportsFromTheWorkspace(t *testing.T) {
	reg, root := structWorkspace(t, aliasFixture)
	out := run(t, reg, "insert_element", map[string]any{
		"path": "c/c.go", "anchor": "end",
		"source": "// Twice doubles.\nfunc Twice() int { return a.Helper() * 2 }",
	})
	c := readRel(t, root, "c/c.go")
	if !strings.Contains(c, `"fixture/a"`) || !strings.Contains(out, "imports added: fixture/a") {
		t.Fatalf("a newly used qualifier is imported:\n%s\n%s", out, c)
	}
	if !strings.Contains(out, "c.Twice  function") {
		t.Fatalf("the answer names the new element by ref:\n%s", out)
	}
}

// The audit's failing test for G6.
func TestCreateFile_ChecksThePackageClauseAndDerivesImports(t *testing.T) {
	reg, root := structWorkspace(t, aliasFixture)
	if err := runErr(reg, "create_file", map[string]any{"path": "c/extra_test.go", "source": "package wrong\n\nfunc TestX() {}\n"}); err == nil || !strings.Contains(err.Error(), "package c") {
		t.Fatalf("a package clause that does not match the directory is refused: %v", err)
	}
	out := run(t, reg, "create_file", map[string]any{
		"path":   "c/extra_test.go",
		"source": "package c\n\nimport \"testing\"\n\nfunc TestUse(t *testing.T) {\n\tif Use(a.T{N: 2}) != 2 {\n\t\tt.Fatal(\"x\")\n\t}\n}\n",
	})
	if !strings.Contains(out, "c.TestUse") || !strings.Contains(out, "imports added: fixture/a") {
		t.Fatalf("create_file answers with the new elements and derived imports:\n%s", out)
	}
	if err := runErr(reg, "create_file", map[string]any{"path": "c/extra_test.go", "source": "package c\n"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("an existing file is changed by the element verbs, not recreated: %v", err)
	}
	if !strings.Contains(readRel(t, root, "c/extra_test.go"), "\"fixture/a\"") {
		t.Fatal("the written file carries the derived import")
	}
}

package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRemovedTestFunctions_DetectsDeletion(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	now := "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "x_test.go"), []byte(now), 0o644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	pre := "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n\nfunc TestGone(t *testing.T) {}\n"
	preWrite := map[string]PreImage{filepath.Join("a", "x_test.go"): existed(pre)}
	got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWrite)
	want := []string{filepath.Join("a", "x_test.go") + ":TestGone"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("removedTestFunctions = %v, want %v", got, want)
	}
}

// guardWorkspace writes files (slash paths) under a fresh workspace.
func guardWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

const (
	guardPreA = "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n\nfunc TestContract(t *testing.T) {}\n"
	guardNowA = "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
)

// External audit F6 (2026-09-19): deleting alpha.TestContract was accepted
// because an unrelated beta.TestContract existed. A namesake in another
// package is a different test; the deletion is a deletion.
func TestRemovedTestFunctions_ANamesakeInAnotherPackageIsNotAMove(t *testing.T) {
	ws := guardWorkspace(t, map[string]string{
		"a/x_test.go": guardNowA,
		"b/y_test.go": "package b\n\nimport \"testing\"\n\nfunc TestContract(t *testing.T) {}\n",
	})
	got := removedTestFunctions(ws, []string{"a/x_test.go"}, map[string]PreImage{"a/x_test.go": existed(guardPreA)})
	if want := []string{"a/x_test.go:TestContract"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("removedTestFunctions = %v, want %v", got, want)
	}
}

// A test that moved to another file of its own package is not removed --
// including the package's external test package, which go test runs with it.
func TestRemovedTestFunctions_AMoveWithinThePackageIsNotRemoved(t *testing.T) {
	for name, movedTo := range map[string]string{
		"same package":          "package a\n\nimport \"testing\"\n\nfunc TestContract(t *testing.T) {}\n",
		"external test package": "package a_test\n\nimport \"testing\"\n\nfunc TestContract(t *testing.T) {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			ws := guardWorkspace(t, map[string]string{"a/x_test.go": guardNowA, "a/z_test.go": movedTo})
			got := removedTestFunctions(ws, []string{"a/x_test.go"}, map[string]PreImage{"a/x_test.go": existed(guardPreA)})
			if len(got) != 0 {
				t.Fatalf("removedTestFunctions = %v, want empty: the test moved within its package", got)
			}
		})
	}
}

// A copy behind a build constraint the default build excludes does not run:
// "moving" a test there removes it.
func TestRemovedTestFunctions_ACopyTheBuildExcludesIsRemoved(t *testing.T) {
	ws := guardWorkspace(t, map[string]string{
		"a/x_test.go": guardNowA,
		"a/z_test.go": "//go:build ignore\n\npackage a\n\nimport \"testing\"\n\nfunc TestContract(t *testing.T) {}\n",
	})
	got := removedTestFunctions(ws, []string{"a/x_test.go"}, map[string]PreImage{"a/x_test.go": existed(guardPreA)})
	if want := []string{"a/x_test.go:TestContract"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("removedTestFunctions = %v, want %v", got, want)
	}
}

func TestRemovedTestFunctions_RenameIsReported(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	now := "package a\n\nimport \"testing\"\n\nfunc TestNewName(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "x_test.go"), []byte(now), 0o644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	pre := "package a\n\nimport \"testing\"\n\nfunc TestOldName(t *testing.T) {}\n"
	preWrite := map[string]PreImage{filepath.Join("a", "x_test.go"): existed(pre)}
	got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWrite)
	want := []string{filepath.Join("a", "x_test.go") + ":TestOldName"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("removedTestFunctions = %v, want %v", got, want)
	}
}

func TestRemovedTestFunctions_IgnoresNonTestFilesAndUnparseable(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	nowNonTest := "package a\n\nfunc Kept() {}\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "foo.go"), []byte(nowNonTest), 0o644); err != nil {
		t.Fatalf("write foo.go: %v", err)
	}
	preNonTest := "package a\n\nfunc Kept() {}\n\nfunc Gone() {}\n"
	preWriteNonTest := map[string]PreImage{filepath.Join("a", "foo.go"): existed(preNonTest)}
	if got := removedTestFunctions(ws, []string{filepath.Join("a", "foo.go")}, preWriteNonTest); len(got) != 0 {
		t.Fatalf("removedTestFunctions non-test = %v, want empty", got)
	}
	nowBad := "package a\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "x_test.go"), []byte(nowBad), 0o644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	preBad := "package a\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {}\n"
	preWriteBad := map[string]PreImage{filepath.Join("a", "x_test.go"): existed(preBad)}
	if got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWriteBad); len(got) != 0 {
		t.Fatalf("removedTestFunctions unparseable = %v, want empty", got)
	}
}

func TestVerifyCompletedToolTurn_RemovedTestFails(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	kept := "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "x_test.go"), []byte(kept), 0o644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	pre := "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n\nfunc TestGone(t *testing.T) {}\n"
	preWrite := map[string]PreImage{filepath.Join("a", "x_test.go"): existed(pre)}
	got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWrite)
	if len(got) != 1 {
		t.Fatalf("removedTestFunctions = %v, want one removal", got)
	}
	if got[0] != filepath.Join("a", "x_test.go")+":TestGone" {
		t.Fatalf("removedTestFunctions = %v, want %v", got, filepath.Join("a", "x_test.go")+":TestGone")
	}
}

func TestVerifyCompletedToolTurn_KeptTestPasses(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	kept := "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "x_test.go"), []byte(kept), 0o644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	preWrite := map[string]PreImage{filepath.Join("a", "x_test.go"): existed(kept)}
	if got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWrite); len(got) != 0 {
		t.Fatalf("removedTestFunctions = %v, want empty", got)
	}
}

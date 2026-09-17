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
	preWrite := map[string]string{filepath.Join("a", "x_test.go"): pre}
	got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWrite)
	want := []string{filepath.Join("a", "x_test.go") + ":TestGone"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("removedTestFunctions = %v, want %v", got, want)
	}
}

func TestRemovedTestFunctions_MovedTestIsNotRemoved(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "b"), 0o755); err != nil {
		t.Fatalf("mkdir b: %v", err)
	}
	nowA := "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "x_test.go"), []byte(nowA), 0o644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	moved := "package b\n\nimport \"testing\"\n\nfunc TestMoved(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(ws, "b", "y_test.go"), []byte(moved), 0o644); err != nil {
		t.Fatalf("write y_test.go: %v", err)
	}
	pre := "package a\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n\nfunc TestMoved(t *testing.T) {}\n"
	preWrite := map[string]string{filepath.Join("a", "x_test.go"): pre}
	got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWrite)
	if len(got) != 0 {
		t.Fatalf("removedTestFunctions = %v, want empty", got)
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
	preWrite := map[string]string{filepath.Join("a", "x_test.go"): pre}
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
	preWriteNonTest := map[string]string{filepath.Join("a", "foo.go"): preNonTest}
	if got := removedTestFunctions(ws, []string{filepath.Join("a", "foo.go")}, preWriteNonTest); len(got) != 0 {
		t.Fatalf("removedTestFunctions non-test = %v, want empty", got)
	}
	nowBad := "package a\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {\n"
	if err := os.WriteFile(filepath.Join(ws, "a", "x_test.go"), []byte(nowBad), 0o644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	preBad := "package a\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {}\n"
	preWriteBad := map[string]string{filepath.Join("a", "x_test.go"): preBad}
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
	preWrite := map[string]string{filepath.Join("a", "x_test.go"): pre}
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
	preWrite := map[string]string{filepath.Join("a", "x_test.go"): kept}
	if got := removedTestFunctions(ws, []string{filepath.Join("a", "x_test.go")}, preWrite); len(got) != 0 {
		t.Fatalf("removedTestFunctions = %v, want empty", got)
	}
}

package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
)

func exclusionRegistry(t *testing.T, dir string, guard bool) *tools.Registry {
	t.Helper()
	reg := tools.NewRegistry()
	if err := codedom.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	if guard {
		reg.SetWriteGuard(func(_ context.Context, name string, args map[string]any) error {
			return tools.RefuseAddedBuildExclusion(name, args, dir)
		})
	}
	return reg
}

func writeSrc(t *testing.T, dir, name, body string) string {
	t.Helper()
	abs := filepath.Join(dir, name)
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

func execNamed(reg *tools.Registry, name string, args map[string]any) error {
	_, err := reg.Execute(context.Background(), name, args)
	return err
}

// The element tools do not call RejectGoSyntaxRegression. This is the
// registry path a caller uses when it skips the session: the guard projects
// the edit, and the cases below are ones the tool would otherwise write.
func TestBuildExclusion_ElementPaths(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "1")
	headerBody := "package p\n\nfunc A() int { return 1 }\n\nfunc B() {\n\t// package p\n}\n"
	twoReturn := "package p\n\nfunc A() int { return 1 }\n\nfunc B() int { return 1 }\n"

	t.Run("unguarded header edit lands", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", headerBody)
		reg := exclusionRegistry(t, dir, false)
		if err := execNamed(reg, "edit_element", map[string]any{
			"path": "a.go", "ref": "header", "old": "package p", "new": "//go:build ignore\n\npackage p",
		}); err != nil {
			t.Fatalf("the tool itself refused the header edit: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "go:build ignore") {
			t.Fatalf("unguarded edit did not hide the file:\n%s", data)
		}
	})

	t.Run("guarded header edit", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", headerBody)
		reg := exclusionRegistry(t, dir, true)
		err := execNamed(reg, "edit_element", map[string]any{
			"path": "a.go", "ref": "header", "old": "package p", "new": "//go:build ignore\n\npackage p",
		})
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the added ignore refused", err)
		}
		if data, _ := os.ReadFile(abs); string(data) != headerBody {
			t.Fatalf("refused edit landed:\n%s", data)
		}
	})

	t.Run("non-unique old in one function", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", twoReturn)
		reg := exclusionRegistry(t, dir, true)
		if err := execNamed(reg, "edit_element", map[string]any{
			"path": "a.go", "ref": "A", "old": "return 1", "new": "return 2",
		}); err != nil {
			t.Fatalf("a unique-in-element edit was refused: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "func A() int { return 2 }") || strings.Contains(string(data), "go:build ignore") {
			t.Fatalf("function edit = %s", data)
		}
	})

	t.Run("already ignored", func(t *testing.T) {
		dir := t.TempDir()
		body := "//go:build ignore\n\n" + twoReturn
		abs := writeSrc(t, dir, "a.go", body)
		reg := exclusionRegistry(t, dir, true)
		if err := execNamed(reg, "edit_element", map[string]any{
			"path": "a.go", "ref": "A", "old": "return 1", "new": "return 2",
		}); err != nil {
			t.Fatalf("editing a file that already excluded: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "return 2") || !strings.Contains(string(data), "go:build ignore") {
			t.Fatalf("already-excluded edit = %s", data)
		}
	})

	t.Run("mention", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", twoReturn)
		reg := exclusionRegistry(t, dir, true)
		if err := execNamed(reg, "edit_element", map[string]any{
			"path": "a.go", "ref": "A", "old": "{ return 1 }",
			"new": "{\n\t// Never add //go:build ignore here.\n\treturn 1\n}",
		}); err != nil {
			t.Fatalf("a mention inside the function was refused: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "Never add //go:build ignore here.") || strings.HasPrefix(string(data), "//go:build") {
			t.Fatalf("mention edit = %s", data)
		}
	})

	t.Run("replace header", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", twoReturn)
		reg := exclusionRegistry(t, dir, true)
		err := execNamed(reg, "replace_element", map[string]any{
			"path": "a.go", "ref": "header", "source": "//go:build ignore\n\npackage p",
		})
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the header replacement refused", err)
		}
		if data, _ := os.ReadFile(abs); string(data) != twoReturn {
			t.Fatalf("refused replacement landed:\n%s", data)
		}
	})

	t.Run("replace header that already excluded", func(t *testing.T) {
		dir := t.TempDir()
		body := "//go:build ignore\n\npackage p\n\nfunc A() int { return 1 }\n"
		abs := writeSrc(t, dir, "a.go", body)
		reg := exclusionRegistry(t, dir, true)
		if err := execNamed(reg, "replace_element", map[string]any{
			"path": "a.go", "ref": "header", "source": "//go:build ignore\n\n// kept\npackage p",
		}); err != nil {
			t.Fatalf("replacing an already-excluded header: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "kept") || !strings.Contains(string(data), "go:build ignore") || !strings.Contains(string(data), "func A()") {
			t.Fatalf("header rewrite = %s", data)
		}
	})

	t.Run("replace function mentions the text", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", twoReturn)
		reg := exclusionRegistry(t, dir, true)
		if err := execNamed(reg, "replace_element", map[string]any{
			"path": "a.go", "ref": "A",
			"source": "// Never add //go:build ignore here.\nfunc A() int { return 9 }",
		}); err != nil {
			t.Fatalf("a function replacement that mentions the text was refused: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "return 9") || strings.HasPrefix(string(data), "//go:build") {
			t.Fatalf("function replacement = %s", data)
		}
	})

	t.Run("insert before header", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", twoReturn)
		reg := exclusionRegistry(t, dir, true)
		err := execNamed(reg, "insert_element", map[string]any{
			"path": "a.go", "anchor": "header", "position": "before", "source": "func C() int { return 3 }",
		})
		if err == nil || !strings.Contains(err.Error(), "header") || strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the tool's header refusal", err)
		}
		if data, _ := os.ReadFile(abs); string(data) != twoReturn {
			t.Fatalf("insert before the header landed:\n%s", data)
		}
	})

	t.Run("insert mention before a function", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", twoReturn)
		reg := exclusionRegistry(t, dir, true)
		if err := execNamed(reg, "insert_element", map[string]any{
			"path": "a.go", "anchor": "A", "position": "before",
			"source": "// Never add //go:build ignore here.\nfunc C() int { return 3 }",
		}); err != nil {
			t.Fatalf("an insert after the package clause was refused: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "func C()") || strings.HasPrefix(string(data), "//go:build") {
			t.Fatalf("insert = %s", data)
		}
	})

	t.Run("replace syntax error covering the file", func(t *testing.T) {
		// A file with no package clause is one syntax_error from byte 0.
		// Replacing that span is a whole-file write, and it can add a
		// header constraint. The guard has to refuse it; the tool would write.
		dir := t.TempDir()
		broken := "this is not go\n"
		abs := writeSrc(t, dir, "a.go", broken)
		source := "//go:build ignore\n\npackage p\n\nfunc A() int { return 1 }"
		open := exclusionRegistry(t, dir, false)
		if err := execNamed(open, "replace_element", map[string]any{
			"path": "a.go", "ref": "syntax_error", "source": source,
		}); err != nil {
			t.Fatalf("the tool itself refused replacing the broken region: %v", err)
		}
		landed, _ := os.ReadFile(abs)
		if !strings.Contains(string(landed), "go:build ignore") {
			t.Fatalf("unguarded replace did not hide the file:\n%s", landed)
		}
		abs = writeSrc(t, dir, "a.go", broken)
		reg := exclusionRegistry(t, dir, true)
		err := execNamed(reg, "replace_element", map[string]any{
			"path": "a.go", "ref": "syntax_error", "source": source,
		})
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the added ignore refused", err)
		}
		if data, _ := os.ReadFile(abs); string(data) != broken {
			t.Fatalf("refused replace landed:\n%s", data)
		}
	})

	t.Run("unresolved ref", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", twoReturn)
		reg := exclusionRegistry(t, dir, true)
		err := execNamed(reg, "edit_element", map[string]any{
			"path": "a.go", "ref": "nope", "old": "return 1", "new": "//go:build ignore\n\npackage p",
		})
		if err == nil || !strings.Contains(err.Error(), "cannot be checked") || strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the unresolved ref refused", err)
		}
		if data, _ := os.ReadFile(abs); string(data) != twoReturn {
			t.Fatalf("unresolved ref wrote:\n%s", data)
		}
	})
}

// edit_lines and insert_lines call RejectGoSyntaxRegression, so a caller
// that skips the session and installs no registry guard still cannot hide a
// file. The registry in these subtests has no write guard.
func TestBuildExclusion_LineToolsSkipTheSession(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "1")
	compiled := "package p\n\nfunc A() int { return 1 }\n"

	t.Run("edit_lines adds ignore", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", compiled)
		reg := exclusionRegistry(t, dir, false)
		err := execNamed(reg, "edit_lines", map[string]any{
			"path": "a.go", "start_line": 1, "end_line": 1, "new_content": "//go:build ignore\n\npackage p",
		})
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the added ignore refused", err)
		}
		if data, _ := os.ReadFile(abs); string(data) != compiled {
			t.Fatalf("refused edit landed: %q", data)
		}
	})

	t.Run("insert_lines at the top", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", compiled)
		reg := exclusionRegistry(t, dir, false)
		err := execNamed(reg, "insert_lines", map[string]any{
			"path": "a.go", "after_line": 0, "content": "//go:build ignore\n",
		})
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the inserted constraint refused", err)
		}
		if data, _ := os.ReadFile(abs); string(data) != compiled {
			t.Fatalf("refused insert landed: %q", data)
		}
	})

	t.Run("already ignored", func(t *testing.T) {
		dir := t.TempDir()
		body := "//go:build ignore\n\n" + compiled
		abs := writeSrc(t, dir, "a.go", body)
		line := 0
		for i, ln := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
			if strings.Contains(ln, "return 1") {
				line = i + 1
			}
		}
		if line == 0 {
			t.Fatal("fixture has no return line")
		}
		reg := exclusionRegistry(t, dir, false)
		if err := execNamed(reg, "edit_lines", map[string]any{
			"path": "a.go", "start_line": line, "end_line": line, "new_content": "func A() int { return 2 }",
		}); err != nil {
			t.Fatalf("editing a file that already excluded: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "return 2") || !strings.Contains(string(data), "go:build ignore") {
			t.Fatalf("already-excluded edit = %q", data)
		}
	})

	t.Run("mention", func(t *testing.T) {
		dir := t.TempDir()
		abs := writeSrc(t, dir, "a.go", compiled)
		reg := exclusionRegistry(t, dir, false)
		if err := execNamed(reg, "edit_lines", map[string]any{
			"path": "a.go", "start_line": 3, "end_line": 3,
			"new_content": "func A() int { return 1 } // Never add //go:build ignore here.",
		}); err != nil {
			t.Fatalf("a mention on a return line was refused: %v", err)
		}
		data, _ := os.ReadFile(abs)
		if !strings.Contains(string(data), "Never add //go:build ignore here.") || strings.HasPrefix(string(data), "//go:build") {
			t.Fatalf("mention edit = %q", data)
		}
	})
}

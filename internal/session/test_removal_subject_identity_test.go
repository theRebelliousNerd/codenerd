package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeModuleFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

// A function deleted from one package released a missing test in another
// that names a same-named function of its own, still there: the deleted set
// was bare names across the whole turn. The subject is now resolved to its
// package, and a function moved between files of its package is not deleted.
func TestRemovedTestFunctions_ASubjectIsItsPackagesNotAnyNamesake(t *testing.T) {
	const alphaTestBefore = "package alpha\n\nimport \"testing\"\n\nfunc TestReset(t *testing.T) {\n\tif Reset() != 0 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n\nfunc TestKept(t *testing.T) {}\n"
	const alphaTestAfter = "package alpha\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
	ws := writeModuleFiles(t, map[string]string{
		"go.mod":          "module probe\n\ngo 1.21\n",
		"alpha/alpha.go":  "package alpha\n\nfunc Reset() int { return 0 }\n",
		"alpha/x_test.go": alphaTestAfter,
		"beta/y.go":       "package beta\n",
	})
	written := []string{"beta/y.go", "alpha/x_test.go"}
	pre := map[string]PreImage{
		"beta/y.go":       {Existed: true, Content: "package beta\n\nfunc Reset() {}\n"},
		"alpha/x_test.go": {Existed: true, Content: alphaTestBefore},
	}
	removed := removedTestFunctions(ws, written, pre)
	if len(removed) != 1 || !strings.Contains(removed[0], "TestReset") {
		t.Fatalf("removed = %v; beta.Reset's deletion released alpha.TestReset, whose subject alpha.Reset survives", removed)
	}

	// The imported subject really deleted: an external test package that
	// called beta.Reset goes with it.
	ws = writeModuleFiles(t, map[string]string{
		"go.mod":              "module probe\n\ngo 1.21\n",
		"beta/y.go":           "package beta\n",
		"beta/y_ext_test.go":  "package beta_test\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {}\n",
		"gamma/gamma_test.go": "package gamma\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n",
	})
	gammaBefore := "package gamma\n\nimport (\n\t\"testing\"\n\n\tb \"probe/beta\"\n)\n\nfunc TestUsesReset(t *testing.T) {\n\tb.Reset()\n}\n\nfunc TestKept(t *testing.T) {}\n"
	written = []string{"beta/y.go", "gamma/gamma_test.go"}
	pre = map[string]PreImage{
		"beta/y.go":           {Existed: true, Content: "package beta\n\nfunc Reset() {}\n"},
		"gamma/gamma_test.go": {Existed: true, Content: gammaBefore},
	}
	if removed := removedTestFunctions(ws, written, pre); len(removed) != 0 {
		t.Fatalf("removed = %v; a test of the imported function the turn deleted goes with it", removed)
	}

	// Moved between files of its package: not deleted.
	ws = writeModuleFiles(t, map[string]string{
		"go.mod":          "module probe\n\ngo 1.21\n",
		"alpha/alpha.go":  "package alpha\n",
		"alpha/moved.go":  "package alpha\n\nfunc Reset() int { return 0 }\n",
		"alpha/x_test.go": alphaTestAfter,
	})
	written = []string{"alpha/alpha.go", "alpha/moved.go", "alpha/x_test.go"}
	pre = map[string]PreImage{
		"alpha/alpha.go":  {Existed: true, Content: "package alpha\n\nfunc Reset() int { return 0 }\n"},
		"alpha/x_test.go": {Existed: true, Content: alphaTestBefore},
	}
	removed = removedTestFunctions(ws, written, pre)
	if len(removed) != 1 || !strings.Contains(removed[0], "TestReset") {
		t.Fatalf("removed = %v; Reset moved to another file of its package and its test still went missing", removed)
	}
}

package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ladder run R1-18 (2026-09-19) was refused for doing the right thing. The
// brief asked for a guard to stop refusing valid edits; the turn deleted the
// guard and leaned on the whole-file parser check beside it, which meant
// deleting the two tests whose only subject was the deleted function. The gate
// answered
//
//	turn removed test(s) without replacing them: ...TestNetDelimiters;
//	a failing test is fixed by fixing the code, not by deleting the test
//
// and failed the turn. Nothing was being hidden: the behaviour those tests
// pinned no longer exists. Until this, no turn could ever delete dead code
// through codeNERD, because the dead code's tests could not go with it.
//
// The forgiveness is narrow on purpose. A test is released only when the turn
// also deleted a function the test names, in a non-test file the same turn
// wrote. A test whose subject is still there is still a removed test.
func TestRemovedTestFunctions_ATestGoesWithTheSubjectTheTurnDeleted(t *testing.T) {
	const beforeSrc = `package guard

func netDelimiters(s string) int { return len(s) }

func StillHere() int { return 1 }
`
	const beforeTest = `package guard

import "testing"

func TestNetDelimiters(t *testing.T) {
	if netDelimiters("x") != 1 {
		t.Fatal("no")
	}
}

func TestStillHere(t *testing.T) {
	if StillHere() != 1 {
		t.Fatal("no")
	}
}
`

	// write lays down the workspace after the turn and returns the pre-images.
	write := func(t *testing.T, afterSrc, afterTest string) (string, []string, map[string]PreImage) {
		t.Helper()
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "guard.go"), []byte(afterSrc), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "guard_test.go"), []byte(afterTest), 0o644); err != nil {
			t.Fatal(err)
		}
		return ws, []string{"guard.go", "guard_test.go"}, map[string]PreImage{
			"guard.go":      {Existed: true, Content: beforeSrc},
			"guard_test.go": {Existed: true, Content: beforeTest},
		}
	}

	t.Run("theSubjectWentToo", func(t *testing.T) {
		// netDelimiters is gone from guard.go, and its test with it.
		afterSrc := "package guard\n\nfunc StillHere() int { return 1 }\n"
		afterTest := "package guard\n\nimport \"testing\"\n\nfunc TestStillHere(t *testing.T) {\n\tif StillHere() != 1 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n"
		ws, written, pre := write(t, afterSrc, afterTest)
		if removed := removedTestFunctions(ws, written, pre); len(removed) != 0 {
			t.Fatalf("a test whose only subject the turn deleted was reported as removed: %v", removed)
		}
	})

	t.Run("theSubjectIsStillThere", func(t *testing.T) {
		// Both functions survive; TestStillHere is simply gone.
		afterTest := "package guard\n\nimport \"testing\"\n\nfunc TestNetDelimiters(t *testing.T) {\n\tif netDelimiters(\"x\") != 1 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n"
		ws, written, pre := write(t, beforeSrc, afterTest)
		removed := removedTestFunctions(ws, written, pre)
		if len(removed) != 1 || !strings.Contains(removed[0], "TestStillHere") {
			t.Fatalf("removed = %v, want the one test whose subject survives", removed)
		}
	})

	t.Run("nothingWasDeletedFromProduction", func(t *testing.T) {
		// The turn deleted a test and no production function at all: the
		// original case the guard exists for.
		afterTest := "package guard\n\nimport \"testing\"\n\nfunc TestStillHere(t *testing.T) {\n\tif StillHere() != 1 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n"
		ws, written, pre := write(t, beforeSrc, afterTest)
		removed := removedTestFunctions(ws, written, pre)
		if len(removed) != 1 || !strings.Contains(removed[0], "TestNetDelimiters") {
			t.Fatalf("removed = %v, want the deleted test reported", removed)
		}
	})
}

// A near-name must not release a test: tokenText writes identifiers as
// "IDENT <name>", so the match is on the whole identifier, never a substring.
func TestRemovedTestFunctions_ANearNameDoesNotReleaseATest(t *testing.T) {
	const beforeSrc = "package guard\n\nfunc netDelimiters() int { return 1 }\n\nfunc Keep() int { return 2 }\n"
	const beforeTest = "package guard\n\nimport \"testing\"\n\nfunc TestKeep(t *testing.T) {\n\tif netDelimitersOf() != 1 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n\nfunc netDelimitersOf() int { return 1 }\n"

	ws := t.TempDir()
	// netDelimiters deleted; TestKeep deleted but it only ever named
	// netDelimitersOf, which is still there.
	if err := os.WriteFile(filepath.Join(ws, "guard.go"), []byte("package guard\n\nfunc Keep() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "guard_test.go"), []byte("package guard\n\nfunc netDelimitersOf() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pre := map[string]PreImage{
		"guard.go":      {Existed: true, Content: beforeSrc},
		"guard_test.go": {Existed: true, Content: beforeTest},
	}
	removed := removedTestFunctions(ws, []string{"guard.go", "guard_test.go"}, pre)
	if len(removed) != 1 || !strings.Contains(removed[0], "TestKeep") {
		t.Fatalf("removed = %v, want TestKeep reported: it names netDelimitersOf, which survives, not the deleted netDelimiters", removed)
	}
}

package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ladder run R1-20 (2026-09-20) deleted three asserts of a predicate nothing
// declares, and updated the repo's undeclared-assert baseline in the same
// change. That change IS pinned: put the production file back with the
// baseline as the turn left it and TestUndeclaredAssertBudget fails, naming
// the predicate and the file. The pinning gate said otherwise --
//
//	This turn changed the functions below and wrote no test, so nothing
//	would notice if a change were lost
//
// -- because it only ever ran tests the TURN wrote, and this turn correctly
// wrote none: the invariant that pins it already existed, in another package.
// The model then spent three repair attempts, 18 model calls and 940,761 input
// tokens writing a probe_test.go that should not exist, the round gave up with
// the suite red, and a correct change ended /unverified.
//
// The sentence was the bug: "nothing would notice" was asserted, never
// checked. A turn that wrote no test is now measured the same way as one that
// did -- put the turn's production changes back and see whether the repository
// notices.
func TestPinnedByExistingTests_AsksTheRepositoryInsteadOfAssuming(t *testing.T) {
	ws := t.TempDir()
	writeMod(t, ws)

	// A package whose behaviour some OTHER package's test pins, which is the
	// shape R1-20 had: the guard lives nowhere near the code it guards.
	mustWrite(t, filepath.Join(ws, "subject", "subject.go"), `package subject

func Emit() string { return "kept" }
`)
	mustWrite(t, filepath.Join(ws, "guard", "guard_test.go"), `package guard

import (
	"testing"

	"probe/subject"
)

func TestSubjectStillEmits(t *testing.T) {
	if subject.Emit() != "kept" {
		t.Fatal("subject changed")
	}
}
`)

	reverted := pinUnit{path: "subject/subject.go", name: "Emit", content: `package subject

func Emit() string { return "lost" }
`}

	t.Run("anExistingTestNoticesTheChange", func(t *testing.T) {
		noticed, detail := pinnedByExistingTests(context.Background(), ws, reverted)
		if !noticed {
			t.Fatalf("the repository's own test was not consulted; a change it pins was reported as pinned by nothing (%s)", detail)
		}
		if !strings.Contains(detail, "TestSubjectStillEmits") {
			t.Fatalf("detail does not name what noticed: %q", detail)
		}
	})

	t.Run("nothingNoticesAChangeNothingPins", func(t *testing.T) {
		// A second function no test mentions at all.
		mustWrite(t, filepath.Join(ws, "subject", "extra.go"), `package subject

func Unwatched() string { return "new" }
`)
		only := pinUnit{path: "subject/extra.go", absent: true, added: true}
		noticed, _ := pinnedByExistingTests(context.Background(), ws, only)
		if noticed {
			t.Fatal("a change nothing pins was reported as pinned; the gate would stop asking for the test that is genuinely missing")
		}
	})
}

func writeMod(t *testing.T, ws string) {
	t.Helper()
	mustWrite(t, filepath.Join(ws, "go.mod"), "module probe\n\ngo 1.21\n")
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// While a recurse loop runs, the chat's own workspace writers are refused:
// a /write beside a running attempt was restored to HEAD by /recurse stop,
// because the attempt's cleanup reverts every file that differs from HEAD
// (audit recurse-compensation-write-ownership). Reads stay open, and with no
// loop running the same /write lands.
func TestChatWritersAreHeldWhileRecurseOwnsTheWorkspace(t *testing.T) {
	m := NewTestModel()
	m.workspace = t.TempDir()
	m.recurse = &recurseState{cancel: func() {}, lines: make(chan string)}

	for _, command := range []string{
		"/write notes.txt new-owner-text",
		"/append notes.txt more",
		"/mkdir newdir",
	} {
		updated, _ := m.handleCommand(command)
		m = updated.(Model)
		if !strings.Contains(lastChatMessage(m), "recurse loop owns the workspace") {
			t.Fatalf("%s while recurse runs: %q, want the refusal", command, lastChatMessage(m))
		}
	}
	for _, p := range []string{"notes.txt", "newdir"} {
		if _, err := os.Stat(filepath.Join(m.workspace, p)); err == nil {
			t.Fatalf("%s was written while the recurse loop held the workspace", p)
		}
	}

	m.recurse = nil
	updated, _ := m.handleCommand("/write notes.txt new-owner-text")
	m = updated.(Model)
	if got, err := os.ReadFile(filepath.Join(m.workspace, "notes.txt")); err != nil || string(got) != "new-owner-text" {
		t.Fatalf("with no loop running /write did not land: %q, %v (%q)", got, err, lastChatMessage(m))
	}
}

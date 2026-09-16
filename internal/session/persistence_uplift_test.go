package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Session IDs arrive from user input (/load-session <id>). A traversal ID
// must fail closed on both save and load — and must not create anything
// outside the sessions directory.
func TestSessionHistory_TraversalIDRefused(t *testing.T) {
	dir := t.TempDir()
	msgs := []ChatMessage{{Role: "user", Content: "hi", Time: time.Now()}}

	// "." maps to sessions/..json — odd but contained, so it is not in the
	// refusal set; only genuine escapes and empties fail here.
	for _, id := range []string{"../escape", "..", "sub/dir", ""} {
		if err := SaveSessionHistory(dir, id, msgs); err == nil {
			t.Errorf("SaveSessionHistory(%q) must fail", id)
		}
		if _, err := LoadSessionHistory(dir, id); err == nil {
			t.Errorf("LoadSessionHistory(%q) must fail", id)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.json")); !os.IsNotExist(err) {
		t.Error("traversal save must not create files outside the store")
	}

	// Ordinary IDs keep working, including the generated sess_<nanos> shape.
	if err := SaveSessionHistory(dir, "sess_123", msgs); err != nil {
		t.Fatalf("valid save failed: %v", err)
	}
	loaded, err := LoadSessionHistory(dir, "sess_123")
	if err != nil {
		t.Fatalf("valid load failed: %v", err)
	}
	if len(loaded.Messages) != 1 || loaded.Messages[0].Content != "hi" {
		t.Errorf("round trip mangled history: %+v", loaded.Messages)
	}
}

// SaveSessionState must create a missing .nerd directory rather than fail:
// a fresh workspace has no store yet when the first state is written.
func TestSaveSessionState_CreatesNerdDir(t *testing.T) {
	dir := t.TempDir()
	state := &SessionState{SessionID: "sess_1", StartedAt: time.Now(), LastActiveAt: time.Now()}
	if err := SaveSessionState(dir, state); err != nil {
		t.Fatalf("save into fresh workspace failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".nerd", "session.json"))
	if err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	if !strings.Contains(string(data), "sess_1") {
		t.Errorf("state file wrong content: %s", data)
	}
}

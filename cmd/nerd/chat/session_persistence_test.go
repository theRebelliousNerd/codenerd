package chat

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveSessionState_WritesWithoutInit(t *testing.T) {
	ws := t.TempDir()

	m := &Model{
		workspace: ws,
		sessionID: "sess_test",
		history: []Message{
			{Role: "assistant", Content: "hello", Time: time.Now()},
		},
	}

	m.saveSessionState()

	if _, err := os.Stat(filepath.Join(ws, ".nerd", "session.json")); err != nil {
		t.Fatalf("expected .nerd/session.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".nerd", "sessions", "sess_test.json")); err != nil {
		t.Fatalf("expected .nerd/sessions/sess_test.json to exist: %v", err)
	}
}

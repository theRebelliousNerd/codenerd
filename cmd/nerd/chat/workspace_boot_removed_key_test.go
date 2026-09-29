package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/logging"
)

// A config still carrying logging.json_format must refuse the chat boot, the
// same refusal the headless CLI path owes (Finding 6, commit 39625fbb):
// performSystemBootShared printed the rejection and booted on with file
// logging torn down. The boot message must carry an error naming the key.
//
// Filename ordering is load-bearing, not cosmetic: a failed Initialize bricks
// later Initialize calls process-wide (the sync.Once is consumed and
// initialized stays false, so every later call returns the stale error), and
// this test fails an Initialize on purpose. diagnostics_test.go and
// system_warnings_test.go are the other logging.Initialize callers here and
// both sort before this file. yolo_*_test.go sorts after and does not touch
// logging.
func TestSharedBoot_RemovedLoggingKeyFailsTheBoot(t *testing.T) {
	bad := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bad, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `{"logging":{"json_format":true}}`
	if err := os.WriteFile(filepath.Join(bad, ".nerd", "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	msg := performSystemBootShared(nil, nil, "", bad)
	done, ok := msg.(bootCompleteMsg)
	if !ok {
		t.Fatalf("performSystemBootShared returned %T, want bootCompleteMsg", msg)
	}
	if done.err == nil {
		t.Fatalf("shared boot in a workspace with logging.json_format succeeded, want a fatal boot error")
	}
	if !strings.Contains(done.err.Error(), "json_format") {
		t.Fatalf("boot error does not name the removed key: %v", done.err)
	}
	if !logging.IsRemovedKeyError(done.err) {
		t.Errorf("boot error is not the removed-key rejection: %T %v", done.err, done.err)
	}
}

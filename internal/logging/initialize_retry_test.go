package logging

import (
	"os"
	"path/filepath"
	"testing"
)

// A refused config does not brick logging for the process: after the
// operator removes the bad key, the next Initialize binds. It was a
// sync.Once, so the first failure was returned by every later call -- which
// also made every test after a deliberately failing one depend on file order,
// and the chat package fail under -count=2.
func TestInitialize_AFailedInitIsRetriedNotLatched(t *testing.T) {
	resetAllLoggingState(t)
	t.Cleanup(func() { resetAllLoggingState(t) })

	ws := t.TempDir()
	cfgPath := filepath.Join(ws, ".nerd", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`{"logging":{"json_format":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Initialize(ws); err == nil || !IsRemovedKeyError(err) {
		t.Fatalf("Initialize over a removed key = %v, want the removed-key refusal", err)
	}

	if err := os.WriteFile(cfgPath, []byte(`{"logging":{"level":"info"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize after the key was removed = %v, want it to bind: the first failure was latched", err)
	}
	other := t.TempDir()
	if err := Initialize(other); err != nil {
		t.Fatalf("Initialize of another workspace after a recovered failure = %v", err)
	}
}

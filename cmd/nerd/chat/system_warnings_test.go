package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/logging"
)

// A warning the user reads must leave a durable record. Measured 2026-09-17 in a
// live chat session: three "[Kernel] Mangle update dropped" lines and a
// learnings-hydration failure were rendered to the screen, and grepping all 26
// log files that session wrote found none of them -- so a headless run lost them
// entirely and nobody could diagnose the turn afterwards.
//
// This drives the real logging system against a real workspace and reads the
// file back: no stub, no injected writer.
func TestRenderSystemWarnings_WritesEveryWarningToTheLog(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `{"logging":{"debug_mode":true,"level":"debug"}}`
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := logging.Initialize(ws); err != nil {
		t.Fatalf("logging.Initialize: %v", err)
	}
	t.Cleanup(func() {
		logging.CloseAll()
		// Leave the process-global logger inert: the temp workspace is about to
		// disappear. Then release the pin ApplyConfig sets, so a later
		// Initialize reads config from disk again.
		logging.ApplyConfig(logging.Config{DebugMode: false})
		logging.ClearInjectedConfig()
	})

	dropped := `[Kernel] Mangle update dropped: "next_action(spawn, ResearcherShard).": parse error`
	hydrate := "Hydrate learnings warning: hydrate learnings incomplete after 249 facts"

	rendered := renderSystemWarnings([]string{dropped, hydrate})

	// What the user sees.
	for _, want := range []string{dropped, hydrate} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered block missing %q", want)
		}
	}

	// What survives the turn.
	logging.CloseAll()
	matches, err := filepath.Glob(filepath.Join(ws, ".nerd", "logs", "*_session.log"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no session log was written at all")
	}
	var logged strings.Builder
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			t.Fatalf("read %s: %v", m, err)
		}
		logged.Write(b)
	}
	for _, want := range []string{dropped, hydrate} {
		if !strings.Contains(logged.String(), want) {
			t.Errorf("warning shown to the user never reached the log: %q", want)
		}
	}
}

// No warnings must add nothing to the reply -- the block is not printed empty.
func TestRenderSystemWarnings_EmptyAddsNothing(t *testing.T) {
	if got := renderSystemWarnings(nil); got != "" {
		t.Errorf("renderSystemWarnings(nil) = %q, want empty", got)
	}
}

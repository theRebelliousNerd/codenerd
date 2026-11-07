package logging

import (
	"os"
	"strings"
	"testing"
)

func TestWriteGoroutineDump_WritesEveryStackWithoutStopping(t *testing.T) {
	configMu.Lock()
	prev := logsDir
	logsDir = t.TempDir()
	configMu.Unlock()
	t.Cleanup(func() { configMu.Lock(); logsDir = prev; configMu.Unlock() })

	release := make(chan struct{})
	go func() { <-release }() // a goroutine parked elsewhere must appear too
	defer close(release)

	path, err := WriteGoroutineDump("test: stalled turn")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"reason: test: stalled turn", "goroutine ", "TestWriteGoroutineDump_WritesEveryStackWithoutStopping", "chan receive"} {
		if !strings.Contains(text, want) {
			t.Fatalf("dump lacks %q:\n%.600s", want, text)
		}
	}
}

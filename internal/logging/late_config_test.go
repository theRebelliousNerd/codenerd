package logging

import (
	"strings"
	"testing"
)

func TestLoggingEnabledAfterDisabledWorkspaceInit(t *testing.T) {
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)
	root := t.TempDir()
	if err := Initialize(root); err != nil {
		t.Fatal(err)
	}
	ApplyConfig(Config{DebugMode: true})
	if err := Initialize(root); err != nil {
		t.Fatal(err)
	}
	Get(CategorySession).Info("late configuration witness")
	if !strings.Contains(readLog(t, root, "session"), "late configuration witness") {
		t.Fatal("enabled logging lost its sink")
	}
}

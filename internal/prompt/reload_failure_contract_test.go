package prompt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReloadAllPromptsReturnsDiscoveredAgentFailure(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "agents", "broken")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte("invalid: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReloadAllPrompts(context.Background(), root, nil); err == nil {
		t.Fatal("broken discovered expert was reported as successful synchronization")
	}
}

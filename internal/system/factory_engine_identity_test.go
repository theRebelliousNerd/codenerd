package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The configured engine is part of the Cortex identity. Switching config.json
// from the API engine to claude-cli used to return the cached Cortex, wired to
// the LLM client the old engine built.
func TestGetOrBootCortexEngineIsPartOfIdentity(t *testing.T) {
	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeEngine := func(engine string) {
		t.Helper()
		cfg := `{"engine":"` + engine + `"}`
		if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, _ []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	writeEngine("api")
	apiCortex, err := getOrBootCortex(context.Background(), workspace, "secret", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = apiCortex.Close() })

	writeEngine("claude-cli")
	cliCortex, err := getOrBootCortex(context.Background(), workspace, "secret", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cliCortex.Close() })

	if cliCortex == apiCortex || bootCalls != 2 {
		t.Fatalf("switching the engine reused the Cortex (boot calls %d)", bootCalls)
	}
}

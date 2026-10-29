package init

import (
	"codenerd/internal/config"
	"os"
	"path/filepath"
	"testing"
)

// testAtomCap is the default shard-agent atom cap (jit.atom_max_chars).
var testAtomCap = config.DefaultJITConfig().AtomMaxChars

// The cap comes from the workspace's jit.atom_max_chars, defaulting when absent.
func TestAtomMaxCharsReadsWorkspaceConfig(t *testing.T) {
	ws := t.TempDir()
	if got := (&Initializer{config: InitConfig{Workspace: ws}}).atomMaxChars(); got != testAtomCap {
		t.Fatalf("no config: cap = %d, want the default %d", got, testAtomCap)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(`{"jit":{"atom_max_chars":600}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := (&Initializer{config: InitConfig{Workspace: ws}}).atomMaxChars(); got != 600 {
		t.Fatalf("configured cap = %d, want 600", got)
	}
}

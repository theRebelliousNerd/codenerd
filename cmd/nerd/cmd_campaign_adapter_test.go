package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The campaign file adapter reads inside the workspace and never writes: it
// used to os.WriteFile any path past the constitution, the Dreamer and the
// post-action validator (system-virtualstore-adapter-policy-v1).
func TestCampaignVirtualStoreAdapter_ShouldReadContainedAndRefuseWrites(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	if err := os.WriteFile(filepath.Join(ws, "plan.md"), []byte("a\nb"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := &campaignVirtualStoreAdapter{}

	if lines, err := adapter.ReadFile("plan.md"); err != nil || len(lines) != 2 {
		t.Fatalf("ReadFile inside the workspace = %q, %v", lines, err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.ReadRaw(outside); err == nil {
		t.Error("ReadRaw read a file outside the workspace")
	}

	target := filepath.Join(ws, "new.go")
	if err := adapter.WriteFile(target, []string{"package x"}); !errors.Is(err, errCampaignAdapterWrite) {
		t.Fatalf("WriteFile err = %v, want the routed-write refusal", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("a refused WriteFile still created the file")
	}
}

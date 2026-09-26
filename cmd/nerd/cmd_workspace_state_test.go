package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	working "codenerd/internal/context"
)

// `nerd status` names the project init recorded, the session a chat would
// resume, and the working-context archives with the retention policy's
// verdict; `nerd memory prune` removes the prunable ones. Before, status
// printed none of the three and the archives were visible only to du.
func TestWorkspaceState_StatusCountsArchivesAndPruneRemovesTheDeadOnes(t *testing.T) {
	ws := t.TempDir()
	nerd := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(filepath.Join(nerd, "context"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nerd, "profile.json"), []byte(`{"name":"probe","language":"go","framework":"cobra"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nerd, "session.json"), []byte(`{"session_id":"sess_probe"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// One archive this process owns, one left by a binary that recorded no
	// owner: nothing can redeem the second.
	live, err := working.OpenWorkingStore(ws, "session/probe/live")
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(nerd, "context", strings.Repeat("ab", 32)+".db")
	if err := os.WriteFile(orphan, make([]byte, 2048), 0o600); err != nil {
		t.Fatal(err)
	}

	status := renderWorkspaceState(ws)
	for _, want := range []string{"Project: probe (go, cobra)", "Latest session: sess_probe", "Working context: 2 archive(s)", "1 redeemable, 1 prunable", "nerd memory prune"} {
		if !strings.Contains(status, want) {
			t.Errorf("status lacks %q:\n%s", want, status)
		}
	}

	dry, err := runMemoryPrune(ws, true)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Removed != 0 {
		t.Fatalf("--dry-run removed %d archive(s)", dry.Removed)
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Fatalf("--dry-run touched the orphan: %v", err)
	}

	report, err := runMemoryPrune(ws, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 || report.Redeemable != 1 {
		t.Fatalf("prune = %s, want the orphan removed and the live archive kept", report)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("the orphan survived the prune: %v", err)
	}
	if after := renderWorkspaceState(ws); !strings.Contains(after, "Working context: 1 archive(s)") || strings.Contains(after, "nerd memory prune") {
		t.Errorf("status after the prune:\n%s", after)
	}
}

// An uninitialized workspace says so rather than printing nothing.
func TestWorkspaceState_AnUninitializedWorkspaceSaysSo(t *testing.T) {
	status := renderWorkspaceState(t.TempDir())
	if !strings.Contains(status, "no .nerd/profile.json") || !strings.Contains(status, "Working context: 0 archive(s)") {
		t.Errorf("status of an empty workspace:\n%s", status)
	}
}

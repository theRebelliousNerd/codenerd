package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/session"
)

// A restart settles the fix chain first: the write journal lives in the
// process the exec replaces, so a fix turn that was stopped with a rebuild
// pending must have its edits undone before the hand-over, not left in the
// tree for a process that can no longer tell whose they are. Observed live
// 2026-11-12: Ctrl+X on a self-repair fix turn restarted within the second and
// left two modified files and no saved attempt.
func TestSelfRestart_SettlesTheFixTurnBeforeTheExec(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	m.isBooting = false
	m.workspace = t.TempDir()
	m.selfRepair = newSelfRepairState()
	m.restarter = pendingRestarter(t, m, func(string, []string, []string) error { return nil })

	path := filepath.Join(m.workspace, "kernel.go")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	journal := session.NewWriteJournal()
	if err := os.WriteFile(path, []byte("the fix turn's edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	journal.Record(m.workspace, []string{"kernel.go"}, map[string]session.PreImage{
		"kernel.go": {Existed: true, Content: "before\n", LastWriteHash: hashOf("the fix turn's edit\n")},
	})
	m.selfRepair.mu.Lock()
	m.selfRepair.journal, m.selfRepair.journalJob = journal, "self-repair-test"
	m.selfRepair.mu.Unlock()

	idle, _ := m.handleRestartCheck()
	if idle.restartRequest == nil {
		t.Fatal("an idle session with a pending rebuild did not ask for the restart")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "before\n" {
		t.Fatalf("the fix turn's edit crossed the restart: %q", got)
	}
	idle.selfRepair.mu.Lock()
	left := idle.selfRepair.journal
	idle.selfRepair.mu.Unlock()
	if left != nil {
		t.Fatal("the journal was not settled before the exec")
	}
	if matches, _ := filepath.Glob(filepath.Join(m.workspace, ".nerd", "self_repair", "attempts", "*")); len(matches) == 0 {
		t.Fatal("the undone attempt was not saved for the retry")
	}
}

func hashOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

package sessionlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeSession(t *testing.T, ws, id string, age time.Duration, files ...string) string {
	t.Helper()
	dir := filepath.Join(SessionsDir(ws), id)
	if len(files) == 0 {
		files = []string{HistoryFile}
	}
	when := time.Now().Add(-age)
	for _, f := range files {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"created_at":"2026-01-02T03:04:05Z","messages":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestMigrateFlatMovesOldSessionFilesIntoFolders(t *testing.T) {
	ws := t.TempDir()
	root := SessionsDir(ws)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, body := range map[string]string{"session-1": `{"a":1}`, "sess_2": `{"b":2}`} {
		if err := os.WriteFile(filepath.Join(root, id+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := MigrateFlat(ws)
	if err != nil || rep.Moved != 2 {
		t.Fatalf("MigrateFlat = %+v, %v", rep, err)
	}
	if got := readFile(t, filepath.Join(root, "session-1", HistoryFile)); got != `{"a":1}` {
		t.Fatalf("content changed in the move: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "session-1.json")); !os.IsNotExist(err) {
		t.Fatal("the flat file must be gone: there is a single path")
	}
	// Once only: a second run finds nothing to do.
	if rep, err := MigrateFlat(ws); err != nil || rep.Moved != 0 {
		t.Fatalf("second MigrateFlat = %+v, %v", rep, err)
	}
}

func TestMigrateFlatNeverOverwritesAFolderThatAlreadyHasAHistory(t *testing.T) {
	ws := t.TempDir()
	root := SessionsDir(ws)
	makeSession(t, ws, "session-1", 0)
	if err := os.WriteFile(filepath.Join(root, "session-1.json"), []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := MigrateFlat(ws)
	if err != nil || rep.Moved != 0 || len(rep.Conflicts) != 1 {
		t.Fatalf("MigrateFlat = %+v, %v", rep, err)
	}
	if got := readFile(t, filepath.Join(root, "session-1.json")); got != `{"stale":true}` {
		t.Fatal("the conflicting flat file must be left alone")
	}
	if got := readFile(t, filepath.Join(root, "session-1", HistoryFile)); got == `{"stale":true}` {
		t.Fatal("the folder's history must not be overwritten")
	}
}

func TestMigrateFlatWithNoSessionsDirIsFine(t *testing.T) {
	if rep, err := MigrateFlat(t.TempDir()); err != nil || rep.Moved != 0 {
		t.Fatalf("MigrateFlat = %+v, %v", rep, err)
	}
}

func TestPruneDeletesSessionsPastTheTTLAndReportsCountAndBytes(t *testing.T) {
	ws := t.TempDir()
	old := makeSession(t, ws, "session-old", 20*24*time.Hour, HistoryFile, "chat/0001-session.json")
	makeSession(t, ws, "session-new", 2*24*time.Hour)
	rep, err := Prune(ws, 14*24*time.Hour, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Removed) != 1 || rep.Removed[0] != "session-old" || rep.Bytes == 0 || rep.Kept != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("the expired session folder must be gone, with everything in it")
	}
	if _, err := os.Stat(filepath.Join(SessionsDir(ws), "session-new")); err != nil {
		t.Fatal("a session inside the TTL must survive")
	}
}

func TestPruneAgesASessionByItsNewestFile(t *testing.T) {
	ws := t.TempDir()
	dir := makeSession(t, ws, "session-busy", 30*24*time.Hour)
	fresh := filepath.Join(dir, "coder-1", TranscriptFile)
	if err := os.MkdirAll(filepath.Dir(fresh), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fresh, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, _ := Prune(ws, 14*24*time.Hour, nil, time.Now())
	if len(rep.Removed) != 0 {
		t.Fatalf("a session with recent agent activity is not idle: %+v", rep)
	}
}

func TestPruneKeepsEverythingWhenTTLIsZero(t *testing.T) {
	ws := t.TempDir()
	makeSession(t, ws, "session-ancient", 1000*24*time.Hour)
	rep, err := Prune(ws, 0, nil, time.Now())
	if err != nil || len(rep.Removed) != 0 {
		t.Fatalf("a zero TTL keeps every session: %+v, %v", rep, err)
	}
	if _, err := os.Stat(filepath.Join(SessionsDir(ws), "session-ancient")); err != nil {
		t.Fatal("session deleted despite keep-forever")
	}
}

func TestPruneNeverDeletesTheBootingSessionOrARunningOne(t *testing.T) {
	ws := t.TempDir()
	makeSession(t, ws, "session-booting", 90*24*time.Hour)
	running := makeSession(t, ws, "session-running", 90*24*time.Hour)
	crashed := makeSession(t, ws, "session-crashed", 90*24*time.Hour)
	now := time.Now()
	if err := writeLease(running, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := writeLease(crashed, now.Add(-time.Hour)); err != nil { // heartbeat long stopped
		t.Fatal(err)
	}
	// Writing the leases refreshed the folders' mtimes; age the sessions again.
	for _, dir := range []string{running, crashed} {
		for _, f := range []string{HistoryFile, LeaseFile} {
			old := now.Add(-90 * 24 * time.Hour)
			_ = os.Chtimes(filepath.Join(dir, f), old, old)
		}
	}
	rep, err := Prune(ws, 14*24*time.Hour, map[string]bool{"session-booting": true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Removed) != 1 || rep.Removed[0] != "session-crashed" {
		t.Fatalf("only the crashed session may go: %+v", rep)
	}
	if len(rep.Running) != 1 || rep.Running[0] != "session-running" {
		t.Fatalf("the running session must be reported spared: %+v", rep)
	}
	for _, id := range []string{"session-booting", "session-running"} {
		if _, err := os.Stat(filepath.Join(SessionsDir(ws), id)); err != nil {
			t.Errorf("%s was deleted", id)
		}
	}
}

func TestStartSessionMigratesPrunesProtectsItselfAndInstallsTheStore(t *testing.T) {
	ws := t.TempDir()
	makeSession(t, ws, "session-expired", 40*24*time.Hour)
	if err := os.WriteFile(filepath.Join(SessionsDir(ws), "session-flat.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The booting session is itself old on disk; it must not be pruned.
	makeSession(t, ws, "session-boot", 40*24*time.Hour)

	h := StartSession(BootOptions{Workspace: ws, Session: "session-boot", TTL: 14 * 24 * time.Hour, Heartbeat: time.Hour})
	t.Cleanup(func() { h.Close(); Install(nil) })

	if h.Migrated.Moved != 1 {
		t.Errorf("migrated = %+v", h.Migrated)
	}
	if len(h.Pruned.Removed) != 1 || h.Pruned.Removed[0] != "session-expired" {
		t.Errorf("pruned = %+v", h.Pruned)
	}
	if Current() == nil || Current().Session() != "session-boot" {
		t.Error("the store must be installed with the booted session")
	}
	if !leaseFresh(filepath.Join(SessionsDir(ws), "session-boot"), time.Now()) {
		t.Error("the booted session must hold a fresh lease")
	}
	// The flat file was migrated *before* pruning, and being fresh it survives.
	if _, err := os.Stat(filepath.Join(SessionsDir(ws), "session-flat", HistoryFile)); err != nil {
		t.Errorf("migrated session missing: %v", err)
	}
	h.Close()
	if leaseFresh(filepath.Join(SessionsDir(ws), "session-boot"), time.Now()) {
		t.Error("Close must release the lease")
	}
}

func TestStartSessionWithTTLOffPrunesNothing(t *testing.T) {
	ws := t.TempDir()
	makeSession(t, ws, "session-ancient", 5000*24*time.Hour)
	h := StartSession(BootOptions{Workspace: ws, Session: "session-boot", TTL: 0, Heartbeat: time.Hour})
	t.Cleanup(func() { h.Close(); Install(nil) })
	if len(h.Pruned.Removed) != 0 {
		t.Fatalf("pruned = %+v", h.Pruned)
	}
}

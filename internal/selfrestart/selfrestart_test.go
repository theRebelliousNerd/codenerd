package selfrestart

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeIdentity(path, build string, size int64) Identity {
	return Identity{Path: path, Size: size, ModTime: time.Unix(1700000000, 0), Inode: 7, Build: build}
}

// A rebuilt binary restarts the session only when it is idle: a turn in
// progress is never interrupted, and a rewrite with the same identity is no
// rebuild.
func TestRestarter_WaitsForIdle(t *testing.T) {
	old := fakeIdentity("/bin/nerd", "aaaa", 10)
	r := NewRestarter(old, t.TempDir(), func(string, []string, []string) error { return nil })
	if r.BinaryChanged(old) || r.ShouldRestart(false) {
		t.Fatal("the same build counted as a rebuild")
	}
	if !r.BinaryChanged(fakeIdentity("/bin/nerd", "bbbb", 11)) {
		t.Fatal("a rebuild was not recorded")
	}
	if r.ShouldRestart(true) {
		t.Fatal("a busy session would be restarted")
	}
	if !r.ShouldRestart(false) {
		t.Fatal("an idle session with a pending rebuild would not restart")
	}
}

// The session reaches the new process through the restore flag: the exec
// gets the new build's path and the process's own arguments with the flag
// naming a file that reads back as the session, once.
func TestRestarter_ExecsTheNewBuildWithTheSession(t *testing.T) {
	var gotArgv0 string
	var gotArgv []string
	r := NewRestarter(fakeIdentity("/bin/nerd", "aaaa", 10), t.TempDir(), func(argv0 string, argv []string, _ []string) error {
		gotArgv0, gotArgv = argv0, argv
		return nil
	})
	r.BinaryChanged(fakeIdentity("/opt/nerd-new", "bbbb", 11))
	st := State{SessionID: "sess-1", WorkMode: "yolo", Brief: "fix the retry loop", Queued: []string{"then run the tests"}, Draft: "half typed"}
	if err := r.Restart(st, []string{"nerd", "chat", RestoreFlag, "/stale.json"}); err != nil {
		t.Fatal(err)
	}
	if gotArgv0 != "/opt/nerd-new" || len(gotArgv) != 4 || gotArgv[2] != RestoreFlag || gotArgv[1] != "chat" {
		t.Fatalf("exec %q %q; want the new build, the old args, one restore flag", gotArgv0, gotArgv)
	}
	back, err := ReadState(gotArgv[3])
	if err != nil {
		t.Fatal(err)
	}
	if back.SessionID != "sess-1" || back.WorkMode != "yolo" || back.Brief != st.Brief || len(back.Queued) != 1 || back.Draft != st.Draft || back.OldBuild != "aaaa" || back.NewBuild != "bbbb" {
		t.Fatalf("restored %+v", back)
	}
	if _, err := ReadState(gotArgv[3]); err == nil {
		t.Fatal("a restore file applied twice")
	}
}

// An exec that fails leaves the old process running with the rebuild still
// pending, removes the restore file, and says why.
func TestRestarter_AFailedExecKeepsTheOldProcess(t *testing.T) {
	dir := t.TempDir()
	r := NewRestarter(fakeIdentity("/bin/nerd", "aaaa", 10), dir, func(string, []string, []string) error {
		return errors.New("exec format error")
	})
	r.BinaryChanged(fakeIdentity("/bin/nerd", "bbbb", 11))
	err := r.Restart(State{SessionID: "s"}, []string{"nerd"})
	if err == nil || !strings.Contains(err.Error(), "keeps running") || !strings.Contains(err.Error(), "exec format error") {
		t.Fatalf("Restart = %v", err)
	}
	if _, pending := r.Pending(); !pending {
		t.Fatal("the rebuild stopped being pending after a failed exec")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a failed exec left a restore file: %v", entries)
	}
}

// The watcher reports a rebuild that replaces the executable by rename and
// one that rewrites it in place, once each has settled into a Go binary.
func TestWatch_SeesARenameOverAndAnInPlaceWrite(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "nerd")
	if err := os.WriteFile(path, binary, 0o755); err != nil {
		t.Fatal(err)
	}
	start, err := IdentityOf(path)
	if err != nil {
		t.Fatal(err)
	}
	changes := make(chan Identity, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := Watch(ctx, path, func(id Identity) { changes <- id }); err != nil {
		t.Fatal(err)
	}
	await := func(what string) Identity {
		t.Helper()
		select {
		case id := <-changes:
			return id
		case <-time.After(10 * time.Second):
			t.Fatalf("no change seen after %s", what)
			return Identity{}
		}
	}

	tmp := filepath.Join(dir, "nerd.new")
	if err := os.WriteFile(tmp, append(append([]byte{}, binary...), []byte("rebuild-1")...), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	renamed := await("a rename over")
	if !start.Changed(renamed) {
		t.Fatal("the rename over reported the old identity")
	}

	if err := os.WriteFile(path, append(append([]byte{}, binary...), []byte("rebuild-22")...), 0o755); err != nil {
		t.Fatal(err)
	}
	written := await("an in-place write")
	if !renamed.Changed(written) {
		t.Fatal("the in-place write reported the previous identity")
	}
}

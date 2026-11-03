package selfrestart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

func TestRestarter_TracksDistinctPendingIdentities(t *testing.T) {
	old := fakeIdentity("/bin/nerd", "a", 10)
	for _, dimension := range []string{"size", "time", "inode"} {
		t.Run(dimension, func(t *testing.T) {
			r := NewRestarter(old, t.TempDir(), nil)
			next := old
			switch dimension {
			case "size":
				next.Size++
			case "time":
				next.ModTime = next.ModTime.Add(-time.Second)
			case "inode":
				next.Inode++
			}
			if !r.BinaryChanged(next) {
				t.Fatal("changed identity was ignored")
			}
			next.Build = "display-only change"
			if r.BinaryChanged(next) {
				t.Fatal("pending identity was admitted twice")
			}
			if r.BinaryChanged(old) || r.ShouldRestart(false) {
				t.Fatal("return to running identity left a stale pending build")
			}
		})
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
	if got := r.CurrentPath(); got != "/bin/nerd" {
		t.Fatalf("CurrentPath = %q, want /bin/nerd", got)
	}
	st := State{
		SessionID:   "sess-1",
		WorkMode:    "yolo",
		Brief:       "fix the retry loop",
		Queued:      []string{"then run the tests"},
		Draft:       "half typed",
		Recurse:     true,
		RecurseArgs: []string{"--waves", "2", "session"},
	}
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
	if back.SessionID != "sess-1" || back.WorkMode != "yolo" || back.Brief != st.Brief || len(back.Queued) != 1 || back.Draft != st.Draft || !back.Recurse || len(back.RecurseArgs) != 3 || back.RecurseArgs[2] != "session" || back.OldBuild != "aaaa" || back.NewBuild != "bbbb" {
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

func TestRestarter_PreservesExecAndCleanupErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		consumed     bool
		blockCleanup bool
	}{
		{name: "restore file removed after exec failure"},
		{name: "already consumed restore file", consumed: true},
		{name: "restore cleanup fails", blockCleanup: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			execErr := errors.New("exec rejected")
			var restorePath string
			r := NewRestarter(fakeIdentity("/bin/nerd", "aaaa", 10), t.TempDir(), func(_ string, argv []string, _ []string) error {
				if len(argv) != 3 || argv[1] != RestoreFlag {
					t.Fatalf("exec argv = %q, want program and restore flag with path", argv)
				}
				restorePath = argv[2]
				if tt.consumed || tt.blockCleanup {
					if err := os.Remove(restorePath); err != nil {
						t.Fatal(err)
					}
				}
				if tt.blockCleanup {
					if err := os.Mkdir(restorePath, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(restorePath, "occupied"), []byte("keep"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return execErr
			})
			next := fakeIdentity("/bin/nerd", "bbbb", 11)
			r.BinaryChanged(next)
			err := r.Restart(State{SessionID: "s"}, []string{"nerd"})
			if !errors.Is(err, execErr) {
				t.Fatalf("Restart error = %v, want wrapped exec error", err)
			}
			var pathErr *os.PathError
			if got := errors.As(err, &pathErr); got != tt.blockCleanup {
				t.Fatalf("Restart cleanup error present = %v, want %v; error = %v", got, tt.blockCleanup, err)
			}
			if tt.blockCleanup {
				if pathErr.Op != "remove" || pathErr.Path != restorePath {
					t.Errorf("cleanup error = %+v, want remove of %q", pathErr, restorePath)
				}
			} else if _, err := os.Stat(restorePath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("restore file remains after exec failure: stat error = %v", err)
			}
			pending, ok := r.Pending()
			if !ok {
				t.Fatal("exec failure cleared the pending build")
			}
			if diff := cmp.Diff(next, pending); diff != "" {
				t.Errorf("pending build mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRestarter_DoesNotExecWithoutSavedState(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		pending   bool
		blockDir  bool
		blockFile bool
	}{
		{name: "no pending build"},
		{name: "state directory is a file", pending: true, blockDir: true},
		{name: "restore destination is a directory", pending: true, blockFile: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "state")
			if tt.blockDir {
				if err := os.WriteFile(dir, []byte("occupied"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tt.blockFile {
				path := filepath.Join(dir, fmt.Sprintf("restore-%d.json", os.Getpid()))
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			r := NewRestarter(fakeIdentity("/bin/nerd", "aaaa", 10), dir, func(string, []string, []string) error {
				called = true
				return errors.New("unexpected exec")
			})
			next := fakeIdentity("/bin/nerd", "bbbb", 11)
			if tt.pending {
				r.BinaryChanged(next)
			}
			err := r.Restart(State{SessionID: "must-not-resume"}, []string{"nerd"})
			if err == nil {
				t.Fatal("Restart accepted a restart without saved state")
			}
			if called {
				t.Fatal("Restart invoked exec without saved state")
			}
			var pathErr *os.PathError
			if tt.blockDir && !errors.As(err, &pathErr) {
				t.Errorf("Restart error = %v, want wrapped filesystem error", err)
			}
			if tt.blockFile {
				var linkErr *os.LinkError
				if !errors.As(err, &linkErr) {
					t.Errorf("Restart error = %v, want wrapped rename error", err)
				}
			}
			pending, ok := r.Pending()
			if ok != tt.pending {
				t.Fatalf("Pending = %v, want %v", ok, tt.pending)
			}
			if ok {
				if diff := cmp.Diff(next, pending); diff != "" {
					t.Errorf("pending build mismatch (-want +got):\n%s", diff)
				}
			} else if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("Restart without pending build touched state directory: stat error = %v", err)
			}
		})
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

func TestCleanExecutablePath_StripsDeletedSuffix(t *testing.T) {
	if got := CleanExecutablePath("/workspace/nerd (deleted)"); got != "/workspace/nerd" {
		t.Fatalf("CleanExecutablePath = %q, want /workspace/nerd", got)
	}
	if path, err := ExecutablePath(); err != nil || path == "" || strings.HasSuffix(path, " (deleted)") {
		t.Fatalf("ExecutablePath = %q, %v", path, err)
	}
}

func TestRestoreArgs_ReplacesFlagsWithoutMutatingInput(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "empty", want: []string{RestoreFlag, "/new.json"}},
		{name: "no restore flag", args: []string{"nerd", "chat", "--verbose"}, want: []string{"nerd", "chat", "--verbose", RestoreFlag, "/new.json"}},
		{name: "double dash separate", args: []string{"nerd", RestoreFlag, "/old.json", "chat"}, want: []string{"nerd", "chat", RestoreFlag, "/new.json"}},
		{name: "single dash separate", args: []string{"nerd", "-restore-session", "/old.json", "chat"}, want: []string{"nerd", "chat", RestoreFlag, "/new.json"}},
		{name: "double dash equals", args: []string{"nerd", "--restore-session=/old.json", "chat"}, want: []string{"nerd", "chat", RestoreFlag, "/new.json"}},
		{name: "single dash equals", args: []string{"nerd", "-restore-session=/old.json", "chat"}, want: []string{"nerd", "chat", RestoreFlag, "/new.json"}},
		{name: "multiple stale flags", args: []string{"nerd", RestoreFlag, "/first.json", "-restore-session=/second.json", "chat"}, want: []string{"nerd", "chat", RestoreFlag, "/new.json"}},
		{name: "missing trailing value", args: []string{"nerd", RestoreFlag}, want: []string{"nerd", RestoreFlag, "/new.json"}},
		{name: "similar flag preserved", args: []string{"nerd", "--restore-session-extra=value"}, want: []string{"nerd", "--restore-session-extra=value", RestoreFlag, "/new.json"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Spare capacity makes accidental reuse of the caller's backing array visible.
			args := make([]string, len(tt.args), len(tt.args)+4)
			copy(args, tt.args)
			backing := args[:cap(args)]
			for i := len(args); i < len(backing); i++ {
				backing[i] = "caller-owned"
			}
			before := slices.Clone(backing)
			got := RestoreArgs(args, "/new.json")
			if !slices.Equal(got, tt.want) {
				t.Fatalf("RestoreArgs() = %q, want %q", got, tt.want)
			}
			if !slices.Equal(backing, before) {
				t.Fatalf("RestoreArgs changed caller storage: got %q, want %q", backing, before)
			}
			if again := RestoreArgs(got, "/new.json"); !slices.Equal(again, tt.want) {
				t.Fatalf("RestoreArgs repeated = %q, want %q", again, tt.want)
			}
		})
	}
}

func TestReadState_RejectsInvalidRestoreFiles(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		data       string
		missing    bool
		checkError func(*testing.T, error)
	}{
		{name: "missing file", missing: true, checkError: func(t *testing.T, err error) {
			t.Helper()
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("ReadState error = %v, want os.ErrNotExist", err)
			}
		}},
		{name: "malformed JSON", data: `{"version":1,`, checkError: func(t *testing.T, err error) {
			t.Helper()
			var syntaxErr *json.SyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Fatalf("ReadState error = %v, want *json.SyntaxError", err)
			}
		}},
		{name: "wrong field type", data: `{"version":1,"session_id":42}`, checkError: func(t *testing.T, err error) {
			t.Helper()
			var typeErr *json.UnmarshalTypeError
			if !errors.As(err, &typeErr) {
				t.Fatalf("ReadState error = %v, want *json.UnmarshalTypeError", err)
			}
		}},
		{name: "unsupported version", data: `{"version":2,"session_id":"must-not-resume"}`},
		{name: "missing version", data: `{"session_id":"must-not-resume"}`},
		{name: "null state", data: `null`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "restore.json")
			if !tt.missing {
				if err := os.WriteFile(path, []byte(tt.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := ReadState(path)
			if err == nil {
				t.Fatal("ReadState accepted an invalid restore file")
			}
			if tt.checkError != nil {
				tt.checkError(t, err)
			}
			if diff := cmp.Diff(State{}, got); diff != "" {
				t.Errorf("ReadState returned partial state (-want +got):\n%s", diff)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid restore file remains replayable: stat error = %v", err)
			}
		})
	}
}

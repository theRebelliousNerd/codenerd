// Package selfrestart moves a running codeNERD onto a rebuilt binary in
// place, keeping its session.
//
// A watcher sees the executable replaced (a rename over it, or a write in
// place that settles). The session decides when: never during a turn, only
// once it is idle. Then the session's state is written to a restore file and
// the process execs the new binary over itself (same PID, same terminal, so
// tmux and attached terminals stay), with -restore-session naming the file;
// the new process reads it once and resumes the conversation, its mode, its
// brief and its held work, and names the work the restart interrupted. An exec that fails leaves the old process
// running and says why. Where exec in place is not available (Windows), the
// rebuild is reported and the user is asked to restart.
package selfrestart

import (
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"codenerd/internal/atomicfile"
)

// RestoreFlag is the command-line flag that names a restore file.
const RestoreFlag = "--restore-session"

// Identity is what distinguishes one build of the executable from another.
type Identity struct {
	Path    string
	Size    int64
	ModTime time.Time
	Inode   uint64
	// Build names the build: its VCS revision (+dirty when built from a
	// modified tree), or its modification time when it records none.
	Build string
}

// Changed reports whether other is a different build at the same path.
func (id Identity) Changed(other Identity) bool {
	return id.Size != other.Size || !id.ModTime.Equal(other.ModTime) || id.Inode != other.Inode
}

// IdentityOf reads the executable at path.
func IdentityOf(path string) (Identity, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Identity{}, err
	}
	id := Identity{Path: path, Size: st.Size(), ModTime: st.ModTime(), Inode: inodeOf(st)}
	id.Build = buildLabel(path, st.ModTime())
	return id, nil
}

// buildLabel names the build of the binary at path.
func buildLabel(path string, mod time.Time) string {
	info, err := buildinfo.ReadFile(path)
	if err == nil {
		rev, dirty := "", false
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if rev != "" {
			rev = rev[:min(len(rev), 12)]
			if dirty {
				rev += "+dirty"
			}
			return rev
		}
	}
	return "built " + mod.Format("2006-01-02 15:04:05")
}

// State is what the restarted process needs to resume the session.
type State struct {
	Version   int      `json:"version"`
	SessionID string   `json:"session_id"`
	WorkMode  string   `json:"work_mode,omitempty"`
	Brief     string   `json:"brief,omitempty"`
	Queued    []string `json:"queued,omitempty"`
	Draft     string   `json:"draft,omitempty"`
	// Interrupted names the work the restart stopped (background jobs, a
	// running campaign, an offered step), for the restored session to say.
	Interrupted []string `json:"interrupted,omitempty"`
	OldBuild    string   `json:"old_build"`
	NewBuild    string   `json:"new_build"`
}

const stateVersion = 1

// WriteState writes st to a restore file under dir and returns its path.
func WriteState(dir string, st State) (string, error) {
	st.Version = stateVersion
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("restore dir: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("restore-%d.json", os.Getpid()))
	if err := atomicfile.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write restore file: %w", err)
	}
	return path, nil
}

// ReadState reads a restore file and removes it: a restore applies once.
func ReadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read restore file: %w", err)
	}
	_ = os.Remove(path)
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("parse restore file: %w", err)
	}
	if st.Version != stateVersion {
		return State{}, fmt.Errorf("restore file version %d, this build reads %d", st.Version, stateVersion)
	}
	return st, nil
}

// RestoreArgs is args with the restore flag naming path, replacing any
// restore flag args already carried.
func RestoreArgs(args []string, path string) []string {
	out := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == RestoreFlag || a == strings.TrimPrefix(RestoreFlag, "-"):
			i++ // its value
			continue
		case strings.HasPrefix(a, RestoreFlag+"=") || strings.HasPrefix(a, strings.TrimPrefix(RestoreFlag, "-")+"="):
			continue
		}
		out = append(out, a)
	}
	return append(out, RestoreFlag, path)
}

// ExecFunc replaces the process image (syscall.Exec on Unix).
type ExecFunc func(argv0 string, argv []string, envv []string) error

// ErrExecUnsupported is the platform's exec-in-place being unavailable.
var ErrExecUnsupported = errors.New("this platform cannot replace a running process in place")

// Restarter decides when to restart and performs it. It is safe for
// concurrent use: the watcher reports changes, the session reports idleness.
type Restarter struct {
	mu      sync.Mutex
	current Identity
	pending *Identity
	exec    ExecFunc
	dir     string
}

// NewRestarter starts from the running build; exec nil means the platform's.
func NewRestarter(current Identity, stateDir string, exec ExecFunc) *Restarter {
	if exec == nil {
		exec = platformExec
	}
	return &Restarter{current: current, exec: exec, dir: stateDir}
}

// SetExec replaces how the process image is replaced (a test injects it).
func (r *Restarter) SetExec(exec ExecFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exec = exec
}

// BinaryChanged records a rebuilt executable. It reports whether the change is
// a new build (a rewrite with the same identity is not).
func (r *Restarter) BinaryChanged(next Identity) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.current.Changed(next) {
		return false
	}
	r.pending = &next
	return true
}

// Pending is the rebuilt build waiting for the session to be idle.
func (r *Restarter) Pending() (Identity, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending == nil {
		return Identity{}, false
	}
	return *r.pending, true
}

// ShouldRestart reports whether to restart now: a rebuild is pending and the
// session is not busy. A turn in progress is never interrupted.
func (r *Restarter) ShouldRestart(busy bool) bool {
	_, pending := r.Pending()
	return pending && !busy
}

// Restart writes st and execs the pending build in place with args (the
// process's own, os.Args) carrying the restore flag. It returns only on
// failure: the restore file is removed, the pending build stays pending, and
// the error says why, with the old process still running.
func (r *Restarter) Restart(st State, args []string) error {
	next, ok := r.Pending()
	if !ok {
		return errors.New("no rebuilt binary is pending")
	}
	st.OldBuild, st.NewBuild = r.current.Build, next.Build
	path, err := WriteState(r.dir, st)
	if err != nil {
		return err
	}
	argv := RestoreArgs(slices.Clone(args), path)
	r.mu.Lock()
	exec := r.exec
	r.mu.Unlock()
	if err := exec(next.Path, argv, os.Environ()); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("could not start the rebuilt codeNERD (%s); this one keeps running: %w", next.Build, err)
	}
	return nil
}

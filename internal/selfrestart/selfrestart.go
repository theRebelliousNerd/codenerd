// Package selfrestart moves a running codeNERD onto a rebuilt binary in
// place, keeping its session.
//
// A watcher sees the executable replaced (a rename over it, or a write in
// place that settles). The session decides when: never during a turn, only
// once it is idle. Then the session's state is written to a restore file and
// the process execs the new binary over itself (same PID, same terminal, so
// tmux and attached terminals stay), with -restore-session naming the file;
// the new process reads it once and resumes the conversation, its mode, its
// brief and its held work, starts again the background jobs and queued turns
// the restart cut (Resume), and names what it does not take up itself
// (Interrupted). An exec that fails leaves the old process
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

// ExecutablePath resolves the running binary's on-disk path. On Linux, if the
// executable was replaced by rename while running, /proc/self/exe carries a
// trailing " (deleted)" suffix; stripping it recovers the replacement path.
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe = CleanExecutablePath(exe)
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = CleanExecutablePath(resolved)
	}
	return filepath.Clean(exe), nil
}

// CleanExecutablePath strips the kernel's " (deleted)" suffix from an unlinked
// /proc/self/exe target so callers still address the on-disk binary path.
func CleanExecutablePath(exe string) string {
	return strings.TrimSuffix(strings.TrimSpace(exe), " (deleted)")
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
	Version     int      `json:"version"`
	SessionID   string   `json:"session_id"`
	WorkMode    string   `json:"work_mode,omitempty"`
	Brief       string   `json:"brief,omitempty"`
	Queued      []string `json:"queued,omitempty"`
	Draft       string   `json:"draft,omitempty"`
	Recurse     bool     `json:"recurse,omitempty"`
	RecurseArgs []string `json:"recurse_args,omitempty"`
	// Interrupted names the work the restart stopped and the new process
	// does not take up itself (a running campaign, which resumes through the
	// campaign resume path; an offered step, which waits for the user), for
	// the restored session to say.
	Interrupted []string `json:"interrupted,omitempty"`
	// Resume is the work the restart cut that the new process starts again
	// by itself once its boot is done: the background jobs that were running
	// and the turns that were queued, in order.
	Resume   []ResumeWork `json:"resume,omitempty"`
	OldBuild string       `json:"old_build"`
	NewBuild string       `json:"new_build"`
}

// Kinds of resumed work.
const (
	ResumeBackground = "background" // a background shard job, started again
	ResumeTurn       = "turn"       // a queued message, queued again
)

// ResumeWork is one piece of work a restart carries over.
type ResumeWork struct {
	Kind string `json:"kind"`
	// A background job: its id (kept, so what refers to it still does), its
	// shard and task, its routed target and constraint, the request it came
	// from, the chain's next step, and its self-repair lane.
	ID         string `json:"id,omitempty"`
	Shard      string `json:"shard,omitempty"`
	Task       string `json:"task,omitempty"`
	Target     string `json:"target,omitempty"`
	Constraint string `json:"constraint,omitempty"`
	Request    string `json:"request,omitempty"`
	FollowUp   string `json:"follow_up,omitempty"`
	SelfRepair string `json:"self_repair,omitempty"`
	// A queued turn: its input, and whether it is a slash command.
	Input   string `json:"input,omitempty"`
	Command bool   `json:"command,omitempty"`
	// Step is set when the queued turn is a chain's step (a self-diagnosis's
	// fix): it runs again as that step, with its preset intent.
	Step *ChainStep `json:"step,omitempty"`
}

// ChainStep is a queued chain step: its instruction (the request), the
// result it acts on (evidence), the verb and target it runs with, and
// whether codeNERD started it itself.
type ChainStep struct {
	Instruction string `json:"instruction"`
	Evidence    string `json:"evidence"`
	Verb        string `json:"verb"`
	Target      string `json:"target"`
	Autonomous  bool   `json:"autonomous,omitempty"`
}

// stateVersion stays 1 while every field added is optional: the process that
// writes the file is the old build and the one that reads it the new, so a
// bump would make every self-update lose its session.
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

// CurrentPath returns the on-disk path of the watched executable.
func (r *Restarter) CurrentPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.current.Path
}

// BinaryChanged records a rebuilt executable. It reports whether the change is
// a new build (a rewrite with the same identity is not).
func (r *Restarter) BinaryChanged(next Identity) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.current.Changed(next) {
		// The executable was restored to the running build. Retire any
		// older candidate rather than executing it under a stale label.
		r.pending = nil
		return false
	}
	if r.pending != nil && !r.pending.Changed(next) {
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
// failure: restore file cleanup is attempted, the pending build stays pending,
// and the error reports exec and cleanup failures, with the old process still running.
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
		if cleanupErr := os.Remove(path); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove restore file %q: %w", path, cleanupErr))
		}
		return fmt.Errorf("could not start the rebuilt codeNERD (%s); this one keeps running: %w", next.Build, err)
	}
	return nil
}

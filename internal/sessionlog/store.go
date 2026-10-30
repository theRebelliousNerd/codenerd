package sessionlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"

	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
)

// Options configures a Store.
type Options struct {
	// Workspace is the project root that holds .nerd.
	Workspace string
	// Session is the session id used for a call that names none.
	Session string
	// Enabled decides, per call, whether exchange files are written. Nil means
	// logging.IsDebugMode: debug_mode or debug_self.
	Enabled func() bool
}

// Store writes a workspace's session folders. It is safe for concurrent use.
type Store struct {
	workspace string
	enabled   func() bool

	mu          sync.Mutex
	session     string
	counters    map[string]int
	transcripts map[string]*Transcript
	// services tracks process-resident health separately from task transcripts.
	services map[string]*serviceState
	touched  map[string]bool

	failures  atomic.Int64
	firstFail atomic.Pointer[error]
	warnOnce  sync.Once
}

// NewStore returns a Store. It does not touch the disk.
func NewStore(o Options) *Store {
	enabled := o.Enabled
	if enabled == nil {
		enabled = logging.IsDebugMode
	}
	return &Store{
		workspace:   o.Workspace,
		session:     o.Session,
		enabled:     enabled,
		counters:    map[string]int{},
		transcripts: map[string]*Transcript{},
		touched:     map[string]bool{},
	}
}

var current atomic.Pointer[Store]

// Install makes s the process's store. A nil s uninstalls.
func Install(s *Store) { current.Store(s) }

// Current returns the installed store, or nil.
func Current() *Store { return current.Load() }

// Active reports whether exchange files are being written right now: a store
// is installed and debug mode or debug-self mode is on.
func Active() bool {
	s := Current()
	return s != nil && s.enabled()
}

// Workspace returns the project root.
func (s *Store) Workspace() string { return s.workspace }

// SetSession changes the session id used for calls that name none.
func (s *Store) SetSession(id string) {
	s.mu.Lock()
	s.session = id
	s.mu.Unlock()
}

// Session returns the default session id.
func (s *Store) Session() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

// Adopt makes id the session calls are filed under when they name none, and
// protects it from pruning. The chat calls it each turn, so /load-session and
// /new-session follow without any other wiring.
func (s *Store) Adopt(id string) {
	if s == nil || ValidateID(id) != nil {
		return
	}
	s.mu.Lock()
	s.session = id
	s.touched[id] = true
	s.mu.Unlock()
}

// resolve picks the session a call is filed under: the one it names when that
// is a real session of this workspace (the default, one already written to, or
// one with a folder), else the default. A call tagged with a placeholder such
// as "current-session" therefore lands in the live session instead of minting
// a folder.
func (s *Store) resolve(named string) string {
	if named == "" || ValidateID(named) != nil {
		return s.Session()
	}
	s.mu.Lock()
	known := named == s.session || s.touched[named]
	s.mu.Unlock()
	if known {
		return named
	}
	if dir, err := SessionDir(s.workspace, named); err == nil {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return named
		}
	}
	return s.Session()
}

// Touched returns the sessions this store has written to, for the lease
// heartbeat.
func (s *Store) Touched() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.touched))
	for id := range s.touched {
		out = append(out, id)
	}
	return out
}

// Failures returns how many writes have failed and the first failure. A failed
// write never fails the model call; it is counted here and warned about once.
func (s *Store) Failures() (int, error) {
	if s == nil {
		return 0, nil
	}
	n := int(s.failures.Load())
	if p := s.firstFail.Load(); p != nil {
		return n, *p
	}
	return n, nil
}

func (s *Store) fail(err error) {
	if s == nil || err == nil {
		return
	}
	s.failures.Add(1)
	s.firstFail.CompareAndSwap(nil, &err)
	s.warnOnce.Do(func() {
		logging.Get(logging.CategorySession).Warn(
			"session log: a write under %s failed (%v); model calls continue, later failures are counted not repeated (Store.Failures)",
			SessionsDir(s.workspace), err)
	})
}

// folderDir returns (creating it) a session's folder for an agent.
func (s *Store) folderDir(session, folder string) (string, error) {
	sdir, err := SessionDir(s.workspace, session)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(sdir, SafeSegment(folder))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.touched[session] = true
	s.mu.Unlock()
	return dir, nil
}

var numbered = regexp.MustCompile(`^(\d{4,})-`)

// next reserves the next exchange number in dir. The counter is seeded from
// the files already there, so numbering continues across a restart, and the
// reservation is taken under a lock, so concurrent calls never share a number.
func (s *Store) next(dir string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, seeded := s.counters[dir]
	if !seeded {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return 0, err
		}
		for _, e := range entries {
			if m := numbered.FindStringSubmatch(e.Name()); m != nil {
				if v, err := strconv.Atoi(m[1]); err == nil && v > n {
					n = v
				}
			}
		}
	}
	n++
	s.counters[dir] = n
	return n, nil
}

func exchangeFileName(n int, purpose string) string {
	return fmt.Sprintf("%04d-%s.json", n, SafeSegment(purpose))
}

func (s *Store) writeFile(path string, data []byte) {
	if err := atomicfile.WriteFile(path, data, 0o644); err != nil {
		s.fail(err)
	}
}

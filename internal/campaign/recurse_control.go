package campaign

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Controlling a running loop from outside it: one loop per workspace (a
// lock the OS releases when the holder dies), a stop the loop honours between
// attempts, and a status read from the journal.

func recurseDir(root string) string      { return filepath.Join(root, ".nerd", "recurse") }
func recurseLockPath(root string) string { return filepath.Join(recurseDir(root), "lock") }
func recurseStopPath(root string) string { return filepath.Join(recurseDir(root), "stop") }

// ErrRecurseRunning reports a second loop on a workspace one already runs on.
// Two loops on one checkout would commit and revert each other's attempts.
var ErrRecurseRunning = errors.New("recurse: another recurse loop is running on this workspace")

// ErrRecurseStopped reports a loop that ended on `nerd campaign recurse stop`.
// It is a clean end, not a failure.
var ErrRecurseStopped = errors.New("recurse: stopped on request")

// acquireRecurseLock takes the workspace's loop lock and records this
// process's pid in it for status. The lock is advisory and held for the life
// of the process; release drops it.
func acquireRecurseLock(root string) (release func(), err error) {
	if err := os.MkdirAll(recurseDir(root), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(recurseLockPath(root), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	ok, err := tryLockFile(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("recurse: lock %s: %w", recurseLockPath(root), err)
	}
	if !ok {
		f.Close()
		pid, _ := os.ReadFile(recurseLockPath(root))
		return nil, fmt.Errorf("%w (pid %s); `nerd campaign recurse stop` ends it", ErrRecurseRunning, strings.TrimSpace(string(pid)))
	}
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
	}
	return func() {
		_ = unlockFile(f)
		_ = f.Close()
	}, nil
}

// recurseRunning reports whether a loop holds the workspace's lock.
func recurseRunning(root string) bool {
	f, err := os.OpenFile(recurseLockPath(root), os.O_RDWR, 0o644)
	if err != nil {
		return false
	}
	defer f.Close()
	ok, err := tryLockFile(f)
	if err != nil {
		return false
	}
	if ok {
		_ = unlockFile(f)
		return false
	}
	return true
}

// RequestRecurseStop asks the loop running on workspace to stop. It stops at
// the next boundary between attempts, so the attempt in flight is judged
// rather than thrown away.
func RequestRecurseStop(workspace string) error {
	if err := os.MkdirAll(recurseDir(workspace), 0o755); err != nil {
		return err
	}
	return os.WriteFile(recurseStopPath(workspace), []byte("stop\n"), 0o644)
}

func recurseStopRequested(root string) bool {
	_, err := os.Stat(recurseStopPath(root))
	return err == nil
}

// RecurseStatus is what the journal says a workspace's loop has done.
type RecurseStatus struct {
	Running       bool
	StopRequested bool
	Pass          int
	// Node is the node being visited, or "" between visits.
	Node string
	// Totals over the journal.
	Cycles, Kept, Reverted, Refused, Unverified int
	// OpenByPass is the open-finding count each finished pass ended on: the
	// trend that says whether the loop is winning.
	OpenByPass []int
	// ThisPass are the current pass's ratchets, oldest first.
	ThisPass []recurseRecord
}

// ReadRecurseStatus reads a workspace's loop status. A workspace recurse has
// never run in has an empty status.
func ReadRecurseStatus(workspace string) (RecurseStatus, error) {
	st := RecurseStatus{Running: recurseRunning(workspace), StopRequested: recurseStopRequested(workspace)}
	recs, err := readRecurseJournal(workspace)
	if err != nil {
		return st, err
	}
	for _, r := range recs {
		switch r.Step {
		case stepPassStart:
			if r.Pass != st.Pass {
				st.ThisPass = nil
			}
			st.Pass = r.Pass
		case stepVisit:
			st.Node = r.Node
		case stepVisitDone:
			st.Node = ""
		case stepPassEnd:
			st.OpenByPass = append(st.OpenByPass, r.Open)
		case stepRatchet:
			st.ThisPass = append(st.ThisPass, r)
			st.Cycles++
			switch r.Outcome {
			case outcomeKept:
				st.Kept++
			case outcomeReverted:
				st.Reverted++
			case outcomeRefused:
				st.Refused++
			case outcomeUnverified:
				st.Unverified++
			}
		}
	}
	return st, nil
}

// String renders the status for a terminal or a chat message.
func (s RecurseStatus) String() string {
	var b strings.Builder
	state := "not running"
	if s.Running {
		state = "running"
	}
	if s.StopRequested {
		state += ", stop requested"
	}
	fmt.Fprintf(&b, "Recurse: %s. Pass %d", state, s.Pass)
	if s.Node != "" {
		fmt.Fprintf(&b, ", visiting %s", s.Node)
	}
	fmt.Fprintf(&b, ".\n%d attempts: %d kept, %d reverted, %d refused, %d unverified.\n", s.Cycles, s.Kept, s.Reverted, s.Refused, s.Unverified)
	if len(s.OpenByPass) > 0 {
		trend := make([]string, len(s.OpenByPass))
		for i, n := range s.OpenByPass {
			trend[i] = strconv.Itoa(n)
		}
		fmt.Fprintf(&b, "Open findings at the end of each pass: %s\n", strings.Join(trend, " -> "))
	}
	for _, r := range s.ThisPass {
		fmt.Fprintf(&b, "  cycle %d  %-9s %s", r.Cycle, strings.TrimPrefix(r.Outcome, "/"), r.Node)
		if r.Commit != "" {
			fmt.Fprintf(&b, "  %s", r.Commit)
		}
		if r.Signature != "" {
			fmt.Fprintf(&b, "  (%s)", r.Signature)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

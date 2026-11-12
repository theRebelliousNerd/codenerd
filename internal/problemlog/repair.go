package problemlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/types"
)

// The self-repair ledger: what codeNERD has tried on its own problems and
// operations, and what came of it (policy/self_repair.mg). It lives under
// .nerd/self_repair so it survives the re-exec onto a fix: the process that
// runs the fix is the one that judges it. The ledger records events; the
// kernel derives what is due and every verdict from its facts.

// Repair lanes.
const (
	LaneRepair   = "repair"   // a hot problem of the problems log
	LaneOptimize = "optimize" // the run's costliest recurring operation
)

// RepairStatus is an attempt's place in its lifecycle.
type RepairStatus string

const (
	// RepairBeingFixed: its diagnosis job or fix turn is running.
	RepairBeingFixed RepairStatus = "being_fixed"
	// RepairPendingRestart: the fix is built, self-checked and installed over
	// the running binary; the process has not restarted onto it yet.
	RepairPendingRestart RepairStatus = "pending_restart"
	// RepairVerifying: a new process runs the fix; the kernel judges it.
	RepairVerifying RepairStatus = "verifying"
	// RepairVerified: the problem stayed away (or the operation got faster).
	RepairVerified RepairStatus = "verified"
	// RepairRegressed: the problem came back under the fix, or the operation
	// got no faster. Counts as a failure.
	RepairRegressed RepairStatus = "regressed"
	// RepairFailed: the job or fix turn ended without an installed fix (it
	// failed, or its build did not pass the self-check). Counts as a failure.
	RepairFailed RepairStatus = "failed"
	// RepairStopped: the user stopped it. Not a failure of the fix.
	RepairStopped RepairStatus = "stopped"
	// RepairNoDefect: its diagnosis found nothing of codeNERD's to fix (the
	// kernel's self_diagnosis_no_defect), so no fix turn ran. Held until its
	// evidence grows (self_repair.mg); not a failure.
	RepairNoDefect RepairStatus = "no_defect"
)

// RepairStatuses is every status, in lifecycle order.
var RepairStatuses = []RepairStatus{RepairBeingFixed, RepairPendingRestart, RepairVerifying, RepairVerified,
	RepairRegressed, RepairFailed, RepairStopped, RepairNoDefect}

// countsAsFailure reports the statuses that move a target toward being left.
func (s RepairStatus) countsAsFailure() bool {
	return s == RepairFailed || s == RepairRegressed
}

// Repair phases while RepairBeingFixed.
const (
	PhaseDiagnosing = "diagnosing" // the read-only background job runs
	PhaseFixing     = "fixing"     // its follow-up fix turn runs
)

// RepairAttempt is one target's record. A target is a problem signature on
// the repair lane, or "timing:<system>.<operation>" on the optimize lane.
type RepairAttempt struct {
	Target string       `json:"target"`
	Lane   string       `json:"lane"`
	Status RepairStatus `json:"status"`
	// Phase is PhaseDiagnosing or PhaseFixing while the status is
	// RepairBeingFixed, "" otherwise.
	Phase string `json:"phase,omitempty"`
	// Job is the background job of the latest attempt.
	Job string `json:"job"`
	// Tries counts attempts started; Failures the ones that failed or
	// regressed.
	Tries    int `json:"tries"`
	Failures int `json:"failures"`
	// UpdatedUnix is when the status last changed; for RepairVerifying, when
	// the process running the fix took it up.
	UpdatedUnix int64 `json:"updated_unix"`
	// BaselineMs is the optimize lane's mean before the fix.
	BaselineMs int64 `json:"baseline_ms,omitempty"`
	// Build names the binary the fix was installed as.
	Build string `json:"build,omitempty"`
	// Note says why the latest attempt ended as it did; a retry is told it,
	// so it does not repeat a fix that did not hold.
	Note string `json:"note,omitempty"`
	// EvidenceCount is, for RepairNoDefect, how often the problem had been
	// logged (or the operation timed) when its diagnosis found nothing to
	// fix: the kernel holds the target until the evidence outgrows it.
	EvidenceCount int `json:"evidence_count,omitempty"`
	// AttemptDiff is, for a fix turn that did not end installed, the
	// workspace-relative file its undone edits were saved to; FailedTests
	// are the tests failing when it ended, one line each. A retry is told
	// both, so it does not make the same change again.
	AttemptDiff string   `json:"attempt_diff,omitempty"`
	FailedTests []string `json:"failed_tests,omitempty"`
}

// FixWindow is when a self-repair fix turn ran, in unix seconds. EndUnix is
// 0 while it runs. A problem logged only inside fix windows is the record of
// those attempts, not a defect to repair (self_problem_from_repair).
type FixWindow struct {
	Job       string `json:"job"`
	StartUnix int64  `json:"start_unix"`
	EndUnix   int64  `json:"end_unix,omitempty"`
}

// RepairLedger is the persistent set of attempts, one per target. It is safe
// for concurrent use: the chat's Update thread and the command goroutine that
// installs a rebuilt binary both record into it.
type RepairLedger struct {
	mu       sync.Mutex
	path     string
	attempts map[string]*RepairAttempt
	windows  []FixWindow
}

const ledgerVersion = 1

type ledgerFile struct {
	Version  int             `json:"version"`
	Attempts []RepairAttempt `json:"attempts"`
	Windows  []FixWindow     `json:"fix_windows,omitempty"`
}

// RepairLedgerPath is the ledger's place in a workspace.
func RepairLedgerPath(workspace string) string {
	return filepath.Join(workspace, ".nerd", "self_repair", "ledger.json")
}

// LoadRepairLedger reads the ledger at path; a missing file is an empty
// ledger. A process that loads the ledger is a new process: every fix that
// was pending a restart now runs here, and is taken up for verification as of
// now.
func LoadRepairLedger(path string, now time.Time) (*RepairLedger, error) {
	l := &RepairLedger{path: path, attempts: map[string]*RepairAttempt{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read self-repair ledger: %w", err)
	}
	var f ledgerFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse self-repair ledger %s: %w", path, err)
	}
	if f.Version != ledgerVersion {
		return nil, fmt.Errorf("self-repair ledger %s is version %d, this build reads %d", path, f.Version, ledgerVersion)
	}
	adopted := false
	// A window still open was the previous process's fix turn, which ended
	// with that process. Its lines are in that process's problems log, never
	// this one's: closed where it began, it attributes nothing here.
	for _, w := range f.Windows {
		if w.EndUnix == 0 {
			w.EndUnix, adopted = w.StartUnix, true
		}
		l.windows = append(l.windows, w)
	}
	for i := range f.Attempts {
		a := f.Attempts[i]
		if a.Status == RepairPendingRestart {
			a.Status, a.UpdatedUnix = RepairVerifying, now.Unix()
			adopted = true
		}
		l.attempts[a.Target] = &a
	}
	if adopted {
		if err := l.saveLocked(); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// Path is the file the ledger persists to.
func (l *RepairLedger) Path() string { return l.path }

// Get returns a copy of target's record.
func (l *RepairLedger) Get(target string) (RepairAttempt, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.attempts[target]
	if !ok {
		return RepairAttempt{}, false
	}
	return *a, true
}

// Attempts returns a copy of every record, by target.
func (l *RepairLedger) Attempts() []RepairAttempt {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.listLocked()
}

func (l *RepairLedger) listLocked() []RepairAttempt {
	out := make([]RepairAttempt, 0, len(l.attempts))
	for _, a := range l.attempts {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Target < out[k].Target })
	return out
}

// Begin records a new attempt on each target, diagnosed by job.
// baselines holds the optimize lane's mean before the fix, by target.
func (l *RepairLedger) Begin(targets []string, lane, job string, baselines map[string]int64, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, t := range targets {
		a, ok := l.attempts[t]
		if !ok {
			a = &RepairAttempt{Target: t}
			l.attempts[t] = a
		}
		a.Lane, a.Status, a.Phase, a.Job = lane, RepairBeingFixed, PhaseDiagnosing, job
		a.Tries++
		a.UpdatedUnix = now.Unix()
		a.BaselineMs = baselines[t]
		a.Build = ""
	}
	return l.saveLocked()
}

// JobTargets is the targets job is working on.
func (l *RepairLedger) JobTargets(job string) []RepairAttempt {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []RepairAttempt
	for _, a := range l.listLocked() {
		if a.Job == job && a.Status == RepairBeingFixed {
			out = append(out, a)
		}
	}
	return out
}

// Fixing moves job's targets from their diagnosis to their fix turn.
func (l *RepairLedger) Fixing(job string, now time.Time) error {
	return l.update(func(a *RepairAttempt) bool {
		if a.Job != job || a.Status != RepairBeingFixed {
			return false
		}
		a.Phase, a.UpdatedUnix = PhaseFixing, now.Unix()
		return true
	})
}

// NoDefect settles job's targets whose diagnosis found nothing of codeNERD's
// to fix, each with the evidence count it was judged on (at least 1), and
// returns how many it settled. It is not a failure.
func (l *RepairLedger) NoDefect(job string, evidence map[string]int, note string, now time.Time) (int, error) {
	n := 0
	err := l.update(func(a *RepairAttempt) bool {
		count, ok := evidence[a.Target]
		if !ok || a.Job != job || a.Status != RepairBeingFixed {
			return false
		}
		settle(a, RepairNoDefect, note, now)
		a.EvidenceCount = max(count, 1)
		n++
		return true
	})
	return n, err
}

// InFixing reports the jobs whose fix turn is running.
func (l *RepairLedger) InFixing() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	seen := map[string]bool{}
	var jobs []string
	for _, a := range l.listLocked() {
		if a.Status == RepairBeingFixed && a.Phase == PhaseFixing && !seen[a.Job] {
			seen[a.Job] = true
			jobs = append(jobs, a.Job)
		}
	}
	return jobs
}

// Installed records that every fix turn now running ended in a rebuilt,
// self-checked binary installed as build: those targets wait for the restart.
func (l *RepairLedger) Installed(build string, now time.Time) (int, error) {
	n := 0
	err := l.update(func(a *RepairAttempt) bool {
		if a.Status != RepairBeingFixed || a.Phase != PhaseFixing {
			return false
		}
		a.Status, a.Phase, a.Build, a.Note, a.UpdatedUnix = RepairPendingRestart, "", build, "", now.Unix()
		n++
		return true
	})
	return n, err
}

// EndJob settles job's targets that are still being fixed with status and
// why. A failure counts toward the targets being left.
func (l *RepairLedger) EndJob(job string, status RepairStatus, note string, now time.Time) (int, error) {
	n := 0
	err := l.update(func(a *RepairAttempt) bool {
		if a.Job != job || a.Status != RepairBeingFixed {
			return false
		}
		settle(a, status, note, now)
		n++
		return true
	})
	return n, err
}

// EndFixing settles every running fix turn's targets: the turn ended.
func (l *RepairLedger) EndFixing(status RepairStatus, note string, now time.Time) (int, error) {
	n := 0
	err := l.update(func(a *RepairAttempt) bool {
		if a.Status != RepairBeingFixed || a.Phase != PhaseFixing {
			return false
		}
		settle(a, status, note, now)
		n++
		return true
	})
	return n, err
}

// Verdict adopts the kernel's verdict on a verifying target.
func (l *RepairLedger) Verdict(target string, status RepairStatus, note string, now time.Time) (bool, error) {
	changed := false
	err := l.update(func(a *RepairAttempt) bool {
		if a.Target != target || a.Status != RepairVerifying {
			return false
		}
		settle(a, status, note, now)
		changed = true
		return true
	})
	return changed, err
}

func settle(a *RepairAttempt, status RepairStatus, note string, now time.Time) {
	a.Status, a.Phase, a.Note, a.UpdatedUnix, a.EvidenceCount = status, "", note, now.Unix(), 0
	a.AttemptDiff, a.FailedTests = "", nil
	if status.countsAsFailure() {
		a.Failures++
	}
}

// Annotate adds to job's failed or stopped attempts what their fix turn left: a
// sentence for the note, the saved diff of its undone edits, and the tests
// failing when it ended. It returns how many attempts it annotated.
func (l *RepairLedger) Annotate(job, note, diff string, failedTests []string) (int, error) {
	n := 0
	err := l.update(func(a *RepairAttempt) bool {
		if a.Job != job || (a.Status != RepairFailed && a.Status != RepairStopped) {
			return false
		}
		if note != "" {
			if a.Note != "" {
				a.Note += "; "
			}
			a.Note += note
		}
		a.AttemptDiff, a.FailedTests = diff, append([]string(nil), failedTests...)
		n++
		return true
	})
	return n, err
}

// OpenFixWindow records that job's fix turn starts now.
func (l *RepairLedger) OpenFixWindow(job string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, w := range l.windows {
		if w.Job == job && w.EndUnix == 0 {
			return nil
		}
	}
	l.windows = append(l.windows, FixWindow{Job: job, StartUnix: now.Unix()})
	return l.saveLocked()
}

// CloseFixWindow records that job's fix turn ended now.
func (l *RepairLedger) CloseFixWindow(job string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	changed := false
	for i := range l.windows {
		if l.windows[i].Job == job && l.windows[i].EndUnix == 0 {
			l.windows[i].EndUnix = max(now.Unix(), l.windows[i].StartUnix)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return l.saveLocked()
}

// FixWindows returns a copy of every fix window, oldest first.
func (l *RepairLedger) FixWindows() []FixWindow {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]FixWindow(nil), l.windows...)
}

// update applies f to every record and saves when it changed one.
func (l *RepairLedger) update(f func(*RepairAttempt) bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	changed := false
	for _, a := range l.attempts {
		if f(a) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return l.saveLocked()
}

func (l *RepairLedger) saveLocked() error {
	if l.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(ledgerFile{Version: ledgerVersion, Attempts: l.listLocked(), Windows: l.windows}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return fmt.Errorf("self-repair ledger dir: %w", err)
	}
	if err := atomicfile.WriteFile(l.path, data, 0o600); err != nil {
		return fmt.Errorf("write self-repair ledger: %w", err)
	}
	return nil
}

// The facts the ledger becomes (policy/self_repair.mg).
const (
	RepairAttemptPredicate  = "self_repair_attempt"
	RepairBaselinePredicate = "self_repair_baseline"
	RepairEvidencePredicate = "self_repair_no_defect_evidence"
	RepairClockPredicate    = "self_repair_clock"
	RepairWindowPredicate   = "self_repair_fix_window"
	RepairFixingPredicate   = "self_repair_fixing"
	RepairDuePredicate      = "self_repair_due"
	RepairShardPredicate    = "self_repair_shard"
	RepairVerifiedPredicate = "self_repair_verified"
	RepairRegressedPred     = "self_repair_regressed"
)

// RepairPredicates are the base predicates ReplaceRepair writes.
var RepairPredicates = []string{RepairAttemptPredicate, RepairBaselinePredicate, RepairEvidencePredicate, RepairClockPredicate, RepairWindowPredicate}

// Facts projects the ledger as self_repair_attempt, self_repair_baseline and
// self_repair_no_defect_evidence rows.
func (l *RepairLedger) Facts() []types.Fact {
	if l == nil {
		return nil
	}
	var out []types.Fact
	for _, a := range l.Attempts() {
		out = append(out, types.Fact{Predicate: RepairAttemptPredicate, Args: []any{
			types.MangleString(a.Target), types.MangleAtom("/" + a.Lane), types.MangleAtom("/" + string(a.Status)),
			types.MangleString(a.Job), int64(a.Tries), int64(a.Failures), a.UpdatedUnix,
		}})
		if a.Lane == LaneOptimize && a.BaselineMs > 0 {
			out = append(out, types.Fact{Predicate: RepairBaselinePredicate, Args: []any{types.MangleString(a.Target), a.BaselineMs}})
		}
		if a.Status == RepairNoDefect {
			out = append(out, types.Fact{Predicate: RepairEvidencePredicate, Args: []any{types.MangleString(a.Target), int64(max(a.EvidenceCount, 1))}})
		}
	}
	return out
}

// WindowFacts projects the fix windows as self_repair_fix_window rows; a
// window still open ends now.
func (l *RepairLedger) WindowFacts(now time.Time) []types.Fact {
	if l == nil {
		return nil
	}
	var out []types.Fact
	for _, w := range l.FixWindows() {
		end := w.EndUnix
		if end == 0 {
			end = max(now.Unix(), w.StartUnix)
		}
		out = append(out, types.Fact{Predicate: RepairWindowPredicate, Args: []any{types.MangleString(w.Job), w.StartUnix, end}})
	}
	return out
}

// ReplaceRepair swaps the kernel's ledger facts and clock for the ledger's
// current state at now. Every retraction error and the assert error come back
// joined.
func ReplaceRepair(k Kernel, l *RepairLedger, now time.Time) error {
	if k == nil {
		return errors.New("problemlog: nil Kernel")
	}
	var errs []error
	for _, pred := range RepairPredicates {
		if err := k.Retract(pred); err != nil {
			errs = append(errs, fmt.Errorf("problemlog: retract %s: %w", pred, err))
		}
	}
	facts := append(l.Facts(), types.Fact{Predicate: RepairClockPredicate, Args: []any{now.Unix()}})
	facts = append(facts, l.WindowFacts(now)...)
	if err := k.AssertBatch(facts); err != nil {
		errs = append(errs, fmt.Errorf("problemlog: assert %d self-repair facts: %w", len(facts), err))
	}
	return errors.Join(errs...)
}

// Due is one target the kernel derives as due, with its lane.
type Due struct {
	Target string
	Lane   string
}

// DueTargets asks the kernel what is due (self_repair_due), sorted by lane
// then target.
func DueTargets(k Querier) ([]Due, error) {
	rows, err := k.Query(RepairDuePredicate)
	if err != nil {
		return nil, fmt.Errorf("problemlog: query %s: %w", RepairDuePredicate, err)
	}
	var out []Due
	for _, f := range rows {
		if len(f.Args) != 2 {
			continue
		}
		out = append(out, Due{Target: factString(f.Args[0]), Lane: trimName(factString(f.Args[1]))})
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].Lane != out[k].Lane {
			return out[i].Lane < out[k].Lane
		}
		return out[i].Target < out[k].Target
	})
	return out, nil
}

// Verdicts asks the kernel which verifying targets held and which regressed.
func Verdicts(k Querier) (verified, regressed []string, err error) {
	held, err := k.Query(RepairVerifiedPredicate)
	if err != nil {
		return nil, nil, fmt.Errorf("problemlog: query %s: %w", RepairVerifiedPredicate, err)
	}
	back, err := k.Query(RepairRegressedPred)
	if err != nil {
		return nil, nil, fmt.Errorf("problemlog: query %s: %w", RepairRegressedPred, err)
	}
	return targetColumn(held), targetColumn(back), nil
}

// targetColumn is the sorted first arguments of one-column rows.
func targetColumn(rows []types.Fact) []string {
	var out []string
	for _, f := range rows {
		if len(f.Args) == 1 {
			out = append(out, factString(f.Args[0]))
		}
	}
	sort.Strings(out)
	return out
}

// RepairShard is the persona the kernel names for a self-repair diagnosis,
// "" when it names none.
func RepairShard(k Querier) (string, error) {
	rows, err := k.Query(RepairShardPredicate)
	if err != nil {
		return "", fmt.Errorf("problemlog: query %s: %w", RepairShardPredicate, err)
	}
	for _, f := range rows {
		if len(f.Args) == 1 {
			return trimName(factString(f.Args[0])), nil
		}
	}
	return "", nil
}

func trimName(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

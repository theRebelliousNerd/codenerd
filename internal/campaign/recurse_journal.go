package campaign

import (
	"fmt"
	"path/filepath"
	"time"

	"codenerd/internal/jsonl"
)

// The recurse journal is the loop's ledger: one JSON line per state change,
// appended as it happens, under .nerd/recurse/journal.jsonl. A killed run
// resumes from it, `nerd campaign recurse status` reads it, and it is the
// record the owner inspects after a night of unattended cycles.
//
// It is not the source of truth for what was kept -- git is. A kept attempt is
// a commit whose trailers name its cycle; the journal line after it can be lost
// to a crash, and resume reconciles the two (resumeRecurse).

// Journal steps.
const (
	stepPassStart = "pass_start"
	stepVisit     = "visit"
	stepAttempt   = "attempt"
	stepRatchet   = "ratchet"
	stepVisitDone = "visit_done"
	stepPassEnd   = "pass_end"
	stepStopped   = "stopped"
	// stepImproveSkipped: the pass's angle had nothing to measure at a node.
	stepImproveSkipped = "improve_skipped"
)

// recurseRecord is one journal line.
type recurseRecord struct {
	Time  time.Time `json:"time"`
	Step  string    `json:"step"`
	Pass  int       `json:"pass"`
	Cycle int       `json:"cycle,omitempty"`
	Node  string    `json:"node,omitempty"`
	// Finding is the target of a fix attempt.
	Finding string `json:"finding,omitempty"`
	// Angle is an improvement attempt's angle.
	Angle string `json:"angle,omitempty"`
	// Metrics are the numbers around an attempt: "tests 10->12, lines 300->280".
	Metrics string `json:"metrics,omitempty"`
	// Outcome is a ratchet's result: kept, reverted, refused, unverified.
	Outcome   string `json:"outcome,omitempty"`
	Signature string `json:"signature,omitempty"`
	Commit    string `json:"commit,omitempty"`
	// Open counts the findings open after a measurement.
	Open   int    `json:"open,omitempty"`
	Detail string `json:"detail,omitempty"`
	// Tried is what a reverted attempt changed, as a bounded patch, and Why
	// is the ratchet's reason for reverting it. The next attempt at the same
	// work is handed both, so it does not repeat the attempt that failed.
	Tried string `json:"tried,omitempty"`
	Why   string `json:"why,omitempty"`
}

// RecurseJournalPath is where the journal lives in a workspace.
func RecurseJournalPath(workspace string) string {
	return filepath.Join(workspace, ".nerd", "recurse", "journal.jsonl")
}

// recurseJournal appends records and fails loudly: a ledger that silently
// stops recording is how an unattended run becomes uninspectable.
type recurseJournal struct {
	log *jsonl.Appender
	now func() time.Time
}

func openRecurseJournal(workspace string) (*recurseJournal, error) {
	log, err := jsonl.Open(RecurseJournalPath(workspace))
	if err != nil {
		return nil, fmt.Errorf("recurse journal: %w", err)
	}
	return &recurseJournal{log: log, now: time.Now}, nil
}

func (j *recurseJournal) append(r recurseRecord) error {
	r.Time = j.now().UTC()
	before, _ := j.log.Failures()
	j.log.Append(r)
	if after, first := j.log.Failures(); after > before {
		return fmt.Errorf("recurse journal: %s: %w", j.log.Path(), first)
	}
	return nil
}

func (j *recurseJournal) close() error { return j.log.Close() }

// readRecurseJournal returns every record, oldest first. A missing journal is
// an empty one.
func readRecurseJournal(workspace string) ([]recurseRecord, error) {
	recs, _, err := jsonl.Read[recurseRecord](RecurseJournalPath(workspace))
	if err != nil {
		return nil, fmt.Errorf("recurse journal: %w", err)
	}
	return recs, nil
}

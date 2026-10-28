package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"codenerd/internal/logging"
)

// Truthful, resumable re-embedding of the generic vectors table.
//
// A force re-embed used to end with a nil error whenever it embedded at least
// one row, whatever it left behind: a row whose embedding failed kept its old
// vector, which -- before vectors were stamped with an embedding space -- stayed
// rankable by brute force and came back into the index through the dimension-only
// reconcile and backfill. Now a failed row keeps its old vector AND its old
// stamp, so it can rank nowhere, and the pass reports it: the run returns an
// *IncompleteReembedError unless every embeddable row is in the target space.
//
// The run's state is a row in embedding_migrations, so an interrupted or
// incomplete run is continued (rows already in the target space are skipped)
// rather than restarted, and a restart after a completed run is a true force.
// What is deliberately not here: a shadow generation that readers switch to
// atomically. The stamp makes each row self-describing, which is what readers
// need; a whole-store activation barrier is a separate design (see
// Docs/audits/embedding-representation-and-migration.md, "as built").

// ErrIncompleteReembed matches every *IncompleteReembedError.
var ErrIncompleteReembed = errors.New("re-embedding left rows outside the target embedding space")

// IncompleteReembedError says a re-embed pass did not bring every embeddable
// row into the target space. The pass's successful work is durable; running the
// pass again resumes it.
type IncompleteReembedError struct {
	Table  string // "vectors", "prompt_atoms", "reasoning_traces" or "learnings"
	Target string // embedding space the pass was producing
	Total  int64  // rows the pass was responsible for
	Done   int64  // rows now in the target space
	Failed int64  // rows this pass could not (re)embed
	RunID  string // migration run to resume; empty for tables with no run record
	Cause  error  // most recent provider error, when there was one
}

func (e *IncompleteReembedError) Error() string {
	msg := fmt.Sprintf("%s re-embed incomplete: %d of %d rows are in space %q, %d failed this pass", e.Table, e.Done, e.Total, e.Target, e.Failed)
	if e.RunID != "" {
		msg += fmt.Sprintf(" (run %s resumes where it stopped)", e.RunID)
	}
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

func (e *IncompleteReembedError) Is(target error) bool { return target == ErrIncompleteReembed }
func (e *IncompleteReembedError) Unwrap() error        { return e.Cause }

const (
	migrationRunning    = "running"
	migrationIncomplete = "incomplete"
	migrationComplete   = "complete"
)

const embeddingMigrationsDDL = `CREATE TABLE IF NOT EXISTS embedding_migrations (
	table_name   TEXT PRIMARY KEY,
	run_id       TEXT NOT NULL,
	target_space TEXT NOT NULL,
	state        TEXT NOT NULL,
	total        INTEGER NOT NULL DEFAULT 0,
	migrated     INTEGER NOT NULL DEFAULT 0,
	failed       INTEGER NOT NULL DEFAULT 0,
	started_at   DATETIME NOT NULL,
	updated_at   DATETIME NOT NULL
)`

// vectorMigrationRun is the run a force pass is part of.
type vectorMigrationRun struct {
	ID      string
	Target  string
	Resumed bool
}

// resumeSkipSpace is the space whose rows a pass skips: the target on a
// resumed run (that is the progress), nothing on a fresh one.
func resumeSkipSpace(r vectorMigrationRun) string {
	if r.Resumed {
		return r.Target
	}
	return ""
}

// beginVectorMigration opens (or resumes) the run for the vectors table. A
// stored run resumes when it did not complete and was producing the same target
// space; any other stored state is replaced by a fresh run.
func (s *LocalStore) beginVectorMigration(target string) (vectorMigrationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(embeddingMigrationsDDL); err != nil {
		return vectorMigrationRun{}, fmt.Errorf("prepare embedding migration record: %w", err)
	}
	var runID, prevTarget, state string
	err := s.db.QueryRow(`SELECT run_id, target_space, state FROM embedding_migrations WHERE table_name = 'vectors'`).Scan(&runID, &prevTarget, &state)
	now := time.Now().UTC()
	switch {
	case err == nil && state != migrationComplete && prevTarget == target:
		if _, err := s.db.Exec(`UPDATE embedding_migrations SET state = ?, updated_at = ? WHERE table_name = 'vectors'`, migrationRunning, now); err != nil {
			return vectorMigrationRun{}, fmt.Errorf("record resumed embedding migration: %w", err)
		}
		return vectorMigrationRun{ID: runID, Target: target, Resumed: true}, nil
	case err == nil || errors.Is(err, sql.ErrNoRows):
		runID = strconv.FormatInt(now.UnixNano(), 36)
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO embedding_migrations
			(table_name, run_id, target_space, state, total, migrated, failed, started_at, updated_at)
			VALUES ('vectors', ?, ?, ?, 0, 0, 0, ?, ?)`, runID, target, migrationRunning, now, now); err != nil {
			return vectorMigrationRun{}, fmt.Errorf("record embedding migration: %w", err)
		}
		return vectorMigrationRun{ID: runID, Target: target}, nil
	default:
		return vectorMigrationRun{}, fmt.Errorf("read embedding migration record: %w", err)
	}
}

// vectorMigrationReport is what a finished (or interrupted) pass certifies.
type vectorMigrationReport struct {
	RunID  string
	Target string
	Cov    VectorCoverage
	Failed int
}

// Complete reports whether every embeddable row is in the target space and the
// pass lost none.
func (r vectorMigrationReport) Complete() bool {
	return r.Failed == 0 && r.Cov.Pending() == 0
}

func (r vectorMigrationReport) incompleteError(cause error) error {
	return &IncompleteReembedError{
		Table: "vectors", Target: r.Target, Total: r.Cov.Total, Done: r.Cov.Eligible,
		Failed: int64(r.Failed), RunID: r.RunID, Cause: cause,
	}
}

// finishVectorMigration measures coverage against the space the pass produced
// and records the outcome. Coverage, not the pass's own counters, decides
// completeness: it is the table as readers will see it.
func (s *LocalStore) finishVectorMigration(run vectorMigrationRun, target string, failed int, cause error) (vectorMigrationReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cov, err := s.vectorCoverageLocked(target)
	if err != nil {
		return vectorMigrationReport{}, err
	}
	report := vectorMigrationReport{RunID: run.ID, Target: target, Cov: cov, Failed: failed}
	state := migrationComplete
	if !report.Complete() {
		state = migrationIncomplete
		logging.Get(logging.CategoryStore).Warn("Force re-embed of vectors incomplete: %d of %d rows are in space %q, %d failed this pass (cause: %v)",
			cov.Eligible, cov.Total, target, failed, cause)
	}
	if _, err := s.db.Exec(`UPDATE embedding_migrations
		SET target_space = ?, state = ?, total = ?, migrated = ?, failed = ?, updated_at = ?
		WHERE table_name = 'vectors' AND run_id = ?`,
		target, state, cov.Total, cov.Eligible, failed, time.Now().UTC(), run.ID); err != nil {
		return report, fmt.Errorf("record embedding migration outcome: %w", err)
	}
	return report, nil
}

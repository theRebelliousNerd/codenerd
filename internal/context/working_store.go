package context

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/sqlpragmas"
	"codenerd/internal/tools"

	_ "github.com/mattn/go-sqlite3"
)

// WorkingRecord retains an observation independently of its active-window life.
// Kind identifies the exact request; Revision identifies the observed artifact;
// Digest identifies what came back, so two requests that differ in their
// arguments but produced the same observation are recognised as one. Start
// and End are the line span a content read covered (End 0: no span recorded),
// so a later read covering an earlier one's span can replace it.
type WorkingRecord struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	Revision string `json:"revision"`
	Kind     string `json:"kind"`
	Step     int64  `json:"step"`
	Body     string `json:"body"`
	Digest   string `json:"digest,omitempty"`
	Start    int64  `json:"start,omitempty"`
	End      int64  `json:"end,omitempty"`
	Failed   bool   `json:"failed"`
}

func workingDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// WorkingStore is durable cold storage; bodies are loaded only after selection.
// Scope is part of every lookup, including explicit recall by record ID.
type WorkingStore struct {
	db    *sql.DB
	scope string
}

func OpenWorkingStore(workspace, scope string) (*WorkingStore, error) {
	dir, err := tools.ResolveWorkspacePath(context.Background(), workspace, filepath.Join(".nerd", "context"))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path, err := tools.ResolveWorkspacePath(context.Background(), workspace, filepath.Join(dir, workingDigest(scope)+".db"))
	if err != nil {
		return nil, err
	}
	// Every SQLite open in the repo goes through the pragma profile (the
	// sqlpragmas open-site audit fails a bare sql.Open); the connector hook
	// tunes each pooled connection, so the pool size below is a choice, not a
	// requirement of the tuning.
	db, err := sqlpragmas.OpenWithPragmas("sqlite3", filepath.ToSlash(path)+"?_busy_timeout=5000&_journal_mode=WAL", sqlpragmas.ProfileHot)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS working_records (
		id TEXT PRIMARY KEY, scope TEXT NOT NULL, entity TEXT NOT NULL,
		revision TEXT NOT NULL, kind TEXT NOT NULL, step INTEGER NOT NULL,
		body TEXT NOT NULL, failed INTEGER NOT NULL, digest TEXT NOT NULL DEFAULT '',
		span_start INTEGER NOT NULL DEFAULT 0, span_end INTEGER NOT NULL DEFAULT 0);
		CREATE INDEX IF NOT EXISTS working_entity ON working_records(scope,entity,step DESC);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &WorkingStore{db: db, scope: scope}, nil
}

func (s *WorkingStore) Close() error { return s.db.Close() }

// WorkingSearchHit is a Search result row: metadata plus body_chars so a
// caller can tell an empty observation from a body it has not read yet. It
// deliberately omits Body — Search discovers handles, and recall_context with
// the id returns the body by page.
type WorkingSearchHit struct {
	ID        string `json:"id"`
	Entity    string `json:"entity"`
	Revision  string `json:"revision"`
	Kind      string `json:"kind"`
	Step      int64  `json:"step"`
	Failed    bool   `json:"failed"`
	BodyChars int64  `json:"body_chars"`
	Start     int64  `json:"start,omitempty"`
	End       int64  `json:"end,omitempty"`
}

// Search discovers handles even after they leave the bounded candidate slice.
// Search is literal, paginated and scope-local; it grants no execution authority.
func (s *WorkingStore) Search(ctx context.Context, query string, offset, limit int) (string, error) {
	if query == "" || len(query) > 4096 || offset < 0 || offset > 1<<30 || limit < 1 || limit > 50 {
		return "", fmt.Errorf("invalid context search bounds")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,entity,revision,kind,step,failed,length(body),span_start,span_end FROM working_records
		WHERE scope=? AND (instr(entity,?)>0 OR instr(kind,?)>0 OR instr(body,?)>0)
		ORDER BY step DESC,id LIMIT ? OFFSET ?`, s.scope, query, query, query, limit, offset)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	result := []WorkingSearchHit{}
	for rows.Next() {
		var h WorkingSearchHit
		if err := rows.Scan(&h.ID, &h.Entity, &h.Revision, &h.Kind, &h.Step, &h.Failed, &h.BodyChars, &h.Start, &h.End); err != nil {
			return "", err
		}
		result = append(result, h)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	data, err := json.Marshal(struct {
		Records  []WorkingSearchHit `json:"historical_records"`
		Next     int                `json:"next_offset"`
		ReadWith string             `json:"read_with"`
	}{result, offset + len(result), `recall_context {"id": "<id>"} returns the observation body`})
	return string(data), err
}

func (s *WorkingStore) Save(ctx context.Context, r WorkingRecord) error {
	// An empty ID would collide every anonymous record onto one row, and
	// ON CONFLICT DO NOTHING would then silently keep the first body and
	// drop the rest. Refuse instead of storing unaddressable observations.
	if r.ID == "" {
		return fmt.Errorf("cannot save a working record with an empty id")
	}
	if r.Digest == "" {
		r.Digest = workingDigest(r.Body)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO working_records (id,scope,entity,revision,kind,step,body,failed,digest,span_start,span_end) VALUES (?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
		r.ID, s.scope, r.Entity, r.Revision, r.Kind, r.Step, r.Body, r.Failed, r.Digest, r.Start, r.End)
	return err
}

// Candidates returns metadata for the current dependency slice, with no body IO.
func (s *WorkingStore) Candidates(ctx context.Context, entities []string, limit int) ([]WorkingRecord, error) {
	if len(entities) == 0 {
		return nil, nil
	}
	// SQLite treats LIMIT -1 as "no limit", so a non-positive limit would
	// silently unbind a slice documented as bounded. The only caller passes
	// 256; anything else is a bug at the call site, not a request for all.
	if limit <= 0 {
		return nil, fmt.Errorf("invalid candidates limit %d", limit)
	}
	args := []any{s.scope}
	marks := make([]string, len(entities))
	for i, e := range entities {
		marks[i] = "?"
		args = append(args, e)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT id,entity,revision,kind,step,failed,digest,span_start,span_end FROM working_records
		WHERE scope=? AND entity IN (`+strings.Join(marks, ",")+`) ORDER BY step DESC,id LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	return scanWorkingMetadata(rows)
}

// Records returns metadata for the named observations, with no body IO.
// Selection loads the loop's recent observations by id: they are its working
// memory whatever file they came from, and the dependency slice Candidates
// serves never reaches a file with no import link to the focus.
func (s *WorkingStore) Records(ctx context.Context, ids []string) ([]WorkingRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := []any{s.scope}
	marks := make([]string, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,entity,revision,kind,step,failed,digest,span_start,span_end FROM working_records
		WHERE scope=? AND id IN (`+strings.Join(marks, ",")+`) ORDER BY step DESC,id`, args...)
	if err != nil {
		return nil, err
	}
	return scanWorkingMetadata(rows)
}

// scanWorkingMetadata reads the metadata columns Candidates and Records select.
func scanWorkingMetadata(rows *sql.Rows) ([]WorkingRecord, error) {
	defer rows.Close()
	var result []WorkingRecord
	for rows.Next() {
		var r WorkingRecord
		if err := rows.Scan(&r.ID, &r.Entity, &r.Revision, &r.Kind, &r.Step, &r.Failed, &r.Digest, &r.Start, &r.End); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Read retrieves a page of a record's body from a character offset, retaining
// provenance on every page. A limit of zero or less reads to the end: the
// store serves what is asked for whole, and whether a body fits a request is
// decided where the request is built, against the configured window. It used
// to refuse any page over 16000 characters, so a selected observation longer
// than that could never be shown in full, only pointed at.
func (s *WorkingStore) Read(ctx context.Context, id string, offset, limit int) (WorkingRecord, int, error) {
	if offset < 0 || offset > 1<<30 {
		return WorkingRecord{}, 0, fmt.Errorf("invalid context page bounds")
	}
	if limit <= 0 {
		limit = 1 << 30
	}
	var r WorkingRecord
	var length int
	err := s.db.QueryRowContext(ctx, `SELECT id,entity,revision,kind,step,failed,substr(body,?,?),length(body)
		FROM working_records WHERE scope=? AND id=?`, offset+1, limit, s.scope, id).
		Scan(&r.ID, &r.Entity, &r.Revision, &r.Kind, &r.Step, &r.Failed, &r.Body, &length)
	if errors.Is(err, sql.ErrNoRows) {
		// The model reads this: a driver's "sql: no rows in result set" gave it
		// nothing to act on, and on 2026-09-18 two such failures in a row, with
		// a refused edit between them, ended a turn on the working policy's
		// tool-failure stop.
		return WorkingRecord{}, 0, fmt.Errorf("no archived observation has id %q in this task's working context (ids appear in the working section and in \"Observation archived\" pointers); recall_context with query=<text> searches the archive when the id is unknown", id)
	}
	return r, length, err
}

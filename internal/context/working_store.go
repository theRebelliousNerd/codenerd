package context

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	digest := workingDigest(scope)
	path, err := tools.ResolveWorkspacePath(context.Background(), workspace, filepath.Join(dir, digest+".db"))
	if err != nil {
		return nil, err
	}
	// The owner record comes before the database: an archive the retention
	// policy finds without one was minted before owners were recorded and is
	// pruned (working_retention.go), so a live scope must never be seen in
	// that state. No record, no archive.
	if err := recordWorkingOwner(dir, digest, workingOwner{PID: os.Getpid(), Host: thisHost()}); err != nil {
		return nil, fmt.Errorf("record the working scope's owner: %w", err)
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
		span_start INTEGER NOT NULL DEFAULT 0, span_end INTEGER NOT NULL DEFAULT 0,
		handle TEXT NOT NULL DEFAULT '');
		CREATE INDEX IF NOT EXISTS working_entity ON working_records(scope,entity,step DESC);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// Archives opened before handles existed have the table and not the
	// column. CREATE TABLE IF NOT EXISTS does not add it.
	if err := ensureWorkingHandles(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &WorkingStore{db: db, scope: scope}, nil
}

// ensureWorkingHandles adds the short-handle column to an archive that
// predates it, fills empty handles in step order, then enforces one handle
// per scope. The unique index is created after the backfill: every legacy
// row starts as the empty string, and that value would collide with itself.
func ensureWorkingHandles(db *sql.DB) error {
	has, err := workingColumn(db, "handle")
	if err != nil {
		return err
	}
	if !has {
		if _, err := db.Exec(`ALTER TABLE working_records ADD COLUMN handle TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if err := backfillWorkingHandles(db); err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS working_handle ON working_records(scope, handle)`)
	return err
}

func workingColumn(db *sql.DB, name string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(working_records)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var col, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &col, &typ, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if col == name {
			return true, nil
		}
	}
	return false, rows.Err()
}

func backfillWorkingHandles(db *sql.DB) error {
	rows, err := db.Query(`SELECT DISTINCT scope FROM working_records WHERE handle = ''`)
	if err != nil {
		return err
	}
	var scopes []string
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			rows.Close()
			return err
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, scope := range scopes {
		if err := backfillWorkingHandleScope(db, scope); err != nil {
			return err
		}
	}
	return nil
}

func backfillWorkingHandleScope(db *sql.DB, scope string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var next int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(CAST(handle AS INTEGER)), 0) FROM working_records WHERE scope = ? AND handle != ''`, scope).Scan(&next); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT id FROM working_records WHERE scope = ? AND handle = '' ORDER BY step ASC, id ASC`, scope)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		next++
		if _, err := tx.Exec(`UPDATE working_records SET handle = ? WHERE id = ?`, strconv.FormatInt(next, 10), id); err != nil {
			return err
		}
	}
	return tx.Commit()
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
// The id in each hit is the short handle, the same string recall_context takes.
// The storage id is not in the payload: a 64-hex id was copied wrong twice on
// 2026-09-29 (session 20260929_052520).
func (s *WorkingStore) Search(ctx context.Context, query string, offset, limit int) (string, error) {
	if query == "" || len(query) > 4096 || offset < 0 || offset > 1<<30 || limit < 1 || limit > 50 {
		return "", fmt.Errorf("invalid context search bounds")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT handle,entity,revision,kind,step,failed,length(body),span_start,span_end FROM working_records
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT handle FROM working_records WHERE scope=? AND id=?`, s.scope, r.ID).Scan(&existing)
	if err == nil {
		// The row is already addressable. A second save keeps that handle;
		// minting another would make the pointer the model holds a lie.
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var next int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(CAST(handle AS INTEGER)), 0) + 1 FROM working_records WHERE scope=? AND handle != ''`, s.scope).Scan(&next); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO working_records (id,scope,entity,revision,kind,step,body,failed,digest,span_start,span_end,handle) VALUES (?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
		r.ID, s.scope, r.Entity, r.Revision, r.Kind, r.Step, r.Body, r.Failed, r.Digest, r.Start, r.End, strconv.FormatInt(next, 10)); err != nil {
		return err
	}
	return tx.Commit()
}

// Handle is the short ordinal for a storage id, empty only when the store
// has no such row.
func (s *WorkingStore) Handle(ctx context.Context, id string) (string, error) {
	var handle string
	err := s.db.QueryRowContext(ctx, `SELECT handle FROM working_records WHERE scope=? AND id=?`, s.scope, id).Scan(&handle)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && handle == "") {
		return "", fmt.Errorf("working observation %s has no recall handle", id)
	}
	return handle, err
}

// ResolveHandle maps a short handle to the storage id and the entity. The
// storage id is not a handle: passing it here is a miss, on purpose.
func (s *WorkingStore) ResolveHandle(ctx context.Context, handle string) (id, entity string, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT id, entity FROM working_records WHERE scope=? AND handle=?`, s.scope, handle).Scan(&id, &entity)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", noWorkingObservation(handle)
	}
	return id, entity, err
}

// Records returns metadata for the named observations, with no body IO: the
// ledger's rows, whatever file they came from.
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

// noWorkingObservation is what the model reads when a handle does not resolve.
// A driver's "sql: no rows in result set" gave it nothing to act on, and on
// 2026-09-18 two such failures in a row, with a refused edit between them,
// ended a turn on the working policy's tool-failure stop.
func noWorkingObservation(id string) error {
	return fmt.Errorf("no archived observation has id %q in this task's working context (ids appear in the working section and in \"Observation archived\" pointers); recall_context with query=<text> searches the archive when the id is unknown", id)
}

// Read retrieves a page of a record's body from a character offset, retaining
// provenance on every page. id is the short handle. A limit of zero or less
// reads to the end: the store serves what is asked for whole, and whether a
// body fits a request is decided where the request is built, against the
// configured window. It used to refuse any page over 16000 characters, so a
// selected observation longer than that could never be shown in full, only
// pointed at. The record's ID is the handle, so the page the model reads
// does not carry the storage id.
func (s *WorkingStore) Read(ctx context.Context, id string, offset, limit int) (WorkingRecord, int, error) {
	if offset < 0 || offset > 1<<30 {
		return WorkingRecord{}, 0, fmt.Errorf("invalid context page bounds")
	}
	if limit <= 0 {
		limit = 1 << 30
	}
	var r WorkingRecord
	var length int
	err := s.db.QueryRowContext(ctx, `SELECT handle,entity,revision,kind,step,failed,substr(body,?,?),length(body)
		FROM working_records WHERE scope=? AND handle=?`, offset+1, limit, s.scope, id).
		Scan(&r.ID, &r.Entity, &r.Revision, &r.Kind, &r.Step, &r.Failed, &r.Body, &length)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkingRecord{}, 0, noWorkingObservation(id)
	}
	return r, length, err
}

package store

import (
	"bytes"
	"codenerd/internal/logging"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Semantic recall reads vec_index, which no trigger can track, and the JIT
// cache keys knowledge on KnowledgeSourceGeneration. Two index-only changes
// left that token alone: a backfill still building the index (recall serves
// a partial index as healthy) and a repair that puts drifted rows back. A
// prompt compiled from the partial or drifted index stayed cached after the
// index recovered (audit knowledge-index-repair-cache-freshness).

// While the backfill is building the index, the knowledge generation is not
// readable, so a compile in that window is not cached as healthy.
func TestKnowledgeSourceGeneration_UnreadableWhileTheIndexIsBuilding(t *testing.T) {
	hold := make(chan struct{})
	backfillStartHold = hold
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		backfillStartHold = nil
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.SetEmbeddingEngine(&mockSimpleEngine{})
	// Reset immediately: the background job must use the hold captured by
	// SetEmbeddingEngine, not race with this write by rereading the hook.
	backfillStartHold = nil
	if !s.vectorExt.Load() {
		close(hold)
		t.Skip("sqlite-vec not available in this build")
	}
	s.backfillMu.Lock()
	done := s.backfillDone
	s.backfillMu.Unlock()
	if done == nil {
		close(hold)
		t.Fatal("backfill lost its pending generation before the captured hold was released")
	}
	select {
	case <-done:
		close(hold)
		t.Fatal("backfill completed before the captured hold was released")
	// Give the background goroutine time to run after the hook was cleared.
	// An immediate default select only checks that it has not been scheduled
	// yet, and would also pass if it reread nil or skipped the hold entirely.
	case <-time.After(100 * time.Millisecond):
	}
	ctx := context.Background()
	if tok, err := s.KnowledgeSourceGeneration(ctx); err == nil {
		close(hold)
		t.Fatalf("generation %q read while the index backfill is pending: a partial index looked complete", tok)
	}
	close(hold)
	wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(wctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.KnowledgeSourceGeneration(ctx); err != nil {
		t.Fatalf("generation after the backfill: %v", err)
	}
}

// Re-indexing drifted rows changes what recall returns and nothing a trigger
// sees: the generation has to move with it.
func TestKnowledgeSourceGeneration_MovesWhenTheIndexIsRepaired(t *testing.T) {
	s := newVecStore(t)
	storeProbes(t, s, 2)
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM vectors ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM vec_index WHERE rowid = ?`, id); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	drifted, err := s.KnowledgeSourceGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if healed, err := s.ReconcileVecIndex(ctx); err != nil || healed != 1 {
		t.Fatalf("ReconcileVecIndex = %d, %v; want the one drifted row", healed, err)
	}
	repaired, err := s.KnowledgeSourceGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repaired == drifted {
		t.Fatalf("generation %q did not move when the index was repaired", repaired)
	}
	again, err := s.KnowledgeSourceGeneration(ctx)
	if err != nil || again != repaired {
		t.Fatalf("an unchanged store gave %q then %q (%v)", repaired, again, err)
	}
}

// Re-backfilling populated rowids must replace their index data without the
// UNIQUE constraint warnings sqlite-vec emits for INSERT OR REPLACE alone.
func TestBackfillVecIndex_ExistingRowIDsAreIdempotent(t *testing.T) {
	s := newVecStore(t)
	storeProbes(t, s, 2)

	workspace := t.TempDir()
	logging.ApplyConfig(logging.Config{DebugMode: true, Level: "debug", Format: "text"})
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ApplyConfig(logging.Config{})
		logging.ClearInjectedConfig()
	})
	if err := logging.Initialize(workspace); err != nil {
		t.Fatalf("initialize logging: %v", err)
	}

	// Give the synchronous backfill a current generation without recreating
	// vec_index, which would remove the conflicting rowids under test.
	done := make(chan struct{})
	s.backfillMu.Lock()
	s.backfillDone = done
	s.backfillMu.Unlock()
	defer func() {
		s.backfillMu.Lock()
		s.backfillDone = nil
		s.backfillMu.Unlock()
		close(done)
	}()
	engine := &mockSimpleEngine{}
	for _, name := range []string{"unchanged", "updated", "repeated"} {
		t.Run(name, func(t *testing.T) {
			if name == "updated" {
				// Only change the source row: a no-op backfill would leave stale
				// content, metadata and embedding in the existing index entry.
				if _, err := s.db.Exec(`UPDATE vectors SET content = ?, metadata = ?, embedding = ? WHERE id = (SELECT MIN(id) FROM vectors)`, "updated probe", `{"i":0,"updated":true}`, `[0.4,0.3,0.2,0.1]`); err != nil {
					t.Fatalf("update source vector: %v", err)
				}
			}
			s.backfillVecIndex(engine.Dimensions(), embeddingSpace(engine), s.vectorExt.Load(), done)
			var count int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM vec_index`).Scan(&count); err != nil {
				t.Fatalf("count index rows: %v", err)
			}
			if count != 2 {
				t.Fatalf("index rows = %d; want 2 after re-backfill", count)
			}
			rows, err := s.db.Query(`SELECT v.id, v.content, v.metadata, v.embedding, i.content, i.metadata, i.embedding FROM vectors v JOIN vec_index i ON i.rowid = v.id ORDER BY v.id`)
			if err != nil {
				t.Fatalf("read backfilled rows: %v", err)
			}
			defer func() {
				if err := rows.Close(); err != nil {
					t.Errorf("close backfilled rows: %v", err)
				}
			}()
			matched := 0
			for rows.Next() {
				var id int64
				var content, metadata, indexedContent, indexedMetadata string
				var embeddingJSON, indexedEmbedding []byte
				if err := rows.Scan(&id, &content, &metadata, &embeddingJSON, &indexedContent, &indexedMetadata, &indexedEmbedding); err != nil {
					t.Fatalf("scan backfilled row: %v", err)
				}
				var vector []float32
				if err := json.Unmarshal(embeddingJSON, &vector); err != nil {
					t.Fatalf("decode source embedding for %d: %v", id, err)
				}
				if indexedContent != content || indexedMetadata != metadata || !bytes.Equal(indexedEmbedding, encodeFloat32Slice(vector)) {
					t.Errorf("index row %d differs from its source after re-backfill", id)
				}
				matched++
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("iterate backfilled rows: %v", err)
			}
			if matched != 2 {
				t.Fatalf("matching rowids = %d; want 2", matched)
			}
		})
	}

	logging.CloseAll()
	paths, err := filepath.Glob(filepath.Join(workspace, ".nerd", "logs", "*_store.log"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("store log paths = %v, %v; want one log", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read store log: %v", err)
	}
	if !strings.Contains(string(data), "Backfill complete: migrated=2, skipped=0") {
		t.Fatalf("store log does not confirm the backfill ran: %s", data)
	}
	if strings.Contains(string(data), "UNIQUE constraint") || strings.Contains(string(data), "[WARN]") {
		t.Fatalf("re-backfill emitted a warning: %s", data)
	}
}

// A failed replacement batch must restore the rows deleted in its transaction
// and must not report uncommitted replacements as migrated.
func TestBackfillVecIndex_FailedBatchPreservesExistingRows(t *testing.T) {
	s := newVecStore(t)
	storeProbes(t, s, 2)

	readIndex := func() map[int64][3]string {
		t.Helper()
		rows, err := s.db.Query(`SELECT rowid, hex(embedding), content, metadata FROM vec_index ORDER BY rowid`)
		if err != nil {
			t.Fatalf("read index snapshot: %v", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				t.Errorf("close index snapshot: %v", err)
			}
		}()
		snapshot := make(map[int64][3]string)
		for rows.Next() {
			var id int64
			var values [3]string
			if err := rows.Scan(&id, &values[0], &values[1], &values[2]); err != nil {
				t.Fatalf("scan index snapshot: %v", err)
			}
			snapshot[id] = values
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate index snapshot: %v", err)
		}
		return snapshot
	}
	before := readIndex()
	if len(before) != 2 {
		t.Fatalf("initial index rows = %d; want 2", len(before))
	}

	// The source vectors pass backfill's dimension check, but not the existing
	// sqlite-vec table's dimension constraint. The INSERT therefore fails only
	// after the batch's existing rowids have been deleted in the transaction.
	engine := &mockSimpleEngine{}
	dim := engine.Dimensions() + 1
	vectorJSON, err := json.Marshal(make([]float32, dim))
	if err != nil {
		t.Fatalf("encode mismatched source embedding: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE vectors SET embedding = ?, content = 'replacement must roll back', metadata = '{"replacement":true}'`, string(vectorJSON)); err != nil {
		t.Fatalf("update source vectors: %v", err)
	}

	workspace := t.TempDir()
	logging.ApplyConfig(logging.Config{DebugMode: true, Level: "debug", Format: "text"})
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ApplyConfig(logging.Config{})
		logging.ClearInjectedConfig()
	})
	if err := logging.Initialize(workspace); err != nil {
		t.Fatalf("initialize logging: %v", err)
	}

	done := make(chan struct{})
	s.backfillMu.Lock()
	s.backfillDone = done
	s.backfillMu.Unlock()
	defer func() {
		s.backfillMu.Lock()
		s.backfillDone = nil
		s.backfillMu.Unlock()
		close(done)
	}()
	s.backfillVecIndex(dim, embeddingSpace(engine), s.vectorExt.Load(), done)

	after := readIndex()
	if len(after) != len(before) {
		t.Errorf("index rows after failed batch = %d; want %d", len(after), len(before))
	}
	for id, want := range before {
		if got, ok := after[id]; !ok || got != want {
			t.Errorf("index row %d after failed batch = %v (present=%t); want %v", id, got, ok, want)
		}
	}

	logging.CloseAll()
	paths, err := filepath.Glob(filepath.Join(workspace, ".nerd", "logs", "*_store.log"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("store log paths = %v, %v; want one log", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read store log: %v", err)
	}
	logText := string(data)
	if !strings.Contains(logText, "Failed to execute batch statement for backfill:") {
		t.Fatalf("backfill did not reach the batch execution failure: %s", logText)
	}
	if !strings.Contains(logText, "Backfill complete: migrated=0, skipped=0") {
		t.Fatalf("failed batch must not count as migrated or skipped: %s", logText)
	}
}

// Exercise driver errors at the batch boundary without changing production
// hooks. SQL still runs against the fixture's real sqlite-vec database.
func TestBackfillVecIndex_BatchDriverErrors(t *testing.T) {
	for _, tc := range []struct {
		name          string
		deleteError   bool
		insertError   bool
		closeError    bool
		rollbackError bool
		wantInsert    int
		wantCommit    int
		wantRollback  int
		wantWarnings  []string
	}{
		{
			name:        "delete failure skips insertion and rolls back",
			deleteError: true, wantRollback: 1,
			wantWarnings: []string{"Failed to delete existing rows for backfill: injected delete failure"},
		},
		{
			name:       "statement close failure still commits replacement",
			closeError: true, wantInsert: 1, wantCommit: 1,
			wantWarnings: []string{"Failed to close batch statement for backfill: injected close failure"},
		},
		{
			name:        "rollback failure is reported after insertion failure",
			insertError: true, rollbackError: true, wantInsert: 1, wantRollback: 1,
			wantWarnings: []string{
				"Failed to execute batch statement for backfill: injected insert failure",
				"Failed to roll back backfill batch: injected rollback failure",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newVecStore(t)
			storeProbes(t, s, 1)
			realDB := s.db
			readIndex := func() [3]string {
				t.Helper()
				var count int
				if err := realDB.QueryRow("SELECT COUNT(*) FROM vec_index").Scan(&count); err != nil {
					t.Fatalf("count index rows: %v", err)
				}
				if count != 1 {
					t.Fatalf("index rows = %d; want 1", count)
				}
				var row [3]string
				if err := realDB.QueryRow("SELECT hex(embedding), content, metadata FROM vec_index").Scan(&row[0], &row[1], &row[2]); err != nil {
					t.Fatalf("read indexed vector: %v", err)
				}
				return row
			}
			before := readIndex()
			if _, err := realDB.Exec(`UPDATE vectors SET embedding = '[0.4,0.3,0.2,0.1]', content = 'replacement', metadata = '{"updated":true}'`); err != nil {
				t.Fatalf("update source vector: %v", err)
			}
			var replacement [3]string
			if err := realDB.QueryRow("SELECT hex(?), content, metadata FROM vectors", encodeFloat32Slice([]float32{0.4, 0.3, 0.2, 0.1})).Scan(&replacement[0], &replacement[1], &replacement[2]); err != nil {
				t.Fatalf("read expected replacement: %v", err)
			}
			if replacement == before {
				t.Fatal("fixture must distinguish replacement from the original index row")
			}

			workspace := t.TempDir()
			logging.ApplyConfig(logging.Config{DebugMode: true, Level: "debug", Format: "text"})
			t.Cleanup(func() {
				logging.CloseAll()
				logging.ApplyConfig(logging.Config{})
				logging.ClearInjectedConfig()
			})
			if err := logging.Initialize(workspace); err != nil {
				t.Fatalf("initialize logging: %v", err)
			}

			faults := &backfillBatchFaults{
				deleteError: tc.deleteError, insertError: tc.insertError,
				closeError: tc.closeError, rollbackError: tc.rollbackError,
			}
			db := sql.OpenDB(&backfillFaultConnector{db: realDB, faults: faults})
			// An idle adapter still owns the fixture's only connection. Release
			// it between operations so readIndex can query the original pool.
			db.SetMaxIdleConns(0)
			s.db = db
			defer func() {
				s.db = realDB
				if err := db.Close(); err != nil {
					t.Errorf("close fault-injection pool: %v", err)
				}
			}()
			done := make(chan struct{})
			s.backfillMu.Lock()
			s.backfillDone = done
			s.backfillMu.Unlock()
			defer func() {
				s.backfillMu.Lock()
				s.backfillDone = nil
				s.backfillMu.Unlock()
				close(done)
			}()
			engine := &mockSimpleEngine{}
			s.backfillVecIndex(engine.Dimensions(), embeddingSpace(engine), s.vectorExt.Load(), done)

			if faults.deletes != 1 || faults.inserts != tc.wantInsert || faults.closes != 1 || faults.commits != tc.wantCommit || faults.rollbacks != tc.wantRollback {
				t.Errorf("batch operations (delete, insert, close, commit, rollback) = (%d, %d, %d, %d, %d); want (1, %d, 1, %d, %d)", faults.deletes, faults.inserts, faults.closes, faults.commits, faults.rollbacks, tc.wantInsert, tc.wantCommit, tc.wantRollback)
			}
			want := before
			if tc.wantCommit == 1 {
				want = replacement
			}
			if got := readIndex(); got != want {
				t.Errorf("index after batch = %v; want %v", got, want)
			}

			logging.CloseAll()
			paths, err := filepath.Glob(filepath.Join(workspace, ".nerd", "logs", "*_store.log"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("store log paths = %v, %v; want one log", paths, err)
			}
			data, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatalf("read store log: %v", err)
			}
			logText := string(data)
			for _, warning := range tc.wantWarnings {
				if !strings.Contains(logText, warning) {
					t.Errorf("missing warning %q in store log: %s", warning, logText)
				}
			}
			if strings.Count(logText, "[WARN]") != len(tc.wantWarnings) {
				t.Errorf("unexpected warnings in store log: %s", logText)
			}
			if !strings.Contains(logText, fmt.Sprintf("Backfill complete: migrated=%d, skipped=0", tc.wantCommit)) {
				t.Errorf("backfill must count only committed replacements: %s", logText)
			}
		})
	}
}

// These adapters delegate database operations to real SQLite, injecting only
// errors that SQLite cannot reliably produce on demand (notably Stmt.Close).
// Close and rollback errors are injected after the real cleanup succeeds.
type backfillBatchFaults struct {
	deleteError, insertError, closeError, rollbackError bool
	deletes, inserts, closes, commits, rollbacks        int
}

type backfillFaultConnector struct {
	db     *sql.DB
	faults *backfillBatchFaults
}

func (c *backfillFaultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire backfill test connection: %w", err)
	}
	return &backfillFaultConn{conn: conn, faults: c.faults}, nil
}

func (c *backfillFaultConnector) Driver() driver.Driver { return c.db.Driver() }

type backfillFaultConn struct {
	conn   *sql.Conn
	tx     *sql.Tx
	faults *backfillBatchFaults
}

func (c *backfillFaultConn) Close() error { return c.conn.Close() }

func (c *backfillFaultConn) Begin() (driver.Tx, error) {
	tx, err := c.conn.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("begin backfill test transaction: %w", err)
	}
	c.tx = tx
	return &backfillFaultTx{conn: c, tx: tx}, nil
}

func (c *backfillFaultConn) Prepare(query string) (driver.Stmt, error) {
	var stmt *sql.Stmt
	var err error
	if c.tx != nil {
		stmt, err = c.tx.PrepareContext(context.Background(), query)
	} else {
		stmt, err = c.conn.PrepareContext(context.Background(), query)
	}
	if err != nil {
		return nil, fmt.Errorf("prepare backfill test statement: %w", err)
	}
	return &backfillFaultStmt{stmt: stmt, faults: c.faults}, nil
}

func (c *backfillFaultConn) ExecContext(ctx context.Context, query string, values []driver.NamedValue) (driver.Result, error) {
	if strings.HasPrefix(query, "DELETE FROM vec_index ") {
		c.faults.deletes++
		if c.faults.deleteError {
			return nil, errors.New("injected delete failure")
		}
	}
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value.Value
	}
	if c.tx != nil {
		return c.tx.ExecContext(ctx, query, args...)
	}
	return c.conn.ExecContext(ctx, query, args...)
}

func (c *backfillFaultConn) QueryContext(ctx context.Context, query string, values []driver.NamedValue) (driver.Rows, error) {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value.Value
	}
	rows, err := c.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query backfill test rows: %w", err)
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.Join(err, rows.Close())
	}
	return &backfillFaultRows{rows: rows, columns: columns}, nil
}

type backfillFaultTx struct {
	conn *backfillFaultConn
	tx   *sql.Tx
}

func (tx *backfillFaultTx) Commit() error {
	tx.conn.faults.commits++
	defer func() { tx.conn.tx = nil }()
	return tx.tx.Commit()
}

func (tx *backfillFaultTx) Rollback() error {
	tx.conn.faults.rollbacks++
	defer func() { tx.conn.tx = nil }()
	if err := tx.tx.Rollback(); err != nil {
		return fmt.Errorf("roll back backfill test transaction: %w", err)
	}
	if tx.conn.faults.rollbackError {
		return errors.New("injected rollback failure")
	}
	return nil
}

type backfillFaultStmt struct {
	stmt   *sql.Stmt
	faults *backfillBatchFaults
}

func (s *backfillFaultStmt) Close() error {
	s.faults.closes++
	if err := s.stmt.Close(); err != nil {
		return fmt.Errorf("close backfill test statement: %w", err)
	}
	if s.faults.closeError {
		return errors.New("injected close failure")
	}
	return nil
}

func (s *backfillFaultStmt) NumInput() int { return -1 }

func (s *backfillFaultStmt) Exec(values []driver.Value) (driver.Result, error) {
	s.faults.inserts++
	if s.faults.insertError {
		return nil, errors.New("injected insert failure")
	}
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value
	}
	return s.stmt.Exec(args...)
}

func (s *backfillFaultStmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, errors.New("unexpected query of backfill INSERT statement")
}

type backfillFaultRows struct {
	rows    *sql.Rows
	columns []string
}

func (r *backfillFaultRows) Columns() []string { return r.columns }
func (r *backfillFaultRows) Close() error      { return r.rows.Close() }
func (r *backfillFaultRows) Next(dest []driver.Value) error {
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return fmt.Errorf("iterate backfill test rows: %w", err)
		}
		return io.EOF
	}
	values := make([]any, len(dest))
	pointers := make([]any, len(dest))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := r.rows.Scan(pointers...); err != nil {
		return fmt.Errorf("scan backfill test row: %w", err)
	}
	for i, value := range values {
		dest[i] = value
	}
	return nil
}

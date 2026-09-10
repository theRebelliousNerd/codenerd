package context

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
// Kind identifies the exact request; Revision identifies the observed artifact.
type WorkingRecord struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	Revision string `json:"revision"`
	Kind     string `json:"kind"`
	Step     int64  `json:"step"`
	Body     string `json:"body"`
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
		body TEXT NOT NULL, failed INTEGER NOT NULL);
		CREATE INDEX IF NOT EXISTS working_entity ON working_records(scope,entity,step DESC);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &WorkingStore{db: db, scope: scope}, nil
}

func (s *WorkingStore) Close() error { return s.db.Close() }

// Search discovers handles even after they leave the bounded candidate slice.
// Search is literal, paginated and scope-local; it grants no execution authority.
func (s *WorkingStore) Search(ctx context.Context, query string, offset, limit int) (string, error) {
	if query == "" || len(query) > 4096 || offset < 0 || offset > 1<<30 || limit < 1 || limit > 50 {
		return "", fmt.Errorf("invalid context search bounds")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,entity,revision,kind,step,failed FROM working_records
		WHERE scope=? AND (instr(entity,?)>0 OR instr(kind,?)>0 OR instr(body,?)>0)
		ORDER BY step DESC,id LIMIT ? OFFSET ?`, s.scope, query, query, query, limit, offset)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	result := []WorkingRecord{}
	for rows.Next() {
		var r WorkingRecord
		if err := rows.Scan(&r.ID, &r.Entity, &r.Revision, &r.Kind, &r.Step, &r.Failed); err != nil {
			return "", err
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	data, err := json.Marshal(struct {
		Records []WorkingRecord `json:"historical_records"`
		Next    int             `json:"next_offset"`
	}{result, offset + len(result)})
	return string(data), err
}

func (s *WorkingStore) Save(ctx context.Context, r WorkingRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO working_records VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
		r.ID, s.scope, r.Entity, r.Revision, r.Kind, r.Step, r.Body, r.Failed)
	return err
}

// Candidates returns metadata for the current dependency slice, with no body IO.
func (s *WorkingStore) Candidates(ctx context.Context, entities []string, limit int) ([]WorkingRecord, error) {
	if len(entities) == 0 {
		return nil, nil
	}
	args := []any{s.scope}
	marks := make([]string, len(entities))
	for i, e := range entities {
		marks[i] = "?"
		args = append(args, e)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT id,entity,revision,kind,step,failed FROM working_records
		WHERE scope=? AND entity IN (`+strings.Join(marks, ",")+`) ORDER BY step DESC,id LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []WorkingRecord
	for rows.Next() {
		var r WorkingRecord
		if err := rows.Scan(&r.ID, &r.Entity, &r.Revision, &r.Kind, &r.Step, &r.Failed); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Read retrieves a bounded page, retaining provenance on every page.
func (s *WorkingStore) Read(ctx context.Context, id string, offset, limit int) (WorkingRecord, int, error) {
	if offset < 0 || offset > 1<<30 || limit < 1 || limit > 16000 {
		return WorkingRecord{}, 0, fmt.Errorf("invalid context page bounds")
	}
	var r WorkingRecord
	var length int
	err := s.db.QueryRowContext(ctx, `SELECT id,entity,revision,kind,step,failed,substr(body,?,?),length(body)
		FROM working_records WHERE scope=? AND id=?`, offset+1, limit, s.scope, id).
		Scan(&r.ID, &r.Entity, &r.Revision, &r.Kind, &r.Step, &r.Failed, &r.Body, &length)
	return r, length, err
}

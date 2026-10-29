package retrieval

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"codenerd/internal/sqlpragmas"
)

// FileVectorStore persists Tier 4's file vectors between processes.
//
// A vector is addressed by what it represents, not by where it came from: the
// digest of the exact head bytes that were embedded, and the embedding space
// (the engine's Name, which carries its model) that embedded them. Two files
// with the same head share one row; an edited file has a new digest and is
// embedded again; a model change opens a new space and never compares across
// it. Without this store every process embedded the whole corpus again, so a
// fresh session's turns ranked a fraction of it while the warm-up caught up.
type FileVectorStore struct {
	db *sql.DB
}

// FileVectorStorePath is where a workspace keeps its file vectors.
func FileVectorStorePath(workspace string) string {
	return filepath.Join(workspace, ".nerd", "file_embeddings.db")
}

// OpenFileVectorStore opens (creating when absent) the vector store at path.
func OpenFileVectorStore(ctx context.Context, path string) (*FileVectorStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("file vector store: %w", err)
	}
	db, err := sqlpragmas.OpenWithPragmas("sqlite3", path, sqlpragmas.ProfileHot)
	if err != nil {
		return nil, fmt.Errorf("file vector store: open %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS file_vectors (
		digest     TEXT NOT NULL,
		space      TEXT NOT NULL,
		dims       INTEGER NOT NULL,
		vec        BLOB NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (digest, space)
	)`); err != nil {
		return nil, errors.Join(fmt.Errorf("file vector store: schema: %w", err), db.Close())
	}
	return &FileVectorStore{db: db}, nil
}

// Close releases the store.
func (st *FileVectorStore) Close() error {
	if st == nil || st.db == nil {
		return nil
	}
	return st.db.Close()
}

// fileVectorLoadBatch bounds one IN (...) list: SQLite caps bound parameters,
// and a whole repository's digests in one statement would exceed it.
const fileVectorLoadBatch = 500

// Load returns the stored vectors for digests in space, keyed by digest. A
// digest with no row is simply absent.
func (st *FileVectorStore) Load(ctx context.Context, space string, digests []string) (map[string][]float32, error) {
	out := make(map[string][]float32, len(digests))
	if st == nil || len(digests) == 0 {
		return out, nil
	}
	if space == "" {
		return out, errors.New("file vector store: empty embedding space")
	}
	for start := 0; start < len(digests); start += fileVectorLoadBatch {
		batch := digests[start:min(start+fileVectorLoadBatch, len(digests))]
		args := make([]any, 0, len(batch)+1)
		args = append(args, space)
		for _, d := range batch {
			args = append(args, d)
		}
		query := `SELECT digest, dims, vec FROM file_vectors WHERE space = ? AND digest IN (?` +
			strings.Repeat(",?", len(batch)-1) + `)`
		rows, err := st.db.QueryContext(ctx, query, args...)
		if err != nil {
			return out, fmt.Errorf("file vector store: load: %w", err)
		}
		for rows.Next() {
			var digest string
			var dims int
			var blob []byte
			if err := rows.Scan(&digest, &dims, &blob); err != nil {
				return out, errors.Join(fmt.Errorf("file vector store: scan: %w", err), rows.Close())
			}
			vec, err := decodeVector(blob, dims)
			if err != nil {
				continue // a damaged row is a miss: the file is embedded again and the row replaced
			}
			out[digest] = vec
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return out, fmt.Errorf("file vector store: load: %w", err)
		}
	}
	return out, nil
}

// Save stores vectors (keyed by digest) in space, replacing any row with the
// same key, in one transaction.
func (st *FileVectorStore) Save(ctx context.Context, space string, vectors map[string][]float32) (err error) {
	if st == nil || len(vectors) == 0 {
		return nil
	}
	if space == "" {
		return errors.New("file vector store: empty embedding space")
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("file vector store: save: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO file_vectors (digest, space, dims, vec, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`)
	if err != nil {
		return fmt.Errorf("file vector store: save: %w", err)
	}
	defer stmt.Close()
	for digest, vec := range vectors {
		if _, err = stmt.ExecContext(ctx, digest, space, len(vec), encodeVector(vec)); err != nil {
			return fmt.Errorf("file vector store: save %s: %w", digest, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("file vector store: save: %w", err)
	}
	return nil
}

// Retain deletes every row of space whose digest is not in keep, so the store
// holds the workspace as it is now rather than every revision ever embedded.
// It returns how many rows it removed. Run it only after a pass that read the
// whole corpus: keep must be the complete set of current digests.
func (st *FileVectorStore) Retain(ctx context.Context, space string, keep map[string]bool) (removed int, err error) {
	if st == nil {
		return 0, nil
	}
	if space == "" {
		return 0, errors.New("file vector store: empty embedding space")
	}
	rows, err := st.db.QueryContext(ctx, `SELECT digest FROM file_vectors WHERE space = ?`, space)
	if err != nil {
		return 0, fmt.Errorf("file vector store: retain: %w", err)
	}
	var stale []string
	for rows.Next() {
		var digest string
		if err := rows.Scan(&digest); err != nil {
			return 0, errors.Join(fmt.Errorf("file vector store: retain: %w", err), rows.Close())
		}
		if !keep[digest] {
			stale = append(stale, digest)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return 0, fmt.Errorf("file vector store: retain: %w", err)
	}
	for _, digest := range stale {
		if _, err := st.db.ExecContext(ctx, `DELETE FROM file_vectors WHERE space = ? AND digest = ?`, space, digest); err != nil {
			return removed, fmt.Errorf("file vector store: retain: %w", err)
		}
		removed++
	}
	return removed, nil
}

func encodeVector(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[4*i:], math.Float32bits(f))
	}
	return buf
}

func decodeVector(blob []byte, dims int) ([]float32, error) {
	if dims <= 0 || len(blob) != 4*dims {
		return nil, fmt.Errorf("vector blob of %d bytes does not hold %d dimensions", len(blob), dims)
	}
	vec := make([]float32, dims)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[4*i:]))
	}
	return vec, nil
}

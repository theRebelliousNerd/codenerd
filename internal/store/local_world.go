package store

import (
	"codenerd/internal/logging"
	"encoding/json"
)

// =============================================================================
// WORLD MODEL CACHE (Fast + Deep AST facts)
// =============================================================================

// WorldFileMeta stores per-file metadata for world cache invalidation.
type WorldFileMeta struct {
	Path        string
	Lang        string
	Size        int64
	ModTime     int64
	Hash        string
	Fingerprint string
}

// WorldFactInput is a lightweight fact carrier for world cache I/O.
// Predicate + Args mirror core/types.Fact without importing core.
type WorldFactInput struct {
	Predicate string
	Args      []interface{}
}

// UpsertWorldFile stores or updates world_files metadata.
func (s *LocalStore) UpsertWorldFile(meta WorldFileMeta) error {
	timer := logging.StartTimer(logging.CategoryStore, "UpsertWorldFile")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO world_files (path, lang, size, modtime, hash, fingerprint, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		   lang = excluded.lang,
		   size = excluded.size,
		   modtime = excluded.modtime,
		   hash = excluded.hash,
		   fingerprint = excluded.fingerprint,
		   updated_at = CURRENT_TIMESTAMP`,
		meta.Path, meta.Lang, meta.Size, meta.ModTime, meta.Hash, meta.Fingerprint,
	)
	return err
}

// DeleteWorldFile removes world_files and all cached facts for a file.
func (s *LocalStore) DeleteWorldFile(path string) error {
	timer := logging.StartTimer(logging.CategoryStore, "DeleteWorldFile")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM world_facts WHERE path = ?", path); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM world_files WHERE path = ?", path); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceWorldFactsForFile replaces cached facts for a file at a given depth.
func (s *LocalStore) ReplaceWorldFactsForFile(path, depth, fingerprint string, facts []WorldFactInput) error {
	timer := logging.StartTimer(logging.CategoryStore, "ReplaceWorldFactsForFile")
	defer timer.Stop()

	if depth == "" {
		depth = "fast"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM world_facts WHERE path = ? AND depth = ?", path, depth); err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO world_facts (path, depth, fingerprint, predicate, args, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range facts {
		argsJSON, _ := json.Marshal(f.Args)
		if _, err := stmt.Exec(path, depth, fingerprint, f.Predicate, string(argsJSON)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadWorldFactsForFile loads cached facts for a file at a given depth.
// Returns the facts and the stored fingerprint (empty if none).
func (s *LocalStore) LoadWorldFactsForFile(path, depth string) ([]WorldFactInput, string, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadWorldFactsForFile")
	defer timer.Stop()

	if depth == "" {
		depth = "fast"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT predicate, args, fingerprint FROM world_facts WHERE path = ? AND depth = ?",
		path, depth,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []WorldFactInput
	var fp string
	for rows.Next() {
		var pred, argsJSON, fingerprint string
		if err := rows.Scan(&pred, &argsJSON, &fingerprint); err != nil {
			continue
		}
		fp = fingerprint
		var args []interface{}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		out = append(out, WorldFactInput{Predicate: pred, Args: args})
	}
	return out, fp, nil
}

// LoadAllWorldFacts loads all cached facts for a given depth.
func (s *LocalStore) LoadAllWorldFacts(depth string) ([]WorldFactInput, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadAllWorldFacts")
	defer timer.Stop()

	if depth == "" {
		depth = "fast"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT predicate, args FROM world_facts WHERE depth = ? ORDER BY path",
		depth,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]WorldFactInput, 0)
	for rows.Next() {
		var pred, argsJSON string
		if err := rows.Scan(&pred, &argsJSON); err != nil {
			continue
		}
		var args []interface{}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		out = append(out, WorldFactInput{Predicate: pred, Args: args})
	}
	return out, nil
}

package store

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"
)

// DeleteKnowledgeAtomsByConcept deletes every knowledge atom whose concept is
// exactly one of concepts, together with the knowledge-atom vectors
// StoreKnowledgeAtomWithEmbedding wrote for them (their metadata carries the
// concept). Both deletions run in one transaction. It returns the number of
// atoms deleted.
//
// knowledge_atoms has no uniqueness constraint, so a writer that regenerates a
// derived set (strategic knowledge, per-document extraction) has to remove
// what it wrote before; appending again stacked duplicates on every run and
// kept atoms from an earlier vision beside the current one.
func (s *LocalStore) DeleteKnowledgeAtomsByConcept(concepts ...string) (int64, error) {
	if len(concepts) == 0 {
		return 0, nil
	}
	return s.deleteKnowledgeAtoms(func(exec func(query string, args ...any) (int64, error)) (int64, error) {
		var deleted int64
		for _, concept := range concepts {
			n, err := exec(`DELETE FROM knowledge_atoms WHERE concept = ?`, concept)
			if err != nil {
				return deleted, err
			}
			deleted += n
			if _, err := exec(`DELETE FROM vectors
				WHERE json_extract(metadata, '$.content_type') = 'knowledge_atom'
				  AND json_extract(metadata, '$.concept') = ?`, concept); err != nil {
				return deleted, err
			}
		}
		return deleted, nil
	})
}

// KnowledgeGeneration is a KB's knowledge rows as one measurement saw them:
// the highest knowledge_atoms id and the highest vectors id at that moment.
// Both tables use AUTOINCREMENT, so a row written after the measurement has
// a higher id than any the measurement saw.
type KnowledgeGeneration struct {
	Atoms, Vectors int64
}

// KnowledgeGeneration reads the KB's current generation.
func (s *LocalStore) KnowledgeGeneration() (KnowledgeGeneration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var g KnowledgeGeneration
	if tableExists(s.db, "knowledge_atoms") {
		if err := s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM knowledge_atoms`).Scan(&g.Atoms); err != nil {
			return g, fmt.Errorf("read knowledge generation: %w", err)
		}
	}
	if tableExists(s.db, "vectors") {
		if err := s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM vectors`).Scan(&g.Vectors); err != nil {
			return g, fmt.Errorf("read knowledge generation: %w", err)
		}
	}
	return g, nil
}

// DeleteKnowledgeAtomsByConceptThrough deletes the atoms of concepts that
// existed at generation g, and their knowledge-atom vectors, in one
// transaction: a row written since -- a refresh's replacement under the same
// concept -- is not the row the deletion was decided over, and survives. It
// returns the number of atoms deleted.
func (s *LocalStore) DeleteKnowledgeAtomsByConceptThrough(g KnowledgeGeneration, concepts ...string) (int64, error) {
	if len(concepts) == 0 {
		return 0, nil
	}
	return s.deleteKnowledgeAtoms(func(exec func(query string, args ...any) (int64, error)) (int64, error) {
		var deleted int64
		for _, concept := range concepts {
			n, err := exec(`DELETE FROM knowledge_atoms WHERE concept = ? AND id <= ?`, concept, g.Atoms)
			if err != nil {
				return deleted, err
			}
			deleted += n
			if _, err := exec(`DELETE FROM vectors
				WHERE json_extract(metadata, '$.content_type') = 'knowledge_atom'
				  AND json_extract(metadata, '$.concept') = ?
				  AND id <= ?`, concept, g.Vectors); err != nil {
				return deleted, err
			}
		}
		return deleted, nil
	})
}

// DeleteKnowledgeAtomsByPrefix deletes every knowledge atom whose concept
// starts with prefix, and their knowledge-atom vectors, in one transaction.
// The prefix is compared literally (no LIKE wildcards). An empty prefix is
// refused: it would empty the table.
func (s *LocalStore) DeleteKnowledgeAtomsByPrefix(prefix string) (int64, error) {
	if prefix == "" {
		return 0, fmt.Errorf("refusing to delete knowledge atoms with an empty concept prefix")
	}
	return s.deleteKnowledgeAtoms(func(exec func(query string, args ...any) (int64, error)) (int64, error) {
		deleted, err := exec(`DELETE FROM knowledge_atoms WHERE substr(concept, 1, length(?)) = ?`, prefix, prefix)
		if err != nil {
			return 0, err
		}
		if _, err := exec(`DELETE FROM vectors
			WHERE json_extract(metadata, '$.content_type') = 'knowledge_atom'
			  AND substr(json_extract(metadata, '$.concept'), 1, length(?)) = ?`, prefix, prefix); err != nil {
			return deleted, err
		}
		return deleted, nil
	})
}

// deleteKnowledgeAtoms runs body in one transaction, then drops vec_index rows
// whose vectors row is gone so ANN search does not return deleted atoms.
func (s *LocalStore) deleteKnowledgeAtoms(body func(exec func(query string, args ...any) (int64, error)) (int64, error)) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !tableExists(s.db, "knowledge_atoms") {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin knowledge atom deletion: %w", err)
	}
	exec := func(query string, args ...any) (int64, error) {
		result, err := tx.Exec(query, args...)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}
	deleted, err := body(exec)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("delete knowledge atoms: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit knowledge atom deletion: %w", err)
	}
	if s.vectorExt.Load() && tableExists(s.db, "vec_index") {
		if _, err := s.db.Exec("DELETE FROM vec_index WHERE rowid NOT IN (SELECT id FROM vectors)"); err != nil {
			logging.Get(logging.CategoryStore).Warn("vec_index ghost purge after knowledge deletion failed: %v (ANN drift)", err)
		}
	}
	return deleted, nil
}

// RetireKnowledgeAtomsFromSource deletes every knowledge atom stored with
// source whose content hash (ComputeContentHash) is not in keep, and their
// knowledge-atom vectors. An import that owns a source calls it with what it
// imports now, so a row from a file that is gone, or from a split that no
// longer exists, does not outlive the import that wrote it. An empty source
// is refused: it would name rows no import owns.
func (s *LocalStore) RetireKnowledgeAtomsFromSource(source string, keep map[string]bool) (int64, error) {
	if strings.TrimSpace(source) == "" {
		return 0, fmt.Errorf("refusing to retire knowledge atoms with an empty source")
	}
	type staleAtom struct {
		id               int64
		concept, content string
	}
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil || !tableExists(db, "knowledge_atoms") {
		return 0, nil
	}
	rows, err := db.Query(`SELECT id, concept, content, COALESCE(content_hash, '') FROM knowledge_atoms WHERE source = ?`, source)
	if err != nil {
		return 0, fmt.Errorf("list knowledge atoms from %s: %w", source, err)
	}
	var stale []staleAtom
	for rows.Next() {
		var a staleAtom
		var hash string
		if err := rows.Scan(&a.id, &a.concept, &a.content, &hash); err != nil {
			rows.Close()
			return 0, fmt.Errorf("list knowledge atoms from %s: %w", source, err)
		}
		if hash == "" {
			hash = ComputeContentHash(a.concept, a.content)
		}
		if !keep[hash] {
			stale = append(stale, a)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, fmt.Errorf("list knowledge atoms from %s: %w", source, err)
	}
	if len(stale) == 0 {
		return 0, nil
	}
	return s.deleteKnowledgeAtoms(func(exec func(query string, args ...any) (int64, error)) (int64, error) {
		var deleted int64
		for _, a := range stale {
			n, err := exec(`DELETE FROM knowledge_atoms WHERE id = ?`, a.id)
			if err != nil {
				return deleted, err
			}
			deleted += n
			if _, err := exec(`DELETE FROM vectors
				WHERE json_extract(metadata, '$.content_type') = 'knowledge_atom'
				  AND json_extract(metadata, '$.concept') = ? AND content = ?`, a.concept, a.content); err != nil {
				return deleted, err
			}
		}
		return deleted, nil
	})
}

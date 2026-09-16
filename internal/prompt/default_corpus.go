package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/core/defaults"
)

// MaterializeDefaultPromptCorpus writes the embedded default prompt corpus DB to dstPath
// if (and only if) dstPath does not already exist.
//
// Returns (true, nil) if the file was written, (false, nil) if no write occurred.
func MaterializeDefaultPromptCorpus(dstPath string) (bool, error) {
	if strings.TrimSpace(dstPath) == "" {
		return false, fmt.Errorf("dstPath is required")
	}

	// Never clobber an existing corpus DB.
	if _, err := os.Stat(dstPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat dstPath: %w", err)
	}

	if !defaults.PromptCorpusAvailable() {
		return false, nil
	}

	data, err := defaults.PromptCorpusDB.ReadFile("prompt_corpus.db")
	if err != nil {
		return false, fmt.Errorf("read embedded prompt corpus: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return false, fmt.Errorf("mkdir prompts dir: %w", err)
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return false, fmt.Errorf("write corpus: %w", err)
	}

	return true, nil
}

// HydrateAtomContextTags ensures the atom_context_tags table contains tag rows for the
// provided atoms that already exist in the DB.
//
// This is useful when the corpus DB was produced by older tooling (or was seeded)
// and is missing normalized tag rows.
func HydrateAtomContextTags(ctx context.Context, db *sql.DB, atoms []*PromptAtom) error {
	if db == nil || len(atoms) == 0 {
		return nil
	}

	// Snapshot which atoms exist in the DB to avoid inserting orphan tag rows.
	ids := make(map[string]struct{})
	rows, err := db.QueryContext(ctx, "SELECT atom_id FROM prompt_atoms")
	if err != nil {
		return fmt.Errorf("query prompt_atoms ids: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids[id] = struct{}{}
	}
	_ = rows.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	type tagRow struct {
		atomID string
		dim    string
		tag    string
	}
	var allTags []tagRow

	insertTag := func(atomID, dim, tag string) {
		if strings.TrimSpace(tag) == "" {
			return
		}
		allTags = append(allTags, tagRow{atomID: atomID, dim: dim, tag: tag})
	}

	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		if _, ok := ids[atom.ID]; !ok {
			continue
		}

		for _, tag := range atom.ContextTags() {
			insertTag(atom.ID, tag.Dimension, tag.Tag)
		}
	}

	if len(allTags) > 0 {
		const chunkSize = 300 // Safe chunk size for SQLite parameter limits (300 * 3 = 900 params)
		for i := 0; i < len(allTags); i += chunkSize {
			end := min(i+chunkSize, len(allTags))
			chunk := allTags[i:end]

			query := "INSERT OR IGNORE INTO atom_context_tags (atom_id, dimension, tag) VALUES "
			args := make([]any, 0, len(chunk)*3)
			placeholders := make([]string, len(chunk))

			for j, row := range chunk {
				placeholders[j] = "(?, ?, ?)"
				args = append(args, row.atomID, row.dim, row.tag)
			}
			query += strings.Join(placeholders, ", ")

			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return fmt.Errorf("batch insert tags: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

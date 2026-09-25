package prompt

import (
	"context"
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Context-tag persistence helpers shared by the atom loader. Embedding
// generation for prompt atoms lives in two places only: cmd/tools/prompt_builder
// bakes the shipped corpus.db, and internal/store re-embeds (and stamps the
// model on) every prompt_atoms row when the engine changes.

// insertContextTagsBatch inserts context tags for multiple atoms in batches.
func insertContextTagsBatch(ctx context.Context, tx *sql.Tx, atoms []*PromptAtom) error {
	type tagEntry struct {
		atomID    string
		dimension string
		tag       string
	}

	var allTags []tagEntry

	for _, atom := range atoms {
		for _, tag := range atom.ContextTags() {
			allTags = append(allTags, tagEntry{atom.ID, tag.Dimension, tag.Tag})
		}
	}

	if len(allTags) == 0 {
		return nil
	}

	// SQLite limits parameters, so we chunk tags too (999 limit / 3 fields = 333 max tags per batch)
	tagChunkSize := 300
	for i := 0; i < len(allTags); i += tagChunkSize {
		end := min(i+tagChunkSize, len(allTags))

		chunk := allTags[i:end]
		placeholders := make([]string, 0, len(chunk))
		args := make([]any, 0, len(chunk)*3)

		for _, t := range chunk {
			placeholders = append(placeholders, "(?, ?, ?)")
			args = append(args, t.atomID, t.dimension, t.tag)
		}

		query := "INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES " + strings.Join(placeholders, ", ")
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}

	return nil
}

// isValidIdentifier ensures column names only contain safe characters
// to prevent SQL injection in DDL statements where parameterized queries cannot be used.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	return !identifierRegexp.MatchString(s)

}

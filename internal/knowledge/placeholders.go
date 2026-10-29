package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Legacy placeholders. Init used to seed every agent KB that research left
// empty with rows templated from the agent's own name ("You are goexpert, a
// specialist agent in the codeNERD ecosystem.", "goexpert exists to carry out
// goexpert tasks in this workspace.", a generic methodology line, one
// "domain knowledge (i of n)" line per topic). They taught nothing, and worse:
// they named every topic, so a later run's coverage check read the topics as
// researched and never researched them. That seed is gone; these rows are
// recognised so they can be pruned and so the fill gate calls a KB holding
// only them empty.

// IsLegacyPlaceholder reports whether a knowledge row is one of the removed
// seed's templates.
func IsLegacyPlaceholder(concept, content string) bool {
	switch concept {
	case "agent_identity":
		return strings.HasPrefix(content, "You are ")
	case "agent_mission":
		return strings.Contains(content, " exists to carry out ")
	case "methodology/overview":
		return strings.Contains(content, " methodology: decompose the task")
	}
	return strings.HasPrefix(concept, "domain/") && strings.Count(concept, "/") == 1 &&
		strings.Contains(content, " domain knowledge (") &&
		strings.Contains(content, "check the repository's own docs and code for project-specific rules")
}

// PlaceholderSQL is IsLegacyPlaceholder as a SQL condition over a
// knowledge_atoms row, for the fill gate (internal/kbfill).
const PlaceholderSQL = `((concept = 'agent_identity' AND content LIKE 'You are %')
 OR (concept = 'agent_mission' AND content LIKE '% exists to carry out %')
 OR (concept = 'methodology/overview' AND content LIKE '% methodology: decompose the task%')
 OR (concept LIKE 'domain/%' AND concept NOT LIKE 'domain/%/%' AND content LIKE '% domain knowledge (%'
     AND content LIKE '%check the repository''s own docs and code for project-specific rules%'))`

// PrunePlaceholders deletes a KB's legacy placeholder rows and their
// knowledge vectors in one transaction, then drops the ANN index rows whose
// vector is gone. It returns how many knowledge rows it removed. Prompt sync
// runs it on every agent KB at boot, init on every KB it builds.
func PrunePlaceholders(ctx context.Context, db *sql.DB) (int64, error) {
	if db == nil {
		return 0, nil
	}
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'knowledge_atoms'`).Scan(&exists); err != nil {
		return 0, fmt.Errorf("look up knowledge_atoms: %w", err)
	}
	if exists == 0 {
		return 0, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin placeholder prune: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var hasVectors int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'vectors'`).Scan(&hasVectors); err != nil {
		return 0, fmt.Errorf("look up vectors: %w", err)
	}
	if hasVectors > 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM vectors
			WHERE json_extract(metadata, '$.content_type') = 'knowledge_atom'
			  AND EXISTS (SELECT 1 FROM knowledge_atoms
			              WHERE concept = json_extract(vectors.metadata, '$.concept')
			                AND content = vectors.content AND `+PlaceholderSQL+`)`); err != nil {
			return 0, fmt.Errorf("delete placeholder vectors: %w", err)
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM knowledge_atoms WHERE `+PlaceholderSQL)
	if err != nil {
		return 0, fmt.Errorf("delete placeholder atoms: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit placeholder prune: %w", err)
	}
	if n == 0 {
		return 0, nil
	}
	var hasIndex int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'vec_index'`).Scan(&hasIndex); err == nil && hasIndex > 0 && hasVectors > 0 {
		if _, err := db.ExecContext(ctx, `DELETE FROM vec_index WHERE rowid NOT IN (SELECT id FROM vectors)`); err != nil {
			return n, fmt.Errorf("pruned %d placeholder atom(s) but the ANN index purge failed: %w", n, err)
		}
	}
	return n, nil
}

package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ReconcileCounts reports the outcome of a boot corpus reconciliation.
// Embedded built-ins win on matching IDs. Canonical fields and exact tags are
// upserted without regenerating embeddings; an existing embedding is retained
// only when the embedding input is unchanged (description when non-empty after
// TrimSpace otherwise content), otherwise it is nulled.
type ReconcileCounts struct {
	Upserted int `json:"upserted"`
	Deleted  int `json:"deleted"`
	// Retained counts embeddings kept because embedding input matched.
	RetainedEmbeddings int `json:"retained_embeddings"`
	// Cleared counts embeddings nulled because embedding input changed.
	ClearedEmbeddings int `json:"cleared_embeddings"`
}

// ReconcileResult is an alias for ReconcileCounts for callers that expect that name.
type ReconcileResult = ReconcileCounts

// ReconcilePromptCorpus reconciles the SQLite corpus at db to match the provided
// embedded atoms. It runs in a single transaction:
//
//   - Upserts canonical prompt_atoms fields and exact atom_context_tags without
//     embeddings; embedding and embedding_task are retained only if the stored
//     embedding input is unchanged (description when non-empty after TrimSpace
//     otherwise content), otherwise they are set to NULL and any vec_prompt_atoms
//     row for that atom is removed.
//   - Removes obsolete rows whose source_file is any non-empty value (legacy
//     built-in ownership includes YAML relative paths) and whose atom_id is
//     absent from the embedded set. Rows with NULL or empty/whitespace
//     source_file (project-owned) are retained.
//   - Embedded IDs win on conflict.
//
// Returns reconciliation counts. The caller must have called EnsureSchema before
// invoking this function, and should register the DB only after this returns.
func ReconcilePromptCorpus(ctx context.Context, db *sql.DB, atoms []*PromptAtom) (ReconcileCounts, error) {
	var empty ReconcileCounts
	if db == nil {
		return empty, fmt.Errorf("db is required")
	}
	if atoms == nil {
		atoms = []*PromptAtom{}
	}
	// Validate no duplicate IDs in embedded set.
	seen := make(map[string]struct{}, len(atoms))
	for _, a := range atoms {
		if a == nil {
			continue
		}
		if _, ok := seen[a.ID]; ok {
			return empty, fmt.Errorf("duplicate embedded atom id %q", a.ID)
		}
		seen[a.ID] = struct{}{}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return empty, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if err := ensurePromptEmbeddingColumns(ctx, tx); err != nil {
		return empty, err
	}

	// Build set of embedded IDs for obsolete deletion.
	embeddedIDs := make([]string, 0, len(atoms))
	for _, a := range atoms {
		if a == nil {
			continue
		}
		embeddedIDs = append(embeddedIDs, a.ID)
	}
	embeddedSet := make(map[string]struct{}, len(embeddedIDs))
	for _, id := range embeddedIDs {
		embeddedSet[id] = struct{}{}
	}

	var counts ReconcileCounts
	var hasVectorTable bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type = 'table' AND name = 'vec_prompt_atoms'
		)
	`).Scan(&hasVectorTable); err != nil {
		return empty, fmt.Errorf("detect vec_prompt_atoms: %w", err)
	}
	// The statements below run once per atom (hundreds of atoms per boot), so
	// each is prepared once for the transaction: preparing per call made
	// SQLite's statement compiler the largest cost of reconciling the corpus.
	selectAtom, err := tx.PrepareContext(ctx, "SELECT content, description, embedding, embedding_task, embedding_model FROM prompt_atoms WHERE atom_id = ?")
	if err != nil {
		return empty, fmt.Errorf("prepare atom select: %w", err)
	}
	defer selectAtom.Close()
	upsertAtom, err := tx.PrepareContext(ctx, upsertAtomSQL)
	if err != nil {
		return empty, fmt.Errorf("prepare atom upsert: %w", err)
	}
	defer upsertAtom.Close()
	deleteTags, err := tx.PrepareContext(ctx, "DELETE FROM atom_context_tags WHERE atom_id = ?")
	if err != nil {
		return empty, fmt.Errorf("prepare tag delete: %w", err)
	}
	defer deleteTags.Close()
	insertTag, err := tx.PrepareContext(ctx, "INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES (?, ?, ?)")
	if err != nil {
		return empty, fmt.Errorf("prepare tag insert: %w", err)
	}
	defer insertTag.Close()
	var deleteVector *sql.Stmt
	if hasVectorTable {
		if deleteVector, err = tx.PrepareContext(ctx, "DELETE FROM vec_prompt_atoms WHERE atom_id = ?"); err != nil {
			return empty, fmt.Errorf("prepare vector delete: %w", err)
		}
		defer deleteVector.Close()
	}
	deleteVectorRow := func(atomID string) error {
		if !hasVectorTable {
			return nil
		}
		if _, err := deleteVector.ExecContext(ctx, atomID); err != nil {
			return fmt.Errorf("delete vector row %s: %w", atomID, err)
		}
		return nil
	}

	// Upsert each embedded atom.
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		// Load existing embedding input to decide retention.
		var existingContent sql.NullString
		var existingDescription sql.NullString
		var existingEmbedding []byte
		var existingTask sql.NullString
		var existingModel sql.NullString
		qerr := selectAtom.QueryRowContext(ctx, atom.ID).Scan(&existingContent, &existingDescription, &existingEmbedding, &existingTask, &existingModel)
		if qerr != nil && qerr != sql.ErrNoRows {
			return empty, fmt.Errorf("select atom %s: %w", atom.ID, qerr)
		}
		isNew := qerr == sql.ErrNoRows

		// Determine embedding input unchanged: description when non-empty otherwise content.
		newInput := atom.Description
		if strings.TrimSpace(newInput) == "" {
			newInput = atom.Content
		}
		var embeddingInputEqual bool
		hadEmbedding := len(existingEmbedding) > 0
		if !isNew {
			var existingInput string
			if existingDescription.Valid && strings.TrimSpace(existingDescription.String) != "" {
				existingInput = existingDescription.String
			} else if existingContent.Valid {
				existingInput = existingContent.String
			}
			embeddingInputEqual = existingInput == newInput
		}

		var embeddingBlob []byte
		var embeddingTask any
		var embeddingModel any
		if !isNew && embeddingInputEqual && hadEmbedding {
			embeddingBlob = existingEmbedding
			if existingTask.Valid {
				embeddingTask = existingTask.String
			} else {
				embeddingTask = nil
			}
			if existingModel.Valid {
				embeddingModel = existingModel.String
			} else {
				embeddingModel = nil
			}
			counts.RetainedEmbeddings++
		} else {
			if !isNew && hadEmbedding {
				counts.ClearedEmbeddings++
			}
			embeddingBlob = nil
			embeddingTask = nil
			embeddingModel = nil
			// A vector row is valid only while the corresponding prompt row has a
			// retained embedding for the same input. This also removes orphaned
			// vectors left by older partial invalidation paths.
			if err := deleteVectorRow(atom.ID); err != nil {
				return empty, err
			}
		}

		// Canonical fields. source_file is set to 'embedded' to mark ownership.
		_, err = upsertAtom.ExecContext(ctx, atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
			nullableString(atom.Description), nullableString(atom.ContentConcise), nullableString(atom.ContentMin),
			string(atom.Category), nullableString(atom.Subcategory),
			importanceColumn(atom.Importance), atom.IsMandatory, nullableString(atom.IsExclusive),
			nullableBytes(embeddingBlob), embeddingTask, embeddingModel,
		)
		if err != nil {
			return empty, fmt.Errorf("upsert atom %s: %w", atom.ID, err)
		}
		counts.Upserted++

		// Exact tags: delete then insert.
		if _, err := deleteTags.ExecContext(ctx, atom.ID); err != nil {
			return empty, fmt.Errorf("clear tags %s: %w", atom.ID, err)
		}
		type tagRow struct {
			dim string
			tag string
		}
		var rows []tagRow
		seenTags := make(map[tagRow]struct{})
		add := func(dim, tag string) {
			if strings.TrimSpace(tag) == "" {
				return
			}
			row := tagRow{dim: dim, tag: tag}
			if _, exists := seenTags[row]; exists {
				return
			}
			seenTags[row] = struct{}{}
			rows = append(rows, row)
		}
		for _, tag := range atom.ContextTags() {
			add(tag.Dimension, tag.Tag)
		}

		for _, r := range rows {
			if _, err := insertTag.ExecContext(ctx, atom.ID, r.dim, r.tag); err != nil {
				return empty, fmt.Errorf("insert tags %s: %w", atom.ID, err)
			}
		}
	}

	// Remove obsolete rows owned by any non-empty source_file (legacy built-ins including YAML paths).
	// Enumerate owned rows to avoid growing NOT IN placeholder list.
	ownedRows, err := tx.QueryContext(ctx, "SELECT atom_id, source_file FROM prompt_atoms WHERE source_file IS NOT NULL")
	if err != nil {
		return empty, fmt.Errorf("select owned atoms: %w", err)
	}
	var obsoleteIDs []string
	for ownedRows.Next() {
		var id string
		var sourceFile string
		if err := ownedRows.Scan(&id, &sourceFile); err != nil {
			ownedRows.Close()
			return empty, fmt.Errorf("scan owned atom: %w", err)
		}
		if strings.TrimSpace(sourceFile) == "" {
			continue
		}
		if _, ok := embeddedSet[id]; !ok {
			obsoleteIDs = append(obsoleteIDs, id)
		}
	}
	if err := ownedRows.Err(); err != nil {
		ownedRows.Close()
		return empty, fmt.Errorf("iterate owned atoms: %w", err)
	}
	ownedRows.Close()

	for _, oid := range obsoleteIDs {
		if err := deleteVectorRow(oid); err != nil {
			return empty, err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM atom_context_tags WHERE atom_id = ?", oid); err != nil {
			return empty, fmt.Errorf("delete obsolete tags %s: %w", oid, err)
		}
		res, err := tx.ExecContext(ctx, "DELETE FROM prompt_atoms WHERE atom_id = ?", oid)
		if err != nil {
			return empty, fmt.Errorf("delete obsolete %s: %w", oid, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			counts.Deleted++
		}
	}

	if err := tx.Commit(); err != nil {
		return empty, fmt.Errorf("commit: %w", err)
	}
	return counts, nil
}

// upsertAtomSQL writes one embedded atom's canonical fields. source_file is set
// to 'embedded' to mark ownership. Unstamped embeddings are never retained:
// only a vector stamped with both a non-empty task and model survives.
const upsertAtomSQL = `
	INSERT INTO prompt_atoms (
		atom_id, version, content, token_count, content_hash,
		description, content_concise, content_min,
		category, subcategory, importance, is_mandatory, is_exclusive,
		source_file, embedding, embedding_task, embedding_model
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'embedded', ?, ?, ?)
	ON CONFLICT(atom_id) DO UPDATE SET
		version = excluded.version,
		content = excluded.content,
		token_count = excluded.token_count,
		content_hash = excluded.content_hash,
		description = excluded.description,
		content_concise = excluded.content_concise,
		content_min = excluded.content_min,
		category = excluded.category,
		subcategory = excluded.subcategory,
		importance = excluded.importance,
		is_mandatory = excluded.is_mandatory,
		is_exclusive = excluded.is_exclusive,
		source_file = excluded.source_file,
		embedding = excluded.embedding,
		embedding_task = excluded.embedding_task,
		embedding_model = excluded.embedding_model
`

// ReconcileEmbeddedCorpus loads the embedded corpus and reconciles the DB to it.
// Convenience wrapper for boot paths that already have a DB handle.
func ReconcileEmbeddedCorpus(ctx context.Context, db *sql.DB) (ReconcileCounts, error) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return ReconcileCounts{}, fmt.Errorf("load embedded corpus: %w", err)
	}
	counts, err := ReconcilePromptCorpus(ctx, db, corpus.All())
	if err != nil {
		return ReconcileCounts{}, err
	}
	dropped, err := DropUnstampedPromptEmbeddings(ctx, db)
	if err != nil {
		return ReconcileCounts{}, err
	}
	counts.ClearedEmbeddings += dropped
	return counts, nil
}

func DropUnstampedPromptEmbeddings(ctx context.Context, db *sql.DB) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := ensurePromptEmbeddingColumns(ctx, tx); err != nil {
		return 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT atom_id, embedding, embedding_task, embedding_model FROM prompt_atoms WHERE embedding IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var unstamped []string
	for rows.Next() {
		var id string
		var blob []byte
		var task, model sql.NullString
		if err := rows.Scan(&id, &blob, &task, &model); err != nil {
			return 0, err
		}
		if len(blob) == 0 {
			continue
		}
		taskStamped := task.Valid && strings.TrimSpace(task.String) != ""
		modelStamped := model.Valid && strings.TrimSpace(model.String) != ""
		if !taskStamped || !modelStamped {
			unstamped = append(unstamped, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()
	for _, id := range unstamped {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE prompt_atoms SET embedding = NULL, embedding_task = NULL, embedding_model = NULL WHERE atom_id = ?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM vec_prompt_atoms WHERE atom_id = ?`, id); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such table") {
				continue
			}
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(unstamped), nil
}

func ensurePromptEmbeddingColumns(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(prompt_atoms)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !cols["embedding"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE prompt_atoms ADD COLUMN embedding BLOB`); err != nil {
			return err
		}
	}
	if !cols["embedding_task"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE prompt_atoms ADD COLUMN embedding_task TEXT`); err != nil {
			return err
		}
	}
	if !cols["embedding_model"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE prompt_atoms ADD COLUMN embedding_model TEXT`); err != nil {
			return err
		}
	}
	return nil
}

// nullableBytes returns nil for empty blobs so SQLite stores NULL.
func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

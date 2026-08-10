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
	deleteVectorRow := func(atomID string) error {
		if !hasVectorTable {
			return nil
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM vec_prompt_atoms WHERE atom_id = ?", atomID); err != nil {
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
		qerr := tx.QueryRowContext(ctx, "SELECT content, description, embedding, embedding_task FROM prompt_atoms WHERE atom_id = ?", atom.ID).Scan(&existingContent, &existingDescription, &existingEmbedding, &existingTask)
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
		if !isNew && embeddingInputEqual && hadEmbedding {
			embeddingBlob = existingEmbedding
			if existingTask.Valid && strings.TrimSpace(existingTask.String) != "" {
				embeddingTask = existingTask.String
			} else {
				embeddingTask = nil
			}
			counts.RetainedEmbeddings++
		} else {
			if !isNew && hadEmbedding && !embeddingInputEqual {
				counts.ClearedEmbeddings++
			}
			embeddingBlob = nil
			embeddingTask = nil
			// A vector row is valid only while the corresponding prompt row has a
			// retained embedding for the same input. This also removes orphaned
			// vectors left by older partial invalidation paths.
			if err := deleteVectorRow(atom.ID); err != nil {
				return empty, err
			}
		}

		// Canonical fields. source_file is set to 'embedded' to mark ownership.
		_, err = tx.ExecContext(ctx, `
				INSERT INTO prompt_atoms (
					atom_id, version, content, token_count, content_hash,
					description, content_concise, content_min,
					category, subcategory, priority, is_mandatory, is_exclusive,
					source_file, embedding, embedding_task
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'embedded', ?, ?)
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
					priority = excluded.priority,
					is_mandatory = excluded.is_mandatory,
					is_exclusive = excluded.is_exclusive,
					source_file = excluded.source_file,
					embedding = excluded.embedding,
					embedding_task = excluded.embedding_task
			`, atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
			nullableString(atom.Description), nullableString(atom.ContentConcise), nullableString(atom.ContentMin),
			string(atom.Category), nullableString(atom.Subcategory),
			atom.Priority, atom.IsMandatory, nullableString(atom.IsExclusive),
			nullableBytes(embeddingBlob), embeddingTask,
		)
		if err != nil {
			return empty, fmt.Errorf("upsert atom %s: %w", atom.ID, err)
		}
		counts.Upserted++

		// Exact tags: delete then insert.
		if _, err := tx.ExecContext(ctx, "DELETE FROM atom_context_tags WHERE atom_id = ?", atom.ID); err != nil {
			return empty, fmt.Errorf("clear tags %s: %w", atom.ID, err)
		}
		type tagRow struct {
			dim string
			tag string
		}
		var rows []tagRow
		seenTags := make(map[tagRow]struct{})
		add := func(dim string, vals []string) {
			for _, v := range vals {
				if strings.TrimSpace(v) == "" {
					continue
				}
				row := tagRow{dim: dim, tag: v}
				if _, exists := seenTags[row]; exists {
					continue
				}
				seenTags[row] = struct{}{}
				rows = append(rows, row)
			}
		}
		add("mode", atom.OperationalModes)
		add("phase", atom.CampaignPhases)
		add("layer", atom.BuildLayers)
		add("init_phase", atom.InitPhases)
		add("northstar_phase", atom.NorthstarPhases)
		add("ouroboros_stage", atom.OuroborosStages)
		add("intent", atom.IntentVerbs)
		add("shard", atom.ShardTypes)
		add("lang", atom.Languages)
		add("framework", atom.Frameworks)
		add("state", atom.WorldStates)
		add("depends_on", atom.DependsOn)
		add("conflicts_with", atom.ConflictsWith)

		if len(rows) > 0 {
			const chunkSize = 300
			for i := 0; i < len(rows); i += chunkSize {
				end := i + chunkSize
				if end > len(rows) {
					end = len(rows)
				}
				chunk := rows[i:end]
				placeholders := make([]string, len(chunk))
				args := make([]any, 0, len(chunk)*3)
				for j, r := range chunk {
					placeholders[j] = "(?, ?, ?)"
					args = append(args, atom.ID, r.dim, r.tag)
				}
				q := "INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES " + strings.Join(placeholders, ", ")
				if _, err := tx.ExecContext(ctx, q, args...); err != nil {
					return empty, fmt.Errorf("insert tags %s: %w", atom.ID, err)
				}
			}
		}
	}

	// Remove obsolete rows owned by any non-empty source_file (legacy built-ins including YAML paths).
	// Enumerate owned rows to avoid growing NOT IN placeholder list.
	ownedRows, err := tx.QueryContext(ctx, "SELECT atom_id FROM prompt_atoms WHERE source_file IS NOT NULL AND TRIM(source_file) != ''")
	if err != nil {
		return empty, fmt.Errorf("select owned atoms: %w", err)
	}
	var obsoleteIDs []string
	for ownedRows.Next() {
		var id string
		if err := ownedRows.Scan(&id); err != nil {
			ownedRows.Close()
			return empty, fmt.Errorf("scan owned atom: %w", err)
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

// ReconcileEmbeddedCorpus loads the embedded corpus and reconciles the DB to it.
// Convenience wrapper for boot paths that already have a DB handle.
func ReconcileEmbeddedCorpus(ctx context.Context, db *sql.DB) (ReconcileCounts, error) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return ReconcileCounts{}, fmt.Errorf("load embedded corpus: %w", err)
	}
	return ReconcilePromptCorpus(ctx, db, corpus.All())
}

// nullableBytes returns nil for empty blobs so SQLite stores NULL.
func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

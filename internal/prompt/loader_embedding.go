package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================================
// EMBEDDED TO SQLITE SYNC
// ============================================================================

// SyncEmbeddedToSQLite copies embedded atoms to SQLite with embedding generation.
// This enables vector search over baked-in atoms.
// It uses content hashing to avoid re-embedding unchanged atoms.
//
// The function is idempotent: safe to call multiple times. Only atoms with
// changed content (detected via hash) will have their embeddings regenerated.
//
// Per System 2 Architecture guidelines, embeddings are generated from the
// atom's description field, not its content. If description is empty,
// the first 500 characters of content are used as a fallback.
func SyncEmbeddedToSQLite(ctx context.Context, dbPath string, engine embedding.EmbeddingEngine) error {
	timer := logging.StartTimer(logging.CategoryStore, "SyncEmbeddedToSQLite")
	defer timer.Stop()

	if engine == nil {
		return fmt.Errorf("embedding engine is required for SyncEmbeddedToSQLite")
	}

	logging.Get(logging.CategoryStore).Info("Syncing embedded corpus to SQLite: %s", dbPath)

	// Load embedded corpus
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return fmt.Errorf("failed to load embedded corpus: %w", err)
	}

	atoms := corpus.All()
	if len(atoms) == 0 {
		logging.Get(logging.CategoryStore).Info("No embedded atoms to sync")
		return nil
	}

	logging.Get(logging.CategoryStore).Info("Loaded %d atoms from embedded corpus", len(atoms))

	// Ensure parent directory exists
	dbDir := filepath.Dir(dbPath)
	if dbDir != "" && dbDir != "." {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
		}
	}

	// Open/create SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}
	defer db.Close()
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileBulkBuild)

	// Ensure schema exists
	loader := NewAtomLoader(engine)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("failed to ensure schema: %w", err)
	}

	// Build a map of existing content hashes to avoid re-embedding unchanged atoms
	existingHashes, err := loadExistingHashes(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to load existing hashes: %w", err)
	}

	logging.Get(logging.CategoryStore).Debug("Found %d existing atoms in database", len(existingHashes))

	// Partition atoms into unchanged (skip) and changed (need embedding)
	var atomsToEmbed []*PromptAtom
	var atomsUnchanged []*PromptAtom

	for _, atom := range atoms {
		existingHash, exists := existingHashes[atom.ID]
		if exists && existingHash == atom.ContentHash {
			atomsUnchanged = append(atomsUnchanged, atom)
		} else {
			atomsToEmbed = append(atomsToEmbed, atom)
		}
	}

	logging.Get(logging.CategoryStore).Info("Sync plan: %d unchanged (skip), %d new/changed (embed)",
		len(atomsUnchanged), len(atomsToEmbed))

	if len(atomsToEmbed) == 0 {
		logging.Get(logging.CategoryStore).Info("All atoms up-to-date, nothing to sync")
		return nil
	}

	// Prepare texts for batch embedding
	// Per System 2 Architecture: embed DESCRIPTION, not content
	textsToEmbed := make([]string, len(atomsToEmbed))
	for i, atom := range atomsToEmbed {
		textsToEmbed[i] = getTextForEmbedding(atom)
	}

	// Generate embeddings in batch for efficiency
	logging.Get(logging.CategoryStore).Info("Generating embeddings for %d atoms using %s",
		len(atomsToEmbed), engine.Name())

	taskType := embedding.SelectTaskType(embedding.ContentTypePromptAtom, false)
	var embeddings [][]float32
	if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok && taskType != "" {
		embeddings, err = batchAware.EmbedBatchWithTask(ctx, textsToEmbed, taskType)
	} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		embeddings = make([][]float32, len(textsToEmbed))
		for i, text := range textsToEmbed {
			vec, embedErr := taskAware.EmbedWithTask(ctx, text, taskType)
			if embedErr != nil {
				return fmt.Errorf("failed to embed atom %d: %w", i, embedErr)
			}
			if len(vec) == 0 {
				return fmt.Errorf("empty embedding for atom %d", i)
			}
			embeddings[i] = vec
		}
	} else {
		embeddings, err = engine.EmbedBatch(ctx, textsToEmbed)
	}
	if err != nil {
		return fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	if len(embeddings) != len(atomsToEmbed) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(atomsToEmbed))
	}

	// Store atoms with embeddings in a transaction for atomicity
	if err := storeAtomsWithEmbeddings(ctx, db, atomsToEmbed, embeddings, taskType); err != nil {
		return fmt.Errorf("failed to store atoms: %w", err)
	}

	logging.Get(logging.CategoryStore).Info("Successfully synced %d atoms to %s", len(atomsToEmbed), dbPath)
	return nil
}

// loadExistingHashes retrieves atom_id -> content_hash mapping from the database.
func loadExistingHashes(ctx context.Context, db *sql.DB) (map[string]string, error) {
	hashes := make(map[string]string)

	rows, err := db.QueryContext(ctx, "SELECT atom_id, content_hash FROM prompt_atoms")
	if err != nil {
		// Table might not exist yet - that's fine
		return hashes, nil
	}
	defer rows.Close()

	for rows.Next() {
		var atomID, contentHash string
		if err := rows.Scan(&atomID, &contentHash); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		hashes[atomID] = contentHash
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return hashes, nil
}

// getTextForEmbedding returns the text to embed for an atom.
// Per System 2 Architecture: use description if available, otherwise first 500 chars of content.
func getTextForEmbedding(atom *PromptAtom) string {
	if atom.Description != "" {
		return atom.Description
	}

	// Fallback: use first 500 characters of content
	content := atom.Content
	if len(content) > 500 {
		content = content[:500]
	}
	return content
}

// storeAtomsWithEmbeddings stores atoms and their embeddings in a single transaction.
func storeAtomsWithEmbeddings(ctx context.Context, db *sql.DB, atoms []*PromptAtom, embeddings [][]float32, taskType string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	chunkSize := 50
	for i := 0; i < len(atoms); i += chunkSize {
		end := min(i+chunkSize, len(atoms))

		chunkAtoms := atoms[i:end]
		chunkEmbeddings := embeddings[i:end]

		if err := storeAtomsChunk(ctx, tx, chunkAtoms, chunkEmbeddings, taskType); err != nil {
			return fmt.Errorf("failed to store chunk of atoms: %w", err)
		}

		// Log progress
		logging.Get(logging.CategoryStore).Debug("Stored %d/%d atoms", end, len(atoms))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func storeAtomsChunk(ctx context.Context, tx *sql.Tx, atoms []*PromptAtom, embeddings [][]float32, taskType string) error {
	if len(atoms) == 0 {
		return nil
	}

	// 1. Bulk insert/update atoms
	placeholders := make([]string, 0, len(atoms))
	args := make([]any, 0, len(atoms)*16)

	for i, atom := range atoms {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		embeddingBlob := encodeFloat32Slice(embeddings[i])

		args = append(args,
			atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
			nullableString(atom.Description), nullableString(atom.ContentConcise), nullableString(atom.ContentMin),
			string(atom.Category), nullableString(atom.Subcategory),
			atom.Priority, atom.IsMandatory, nullableString(atom.IsExclusive),
			embeddingBlob, nullableString(taskType), "embedded",
		)
	}

	query := `
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash,
			description, content_concise, content_min,
			category, subcategory,
			priority, is_mandatory, is_exclusive,
			embedding, embedding_task, source_file
		) VALUES ` + strings.Join(placeholders, ", ") + `
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
			embedding = excluded.embedding,
			embedding_task = excluded.embedding_task,
			source_file = excluded.source_file`

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to execute atom batch insert: %w", err)
	}

	// 2. Bulk delete context tags
	deletePlaceholders := make([]string, 0, len(atoms))
	deleteArgs := make([]any, 0, len(atoms))
	for _, atom := range atoms {
		deletePlaceholders = append(deletePlaceholders, "?")
		deleteArgs = append(deleteArgs, atom.ID)
	}

	deleteQuery := fmt.Sprintf("DELETE FROM atom_context_tags WHERE atom_id IN (%s)", strings.Join(deletePlaceholders, ", "))
	if _, err := tx.ExecContext(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("failed to clear tags for atom chunk: %w", err)
	}

	// 3. Bulk insert context tags
	return insertContextTagsBatch(ctx, tx, atoms)
}

// insertContextTagsBatch inserts context tags for multiple atoms in batches.
func insertContextTagsBatch(ctx context.Context, tx *sql.Tx, atoms []*PromptAtom) error {
	type tagEntry struct {
		atomID    string
		dimension string
		tag       string
	}

	var allTags []tagEntry

	for _, atom := range atoms {
		addDim := func(dimension string, values []string) {
			for _, v := range values {
				allTags = append(allTags, tagEntry{atom.ID, dimension, v})
			}
		}
		addDim("mode", atom.OperationalModes)
		addDim("phase", atom.CampaignPhases)
		addDim("layer", atom.BuildLayers)
		addDim("init_phase", atom.InitPhases)
		addDim("northstar_phase", atom.NorthstarPhases)
		addDim("ouroboros_stage", atom.OuroborosStages)
		addDim("intent", atom.IntentVerbs)
		addDim("shard", atom.ShardTypes)
		addDim("lang", atom.Languages)
		addDim("framework", atom.Frameworks)
		addDim("state", atom.WorldStates)
		addDim("depends_on", atom.DependsOn)
		addDim("conflicts_with", atom.ConflictsWith)
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

// sanitizeIdentifier ensures column names only contain safe characters
// to prevent SQL injection in DDL statements where parameterized queries cannot be used.
func sanitizeIdentifier(s string) string {
	return identifierRegexp.ReplaceAllString(s, "")
}

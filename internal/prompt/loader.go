// Package prompt - Runtime YAML → SQLite ingestion for prompt atoms.
// This loader enables runtime loading of agent-specific and project-level prompt atoms
// from YAML files into SQLite databases for JIT prompt compilation.
package prompt

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"

	_ "github.com/mattn/go-sqlite3"
)

var identifierRegexp = regexp.MustCompile("[^a-zA-Z0-9_]+")

// AtomLoader handles runtime loading and persistence of prompt atoms.
type AtomLoader struct {
	embeddingEngine embedding.EmbeddingEngine
	embeddingDims   int
}

// NewAtomLoader creates a new atom loader with optional embedding support.
// If embeddingEngine is nil, atoms will be stored without embeddings.
func NewAtomLoader(embeddingEngine embedding.EmbeddingEngine) *AtomLoader {
	dims := 0
	if embeddingEngine != nil {
		dims = embeddingEngine.Dimensions()
	}
	return &AtomLoader{
		embeddingEngine: embeddingEngine,
		embeddingDims:   dims,
	}
}

// LoadFromYAML loads prompt atoms from a YAML file and stores them in the database.
// Returns the number of atoms loaded.
func (l *AtomLoader) LoadFromYAML(ctx context.Context, yamlPath string, db *sql.DB) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadFromYAML")
	defer timer.Stop()

	logging.Get(logging.CategoryStore).Info("Loading prompt atoms from YAML: %s", yamlPath)

	// Parse YAML file
	atoms, err := l.ParseYAML(yamlPath)
	if err != nil {
		return 0, fmt.Errorf("failed to parse YAML file %s: %w", yamlPath, err)
	}

	logging.Get(logging.CategoryStore).Info("Parsed %d atoms from %s", len(atoms), filepath.Base(yamlPath))

	// Store atoms in database
	stored := 0
	for _, atom := range atoms {
		if err := l.StoreAtom(ctx, db, atom); err != nil {
			return stored, fmt.Errorf("store atom %s from %s: %w", atom.ID, yamlPath, err)
		}
		stored++
	}

	logging.Get(logging.CategoryStore).Info("Successfully stored %d/%d atoms", stored, len(atoms))
	return stored, nil
}

// LoadFromDirectory recursively loads all YAML files from a directory.
func (l *AtomLoader) LoadFromDirectory(ctx context.Context, dirPath string, db *sql.DB) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadFromDirectory")
	defer timer.Stop()

	logging.Get(logging.CategoryStore).Info("Loading prompt atoms from directory: %s", dirPath)

	parsed, migrations, err := ParsePromptAtomDirectory(dirPath)
	if err != nil {
		return 0, fmt.Errorf("parse atom directory %s: %w", dirPath, err)
	}
	logAtomMigrations(migrations)

	totalStored := 0
	for _, record := range parsed {
		if err := l.StoreAtom(ctx, db, record.Atom); err != nil {
			return totalStored, fmt.Errorf("store atom %s from %s: %w", record.Atom.ID, record.SourcePath, err)
		}
		totalStored++
	}

	logging.Get(logging.CategoryStore).Info("Loaded total of %d atoms from directory", totalStored)
	return totalStored, nil
}

// EnsureSchema creates the prompt_atoms table and atom_context_tags table.
func (l *AtomLoader) EnsureSchema(ctx context.Context, db *sql.DB) error {
	// Step 1: Create tables WITHOUT indexes first (so we can run migrations)
	tableSchema := `
		CREATE TABLE IF NOT EXISTS prompt_atoms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			atom_id TEXT NOT NULL UNIQUE,
			version INTEGER DEFAULT 1,
			content TEXT NOT NULL,
			token_count INTEGER NOT NULL,
			content_hash TEXT NOT NULL,

			-- Polymorphism
			description TEXT,
			content_concise TEXT,
			content_min TEXT,

			-- Classification
			category TEXT NOT NULL,
			subcategory TEXT,

			-- Composition
			priority INTEGER DEFAULT 50,
			is_mandatory BOOLEAN DEFAULT FALSE,
			is_exclusive TEXT,
			depends_on TEXT,
			conflicts_with TEXT,

			-- Embeddings
			embedding BLOB,
			embedding_task TEXT DEFAULT 'RETRIEVAL_DOCUMENT',

			-- Metadata
			source_file TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS atom_context_tags (
			atom_id TEXT NOT NULL,
			dimension TEXT NOT NULL,
			tag TEXT NOT NULL,
			is_exclusion BOOLEAN DEFAULT FALSE,
			PRIMARY KEY (atom_id, dimension, tag),
			FOREIGN KEY(atom_id) REFERENCES prompt_atoms(atom_id) ON DELETE CASCADE
		);
	`

	if _, err := db.Exec(tableSchema); err != nil {
		return fmt.Errorf("failed to create prompt tables: %w", err)
	}

	// Step 2: Run schema migrations BEFORE creating indexes
	// This ensures columns exist before we try to index them
	cols := []string{"description", "content_concise", "content_min"}
	for _, col := range cols {
		// Check if column exists by querying pragma
		var exists bool
		rows, err := db.Query("PRAGMA table_info(prompt_atoms)")
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to query table info: %v", err)
			continue
		}
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dfltValue any
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
				continue
			}
			if name == col {
				exists = true
				break
			}
		}
		rows.Close()

		if !exists {
			// Column missing, add it safely
			// Validate the column name to prevent SQL injection in DDL statement
			if !isValidIdentifier(col) {
				logging.Get(logging.CategoryStore).Warn("Invalid column name %s, skipping", col)
				continue
			}
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE prompt_atoms ADD COLUMN \"%s\" TEXT", col)); err != nil {
				logging.Get(logging.CategoryStore).Warn("Failed to add column %s: %v", col, err)
			} else {
				logging.Get(logging.CategoryStore).Info("Added missing column %s to prompt_atoms", col)
			}
		}
	}

	// Step 3: Create indexes AFTER migrations (columns now exist)
	indexSchema := `
		CREATE INDEX IF NOT EXISTS idx_atoms_category ON prompt_atoms(category);
		CREATE INDEX IF NOT EXISTS idx_atoms_description ON prompt_atoms(description);
		CREATE INDEX IF NOT EXISTS idx_tags_lookup ON atom_context_tags(dimension, tag);
	`

	if _, err := db.Exec(indexSchema); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// ParseYAML parses a YAML file containing prompt atom definitions.
func (l *AtomLoader) ParseYAML(path string) ([]*PromptAtom, error) {
	parsed, migrations, err := ParsePromptAtomFile(path)
	if err != nil {
		return nil, err
	}
	logAtomMigrations(migrations)
	atoms := make([]*PromptAtom, 0, len(parsed))
	for _, record := range parsed {
		atoms = append(atoms, record.Atom)
	}
	return atoms, nil
}

func logAtomMigrations(migrations []AtomSchemaMigration) {
	for _, migration := range migrations {
		logging.Get(logging.CategoryStore).Warn(
			"Prompt atom %s used compatibility migration %s: %s",
			migration.AtomID,
			migration.Code,
			migration.Message,
		)
	}
}

// StoreAtom stores a prompt atom in the database with optional embedding.
func (l *AtomLoader) StoreAtom(ctx context.Context, db *sql.DB, atom *PromptAtom) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := l.storeAtomTx(ctx, tx, atom); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

// ReplaceAtoms stores the provided prompt atoms transactionally after pruning the
// existing prompt atom set in the target database.
// ReplaceAtoms stores the provided prompt atoms transactionally after pruning the
// existing prompt atom set in the target database.
func (l *AtomLoader) ReplaceAtoms(ctx context.Context, db *sql.DB, atoms []*PromptAtom) error {
	// 1. Process Embeddings BEFORE starting the transaction
	// This prevents holding open database transactions while waiting on network I/O
	var embeddings [][]float32
	var taskType string

	if l.embeddingEngine != nil && len(atoms) > 0 {
		textsToEmbed := make([]string, len(atoms))
		for i, atom := range atoms {
			text := atom.Description
			if text == "" {
				text = atom.Content
			}
			textsToEmbed[i] = text
		}

		taskType = embedding.SelectTaskType(embedding.ContentTypePromptAtom, false)
		var err error

		if batchAware, ok := l.embeddingEngine.(embedding.TaskTypeBatchAwareEngine); ok && taskType != "" {
			embeddings, err = batchAware.EmbedBatchWithTask(ctx, textsToEmbed, taskType)
		} else if taskAware, ok := l.embeddingEngine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
			embeddings = make([][]float32, len(textsToEmbed))
			for i, text := range textsToEmbed {
				vec, embedErr := taskAware.EmbedWithTask(ctx, text, taskType)
				if embedErr != nil {
					logging.Get(logging.CategoryStore).Warn("Failed to embed atom %d: %v", i, embedErr)
				} else {
					embeddings[i] = vec
				}
			}
		} else {
			embeddings, err = l.embeddingEngine.EmbedBatch(ctx, textsToEmbed)
		}

		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to generate batch embeddings: %v", err)
			embeddings = make([][]float32, len(atoms)) // Fallback to empty embeddings
		}
	} else {
		embeddings = make([][]float32, len(atoms))
	}

	// 2. Start Transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM atom_context_tags"); err != nil {
		return fmt.Errorf("clear tags failed: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM prompt_atoms"); err != nil {
		return fmt.Errorf("clear atoms failed: %w", err)
	}

	// 3. Batch Insert using SQLite Multi-row Insert (Chunking)
	if len(atoms) == 0 {
		return tx.Commit()
	}

	// SQLite maximum variables per query is 32766, but 999 is safer for older versions.
	// We have 15 columns per atom insert. So max chunk size is 999 / 15 = 66
	chunkSize := 60
	for i := 0; i < len(atoms); i += chunkSize {
		end := min(i+chunkSize, len(atoms))
		chunk := atoms[i:end]
		chunkEmbeds := embeddings[i:end]

		// Build the query
		query := "INSERT INTO prompt_atoms (atom_id, version, content, token_count, content_hash, description, content_concise, content_min, category, subcategory, priority, is_mandatory, is_exclusive, embedding, embedding_task) VALUES "
		var vals []any
		var placeholders []string

		for j, atom := range chunk {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

			var embeddingBlob []byte
			if len(chunkEmbeds[j]) > 0 {
				embeddingBlob = encodeFloat32Slice(chunkEmbeds[j])
			}

			var etask *string
			if len(embeddingBlob) > 0 && taskType != "" {
				etask = &taskType
			}

			vals = append(vals,
				atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
				nullableString(atom.Description), nullableString(atom.ContentConcise), nullableString(atom.ContentMin),
				string(atom.Category), nullableString(atom.Subcategory),
				atom.Priority, atom.IsMandatory, nullableString(atom.IsExclusive),
				embeddingBlob, etask,
			)
		}

		query += strings.Join(placeholders, ",")

		if _, err := tx.ExecContext(ctx, query, vals...); err != nil {
			return fmt.Errorf("batch insert chunk failed: %w", err)
		}
	}

	// 4. Batch Insert Context Tags
	// We will chunk tags insertion similarly. Tag table has 3 columns (atom_id, dimension, tag)
	// Max chunk size = 999 / 3 = 333
	tagChunkSize := 300
	var tagVals []any
	var tagPlaceholders []string

	flushTags := func() error {
		if len(tagVals) == 0 {
			return nil
		}
		query := "INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES " + strings.Join(tagPlaceholders, ",")
		if _, err := tx.ExecContext(ctx, query, tagVals...); err != nil {
			return err
		}
		tagVals = tagVals[:0]
		tagPlaceholders = tagPlaceholders[:0]
		return nil
	}

	for _, atom := range atoms {
		// Helper to queue tag
		queueTag := func(dim, tag string) error {
			tagVals = append(tagVals, atom.ID, dim, tag)
			tagPlaceholders = append(tagPlaceholders, "(?, ?, ?)")

			if len(tagPlaceholders) >= tagChunkSize {
				return flushTags()
			}
			return nil
		}

		addTags := func(dim string, values []string) error {
			for _, val := range values {
				if err := queueTag(dim, val); err != nil {
					return err
				}
			}
			return nil
		}

		if err := addTags("mode", atom.OperationalModes); err != nil {
			return err
		}
		if err := addTags("phase", atom.CampaignPhases); err != nil {
			return err
		}
		if err := addTags("layer", atom.BuildLayers); err != nil {
			return err
		}
		if err := addTags("init_phase", atom.InitPhases); err != nil {
			return err
		}
		if err := addTags("northstar_phase", atom.NorthstarPhases); err != nil {
			return err
		}
		if err := addTags("ouroboros_stage", atom.OuroborosStages); err != nil {
			return err
		}
		if err := addTags("intent", atom.IntentVerbs); err != nil {
			return err
		}
		if err := addTags("shard", atom.ShardTypes); err != nil {
			return err
		}
		if err := addTags("lang", atom.Languages); err != nil {
			return err
		}
		if err := addTags("framework", atom.Frameworks); err != nil {
			return err
		}
		if err := addTags("state", atom.WorldStates); err != nil {
			return err
		}
		if err := addTags("depends_on", atom.DependsOn); err != nil {
			return err
		}
		if err := addTags("conflicts_with", atom.ConflictsWith); err != nil {
			return err
		}
	}

	if err := flushTags(); err != nil {
		return fmt.Errorf("batch insert tags failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

func (l *AtomLoader) storeAtomTx(ctx context.Context, tx *sql.Tx, atom *PromptAtom) error {
	// Generate embedding if engine is available
	var embeddingBlob []byte
	var embeddingTask string

	// Embed DESCRIPTION if available, otherwise CONTENT
	textToEmbed := atom.Description
	if textToEmbed == "" {
		textToEmbed = atom.Content
	}

	if l.embeddingEngine != nil {
		taskType := embedding.SelectTaskType(embedding.ContentTypePromptAtom, false)
		var embeddingVec []float32
		var err error
		if taskAware, ok := l.embeddingEngine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
			embeddingVec, err = taskAware.EmbedWithTask(ctx, textToEmbed, taskType)
		} else {
			embeddingVec, err = l.embeddingEngine.Embed(ctx, textToEmbed)
		}
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to generate embedding for atom %s: %v", atom.ID, err)
			// Continue without embedding
		} else {
			embeddingBlob = encodeFloat32Slice(embeddingVec)
			embeddingTask = taskType
		}
	}

	// 1. Upsert Atom (Base Fields Only)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash,
			description, content_concise, content_min,
			category, subcategory,
			priority, is_mandatory, is_exclusive,
			embedding, embedding_task
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			embedding_task = excluded.embedding_task`,
		atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
		nullableString(atom.Description), nullableString(atom.ContentConcise), nullableString(atom.ContentMin),
		string(atom.Category), nullableString(atom.Subcategory),
		atom.Priority, atom.IsMandatory, nullableString(atom.IsExclusive),
		embeddingBlob, nullableString(embeddingTask),
	)
	if err != nil {
		return fmt.Errorf("upsert atom failed: %w", err)
	}

	// 2. Update Context Tags (Delete + Insert)
	if _, err := tx.ExecContext(ctx, "DELETE FROM atom_context_tags WHERE atom_id = ?", atom.ID); err != nil {
		return fmt.Errorf("clear tags failed: %w", err)
	}

	if err := insertContextTagsBatch(ctx, tx, []*PromptAtom{atom}); err != nil {
		return fmt.Errorf("failed to insert context tags: %w", err)
	}
	return nil
}

// LoadAgentPrompts loads prompt atoms for a specific agent from .nerd/agents/{name}/prompts.yaml
// into the agent's unified knowledge database at .nerd/shards/{name}_knowledge.db.
func LoadAgentPrompts(ctx context.Context, agentName string, nerdDir string, embeddingEngine embedding.EmbeddingEngine) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadAgentPrompts")
	defer timer.Stop()

	logging.Get(logging.CategoryStore).Info("Loading prompts for agent: %s", agentName)

	// Check if agent prompts.yaml exists
	promptsPath := filepath.Join(nerdDir, "agents", agentName, "prompts.yaml")
	if _, err := os.Stat(promptsPath); os.IsNotExist(err) {
		logging.Get(logging.CategoryStore).Debug("No prompts.yaml found for agent %s", agentName)
		return 0, nil
	}

	// Initialize loader
	loader := NewAtomLoader(embeddingEngine)

	// Open agent's unified knowledge database (NOT a separate prompts DB)
	dbPath := filepath.Join(nerdDir, "shards", fmt.Sprintf("%s_knowledge.db", strings.ToLower(agentName)))

	// Ensure the knowledge DB exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("agent knowledge database does not exist: %s (run 'nerd init' first)", dbPath)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open agent knowledge database: %w", err)
	}
	defer db.Close()
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

	// Ensure prompt_atoms table exists in this knowledge DB
	// This is safe to call multiple times (CREATE TABLE IF NOT EXISTS)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		return 0, fmt.Errorf("failed to ensure prompt_atoms table: %w", err)
	}

	// Load from YAML into the agent's knowledge DB
	count, err := loader.LoadFromYAML(ctx, promptsPath, db)
	if err != nil {
		return 0, fmt.Errorf("failed to load prompts: %w", err)
	}

	logging.Get(logging.CategoryStore).Info("Loaded %d prompts for agent %s into %s", count, agentName, dbPath)
	return count, nil
}

// LoadProjectPrompts loads project-level prompt atoms from .nerd/prompts/*.yaml
// Note: Project-level prompts are currently NOT USED. Prompts are stored per-agent.
// This function is kept for backward compatibility but logs a deprecation warning.
func LoadProjectPrompts(ctx context.Context, nerdDir string, embeddingEngine embedding.EmbeddingEngine) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadProjectPrompts")
	defer timer.Stop()

	promptsDir := filepath.Join(nerdDir, "prompts")
	if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
		logging.Get(logging.CategoryStore).Debug("No prompts directory found")
		return 0, nil
	}

	// Check for YAML files - if they exist, warn user to migrate
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read prompts directory: %w", err)
	}

	yamlCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			yamlCount++
		}
	}

	if yamlCount > 0 {
		logging.Get(logging.CategoryStore).Warn("LoadProjectPrompts called but project-level prompts are deprecated - prompts should be per-agent in .nerd/agents/{name}/prompts.yaml")
		logging.Get(logging.CategoryStore).Warn("Found %d YAML files in .nerd/prompts/ - these should be migrated to per-agent prompts.yaml files", yamlCount)
	} else {
		logging.Get(logging.CategoryStore).Debug("No legacy project-level prompt YAML files found")
	}

	return 0, nil
}

// ReloadAllPrompts reloads all prompt atoms (project-level and agent-specific).
func ReloadAllPrompts(ctx context.Context, nerdDir string, embeddingEngine embedding.EmbeddingEngine) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ReloadAllPrompts")
	defer timer.Stop()

	logging.Get(logging.CategoryStore).Info("Reloading all prompt atoms")

	totalCount := 0

	// Load project-level prompts
	count, err := LoadProjectPrompts(ctx, nerdDir, embeddingEngine)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to load project prompts: %v", err)
	} else {
		totalCount += count
	}

	// Find all agents
	agentsDir := filepath.Join(nerdDir, "agents")
	if _, err := os.Stat(agentsDir); err == nil {
		entries, err := os.ReadDir(agentsDir)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to read agents directory: %v", err)
		} else {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				agentName := entry.Name()
				count, err := LoadAgentPrompts(ctx, agentName, nerdDir, embeddingEngine)
				if err != nil {
					logging.Get(logging.CategoryStore).Error("Failed to load prompts for agent %s: %v", agentName, err)
				} else {
					totalCount += count
				}
			}
		}
	}

	logging.Get(logging.CategoryStore).Info("Reloaded total of %d prompt atoms", totalCount)
	return totalCount, nil
}

// ============================================================================
// JIT COMPILER INTEGRATION
// ============================================================================

// RegisterAgentDBWithJIT opens an agent's knowledge database and registers it with a JIT prompt compiler.
// The DB handle is kept open for the duration of the shard's lifecycle. The caller is responsible
// for calling UnregisterShardDB when the shard is deactivated to close the DB connection.
//
// Parameters:
//   - compiler: The JIT prompt compiler to register the DB with
//   - agentName: The name of the agent (used as shard ID for the compiler)
//   - dbPath: Full path to the agent's knowledge database (.nerd/shards/{name}_knowledge.db)
//
// Returns error if the DB cannot be opened or registered.
func RegisterAgentDBWithJIT(compiler *JITPromptCompiler, agentName, dbPath string) error {
	if compiler == nil {
		return fmt.Errorf("JIT compiler is nil")
	}

	// Open the database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open agent knowledge database %s: %w", dbPath, err)
	}
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

	// Verify the connection is valid
	if pingErr := db.Ping(); pingErr != nil {
		db.Close()
		return fmt.Errorf("failed to ping agent knowledge database %s: %w", dbPath, pingErr)
	}

	// Register with the JIT compiler
	compiler.RegisterShardDB(agentName, db)

	logging.Get(logging.CategoryStore).Info("Registered agent DB with JIT compiler: %s -> %s", agentName, dbPath)
	return nil
}

// CreateJITDBRegistrar creates a JITDBRegistrar callback function that registers agent DBs
// with the given JIT prompt compiler. This is used by ShardManager to wire up the integration.
//
// Usage in main/bootstrap code:
//
//	compiler, _ := prompt.NewJITPromptCompiler(...)
//	shardMgr.SetJITRegistrar(prompt.CreateJITDBRegistrar(compiler))
func CreateJITDBRegistrar(compiler *JITPromptCompiler) func(agentName, dbPath string) error {
	return func(agentName, dbPath string) error {
		return RegisterAgentDBWithJIT(compiler, agentName, dbPath)
	}
}

// CreateJITDBUnregistrar creates a JITDBUnregistrar callback function that unregisters
// agent DBs from the given JIT prompt compiler. This closes the DB connection and frees resources.
//
// Usage in main/bootstrap code:
//
//	compiler, _ := prompt.NewJITPromptCompiler(...)
//	shardMgr.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(compiler))
func CreateJITDBUnregistrar(compiler *JITPromptCompiler) func(agentName string) {
	return func(agentName string) {
		if compiler == nil {
			return
		}
		compiler.UnregisterShardDB(agentName)
		logging.Get(logging.CategoryStore).Debug("Unregistered agent DB from JIT compiler: %s", agentName)
	}
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// encodeFloat32Slice converts a float32 slice to bytes (little-endian).
func encodeFloat32Slice(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// nullableString returns nil for empty strings, otherwise the string.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// toJSONString converts JSON bytes to string, returning nil for empty arrays.
func toJSONString(data []byte) any {
	if len(data) == 0 || string(data) == "[]" || string(data) == "null" {
		return nil
	}
	return string(data)
}

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
	"strings"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

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
			logging.Get(logging.CategoryStore).Error("Failed to store atom %s: %v", atom.ID, err)
			continue
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

	totalStored := 0
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Load this file
		stored, loadErr := l.LoadFromYAML(ctx, path, db)
		if loadErr != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to load %s: %v", path, loadErr)
			return nil // Continue processing other files
		}

		totalStored += stored
		return nil
	})

	if err != nil {
		return totalStored, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
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
			var dfltValue interface{}
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
			// Column missing, add it
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE prompt_atoms ADD COLUMN %s TEXT", col)); err != nil {
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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse as array of atoms
	var rawAtoms []yamlAtomDefinition
	if err := yaml.Unmarshal(data, &rawAtoms); err != nil {
		// Try parsing as single atom
		var single yamlAtomDefinition
		if singleErr := yaml.Unmarshal(data, &single); singleErr != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		rawAtoms = []yamlAtomDefinition{single}
	}

	// Convert to PromptAtom structs
	var atoms []*PromptAtom
	for _, raw := range rawAtoms {
		atom, err := l.convertYAMLAtom(raw, path)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Skipping invalid atom in %s: %v", path, err)
			continue
		}
		atoms = append(atoms, atom)
	}

	return atoms, nil
}

// yamlAtomDefinition matches the YAML structure used in internal/prompt/atoms/*.yaml
// and .nerd/agents/*/prompts.yaml.
type yamlAtomDefinition struct {
	ID          string `yaml:"id"`
	Category    string `yaml:"category"`
	Subcategory string `yaml:"subcategory,omitempty"`

	// Polymorphism / semantic embedding helpers
	Description    string `yaml:"description,omitempty"`
	ContentConcise string `yaml:"content_concise,omitempty"`
	ContentMin     string `yaml:"content_min,omitempty"`

	Priority      int      `yaml:"priority"`
	IsMandatory   bool     `yaml:"is_mandatory"`
	IsExclusive   string   `yaml:"is_exclusive,omitempty"`
	DependsOn     []string `yaml:"depends_on,omitempty"`
	ConflictsWith []string `yaml:"conflicts_with,omitempty"`

	OperationalModes []string `yaml:"operational_modes,omitempty"`
	CampaignPhases   []string `yaml:"campaign_phases,omitempty"`
	BuildLayers      []string `yaml:"build_layers,omitempty"`
	InitPhases       []string `yaml:"init_phases,omitempty"`
	NorthstarPhases  []string `yaml:"northstar_phases,omitempty"`
	OuroborosStages  []string `yaml:"ouroboros_stages,omitempty"`
	IntentVerbs      []string `yaml:"intent_verbs,omitempty"`
	ShardTypes       []string `yaml:"shard_types,omitempty"`
	Languages        []string `yaml:"languages,omitempty"`
	Frameworks       []string `yaml:"frameworks,omitempty"`
	WorldStates      []string `yaml:"world_states,omitempty"`

	Content     string `yaml:"content,omitempty"`
	ContentFile string `yaml:"content_file,omitempty"`
}

// convertYAMLAtom converts a YAML atom definition to a PromptAtom.
func (l *AtomLoader) convertYAMLAtom(raw yamlAtomDefinition, sourcePath string) (*PromptAtom, error) {
	if raw.ID == "" {
		return nil, fmt.Errorf("atom missing ID")
	}

	if raw.Category == "" {
		return nil, fmt.Errorf("atom %s missing category", raw.ID)
	}
	// Normalize categories for backwards compatibility with older templates.
	raw.Category = strings.ToLower(strings.TrimSpace(raw.Category))
	switch raw.Category {
	case "domain_knowledge":
		raw.Category = string(CategoryDomain)
	}

	// Resolve content
	content := raw.Content
	if raw.ContentFile != "" && content == "" {
		contentPath := filepath.Join(filepath.Dir(sourcePath), raw.ContentFile)
		contentData, err := os.ReadFile(contentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read content file %s: %w", raw.ContentFile, err)
		}
		content = string(contentData)
	}

	if content == "" {
		return nil, fmt.Errorf("atom %s has no content", raw.ID)
	}

	// Compute token count and hash
	tokenCount := EstimateTokens(content)
	contentHash := HashContent(content)

	atom := &PromptAtom{
		ID:               raw.ID,
		Version:          1,
		Category:         AtomCategory(raw.Category),
		Subcategory:      raw.Subcategory,
		Content:          content,
		TokenCount:       tokenCount,
		ContentHash:      contentHash,
		Description:      raw.Description,
		ContentConcise:   raw.ContentConcise,
		ContentMin:       raw.ContentMin,
		Priority:         raw.Priority,
		IsMandatory:      raw.IsMandatory,
		IsExclusive:      raw.IsExclusive,
		DependsOn:        raw.DependsOn,
		ConflictsWith:    raw.ConflictsWith,
		OperationalModes: raw.OperationalModes,
		CampaignPhases:   raw.CampaignPhases,
		BuildLayers:      raw.BuildLayers,
		InitPhases:       raw.InitPhases,
		NorthstarPhases:  raw.NorthstarPhases,
		OuroborosStages:  raw.OuroborosStages,
		IntentVerbs:      raw.IntentVerbs,
		ShardTypes:       raw.ShardTypes,
		Languages:        raw.Languages,
		Frameworks:       raw.Frameworks,
		WorldStates:      raw.WorldStates,
		CreatedAt:        time.Now(),
	}

	// Validate
	if err := atom.Validate(); err != nil {
		return nil, fmt.Errorf("invalid atom: %w", err)
	}

	return atom, nil
}

// StoreAtom stores a prompt atom in the database with optional embedding.
func (l *AtomLoader) StoreAtom(ctx context.Context, db *sql.DB, atom *PromptAtom) error {
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

	// Helper to collect tags
	tags := make(map[string][]string)
	addTags := func(dim string, values []string) {
		if len(values) > 0 {
			tags[dim] = values
		}
	}
	addTags("mode", atom.OperationalModes)
	addTags("phase", atom.CampaignPhases)
	addTags("layer", atom.BuildLayers)
	addTags("init_phase", atom.InitPhases)
	addTags("northstar_phase", atom.NorthstarPhases)
	addTags("ouroboros_stage", atom.OuroborosStages)
	addTags("intent", atom.IntentVerbs)
	addTags("shard", atom.ShardTypes)
	addTags("lang", atom.Languages)
	addTags("framework", atom.Frameworks)
	addTags("state", atom.WorldStates)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Upsert Atom (Base Fields Only)
	_, err = tx.ExecContext(ctx, `
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

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for dim, values := range tags {
		for _, tag := range values {
			if _, err := stmt.ExecContext(ctx, atom.ID, dim, tag); err != nil {
				return fmt.Errorf("insert tag failed: %w", err)
			}
		}
	}

	// Handling DependsOn and ConflictsWith as tags or separate tables?
	// The User Plan mentioned "atom_requires", "atom_conflicts" predicates.
	// But in DB, where do they go?
	// I should add them to `atom_context_tags` with dim='depends_on' / 'conflicts_with'?
	// OR create separate link tables `atom_dependencies`, `atom_conflicts`.
	// For simplicity, let's use `atom_context_tags` with reserved dimensions.
	// This works for simple ID references.
	for _, dep := range atom.DependsOn {
		if _, err := stmt.ExecContext(ctx, atom.ID, "depends_on", dep); err != nil {
			return fmt.Errorf("insert dep failed: %w", err)
		}
	}
	for _, conf := range atom.ConflictsWith {
		if _, err := stmt.ExecContext(ctx, atom.ID, "conflicts_with", conf); err != nil {
			return fmt.Errorf("insert conflict failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
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
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// toJSONString converts JSON bytes to string, returning nil for empty arrays.
func toJSONString(data []byte) interface{} {
	if len(data) == 0 || string(data) == "[]" || string(data) == "null" {
		return nil
	}
	return string(data)
}

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

	embeddings, err := engine.EmbedBatch(ctx, textsToEmbed)
	if err != nil {
		return fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	if len(embeddings) != len(atomsToEmbed) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(atomsToEmbed))
	}

	// Store atoms with embeddings in a transaction for atomicity
	if err := storeAtomsWithEmbeddings(ctx, db, atomsToEmbed, embeddings); err != nil {
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
func storeAtomsWithEmbeddings(ctx context.Context, db *sql.DB, atoms []*PromptAtom, embeddings [][]float32) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statements for efficiency
	atomStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash,
			description, content_concise, content_min,
			category, subcategory,
			priority, is_mandatory, is_exclusive,
			embedding, embedding_task, source_file
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			source_file = excluded.source_file`)
	if err != nil {
		return fmt.Errorf("failed to prepare atom statement: %w", err)
	}
	defer atomStmt.Close()

	deleteTagsStmt, err := tx.PrepareContext(ctx, "DELETE FROM atom_context_tags WHERE atom_id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare delete tags statement: %w", err)
	}
	defer deleteTagsStmt.Close()

	insertTagStmt, err := tx.PrepareContext(ctx, "INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert tag statement: %w", err)
	}
	defer insertTagStmt.Close()

	// Process each atom
	for i, atom := range atoms {
		embeddingBlob := encodeFloat32Slice(embeddings[i])

		// Insert/update atom
		_, err := atomStmt.ExecContext(ctx,
			atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
			nullableString(atom.Description), nullableString(atom.ContentConcise), nullableString(atom.ContentMin),
			string(atom.Category), nullableString(atom.Subcategory),
			atom.Priority, atom.IsMandatory, nullableString(atom.IsExclusive),
			embeddingBlob, "RETRIEVAL_DOCUMENT", "embedded",
		)
		if err != nil {
			return fmt.Errorf("failed to insert atom %s: %w", atom.ID, err)
		}

		// Clear existing tags
		if _, err := deleteTagsStmt.ExecContext(ctx, atom.ID); err != nil {
			return fmt.Errorf("failed to clear tags for atom %s: %w", atom.ID, err)
		}

		// Insert context tags
		if err := insertContextTags(ctx, insertTagStmt, atom); err != nil {
			return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
		}

		// Log progress every 50 atoms
		if (i+1)%50 == 0 || i == len(atoms)-1 {
			logging.Get(logging.CategoryStore).Debug("Stored %d/%d atoms", i+1, len(atoms))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// insertContextTags inserts all context tags for an atom.
func insertContextTags(ctx context.Context, stmt *sql.Stmt, atom *PromptAtom) error {
	// Helper to insert tags for a dimension
	insertDim := func(dimension string, values []string) error {
		for _, tag := range values {
			if _, err := stmt.ExecContext(ctx, atom.ID, dimension, tag); err != nil {
				return err
			}
		}
		return nil
	}

	// Insert all dimension tags
	if err := insertDim("mode", atom.OperationalModes); err != nil {
		return err
	}
	if err := insertDim("phase", atom.CampaignPhases); err != nil {
		return err
	}
	if err := insertDim("layer", atom.BuildLayers); err != nil {
		return err
	}
	if err := insertDim("init_phase", atom.InitPhases); err != nil {
		return err
	}
	if err := insertDim("northstar_phase", atom.NorthstarPhases); err != nil {
		return err
	}
	if err := insertDim("ouroboros_stage", atom.OuroborosStages); err != nil {
		return err
	}
	if err := insertDim("intent", atom.IntentVerbs); err != nil {
		return err
	}
	if err := insertDim("shard", atom.ShardTypes); err != nil {
		return err
	}
	if err := insertDim("lang", atom.Languages); err != nil {
		return err
	}
	if err := insertDim("framework", atom.Frameworks); err != nil {
		return err
	}
	if err := insertDim("state", atom.WorldStates); err != nil {
		return err
	}

	// Insert dependency and conflict tags
	if err := insertDim("depends_on", atom.DependsOn); err != nil {
		return err
	}
	if err := insertDim("conflicts_with", atom.ConflictsWith); err != nil {
		return err
	}

	return nil
}

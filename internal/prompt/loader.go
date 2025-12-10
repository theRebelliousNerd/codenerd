// Package prompt - Runtime YAML → SQLite ingestion for prompt atoms.
// This loader enables runtime loading of agent-specific and project-level prompt atoms
// from YAML files into SQLite databases for JIT prompt compilation.
package prompt

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
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
	atoms, err := l.parseYAMLFile(yamlPath)
	if err != nil {
		return 0, fmt.Errorf("failed to parse YAML file %s: %w", yamlPath, err)
	}

	logging.Get(logging.CategoryStore).Info("Parsed %d atoms from %s", len(atoms), filepath.Base(yamlPath))

	// Store atoms in database
	stored := 0
	for _, atom := range atoms {
		if err := l.storeAtom(ctx, db, atom); err != nil {
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

// ensurePromptAtomsTable creates the prompt_atoms table and atom_context_tags table.
func ensurePromptAtomsTable(db *sql.DB) error {
	schema := `
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

		CREATE INDEX IF NOT EXISTS idx_atoms_category ON prompt_atoms(category);
		CREATE INDEX IF NOT EXISTS idx_atoms_description ON prompt_atoms(description);
		CREATE INDEX IF NOT EXISTS idx_tags_lookup ON atom_context_tags(dimension, tag);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create prompt tables: %w", err)
	}

	// Schema Migration: Add new columns if missing (for v4->v5 transition)
	cols := []string{"description", "content_concise", "content_min"}
	for _, col := range cols {
		var dummy interface{}
		err := db.QueryRow(fmt.Sprintf("SELECT %s FROM prompt_atoms LIMIT 1", col)).Scan(&dummy)
		if err != nil {
			// Column missing, add it
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE prompt_atoms ADD COLUMN %s TEXT", col)); err != nil {
				logging.Get(logging.CategoryStore).Warn("Failed to add column %s: %v", col, err)
			}
		}
	}

	return nil
}

// parseYAMLFile parses a YAML file containing prompt atom definitions.
func (l *AtomLoader) parseYAMLFile(path string) ([]*PromptAtom, error) {
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
			logging.Get(logging.CategoryStore).Warn("Skipping invalid atom in %s: %v", path, err)
			continue
		}
		atoms = append(atoms, atom)
	}

	return atoms, nil
}

// yamlAtomDefinition matches the YAML structure used in build/prompt_atoms/*.yaml
type yamlAtomDefinition struct {
	ID          string `yaml:"id"`
	Category    string `yaml:"category"`
	Subcategory string `yaml:"subcategory,omitempty"`

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

// storeAtom stores a prompt atom in the database with optional embedding.
func (l *AtomLoader) storeAtom(ctx context.Context, db *sql.DB, atom *PromptAtom) error {
	// Generate embedding if engine is available
	var embeddingBlob []byte
	var embeddingTask string

	// Embed DESCRIPTION if available, otherwise CONTENT
	textToEmbed := atom.Description
	if textToEmbed == "" {
		textToEmbed = atom.Content
	}

	if l.embeddingEngine != nil {
		embedding, err := l.embeddingEngine.Embed(ctx, textToEmbed)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to generate embedding for atom %s: %v", atom.ID, err)
			// Continue without embedding
		} else {
			embeddingBlob = encodeFloat32Slice(embedding)
			embeddingTask = "RETRIEVAL_DOCUMENT"
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

	dependsOnJSON, _ := json.Marshal(atom.DependsOn)
	conflictsWithJSON, _ := json.Marshal(atom.ConflictsWith)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Upsert Atom
	_, err = tx.ExecContext(ctx, `
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash,
			description, content_concise, content_min,
			category, subcategory,
			priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			embedding, embedding_task
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			depends_on = excluded.depends_on,
			conflicts_with = excluded.conflicts_with,
			embedding = excluded.embedding,
			embedding_task = excluded.embedding_task`,
		atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
		nullableString(atom.Description), nullableString(atom.ContentConcise), nullableString(atom.ContentMin),
		string(atom.Category), nullableString(atom.Subcategory),
		atom.Priority, atom.IsMandatory, nullableString(atom.IsExclusive),
		toJSONString(dependsOnJSON), toJSONString(conflictsWithJSON),
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
			// Normalize tag (ensure starts with / if not present?)
			// User example had /mode, /active. YAML usually has raw strings.
			// Assuming they match what's in YAML. Mangle expects constants like /foo.
			// We will insert exactly what is in YAML. Runtime transformation to Mangle Atom might be needed if YAML is "active".
			// But Atoms.go MatchContext expects strings.
			// Let's assume strings are fine.
			if _, err := stmt.ExecContext(ctx, atom.ID, dim, tag); err != nil {
				return fmt.Errorf("insert tag failed: %w", err)
			}
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
	if err := ensurePromptAtomsTable(db); err != nil {
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

	logging.Get(logging.CategoryStore).Warn("LoadProjectPrompts called but project-level prompts are deprecated - prompts should be per-agent in .nerd/agents/{name}/prompts.yaml")

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
		logging.Get(logging.CategoryStore).Warn("Found %d YAML files in .nerd/prompts/ - these should be migrated to per-agent prompts.yaml files", yamlCount)
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

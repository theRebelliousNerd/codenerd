// Package main implements the prompt builder tool for creating a baked-in prompt corpus
// database for JIT prompt compilation. It parses YAML atom definitions from
// internal/prompt/atoms/ (or a custom -input path) and optionally generates embeddings
// for semantic search during prompt assembly.
//
// Usage: go run ./cmd/tools/prompt_builder
//
// Build with sqlite-vec support:
//
//	$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go run -tags=sqlite_vec ./cmd/tools/prompt_builder
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/embedding"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

const (
	// batchSize for embedding generation
	batchSize = 32

	// embedding dimensions for gemini-embedding-001
	embeddingDimensions = 3072
)

// AtomDefinition is the YAML structure for atom source files.
type AtomDefinition struct {
	// Core identity
	ID          string `yaml:"id"`
	Category    string `yaml:"category"`
	Subcategory string `yaml:"subcategory,omitempty"`

	// Polymorphism / embedding helpers (optional)
	Description    string `yaml:"description,omitempty"`
	ContentConcise string `yaml:"content_concise,omitempty"`
	ContentMin     string `yaml:"content_min,omitempty"`

	// Composition
	Priority      int      `yaml:"priority"`
	IsMandatory   bool     `yaml:"is_mandatory"`
	IsExclusive   string   `yaml:"is_exclusive,omitempty"`
	DependsOn     []string `yaml:"depends_on,omitempty"`
	ConflictsWith []string `yaml:"conflicts_with,omitempty"`

	// Contextual Selectors
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

	// Content (can be inline or reference a file)
	Content     string `yaml:"content,omitempty"`
	ContentFile string `yaml:"content_file,omitempty"`
}

// ProcessedAtom is an AtomDefinition with computed fields.
type ProcessedAtom struct {
	AtomDefinition
	TokenCount  int
	ContentHash string
	SourceFile  string
}

func main() {
	inputDir := flag.String("input", "internal/prompt/atoms", "Input directory with YAML atom definitions")
	outputDB := flag.String("output", "internal/core/defaults/prompt_corpus.db", "Output SQLite database")
	skipEmbeddings := flag.Bool("skip-embeddings", false, "Skip embedding generation (faster for testing)")
	flag.Parse()

	fmt.Println("=================================================")
	fmt.Println("  PROMPT BUILDER - JIT Prompt Corpus DB")
	fmt.Println("=================================================")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Step 1: Get API key (unless skipping embeddings)
	var engine embedding.EmbeddingEngine
	if !*skipEmbeddings {
		apiKey := getAPIKey()
		if apiKey == "" {
			fmt.Println("ERROR: No API key found")
			fmt.Println("Set GEMINI_API_KEY environment variable or configure .nerd/config.json")
			fmt.Println("Or use -skip-embeddings for testing without embeddings")
			os.Exit(1)
		}
		fmt.Printf("[OK] API key found (length=%d)\n", len(apiKey))

		// Step 2: Create embedding engine
		var err error
		engine, err = createEmbeddingEngine(apiKey)
		if err != nil {
			fmt.Printf("ERROR: Failed to create embedding engine: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[OK] Embedding engine created: %s (dimensions=%d)\n", engine.Name(), engine.Dimensions())
	} else {
		fmt.Println("[SKIP] Embedding generation disabled")
	}

	// Step 3: Parse all YAML files and extract atom definitions
	atoms, err := extractAtomDefinitions(*inputDir)
	if err != nil {
		fmt.Printf("ERROR: Failed to extract atom definitions: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] Extracted %d atom definitions\n", len(atoms))

	if len(atoms) == 0 {
		fmt.Println("WARNING: No atoms extracted. Nothing to embed.")
		os.Exit(0)
	}

	// Step 4: Validate atoms
	if err := validateAtoms(atoms); err != nil {
		fmt.Printf("ERROR: Validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] Atom validation passed")

	// Step 5: Create database
	db, err := createDatabase(*outputDB)
	if err != nil {
		fmt.Printf("ERROR: Failed to create database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Printf("[OK] Database created: %s\n", *outputDB)

	// Step 6: Generate embeddings and store
	if err := generateAndStoreAtoms(ctx, engine, db, atoms, *skipEmbeddings); err != nil {
		fmt.Printf("ERROR: Failed to store atoms: %v\n", err)
		os.Exit(1)
	}

	// Step 7: Print summary
	printSummary(db, *outputDB)

	fmt.Println()
	fmt.Println("=================================================")
	fmt.Println("  PROMPT CORPUS BUILD COMPLETE")
	fmt.Println("=================================================")
}

// getAPIKey retrieves the Gemini API key from environment or config file.
func getAPIKey() string {
	// First try environment variable
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}

	// Try to read from .nerd/config.json
	configPaths := []string{
		".nerd/config.json",
		filepath.Join(os.Getenv("HOME"), ".nerd/config.json"),
		filepath.Join(os.Getenv("USERPROFILE"), ".nerd/config.json"),
	}

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Simple extraction - look for genai_api_key or gemini_api_key
		content := string(data)
		for _, key := range []string{"genai_api_key", "gemini_api_key"} {
			if idx := strings.Index(content, key); idx != -1 {
				// Find the value after the key
				rest := content[idx+len(key):]
				// Skip ":" and whitespace, find the quoted value
				if start := strings.Index(rest, `"`); start != -1 {
					rest = rest[start+1:]
					if end := strings.Index(rest, `"`); end != -1 {
						return rest[:end]
					}
				}
			}
		}
	}

	return ""
}

// createEmbeddingEngine creates a GenAI embedding engine configured for document retrieval.
func createEmbeddingEngine(apiKey string) (embedding.EmbeddingEngine, error) {
	cfg := embedding.Config{
		Provider:    "genai",
		GenAIAPIKey: apiKey,
		GenAIModel:  "gemini-embedding-001",
		TaskType:    "RETRIEVAL_DOCUMENT", // Optimized for document indexing
	}

	return embedding.NewEngine(cfg)
}

// extractAtomDefinitions parses all YAML files from the input directory.
func extractAtomDefinitions(inputDir string) ([]ProcessedAtom, error) {
	var atoms []ProcessedAtom

	// Walk the input directory
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .yaml and .yml files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		fileAtoms, parseErr := parseYAMLFile(path, inputDir)
		if parseErr != nil {
			fmt.Printf("  WARNING: Failed to parse %s: %v\n", filepath.Base(path), parseErr)
			return nil
		}

		atoms = append(atoms, fileAtoms...)
		fmt.Printf("  Parsed %s... found %d atoms\n", filepath.Base(path), len(fileAtoms))

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk input directory: %w", err)
	}

	return atoms, nil
}

// parseYAMLFile parses a single YAML file containing atom definitions.
func parseYAMLFile(path, baseDir string) ([]ProcessedAtom, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// YAML files can contain multiple documents (using ---)
	// or a single list of atoms
	var rawAtoms []AtomDefinition

	// Try parsing as a list first
	if err := yaml.Unmarshal(data, &rawAtoms); err != nil {
		// Try parsing as a single atom
		var single AtomDefinition
		if singleErr := yaml.Unmarshal(data, &single); singleErr != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		rawAtoms = []AtomDefinition{single}
	}

	// Process each atom
	var processed []ProcessedAtom
	relPath, _ := filepath.Rel(baseDir, path)

	for _, atom := range rawAtoms {
		// Skip atoms without ID
		if atom.ID == "" {
			continue
		}

		// Resolve content from file if specified
		content := atom.Content
		if atom.ContentFile != "" && content == "" {
			contentPath := filepath.Join(filepath.Dir(path), atom.ContentFile)
			contentData, err := os.ReadFile(contentPath)
			if err != nil {
				fmt.Printf("    WARNING: Failed to read content file %s: %v\n", atom.ContentFile, err)
				continue
			}
			content = string(contentData)
		}

		if content == "" {
			fmt.Printf("    WARNING: Atom %s has no content\n", atom.ID)
			continue
		}

		// Compute token count (chars/4 approximation)
		tokenCount := len(content) / 4
		if tokenCount == 0 {
			tokenCount = 1
		}

		// Compute content hash
		hash := sha256.Sum256([]byte(content))
		contentHash := hex.EncodeToString(hash[:])

		processed = append(processed, ProcessedAtom{
			AtomDefinition: AtomDefinition{
				ID:               atom.ID,
				Category:         atom.Category,
				Subcategory:      atom.Subcategory,
				Priority:         atom.Priority,
				IsMandatory:      atom.IsMandatory,
				IsExclusive:      atom.IsExclusive,
				DependsOn:        atom.DependsOn,
				ConflictsWith:    atom.ConflictsWith,
				OperationalModes: atom.OperationalModes,
				CampaignPhases:   atom.CampaignPhases,
				BuildLayers:      atom.BuildLayers,
				InitPhases:       atom.InitPhases,
				NorthstarPhases:  atom.NorthstarPhases,
				OuroborosStages:  atom.OuroborosStages,
				IntentVerbs:      atom.IntentVerbs,
				ShardTypes:       atom.ShardTypes,
				Languages:        atom.Languages,
				Frameworks:       atom.Frameworks,
				WorldStates:      atom.WorldStates,
				Content:          content,
			},
			TokenCount:  tokenCount,
			ContentHash: contentHash,
			SourceFile:  relPath,
		})
	}

	return processed, nil
}

// validateAtoms checks for duplicate IDs and validates dependencies.
func validateAtoms(atoms []ProcessedAtom) error {
	// Check for duplicate IDs
	seen := make(map[string]string) // id -> source file
	for _, atom := range atoms {
		if existing, ok := seen[atom.ID]; ok {
			return fmt.Errorf("duplicate atom ID %q in %s (first seen in %s)", atom.ID, atom.SourceFile, existing)
		}
		seen[atom.ID] = atom.SourceFile
	}

	// Validate dependencies exist
	for _, atom := range atoms {
		for _, dep := range atom.DependsOn {
			if _, ok := seen[dep]; !ok {
				return fmt.Errorf("atom %q depends on non-existent atom %q", atom.ID, dep)
			}
		}
		for _, conflict := range atom.ConflictsWith {
			if _, ok := seen[conflict]; !ok {
				fmt.Printf("  WARNING: Atom %q conflicts with non-existent atom %q\n", atom.ID, conflict)
			}
		}
	}

	// Validate categories
	validCategories := map[string]bool{
		"identity":      true,
		"protocol":      true,
		"safety":        true,
		"methodology":   true,
		"hallucination": true,
		"language":      true,
		"framework":     true,
		"domain":        true,
		"campaign":      true,
		"init":          true,
		"northstar":     true,
		"ouroboros":     true,
		"context":       true,
		"exemplar":      true,
	}

	for _, atom := range atoms {
		if atom.Category == "" {
			return fmt.Errorf("atom %q has no category", atom.ID)
		}
		if !validCategories[atom.Category] {
			fmt.Printf("  WARNING: Atom %q has non-standard category %q\n", atom.ID, atom.Category)
		}
	}

	return nil
}

// createDatabase creates the SQLite database with schema.
func createDatabase(outputPath string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Remove existing database
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing database: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create schema that matches the runtime loader/compiler expectations.
	schema := `
		CREATE TABLE prompt_atoms (
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
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Try to create sqlite-vec virtual table for vector search
	vecSchema := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_prompt_atoms USING vec0(
			embedding float[%d],
			atom_id TEXT,
			category TEXT,
			priority INTEGER
		);
	`, embeddingDimensions)

	if _, err := db.Exec(vecSchema); err != nil {
		fmt.Printf("  WARNING: sqlite-vec not available, using standard table only: %v\n", err)
		// Continue without vec - the standard table will still work
	} else {
		fmt.Println("  [OK] sqlite-vec virtual table created")
	}

	return db, nil
}

// generateAndStoreAtoms generates embeddings in batches and stores atoms.
func generateAndStoreAtoms(ctx context.Context, engine embedding.EmbeddingEngine, db *sql.DB, atoms []ProcessedAtom, skipEmbeddings bool) error {
	total := len(atoms)
	processed := 0

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertAtomStmt, err := tx.Prepare(`
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash,
			description, content_concise, content_min,
			category, subcategory,
			priority, is_mandatory, is_exclusive,
			embedding, embedding_task, source_file
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare atom insert statement: %w", err)
	}
	defer insertAtomStmt.Close()

	insertTagStmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO atom_context_tags (atom_id, dimension, tag) VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare tag insert statement: %w", err)
	}
	defer insertTagStmt.Close()

	// Check if vec_prompt_atoms exists
	var vecAvailable bool
	row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='vec_prompt_atoms'")
	if err := row.Scan(new(string)); err == nil {
		vecAvailable = true
	}

	var vecStmt *sql.Stmt
	if vecAvailable && !skipEmbeddings {
		vecStmt, err = tx.Prepare(`INSERT INTO vec_prompt_atoms (embedding, atom_id, category, priority) VALUES (?, ?, ?, ?)`)
		if err != nil {
			fmt.Printf("  WARNING: Failed to prepare vec statement: %v\n", err)
			vecAvailable = false
		} else {
			defer vecStmt.Close()
		}
	}

	// Process in batches for embeddings
	for i := 0; i < total; i += batchSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		end := i + batchSize
		if end > total {
			end = total
		}
		batch := atoms[i:end]

		// Generate embeddings for batch (unless skipped)
		var embeddings [][]float32
		if !skipEmbeddings && engine != nil {
			texts := make([]string, len(batch))
			for j, atom := range batch {
				texts[j] = getTextForEmbedding(atom)
			}

			var embedErr error
			embeddings, embedErr = engine.EmbedBatch(ctx, texts)
			if embedErr != nil {
				return fmt.Errorf("failed to generate embeddings for batch %d: %w", i/batchSize, embedErr)
			}
		}

		// Store each atom
		for j, atom := range batch {
			var embeddingBlob []byte
			var embeddingTask interface{}

			if !skipEmbeddings && embeddings != nil && j < len(embeddings) {
				embeddingBlob = encodeFloat32Slice(embeddings[j])
				embeddingTask = "RETRIEVAL_DOCUMENT"
			}

			_, err := insertAtomStmt.Exec(
				atom.ID,
				1, // version
				atom.Content,
				atom.TokenCount,
				atom.ContentHash,
				nullableString(atom.Description),
				nullableString(atom.ContentConcise),
				nullableString(atom.ContentMin),
				atom.Category,
				nullableString(atom.Subcategory),
				atom.Priority,
				atom.IsMandatory,
				nullableString(atom.IsExclusive),
				embeddingBlob,
				embeddingTask,
				atom.SourceFile,
			)
			if err != nil {
				return fmt.Errorf("failed to insert atom %s: %w", atom.ID, err)
			}

			// Context tags (normalized, matches runtime loader).
			insertTags := func(dim string, values []string) error {
				for _, v := range values {
					if strings.TrimSpace(v) == "" {
						continue
					}
					if _, err := insertTagStmt.Exec(atom.ID, dim, v); err != nil {
						return err
					}
				}
				return nil
			}

			if err := insertTags("mode", atom.OperationalModes); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("phase", atom.CampaignPhases); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("layer", atom.BuildLayers); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("init_phase", atom.InitPhases); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("northstar_phase", atom.NorthstarPhases); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("ouroboros_stage", atom.OuroborosStages); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("intent", atom.IntentVerbs); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("shard", atom.ShardTypes); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("lang", atom.Languages); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("framework", atom.Frameworks); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("state", atom.WorldStates); err != nil {
				return fmt.Errorf("failed to insert tags for atom %s: %w", atom.ID, err)
			}

			if err := insertTags("depends_on", atom.DependsOn); err != nil {
				return fmt.Errorf("failed to insert depends_on tags for atom %s: %w", atom.ID, err)
			}
			if err := insertTags("conflicts_with", atom.ConflictsWith); err != nil {
				return fmt.Errorf("failed to insert conflicts_with tags for atom %s: %w", atom.ID, err)
			}

			// Also insert into vec_prompt_atoms if available
			if vecAvailable && vecStmt != nil && embeddingBlob != nil {
				if _, err := vecStmt.Exec(embeddingBlob, atom.ID, atom.Category, atom.Priority); err != nil {
					fmt.Printf("  WARNING: Failed to insert into vec_prompt_atoms: %v\n", err)
				}
			}
		}

		processed += len(batch)
		if skipEmbeddings {
			fmt.Printf("\r  Storing atoms... %d/%d", processed, total)
		} else {
			fmt.Printf("\r  Generating embeddings and storing... %d/%d", processed, total)
		}
	}
	fmt.Println() // New line after progress

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func getTextForEmbedding(atom ProcessedAtom) string {
	if atom.Description != "" {
		return atom.Description
	}
	return atom.Content
}

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

// printSummary prints statistics about the generated database.
func printSummary(db *sql.DB, outputPath string) {
	fmt.Println()
	fmt.Println("--- Summary ---")

	// Total atoms
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&total); err == nil {
		fmt.Printf("  Total atoms: %d\n", total)
	}

	// Total tokens
	var totalTokens int
	if err := db.QueryRow("SELECT SUM(token_count) FROM prompt_atoms").Scan(&totalTokens); err == nil {
		fmt.Printf("  Total tokens: %d\n", totalTokens)
	}

	// Mandatory atoms
	var mandatory int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE is_mandatory = 1").Scan(&mandatory); err == nil {
		fmt.Printf("  Mandatory atoms: %d\n", mandatory)
	}

	// Atoms by category
	rows, err := db.Query(`
		SELECT category, COUNT(*) as cnt, SUM(token_count) as tokens
		FROM prompt_atoms
		GROUP BY category
		ORDER BY cnt DESC
	`)
	if err == nil {
		defer rows.Close()
		fmt.Println("  By category:")
		for rows.Next() {
			var cat string
			var cnt, tokens int
			if err := rows.Scan(&cat, &cnt, &tokens); err == nil {
				fmt.Printf("    %-20s %3d atoms (%5d tokens)\n", cat, cnt, tokens)
			}
		}
	}

	// Atoms by source file
	rows, err = db.Query(`
		SELECT source_file, COUNT(*) as cnt
		FROM prompt_atoms
		GROUP BY source_file
		ORDER BY cnt DESC
	`)
	if err == nil {
		defer rows.Close()
		fmt.Println("  By source file:")
		for rows.Next() {
			var file string
			var cnt int
			if err := rows.Scan(&file, &cnt); err == nil {
				fmt.Printf("    %-40s %d\n", file, cnt)
			}
		}
	}

	// Check vec_prompt_atoms
	var vecCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM vec_prompt_atoms").Scan(&vecCount); err == nil {
		fmt.Printf("  vec_prompt_atoms entries: %d\n", vecCount)
	}

	// File size
	if info, err := os.Stat(outputPath); err == nil {
		fmt.Printf("  Database size: %.2f MB\n", float64(info.Size())/(1024*1024))
	}
}

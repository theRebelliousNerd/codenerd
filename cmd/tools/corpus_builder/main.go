// Package main implements the corpus builder tool for creating a baked-in vector database
// for intent classification. It parses all .mg files in internal/core/defaults/ and extracts
// DATA facts (facts without :- bodies) to generate embeddings for semantic search.
//
// Usage: go run ./cmd/tools/corpus_builder
//
// Build with sqlite-vec support:
//
//	$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go run -tags=sqlite_vec ./cmd/tools/corpus_builder
package main

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

	"codenerd/internal/core"
	"codenerd/internal/embedding"

	_ "github.com/mattn/go-sqlite3"
)

const (
	// batchSize for embedding generation
	batchSize = 32

	// outputPath for the generated database
	outputPath = "internal/core/defaults/intent_corpus.db"

	// embedding dimensions for text-embedding-004 / gemini-embedding-001
	// Note: Google updated these models to return 3072 dimensions
	embeddingDimensions = 3072
)

// CorpusEntry represents a single entry to be embedded.
type CorpusEntry struct {
	Predicate   string
	TextContent string
	Verb        string
	Target      string
	Category    string
	SourceFile  string
}

func main() {
	fmt.Println("=================================================")
	fmt.Println("  CORPUS BUILDER - Intent Classification DB")
	fmt.Println("=================================================")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Step 1: Get API key
	apiKey := getAPIKey()
	if apiKey == "" {
		fmt.Println("ERROR: No API key found")
		fmt.Println("Set GEMINI_API_KEY environment variable or configure .nerd/config.json")
		os.Exit(1)
	}
	fmt.Printf("[OK] API key found (length=%d)\n", len(apiKey))

	// Step 2: Create embedding engine
	engine, err := createEmbeddingEngine(apiKey)
	if err != nil {
		fmt.Printf("ERROR: Failed to create embedding engine: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] Embedding engine created: %s (dimensions=%d)\n", engine.Name(), engine.Dimensions())

	// Step 3: Parse all .mg files and extract DATA facts
	entries, err := extractCorpusEntries()
	if err != nil {
		fmt.Printf("ERROR: Failed to extract corpus entries: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] Extracted %d corpus entries\n", len(entries))

	if len(entries) == 0 {
		fmt.Println("WARNING: No entries extracted. Nothing to embed.")
		os.Exit(0)
	}

	// Step 4: Create database
	db, err := createDatabase()
	if err != nil {
		fmt.Printf("ERROR: Failed to create database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Printf("[OK] Database created: %s\n", outputPath)

	// Step 5: Generate embeddings and store
	if err := generateAndStoreEmbeddings(ctx, engine, db, entries); err != nil {
		fmt.Printf("ERROR: Failed to generate embeddings: %v\n", err)
		os.Exit(1)
	}

	// Step 6: Print summary
	printSummary(db)

	fmt.Println()
	fmt.Println("=================================================")
	fmt.Println("  CORPUS BUILD COMPLETE")
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

// extractCorpusEntries parses all .mg files and extracts DATA facts.
func extractCorpusEntries() ([]CorpusEntry, error) {
	var entries []CorpusEntry

	// Find all .mg files in defaults/ and defaults/schema/
	mgFiles, err := findMGFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to find .mg files: %w", err)
	}

	for _, file := range mgFiles {
		fileEntries, err := extractFromFile(file)
		if err != nil {
			fmt.Printf("  WARNING: Failed to parse %s: %v\n", filepath.Base(file), err)
			continue
		}
		entries = append(entries, fileEntries...)
		fmt.Printf("  Parsed %s... found %d facts\n", filepath.Base(file), len(fileEntries))
	}

	return entries, nil
}

// findMGFiles locates all .mg files in the defaults directories.
func findMGFiles() ([]string, error) {
	var files []string

	// Get the embedded content paths
	basePath := "internal/core/defaults"

	// Walk the embedded filesystem for .mg files
	// Since we're using embedded files, we need to use the core package's GetDefaultContent
	// But for now, let's list the known files
	knownFiles := []string{
		"taxonomy.mg",
		"doc_taxonomy.mg",
		"build_topology.mg",
		"schema/intent.mg",
	}

	for _, f := range knownFiles {
		files = append(files, filepath.Join(basePath, f))
	}

	// Also try to find additional files from the filesystem
	fsBasePath := filepath.Join(".", basePath)
	if entries, err := os.ReadDir(fsBasePath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mg") {
				fullPath := filepath.Join(basePath, entry.Name())
				if !contains(files, fullPath) {
					files = append(files, fullPath)
				}
			}
		}
	}

	// Check schema subdirectory
	schemaPath := filepath.Join(fsBasePath, "schema")
	if entries, err := os.ReadDir(schemaPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mg") {
				fullPath := filepath.Join(basePath, "schema", entry.Name())
				if !contains(files, fullPath) {
					files = append(files, fullPath)
				}
			}
		}
	}

	return files, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// extractFromFile parses a single .mg file and extracts DATA facts.
func extractFromFile(path string) ([]CorpusEntry, error) {
	// Try embedded content first
	embeddedPath := strings.TrimPrefix(path, "internal/core/defaults/")
	content, err := core.GetDefaultContent(embeddedPath)
	if err != nil {
		// Fall back to filesystem
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read file: %w", readErr)
		}
		content = string(data)
	}

	// Parse the content using the Mangle parser
	facts, err := core.ParseFactsFromString(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}

	var entries []CorpusEntry
	sourceFile := filepath.Base(path)

	for _, fact := range facts {
		entry, ok := factToCorpusEntry(fact, sourceFile)
		if ok {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// factToCorpusEntry converts a Mangle fact to a corpus entry if it's extractable.
func factToCorpusEntry(fact core.Fact, sourceFile string) (CorpusEntry, bool) {
	// Predicates we want to extract and their embedding strategy
	switch fact.Predicate {
	case "intent_definition":
		// intent_definition("How many files?", /stats, "count")
		// Embed the sentence (first argument)
		if len(fact.Args) >= 3 {
			sentence := argToString(fact.Args[0])
			verb := argToString(fact.Args[1])
			target := argToString(fact.Args[2])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: sentence,
				Verb:        verb,
				Target:      target,
				SourceFile:  sourceFile,
			}, true
		}

	case "intent_category":
		// intent_category("How many files?", /query)
		// Embed the sentence with category context
		if len(fact.Args) >= 2 {
			sentence := argToString(fact.Args[0])
			category := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: sentence,
				Category:    category,
				SourceFile:  sourceFile,
			}, true
		}

	case "verb_synonym":
		// verb_synonym(/review, "audit")
		// Embed the synonym (second argument)
		if len(fact.Args) >= 2 {
			verb := argToString(fact.Args[0])
			synonym := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: synonym,
				Verb:        verb,
				SourceFile:  sourceFile,
			}, true
		}

	case "verb_pattern":
		// verb_pattern(/review, "(?i)review.*code")
		// Embed the pattern (useful for fuzzy matching)
		if len(fact.Args) >= 2 {
			verb := argToString(fact.Args[0])
			pattern := argToString(fact.Args[1])
			// Clean up regex for embedding
			cleanPattern := cleanRegexForEmbedding(pattern)
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: cleanPattern,
				Verb:        verb,
				SourceFile:  sourceFile,
			}, true
		}

	case "verb_def":
		// verb_def(/review, /query, /reviewer, 100)
		// Construct searchable text: "review query reviewer"
		if len(fact.Args) >= 3 {
			verb := argToString(fact.Args[0])
			category := argToString(fact.Args[1])
			shard := argToString(fact.Args[2])
			text := fmt.Sprintf("%s %s %s", cleanAtom(verb), cleanAtom(category), cleanAtom(shard))
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: text,
				Verb:        verb,
				Category:    category,
				SourceFile:  sourceFile,
			}, true
		}

	case "verb_composition":
		// verb_composition(/review, /fix, "sequential", 95)
		// Construct: "review then fix"
		if len(fact.Args) >= 3 {
			verb1 := argToString(fact.Args[0])
			verb2 := argToString(fact.Args[1])
			relation := argToString(fact.Args[2])
			var text string
			switch relation {
			case "sequential":
				text = fmt.Sprintf("%s then %s", cleanAtom(verb1), cleanAtom(verb2))
			case "parallel":
				text = fmt.Sprintf("%s and %s", cleanAtom(verb1), cleanAtom(verb2))
			case "conditional":
				text = fmt.Sprintf("if %s then %s", cleanAtom(verb1), cleanAtom(verb2))
			case "fallback":
				text = fmt.Sprintf("%s or fallback to %s", cleanAtom(verb1), cleanAtom(verb2))
			default:
				text = fmt.Sprintf("%s %s %s", cleanAtom(verb1), relation, cleanAtom(verb2))
			}
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: text,
				Verb:        verb1,
				Target:      verb2,
				SourceFile:  sourceFile,
			}, true
		}

	case "step_connector":
		// step_connector("first", "sequential_start", /true)
		// Embed the connector word
		if len(fact.Args) >= 2 {
			connector := argToString(fact.Args[0])
			connType := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: connector,
				Category:    connType,
				SourceFile:  sourceFile,
			}, true
		}

	case "completion_marker":
		// completion_marker("done", "completion")
		if len(fact.Args) >= 2 {
			marker := argToString(fact.Args[0])
			markerType := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: marker,
				Category:    markerType,
				SourceFile:  sourceFile,
			}, true
		}

	case "pronoun_ref":
		// pronoun_ref("it", "previous_target")
		if len(fact.Args) >= 2 {
			pronoun := argToString(fact.Args[0])
			resolution := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: pronoun,
				Category:    resolution,
				SourceFile:  sourceFile,
			}, true
		}

	case "constraint_marker":
		// constraint_marker("but not", "exclusion")
		if len(fact.Args) >= 2 {
			marker := argToString(fact.Args[0])
			constraintType := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: marker,
				Category:    constraintType,
				SourceFile:  sourceFile,
			}, true
		}

	case "iterative_marker":
		// iterative_marker("each", "collection")
		if len(fact.Args) >= 2 {
			marker := argToString(fact.Args[0])
			iterType := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: marker,
				Category:    iterType,
				SourceFile:  sourceFile,
			}, true
		}

	case "priority_marker":
		// priority_marker("urgent", "high")
		if len(fact.Args) >= 2 {
			marker := argToString(fact.Args[0])
			level := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: marker,
				Category:    level,
				SourceFile:  sourceFile,
			}, true
		}

	case "verification_marker":
		// verification_marker("make sure", "verification_required")
		if len(fact.Args) >= 2 {
			marker := argToString(fact.Args[0])
			verType := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: marker,
				Category:    verType,
				SourceFile:  sourceFile,
			}, true
		}

	case "build_phase_type":
		// build_phase_type(/scaffold, 10)
		if len(fact.Args) >= 1 {
			phase := argToString(fact.Args[0])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: cleanAtom(phase),
				Category:    "build_phase",
				SourceFile:  sourceFile,
			}, true
		}

	case "phase_synonym":
		// phase_synonym(/scaffold, "setup")
		if len(fact.Args) >= 2 {
			phase := argToString(fact.Args[0])
			synonym := argToString(fact.Args[1])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: synonym,
				Verb:        phase,
				SourceFile:  sourceFile,
			}, true
		}

	case "layer_priority":
		// layer_priority(/scaffold, 10)
		if len(fact.Args) >= 1 {
			layer := argToString(fact.Args[0])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: cleanAtom(layer),
				Category:    "layer",
				SourceFile:  sourceFile,
			}, true
		}

	case "multistep_pattern", "multistep_keyword", "multistep_verb_pair":
		// Generic handling for multistep predicates
		if len(fact.Args) >= 1 {
			text := argToString(fact.Args[0])
			return CorpusEntry{
				Predicate:   fact.Predicate,
				TextContent: text,
				Category:    "multistep",
				SourceFile:  sourceFile,
			}, true
		}
	}

	return CorpusEntry{}, false
}

// argToString converts a fact argument to a string.
func argToString(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case core.MangleAtom:
		return string(v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// cleanAtom removes the leading / from Mangle atoms.
func cleanAtom(s string) string {
	return strings.TrimPrefix(s, "/")
}

// cleanRegexForEmbedding removes regex metacharacters for embedding.
func cleanRegexForEmbedding(pattern string) string {
	// Remove common regex constructs
	pattern = strings.ReplaceAll(pattern, "(?i)", "")
	pattern = strings.ReplaceAll(pattern, ".*", " ")
	pattern = strings.ReplaceAll(pattern, ".+", " ")
	pattern = strings.ReplaceAll(pattern, "\\s+", " ")
	pattern = strings.ReplaceAll(pattern, "\\s*", " ")
	pattern = strings.ReplaceAll(pattern, "(", "")
	pattern = strings.ReplaceAll(pattern, ")", "")
	pattern = strings.ReplaceAll(pattern, "|", " or ")
	pattern = strings.ReplaceAll(pattern, "^", "")
	pattern = strings.ReplaceAll(pattern, "$", "")
	pattern = strings.TrimSpace(pattern)
	return pattern
}

// createDatabase creates the SQLite database with schema.
func createDatabase() (*sql.DB, error) {
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

	// Create schema
	schema := `
		CREATE TABLE corpus_embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			predicate TEXT NOT NULL,
			text_content TEXT NOT NULL,
			verb TEXT,
			target TEXT,
			category TEXT,
			source_file TEXT NOT NULL,
			embedding BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_predicate ON corpus_embeddings(predicate);
		CREATE INDEX idx_verb ON corpus_embeddings(verb);
		CREATE INDEX idx_category ON corpus_embeddings(category);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Try to create sqlite-vec virtual table
	vecSchema := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_corpus USING vec0(
			embedding float[%d],
			content TEXT,
			predicate TEXT,
			verb TEXT
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

// generateAndStoreEmbeddings generates embeddings in batches and stores them.
func generateAndStoreEmbeddings(ctx context.Context, engine embedding.EmbeddingEngine, db *sql.DB, entries []CorpusEntry) error {
	total := len(entries)
	processed := 0

	// Prepare statements
	insertStmt, err := db.Prepare(`
		INSERT INTO corpus_embeddings (predicate, text_content, verb, target, category, source_file, embedding)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer insertStmt.Close()

	// Check if vec_corpus exists
	var vecAvailable bool
	row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='vec_corpus'")
	if err := row.Scan(new(string)); err == nil {
		vecAvailable = true
	}

	var vecStmt *sql.Stmt
	if vecAvailable {
		vecStmt, err = db.Prepare(`INSERT INTO vec_corpus (embedding, content, predicate, verb) VALUES (?, ?, ?, ?)`)
		if err != nil {
			fmt.Printf("  WARNING: Failed to prepare vec statement: %v\n", err)
			vecAvailable = false
		} else {
			defer vecStmt.Close()
		}
	}

	// Process in batches
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
		batch := entries[i:end]

		// Collect texts for batch embedding
		texts := make([]string, len(batch))
		for j, entry := range batch {
			texts[j] = entry.TextContent
		}

		// Generate embeddings
		taskType := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
		var embeddings [][]float32
		var err error
		if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok && taskType != "" {
			embeddings, err = batchAware.EmbedBatchWithTask(ctx, texts, taskType)
		} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
			embeddings = make([][]float32, len(texts))
			for j, text := range texts {
				vec, embedErr := taskAware.EmbedWithTask(ctx, text, taskType)
				if embedErr != nil {
					return fmt.Errorf("failed to embed corpus entry %d: %w", i+j, embedErr)
				}
				if len(vec) == 0 {
					return fmt.Errorf("empty embedding for corpus entry %d", i+j)
				}
				embeddings[j] = vec
			}
		} else {
			embeddings, err = engine.EmbedBatch(ctx, texts)
		}
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for batch %d: %w", i/batchSize, err)
		}

		// Store each entry
		for j, entry := range batch {
			embeddingBlob := encodeFloat32Slice(embeddings[j])

			_, err := insertStmt.Exec(
				entry.Predicate,
				entry.TextContent,
				nullableString(entry.Verb),
				nullableString(entry.Target),
				nullableString(entry.Category),
				entry.SourceFile,
				embeddingBlob,
			)
			if err != nil {
				return fmt.Errorf("failed to insert entry: %w", err)
			}

			// Also insert into vec_corpus if available
			if vecAvailable {
				if _, err := vecStmt.Exec(embeddingBlob, entry.TextContent, entry.Predicate, nullableString(entry.Verb)); err != nil {
					// Log but don't fail - main table is more important
					fmt.Printf("  WARNING: Failed to insert into vec_corpus: %v\n", err)
				}
			}
		}

		processed += len(batch)
		fmt.Printf("\r  Generating embeddings... %d/%d", processed, total)
	}
	fmt.Println() // New line after progress

	return nil
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
func printSummary(db *sql.DB) {
	fmt.Println()
	fmt.Println("--- Summary ---")

	// Total entries
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM corpus_embeddings").Scan(&total); err == nil {
		fmt.Printf("  Total entries: %d\n", total)
	}

	// Entries by predicate
	rows, err := db.Query(`
		SELECT predicate, COUNT(*) as cnt
		FROM corpus_embeddings
		GROUP BY predicate
		ORDER BY cnt DESC
	`)
	if err == nil {
		defer rows.Close()
		fmt.Println("  By predicate:")
		for rows.Next() {
			var pred string
			var cnt int
			if err := rows.Scan(&pred, &cnt); err == nil {
				fmt.Printf("    %-25s %d\n", pred, cnt)
			}
		}
	}

	// Entries by source file
	rows, err = db.Query(`
		SELECT source_file, COUNT(*) as cnt
		FROM corpus_embeddings
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
				fmt.Printf("    %-25s %d\n", file, cnt)
			}
		}
	}

	// Check vec_corpus
	var vecCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM vec_corpus").Scan(&vecCount); err == nil {
		fmt.Printf("  vec_corpus entries: %d\n", vecCount)
	}

	// File size
	if info, err := os.Stat(outputPath); err == nil {
		fmt.Printf("  Database size: %.2f MB\n", float64(info.Size())/(1024*1024))
	}
}

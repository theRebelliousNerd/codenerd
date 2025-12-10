package prompt

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	_ "github.com/mattn/go-sqlite3"
)

func TestEnsurePromptAtomsTable(t *testing.T) {
	// Create temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_knowledge.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// Test EnsureSchema
	loader := NewAtomLoader(nil)
	if err := loader.EnsureSchema(context.Background(), db); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='prompt_atoms'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 prompt_atoms table, got %d", count)
	}
}

func TestLoadAgentPromptsUnifiedStorage(t *testing.T) {
	// Create temp workspace
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	shardsDir := filepath.Join(nerdDir, "shards")
	agentsDir := filepath.Join(nerdDir, "agents", "testAgent")

	// Create directories
	if err := os.MkdirAll(shardsDir, 0755); err != nil {
		t.Fatalf("Failed to create shards dir: %v", err)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	// Create a knowledge DB (must execute something to create the file)
	kbPath := filepath.Join(shardsDir, "testagent_knowledge.db")
	db, err := sql.Open("sqlite3", kbPath)
	if err != nil {
		t.Fatalf("Failed to create knowledge DB: %v", err)
	}
	// Execute something to actually create the file
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS knowledge_atoms (id INTEGER PRIMARY KEY)")
	if err != nil {
		db.Close()
		t.Fatalf("Failed to initialize knowledge DB: %v", err)
	}
	db.Close()

	// Create a test prompts.yaml
	promptsYAML := `- id: test/atom
  category: identity
  priority: 100
  is_mandatory: true
  content: "This is a test atom"
`
	promptsPath := filepath.Join(agentsDir, "prompts.yaml")
	if err := os.WriteFile(promptsPath, []byte(promptsYAML), 0644); err != nil {
		t.Fatalf("Failed to write prompts.yaml: %v", err)
	}

	// Test LoadAgentPrompts
	ctx := context.Background()
	count, err := LoadAgentPrompts(ctx, "testAgent", nerdDir, nil)
	if err != nil {
		t.Fatalf("LoadAgentPrompts failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 atom loaded, got %d", count)
	}

	// Verify atom was stored in knowledge DB
	db, err = sql.Open("sqlite3", kbPath)
	if err != nil {
		t.Fatalf("Failed to open knowledge DB: %v", err)
	}
	defer db.Close()

	var atomCount int
	err = db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&atomCount)
	if err != nil {
		t.Fatalf("Failed to query prompt_atoms: %v", err)
	}

	if atomCount != 1 {
		t.Errorf("Expected 1 atom in DB, got %d", atomCount)
	}

	// Verify atom content
	var atomID, content string
	err = db.QueryRow("SELECT atom_id, content FROM prompt_atoms WHERE atom_id = ?", "test/atom").Scan(&atomID, &content)
	if err != nil {
		t.Fatalf("Failed to query atom: %v", err)
	}

	if atomID != "test/atom" {
		t.Errorf("Expected atom_id 'test/atom', got '%s'", atomID)
	}

	if content != "This is a test atom" {
		t.Errorf("Expected content 'This is a test atom', got '%s'", content)
	}

	t.Logf("Prompts successfully loaded into unified knowledge DB at %s", kbPath)
}

// ============================================================================
// MOCK EMBEDDING ENGINE
// ============================================================================

// mockEmbeddingEngine is a test double for embedding.EmbeddingEngine.
type mockEmbeddingEngine struct {
	dimensions    int
	embedCount    int
	batchCount    int
	shouldFail    bool
	failOnBatch   bool
	returnedVecs  [][]float32
}

func newMockEmbeddingEngine(dims int) *mockEmbeddingEngine {
	return &mockEmbeddingEngine{
		dimensions: dims,
	}
}

func (m *mockEmbeddingEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.shouldFail {
		return nil, errors.New("mock embedding failure")
	}
	m.embedCount++
	return m.generateVector(text), nil
}

func (m *mockEmbeddingEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if m.failOnBatch {
		return nil, errors.New("mock batch embedding failure")
	}
	m.batchCount++

	results := make([][]float32, len(texts))
	for i, text := range texts {
		results[i] = m.generateVector(text)
	}
	m.returnedVecs = results
	return results, nil
}

func (m *mockEmbeddingEngine) Dimensions() int {
	return m.dimensions
}

func (m *mockEmbeddingEngine) Name() string {
	return "mock-embedding-engine"
}

// generateVector creates a deterministic vector based on the text hash.
func (m *mockEmbeddingEngine) generateVector(text string) []float32 {
	vec := make([]float32, m.dimensions)
	// Create a simple deterministic vector based on text length and first char
	seed := float32(len(text))
	if len(text) > 0 {
		seed += float32(text[0])
	}
	for i := range vec {
		vec[i] = (seed + float32(i)) / 1000.0
	}
	return vec
}

// ============================================================================
// SYNC EMBEDDED TO SQLITE TESTS
// ============================================================================

func TestSyncEmbeddedToSQLite(t *testing.T) {
	tests := []struct {
		name        string
		setupEngine func() *mockEmbeddingEngine
		wantErr     bool
		errContains string
	}{
		{
			name: "nil engine returns error",
			setupEngine: func() *mockEmbeddingEngine {
				return nil
			},
			wantErr:     true,
			errContains: "embedding engine is required",
		},
		{
			name: "batch embedding failure propagates",
			setupEngine: func() *mockEmbeddingEngine {
				engine := newMockEmbeddingEngine(768)
				engine.failOnBatch = true
				return engine
			},
			wantErr:     true,
			errContains: "failed to generate batch embeddings",
		},
		{
			name: "successful sync with mock engine",
			setupEngine: func() *mockEmbeddingEngine {
				return newMockEmbeddingEngine(768)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			dbPath := filepath.Join(tmpDir, "test_corpus.db")
			ctx := context.Background()

			var engine *mockEmbeddingEngine
			if tt.setupEngine != nil {
				engine = tt.setupEngine()
			}

			// For nil engine test, pass nil directly
			var engineInterface interface{}
			if engine != nil {
				engineInterface = engine
			}

			var err error
			if engineInterface == nil {
				err = SyncEmbeddedToSQLite(ctx, dbPath, nil)
			} else {
				err = SyncEmbeddedToSQLite(ctx, dbPath, engine)
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("SyncEmbeddedToSQLite() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("SyncEmbeddedToSQLite() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("SyncEmbeddedToSQLite() unexpected error: %v", err)
				return
			}

			// Verify database was created and has content
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				t.Errorf("Database file was not created at %s", dbPath)
				return
			}

			// Verify atoms were stored
			db, dbErr := sql.Open("sqlite3", dbPath)
			if dbErr != nil {
				t.Fatalf("Failed to open created database: %v", dbErr)
			}
			defer db.Close()

			var atomCount int
			if scanErr := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&atomCount); scanErr != nil {
				t.Fatalf("Failed to count atoms: %v", scanErr)
			}

			// Get expected count from embedded corpus
			corpus, corpusErr := LoadEmbeddedCorpus()
			if corpusErr != nil {
				t.Fatalf("Failed to load embedded corpus: %v", corpusErr)
			}

			expectedCount := corpus.Count()
			if atomCount != expectedCount {
				t.Errorf("Atom count mismatch: got %d, want %d", atomCount, expectedCount)
			}

			// Verify embeddings were stored
			var embeddingCount int
			if scanErr := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE embedding IS NOT NULL").Scan(&embeddingCount); scanErr != nil {
				t.Fatalf("Failed to count embeddings: %v", scanErr)
			}

			if embeddingCount != expectedCount {
				t.Errorf("Embedding count mismatch: got %d, want %d", embeddingCount, expectedCount)
			}

			t.Logf("Successfully synced %d atoms with embeddings", atomCount)
		})
	}
}

func TestSyncEmbeddedToSQLite_Idempotency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "idempotent_test.db")
	ctx := context.Background()

	engine := newMockEmbeddingEngine(768)

	// First sync
	if err := SyncEmbeddedToSQLite(ctx, dbPath, engine); err != nil {
		t.Fatalf("First sync failed: %v", err)
	}
	firstBatchCount := engine.batchCount

	// Get atom count after first sync
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	var firstCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&firstCount); err != nil {
		db.Close()
		t.Fatalf("Failed to count atoms: %v", err)
	}
	db.Close()

	// Second sync (should be idempotent)
	if err := SyncEmbeddedToSQLite(ctx, dbPath, engine); err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}
	secondBatchCount := engine.batchCount

	// Verify no additional embeddings were generated (batch count unchanged)
	if secondBatchCount != firstBatchCount {
		t.Errorf("Idempotency violated: batch count changed from %d to %d", firstBatchCount, secondBatchCount)
	}

	// Verify atom count unchanged
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var secondCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&secondCount); err != nil {
		t.Fatalf("Failed to count atoms: %v", err)
	}

	if secondCount != firstCount {
		t.Errorf("Atom count changed: got %d, want %d", secondCount, firstCount)
	}

	t.Logf("Idempotency verified: %d atoms, batch called %d times total", secondCount, secondBatchCount)
}

func TestSyncEmbeddedToSQLite_ContextTagsPersisted(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "tags_test.db")
	ctx := context.Background()

	engine := newMockEmbeddingEngine(768)

	if err := SyncEmbeddedToSQLite(ctx, dbPath, engine); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify context tags table has entries (if embedded atoms have tags)
	var tagCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM atom_context_tags").Scan(&tagCount); err != nil {
		t.Fatalf("Failed to count tags: %v", err)
	}

	t.Logf("Synced %d context tags", tagCount)

	// Verify tags have correct structure
	rows, err := db.Query("SELECT DISTINCT dimension FROM atom_context_tags LIMIT 20")
	if err != nil {
		t.Fatalf("Failed to query dimensions: %v", err)
	}
	defer rows.Close()

	var dimensions []string
	for rows.Next() {
		var dim string
		if err := rows.Scan(&dim); err != nil {
			t.Fatalf("Failed to scan dimension: %v", err)
		}
		dimensions = append(dimensions, dim)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("Row iteration error: %v", err)
	}

	t.Logf("Found dimensions: %v", dimensions)
}

func TestGetTextForEmbedding(t *testing.T) {
	tests := []struct {
		name        string
		atom        *PromptAtom
		wantContent string
	}{
		{
			name: "uses description when available",
			atom: &PromptAtom{
				ID:          "test/desc",
				Description: "This is the description",
				Content:     "This is the full content that should not be used",
			},
			wantContent: "This is the description",
		},
		{
			name: "uses content when description empty",
			atom: &PromptAtom{
				ID:      "test/nodesc",
				Content: "This content should be used",
			},
			wantContent: "This content should be used",
		},
		{
			name: "truncates content at 500 chars",
			atom: &PromptAtom{
				ID:      "test/long",
				Content: string(make([]byte, 600)), // 600 zero bytes
			},
			wantContent: string(make([]byte, 500)), // Truncated to 500
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTextForEmbedding(tt.atom)
			if diff := cmp.Diff(tt.wantContent, got); diff != "" {
				t.Errorf("getTextForEmbedding() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadExistingHashes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "hashes_test.db")
	ctx := context.Background()

	// Create database with some atoms
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	loader := NewAtomLoader(nil)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		db.Close()
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	// Insert test atoms
	testAtoms := []struct {
		id   string
		hash string
	}{
		{"atom/one", "hash1"},
		{"atom/two", "hash2"},
		{"atom/three", "hash3"},
	}

	for _, a := range testAtoms {
		_, err := db.Exec(`
			INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category)
			VALUES (?, 'test content', 10, ?, 'identity')`,
			a.id, a.hash)
		if err != nil {
			db.Close()
			t.Fatalf("Failed to insert test atom: %v", err)
		}
	}
	db.Close()

	// Re-open and test loadExistingHashes
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db.Close()

	hashes, err := loadExistingHashes(ctx, db)
	if err != nil {
		t.Fatalf("loadExistingHashes() error: %v", err)
	}

	if len(hashes) != len(testAtoms) {
		t.Errorf("Hash count mismatch: got %d, want %d", len(hashes), len(testAtoms))
	}

	for _, a := range testAtoms {
		got, exists := hashes[a.id]
		if !exists {
			t.Errorf("Missing hash for atom %s", a.id)
			continue
		}
		if got != a.hash {
			t.Errorf("Hash mismatch for atom %s: got %s, want %s", a.id, got, a.hash)
		}
	}
}

func TestLoadExistingHashes_EmptyTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty_hashes_test.db")
	ctx := context.Background()

	// Create database with schema but no atoms
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	loader := NewAtomLoader(nil)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		db.Close()
		t.Fatalf("Failed to ensure schema: %v", err)
	}
	db.Close()

	// Re-open and test
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db.Close()

	hashes, err := loadExistingHashes(ctx, db)
	if err != nil {
		t.Fatalf("loadExistingHashes() error: %v", err)
	}

	if len(hashes) != 0 {
		t.Errorf("Expected empty hash map, got %d entries", len(hashes))
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

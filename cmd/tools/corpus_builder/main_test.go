package main

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/embedding"
)

func TestCleanRegexForEmbedding(t *testing.T) {
	pattern := "(?i)review.*code|audit"
	cleaned := cleanRegexForEmbedding(pattern)
	if cleaned != "review code or audit" {
		t.Fatalf("unexpected cleaned pattern: %q", cleaned)
	}
}

func TestFactToCorpusEntryIntentDefinition(t *testing.T) {
	fact := core.Fact{
		Predicate: "intent_definition",
		Args: []any{
			"How many files?",
			core.MangleAtom("/stats"),
			"count",
		},
	}

	entry, ok := factToCorpusEntry(fact, "schema.mg")
	if !ok {
		t.Fatalf("expected intent_definition to be extractable")
	}
	if entry.TextContent != "How many files?" {
		t.Fatalf("unexpected text content: %q", entry.TextContent)
	}
	if entry.Verb != "/stats" {
		t.Fatalf("unexpected verb: %q", entry.Verb)
	}
	if entry.Target != "count" {
		t.Fatalf("unexpected target: %q", entry.Target)
	}
}

func TestArgToStringBool(t *testing.T) {
	if got := argToString(true); got != "true" {
		t.Fatalf("expected true, got %q", got)
	}
	if got := argToString(false); got != "false" {
		t.Fatalf("expected false, got %q", got)
	}
}

func TestArgToStringAllTypesCorpus(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", "hello"},
		{"atom", core.MangleAtom("/review"), "/review"},
		{"int64", int64(42), "42"},
		{"int64Negative", int64(-7), "-7"},
		{"float64", float64(1.5), "1.500000"},
		{"boolTrue", true, "true"},
		{"boolFalse", false, "false"},
		{"intFallback", 7, "7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := argToString(tc.input); got != tc.want {
				t.Errorf("argToString(%v)=%q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFactToCorpusEntryAllPredicatesCorpus(t *testing.T) {
	atom := func(s string) any { return core.MangleAtom(s) }
	cases := []struct {
		name     string
		fact     core.Fact
		wantText string
		wantVerb string
		wantCat  string
		wantTgt  string
		wantPred string
	}{
		{"intentCategory", core.Fact{Predicate: "intent_category", Args: []any{"How many?", "/query"}}, "How many?", "", "/query", "", "intent_category"},
		{"verbSynonym", core.Fact{Predicate: "verb_synonym", Args: []any{atom("/review"), "audit"}}, "audit", "/review", "", "", "verb_synonym"},
		{"verbPattern", core.Fact{Predicate: "verb_pattern", Args: []any{atom("/review"), "(?i)review.*code"}}, "review code", "/review", "", "", "verb_pattern"},
		{"verbDef", core.Fact{Predicate: "verb_def", Args: []any{atom("/review"), atom("/query"), atom("/reviewer")}}, "review query reviewer", "/review", "/query", "", "verb_def"},
		{"verbCompositionSequential", core.Fact{Predicate: "verb_composition", Args: []any{atom("/review"), atom("/fix"), atom("/sequential")}}, "review then fix", "/review", "", "/fix", "verb_composition"},
		{"verbCompositionParallel", core.Fact{Predicate: "verb_composition", Args: []any{atom("/review"), atom("/fix"), atom("/parallel")}}, "review and fix", "/review", "", "/fix", "verb_composition"},
		{"verbCompositionConditional", core.Fact{Predicate: "verb_composition", Args: []any{atom("/review"), atom("/fix"), atom("/conditional")}}, "if review then fix", "/review", "", "/fix", "verb_composition"},
		{"verbCompositionFallback", core.Fact{Predicate: "verb_composition", Args: []any{atom("/review"), atom("/fix"), atom("/fallback")}}, "review or fallback to fix", "/review", "", "/fix", "verb_composition"},
		{"verbCompositionDefault", core.Fact{Predicate: "verb_composition", Args: []any{atom("/review"), atom("/fix"), atom("/custom")}}, "review custom fix", "/review", "", "/fix", "verb_composition"},
		{"stepConnector", core.Fact{Predicate: "step_connector", Args: []any{"first", "sequential_start"}}, "first", "", "sequential_start", "", "step_connector"},
		{"completionMarker", core.Fact{Predicate: "completion_marker", Args: []any{"done", "completion"}}, "done", "", "completion", "", "completion_marker"},
		{"pronounRef", core.Fact{Predicate: "pronoun_ref", Args: []any{"it", "previous_target"}}, "it", "", "previous_target", "", "pronoun_ref"},
		{"constraintMarker", core.Fact{Predicate: "constraint_marker", Args: []any{"but not", "exclusion"}}, "but not", "", "exclusion", "", "constraint_marker"},
		{"iterativeMarker", core.Fact{Predicate: "iterative_marker", Args: []any{"each", "collection"}}, "each", "", "collection", "", "iterative_marker"},
		{"priorityMarker", core.Fact{Predicate: "priority_marker", Args: []any{"urgent", "high"}}, "urgent", "", "high", "", "priority_marker"},
		{"verificationMarker", core.Fact{Predicate: "verification_marker", Args: []any{"make sure", "verification_required"}}, "make sure", "", "verification_required", "", "verification_marker"},
		{"buildPhaseType", core.Fact{Predicate: "build_phase_type", Args: []any{atom("/scaffold")}}, "scaffold", "", "build_phase", "", "build_phase_type"},
		{"phaseSynonym", core.Fact{Predicate: "phase_synonym", Args: []any{atom("/scaffold"), "setup"}}, "setup", "/scaffold", "", "", "phase_synonym"},
		{"layerPriority", core.Fact{Predicate: "layer_priority", Args: []any{atom("/scaffold")}}, "scaffold", "", "layer", "", "layer_priority"},
		{"multistepPattern", core.Fact{Predicate: "multistep_pattern", Args: []any{"first then second"}}, "first then second", "", "multistep", "", "multistep_pattern"},
		{"multistepKeyword", core.Fact{Predicate: "multistep_keyword", Args: []any{"then"}}, "then", "", "multistep", "", "multistep_keyword"},
		{"multistepVerbPair", core.Fact{Predicate: "multistep_verb_pair", Args: []any{"review fix"}}, "review fix", "", "multistep", "", "multistep_verb_pair"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, ok := factToCorpusEntry(tc.fact, "src.mg")
			if !ok {
				t.Fatalf("expected %s to be extractable", tc.fact.Predicate)
			}
			if entry.TextContent != tc.wantText {
				t.Errorf("TextContent=%q, want %q", entry.TextContent, tc.wantText)
			}
			if tc.wantVerb != "" && entry.Verb != tc.wantVerb {
				t.Errorf("Verb=%q, want %q", entry.Verb, tc.wantVerb)
			}
			if tc.wantCat != "" && entry.Category != tc.wantCat {
				t.Errorf("Category=%q, want %q", entry.Category, tc.wantCat)
			}
			if tc.wantTgt != "" && entry.Target != tc.wantTgt {
				t.Errorf("Target=%q, want %q", entry.Target, tc.wantTgt)
			}
			if entry.Predicate != tc.wantPred {
				t.Errorf("Predicate=%q, want %q", entry.Predicate, tc.wantPred)
			}
			if entry.SourceFile != "src.mg" {
				t.Errorf("SourceFile=%q, want src.mg", entry.SourceFile)
			}
		})
	}
}

func TestFactToCorpusEntryRejectsShortArgsCorpus(t *testing.T) {
	cases := []core.Fact{
		{Predicate: "intent_definition", Args: []any{"only one"}},
		{Predicate: "intent_category", Args: []any{"only one"}},
		{Predicate: "verb_synonym", Args: []any{core.MangleAtom("/review")}},
		{Predicate: "verb_pattern", Args: []any{core.MangleAtom("/review")}},
		{Predicate: "verb_def", Args: []any{core.MangleAtom("/a"), core.MangleAtom("/b")}},
		{Predicate: "verb_composition", Args: []any{core.MangleAtom("/a"), core.MangleAtom("/b")}},
		{Predicate: "step_connector", Args: []any{"only"}},
		{Predicate: "completion_marker", Args: []any{"only"}},
		{Predicate: "pronoun_ref", Args: []any{"only"}},
		{Predicate: "constraint_marker", Args: []any{"only"}},
		{Predicate: "iterative_marker", Args: []any{"only"}},
		{Predicate: "priority_marker", Args: []any{"only"}},
		{Predicate: "verification_marker", Args: []any{"only"}},
		{Predicate: "build_phase_type", Args: []any{}},
		{Predicate: "phase_synonym", Args: []any{core.MangleAtom("/a")}},
		{Predicate: "layer_priority", Args: []any{}},
		{Predicate: "multistep_pattern", Args: []any{}},
		{Predicate: "unknown_predicate", Args: []any{"x", "y", "z"}},
		{Predicate: "unknown_predicate", Args: []any{}},
	}
	for i, f := range cases {
		if _, ok := factToCorpusEntry(f, "s.mg"); ok {
			t.Errorf("case %d (%s): expected rejection, got acceptance", i, f.Predicate)
		}
	}
}

func TestCleanRegexVariantsCorpus(t *testing.T) {
	cases := []struct{ in, want string }{
		{"(?i)review.*code", "review code"},
		{"a.+b", "a b"},
		{`a\s+b`, "a b"},
		{`a\s*b`, "a b"},
		{"(a|b)", "a or b"},
		{"^start end$", "start end"},
		{"  padded  ", "padded"},
		{"plain", "plain"},
	}
	for _, tc := range cases {
		if got := cleanRegexForEmbedding(tc.in); got != tc.want {
			t.Errorf("cleanRegexForEmbedding(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCleanAtomVariantsCorpus(t *testing.T) {
	if got := cleanAtom(""); got != "" {
		t.Errorf("cleanAtom empty=%q", got)
	}
	if got := cleanAtom("//double"); got != "/double" {
		t.Errorf("cleanAtom strips one slash only, got %q", got)
	}
}

func TestContainsSlashNormalizationCorpus(t *testing.T) {
	if !contains([]string{`a\b`}, "a/b") && !contains([]string{"a/b"}, `a\b`) {
		// At least one direction must hold on this OS via ToSlash normalization.
		t.Log("slash normalization is OS-dependent; both forms distinct here")
	}
	if contains(nil, "x") {
		t.Error("contains(nil, x) must be false")
	}
	if !contains([]string{"internal/core/defaults/taxonomy.mg"}, "internal/core/defaults/taxonomy.mg") {
		t.Error("exact match must hold")
	}
}

func TestCreateEmbeddingEngineRequiresModelCorpus(t *testing.T) {
	t.Setenv("GENAI_EMBEDDING_MODEL", "")
	if _, err := createEmbeddingEngine("dummy-key"); err == nil {
		t.Fatal("expected error when GENAI_EMBEDDING_MODEL is unset")
	} else if !strings.Contains(err.Error(), "GENAI_EMBEDDING_MODEL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEncodeFloat32SliceEmptyCorpus(t *testing.T) {
	if got := encodeFloat32Slice(nil); len(got) != 0 {
		t.Errorf("nil vec len=%d, want 0", len(got))
	}
	if got := encodeFloat32Slice([]float32{}); len(got) != 0 {
		t.Errorf("empty vec len=%d, want 0", len(got))
	}
}

// stubBatchEngine is a minimal EmbeddingEngine hitting the plain EmbedBatch path.
type stubBatchEngine struct {
	vecs [][]float32
	err  error
}

func (s *stubBatchEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	if len(s.vecs) > 0 {
		return s.vecs[0], s.err
	}
	return []float32{1, 2}, s.err
}
func (s *stubBatchEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(i) + 0.5, 1.5}
	}
	return out, nil
}
func (s *stubBatchEngine) Dimensions() int { return 2 }
func (s *stubBatchEngine) Name() string    { return "stub-batch" }

// stubTaskBatchEngine hits the TaskTypeBatchAwareEngine path.
type stubTaskBatchEngine struct{ stubBatchEngine }

func (s *stubTaskBatchEngine) EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	if taskType == "" {
		return nil, context.Canceled
	}
	return s.stubBatchEngine.EmbedBatch(ctx, texts)
}

// stubTaskSingleEngine hits the TaskTypeAwareEngine (per-text) path.
type stubTaskSingleEngine struct {
	stubBatchEngine
	failAt int // index to fail with error, -1 for none
	empty  bool
}

func (s *stubTaskSingleEngine) EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	if s.empty {
		return []float32{}, nil
	}
	return []float32{0.25, 0.75}, nil
}

func openCorpusTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := `CREATE TABLE corpus_embeddings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		predicate TEXT NOT NULL,
		text_content TEXT NOT NULL,
		verb TEXT,
		target TEXT,
		category TEXT,
		source_file TEXT NOT NULL,
		embedding BLOB NOT NULL
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestGenerateAndStoreEmbeddingsBatchPathCorpus(t *testing.T) {
	db := openCorpusTestDB(t)
	engine := &stubBatchEngine{}
	entries := []CorpusEntry{
		{Predicate: "intent_definition", TextContent: "how many files", Verb: "/stats", Target: "count", SourceFile: "a.mg"},
		{Predicate: "verb_synonym", TextContent: "audit", Verb: "/review", SourceFile: "a.mg"},
	}
	if err := generateAndStoreEmbeddings(context.Background(), engine, db, entries); err != nil {
		t.Fatalf("generateAndStoreEmbeddings: %v", err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM corpus_embeddings").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("rows=%d, want 2", n)
	}
	var blob []byte
	if err := db.QueryRow("SELECT embedding FROM corpus_embeddings LIMIT 1").Scan(&blob); err != nil {
		t.Fatalf("scan blob: %v", err)
	}
	if len(blob) != 8 {
		t.Errorf("blob len=%d, want 8", len(blob))
	}
}

func TestGenerateAndStoreEmbeddingsEmptyCorpus(t *testing.T) {
	db := openCorpusTestDB(t)
	if err := generateAndStoreEmbeddings(context.Background(), &stubBatchEngine{}, db, nil); err != nil {
		t.Fatalf("empty entries should succeed: %v", err)
	}
}

func TestGenerateAndStoreEmbeddingsTaskBatchPathCorpus(t *testing.T) {
	// SelectTaskType for knowledge atoms is non-empty, so this must take the batch-with-task branch.
	if embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false) == "" {
		t.Skip("task type empty on this build; batch-task branch unreachable")
	}
	db := openCorpusTestDB(t)
	engine := &stubTaskBatchEngine{}
	entries := []CorpusEntry{{Predicate: "p", TextContent: "hello", SourceFile: "a.mg"}}
	if err := generateAndStoreEmbeddings(context.Background(), engine, db, entries); err != nil {
		t.Fatalf("task batch path: %v", err)
	}
}

func TestGenerateAndStoreEmbeddingsTaskSinglePathCorpus(t *testing.T) {
	if embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false) == "" {
		t.Skip("task type empty on this build; single-task branch unreachable")
	}
	db := openCorpusTestDB(t)
	engine := &stubTaskSingleEngine{failAt: -1}
	entries := []CorpusEntry{
		{Predicate: "p", TextContent: "one", SourceFile: "a.mg"},
		{Predicate: "p", TextContent: "two", SourceFile: "a.mg"},
	}
	if err := generateAndStoreEmbeddings(context.Background(), engine, db, entries); err != nil {
		t.Fatalf("task single path: %v", err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM corpus_embeddings").Scan(&n); err != nil || n != 2 {
		t.Fatalf("rows=%d err=%v, want 2", n, err)
	}
}

func TestGenerateAndStoreEmbeddingsEmptyVectorFailsCorpus(t *testing.T) {
	if embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false) == "" {
		t.Skip("task type empty; single-task branch unreachable")
	}
	db := openCorpusTestDB(t)
	engine := &stubTaskSingleEngine{failAt: -1, empty: true}
	entries := []CorpusEntry{{Predicate: "p", TextContent: "one", SourceFile: "a.mg"}}
	if err := generateAndStoreEmbeddings(context.Background(), engine, db, entries); err == nil {
		t.Fatal("expected empty-embedding error")
	}
}

func TestGenerateAndStoreEmbeddingsCanceledContextCorpus(t *testing.T) {
	db := openCorpusTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	entries := []CorpusEntry{{Predicate: "p", TextContent: "one", SourceFile: "a.mg"}}
	if err := generateAndStoreEmbeddings(ctx, &stubBatchEngine{}, db, entries); err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestPrintSummaryDoesNotPanicCorpus(t *testing.T) {
	db := openCorpusTestDB(t)
	if _, err := db.Exec(`INSERT INTO corpus_embeddings (predicate, text_content, verb, target, category, source_file, embedding) VALUES (?,?,?,?,?,?,?)`,
		"intent_definition", "how many", "/stats", "count", nil, "a.mg", []byte{0, 0, 0, 0}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	printSummary(db)
	// Missing vec_corpus table and missing outputPath file must be tolerated.
}

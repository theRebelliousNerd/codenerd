package prompt_evolution

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/prompt"
)

// =============================================================================
// TYPE TESTS
// =============================================================================

func TestJudgeVerdict_IsFail(t *testing.T) {
	tests := []struct {
		name     string
		verdict  string
		expected bool
	}{
		{"FAIL verdict", "FAIL", true},
		{"PASS verdict", "PASS", false},
		{"empty verdict", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &JudgeVerdict{Verdict: tt.verdict}
			if got := v.IsFail(); got != tt.expected {
				t.Errorf("IsFail() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestJudgeVerdict_IsPass(t *testing.T) {
	tests := []struct {
		name     string
		verdict  string
		expected bool
	}{
		{"PASS verdict", "PASS", true},
		{"FAIL verdict", "FAIL", false},
		{"empty verdict", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &JudgeVerdict{Verdict: tt.verdict}
			if got := v.IsPass(); got != tt.expected {
				t.Errorf("IsPass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStrategy_TotalUses(t *testing.T) {
	s := &Strategy{
		SuccessCount: 5,
		FailureCount: 3,
	}
	if got := s.TotalUses(); got != 8 {
		t.Errorf("TotalUses() = %v, want 8", got)
	}
}

func TestGeneratedAtom_SuccessRate(t *testing.T) {
	tests := []struct {
		name         string
		usageCount   int
		successCount int
		expected     float64
	}{
		{"no usage returns 0.5", 0, 0, 0.5},
		{"100% success", 10, 10, 1.0},
		{"50% success", 10, 5, 0.5},
		{"0% success", 10, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ga := &GeneratedAtom{
				UsageCount:   tt.usageCount,
				SuccessCount: tt.successCount,
			}
			if got := ga.SuccessRate(); got != tt.expected {
				t.Errorf("SuccessRate() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGeneratedAtom_ShouldPromote(t *testing.T) {
	tests := []struct {
		name         string
		usageCount   int
		successCount int
		threshold    float64
		expected     bool
	}{
		{"too few uses", 2, 2, 0.7, false},
		{"below threshold", 5, 2, 0.7, false},
		{"at threshold", 10, 7, 0.7, true},
		{"above threshold", 10, 9, 0.7, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ga := &GeneratedAtom{
				UsageCount:   tt.usageCount,
				SuccessCount: tt.successCount,
			}
			if got := ga.ShouldPromote(tt.threshold); got != tt.expected {
				t.Errorf("ShouldPromote(%v) = %v, want %v", tt.threshold, got, tt.expected)
			}
		})
	}
}

func TestAllProblemTypes(t *testing.T) {
	types := AllProblemTypes()
	if len(types) != 16 {
		t.Errorf("AllProblemTypes() returned %d types, want 16", len(types))
	}

	// Verify expected types are present
	expectedTypes := map[ProblemType]bool{
		ProblemDebugging:       false,
		ProblemFeatureCreation: false,
		ProblemRefactoring:     false,
		ProblemTesting:         false,
	}

	for _, pt := range types {
		if _, ok := expectedTypes[pt]; ok {
			expectedTypes[pt] = true
		}
	}

	for pt, found := range expectedTypes {
		if !found {
			t.Errorf("Expected problem type %s not found in AllProblemTypes()", pt)
		}
	}
}

// =============================================================================
// PROBLEM CLASSIFIER TESTS
// =============================================================================

func TestProblemClassifier_Classify(t *testing.T) {
	classifier := NewProblemClassifier()

	tests := []struct {
		name        string
		taskRequest string
	}{
		{"debugging keyword", "fix the bug in user service"},
		{"feature keyword", "add a new login feature"},
		{"refactoring keyword", "refactor the database layer"},
		{"test keyword", "write tests for the API"},
		{"docs keyword", "document the configuration"},
		{"performance keyword", "optimize the query performance"},
		{"security keyword", "fix the security vulnerability"},
		{"unknown task", "do something amazing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, confidence := classifier.Classify(tt.taskRequest)
			// Verify we get a valid result and confidence
			if result == "" {
				t.Errorf("Classify(%q) returned empty problem type", tt.taskRequest)
			}
			if confidence < 0 || confidence > 1 {
				t.Errorf("Confidence should be in [0, 1], got %v", confidence)
			}
			t.Logf("Classify(%q) = %v (confidence: %.2f)", tt.taskRequest, result, confidence)
		})
	}
}

func TestProblemClassifier_ClassifyWithContext(t *testing.T) {
	classifier := NewProblemClassifier()

	// Test that shard type doesn't cause errors and returns valid results
	testCases := []struct {
		taskRequest string
		shardType   string
	}{
		{"do something", "/tester"},
		{"do something", "/reviewer"},
		{"do something", "/coder"},
		{"do something", ""},
	}

	for _, tc := range testCases {
		result, confidence := classifier.ClassifyWithContext(context.Background(), tc.taskRequest, tc.shardType)
		if result == "" {
			t.Errorf("ClassifyWithContext(%q, %q) returned empty problem type", tc.taskRequest, tc.shardType)
		}
		if confidence < 0 || confidence > 1 {
			t.Errorf("Confidence should be in [0, 1], got %v", confidence)
		}
		t.Logf("ClassifyWithContext(%q, %q) = %v (confidence: %.2f)", tc.taskRequest, tc.shardType, result, confidence)
	}
}

func TestGetProblemTypeDescription(t *testing.T) {
	desc := GetProblemTypeDescription(ProblemDebugging)
	if desc == "" {
		t.Error("Expected non-empty description for debugging")
	}

	desc = GetProblemTypeDescription(ProblemType("unknown"))
	if desc == "" {
		t.Error("Expected non-empty default description for unknown type")
	}
}

func TestProblemClassifier_EmptyInput(t *testing.T) {
	classifier := NewProblemClassifier()
	pt, conf := classifier.Classify("")
	if pt == "" {
		t.Fatal("expected default problem type for empty input")
	}
	if conf < 0 || conf > 1 {
		t.Fatalf("confidence out of range: %v", conf)
	}
	if conf > 0.5 {
		t.Fatalf("expected low confidence for empty input, got %v", conf)
	}
}

func TestProblemClassifier_HugeInput(t *testing.T) {
	classifier := NewProblemClassifier()
	huge := strings.Repeat("a", 1024*1024) + " fix the bug in service"
	start := time.Now()
	pt, conf := classifier.Classify(huge)
	elapsed := time.Since(start)
	if pt == "" {
		t.Fatal("expected classification for huge input")
	}
	if conf < 0 || conf > 1 {
		t.Fatalf("confidence out of range: %v", conf)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("classification too slow for huge input: %v", elapsed)
	}
}

func TestProblemClassifier_BinaryInput(t *testing.T) {
	classifier := NewProblemClassifier()
	input := string([]byte{0x00, 0xff, 0x01, 0x10, 0x7f, 0x41, 0x42})
	pt, conf := classifier.Classify(input)
	if pt == "" {
		t.Fatal("expected classification for binary input")
	}
	if conf < 0 || conf > 1 {
		t.Fatalf("confidence out of range: %v", conf)
	}
}

// =============================================================================
// EVOLVER CONFIG TESTS
// =============================================================================

func TestDefaultEvolverConfig(t *testing.T) {
	config := DefaultEvolverConfig()

	if config == nil {
		t.Fatal("DefaultEvolverConfig() returned nil")
	}

	// Verify reasonable defaults
	if config.MinFailuresForEvolution < 1 {
		t.Error("MinFailuresForEvolution should be at least 1")
	}
	if config.EvolutionInterval < time.Minute {
		t.Error("EvolutionInterval should be at least 1 minute")
	}
	if config.ConfidenceThreshold <= 0 || config.ConfidenceThreshold > 1 {
		t.Error("ConfidenceThreshold should be in (0, 1]")
	}
	if config.MaxAtomsPerEvolution < 1 {
		t.Error("MaxAtomsPerEvolution should be at least 1")
	}
}

// =============================================================================
// FEEDBACK COLLECTOR TESTS
// =============================================================================

func TestFeedbackCollector(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "feedback_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create feedback collector
	fc, err := NewFeedbackCollector(tempDir)
	if err != nil {
		t.Fatalf("NewFeedbackCollector failed: %v", err)
	}
	defer fc.Close()

	// Record an execution WITHOUT a verdict first (simulates initial recording)
	exec := &ExecutionRecord{
		TaskID:      "task-001",
		SessionID:   "session-001",
		ShardType:   "/coder",
		TaskRequest: "fix the bug",
		ProblemType: string(ProblemDebugging),
		Timestamp:   time.Now(),
		ExecutionResult: ExecutionResult{
			Success: false,
			Output:  "test failed",
		},
		// No Verdict yet - this is an unevaluated execution
	}

	if err := fc.Record(exec); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Get stats - failures are only counted when verdict is set and IsFail()
	total, failures := fc.GetStats()
	if total != 1 {
		t.Errorf("Expected 1 total execution, got %d", total)
	}
	if failures != 0 {
		t.Errorf("Expected 0 failures (no verdict yet), got %d", failures)
	}

	// Get unevaluated - should be 1 since we haven't set a verdict
	unevaluated, err := fc.GetUnevaluated(10)
	if err != nil {
		t.Fatalf("GetUnevaluated failed: %v", err)
	}
	if len(unevaluated) != 1 {
		t.Errorf("Expected 1 unevaluated, got %d", len(unevaluated))
	}

	// Update verdict - this should mark it as evaluated
	verdict := &JudgeVerdict{
		Verdict:     "FAIL",
		Explanation: "Test assertion failed",
		Category:    CategoryLogicError,
		Confidence:  0.9,
		TaskID:      "task-001",
	}
	if err := fc.UpdateVerdict("task-001", verdict); err != nil {
		t.Fatalf("UpdateVerdict failed: %v", err)
	}

	// Verify verdict was stored - should be 0 unevaluated now
	unevaluated, _ = fc.GetUnevaluated(10)
	if len(unevaluated) != 0 {
		t.Errorf("Expected 0 unevaluated after verdict, got %d", len(unevaluated))
	}
}

func TestFeedbackCollector_NilRecord(t *testing.T) {
	tempDir := t.TempDir()
	fc, err := NewFeedbackCollector(tempDir)
	if err != nil {
		t.Fatalf("NewFeedbackCollector failed: %v", err)
	}
	defer fc.Close()
	if err := fc.Record(nil); err == nil {
		t.Fatal("expected error for nil record")
	}
}

func TestFeedbackCollector_LargePayload(t *testing.T) {
	tempDir := t.TempDir()
	fc, err := NewFeedbackCollector(tempDir)
	if err != nil {
		t.Fatalf("NewFeedbackCollector failed: %v", err)
	}
	defer fc.Close()

	large := strings.Repeat("x", 10*1024*1024+1)
	exec := &ExecutionRecord{
		TaskID:      "task-large-001",
		SessionID:   "session-large",
		ShardType:   "/coder",
		TaskRequest: large,
		Timestamp:   time.Now(),
		ExecutionResult: ExecutionResult{
			Success: false,
			Output:  large,
		},
	}
	if err := fc.Record(exec); err != nil {
		t.Fatalf("expected large payload to be recorded, got error: %v", err)
	}
}

func TestFeedbackCollector_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	fc, err := NewFeedbackCollector(tempDir)
	if err != nil {
		t.Fatalf("NewFeedbackCollector failed: %v", err)
	}
	defer fc.Close()

	const workers = 50
	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := &ExecutionRecord{
				TaskID:      "task-concurrent-" + strconv.Itoa(i),
				SessionID:   "session-concurrent",
				ShardType:   "/coder",
				TaskRequest: "concurrent write",
				Timestamp:   time.Now(),
				ExecutionResult: ExecutionResult{
					Success: true,
				},
			}
			if err := fc.Record(rec); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent record failed: %v", err)
	}

	total, _ := fc.GetStats()
	if total != workers {
		t.Fatalf("expected %d records, got %d", workers, total)
	}
}

func TestFeedbackCollector_DatabaseError(t *testing.T) {
	tempDir := t.TempDir()
	fc, err := NewFeedbackCollector(tempDir)
	if err != nil {
		t.Fatalf("NewFeedbackCollector failed: %v", err)
	}
	if err := fc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	rec := &ExecutionRecord{
		TaskID:      "task-db-error",
		SessionID:   "session-db-error",
		ShardType:   "/coder",
		TaskRequest: "db closed",
		Timestamp:   time.Now(),
		ExecutionResult: ExecutionResult{
			Success: false,
		},
	}
	if err := fc.Record(rec); err == nil {
		t.Fatal("expected record error after database close")
	}
}

// =============================================================================
// STRATEGY STORE TESTS
// =============================================================================

func TestStrategyStore(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "strategy_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create strategy store
	ss, err := NewStrategyStore(tempDir)
	if err != nil {
		t.Fatalf("NewStrategyStore failed: %v", err)
	}
	defer ss.Close()

	// Generate default strategies
	ss.GenerateDefaultStrategies()

	// Get stats
	total, avgSuccess, err := ss.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if total == 0 {
		t.Error("Expected default strategies to be generated")
	}
	if avgSuccess < 0 || avgSuccess > 1 {
		t.Errorf("Average success rate should be in [0, 1], got %v", avgSuccess)
	}

	// Select strategies
	strategies, err := ss.SelectStrategies(ProblemDebugging, "/coder", 3)
	if err != nil {
		t.Fatalf("SelectStrategies failed: %v", err)
	}
	if len(strategies) == 0 {
		t.Error("Expected at least one strategy for debugging")
	}

	// Record outcome
	if len(strategies) > 0 {
		if err := ss.RecordOutcome(strategies[0].ID, "test-task-001", true); err != nil {
			t.Fatalf("RecordOutcome failed: %v", err)
		}
	}
}

func TestStrategyStore_MassiveStrategyCount(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "strategy_massive_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ss, err := NewStrategyStore(tempDir)
	if err != nil {
		t.Fatalf("NewStrategyStore failed: %v", err)
	}
	defer ss.Close()

	const strategyCount = 10000
	now := time.Now()

	tx, err := ss.db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO strategies
		(id, problem_type, shard_type, content, success_count, failure_count, success_rate,
		 last_used, last_refined, version, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("failed to prepare bulk insert: %v", err)
	}

	for i := 0; i < strategyCount; i++ {
		successCount := (i % 10) + 1
		failureCount := i % 3
		successRate := float64(successCount) / float64(successCount+failureCount+1)
		createdAt := now.Add(-time.Duration(i) * time.Second)
		if _, err := stmt.Exec(
			"massive-strategy-"+strconv.Itoa(i),
			ProblemDebugging,
			"/coder",
			"bulk strategy content "+strconv.Itoa(i),
			successCount,
			failureCount,
			successRate,
			createdAt,
			createdAt,
			1,
			"generated",
			createdAt,
		); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			t.Fatalf("bulk insert failed at row %d: %v", i, err)
		}
	}

	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		t.Fatalf("failed to close bulk insert statement: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit bulk insert: %v", err)
	}

	total, _, err := ss.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if total < strategyCount {
		t.Fatalf("expected at least %d strategies, got %d", strategyCount, total)
	}

	selectStart := time.Now()
	selected, err := ss.SelectStrategies(ProblemDebugging, "/coder", 10)
	if err != nil {
		t.Fatalf("SelectStrategies failed: %v", err)
	}
	if len(selected) != 10 {
		t.Fatalf("expected 10 selected strategies, got %d", len(selected))
	}
	if elapsed := time.Since(selectStart); elapsed > 2*time.Second {
		t.Fatalf("SelectStrategies too slow with %d strategies: %v", strategyCount, elapsed)
	}

	for i := 1; i < len(selected); i++ {
		if selected[i-1].SuccessRate < selected[i].SuccessRate {
			t.Fatalf("strategies are not sorted by success_rate desc at index %d", i)
		}
	}
}

// =============================================================================
// ATOM GENERATOR HELPER TESTS
// =============================================================================

func TestExtractYAMLBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "yaml code block",
			input: `Here's the atom:
` + "```yaml" + `
- id: test/atom
  content: hello
` + "```",
			expected: `- id: test/atom
  content: hello`,
		},
		{
			name:     "no yaml block",
			input:    "just plain text",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractYAMLBlock(tt.input)
			if result != tt.expected {
				t.Errorf("extractYAMLBlock() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMapCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected prompt.AtomCategory
	}{
		{"methodology", prompt.CategoryMethodology},
		{"language", prompt.CategoryLanguage},
		{"framework", prompt.CategoryFramework},
		{"domain", prompt.CategoryDomain},
		{"exemplar", prompt.CategoryExemplar},
		{"unknown", prompt.CategoryMethodology}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapCategory(tt.input)
			if result != tt.expected {
				t.Errorf("mapCategory(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	// 4 chars per token is the rough estimate
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"test", 1},
		{"hello world", 2}, // 11 chars / 4 = 2
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := estimateTokens(tt.input)
			if result != tt.expected {
				t.Errorf("estimateTokens(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAtomGenerator_EmptyFailures(t *testing.T) {
	ag := NewAtomGenerator(&mockLLMClient{}, nil)
	atoms, err := ag.GenerateFromFailures(context.Background(), nil, "/coder", string(ProblemDebugging))
	if err != nil {
		t.Fatalf("GenerateFromFailures returned error: %v", err)
	}
	if len(atoms) != 0 {
		t.Fatalf("expected no atoms for empty failures, got %d", len(atoms))
	}
}

type malformedAtomLLMClient struct{}

func (m *malformedAtomLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "invalid", nil
}

func (m *malformedAtomLLMClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	return "not yaml and not atom json", nil
}

func (m *malformedAtomLLMClient) CompleteWithOptions(ctx context.Context, system, user string, opts map[string]interface{}) (string, error) {
	return m.CompleteWithSystem(ctx, system, user)
}

func TestAtomGenerator_MalformedLLMResponse(t *testing.T) {
	ag := NewAtomGenerator(&malformedAtomLLMClient{}, nil)
	failures := []*JudgeVerdict{{
		Verdict:     "FAIL",
		Explanation: "bad output",
		Category:    CategoryLogicError,
		TaskID:      "task-malformed",
	}}
	if _, err := ag.GenerateFromFailures(context.Background(), failures, "/coder", string(ProblemDebugging)); err == nil {
		t.Fatal("expected parsing error for malformed LLM response")
	}
}

// =============================================================================
// PROMPT EVOLVER TESTS
// =============================================================================

// mockLLMClient is a minimal mock for testing
type mockLLMClient struct{}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "mock response", nil
}

func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	// Return mock YAML for atom generation
	return "```yaml\n- id: test/evolved/mock\n  category: methodology\n  content: Test content\n```", nil
}

func (m *mockLLMClient) CompleteWithOptions(ctx context.Context, system, user string, opts map[string]interface{}) (string, error) {
	return m.CompleteWithSystem(ctx, system, user)
}

type partialBatchJudgeLLMClient struct {
}

func (m *partialBatchJudgeLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (m *partialBatchJudgeLLMClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	if strings.Contains(user, "## Task Request\nsecond\n") {
		return "not-json", nil
	}
	return `{"verdict":"FAIL","explanation":"failed","category":"LOGIC_ERROR","improvement_rule":"When testing, always validate the output"}`, nil
}

func TestPromptEvolver_Lifecycle(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "evolver_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create evolver
	config := DefaultEvolverConfig()
	config.MinFailuresForEvolution = 1 // Lower threshold for testing
	config.EnableStrategies = true

	evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, config)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer evolver.Close()

	// Verify directories created
	evolvedDir := filepath.Join(tempDir, "prompts", "evolved")
	if _, err := os.Stat(evolvedDir); os.IsNotExist(err) {
		t.Error("Expected evolved directory to be created")
	}

	pendingDir := filepath.Join(evolvedDir, "pending")
	if _, err := os.Stat(pendingDir); os.IsNotExist(err) {
		t.Error("Expected pending directory to be created")
	}

	// Record execution
	exec := &ExecutionRecord{
		TaskID:      "task-001",
		SessionID:   "session-001",
		ShardType:   "/coder",
		TaskRequest: "fix the bug",
		Timestamp:   time.Now(),
		ExecutionResult: ExecutionResult{
			Success: false,
			Output:  "test failed",
		},
	}

	if err := evolver.RecordExecution(exec); err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Get stats
	stats := evolver.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.TotalExecutionsRecorded != 1 {
		t.Errorf("Expected 1 execution recorded, got %d", stats.TotalExecutionsRecorded)
	}

	// Should be able to run evolution
	if !evolver.ShouldRunEvolution() {
		t.Error("Expected ShouldRunEvolution to return true initially")
	}
}

func TestPromptEvolver_GetEvolvedAtoms(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "evolver_atoms_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, nil)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer evolver.Close()

	// Initially empty
	atoms := evolver.GetEvolvedAtoms()
	if len(atoms) != 0 {
		t.Errorf("Expected 0 evolved atoms initially, got %d", len(atoms))
	}

	pending := evolver.GetPendingAtoms()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending atoms initially, got %d", len(pending))
	}

	promoted := evolver.GetPromotedAtoms()
	if len(promoted) != 0 {
		t.Errorf("Expected 0 promoted atoms initially, got %d", len(promoted))
	}
}

func TestPromptEvolver_SelectStrategies(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "evolver_strategies_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultEvolverConfig()
	config.EnableStrategies = true

	evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, config)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer evolver.Close()

	// Select strategies for a task
	strategies, err := evolver.SelectStrategies("fix the failing test", "/tester")
	if err != nil {
		t.Fatalf("SelectStrategies failed: %v", err)
	}

	// Should have default strategies available
	if len(strategies) == 0 {
		t.Log("No strategies selected (default strategies may not be seeded yet)")
	}
}

func TestPromptEvolver_WriteFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "evolver_write_failure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, nil)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer evolver.Close()

	blockedPath := filepath.Join(tempDir, "pending-file")
	if err := os.WriteFile(blockedPath, []byte("not-a-directory"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	evolver.pendingDir = blockedPath
	atom := &GeneratedAtom{
		Atom: &prompt.PromptAtom{
			ID:       "test/write/failure",
			Category: prompt.CategoryMethodology,
			Content:  "Always validate write paths before persistence",
		},
		Source:     "failure_analysis",
		SourceIDs:  []string{"task-write-failure"},
		Confidence: 0.5,
		CreatedAt:  time.Now(),
	}

	if err := evolver.storeEvolvedAtom(atom); err == nil {
		t.Fatal("expected storeEvolvedAtom to fail when pendingDir is not writable as a directory")
	}
	if _, exists := evolver.evolvedAtoms[atom.Atom.ID]; exists {
		t.Fatal("storeEvolvedAtom should not keep atom in memory when persistence fails")
	}
}

func TestPromptEvolver_InvalidConfig(t *testing.T) {
	base := DefaultEvolverConfig()
	cases := []struct {
		name   string
		mutate func(*EvolverConfig)
	}{
		{
			name: "min failures below 1",
			mutate: func(c *EvolverConfig) {
				c.MinFailuresForEvolution = 0
			},
		},
		{
			name: "non-positive interval",
			mutate: func(c *EvolverConfig) {
				c.EvolutionInterval = 0
			},
		},
		{
			name: "max atoms below 1",
			mutate: func(c *EvolverConfig) {
				c.MaxAtomsPerEvolution = 0
			},
		},
		{
			name: "confidence below 0",
			mutate: func(c *EvolverConfig) {
				c.ConfidenceThreshold = -0.1
			},
		},
		{
			name: "confidence above 1",
			mutate: func(c *EvolverConfig) {
				c.ConfidenceThreshold = 1.1
			},
		},
		{
			name: "strategy refine threshold below 1",
			mutate: func(c *EvolverConfig) {
				c.StrategyRefineThreshold = 0
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "evolver_invalid_config_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			cfg := *base
			tc.mutate(&cfg)
			evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, &cfg)
			if err == nil {
				if evolver != nil {
					_ = evolver.Close()
				}
				t.Fatal("expected NewPromptEvolver to fail for invalid config")
			}
		})
	}

	t.Run("empty judge model falls back to default", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "evolver_config_default_model_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		cfg := *base
		cfg.JudgeModel = ""
		evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, &cfg)
		if err != nil {
			t.Fatalf("unexpected error for empty judge model: %v", err)
		}
		defer evolver.Close()

		if evolver.judge == nil || evolver.judge.modelName != "gemini-3-pro" {
			t.Fatalf("expected default judge model gemini-3-pro, got %+v", evolver.judge)
		}
	})
}

func TestPromptEvolver_ConcurrentAccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "evolver_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultEvolverConfig()
	config.MinFailuresForEvolution = 1

	evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, config)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer evolver.Close()

	const writers = 48
	const readers = 24

	var wg sync.WaitGroup
	errCh := make(chan error, writers+readers)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := &ExecutionRecord{
				TaskID:      "task-concurrent-" + strconv.Itoa(idx),
				SessionID:   "session-concurrent",
				ShardType:   "/coder",
				TaskRequest: "concurrent record execution",
				Timestamp:   time.Now(),
				ExecutionResult: ExecutionResult{
					Success: idx%2 == 0,
					Output:  "output " + strconv.Itoa(idx),
				},
			}
			if err := evolver.RecordExecution(rec); err != nil {
				errCh <- err
			}
		}(i)
	}

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				stats := evolver.GetStats()
				if stats == nil {
					errCh <- os.ErrInvalid
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent access error: %v", err)
		}
	}

	stats := evolver.GetStats()
	if stats.TotalExecutionsRecorded != writers {
		t.Fatalf("expected %d executions recorded, got %d", writers, stats.TotalExecutionsRecorded)
	}
}

func TestTaskJudge_EvaluateBatchPreservesOrdering(t *testing.T) {
	judge := NewTaskJudge(&partialBatchJudgeLLMClient{}, "test")

	execs := []*ExecutionRecord{
		{
			TaskID:      "task-1",
			ShardType:   "/coder",
			TaskRequest: "first",
			Timestamp:   time.Now(),
		},
		{
			TaskID:      "task-2",
			ShardType:   "/coder",
			TaskRequest: "second",
			Timestamp:   time.Now(),
		},
		{
			TaskID:      "task-3",
			ShardType:   "/coder",
			TaskRequest: "third",
			Timestamp:   time.Now(),
		},
	}

	verdicts, err := judge.EvaluateBatch(context.Background(), execs)
	if err != nil {
		t.Fatalf("EvaluateBatch failed: %v", err)
	}
	if len(verdicts) != len(execs) {
		t.Fatalf("expected %d verdict slots, got %d", len(execs), len(verdicts))
	}
	if verdicts[1] != nil {
		t.Fatalf("expected failed parse to remain nil at index 1, got %+v", verdicts[1])
	}
	if verdicts[0] == nil || verdicts[0].TaskID != "task-1" {
		t.Fatalf("expected verdict for task-1 at index 0, got %+v", verdicts[0])
	}
	if verdicts[2] == nil || verdicts[2].TaskID != "task-3" {
		t.Fatalf("expected verdict for task-3 at index 2, got %+v", verdicts[2])
	}
}

func TestFeedbackCollector_RoundTripsPromptContext(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "feedback_prompt_context_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fc, err := NewFeedbackCollector(tempDir)
	if err != nil {
		t.Fatalf("NewFeedbackCollector failed: %v", err)
	}
	defer fc.Close()

	exec := &ExecutionRecord{
		TaskID:      "task-prompt-context",
		SessionID:   "session-ctx",
		ShardType:   "/coder",
		TaskRequest: "fix the prompt wiring",
		Timestamp:   time.Now(),
		PromptManifest: &prompt.PromptManifest{
			ContextHash: "ctx-123",
			Selected: []prompt.AtomManifestEntry{
				{ID: "atom/a", Category: "methodology", Source: "embedded"},
			},
		},
		AtomIDs:          []string{"atom/a", "atom/b"},
		ThoughtSummary:   "Reason through the bug first.",
		ThinkingTokens:   42,
		GroundingSources: []string{"https://example.com/doc"},
		ExecutionResult: ExecutionResult{
			Success: true,
			Output:  "done",
		},
	}

	if err := fc.Record(exec); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	rows, err := fc.GetRecentByShardType("/coder", 1)
	if err != nil {
		t.Fatalf("GetRecentByShardType failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 record, got %d", len(rows))
	}
	got := rows[0]
	if got.PromptManifest == nil || got.PromptManifest.ContextHash != "ctx-123" {
		t.Fatalf("expected prompt manifest to round-trip, got %+v", got.PromptManifest)
	}
	if len(got.AtomIDs) != 2 || got.AtomIDs[0] != "atom/a" {
		t.Fatalf("expected atom IDs to round-trip, got %v", got.AtomIDs)
	}
	if got.ThoughtSummary != exec.ThoughtSummary || got.ThinkingTokens != exec.ThinkingTokens {
		t.Fatalf("expected thinking metadata to round-trip, got summary=%q tokens=%d", got.ThoughtSummary, got.ThinkingTokens)
	}
	if len(got.GroundingSources) != 1 || got.GroundingSources[0] != exec.GroundingSources[0] {
		t.Fatalf("expected grounding sources to round-trip, got %v", got.GroundingSources)
	}
}

// =============================================================================
// INTEGRATION-STYLE TESTS
// =============================================================================

func TestFullEvolutionCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "evolution_cycle_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create evolver with low threshold
	config := DefaultEvolverConfig()
	config.MinFailuresForEvolution = 1
	config.AutoPromote = false // Don't auto-promote for testing

	evolver, err := NewPromptEvolver(tempDir, &mockLLMClient{}, config)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer evolver.Close()

	// Record a failure
	exec := &ExecutionRecord{
		TaskID:      "task-cycle-001",
		SessionID:   "session-001",
		ShardType:   "/coder",
		TaskRequest: "implement the feature",
		ProblemType: string(ProblemFeatureCreation),
		Timestamp:   time.Now(),
		ExecutionResult: ExecutionResult{
			Success: false,
			Output:  "compilation error: undefined variable",
		},
		Verdict: &JudgeVerdict{
			Verdict:         "FAIL",
			Explanation:     "Agent referenced undefined variable",
			Category:        CategorySyntaxError,
			Confidence:      0.9,
			ImprovementRule: "Always verify variable declarations before use",
			TaskID:          "task-cycle-001",
		},
	}

	if err := evolver.RecordExecution(exec); err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Run evolution cycle
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := evolver.RunEvolutionCycle(ctx)
	if err != nil {
		t.Fatalf("RunEvolutionCycle failed: %v", err)
	}

	if result == nil {
		t.Fatal("RunEvolutionCycle returned nil result")
	}

	// Verify evolution count increased
	stats := evolver.GetStats()
	if stats.TotalCycles != 1 {
		t.Errorf("Expected 1 evolution cycle, got %d", stats.TotalCycles)
	}

	t.Logf("Evolution result: failures=%d, atoms=%d, errors=%v",
		result.FailuresAnalyzed, result.AtomsGenerated, result.Errors)
}

type blockingJudgeLLMClient struct{}

func (m *blockingJudgeLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (m *blockingJudgeLLMClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func TestTaskJudge_NilExecution(t *testing.T) {
	judge := NewTaskJudge(&mockLLMClient{}, "test")
	if _, err := judge.Evaluate(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil execution")
	}
}

func TestTaskJudge_ContextCancellation(t *testing.T) {
	judge := NewTaskJudge(&blockingJudgeLLMClient{}, "test")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exec := &ExecutionRecord{
		TaskID:      "task-cancel",
		SessionID:   "session-cancel",
		ShardType:   "/coder",
		TaskRequest: "cancelled task",
		Timestamp:   time.Now(),
		ExecutionResult: ExecutionResult{
			Success: false,
		},
	}

	if _, err := judge.Evaluate(ctx, exec); err == nil {
		t.Fatal("expected context cancellation error")
	}
}

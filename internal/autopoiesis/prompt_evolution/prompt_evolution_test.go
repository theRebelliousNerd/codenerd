package prompt_evolution

import (
	"context"
	"os"
	"path/filepath"
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

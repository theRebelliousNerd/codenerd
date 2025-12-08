// Package tester implements the Tester ShardAgent per §7.0 Sharding.
// It specializes in test execution, generation, coverage analysis, and TDD repair loops.
package tester

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

// TesterConfig holds configuration for the tester shard.
type TesterConfig struct {
	Framework     string        // "gotest", "jest", "pytest", "cargo", "auto" (default: auto)
	CoverageGoal  float64       // Target coverage percentage (default: 80.0)
	CoverageCmd   string        // Coverage extraction command (auto-detected if empty)
	AutoFix       bool          // Auto-fix failing tests via TDD loop
	MaxRetries    int           // TDD retry limit (default: 3)
	TestTimeout   time.Duration // Per-test timeout (default: 5 minutes)
	BuildTimeout  time.Duration // Build timeout (default: 2 minutes)
	ParallelTests bool          // Run tests in parallel
	WorkingDir    string        // Workspace directory
	TestPatterns  []string      // Glob patterns for test files
	VerboseOutput bool          // Include detailed output in results
}

// DefaultTesterConfig returns sensible defaults for testing.
func DefaultTesterConfig() TesterConfig {
	return TesterConfig{
		Framework:     "auto",
		CoverageGoal:  80.0,
		CoverageCmd:   "",
		AutoFix:       false,
		MaxRetries:    3,
		TestTimeout:   5 * time.Minute,
		BuildTimeout:  2 * time.Minute,
		ParallelTests: false,
		WorkingDir:    ".",
		TestPatterns:  []string{"*_test.go", "*.test.ts", "test_*.py", "*_test.rs"},
		VerboseOutput: false,
	}
}

// =============================================================================
// TEST RESULT TYPES
// =============================================================================

// TestResult represents the outcome of a test run.
type TestResult struct {
	Passed      bool              `json:"passed"`
	Output      string            `json:"output"`
	Coverage    float64           `json:"coverage"`
	FailedTests []FailedTest      `json:"failed_tests"`
	PassedTests []string          `json:"passed_tests"`
	Duration    time.Duration     `json:"duration"`
	Diagnostics []core.Diagnostic `json:"diagnostics"`
	Retries     int               `json:"retries"`
	Framework   string            `json:"framework"`
	TestType    string            `json:"test_type"` // "unit", "integration", "e2e", or "unknown"
}

// FailedTest represents a single failed test.
type FailedTest struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// GeneratedTest represents a newly generated test.
type GeneratedTest struct {
	FilePath        string   `json:"file_path"`
	TargetFile      string   `json:"target_file"`
	Content         string   `json:"content"`
	TestCount       int      `json:"test_count"`
	FunctionsTested []string `json:"functions_tested"`
}

// TesterTask represents a parsed test task.
type TesterTask struct {
	Action   string // "run_tests", "generate_tests", "coverage", "tdd"
	Target   string // File path, package, or function
	File     string // Specific file (for function tests)
	Function string // Specific function
	Package  string // Package path
	Options  map[string]string
}

// =============================================================================
// TESTER SHARD
// =============================================================================

// TesterShard is specialized for test generation and TDD loops.
// Per Cortex 1.5.0 §7.0, this is a Type A Ephemeral Generalist or
// can become a Type B Persistent Specialist when hydrated with testing patterns.
type TesterShard struct {
	mu sync.RWMutex

	// Identity
	id     string
	config core.ShardConfig
	state  core.ShardState

	// Tester-specific configuration
	testerConfig TesterConfig

	// Components (required)
	kernel       *core.RealKernel   // Own kernel instance for logic-driven testing
	llmClient    core.LLMClient     // LLM for test generation
	virtualStore *core.VirtualStore // Action routing

	// TDD Loop integration
	tddLoop *core.TDDLoop

	// State tracking
	startTime   time.Time
	testHistory []TestResult
	diagnostics []core.Diagnostic

	// Autopoiesis tracking (§8.3) - in-memory, synced to LearningStore
	successPatterns map[string]int     // Patterns that pass tests
	failurePatterns map[string]int     // Patterns that repeatedly fail
	learningStore   core.LearningStore // Persistent learning storage

	// Policy loading guard (prevents duplicate Decl errors)
	policyLoaded bool
}

// NewTesterShard creates a new Tester shard with default configuration.
func NewTesterShard() *TesterShard {
	return NewTesterShardWithConfig(DefaultTesterConfig())
}

// NewTesterShardWithConfig creates a tester shard with custom configuration.
func NewTesterShardWithConfig(testerConfig TesterConfig) *TesterShard {
	return &TesterShard{
		config:          core.DefaultSpecialistConfig("tester", ""),
		state:           core.ShardStateIdle,
		testerConfig:    testerConfig,
		testHistory:     make([]TestResult, 0),
		diagnostics:     make([]core.Diagnostic, 0),
		successPatterns: make(map[string]int),
		failurePatterns: make(map[string]int),
	}
}

// =============================================================================
// DEPENDENCY INJECTION
// =============================================================================

// SetLLMClient sets the LLM client for test generation.
func (t *TesterShard) SetLLMClient(client core.LLMClient) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.llmClient = client
}

// SetParentKernel sets the Mangle kernel for logic-driven testing.
func (t *TesterShard) SetParentKernel(k core.Kernel) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		t.kernel = rk
	} else {
		panic("TesterShard requires *core.RealKernel")
	}
}

// SetVirtualStore sets the virtual store for action routing.
func (t *TesterShard) SetVirtualStore(vs *core.VirtualStore) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.virtualStore = vs
}

// SetLearningStore sets the learning store for persistent autopoiesis.
func (t *TesterShard) SetLearningStore(ls core.LearningStore) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.learningStore = ls
	// Load existing patterns from store
	t.loadLearnedPatterns()
}

// =============================================================================
// SHARD INTERFACE IMPLEMENTATION
// =============================================================================

// GetID returns the shard ID.
func (t *TesterShard) GetID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

// GetState returns the current state.
func (t *TesterShard) GetState() core.ShardState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// GetConfig returns the shard configuration.
func (t *TesterShard) GetConfig() core.ShardConfig {
	return t.config
}

// GetKernel returns the kernel (for fact propagation).
func (t *TesterShard) GetKernel() *core.RealKernel {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.kernel
}

// Stop stops the shard.
func (t *TesterShard) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = core.ShardStateCompleted
	return nil
}

// =============================================================================
// MAIN EXECUTION
// =============================================================================

// Execute performs the testing task.
// Task formats:
//   - "run_tests file:PATH" or "run_tests package:PACKAGE"
//   - "generate_tests file:PATH" or "generate_tests function:NAME in:FILE"
//   - "coverage file:PATH"
//   - "tdd file:PATH" (full TDD repair loop)
//   - "regenerate_mocks file:PATH" (regenerate mocks for an interface)
//   - "detect_stale_mocks file:PATH" (check for stale mocks in test file)
func (t *TesterShard) Execute(ctx context.Context, task string) (string, error) {
	t.mu.Lock()
	t.state = core.ShardStateRunning
	t.startTime = time.Now()
	t.id = fmt.Sprintf("tester-%d", time.Now().UnixNano())
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		t.state = core.ShardStateCompleted
		t.mu.Unlock()
	}()

	fmt.Printf("[TesterShard:%s] Starting task: %s\n", t.id, task)

	// Initialize kernel if not set
	if t.kernel == nil {
		t.kernel = core.NewRealKernel()
	}
	// Load tester-specific policy (only once to avoid duplicate Decl errors)
	if !t.policyLoaded {
		_ = t.kernel.LoadPolicyFile("tester.mg")
		t.policyLoaded = true
	}

	// Parse the task
	parsedTask, err := t.parseTask(task)
	if err != nil {
		return "", fmt.Errorf("failed to parse task: %w", err)
	}

	// Assert initial facts to kernel
	t.assertInitialFacts(parsedTask)

	// Route to appropriate handler
	var result *TestResult
	switch parsedTask.Action {
	case "run_tests":
		result, err = t.runTests(ctx, parsedTask)
	case "generate_tests":
		return t.generateTests(ctx, parsedTask)
	case "coverage":
		result, err = t.runCoverage(ctx, parsedTask)
	case "tdd":
		result, err = t.runTDDLoop(ctx, parsedTask)
	case "regenerate_mocks":
		return t.handleRegenerateMocks(ctx, parsedTask)
	case "detect_stale_mocks":
		return t.handleDetectStaleMocks(ctx, parsedTask)
	default:
		// Default to run_tests
		result, err = t.runTests(ctx, parsedTask)
	}

	if err != nil {
		return "", err
	}

	// Generate facts for propagation
	facts := t.generateFacts(result)
	for _, fact := range facts {
		if t.kernel != nil {
			_ = t.kernel.Assert(fact)
		}
	}

	// Track history
	t.mu.Lock()
	t.testHistory = append(t.testHistory, *result)
	t.mu.Unlock()

	// Format output
	return t.formatResult(result), nil
}

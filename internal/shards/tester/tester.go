// Package tester implements the Tester ShardAgent per §7.0 Sharding.
// It specializes in test execution, generation, coverage analysis, and TDD repair loops.
package tester

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
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

	// JIT Prompt Compiler integration
	promptAssembler *articulation.PromptAssembler // Optional JIT prompt assembler
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

// SetSessionContext sets the session context (for dream mode, etc.).
func (t *TesterShard) SetSessionContext(ctx *core.SessionContext) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config.SessionContext = ctx
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

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt generation.
// When set, the shard will use JIT compilation for system prompts when available.
func (t *TesterShard) SetPromptAssembler(pa *articulation.PromptAssembler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.promptAssembler = pa
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
// DREAM MODE (Simulation/Learning)
// =============================================================================

// describeDreamPlan returns a description of what the tester would do WITHOUT executing.
func (t *TesterShard) describeDreamPlan(ctx context.Context, task string) (string, error) {
	fmt.Printf("[TesterShard] DREAM MODE - describing plan without execution\n")

	if t.llmClient == nil {
		return "TesterShard would run tests and analyze coverage, but no LLM client available for dream description.", nil
	}

	prompt := fmt.Sprintf(`You are a testing agent in DREAM MODE. Describe what you WOULD do for this task WITHOUT actually doing it.

Task: %s

Provide a structured analysis:
1. **Understanding**: What kind of testing is being asked?
2. **Test Targets**: What files/packages would I test?
3. **Test Strategy**: What approach would I take? (unit, integration, TDD loop?)
4. **Tools Needed**: What testing tools/frameworks would I use?
5. **Expected Outcomes**: What might the tests reveal?
6. **Questions**: What would I need clarified?

Remember: This is a simulation. Describe the plan, don't execute it.`, task)

	response, err := t.llmClient.Complete(ctx, prompt)
	if err != nil {
		return fmt.Sprintf("TesterShard dream analysis failed: %v", err), nil
	}

	return response, nil
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
	timer := logging.StartTimer(logging.CategoryTester, "Execute")
	logging.Tester("Starting task execution: %s", task)

	t.mu.Lock()
	t.state = core.ShardStateRunning
	t.startTime = time.Now()
	t.id = fmt.Sprintf("tester-%d", time.Now().UnixNano())
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		t.state = core.ShardStateCompleted
		t.mu.Unlock()
		timer.StopWithInfo()
	}()

	// DREAM MODE: Only describe what we would do, don't execute
	if t.config.SessionContext != nil && t.config.SessionContext.DreamMode {
		logging.TesterDebug("DREAM MODE enabled, describing plan without execution")
		return t.describeDreamPlan(ctx, task)
	}

	logging.TesterDebug("[TesterShard:%s] Initializing for task", t.id)

	// Initialize kernel if not set
	if t.kernel == nil {
		logging.TesterDebug("Creating new RealKernel instance")
		t.kernel = core.NewRealKernel()
	}
	// Load tester-specific policy (only once to avoid duplicate Decl errors)
	if !t.policyLoaded {
		logging.TesterDebug("Loading tester.mg policy file")
		_ = t.kernel.LoadPolicyFile("tester.mg")
		t.policyLoaded = true
	}

	// Parse the task
	parseTimer := logging.StartTimer(logging.CategoryTester, "ParseTask")
	parsedTask, err := t.parseTask(task)
	parseTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryTester).Error("Failed to parse task: %v", err)
		return "", fmt.Errorf("failed to parse task: %w", err)
	}
	logging.Tester("Parsed task: action=%s, target=%s, function=%s",
		parsedTask.Action, parsedTask.Target, parsedTask.Function)

	// Assert initial facts to kernel
	logging.TesterDebug("Asserting initial facts to kernel")
	t.assertInitialFacts(parsedTask)

	// Route to appropriate handler
	var result *TestResult
	logging.Tester("Routing to handler: %s", parsedTask.Action)
	switch parsedTask.Action {
	case "run_tests":
		logging.TesterDebug("Executing run_tests handler")
		result, err = t.runTests(ctx, parsedTask)
	case "generate_tests":
		logging.TesterDebug("Executing generate_tests handler")
		return t.generateTests(ctx, parsedTask)
	case "coverage":
		logging.TesterDebug("Executing coverage handler")
		result, err = t.runCoverage(ctx, parsedTask)
	case "tdd":
		logging.TesterDebug("Executing TDD loop handler")
		result, err = t.runTDDLoop(ctx, parsedTask)
	case "regenerate_mocks":
		logging.TesterDebug("Executing regenerate_mocks handler")
		return t.handleRegenerateMocks(ctx, parsedTask)
	case "detect_stale_mocks":
		logging.TesterDebug("Executing detect_stale_mocks handler")
		return t.handleDetectStaleMocks(ctx, parsedTask)
	default:
		logging.TesterDebug("Unknown action, defaulting to run_tests")
		result, err = t.runTests(ctx, parsedTask)
	}

	if err != nil {
		logging.Get(logging.CategoryTester).Error("Handler failed: %v", err)
		return "", err
	}

	// Generate facts for propagation
	facts := t.generateFacts(result)
	logging.TesterDebug("Generated %d facts for propagation", len(facts))
	for _, fact := range facts {
		if t.kernel != nil {
			_ = t.kernel.Assert(fact)
		}
	}

	// Track history
	t.mu.Lock()
	t.testHistory = append(t.testHistory, *result)
	t.mu.Unlock()

	logging.Tester("Task completed: passed=%v, coverage=%.1f%%, duration=%v, retries=%d",
		result.Passed, result.Coverage, result.Duration, result.Retries)

	// Format output
	return t.formatResult(result), nil
}

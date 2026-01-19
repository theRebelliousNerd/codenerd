// Package chat provides test utilities for TUI testing.
// This file contains mocks, fixtures, and helpers for testing the chat package.
package chat

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
	"codenerd/internal/ux"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// =============================================================================
// MOCK KERNEL
// =============================================================================

// MockKernel simulates the core.RealKernel for testing.
type MockKernel struct {
	mu            sync.Mutex
	facts         map[string][]core.Fact
	assertedFacts []core.Fact
	queryCalls    []string
	queryResults  map[string][]core.Fact
	resetCalled   bool
	traceResult   *mangle.DerivationTrace
}

// NewMockKernel creates a new mock kernel with empty state.
func NewMockKernel() *MockKernel {
	return &MockKernel{
		facts:        make(map[string][]core.Fact),
		queryResults: make(map[string][]core.Fact),
	}
}

// Assert simulates asserting a fact.
func (m *MockKernel) Assert(f core.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assertedFacts = append(m.assertedFacts, f)
	m.facts[f.Predicate] = append(m.facts[f.Predicate], f)
	return nil
}

// Retract simulates retracting a fact.
func (m *MockKernel) Retract(predicate string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.facts, predicate)
	return nil
}

// Reset simulates resetting the kernel.
func (m *MockKernel) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetCalled = true
	m.facts = make(map[string][]core.Fact)
	m.assertedFacts = nil
}

// Query simulates querying facts.
func (m *MockKernel) Query(_ context.Context, query string) ([]core.Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCalls = append(m.queryCalls, query)
	if results, ok := m.queryResults[query]; ok {
		return results, nil
	}
	return nil, nil
}

// TraceQuery simulates trace query.
func (m *MockKernel) TraceQuery(_ context.Context, _ string) (*mangle.DerivationTrace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.traceResult, nil
}

// SetQueryResult configures a mock response for a specific query.
func (m *MockKernel) SetQueryResult(query string, results []core.Fact) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryResults[query] = results
}

// SetTraceResult configures the trace result.
func (m *MockKernel) SetTraceResult(trace *mangle.DerivationTrace) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traceResult = trace
}

// GetAssertedFacts returns all facts that were asserted.
func (m *MockKernel) GetAssertedFacts() []core.Fact {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]core.Fact, len(m.assertedFacts))
	copy(result, m.assertedFacts)
	return result
}

// GetQueryCalls returns all queries that were made.
func (m *MockKernel) GetQueryCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.queryCalls))
	copy(result, m.queryCalls)
	return result
}

// WasResetCalled returns true if Reset was called.
func (m *MockKernel) WasResetCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resetCalled
}

// =============================================================================
// MOCK LLM CLIENT
// =============================================================================

// MockLLMClient simulates the perception.LLMClient for testing.
// Implements types.LLMClient interface.
type MockLLMClient struct {
	mu            sync.Mutex
	responses     map[string]string
	defaultResp   string
	callCount     int
	lastPrompt    string
	systemPrompts []string
	shouldError   bool
	errorMsg      string
	toolResponses map[string]*types.LLMToolResponse
}

// NewMockLLMClient creates a new mock LLM client.
func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		responses:     make(map[string]string),
		defaultResp:   "Mock LLM response",
		toolResponses: make(map[string]*types.LLMToolResponse),
	}
}

// Complete implements types.LLMClient - simple completion without system prompt.
func (m *MockLLMClient) Complete(_ context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastPrompt = prompt

	if m.shouldError {
		return "", &MockError{msg: m.errorMsg}
	}

	if resp, ok := m.responses[prompt]; ok {
		return resp, nil
	}
	return m.defaultResp, nil
}

// CompleteWithSystem implements types.LLMClient - completion with system prompt.
func (m *MockLLMClient) CompleteWithSystem(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastPrompt = userPrompt
	m.systemPrompts = append(m.systemPrompts, systemPrompt)

	if m.shouldError {
		return "", &MockError{msg: m.errorMsg}
	}

	if resp, ok := m.responses[userPrompt]; ok {
		return resp, nil
	}
	return m.defaultResp, nil
}

// CompleteWithTools implements types.LLMClient - completion with tool definitions.
func (m *MockLLMClient) CompleteWithTools(_ context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastPrompt = userPrompt
	m.systemPrompts = append(m.systemPrompts, systemPrompt)

	if m.shouldError {
		return nil, &MockError{msg: m.errorMsg}
	}

	// Check for specific tool response
	if resp, ok := m.toolResponses[userPrompt]; ok {
		return resp, nil
	}

	// Return default text response
	return &types.LLMToolResponse{
		Text:       m.defaultResp,
		StopReason: "end_turn",
	}, nil
}

// SetResponse configures a specific response for a prompt.
func (m *MockLLMClient) SetResponse(prompt, response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[prompt] = response
}

// SetDefaultResponse sets the default response.
func (m *MockLLMClient) SetDefaultResponse(response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultResp = response
}

// SetToolResponse configures a specific tool response for a prompt.
func (m *MockLLMClient) SetToolResponse(prompt string, resp *types.LLMToolResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolResponses[prompt] = resp
}

// SetError configures the client to return an error.
func (m *MockLLMClient) SetError(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = true
	m.errorMsg = msg
}

// ClearError removes error state.
func (m *MockLLMClient) ClearError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = false
	m.errorMsg = ""
}

// GetCallCount returns the number of Complete calls.
func (m *MockLLMClient) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// GetLastPrompt returns the last prompt sent.
func (m *MockLLMClient) GetLastPrompt() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastPrompt
}

// GetSystemPrompts returns all system prompts sent.
func (m *MockLLMClient) GetSystemPrompts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.systemPrompts))
	copy(result, m.systemPrompts)
	return result
}

// MockError is a simple error type for testing.
type MockError struct {
	msg string
}

func (e *MockError) Error() string {
	return e.msg
}

// =============================================================================
// TEST MODEL BUILDER
// =============================================================================

// TestModelOption configures a test model.
type TestModelOption func(*Model)

// NewTestModel creates a minimal Model suitable for testing.
// It initializes all required fields with lightweight mocks/defaults.
func NewTestModel(opts ...TestModelOption) Model {
	// Initialize minimal UI components
	ta := textarea.New()
	ta.Placeholder = "Test input..."
	ta.SetWidth(80)
	ta.SetHeight(3)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	vp := viewport.New(80, 20)
	errVP := viewport.New(80, 4)

	styles := ui.DefaultStyles()

	// Create shutdown context
	ctx, cancel := context.WithCancel(context.Background())

	// Create model with safe defaults
	m := Model{
		textarea:           ta,
		viewport:           vp,
		errorVP:            errVP,
		spinner:            sp,
		styles:             styles,
		history:            []Message{},
		renderedCache:      make(map[int]string),
		cacheInvalidFrom:   0,
		Config:             config.DefaultUserConfig(),
		statusChan:         make(chan string, 10),
		workspace:          "/tmp/test-workspace",
		isBooting:          false,
		ready:              true,
		width:              100,
		height:             50,
		viewMode:           ChatView,
		mouseEnabled:       true,
		shutdownOnce:       &sync.Once{},
		shutdownCtx:        ctx,
		shutdownCancel:     cancel,
		goroutineWg:        &sync.WaitGroup{},
		preferencesMgr:     ux.NewPreferencesManager("/tmp/test-workspace"),
		transparencyMgr:    transparency.NewTransparencyManager(nil),
		continuationMode:   ContinuationModeAuto,
		shardResultHistory: make([]*ShardResult, 0),
	}

	// Try to initialize glamour renderer (may fail in test environment)
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	m.renderer = renderer

	// Apply options
	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// WithMockKernel sets up a mock kernel.
func WithMockKernel(mk *MockKernel) TestModelOption {
	return func(m *Model) {
		// Note: We can't directly set kernel since it's a *core.RealKernel
		// This is a limitation - tests will need to use real kernel or skip kernel tests
		_ = mk // Used in tests that check kernel behavior
	}
}

// WithBooting sets the model to booting state.
func WithBooting(booting bool) TestModelOption {
	return func(m *Model) {
		m.isBooting = booting
		m.ready = !booting
	}
}

// WithViewMode sets the view mode.
func WithViewMode(mode ViewMode) TestModelOption {
	return func(m *Model) {
		m.viewMode = mode
	}
}

// WithHistory adds messages to history.
func WithHistory(messages ...Message) TestModelOption {
	return func(m *Model) {
		m.history = append(m.history, messages...)
	}
}

// WithSize sets the terminal dimensions.
func WithSize(width, height int) TestModelOption {
	return func(m *Model) {
		m.width = width
		m.height = height
		m.viewport = viewport.New(width, height-10)
		m.textarea.SetWidth(width - 4)
	}
}

// WithInputMode sets the input mode.
func WithInputMode(mode InputMode) TestModelOption {
	return func(m *Model) {
		switch mode {
		case InputModeClarification:
			m.awaitingClarification = true
		case InputModePatch:
			m.awaitingPatch = true
		case InputModeAgentWizard:
			m.awaitingAgentDefinition = true
		case InputModeConfigWizard:
			m.awaitingConfigWizard = true
		case InputModeOnboarding:
			m.awaitingOnboarding = true
		}
	}
}

// WithContinuationMode sets the continuation mode.
func WithContinuationMode(mode ContinuationMode) TestModelOption {
	return func(m *Model) {
		m.continuationMode = mode
	}
}

// WithPendingSubtasks adds pending subtasks.
func WithPendingSubtasks(tasks ...Subtask) TestModelOption {
	return func(m *Model) {
		m.pendingSubtasks = tasks
		m.continuationTotal = len(tasks)
	}
}

// WithLoading sets the loading state.
func WithLoading(loading bool) TestModelOption {
	return func(m *Model) {
		m.isLoading = loading
	}
}

// WithWorkspace sets the workspace path.
func WithWorkspace(workspace string) TestModelOption {
	return func(m *Model) {
		m.workspace = workspace
	}
}

// WithLLMClient sets the LLM client.
func WithLLMClient(client perception.LLMClient) TestModelOption {
	return func(m *Model) {
		m.client = client
	}
}

// WithRealKernel creates and attaches a real Mangle kernel.
// This requires a valid workspace path to be set first.
func WithRealKernel(t *testing.T, workspace string) TestModelOption {
	return func(m *Model) {
		kernel, err := core.NewRealKernelWithWorkspace(workspace)
		if err != nil {
			t.Fatalf("Failed to create real kernel: %v", err)
		}
		m.kernel = kernel
		m.workspace = workspace
	}
}

// WithVirtualStore creates and attaches a VirtualStore.
func WithVirtualStore(vs *core.VirtualStore) TestModelOption {
	return func(m *Model) {
		m.virtualStore = vs
	}
}

// WithTransducer sets the transducer.
func WithTransducer(t perception.Transducer) TestModelOption {
	return func(m *Model) {
		m.transducer = t
	}
}

// =============================================================================
// PERFORMANCE TRACKING
// =============================================================================

// PerformanceTracker tracks timing metrics for tests.
type PerformanceTracker struct {
	mu      sync.Mutex
	metrics map[string][]time.Duration
}

// NewPerformanceTracker creates a new performance tracker.
func NewPerformanceTracker() *PerformanceTracker {
	return &PerformanceTracker{
		metrics: make(map[string][]time.Duration),
	}
}

// Track measures the duration of a function and records it.
func (p *PerformanceTracker) Track(name string, fn func()) time.Duration {
	start := time.Now()
	fn()
	duration := time.Since(start)

	p.mu.Lock()
	p.metrics[name] = append(p.metrics[name], duration)
	p.mu.Unlock()

	return duration
}

// Report logs all recorded metrics.
func (p *PerformanceTracker) Report(t *testing.T) {
	p.mu.Lock()
	defer p.mu.Unlock()

	t.Log("=== Performance Report ===")
	for name, durations := range p.metrics {
		var total time.Duration
		for _, d := range durations {
			total += d
		}
		avg := total / time.Duration(len(durations))
		t.Logf("  %s: avg=%v, count=%d, total=%v", name, avg, len(durations), total)
	}
}

// GetAverage returns the average duration for a metric.
func (p *PerformanceTracker) GetAverage(name string) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	durations, ok := p.metrics[name]
	if !ok || len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

// =============================================================================
// MOCK EXECUTOR
// =============================================================================

// MockExecutor implements tactile.Executor for testing.
type MockExecutor struct {
	mu         sync.Mutex
	executions []string
	results    map[string]*tactile.ExecutionResult
	shouldFail bool
	failMsg    string
}

// NewMockExecutor creates a new mock executor.
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		results: make(map[string]*tactile.ExecutionResult),
	}
}

// Execute implements tactile.Executor.
func (e *MockExecutor) Execute(_ context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.executions = append(e.executions, cmd.Binary)

	if e.shouldFail {
		return nil, &MockError{msg: e.failMsg}
	}

	if result, ok := e.results[cmd.Binary]; ok {
		return result, nil
	}
	return &tactile.ExecutionResult{
		ExitCode: 0,
		Stdout:   "executed: " + cmd.Binary,
		Stderr:   "",
		Duration: 100 * time.Millisecond,
	}, nil
}

// Capabilities implements tactile.Executor.
func (e *MockExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{
		Name:                     "mock",
		Platform:                 "test",
		SupportsResourceLimits:   false,
		SupportsResourceUsage:    false,
		SupportedSandboxModes:    []tactile.SandboxMode{tactile.SandboxNone},
		SupportsNetworkIsolation: false,
		SupportsStdin:            true,
		MaxTimeout:               time.Hour,
		DefaultTimeout:           30 * time.Second,
	}
}

// Validate implements tactile.Executor.
func (e *MockExecutor) Validate(_ tactile.Command) error {
	return nil
}

// SetResult configures a specific result for a command binary.
func (e *MockExecutor) SetResult(binary string, result *tactile.ExecutionResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.results[binary] = result
}

// SetFail configures the executor to fail.
func (e *MockExecutor) SetFail(msg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shouldFail = true
	e.failMsg = msg
}

// GetExecutions returns all executed command binaries.
func (e *MockExecutor) GetExecutions() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]string, len(e.executions))
	copy(result, e.executions)
	return result
}

// =============================================================================
// LIVE TEST HELPERS
// =============================================================================

// SetupLiveWorkspace creates a temp workspace with required directories.
func SetupLiveWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()

	// Create .nerd directory structure
	dirs := []string{
		".nerd/mangle",
		".nerd/prompts",
		".nerd/logs",
		".nerd/shards",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(workspace, dir), 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", dir, err)
		}
	}

	return workspace
}

// SetupLiveModel creates a Model with real kernel and mock LLM.
func SetupLiveModel(t *testing.T) (Model, *PerformanceTracker) {
	t.Helper()
	perf := NewPerformanceTracker()

	workspace := SetupLiveWorkspace(t)

	var kernel *core.RealKernel
	perf.Track("kernel_creation", func() {
		var err error
		kernel, err = core.NewRealKernelWithWorkspace(workspace)
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}
	})

	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("Mock response for testing")

	// Create model with all components
	m := NewTestModel(WithSize(100, 50))

	// Set real components
	m.kernel = kernel
	m.workspace = workspace
	m.client = mockClient
	m.virtualStore = core.NewVirtualStore(nil) // No executor for basic tests
	m.ready = true
	m.isBooting = false

	return m, perf
}

// SetupFullIntegrationModel creates a Model with real kernel, mock LLM, and executor.
func SetupFullIntegrationModel(t *testing.T) (Model, *PerformanceTracker, *MockExecutor) {
	t.Helper()
	perf := NewPerformanceTracker()

	workspace := SetupLiveWorkspace(t)
	mockExecutor := NewMockExecutor()
	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("Mock response for full integration testing")

	var kernel *core.RealKernel
	perf.Track("kernel_creation", func() {
		var err error
		kernel, err = core.NewRealKernelWithWorkspace(workspace)
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}
	})

	m := NewTestModel(WithSize(100, 50))
	m.kernel = kernel
	m.workspace = workspace
	m.client = mockClient
	m.virtualStore = core.NewVirtualStore(mockExecutor)
	m.ready = true
	m.isBooting = false

	return m, perf, mockExecutor
}

// =============================================================================
// MESSAGE FIXTURES
// =============================================================================

// TestMessages provides common message fixtures for testing.
var TestMessages = struct {
	// Window Events
	WindowResize100x50 tea.Msg
	WindowResize80x24  tea.Msg

	// Key Events
	KeyEnter    tea.Msg
	KeyEsc      tea.Msg
	KeyCtrlC    tea.Msg
	KeyCtrlX    tea.Msg
	KeyShiftTab tea.Msg
	KeyUp       tea.Msg
	KeyDown     tea.Msg
	KeyTab      tea.Msg

	// Boot Messages
	BootComplete tea.Msg
	ScanComplete tea.Msg

	// Sample user message
	UserMessage Message
	// Sample assistant message
	AssistantMessage Message
}{
	WindowResize100x50: tea.WindowSizeMsg{Width: 100, Height: 50},
	WindowResize80x24:  tea.WindowSizeMsg{Width: 80, Height: 24},

	KeyEnter:    tea.KeyMsg{Type: tea.KeyEnter},
	KeyEsc:      tea.KeyMsg{Type: tea.KeyEsc},
	KeyCtrlC:    tea.KeyMsg{Type: tea.KeyCtrlC},
	KeyCtrlX:    tea.KeyMsg{Type: tea.KeyCtrlX},
	KeyShiftTab: tea.KeyMsg{Type: tea.KeyShiftTab},
	KeyUp:       tea.KeyMsg{Type: tea.KeyUp},
	KeyDown:     tea.KeyMsg{Type: tea.KeyDown},
	KeyTab:      tea.KeyMsg{Type: tea.KeyTab},

	BootComplete: bootCompleteMsg{
		components: &SystemComponents{
			InitialMessages: []Message{
				{Role: "assistant", Content: "System Ready", Time: time.Now()},
			},
			Workspace: "/tmp/test",
		},
		err: nil,
	},
	ScanComplete: scanCompleteMsg{
		fileCount: 100,
		factCount: 500,
		duration:  time.Second,
		err:       nil,
	},

	UserMessage: Message{
		Role:    "user",
		Content: "Test user message",
		Time:    time.Now(),
	},
	AssistantMessage: Message{
		Role:    "assistant",
		Content: "Test assistant response",
		Time:    time.Now(),
	},
}

// MakeKeyMsg creates a key message from a string (e.g., "a", "1", "/").
func MakeKeyMsg(key string) tea.KeyMsg {
	if len(key) == 1 {
		return tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune(key),
		}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

// MakeStatusMsg creates a status message.
func MakeStatusMsg(status string) statusMsg {
	return statusMsg(status)
}

// =============================================================================
// TEST HELPERS
// =============================================================================

// SimulateMessages sends multiple messages through Update and returns the final model.
func SimulateMessages(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}
	return m
}

// SimulateInput simulates typing text and pressing Enter.
func SimulateInput(m Model, input string) (Model, tea.Cmd) {
	m.textarea.SetValue(input)
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return newModel.(Model), cmd
}

// AssertNoNilPanic runs an assertion and recovers from panics.
// Returns an error if a panic occurred.
func AssertNoNilPanic(fn func()) (panicValue interface{}) {
	defer func() {
		panicValue = recover()
	}()
	fn()
	return nil
}

// AssertHistoryContains checks if history contains a message with the given substring.
func AssertHistoryContains(m Model, substring string) bool {
	for _, msg := range m.history {
		if testContainsSubstring(msg.Content, substring) {
			return true
		}
	}
	return false
}

// AssertHistoryLastMessage checks the last message in history.
func AssertHistoryLastMessage(m Model, role string) bool {
	if len(m.history) == 0 {
		return false
	}
	return m.history[len(m.history)-1].Role == role
}

// testContainsSubstring is a simple substring check for tests.
func testContainsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// WaitForCondition polls a condition with timeout.
func WaitForCondition(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// =============================================================================
// COMMAND TEST TABLE
// =============================================================================

// CommandTestCase defines a test case for command handling.
type CommandTestCase struct {
	Name       string
	Input      string
	Setup      func(*Model)
	Assert     func(Model) error
	WantErr    bool
	SkipReason string // If non-empty, test is skipped
}

// RunCommandTests is a placeholder - actual implementation uses *testing.T directly.
// See commands_test.go for table-driven command tests.

// =============================================================================
// INTENT FIXTURES
// =============================================================================

// TestIntents provides common intent fixtures.
var TestIntents = struct {
	ReviewCode perception.Intent
	FixBug     perception.Intent
	Explain    perception.Intent
	Query      perception.Intent
}{
	ReviewCode: perception.Intent{
		Category:   "analysis",
		Verb:       "review",
		Target:     "main.go",
		Confidence: 0.95,
	},
	FixBug: perception.Intent{
		Category:   "modification",
		Verb:       "fix",
		Target:     "bug in parser",
		Confidence: 0.90,
	},
	Explain: perception.Intent{
		Category:   "query",
		Verb:       "explain",
		Target:     "how does X work",
		Confidence: 0.85,
	},
	Query: perception.Intent{
		Category:   "query",
		Verb:       "query",
		Target:     "user_intent",
		Confidence: 0.88,
	},
}

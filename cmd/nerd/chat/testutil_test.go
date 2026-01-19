// Package chat provides test utilities for TUI testing.
// This file contains mocks, fixtures, and helpers for testing the chat package.
package chat

import (
	"context"
	"sync"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/transparency"
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
type MockLLMClient struct {
	mu            sync.Mutex
	responses     map[string]string
	defaultResp   string
	callCount     int
	lastPrompt    string
	systemPrompts []string
	shouldError   bool
	errorMsg      string
}

// NewMockLLMClient creates a new mock LLM client.
func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		responses:   make(map[string]string),
		defaultResp: "Mock LLM response",
	}
}

// Complete simulates an LLM completion.
func (m *MockLLMClient) Complete(_ context.Context, systemPrompt, userPrompt string) (string, error) {
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

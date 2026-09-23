//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS FOR BOUNDARY TESTING
// =============================================================================

type spawnerMockJITCompiler struct {
	mu           sync.Mutex
	delay        time.Duration
	failCompile  bool
	emptyPrompt  bool
	compileCount int
}

func (m *spawnerMockJITCompiler) Compile(ctx context.Context, compCtx *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	m.mu.Lock()
	m.compileCount++
	delay := m.delay
	fail := m.failCompile
	empty := m.emptyPrompt
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if fail {
		return nil, errors.New("mock compilation failure")
	}

	promptText := "You are a helpful assistant."
	if empty {
		promptText = ""
	}

	return &prompt.CompilationResult{
		Prompt: promptText,
	}, nil
}

type spawnerMockConfigFactory struct {
	mu           sync.Mutex
	delay        time.Duration
	failGenerate bool
	nilConfig    bool
	genCount     int
}

func (m *spawnerMockConfigFactory) Generate(ctx context.Context, res *prompt.CompilationResult, intentVerbs ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	m.mu.Lock()
	m.genCount++
	delay := m.delay
	fail := m.failGenerate
	nilCfg := m.nilConfig
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if fail {
		return nil, errors.New("mock config generation failure")
	}

	if nilCfg {
		return &config.EffectiveAgentRuntimeConfig{}, nil
	}

	return &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"read_file", "write_file"},
		Policies:     []string{"policy.mg"},
	}, nil
}

// spawnerMockTransducer implements perception.Transducer with explicit
// failures: no path under test here re-perceives intent, so any call is a
// wiring surprise that must fail loudly. (The previous nil-embedded mock
// would have panicked instead, and only if the stars aligned.)
type spawnerMockTransducer struct{}

func (m *spawnerMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{}, errors.New("spawnerMockTransducer.ParseIntent: not scripted for this test")
}

func (m *spawnerMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return perception.Intent{}, errors.New("spawnerMockTransducer.ParseIntentWithContext: not scripted for this test")
}

func (m *spawnerMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{}, nil, errors.New("spawnerMockTransducer.ParseIntentWithGCD: not scripted for this test")
}

func (m *spawnerMockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, errors.New("spawnerMockTransducer.ResolveFocus: not scripted for this test")
}

func (m *spawnerMockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}

func (m *spawnerMockTransducer) SetStrategicContext(context string) {}

// =============================================================================
// REAL DEPENDENCIES FOR E2E TESTING
// =============================================================================
// To satisfy the integration test requirement, we must cross at least two boundaries.
// We will use a real Spawner, a real TaskExecutor (JITExecutor), and mock only the LLM.

type realTestEnv struct {
	Spawner      *session.Spawner
	Executor     session.TaskExecutor
	MockLLM      *mockRealLLM
	MockCompiler *spawnerMockJITCompiler
	MockConfig   *spawnerMockConfigFactory
}

type mockRealLLM struct {
	mu        sync.Mutex
	responses []string
	idx       int
	// blockCh, when non-nil, parks every completion until it closes or the
	// context ends, simulating a long-running model for stop/cancel tests.
	blockCh chan struct{}
	// toolQueue scripts native function-calling turns. When empty,
	// CompleteWithTools falls back to the text responses so prose-only
	// fixtures behave identically on the native path.
	toolQueue []*types.LLMToolResponse
	toolIdx   int
	// errQueue scripts model-side failures (disconnects, 500s). The next
	// completion of any kind consumes and returns the head error instead
	// of a response, simulating a client that dies mid-turn.
	errQueue []error
}

func (m *mockRealLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return m.CompleteWithSystem(ctx, "", prompt)
}

func (m *mockRealLLM) awaitUnblock(ctx context.Context) error {
	m.mu.Lock()
	ch := m.blockCh
	m.mu.Unlock()
	if ch == nil {
		return nil
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// nextErr consumes one scripted failure. Callers must hold m.mu.
func (m *mockRealLLM) nextErr() error {
	if len(m.errQueue) == 0 {
		return nil
	}
	err := m.errQueue[0]
	m.errQueue = m.errQueue[1:]
	return err
}

// scriptErrors queues model-side failures consumed by the next completions.
func (m *mockRealLLM) scriptErrors(errs ...error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errQueue = append(m.errQueue, errs...)
}

// resetScript replaces the whole script (prose, tool turns, errors) and
// rewinds every cursor, so sequential agent runs each consume a
// deterministic script instead of leftovers.
func (m *mockRealLLM) resetScript(responses []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = responses
	m.idx = 0
	m.toolQueue = nil
	m.toolIdx = 0
	m.errQueue = nil
}

func (m *mockRealLLM) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	if err := m.awaitUnblock(ctx); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.nextErr(); err != nil {
		return "", err
	}
	if m.idx >= len(m.responses) {
		return "{}", nil
	}
	resp := m.responses[m.idx]
	m.idx++
	return resp, nil
}

func (m *mockRealLLM) CompleteWithStreaming(ctx context.Context, system, prompt string, requireSchema bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	errCh := make(chan error)
	go func() {
		defer close(ch)
		defer close(errCh)
		res, err := m.CompleteWithSystem(ctx, system, prompt)
		if err != nil {
			errCh <- err
			return
		}
		ch <- res
	}()
	return ch, errCh
}

func (m *mockRealLLM) CompleteWithTools(ctx context.Context, system, prompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if err := m.awaitUnblock(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.nextErr(); err != nil {
		return nil, err
	}
	if m.toolIdx < len(m.toolQueue) {
		resp := m.toolQueue[m.toolIdx]
		m.toolIdx++
		return resp, nil
	}
	// No scripted tool turn: surface the next prose fixture (or the legacy
	// empty object) so text-only tests behave the same on native generation.
	if m.idx < len(m.responses) {
		resp := m.responses[m.idx]
		m.idx++
		return &types.LLMToolResponse{Text: resp}, nil
	}
	return &types.LLMToolResponse{Text: "{}"}, nil
}

// CompleteWithToolResults continues a scripted native tool conversation. It
// is also a working turn's first call (the working loop sends its request as
// messages), so it serves the next prose fixture the way CompleteWithTools
// does. An exhausted script ends the turn with benign prose instead of
// looping forever.
func (m *mockRealLLM) CompleteWithToolResults(ctx context.Context, system string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if err := m.awaitUnblock(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.nextErr(); err != nil {
		return nil, err
	}
	if m.toolIdx < len(m.toolQueue) {
		resp := m.toolQueue[m.toolIdx]
		m.toolIdx++
		return resp, nil
	}
	if m.idx < len(m.responses) {
		resp := m.responses[m.idx]
		m.idx++
		return &types.LLMToolResponse{Text: resp}, nil
	}
	return &types.LLMToolResponse{Text: "done"}, nil
}

// scriptToolTurns queues native function-calling turns: generation consumes
// from the front, follow-ups continue through the same queue.
func (m *mockRealLLM) scriptToolTurns(turns ...*types.LLMToolResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolQueue = append(m.toolQueue, turns...)
}

func (m *mockRealLLM) Dimensions() int { return 1536 }

// registerSpawnerMockToolsOnce guards the global fixture-tool registration.
// buildToolDefinitions filters the config grant against tools.Global(), so
// the read_file/write_file grant only reaches the model when handlers exist.
// Real tool names (not fictional ones): the constitutional safety gate derives
// permitted(...) from safe_action/1, which knows /read_file and /write_file.
//
// Only read_file is stubbed here, and reads need no landing so canned text is
// honest. write_file comes from the shared write-turn fixture
// (registerWriteTurnTool): the post-action validator checks that a write
// really landed, so a no-op stub would be correctly judged hollow — and a
// second global write_file would race the shared one.
var registerSpawnerMockToolsOnce sync.Once

func registerSpawnerMockTools() {
	registerSpawnerMockToolsOnce.Do(func() {
		_ = tools.Global().Register(&tools.Tool{Name: "read_file", Effect: tools.EffectRead, Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			// Size-aware read: the extreme-payload test needs a real 10MB
			// tool result flowing VS -> Executor -> LLM, not a comment
			// promising one. Ordinary paths get ordinary canned text.
			if path, _ := args["path"].(string); path == "big.bin" {
				return strings.Repeat("x", 10<<20), nil
			}
			return "file contents ok", nil
		}})
	})
}

func setupRealIntegrationEnv(t *testing.T, responses ...string) *realTestEnv {
	t.Helper()
	registerSpawnerMockTools()

	mockLLM := &mockRealLLM{responses: responses}
	mockCompiler := &spawnerMockJITCompiler{}
	mockConfig := &spawnerMockConfigFactory{}
	// A real kernel: the executor asserts turn facts into it as soon as an
	// agent runs, and the nil-embedded mock this used to be panicked on the
	// first Assert (executor.go recordTurn), so every test here that let an
	// agent execute died at baseline.
	mockKernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("real kernel: %v", err)
	}
	// A real VirtualStore wired to the real kernel: write_file carries
	// EffectWrite, and executeToolCall requires an InteractiveExecutiveGate
	// for anything that is not EffectRead. A nil store refuses every write
	// with "mandatory executive gate unavailable".
	vstore := core.NewVirtualStore(nil)
	wireDreamer(vstore, mockKernel)
	mockTransducer := &spawnerMockTransducer{}

	spawnerConfig := session.DefaultSpawnerConfig()
	spawnerConfig.MaxActiveSubagents = 50

	s := session.NewSpawner(
		mockKernel,
		vstore,
		mockLLM,
		mockCompiler,
		mockConfig,
		mockTransducer,
		spawnerConfig,
	)

	// We instantiate a real JITExecutor to cross the Spawner -> Executor boundary
	baseExec := session.NewExecutor(mockKernel, vstore, mockLLM, mockCompiler, mockConfig, mockTransducer)
	// Fixture writes land in a per-test temp workspace, and post-edit build /
	// test / critic verification is off: fixture files live outside any Go
	// module (same precedent as writeTurnExecutorConfig). The safety gate and
	// the landing validator stay ON.
	fixtureCfg := writeTurnExecutorConfig(t)
	baseExec.SetConfig(fixtureCfg)
	s.SetExecutorConfig(&fixtureCfg)
	exec := session.NewJITExecutor(baseExec, s, mockTransducer)

	return &realTestEnv{
		Spawner:      s,
		Executor:     exec,
		MockLLM:      mockLLM,
		MockCompiler: mockCompiler,
		MockConfig:   mockConfig,
	}
}

// =============================================================================
// ADVERSARIAL INTEGRATION TESTS
// =============================================================================

// TestE2E_Session_JITCompilerHangs_SpawnerTimeouts tests Temporal Failure
func TestE2E_Session_JITCompilerHangs_SpawnerTimeouts(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)

	env.MockCompiler.mu.Lock()
	env.MockCompiler.delay = 1 * time.Hour
	env.MockCompiler.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := session.SpawnRequest{
		Name:       "test-agent",
		Task:       "do something",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	start := time.Now()
	_, err := env.Spawner.Spawn(ctx, req)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected error due to context timeout, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected context error, got: %v", err)
	}

	if duration > 1*time.Second {
		t.Errorf("Spawn took too long, context was not respected. Duration: %v", duration)
	}

	ctx2 := context.Background()
	env.MockCompiler.mu.Lock()
	env.MockCompiler.delay = 0
	env.MockCompiler.mu.Unlock()

	var successCount int
	for i := 0; i < 50; i++ {
		_, err := env.Spawner.Spawn(ctx2, req)
		if err == nil {
			successCount++
		}
	}

	if successCount != 50 {
		t.Errorf("Capacity leaked! Only managed to spawn %d after a timeout, expected 50", successCount)
	}
}

// TestE2E_Session_SpawnerConcurrentSpawns_NoStateCorruption tests State Corruption & Resource Exhaustion
func TestE2E_Session_SpawnerConcurrentSpawns_NoStateCorruption(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)

	env.MockCompiler.mu.Lock()
	env.MockCompiler.delay = 10 * time.Millisecond
	env.MockCompiler.mu.Unlock()

	req := session.SpawnRequest{
		Name:       "concurrent-agent",
		Task:       "race me",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/race",
	}

	numSpawns := 200
	var wg sync.WaitGroup
	var successCount int32
	var failCount int32
	var spawnedMu sync.Mutex
	var spawned []*session.SubAgent

	for i := 0; i < numSpawns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			agent, err := env.Spawner.Spawn(context.Background(), req)
			if err != nil {
				atomic.AddInt32(&failCount, 1)
				return
			}
			atomic.AddInt32(&successCount, 1)
			spawnedMu.Lock()
			spawned = append(spawned, agent)
			spawnedMu.Unlock()
		}()
	}

	wg.Wait()

	if successCount != 50 {
		t.Errorf("Expected exactly 50 successful spawns (maxActiveSubagents), got %d", successCount)
	}
	if failCount != 150 {
		t.Errorf("Expected exactly 150 rejected spawns, got %d", failCount)
	}

	// Spawn registers an agent and hands it back idle; the caller runs it. An
	// idle agent holds its capacity slot until it has run to completion, so
	// the registry must hold exactly the fifty that won the race, no more,
	// and none of them is running yet. (Run-to-release is covered by the
	// Execute tests below asserting an empty ListActive after each run.)
	if got := len(spawned); got != 50 {
		t.Errorf("collected %d admitted agents, want 50", got)
	}
	if got := len(env.Spawner.GetMetrics()); got != 50 {
		t.Errorf("registry holds %d agents after the burst, want the 50 that were admitted", got)
	}
	if active := len(env.Spawner.ListActive()); active != 0 {
		t.Errorf("ListActive returned %d before any agent was run, want 0", active)
	}
	if _, err := env.Spawner.Spawn(context.Background(), req); err == nil {
		t.Error("a 51st spawn was admitted while 50 idle agents hold the capacity")
	}
}

// TestE2E_Session_ConfigFactory_Fails_SpawnDegrades tests Partial Pipeline Failure:
// a config-generation failure degrades to an empty fallback config, it does
// not abort the spawn (spawner.go: "Continue with empty config").
func TestE2E_Session_ConfigFactory_Fails_SpawnDegrades(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)

	env.MockConfig.mu.Lock()
	env.MockConfig.failGenerate = true
	env.MockConfig.mu.Unlock()

	req := session.SpawnRequest{
		Name:       "test-agent",
		Task:       "do something",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	agent, err := env.Spawner.Spawn(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error (graceful degradation), got: %v", err)
	}
	if agent == nil {
		t.Fatal("Expected agent to be created despite config generation failure")
	}
}

// TestE2E_Session_SubagentStop_HaltsExecutionLoop tests Recovery
func TestE2E_Session_SubagentStop_HaltsExecutionLoop(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)
	// Park the model mid-turn so there is a running loop to halt. Spawn alone
	// leaves the agent Idle; only Run starts execution.
	env.MockLLM.blockCh = make(chan struct{})

	req := session.SpawnRequest{
		Name:       "test-agent",
		Task:       "do something",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	agent, err := env.Spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	agent.Run(context.Background(), req.Task)
	waitForAgentState(t, agent, session.SubAgentStateRunning, 5*time.Second)

	if err := agent.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	// Stop cancels the run context, so the parked LLM call fails and the
	// agent lands Failed — promptly, not after a timeout.
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = agent.WaitWithContext(waitCtx)

	if agent.GetState() != session.SubAgentStateFailed {
		t.Errorf("Agent state should be Failed after stop mid-run, got %v", agent.GetState())
	}
}

// waitForAgentState polls until the agent reaches want or the timeout fires.
func waitForAgentState(t *testing.T, agent *session.SubAgent, want session.SubAgentState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for agent.GetState() != want {
		if !time.Now().Before(deadline) {
			t.Fatalf("agent %s never reached %v (still %v)", agent.GetName(), want, agent.GetState())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestE2E_Session_StopAll_DuringMassiveActivity tests State Corruption
func TestE2E_Session_StopAll_DuringMassiveActivity(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)
	// Park every model call so all 20 agents stay Running (an instant mock
	// would let each finish before the next spawns).
	env.MockLLM.blockCh = make(chan struct{})

	req := session.SpawnRequest{
		Name:       "test-agent",
		Task:       "do something",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	for i := 0; i < 20; i++ {
		agent, err := env.Spawner.Spawn(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		agent.Run(context.Background(), req.Task)
	}

	deadline := time.Now().Add(10 * time.Second)
	for len(env.Spawner.ListActive()) != 20 {
		if !time.Now().Before(deadline) {
			t.Fatalf("Expected 20 active agents, got %d", len(env.Spawner.ListActive()))
		}
		time.Sleep(5 * time.Millisecond)
	}

	env.Spawner.StopAll()

	deadline = time.Now().Add(10 * time.Second)
	for len(env.Spawner.ListActive()) != 0 {
		if !time.Now().Before(deadline) {
			t.Fatalf("StopAll left %d agents running", len(env.Spawner.ListActive()))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestE2E_Session_JITCompilerReturnsEmptyPrompt_ExecutorHandlesGracefully tests Semantic Failure
func TestE2E_Session_JITCompilerReturnsEmptyPrompt_ExecutorHandlesGracefully(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)

	env.MockCompiler.mu.Lock()
	env.MockCompiler.emptyPrompt = true
	env.MockCompiler.mu.Unlock()

	req := session.SpawnRequest{
		Name:       "test-agent",
		Task:       "do something",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	agent, err := env.Spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatalf("Spawn should gracefully handle empty prompts: %v", err)
	}

	if agent == nil {
		t.Fatal("Agent should not be nil")
	}

	if agent.GetName() != "test-agent" {
		t.Errorf("Expected agent name test-agent, got %s", agent.GetName())
	}
}

// TestE2E_Session_ExecutorMultiTurnStateLeak_IsolatedHistory tests End-to-End Data Integrity
func TestE2E_Session_ExecutorMultiTurnStateLeak_IsolatedHistory(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)

	reqA := session.SpawnRequest{
		Name:       "agent-a",
		Task:       "task a",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	reqB := session.SpawnRequest{
		Name:       "agent-b",
		Task:       "task b",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	agentA, err := env.Spawner.Spawn(context.Background(), reqA)
	if err != nil {
		t.Fatal(err)
	}

	agentB, err := env.Spawner.Spawn(context.Background(), reqB)
	if err != nil {
		t.Fatal(err)
	}

	if agentA.GetID() == agentB.GetID() {
		t.Errorf("Agents should have unique IDs")
	}

	// Behavioral isolation: run each agent alone with a marker script and
	// prove neither result contains the other's marker. The mock is reset
	// between phases, so any cross-marker could only come from leaked
	// agent-side conversation state. (Pure prose would trip the hollow
	// success guard — /test requires completed tool calls — so each phase
	// reads a file and concludes with its marker.)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	phase := func(agent *session.SubAgent, marker string) string {
		t.Helper()
		env.MockLLM.resetScript(nil)
		env.MockLLM.scriptToolTurns(
			&types.LLMToolResponse{Text: "reading", ToolCalls: []types.ToolCall{{ID: "read-1", Name: "read_file", Input: map[string]any{"path": "notes.txt"}}}},
			&types.LLMToolResponse{Text: marker},
		)
		agent.Run(ctx, "read the notes")
		res, err := agent.WaitWithContext(ctx)
		if err != nil {
			t.Fatalf("agent %s run failed: %v", agent.GetName(), err)
		}
		return res
	}

	resA := phase(agentA, "ALPHA-result-marker")
	if !strings.Contains(resA, "ALPHA-result-marker") {
		t.Fatalf("agentA lost its own marker: %q", resA)
	}
	resB := phase(agentB, "BETA-result-marker")
	if !strings.Contains(resB, "BETA-result-marker") {
		t.Fatalf("agentB lost its own marker: %q", resB)
	}
	if strings.Contains(resB, "ALPHA-result-marker") {
		t.Fatalf("agentA state leaked into agentB: %q", resB)
	}
	if strings.Contains(resA, "BETA-result-marker") {
		t.Fatalf("agentB state leaked into agentA: %q", resA)
	}
}

// TestE2E_Executor_ExecutesTask_Successfully integrates the Universal Executor boundary.
// It verifies that a task request routed through the JITExecutor successfully talks to the spawner,
// handles the LLM response, and finishes the task lifecycle correctly.
func TestE2E_Executor_ExecutesTask_Successfully(t *testing.T) {
	t.Parallel()

	// Simulate a tool-using model: /fix is write-oriented, so prose alone is
	// hollow and correctly blocked. The turn reads the buggy file, writes the
	// fix through the shared write-turn stub (which really lands the file —
	// the post-action validator rejects no-op writes), then concludes.
	env := setupRealIntegrationEnv(t)
	writeCall := writeTurnCall(t)
	env.MockLLM.scriptToolTurns(
		&types.LLMToolResponse{Text: "fixing", ToolCalls: []types.ToolCall{
			{ID: "1", Name: "read_file", Input: map[string]any{"path": "bug.go"}},
			writeCall,
		}},
		&types.LLMToolResponse{Text: `{"message": "I fixed it"}`},
	)

	req := session.TaskRequest{
		IntentVerb: "/fix", // persona is expressed through IntentVerb; TaskRequest has no Persona field
		Task:       "fix the bug",
	}

	res, err := env.Executor.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected successful execution, got error: %v", err)
	}

	if !strings.Contains(res, "I fixed it") {
		t.Errorf("Expected response to contain 'I fixed it', got: %s", res)
	}

	// The fix must have landed on disk, not just been reported.
	writtenPath, _ := writeCall.Input["path"].(string)
	content, readErr := os.ReadFile(writtenPath)
	if readErr != nil {
		t.Errorf("Expected written file %s to exist: %v", writtenPath, readErr)
	} else if !strings.Contains(string(content), "write-turn fixture") {
		t.Errorf("Written file %s has unexpected content: %q", writtenPath, content)
	}

	// Verify the agent was properly spawned and cleaned up
	if len(env.Spawner.ListActive()) > 0 {
		t.Errorf("Expected active agent list to be empty after execution")
	}
}

// TestE2E_Executor_HandlesMalformedPiggyback_FromLLM validates the resilient execution loop.
// The Spawner generates config, the Executor spawns the agent, but the LLM returns garbage JSON.
// The Executor must not panic or hang: it degrades to a raw-text fallback and succeeds.
func TestE2E_Executor_HandlesMalformedPiggyback_FromLLM(t *testing.T) {
	t.Parallel()

	// Provide syntactically invalid JSON
	env := setupRealIntegrationEnv(t, `{"tool_call": { unclosed bracket...`)

	req := session.TaskRequest{
		IntentVerb: "/review", // persona is expressed through IntentVerb; TaskRequest has no Persona field
		Task:       "review the PR",
	}

	res, err := env.Executor.Execute(context.Background(), req)

	// Malformed model output degrades gracefully: no control packet is
	// trusted, the raw text is echoed as the surface, and the turn succeeds
	// without panicking, hanging, or leaking the agent.
	if err != nil {
		t.Fatalf("Executor should degrade gracefully on malformed JSON, got error: %v", err)
	}
	if !strings.Contains(res, "unclosed bracket") {
		t.Errorf("Expected raw-text fallback echoing the malformed payload, got: %s", res)
	}
	if len(env.Spawner.ListActive()) != 0 {
		t.Errorf("Expected no leaked agents after malformed turn")
	}
}

// TestE2E_Executor_DelegationWithPriority respects priority queueing during high contention.
// TestE2E_Executor_ExecuteWithContext_Succeeds smokes the priority-carrying
// Execute entrypoint end to end. Priority ORDERING under contention is a
// scheduler property and belongs to the scheduler suite, not here.
func TestE2E_Executor_ExecuteWithContext_Succeeds(t *testing.T) {
	t.Parallel()
	// /test requires side effects, so the scripted turn reads before concluding.
	env := setupRealIntegrationEnv(t)
	env.MockLLM.scriptToolTurns(
		&types.LLMToolResponse{Text: "testing", ToolCalls: []types.ToolCall{{ID: "1", Name: "read_file", Input: map[string]any{"path": "suite_test.go"}}}},
		&types.LLMToolResponse{Text: `{"message": "done"}`},
	)

	req := session.TaskRequest{
		IntentVerb: "/test",
		Task:       "run tests",
	}

	// Call ExecuteWithContext to simulate a high-priority interactive spawn
	_, err := env.Executor.ExecuteWithContext(context.Background(), req, nil, types.PriorityHigh)
	if err != nil {
		t.Fatalf("ExecuteWithContext failed: %v", err)
	}
}

// TestE2E_Executor_CancelMidFlight_HaltsAgent execution
func TestE2E_Executor_CancelMidFlight_HaltsAgent(t *testing.T) {
	t.Parallel()

	// Park the model so the turn genuinely hangs until the context dies.
	// (The old version locked and unlocked the mock mutex and hoped the
	// loop would spin; nothing actually blocked.)
	env := setupRealIntegrationEnv(t)
	env.MockLLM.blockCh = make(chan struct{})

	req := session.TaskRequest{
		IntentVerb: "/research",
		Task:       "long running research",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := env.Executor.Execute(ctx, req)
	duration := time.Since(start)

	if err == nil {
		t.Fatalf("Expected error due to context cancellation")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context error, got: %v", err)
	}

	// The model is parked until the test ends, so returning at all is the
	// context's doing; the bound only tells that apart from another clock
	// (a request timeout is minutes) ending the turn. What remains is the
	// isolated spawn's setup before its first context check: 0.35-0.5s alone
	// and 1.06s under the full suite's parallel load (2026-09-23), which the
	// 1s bound this had could not hold.
	if duration > 5*time.Second {
		t.Errorf("Execution took too long, context was not respected. Duration: %v", duration)
	}
}

// TestE2E_Spawner_OuroborosPatchDrift validates whether active subagents inherit patched configs.
// If Autopoiesis patches a policy, do running agents receive it?
func TestE2E_Spawner_OuroborosPatchDrift(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)

	req := session.SpawnRequest{
		Name:       "patch-test-agent",
		Task:       "do something",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	agent, err := env.Spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate Autopoiesis dynamically removing a tool from the factory
	env.MockConfig.mu.Lock()
	env.MockConfig.nilConfig = true
	env.MockConfig.mu.Unlock()

	// The active agent should retain its JIT snapshot (not dynamically update)
	// This asserts the behavior of configuration drift.
	// We cannot access agent.config, but we can spawn a new one to verify the state changed.

	agent2, err := env.Spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if agent.GetID() == agent2.GetID() {
		t.Errorf("Agents should have unique IDs")
	}

	// Behavioral proof of snapshot isolation: run a read turn on BOTH agents
	// after the flip. agent must still read (it keeps its spawn-time
	// grant); agent2 must be denied (empty grant exposes no tools).
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	readTurn := func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "reading", ToolCalls: []types.ToolCall{{ID: "read-1", Name: "read_file", Input: map[string]any{"path": "notes.txt"}}}}
	}
	conclude := &types.LLMToolResponse{Text: `{"message": "read complete"}`}

	env.MockLLM.resetScript(nil)
	env.MockLLM.scriptToolTurns(readTurn(), conclude)
	agent.Run(ctx, "read the notes")
	res1, err := agent.WaitWithContext(ctx)
	if err != nil {
		t.Fatalf("pre-flip agent lost its grant after the factory flip: %v", err)
	}
	if !strings.Contains(res1, "read complete") {
		t.Fatalf("pre-flip agent read did not complete: %q", res1)
	}

	env.MockLLM.resetScript(nil)
	env.MockLLM.scriptToolTurns(readTurn(), conclude)
	agent2.Run(ctx, "read the notes")
	res2, err := agent2.WaitWithContext(ctx)
	if err == nil {
		t.Fatalf("post-flip agent with an empty grant should have been denied the read, got: %q", res2)
	}
}

// TestE2E_Session_CampaignExecution_SequentialPhases_IndependentResults checks that
// back-to-back campaign phases through one executor stay independent: each
// phase concludes with its own scripted message (no cross-talk) and no agent
// leaks between phases.
func TestE2E_Session_CampaignExecution_SequentialPhases_IndependentResults(t *testing.T) {
	t.Parallel()
	// Both phases use tools (/research requires side effects); each scripted
	// turn reads once, then concludes with its phase message.
	env := setupRealIntegrationEnv(t)
	env.MockLLM.scriptToolTurns(
		&types.LLMToolResponse{Text: "researching", ToolCalls: []types.ToolCall{{ID: "p1", Name: "read_file", Input: map[string]any{"path": "docs.md"}}}},
		&types.LLMToolResponse{Text: `{"message": "Phase 1 complete"}`},
		&types.LLMToolResponse{Text: "auditing", ToolCalls: []types.ToolCall{{ID: "p2", Name: "read_file", Input: map[string]any{"path": "store.go"}}}},
		&types.LLMToolResponse{Text: `{"message": "Phase 2 complete"}`},
	)

	// Simulate a high-memory footprint task in Phase 1
	req1 := session.TaskRequest{
		IntentVerb: "/research", // persona is expressed through IntentVerb; TaskRequest has no Persona field
		Task:       "Ingest and compress 500 pages of technical documentation regarding the VirtualStore",
	}

	// Simulate a precise constraint task in Phase 2
	req2 := session.TaskRequest{
		IntentVerb: "/review", // persona is expressed through IntentVerb; TaskRequest has no Persona field
		Task:       "Audit the VirtualStore modifications against security policy",
	}

	ctx := context.Background()

	// Phase 1 Execution
	res1, err := env.Executor.Execute(ctx, req1)
	if err != nil {
		t.Fatalf("Phase 1 execution failed: %v", err)
	}
	if !strings.Contains(res1, "Phase 1 complete") {
		t.Errorf("Unexpected LLM response for Phase 1: %s", res1)
	}

	// Phase 2 Execution
	res2, err := env.Executor.Execute(ctx, req2)
	if err != nil {
		t.Fatalf("Phase 2 execution failed: %v", err)
	}
	if !strings.Contains(res2, "Phase 2 complete") {
		t.Errorf("Unexpected LLM response for Phase 2: %s", res2)
	}

	// Post-Condition: Check Spawner Cleanup
	if len(env.Spawner.ListActive()) != 0 {
		t.Errorf("Spawner did not correctly clean up agents after sequential campaign phases")
	}
}

// TestE2E_Session_ToolAllowlist_UnregisteredTool_Rejection proves that when the
// model invokes a tool outside its grant, the executor's allowlist gate denies
// it before any backend resolution, and the turn fails with a permission-class
// error. (True Dreamer-sandbox rejection is covered in
// dreamer_virtualstore_integration_test.go; this path never reaches simulation.)
func TestE2E_Session_ToolAllowlist_UnregisteredTool_Rejection(t *testing.T) {
	t.Parallel()

	// The LLM attempts to invoke a forbidden tool. system_exec exists nowhere
	// in any registry, so the attempt is doubly safe: the JIT allowlist gate
	// denies it before any backend resolution is even attempted.
	env := setupRealIntegrationEnv(t)
	env.MockLLM.scriptToolTurns(
		&types.LLMToolResponse{Text: "cleaning", ToolCalls: []types.ToolCall{{ID: "evil", Name: "system_exec", Input: map[string]any{"cmd": "rm -rf /"}}}},
		&types.LLMToolResponse{Text: "done"},
	)

	// The fixture grant covers read_file/write_file only: system_exec must fail
	// at the executor's allowlist gate, never reaching any backend.

	req := session.TaskRequest{
		IntentVerb: "/test",
		Task:       "Clean up the build directory",
	}

	_, err := env.Executor.ExecuteWithContext(context.Background(), req, nil, types.PriorityLow)

	// Even though the LLM generated syntactically valid JSON for the tool call,
	// the system should intercept the unauthorized tool execution attempt and return an error
	// wrapping the ToolPermissionError or parse/retry limit error.
	if err == nil {
		t.Fatalf("Expected execution to fail due to sandbox/permission violation on system_exec")
	}

	if !strings.Contains(err.Error(), "permission") && !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "tool") {
		t.Errorf("Expected permission or tool retry error, got: %v", err)
	}
}

// TestE2E_Session_ToolGating_DeniesUngrantedTool proves the first layer of
// defense: a model-invoked tool outside the fixture grant is denied at the
// executor's tool gating layer before it can reach the kernel. (Kernel-side
// rule validation lives in autopoiesis_kernel_ouroboros_integration_test.go.)
func TestE2E_Session_ToolGating_DeniesUngrantedTool(t *testing.T) {
	t.Parallel()

	// The LLM attempts an ungranted tool. assert_rule is outside the fixture
	// grant (read_file, write_file), so the executor's tool gating layer
	// denies it before it can reach the kernel — the first layer of defense.
	env := setupRealIntegrationEnv(t)
	env.MockLLM.scriptToolTurns(
		&types.LLMToolResponse{Text: "formulating", ToolCalls: []types.ToolCall{{ID: "r1", Name: "assert_rule", Input: map[string]any{"rule": "p(X) :- not p(X)."}}}},
		&types.LLMToolResponse{Text: "done"},
	)

	// /test requires side effects (unlike /learn, which has no action mapping
	// and is hollow-exempt), so the denied call fails the turn instead of
	// degrading to prose.
	req := session.TaskRequest{
		IntentVerb: "/test",
		Task:       "Formulate a new architectural rule",
	}

	_, err := env.Executor.Execute(context.Background(), req)

	// assert_rule is outside the fixture grant (read_file, write_file), so the
	// turn fails at the tool gating layer before reaching the kernel.
	if err == nil {
		t.Fatalf("Expected failure due to tool permission denial on assert_rule")
	}
}

// TestE2E_Session_ExtremePayload_MemoryBounds_Enforcement verifies that
// transferring a massive payload (e.g. 10MB file content) from the VirtualStore
// back through the Executor to the LLM does not crash the system.
func TestE2E_Session_ExtremePayload_MemoryBounds_Enforcement(t *testing.T) {
	t.Parallel()

	env := setupRealIntegrationEnv(t)

	// A real 10MB tool result flows read_file(big.bin) -> VirtualStore ->
	// Executor -> model context. The turn must complete without error,
	// panic, or truncation of the conclude marker.
	env.MockLLM.scriptToolTurns(
		&types.LLMToolResponse{Text: "reading big", ToolCalls: []types.ToolCall{{ID: "read-big", Name: "read_file", Input: map[string]any{"path": "big.bin"}}}},
		&types.LLMToolResponse{Text: `{"message": "Big payload test complete"}`},
	)
	req := session.TaskRequest{
		IntentVerb: "/test",
		Task:       "Process the database dump",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := env.Executor.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute failed on 10MB tool result: %v", err)
	}
	if !strings.Contains(res, "Big payload test complete") {
		t.Fatalf("Expected big-payload completion, got %q", res)
	}
}

// TestE2E_Session_LLMClient_NetworkDisconnect_MidStream tests the pipeline's
// resilience if the LLM client encounters a network partition while streaming
// the response back to the Executor.
func TestE2E_Session_LLMClient_NetworkDisconnect_MidTurn(t *testing.T) {
	t.Parallel()
	env := setupRealIntegrationEnv(t)

	// One healthy tool turn lands, then the model connection dies. The turn
	// must surface the failure (not hang, not silently conclude) and
	// release its agent.
	env.MockLLM.scriptToolTurns(
		&types.LLMToolResponse{Text: "reading", ToolCalls: []types.ToolCall{{ID: "read-1", Name: "read_file", Input: map[string]any{"path": "notes.txt"}}}},
	)
	env.MockLLM.scriptErrors(errors.New("network disconnect: connection reset by peer"))
	req := session.TaskRequest{
		IntentVerb: "/test",
		Task:       "Process the database dump",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := env.Executor.Execute(ctx, req)
	if err == nil {
		t.Fatal("Expected Execute to surface the mid-turn disconnect, got success")
	}
	if !strings.Contains(err.Error(), "disconnect") {
		t.Fatalf("Expected the disconnect error to surface, got: %v", err)
	}
	if active := env.Spawner.ListActive(); len(active) != 0 {
		t.Fatalf("Agent leaked after disconnect: %v", active)
	}
}

// ResolveAllowedTools projects the same fixture envelope before JIT selection.
func (m *spawnerMockConfigFactory) ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error) {
	resolved, err := m.Generate(ctx, &prompt.CompilationResult{}, intents...)
	if err != nil || resolved == nil {
		return nil, err
	}
	return append([]string(nil), resolved.AllowedTools...), nil
}

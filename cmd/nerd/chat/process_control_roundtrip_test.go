package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

type processControlRecordingKernel struct {
	*core.RealKernel
	mu      sync.Mutex
	asserts []core.Fact
	loads   []core.Fact
	retract []core.Fact
}

func (k *processControlRecordingKernel) Assert(fact core.Fact) error {
	k.mu.Lock()
	k.asserts = append(k.asserts, fact)
	k.mu.Unlock()
	return k.RealKernel.Assert(fact)
}

func (k *processControlRecordingKernel) LoadFacts(facts []core.Fact) error {
	k.mu.Lock()
	k.loads = append(k.loads, facts...)
	k.mu.Unlock()
	return k.RealKernel.LoadFacts(facts)
}

func (k *processControlRecordingKernel) RetractFact(fact core.Fact) error {
	k.mu.Lock()
	k.retract = append(k.retract, fact)
	k.mu.Unlock()
	return k.RealKernel.RetractFact(fact)
}

func (k *processControlRecordingKernel) counts(predicate string) (asserted, loaded, retracted int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, fact := range k.asserts {
		if fact.Predicate == predicate {
			asserted++
		}
	}
	for _, fact := range k.loads {
		if fact.Predicate == predicate {
			loaded++
		}
	}
	for _, fact := range k.retract {
		if fact.Predicate == predicate {
			retracted++
		}
	}
	return asserted, loaded, retracted
}

type processControlTaskExecutor struct {
	mu       sync.Mutex
	calls    int
	check    func() error
	response string
}

func (e *processControlTaskExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	return e.ExecuteWithContext(ctx, req, nil, types.PriorityNormal)
}

func (e *processControlTaskExecutor) ExecuteWithContext(ctx context.Context, _ session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	e.mu.Lock()
	e.calls++
	check := e.check
	response := e.response
	e.mu.Unlock()
	if check != nil {
		if err := check(); err != nil {
			return "", err
		}
	}
	return response, nil
}

func (e *processControlTaskExecutor) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", fmt.Errorf("unexpected async execution")
}

func (e *processControlTaskExecutor) GetResult(string) (string, bool, error) {
	return "", false, fmt.Errorf("unexpected result lookup")
}

func (e *processControlTaskExecutor) WaitForResult(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected wait")
}

func (e *processControlTaskExecutor) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

func TestProcessInputControlPacketIsGovernedBeforeKnowledgeConsultation(t *testing.T) {
	workspace := SetupLiveWorkspace(t)
	baseKernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("create kernel: %v", err)
	}
	kernel := &processControlRecordingKernel{RealKernel: baseKernel}
	client := NewMockLLMClient()

	intent := perception.Intent{
		Category:   "/query",
		Verb:       "/explain",
		Target:     "kernel",
		Confidence: 0.9,
		Response:   "I will explain the kernel.",
	}
	client.SetDefaultResponse(articulationResponseForTest(
		t,
		"I will consult a specialist.",
		intent,
		[]string{
			"observation(\"chat_control_probe\", \"safe observation\").",
			"permitted(/chat_control_forbidden, \"target\", \"payload\").",
			"user_intent(/model_injected_intent, /mutation, /fix, \"secret.go\", \"\").",
			"build_state(/chat_model_claimed_passing).",
			"candidate_action(/chat_control_forbidden).",
		},
		[]map[string]string{{"op": "note", "key": "chat_control_probe", "value": "stored once"}},
		true,
		[]map[string]string{{"specialist": "researcher", "query": "kernel policy", "purpose": "answer the question", "priority": "required"}},
	))

	m := NewTestModel(WithSize(100, 50))
	t.Cleanup(m.shutdownCancel)
	m.kernel = kernel
	m.workspace = workspace
	m.client = client
	m.transducer = &clarifyStubTransducer{intent: intent}
	m.virtualStore = core.NewVirtualStore(nil)
	compressorKernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("create compressor kernel: %v", err)
	}
	m.compressor = ctxcompress.NewCompressor(compressorKernel, nil, client)
	m.compressor.SetKernelReader(kernel)

	executor := &processControlTaskExecutor{response: "The constitution controls permissions."}
	executor.check = func() error {
		hasObservation := false
		observations, err := kernel.Query("observation")
		if err != nil {
			return fmt.Errorf("query observation: %w", err)
		}
		for _, fact := range observations {
			if len(fact.Args) == 2 && fact.Args[0] == "chat_control_probe" && fact.Args[1] == "safe observation" {
				hasObservation = true
			}
		}
		if !hasObservation {
			return fmt.Errorf("specialist started before the allowed observation was asserted")
		}

		for _, predicate := range []string{"permitted", "build_state", "candidate_action"} {
			facts, err := kernel.Query(predicate)
			if err != nil {
				return fmt.Errorf("query %s: %w", predicate, err)
			}
			for _, fact := range facts {
				if len(fact.Args) > 0 && strings.Contains(fmt.Sprint(fact.Args[0]), "chat_control_forbidden") {
					return fmt.Errorf("specialist saw forbidden %s fact %s", predicate, fact)
				}
				if predicate == "build_state" && len(fact.Args) > 0 && fact.Args[0] == "/chat_model_claimed_passing" {
					return fmt.Errorf("specialist saw model-authored build witness %s", fact)
				}
			}
		}

		intents, err := kernel.Query("user_intent")
		if err != nil {
			return fmt.Errorf("query user_intent: %w", err)
		}
		for _, fact := range intents {
			if len(fact.Args) > 0 && fact.Args[0] == "/model_injected_intent" {
				return fmt.Errorf("specialist saw model-authored user intent %s", fact)
			}
		}

		notes, err := kernel.Query("session_note")
		if err != nil {
			return fmt.Errorf("query session_note: %w", err)
		}
		noteCount := 0
		for _, fact := range notes {
			if len(fact.Args) == 2 && fact.Args[0] == "chat_control_probe" && fact.Args[1] == "stored once" {
				noteCount++
			}
		}
		if noteCount != 1 {
			return fmt.Errorf("specialist saw %d matching session notes, want one", noteCount)
		}

		corrections, err := kernel.Query("self_correction")
		if err != nil {
			return fmt.Errorf("query self-correction: %w", err)
		}
		hasCorrection := false
		for _, fact := range corrections {
			if len(fact.Args) == 2 && fact.Args[0] == "chat control probe" {
				hasCorrection = true
			}
		}
		if !hasCorrection {
			return fmt.Errorf("specialist started before self-correction was asserted")
		}
		return nil
	}
	m.taskExecutor = executor

	msg := awaitProcessStream(t, m.processInput("explain kernel policy")())
	knowledgeMsg, ok := msg.(knowledgeGatheredMsg)
	if !ok {
		t.Fatalf("processInput returned %T, want knowledgeGatheredMsg", msg)
	}
	if !strings.Contains(knowledgeMsg.InterimResponse, "BLOCKED by Constitution") {
		t.Errorf("candidate action was not denied by the kernel before knowledge consultation: %q", knowledgeMsg.InterimResponse)
	}
	m.goroutineWg.Wait()
	if calls := executor.callCount(); calls != 1 {
		t.Fatalf("specialist execution count = %d, want one", calls)
	}

	compressedTurn := awaitCompressedTurn(t, m.compressor)
	if len(compressedTurn.MemoryOperations) != 1 || compressedTurn.MemoryOperations[0].Op != "note" || compressedTurn.MemoryOperations[0].Key != "chat_control_probe" || compressedTurn.MemoryOperations[0].Value != "stored once" {
		t.Errorf("compressed memory-operation metadata = %+v, want the governed note operation", compressedTurn.MemoryOperations)
	}
	compressedUpdates := make(map[string]int)
	for _, update := range compressedTurn.MangleUpdates {
		fact, err := core.ParseSingleFact(update)
		if err != nil {
			t.Errorf("compressed Mangle update %q is invalid: %v", update, err)
			continue
		}
		compressedUpdates[fact.Predicate]++
		switch fact.Predicate {
		case "permitted", "user_intent", "build_state":
			t.Errorf("blocked model-authored %s was retained in compressed controls: %s", fact.Predicate, fact)
		}
	}
	if compressedUpdates["observation"] != 1 || compressedUpdates["candidate_action"] != 0 {
		t.Errorf("compressed governed update counts = %v, want one observation and no denied candidate proposal", compressedUpdates)
	}
	if len(compressedTurn.MangleUpdates) != 1 {
		t.Errorf("compressed Mangle updates = %v, want only the allowed observation", compressedTurn.MangleUpdates)
	}

	for predicate, want := range map[string]int{"observation": 0, "session_note": 0, "candidate_action": 0} {
		facts, err := compressorKernel.Query(predicate)
		if err != nil {
			t.Fatalf("query compressor %s: %v", predicate, err)
		}
		if len(facts) != want {
			t.Errorf("compressor kernel has %d %s facts after recording controls, want %d (no replay)", len(facts), predicate, want)
		}
	}

	if asserted, loaded, _ := kernel.counts("observation"); asserted != 1 || loaded != 0 {
		t.Errorf("allowed observation entered kernel via Assert=%d LoadFacts=%d, want one Assert and no LoadFacts", asserted, loaded)
	}
	for _, predicate := range []string{"permitted", "build_state"} {
		if asserted, loaded, _ := kernel.counts(predicate); asserted != 0 || loaded != 0 {
			t.Errorf("model-authored %s entered through Assert=%d LoadFacts=%d", predicate, asserted, loaded)
		}
	}
	if asserted, loaded, retracted := kernel.counts("candidate_action"); asserted != 1 || loaded != 0 || retracted != 1 {
		t.Errorf("candidate proposal effects Assert=%d LoadFacts=%d RetractFact=%d, want temporary 1/0/1", asserted, loaded, retracted)
	}
	if asserted, loaded, retracted := kernel.counts("session_note"); asserted != 1 || loaded != 0 || retracted != 1 {
		t.Errorf("session note effects Assert=%d LoadFacts=%d RetractFact=%d, want 1/0/1", asserted, loaded, retracted)
	}
	if asserted, loaded, _ := kernel.counts("self_correction"); asserted != 1 || loaded != 0 {
		t.Errorf("self-correction effects Assert=%d LoadFacts=%d, want one Assert and no LoadFacts", asserted, loaded)
	}
}

func TestProcessInputMutationWithoutVerifierDoesNotClaimCompletion(t *testing.T) {
	workspace := SetupLiveWorkspace(t)
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("create kernel: %v", err)
	}
	client := NewMockLLMClient()

	intent := perception.Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "README.md",
		Confidence: 0.95,
		Response:   "I can help with that.",
	}
	client.SetDefaultResponse(articulationResponseForTest(
		t,
		"I can help with that.",
		intent,
		nil,
		nil,
		false,
		nil,
	))

	m := NewTestModel(WithSize(100, 50))
	t.Cleanup(m.shutdownCancel)
	m.kernel = kernel
	m.workspace = workspace
	m.client = client
	m.transducer = &clarifyStubTransducer{intent: intent}
	m.virtualStore = core.NewVirtualStore(nil)
	executor := &processControlTaskExecutor{response: "README.md was fixed."}
	m.taskExecutor = executor

	msg := m.processInput("fix typo in README.md")()
	result, ok := msg.(assistantMsg)
	if !ok {
		t.Fatalf("processInput returned %T, want assistantMsg", msg)
	}
	if strings.Contains(result.Surface, "Fix Applied") || strings.Contains(result.Surface, "Refactoring Complete") || strings.Contains(result.Surface, "✅") {
		t.Fatalf("unverified mutation claimed completion: %q", result.Surface)
	}
	if !strings.Contains(result.Surface, "no task verifier is configured") || !strings.Contains(result.Surface, "No completion is claimed") {
		t.Fatalf("missing verifier outcome is not explicit: %q", result.Surface)
	}
	if calls := executor.callCount(); calls != 0 {
		t.Fatalf("task executor ran %d times without a verifier; want fail-closed refusal", calls)
	}
}

func TestFormatUnverifiedDelegationResponseNeverClaimsCompletion(t *testing.T) {
	intent := perception.Intent{Verb: "/fix"}
	for _, attempted := range []bool{false, true} {
		response := formatUnverifiedDelegationResponse(intent, "coder", "verification evidence is incomplete", attempted)
		if strings.Contains(response, "Fix Applied") || strings.Contains(response, "✅") {
			t.Errorf("attempted=%v rendered an unverified result optimistically: %q", attempted, response)
		}
		if !strings.Contains(response, "No completion is claimed") {
			t.Errorf("attempted=%v response omitted the missing completion claim: %q", attempted, response)
		}
	}
}

func articulationResponseForTest(
	t *testing.T,
	surface string,
	intent perception.Intent,
	mangleUpdates []string,
	memoryOperations []map[string]string,
	selfCorrection bool,
	knowledgeRequests []map[string]string,
) string {
	t.Helper()
	response := map[string]any{
		"surface_response": surface,
		"control_packet": map[string]any{
			"intent_classification": map[string]any{
				"category":   intent.Category,
				"verb":       intent.Verb,
				"target":     intent.Target,
				"constraint": intent.Constraint,
				"confidence": intent.Confidence,
			},
			"mangle_updates":    mangleUpdates,
			"memory_operations": memoryOperations,
			"self_correction": map[string]any{
				"triggered":  selfCorrection,
				"hypothesis": "chat control probe",
			},
			"decision_summary":   "",
			"knowledge_requests": knowledgeRequests,
			"context_feedback": map[string]any{
				"overall_usefulness": 0,
				"helpful_facts":      []string{},
				"noise_facts":        []string{},
				"missing_context":    "",
			},
			"tool_requests": []any{},
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal articulation response: %v", err)
	}
	return string(data)
}

func awaitProcessStream(t *testing.T, msg tea.Msg) tea.Msg {
	t.Helper()
	stream, ok := msg.(streamStartMsg)
	if !ok {
		return msg
	}
	streamChan := stream.streamChan
	thoughtsChan := stream.thoughtsChan
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case _, open := <-streamChan:
			if !open {
				streamChan = nil
			}
		case _, open := <-thoughtsChan:
			if !open {
				thoughtsChan = nil
			}
		case result := <-stream.resultChan:
			return result
		case err := <-stream.errChan:
			if err != nil {
				t.Fatalf("articulation failed: %v", err)
			}
		case <-timer.C:
			t.Fatal("articulation stream did not complete")
		}
	}
}

func awaitCompressedTurn(t *testing.T, compressor *ctxcompress.Compressor) ctxcompress.CompressedTurn {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		state := compressor.GetState()
		if state != nil && len(state.RecentTurns) > 0 {
			return state.RecentTurns[len(state.RecentTurns)-1]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("compressor did not record the articulation turn")
	return ctxcompress.CompressedTurn{}
}

// TestArticulationPrompt_AsksForDecisionSummary pins the control packet field
// the chat articulation prompt teaches: a short decision_summary, never the
// retired free-form reasoning_trace.
func TestArticulationPrompt_AsksForDecisionSummary(t *testing.T) {
	intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "auth.go"}
	client := NewMockLLMClient()
	_, _ = articulateWithConversation(context.Background(), client, intent,
		payloadForArticulation(intent, nil), nil, "sys", &ConversationContext{}, nil, nil)
	client.mu.Lock()
	prompt := client.lastPrompt
	client.mu.Unlock()

	if !strings.Contains(prompt, `"decision_summary": "one or two sentences: what you decided this turn and why"`) {
		t.Fatalf("articulation prompt does not ask for a decision_summary:\n%s", prompt)
	}
	if strings.Contains(prompt, "reasoning_trace") {
		t.Fatalf("articulation prompt still names the retired reasoning_trace:\n%s", prompt)
	}
}

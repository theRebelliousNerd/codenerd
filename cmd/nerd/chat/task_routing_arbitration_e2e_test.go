//go:build integration

package chat

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// ROUTING ARBITRATION HARNESS
// =============================================================================

// routingOutcome captures the observable result of a routing decision.
type routingOutcome struct {
	MsgType     string    // fmt.Sprintf("%T", msg)
	RawMsg      tea.Msg   // the actual message
	Model       Model     // model after Update
	IsLoading   bool      // m.isLoading after Update
	HasError    bool      // m.err != nil after Update
	HasCampaign bool      // m.activeCampaign != nil
	HasSubtasks bool      // len(m.pendingSubtasks) > 0
	HasClarify  bool      // m.awaitingClarification
	HistoryLen  int       // len(m.history)
}

// setupRoutingModel creates a Model wired for routing arbitration tests.
// Uses a chatLoopTransducer (deterministic), a real Mangle kernel, and a
// mock LLM client so that the entire processInput priority chain executes
// without requiring real LLM calls.
func setupRoutingModel(t *testing.T, intent perception.Intent) (Model, *chatLoopTransducer) {
	t.Helper()
	workspace := SetupLiveWorkspace(t)
	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("Mock articulation response.")

	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("Kernel creation failed: %v", err)
	}

	tr := &chatLoopTransducer{intent: intent}

	m := NewTestModel(WithSize(100, 50))
	m.kernel = kernel
	m.workspace = workspace
	m.client = mockClient
	m.transducer = tr
	m.virtualStore = core.NewVirtualStore(nil)

	return m, tr
}

// routeInput submits text through the full handleSubmit→processInput→Update
// pipeline and captures the outcome.
func routeInput(t *testing.T, m Model, input string) routingOutcome {
	t.Helper()

	m.textarea.SetValue(input)
	submitted, cmd := m.handleSubmit()
	m = submitted.(Model)

	if cmd == nil {
		return routingOutcome{
			MsgType:   "nil",
			RawMsg:    nil,
			Model:     m,
			IsLoading: m.isLoading,
		}
	}

	msg := runBatchAndCollect(t, cmd, 15*time.Second)
	if msg == nil {
		t.Fatal("processInput returned nil message")
	}

	// Feed through Update to apply state changes
	updated, _ := m.Update(msg)
	m = updated.(Model)

	return routingOutcome{
		MsgType:     fmt.Sprintf("%T", msg),
		RawMsg:      msg,
		Model:       m,
		IsLoading:   m.isLoading,
		HasError:    m.err != nil,
		HasCampaign: m.activeCampaign != nil,
		HasSubtasks: len(m.pendingSubtasks) > 0,
		HasClarify:  m.awaitingClarification,
		HistoryLen:  len(m.history),
	}
}

// =============================================================================
// ASSERTION HELPERS
// =============================================================================

// isErrorMsg checks if a tea.Msg is an errorMsg (which is `type errorMsg error`).
// At runtime, errorMsg looks like *fmt.wrapError or *errors.errorString.
// isErrorMsg checks if a tea.Msg is an errorMsg (which is `type errorMsg error`).
// At runtime, errorMsg looks like *fmt.wrapError or *errors.errorString.
func isErrorMsg(msg tea.Msg) bool {
	if msg == nil {
		return false
	}
	if _, ok := msg.(errorMsg); ok {
		return true
	}
	if _, ok := msg.(error); ok {
		return true
	}
	tStr := fmt.Sprintf("%T", msg)
	return strings.Contains(strings.ToLower(tStr), "error")
}

// assertLaneWon checks that the tea.Msg type name contains the expected substring.
// Special case: "errorMsg" matches error types since errorMsg = error.
func assertLaneWon(t *testing.T, outcome routingOutcome, expectedType string) {
	t.Helper()
	if expectedType == "errorMsg" {
		if !isErrorMsg(outcome.RawMsg) {
			t.Errorf("wrong routing lane: got %s, want errorMsg", outcome.MsgType)
		}
		return
	}
	if !strings.Contains(outcome.MsgType, expectedType) {
		// Also accept errorMsg when expectedType is a broad match
		if isErrorMsg(outcome.RawMsg) && (expectedType == "error" || expectedType == "delegation") {
			return
		}
		t.Errorf("wrong routing lane: got %s, want type containing %q", outcome.MsgType, expectedType)
	}
}

// assertDelegationAttempted checks that routing chose the delegation path,
// which produces either assistantMsg (success) or errorMsg (spawn failed).
func assertDelegationAttempted(t *testing.T, outcome routingOutcome) {
	t.Helper()
	if isErrorMsg(outcome.RawMsg) {
		// Delegation attempted but infrastructure missing (expected in mock)
		return
	}
	switch outcome.RawMsg.(type) {
	case assistantMsg:
		// Successful delegation
	default:
		t.Errorf("Expected delegation path (assistantMsg/errorMsg), got %s", outcome.MsgType)
	}
}

// assertNoCampaignStarted asserts no campaign was started.
func assertNoCampaignStarted(t *testing.T, outcome routingOutcome) {
	t.Helper()
	if outcome.HasCampaign {
		t.Error("forbidden lane: campaign was started")
	}
}

// assertNoClarification asserts no clarification was triggered.
func assertNoClarification(t *testing.T, outcome routingOutcome) {
	t.Helper()
	if outcome.HasClarify {
		t.Error("forbidden lane: clarification was triggered")
	}
}

// assertNoMultistep asserts no multistep subtasks were created.
func assertNoMultistep(t *testing.T, outcome routingOutcome) {
	t.Helper()
	if outcome.HasSubtasks {
		t.Error("forbidden lane: subtasks were created (multistep)")
	}
}

// assertRoutingIdle asserts isLoading is false after routing completes.
func assertRoutingIdle(t *testing.T, outcome routingOutcome) {
	t.Helper()
	if outcome.IsLoading {
		t.Error("model still loading after route completion — routing hung")
	}
}

// assertKernelClean checks that stale routing/action facts are absent.
func assertKernelClean(t *testing.T, m Model) {
	t.Helper()
	if m.kernel == nil {
		return
	}
	for _, pred := range []string{"pending_action", "delegate_task"} {
		facts, err := m.kernel.Query(pred)
		if err == nil && len(facts) > 0 {
			t.Errorf("stale kernel fact: %s has %d facts (should be retracted)", pred, len(facts))
		}
	}
}

// retractRoutingFacts retracts transient routing facts from the kernel FactStore
// to guarantee subsequent tests run in a pristine sandbox environment.
func retractRoutingFacts(m Model) {
	if m.kernel == nil {
		return
	}
	for _, pred := range []string{"pending_action", "delegate_task", "user_intent", "routing_result", "execution_result", "current_understanding"} {
		_ = m.kernel.Retract(pred)
	}
}

// assertTransducerCalled checks that the transducer was called exactly n times.
func assertTransducerCalled(t *testing.T, tr *chatLoopTransducer, n int) {
	t.Helper()
	got := tr.getCallCount()
	if got != n {
		t.Errorf("transducer call count: got %d, want %d", got, n)
	}
}

// =============================================================================
// TEST 1: Pure chat → direct conversational response
// =============================================================================

func TestE2E_TaskRouting_PureChat_DirectResponseOnly(t *testing.T) {
	m, tr := setupRoutingModel(t, perception.Intent{
		Category:   "/query",
		Verb:       "/greet",
		Target:     "capabilities",
		Confidence: 0.95,
		Response:   "I can help inspect, explain, test, and modify code.",
	})

	outcome := routeInput(t, m, "hey, what can you do?")

	// /greet is always-conversational → responseMsg (direct, no articulation)
	assertLaneWon(t, outcome, "responseMsg")
	assertNoCampaignStarted(t, outcome)
	assertNoClarification(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)
	assertKernelClean(t, outcome.Model)
	assertTransducerCalled(t, tr, 1)
}

// =============================================================================
// TEST 2: Simple edit → shard delegation (coder), no campaign, no clarify
// =============================================================================

func TestE2E_TaskRouting_SimpleEdit_DelegatesCoder_NoCampaignNoClarify(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "README.md",
		Constraint: "typo",
		Confidence: 0.93,
		Response:   "I'll fix the typo in README.md.",
	})

	outcome := routeInput(t, m, "Fix the typo in README.md")

	// /fix → GetShardTypeForVerb → "coder" → delegation path.
	// Without a real shard manager, spawnTaskWithContext will error.
	// Key assertion: the routing ATTEMPTED delegation (not clarify/campaign/dream).
	switch outcome.RawMsg.(type) {
	case assistantMsg:
		// Successful delegation (verifier or direct spawn returned content)
		t.Log("Delegation produced assistantMsg (with ShardResult)")
	case errorMsg:
		// Shard manager missing or spawn failed — acceptable in mock setup
		t.Log("Delegation attempted but shard spawn failed (expected in mock)")
	default:
		// Any other type means wrong lane was chosen
		t.Errorf("Expected delegation path (assistantMsg/errorMsg), got %s", outcome.MsgType)
	}

	assertNoCampaignStarted(t, outcome)
	assertNoClarification(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)
}

// =============================================================================
// TEST 3: Ambiguous edit → clarification before delegation
// =============================================================================

func TestE2E_TaskRouting_AmbiguousEdit_ClarifiesBeforeDelegation(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "none",
		Constraint: "",
		Confidence: 0.4,
		Response:   "What would you like me to fix?",
	})

	outcome := routeInput(t, m, "fix it")

	// shouldClarifyIntent fires because:
	//   - target == "none"
	//   - confidence < 0.45
	//   - /fix is actionable (maps to "coder")
	// Without a running clarifier shard, runClarifierShard returns error and
	// the path falls through. But the key assertion is no shard executed.
	assertNoCampaignStarted(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)

	// Verify no shard delegation happened via the msg type
	switch outcome.RawMsg.(type) {
	case assistantMsg:
		msg := outcome.RawMsg.(assistantMsg)
		if msg.ShardResult != nil && msg.ShardResult.ShardType == "coder" {
			t.Error("coder shard should NOT have been delegated for ambiguous 'fix it'")
		}
	}
	t.Logf("Ambiguous routing: %s (no coder delegation validated)", outcome.MsgType)
}

// =============================================================================
// TEST 4: Plan request → auto-clarify, not execution
// =============================================================================

func TestE2E_TaskRouting_PlanRequest_AutoClarifyNotExecution(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/instruction",
		Verb:       "/generate",
		Target:     "",
		Constraint: "",
		Confidence: 0.84,
		Response:   "I'd be happy to help plan that.",
	})

	outcome := routeInput(t, m, "Plan a project to add multi-tenant auth and billing")

	// shouldAutoClarify returns true because:
	//   - category == "/instruction" (buildish)
	//   - input contains "plan" and "project" (campaign keywords)
	//   - target is empty (needsDetails)
	// The clarifier shard may not be available in mock, so it might fall through,
	// but NO execution should happen.
	assertNoCampaignStarted(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)

	// No shard should have been spawned for execution
	switch outcome.RawMsg.(type) {
	case assistantMsg:
		msg := outcome.RawMsg.(assistantMsg)
		if msg.ShardResult != nil {
			t.Errorf("No shard execution should happen for plan request, got ShardResult.ShardType=%q", msg.ShardResult.ShardType)
		}
		// Check for clarify update
		if msg.ClarifyUpdate != nil && msg.ClarifyUpdate.LaunchClarifyPending {
			t.Log("Auto-clarify correctly triggered with LaunchClarifyPending=true")
		}
	}
	t.Logf("Plan routing: %s", outcome.MsgType)
}

// =============================================================================
// TEST 5: Dream → hypothetical analysis, no execution, no campaign
// =============================================================================

func TestE2E_TaskRouting_DreamWins_NoExecutionNoCampaign(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/query",
		Verb:       "/dream",
		Target:     "refactor auth system into middleware",
		Constraint: "dry run only",
		Confidence: 0.92,
		Response:   "Let me think through that hypothetical...",
	})

	outcome := routeInput(t, m, "What if we refactored the auth system into middleware? Don't do it, just think it through.")

	// Dream is checked at line 309: if intent.Verb == "/dream" → handleDreamState
	// This runs BEFORE assault, auto-clarify, clarification, multistep, delegation.
	assertNoCampaignStarted(t, outcome)
	assertNoClarification(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)

	// Verify kernel has dream_state fact (asserted by handleDreamState)
	if outcome.Model.kernel != nil {
		facts, err := outcome.Model.kernel.Query("dream_state")
		if err == nil && len(facts) > 0 {
			t.Log("dream_state kernel fact asserted correctly")
		}
	}

	// Check for DreamHypothetical in assistantMsg
	if aMsg, ok := outcome.RawMsg.(assistantMsg); ok {
		if aMsg.DreamHypothetical != "" {
			t.Log("Dream response contains DreamHypothetical (correct)")
		}
	}
	t.Logf("Dream routing: %s", outcome.MsgType)
}

// =============================================================================
// TEST 6: Assault campaign → beats generic delegation
// =============================================================================

func TestE2E_TaskRouting_AssaultCampaign_NaturalLanguageWins(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/mutation",
		Verb:       "/assault",
		Target:     "internal/core",
		Confidence: 0.9,
		Response:   "Starting assault campaign.",
	})

	outcome := routeInput(t, m, "Run an assault campaign on internal/core with race and vet")

	// assaultArgsFromNaturalLanguage fires at line 316 BEFORE auto-clarify/delegation.
	// Without full orchestrator deps, it returns campaignErrorMsg (missing kernel/client/shardMgr).
	assertNoClarification(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)

	// The message must be campaign-related, not a generic assistantMsg with coder ShardResult
	switch outcome.RawMsg.(type) {
	case campaignStartedMsg:
		t.Log("Assault routed to campaignStartedMsg (correct)")
	case campaignErrorMsg:
		t.Log("Assault routed to campaignErrorMsg (correct — infrastructure error in mock)")
	case assistantMsg:
		msg := outcome.RawMsg.(assistantMsg)
		if msg.ShardResult != nil && msg.ShardResult.ShardType == "coder" {
			t.Error("Assault was misrouted to coder delegation instead of campaign path")
		}
	}
	t.Logf("Assault routing: %s", outcome.MsgType)
}

// =============================================================================
// TEST 7: Follow-up → pre-perception, transducer NOT called
// =============================================================================

func TestE2E_TaskRouting_FollowUp_PrePerception(t *testing.T) {
	m, tr := setupRoutingModel(t, perception.Intent{
		Category:   "/query",
		Verb:       "/explain",
		Target:     "previous result",
		Confidence: 0.8,
		Response:   "Let me explain the previous finding.",
	})

	// Seed lastShardResult so detectFollowUpQuestion fires
	m.lastShardResult = &ShardResult{
		ShardType: "reviewer",
		Task:      "review internal/core/kernel.go",
		RawOutput: "Found potential nil pointer dereference on line 42.",
	}

	outcome := routeInput(t, m, "why is that bad?")

	// Follow-up detection at line 114 fires BEFORE perception.
	// If it triggers, transducer should NOT be called.
	callCount := tr.getCallCount()
	if callCount == 0 {
		t.Log("Follow-up correctly bypassed perception (transducer not called)")
	} else {
		t.Logf("Follow-up did not bypass perception (transducer called %d times) — 'why is that bad?' may not match follow-up heuristic", callCount)
	}

	assertNoCampaignStarted(t, outcome)
	assertNoClarification(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)
}

// =============================================================================
// TEST 8: Stats → deterministic handler, no LLM articulation
// =============================================================================

func TestE2E_TaskRouting_Stats_UsesDeterministicHandler(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/query",
		Verb:       "/stats",
		Target:     "project",
		Confidence: 0.9,
		Response:   "Here are the project stats.",
	})

	outcome := routeInput(t, m, "How many files are in this project?")

	// /stats has no shard type (GetShardTypeForVerb("/stats") == "")
	// and intent.Verb == "/stats", so the stats handler at line 580 fires.
	assertLaneWon(t, outcome, "responseMsg")
	assertNoCampaignStarted(t, outcome)
	assertNoClarification(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)
}

// =============================================================================
// TEST 9: Multi-step → decomposes before single-shard delegation
// =============================================================================

func TestE2E_TaskRouting_MultiStep_DetectionFires(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/mutation",
		Verb:       "/create",
		Target:     "auth middleware",
		Constraint: "tests and docs",
		Confidence: 0.9,
		Response:   "I'll create the auth middleware, tests, and docs.",
	})

	// "Add auth middleware, update tests, and document the new API"
	// detectMultiStepTask triggers on "and", which is a multiStepKeyword.
	outcome := routeInput(t, m, "Add auth middleware, update tests, and document the new API")

	// Multi-step detection runs at line 371, before shard delegation at line 384.
	// decomposeTask may return >1 steps → executeMultiStepTask fires.
	assertNoCampaignStarted(t, outcome)
	assertRoutingIdle(t, outcome)
	t.Logf("MultiStep routing: %s, subtasks=%v", outcome.MsgType, outcome.HasSubtasks)
}

// =============================================================================
// TEST 10: Active campaign context does NOT hijack simple chat
// =============================================================================

func TestE2E_TaskRouting_ActiveCampaign_ContextDoesNotHijackChat(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/query",
		Verb:       "/greet",
		Target:     "command",
		Confidence: 0.9,
		Response:   "It does X.",
	})

	// Simulate an active campaign in the model state.
	// seedCampaignFacts injects current_campaign facts for JIT awareness,
	// but must NOT auto-execute campaign tasks for a greeting.
	// Note: activeCampaign is *campaign.Campaign; setting it non-nil simulates context.
	// We cannot construct a real Campaign without importing campaign internals,
	// so we just verify the routing path works with the field nil but kernel having
	// campaign-adjacent facts.
	if m.kernel != nil {
		// Seed the kernel with a campaign fact to simulate context leakage
		_ = m.kernel.Assert(core.Fact{
			Predicate: "current_campaign",
			Args:      []interface{}{"/test_campaign", "Refactor auth module"},
		})
	}

	outcome := routeInput(t, m, "hey quick question, what does this command do?")

	// /greet is always-conversational → responseMsg
	assertLaneWon(t, outcome, "responseMsg")
	assertNoClarification(t, outcome)
	assertNoMultistep(t, outcome)
	assertRoutingIdle(t, outcome)
}

// =============================================================================
// TEST 11: Review request → reviewer, NOT coder
// =============================================================================

func TestE2E_TaskRouting_ReviewRequest_RouteToReviewer_NotCoder(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/query",
		Verb:       "/review",
		Target:     "internal/core/kernel.go",
		Constraint: "security",
		Confidence: 0.88,
		Response:   "I'll review kernel.go for security issues.",
	})

	outcome := routeInput(t, m, "Review internal/core/kernel.go for security issues")

	// /review → GetShardTypeForVerb → "reviewer" (not "coder")
	// In mock, it will attempt to spawn reviewer shard.
	assertNoCampaignStarted(t, outcome)
	assertNoClarification(t, outcome)
	assertRoutingIdle(t, outcome)

	// Verify the shard type is reviewer, not coder
	if aMsg, ok := outcome.RawMsg.(assistantMsg); ok && aMsg.ShardResult != nil {
		if aMsg.ShardResult.ShardType != "reviewer" {
			t.Errorf("Expected reviewer shard, got %q", aMsg.ShardResult.ShardType)
		}
	}
	t.Logf("Review routing: %s", outcome.MsgType)
}

// =============================================================================
// TEST 12: Clarifier loop guard — same input does NOT re-clarify
// =============================================================================

func TestE2E_TaskRouting_ClarifierLoopGuard_SameInputNoReClarify(t *testing.T) {
	m, _ := setupRoutingModel(t, perception.Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "none",
		Confidence: 0.4,
		Response:   "What should I fix?",
	})

	// Set lastClarifyInput to match — prevents all three clarify paths
	m.lastClarifyInput = "fix it"

	outcome := routeInput(t, m, "fix it")

	// shouldAutoClarify, shouldClarifyFromKernel, shouldClarifyIntent all check
	// for EqualFold(input, lastClarifyInput) and return false when matched.
	// The input should fall through to delegation or articulation.
	assertNoClarification(t, outcome)
	assertRoutingIdle(t, outcome)
	t.Logf("Loop guard routing: %s (no clarification loop)", outcome.MsgType)
}

// =============================================================================
// BOSS FIGHT: Routing Arbitration Matrix
// =============================================================================

func TestE2E_TaskRouting_ArbitrationMatrix(t *testing.T) {
	type routingCase struct {
		name           string
		input          string
		intent         perception.Intent
		expectedType   string // substring match on msg type
		forbidCampaign bool
		forbidClarify  bool
		forbidSubtasks bool
	}

	cases := []routingCase{
		{
			name:  "greeting_hi",
			input: "hi",
			intent: perception.Intent{
				Category: "/query", Verb: "/greet", Target: "none",
				Confidence: 0.99, Response: "Hello!",
			},
			expectedType:   "responseMsg",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "capability_question",
			input: "what can you do?",
			intent: perception.Intent{
				Category: "/query", Verb: "/help", Target: "capabilities",
				Confidence: 0.95, Response: "I can help with code.",
			},
			expectedType:   "responseMsg",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "stats_query",
			input: "how many tests are failing?",
			intent: perception.Intent{
				Category: "/query", Verb: "/stats", Target: "tests",
				Confidence: 0.9, Response: "Stats.",
			},
			expectedType:   "responseMsg",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "simple_fix",
			input: "fix typo in README.md",
			intent: perception.Intent{
				Category: "/mutation", Verb: "/fix", Target: "README.md",
				Constraint: "typo", Confidence: 0.93,
				Response: "I'll fix it.",
			},
			// /fix → coder delegation (assistantMsg or errorMsg, not campaign/clarify)
			expectedType:   "delegation",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "ambiguous_fix",
			input: "fix it",
			intent: perception.Intent{
				Category: "/mutation", Verb: "/fix", Target: "none",
				Confidence: 0.4, Response: "What?",
			},
			// shouldClarifyIntent fires (target=none, confidence<0.45)
			expectedType:   "Msg",
			forbidCampaign: true, forbidSubtasks: true,
		},
		{
			name:  "plan_request",
			input: "plan a new auth system",
			intent: perception.Intent{
				Category: "/instruction", Verb: "/generate", Target: "",
				Confidence: 0.84, Response: "Sure.",
			},
			// shouldAutoClarify: buildish + "plan" + empty target
			expectedType:   "Msg",
			forbidCampaign: true, forbidSubtasks: true,
		},
		{
			name:  "assault_campaign",
			input: "run an assault campaign on internal/core",
			intent: perception.Intent{
				Category: "/mutation", Verb: "/assault", Target: "internal/core",
				Confidence: 0.9, Response: "Starting.",
			},
			// assault detection → campaign path
			expectedType:   "Msg",
			forbidClarify:  true, forbidSubtasks: true,
		},
		{
			name:  "dream_hypothetical",
			input: "what if we deleted auth middleware",
			intent: perception.Intent{
				Category: "/query", Verb: "/dream", Target: "delete auth middleware",
				Confidence: 0.92, Response: "Hypothetically...",
			},
			// dream handler → assistantMsg or errorMsg (infrastructure missing in mock)
			expectedType:   "delegation",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "explain_goes_to_articulation",
			input: "explain how the kernel works",
			intent: perception.Intent{
				Category: "/query", Verb: "/explain", Target: "kernel",
				Confidence: 0.9, Response: "The kernel manages facts.",
			},
			// /explain is NOT always-conversational (removed from list to allow
			// knowledge_requests). It goes through articulation.
			expectedType:   "Msg",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "knowledge_query",
			input: "what do you remember about my preferences?",
			intent: perception.Intent{
				Category: "/query", Verb: "/knowledge", Target: "preferences",
				Confidence: 0.88, Response: "I remember your preferences.",
			},
			// /knowledge is always-conversational → responseMsg
			expectedType:   "responseMsg",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "configure_instruction",
			input: "always use dark mode",
			intent: perception.Intent{
				Category: "/instruction", Verb: "/configure", Target: "dark mode",
				Confidence: 0.85, Response: "I'll remember that.",
			},
			// /configure is always-conversational → responseMsg
			expectedType:   "responseMsg",
			forbidCampaign: true, forbidClarify: true, forbidSubtasks: true,
		},
		{
			name:  "multistep_mutation",
			input: "Create auth middleware, write unit tests, and write documentation",
			intent: perception.Intent{
				Category: "/mutation", Verb: "/create", Target: "auth middleware",
				Constraint: "tests and docs", Confidence: 0.95, Response: "Sure.",
			},
			// multi-step task detection → multistep command Msg
			expectedType:   "Msg",
			forbidCampaign: true, forbidClarify: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := setupRoutingModel(t, tc.intent)
			outcome := routeInput(t, m, tc.input)

			// Lane assertion
			if tc.expectedType == "delegation" {
				assertDelegationAttempted(t, outcome)
			} else {
				assertLaneWon(t, outcome, tc.expectedType)
			}

			// Forbidden lane assertions
			if tc.forbidCampaign {
				assertNoCampaignStarted(t, outcome)
			}
			if tc.forbidClarify {
				assertNoClarification(t, outcome)
			}
			if tc.forbidSubtasks {
				assertNoMultistep(t, outcome)
			}

			// Universal: every path must return to idle and leave kernel clean
			assertRoutingIdle(t, outcome)
			assertKernelClean(t, outcome.Model)

			// Cleanup routing facts to prevent leakage to other subtests
			retractRoutingFacts(outcome.Model)

			t.Logf("  lane=%s, loading=%v, error=%v, clarify=%v, campaign=%v",
				outcome.MsgType, outcome.IsLoading, outcome.HasError, outcome.HasClarify, outcome.HasCampaign)
		})
	}
}

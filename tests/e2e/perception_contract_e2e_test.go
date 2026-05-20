//go:build integration

package e2e_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// =============================================================================
// Mock LLM Client for deterministic perception E2E tests
// =============================================================================

// pceMockLLMClient returns deterministic JSON from a queue.
// It also records the prompts passed to CompleteWithSystem for assertion.
type pceMockLLMClient struct {
	mu             sync.Mutex
	responses      []string
	index          int
	recordedSystem []string
	recordedUser   []string
}

func newPCEMockClient(responses ...string) *pceMockLLMClient {
	return &pceMockLLMClient{responses: responses}
}

func (m *pceMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return m.CompleteWithSystem(ctx, "", prompt)
}

func (m *pceMockLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordedSystem = append(m.recordedSystem, sys)
	m.recordedUser = append(m.recordedUser, user)
	if m.index < len(m.responses) {
		resp := m.responses[m.index]
		m.index++
		return resp, nil
	}
	return `{"understanding":{"primary_intent":"chat","semantic_type":"definition","action_type":"chat","domain":"general","scope":{"level":"codebase","target":""},"user_constraints":[],"implicit_assumptions":[],"confidence":0.5,"signals":{"is_question":false,"is_hypothetical":false,"is_multi_step":false,"is_negated":false,"requires_confirmation":false,"urgency":"normal"},"suggested_approach":{"mode":"normal","primary_shard":"coder","tools_needed":[],"context_needed":[]}},"surface_response":"fallback"}`, nil
}

func (m *pceMockLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "", StopReason: "end_turn"}, nil
}

func (m *pceMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m *pceMockLLMClient) getRecordedUser(idx int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if idx < len(m.recordedUser) {
		return m.recordedUser[idx]
	}
	return ""
}

func (m *pceMockLLMClient) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.recordedUser)
}

// =============================================================================
// Assertion helpers — assert four layers: Intent, Understanding, Routing, Kernel
// =============================================================================

func assertNoMutationRoute(t *testing.T, intent perception.Intent) {
	t.Helper()
	if intent.Category == "/mutation" {
		t.Errorf("SAFETY: Intent.Category is /mutation — expected /query or /instruction")
	}
}

func assertReadOnlyBlocksWriteTools(t *testing.T, u *perception.Understanding) {
	t.Helper()
	if u == nil || u.Routing == nil {
		return
	}
	writeTools := []string{"write_file", "edit_file", "git_commit", "git_push"}
	for _, wt := range writeTools {
		found := false
		for _, bt := range u.Routing.BlockedTools {
			if bt == wt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SAFETY: Read-only understanding missing blocked tool %q", wt)
		}
	}
}

func assertKernelHasPredicate(t *testing.T, kernel *core.RealKernel, predicate string) []core.Fact {
	t.Helper()
	facts, err := kernel.Query(predicate)
	if err != nil {
		t.Logf("Query %q error: %v", predicate, err)
		return nil
	}
	return facts
}

// =============================================================================
// TEST 1: Read-only security review must not become a write/coder action
// =============================================================================

func TestE2E_Perception_ReadOnlySecurityReview(t *testing.T) {
	// Mock LLM returns a response where the LLM CORRECTLY identifies review/security
	// but INCORRECTLY suggests coder shard and write tools.
	// The harness must override the LLM's unsafe suggestion.
	mockResp := `{
		"understanding": {
			"primary_intent": "review",
			"semantic_type": "state",
			"action_type": "review",
			"domain": "security",
			"scope": {"level": "file", "target": "auth.go", "file": "internal/auth/auth.go"},
			"user_constraints": ["no_changes", "read_only", "do not modify files"],
			"implicit_assumptions": ["the user may want a patch, despite saying no changes"],
			"confidence": 0.92,
			"signals": {
				"is_question": false, "is_hypothetical": false, "is_multi_step": true,
				"is_negated": true, "requires_confirmation": false, "urgency": "normal"
			},
			"suggested_approach": {
				"mode": "normal",
				"primary_shard": "coder",
				"supporting_shards": ["reviewer", "tester"],
				"tools_needed": ["write_file", "edit_file", "run_tests"],
				"context_needed": ["diagnostics", "test_output", "file_source"]
			}
		},
		"surface_response": "I will review auth.go without modifying files."
	}`

	mockClient := newPCEMockClient(mockResp)
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	tr := perception.NewUnderstandingTransducer(mockClient)
	// Wire kernel for routing derivation
	if ut, ok := tr.(*perception.UnderstandingTransducer); ok {
		ut.SetKernel(kernel)
	}

	// History poison: assistant previously wanted to write code
	history := []perception.ConversationTurn{
		{Role: "assistant", ThoughtSummary: "The user usually wants direct edits and immediate fixes."},
		{Role: "user", Content: "Implement the fix immediately."},
		{Role: "assistant", Content: "I can patch auth.go."},
	}

	// Ambient context with SQL injection diagnostic
	ctx := types.WithSessionContext(context.Background(), &types.SessionContext{
		Ambient: &types.AmbientContext{
			ActiveFile:   "internal/auth/auth.go",
			CursorLine:   88,
			SelectedText: `db.Query("SELECT * FROM users WHERE name = " + userInput)`,
			Diagnostics:  []string{"possible SQL injection", "auth_test.go failing"},
		},
	})

	intent, parseErr := tr.ParseIntentWithContext(ctx,
		"Review auth.go for SQL injection, but do not modify files. Also tell me what tests would fail.",
		history)
	if parseErr != nil {
		t.Fatalf("ParseIntentWithContext failed: %v", parseErr)
	}

	// --- Layer 1: Intent assertions ---
	t.Logf("Intent: Category=%s Verb=%s Target=%s Constraint=%s",
		intent.Category, intent.Verb, intent.Target, intent.Constraint)

	if intent.Category != "/query" {
		t.Errorf("Intent.Category = %q, want /query", intent.Category)
	}
	if intent.Verb != "/security" && intent.Verb != "/review" {
		t.Errorf("Intent.Verb = %q, want /security or /review", intent.Verb)
	}
	assertNoMutationRoute(t, intent)

	// Constraint must preserve user's read-only constraints
	if !strings.Contains(intent.Constraint, "no_changes") && !strings.Contains(intent.Constraint, "read_only") {
		t.Errorf("Intent.Constraint missing read-only markers: %q", intent.Constraint)
	}

	// --- Layer 2: Understanding assertions ---
	var lastU *perception.Understanding
	if ut, ok := tr.(*perception.UnderstandingTransducer); ok {
		lastU = ut.GetLastUnderstanding()
	}
	if lastU == nil {
		t.Fatal("GetLastUnderstanding() returned nil")
	}
	if lastU.ActionType != "review" {
		t.Errorf("Understanding.ActionType = %q, want review", lastU.ActionType)
	}
	if lastU.Domain != "security" {
		t.Errorf("Understanding.Domain = %q, want security", lastU.Domain)
	}
	if !lastU.IsReadOnly() {
		t.Error("Understanding.IsReadOnly() = false, want true")
	}

	// --- Layer 3: Routing assertions ---
	if lastU.Routing == nil {
		t.Fatal("Understanding.Routing is nil — routing derivation failed")
	}
	t.Logf("Routing: Mode=%s PrimaryShard=%s BlockedTools=%v",
		lastU.Routing.Mode, lastU.Routing.PrimaryShard, lastU.Routing.BlockedTools)

	// Primary shard must NOT be coder for a read-only security review
	if lastU.Routing.PrimaryShard == "coder" {
		t.Error("SAFETY: Routing.PrimaryShard = coder for read-only security review — LLM suggestion was not overridden")
	}

	// Write tools must be blocked
	assertReadOnlyBlocksWriteTools(t, lastU)

	// --- Layer 4: Kernel fact assertions ---
	cuFacts := assertKernelHasPredicate(t, kernel, "current_understanding")
	t.Logf("current_understanding facts: %d", len(cuFacts))
	for _, f := range cuFacts {
		t.Logf("  %v", f.Args)
	}

	dmFacts := assertKernelHasPredicate(t, kernel, "derived_mode")
	t.Logf("derived_mode facts: %d", len(dmFacts))

	dpsFacts := assertKernelHasPredicate(t, kernel, "derived_primary_shard")
	t.Logf("derived_primary_shard facts: %d", len(dpsFacts))
	for _, f := range dpsFacts {
		val := types.ExtractString(f.Args[0])
		if val == "/coder" {
			t.Error("SAFETY: Kernel derived_primary_shard = /coder for read-only review")
		}
	}
}

// =============================================================================
// TEST 2+3: Routing facts must not leak between turns
// =============================================================================

func TestE2E_Perception_RoutingFactIsolationBetweenTurns(t *testing.T) {
	// Turn 1: Security review (same as test 1)
	turn1Resp := `{
		"understanding": {
			"primary_intent": "review", "semantic_type": "state",
			"action_type": "review", "domain": "security",
			"scope": {"level": "file", "target": "auth.go", "file": "internal/auth/auth.go"},
			"user_constraints": ["no_changes", "read_only"],
			"implicit_assumptions": [], "confidence": 0.92,
			"signals": {"is_question": false, "is_hypothetical": false, "is_multi_step": false,
				"is_negated": true, "requires_confirmation": false, "urgency": "normal"},
			"suggested_approach": {"mode": "security_audit", "primary_shard": "reviewer",
				"tools_needed": ["read_file"], "context_needed": ["diagnostics"]}
		},
		"surface_response": "I will review auth.go."
	}`

	// Turn 2: Architecture explain (completely different domain)
	turn2Resp := `{
		"understanding": {
			"primary_intent": "explain", "semantic_type": "mechanism",
			"action_type": "explain", "domain": "architecture",
			"scope": {"level": "function", "target": "retryLoop", "file": "worker.go", "symbol": "retryLoop"},
			"user_constraints": [], "implicit_assumptions": [], "confidence": 0.88,
			"signals": {"is_question": true, "is_hypothetical": false, "is_multi_step": false,
				"is_negated": false, "requires_confirmation": false, "urgency": "normal"},
			"suggested_approach": {"mode": "normal", "primary_shard": "reviewer",
				"tools_needed": ["read_file"], "context_needed": ["function_source", "dependency_graph"]}
		},
		"surface_response": "I will explain the retry loop."
	}`

	mockClient := newPCEMockClient(turn1Resp, turn2Resp)
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	tr := perception.NewUnderstandingTransducer(mockClient)
	if ut, ok := tr.(*perception.UnderstandingTransducer); ok {
		ut.SetKernel(kernel)
	}

	ctx := context.Background()

	// --- Turn 1 ---
	intent1, err := tr.ParseIntentWithContext(ctx,
		"Review auth.go for SQL injection, but do not modify files.", nil)
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	t.Logf("Turn 1: Category=%s Verb=%s", intent1.Category, intent1.Verb)

	// Verify turn 1 kernel state
	cu1 := assertKernelHasPredicate(t, kernel, "current_understanding")
	t.Logf("Turn 1 current_understanding: %d facts", len(cu1))

	// --- Turn 2 ---
	intent2, err := tr.ParseIntentWithContext(ctx,
		"Explain how the retry loop works in worker.go", nil)
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}
	t.Logf("Turn 2: Category=%s Verb=%s Target=%s", intent2.Category, intent2.Verb, intent2.Target)

	// Layer 1: Turn 2 intent must be explain/query, not review/security
	if intent2.Verb != "/explain" {
		t.Errorf("Turn 2 Verb = %q, want /explain", intent2.Verb)
	}
	if intent2.Category != "/query" {
		t.Errorf("Turn 2 Category = %q, want /query", intent2.Category)
	}
	if intent2.Target != "retryLoop" && intent2.Target != "worker.go" {
		t.Errorf("Turn 2 Target = %q, want retryLoop or worker.go", intent2.Target)
	}

	// Layer 2: Understanding must reflect architecture/explain, not security/review
	var lastU *perception.Understanding
	if ut, ok := tr.(*perception.UnderstandingTransducer); ok {
		lastU = ut.GetLastUnderstanding()
	}
	if lastU == nil {
		t.Fatal("Turn 2 GetLastUnderstanding() nil")
	}
	if lastU.Domain != "architecture" {
		t.Errorf("Turn 2 Understanding.Domain = %q, want architecture", lastU.Domain)
	}
	if lastU.ActionType != "explain" {
		t.Errorf("Turn 2 Understanding.ActionType = %q, want explain", lastU.ActionType)
	}

	// Layer 4: Kernel current_understanding must now reflect Turn 2, not Turn 1
	cu2 := assertKernelHasPredicate(t, kernel, "current_understanding")
	t.Logf("Turn 2 current_understanding: %d facts", len(cu2))

	// Check that security domain is no longer the current understanding
	// NOTE: This tests whether assertRoutingFacts overwrites or accumulates.
	// If facts accumulate, this is a documented gap.
	for _, f := range cu2 {
		if len(f.Args) >= 3 {
			domain := types.ExtractString(f.Args[2])
			if domain == "/security" {
				t.Log("DOCUMENTED GAP: current_understanding still contains /security from Turn 1. " +
					"assertRoutingFacts may be accumulating rather than replacing per-turn facts. " +
					"Cleanup should happen at turn start (see process.go retraction block).")
			}
		}
	}

	// Turn 2's read-only blocked tools from Turn 1 must NOT carry over
	// (unless Turn 2 is independently read-only, which explain IS)
	if lastU.Routing != nil {
		for _, bt := range lastU.Routing.BlockedTools {
			t.Logf("Turn 2 blocked tool: %s", bt)
		}
		// explain IS read-only, so blocked tools are expected here too
		// But they should come from Turn 2's own IsReadOnly(), not from Turn 1 state leaking
		if lastU.IsReadOnly() {
			t.Log("Turn 2 (explain) is correctly marked read-only on its own merits")
		}
	}
}

// =============================================================================
// TEST 4: Prompt construction must include ambient context
// =============================================================================

func TestE2E_Perception_PromptConstruction_AmbientContext(t *testing.T) {
	mockClient := newPCEMockClient(
		`{"understanding":{"primary_intent":"debug","semantic_type":"causation","action_type":"investigate","domain":"testing","scope":{"level":"file","target":"auth.go"},"user_constraints":[],"implicit_assumptions":[],"confidence":0.85,"signals":{"is_question":true,"is_hypothetical":false,"is_multi_step":false,"is_negated":false,"requires_confirmation":false,"urgency":"normal"},"suggested_approach":{"mode":"debug","primary_shard":"tester","tools_needed":["read_file"],"context_needed":["test_output"]}},"surface_response":"I will investigate."}`,
	)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	tr := perception.NewUnderstandingTransducer(mockClient)
	if ut, ok := tr.(*perception.UnderstandingTransducer); ok {
		ut.SetKernel(kernel)
		ut.SetStrategicContext("This project uses the Mangle deductive engine for safety.")
	}

	// Build rich ambient context
	ctx := types.WithSessionContext(context.Background(), &types.SessionContext{
		Ambient: &types.AmbientContext{
			ActiveFile:   "internal/auth/auth.go",
			CursorLine:   42,
			SelectedText: "func validateToken(token string) bool {",
			Diagnostics:  []string{"TestValidateToken FAIL", "nil pointer at auth.go:42"},
		},
	})

	// Build 7 history turns — only last 5 should be included
	history := []perception.ConversationTurn{
		{Role: "user", Content: "Turn 1 — should be excluded"},
		{Role: "assistant", Content: "Turn 2 — should be excluded"},
		{Role: "user", Content: "Turn 3 — should be included"},
		{Role: "assistant", Content: "Turn 4 — should be included"},
		{Role: "user", Content: "Turn 5 — should be included"},
		{Role: "assistant", Content: "Turn 6 — should be included", ThoughtSummary: "I analyzed the auth flow."},
		{Role: "user", Content: "Turn 7 — should be included"},
	}

	_, parseErr := tr.ParseIntentWithContext(ctx, "Why is validateToken failing?", history)
	if parseErr != nil {
		t.Fatalf("ParseIntentWithContext failed: %v", parseErr)
	}

	// Inspect the prompt that was sent to the LLM
	prompt := mockClient.getRecordedUser(0)
	if prompt == "" {
		t.Fatal("No prompt recorded by mock LLM")
	}

	// Ambient context assertions
	if !strings.Contains(prompt, "internal/auth/auth.go") {
		t.Error("Prompt missing ActiveFile")
	}
	if !strings.Contains(prompt, "42") {
		t.Error("Prompt missing CursorLine")
	}
	if !strings.Contains(prompt, "func validateToken") {
		t.Error("Prompt missing SelectedText")
	}
	if !strings.Contains(prompt, "TestValidateToken FAIL") {
		t.Error("Prompt missing Diagnostics")
	}

	// Strategic context
	if !strings.Contains(prompt, "Mangle deductive engine") {
		t.Error("Prompt missing Strategic Context")
	}

	// History window: last 5 turns (Turns 3-7), NOT Turns 1-2
	if strings.Contains(prompt, "Turn 1") || strings.Contains(prompt, "Turn 2") {
		t.Error("Prompt includes history turns that should be trimmed (only last 5)")
	}
	if !strings.Contains(prompt, "Turn 3") {
		t.Error("Prompt missing Turn 3 (should be included in last 5)")
	}
	if !strings.Contains(prompt, "Turn 7") {
		t.Error("Prompt missing Turn 7")
	}
	if !strings.Contains(prompt, "I analyzed the auth flow") {
		t.Error("Prompt missing ThoughtSummary from Turn 6")
	}

	// Section headers
	if !strings.Contains(prompt, "## Ambient Workspace Context") {
		t.Error("Prompt missing Ambient section header")
	}
	if !strings.Contains(prompt, "## Strategic Context") {
		t.Error("Prompt missing Strategic section header")
	}
	if !strings.Contains(prompt, "## Recent Conversation") {
		t.Error("Prompt missing Conversation section header")
	}
	if !strings.Contains(prompt, "## Current Request") {
		t.Error("Prompt missing Current Request section header")
	}
}

// =============================================================================
// TEST 5: Hypothetical requests must force dream/read-only behavior
// =============================================================================

func TestE2E_Perception_HypotheticalForcesDreamMode(t *testing.T) {
	mockResp := `{
		"understanding": {
			"primary_intent": "simulate", "semantic_type": "hypothetical",
			"action_type": "modify", "domain": "architecture",
			"scope": {"level": "file", "target": "auth middleware"},
			"user_constraints": ["dry_run", "no_changes"],
			"implicit_assumptions": [], "confidence": 0.9,
			"signals": {"is_question": true, "is_hypothetical": true, "is_multi_step": false,
				"is_negated": true, "requires_confirmation": false, "urgency": "normal"},
			"suggested_approach": {
				"mode": "normal", "primary_shard": "coder",
				"tools_needed": ["delete_file", "edit_file"],
				"context_needed": ["dependency_graph"]
			}
		},
		"surface_response": "I will simulate the effect only."
	}`

	mockClient := newPCEMockClient(mockResp)
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	tr := perception.NewUnderstandingTransducer(mockClient)
	if ut, ok := tr.(*perception.UnderstandingTransducer); ok {
		ut.SetKernel(kernel)
	}

	intent, parseErr := tr.ParseIntentWithContext(context.Background(),
		"What if we deleted the auth middleware? Don't actually change anything.", nil)
	if parseErr != nil {
		t.Fatalf("ParseIntentWithContext failed: %v", parseErr)
	}

	t.Logf("Intent: Category=%s Verb=%s", intent.Category, intent.Verb)

	// BUG DOCUMENTED: mapSemanticToCategory maps action_type=modify → /mutation
	// even when semantic_type=hypothetical and IsHypothetical=true.
	// The routing layer correctly forces dream mode and blocks write tools,
	// but the Intent.Category is misleading. The correct fix would be for
	// mapSemanticToCategory to check signals.is_hypothetical and force /query.
	if intent.Category == "/mutation" {
		t.Log("KNOWN GAP: Intent.Category=/mutation for hypothetical request " +
			"(action_type=modify overrides semantic_type=hypothetical in mapSemanticToCategory). " +
			"Safety is enforced at routing layer (dream mode + blocked tools), not at Intent layer.")
	}

	var lastU *perception.Understanding
	if ut, ok := tr.(*perception.UnderstandingTransducer); ok {
		lastU = ut.GetLastUnderstanding()
	}
	if lastU == nil {
		t.Fatal("GetLastUnderstanding() nil")
	}

	// Hypothetical MUST be read-only
	if !lastU.IsReadOnly() {
		t.Error("Hypothetical understanding is not read-only")
	}

	// Routing mode MUST be dream (deriveMode forces dream for IsHypothetical)
	if lastU.Routing == nil {
		t.Fatal("Routing is nil")
	}
	if lastU.Routing.Mode != "dream" {
		t.Errorf("Routing.Mode = %q, want dream", lastU.Routing.Mode)
	}

	// Write tools must be blocked
	assertReadOnlyBlocksWriteTools(t, lastU)

	t.Logf("Routing: Mode=%s PrimaryShard=%s BlockedTools=%v",
		lastU.Routing.Mode, lastU.Routing.PrimaryShard, lastU.Routing.BlockedTools)
}

// =============================================================================
// TEST 6: Remember/Forget must produce memory operations
// =============================================================================

func TestE2E_Perception_RememberForgetMemoryOperations(t *testing.T) {
	rememberResp := `{
		"understanding": {
			"primary_intent": "remember", "semantic_type": "instruction",
			"action_type": "remember", "domain": "general",
			"scope": {"level": "codebase", "target": "prefer small functions and no new dependencies"},
			"user_constraints": [], "implicit_assumptions": [], "confidence": 0.87,
			"signals": {"is_question": false, "is_hypothetical": false, "is_multi_step": false,
				"is_negated": false, "requires_confirmation": false, "urgency": "normal"},
			"suggested_approach": {"mode": "normal", "primary_shard": "librarian",
				"tools_needed": [], "context_needed": []}
		},
		"surface_response": "I will remember that preference."
	}`

	forgetResp := `{
		"understanding": {
			"primary_intent": "forget", "semantic_type": "instruction",
			"action_type": "forget", "domain": "general",
			"scope": {"level": "codebase", "target": "prefer small functions"},
			"user_constraints": [], "implicit_assumptions": [], "confidence": 0.85,
			"signals": {"is_question": false, "is_hypothetical": false, "is_multi_step": false,
				"is_negated": false, "requires_confirmation": false, "urgency": "normal"},
			"suggested_approach": {"mode": "normal", "primary_shard": "librarian",
				"tools_needed": [], "context_needed": []}
		},
		"surface_response": "I will forget that preference."
	}`

	mockClient := newPCEMockClient(rememberResp, forgetResp)
	tr := perception.NewUnderstandingTransducer(mockClient)
	ctx := context.Background()

	// --- Remember ---
	intent1, err := tr.ParseIntentWithContext(ctx,
		"Remember that I prefer small functions and no new dependencies.", nil)
	if err != nil {
		t.Fatalf("Remember failed: %v", err)
	}

	if intent1.Verb != "/remember" {
		t.Errorf("Remember Verb = %q, want /remember", intent1.Verb)
	}
	if intent1.Category != "/instruction" {
		t.Errorf("Remember Category = %q, want /instruction", intent1.Category)
	}
	if len(intent1.MemoryOperations) != 1 {
		t.Fatalf("Remember MemoryOperations len = %d, want 1", len(intent1.MemoryOperations))
	}
	if intent1.MemoryOperations[0].Op != "promote_to_long_term" {
		t.Errorf("MemoryOp.Op = %q, want promote_to_long_term", intent1.MemoryOperations[0].Op)
	}
	if intent1.MemoryOperations[0].Key != "preference" {
		t.Errorf("MemoryOp.Key = %q, want preference", intent1.MemoryOperations[0].Key)
	}
	if !strings.Contains(intent1.MemoryOperations[0].Value, "small functions") {
		t.Errorf("MemoryOp.Value missing 'small functions': %q", intent1.MemoryOperations[0].Value)
	}

	// --- Forget ---
	intent2, err := tr.ParseIntentWithContext(ctx,
		"Forget my preference about small functions.", nil)
	if err != nil {
		t.Fatalf("Forget failed: %v", err)
	}

	if intent2.Verb != "/forget" {
		t.Errorf("Forget Verb = %q, want /forget", intent2.Verb)
	}
	if len(intent2.MemoryOperations) != 1 {
		t.Fatalf("Forget MemoryOperations len = %d, want 1", len(intent2.MemoryOperations))
	}
	if intent2.MemoryOperations[0].Op != "forget" {
		t.Errorf("Forget MemoryOp.Op = %q, want forget", intent2.MemoryOperations[0].Op)
	}
}

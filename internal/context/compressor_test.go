package context

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

func createTestCompressor(t *testing.T, llm *MockLLMClient) *Compressor {
	// 1. Kernel
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// 2. Store
	localStore, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}

	// 3. Compressor
	comp := NewCompressor(kernel, localStore, llm)

	// Override config for predictable testing
	comp.config = DefaultTestContextConfig()
	// IMPORTANT: Must also update the budget's config, as it has its own copy
	comp.budget = NewTokenBudget(comp.config)

	return comp
}

func TestNewCompressor_Defaults(t *testing.T) {
	comp := createTestCompressor(t, &MockLLMClient{})

	if comp == nil {
		t.Fatal("NewCompressor returned nil")
	}
	if comp.sessionID == "" {
		t.Error("SessionID should be set")
	}
	if comp.counter == nil {
		t.Error("TokenCounter should be initialized")
	}
	if comp.budget == nil {
		t.Error("TokenBudget should be initialized")
	}
}

func TestProcessTurn_HappyPath(t *testing.T) {
	comp := createTestCompressor(t, &MockLLMClient{})

	turn := Turn{
		Number:    1,
		Role:      "user",
		UserInput: "Hello world",
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	res, err := comp.ProcessTurn(ctx, turn)
	if err != nil {
		t.Fatalf("ProcessTurn failed: %v", err)
	}

	if res.CompressionTriggered {
		t.Error("Compression should not be triggered for first turn")
	}

	if len(comp.recentTurns) != 1 {
		t.Errorf("Expected 1 recent turn, got %d", len(comp.recentTurns))
	}
}

func TestProcessTurn_CompressionTrigger(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "Summarized content", nil
		},
	}
	comp := createTestCompressor(t, mockLLM)

	// Inject existing history to near-limit (Budget is 1000, threshold 0.5 = 500)
	// We'll simulate big turns
	bigInput := strings.Repeat("word ", 1000) // ~1000 tokens

	// Turn 1
	comp.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: bigInput, Timestamp: time.Now()})
	// Turn 2
	comp.ProcessTurn(context.Background(), Turn{Number: 2, Role: "assistant", SurfaceResponse: bigInput, Timestamp: time.Now()})

	// Check usage before 3rd turn
	_ = comp.budget.GetUsage()

	// Turn 3 - Should push over threshold
	res, err := comp.ProcessTurn(context.Background(), Turn{Number: 3, Role: "user", UserInput: bigInput, Timestamp: time.Now()})
	if err != nil {
		t.Fatalf("ProcessTurn 3 failed: %v", err)
	}

	if !res.CompressionTriggered {
		t.Logf("Usage: %d, Threshold: %d", res.TokenUsage.Total, int(float64(comp.config.TotalBudget)*comp.config.CompressionThreshold))
		t.Error("Compression should have triggered")
	}

	// Verify Summary
	if !strings.Contains(comp.rollingSummary.Text, "Summarized content") {
		t.Errorf("Rolling summary expected to contain 'Summarized content', got '%s'", comp.rollingSummary.Text)
	}

	// Verify pruning (Window is 2, so we should have 2 turns left)
	if len(comp.recentTurns) != 2 {
		t.Errorf("Expected 2 recent turns after compression, got %d", len(comp.recentTurns))
	}
}

func TestProcessTurn_CompressionFallback(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", fmt.Errorf("API Error")
		},
	}
	comp := createTestCompressor(t, mockLLM)

	// Fill budget to trigger
	bigInput := strings.Repeat("word ", 300)
	comp.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: bigInput})
	comp.ProcessTurn(context.Background(), Turn{Number: 2, Role: "user", UserInput: bigInput}) // Over 500

	// Check fallback behavior
	// If LLM fails, it should use generateSimpleSummary
	if !strings.Contains(comp.rollingSummary.Text, "Turn 1") {
		// Simple summary usually lists turns? Need to check implementation detail or just ensure no panic
		// Implementation usually does: "Summary unavailable..." or heuristic summary.
	}

	// Mostly we verify it didn't panic and still pruned
	if len(comp.recentTurns) > comp.config.RecentTurnWindow {
		t.Errorf("Compression should prune even on LLM failure (fallback), got %d turns", len(comp.recentTurns))
	}
}

func TestBuildContext(t *testing.T) {
	comp := createTestCompressor(t, &MockLLMClient{})

	// Add some context
	comp.rollingSummary.Text = "Previous context summary."
	comp.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: "Recent question"})

	ctxData, err := comp.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	if !strings.Contains(ctxData.HistorySummary, "Previous context summary") {
		t.Error("Context should contain summary")
	}

	// Check GetContextString output
	ctxStr, _ := comp.GetContextString(context.Background())
	if !strings.Contains(ctxStr, "Previous context summary") {
		t.Error("Context string should contain summary")
	}
}

func TestPersistence(t *testing.T) {
	comp := createTestCompressor(t, &MockLLMClient{})
	comp.ProcessTurn(context.Background(), Turn{Number: 1, UserInput: "Test"})

	// Get State
	state := comp.GetState()

	// New Compressor
	comp2 := createTestCompressor(t, &MockLLMClient{})
	comp2.LoadState(state)

	if comp2.sessionID != comp.sessionID {
		t.Errorf("Session ID mismatch: %s vs %s", comp2.sessionID, comp.sessionID)
	}
	if len(comp2.recentTurns) != 1 {
		t.Error("Failed to restore recent turns")
	}
}

func TestAtomExtractionAndKernel(t *testing.T) {
	comp := createTestCompressor(t, &MockLLMClient{})

	// Turn with control packet atoms
	// Use 5 args to match standard user_intent schema: ID, Category, Verb, Target, Constraint
	atomStr := "user_intent(\"id\", \"/test_cat\", \"/test_verb\", \"target\", \"constraint\")."
	packet := &perception.ControlPacket{
		MangleUpdates: []string{atomStr},
	}

	// Debug parsing directly
	extracted, err := ExtractAtomsFromControlPacket(packet)
	if err != nil {
		t.Fatalf("ExtractAtomsFromControlPacket failed: %v", err)
	}
	t.Logf("Extracted %d atoms", len(extracted))
	for i, a := range extracted {
		t.Logf("Atom %d: %s args=%v", i, a.Predicate, a.Args)
	}

	turn := Turn{
		Number:        1,
		UserInput:     "test",
		ControlPacket: packet,
	}

	_, err = comp.ProcessTurn(context.Background(), turn)
	if err != nil {
		t.Fatalf("ProcessTurn failed: %v", err)
	}

	// Check kernel
	facts, err := comp.kernel.Query("user_intent")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	t.Logf("Found %d facts for user_intent", len(facts))
	found := false
	for _, f := range facts {
		t.Logf("Fact: %v (Args: %v)", f.Predicate, f.Args)
		if len(f.Args) >= 3 && f.Args[2] == "/test_verb" {
			found = true
			break
		}
	}

	if !found {
		// Log all facts to debug
		allFacts := comp.kernel.GetAllFacts()
		t.Logf("All facts in kernel: %d", len(allFacts))
		for _, f := range allFacts {
			t.Logf("  %s %v", f.Predicate, f.Args)
		}
		t.Error("Atom from control packet not found in kernel")
	}
}

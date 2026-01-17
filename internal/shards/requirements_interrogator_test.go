package shards

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/articulation"
)

// =============================================================================
// REQUIREMENTS INTERROGATOR TESTS
// =============================================================================

func TestNewRequirementsInterrogatorShard(t *testing.T) {
	t.Parallel()

	shard := NewRequirementsInterrogatorShard()
	if shard == nil {
		t.Fatal("expected non-nil shard")
	}
	if shard.ID != "requirements_interrogator" {
		t.Errorf("ID mismatch: got %q", shard.ID)
	}
}

func TestRequirementsInterrogatorShard_Execute_NoLLM(t *testing.T) {
	t.Parallel()

	shard := NewRequirementsInterrogatorShard()

	// Execute without LLM client should return fallback questions
	resp, err := shard.Execute(context.Background(), "build something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "What is the exact scope") {
		t.Error("expected fallback questions in response")
	}
}

func TestRequirementsInterrogatorShard_Execute_EmptyTask(t *testing.T) {
	t.Parallel()

	shard := NewRequirementsInterrogatorShard()

	resp, err := shard.Execute(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "No task provided") {
		t.Error("expected error message for empty task")
	}
}

func TestRequirementsInterrogatorShard_Setters(t *testing.T) {
	t.Parallel()

	shard := NewRequirementsInterrogatorShard()

	client := &mockLLMClient{}
	shard.SetLLMClient(client)

	// Verify setter worked (indirectly via execution behavior or reflection if needed,
	// but here just running without panic is a starting point)

	// PromptAssembler setter
	kn := &mockKernel{}
	pa, _ := articulation.NewPromptAssembler(kn)
	shard.SetPromptAssembler(pa)
}

func TestRequirementsInterrogatorShard_Execute_WithLLM_NoJIT(t *testing.T) {
	// Tests the path where LLM is set but JIT is not ready/configured
	// It should error because static prompt was deleted
	t.Parallel()

	shard := NewRequirementsInterrogatorShard()
	client := &mockLLMClient{}
	shard.SetLLMClient(client)

	// Setup dummy Assembler that isn't JIT ready
	kn := &mockKernel{}
	pa, _ := articulation.NewPromptAssembler(kn)
	shard.SetPromptAssembler(pa)

	_, err := shard.Execute(context.Background(), "task")
	if err == nil {
		t.Error("expected error when JIT is not ready")
	}
}

// Mock PromptAssembler interaction would require interface mocking or deeper integration,
// limiting scope to unit tests of the methods we can reach.

func TestExtractQuestions(t *testing.T) {
	t.Parallel()

	shard := NewRequirementsInterrogatorShard()

	raw := "- What is the goal?\n- Who is the user?\nIrrelevant text"
	questions := shard.extractQuestions(raw)

	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
	if questions[0] != "What is the goal?" {
		t.Errorf("unexpected question: %q", questions[0])
	}
}

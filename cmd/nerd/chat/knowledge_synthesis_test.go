package chat

import (
	"strings"
	"testing"
	"time"
)

// knowledge_synthesis_test.go pins the knowledge-gathering contract after the
// pipeline-re-entry removal: gathered specialist knowledge is synthesized into
// the final answer with exactly ONE LLM call, and the perception/routing
// pipeline is never re-entered.

func TestSynthesizeWithKnowledge_OneLLMCallNoReentry(t *testing.T) {
	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("The JIT system compiles prompt atoms at runtime.")

	m := NewTestModel()
	m.client = mockClient

	results := []KnowledgeResult{
		{
			Specialist: "mangleexpert",
			Query:      "what is the JIT system",
			Response:   "JIT assembles persona atoms per intent.",
			Timestamp:  time.Now(),
		},
		{
			Specialist: "researcher",
			Query:      "JIT compiler docs",
			Response:   "",  // empty response must be skipped, not break synthesis
			Error:      nil, //
		},
	}

	msg := m.synthesizeWithKnowledge("what is the jit system", results)()

	aMsg, ok := msg.(assistantMsg)
	if !ok {
		t.Fatalf("synthesis returned %T, want assistantMsg", msg)
	}
	if aMsg.Surface != "The JIT system compiles prompt atoms at runtime." {
		t.Errorf("Surface = %q, want the synthesized answer", aMsg.Surface)
	}
	if got := mockClient.GetCallCount(); got != 1 {
		t.Errorf("LLM call count = %d, want exactly 1 (synthesis must not re-enter the pipeline)", got)
	}

	// The synthesis prompt must carry the original question and the gathered
	// knowledge so the model can actually answer.
	prompt := mockClient.GetLastPrompt()
	if !strings.Contains(prompt, "what is the jit system") {
		t.Error("synthesis prompt missing the original question")
	}
	if !strings.Contains(prompt, "JIT assembles persona atoms per intent.") {
		t.Error("synthesis prompt missing the gathered specialist knowledge")
	}
}

func TestSynthesizeWithKnowledge_AllConsultationsFailed(t *testing.T) {
	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("Best-effort answer.")

	m := NewTestModel()
	m.client = mockClient

	results := []KnowledgeResult{
		{Specialist: "researcher", Query: "q", Response: "", Timestamp: time.Now()},
	}

	msg := m.synthesizeWithKnowledge("what is X", results)()
	if _, ok := msg.(assistantMsg); !ok {
		t.Fatalf("synthesis returned %T, want assistantMsg even when consultations were empty", msg)
	}
	prompt := mockClient.GetLastPrompt()
	if !strings.Contains(prompt, "consultations failed") {
		t.Error("synthesis prompt should tell the model the consultations were empty")
	}
}

func TestHandleKnowledgeRequests_CapsSpecialistCount(t *testing.T) {
	if maxKnowledgeRequestsPerTurn != 2 {
		t.Fatalf("maxKnowledgeRequestsPerTurn = %d, want 2 — if raised intentionally, update the latency math in the docs", maxKnowledgeRequestsPerTurn)
	}
}

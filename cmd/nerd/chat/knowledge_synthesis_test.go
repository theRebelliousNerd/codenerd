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

	msg := m.synthesizeWithKnowledge(0, "what is the jit system", knowledgeParent{}, results)()

	aMsg, ok := msg.(knowledgeSynthesizedMsg)
	if !ok || aMsg.Err != nil {
		t.Fatalf("synthesis returned %#v, want a successful knowledgeSynthesizedMsg", msg)
	}
	if aMsg.Response != "The JIT system compiles prompt atoms at runtime." {
		t.Errorf("Response = %q, want the synthesized answer", aMsg.Response)
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

	msg := m.synthesizeWithKnowledge(0, "what is X", knowledgeParent{}, results)()
	if result, ok := msg.(knowledgeSynthesizedMsg); !ok || result.Err != nil {
		t.Fatalf("synthesis returned %#v, want a successful terminal result even when consultations were empty", msg)
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

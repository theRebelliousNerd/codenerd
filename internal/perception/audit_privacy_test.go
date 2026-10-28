package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

type auditChannelStore chan *ReasoningTrace

func (s auditChannelStore) StoreReasoningTrace(record *ReasoningTrace) error {
	s <- record
	return nil
}

func TestNativeThinkingStaysOutOfInteractionAudit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"type":"message","content":[{"type":"thinking","thinking":"private-block-marker","signature":"native-signature"},{"type":"text","text":"Visible task result"}],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":3}}`)
	}))
	defer srv.Close()
	store := make(auditChannelStore, 1)
	client := NewTracingLLMClient(anthropicTestClient(srv.URL), store)
	response, err := client.CompleteWithTools(context.Background(), "system", "user", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Blocks) != 2 || response.Blocks[0].Signature != "native-signature" {
		t.Fatal("native continuity blocks must remain available for provider replay")
	}
	select {
	case record := <-store:
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if record.Response != "Visible task result" || strings.Contains(string(data), "private-block-marker") || strings.Contains(string(data), "native-signature") {
			t.Fatalf("interaction audit contains native thinking data: %s", data)
		}
	case <-time.After(time.Second):
		t.Fatal("interaction audit was not stored")
	}
}

func TestAnthropicRefusalDoesNotFailOver(t *testing.T) {
	for _, operation := range []string{"chat", "system", "tools"} {
		t.Run(operation, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"Request declined."}],"stop_reason":"refusal","usage":{"input_tokens":2,"output_tokens":3}}`)
			}))
			defer srv.Close()
			secondary := &fallbackFakeClient{resp: "secondary", toolResp: &types.LLMToolResponse{Text: "secondary"}}
			client := NewFallbackClient("refusal-test", anthropicTestClient(srv.URL), secondary)
			var err error
			switch operation {
			case "chat":
				_, err = client.Complete(context.Background(), "user")
			case "system":
				_, err = client.CompleteWithSystem(context.Background(), "system", "user")
			case "tools":
				_, err = client.CompleteWithTools(context.Background(), "system", "user", nil)
			}
			if types.StreamOutcomeOf(err) != types.StreamRefused {
				t.Fatalf("refusal identity lost: %v", err)
			}
			if requests != 1 || secondary.completeCalls+secondary.systemCalls+secondary.toolsCalls != 0 {
				t.Fatalf("refusal was retried: primary=%d secondary=%+v", requests, secondary)
			}
		})
	}
}

func TestLearningUsesVisibleResultsFromLegacyEnvelopes(t *testing.T) {
	eng := testTaxonomyEngine(t)
	critic := &recordingCritic{}
	eng.client = critic
	history := []ReasoningTrace{
		{UserPrompt: "Correct the classification", Response: `{"control_packet":{"reasoning_trace":"legacy-private-marker"},"surface_response":"Visible correction"}`},
		{Response: `{"control_packet":{"reasoning_trace":"partial-private-marker`},
		{Response: `{"chain_of_thought":["structured-private-marker"],"result":"Observed result"}`},
	}
	if _, err := eng.LearnFromInteraction(context.Background(), history); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(critic.lastPrompt, "Visible correction") || !strings.Contains(critic.lastPrompt, "Observed result") {
		t.Fatal("critic lost visible task results")
	}
	for _, forbidden := range []string{"private-marker", "chain_of_thought", "reasoning_trace"} {
		if strings.Contains(critic.lastPrompt, forbidden) {
			t.Errorf("critic received retired control content %q", forbidden)
		}
	}
	stored := newReasoningTrace(traceContext{}, "model", "system", "user", history[0].Response, 0, 0, nil)
	if stored.Response != "Visible correction" {
		t.Fatalf("audit did not retain only the visible answer: %q", stored.Response)
	}
}

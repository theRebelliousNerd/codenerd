package perception

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// newTestOpenAIClient builds an OpenAI client against a fake server.
func newTestOpenAIClient(t *testing.T, baseURL, model string) *OpenAIClient {
	t.Helper()
	c, err := NewOpenAIClient(OpenAIConfig{APIKey: "k", BaseURL: baseURL, Model: model, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}
	return c
}

// openAIResponsesBody is a completed Responses reply with one answer message.
func openAIResponsesBody(input, output int) map[string]any {
	return map[string]any{
		"id":     "resp_1",
		"status": "completed",
		"output": []map[string]any{{
			"id": "msg_1", "type": "message", "role": "assistant", "phase": "final_answer",
			"content": []map[string]any{{"type": "output_text", "text": "hi"}},
		}},
		"usage": map[string]any{"input_tokens": input, "output_tokens": output, "total_tokens": input + output},
	}
}

// sseResponsesStream writes a Responses event stream: deltas, then a
// completed response carrying usage with 4 cached and 2 reasoning tokens.
func sseResponsesStream(w http.ResponseWriter, deltas []string, input, output int) {
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"msg_1\",\"type\":\"message\",\"phase\":\"final_answer\"}}\n\n")
	for _, d := range deltas {
		fmt.Fprintf(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"delta\":%q}\n\n", d)
	}
	fmt.Fprintf(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[],"+
		"\"usage\":{\"input_tokens\":%d,\"output_tokens\":%d,\"total_tokens\":%d,\"input_tokens_details\":{\"cached_tokens\":4},\"output_tokens_details\":{\"reasoning_tokens\":2}}}}\n\n",
		input, output, input+output)
}

// responsesDeveloperText returns the developer (system) message of a
// Responses request body.
func responsesDeveloperText(t *testing.T, body map[string]any) (string, bool) {
	t.Helper()
	input, _ := body["input"].([]any)
	for _, raw := range input {
		m, _ := raw.(map[string]any)
		if m["role"] != "developer" {
			continue
		}
		content, _ := m["content"].([]any)
		for _, c := range content {
			block, _ := c.(map[string]any)
			if text, ok := block["text"].(string); ok {
				return text, true
			}
		}
	}
	return "", false
}

// openaiTestClient is the conformance suites' OpenAI adapter.
func openaiTestClient(baseURL string) *OpenAIClient {
	c, err := NewOpenAIClient(OpenAIConfig{APIKey: "test-key", BaseURL: baseURL, Model: "gpt-6.1-sol", Timeout: time.Minute})
	if err != nil {
		panic(err)
	}
	return c
}

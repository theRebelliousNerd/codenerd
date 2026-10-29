package perception

import "testing"

func TestExtractToolCalls(t *testing.T) {
	c := NewGeminiClientWithConfig(GeminiConfig{Model: "gemini-2.0"})

	// nil response and empty candidates yield no calls.
	if got := c.extractToolCalls(nil); got != nil {
		t.Errorf("nil response should yield nil, got %v", got)
	}
	if got := c.extractToolCalls(&GeminiResponse{}); got != nil {
		t.Errorf("no candidates should yield nil, got %v", got)
	}

	resp := &GeminiResponse{ThoughtSignature: "resp-sig"}
	cand := GeminiResponseCandidate{}
	cand.Content.Parts = []GeminiResponsePart{
		// Plain text part (no function call) is skipped.
		{Text: "thinking..."},
		// FunctionCall with its own signature wins.
		{FunctionCall: &GeminiFunctionCall{Name: "search", ThoughtSignature: "fc-sig"}},
		// FunctionCall with no fc-sig but a part-level signature falls back to it.
		{FunctionCall: &GeminiFunctionCall{Name: "read"}, ThoughtSignature: "part-sig"},
		// FunctionCall with no signature at all falls back to the response signature.
		{FunctionCall: &GeminiFunctionCall{Name: "write"}},
	}
	resp.Candidates = []GeminiResponseCandidate{cand}

	calls := c.extractToolCalls(resp)
	if len(calls) != 3 {
		t.Fatalf("expected 3 tool calls (text part skipped), got %d", len(calls))
	}

	// IDs are sequential by call index.
	if calls[0].id != "call_0" || calls[1].id != "call_1" || calls[2].id != "call_2" {
		t.Errorf("unexpected call IDs: %q %q %q", calls[0].id, calls[1].id, calls[2].id)
	}
	if calls[0].name != "search" || calls[0].signature != "fc-sig" {
		t.Errorf("call 0 = %+v, want search/fc-sig", calls[0])
	}
	if calls[1].name != "read" || calls[1].signature != "part-sig" {
		t.Errorf("call 1 = %+v, want read/part-sig (part-level fallback)", calls[1])
	}
	if calls[2].name != "write" || calls[2].signature != "resp-sig" {
		t.Errorf("call 2 = %+v, want write/resp-sig (response fallback)", calls[2])
	}
}

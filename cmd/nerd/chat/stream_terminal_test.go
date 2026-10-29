package chat

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// terminalScriptClient streams a fixed body and then ends the stream the way
// finish says: publish a terminal result, or (finish nil) close without one.
type terminalScriptClient struct {
	*MockLLMClient
	body   string
	finish *types.StreamResult
}

func (c *terminalScriptClient) CompleteWithStreaming(ctx context.Context, _, _ string, _ bool) (<-chan string, <-chan types.StreamResult) {
	w, content, done := types.NewStream()
	go func() {
		defer w.Close()
		if err := w.Content(ctx, c.body); err != nil {
			w.Fail(err)
			return
		}
		if c.finish != nil {
			w.Finish(*c.finish)
		}
	}()
	return content, done
}

const terminalTestEnvelope = `{"control_packet":{"intent_classification":{"category":"query","verb":"explain","target":"x","confidence":0.9},"mangle_updates":[],"memory_operations":[]},"surface_response":"all done"}`

func articulateScripted(t *testing.T, finish *types.StreamResult) (*ArticulationOutput, error) {
	t.Helper()
	client := &terminalScriptClient{MockLLMClient: NewMockLLMClient(), body: terminalTestEnvelope, finish: finish}
	intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "x"}
	streamChan := make(chan string, 64)
	return articulateWithConversation(context.Background(), client, intent,
		payloadForArticulation(intent, nil), nil, "sys", &ConversationContext{}, streamChan, nil)
}

// A parseable envelope is not proof the provider finished: a stream whose
// terminal channel closes without a result must fail the turn instead of
// handing its control packet to the response processor.
func TestArticulateWithConversation_StreamClosedWithoutTerminal_IsIncomplete(t *testing.T) {
	_, err := articulateScripted(t, nil)
	if err == nil {
		t.Fatal("expected an error for a stream closed without a terminal result")
	}
	if !errors.Is(err, types.ErrStreamIncomplete) {
		t.Fatalf("error = %v, want one matching types.ErrStreamIncomplete", err)
	}
}

func TestArticulateWithConversation_CompleteTerminal_Succeeds(t *testing.T) {
	out, err := articulateScripted(t, &types.StreamResult{StopReason: "end_turn"})
	if err != nil {
		t.Fatalf("complete stream failed: %v", err)
	}
	if out.Surface != "all done" {
		t.Fatalf("Surface = %q, want %q", out.Surface, "all done")
	}
}

// Truncation is not an intact answer even when the prefix closes valid JSON.
func TestArticulateWithConversation_TruncatedTerminal_FailsDespiteValidJSON(t *testing.T) {
	_, err := articulateScripted(t, &types.StreamResult{
		StopReason: "max_tokens",
		Err:        &types.OutputTruncated{Provider: "anthropic", Method: "CompleteWithStreaming", Reason: "max_tokens"},
	})
	if err == nil {
		t.Fatal("expected truncation to fail the turn")
	}
	if _, ok := types.AsOutputTruncated(err); !ok {
		t.Fatalf("error = %v, want a wrapped *types.OutputTruncated", err)
	}
}

// A failed turn's streamed prefix must not linger on screen as an answer.
func TestUpdate_ErrorMsg_ClearsStreamedPrefix(t *testing.T) {
	m := NewTestModel()
	m.isStreaming = true
	m.currentStream = "partial answer"
	m.currentThought = "partial thought"

	updated, _ := m.Update(errorMsg(errors.New("stream incomplete")))
	got := updated.(Model)
	if got.isStreaming || got.currentStream != "" || got.currentThought != "" {
		t.Fatalf("after errorMsg: isStreaming=%v currentStream=%q currentThought=%q, want all cleared",
			got.isStreaming, got.currentStream, got.currentThought)
	}
}

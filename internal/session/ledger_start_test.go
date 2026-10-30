package session

import (
	"context"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The request carried its messages from the first tool round on, so a demand
// appended after the user's input before any tool round ran sat where the
// anchor stands in and was never sent. Messages after the input now ride.
func TestWorkingRequest_KeepsADemandAddedBeforeAnyToolRound(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), nil, "fix the parser", &prompt.CompilationContext{ShardID: "probe"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	const demand = "LEDGER_DEMAND_MARKER answer the review"
	history := []types.Message{
		{Role: "user", Text: "fix the parser"},
		{Role: "user", Text: demand},
	}
	messages, err := e.prepareWorkingRequest(ctx, "sys", history, nil)
	if err != nil {
		t.Fatalf("prepareWorkingRequest: %v", err)
	}
	for _, m := range messages {
		if m.Text == demand {
			return
		}
	}
	t.Fatalf("the demand was dropped from the request: %+v", messages)
}

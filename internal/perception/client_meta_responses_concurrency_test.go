package perception

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

// strategicRaceStubClient answers classification with a fixed envelope after a
// strategicRaceStubClient answers classification with a fixed envelope after a
// short delay, widening the window in which a writer can race the reader.
type strategicRaceStubClient struct{}

func (s *strategicRaceStubClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (s *strategicRaceStubClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	time.Sleep(2 * time.Millisecond)
	return `{"primary_intent":"explain","semantic_type":"definition","action_type":"explain","domain":"general","confidence":0.9,"surface_response":"ok"}`, nil
}

func (s *strategicRaceStubClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	close(ch)
	ech := make(chan error)
	close(ech)
	return ch, ech
}

func (s *strategicRaceStubClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok"}, nil
}

// SetStrategicContext runs live from session boot while in-flight
// Transduce/Understand calls read the field; hammering both must be
// race-clean under -race.
func TestUnderstandingTransducer_SetStrategicContextConcurrentWithTransduce(t *testing.T) {
	tr := NewUnderstandingTransducer(&strategicRaceStubClient{}).(*UnderstandingTransducer)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			tr.SetStrategicContext(strings.Repeat(fmt.Sprintf("strategic-%d ", i), 50))
		}(i)
		go func() {
			defer wg.Done()
			_, _ = tr.ParseIntentWithContext(ctx, "what does the auth module do?", nil)
		}()
	}
	wg.Wait()
}

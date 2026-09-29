package perception

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
)

// TestLearnedPatternContext_FollowsEmbeddingRequestTimeout pins one pattern
// embed to embedding.request_timeout. The context stays detached: the
// consolidation drain has no process context to parent on.
func TestLearnedPatternContext_FollowsEmbeddingRequestTimeout(t *testing.T) {
	prev := config.EmbeddingRequestTimeout()
	t.Cleanup(func() { config.SetEmbeddingRequestTimeout(prev) })
	config.SetEmbeddingRequestTimeout(90 * time.Second)

	ctx, cancel := learnedPatternContext()
	defer cancel()
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("learnedPatternContext has no deadline")
	}
	rem := time.Until(dl)
	if rem <= 60*time.Second || rem > 90*time.Second {
		t.Fatalf("embed deadline remaining %s, want embedding.request_timeout (90s)", rem)
	}
}

type criticDeadlineClient struct {
	baseMockLLMClient
	got chan context.Context
}

func (c *criticDeadlineClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.got <- ctx
	return "", nil
}

// TestConsolidationWorker_CriticBoundIsPerCallNotJobClock pins that a
// consolidation batch is not wrapped in a two-minute clock. The one critic
// completion carries llm_timeouts.per_call_timeout, which is longer than the
// old job clock here, so a remaining deadline under two minutes would mean
// the batch clock is back.
func TestConsolidationWorker_CriticBoundIsPerCallNotJobClock(t *testing.T) {
	prev := config.GetLLMTimeouts()
	t.Cleanup(func() { config.SetLLMTimeouts(prev) })
	configured := prev
	configured.PerCallTimeout = 7 * time.Minute
	config.SetLLMTimeouts(configured)

	client := &criticDeadlineClient{got: make(chan context.Context, 1)}
	cw := NewConsolidationWorker(&TaxonomyEngine{client: client})
	cw.Start()
	t.Cleanup(cw.Stop)
	cw.Enqueue([]ReasoningTrace{{
		UserPrompt: "No, I meant the other file",
		Response:   "edited the wrong file",
		Success:    false,
	}})

	var ctx context.Context
	select {
	case ctx = <-client.got:
	case <-time.After(5 * time.Second):
		t.Fatal("consolidation did not call the critic")
	}
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("critic call has no deadline")
	}
	rem := time.Until(dl)
	if rem <= 2*time.Minute {
		t.Fatalf("critic deadline remaining %s, which is the old two-minute batch clock", rem)
	}
	if rem < 6*time.Minute || rem > 7*time.Minute {
		t.Fatalf("critic deadline remaining %s, want per_call_timeout (7m)", rem)
	}

	stopped := make(chan struct{})
	go func() {
		cw.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after the critic call")
	}
}

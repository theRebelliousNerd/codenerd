package system

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// gatedCritic is a learning critic whose one call returns when released.
type gatedCritic struct {
	types.LLMClient
	entered  chan struct{}
	release  chan struct{}
	finished atomic.Int32
}

func (c *gatedCritic) CompleteWithSystem(ctx context.Context, _, _ string) (string, error) {
	c.entered <- struct{}{}
	select {
	case <-c.release:
		c.finished.Add(1)
		return "", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ClosePerceptionLayer drains the learning worker, but it ran under the close
// step's timeout: a drain longer than the timeout was abandoned, and Close went
// on to close the databases the worker was still persisting through. The drain
// is now joined untimed before the timed step, so a job that outlives the
// step's timeout has finished when closePerceptionLayer returns.
func TestClosePerceptionLayer_DrainOutlivesTheStepTimeout(t *testing.T) {
	engine, err := perception.NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("NewTaxonomyEngine: %v", err)
	}
	critic := &gatedCritic{entered: make(chan struct{}, 1), release: make(chan struct{})}
	engine.SetClient(critic)
	// The critic compiles its system prompt from its role atoms before it
	// calls the model, as in a booted session.
	engine.SetRolePrompts(newBootRolePrompts(t))
	previous := perception.SharedTaxonomy
	perception.SharedTaxonomy = engine
	t.Cleanup(func() {
		perception.SharedTaxonomy = previous
		engine.StopWorker()
	})

	history := []perception.ReasoningTrace{
		{UserPrompt: "fix the build", Response: "ok"},
		{UserPrompt: "no, the other one", Response: "ok"},
	}
	if got := engine.QueueForLearning(history); got != perception.AdmissionAccepted {
		t.Fatalf("QueueForLearning = %q, want accepted", got)
	}
	<-critic.entered

	const stepTimeout = 20 * time.Millisecond
	release := time.AfterFunc(10*stepTimeout, func() { close(critic.release) })
	defer release.Stop()

	if err := closePerceptionLayer(stepTimeout); err != nil {
		t.Fatalf("closePerceptionLayer: %v", err)
	}
	if got := critic.finished.Load(); got != 1 {
		t.Fatalf("the critic call had finished %d time(s) when the close returned, want 1: the drain was abandoned at the step timeout", got)
	}
}

package core

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/types"
)

// modelMock is a client configured for one model: it reports the model and
// answers with it, as a request builder that reads its own field does.
type modelMock struct {
	mockLLMClient
	model string
}

func newModelMock(model string) *modelMock {
	m := &modelMock{model: model}
	m.completeFunc = func(context.Context, string) (string, error) { return model, nil }
	return m
}

func (m *modelMock) GetModel() string { return m.model }

// A shard profile that names only a model is served by that model on the
// slot's own provider. The wrapped client builds its requests from its own
// configured model, so before this route the profile's model was accepted
// and silently replaced by the slot's (audit ollama-shard-model-override).
func TestScheduledLLMCall_ModelOnlyOverrideRoutesOnTheSlotProvider(t *testing.T) {
	base := newModelMock("model-a")
	var built []string
	var mu sync.Mutex
	c := NewScheduledLLMCall("model-route-a", base)
	c.Router = func(provider, model string) (LLMClient, error) {
		mu.Lock()
		built = append(built, provider+"|"+model)
		mu.Unlock()
		return newModelMock(model), nil
	}

	onB := types.WithModelName(context.Background(), "model-b")
	if got, err := c.CompleteWithSystem(onB, "sys", "hi"); err != nil || got != "model-b" {
		t.Fatalf("a model-only override was served by %q (err %v), want model-b", got, err)
	}
	// The slot's own model, and no model at all, stay on the wrapped client.
	for _, ctx := range []context.Context{context.Background(), types.WithModelName(context.Background(), " model-a ")} {
		if got, err := c.CompleteWithSystem(ctx, "sys", "hi"); err != nil || got != "model-a" {
			t.Fatalf("an unrouted call was served by %q (err %v), want model-a", got, err)
		}
	}
	if len(built) != 1 || built[0] != "|model-b" {
		t.Fatalf("router calls = %v, want one client for model-b on the slot's provider (\"\")", built)
	}

	// Overlapping A and B calls each keep their own model, and the wrapped
	// client is never reconfigured.
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := range 32 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, want := context.Background(), "model-a"
			if i%2 == 1 {
				ctx, want = onB, "model-b"
			}
			if got, err := c.CompleteWithSystem(ctx, "sys", "hi"); err != nil || got != want {
				errs <- got + " for " + want
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("concurrent call served by %s", e)
	}
	if base.GetModel() != "model-a" {
		t.Fatalf("the wrapped client was reconfigured to %q", base.GetModel())
	}
}

// A model-only override this client cannot route fails the call by name; it
// is never sent to the slot's model.
func TestScheduledLLMCall_UnroutableModelOverrideIsRefused(t *testing.T) {
	base := newModelMock("model-a")
	c := NewScheduledLLMCall("model-route-b", base)
	_, err := c.Complete(types.WithModelName(context.Background(), "model-b"), "hi")
	if err == nil || !strings.Contains(err.Error(), `"model-b"`) || !strings.Contains(err.Error(), `"model-a"`) {
		t.Fatalf("err = %v, want a refusal naming both models", err)
	}
	if base.callCount != 0 {
		t.Fatalf("the slot's model served %d call(s) meant for another model", base.callCount)
	}
}

// ClientFor is the resolution preparation reads: it reports a route only when
// the call leaves the wrapped client.
func TestScheduledLLMCall_ClientForReportsTheRoute(t *testing.T) {
	base := newModelMock("model-a")
	c := NewScheduledLLMCall("model-route-c", base)
	c.Router = func(_, model string) (LLMClient, error) { return newModelMock(model), nil }

	if got, routed, err := c.ClientFor(context.Background()); err != nil || routed || got != LLMClient(base) {
		t.Fatalf("unrouted: client %v routed %v err %v", got, routed, err)
	}
	got, routed, err := c.ClientFor(types.WithModelName(context.Background(), "model-b"))
	if err != nil || !routed {
		t.Fatalf("routed: routed %v err %v", routed, err)
	}
	if m, ok := got.(*modelMock); !ok || m.model != "model-b" {
		t.Fatalf("routed client = %#v, want model-b's", got)
	}
}

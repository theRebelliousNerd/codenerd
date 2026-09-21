package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func namedMock(reply string) *mockLLMClient {
	return &mockLLMClient{completeFunc: func(context.Context, string) (string, error) { return reply, nil }}
}

// A call whose context names a provider runs on that provider's client; every
// other call runs on the wrapped one. This is what lets two shard profiles run
// on two vendors behind one slot.
func TestScheduledLLMCall_RoutesByTheProviderTheContextNames(t *testing.T) {
	base, routed := namedMock("from meta"), namedMock("from openrouter")
	var built []string
	c := NewScheduledLLMCall("route-test-a", base)
	c.Router = func(provider, model string) (LLMClient, error) {
		built = append(built, provider+"|"+model)
		return routed, nil
	}

	if got, err := c.Complete(context.Background(), "hi"); err != nil || got != "from meta" {
		t.Fatalf("an unrouted call: %q, %v", got, err)
	}
	ctx := types.WithProvider(types.WithModelName(context.Background(), "stealth/union-alpha"), "openrouter")
	for range 2 {
		if got, err := c.CompleteWithSystem(ctx, "sys", "hi"); err != nil || got != "from openrouter" {
			t.Fatalf("a routed call: %q, %v", got, err)
		}
	}
	if len(built) != 1 || built[0] != "openrouter|stealth/union-alpha" {
		t.Errorf("router calls = %v, want the client built once for that provider and model", built)
	}
	if base.callCount != 1 {
		t.Errorf("the wrapped client served %d calls, want only the unrouted one", base.callCount)
	}
}

// A route that cannot be built fails the call. It never falls back: a shard
// configured for one vendor and silently run on another is the defect.
func TestScheduledLLMCall_AFailedRouteNeverFallsBack(t *testing.T) {
	base := namedMock("from meta")
	ctx := types.WithProvider(types.WithModelName(context.Background(), "a/b"), "openrouter")

	noRouter := NewScheduledLLMCall("route-test-b", base)
	if _, err := noRouter.Complete(ctx, "hi"); err == nil || !strings.Contains(err.Error(), "no provider router") {
		t.Errorf("a routed call with no router: err = %v", err)
	}

	failing := NewScheduledLLMCall("route-test-c", base)
	failing.Router = func(string, string) (LLMClient, error) { return nil, errors.New("openrouter_api_key is empty") }
	_, err := failing.CompleteWithTools(ctx, "sys", "hi", nil)
	if err == nil || !strings.Contains(err.Error(), "openrouter_api_key is empty") || !strings.Contains(err.Error(), `"openrouter"`) {
		t.Errorf("a route that cannot be built: err = %v", err)
	}
	if base.callCount != 0 {
		t.Errorf("the wrapped client served %d call(s) that were routed elsewhere", base.callCount)
	}
}

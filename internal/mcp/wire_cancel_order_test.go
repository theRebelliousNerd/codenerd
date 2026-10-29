package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// cancellingCaller answers every page, and cancels the caller's context while
// the second page is in flight -- the reply still arrives, as an HTTP reply
// already on the wire does.
type cancellingCaller struct {
	cancel context.CancelFunc
	calls  int
}

func (c *cancellingCaller) call(ctx context.Context, method string, params any) (*mcpResponse, error) {
	c.calls++
	if c.calls == 2 {
		c.cancel()
	}
	return &mcpResponse{Result: json.RawMessage(toolPage("more", "a"))}, nil
}

// The outcome of a collection the caller cancelled is the cancellation, never
// a stop condition read off a page that arrived after it: deterministic,
// where the HTTP-level test depends on which side noticed the cancel first.
func TestListPaged_APageAfterCancellationIsNotCounted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	caller := &cancellingCaller{cancel: cancel}
	tools, err := listToolsVia(ctx, caller)
	var incomplete *IncompleteCatalogError
	if !errors.As(err, &incomplete) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want an incomplete catalog caused by the cancellation", err)
	}
	if len(tools) != 1 || incomplete.Pages != 1 {
		t.Fatalf("tools=%d pages=%d, want the one page gathered before the cancel", len(tools), incomplete.Pages)
	}
}

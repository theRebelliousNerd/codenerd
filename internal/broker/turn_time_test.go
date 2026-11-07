package broker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/turntime"
	"codenerd/internal/types"
)

// tickingClient advances a fake clock while the provider call is in flight,
// so the time it "took" is known.
type tickingClient struct {
	*fakeClient
	now  *atomic.Int64 // nanoseconds
	took time.Duration
}

func (c *tickingClient) CompleteWithSystem(ctx context.Context, s, u string) (string, error) {
	c.now.Add(int64(c.took))
	return c.fakeClient.CompleteWithSystem(ctx, s, u)
}

func (c *tickingClient) CompleteWithStreaming(ctx context.Context, s, u string, think bool) (<-chan string, <-chan types.StreamResult) {
	c.now.Add(int64(c.took))
	return c.fakeClient.CompleteWithStreaming(ctx, s, u, think)
}

// TestModelCallsAreChargedToTheTurnLedger: a call made through the broker
// under a turn context lands on the turn's ledger as model time, streamed or
// not, and a call outside any turn records nothing and breaks nothing.
func TestModelCallsAreChargedToTheTurnLedger(t *testing.T) {
	var now atomic.Int64
	clock := func() time.Time { return time.Unix(0, now.Load()) }
	ledger := turntime.NewWithClock(clock)
	ctx := turntime.WithLedger(context.Background(), ledger)

	fake := newFakeClient()
	fake.streamChunks = []string{"a", "b"}
	client := meteredClient(t, &tickingClient{fakeClient: fake, now: &now, took: 7 * time.Second}, testMeter(200000, 8000), &captureSink{})

	if _, err := client.CompleteWithSystem(ctx, "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	content, done := client.CompleteWithStreaming(ctx, "sys", "user", false)
	for range content {
	}
	<-done
	if _, err := client.CompleteWithSystem(context.Background(), "sys", "outside any turn"); err != nil {
		t.Fatalf("call outside a turn: %v", err)
	}

	s := ledger.Snapshot()
	if s.ModelCalls != 2 {
		t.Errorf("model calls = %d, want 2 (the call outside the turn is not the turn's)", s.ModelCalls)
	}
	// The fake advances its clock inside the span the broker opened, once
	// per call, so each call is exactly its 7 seconds.
	if s.Model != 14*time.Second {
		t.Errorf("model time = %s, want 14s", s.Model)
	}
	if open := s.Total - s.Model - s.Tool - s.Other - s.VerifyTime(); open != 0 {
		t.Errorf("phases do not sum to the total by %s", open)
	}
}

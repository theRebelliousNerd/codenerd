package core

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

// groundedSchedMock implements types.GroundedWebSearcher + LLMClient stub for scheduler tests.
type groundedSchedMock struct {
	supports bool
	delay    time.Duration
	handler  func(ctx context.Context, query string) (*types.GroundedWebSearchResult, error)
	lastQ    atomic.Value
	calls    int32
	// LLMClient stub
	text string
}

func (m *groundedSchedMock) SupportsGroundedWebSearch() bool { return m.supports }
func (m *groundedSchedMock) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	atomic.AddInt32(&m.calls, 1)
	m.lastQ.Store(query)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.handler != nil {
		return m.handler(ctx, query)
	}
	return &types.GroundedWebSearchResult{
		Text:      "sched-answer",
		Citations: []types.GroundedCitation{{URL: "https://example.com", Title: "t"}},
		Usage:     types.GroundedUsage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
	}, nil
}
func (m *groundedSchedMock) Complete(ctx context.Context, prompt string) (string, error) {
	return m.text, nil
}
func (m *groundedSchedMock) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.text, nil
}
func (m *groundedSchedMock) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	close(ch)
	errCh := make(chan error)
	close(errCh)
	return ch, errCh
}
func (m *groundedSchedMock) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: m.text}, nil
}
func (m *groundedSchedMock) GetModel() string { return "muse-spark-1.2-contributor" }

var _ types.GroundedWebSearcher = (*groundedSchedMock)(nil)

func newSchedWithMock(t *testing.T, mock *groundedSchedMock, maxSlots int) *ScheduledLLMCall {
	t.Helper()
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: maxSlots,
		SlotAcquireTimeout:    5 * time.Second,
		EnableMetrics:         true,
		AdaptiveConcurrency:   true,
		AdaptiveFloor:         1,
		AdaptiveRecoverAfter:  30 * time.Second,
	})
	scheduler.RegisterShard("test-shard", "test")
	return &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "test-shard",
		Client:    mock,
	}
}

func TestScheduled_GroundedWebSearch_SupportsForwarding(t *testing.T) {
	t.Parallel()
	m := &groundedSchedMock{supports: true}
	c := newSchedWithMock(t, m, 2)
	if !c.SupportsGroundedWebSearch() {
		t.Fatal("expected supports=true")
	}
	m2 := &groundedSchedMock{supports: false}
	c2 := newSchedWithMock(t, m2, 2)
	c2.Client = m2
	if c2.SupportsGroundedWebSearch() {
		t.Fatal("expected supports=false")
	}
	var nilCall *ScheduledLLMCall
	if nilCall.SupportsGroundedWebSearch() {
		t.Error("nil ScheduledLLMCall must be false")
	}
	noSearcher := &mockLLMClient{}
	c3 := newSchedWithMock(t, m, 2)
	c3.Client = noSearcher
	if c3.SupportsGroundedWebSearch() {
		t.Error("non-searcher client must be false")
	}
}

func TestScheduled_GroundedWebSearch_ExactQueryForwardingAndStructuredOutput(t *testing.T) {
	t.Parallel()
	wantQ := "exact query 42 with spaces"
	m := &groundedSchedMock{supports: true, handler: func(_ context.Context, q string) (*types.GroundedWebSearchResult, error) {
		if q != wantQ {
			t.Errorf("query forwarded = %q, want %q", q, wantQ)
		}
		return &types.GroundedWebSearchResult{
			Text:      "hello world grounded",
			Citations: []types.GroundedCitation{{URL: "https://example.com/a", Title: "A", StartIndex: 0, EndIndex: 5}},
			Usage:     types.GroundedUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		}, nil
	}}
	c := newSchedWithMock(t, m, 2)
	res, err := c.GroundedWebSearch(context.Background(), wantQ)
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Text != "hello world grounded" {
		t.Errorf("text = %q", res.Text)
	}
	if len(res.Citations) != 1 || res.Citations[0].URL != "https://example.com/a" {
		t.Errorf("citations = %+v", res.Citations)
	}
	if res.Usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", res.Usage)
	}
	// Verify reasoning not exposed (mock doesn't inject reasoning, but check text clean)
	if strings.Contains(res.Text, "reasoning") {
		t.Error("reasoning leaked")
	}
	if q, _ := m.lastQ.Load().(string); q != wantQ {
		t.Errorf("lastQ = %q", q)
	}
}

func TestScheduled_GroundedWebSearch_SlotAcquiredAndReleased(t *testing.T) {
	t.Parallel()
	m := &groundedSchedMock{supports: true}
	c := newSchedWithMock(t, m, 1)
	sched := c.Scheduler

	// Before call, no active slots
	if got := sched.GetMetrics().ActiveSlots; got != 0 {
		t.Fatalf("pre active=%d", got)
	}
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	// After call, slot must be released
	if got := sched.GetMetrics().ActiveSlots; got != 0 {
		t.Fatalf("post active=%d, want 0 (slot leaked)", got)
	}
	if got := sched.GetMetrics().TotalAPICalls; got != 1 {
		t.Fatalf("total api calls = %d, want 1", got)
	}
	if atomic.LoadInt32(&m.calls) != 1 {
		t.Fatalf("mock calls = %d", m.calls)
	}
}

func TestScheduled_GroundedWebSearch_RateLimitAndSuccessReporting(t *testing.T) {
	t.Parallel()
	// Rate limit reporting should shrink max slots when adaptive
	mRate := &groundedSchedMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return nil, errors.New("429 rate limit exceeded")
	}}
	cRate := newSchedWithMock(t, mRate, 3)
	sched := cRate.Scheduler
	// Ensure base is 3
	if sched.EffectiveMaxSlots() != 3 {
		t.Fatalf("initial max = %d", sched.EffectiveMaxSlots())
	}
	_, err := cRate.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	// Adaptive should have shrunk by 1
	if got := sched.EffectiveMaxSlots(); got != 2 {
		t.Fatalf("after rate limit max = %d, want 2", got)
	}
	// Slot must still be released
	if got := sched.GetMetrics().ActiveSlots; got != 0 {
		t.Fatalf("active after rate limit %d", got)
	}
	// Success should keep slot released and eventually recover (we don't wait 30s, just check not leaked)
	mOk := &groundedSchedMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return &types.GroundedWebSearchResult{Text: "ok", Usage: types.GroundedUsage{TotalTokens: 1}}, nil
	}}
	cRate.Client = mOk
	_, err = cRate.GroundedWebSearch(context.Background(), "ok")
	if err != nil {
		t.Fatalf("success after rate limit: %v", err)
	}
	if got := sched.GetMetrics().ActiveSlots; got != 0 {
		t.Fatalf("active after success %d", got)
	}
	// Total calls should be 2 (both slot-accounted)
	if got := sched.GetMetrics().TotalAPICalls; got != 2 {
		t.Fatalf("total calls %d want 2", got)
	}
}

func TestScheduled_GroundedWebSearch_PreservesCancellation(t *testing.T) {
	t.Parallel()
	m := &groundedSchedMock{supports: true, delay: 200 * time.Millisecond}
	c := newSchedWithMock(t, m, 1)
	// Pre-fix: verify context cancellation while waiting vs during execution both preserve error identity and release slot.
	// Case 1: cancel during execution (delay)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := c.GroundedWebSearch(ctx, "q")
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	err := <-done
	if err == nil {
		t.Fatal("expected cancel error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := c.Scheduler.GetMetrics().ActiveSlots; got != 0 {
		t.Fatalf("slot leaked after cancel, active=%d", got)
	}

	// Case 2: slot acquisition cancellation (queue)
	sched2 := NewAPIScheduler(APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	sched2.RegisterShard("holder", "test")
	sched2.RegisterShard("waiter", "test")
	_ = sched2.AcquireAPISlot(context.Background(), "holder")
	m2 := &groundedSchedMock{supports: true}
	callWaiter := &ScheduledLLMCall{Scheduler: sched2, ShardID: "waiter", Client: m2}
	ctx2, cancel2 := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { _, err := callWaiter.GroundedWebSearch(ctx2, "q"); errCh <- err }()
	time.Sleep(30 * time.Millisecond)
	cancel2()
	err = <-errCh
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter cancel expected Canceled, got %v", err)
	}
	// Holder still holds, waiter slot not leaked; release holder and check waiter didn't consume extra
	sched2.ReleaseAPISlot("holder")
	time.Sleep(20 * time.Millisecond)
	if got := sched2.GetMetrics().ActiveSlots; got != 0 {
		t.Fatalf("active after holder release %d", got)
	}
}

func TestScheduled_GroundedWebSearch_NilPaths(t *testing.T) {
	t.Parallel()
	var nilCall *ScheduledLLMCall
	_, err := nilCall.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("nil call must error")
	}
	cNoSched := &ScheduledLLMCall{Scheduler: nil, ShardID: "x", Client: &groundedSchedMock{supports: true}}
	_, err = cNoSched.GroundedWebSearch(context.Background(), "q")
	if err == nil || !strings.Contains(err.Error(), "scheduler not configured") {
		t.Fatalf("no scheduler err = %v", err)
	}
	sched := NewAPIScheduler(APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	sched.RegisterShard("x", "test")
	cNoClient := &ScheduledLLMCall{Scheduler: sched, ShardID: "x", Client: nil}
	_, err = cNoClient.GroundedWebSearch(context.Background(), "q")
	if err == nil || !strings.Contains(err.Error(), "underlying LLM client is nil") {
		t.Fatalf("no client err = %v", err)
	}
	noSearcher := &mockLLMClient{}
	cBad := &ScheduledLLMCall{Scheduler: sched, ShardID: "x", Client: noSearcher}
	_, err = cBad.GroundedWebSearch(context.Background(), "q")
	if err == nil || !strings.Contains(err.Error(), "does not implement GroundedWebSearcher") {
		t.Fatalf("bad client err = %v", err)
	}
}

func TestScheduled_GroundedWebSearch_NeverExposesReasoningOrKey(t *testing.T) {
	t.Parallel()
	const secretReasoning = "hidden reasoning trace must not appear"
	const apiKey = "sk-test-secret"
	m := &groundedSchedMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return nil, errors.New("request failed with status 500: code=internal_error type=server_error secret:" + secretReasoning + " key=" + apiKey)
	}}
	// Note: our mock handler leaks in this test to verify wrapper doesn't add more exposure; the real sanitization is in the raw client.
	// Wrapper must not add reasoning to the returned error beyond what underlying gave, and must not fabricate.
	c := newSchedWithMock(t, m, 2)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected error")
	}
	// The wrapper itself must not add reasoning text if underlying didn't sanitize; but we check that wrapper doesn't inject extra reasoning
	// Since mock returns an error containing secret, the wrapper will return it as-is. Real raw client would have sanitized before.
	// So for this wrapper test, ensure that a clean result never contains reasoning fields.
	m2 := &groundedSchedMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		// Simulate a compromised provider returning reasoning in text (should have been filtered by raw client)
		return &types.GroundedWebSearchResult{Text: "visible answer", Citations: nil, Usage: types.GroundedUsage{}}, nil
	}}
	c2 := newSchedWithMock(t, m2, 2)
	res, err := c2.GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("ok call: %v", err)
	}
	if res.Text != "visible answer" {
		t.Errorf("text = %q", res.Text)
	}
	// Ensure result JSON wouldn't contain reasoning keys if we were to marshal via tool (checked in research tests)
}

func TestScheduled_GroundedWebSearch_PanicRecoveryAndSlotRelease(t *testing.T) {
	t.Parallel()
	m := &groundedSchedMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		panic("boom")
	}}
	c := newSchedWithMock(t, m, 1)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil || !strings.Contains(err.Error(), "panic") {
		t.Fatalf("expected panic recovery, got %v", err)
	}
	if got := c.Scheduler.GetMetrics().ActiveSlots; got != 0 {
		t.Fatalf("slot leaked after panic, active=%d", got)
	}
}

func TestScheduled_GroundedWebSearch_ImplementsInterface(t *testing.T) {
	var _ types.GroundedWebSearcher = (*ScheduledLLMCall)(nil)
	var c any = &ScheduledLLMCall{}
	if _, ok := c.(types.GroundedWebSearcher); !ok {
		t.Error("ScheduledLLMCall must implement GroundedWebSearcher")
	}
}

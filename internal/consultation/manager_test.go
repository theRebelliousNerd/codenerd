package consultation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// gateSpawner blocks every call until release is closed, counting calls and
// recording whether its context was cancelled.
type gateSpawner struct {
	calls     atomic.Int32
	release   chan struct{}
	cancelled chan struct{}
	once      sync.Once
	reply     func(spec, task string) (string, error)
}

func newGateSpawner() *gateSpawner {
	return &gateSpawner{release: make(chan struct{}), cancelled: make(chan struct{})}
}

func (g *gateSpawner) SpawnConsultation(ctx context.Context, spec, task string) (string, error) {
	g.calls.Add(1)
	select {
	case <-g.release:
	case <-ctx.Done():
		g.once.Do(func() { close(g.cancelled) })
		return "", ctx.Err()
	}
	if g.reply != nil {
		return g.reply(spec, task)
	}
	return "ADVICE:\nuse interfaces\nCONFIDENCE: 80\nREFERENCES:\nref-a\nCAVEATS:\nwatch cycles", nil
}

type funcSpawner func(ctx context.Context, spec, task string) (string, error)

func (f funcSpawner) SpawnConsultation(ctx context.Context, spec, task string) (string, error) {
	return f(ctx, spec, task)
}

func waitCalls(t *testing.T, g *gateSpawner, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for g.calls.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("spawner calls = %d, want %d", g.calls.Load(), want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestConsult_DifferentRequesterIsSeparateComputation(t *testing.T) {
	var calls atomic.Int32
	var prompts sync.Map
	m := New(funcSpawner(func(_ context.Context, spec, task string) (string, error) {
		calls.Add(1)
		prompts.Store(task, true)
		return "ADVICE: fine", nil
	}))
	a, err := m.Consult(context.Background(), Request{Requester: "coder", Specialist: "architect", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Consult(context.Background(), Request{Requester: "tester", Specialist: "architect", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (requester is rendered into the prompt)", calls.Load())
	}
	if a.Requester != "coder" || b.Requester != "tester" || a.ContentKey == b.ContentKey {
		t.Fatalf("receipts share identity: %+v / %+v", a, b)
	}
	if a.RequestID == b.RequestID {
		t.Fatalf("request IDs collide: %s", a.RequestID)
	}
}

func TestConsult_SequentialIdenticalRequestsAreNotCached(t *testing.T) {
	var calls atomic.Int32
	m := New(funcSpawner(func(context.Context, string, string) (string, error) {
		calls.Add(1)
		return "ADVICE: fine", nil
	}))
	for range 2 {
		if _, err := m.Consult(context.Background(), Request{Specialist: "coder", Question: "q"}); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2: completed advice must not be reused without an evidence revision", calls.Load())
	}
}

func TestConsult_ConcurrentIdenticalMissesSpawnOnceAndIsolateMutation(t *testing.T) {
	g := newGateSpawner()
	m := New(g)
	req := Request{Specialist: "coder", Question: "q", Context: "c"}
	var wg sync.WaitGroup
	results := make([]*Response, 2)
	for i := range results {
		wg.Go(func() {
			r, err := m.Consult(context.Background(), req)
			if err != nil {
				t.Error(err)
			}
			results[i] = r
		})
	}
	waitCalls(t, g, 1)
	// Let the second consumer join before releasing.
	deadline := time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		joined := len(m.inflight) == 1 && m.inflight[contentKey(Request{Requester: DefaultRequester, Specialist: "coder", Question: "q", Context: "c"})].waiters == 2
		m.mu.Unlock()
		if joined || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(g.release)
	wg.Wait()
	if g.calls.Load() != 1 {
		t.Fatalf("spawner calls = %d, want 1", g.calls.Load())
	}
	kinds := map[ReuseKind]int{}
	for _, r := range results {
		kinds[r.Reuse]++
	}
	if kinds[ReuseComputed] != 1 || kinds[ReuseCoalesced] != 1 {
		t.Fatalf("reuse kinds = %v, want one computed and one coalesced", kinds)
	}
	if results[0].RequestID == results[1].RequestID {
		t.Fatal("coalesced consumers share a request ID")
	}
	results[0].References[0] = "mutated"
	results[0].Caveats = append(results[0].Caveats[:0], "mutated")
	if results[1].References[0] != "ref-a" || results[1].Caveats[0] != "watch cycles" {
		t.Fatalf("mutation leaked across consumers: %+v", results[1])
	}
}

func TestConsult_CancelledFollowerDoesNotCancelSibling(t *testing.T) {
	g := newGateSpawner()
	m := New(g)
	req := Request{Specialist: "coder", Question: "q"}

	shortCtx, cancelShort := context.WithCancel(context.Background())
	shortErr := make(chan error, 1)
	go func() {
		_, err := m.Consult(shortCtx, req)
		shortErr <- err
	}()
	waitCalls(t, g, 1)
	longRes := make(chan *Response, 1)
	go func() {
		r, err := m.Consult(context.Background(), req)
		if err != nil {
			t.Error(err)
		}
		longRes <- r
	}()
	time.Sleep(20 * time.Millisecond)
	cancelShort()
	if err := <-shortErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("short consumer err = %v, want canceled", err)
	}
	select {
	case <-g.cancelled:
		t.Fatal("producer cancelled while a consumer still waits")
	case <-time.After(20 * time.Millisecond):
	}
	close(g.release)
	if r := <-longRes; r == nil || r.Advice != "use interfaces" {
		t.Fatalf("surviving consumer got %+v", r)
	}
}

func TestConsult_AllConsumersGoneCancelsProducer(t *testing.T) {
	g := newGateSpawner()
	m := New(g)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := m.Consult(ctx, Request{Specialist: "coder", Question: "q"})
		done <- err
	}()
	waitCalls(t, g, 1)
	cancel()
	<-done
	select {
	case <-g.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("producer not cancelled after its last consumer left")
	}
	m.mu.Lock()
	n := len(m.inflight)
	m.mu.Unlock()
	if n != 0 {
		t.Fatalf("inflight entries = %d after abandonment, want 0", n)
	}
}

func TestConsult_NilSpawnerFailsVisibly(t *testing.T) {
	m := New(nil)
	if _, err := m.Consult(context.Background(), Request{Specialist: "coder"}); !errors.Is(err, ErrNoSpawner) {
		t.Fatalf("err = %v, want ErrNoSpawner", err)
	}
	b := m.RequestBatchConsultation(context.Background(), BatchRequest{Question: "q", Specialists: []string{"coder", "tester"}})
	if len(b.Entries) != 2 || b.Complete() {
		t.Fatalf("batch = %+v, want two failed entries", b)
	}
	for _, e := range b.Entries {
		if e.Status != EntryFailed || e.Retryable {
			t.Fatalf("entry %+v: configuration failure must be failed and non-retryable", e)
		}
	}
}

func TestBatch_DedupesAndKeepsPartialSuccess(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	m := New(funcSpawner(func(_ context.Context, spec, task string) (string, error) {
		mu.Lock()
		seen[spec]++
		mu.Unlock()
		if spec == "tester" {
			return "", errors.New("transport down")
		}
		if !strings.Contains(task, "[Topic] question") {
			t.Errorf("topic not merged into question: %q", task)
		}
		return "ADVICE: ok", nil
	}))
	b := m.RequestBatchConsultation(context.Background(), BatchRequest{
		Topic: "Topic", Question: "question",
		Specialists: []string{"coder", " ", "tester", "coder", "reviewer"},
	})
	if len(b.Entries) != 3 {
		t.Fatalf("entries = %d, want 3 distinct specialists", len(b.Entries))
	}
	if seen["coder"] != 1 {
		t.Fatalf("coder consulted %d times, want 1", seen["coder"])
	}
	order := []string{b.Entries[0].Specialist, b.Entries[1].Specialist, b.Entries[2].Specialist}
	if strings.Join(order, ",") != "coder,tester,reviewer" {
		t.Fatalf("order = %v", order)
	}
	if got := len(b.Succeeded()); got != 2 {
		t.Fatalf("succeeded = %d, want 2", got)
	}
	failed := b.Failed()
	if len(failed) != 1 || failed[0].Specialist != "tester" || !failed[0].Retryable || b.Complete() || b.Err() == nil {
		t.Fatalf("failed = %+v complete=%v", failed, b.Complete())
	}
}

func TestBatch_PreservesErrorIdentityAndRetryability(t *testing.T) {
	t.Parallel()
	transportErr := errors.New("transport unavailable")
	tests := []struct {
		name      string
		cause     error
		status    EntryStatus
		retryable bool
	}{
		{name: "cancelled", cause: context.Canceled, status: EntryCancelled, retryable: true},
		{name: "deadline exceeded", cause: context.DeadlineExceeded, status: EntryCancelled, retryable: true},
		{name: "transport failure", cause: transportErr, status: EntryFailed, retryable: true},
		{name: "configuration failure", cause: ErrNoSpawner, status: EntryFailed},
		{name: "configuration takes precedence over cancellation", cause: errors.Join(ErrNoSpawner, context.Canceled), status: EntryFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := New(funcSpawner(func(context.Context, string, string) (string, error) {
				return "ADVICE: must not be published", fmt.Errorf("specialist invocation: %w", tt.cause)
			}))
			batch := m.RequestBatchConsultation(t.Context(), BatchRequest{Specialists: []string{"coder"}})
			if batch.RequestErr != nil || len(batch.Entries) != 1 {
				t.Fatalf("batch = %+v, want one specialist outcome and no request error", batch)
			}
			entry := batch.Entries[0]
			if entry.Specialist != "coder" || entry.Status != tt.status || entry.Retryable != tt.retryable || entry.Response != nil {
				t.Errorf("entry = %+v, want specialist coder, status %s, retryable %v, and no response", entry, tt.status, tt.retryable)
			}
			if !errors.Is(entry.Err, tt.cause) {
				t.Errorf("entry error = %v, want cause %v", entry.Err, tt.cause)
			}
			if !errors.Is(batch.Err(), tt.cause) {
				t.Errorf("batch error = %v, want cause %v", batch.Err(), tt.cause)
			}
			if batch.Complete() || len(batch.Succeeded()) != 0 || len(batch.Failed()) != 1 {
				t.Errorf("batch = %+v, want incomplete with one failure and no successes", batch)
			}
		})
	}
}

func TestBatch_NoSpecialistsIsRequestError(t *testing.T) {
	b := New(funcSpawner(func(context.Context, string, string) (string, error) { return "", nil })).
		RequestBatchConsultation(context.Background(), BatchRequest{Question: "q"})
	if b.RequestErr == nil || b.Complete() {
		t.Fatalf("batch = %+v, want request error", b)
	}
}

func TestParseArtifact_ConfidenceIsOneLine(t *testing.T) {
	a := parseArtifact("k", "coder", "VOTE: APPROVE\nCONFIDENCE: 85\n\nREASONING:\nsound plan", 0)
	if a.Confidence != 0.85 {
		t.Fatalf("confidence = %v, want 0.85", a.Confidence)
	}
	if a.ParseStatus != ParseUnstructured || !strings.Contains(a.Advice, "REASONING") || a.Raw == "" {
		t.Fatalf("unstructured reply must keep its whole text: %+v", a)
	}
}

func TestBuildPrompt_ContractReplacesDefaultFormat(t *testing.T) {
	p := buildPrompt(Request{Requester: "system", Question: "q", ResponseContract: "VOTE: [APPROVE]"})
	if strings.Contains(p, "ADVICE:") || !strings.Contains(p, "VOTE: [APPROVE]") {
		t.Fatalf("prompt carries conflicting formats:\n%s", p)
	}
	if contentKey(Request{ResponseContract: "a"}) == contentKey(Request{ResponseContract: "b"}) {
		t.Fatal("response contract not in content key")
	}
}

// CONFIDENCE arrives as 0-100 per the prompt, but models also write ratios
// and out-of-range values: the first must not parse as zero and the second
// must not escape the documented 0-1 scale.
func TestParseConfidence_Matrix(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"85", 0.85, true},
		{"85%", 0.85, true},
		{"0.85", 0.85, true},
		{"100", 1.0, true},
		{"150", 1.0, true},
		{"-20", 0.0, true},
		{"high", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseConfidence(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseConfidence(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestConsult_BlankSpecialistIsRequestErrorWithoutSpawning(t *testing.T) {
	var calls atomic.Int32
	m := New(funcSpawner(func(context.Context, string, string) (string, error) {
		calls.Add(1)
		return "ADVICE: fine", nil
	}))
	for _, spec := range []string{"", "   "} {
		if _, err := m.Consult(context.Background(), Request{Specialist: spec, Question: "q"}); err == nil || strings.Contains(err.Error(), "failed:") {
			t.Fatalf("Consult(%q) err = %v, want request error without spawner wrapping", spec, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("spawner calls = %d, want 0: blank specialist must not spawn", calls.Load())
	}
}

func TestConsult_NilManagerFailsVisibly(t *testing.T) {
	var m *Manager
	if _, err := m.Consult(context.Background(), Request{Specialist: "coder"}); !errors.Is(err, ErrNoSpawner) {
		t.Fatalf("nil manager err = %v, want ErrNoSpawner", err)
	}
}

func TestConsult_ConcurrentIdenticalFailuresShareOneSpawnAndRetry(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	m := New(funcSpawner(func(ctx context.Context, spec, task string) (string, error) {
		if calls.Add(1) == 1 {
			select {
			case <-release:
			case <-ctx.Done():
				return "", ctx.Err()
			}
			return "", errors.New("boom")
		}
		return "ADVICE: recovered", nil
	}))
	req := Request{Specialist: "coder", Question: "q"}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = m.Consult(context.Background(), req)
		})
	}
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	deadline = time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		joined := len(m.inflight) == 1 && m.inflight[contentKey(Request{Requester: DefaultRequester, Specialist: "coder", Question: "q"})].waiters == 2
		m.mu.Unlock()
		if joined || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("spawner calls = %d, want 1: concurrent failures share one computation", calls.Load())
	}
	for _, err := range errs {
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("coalesced failure err = %v, want boom", err)
		}
	}
	m.mu.Lock()
	n := len(m.inflight)
	m.mu.Unlock()
	if n != 0 {
		t.Fatalf("inflight entries = %d after failure, want 0: failures are never retained", n)
	}
	if _, err := m.Consult(context.Background(), req); err != nil {
		t.Fatalf("retry after failure err = %v, want success", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("spawner calls = %d, want 2: failure must not be cached", calls.Load())
	}
}

func TestConsult_CancelledCtxPrefersCompletedSuccess(t *testing.T) {
	var calls atomic.Int32
	m := New(funcSpawner(func(context.Context, string, string) (string, error) {
		calls.Add(1)
		return "ADVICE: must not be called", nil
	}))
	base := Request{Requester: "coder", Specialist: "coder", Question: "q"}
	key := contentKey(base)
	artifact := parseArtifact(key, "coder", "ADVICE:\nuse interfaces\nCONFIDENCE: 80\nREFERENCES:\nref-a\nCAVEATS:\nwatch cycles", time.Millisecond)
	f := &flight{waiters: 1, done: make(chan struct{}), artifact: artifact}
	close(f.done)
	m.mu.Lock()
	m.inflight[key] = f
	m.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 100 {
		resp, err := m.Consult(ctx, base)
		if err != nil {
			t.Fatalf("cancelled consumer with completed computation err = %v, want success: completed receipt wins over abandonment", err)
		}
		if resp.Advice != "use interfaces" {
			t.Fatalf("advice = %q, want %q", resp.Advice, "use interfaces")
		}
		if resp.Reuse != ReuseCoalesced {
			t.Fatalf("reuse = %q, want %q: joining a completed flight is coalesced", resp.Reuse, ReuseCoalesced)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("spawner calls = %d, want 0: completed flight must not spawn", calls.Load())
	}
}

func TestConsult_CancelledCtxPrefersCompletedFailure(t *testing.T) {
	var calls atomic.Int32
	m := New(funcSpawner(func(context.Context, string, string) (string, error) {
		calls.Add(1)
		return "ADVICE: must not be called", nil
	}))
	base := Request{Requester: "coder", Specialist: "coder", Question: "q"}
	key := contentKey(base)
	f := &flight{waiters: 1, done: make(chan struct{}), err: errors.New("boom")}
	close(f.done)
	m.mu.Lock()
	m.inflight[key] = f
	m.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 100 {
		_, err := m.Consult(ctx, base)
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("cancelled consumer with failed computation err = %v, want boom: completed failure wins over abandonment", err)
		}
		if strings.Contains(err.Error(), "abandoned") {
			t.Fatalf("cancelled consumer with failed computation err = %v, must not report abandonment when the computation already failed", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("spawner calls = %d, want 0: completed flight must not spawn", calls.Load())
	}
}

package consultation

import (
	"context"
	"errors"
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

package broker

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// collect drains a broker stream and returns its text and terminal result. It
// takes the stream as returned, so a call can be passed straight in.
func collect(content <-chan string, done <-chan types.StreamResult) (string, types.StreamResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return types.CollectStream(ctx, content, done, nil)
}

func TestStreamingSettlesBeforeTheTerminalIsPublished(t *testing.T) {
	// Settling at return time would record every streamed turn as free, which
	// is most of the traffic in chat mode. Settling after channel closure, with
	// nothing a consumer could wait on, lost the last receipt of a process that
	// exited when its stream closed. The terminal is that thing to wait on.
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.streamChunks = []string{"hel", "lo ", "world"}
	fake.reportInput, fake.reportOutput = 900, 300

	client := meteredClient(t, fake, meter, sink)
	got, res := collect(client.CompleteWithStreaming(context.Background(), "sys", "user", false))

	if res.Err != nil || res.StopReason != "end_turn" {
		t.Fatalf("terminal = %+v, want a complete end_turn result", res)
	}
	if got != "hello world" {
		t.Errorf("stream content = %q, want %q", got, "hello world")
	}
	// No waiting: observing the terminal is the proof accounting finished.
	r, ok := sink.last()
	if !ok || r.Method != "CompleteWithStreaming" {
		t.Fatalf("no streaming receipt by the time the terminal was observed: %+v", r)
	}
	total := meter.Ledger().Total()
	if total.InputTokens != 900 || total.OutputTokens != 300 {
		t.Errorf("streamed usage not recorded: %+v", total)
	}
}

func TestStreamingRefusalDeliversErrorOnTheTerminal(t *testing.T) {
	// A streaming caller reads channels; returning nil channels or panicking
	// would take down the chat loop.
	meter := NewMeter(MeterConfig{Window: 1000, OutputReserve: 900})
	fake := newFakeClient()

	cfg := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	cfg.Counter = fixedCounter{tokens: 9000, confidence: ConfidenceExact}
	client, err := Wrap(fake, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	got, res := collect(client.CompleteWithStreaming(context.Background(), "sys", "user", false))
	if got != "" {
		t.Error("a refused stream produced content")
	}
	if _, ok := IsAdmissionError(res.Err); !ok {
		t.Errorf("a refused stream's terminal = %v, want an admission error", res.Err)
	}
	if len(fake.callNames()) != 0 {
		t.Error("a refused stream still reached the provider")
	}
}

func TestAbandonedStreamDoesNotLeakGoroutines(t *testing.T) {
	// A consumer that walks away mid-stream — the user hits Ctrl+C, a timeout
	// fires — must not strand the forwarders on a send nobody will receive.
	meter := testMeter(200000, 8000)

	fake := newFakeClient()
	for i := 0; i < 5000; i++ {
		fake.streamChunks = append(fake.streamChunks, "chunk")
	}

	client := meteredClient(t, fake, meter, nil)

	settle := func() { runtime.GC(); time.Sleep(50 * time.Millisecond) }
	settle()
	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		content, _ := client.CompleteWithStreaming(ctx, "sys", "user", false)
		// Read exactly one chunk, then abandon the stream entirely.
		<-content
		cancel()
	}

	waitFor(t, func() bool {
		runtime.GC()
		return runtime.NumGoroutine() <= before+8
	})

	after := runtime.NumGoroutine()
	if after > before+8 {
		t.Errorf("abandoned streams leaked goroutines: %d before, %d after", before, after)
	}
}

func TestStreamingErrorReachesTheReceipt(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.streamChunks = []string{"partial"}
	fake.respErr = context.DeadlineExceeded

	client := meteredClient(t, fake, meter, sink)
	_, res := collect(client.CompleteWithStreaming(context.Background(), "sys", "user", false))
	if !errors.Is(res.Err, context.DeadlineExceeded) {
		t.Fatalf("terminal = %v, want the provider's error", res.Err)
	}
	r, _ := sink.last()
	if r.Err == "" {
		t.Error("a stream that ended in error produced a receipt claiming success")
	}
}

// A stream that closes without a terminal is incomplete, for the consumer and
// on the receipt. Treating closure as success is how a cut answer used to be
// accepted whole.
func TestStreamClosedWithoutTerminalIsIncompleteEverywhere(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.streamChunks = []string{"{\"surface_response\": \"looks whole\"}"}
	fake.streamNoTerminal = true

	client := meteredClient(t, fake, meter, sink)
	_, res := collect(client.CompleteWithStreaming(context.Background(), "sys", "user", false))
	if !errors.Is(res.Err, types.ErrStreamIncomplete) || res.Outcome() != types.StreamIncomplete {
		t.Fatalf("terminal = %v (%s), want incomplete", res.Err, res.Outcome())
	}
	r, _ := sink.last()
	if !strings.Contains(r.Err, "incomplete") {
		t.Errorf("receipt error = %q, want the incomplete outcome recorded", r.Err)
	}
}

// The detail-transport witness: a streamed Anthropic-style report's cache
// categories reach the receipt unchanged. Before the normalized report the
// observer carried input/output only and stream settlement passed no
// response metadata, so every streamed cache count became zero in the
// receipt while usage.json kept it.
func TestStreamedCacheDetailReachesTheReceipt(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.streamChunks = []string{"ok"}
	fake.reportUsage = &types.UsageMetadata{
		InputTokens: 1000, OutputTokens: 200, CachedContentTokens: 600, CacheWriteTokens: 300,
		Known: types.UsageTokens | types.UsageCacheRead | types.UsageCacheWrite,
	}
	fake.streamUsage = *fake.reportUsage

	client := meteredClient(t, fake, meter, sink)
	if _, res := collect(client.CompleteWithStreaming(context.Background(), "sys", "user", false)); res.Err != nil {
		t.Fatalf("terminal: %v", res.Err)
	}
	r, _ := sink.last()
	if r.Actual.CachedTokens != 600 || r.Actual.CacheWriteTokens != 300 {
		t.Fatalf("receipt cache = read %d write %d, want 600/300", r.Actual.CachedTokens, r.Actual.CacheWriteTokens)
	}
	if r.UsageStatus != UsageReported || !(types.UsageMetadata{Known: r.UsageKnown}).Has(types.UsageCacheRead|types.UsageCacheWrite) {
		t.Fatalf("receipt status %q known %b, want reported with both cache categories", r.UsageStatus, r.UsageKnown)
	}
	if r.UsageConflict != "" {
		t.Fatalf("agreeing routes recorded a conflict: %s", r.UsageConflict)
	}
	if got := meter.Ledger().Total(); got.InputTokens != 1000 || got.OutputTokens != 200 {
		t.Fatalf("ledger = %+v, want exactly one charge of 1000/200", got)
	}
}

// waitFor polls cond until it holds or the deadline passes, for state that
// settles on a goroutine with nothing to wait on.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not hold within the deadline")
}

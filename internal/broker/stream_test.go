package broker

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestStreamingSettlesWhenTheStreamEnds(t *testing.T) {
	// Settling at return time would record every streamed turn as free, which
	// is most of the traffic in chat mode.
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.streamChunks = []string{"hel", "lo ", "world"}
	fake.reportInput, fake.reportOutput = 900, 300

	client := meteredClient(t, fake, meter, sink)

	content, errs := client.CompleteWithStreaming(context.Background(), "sys", "user", false)

	var got string
	for chunk := range content {
		got += chunk
	}
	for range errs {
	}

	if got != "hello world" {
		t.Errorf("stream content = %q, want %q", got, "hello world")
	}

	// The settling goroutine runs after both channels drain, and the receipt is
	// the last thing it emits, after the ledger, so waiting on the ledger can
	// read the sink before the receipt exists.
	waitFor(t, func() bool { _, ok := sink.last(); return ok })

	total := meter.Ledger().Total()
	if total.InputTokens != 900 || total.OutputTokens != 300 {
		t.Errorf("streamed usage not recorded: %+v", total)
	}
	if r, ok := sink.last(); !ok || r.Method != "CompleteWithStreaming" {
		t.Errorf("no streaming receipt emitted: %+v", r)
	}
}

func TestStreamingRefusalDeliversErrorOnTheChannel(t *testing.T) {
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

	content, errs := client.CompleteWithStreaming(context.Background(), "sys", "user", false)

	for range content {
		t.Error("a refused stream produced content")
	}

	var sawErr bool
	for e := range errs {
		if _, ok := IsAdmissionError(e); ok {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("a refused stream did not deliver an admission error on the error channel")
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
	content, errs := client.CompleteWithStreaming(context.Background(), "sys", "user", false)
	for range content {
	}
	for range errs {
	}

	waitFor(t, func() bool {
		r, ok := sink.last()
		return ok && r.Err != ""
	})

	r, _ := sink.last()
	if r.Err == "" {
		t.Error("a stream that ended in error produced a receipt claiming success")
	}
}

// waitFor polls cond until it holds or the deadline passes. Streaming settles
// on a goroutine, so a bare assertion would race the thing it is asserting.
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

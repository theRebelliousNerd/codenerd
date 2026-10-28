package broker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// gateSink holds every receipt until released, so a test can observe a call
// that was admitted and has not settled.
type gateSink struct {
	release chan struct{}
	inner   captureSink
	once    sync.Once
}

func (g *gateSink) Record(r Receipt) {
	<-g.release
	g.inner.Record(r)
}

func (g *gateSink) open() { g.once.Do(func() { close(g.release) }) }

// A consumer that walks away from a stream never observes its terminal. Drain
// is the barrier a shutdown or cost export waits on: it reports pending
// accounting instead of letting the caller read a log that is about to gain a
// receipt, and returns only once that receipt is recorded.
func TestDrainWaitsForAnAbandonedStreamsReceipt(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &gateSink{release: make(chan struct{})}
	t.Cleanup(sink.open)
	fake := newFakeClient()
	fake.streamChunks = []string{"a", "b"}
	fake.reportInput, fake.reportOutput = 100, 10
	client := meteredClient(t, fake, meter, sink)

	ctx, cancel := context.WithCancel(context.Background())
	client.CompleteWithStreaming(ctx, "sys", "user", false) // channels are never read
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for Pending() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if Pending() != 1 {
		t.Fatalf("Pending = %d, want the admitted stream in flight", Pending())
	}
	short, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stop()
	if err := Drain(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain with accounting pending = %v, want the deadline", err)
	}
	if got := sink.inner.all(); len(got) != 0 {
		t.Fatalf("receipt recorded before the sink released it: %d", len(got))
	}

	sink.open()
	wait, stop2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop2()
	if err := Drain(wait); err != nil {
		t.Fatalf("Drain after the sink released = %v", err)
	}
	if got := sink.inner.all(); len(got) != 1 || got[0].Method != "CompleteWithStreaming" {
		t.Fatalf("receipts after Drain = %+v, want the stream's", got)
	}
	if Pending() != 0 {
		t.Fatalf("Pending = %d after Drain", Pending())
	}
}

func TestDrainReturnsAtOnceWhenNothingIsInFlight(t *testing.T) {
	if err := Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
}

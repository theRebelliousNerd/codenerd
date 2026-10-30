package broker

import (
	"context"
	"strings"
	"testing"
	"time"
)

// startAbandonedStream admits a stream whose consumer walks away, settling
// into gate, and waits until it is in flight.
func startAbandonedStream(t *testing.T, gate *gateSink) {
	t.Helper()
	fake := newFakeClient()
	fake.streamChunks = []string{"a", "b"}
	fake.reportInput, fake.reportOutput = 100, 10
	client := meteredClient(t, fake, testMeter(200000, 8000), gate)
	ctx, cancel := context.WithCancel(context.Background())
	client.CompleteWithStreaming(ctx, "sys", "user", false)
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for Pending() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if Pending() == 0 {
		t.Fatal("fixture: the stream never came in flight")
	}
}

func installExtraSink(sink ReceiptSink) {
	m := Default()
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.setExtraSinkLocked(sink)
}

func extraSinkIs(sink ReceiptSink) bool {
	m := Default()
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.extraSink == sink
}

// Cortex.Close detached and closed the receipt log while calls were still
// settling, and a closed log drops their receipts silently. The detach now
// waits for accounting first; a shutdown that cannot wait detaches all the
// same and says how many receipts it lost.
func TestDrainAndDetachExtraSink_DetachesOnlyAfterAccountingSettles(t *testing.T) {
	gate := &gateSink{release: make(chan struct{})}
	t.Cleanup(gate.open)
	startAbandonedStream(t, gate)
	extra := &captureSink{}
	installExtraSink(extra)
	t.Cleanup(func() { _, _ = DetachExtraSink(extra) })

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := DrainAndDetachExtraSink(ctx, extra)
		done <- err
	}()
	time.Sleep(30 * time.Millisecond)
	if !extraSinkIs(extra) {
		t.Fatal("the extra sink was detached while a call was still settling")
	}
	gate.open()
	if err := <-done; err != nil {
		t.Fatalf("DrainAndDetachExtraSink = %v", err)
	}
	if extraSinkIs(extra) || len(gate.inner.all()) != 1 {
		t.Fatalf("after settling: still attached=%v, receipts=%d", extraSinkIs(extra), len(gate.inner.all()))
	}
}

func TestDrainAndDetachExtraSink_ReportsTheReceiptsAShutdownCannotWaitFor(t *testing.T) {
	gate := &gateSink{release: make(chan struct{})}
	t.Cleanup(gate.open)
	startAbandonedStream(t, gate)
	extra := &captureSink{}
	installExtraSink(extra)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	detached, err := DrainAndDetachExtraSink(ctx, extra)
	if !detached || err == nil || !strings.Contains(err.Error(), "1 metered call(s) still settling") {
		t.Fatalf("detached=%v err=%v; want the detach done and the lost receipt named", detached, err)
	}
	gate.open()
	wait, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := Drain(wait); err != nil {
		t.Fatal(err)
	}
}

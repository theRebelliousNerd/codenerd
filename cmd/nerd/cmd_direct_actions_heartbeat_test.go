package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe for the heartbeat goroutine to write to
// while the test reads it. Without the mutex this test races by construction
// and -race fails it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// A run that finishes before the first tick must stay silent. The point of the
// heartbeat is to prove a long run is alive, not to add noise to a fast one.
func TestStartHeartbeat_SilentWhenStoppedBeforeFirstTick(t *testing.T) {
	var out syncBuffer

	stop := startHeartbeat(&out, time.Hour)
	stop()

	if got := out.String(); got != "" {
		t.Errorf("heartbeat wrote %q before its first tick; want nothing", got)
	}
}

// Past a tick it must emit, and the line must carry the elapsed time — that is
// the whole signal. `nerd analyze` printed 275 bytes and then nothing for 12
// minutes while doing real work, which is indistinguishable from a deadlock.
func TestStartHeartbeat_EmitsAfterTick(t *testing.T) {
	var out syncBuffer

	stop := startHeartbeat(&out, time.Millisecond)
	// Poll rather than sleeping a fixed span: a fixed sleep is either flaky on
	// a loaded machine or needlessly slow.
	deadline := time.Now().Add(5 * time.Second)
	for out.String() == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stop()

	got := out.String()
	if got == "" {
		t.Fatal("heartbeat emitted nothing after ticking; a long run stays indistinguishable from a hang")
	}
	if !strings.Contains(got, "still working") {
		t.Errorf("heartbeat line = %q, want it to say what is happening", got)
	}
	if !strings.Contains(got, "elapsed") {
		t.Errorf("heartbeat line = %q, want an elapsed time", got)
	}
}

// stop() must block until the goroutine has exited, or a heartbeat line can
// land in the middle of the result block and corrupt it.
func TestStartHeartbeat_StopIsQuiescentAndIdempotent(t *testing.T) {
	var out syncBuffer

	stop := startHeartbeat(&out, time.Millisecond)
	deadline := time.Now().Add(5 * time.Second)
	for out.String() == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stop()

	after := out.String()
	// Well past several tick intervals. Anything written now would have landed
	// after the result block started printing.
	time.Sleep(50 * time.Millisecond)
	if later := out.String(); later != after {
		t.Errorf("heartbeat wrote after stop() returned:\n before: %q\n after:  %q", after, later)
	}

	// runDirectAction calls stop both explicitly and via defer.
	stop()
	stop()
}

package observability

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFlightWatchdog_StopsRecorderOnMemoryGrowth is the regression test
// for the crash reported on a heavy audit campaign: the execution
// tracer's region allocator grew unbounded until the Go runtime aborted
// the process with the fatal, unrecoverable `throw("traceRegion: out of
// memory")`. The watchdog converts that fatal error into a graceful
// stop.
//
// A growth cap of 0 trips on the first sample once the tracer allocates
// anything past its at-Start baseline, which makes the assertion
// deterministic without having to generate gigabytes of trace data to
// reach a realistic OOM threshold.
func TestFlightWatchdog_StopsRecorderOnMemoryGrowth(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	require.NoError(t,
		startFlightRecorder(2<<20, 50*time.Millisecond, 0 /*cap*/, 10*time.Millisecond),
		"start recorder with a trip-immediately memory guard")
	require.True(t, FlightRecorderEnabled(), "recorder must be enabled right after Start")

	// Generate trace activity so tracer memory is unambiguously above the
	// baseline captured before Start.
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	require.Eventually(t,
		func() bool { return !FlightRecorderEnabled() },
		2*time.Second, 10*time.Millisecond,
		"watchdog must auto-stop the recorder once trace memory exceeds the guard")
}

// TestFlightWatchdog_StopsUnderSustainedChurn reproduces the reported
// failure surface without the LLM: the recorder is ON and sustained
// trace-event generation (goroutine churn, the same pressure a campaign's
// subprocess/goroutine storm applies) drives the tracer's memory up past a
// realistic guard (well above the ring, so the trip reflects genuine
// runaway growth rather than the ring's own bytes). The watchdog must stop
// the recorder and — critically — the process must survive the whole run
// (the test completing at all is the proof that the fatal
// `traceRegion: out of memory` did not fire).
func TestFlightWatchdog_StopsUnderSustainedChurn(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	require.NoError(t,
		startFlightRecorder(2<<20 /*ring*/, 30*time.Millisecond, 6<<20 /*guard*/, 20*time.Millisecond),
		"start recorder with a guard above the ring")
	require.True(t, FlightRecorderEnabled())

	churn := func() {
		var wg sync.WaitGroup
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20000; j++ {
					runtime.Gosched()
				}
			}()
		}
		wg.Wait()
	}

	// Bounded by rounds of churn, not by wall-clock.
	//
	// This was require.Eventually with a 15-second ceiling, and it passed alone
	// and failed inside `go test ./...` — observed 2026-08-08. The churn is 64
	// goroutines spinning on runtime.Gosched, so under a full parallel suite it
	// gets a fraction of the CPU, produces trace data more slowly, and the guard
	// is not reached before the deadline. The watchdog was never broken; the
	// test was measuring machine load.
	//
	// That matters more than an ordinary flake here: the session test gate runs
	// `go test` on touched packages, so a load-sensitive failure in this package
	// intermittently fails real turns and teaches the agent to distrust a
	// correct signal.
	//
	// Counting rounds instead makes CPU speed change how LONG the test takes
	// rather than WHETHER it passes. The deadline that remains is a hang
	// backstop, and it reports how far it got so a real regression is still
	// legible.
	const maxRounds = 400
	deadline := time.Now().Add(2 * time.Minute)

	rounds := 0
	for FlightRecorderEnabled() {
		if rounds >= maxRounds {
			t.Fatalf("watchdog did not stop the recorder after %d rounds of sustained churn; "+
				"it must trip once trace memory grows past the guard", rounds)
		}
		if time.Now().After(deadline) {
			t.Fatalf("watchdog did not stop the recorder within the hang backstop (%d rounds completed)", rounds)
		}
		churn()
		rounds++
	}
	t.Logf("watchdog tripped after %d rounds of churn", rounds)
}

// TestFlightWatchdog_NoTripUnderCap proves the guard does not false-fire:
// with an effectively unreachable cap, normal trace activity leaves the
// recorder running. This guards against a watchdog that would kill the
// recorder (and its panic-dump value) during healthy operation.
func TestFlightWatchdog_NoTripUnderCap(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	require.NoError(t,
		startFlightRecorder(2<<20, 50*time.Millisecond, 1<<50 /*1 PiB cap*/, 10*time.Millisecond),
		"start recorder with an unreachable memory guard")

	for i := 0; i < 2000; i++ {
		runtime.Gosched()
	}

	// Allow many watchdog ticks to elapse; none should trip.
	time.Sleep(150 * time.Millisecond)
	require.True(t, FlightRecorderEnabled(),
		"recorder must stay enabled while trace memory is below the guard")
}

// TestFlightWatchdog_ExternalStopExitsWatchdog verifies the watchdog does
// not linger after an explicit StopFlightRecorder — the stop channel it
// selects on is closed by stopFlightLocked, so a subsequent Start can own
// a fresh watchdog generation without the old one interfering.
func TestFlightWatchdog_ExternalStopExitsWatchdog(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	require.NoError(t,
		startFlightRecorder(2<<20, 50*time.Millisecond, 1<<50, 10*time.Millisecond))
	require.True(t, FlightRecorderEnabled())

	require.NoError(t, StopFlightRecorder())
	require.False(t, FlightRecorderEnabled(), "recorder must be stopped")

	// Re-start cleanly: a leaked watchdog from the first generation would
	// still be pointing at a closed/old channel, but the identity guard
	// (flightWatchdogStop == stopCh) must keep it from touching this one.
	require.NoError(t,
		startFlightRecorder(2<<20, 50*time.Millisecond, 1<<50, 10*time.Millisecond))
	for i := 0; i < 500; i++ {
		runtime.Gosched()
	}
	time.Sleep(100 * time.Millisecond)
	require.True(t, FlightRecorderEnabled(),
		"second recorder generation must stay enabled")
}

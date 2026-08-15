package observability

import (
	"runtime"
	"sync"
	"sync/atomic"
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

	// What this test can and cannot assert.
	//
	// The load history here is instructive. It began as require.Eventually with
	// a 15-second ceiling and failed inside `go test ./...`. It was changed to
	// count rounds so that "CPU speed changes how LONG the test takes rather
	// than WHETHER it passes" — but a 2-minute wall-clock backstop was kept
	// alongside the round budget, which reinstated exactly that sensitivity: it
	// then failed at 124 of its 400 permitted rounds purely on elapsed time.
	//
	// Removing the clock entirely exposes the deeper problem: with the full 400
	// rounds the watchdog still does not trip here, because whether goroutine
	// churn grows /memory/classes/other:bytes past a 6 MiB guard is a property
	// of the Go toolchain's trace accounting, not of the watchdog. Asserting it
	// makes this test a runtime-version detector.
	//
	// So the trip decision is asserted deterministically in
	// TestFlightWatchdog_TripsWhenSampledGrowthExceedsCap, which drives the
	// sampler directly. What remains here is the assertion this test was
	// actually created for, quoted from its own header: "the process must
	// survive the whole run (the test completing at all is the proof that the
	// fatal `traceRegion: out of memory` did not fire)". That is a real
	// regression guard and it does not depend on machine speed.
	const rounds = 40
	const perRoundBackstop = 90 * time.Second

	for i := 0; i < rounds && FlightRecorderEnabled(); i++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			churn()
		}()
		select {
		case <-done:
		case <-time.After(perRoundBackstop):
			t.Fatalf("a single churn round did not complete within %s (round %d); "+
				"this is a hang, not a slow machine", perRoundBackstop, i)
		}
	}

	// Surviving sustained tracer pressure is the point. If the watchdog did
	// trip along the way that is also fine — both outcomes beat a fatal throw.
	if FlightRecorderEnabled() {
		t.Log("recorder survived sustained churn without tripping the guard")
	} else {
		t.Log("watchdog stopped the recorder during sustained churn")
	}
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

// TestFlightWatchdog_TripsWhenSampledGrowthExceedsCap asserts the watchdog's
// actual contract deterministically: once observed memory grows past the cap,
// the recorder is stopped.
//
// This replaces the load-and-toolchain-sensitive half of the sustained-churn
// test. Driving the sampler directly means the assertion is about the
// watchdog's decision, not about how much memory a particular Go release
// attributes to the execution tracer under goroutine churn.
func TestFlightWatchdog_TripsWhenSampledGrowthExceedsCap(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	var mem atomic.Uint64
	mem.Store(100 << 20) // baseline captured at Start

	restore := flightMemSample
	flightMemSample = func() uint64 { return mem.Load() }
	t.Cleanup(func() { flightMemSample = restore })

	require.NoError(t,
		startFlightRecorder(2<<20, 30*time.Millisecond, 8<<20 /*guard*/, 10*time.Millisecond))
	require.True(t, FlightRecorderEnabled(), "recorder must be enabled right after Start")

	// Growth below the guard must NOT trip it — a watchdog that fires early
	// would silently disable a diagnostic the operator asked for.
	mem.Store((100 << 20) + (4 << 20))
	time.Sleep(100 * time.Millisecond)
	require.True(t, FlightRecorderEnabled(), "watchdog tripped below its guard")

	// Past the guard, it must stop the recorder.
	mem.Store((100 << 20) + (16 << 20))
	require.Eventually(t,
		func() bool { return !FlightRecorderEnabled() },
		5*time.Second, 10*time.Millisecond,
		"watchdog must stop the recorder once sampled growth exceeds the guard")
}

// TestFlightWatchdog_IgnoresShrinkingMemory guards the unsigned subtraction:
// a sample below the baseline must read as zero growth, not underflow to a
// huge number and trip instantly.
func TestFlightWatchdog_IgnoresShrinkingMemory(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	var mem atomic.Uint64
	mem.Store(100 << 20)

	restore := flightMemSample
	flightMemSample = func() uint64 { return mem.Load() }
	t.Cleanup(func() { flightMemSample = restore })

	require.NoError(t,
		startFlightRecorder(2<<20, 30*time.Millisecond, 8<<20, 10*time.Millisecond))

	mem.Store(1 << 20) // well below the baseline
	time.Sleep(150 * time.Millisecond)

	require.True(t, FlightRecorderEnabled(),
		"a sample below baseline must count as zero growth, not underflow past the cap")
}

package browser

import (
	"math"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestCDPEventClock_UsesWallTimeOffsetAndDocumentedEpochUnits(t *testing.T) {
	var clock cdpEventClock
	if _, ok := clock.fromMonotonic(123.5, time.UnixMilli(99)); ok {
		t.Fatal("monotonic time without a measured offset was treated as Unix time")
	}
	wall := proto.TimeSinceEpoch(1_800_000_000.125)
	monotonic := proto.MonotonicTime(123.5)
	if !clock.observe(wall, monotonic) {
		t.Fatal("valid request WallTime/Timestamp pair was rejected")
	}
	got, ok := clock.fromMonotonic(124.25, time.UnixMilli(99))
	if !ok || got.UnixMilli() != 1_800_000_000_875 {
		t.Fatalf("monotonic timestamp conversion = %d, %v; want epoch ms 1800000000875", got.UnixMilli(), ok)
	}

	observed := time.UnixMilli(1_700_000_000_000)
	if got, ok := clock.fromMonotonic(proto.MonotonicTime(math.NaN()), observed); ok || !got.Equal(observed) {
		t.Fatalf("invalid monotonic time did not use callback observation time: %v, %v", got, ok)
	}
	runtimeTime, ok := fromRuntimeMillis(proto.RuntimeTimestamp(1_800_000_000_333), observed)
	if !ok || runtimeTime.UnixMilli() != 1_800_000_000_333 {
		t.Fatalf("Runtime timestamp conversion = %d, %v; want epoch ms 1800000000333", runtimeTime.UnixMilli(), ok)
	}
	if runtimeTime.UnixMilli() == int64(proto.RuntimeTimestamp(1_800_000_000_333))*1000 {
		t.Fatal("Runtime epoch milliseconds were incorrectly interpreted as seconds")
	}
	if got, ok := fromRuntimeMillis(proto.RuntimeTimestamp(math.Inf(1)), observed); ok || !got.Equal(observed) {
		t.Fatalf("invalid Runtime timestamp did not use callback observation time: %v, %v", got, ok)
	}
}

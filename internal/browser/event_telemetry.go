package browser

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// cdpEventClock converts CDP monotonic seconds to Unix time using the paired
// Network.requestWillBeSent WallTime and Timestamp values. A callback time is
// used only before a valid pair exists or when the event timestamp is invalid.
type cdpEventClock struct {
	mu         sync.RWMutex
	offsetSecs float64
	calibrated bool
}

func (c *cdpEventClock) observe(wall proto.TimeSinceEpoch, monotonic proto.MonotonicTime) bool {
	wallSecs, monotonicSecs := float64(wall), float64(monotonic)
	if !validCDPSeconds(wallSecs) || !validCDPSeconds(monotonicSecs) || wallSecs <= 0 {
		return false
	}
	offset := wallSecs - monotonicSecs
	if !validCDPSeconds(offset) {
		return false
	}
	c.mu.Lock()
	c.offsetSecs = offset
	c.calibrated = true
	c.mu.Unlock()
	return true
}

func (c *cdpEventClock) fromMonotonic(timestamp proto.MonotonicTime, observed time.Time) (time.Time, bool) {
	monotonicSecs := float64(timestamp)
	if !validCDPSeconds(monotonicSecs) {
		return observed, false
	}
	c.mu.RLock()
	offset, calibrated := c.offsetSecs, c.calibrated
	c.mu.RUnlock()
	if !calibrated {
		return observed, false
	}
	epochSecs := offset + monotonicSecs
	if !validCDPSeconds(epochSecs) || epochSecs <= 0 {
		return observed, false
	}
	return time.UnixMilli(int64(math.Round(epochSecs * 1000))), true
}

// fromRuntimeMillis uses CDP Runtime/Log's documented epoch-millisecond
// timestamp directly. It does not reinterpret network monotonic seconds.
func fromRuntimeMillis(timestamp proto.RuntimeTimestamp, observed time.Time) (time.Time, bool) {
	value := float64(timestamp)
	if !validCDPMillis(value) || value <= 0 {
		return observed, false
	}
	return time.UnixMilli(int64(math.Round(value))), true
}

func validCDPSeconds(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 253402300799
}

func validCDPMillis(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 253402300799999
}

func cancelSessionEventStream(record *sessionRecord) {
	if record == nil {
		return
	}
	if record.streamCancel != nil {
		record.streamCancel()
	}
}

func waitSessionEventStream(record *sessionRecord) {
	if record == nil {
		return
	}
	if record.streamDone != nil {
		<-record.streamDone
	}
}

func stopSessionEventStream(record *sessionRecord) {
	cancelSessionEventStream(record)
	waitSessionEventStream(record)
}

func isEventStreamProbe(ev *proto.RuntimeConsoleAPICalled, token string) bool {
	return ev != nil && token != "" && stringifyConsoleArgs(ev.Args) == token
}

// waitForEventListeners sends an inert console marker until each EachEvent
// loop has received it. Rod enables event domains when EachEvent is created,
// but subscribes to the event bus only when the returned wait function starts.
// The marker closes that gap before CreateTab performs its first navigation.
func waitForEventListeners(ctx context.Context, page *rod.Page, token string, listeners ...<-chan struct{}) error {
	if len(listeners) == 0 {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	js := fmt.Sprintf("() => { console.debug(%q); return true; }", token)
	ready := make([]bool, len(listeners))
	remaining := len(listeners)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for remaining > 0 {
		if _, err := page.Context(probeCtx).Evaluate(&rod.EvalOptions{JS: js, ByValue: true}); err != nil {
			return fmt.Errorf("probe browser event listeners: %w", err)
		}
		for i, listener := range listeners {
			if ready[i] {
				continue
			}
			select {
			case <-listener:
				ready[i] = true
				remaining--
			default:
			}
		}
		if remaining == 0 {
			return nil
		}
		select {
		case <-probeCtx.Done():
			return fmt.Errorf("wait for browser event listeners: %w", probeCtx.Err())
		case <-ticker.C:
		}
	}
	return nil
}

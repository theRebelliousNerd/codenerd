package ui

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_SingleCall(t *testing.T) {
	var called int32
	debouncer := NewDebouncer(50 * time.Millisecond)

	debouncer.Debounce(func() {
		atomic.AddInt32(&called, 1)
	})

	// Wait for debounce to execute
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("Expected 1 call, got %d", called)
	}
}

func TestDebouncer_RapidCalls(t *testing.T) {
	var called int32
	var lastValue int32
	debouncer := NewDebouncer(50 * time.Millisecond)

	// Rapid successive calls
	for i := 1; i <= 10; i++ {
		value := int32(i)
		debouncer.Debounce(func() {
			atomic.StoreInt32(&lastValue, value)
			atomic.AddInt32(&called, 1)
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for final debounce
	time.Sleep(100 * time.Millisecond)

	// Should only call once with the last value
	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("Expected 1 call for rapid succession, got %d", called)
	}

	if atomic.LoadInt32(&lastValue) != 10 {
		t.Errorf("Expected last value 10, got %d", lastValue)
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	var called int32
	debouncer := NewDebouncer(50 * time.Millisecond)

	debouncer.Debounce(func() {
		atomic.AddInt32(&called, 1)
	})

	// Cancel before execution
	time.Sleep(10 * time.Millisecond)
	debouncer.Cancel()

	// Wait to ensure it doesn't execute
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&called) != 0 {
		t.Errorf("Expected 0 calls after cancel, got %d", called)
	}
}

func TestDebouncer_Immediate(t *testing.T) {
	var called int32
	debouncer := NewDebouncer(50 * time.Millisecond)

	// Schedule a debounced call
	debouncer.Debounce(func() {
		atomic.AddInt32(&called, 1)
	})

	// Execute immediately (should cancel pending and run now)
	debouncer.Immediate(func() {
		atomic.AddInt32(&called, 10)
	})

	// Wait to ensure debounced call doesn't execute
	time.Sleep(100 * time.Millisecond)

	// Should only have the immediate call
	if atomic.LoadInt32(&called) != 10 {
		t.Errorf("Expected 10 (immediate only), got %d", called)
	}
}

func TestResizeDebouncer_SingleResize(t *testing.T) {
	var calledWidth, calledHeight int32
	rd := NewResizeDebouncer(50 * time.Millisecond)

	rd.Resize(800, 600, func(w, h int) {
		atomic.StoreInt32(&calledWidth, int32(w))
		atomic.StoreInt32(&calledHeight, int32(h))
	})

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&calledWidth) != 800 {
		t.Errorf("Expected width 800, got %d", calledWidth)
	}
	if atomic.LoadInt32(&calledHeight) != 600 {
		t.Errorf("Expected height 600, got %d", calledHeight)
	}
}

func TestResizeDebouncer_RapidResizes(t *testing.T) {
	var callCount int32
	var finalWidth, finalHeight int32
	rd := NewResizeDebouncer(50 * time.Millisecond)

	// Simulate rapid resize events
	for i := 1; i <= 10; i++ {
		width := 800 + i*10
		height := 600 + i*10
		rd.Resize(width, height, func(w, h int) {
			atomic.AddInt32(&callCount, 1)
			atomic.StoreInt32(&finalWidth, int32(w))
			atomic.StoreInt32(&finalHeight, int32(h))
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for final debounce
	time.Sleep(100 * time.Millisecond)

	// Should only call once with final dimensions
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected 1 handler call, got %d", callCount)
	}

	if atomic.LoadInt32(&finalWidth) != 900 {
		t.Errorf("Expected final width 900, got %d", finalWidth)
	}
	if atomic.LoadInt32(&finalHeight) != 700 {
		t.Errorf("Expected final height 700, got %d", finalHeight)
	}
}

func TestResizeDebouncer_GetLastSize(t *testing.T) {
	rd := NewResizeDebouncer(50 * time.Millisecond)

	rd.Resize(1024, 768, func(w, h int) {
		// Handler
	})

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	w, h := rd.GetLastSize()
	if w != 1024 || h != 768 {
		t.Errorf("Expected last size (1024, 768), got (%d, %d)", w, h)
	}
}

func BenchmarkDebouncer_RapidCalls(b *testing.B) {
	debouncer := NewDebouncer(10 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		debouncer.Debounce(func() {
			// No-op
		})
	}

	// Cancel to clean up
	debouncer.Cancel()
}

func BenchmarkResizeDebouncer_RapidResizes(b *testing.B) {
	rd := NewResizeDebouncer(10 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rd.Resize(800+i, 600+i, func(w, h int) {
			// No-op
		})
	}

	// Cancel to clean up
	rd.Cancel()
}

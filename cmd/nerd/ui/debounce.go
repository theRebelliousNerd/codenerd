// Package ui provides debouncing utilities for event handling
package ui

import (
	"sync"
	"time"
)

// Debouncer provides debouncing for rapid events like window resizes
type Debouncer struct {
	mu       sync.Mutex
	timer    *time.Timer
	duration time.Duration
}

// NewDebouncer creates a new debouncer with the specified duration
func NewDebouncer(duration time.Duration) *Debouncer {
	return &Debouncer{
		duration: duration,
	}
}

// Debounce executes the function after the debounce duration has elapsed
// without any new calls. Rapid successive calls reset the timer.
func (d *Debouncer) Debounce(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel existing timer if any
	if d.timer != nil {
		d.timer.Stop()
	}

	// Create new timer
	d.timer = time.AfterFunc(d.duration, fn)
}

// Cancel cancels any pending debounced function call
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// Immediate executes the function immediately and cancels any pending call
func (d *Debouncer) Immediate(fn func()) {
	d.Cancel()
	fn()
}

// ResizeDebouncer is a specialized debouncer for window resize events
type ResizeDebouncer struct {
	debouncer    *Debouncer
	lastWidth    int
	lastHeight   int
	pendingWidth  int
	pendingHeight int
	mu           sync.Mutex
}

// NewResizeDebouncer creates a debouncer optimized for resize events
func NewResizeDebouncer(duration time.Duration) *ResizeDebouncer {
	return &ResizeDebouncer{
		debouncer: NewDebouncer(duration),
	}
}

// Resize debounces a resize event, calling the handler only after the specified duration
func (rd *ResizeDebouncer) Resize(width, height int, handler func(int, int)) {
	rd.mu.Lock()
	rd.pendingWidth = width
	rd.pendingHeight = height
	rd.mu.Unlock()

	rd.debouncer.Debounce(func() {
		rd.mu.Lock()
		w, h := rd.pendingWidth, rd.pendingHeight
		rd.lastWidth = w
		rd.lastHeight = h
		rd.mu.Unlock()

		handler(w, h)
	})
}

// GetLastSize returns the last processed size
func (rd *ResizeDebouncer) GetLastSize() (width, height int) {
	rd.mu.Lock()
	defer rd.mu.Unlock()
	return rd.lastWidth, rd.lastHeight
}

// Cancel cancels any pending resize
func (rd *ResizeDebouncer) Cancel() {
	rd.debouncer.Cancel()
}

// DefaultResizeDuration is the recommended debounce duration for resize events
const DefaultResizeDuration = 300 * time.Millisecond

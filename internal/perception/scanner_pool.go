// Package perception scanner pool — reuses 64KB SSE-line scratch buffers
// across streaming LLM clients to reduce GC pressure on long-running sessions.
//
// bufio.Scanner cannot be reset, so we don't pool the scanner itself — we pool
// the 64KB backing buffer that scanner.Buffer(...) installs. A fresh
// bufio.Scanner is cheap (a small struct + a few pointers); the costly
// allocation each request was the 64KB buffer.
//
// Usage:
//
//	scanner, cleanup := newPooledScanner(resp.Body, 1024*1024)
//	defer cleanup()
//	for scanner.Scan() { ... }
//
// The cleanup func returns the buffer to the pool. It MUST be called only
// after the scanner has finished — in the streaming clients this means after
// the scanner goroutine has signalled <-scanDone.
package perception

import (
	"bufio"
	"io"
	"sync"
)

// scannerBufPool reuses 64 KiB scratch buffers for bufio.Scanner instances
// used by streaming SSE response handlers. Sized at 64 KiB to match the
// existing initial-buffer hint in the streaming clients (and roughly the
// default bufio.MaxScanTokenSize).
var scannerBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 64*1024)
		return &b
	},
}

// newPooledScanner wraps r in a bufio.Scanner whose initial line buffer is
// borrowed from scannerBufPool. The returned cleanup func returns the buffer
// to the pool — callers must invoke it (via defer) once the scanner is
// guaranteed to no longer be in use.
//
// max is the upper bound on line length (passed to scanner.Buffer); callers
// should pass the same max they were previously using (typically 1 MiB for
// SSE streams that can carry large JSON deltas).
//
// If the scanner needs to grow past 64 KiB for a single line, bufio will
// allocate a larger buffer internally and use that instead — the pooled
// 64 KiB slice still goes back to the pool unchanged.
func newPooledScanner(r io.Reader, max int) (*bufio.Scanner, func()) {
	bufPtr := scannerBufPool.Get().(*[]byte)
	sc := bufio.NewScanner(r)
	sc.Buffer(*bufPtr, max)
	return sc, func() {
		scannerBufPool.Put(bufPtr)
	}
}

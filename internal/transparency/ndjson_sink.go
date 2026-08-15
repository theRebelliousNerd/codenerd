package transparency

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// NDJSONEventEnvVar names the file that headless runs write their Glass Box
// stream to. Interactive chat has a TUI subscriber; `nerd run` and campaign
// assaults have none, so without this their event stream is discarded at the
// bus and a post-mortem has nothing to read.
//
//	CODENERD_GLASSBOX_NDJSON=/tmp/run.ndjson nerd run "…"
const NDJSONEventEnvVar = "CODENERD_GLASSBOX_NDJSON"

// ndjsonEvent is the wire form. It is deliberately a separate struct from
// GlassBoxEvent: the exported field names here are a consumed format, and
// renaming a Go field should not silently break a downstream parser.
type ndjsonEvent struct {
	ID         uint64 `json:"id"`
	Timestamp  string `json:"ts"`
	Category   string `json:"category"`
	Summary    string `json:"summary"`
	Details    string `json:"details,omitempty"`
	TurnID     int    `json:"turn_id,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Source     string `json:"source,omitempty"`
}

// NDJSONSink writes each event as one JSON object per line.
type NDJSONSink struct {
	mu     sync.Mutex
	w      *bufio.Writer
	closer io.Closer
	turn   int  // when > 0, only events from this turn are written
	filter bool // whether turn filtering is active
	closed bool
}

// NewNDJSONSink writes events to w. If w is also an io.Closer it is closed
// when the sink is closed.
func NewNDJSONSink(w io.Writer) *NDJSONSink {
	s := &NDJSONSink{w: bufio.NewWriter(w)}
	if c, ok := w.(io.Closer); ok {
		s.closer = c
	}
	return s
}

// NewNDJSONFileSink opens (creating parent directories) path for append.
func NewNDJSONFileSink(path string) (*NDJSONSink, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("ndjson sink: empty path")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("ndjson sink: create dir %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("ndjson sink: open %s: %w", path, err)
	}
	return NewNDJSONSink(f), nil
}

// OnlyTurn restricts the sink to a single conversation turn. This is what a
// per-turn export attaches to a campaign artifact.
func (s *NDJSONSink) OnlyTurn(turnID int) *NDJSONSink {
	s.mu.Lock()
	s.turn = turnID
	s.filter = true
	s.mu.Unlock()
	return s
}

// Write implements EventSink. Errors are swallowed by design: transparency
// must never fail the operation it is observing.
func (s *NDJSONSink) Write(event GlassBoxEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.w == nil {
		return
	}
	if s.filter && event.TurnID != s.turn {
		return
	}

	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	line, err := json.Marshal(ndjsonEvent{
		ID:         event.ID,
		Timestamp:  ts.UTC().Format(time.RFC3339Nano),
		Category:   string(event.Category),
		Summary:    event.Summary,
		Details:    event.Details,
		TurnID:     event.TurnID,
		DurationMs: event.Duration.Milliseconds(),
		Source:     event.Source,
	})
	if err != nil {
		return
	}
	_, _ = s.w.Write(line)
	_ = s.w.WriteByte('\n')
	// Flush per event: a headless run that panics still leaves a usable file.
	_ = s.w.Flush()
}

// Close flushes and closes the underlying writer.
func (s *NDJSONSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.w != nil {
		_ = s.w.Flush()
	}
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}

// attachEnvNDJSONSink wires the env-configured sink onto a new bus.
// Silent no-op when the variable is unset; a bad path is reported once on
// stderr rather than failing boot, since telemetry must not block a run.
func attachEnvNDJSONSink(b *GlassBoxEventBus) {
	path := strings.TrimSpace(os.Getenv(NDJSONEventEnvVar))
	if path == "" {
		return
	}
	sink, err := NewNDJSONFileSink(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[transparency] %s: %v\n", NDJSONEventEnvVar, err)
		return
	}
	b.AddSink(sink)
}

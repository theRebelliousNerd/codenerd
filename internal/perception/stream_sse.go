package perception

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// sseMaxEvent bounds one assembled server-sent event. Every streaming client
// used 1 MiB as its line bound; an event can span lines, so the bound is on
// the assembled data, where an unbounded multi-line event would otherwise grow
// without limit.
const sseMaxEvent = 1024 * 1024

// sseEvent is one dispatched server-sent event.
type sseEvent struct {
	// Event is the event field ("message" when the stream named none).
	Event string
	// Data is the event's data lines joined with "\n".
	Data string
}

// sseReader frames a text/event-stream body into events per the WHATWG
// server-sent events parsing rules: an event ends at a blank line; its data
// lines are joined with "\n"; lines starting with ':' are comments; a single
// space after the field colon is dropped; unknown fields are ignored.
//
// Every streaming client used to treat each "data:" line as a complete JSON
// document. That is not the framing rule: one event may carry its payload over
// several data lines, and line-at-a-time parsing would reject (or, where
// malformed lines were skipped, silently drop) a valid event.
//
// A final event not followed by a blank line is still dispatched at EOF. That
// is more lenient than a browser, deliberately: whether the stream is
// complete is decided by the provider's terminal evidence, never by framing.
type sseReader struct {
	scanner *bufio.Scanner
	release func()
}

func newSSEReader(body io.Reader) *sseReader {
	scanner, release := newPooledScanner(body, sseMaxEvent)
	return &sseReader{scanner: scanner, release: release}
}

// Close returns the reader's scratch buffer to the pool.
func (r *sseReader) Close() { r.release() }

// errSSEOversize reports an event larger than sseMaxEvent.
var errSSEOversize = errors.New("server-sent event exceeds 1 MiB")

// Next returns the next event, io.EOF after the last one, or the transport
// error that ended the body.
func (r *sseReader) Next() (sseEvent, error) {
	var (
		data    strings.Builder
		event   string
		hasData bool
	)
	dispatch := func() sseEvent {
		if event == "" {
			event = "message"
		}
		return sseEvent{Event: event, Data: data.String()}
	}
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			if hasData {
				return dispatch(), nil
			}
			event = ""
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "data":
			if hasData {
				data.WriteByte('\n')
			}
			data.WriteString(value)
			hasData = true
			if data.Len() > sseMaxEvent {
				return sseEvent{}, errSSEOversize
			}
		case "event":
			event = value
		}
	}
	if err := r.scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return sseEvent{}, errSSEOversize
		}
		return sseEvent{}, fmt.Errorf("read event stream: %w", err)
	}
	if hasData {
		return dispatch(), nil
	}
	return sseEvent{}, io.EOF
}

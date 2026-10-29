package perception

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func readAllSSE(t *testing.T, body string) ([]sseEvent, error) {
	t.Helper()
	r := newSSEReader(strings.NewReader(body))
	defer r.Close()
	var out []sseEvent
	for {
		ev, err := r.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, ev)
	}
}

func TestSSEReader_FramesPerTheEventStreamRules(t *testing.T) {
	body := ": comment\r\n" +
		"event: delta\r\n" +
		"data: {\"a\":1,\r\n" +
		"data:\"b\":2}\r\n" +
		"id: 7\r\n" +
		"\r\n" +
		"\n" +
		"data: second\n\n" +
		"data: unterminated"
	events, err := readAllSSE(t, body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []sseEvent{
		{Event: "delta", Data: "{\"a\":1,\n\"b\":2}"},
		{Event: "message", Data: "second"},
		{Event: "message", Data: "unterminated"},
	}
	if len(events) != len(want) {
		t.Fatalf("events = %+v, want %+v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event %d = %+v, want %+v", i, events[i], want[i])
		}
	}
}

func TestSSEReader_OversizeEventIsAnError(t *testing.T) {
	line := "data: " + strings.Repeat("x", 600*1024) + "\n"
	_, err := readAllSSE(t, line+line+"\n")
	if !errors.Is(err, errSSEOversize) {
		t.Fatalf("err = %v, want errSSEOversize", err)
	}
}

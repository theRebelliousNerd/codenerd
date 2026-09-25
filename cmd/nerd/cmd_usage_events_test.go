package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"codenerd/internal/usage"
)

// `nerd usage --events N` reads the event ring back from usage.json. The ring
// was written (when enabled) and read by nothing, so an operator could not see
// which calls the aggregates were made of.
func TestUsageEvents_ShouldListTheNewestCallsFromUsageJSON(t *testing.T) {
	ws := t.TempDir()
	writer, err := usage.NewTracker(ws, usage.WithEventLog())
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"model-one", "model-two", "model-three"} {
		writer.Track(context.Background(), model, "acme", 100, 20, "chat")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// A fresh reader, as the command builds one.
	reader, err := usage.NewTracker(ws)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	events := lastUsageEvents(reader.Events(), 2)
	var out bytes.Buffer
	renderUsageEvents(&out, events)
	text := out.String()
	if strings.Contains(text, "model-one") || !strings.Contains(text, "model-two") || !strings.Contains(text, "model-three") {
		t.Fatalf("--events 2 should list the two newest calls:\n%s", text)
	}
	if strings.Index(text, "model-two") > strings.Index(text, "model-three") {
		t.Errorf("events should be listed oldest first:\n%s", text)
	}

	out.Reset()
	renderUsageEvents(&out, nil)
	if !strings.Contains(out.String(), "usage.event_log") {
		t.Errorf("an empty ring should say how to turn the log on: %q", out.String())
	}
}

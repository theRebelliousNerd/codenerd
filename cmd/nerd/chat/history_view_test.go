package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// historyCapture records the history each perception read was given.
type historyCapture struct {
	scriptedTransducer
	seen [][]perception.ConversationTurn
}

func (h *historyCapture) ParseIntentWithContext(ctx context.Context, in string, hist []perception.ConversationTurn) (perception.Intent, error) {
	h.seen = append(h.seen, hist)
	return h.scriptedTransducer.ParseIntentWithContext(ctx, in, hist)
}

// Once compression is active, perception still gets the whole conversation:
// it renders no compressed history, so the host's cut only removed turns,
// and its own selection's notice could not count them.
func TestPerception_GetsTheWholeConversationOnceCompressionIsActive(t *testing.T) {
	t.Parallel()
	m, _ := journeyModel(t, config.WorkModePropose)
	capture := &historyCapture{scriptedTransducer: scriptedTransducer{intents: []perception.Intent{
		{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Option two was the cache."},
	}}}
	m.transducer = capture
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.compressor = ctxcompress.NewCompressor(kernel, nil, m.client)
	if err := m.compressor.LoadState(&ctxcompress.CompressedState{SessionID: "s", RollingSummary: ctxcompress.RollingSummary{Segments: []ctxcompress.HistorySegment{{}}}}); err != nil {
		t.Fatal(err)
	}
	if !m.compressor.IsCompressionActive() {
		t.Fatal("fixture: compression is not active")
	}
	for i := 0; i < 10; i++ {
		m.history = append(m.history, Message{Role: "user", Content: fmt.Sprintf("question %d", i)}, Message{Role: "assistant", Content: fmt.Sprintf("answer %d", i)})
	}
	m, _, _ = say(t, m, "compare that with your earlier second option")
	if len(capture.seen) == 0 {
		t.Fatal("perception was not called")
	}
	if got := len(capture.seen[0]); got != 20 {
		t.Fatalf("perception got %d of 20 prior messages; it selects its own window", got)
	}
}

func TestArticulationHistoryView_AFailedBuildIsStatedNotSilent(t *testing.T) {
	if got := articulationHistoryView("summary of turns 1-40", nil, 40); got != "summary of turns 1-40" {
		t.Fatalf("built view = %q", got)
	}
	got := articulationHistoryView("", errors.New("context budget exceeded"), 34)
	if !strings.Contains(got, "34 earlier message(s)") || !strings.Contains(got, "context budget exceeded") {
		t.Fatalf("failed view = %q", got)
	}
	if got := articulationHistoryView("", errors.New("x"), 0); got != "" {
		t.Fatalf("nothing omitted, yet %q", got)
	}
}

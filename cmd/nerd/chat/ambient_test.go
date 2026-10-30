package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// sessionRecordingTransducer records the session context perception is
// handed with each request.
type sessionRecordingTransducer struct {
	scriptedTransducer
	seen []*types.SessionContext
}

func (s *sessionRecordingTransducer) ParseIntentWithContext(ctx context.Context, in string, h []perception.ConversationTurn) (perception.Intent, error) {
	s.seen = append(s.seen, types.GetSessionContext(ctx))
	return s.scriptedTransducer.ParseIntentWithContext(ctx, in, h)
}

// The editor bridge's .nerd/ambient.json reaches perception with the turn's
// request, beside the brief.
func TestAmbientContext_ReachesPerception(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	rec := &sessionRecordingTransducer{scriptedTransducer: scriptedTransducer{intents: []perception.Intent{{Category: "/query", Verb: "/explain", Target: "the retry loop", Response: "It retries three times."}}}}
	m.transducer = rec
	nerdDir := filepath.Join(m.workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, types.AmbientFile), []byte(`{"active_file":"internal/retry/loop.go","cursor_line":42}`), 0o600); err != nil {
		t.Fatal(err)
	}

	say(t, m, "what does this loop do?")
	if len(rec.seen) == 0 || rec.seen[0] == nil || rec.seen[0].Ambient == nil {
		t.Fatalf("perception saw no ambient context: %+v", rec.seen)
	}
	if amb := rec.seen[0].Ambient; amb.ActiveFile != "internal/retry/loop.go" || amb.CursorLine != 42 {
		t.Fatalf("ambient = %+v", amb)
	}
}

package chat

import (
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

// processInput ran reflection recall on its copy of the model and never
// handed the result back through Update, so the idle /reflection said no
// recall had run although one had just completed (audit
// reflection-observation-publication). The turn's receipt now travels on
// its completion message.
func TestReflection_ATurnsRecallIsVisibleToSlashReflection(t *testing.T) {
	t.Parallel()
	ask := perception.Intent{
		Category: "/query", Verb: "/explain", Target: "the retry policy", Confidence: 0.95,
	}
	m, _ := journeyModel(t, config.WorkModePropose, ask)
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m.localDB = db
	if got := m.renderReflectionStatus(); !strings.Contains(got, "No reflection recall has run") {
		t.Fatalf("precondition: a fresh session reports a recall: %q", got)
	}

	m, msg, _ := say(t, m, "explain how the retry policy decides when to give up")
	status := m.renderReflectionStatus()
	if strings.Contains(status, "No reflection recall has run") || !strings.Contains(status, "Query:") {
		t.Fatalf("after a turn that ran recall (%T), /reflection says: %q", msg, status)
	}
}

// A turn that hands its work to the background publishes its recall on the
// handoff, so the job's session context is built with the request's own hits;
// a turn that ran no recall leaves the previous receipt alone.
func TestPublishReflection_TravelsOnTheHandoffAndSparesSkippedTurns(t *testing.T) {
	t.Parallel()
	ran := &ReflectionState{Query: "retry policy", ContextHits: []string{"hit"}}
	if got := publishReflection(backgroundStartMsg{Surface: "on it"}, ran).(backgroundStartMsg); got.Reflection != ran {
		t.Fatalf("handoff reflection = %v, want the turn's", got.Reflection)
	}
	if got := publishReflection(assistantMsg{Surface: "hi"}, nil).(assistantMsg); got.Reflection != nil {
		t.Fatalf("a turn that ran no recall published %v", got.Reflection)
	}
	m := NewTestModel()
	m.lastReflection = ran
	next, _ := m.Update(assistantMsg{Surface: "short answer"})
	if next.(Model).lastReflection != ran {
		t.Fatal("a turn without recall erased the previous receipt")
	}
}

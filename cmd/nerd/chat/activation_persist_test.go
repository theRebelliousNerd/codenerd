package chat

import (
	"path/filepath"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// Nothing wrote the activation log that boot hydrates as activation facts, so
// a resumed session started with no focus. Saving the session now logs the
// kernel's activation of every fact in working memory (context_atom), and
// the log hands it back on the scale hydration reads.
func TestSaveSession_PersistsTheKernelsFocus(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m.localDB = db
	m.sessionID = "sess-focus"
	if err := m.kernel.Assert(core.Fact{Predicate: "user_intent", Args: []any{
		types.MangleAtom("/current_intent"), types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "internal/retry/policy.go", ""}}); err != nil {
		t.Fatal(err)
	}

	if err := m.saveSessionState(); err != nil {
		t.Fatalf("saveSessionState: %v", err)
	}
	got, err := db.GetRecentActivations(10, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	if score, ok := got["internal/retry/policy.go"]; !ok || types.PercentFromRatio(score) != 90 {
		t.Fatalf("recent activations = %v; want the intent's target at the kernel's 90", got)
	}
}

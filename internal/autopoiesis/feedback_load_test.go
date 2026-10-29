package autopoiesis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LearningStore.load unmarshalled tool_learnings.json with the error dropped.
// A truncated or half-written file loaded as an empty store — every tool's
// success rate, known issues and anti-patterns gone, with no error, no log and
// no way to tell it from a first run. The next save then overwrote the damaged
// file with that empty map, making the loss permanent.

func TestLearningStore_Load_RoundTripsGoodFile(t *testing.T) {
	dir := t.TempDir()
	store := NewLearningStore(dir)
	store.RecordLearning("mangle_diff", &ExecutionFeedback{ToolName: "mangle_diff", Success: true}, nil)

	reloaded := NewLearningStore(dir)
	if reloaded.GetLearning("mangle_diff") == nil {
		t.Fatal("a saved learning did not survive a reload")
	}
}

func TestLearningStore_Load_CorruptFileIsPreservedNotSilentlyDiscarded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool_learnings.json")
	if err := os.WriteFile(path, []byte(`{"mangle_diff": {"tool_name": "mangle_d`), 0o644); err != nil {
		t.Fatalf("write corrupt store: %v", err)
	}

	store := NewLearningStore(dir)
	if store.GetLearning("mangle_diff") != nil {
		t.Error("a corrupt file must not half-load")
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("the corrupt file is still in place; the next save will overwrite it")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tool_learnings.json.corrupt-") {
			found = true
		}
	}
	if !found {
		t.Errorf("the corrupt learning store was discarded rather than preserved: %v", entries)
	}
}

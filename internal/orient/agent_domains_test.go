package orient

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// writeProfileSnapshot saves a snapshot holding only profile signals, the
// shape init leaves behind (snapshot.json carries profile_signal rows).
func writeProfileSnapshot(t *testing.T, root string, signals [][2]string) {
	t.Helper()
	var facts []types.Fact
	for _, s := range signals {
		facts = append(facts, types.Fact{Predicate: "profile_signal", Args: []any{types.MangleAtom(s[0]), s[1]}})
	}
	body, err := json.Marshal(Snapshot{Config: config.DefaultOrientConfig(), Facts: facts})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(snapshotPath(root)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(snapshotPath(root), body, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

// The catalog domain of each stack agent comes back from the saved snapshot:
// a Go project with Bubble Tea and Mangle dependencies gives GoExpert "go",
// BubbleTeaExpert "bubbletea" and MangleExpert "mangle"; a dependency the
// catalog has no agent for gives nothing.
func TestAgentDomainsFromTheSavedSnapshot(t *testing.T) {
	root := t.TempDir()
	writeProfileSnapshot(t, root, [][2]string{
		{"/language", "go"},
		{"/dependency", "bubbletea"},
		{"/dependency", "mangle"},
		{"/dependency", "uuid"},
	})
	got, err := AgentDomains(context.Background(), root)
	if err != nil {
		t.Fatalf("AgentDomains: %v", err)
	}
	for name, want := range map[string][]string{
		"GoExpert":        {"go"},
		"BubbleTeaExpert": {"bubbletea"},
		"MangleExpert":    {"mangle"},
	} {
		if !slices.Equal(got[name], want) {
			t.Errorf("%s domains = %v, want %v (all: %v)", name, got[name], want, got)
		}
	}
}

func TestAgentDomainsWithoutASnapshotSaysSo(t *testing.T) {
	if _, err := AgentDomains(context.Background(), t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("AgentDomains on a workspace with no snapshot = %v, want fs.ErrNotExist", err)
	}
}

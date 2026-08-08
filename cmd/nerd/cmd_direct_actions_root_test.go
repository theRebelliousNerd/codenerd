package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestFindNewRootEntries_ReportsNewEntries(t *testing.T) {
	before := map[string]bool{"go.mod": true, "README.md": true}
	after := map[string]bool{"go.mod": true, "README.md": true, "gate_cover.out": true, "research": true}
	got := findNewRootEntries(before, after)
	want := []string{"gate_cover.out", "research"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findNewRootEntries = %v, want %v", got, want)
	}
}

func TestFindNewRootEntries_ReportsNewDirectory(t *testing.T) {
	before := map[string]bool{"go.mod": true}
	after := map[string]bool{"go.mod": true, "research": true}
	got := findNewRootEntries(before, after)
	if len(got) != 1 || got[0] != "research" {
		t.Fatalf("expected directory 'research' reported, got %v", got)
	}
}

func TestFindNewRootEntries_NothingWhenMatch(t *testing.T) {
	before := map[string]bool{"go.mod": true, "README.md": true}
	after := map[string]bool{"go.mod": true, "README.md": true}
	if got := findNewRootEntries(before, after); len(got) != 0 {
		t.Fatalf("expected no new entries, got %v", got)
	}
}

func TestFindNewRootEntries_NilBaselineSilent(t *testing.T) {
	after := map[string]bool{"gate_cover.out": true}
	if got := findNewRootEntries(nil, after); got != nil {
		t.Fatalf("nil before should yield nil, got %v", got)
	}
	if got := findNewRootEntries(map[string]bool{"a": true}, nil); got != nil {
		t.Fatalf("nil after should yield nil, got %v", got)
	}
}

func TestSnapshotDirectRoot_IncludesDirectoriesAndExcludesDotNerd(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "research"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	snap := snapshotDirectRoot(ws)
	if snap == nil {
		t.Fatal("snapshotDirectRoot returned nil for valid workspace")
	}
	if !snap["go.mod"] {
		t.Error("root file go.mod not captured")
	}
	if !snap["research"] {
		t.Error("directory 'research' not captured; snapshot must include directories")
	}
	if snap[".nerd"] {
		t.Error(".nerd should be excluded from snapshot")
	}
}

func TestSnapshotDirectRoot_NilOnBadWorkspace(t *testing.T) {
	if got := snapshotDirectRoot(""); got != nil {
		t.Errorf("empty workspace should yield nil, got %v", got)
	}
	if got := snapshotDirectRoot("   "); got != nil {
		t.Errorf("whitespace workspace should yield nil, got %v", got)
	}
	if got := snapshotDirectRoot(filepath.Join(t.TempDir(), "nope")); got != nil {
		t.Errorf("missing workspace should yield nil, got %v", got)
	}
}

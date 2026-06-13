package campaign

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasPrefixAndMatchers(t *testing.T) {
	if hasPrefix("internal/foo/bar.go", "internal/foo") != true {
		t.Error("internal/foo should prefix internal/foo/bar.go")
	}
	if hasPrefix("internal/foobar.go", "internal/foo") {
		t.Error("internal/foo must not prefix internal/foobar.go (segment boundary)")
	}
	if hasPrefix("anything", "") {
		t.Error("empty prefix never matches")
	}

	// matchesInclude: empty include = match everything.
	if !matchesInclude("a/b.go", nil) {
		t.Error("empty include list should match")
	}
	if !matchesInclude("internal/x.go", []string{"cmd", "internal"}) {
		t.Error("path under internal should be included")
	}
	if matchesInclude("vendor/x.go", []string{"cmd", "internal"}) {
		t.Error("vendor path should not be included")
	}

	// matchesExclude: empty exclude = exclude nothing.
	if matchesExclude("a/b.go", nil) {
		t.Error("empty exclude list should exclude nothing")
	}
	if !matchesExclude("internal/gen/x.go", []string{"internal/gen"}) {
		t.Error("path under internal/gen should be excluded")
	}
}

func TestUniqueStrings(t *testing.T) {
	got := uniqueStrings([]string{"a", " a ", "b", "", "  ", "b", "c"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("uniqueStrings=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("uniqueStrings[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestShortHashAndResultKey(t *testing.T) {
	h := shortHash("hello")
	if len(h) != 10 {
		t.Errorf("shortHash len=%d, want 10", len(h))
	}
	if shortHash("hello") != h {
		t.Error("shortHash should be deterministic")
	}
	if shortHash("world") == h {
		t.Error("shortHash should differ for different input")
	}

	key := assaultResultKey(2, AssaultStageGoTest, 1, "internal/foo")
	if key != "2|/go_test|1|internal/foo" {
		t.Errorf("assaultResultKey=%q, unexpected format", key)
	}
}

func TestGroupModuleAndSubsystem(t *testing.T) {
	if groupModule("internal/foo/bar") != "internal" {
		t.Error("groupModule should return the first path segment")
	}
	if groupModule("") != "" || groupModule(".") != "." {
		t.Error("groupModule edge cases (empty/dot) mishandled")
	}

	// Subsystem keeps two segments for internal/cmd/pkg roots.
	if groupSubsystem("internal/foo/bar") != "internal/foo" {
		t.Error("groupSubsystem should keep internal/<sub>")
	}
	if groupSubsystem("cmd/nerd/main.go") != "cmd/nerd" {
		t.Error("groupSubsystem should keep cmd/<sub>")
	}
	// Non-special roots collapse to the first segment.
	if groupSubsystem("docs/guide/x") != "docs" {
		t.Error("groupSubsystem should collapse non-internal roots to first segment")
	}
}

func TestGetCampaignPhaseAndShardForRole(t *testing.T) {
	if GetCampaignPhaseForRole(RoleAssault) != "/assault" {
		t.Error("assault role -> /assault phase")
	}
	if GetCampaignPhaseForRole(RoleLibrarian) != "/doc_classification" {
		t.Error("librarian role -> /doc_classification phase")
	}
	if GetCampaignPhaseForRole(CampaignRole("nope")) != "/active" {
		t.Error("unknown role -> /active default")
	}

	if GetShardTypeForRole(RoleAssault) != "analyzer" {
		t.Error("assault role uses analyzer shard")
	}
	if GetShardTypeForRole(RolePlanner) != "planner" {
		t.Error("planner role uses planner shard")
	}
}

func TestCheckpointRunner_DetectCommands(t *testing.T) {
	// A Go project (go.mod present) yields go commands.
	goDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cr := &CheckpointRunner{workspace: goDir}
	if got := cr.detectTestCommand(); got != "go test ./..." {
		t.Errorf("detectTestCommand(go)=%q, want 'go test ./...'", got)
	}
	if got := cr.detectBuildCommand(); got != "go build ./..." {
		t.Errorf("detectBuildCommand(go)=%q, want 'go build ./...'", got)
	}

	// A Node project (package.json) yields npm commands.
	nodeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nodeDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	crNode := &CheckpointRunner{workspace: nodeDir}
	if got := crNode.detectTestCommand(); got != "npm test" {
		t.Errorf("detectTestCommand(node)=%q, want 'npm test'", got)
	}

	// An empty workspace falls back to the go defaults.
	crEmpty := &CheckpointRunner{workspace: t.TempDir()}
	if got := crEmpty.detectBuildCommand(); got != "go build ./..." {
		t.Errorf("detectBuildCommand(empty)=%q, want go default", got)
	}
}

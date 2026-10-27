package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

func runChatFileRead(t *testing.T, root, target string) ([]core.Fact, []string) {
	t.Helper()
	m := NewTestModel(WithWorkspace(root))
	warnings := []string{}
	results, _ := m.executeInfoGatheringActions(
		context.Background(),
		"read the requested file",
		perception.Intent{Target: target},
		[]core.Fact{{Predicate: "action", Args: []any{"/fs_read"}}},
		&warnings,
	)
	return results, warnings
}

func TestChatInfoGatheringRead_EnforcesWorkspaceContainment(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "chat-read-outside.txt")
	if err := os.WriteFile(outside, []byte("outside secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	for _, tc := range []struct {
		name   string
		target string
	}{
		{name: "outside absolute path", target: outside},
		{name: "parent traversal", target: filepath.Join("..", filepath.Base(outside))},
		{name: "symlink escape", target: "escape.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results, warnings := runChatFileRead(t, root, tc.target)
			if len(results) != 0 {
				t.Fatalf("outside read returned facts: %#v", results)
			}
			for _, result := range results {
				if result.Predicate == "file_content" {
					t.Fatalf("denied read asserted file_content: %#v", result)
				}
			}
			if len(warnings) == 0 || !strings.Contains(warnings[0], "Failed to read file") {
				t.Fatalf("denied read was not surfaced to the turn: %v", warnings)
			}
		})
	}
}

func TestChatInfoGatheringRead_AllowsCanonicalInRootTargets(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside.txt")
	if err := os.WriteFile(inside, []byte("inside source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, filepath.Join(root, "inside-link.txt")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	for _, target := range []string{"inside.txt", inside, "inside-link.txt"} {
		t.Run(target, func(t *testing.T) {
			results, warnings := runChatFileRead(t, root, target)
			if len(warnings) != 1 || !strings.Contains(warnings[0], "Read file:") {
				t.Fatalf("in-root read did not complete: %v", warnings)
			}
			if len(results) != 1 || results[0].Predicate != "file_content" {
				t.Fatalf("in-root read results = %#v, want one file_content fact", results)
			}
			if got := results[0].Args[1]; got != "inside source" {
				t.Fatalf("file_content = %q, want %q", got, "inside source")
			}
		})
	}
}

func TestPerceivedForgetOnlyRemovesMatchingSessionNote(t *testing.T) {
	for _, key := range []string{"permitted", "user_intent", "security_violation"} {
		t.Run(key, func(t *testing.T) {
			kernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatalf("NewRealKernel: %v", err)
			}
			facts := []core.Fact{
				{Predicate: "permitted", Args: []any{types.MangleAtom("/write_file"), "src/secret.txt", "approved"}},
				{Predicate: "user_intent", Args: []any{types.MangleAtom("/current_intent"), types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "src/main.go", ""}},
				{Predicate: "security_violation", Args: []any{types.MangleAtom("/write_file"), "blocked by policy", int64(7)}},
				{Predicate: "session_note", Args: []any{key, "outdated note"}},
				{Predicate: "session_note", Args: []any{"unrelated", "keep this note"}},
			}
			for _, fact := range facts {
				if err := kernel.Assert(fact); err != nil {
					t.Fatalf("assert %s: %v", fact.Predicate, err)
				}
			}

			m := NewTestModel()
			m.kernel = kernel
			warnings := []string{}
			m.processMemoryOperations(perception.Intent{
				MemoryOperations: []articulation.MemoryOperation{{Op: "forget", Key: key}},
			}, &warnings)

			for _, predicate := range []string{"permitted", "user_intent", "security_violation"} {
				rows, err := kernel.Query(predicate)
				if err != nil {
					t.Fatalf("query %s: %v", predicate, err)
				}
				if len(rows) != 1 {
					t.Errorf("forget key %q removed %s executive fact(s); want all three executive witnesses preserved", key, predicate)
				}
			}

			notes, err := kernel.Query("session_note")
			if err != nil {
				t.Fatalf("query session_note: %v", err)
			}
			if len(notes) != 1 || types.ExtractString(notes[0].Args[0]) != "unrelated" {
				t.Fatalf("forget key %q left wrong session notes: %#v", key, notes)
			}
		})
	}
}

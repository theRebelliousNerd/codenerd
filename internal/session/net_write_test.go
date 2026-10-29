package session

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

// owedGates are the gates the kernel derives this turn owes.
func owedGates(t *testing.T, e *Executor, turn types.MangleAtom) map[string]bool {
	t.Helper()
	rows, err := e.kernel.Query("turn_owes_gate")
	if err != nil {
		t.Fatalf("query turn_owes_gate: %v", err)
	}
	owed := map[string]bool{}
	for _, row := range rows {
		if len(row.Args) == 2 && types.ExtractString(row.Args[0]) == string(turn) {
			owed[types.ExtractString(row.Args[1])] = true
		}
	}
	return owed
}

// What a write owes is owed for the change it leaves. A policy file the turn
// edited and then put back byte for byte is net unchanged and owes no test
// run; edited again, it owes one. Ladder R1-24 charged /test_run for such a
// reverted .mg edit.
func TestAssertTurnWrites_ARevertedWriteOwesNothing(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	const rel = "policy/rule.mg"
	const original = "rule_fact(/a).\n"
	path := filepath.Join(e.config.WorkspaceRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	result := mutationResult()
	result.WrittenPaths = []string{rel}
	result.PreWriteContents = map[string]PreImage{rel: existed(original)}
	turn := result.turnAtom()

	e.assertTurnWrites(turn, result)
	if owed := owedGates(t, e, turn); owed["/test_run"] {
		t.Fatalf("a write reverted byte for byte owes %v; it is net unchanged", owed)
	}

	if err := os.WriteFile(path, []byte("rule_fact(/b).\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.assertTurnWrites(turn, result)
	if owed := owedGates(t, e, turn); !owed["/test_run"] {
		t.Fatalf("an edited policy file owes %v, want /test_run", owed)
	}

	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	e.assertTurnWrites(turn, result)
	if owed := owedGates(t, e, turn); owed["/test_run"] {
		t.Fatalf("reverted again, the write still owes %v", owed)
	}
}

// A file the turn created and then removed is net unchanged too.
func TestWriteReverted_ACreatedThenRemovedFileIsReverted(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	pre := map[string]PreImage{"new.txt": {}}
	if !writeReverted(ws, "new.txt", pre) {
		t.Fatal("a created file that is gone again is not reverted")
	}
	if err := os.WriteFile(filepath.Join(ws, "new.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if writeReverted(ws, "new.txt", pre) {
		t.Fatal("a created file that still exists is reverted")
	}
	if writeReverted(ws, "unrecorded.txt", pre) {
		t.Fatal("a path with no recorded pre-image is reverted")
	}
}

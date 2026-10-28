package session

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/observation"
)

// Ladder C4: a caller that fails a turn undoes the turn's own writes, so the
// return carries each one with what it held before the turn and what the
// turn left.
func TestWithTurnWrites_RecordsEachWriteBeforeAndAfter(t *testing.T) {
	ws := t.TempDir()
	for name, content := range map[string]string{"modified.go": "after", "created.go": "new"} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res := &ExecutionResult{
		WrittenPaths: []string{"modified.go", "created.go", "deleted.go", "unread.go"},
		PreWriteContents: map[string]PreImage{
			"modified.go": {Existed: true, Content: "before"},
			"created.go":  {},
			"deleted.go":  {Existed: true, Content: "was here"},
			"unread.go":   {Unknown: "permission denied"},
		},
	}

	got := withTurnWrites(observation.Return{}, ws, res).Writes

	want := []observation.FileWrite{
		{Path: filepath.Join(ws, "modified.go"), Before: observation.FileState{Known: true, Exists: true, Content: "before"}, After: observation.FileState{Known: true, Exists: true, Content: "after"}},
		{Path: filepath.Join(ws, "created.go"), Before: observation.FileState{Known: true}, After: observation.FileState{Known: true, Exists: true, Content: "new"}},
		{Path: filepath.Join(ws, "deleted.go"), Before: observation.FileState{Known: true, Exists: true, Content: "was here"}, After: observation.FileState{Known: true}},
		{Path: filepath.Join(ws, "unread.go"), Before: observation.FileState{}, After: observation.FileState{Known: true}},
	}
	if len(got) != len(want) {
		t.Fatalf("Writes = %+v, want %d entries", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Writes[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The delegation judge measures the turn's workspace again after it judges,
// so the return names the tree its snapshot and writes describe -- also for a
// turn that produced no result.
func TestWithTurnWrites_BindsTheReturnToItsWorkspace(t *testing.T) {
	ws := t.TempDir()
	if got := withTurnWrites(observation.Return{}, ws, &ExecutionResult{}).Workspace; got != ws {
		t.Errorf("Workspace = %q, want %q", got, ws)
	}
	if got := withTurnWrites(observation.Return{}, ws, nil).Workspace; got != ws {
		t.Errorf("Workspace with no result = %q, want %q", got, ws)
	}
}

package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// A turn that runs under a write journal records what it wrote when it ends,
// on the error path too: a fix turn whose repair gave up with its tests red
// is exactly the span its caller undoes. The journal names the failing test,
// and undoing puts the turn's own files back -- a file the turn created is
// removed, a file it changed gets its bytes back -- while a file another
// writer changed after the turn is left as found.
func TestWriteJournal_RecordsAFailedTurnAndUndoesOnlyItsOwnWrites(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 1 })
	mainPath := filepath.Join(h.ws, "main.go")
	testPath := filepath.Join(h.ws, "main_test.go")
	notesPath := filepath.Join(h.ws, "notes.go")
	const notesBefore = "package main\n\n// notes before the turn\n"
	if err := os.WriteFile(notesPath, []byte(notesBefore), 0o644); err != nil {
		t.Fatal(err)
	}
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", mainPath, repairMainGo),
			h.writeCall("c2", "write_file", testPath, repairTestBroken),
			h.writeCall("c3", "write_file", notesPath, "package main\n\n// notes the turn wrote\n"),
		}}
	}
	h.onRepair = func(int, []types.Message) *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "trying", ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", testPath, strings.Replace(repairTestBroken, "broken", "still broken", 1)),
		}}
	}
	journal := NewWriteJournal()
	ctx, cancel := context.WithTimeout(WithWriteJournal(context.Background(), journal), 3*time.Minute)
	defer cancel()
	_, err := h.executor.ProcessWithIntent(ctx, "fix the probe test", presetIntentForTask("/fix", "fix the probe test", ""))
	if err == nil || !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("ProcessWithIntent error = %v, want the repair to give up", err)
	}

	if got := strings.Join(journal.Paths(), ","); got != "main.go,main_test.go,notes.go" {
		t.Fatalf("journal paths = %q, want the turn's three writes", got)
	}
	failures := journal.TestFailures()
	if len(failures) == 0 || !strings.Contains(failures[0], "TestProbe") || !strings.Contains(failures[0], "still broken") {
		t.Fatalf("journal failures = %q, want TestProbe with its last message", failures)
	}

	// Another writer edits main.go after the turn: that file is theirs now.
	const foreign = "package main\n\n// another session's edit\nfunc main() {}\n"
	if err := os.WriteFile(mainPath, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	undo := journal.Undo()
	if undo.Err != nil {
		t.Fatalf("undo: %v", undo.Err)
	}
	if !strings.Contains(undo.Patch, "main_test.go") || !strings.Contains(undo.Patch, "still broken") {
		t.Fatalf("the patch does not hold the attempt:\n%s", undo.Patch)
	}
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Fatalf("the test the turn created is still there: %v", err)
	}
	if data, _ := os.ReadFile(notesPath); string(data) != notesBefore {
		t.Fatalf("notes.go = %q, want it as the turn found it", data)
	}
	if data, _ := os.ReadFile(mainPath); string(data) != foreign {
		t.Fatalf("main.go = %q: the other writer's edit was reverted", data)
	}
	if len(undo.Left) != 1 || !strings.HasPrefix(undo.Left[0], "main.go (changed since") {
		t.Fatalf("left = %q, want main.go named as changed since", undo.Left)
	}
	if strings.Join(undo.Restored, ",") != "main_test.go,notes.go" {
		t.Fatalf("restored = %q", undo.Restored)
	}
	if again := journal.Undo(); len(again.Restored)+len(again.Left) != 0 {
		t.Fatalf("a second undo did something: %+v", again)
	}
}

// Several turns under one journal (a verified delegation's attempts) merge:
// the first record of a path keeps what it held before the span, the last
// keeps what the span last wrote. A preimage the turn did not record is never
// guessed at.
func TestWriteJournal_MergesTurnsFirstPreimageLastWrite(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := filepath.Join(ws, "a.go")
	hash := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	j := NewWriteJournal()
	j.Record(ws, []string{"a.go"}, map[string]PreImage{"a.go": {Existed: true, Content: "v0", LastWriteHash: hash("v1")}})
	j.Record(ws, []string{"a.go", "b.go"}, map[string]PreImage{"a.go": {Existed: true, Content: "v1", LastWriteHash: hash("v2")}})
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	undo := j.Undo()
	if data, _ := os.ReadFile(path); string(data) != "v0" {
		t.Fatalf("a.go = %q, want what it held before the first turn", data)
	}
	if len(undo.Left) != 1 || !strings.HasPrefix(undo.Left[0], "b.go (what it held before is unknown") {
		t.Fatalf("left = %q, want b.go named as unknown", undo.Left)
	}
}

package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// AG7 finding 7: the restore wrote the first-touch snapshot back
// unconditionally, so an edit another agent made after the snapshot was
// silently destroyed. The turn now records what it itself last wrote, and
// the restore leaves a file another agent touched as found.

// turnWrite is the write path in miniature: the pre-write snapshot, the
// write, and the last-write record -- the same three steps
// executeAndRecordToolCall takes around a successful write mutation.
func turnWrite(t *testing.T, ws string, result *ExecutionResult, rel, content string) {
	t.Helper()
	args := map[string]any{"path": rel}
	snapshotPreWriteContents(result, args, ws)
	if err := recordWrittenPaths(result, args, ws); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	recordLastWrittenContents(result, args, ws)
}

func TestRestore_LeavesAnotherAgentsEditAsFound(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "a.go")
	if err := os.WriteFile(path, []byte("package a // v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{}
	turnWrite(t, ws, result, "a.go", "package a // v1 the turn\n")
	// Another agent edits after the turn's last write.
	if err := os.WriteFile(path, []byte("package a // v2 other agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, conflicts, err := turnFiles{pre: map[string]PreImage{}}.restore(ws, result)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %v, want nothing: the file was left as found", changed)
	}
	if len(conflicts) != 1 || conflicts[0] != "a.go" {
		t.Fatalf("conflicts = %v, want [a.go]", conflicts)
	}
	if data, _ := os.ReadFile(path); string(data) != "package a // v2 other agent\n" {
		t.Fatalf("the other agent's edit was destroyed: %q", data)
	}
	if !slices.Contains(result.WrittenPaths, "a.go") {
		t.Fatalf("WrittenPaths = %v, want the conflicted write kept", result.WrittenPaths)
	}
}

func TestRestore_RestoresWhenOnlyTheTurnWrote(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "a.go")
	if err := os.WriteFile(path, []byte("package a // v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{}
	turnWrite(t, ws, result, "a.go", "package a // v1 the turn\n")

	changed, conflicts, err := turnFiles{pre: map[string]PreImage{}}.restore(ws, result)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %v, want none: only the turn wrote here", conflicts)
	}
	if len(changed) != 1 || changed[0] != "a.go" {
		t.Fatalf("changed = %v, want [a.go]", changed)
	}
	if data, _ := os.ReadFile(path); string(data) != "package a // v0\n" {
		t.Fatalf("the turn's edit was not put back: %q", data)
	}
}

// A file the turn created and someone else then changed follows the same
// rule: it is left as found, not deleted.
func TestRestore_CreatedFileChangedByAnotherIsLeft(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "new.go")
	result := &ExecutionResult{}
	turnWrite(t, ws, result, "new.go", "package new // v1 the turn\n")
	if err := os.WriteFile(path, []byte("package new // v2 other agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, conflicts, err := turnFiles{pre: map[string]PreImage{}}.restore(ws, result)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %v, want nothing", changed)
	}
	if len(conflicts) != 1 || conflicts[0] != "new.go" {
		t.Fatalf("conflicts = %v, want [new.go]", conflicts)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != "package new // v2 other agent\n" {
		t.Fatalf("the created file was removed or reverted: %q (%v)", data, readErr)
	}
}

func TestRestore_CreatedFileUntouchedIsRemoved(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "new.go")
	result := &ExecutionResult{}
	turnWrite(t, ws, result, "new.go", "package new // v1 the turn\n")

	changed, conflicts, err := turnFiles{pre: map[string]PreImage{}}.restore(ws, result)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %v, want none", conflicts)
	}
	if len(changed) != 1 {
		t.Fatalf("changed = %v, want the removal", changed)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("the turn's created file is still there: %v", statErr)
	}
}

// A file the turn deleted and someone else recreated is left as found: the
// recorded removal matches absence, not the stranger's bytes.
func TestRestore_DeletedFileRecreatedByAnotherIsLeft(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "gone.go")
	if err := os.WriteFile(path, []byte("package gone // v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{}
	args := map[string]any{"path": "gone.go"}
	snapshotPreWriteContents(result, args, ws)
	if err := recordWrittenPaths(result, args, ws); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	recordLastWrittenContents(result, args, ws)
	if err := os.WriteFile(path, []byte("package gone // recreated by another\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, conflicts, err := turnFiles{pre: map[string]PreImage{}}.restore(ws, result)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %v, want nothing", changed)
	}
	if len(conflicts) != 1 || conflicts[0] != "gone.go" {
		t.Fatalf("conflicts = %v, want [gone.go]", conflicts)
	}
	if data, _ := os.ReadFile(path); string(data) != "package gone // recreated by another\n" {
		t.Fatalf("the recreated file was destroyed: %q", data)
	}
}

// The give-up path names the conflict in its own sentence: the report must
// never claim a restore it did not do.
func TestLeaveBuildableTree_NamesConflictsLeftAsFound(t *testing.T) {
	e, ws := newBuildableTreeWorkspace(t)
	result := &ExecutionResult{}
	turnWrite(t, ws, result, "a.go", "package buildable\n\nfunc A() int { return missing() }\n")
	turnWrite(t, ws, result, "b.go", "package buildable\n\nvar B = missing()\n")
	// Another agent fixes a.go their own way after the turn broke it; the
	// turn's own b.go still breaks the build, so the give-up path runs.
	other := "package buildable\n\nfunc A() int { return 3 }\n"
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}

	note := e.leaveBuildableTree(context.Background(), result)
	if !strings.Contains(note, "a.go") || !strings.Contains(note, "left as found") {
		t.Fatalf("note = %q, want it to name a.go as left as found", note)
	}
	if data, _ := os.ReadFile(filepath.Join(ws, "a.go")); string(data) != other {
		t.Fatalf("the other agent's fix was destroyed: %q", data)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "b.go")); !os.IsNotExist(statErr) {
		t.Fatalf("the turn's own untouched file was not restored: %v", statErr)
	}
	if !slices.Contains(result.WrittenPaths, "a.go") {
		t.Fatalf("WrittenPaths = %v, want the conflicted write kept", result.WrittenPaths)
	}
}

// A restore writes the snapshot, and that write is the turn's own. The next
// restore has to see those bytes as the turn's, or it reads its own undo as
// another agent's edit and leaves the file where the first restore put it
// (AG7 finding 7, the second restore). The two wants differ on purpose: a
// second restore of the same bytes writes nothing, so it would not show the bug.
func TestRestore_ALaterRestoreSeesTheHarnessWrite(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "a.go")
	v0 := "package a // v0\n"
	v1 := "package a // v1 episode\n"
	v2 := "package a // v2 the turn\n"
	if err := os.WriteFile(path, []byte(v2), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(v2))
	result := &ExecutionResult{
		WrittenPaths: []string{"a.go"},
		PreWriteContents: map[string]PreImage{
			"a.go": {Existed: true, Content: v0, LastWriteHash: hex.EncodeToString(sum[:])},
		},
	}
	first := turnFiles{
		written: []string{"a.go"},
		pre:     map[string]PreImage{"a.go": {Existed: true, Content: v1}},
	}
	changed, conflicts, err := first.restore(ws, result)
	if err != nil {
		t.Fatalf("first restore: %v", err)
	}
	if len(conflicts) != 0 || len(changed) != 1 {
		t.Fatalf("first restore changed=%v conflicts=%v, want [a.go] and no conflict", changed, conflicts)
	}
	if data, _ := os.ReadFile(path); string(data) != v1 {
		t.Fatalf("first restore left %q, want the episode snapshot", data)
	}
	wantHash := sha256.Sum256([]byte(v1))
	if got := result.PreWriteContents["a.go"].LastWriteHash; got != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("hash after the harness write = %q, want the restored bytes", got)
	}

	second := turnFiles{written: []string{"a.go"}, pre: map[string]PreImage{}}
	changed, conflicts, err = second.restore(ws, result)
	if err != nil {
		t.Fatalf("second restore: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("second restore conflicts = %v, want none: the first restore was the turn's own write", conflicts)
	}
	if data, _ := os.ReadFile(path); string(data) != v0 {
		t.Fatalf("second restore left %q, want the pre-turn bytes", data)
	}
}

// The conflict sentence has to survive on the result. A forcing round's error
// is settled into the verdict, and the check's output is the text that remains.
// The green verdict stays unrestored: the red gate is what the verdict sees.
func TestUndoRedRound_ConflictIsNamedAndTheRedGateStays(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "a.go")
	if err := os.WriteFile(path, []byte("package a // v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{TestCheck: TestVerification{Outcome: VerifyFailed, Output: "tests failed"}}
	turnWrite(t, ws, result, "a.go", "package a // v1 the turn\n")
	other := "package a // v2 other agent\n"
	if err := os.WriteFile(path, []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}

	undone := undoRedRound("tests", ws, result, turnFiles{pre: map[string]PreImage{}}, TestVerification{Outcome: VerifyPassed, OK: true}, ErrVerificationFailed)
	if undone {
		t.Fatal("a conflicted round was reported undone")
	}
	if result.TestCheck.Verdict() != VerifyFailed {
		t.Fatalf("verdict = %s, want the red gate to stay", result.TestCheck.Verdict())
	}
	if !strings.Contains(result.TestCheck.Output, "a.go") || !strings.Contains(result.TestCheck.Output, "left as found") {
		t.Fatalf("output = %q, want the conflict named", result.TestCheck.Output)
	}
	if data, _ := os.ReadFile(path); string(data) != other {
		t.Fatalf("the other agent's edit was destroyed: %q", data)
	}
}

package session

import (
	"os"
	"path/filepath"
	"testing"
)

// existed is the preimage of a file that held content before the turn.
func existed(content string) PreImage { return PreImage{Existed: true, Content: content} }

// Absent, empty and unreadable are three different preimages. When all three
// were recorded as "", undoing a round deleted a pre-existing empty file and
// would have deleted one it merely failed to read.
func TestReadPreImage_AbsentEmptyAndUnreadableAreThreeThings(t *testing.T) {
	ws := t.TempDir()
	empty := filepath.Join(ws, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(ws, "a-directory")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := readPreImage(filepath.Join(ws, "missing.txt")); got != (PreImage{}) {
		t.Errorf("absent file: %+v, want the absent preimage", got)
	}
	if got := readPreImage(empty); got != existed("") {
		t.Errorf("empty file: %+v, want an existing empty file", got)
	}
	got := readPreImage(dir) // exists, cannot be read as a file
	if got.Known() {
		t.Errorf("unreadable path: %+v, want an unknown preimage, not an absent or empty one", got)
	}
}

// External audit F7 (2026-09-19), reproduced: a round writes a file that
// already existed empty, for the first time in the turn, then gives up and is
// undone. The file must come back empty, not be deleted.
func TestRestore_APreExistingEmptyFileIsRestoredNotDeleted(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "empty.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{}
	snap := snapshotTurnFiles(ws, result) // the turn had written nothing yet

	snapshotPreWriteContents(result, map[string]any{"path": "empty.txt"}, ws)
	result.WrittenPaths = append(result.WrittenPaths, "empty.txt")
	if err := os.WriteFile(path, []byte("the round's content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := snap.restore(ws, result); err != nil {
		t.Fatalf("restore: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the pre-existing empty file is gone after the restore: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("restored content = %q, want the empty file it was", data)
	}
}

// A file whose earlier state is unknown is not guessed at: the restore refuses
// before touching anything.
func TestRestore_RefusesAnUnknownPreimage(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "x.go")
	if err := os.WriteFile(path, []byte("package x // the round's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{
		WrittenPaths:     []string{"x.go"},
		PreWriteContents: map[string]PreImage{"x.go": {Unknown: "access is denied"}},
	}
	if _, err := (turnFiles{pre: map[string]PreImage{}}).restore(ws, result); err == nil {
		t.Fatal("restore accepted a file whose earlier state is unknown")
	}
	if data, _ := os.ReadFile(path); string(data) != "package x // the round's\n" {
		t.Fatalf("a refused restore still touched the file: %q", data)
	}
}

// A write the restore could not undo stays in WrittenPaths, so the gates that
// follow still see it (audit F7's second point).
func TestRestore_AWriteItCouldNotUndoStaysWritten(t *testing.T) {
	ws := t.TempDir()
	// The round created a directory where the turn expected a file to be
	// absent: removing a non-empty directory with os.Remove fails.
	created := filepath.Join(ws, "made")
	if err := os.MkdirAll(filepath.Join(created, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{
		WrittenPaths:     []string{"made"},
		PreWriteContents: map[string]PreImage{"made": {}},
	}
	if _, err := (turnFiles{pre: map[string]PreImage{}}).restore(ws, result); err == nil {
		t.Fatal("restore reported success over a write it could not undo")
	}
	if len(result.WrittenPaths) != 1 || result.WrittenPaths[0] != "made" {
		t.Fatalf("WrittenPaths = %v, want the write that could not be undone kept", result.WrittenPaths)
	}
}

// The test baseline puts each written file back as it was: an empty file that
// existed is an empty file in the baseline, not a deleted one; a created file
// is absent; an unknown preimage has no baseline at all.
func TestWriteOverlayFiles_ThreePreimages(t *testing.T) {
	ws, tmp := t.TempDir(), t.TempDir()
	replace, err := writeOverlayFiles(tmp, ws, map[string]PreImage{"empty.go": existed(""), "created.go": {}})
	if err != nil {
		t.Fatalf("writeOverlayFiles: %v", err)
	}
	if got := replace[filepath.Join(ws, "created.go")]; got != "" {
		t.Errorf("created file maps to %q, want \"\" (absent in the baseline)", got)
	}
	emptyOverlay := replace[filepath.Join(ws, "empty.go")]
	if emptyOverlay == "" {
		t.Fatal("an existing empty file is deleted in the baseline, want it restored empty")
	}
	if data, err := os.ReadFile(emptyOverlay); err != nil || len(data) != 0 {
		t.Errorf("empty file's overlay = %q (%v), want an empty file", data, err)
	}

	if _, err := writeOverlayFiles(tmp, ws, map[string]PreImage{"x.go": {Unknown: "access is denied"}}); err == nil {
		t.Error("an unknown preimage produced a baseline")
	}
}

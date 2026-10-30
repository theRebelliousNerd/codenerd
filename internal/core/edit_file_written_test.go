package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// An edit writes the file, so it records file_written with the new content's
// hash and the time, like a whole-file write. It recorded only file_edited
// and modified, neither timestamped, so what a running turn had written
// (running_work_wrote) and file_written's other readers missed every edit.
func TestHandleEditFile_RecordsFileWritten(t *testing.T) {
	vs, dir := createActionsTestVS(t)
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := time.Now().Unix()
	res, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID:  "edit-written",
		Target:    "a.go",
		SessionID: "s-edit",
		Payload:   map[string]any{"old": "func A() {}", "new": "func A() { _ = 1 }"},
	})
	if err != nil || !res.Success {
		t.Fatalf("edit failed: %v %s", err, res.Error)
	}
	data, err := os.ReadFile(filepath.Join(dir, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	for _, f := range res.FactsToAdd {
		if f.Predicate != "file_written" {
			continue
		}
		if f.Args[0] != vs.factPath(filepath.Join(dir, "a.go")) || f.Args[1] != hex.EncodeToString(sum[:]) || f.Args[2] != "s-edit" {
			t.Fatalf("file_written = %v", f.Args)
		}
		if at, ok := f.Args[3].(int64); !ok || at < before {
			t.Fatalf("file_written time = %v, want a Unix time at or after %d", f.Args[3], before)
		}
		return
	}
	t.Fatalf("an edit recorded no file_written: %v", res.FactsToAdd)
}

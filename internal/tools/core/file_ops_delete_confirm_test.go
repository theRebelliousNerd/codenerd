package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The defect these guard (F-SEC-3, from codeNERD's own security review of
// file_ops.go): delete_file described itself as "requires explicit permission"
// and enforced nothing, while VirtualStore.handleDeleteFile
// (virtual_store_file_actions.go:322) refuses without confirmed:true.
//
// Two paths to the same irreversible operation disagreeing about whether
// consent is required means the weaker one is the real policy.

func seedDeletable(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", dir)
	path := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(path, []byte("delete me"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	return path
}

func TestDeleteFile_RefusesWithoutConfirmation(t *testing.T) {
	path := seedDeletable(t)

	_, err := executeDeleteFile(context.Background(), map[string]any{"path": path})
	if err == nil {
		t.Fatal("delete_file removed a file without confirmation")
	}
	if !strings.Contains(err.Error(), "confirmed") {
		t.Errorf("error does not name the missing argument: %v", err)
	}

	// The decisive assertion: refusing must mean the file survives.
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("file was deleted despite the refusal: %v", statErr)
	}
}

// confirmed:false is an explicit "no", not a missing field, and must be
// honoured as such.
func TestDeleteFile_RefusesOnExplicitFalse(t *testing.T) {
	path := seedDeletable(t)

	if _, err := executeDeleteFile(context.Background(), map[string]any{
		"path": path, "confirmed": false,
	}); err == nil {
		t.Fatal("delete_file accepted confirmed:false")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("file was deleted on an explicit refusal: %v", statErr)
	}
}

func TestDeleteFile_ProceedsWhenConfirmed(t *testing.T) {
	path := seedDeletable(t)

	result, err := executeDeleteFile(context.Background(), map[string]any{
		"path": path, "confirmed": true,
	})
	if err != nil {
		t.Fatalf("a confirmed delete was refused: %v", err)
	}
	if !strings.Contains(result, "Deleted") {
		t.Errorf("unexpected result: %s", result)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file survived a confirmed delete")
	}
}

// The requirement must be declared, so the model supplies it rather than
// discovering the rule by failing a call.
func TestDeleteFileTool_DeclaresConfirmedAsRequired(t *testing.T) {
	tool := DeleteFileTool()

	found := false
	for _, r := range tool.Schema.Required {
		if r == "confirmed" {
			found = true
		}
	}
	if !found {
		t.Errorf("confirmed is enforced but not declared required: %v", tool.Schema.Required)
	}
	if _, ok := tool.Schema.Properties["confirmed"]; !ok {
		t.Error("confirmed has no schema property, so the model is not told it exists")
	}
}

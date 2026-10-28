package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

// create_file writes its source formatted, so the paranoid validator must not
// compare the payload to the bytes on disk: that rejected every create.
func TestValidateInteractive_CreateFileIsNotRejectedForFormatting(t *testing.T) {
	vs := gateStore(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "x_test.go")
	written := "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"
	if err := os.WriteFile(path, []byte(written), 0o644); err != nil {
		t.Fatal(err)
	}
	source := "package x\nfunc TestX(t *testing.T){}\n" // what the model sent
	got := vs.ValidateInteractiveToolResult(context.Background(), "cf-1", "create_file",
		map[string]any{"path": path, "source": source}, "created x_test.go, gofmt'd", true)
	if got.State == types.ToolValidationRejected {
		t.Fatalf("create_file was rejected for being formatted: %+v", got)
	}
}

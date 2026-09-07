package core

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/tools"
	coretools "codenerd/internal/tools/core"
)

func TestFileValidatorMatchesRealToolLineEndingContract(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "value.go")
	if err := os.WriteFile(path, []byte("package fixture\r\nconst Value = 1\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := tools.WithWorkspaceRoot(t.Context(), root)
	args := map[string]any{"path": "value.go", "content": "package fixture\nconst Value = 2\n"}
	out, err := coretools.WriteFileTool().Execute(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	request := ActionRequest{Type: ActionWriteFile, Target: path, Payload: args}
	validator := NewFileWriteValidator()
	if got := validator.Validate(ctx, request, ActionResult{Success: true, Output: out}); !got.Verified {
		t.Fatalf("real write rejected: %+v", got)
	}
	registry := NewValidatorRegistry()
	RegisterAllValidators(registry)
	if got := registry.Validate(ctx, request, ActionResult{Success: true, Output: out}); !ValidateAll(got) {
		t.Fatalf("production validator pipeline rejected write: %+v", got)
	}
	if err := os.WriteFile(path, []byte("package fixture\r\nconst Value = 99\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := validator.Validate(ctx, request, ActionResult{Success: true}); got.Verified {
		t.Fatal("different content accepted")
	}
}

func TestEditValidatorUsesRealToolArgumentsAndAllowsOverlappingReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "value.txt")
	if err := os.WriteFile(path, []byte("value\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := tools.WithWorkspaceRoot(t.Context(), root)
	args := map[string]any{"path": "value.txt", "old_text": "value", "new_text": "value extended"}
	if _, err := coretools.EditFileTool().Execute(ctx, args); err != nil {
		t.Fatal(err)
	}
	request := ActionRequest{Type: ActionEditFile, Target: path, Payload: args}
	validator := NewFileEditValidator()
	if got := validator.Validate(ctx, request, ActionResult{Success: true}); !got.Verified {
		t.Fatalf("real edit rejected: %+v", got)
	}
	if err := os.WriteFile(path, []byte("value\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := validator.Validate(ctx, request, ActionResult{Success: true}); got.Verified {
		t.Fatal("missing replacement accepted")
	}
}

func TestApplyEditsValidatesEveryResultingFile(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("changed"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r := NewValidatorRegistry()
	RegisterAllValidators(r)
	v := &VirtualStore{workingDir: root, validators: r}
	args := map[string]any{"edits": []any{map[string]any{"path": "a.txt"}, map[string]any{"path": "b.txt"}}}
	if err := v.ValidateInteractiveToolResult(t.Context(), "batch", "apply_edits", args, "applied", true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateInteractiveToolResult(t.Context(), "batch", "apply_edits", args, "applied", true); err == nil {
		t.Fatal("missing second artifact accepted")
	}
}

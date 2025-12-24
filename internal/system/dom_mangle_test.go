package system_test

import (
	"codenerd/internal/core"
	"codenerd/internal/system"
	"codenerd/internal/tactile"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodeDOM_Mangle_EndToEnd(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd", "mangle"), 0755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}

	demoFile := filepath.Join(ws, "demo.mg")
	demo := `Decl parent(A.Type<name>, B.Type<name>).
parent(/a, /b).
ancestor(X, Y) :-
    parent(X, Y).
`
	if err := os.WriteFile(demoFile, []byte(demo), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	kernel.SetWorkspace(ws)

	executor := tactile.NewSafeExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = ws
	vs := core.NewVirtualStoreWithConfig(executor, vsCfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	scope := system.NewHolographicCodeScope(ws, kernel, nil, 0)
	vs.SetCodeScope(scope)

	fileEditor := tactile.NewFileEditor()
	fileEditor.SetWorkingDir(ws)
	vs.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"test-open", "/open_file", demoFile}}); err != nil {
		t.Fatalf("open_file: %v", err)
	}
	if got := countFacts(t, kernel, "file_in_scope"); got < 1 {
		t.Fatalf("expected file_in_scope >= 1, got %d", got)
	}
	if got := countFacts(t, kernel, "code_element"); got == 0 {
		t.Fatalf("expected code_element > 0")
	}

	ref := "rule:ancestor/2#1"
	newRule := "ancestor(X, Y) :-\n\tparent(X, Y).\n"
	if _, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"test-edit",
			"/edit_element",
			ref,
			map[string]interface{}{"content": newRule},
		},
	}); err != nil {
		t.Fatalf("edit_element: %v", err)
	}

	afterJSON, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"test-get",
			"/get_element",
			ref,
			map[string]interface{}{"include_body": true},
		},
	})
	if err != nil {
		t.Fatalf("get_element: %v", err)
	}
	var after core.CodeElement
	if err := json.Unmarshal([]byte(afterJSON), &after); err != nil {
		t.Fatalf("unmarshal get_element: %v", err)
	}
	if !strings.Contains(after.Body, "\tparent(X, Y).") {
		t.Fatalf("expected edited body to contain %q", "\tparent(X, Y).")
	}

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"test-close", "/close_scope", ""}}); err != nil {
		t.Fatalf("close_scope: %v", err)
	}
	if got := countFacts(t, kernel, "code_element"); got != 0 {
		t.Fatalf("expected code_element to be cleared after close_scope, got %d", got)
	}
}

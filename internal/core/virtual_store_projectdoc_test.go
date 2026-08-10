package core

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

var errKernelDown = errors.New("kernel unavailable")

// forbidKernel returns one project_forbidden_path fact and errors on demand.
type forbidKernel struct {
	types.Kernel
	match  string
	reason string
	err    error
}

func (k *forbidKernel) Query(predicate string) ([]types.Fact, error) {
	if k.err != nil {
		return nil, k.err
	}
	if predicate != "project_forbidden_path" {
		return nil, nil
	}
	return []types.Fact{{
		Predicate: "project_forbidden_path",
		Args:      []any{k.match, k.reason},
	}}, nil
}

// The hole this closes: shards route file writes through the VirtualStore, not
// through session.Executor, and only the Executor checked nerd.md. A shard
// could write a path the interactive path refused.
func TestVirtualStore_ProjectForbidsWrite_BlocksEveryWriteAction(t *testing.T) {
	k := &forbidKernel{match: ".nerd/config.json", reason: "live user config"}
	q := k

	writes := []ActionType{
		ActionWriteFile, ActionEditFile, ActionDeleteFile,
		ActionEditLines, ActionInsertLines, ActionDeleteLines,
		ActionEditElement, ActionFSWrite,
	}
	for _, action := range writes {
		t.Run(string(action), func(t *testing.T) {
			reason, blocked := projectForbidsWriteWith(q, ActionRequest{
				Type:   action,
				Target: ".nerd/config.json",
			})
			if !blocked {
				t.Errorf("%s to a protected path was not blocked", action)
			}
			if reason != "live user config" {
				t.Errorf("reason = %q, want the rule's reason", reason)
			}
		})
	}
}

// Reading a protected file is fine and often necessary.
func TestVirtualStore_ProjectForbidsWrite_AllowsReads(t *testing.T) {
	q := &forbidKernel{match: ".nerd/config.json", reason: "live user config"}

	for _, action := range []ActionType{ActionReadFile, ActionGrep, ActionGlob, ActionListFiles, ActionFSRead} {
		if _, blocked := projectForbidsWriteWith(q, ActionRequest{Type: action, Target: ".nerd/config.json"}); blocked {
			t.Errorf("%s of a protected path was blocked; only writes are gated", action)
		}
	}
}

// Matching must survive separator and case differences, because the tool that
// names the target chose the spelling, not the rule author.
func TestVirtualStore_ProjectForbidsWrite_NormalizesSeparatorsAndCase(t *testing.T) {
	q := &forbidKernel{match: ".nerd/config.json", reason: "live user config"}

	for _, target := range []string{
		`.nerd\config.json`,
		`C:\CodeProjects\codeNERD\.nerd\config.json`,
		`.NERD/CONFIG.JSON`,
	} {
		if _, blocked := projectForbidsWriteWith(q, ActionRequest{Type: ActionWriteFile, Target: target}); !blocked {
			t.Errorf("target %q was not matched against the rule", target)
		}
	}
}

func TestVirtualStore_ProjectForbidsWrite_UnprotectedPathPassesThrough(t *testing.T) {
	q := &forbidKernel{match: ".nerd/config.json", reason: "live user config"}

	if _, blocked := projectForbidsWriteWith(q, ActionRequest{
		Type:   ActionWriteFile,
		Target: "internal/session/executor.go",
	}); blocked {
		t.Error("an unprotected path was blocked")
	}
}

func TestVirtualStore_ProjectForbidsWrite_BlocksMissingTarget(t *testing.T) {
	q := &forbidKernel{match: ".nerd/config.json", reason: "live user config"}
	if reason, blocked := projectForbidsWriteWith(q, ActionRequest{Type: ActionWriteFile}); !blocked || reason == "" {
		t.Fatalf("blocked=%v reason=%q, want missing-target denial", blocked, reason)
	}
}

// A kernel hiccup makes protection unknowable, so writes fail closed while
// reads remain available through their separate path.
func TestVirtualStore_ProjectForbidsWrite_FailsClosedOnKernelError(t *testing.T) {
	q := &forbidKernel{err: errKernelDown}

	reason, blocked := projectForbidsWriteWith(q, ActionRequest{
		Type:   ActionWriteFile,
		Target: ".nerd/config.json",
	})
	if !blocked {
		t.Error("a kernel query failure allowed a write while protection was unknown")
	}
	if reason == "" {
		t.Error("a degraded-policy denial must explain why it fired")
	}
}

func TestVirtualStore_ToolWriteGuardFailsClosedOnKernelError(t *testing.T) {
	v := &VirtualStore{kernel: &forbidKernel{err: errKernelDown}}
	guard := v.toolWriteGuard()
	if err := guard(nil, "write_file", map[string]any{"path": ".nerd/config.json"}); err == nil {
		t.Fatal("tool-layer write guard allowed a write while policy was unavailable")
	}
}

func TestVirtualStore_ToolWriteGuardBlocksMissingTarget(t *testing.T) {
	v := &VirtualStore{kernel: &forbidKernel{match: ".nerd/config.json", reason: "live user config"}}
	if err := v.toolWriteGuard()(nil, "write_file", map[string]any{"content": "x"}); err == nil {
		t.Fatal("tool-layer write guard allowed a write without a recognized target")
	}
}

func TestVirtualStore_ToolWriteGuardBlocksProtectedNestedTarget(t *testing.T) {
	v := &VirtualStore{kernel: &forbidKernel{match: ".nerd/config.json", reason: "live user config"}}
	err := v.toolWriteGuard()(nil, "apply_edits", map[string]any{
		"edits": []any{
			map[string]any{"path": "internal/session/executor.go"},
			map[string]any{"path": ".nerd/config.json"},
		},
	})
	if err == nil {
		t.Fatal("tool-layer write guard allowed a batch containing a protected nested target")
	}
}

// No kernel means no policy authority. This goes through the method rather
// than the seam so the production nil-kernel path stays fail closed.
func TestVirtualStore_ProjectForbidsWrite_NoKernelBlocks(t *testing.T) {
	v := &VirtualStore{}
	if _, blocked := v.projectForbidsWrite(ActionRequest{
		Type:   ActionWriteFile,
		Target: ".nerd/config.json",
	}); !blocked {
		t.Error("write allowed with no policy authority attached")
	}
}

func TestVirtualStore_ToolWriteGuardBlocksWithoutKernel(t *testing.T) {
	v := &VirtualStore{}
	if err := v.toolWriteGuard()(nil, "write_file", map[string]any{"path": "internal/core/kernel.go"}); err == nil {
		t.Fatal("tool-layer write guard allowed a write with no policy authority")
	}
}

func TestVirtualStore_ToolWriteGuardStopsShellIncidentAtRegistry(t *testing.T) {
	v := &VirtualStore{}
	registry := tools.NewRegistry()
	v.installToolWriteGuard(registry)

	executed := false
	if err := registry.Register(&tools.Tool{
		Name:     "run_command",
		Category: tools.CategoryCode,
		Schema: tools.ToolSchema{
			Required: []string{"command"},
		},
		Execute: func(context.Context, map[string]any) (string, error) {
			executed = true
			return "executed", nil
		},
	}); err != nil {
		t.Fatalf("register command probe: %v", err)
	}

	for _, command := range []string{
		"git checkout -- internal/logging/logging.go internal/logging/audit.go",
		`python -c "import shutil; shutil.rmtree('internal/browser/repotrace')"`,
		"git status && Remove-Item internal/logging/logging.go",
	} {
		executed = false
		if _, err := registry.Execute(context.Background(), "run_command", map[string]any{"command": command}); err == nil {
			t.Fatalf("registry allowed incident command %q", command)
		}
		if executed {
			t.Fatalf("incident command %q reached the registered handler", command)
		}
	}

	executed = false
	result, err := registry.Execute(context.Background(), "run_command", map[string]any{"command": "go test ./internal/projectdoc"})
	if err != nil {
		t.Fatalf("verification command was denied: %v", err)
	}
	if !executed || result.Result != "executed" {
		t.Fatalf("verification command did not reach handler: executed=%v result=%q", executed, result.Result)
	}

	gitExecuted := false
	if err := registry.Register(&tools.Tool{
		Name:     "git_operation",
		Category: tools.CategoryCode,
		Schema: tools.ToolSchema{
			Required: []string{"operation"},
		},
		Execute: func(context.Context, map[string]any) (string, error) {
			gitExecuted = true
			return "executed", nil
		},
	}); err != nil {
		t.Fatalf("register git probe: %v", err)
	}
	if _, err := registry.Execute(context.Background(), "git_operation", map[string]any{
		"operation": "checkout",
		"args":      "-- internal/logging/logging.go",
	}); err == nil {
		t.Fatal("registry allowed structured git checkout")
	}
	if gitExecuted {
		t.Fatal("structured git checkout reached the registered handler")
	}
}

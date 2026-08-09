package core

import (
	"errors"
	"testing"

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

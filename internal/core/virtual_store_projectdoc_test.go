package core

import (
	"errors"
	"testing"

	"codenerd/internal/types"
)

var errKernelDown = errors.New("kernel unavailable")

// forbidKernel returns one project_forbidden_path fact and errors on demand.
type forbidKernel struct {
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

// A kernel hiccup must not block every write; it must be visible instead.
func TestVirtualStore_ProjectForbidsWrite_FailsOpenOnKernelError(t *testing.T) {
	q := &forbidKernel{err: errKernelDown}

	if _, blocked := projectForbidsWriteWith(q, ActionRequest{
		Type:   ActionWriteFile,
		Target: ".nerd/config.json",
	}); blocked {
		t.Error("a kernel query failure blocked the write; the gate must fail open and warn")
	}
}

// No kernel attached is not evidence of protection either. This goes through
// the method rather than the seam, so it also covers the nil-kernel short
// circuit in VirtualStore.projectForbidsWrite.
func TestVirtualStore_ProjectForbidsWrite_NoKernelAllows(t *testing.T) {
	v := &VirtualStore{}
	if _, blocked := v.projectForbidsWrite(ActionRequest{
		Type:   ActionWriteFile,
		Target: ".nerd/config.json",
	}); blocked {
		t.Error("write blocked with no kernel attached")
	}
}

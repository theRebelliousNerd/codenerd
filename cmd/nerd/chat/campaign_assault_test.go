package chat

import (
	"codenerd/internal/perception"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssaultArgs_IncludeFolder(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "internal/core"}

	args := assaultArgs(ws, "run an assault campaign on internal/core", intent)
	if len(args) != 1 || args[0] != "internal/core" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestAssaultArgs_PackageScope(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "internal/core"}

	args := assaultArgs(ws, "run an assault on package internal/core", intent)
	if len(args) < 2 || args[0] != "package" || args[1] != "internal/core" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestAssaultArgs_Flags(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "internal/core"}

	args := assaultArgs(ws, "run an assault on internal/core with -race and go vet (no nemesis)", intent)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "internal/core") {
		t.Fatalf("expected include internal/core, got: %#v", args)
	}
	if !strings.Contains(joined, "--race") {
		t.Fatalf("expected --race, got: %#v", args)
	}
	if !strings.Contains(joined, "--vet") {
		t.Fatalf("expected --vet, got: %#v", args)
	}
	if !strings.Contains(joined, "--no-nemesis") {
		t.Fatalf("expected --no-nemesis, got: %#v", args)
	}
}

func TestAssaultArgs_GenericTargetIgnored(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "codebase"}

	args := assaultArgs(ws, "run an assault on the whole repo", intent)
	if len(args) != 0 {
		t.Fatalf("expected no args for whole-repo assault, got: %#v", args)
	}
}

func TestNormalizeAssaultInclude_AbsolutePath(t *testing.T) {
	ws := t.TempDir()
	abs := filepath.Join(ws, "internal", "core", "kernel.go")

	inc := normalizeAssaultInclude(ws, abs)
	if runtime.GOOS == "windows" {
		// On Windows we expect slash-normalized.
		if inc != "internal/core" {
			t.Fatalf("unexpected include: %q", inc)
		}
		return
	}
	if inc != filepath.ToSlash(filepath.Join("internal", "core")) {
		t.Fatalf("unexpected include: %q", inc)
	}
}

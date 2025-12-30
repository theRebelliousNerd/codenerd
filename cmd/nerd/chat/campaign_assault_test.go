package chat

import (
	"codenerd/internal/perception"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssaultArgsFromNaturalLanguage_IncludeFolder(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "internal/core"}

	args, ok := assaultArgsFromNaturalLanguage(ws, "run an assault campaign on internal/core", intent)
	if !ok {
		t.Fatal("expected assault to be detected")
	}
	if len(args) != 1 || args[0] != "internal/core" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestAssaultArgsFromNaturalLanguage_PackageScope(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "internal/core"}

	args, ok := assaultArgsFromNaturalLanguage(ws, "run an assault on package internal/core", intent)
	if !ok {
		t.Fatal("expected assault to be detected")
	}
	if len(args) < 2 || args[0] != "package" || args[1] != "internal/core" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestAssaultArgsFromNaturalLanguage_Flags(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "internal/core"}

	args, ok := assaultArgsFromNaturalLanguage(ws, "run an assault on internal/core with -race and go vet (no nemesis)", intent)
	if !ok {
		t.Fatal("expected assault to be detected")
	}
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

func TestAssaultArgsFromNaturalLanguage_CampaignVerbFallback(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/campaign", Target: "soak test internal/core"}

	args, ok := assaultArgsFromNaturalLanguage(ws, "start a campaign to soak test internal/core", intent)
	if !ok {
		t.Fatal("expected assault to be detected from /campaign wording")
	}
	if len(args) != 1 || args[0] != "internal/core" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestAssaultArgsFromNaturalLanguage_GenericTargetIgnored(t *testing.T) {
	ws := t.TempDir()
	intent := perception.Intent{Verb: "/assault", Target: "codebase"}

	args, ok := assaultArgsFromNaturalLanguage(ws, "run an assault on the whole repo", intent)
	if !ok {
		t.Fatal("expected assault to be detected")
	}
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

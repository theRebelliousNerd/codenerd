package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F-VERIFY-1: the executor never compiled its own edits, so a turn could write
// four compile errors and still assert task_status(/manual_instruction,
// /complete) with exit 0. These tests pin the gate that stops that.

func TestTouchedGoFiles(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  bool
	}{
		{"nothing written", nil, false},
		{"markdown only", []string{"Docs/architecture/README.md", ".nerd/notes.txt"}, false},
		{"one go file", []string{"Docs/x.md", "internal/session/executor.go"}, true},
		{"uppercase extension", []string{"cmd/nerd/Main.GO"}, true},
		{"whitespace padded", []string{"  internal/core/kernel.go  "}, true},
		{"go in the middle of a name", []string{"internal/gopher/notes.md"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := touchedGoFiles(tc.paths); got != tc.want {
				t.Errorf("touchedGoFiles(%v) = %v; want %v", tc.paths, got, tc.want)
			}
		})
	}
}

// A verification that did not run must never be reported as a pass. This is the
// distinction that keeps the gate honest: BuildVerification{Ran:false} means
// "unknown", and callers treat it as such.
func TestVerifyBuild_EmptyWorkspaceDoesNotRun(t *testing.T) {
	v := verifyBuild(context.Background(), "  ", nil)
	if v.Ran {
		t.Error("verifyBuild claimed to run against an empty workspace path")
	}
	if v.OK {
		t.Error("a verification that did not run must not report OK")
	}
}

// The real gate: a package that does not compile must come back Ran && !OK,
// with the compiler's own message available to hand to the repair round.
func TestVerifyBuild_DetectsBrokenPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a throwaway package")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module verifyprobe\n\ngo 1.21\n")

	write("main.go", "package main\n\nfunc main() {}\n")
	if v := verifyBuild(context.Background(), ws, nil); !v.Ran || !v.OK {
		t.Fatalf("healthy package reported Ran=%v OK=%v output=%q", v.Ran, v.OK, v.Output)
	}

	// Exactly the mistake observed live: a call to a helper that was planned
	// but never written.
	write("main.go", "package main\n\nfunc main() { neverWritten() }\n")
	v := verifyBuild(context.Background(), ws, nil)
	if !v.Ran {
		t.Fatal("verification did not run against a broken package")
	}
	if v.OK {
		t.Fatal("verification passed a package that does not compile")
	}
	if !strings.Contains(v.Output, "neverWritten") {
		t.Errorf("compiler output does not name the offending symbol, so the repair round has nothing to work from: %q", v.Output)
	}
}

func TestBuildRepairPrompt_CarriesTheCompilerOutput(t *testing.T) {
	out := "cmd/nerd/cmd_instruction.go:362:23: undefined: regexp"
	p := buildRepairPrompt(out)

	if !strings.Contains(p, out) {
		t.Error("repair prompt drops the compiler output, which is the only thing that makes the round useful")
	}
	// The prompt must state the failure, not ask whether one occurred — the
	// compiler already decided.
	if !strings.Contains(strings.ToLower(p), "do not compile") {
		t.Error("repair prompt does not state the failure as fact")
	}
	// Naming the three observed mistake classes is the difference between a
	// generic retry and a targeted one.
	for _, want := range []string{"imported and not used", "no new variables on left side of :=", "undefined:"} {
		if !strings.Contains(p, want) {
			t.Errorf("repair prompt does not name the %q mistake class", want)
		}
	}
}

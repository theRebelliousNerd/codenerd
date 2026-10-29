package core

import "testing"

// A component that turns arbitrary text into a shell command is an injection
// surface regardless of who is expected to author the text.
//
// Observed live 2026-08-08: an instruction beginning "Add table-driven tests to
// internal/tactile/python/..." was classified /run_tests, and the whole sentence
// was handed to `bash -c`:
//
//	bash: -c: line 1: syntax error near unexpected token `('
//
// Harmless there only because prose is not valid shell. Campaign task
// descriptions, delegated shard tasks and nerd.md content reach the same code
// path, and any of those containing `;` or a backtick would have been executed.
func TestLooksLikeShellCommand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		// Real commands must still work — a guard that rejects everything is
		// just a different bug.
		{"go test", "go test ./...", true},
		{"go build with flags", "go build -o nerd.exe ./cmd/nerd", true},
		{"pytest", "pytest -xvs tests/", true},
		{"absolute path binary", "/usr/bin/make check", true},
		{"windows path binary", `C:\tools\make.exe all`, true},
		{"package spec", "go test ./internal/core", true},

		// The observed failure and its relatives.
		{"the observed prose", "Add table-driven tests to internal/tactile/python/environment_test.go for the pure functions", false},
		{"leading capital word", "Run the tests please", false},
		{"trailing prose after a real command is still a command", "go test ./... and report", true},
		{"unknown head binary", "frobnicate --all", false},
		{"multi-line", "go test ./...\nrm -rf /", false},
		{"empty", "", false},
		{"whitespace only", "   ", false},

		// A lowercase sentence is still prose: "add" is not a program.
		{"lowercase prose", "add tests then run them", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeShellCommand(tc.in); got != tc.want {
				t.Errorf("looksLikeShellCommand(%q) = %v; want %v", tc.in, got, tc.want)
			}
		})
	}
}

// Length is bounded so a very long "command" cannot slip through on the head
// token alone.
func TestLooksLikeShellCommand_RejectsOverlongInput(t *testing.T) {
	long := "go test"
	for len(long) <= 300 {
		long += " ./pkg"
	}
	if looksLikeShellCommand(long) {
		t.Error("an over-length target was accepted as a command")
	}
}

func TestCommandFromActionRequest(t *testing.T) {
	const def = "go test ./..."

	t.Run("explicit command payload wins", func(t *testing.T) {
		req := ActionRequest{
			Target:  "Add table-driven tests to the thing",
			Payload: map[string]any{"command": "go test ./internal/core"},
		}
		if got := commandFromActionRequest(req, def); got != "go test ./internal/core" {
			t.Errorf("got %q; want the explicit payload command", got)
		}
	})

	t.Run("prose target falls back to the default instead of executing", func(t *testing.T) {
		req := ActionRequest{Target: "Add table-driven tests to internal/tactile/python/environment_test.go"}
		if got := commandFromActionRequest(req, def); got != def {
			t.Errorf("got %q; want the default — prose must never reach bash", got)
		}
	})

	t.Run("command-shaped target is still honoured", func(t *testing.T) {
		req := ActionRequest{Target: "go test ./internal/session"}
		if got := commandFromActionRequest(req, def); got != "go test ./internal/session" {
			t.Errorf("got %q; want the target, which is a real command", got)
		}
	})

	t.Run("empty target uses the default", func(t *testing.T) {
		if got := commandFromActionRequest(ActionRequest{}, def); got != def {
			t.Errorf("got %q; want the default", got)
		}
	})

	// The specific shape that makes this a security fix rather than a tidy-up.
	t.Run("injection-shaped prose does not reach the shell", func(t *testing.T) {
		req := ActionRequest{Target: "please run the tests; rm -rf /tmp/x"}
		if got := commandFromActionRequest(req, def); got != def {
			t.Errorf("got %q; a target carrying a chained command was accepted", got)
		}
	})
}

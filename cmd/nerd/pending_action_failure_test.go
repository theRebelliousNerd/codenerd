package main

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

// An action that ran to completion and reported failure must not be reported as
// executed.
//
// Observed live 2026-08-08: an instruction was misclassified /run_tests, the
// handler executed and reported success=false, and `nerd run` printed a
// completion envelope asserting task_status(/manual_instruction, /complete)
// with "all 8 subtasks evidenced" — and exited 0.
//
// The routing layer cannot fix this for everyone: internal/core/tdd_loop.go
// needs a failing `run_tests` to be data rather than an error. So the
// distinction has to be made here, and these tests pin the shape of the result
// that the one-shot CLI must treat as a failure.
func TestActionResultFailureIsDistinguishable(t *testing.T) {
	cases := []struct {
		name       string
		result     core.ActionResult
		wantFailed bool
	}{
		{"success with output", core.ActionResult{Success: true, Output: "ok"}, false},
		{"success with empty output", core.ActionResult{Success: true}, false},
		{"failure with error text", core.ActionResult{Success: false, Error: "exit status 2"}, true},
		{"failure with only output", core.ActionResult{Success: false, Output: "bash: syntax error"}, true},
		{"failure with nothing", core.ActionResult{Success: false}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := !tc.result.Success; got != tc.wantFailed {
				t.Fatalf("Success=%v read as failed=%v; want %v", tc.result.Success, got, tc.wantFailed)
			}
		})
	}
}

// The failure message must carry something actionable. A bare "action reported
// failure" sends the reader to the logs for information the result already had.
func TestActionFailureDetailSelection(t *testing.T) {
	detailOf := func(r core.ActionResult) string {
		detail := strings.TrimSpace(r.Error)
		if detail == "" {
			detail = strings.TrimSpace(r.Output)
		}
		if detail == "" {
			detail = "handler reported failure with no detail"
		}
		return detail
	}

	if got := detailOf(core.ActionResult{Error: "exit status 2", Output: "noise"}); got != "exit status 2" {
		t.Errorf("Error field should win, got %q", got)
	}
	if got := detailOf(core.ActionResult{Output: "bash: syntax error"}); got != "bash: syntax error" {
		t.Errorf("Output should be used when Error is empty, got %q", got)
	}
	if got := detailOf(core.ActionResult{}); !strings.Contains(got, "no detail") {
		t.Errorf("a failure with nothing must still say so, got %q", got)
	}
}

package main

import (
	"strings"
	"testing"
)

// F-RUN-1 regression tests.
//
// The live failure: a compound instruction ("wire the TurnStart, TurnEnd and
// IntentParsed audit events") had one of its three parts silently dropped, and
// the run still asserted task_status(/manual_instruction, /complete) with exit
// 0. The result text was non-empty and plausible, so the pre-existing
// hollow-success guard — which only fires when nothing at all happened — could
// not catch it.
//
// These tests pin both halves of the fix: the decomposition must recognise a
// compound instruction without shredding a single one, and the accounting must
// notice a subtask the result never mentions.

func TestExtractRequestedSubtasks_SingleTaskIsNotCompound(t *testing.T) {
	// The expensive failure mode is over-splitting: every single-task
	// instruction containing a comma or the word "and" would be permanently
	// downgraded to /partial, which is worse than the bug being fixed.
	singles := []string{
		"fix the bug in internal/core/kernel.go",
		"add a test for ResolveWorkspacePath, covering symlinks",
		"refactor the executor so it reads cleanly and consistently",
		"document the campaign orchestrator",
		"",
	}
	for _, in := range singles {
		if got := extractRequestedSubtasks(in); len(got) >= 2 {
			t.Errorf("extractRequestedSubtasks(%q) = %v (len %d); want fewer than 2 — single tasks must not be split",
				in, got, len(got))
		}
	}
}

func TestExtractRequestedSubtasks_CompoundForms(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantMin int
	}{
		{"and-separated", "wire the TurnStart event and wire the TurnEnd event and wire the IntentParsed event", 3},
		{"semicolons", "add the audit reader; wire the write guard; document both", 3},
		{"then", "run the scan then commit the result", 2},
		{"numbered list", "1. add the reader\n2. wire the guard\n3. write the docs", 3},
		{"bulleted list", "- add the reader\n- wire the guard", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRequestedSubtasks(tc.input)
			if len(got) < tc.wantMin {
				t.Fatalf("extractRequestedSubtasks(%q) = %v (len %d); want at least %d fragments",
					tc.input, got, len(got), tc.wantMin)
			}
		})
	}
}

func TestFindUnaccountedSubtasks_ReproducesTheLiveMiss(t *testing.T) {
	// Verbatim shape of the observed failure: two of three audit events are
	// reported, the third is never mentioned, and the summary reads as success.
	subtasks := extractRequestedSubtasks(
		"wire the TurnStart audit event and wire the TurnEnd audit event and wire the IntentParsed audit event")
	if len(subtasks) < 3 {
		t.Fatalf("precondition failed: expected 3 subtasks, got %d (%v)", len(subtasks), subtasks)
	}

	output := "Wired the TurnStart audit event in session/executor.go and the TurnEnd audit event alongside it. Both are emitted on every turn."

	missing := findUnaccountedSubtasks(subtasks, output)
	if len(missing) == 0 {
		t.Fatalf("findUnaccountedSubtasks did not flag the dropped subtask; subtasks=%v output=%q", subtasks, output)
	}
	joined := strings.ToLower(strings.Join(missing, " "))
	if !strings.Contains(joined, "intentparsed") {
		t.Errorf("expected the unmentioned IntentParsed subtask to be flagged, got %v", missing)
	}
	// The two that WERE done must not be reported as missing — a check that
	// cries wolf on completed work is one that gets disabled.
	if strings.Contains(joined, "turnstart") || strings.Contains(joined, "turnend") {
		t.Errorf("flagged a subtask the output clearly evidences: %v", missing)
	}
}

func TestFindUnaccountedSubtasks_AllEvidencedIsClean(t *testing.T) {
	subtasks := extractRequestedSubtasks(
		"wire the TurnStart audit event and wire the TurnEnd audit event and wire the IntentParsed audit event")
	output := "Wired TurnStart, TurnEnd and IntentParsed audit events; all three now emit."

	if missing := findUnaccountedSubtasks(subtasks, output); len(missing) != 0 {
		t.Errorf("expected no gaps when every subtask is evidenced, got %v", missing)
	}
}

func TestFindUnaccountedSubtasks_EmptyInputIsSafe(t *testing.T) {
	if got := findUnaccountedSubtasks(nil, "anything"); got != nil {
		t.Errorf("findUnaccountedSubtasks(nil, ...) = %v; want nil", got)
	}
}

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/tools"
	coretools "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// Incomplete validation is not affirmative validation
// (Docs/audits/tool-effect-receipts-and-validation-completeness.md, TOOL-EFFECT-02).

func TestSummarizeValidation_States(t *testing.T) {
	for _, tc := range []struct {
		name    string
		results []ValidationResult
		want    types.ToolValidationState
		reason  string
	}{
		{"no readings", nil, types.ToolValidationUnvalidated, ""},
		{"every validator deferred", []ValidationResult{{Verified: true, Confidence: 0, Method: ValidationMethodSkipped}}, types.ToolValidationUnvalidated, ""},
		{"an affirmative pass", []ValidationResult{{Verified: true, Confidence: 1, Method: "hash"}, {Verified: true, Confidence: 0}}, types.ToolValidationVerified, ""},
		{"a definitive failure", []ValidationResult{{Verified: true, Confidence: 1}, {Verified: false, Confidence: 1, Error: "syntax"}}, types.ToolValidationRejected, "syntax"},
		// The ordering fixture: FirstFailure read only the first failed
		// reading, so a low-confidence objection ahead of a definitive one
		// hid it.
		{"a low-confidence objection ahead of a definitive failure", []ValidationResult{
			{Verified: false, Confidence: 0.3, Error: "maybe"},
			{Verified: false, Confidence: 0.95, Error: "definitely"},
		}, types.ToolValidationRejected, "definitely"},
		{"a low-confidence objection alone", []ValidationResult{{Verified: true, Confidence: 1}, {Verified: false, Confidence: 0.3, Error: "maybe"}}, types.ToolValidationVerified, ""},
		{"a validator that could not complete", []ValidationResult{{Verified: true, Confidence: 1}, {State: ValidationUnavailable, Error: "validation cancelled"}}, types.ToolValidationIncomplete, "validation cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := SummarizeValidation("x", tc.results)
			if got.State != tc.want || got.Reason != tc.reason {
				t.Fatalf("SummarizeValidation = %+v, want %s (%q)", got, tc.want, tc.reason)
			}
			if len(got.Observations) != len(tc.results) {
				t.Fatalf("observations = %d, want every reading kept (%d)", len(got.Observations), len(tc.results))
			}
		})
	}
}

// A canceled pass is unavailable, never a failure: the self-healing arms
// would otherwise rewrite or roll back an artifact nothing measured.
func TestValidatorRegistry_CancellationIsUnavailable(t *testing.T) {
	r := NewValidatorRegistry()
	RegisterAllValidators(r)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := r.Validate(ctx, ActionRequest{ActionID: "a1", Type: ActionWriteFile, Target: "/nonexistent"}, ActionResult{Success: true})
	if len(results) != 1 || results[0].State != ValidationUnavailable || results[0].Validator == "" {
		t.Fatalf("results = %+v, want one unavailable reading naming its validator", results)
	}
	if got := SummarizeValidation("x", results); got.State != types.ToolValidationIncomplete {
		t.Fatalf("summary = %+v, want incomplete", got)
	}
	facts := results[0].ToFacts()
	if facts[0].Predicate != "action_validation_unavailable" {
		t.Fatalf("facts = %+v, want action_validation_unavailable, not a failure", facts)
	}
}

func TestValidationResult_ToFacts_SkippedIsNotVerified(t *testing.T) {
	vr := ValidationResult{ActionID: "a1", ActionType: ActionWriteFile, Verified: true, Confidence: 0, Method: ValidationMethodSkipped}
	facts := vr.ToFacts()
	if facts[0].Predicate != "action_validation_skipped" {
		t.Fatalf("facts = %+v, want action_validation_skipped, not an affirmative action_verified", facts)
	}
	if m, ok := facts[1].Args[1].(MangleAtom); !ok || m != "/skipped" {
		t.Fatalf("validation_method_used method = %#v, want the /name the schema declares", facts[1].Args[1])
	}
}

// The audit's concrete witness: a write of incomplete JSON passes the write
// handler (it checks Go syntax only) and the syntax validator rejects it at
// full confidence. The interactive bridge reports the rejection typed; the
// session keeps the bytes (internal/session tool_effects_test.go).
func TestValidateInteractiveToolResult_RejectsTheWrittenIncompleteJSON(t *testing.T) {
	root := t.TempDir()
	ctx := tools.WithWorkspaceRoot(t.Context(), root)
	args := map[string]any{"path": "a.json", "content": `{"a":`}
	out, err := coretools.WriteFileTool().Execute(ctx, args)
	if err != nil {
		t.Fatalf("the write handler refused the JSON (a preflight refusal is a different fixture): %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "a.json")); string(got) != `{"a":` {
		t.Fatalf("bytes = %q, want the handler's write", got)
	}
	r := NewValidatorRegistry()
	RegisterAllValidators(r)
	v := &VirtualStore{workingDir: root, validators: r}
	got := v.ValidateInteractiveToolResult(ctx, "w1", "write_file", args, out, true)
	if got.State != types.ToolValidationRejected {
		t.Fatalf("validation = %+v, want rejected", got)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got := v.ValidateInteractiveToolResult(canceled, "w2", "write_file", args, out, true); got.State != types.ToolValidationIncomplete {
		t.Fatalf("canceled validation = %+v, want incomplete", got)
	}
	if got := v.ValidateInteractiveToolResult(ctx, "w3", "write_file", args, out, false); got.State != types.ToolValidationNotRun {
		t.Fatalf("validation of no result = %+v, want not_run", got)
	}
}

// side_effect_attempted reads the new rows: a skipped or unavailable reading
// still proves the tool ran, and leaves it in validation limbo.
func TestValidationPolicy_IncompleteReadingsAreLimboNotFailure(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []Fact{
		{Predicate: "action_validation_unavailable", Args: []any{"u1", MangleAtom("/write_file"), "validation cancelled", int64(1)}},
		{Predicate: "action_validation_skipped", Args: []any{"s1", MangleAtom("/write_file"), MangleAtom("/skipped"), int64(1)}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	limbo, err := k.Query("unvalidated_side_effect")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range limbo {
		seen[types.ExtractString(f.Args[0])] = true
	}
	if !seen["u1"] || !seen["s1"] {
		t.Fatalf("unvalidated_side_effect = %v, want both incomplete readings in limbo", limbo)
	}
	failed, err := k.Query("action_failed_validation")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range failed {
		if types.ExtractString(f.Args[0]) == "u1" {
			t.Fatal("an unavailable reading derived action_failed_validation")
		}
	}
}

package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/types"
	"context"
	"strings"
	"testing"
)

// TestRunShardValidationCheckpoint_VerdictParsing verifies the structured
// reviewer verdict on the live path: runShardValidationCheckpoint reads
// checkpoint_verdict/4 from the KERNEL (where the session executor asserts
// control_packet.mangle_updates), not from the string returned by spawnTask
// (which is only the surface_response). A raw-envelope string is still
// accepted as a compatibility path for executors that return it verbatim;
// anything else fails closed. Free-text PASS/FAIL — including the old
// substring shapes — and bare atoms outside the envelope must never satisfy
// the checkpoint.
//
// Each case drives the checkpoint through a fake task executor that behaves
// like the real JITExecutor (returns surface text only) plus a kernel double
// seeded with checkpoint_verdict facts, following the MockTaskExecutor /
// MockKernel setup used by checkpoint_manual_review_test.go and
// checkpoint_failclosed_test.go.
func TestRunShardValidationCheckpoint_VerdictParsing(t *testing.T) {
	tests := []struct {
		name           string
		surface        string // what Execute returns on the live path (surface_response only)
		rawEnvelope    string // when set, Execute returns this raw envelope instead (compat path)
		kernelFacts    []core.Fact
		wantPass       bool
		wantFailClosed bool // if true, also assert details says verdict could not be determined
	}{
		{
			name:    "pass_via_kernel",
			surface: "done",
			kernelFacts: []core.Fact{
				{Predicate: "checkpoint_verdict", Args: []any{"test-phase", types.MangleAtom("/pass"), "all objectives met", int64(95)}},
			},
			wantPass: true,
		},
		{
			name:    "fail_via_kernel",
			surface: "done",
			kernelFacts: []core.Fact{
				{Predicate: "checkpoint_verdict", Args: []any{"test-phase", types.MangleAtom("/fail"), "three sites unverified", int64(80)}},
			},
			wantPass: false,
		},
		{
			name:        "pass_via_raw_envelope_compat",
			rawEnvelope: `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"test-phase\", /pass, \"all objectives met\", 95)"]}, "surface_response": "done"}`,
			wantPass:    true,
		},
		{
			name:        "fail_via_raw_envelope_compat",
			rawEnvelope: `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"test-phase\", /fail, \"three sites unverified\", 80)"]}, "surface_response": "done"}`,
			wantPass:    false,
		},
		{
			name:    "bare_atom_in_surface_is_not_a_verdict",
			surface: `checkpoint_verdict("test-phase", /pass, "all objectives met", 95)`,
			// A bare atom in the surface string is inert: only a kernel
			// fact or a control_packet.mangle_updates entry decides.
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:    "old_substring_shapes_no_longer_satisfy",
			surface: "**PASS - Discovery objectives met.**\n\nNo failures found in the audit.",
			// Exact live response that the old substring parser recorded as
			// a failure and the interim first-line parser recorded as a
			// pass. Under the structured contract prose is not a verdict:
			// no checkpoint_verdict/4 means fail closed, never pass.
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:           "prose_fail_is_not_a_verdict",
			surface:        "FAIL: objectives not met, three sites unverified",
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:    "fail_closed_verdict_for_wrong_phase",
			surface: "done",
			kernelFacts: []core.Fact{
				{Predicate: "checkpoint_verdict", Args: []any{"other-phase", types.MangleAtom("/pass"), "all objectives met", int64(95)}},
			},
			// A well-formed fact for another phase cannot satisfy this one.
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:    "fail_closed_malformed_verdict_token",
			surface: "done",
			kernelFacts: []core.Fact{
				{Predicate: "checkpoint_verdict", Args: []any{"test-phase", types.MangleAtom("/maybe"), "unsure", int64(50)}},
			},
			// Verdict must be /pass or /fail; anything else is malformed.
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:           "fail_closed_no_verdict_anywhere",
			surface:        "The phase looks reasonable.",
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:           "fail_closed_empty_response",
			surface:        "",
			wantPass:       false,
			wantFailClosed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kernel := &MockKernel{Facts: append([]core.Fact(nil), tc.kernelFacts...)}
			reviewText := tc.surface
			if tc.rawEnvelope != "" {
				reviewText = tc.rawEnvelope
			}
			executor := &MockTaskExecutor{
				ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
					// Live path: the reviewer asserts its verdict DURING execution,
					// i.e. after the runner's retract-before-spawn. Seeded Facts
					// represent pre-existing state (which must be retracted), so
					// re-assert them here to simulate the reviewer's live assertion.
					for _, f := range tc.kernelFacts {
						_ = kernel.Assert(f)
					}
					return reviewText, nil
				},
			}
			cr := NewCheckpointRunner(nil, executor, t.TempDir(), kernel)
			phase := &Phase{Name: "test-phase"}

			passed, details, err := cr.runShardValidationCheckpoint(context.Background(), phase)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if passed != tc.wantPass {
				t.Errorf("runShardValidationCheckpoint %q: passed=%v, want %v (details=%q)", tc.name, passed, tc.wantPass, details)
			}
			if tc.wantFailClosed {
				lower := strings.ToLower(details)
				if !strings.Contains(lower, "could not be determined") {
					t.Errorf("fail-closed case %q: details should say verdict could not be determined; got %q", tc.name, details)
				}
			} else if tc.wantPass {
				if !strings.Contains(details, "Review passed") {
					t.Errorf("pass case %q: details should contain 'Review passed'; got %q", tc.name, details)
				}
			} else {
				// Expected fail via explicit verdict — details should say
				// "Review failed".
				if !strings.Contains(details, "Review failed") {
					t.Errorf("fail case %q: details should contain 'Review failed'; got %q", tc.name, details)
				}
			}
			// Also exercise the public Run path for one representative
			// fail-closed case to ensure the wiring through Run is covered.
			if tc.name == "fail_closed_no_verdict_anywhere" {
				passed2, details2, err := cr.Run(context.Background(), phase, VerifyShardValidate)
				if err != nil {
					t.Fatalf("Run(VerifyShardValidate) unexpected error: %v", err)
				}
				if passed2 != tc.wantPass {
					t.Errorf("Run(VerifyShardValidate) %q: passed=%v, want %v (details=%q)", tc.name, passed2, tc.wantPass, details2)
				}
				if !strings.Contains(strings.ToLower(details2), "could not be determined") {
					t.Errorf("Run fail-closed details should say verdict could not be determined; got %q", details2)
				}
			}
		})
	}
}
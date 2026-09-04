package campaign

import (
	"codenerd/internal/session"
	"context"
	"strings"
	"testing"
)

// TestRunShardValidationCheckpoint_VerdictParsing verifies the structured
// reviewer verdict: runShardValidationCheckpoint accepts only a
// checkpoint_verdict/4 fact carried in the reviewer's control-packet
// mangle_updates and fails closed on anything else. Free-text PASS/FAIL —
// including the old substring shapes — and bare atoms outside the envelope
// must never satisfy the checkpoint.
//
// Each case drives the checkpoint through a fake task executor that returns
// the given review text, following the MockTaskExecutor setup used by
// checkpoint_manual_review_test.go and checkpoint_failclosed_test.go.
func TestRunShardValidationCheckpoint_VerdictParsing(t *testing.T) {
	tests := []struct {
		name           string
		reviewText     string
		wantPass       bool
		wantFailClosed bool // if true, also assert details says verdict could not be determined
	}{
		{
			name: "pass_via_json_envelope",
			reviewText: `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"test-phase\", /pass, \"all objectives met\", 95)"]}, "surface_response": "done"}`,
			wantPass:   true,
		},
		{
			name: "fail_via_json_envelope",
			reviewText: `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"test-phase\", /fail, \"three sites unverified\", 80)"]}, "surface_response": "done"}`,
			wantPass:   false,
		},
		{
			name: "bare_atom_outside_envelope_is_not_a_verdict",
			// A bare atom outside a JSON control-packet envelope is inert:
			// only control_packet.mangle_updates entries decide.
			reviewText:     `checkpoint_verdict("test-phase", /pass, "all objectives met", 95)`,
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:           "bare_fail_atom_outside_envelope_is_not_a_verdict",
			reviewText:     `checkpoint_verdict("test-phase", /fail, "objectives not met", 80)`,
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name: "old_substring_shapes_no_longer_satisfy",
			// Exact live response that the old substring parser recorded as
			// a failure and the interim first-line parser recorded as a
			// pass. Under the structured contract prose is not a verdict:
			// no checkpoint_verdict/4 means fail closed, never pass.
			reviewText:     "**PASS - Discovery objectives met.**\n\nNo failures found in the audit.",
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name: "prose_fail_is_not_a_verdict",
			// A leading prose FAIL is equally inert without the atom.
			reviewText:     "FAIL: objectives not met, three sites unverified",
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name: "fail_closed_verdict_for_wrong_phase",
			// A well-formed atom for another phase cannot satisfy this one.
			reviewText:     `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"other-phase\", /pass, \"all objectives met\", 95)"]}, "surface_response": "done"}`,
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name: "fail_closed_malformed_verdict_token",
			// Verdict must be /pass or /fail; anything else is malformed.
			reviewText:     `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"test-phase\", /maybe, \"unsure\", 50)"]}, "surface_response": "done"}`,
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:           "fail_closed_no_verdict_anywhere",
			reviewText:     "The phase looks reasonable.",
			wantPass:       false,
			wantFailClosed: true,
		},
		{
			name:           "fail_closed_empty_response",
			reviewText:     "",
			wantPass:       false,
			wantFailClosed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			executor := &MockTaskExecutor{
				ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
					// The checkpoint spawns a "/review" task; return the
					// table's reviewText verbatim so the parser's handling
					// of that exact shape is exercised.
					return tc.reviewText, nil
				},
			}
			cr := NewCheckpointRunner(nil, executor, t.TempDir())
			phase := &Phase{Name: "test-phase"}

			passed, details, err := cr.runShardValidationCheckpoint(context.Background(), phase)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if passed != tc.wantPass {
				t.Errorf("runShardValidationCheckpoint with review %q: passed=%v, want %v (details=%q)", tc.reviewText, passed, tc.wantPass, details)
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
					t.Errorf("Run(VerifyShardValidate) with review %q: passed=%v, want %v (details=%q)", tc.reviewText, passed2, tc.wantPass, details2)
				}
				if !strings.Contains(strings.ToLower(details2), "could not be determined") {
					t.Errorf("Run fail-closed details should say verdict could not be determined; got %q", details2)
				}
			}
		})
	}
}
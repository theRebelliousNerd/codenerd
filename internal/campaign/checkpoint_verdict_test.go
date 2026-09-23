package campaign

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

func verdictFact(key, verdict, reason string, confidence int64) core.Fact {
	return core.Fact{Predicate: "checkpoint_verdict", Args: []any{key, types.MangleAtom(verdict), reason, confidence}}
}

// TestRunShardValidationCheckpoint_VerdictIsDerived drives the review
// checkpoint against the shipped kernel. The reviewer's checkpoint_verdict/4
// reaches the kernel through its control packet (the session executor asserts
// mangle_updates) or, for an executor that returns the envelope verbatim,
// through the runner; either way the outcome is checkpoint_verdict_outcome,
// derived by policy/campaign_decisions.mg, never decided in Go. Prose, bare
// atoms outside the envelope and a verdict for another phase fail closed.
func TestRunShardValidationCheckpoint_VerdictIsDerived(t *testing.T) {
	tests := []struct {
		name        string
		surface     string // what Execute returns on the live path (surface_response only)
		rawEnvelope string // when set, Execute returns this raw envelope instead
		kernelFacts []core.Fact
		minConf     int
		want        string // "pass", "fail", "inconclusive" or "undetermined"
	}{
		{name: "pass_via_kernel", surface: "done", kernelFacts: []core.Fact{verdictFact("test-phase", "/pass", "all objectives met", 95)}, want: "pass"},
		{name: "fail_via_kernel", surface: "done", kernelFacts: []core.Fact{verdictFact("test-phase", "/fail", "three sites unverified", 80)}, want: "fail"},
		{
			name:        "pass_via_raw_envelope",
			rawEnvelope: `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"test-phase\", /pass, \"all objectives met\", 95)"]}, "surface_response": "done"}`,
			want:        "pass",
		},
		{
			name:        "fail_via_raw_envelope",
			rawEnvelope: `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"test-phase\", /fail, \"three sites unverified\", 80)"]}, "surface_response": "done"}`,
			want:        "fail",
		},
		{
			// Any /fail wins: a reviewer that asserted both did not pass the
			// phase, whichever row a Go loop happened to meet first.
			name: "a_fail_row_beside_a_pass_row_fails",
			kernelFacts: []core.Fact{
				verdictFact("test-phase", "/pass", "looks fine", 95),
				verdictFact("test-phase", "/fail", "the parser is unwired", 70),
			},
			want: "fail",
		},
		{
			// A pass below campaign.checkpoint_min_confidence does not pass.
			name:        "a_low_confidence_pass_is_inconclusive",
			kernelFacts: []core.Fact{verdictFact("test-phase", "/pass", "probably fine", 30)},
			want:        "inconclusive",
		},
		{
			name:        "the_confidence_floor_is_the_config's",
			kernelFacts: []core.Fact{verdictFact("test-phase", "/pass", "mostly verified", 80)},
			minConf:     90,
			want:        "inconclusive",
		},
		{
			name:        "a_pass_at_the_floor_passes",
			kernelFacts: []core.Fact{verdictFact("test-phase", "/pass", "verified", 50)},
			want:        "pass",
		},
		{name: "bare_atom_in_surface_is_not_a_verdict", surface: `checkpoint_verdict("test-phase", /pass, "all objectives met", 95)`, want: "undetermined"},
		{name: "old_substring_shapes_no_longer_satisfy", surface: "**PASS - Discovery objectives met.**\n\nNo failures found in the audit.", want: "undetermined"},
		{name: "prose_fail_is_not_a_verdict", surface: "FAIL: objectives not met, three sites unverified", want: "undetermined"},
		{name: "verdict_for_another_phase", surface: "done", kernelFacts: []core.Fact{verdictFact("other-phase", "/pass", "all objectives met", 95)}, want: "undetermined"},
		{name: "malformed_verdict_token", surface: "done", kernelFacts: []core.Fact{verdictFact("test-phase", "/maybe", "unsure", 50)}, want: "undetermined"},
		{name: "no_verdict_anywhere", surface: "The phase looks reasonable.", want: "undetermined"},
		{name: "empty_response", surface: "", want: "undetermined"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kernel := policyKernel(t, func(c *config.CampaignConfig) {
				if tc.minConf != 0 {
					c.CheckpointMinConfidence = &tc.minConf
				}
			})
			reviewText := tc.surface
			if tc.rawEnvelope != "" {
				reviewText = tc.rawEnvelope
			}
			executor := &MockTaskExecutor{
				ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
					// The reviewer asserts its verdict during execution, after
					// the runner's retract-before-spawn.
					for _, f := range tc.kernelFacts {
						if err := kernel.Assert(f); err != nil {
							t.Fatalf("assert %v: %v", f, err)
						}
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
			if passed != (tc.want == "pass") {
				t.Fatalf("passed = %v, want %s (details=%q)", passed, tc.want, details)
			}
			wantDetail := map[string]string{
				"pass":         "Review passed",
				"fail":         "Review failed",
				"inconclusive": "Review inconclusive",
				"undetermined": "could not be determined",
			}[tc.want]
			if !strings.Contains(details, wantDetail) {
				t.Errorf("details = %q, want it to contain %q", details, wantDetail)
			}
			if rows := verdictRows(kernel, "test-phase"); len(rows) != 0 {
				t.Errorf("the settled verdict's rows stayed in the kernel: %v", rows)
			}
		})
	}
}

// Phase names are not unique; the verdict is keyed by the phase ID. A verdict
// the reviewer keyed by the phase's name does not settle it.
func TestRunShardValidationCheckpoint_TheVerdictIsKeyedByPhaseID(t *testing.T) {
	for _, tc := range []struct {
		key  string
		pass bool
	}{
		{"phase_7b_3", true},
		{"Verification", false},
	} {
		kernel := policyKernel(t, nil)
		var prompt string
		executor := &MockTaskExecutor{ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			prompt = req.Task
			_ = kernel.Assert(verdictFact(tc.key, "/pass", "verified", 95))
			return "done", nil
		}}
		cr := NewCheckpointRunner(nil, executor, t.TempDir(), kernel)
		passed, details, err := cr.runShardValidationCheckpoint(context.Background(), &Phase{ID: "/phase_7b_3", Name: "Verification"})
		if err != nil {
			t.Fatal(err)
		}
		if passed != tc.pass {
			t.Errorf("verdict keyed %q: passed = %v, want %v (%s)", tc.key, passed, tc.pass, details)
		}
		if !strings.Contains(prompt, `PhaseKey must be exactly "phase_7b_3"`) {
			t.Errorf("the reviewer was not told the phase key:\n%s", prompt)
		}
	}
}

// A method the runner cannot run is unverified, not passed; it used to return
// passed with "Unknown verification method, skipping".
func TestCheckpointRunner_AnUnknownMethodFailsClosed(t *testing.T) {
	cr := NewCheckpointRunner(nil, nil, t.TempDir(), policyKernel(t, nil))
	passed, details, err := cr.Run(context.Background(), &Phase{ID: "/phase_x", Name: "x"}, VerificationMethod("/bogus"))
	if passed || err == nil {
		t.Fatalf("Run(/bogus) = passed %v, err %v (%s); want a failure with an error", passed, err, details)
	}
}

// The policy blocks a campaign whose phase names a method the runner cannot
// run, by name, instead of letting it reach a checkpoint that cannot pass.
func TestPolicy_APhaseWithAnUnknownMethodBlocksTheCampaign(t *testing.T) {
	c := &Campaign{
		ID: "/campaign_um", Type: CampaignTypeFeature, Title: "unknown method", Status: StatusActive,
		Phases: []Phase{{
			ID: "/phase_um", CampaignID: "/campaign_um", Name: "p", Status: PhaseInProgress,
			Objectives: []PhaseObjective{{Type: ObjectiveCreate, Description: "x", VerificationMethod: VerificationMethod("/eyeball_it")}},
		}},
	}
	o := &Orchestrator{kernel: realKernelFor(t, c, true), campaign: c}
	got, err := o.derivedFor("campaign_blocked", "/campaign_um")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(got, ","), "/unverifiable_objective") {
		t.Fatalf("campaign_blocked = %v, want /unverifiable_objective", got)
	}

	c.Phases[0].Objectives[0].VerificationMethod = VerifyShardValidate
	o = &Orchestrator{kernel: realKernelFor(t, c, true), campaign: c}
	got, err = o.derivedFor("campaign_blocked", "/campaign_um")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(got, ","), "/unverifiable_objective") {
		t.Fatalf("a runnable method blocked the campaign: %v", got)
	}
}

// verification_method_known is the policy's copy of the methods
// CheckpointRunner.Run can run. The two lists must agree: a method the policy
// thinks is runnable but Run is not fails every checkpoint of that phase, and
// one Run can run but the policy does not know blocks a campaign that could
// have been verified.
func TestPolicy_TheKnownVerificationMethodsAreTheOnesRunRuns(t *testing.T) {
	fset := token.NewFileSet()
	declared := map[string]string{} // constant name -> value
	typesFile, err := parser.ParseFile(fset, "types.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(typesFile, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || vs.Type == nil {
			return true
		}
		if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "VerificationMethod" {
			return true
		}
		for i, name := range vs.Names {
			if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
				v, _ := strconv.Unquote(lit.Value)
				declared[name.Name] = v
			}
		}
		return true
	})

	runFile, err := parser.ParseFile(fset, "checkpoint.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var run []string
	for _, d := range runFile.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "Run" || fd.Recv == nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			cc, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, e := range cc.List {
				if id, ok := e.(*ast.Ident); ok {
					v, known := declared[id.Name]
					if !known {
						t.Errorf("Run switches on %s, which is not a declared VerificationMethod", id.Name)
					}
					run = append(run, v)
				}
			}
			return true
		})
	}
	slices.Sort(run)

	facts, err := policyKernel(t, nil).Query("verification_method_known")
	if err != nil {
		t.Fatal(err)
	}
	var known []string
	for _, f := range facts {
		known = append(known, types.ExtractString(f.Args[0]))
	}
	slices.Sort(known)
	if len(run) == 0 || !slices.Equal(run, known) {
		t.Fatalf("CheckpointRunner.Run runs %v; the policy knows %v", run, known)
	}
}

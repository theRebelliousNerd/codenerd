package articulation

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// buildSessionContext is the JIT compiler's fallback: it runs when compilation
// FAILED, which is exactly when the system is already degraded and least able
// to absorb a context-window error on top. Lists and lines are rendered whole.
// One payload larger than the derived block ceiling still has to come back
// under that ceiling with a visible marker, or one shard summary becomes the
// next prompt.
func TestBuildSessionContext_Bounds(t *testing.T) {
	pa := &PromptAssembler{}
	ceiling := pa.sessionContextCharCeiling()
	blob := strings.Repeat("B", ceiling+4096)

	tests := []struct {
		name string
		ctx  *types.SessionContext
	}{
		{
			name: "oversized diagnostics",
			ctx:  &types.SessionContext{CurrentDiagnostics: []string{blob}},
		},
		{
			name: "oversized findings and reflection hits",
			ctx: &types.SessionContext{
				RecentFindings: []string{blob},
				ReflectionHits: []string{blob},
			},
		},
		{
			name: "a shard that returned a whole file as its summary",
			ctx: &types.SessionContext{
				PriorShardOutputs: []types.ShardSummary{
					{ShardType: "reviewer", Task: blob, Summary: blob, Success: false},
				},
			},
		},
		{
			name: "oversized knowledge atoms and specialist hints",
			ctx: &types.SessionContext{
				KnowledgeAtoms:  []string{blob},
				SpecialistHints: []string{blob},
			},
		},
		{
			name: "oversized safety text",
			ctx: &types.SessionContext{
				BlockedActions: []string{blob},
				SafetyWarnings: []string{blob},
			},
		},
		{
			name: "everything at once",
			ctx: &types.SessionContext{
				CurrentDiagnostics: []string{blob},
				FailingTests:       []string{blob},
				RecentFindings:     []string{blob},
				ReflectionHits:     []string{blob},
				ImpactedFiles:      []string{blob},
				GitBranch:          "main",
				GitRecentCommits:   []string{blob},
				CampaignActive:     true,
				CampaignGoal:       blob,
				RecentActions:      []string{blob},
				KnowledgeAtoms:     []string{blob},
				BlockedActions:     []string{blob},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pa.buildSessionContext(&PromptContext{SessionCtx: tt.ctx})

			if len(got) > ceiling+512 {
				t.Errorf("blackboard block is %d chars, cap is %d", len(got), ceiling)
			}
			if got != "" && !types.IsClamped(got) {
				t.Error("an over-cap blackboard block must carry a visible truncation marker")
			}
		})
	}
}

// An ordinary session must pass through untouched. A bound that reshapes normal
// output is a bound that will be turned off.
func TestBuildSessionContext_OrdinarySessionIsUnchanged(t *testing.T) {
	pa := &PromptAssembler{}
	got := pa.buildSessionContext(&PromptContext{SessionCtx: &types.SessionContext{
		CurrentDiagnostics: []string{"main.go:12: undefined: foo"},
		FailingTests:       []string{"TestRouter"},
		GitBranch:          "feature/x",
	}})

	for _, want := range []string{"main.go:12: undefined: foo", "TestRouter", "feature/x"} {
		if !strings.Contains(got, want) {
			t.Errorf("ordinary session lost %q", want)
		}
	}
	if types.IsClamped(got) {
		t.Error("ordinary session must not be truncated")
	}
}

// The old per-line cap was 500 characters. A diagnostic longer than that is
// still the diagnostic: it is kept whole while it fits the block ceiling, and
// only the ceiling clamps it.
func TestBuildSessionContext_LinePastTheOldCapIsKept(t *testing.T) {
	pa := &PromptAssembler{}
	line := strings.Repeat("p", 600)
	got := pa.buildSessionContext(&PromptContext{SessionCtx: &types.SessionContext{
		CurrentDiagnostics: []string{line},
	}})
	if !strings.Contains(got, line) {
		t.Fatalf("a %d-char diagnostic was cut; the old 500-char line cap is gone", len(line))
	}
	if types.IsClamped(got) {
		t.Fatal("a line that fits the block ceiling was truncated")
	}
}

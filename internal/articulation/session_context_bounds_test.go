package articulation

import (
	"strings"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// buildSessionContext is the JIT compiler's fallback: it runs when compilation
// FAILED, which is exactly when the system is already degraded and least able
// to absorb a context-window error on top. Every list was capped at 20 elements
// and no element's length was capped at all, so one shard returning a 4 MB
// summary — a reviewer dumping a file, a tester pasting full `go test` output —
// put 4 MB into the next prompt.
func TestBuildSessionContext_Bounds(t *testing.T) {
	blob := strings.Repeat("B", 200_000)
	many := func(n int, s string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = s
		}
		return out
	}

	tests := []struct {
		name string
		ctx  *types.SessionContext
	}{
		{
			name: "oversized diagnostics",
			ctx:  &types.SessionContext{CurrentDiagnostics: many(500, blob)},
		},
		{
			name: "oversized findings and reflection hits",
			ctx: &types.SessionContext{
				RecentFindings: many(500, blob),
				ReflectionHits: many(500, blob),
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
				KnowledgeAtoms:  many(500, blob),
				SpecialistHints: many(500, blob),
			},
		},
		{
			name: "oversized safety text",
			ctx: &types.SessionContext{
				BlockedActions: many(500, blob),
				SafetyWarnings: many(500, blob),
			},
		},
		{
			name: "everything at once",
			ctx: &types.SessionContext{
				CurrentDiagnostics: many(500, blob),
				FailingTests:       many(500, blob),
				RecentFindings:     many(500, blob),
				ReflectionHits:     many(500, blob),
				ImpactedFiles:      many(500, blob),
				GitBranch:          "main",
				GitRecentCommits:   many(500, blob),
				CampaignActive:     true,
				CampaignGoal:       blob,
				RecentActions:      many(500, blob),
				KnowledgeAtoms:     many(500, blob),
				BlockedActions:     many(500, blob),
			},
		},
	}

	pa := &PromptAssembler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pa.buildSessionContext(&PromptContext{SessionCtx: tt.ctx})

			if len(got) > maxSessionContextChars+512 {
				t.Errorf("blackboard block is %d chars, cap is %d", len(got), maxSessionContextChars)
			}
			if got != "" && !prompt.IsClamped(got) {
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
	if prompt.IsClamped(got) {
		t.Error("ordinary session must not be truncated")
	}
}

func TestSessionContextLine(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantMarker bool
	}{
		{name: "a real diagnostic passes through", in: "main.go:12: undefined: foo"},
		{name: "a pasted payload is clamped", in: strings.Repeat("p", 100_000), wantMarker: true},
		{name: "empty stays empty", in: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sessionContextLine(tt.in)
			if prompt.IsClamped(got) != tt.wantMarker {
				t.Fatalf("IsClamped = %v, want %v", prompt.IsClamped(got), tt.wantMarker)
			}
			if !tt.wantMarker && got != tt.in {
				t.Errorf("in-bounds line was modified: %q", got)
			}
		})
	}
}

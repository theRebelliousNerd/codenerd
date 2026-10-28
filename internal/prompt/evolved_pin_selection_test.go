package prompt

import "testing"

func pinnedContext(provider, model, shard string) *CompilationContext {
	cc := NewCompilationContext()
	cc.Provider, cc.Model, cc.ShardType = provider, model, shard
	return cc
}

// An evolved atom's model pin and role pin are a conjunction: an atom pinned to
// (opus-x, coder) is served to neither (opus-y, coder) nor (opus-x, tester).
// This holds for promoted atoms and for trial-admitted quarantined ones.
func TestEvolvedAtomsSelectedOnModelAndRoleTogether(t *testing.T) {
	nerdDir := t.TempDir()
	atom := NewPromptAtom("evolved/pinned", CategoryMethodology, "pinned guidance")
	atom.Providers = []string{"anthropic"}
	atom.Models = []string{"claude_opus_4_20260501"}
	atom.ShardTypes = []string{"coder"}
	atom.NormalizeSelectors()
	writeEvolvedCandidate(t, nerdDir, "promoted", atom)

	trial := NewPromptAtom("evolved/pinned-trial", CategoryMethodology, "trial guidance")
	trial.Providers = []string{"anthropic"}
	trial.Models = []string{"claude_opus_4_20260501"}
	trial.ShardTypes = []string{"coder"}
	trial.NormalizeSelectors()
	writeEvolvedCandidate(t, nerdDir, "pending", trial)

	mgr := NewEvolvedAtomManager(nerdDir)
	admitted := map[string]string{"evolved/pinned-trial": EvolvedCandidateHash(mgr.Get("evolved/pinned-trial"))}

	tests := []struct {
		name string
		cc   *CompilationContext
		want int
	}{
		{"same model, same role", pinnedContext("anthropic", "claude-opus-4-20260501", "/coder"), 2},
		{"other snapshot, same role", pinnedContext("anthropic", "claude-opus-4-20260901", "/coder"), 0},
		{"same model, other role", pinnedContext("anthropic", "claude-opus-4-20260501", "/tester"), 0},
		{"other model and role", pinnedContext("anthropic", "claude-opus-4-20260901", "/tester"), 0},
		{"other provider", pinnedContext("openai", "claude-opus-4-20260501", "/coder"), 0},
		{"unknown model", pinnedContext("anthropic", "", "/coder"), 0},
		{"unknown role", pinnedContext("anthropic", "claude-opus-4-20260501", ""), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(mgr.GetMatching(tc.cc, admitted)); got != tc.want {
				t.Errorf("served %d evolved atoms, want %d", got, tc.want)
			}
		})
	}
}

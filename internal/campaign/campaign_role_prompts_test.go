package campaign_test

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	nerdsystem "codenerd/internal/system"
)

// newCampaignCompiler is the JIT compiler a booted session builds: a real
// kernel and the embedded atom corpus the binary ships.
func newCampaignCompiler(t *testing.T) (*core.RealKernel, *prompt.JITPromptCompiler, *prompt.EmbeddedCorpus) {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(nerdsystem.NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(embedded),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	return kernel, compiler, embedded
}

// firstContentLine is an atom's first non-blank content line: present in a
// compiled prompt exactly when the atom was compiled in.
func firstContentLine(atom *prompt.PromptAtom) string {
	for _, line := range strings.Split(atom.Content, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// Every campaign role's prompt is compiled from its own atoms. Each mandatory
// atom of the role's family that declares the role's phase reaches the
// compile; the worker substrate a campaign role has no use for (intent
// protocols, the refactoring discipline, OODA, tool use) and any Piggyback
// envelope do not. Measured 2026-10-28 before the intent and the phases were
// aligned: the extractor and taxonomy compiles carried none of their role's
// atoms, and every role carried eight worker intent atoms.
func TestCampaignRolePrompts_AreCompiledFromTheirAtoms(t *testing.T) {
	t.Parallel()
	kernel, compiler, corpus := newCampaignCompiler(t)
	assembler, err := articulation.NewPromptAssemblerWithJIT(kernel, compiler)
	if err != nil {
		t.Fatalf("NewPromptAssemblerWithJIT: %v", err)
	}
	provider := campaign.NewJITPromptProvider(assembler)

	for _, role := range campaign.CampaignRoles() {
		t.Run(string(role), func(t *testing.T) {
			got, err := provider.GetPrompt(context.Background(), role, "/campaign_roles", nil)
			if err != nil {
				t.Fatalf("GetPrompt(%s): %v", role, err)
			}
			family := campaign.CampaignRoleAtomFamily(role) + "/"
			phase := strings.TrimPrefix(campaign.GetCampaignPhaseForRole(role), "/")
			own := 0
			for _, atom := range corpus.All() {
				if !atom.IsMandatory || !strings.HasPrefix(atom.ID, family) || !containsTag(atom.CampaignPhases, phase) {
					continue
				}
				own++
				if line := firstContentLine(atom); !strings.Contains(got, line) {
					t.Errorf("the %s prompt lacks its atom %s (%q)", role, atom.ID, line)
				}
			}
			if own == 0 {
				t.Fatalf("no mandatory atom under %s declares campaign phase %q; the role compiles from nothing of its own", family, phase)
			}
			for _, leaked := range []string{
				"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE",   // protocol/piggyback/envelope
				"OODA LOOP: THE DECISION CYCLE",         // methodology/ooda/core
				"REVIEW INTENT PROTOCOL",                // intent/review/core
				"HALLUCINATION: BREAKING API STABILITY", // hallucination/refactoring/api_stability
				"## Tool Awareness & Self-Extension",    // capability/tool_thinking
			} {
				if strings.Contains(got, leaked) {
					t.Errorf("the %s prompt carries %q, which a campaign role's parsed reply cannot use", role, leaked)
				}
			}
		})
	}
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.TrimPrefix(t, "/") == tag {
			return true
		}
	}
	return false
}

// The checkpoint verdict contract reaches the reviewer's and the nemesis's
// turn only through the checkpoint prompt phase: the task carries data, the
// atom carries the checkpoint_verdict/4 reply. The same turn without the
// phase carries no contract.
func TestCheckpointVerdictContracts_ReachOnlyTheCheckpointTurn(t *testing.T) {
	t.Parallel()
	_, compiler, _ := newCampaignCompiler(t)
	for _, tc := range []struct {
		shard, intent, phase, contract string
	}{
		{"/reviewer", "/review", campaign.CheckpointReviewPromptPhase, `checkpoint_verdict(\"my-phase\", /pass, \"all objectives met\", 95).`},
		{"/nemesis", "/nemesis", campaign.CheckpointNemesisPromptPhase, `checkpoint_verdict(\"my-phase\", /pass, \"no weaknesses found\", 95).`},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			compile := func(phase string) string {
				cc := prompt.NewCompilationContext()
				cc.ShardType = tc.shard
				cc.OperationalMode = "/active"
				cc.IntentVerb = tc.intent
				cc.CampaignPhase = phase
				res, err := compiler.Compile(context.Background(), cc)
				if err != nil {
					t.Fatalf("Compile(%s, phase %q): %v", tc.shard, phase, err)
				}
				return res.Prompt
			}
			got := compile(tc.phase)
			for _, want := range []string{
				tc.contract,
				"The atom must end with a period; it is asserted into the kernel as a fact.",
				"Free-text PASS/FAIL is not accepted; only checkpoint_verdict/4 decides.",
				"PhaseKey must be exactly the verdict key the task gives.",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("the %s checkpoint turn lacks %q", tc.shard, want)
				}
			}
			if without := compile(""); strings.Contains(without, "Free-text PASS/FAIL is not accepted") {
				t.Errorf("a %s turn outside a checkpoint carries the verdict contract", tc.shard)
			}
		})
	}
}

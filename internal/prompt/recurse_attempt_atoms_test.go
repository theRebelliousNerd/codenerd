package prompt

import (
	"strings"
	"testing"
)

// A recurse attempt's instructions are phase-gated atoms. The phase literals
// are campaign.RecurseFixPromptPhase and campaign.RecurseImprovePromptPhase;
// this package must not import campaign (PromptProvider exists so that cycle
// stays closed), so the two sides are pinned by the shared spelling.
func TestRecurseAttemptAtoms_PhaseSelectsFixOrImprove(t *testing.T) {
	c, _ := newEmbeddedCorpusCompiler(t)

	fixCC := coderTurnContext(t, "/fix", "store/store.go", "/go", nil)
	fixCC.CampaignPhase = "/recurse_fix"
	fixIDs, fixResult := compiledAtoms(t, c, fixCC)
	if _, ok := fixIDs["campaign/recurse/fix"]; !ok {
		t.Fatal("a /recurse_fix compile omitted campaign/recurse/fix")
	}
	if _, ok := fixIDs["campaign/recurse/improve"]; ok {
		t.Fatal("a /recurse_fix compile included campaign/recurse/improve")
	}
	if !strings.Contains(fixResult.Prompt, "Do not weaken, skip or delete") {
		t.Fatal("fix prompt missing the fix atom")
	}
	if strings.Contains(fixResult.Prompt, "no test pins yet") {
		t.Fatal("fix prompt contains the improve atom")
	}

	improveCC := coderTurnContext(t, "/fix", "store/store.go", "/go", nil)
	improveCC.CampaignPhase = "/recurse_improve"
	improveIDs, improveResult := compiledAtoms(t, c, improveCC)
	if _, ok := improveIDs["campaign/recurse/improve"]; !ok {
		t.Fatal("a /recurse_improve compile omitted campaign/recurse/improve")
	}
	if _, ok := improveIDs["campaign/recurse/fix"]; ok {
		t.Fatal("a /recurse_improve compile included campaign/recurse/fix")
	}
	if !strings.Contains(improveResult.Prompt, "no test pins yet") {
		t.Fatal("improve prompt missing the improve atom")
	}
	if strings.Contains(improveResult.Prompt, "Do not weaken, skip or delete") {
		t.Fatal("improve prompt contains the fix atom")
	}

	unset := coderTurnContext(t, "/fix", "store/store.go", "/go", nil)
	unsetIDs, unsetResult := compiledAtoms(t, c, unset)
	if _, ok := unsetIDs["campaign/recurse/fix"]; ok {
		t.Fatal("a compile with no campaign phase included campaign/recurse/fix")
	}
	if _, ok := unsetIDs["campaign/recurse/improve"]; ok {
		t.Fatal("a compile with no campaign phase included campaign/recurse/improve")
	}
	if strings.Contains(unsetResult.Prompt, "Do not weaken, skip or delete") || strings.Contains(unsetResult.Prompt, "no test pins yet") {
		t.Fatal("a compile with no campaign phase carried a recurse attempt atom")
	}
}

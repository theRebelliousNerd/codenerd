package prompt

import (
	"testing"
)

// The run_check atom teaches the turn that carries a campaign acceptance
// check to run it and keep going until it passes. It is gated on the tool:
// without run_check in the effective catalog the guidance must be omitted,
// never widened into.
func TestEmbeddedCorpus_RunCheckAtomIsGatedOnItsTool(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	var atom *PromptAtom
	for _, a := range corpus.All() {
		if a != nil && a.ID == "capability/run_check" {
			atom = a
			break
		}
	}
	if atom == nil {
		t.Fatal("capability/run_check is not in the embedded corpus")
	}
	if len(atom.RequiresTools) != 1 || atom.RequiresTools[0] != "run_check" {
		t.Fatalf("RequiresTools = %v, want [run_check]", atom.RequiresTools)
	}
	if atomToolSatisfied(atom, availableToolSet(NewCompilationContext())) {
		t.Fatal("an empty catalog must block the run_check atom")
	}
	offered := availableToolSet(&CompilationContext{AvailableTools: []string{"read_file", "run_check"}})
	if !atomToolSatisfied(atom, offered) {
		t.Fatal("a catalog offering run_check must admit the run_check atom")
	}
}

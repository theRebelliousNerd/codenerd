package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// trialKernel answers prompt_trial_admitted from a fixed table.
type trialKernel struct {
	admitted map[string]string
}

func (k trialKernel) Query(predicate string) ([]Fact, error) {
	if predicate != PromptTrialAdmittedPredicate {
		return nil, nil
	}
	var out []Fact
	for id, version := range k.admitted {
		out = append(out, Fact{Predicate: predicate, Args: []any{id, version}})
	}
	return out, nil
}

func (trialKernel) AssertBatch([]any) error { return nil }

func writeEvolvedCandidate(t *testing.T, nerdDir, state string, atom *PromptAtom) {
	t.Helper()
	dir := filepath.Join(nerdDir, "prompts", "evolved", state)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(struct {
		Atom   *PromptAtom `yaml:"atom"`
		Source string      `yaml:"source"`
	}{Atom: atom, Source: "failure_analysis"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(atom.ID)+".yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func coderContext(kernel KernelQuerier) *CompilationContext {
	cc := NewCompilationContext()
	cc.ShardType = "/coder"
	cc.ShardID = "coder"
	if kernel != nil {
		cc.Kernel = kernel
	}
	return cc
}

// GAP-PE-01: a pending (quarantined) candidate never reaches a normal
// compile; a promoted one does.
func TestEvolvedAtoms_QuarantinedNeverServedWithoutTrial(t *testing.T) {
	nerdDir := t.TempDir()
	quarantined := NewPromptAtom("evolved/quarantined", CategoryMethodology, "unreviewed guidance")
	quarantined.ShardTypes = []string{"/coder"}
	quarantined.IsMandatory = true
	promoted := NewPromptAtom("evolved/promoted", CategoryMethodology, "reviewed guidance")
	promoted.ShardTypes = []string{"/coder"}
	writeEvolvedCandidate(t, nerdDir, "pending", quarantined)
	writeEvolvedCandidate(t, nerdDir, "promoted", promoted)

	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatal(err)
	}
	defer compiler.Close()
	mgr := NewEvolvedAtomManager(nerdDir)
	compiler.RegisterEvolvedAtomManager(mgr)

	if state, _ := mgr.State("evolved/quarantined"); state != EvolvedQuarantined {
		t.Fatalf("pending file loaded as %q", state)
	}
	served := compiler.collectEvolvedAtoms(coderContext(nil))
	if len(served) != 1 || served[0].ID != "evolved/promoted" {
		t.Fatalf("normal compile served %v, want only the promoted candidate", atomIDs(served))
	}

	// An envelope for a different version admits nothing.
	stale := trialKernel{admitted: map[string]string{"evolved/quarantined": "not-this-version"}}
	if got := compiler.collectEvolvedAtoms(coderContext(stale)); len(got) != 1 {
		t.Fatalf("version mismatch admitted the candidate: %v", atomIDs(got))
	}

	// The kernel-admitted exact version is served, as an optional atom.
	version := EvolvedCandidateHash(mgr.Get("evolved/quarantined"))
	admitted := trialKernel{admitted: map[string]string{"evolved/quarantined": version}}
	got := compiler.collectEvolvedAtoms(coderContext(admitted))
	if len(got) != 2 {
		t.Fatalf("trial compile served %v, want both", atomIDs(got))
	}
	for _, atom := range got {
		if atom.ID == "evolved/quarantined" && atom.IsMandatory {
			t.Error("a trial candidate entered the mandatory skeleton")
		}
	}
	if !mgr.Get("evolved/quarantined").IsMandatory {
		t.Error("serving a trial copy mutated the loaded candidate")
	}
}

// The version identity covers content and selectors, not derived fields.
func TestEvolvedCandidateHash_PinsContentAndScope(t *testing.T) {
	base := NewPromptAtom("evolved/x", CategoryMethodology, "body")
	base.ShardTypes = []string{"/coder"}
	h := EvolvedCandidateHash(base)

	derived := *base
	derived.TokenCount = 999
	derived.ContentHash = "other"
	derived.ShardTypes = []string{"coder"} // normalized spelling of the same selector
	if EvolvedCandidateHash(&derived) != h {
		t.Error("derived fields or selector spelling changed the version")
	}
	edited := *base
	edited.Content = "body, edited"
	if EvolvedCandidateHash(&edited) == h {
		t.Error("an edited body kept the approved version")
	}
	widened := *base
	widened.ShardTypes = []string{"/coder", "/reviewer"}
	if EvolvedCandidateHash(&widened) == h {
		t.Error("a widened scope kept the approved version")
	}
}

func atomIDs(atoms []*PromptAtom) []string {
	ids := make([]string, 0, len(atoms))
	for _, a := range atoms {
		ids = append(ids, a.ID)
	}
	return ids
}

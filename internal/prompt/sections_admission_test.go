package prompt

import (
	"context"
	"testing"

	"codenerd/internal/core"
)

// Section expansion used to judge admission with MatchesContext, a second
// matcher beside the kernel (Docs/audits/jit-dependency-closed-budgeting.md,
// post-implementation refinement, findings A-C). Each case compiles a small
// corpus through the production pipeline on a real kernel.
func compileSections(t *testing.T, atoms []*PromptAtom, cc *CompilationContext) map[string]bool {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(NewEmbeddedCorpus(atoms)), WithKernel(&realKernelQuerier{k: k}))
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	result, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	got := map[string]bool{}
	for _, a := range result.IncludedAtoms {
		got[a.ID] = true
	}
	return got
}

func sectionCorpus() (head, section *PromptAtom, atoms []*PromptAtom) {
	identity := NewPromptAtom("identity/core", CategoryIdentity, "You are the agent.")
	identity.IsMandatory = true
	head = NewPromptAtom("protocol/h", CategoryProtocol, "Follow the protocol.")
	head.IsMandatory = true
	section = NewPromptAtom("protocol/h/s1", CategoryProtocol, "The protocol's first step in detail.")
	section.SectionOf, section.SectionOrder, section.DependsOn = head.ID, 1, []string{head.ID}
	return head, section, []*PromptAtom{identity, head, section}
}

func TestExpandSections_KernelAdmissionGovernsSections(t *testing.T) {
	t.Run("control: an admitted section ships with its head", func(t *testing.T) {
		head, section, atoms := sectionCorpus()
		got := compileSections(t, atoms, NewCompilationContextWithBudget(100000))
		if !got[head.ID] || !got[section.ID] {
			t.Fatalf("included %v, want the head and its section", got)
		}
	})

	t.Run("A: a section needing an unavailable tool is not added", func(t *testing.T) {
		head, section, atoms := sectionCorpus()
		section.RequiresTools = []string{"run_check"}
		cc := NewCompilationContextWithBudget(100000)
		got := compileSections(t, atoms, cc)
		if !got[head.ID] || got[section.ID] {
			t.Fatalf("included %v, want the head without the tool-gated section", got)
		}
		cc.AvailableTools = []string{"run_check"}
		if got := compileSections(t, atoms, cc); !got[section.ID] {
			t.Fatalf("included %v, want the section once its tool is available", got)
		}
	})

	t.Run("B: a section a mandatory atom vetoes is not added", func(t *testing.T) {
		head, section, atoms := sectionCorpus()
		veto := NewPromptAtom("safety/veto", CategorySafety, "Never follow that step.")
		veto.IsMandatory = true
		veto.ConflictsWith = []string{section.ID}
		got := compileSections(t, append(atoms, veto), NewCompilationContextWithBudget(100000))
		if !got[head.ID] || !got[veto.ID] || got[section.ID] {
			t.Fatalf("included %v, want the head and the veto without the vetoed section", got)
		}
	})

	t.Run("C: a missing intent does not drop a section the kernel admits", func(t *testing.T) {
		head, section, atoms := sectionCorpus()
		head.IntentVerbs = []string{"/research"}
		section.IntentVerbs = []string{"/research"}
		got := compileSections(t, atoms, NewCompilationContextWithBudget(100000))
		if !got[head.ID] || !got[section.ID] {
			t.Fatalf("included %v, want the head and its section (intent is situational: no intent does not block)", got)
		}
	})
}

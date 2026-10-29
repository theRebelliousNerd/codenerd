package prompt

import (
	"errors"
	"testing"
)

func sectionAtom(id, head string, order int, mandatory bool) *PromptAtom {
	a := &PromptAtom{ID: id, Category: CategoryProtocol, Content: id, IsMandatory: mandatory, SectionOf: head, SectionOrder: order}
	if head != "" {
		a.DependsOn = []string{head}
	}
	return a
}

func admitAll(*PromptAtom) bool { return true }

func ids(scored []*ScoredAtom) map[string]float64 {
	out := map[string]float64{}
	for _, s := range scored {
		out[s.Atom.ID] = s.Combined
	}
	return out
}

func TestExpandSections_HeadBringsOrderedSections(t *testing.T) {
	head := sectionAtom("h", "", 0, false)
	s1, s2 := sectionAtom("h/a", "h", 1, false), sectionAtom("h/b", "h", 2, false)
	other := sectionAtom("x", "", 0, false)
	got := ids(expandSections([]*ScoredAtom{{Atom: head, Combined: 0.8}, {Atom: other, Combined: 0.5}}, []*PromptAtom{head, s1, s2, other}, admitAll))
	if len(got) != 4 {
		t.Fatalf("selected %v; want the head, both sections and x", got)
	}
	if !(got["h"] > got["h/a"] && got["h/a"] > got["h/b"] && got["h/b"] > got["x"]) {
		t.Fatalf("scores %v; want head > section 1 > section 2 > the next atom", got)
	}
}

func TestExpandSections_LoneSectionBringsItsHeadOrGoes(t *testing.T) {
	head := sectionAtom("h", "", 0, false)
	sec := sectionAtom("h/a", "h", 1, false)
	got := ids(expandSections([]*ScoredAtom{{Atom: sec, Combined: 0.6}}, []*PromptAtom{head, sec}, admitAll))
	if _, ok := got["h"]; !ok {
		t.Fatalf("selected %v; a section selected alone must bring its head", got)
	}
	got = ids(expandSections([]*ScoredAtom{{Atom: sec, Combined: 0.6}}, []*PromptAtom{sec}, admitAll))
	if len(got) != 0 {
		t.Fatalf("selected %v; a section whose head is not a candidate must go", got)
	}
}

func TestCloseDependencies_CascadesAndRefusesMandatoryLoss(t *testing.T) {
	head := sectionAtom("h", "", 0, false)
	s1 := sectionAtom("h/a", "h", 1, false)
	dep := &PromptAtom{ID: "d", DependsOn: []string{"h/a"}}
	kept, dropped, err := closeDependencies([]*OrderedAtom{{Atom: s1}, {Atom: dep}, {Atom: &PromptAtom{ID: "z"}}})
	if err != nil || len(kept) != 1 || kept[0].Atom.ID != "z" || len(dropped) != 2 {
		t.Fatalf("kept %v dropped %v err %v; want only z, the section and its dependent removed", kept, dropped, err)
	}
	if kept, _, err := closeDependencies([]*OrderedAtom{{Atom: head}, {Atom: s1}}); err != nil || len(kept) != 2 {
		t.Fatalf("a closed set changed: %v %v", kept, err)
	}
	mand := sectionAtom("m", "h", 1, true)
	_, _, err = closeDependencies([]*OrderedAtom{{Atom: mand}})
	var infeasible errInfeasibleClosure
	if !errors.As(err, &infeasible) || infeasible.Atom != "m" {
		t.Fatalf("err = %v; want the mandatory atom's lost prerequisite reported", err)
	}
}

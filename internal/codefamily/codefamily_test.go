package codefamily

import (
	"strings"
	"testing"
)

func TestParse_ValidAndProjected(t *testing.T) {
	fams, err := Parse([]byte(`{"schema_version":1,"families":[{"family_id":"retry","members":["fn:a.Retry","fn:b.Retry"],"shared_requirements":["honor cancellation"]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	facts := Facts(fams)
	if len(facts) != 4 || facts[0].Predicate != "code_family" || facts[3].Predicate != "code_family_requirement" {
		t.Fatalf("facts = %v", facts)
	}
}

func TestParse_RefusesUnsoundContracts(t *testing.T) {
	for name, doc := range map[string]string{
		"unknown field":    `{"schema_version":1,"families":[],"extra":1}`,
		"wrong version":    `{"schema_version":2,"families":[]}`,
		"one member":       `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X"],"shared_requirements":["r"]}]}`,
		"no requirement":   `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X","fn:b.X"]}]}`,
		"duplicate id":     `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X","fn:b.X"],"shared_requirements":["r"]},{"family_id":"f","members":["fn:c.X","fn:d.X"],"shared_requirements":["r"]}]}`,
		"duplicate member": `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X","fn:a.X"],"shared_requirements":["r"]}]}`,
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), FileName) {
			t.Errorf("%s: error %v does not name the file", name, err)
		}
	}
}

func TestLoad_MissingFileIsNoFamilies(t *testing.T) {
	fams, err := Load(t.TempDir())
	if err != nil || fams != nil {
		t.Fatalf("Load = %v, %v; want no families and no error", fams, err)
	}
}

func TestResolve_NamesMissingAndAmbiguousMembersOnly(t *testing.T) {
	families := []Family{{
		ID:                 "retry",
		Members:            []string{"fn:a.Retry", "fn:b.Gone", "fn:c.Dup", "py:mod.retry"},
		SharedRequirements: []string{"bounded"},
	}}
	refs := map[string][]string{
		"fn:a.Retry": {"a/retry.go"},
		"fn:c.Dup":   {"c1/dup.go", "c2/dup.go"},
	}
	got := Resolve(families, refs)
	if len(got) != 2 || got[0].Ref != "fn:b.Gone" || got[1].Ref != "fn:c.Dup" {
		t.Fatalf("Resolve = %+v; want the missing and the ambiguous fn: member, not the resolved one or the non-Go ref", got)
	}
	if !strings.Contains(got[1].Why, "2 files") {
		t.Errorf("ambiguity reason %q", got[1].Why)
	}
	facts := UnresolvedFacts(got)
	if len(facts) != 2 || facts[0].Predicate != "code_family_member_unresolved" {
		t.Errorf("facts %+v", facts)
	}
}

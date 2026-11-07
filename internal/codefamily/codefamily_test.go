package codefamily

import (
	"os"
	"path/filepath"
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

func TestParse_RefusesTrailingDataAndMalformed(t *testing.T) {
	base := `{"schema_version":1,"families":[]}`
	for name, doc := range map[string]string{
		"trailing object":  base + ` {}`,
		"trailing garbage": base + ` garbage`,
		"invalid json":     `{"schema_version":`,
		"empty":            ``,
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), FileName) {
			t.Errorf("%s: error %v does not name the file", name, err)
		}
	}
	if _, err := Parse([]byte(base + " \n")); err != nil {
		t.Errorf("trailing whitespace should be accepted, got %v", err)
	}
}

func TestParse_AcceptsEmptyFamilies(t *testing.T) {
	fams, err := Parse([]byte(`{"schema_version":1,"families":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(fams) != 0 {
		t.Fatalf("families = %+v; want empty", fams)
	}
	if facts := Facts(fams); len(facts) != 0 {
		t.Fatalf("facts = %v; want none", facts)
	}
}

func TestValidate_RejectsBlankMembersAndRequirements(t *testing.T) {
	for name, fams := range map[string][]Family{
		"empty id":               {{ID: "", Members: []string{"fn:a.X", "fn:b.X"}, SharedRequirements: []string{"r"}}},
		"whitespace id":          {{ID: "   ", Members: []string{"fn:a.X", "fn:b.X"}, SharedRequirements: []string{"r"}}},
		"empty member":           {{ID: "f", Members: []string{"fn:a.X", ""}, SharedRequirements: []string{"r"}}},
		"whitespace member":      {{ID: "f", Members: []string{"fn:a.X", "   "}, SharedRequirements: []string{"r"}}},
		"empty requirement":      {{ID: "f", Members: []string{"fn:a.X", "fn:b.X"}, SharedRequirements: []string{""}}},
		"whitespace requirement": {{ID: "f", Members: []string{"fn:a.X", "fn:b.X"}, SharedRequirements: []string{"  "}}},
		"no members":             {{ID: "f", Members: nil, SharedRequirements: []string{"r"}}},
		"duplicate after trim":   {{ID: "f", Members: []string{" fn:a.X ", "fn:a.X"}, SharedRequirements: []string{"r"}}},
	} {
		if err := Validate(fams); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), FileName) {
			t.Errorf("%s: error %v does not name the file", name, err)
		}
	}
}

func TestValidate_SortsProblemsDeterministically(t *testing.T) {
	fams := []Family{
		{ID: "b", Members: []string{"fn:b.X", "fn:b.Y"}},
		{ID: "a", Members: []string{"fn:a.X", "fn:a.Y"}},
	}
	err := Validate(fams)
	if err == nil {
		t.Fatal("accepted families with no requirements")
	}
	msg := err.Error()
	ia := strings.Index(msg, `"a"`)
	ib := strings.Index(msg, `"b"`)
	if ia < 0 || ib < 0 || ia > ib {
		t.Errorf("problems not sorted: %v", err)
	}
}

func TestValidate_AcceptsNilAndEmpty(t *testing.T) {
	if err := Validate(nil); err != nil {
		t.Errorf("Validate(nil) = %v; want nil", err)
	}
	if err := Validate([]Family{}); err != nil {
		t.Errorf("Validate(empty) = %v; want nil", err)
	}
}

func TestLoad_RoundTripsValidFile(t *testing.T) {
	dir := t.TempDir()
	doc := `{"schema_version":1,"families":[{"family_id":"retry","members":["fn:a.Retry","fn:b.Retry"],"shared_requirements":["bounded"],"allowed_differences":["latency budget"]}]}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(doc), 0644); err != nil {
		t.Fatal(err)
	}
	fams, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fams) != 1 || fams[0].ID != "retry" {
		t.Fatalf("Load = %+v; want one retry family", fams)
	}
	if len(fams[0].AllowedDifferences) != 1 || fams[0].AllowedDifferences[0] != "latency budget" {
		t.Errorf("AllowedDifferences = %v; want documented difference preserved", fams[0].AllowedDifferences)
	}
	bad := `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X"],"shared_requirements":["r"]}]}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("Load accepted an unsound contract")
	} else if !strings.Contains(err.Error(), FileName) {
		t.Errorf("Load error %v does not name the file", err)
	}
}

func TestFacts_TrimsWhitespaceAndIgnoresAllowedDifferences(t *testing.T) {
	spaced := []Family{{ID: " retry ", Members: []string{" fn:a.Retry ", "fn:b.Retry "}, SharedRequirements: []string{" bounded "}, AllowedDifferences: []string{"varies"}}}
	trimmed := []Family{{ID: "retry", Members: []string{"fn:a.Retry", "fn:b.Retry"}, SharedRequirements: []string{"bounded"}}}
	got := Facts(spaced)
	want := Facts(trimmed)
	if len(got) != 4 || len(want) != 4 {
		t.Fatalf("facts = %v, want 4 rows each", got)
	}
	for i := range got {
		if got[i].Predicate != want[i].Predicate || got[i].String() != want[i].String() {
			t.Fatalf("row %d = %v, want %v (whitespace must be trimmed)", i, got[i], want[i])
		}
	}
	for _, f := range got {
		if strings.Contains(f.String(), "varies") {
			t.Errorf("fact %v projects AllowedDifferences, which are documented not enforced", f)
		}
	}
	if got := Facts(nil); len(got) != 0 {
		t.Errorf("Facts(nil) = %v; want none", got)
	}
}

func TestResolve_SortsAcrossFamiliesAndTrims(t *testing.T) {
	families := []Family{
		{ID: "b", Members: []string{" fn:gone.One "}, SharedRequirements: []string{"r"}},
		{ID: "a", Members: []string{"fn:gone.Two", "fn:ok.One"}, SharedRequirements: []string{"r"}},
	}
	refs := map[string][]string{"fn:ok.One": {"ok.go"}}
	got := Resolve(families, refs)
	if len(got) != 2 || got[0].Family != "a" || got[0].Ref != "fn:gone.Two" || got[1].Family != "b" || got[1].Ref != "fn:gone.One" {
		t.Fatalf("Resolve = %+v; want sorted trimmed missing refs", got)
	}
	for _, u := range got {
		if u.Why == "" || !strings.Contains(u.Why, "names no") {
			t.Errorf("Why = %q; want missing-function reason", u.Why)
		}
	}
	ok := Resolve([]Family{{ID: "x", Members: []string{"fn:ok.One"}, SharedRequirements: []string{"r"}}}, refs)
	if len(ok) != 0 {
		t.Errorf("single-file fn: member should resolve, got %+v", ok)
	}
	if got := Resolve([]Family{{ID: "x", Members: []string{"fn:gone.Solo"}, SharedRequirements: []string{"r"}}}, nil); len(got) != 1 {
		t.Errorf("nil refs should report missing, got %+v", got)
	}
}

func TestUnresolvedFacts_EmptyAndProjectsArgs(t *testing.T) {
	if got := UnresolvedFacts(nil); len(got) != 0 {
		t.Errorf("UnresolvedFacts(nil) = %v; want none", got)
	}
	if got := UnresolvedFacts([]Unresolved{}); len(got) != 0 {
		t.Errorf("UnresolvedFacts(empty) = %v; want none", got)
	}
	one := []Unresolved{{Family: "retry", Ref: "fn:b.Gone", Why: "names no Go function or method in the workspace"}}
	facts := UnresolvedFacts(one)
	if len(facts) != 1 || facts[0].Predicate != "code_family_member_unresolved" {
		t.Fatalf("facts = %+v", facts)
	}
	if len(facts[0].Args) != 3 {
		t.Fatalf("args = %v; want family, ref, reason", facts[0].Args)
	}
	s := facts[0].String()
	for _, want := range []string{"retry", "fn:b.Gone", "names no"} {
		if !strings.Contains(s, want) {
			t.Errorf("fact %v does not contain %q", facts[0], want)
		}
	}
}

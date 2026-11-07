package specs

import (
	"strings"
	"testing"
)

const goodSpec = `
spec_requirement(/eval_exists, "Kernel.Eval exists and is documented").
requires(/eval_exists, "internal/core.Kernel.Eval", /exists).
requires(/eval_exists, "internal/core.Kernel.Eval", /has_doc_comment).
requires_calls(/eval_exists, "internal/core.Kernel.Eval", "internal/core.Func").
`

func TestParseSpecFile(t *testing.T) {
	f, err := ParseSpecFile("Docs/specs/x.mg", goodSpec)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Requirements) != 1 || f.Requirements[0].ID != "/eval_exists" || !strings.Contains(f.Requirements[0].Text, "documented") {
		t.Fatalf("requirements: %+v", f.Requirements)
	}
	if len(f.Requires) != 2 || f.Requires[1].Property != PropHasDocComment || f.Requires[0].CodeRef != "internal/core.Kernel.Eval" {
		t.Fatalf("requires: %+v", f.Requires)
	}
	if len(f.Calls) != 1 || f.Calls[0].Other != "internal/core.Func" {
		t.Fatalf("calls: %+v", f.Calls)
	}
}

func TestSpecFileIsDataOnly(t *testing.T) {
	cases := map[string]string{
		"a rule":              "spec_requirement(/a, \"t\").\nrequires(/a, \"x\", /exists) :- requires(/a, \"y\", /exists).",
		"a declaration":       "Decl extra(X) bound [/string].\nspec_requirement(/a, \"t\").",
		"another predicate":   "spec_requirement(/a, \"t\").\npermitted(/delete_everything).",
		"an unknown property": "spec_requirement(/a, \"t\").\nrequires(/a, \"x\", /is_beautiful).",
		"calls as a property": "spec_requirement(/a, \"t\").\nrequires(/a, \"x\", /calls).",
		"no requirement text": "requires(/orphan, \"x\", /exists).",
		"a wrong argument":    "spec_requirement(/a, 7).",
	}
	for name, src := range cases {
		if f, err := ParseSpecFile("Docs/specs/bad.mg", src); err == nil {
			t.Errorf("%s was accepted: %+v", name, f)
		}
	}
}

func TestSpecFileRefusalNamesEveryProblem(t *testing.T) {
	_, err := ParseSpecFile("Docs/specs/bad.mg", "spec_requirement(/a, \"t\").\npermitted(/x).\nrequires(/a, \"x\", /nope).")
	if err == nil || !strings.Contains(err.Error(), "permitted") || !strings.Contains(err.Error(), "/nope") {
		t.Fatalf("both refusals should be named: %v", err)
	}
}

func TestResultFactsCarryEveryLayer(t *testing.T) {
	sf, err := ParseSpecFile("Docs/specs/x.mg", goodSpec)
	if err != nil {
		t.Fatal(err)
	}
	ix := fixtureIndex()
	doc := scanGuide(t)
	install := doc.Sections[1]
	res := Resolve(Input{
		Docs: []*DocScan{doc}, Code: ix, Specs: []*SpecFile{sf}, Now: 9,
		// The code of one link and the section of another moved since they were confirmed.
		Confirmed: Confirmations{
			{Spec: install.Ref, Code: "internal/core.Func"}:        {SpecFingerprint: install.Fingerprint, CodeFingerprint: "older", ConfirmedAt: 1},
			{Spec: install.Ref, Code: "internal/core.Kernel.Eval"}: {SpecFingerprint: "older", CodeFingerprint: "fp:internal/core.Kernel.Eval", ConfirmedAt: 1},
		},
		Measure: func(e Element) Measurement {
			return Measurement{HasTest: true, Calls: []string{"internal/core.Func"}}
		},
	})
	count := map[string]int{}
	for _, f := range res.Facts() {
		count[f.Predicate]++
	}
	for _, pred := range Predicates {
		if count[pred] == 0 {
			t.Errorf("no %s fact", pred)
		}
	}
	if m, ok := res.Measured["internal/core.Kernel.Eval"]; !ok || !m.HasTest {
		t.Fatalf("requirement target not measured: %+v", res.Measured)
	}
}

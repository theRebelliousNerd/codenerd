package testspec

import (
	"codenerd/internal/browser"
	"os"
	"strings"
	"testing"
)

func TestParseYAMLNormalizesPortableFixture(t *testing.T) {
	raw := `name: login
session_id: session-a
actions:
  - type: fill
    fields:
      - target: {id: email, input_type: email}
        value: user@example.test
      - target: {id: password, input_type: password}
        value_env: CODENERD_BROWSER_TEST_PASSWORD
assertions:
  - name: no errors
    query: user_visible_error(S, Kind, Message, Timestamp).
    expect: absent
`
	spec, err := ParseYAML(raw)
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	if len(spec.Actions) != 1 || len(spec.Assertions) != 1 || spec.Assertions[0].Scope != "fresh" || strings.HasSuffix(spec.Assertions[0].Query, ".") {
		t.Fatalf("unexpected normalized spec: %+v", spec)
	}
	encoded, err := MarshalYAML(spec)
	if err != nil || strings.Contains(encoded, "ref:") || !strings.Contains(encoded, "value_env: CODENERD_BROWSER_TEST_PASSWORD") {
		t.Fatalf("portable YAML: %v\n%s", err, encoded)
	}
}

func TestParseYAMLRejectsOpaqueRefsSecretsAliasesAndUnknownFields(t *testing.T) {
	cases := []string{
		`actions: [{type: interact, ref: e1_1}]\nassertions: [{name: state, query: "current_url(S, U)"}]`,
		`actions: [{type: interact, action: type, target: {id: password, input_type: password}, value: leaked}]\nassertions: [{name: state, query: "current_url(S, U)"}]`,
		`actions: &a [{type: navigate, url: "https://example.test"}]\ncopy: *a\nassertions: [{name: state, query: "current_url(S, U)"}]`,
		`unknown: true\nassertions: [{name: state, query: "current_url(S, U)"}]`,
	}
	for _, raw := range cases {
		raw = strings.ReplaceAll(raw, `\n`, "\n")
		if _, err := ParseYAML(raw); err == nil {
			t.Fatalf("ParseYAML accepted unsafe fixture:\n%s", raw)
		}
	}
}

func TestResolveEnvironmentDoesNotMutateFixture(t *testing.T) {
	t.Setenv("CODENERD_BROWSER_TEST_PASSWORD", "unit-secret")
	spec, err := ParseYAML(`
actions:
  - type: interact
    action: type
    target: {id: password, input_type: password}
    value_env: CODENERD_BROWSER_TEST_PASSWORD
assertions:
  - {name: state, query: "current_url(S, U)", scope: current}
`)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveEnvironment(spec)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Actions[0].Value != "unit-secret" || resolved.Actions[0].ValueEnv != "" {
		t.Fatalf("environment not resolved: %+v", resolved.Actions[0])
	}
	if spec.Actions[0].Value != "" || spec.Actions[0].ValueEnv != "CODENERD_BROWSER_TEST_PASSWORD" {
		t.Fatalf("portable fixture mutated: %+v", spec.Actions[0])
	}
}

func TestResolveEnvironmentRejectsEmptyOrOversizedValues(t *testing.T) {
	spec, err := ParseYAML(`
actions:
  - type: interact
    action: type
    target: {id: password, input_type: password}
    value_env: CODENERD_BROWSER_TEST_PASSWORD
assertions:
  - {name: state, query: "current_url(S, U)", scope: current}
`)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"empty":     "",
		"oversized": strings.Repeat("x", maxValueBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CODENERD_BROWSER_TEST_PASSWORD", value)
			if _, err := ResolveEnvironment(spec); err == nil {
				t.Fatalf("ResolveEnvironment accepted %s environment value", name)
			}
		})
	}
}

func TestNormalizeFillsDefaults(t *testing.T) {
	spec := Spec{
		Name:      "  login  ",
		SessionID: "  s1  ",
		Actions: []browser.ActionOperation{
			{Type: "navigate", URL: "https://example.test"},
		},
		Assertions: []Assertion{
			{Query: "current_url(S, U)."},
		},
	}
	if err := Normalize(&spec); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if spec.Name != "login" {
		t.Fatalf("name not trimmed: %q", spec.Name)
	}
	if spec.SessionID != "s1" {
		t.Fatalf("session not trimmed: %q", spec.SessionID)
	}
	got := spec.Assertions[0]
	if got.Name != "assertion_1" {
		t.Fatalf("assertion name default: %q", got.Name)
	}
	if got.Expect != "present" {
		t.Fatalf("expect default: %q", got.Expect)
	}
	if got.Scope != "fresh" {
		t.Fatalf("scope with actions must be fresh: %q", got.Scope)
	}
	if got.Query != "current_url(S, U)" {
		t.Fatalf("query dot not stripped: %q", got.Query)
	}

	queryless := Spec{
		Assertions: []Assertion{{Query: "ok"}},
	}
	if err := Normalize(&queryless); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if queryless.Name != "browser test" {
		t.Fatalf("name default: %q", queryless.Name)
	}
	if queryless.Assertions[0].Scope != "current" {
		t.Fatalf("scope without actions must be current: %q", queryless.Assertions[0].Scope)
	}

	cased := Spec{
		Assertions: []Assertion{{Query: "ok", Expect: " ABSENT ", Scope: " FRESH "}},
	}
	if err := Normalize(&cased); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if cased.Assertions[0].Expect != "absent" || cased.Assertions[0].Scope != "fresh" {
		t.Fatalf("expect/scope not lowercased: %+v", cased.Assertions[0])
	}
}

func TestNormalizeRejectsInvalidSpecs(t *testing.T) {
	validAssertion := func() Assertion {
		return Assertion{Name: "a", Query: "current_url(S, U)"}
	}
	manyActions := make([]browser.ActionOperation, MaxActions+1)
	for i := range manyActions {
		manyActions[i] = browser.ActionOperation{Type: "navigate", URL: "https://example.test"}
	}
	manyAssertions := make([]Assertion, MaxAssertions+1)
	for i := range manyAssertions {
		manyAssertions[i] = Assertion{Name: "a", Query: "q"}
	}
	cases := map[string]Spec{
		"name oversized":         {Name: strings.Repeat("x", MaxNameBytes+1), Assertions: []Assertion{validAssertion()}},
		"actions over limit":     {Actions: manyActions, Assertions: []Assertion{validAssertion()}},
		"no assertions":          {Name: "t"},
		"assertions over limit":  {Assertions: manyAssertions},
		"assertion name too big": {Assertions: []Assertion{{Name: strings.Repeat("x", MaxNameBytes+1), Query: "q"}}},
		"query empty":            {Assertions: []Assertion{{Name: "a", Query: "  "}}},
		"query dot only":         {Assertions: []Assertion{{Name: "a", Query: " . "}}},
		"query oversized":        {Assertions: []Assertion{{Name: "a", Query: strings.Repeat("x", MaxQueryBytes+1)}}},
		"expect invalid":         {Assertions: []Assertion{{Name: "a", Query: "q", Expect: "maybe"}}},
		"scope invalid":          {Assertions: []Assertion{{Name: "a", Query: "q", Scope: "everywhere"}}},
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := spec
			if err := Normalize(&candidate); err == nil {
				t.Fatalf("Normalize accepted %s", name)
			}
		})
	}
	if err := Normalize(nil); err == nil {
		t.Fatalf("Normalize accepted nil spec")
	}
}

func TestParseYAMLBoundsAndDocuments(t *testing.T) {
	if _, err := ParseYAML(""); err == nil {
		t.Fatalf("ParseYAML accepted empty input")
	}
	if _, err := ParseYAML(strings.Repeat("x", MaxFixtureBytes+1)); err == nil {
		t.Fatalf("ParseYAML accepted oversized input")
	}
	if _, err := ParseYAML("{{{{"); err == nil {
		t.Fatalf("ParseYAML accepted invalid yaml")
	}
	multi := "assertions: [{name: a, query: q}]\n---\nassertions: [{name: b, query: q}]\n"
	if _, err := ParseYAML(multi); err == nil {
		t.Fatalf("ParseYAML accepted multi-document input")
	}
	trailing := "assertions: [{name: a, query: q}]\n---\n: bad\n"
	if _, err := ParseYAML(trailing); err == nil {
		t.Fatalf("ParseYAML accepted bad trailing document")
	}
	spec, err := ParseYAML("assertions: [{name: a, query: q}]\n")
	if err != nil {
		t.Fatalf("ParseYAML minimal: %v", err)
	}
	if spec.Name != "browser test" || spec.Assertions[0].Scope != "current" {
		t.Fatalf("minimal defaults wrong: %+v", spec)
	}
}

func TestParseValueBranches(t *testing.T) {
	valid := map[string]any{
		"assertions": []any{map[string]any{"name": "a", "query": "current_url(S, U)"}},
	}
	spec, err := ParseValue(valid)
	if err != nil {
		t.Fatalf("ParseValue valid: %v", err)
	}
	if spec.Assertions[0].Scope != "current" {
		t.Fatalf("ParseValue scope default: %q", spec.Assertions[0].Scope)
	}
	if _, err := ParseValue(map[string]any{"unknown": true, "assertions": []any{map[string]any{"name": "a", "query": "q"}}}); err == nil {
		t.Fatalf("ParseValue accepted unknown field")
	}
	if _, err := ParseValue(map[string]any{"name": "t"}); err == nil {
		t.Fatalf("ParseValue accepted spec without assertions")
	}
	if _, err := ParseValue(make(chan int)); err == nil {
		t.Fatalf("ParseValue accepted unmarshalable value")
	}
	oversized := map[string]any{
		"name":       strings.Repeat("x", MaxFixtureBytes+1),
		"assertions": []any{map[string]any{"name": "a", "query": "q"}},
	}
	if _, err := ParseValue(oversized); err == nil {
		t.Fatalf("ParseValue accepted oversized payload")
	}
}

func TestMarshalYAMLBranches(t *testing.T) {
	spec := Spec{
		Name:       "login",
		Actions:    []browser.ActionOperation{{Type: "navigate", URL: "https://example.test"}},
		Assertions: []Assertion{{Name: "state", Query: "current_url(S, U)", Expect: "present", Scope: "fresh"}},
	}
	encoded, err := MarshalYAML(spec)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	roundtrip, err := ParseYAML(encoded)
	if err != nil {
		t.Fatalf("ParseYAML(MarshalYAML): %v", err)
	}
	if len(roundtrip.Actions) != 1 || roundtrip.Actions[0].URL != "https://example.test" {
		t.Fatalf("roundtrip lost action: %+v", roundtrip)
	}
	if _, err := MarshalYAML(Spec{Name: "empty"}); err == nil {
		t.Fatalf("MarshalYAML accepted spec without assertions")
	}
	bigValue := strings.Repeat("x", 12<<10)
	actions := make([]browser.ActionOperation, MaxActions)
	for i := range actions {
		actions[i] = browser.ActionOperation{
			Type:   "interact",
			Action: "type",
			Target: &browser.ElementMatcher{ID: "field"},
			Value:  bigValue,
		}
	}
	big := Spec{Name: "big", Actions: actions, Assertions: []Assertion{{Name: "a", Query: "q"}}}
	if _, err := MarshalYAML(big); err == nil {
		t.Fatalf("MarshalYAML accepted oversized encoding")
	}
}

func TestNormalizeActionTypes(t *testing.T) {
	matcher := func() *browser.ElementMatcher { return &browser.ElementMatcher{ID: "field"} }
	sensitive := func() *browser.ElementMatcher { return &browser.ElementMatcher{ID: "password"} }
	assertion := func() []Assertion { return []Assertion{{Name: "a", Query: "q"}} }
	cases := []struct {
		name    string
		action  browser.ActionOperation
		wantErr bool
	}{
		{"navigate ok", browser.ActionOperation{Type: "navigate", URL: "https://example.test"}, false},
		{"navigate upper", browser.ActionOperation{Type: "NAVIGATE", URL: "https://example.test"}, false},
		{"navigate empty", browser.ActionOperation{Type: "navigate"}, true},
		{"navigate oversized", browser.ActionOperation{Type: "navigate", URL: strings.Repeat("x", maxURLBytes+1)}, true},
		{"interact click default", browser.ActionOperation{Type: "interact", Target: matcher()}, false},
		{"interact ref rejected", browser.ActionOperation{Type: "interact", Ref: "e1_1"}, true},
		{"interact nil target", browser.ActionOperation{Type: "interact", Action: "click"}, true},
		{"interact empty matcher", browser.ActionOperation{Type: "interact", Target: &browser.ElementMatcher{}}, true},
		{"interact unsupported", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "dance"}, true},
		{"interact click with value", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "click", Value: "x"}, true},
		{"interact click with env", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "clear", ValueEnv: "SOME_VAR"}, true},
		{"interact type missing value", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "type"}, true},
		{"interact select missing value", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "select"}, true},
		{"interact type both", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "type", Value: "a", ValueEnv: "B"}, true},
		{"interact type bad env", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "type", ValueEnv: "bad-name!"}, true},
		{"interact sensitive literal", browser.ActionOperation{Type: "interact", Target: sensitive(), Action: "type", Value: "leaked"}, true},
		{"interact value oversized", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "type", Value: strings.Repeat("x", maxValueBytes+1)}, true},
		{"interact toggle ok", browser.ActionOperation{Type: "interact", Target: matcher(), Action: "toggle"}, false},
		{"fill ok", browser.ActionOperation{Type: "fill", Fields: []browser.FillField{{Target: matcher(), Value: "v"}}}, false},
		{"fill empty", browser.ActionOperation{Type: "fill"}, true},
		{"fill ref field", browser.ActionOperation{Type: "fill", Fields: []browser.FillField{{Ref: "e1"}}}, true},
		{"fill missing value", browser.ActionOperation{Type: "fill", Fields: []browser.FillField{{Target: matcher()}}}, true},
		{"fill submit button", browser.ActionOperation{Type: "fill", Fields: []browser.FillField{{Target: matcher(), Value: "v"}}, SubmitButton: "go"}, true},
		{"fill bad submit target", browser.ActionOperation{Type: "fill", Fields: []browser.FillField{{Target: matcher(), Value: "v"}}, SubmitTarget: &browser.ElementMatcher{}}, true},
		{"fill good submit target", browser.ActionOperation{Type: "fill", Fields: []browser.FillField{{Target: matcher(), Value: "v"}}, SubmitTarget: matcher()}, false},
		{"key ok", browser.ActionOperation{Type: "key", Key: "Enter"}, false},
		{"key empty", browser.ActionOperation{Type: "key"}, true},
		{"key oversized", browser.ActionOperation{Type: "key", Key: strings.Repeat("x", 65)}, true},
		{"history back", browser.ActionOperation{Type: "history", Action: "back"}, false},
		{"history forward", browser.ActionOperation{Type: "history", Action: "forward"}, false},
		{"history reload", browser.ActionOperation{Type: "history", Action: "reload"}, false},
		{"history bad", browser.ActionOperation{Type: "history", Action: "jump"}, true},
		{"sleep ok", browser.ActionOperation{Type: "sleep", DurationMS: 100}, false},
		{"sleep negative", browser.ActionOperation{Type: "sleep", DurationMS: -1}, true},
		{"sleep too long", browser.ActionOperation{Type: "sleep", DurationMS: 30001}, true},
		{"unknown type", browser.ActionOperation{Type: "hover"}, true},
		{"empty type", browser.ActionOperation{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := Spec{Actions: []browser.ActionOperation{tc.action}, Assertions: assertion()}
			err := Normalize(&spec)
			if tc.wantErr && err == nil {
				t.Fatalf("Normalize accepted %s: %+v", tc.name, tc.action)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Normalize rejected %s: %v", tc.name, err)
			}
		})
	}
	overflow := Spec{
		Actions: []browser.ActionOperation{{Type: "fill", Fields: append(make([]browser.FillField, 0, 51), func() []browser.FillField {
			fields := make([]browser.FillField, 51)
			for i := range fields {
				fields[i] = browser.FillField{Target: matcher(), Value: "v"}
			}
			return fields
		}()...)}},
		Assertions: assertion(),
	}
	if err := Normalize(&overflow); err == nil {
		t.Fatalf("Normalize accepted fill with 51 fields")
	}
}

func TestResolveEnvironmentBranches(t *testing.T) {
	const envName = "CODENERD_BROWSER_TESTSPECS_PINNED"
	t.Setenv(envName, "resolved-value")
	spec, err := ParseYAML("actions:\n  - type: interact\n    action: type\n    target: {id: login}\n    value_env: " + envName + "\nassertions:\n  - {name: state, query: q}\n")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveEnvironment(spec)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Actions[0].Value != "resolved-value" || resolved.Actions[0].ValueEnv != "" {
		t.Fatalf("action env not resolved: %+v", resolved.Actions[0])
	}

	fillSpec, err := ParseYAML("actions:\n  - type: fill\n    fields:\n      - target: {id: login}\n        value_env: " + envName + "\nassertions:\n  - {name: state, query: q}\n")
	if err != nil {
		t.Fatal(err)
	}
	fillResolved, err := ResolveEnvironment(fillSpec)
	if err != nil {
		t.Fatal(err)
	}
	if fillResolved.Actions[0].Fields[0].Value != "resolved-value" || fillResolved.Actions[0].Fields[0].ValueEnv != "" {
		t.Fatalf("fill env not resolved: %+v", fillResolved.Actions[0].Fields[0])
	}
	if fillSpec.Actions[0].Fields[0].Value != "" {
		t.Fatalf("fill fixture mutated")
	}

	plain := Spec{
		Actions:    []browser.ActionOperation{{Type: "navigate", URL: "https://example.test"}},
		Assertions: []Assertion{{Name: "a", Query: "q"}},
	}
	unchanged, err := ResolveEnvironment(plain)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Actions[0].URL != "https://example.test" {
		t.Fatalf("plain resolve changed action: %+v", unchanged.Actions[0])
	}

	for _, tc := range []struct {
		name string
		spec Spec
	}{
		{"invalid action env", Spec{Actions: []browser.ActionOperation{{Type: "interact", Action: "type", Target: &browser.ElementMatcher{ID: "f"}, ValueEnv: "bad-name!"}}, Assertions: []Assertion{{Name: "a", Query: "q"}}}},
		{"invalid field env", Spec{Actions: []browser.ActionOperation{{Type: "fill", Fields: []browser.FillField{{Target: &browser.ElementMatcher{ID: "f"}, ValueEnv: "bad name"}}}}, Assertions: []Assertion{{Name: "a", Query: "q"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveEnvironment(tc.spec); err == nil {
				t.Fatalf("ResolveEnvironment accepted %s", tc.name)
			}
		})
	}

	missing := Spec{
		Actions:    []browser.ActionOperation{{Type: "interact", Action: "type", Target: &browser.ElementMatcher{ID: "f"}, ValueEnv: "CODENERD_BROWSER_TESTSPECS_MISSING_999"}},
		Assertions: []Assertion{{Name: "a", Query: "q"}},
	}
	os.Unsetenv("CODENERD_BROWSER_TESTSPECS_MISSING_999")
	if _, err := ResolveEnvironment(missing); err == nil {
		t.Fatalf("ResolveEnvironment accepted missing variable")
	}
}

func TestCloneSpecDeepCopy(t *testing.T) {
	t.Setenv("CODENERD_BROWSER_TESTSPECS_PINNED", "secret-value")
	original := Spec{
		Name: "orig",
		Actions: []browser.ActionOperation{
			{
				Type:         "fill",
				Target:       &browser.ElementMatcher{ID: "before"},
				SubmitTarget: &browser.ElementMatcher{ID: "submit"},
				Fields: []browser.FillField{
					{Target: &browser.ElementMatcher{ID: "email"}, ValueEnv: "CODENERD_BROWSER_TESTSPECS_PINNED"},
				},
			},
		},
		Assertions: []Assertion{{Name: "a", Query: "q"}},
	}
	resolved, err := ResolveEnvironment(original)
	if err != nil {
		t.Fatal(err)
	}
	resolved.Actions[0].Target.ID = "mutated"
	resolved.Actions[0].SubmitTarget.ID = "mutated"
	resolved.Actions[0].Fields[0].Target.ID = "mutated"
	resolved.Actions[0].Fields[0].Value = "mutated"
	resolved.Assertions[0].Query = "mutated"
	if original.Actions[0].Target.ID != "before" {
		t.Fatalf("target aliased: %+v", original.Actions[0].Target)
	}
	if original.Actions[0].SubmitTarget.ID != "submit" {
		t.Fatalf("submit target aliased: %+v", original.Actions[0].SubmitTarget)
	}
	if original.Actions[0].Fields[0].Target.ID != "email" {
		t.Fatalf("field target aliased: %+v", original.Actions[0].Fields[0].Target)
	}
	if original.Actions[0].Fields[0].ValueEnv != "CODENERD_BROWSER_TESTSPECS_PINNED" {
		t.Fatalf("field value_env cleared in original: %+v", original.Actions[0].Fields[0])
	}
	if original.Assertions[0].Query != "q" {
		t.Fatalf("assertions aliased: %+v", original.Assertions)
	}
}

func TestValidateYAMLNodeLimits(t *testing.T) {
	if err := validateYAMLNode(nil, 0, new(int)); err != nil {
		t.Fatalf("nil node must be valid: %v", err)
	}
	deep := ""
	indent := ""
	for i := 0; i < 50; i++ {
		deep += indent + "a:\n"
		indent += "  "
	}
	deep += indent + "b: 1\n"
	if _, err := ParseYAML(deep + "assertions: [{name: a, query: q}]\n"); err == nil {
		t.Fatalf("ParseYAML accepted over-deep yaml")
	}
	wide := strings.Repeat("- 1\n", maxYAMLNodes+100)
	if _, err := ParseYAML(wide); err == nil {
		t.Fatalf("ParseYAML accepted over-wide yaml")
	}
}

func TestValidEnvironmentName(t *testing.T) {
	for _, name := range []string{"A", "a_b1", "CODENERD_X", "_lead", "a1"} {
		if !validEnvironmentName(name) {
			t.Fatalf("validEnvironmentName rejected %q", name)
		}
	}
	for _, name := range []string{"", "1abc", "bad-name!", "has space", "dot.name"} {
		if validEnvironmentName(name) {
			t.Fatalf("validEnvironmentName accepted %q", name)
		}
	}
	if _, err := environmentValue("bad-name!", "here"); err == nil {
		t.Fatalf("environmentValue accepted invalid name")
	}
}

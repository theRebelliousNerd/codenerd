package testspec

import (
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

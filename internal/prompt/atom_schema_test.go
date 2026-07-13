package prompt

import (
	"strings"
	"testing"
)

func TestParsePromptAtomYAML_StrictCanonicalSchema(t *testing.T) {
	data := []byte(`- schema_version: 1
  version: 2
  id: test/canonical
  category: methodology
  priority: 70
  is_mandatory: false
  shard_types: [/coder]
  world_states: [reflection_hits, no_tool_call_retry]
  content: canonical body
`)

	parsed, migrations, err := ParsePromptAtomYAML(data, "canonical.yaml", nil)
	if err != nil {
		t.Fatalf("parse canonical atom: %v", err)
	}
	if len(migrations) != 0 {
		t.Fatalf("canonical atom unexpectedly migrated: %+v", migrations)
	}
	if len(parsed) != 1 {
		t.Fatalf("got %d atoms, want 1", len(parsed))
	}
	atom := parsed[0].Atom
	if atom.Version != 2 {
		t.Fatalf("version = %d, want 2", atom.Version)
	}
	if len(atom.ShardTypes) != 1 || atom.ShardTypes[0] != "coder" {
		t.Fatalf("selectors were not normalized: %#v", atom.ShardTypes)
	}
}

func TestParsePromptAtomYAML_BoundedLegacyMigrations(t *testing.T) {
	data := []byte(`id: test/legacy
category: intent
name: Intent Understanding
version: 1.0.0
agent_types: [/coder]
selectors:
  - always: true
content: legacy body
`)

	parsed, migrations, err := ParsePromptAtomYAML(data, "legacy.yaml", nil)
	if err != nil {
		t.Fatalf("parse legacy atom: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("got %d atoms, want 1", len(parsed))
	}
	atom := parsed[0].Atom
	if atom.Subcategory != "intent_understanding" {
		t.Fatalf("subcategory = %q", atom.Subcategory)
	}
	if len(atom.ShardTypes) != 1 || atom.ShardTypes[0] != "coder" {
		t.Fatalf("agent_types migration lost selector: %#v", atom.ShardTypes)
	}
	if atom.Priority != 0 || atom.IsMandatory {
		t.Fatalf("legacy defaults changed: priority=%d mandatory=%v", atom.Priority, atom.IsMandatory)
	}

	wantCodes := []string{
		"legacy-semver-version",
		"legacy-name",
		"agent-types-alias",
		"legacy-selectors",
		"legacy-priority-default",
		"legacy-mandatory-default",
	}
	for _, code := range wantCodes {
		found := false
		for _, migration := range migrations {
			if migration.Code == code {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing migration %s in %+v", code, migrations)
		}
	}
}

func TestParsePromptAtomYAML_FailsWholeDocument(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "unknown field",
			yaml: "id: test/unknown\ncategory: identity\npriority: 1\nis_mandatory: false\ncontent: body\ntyop: true\n",
			want: "field tyop not found",
		},
		{
			name: "future schema",
			yaml: "schema_version: 2\nid: test/future\ncategory: identity\npriority: 1\nis_mandatory: false\ncontent: body\n",
			want: "unsupported schema_version",
		},
		{
			name: "alias collision",
			yaml: "id: test/collision\ncategory: identity\npriority: 1\nis_mandatory: false\nagent_types: [/coder]\nshard_types: [/tester]\ncontent: body\n",
			want: "cannot both be set",
		},
		{
			name: "unknown world state",
			yaml: "id: test/world\ncategory: identity\npriority: 1\nis_mandatory: false\nworld_states: [invented]\ncontent: body\n",
			want: "unknown value",
		},
		{
			name: "one invalid item rejects sequence",
			yaml: "- id: test/good\n  category: identity\n  priority: 1\n  is_mandatory: false\n  content: body\n- id: test/bad\n  category: identity\n  priority: 1\n  is_mandatory: false\n  content: ''\n",
			want: "missing required field",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, _, err := ParsePromptAtomYAML([]byte(test.yaml), "test.yaml", nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			if len(parsed) != 0 {
				t.Fatalf("partial atoms escaped failed document: %d", len(parsed))
			}
		})
	}
}

func TestKnownWorldStatesFollowLiveContextVocabulary(t *testing.T) {
	known := KnownWorldStates()
	for _, value := range []string{"reflection_hits", "no_tool_call_retry"} {
		if _, ok := known[value]; !ok {
			t.Errorf("live world state %q missing from schema vocabulary", value)
		}
	}
}

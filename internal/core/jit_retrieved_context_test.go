package core

import "testing"

func TestJITRetrievedContextRequiresWitnessAndRespectsScope(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	assertPinFixture(t, k,
		`atom("knowledge/matching")`,
		`atom("knowledge/missing")`,
		`atom("knowledge/sibling")`,
		`atom_priority("knowledge/matching", 85)`,
		`atom_priority("knowledge/missing", 85)`,
		`atom_priority("knowledge/sibling", 85)`,
		`prompt_atom("knowledge/matching", /knowledge, 85, 20, /false)`,
		`prompt_atom("knowledge/missing", /knowledge, 85, 20, /false)`,
		`prompt_atom("knowledge/sibling", /knowledge, 85, 20, /false)`,
		`atom_tag("knowledge/matching", /shard, /expert)`,
		`atom_tag("knowledge/missing", /shard, /expert)`,
		`atom_tag("knowledge/sibling", /shard, /other)`,
		`current_context(/shard, /expert)`,
		`retrieved_context("knowledge/matching")`,
		`retrieved_context("knowledge/sibling")`,
	)
	facts, err := k.Query("selected_result")
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, f := range facts {
		if len(f.Args) > 0 {
			if id, ok := f.Args[0].(string); ok {
				selected[id] = true
			}
		}
	}
	if !selected["knowledge/matching"] {
		t.Fatal("retrieved runtime knowledge was not admitted")
	}
	if selected["knowledge/missing"] || selected["knowledge/sibling"] {
		t.Fatalf("retrieval bypassed missing-source or scope control: %v", selected)
	}
}

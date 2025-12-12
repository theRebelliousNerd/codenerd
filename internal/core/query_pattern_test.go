package core

import (
	"testing"

	"github.com/google/mangle/factstore"
)

func TestKernelQueryPatternFiltering(t *testing.T) {
	k := &RealKernel{
		facts:      make([]Fact, 0),
		factIndex:  make(map[string]struct{}),
		store:      factstore.NewSimpleInMemoryStore(),
		policyDirty: true,
	}

	k.schemas = `
Decl probe(A, B).
Decl name_probe(A).
`
	k.policy = ``
	k.learned = ``

	k.facts = append(k.facts,
		Fact{Predicate: "probe", Args: []interface{}{"alpha", "one"}},
		Fact{Predicate: "probe", Args: []interface{}{"beta", "one"}},
		Fact{Predicate: "probe", Args: []interface{}{"beta", "two"}},
		Fact{Predicate: "name_probe", Args: []interface{}{"/foo"}},
	)
	k.rebuildFactIndexLocked()

	if err := k.evaluate(); err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}

	all, err := k.Query("probe")
	if err != nil {
		t.Fatalf("Query(probe) error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("Query(probe) got %d facts, want %d", len(all), 3)
	}

	betaAny, err := k.Query(`probe("beta", B)`)
	if err != nil {
		t.Fatalf("Query(probe(\"beta\", B)) error = %v", err)
	}
	if len(betaAny) != 2 {
		t.Fatalf("Query(probe(\"beta\", B)) got %d facts, want %d", len(betaAny), 2)
	}

	betaTwo, err := k.Query(`probe("beta", "two")`)
	if err != nil {
		t.Fatalf("Query(probe(\"beta\", \"two\")) error = %v", err)
	}
	if len(betaTwo) != 1 {
		t.Fatalf("Query(probe(\"beta\", \"two\")) got %d facts, want %d", len(betaTwo), 1)
	}

	nameMatch, err := k.Query("name_probe(/foo)")
	if err != nil {
		t.Fatalf("Query(name_probe(/foo)) error = %v", err)
	}
	if len(nameMatch) != 1 {
		t.Fatalf("Query(name_probe(/foo)) got %d facts, want %d", len(nameMatch), 1)
	}

	bridge := NewAutopoiesisBridge(k)
	if !bridge.QueryBool(`probe("beta", "two")`) {
		t.Fatalf("QueryBool(probe(\"beta\", \"two\")) got false, want true")
	}
	if bridge.QueryBool(`probe("missing", _)`) {
		t.Fatalf("QueryBool(probe(\"missing\", _)) got true, want false")
	}
}


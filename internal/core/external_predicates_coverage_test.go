package core

import (
	"errors"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

func TestVirtualExternalPredicate_ShouldPushdown(t *testing.T) {
	v := &virtualExternalPredicate{}
	if v.ShouldPushdown() != false {
		t.Error("ShouldPushdown should always return false")
	}
}

func TestVirtualExternalPredicate_ShouldQuery(t *testing.T) {
	v := &virtualExternalPredicate{}
	if v.ShouldQuery(nil, nil, nil) != true {
		t.Error("ShouldQuery should always return true")
	}
}

func TestVirtualExternalPredicate_ExecuteQuery(t *testing.T) {
	ps := ast.PredicateSym{Symbol: "test_pred", Arity: 2}

	handlerCalled := false
	var passedQuery ast.Atom
	mockHandler := func(query ast.Atom) ([]ast.Atom, error) {
		handlerCalled = true
		passedQuery = query
		return []ast.Atom{
			{
				Predicate: ps,
				Args:      []ast.BaseTerm{ast.String("input_val"), ast.String("output_val")},
			},
		}, nil
	}

	v := &virtualExternalPredicate{
		predSym: ps,
		handler: mockHandler,
		mode:    []ast.ArgMode{ast.ArgModeInput, ast.ArgModeOutput},
	}

	inputs := []ast.Constant{ast.String("input_val")}
	filters := []ast.BaseTerm{ast.Variable{Symbol: "OutVar"}}
	var cbCalled bool
	var cbOutputs []ast.BaseTerm
	cb := func(outputs []ast.BaseTerm) {
		cbCalled = true
		cbOutputs = outputs
	}

	err := v.ExecuteQuery(inputs, filters, nil, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("expected handler to be called")
	}
	if passedQuery.Predicate.Symbol != "test_pred" || len(passedQuery.Args) != 2 {
		t.Errorf("unexpected query: %v", passedQuery)
	}
	if passedQuery.Args[0] != ast.String("input_val") {
		t.Errorf("expected arg[0] to be 'input_val', got: %v", passedQuery.Args[0])
	}
	if passedQuery.Args[1] != (ast.Variable{Symbol: "OutVar"}) {
		t.Errorf("expected arg[1] to be OutVar, got: %v", passedQuery.Args[1])
	}

	if !cbCalled {
		t.Error("expected callback to be called")
	}
	if len(cbOutputs) != 1 || cbOutputs[0] != ast.String("output_val") {
		t.Errorf("expected cbOutputs to contain 'output_val', got: %v", cbOutputs)
	}
}

func TestVirtualExternalPredicate_ExecuteQuery_Error(t *testing.T) {
	ps := ast.PredicateSym{Symbol: "test_pred", Arity: 1}
	mockHandler := func(query ast.Atom) ([]ast.Atom, error) {
		return nil, errors.New("some handler error")
	}
	v := &virtualExternalPredicate{
		predSym: ps,
		handler: mockHandler,
		mode:    []ast.ArgMode{ast.ArgModeInput},
	}
	err := v.ExecuteQuery([]ast.Constant{ast.String("val")}, nil, nil, func([]ast.BaseTerm) {})
	if err == nil || err.Error() != "some handler error" {
		t.Errorf("expected 'some handler error', got: %v", err)
	}
}

func TestVirtualStore_BuildExternalPredicates(t *testing.T) {
	var vs *VirtualStore
	if vs.BuildExternalPredicates() != nil {
		t.Error("expected nil map for nil virtual store")
	}

	vs = NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	callbacks := vs.BuildExternalPredicates()
	if callbacks == nil {
		t.Fatal("expected non-nil callbacks map")
	}

	expectedKeys := []string{
		"query_learned",
		"query_session",
		"recall_similar",
		"query_knowledge_graph",
		"query_strategic",
		"query_activations",
		"has_learned",
		"query_traces",
		"query_trace_stats",
	}

	for _, key := range expectedKeys {
		found := false
		for sym := range callbacks {
			if sym.Symbol == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected predicate %q in callbacks", key)
		}
	}
}

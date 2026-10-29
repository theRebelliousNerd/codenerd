package perception

import (
	"context"
	"testing"
)

type mockRoutingKernel2 struct {
	queries map[string][]RoutingMatch
	valid   map[string]bool
	asserts []string
}

func (m *mockRoutingKernel2) QueryRouting(ctx context.Context, predicate string, arg string) ([]RoutingMatch, error) {
	key := predicate + ":" + arg
	if res, ok := m.queries[key]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *mockRoutingKernel2) ValidateField(ctx context.Context, field, value string) bool {
	key := field + ":" + value
	return m.valid[key]
}

func (m *mockRoutingKernel2) AssertRoutingFact(predicate string, args ...any) error {
	m.asserts = append(m.asserts, predicate)
	return nil
}

func (m *mockRoutingKernel2) RetractRoutingPredicate(predicate string) error {
	return nil
}

func TestLLMTransducer_assertRoutingFacts(t *testing.T) {
	rk := &mockRoutingKernel2{}
	tr := NewLLMTransducer(nil, rk, "prompt")

	u := &Understanding{
		SemanticType: "sem",
		ActionType:   "act",
		Domain:       "dom",
	}
	routing := &Routing{
		Mode:              "test_mode",
		PrimaryShard:      "coder",
		ContextPriorities: map[string]int{"ctx1": 10},
		ToolPriorities:    map[string]int{"tool1": 20},
	}

	tr.assertRoutingFacts(rk, u, routing)

	expectedFacts := map[string]bool{
		"current_understanding":    true,
		"derived_mode":             true,
		"derived_primary_shard":    true,
		"derived_context_priority": true,
		"derived_tool_priority":    true,
	}

	for _, a := range rk.asserts {
		if !expectedFacts[a] {
			t.Errorf("unexpected fact asserted: %s", a)
		}
	}
}

func TestLLMTransducer_deriveBlockedTools(t *testing.T) {
	ctx := context.Background()
	rk := &mockRoutingKernel2{
		queries: map[string][]RoutingMatch{
			"constraint_blocks_tool:no_network": {
				{Target: "curl"},
			},
		},
	}
	tr := NewLLMTransducer(nil, rk, "prompt")

	u := &Understanding{
		UserConstraints: []string{"no_network"},
		SemanticType:    "query",   // read-only triggers
		ActionType:      "explain", // triggers read-only mode
	}

	blocked := tr.deriveBlockedTools(ctx, u)
	foundCurl := false
	foundWriteFile := false
	for _, b := range blocked {
		if b == "curl" {
			foundCurl = true
		}
		if b == "write_file" {
			foundWriteFile = true
		}
	}

	if !foundCurl {
		t.Errorf("expected curl to be blocked")
	}

	if !foundWriteFile {
		t.Errorf("expected write_file to be blocked")
	}
}

func TestMax(t *testing.T) {
	if max(1, 2) != 2 {
		t.Errorf("max(1, 2) should be 2")
	}
	if max(5, 3) != 5 {
		t.Errorf("max(5, 3) should be 5")
	}
}

func TestNewRealKernelRouter(t *testing.T) {
	r := NewRealKernelRouter(nil)
	if r.kernel != nil {
		t.Errorf("expected nil kernel")
	}

	// Test nil kernel paths
	matches, err := r.QueryRouting(context.Background(), "pred", "arg")
	if err != nil || matches != nil {
		t.Errorf("expected nil/nil on nil kernel QueryRouting")
	}

	if misses, err := r.VocabularyMisses(); err != nil || misses != nil {
		t.Errorf("expected nil/nil on nil kernel VocabularyMisses, got %v, %v", misses, err)
	}

	if err := r.AssertRoutingFact("pred", "arg"); err != nil {
		t.Errorf("expected nil on nil kernel AssertRoutingFact")
	}

	if err := r.RetractRoutingPredicate("pred"); err != nil {
		t.Errorf("expected nil on nil kernel RetractRoutingPredicate")
	}
}

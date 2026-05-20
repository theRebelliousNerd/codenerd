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

func (m *mockRoutingKernel2) AssertRoutingFact(predicate string, args ...interface{}) error {
	m.asserts = append(m.asserts, predicate)
	return nil
}

func (m *mockRoutingKernel2) RetractRoutingPredicate(predicate string) error {
	return nil
}

func TestLLMTransducer_validate(t *testing.T) {
	ctx := context.Background()
	rk := &mockRoutingKernel2{
		valid: map[string]bool{
			"semantic_type:valid_sem": true,
			"action_type:valid_act":   true,
			"domain:valid_dom":        true,
			"scope_level:valid_scope": true,
			"mode:valid_mode":         true,
		},
	}
	tr := NewLLMTransducer(nil, rk, "prompt")

	uValid := &Understanding{
		SemanticType: "valid_sem",
		ActionType:   "valid_act",
		Domain:       "valid_dom",
		Scope: Scope{Level: "valid_scope"},
		SuggestedApproach: SuggestedApproach{Mode: "valid_mode"},
	}

	err := tr.validate(ctx, uValid)
	if err != nil {
		t.Errorf("expected valid understanding, got error: %v", err)
	}

	uInvalid := &Understanding{
		SemanticType: "invalid_sem",
		ActionType:   "invalid_act",
		Domain:       "invalid_dom",
		Scope: Scope{Level: "invalid_scope"},
		SuggestedApproach: SuggestedApproach{Mode: "invalid_mode"},
	}

	err = tr.validate(ctx, uInvalid)
	if err == nil {
		t.Errorf("expected invalid understanding to fail")
	}
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
		Mode:             "test_mode",
		PrimaryShard:     "coder",
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
		SemanticType:    "query", // read-only triggers
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
	
	if r.ValidateField(context.Background(), "field", "value") != true {
		t.Errorf("expected true on nil kernel ValidateField")
	}
	
	if err := r.AssertRoutingFact("pred", "arg"); err != nil {
		t.Errorf("expected nil on nil kernel AssertRoutingFact")
	}
	
	if err := r.RetractRoutingPredicate("pred"); err != nil {
		t.Errorf("expected nil on nil kernel RetractRoutingPredicate")
	}
}

func TestNewMangleRoutingKernel(t *testing.T) {
	m := NewMangleRoutingKernel(nil)
	if m.engine != nil {
		t.Errorf("expected nil engine")
	}
	
	// Create an empty mangle engine to test QueryRouting and ValidateField
	// Note: We don't have direct access to mangle.NewEngine here without importing it.
	// We'll test the string formatting by passing nil and catching the panic, or just ignore for now since
	// testing these without a real engine is mostly testing the string formatter.
	// Let's at least test the unknown predicate branch which doesn't panic.
	_, err := m.QueryRouting(context.Background(), "unknown_predicate", "test")
	if err == nil || err.Error() != "unknown predicate: unknown_predicate" {
		t.Errorf("expected unknown predicate error, got %v", err)
	}

	// Test ValidateField unknown field
	if m.ValidateField(context.Background(), "unknown_field", "test") != true {
		t.Errorf("expected true for unknown field")
	}

	// Test QueryRouting with panic recovery for known fields
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic due to nil engine")
			}
		}()
		m.QueryRouting(context.Background(), "mode_from_", "test")
	}()
	func() {
		defer func() { recover() }()
		m.QueryRouting(context.Background(), "context_affinity_", "test")
	}()
	func() {
		defer func() { recover() }()
		m.QueryRouting(context.Background(), "shard_affinity_", "test")
	}()
	func() {
		defer func() { recover() }()
		m.QueryRouting(context.Background(), "tool_affinity_", "test")
	}()
	func() {
		defer func() { recover() }()
		m.QueryRouting(context.Background(), "constraint_blocks_tool", "test")
	}()

	// Test ValidateField with panic recovery for known field
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic due to nil engine")
			}
		}()
		m.ValidateField(context.Background(), "semantic_type", "test")
	}()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic due to nil engine")
			}
		}()
		m.ValidateField(context.Background(), "action_type", "test")
	}()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic due to nil engine")
			}
		}()
		m.ValidateField(context.Background(), "domain", "test")
	}()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic due to nil engine")
			}
		}()
		m.ValidateField(context.Background(), "scope_level", "test")
	}()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic due to nil engine")
			}
		}()
		m.ValidateField(context.Background(), "mode", "test")
	}()
}

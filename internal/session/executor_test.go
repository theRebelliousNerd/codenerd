package session

import (
	"codenerd/internal/types"
	"testing"
)

// MockKernel implements types.Kernel for testing.
type MockKernel struct {
	facts   []types.Fact
	asserts []types.Fact // Track assertions for verification
}

func (m *MockKernel) LoadFacts(facts []types.Fact) error {
	m.facts = append(m.facts, facts...)
	return nil
}

func (m *MockKernel) Query(predicate string) ([]types.Fact, error) {
	var results []types.Fact
	for _, f := range m.facts {
		if f.Predicate == predicate {
			results = append(results, f)
		}
	}
	return results, nil
}

func (m *MockKernel) QueryAll() (map[string][]types.Fact, error) {
	return nil, nil
}

func (m *MockKernel) Assert(fact types.Fact) error {
	m.facts = append(m.facts, fact)
	m.asserts = append(m.asserts, fact)
	return nil
}

func (m *MockKernel) Retract(predicate string) error {
	var newFacts []types.Fact
	for _, f := range m.facts {
		if f.Predicate != predicate {
			newFacts = append(newFacts, f)
		}
	}
	m.facts = newFacts
	return nil
}

func (m *MockKernel) RetractFact(fact types.Fact) error {
	// Simple retraction mock
	var newFacts []types.Fact
	for _, f := range m.facts {
		// Only retract if predicate matches (simplified for test)
		if f.Predicate != fact.Predicate {
			newFacts = append(newFacts, f)
		}
	}
	m.facts = newFacts
	return nil
}

func (m *MockKernel) UpdateSystemFacts() error { return nil }
func (m *MockKernel) Reset() {}
func (m *MockKernel) AppendPolicy(policy string) {}
func (m *MockKernel) RetractExactFactsBatch(facts []types.Fact) error { return nil }
func (m *MockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error { return nil }

func TestExecutor_CheckSafety_ConstitutionalGate(t *testing.T) {
	// 1. Setup
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_1",
		Name: "readFile",
		Args: map[string]interface{}{
			"path": "secret.txt",
		},
	}
	target := "secret.txt"
	payload := `{"path":"secret.txt"}`

	// 2. Case: Denied Action (No permitted fact)
	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected action to be denied when no permitted fact exists")
	}

	// Verify pending_action was asserted
	foundPending := false
	for _, f := range mockKernel.asserts {
		if f.Predicate == "pending_action" {
			// Check args: ActionID, ActionType, Target, Payload, Timestamp
			if len(f.Args) == 5 && f.Args[0] == "call_1" {
				foundPending = true
				break
			}
		}
	}
	if !foundPending {
		t.Error("pending_action fact was not asserted")
	}

	// 3. Case: Permitted Action
	// Add permitted fact: permitted(Action, Target, Payload)
	// Action must be MangleAtom "/readFile"
	// We use string "/readFile" which matches fmt.Sprintf("%v", arg) check
	mockKernel.facts = append(mockKernel.facts, types.Fact{
		Predicate: "permitted",
		Args: []interface{}{
			"/readFile",
			target,
			payload,
		},
	})

	allowed = executor.checkSafety(toolCall)
	if !allowed {
		t.Error("Expected action to be allowed when permitted fact exists")
	}

	// 4. Case: Mismatch Target
	mockKernel.facts = []types.Fact{{
		Predicate: "permitted",
		Args: []interface{}{
			"/readFile",
			"other.txt", // Different target
			payload,
		},
	}}

	allowed = executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected action to be denied when target mismatches")
	}

	// 5. Case: No Kernel (Fail Open)
	executorNoKernel := &Executor{
		kernel: nil,
		config: DefaultExecutorConfig(),
	}
	allowed = executorNoKernel.checkSafety(toolCall)
	if !allowed {
		t.Error("Expected action to be allowed (fail open) when kernel is nil")
	}
}

import re

with open("tests/e2e/session_clean_loop_integration_test.go", "r") as f:
    content = f.read()

mock_kernel_code = """
type mockKernel struct {
	asserted []types.Fact
	mu       sync.Mutex
}
func (m *mockKernel) LoadFacts(facts []types.Fact) error { return nil }
func (m *mockKernel) Query(predicate string) ([]types.Fact, error) { return nil, nil }
func (m *mockKernel) QueryAll() (map[string][]types.Fact, error) { return nil, nil }
func (m *mockKernel) Assert(fact types.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asserted = append(m.asserted, fact)
	return nil
}
func (m *mockKernel) AssertBatch(facts []types.Fact) error { return nil }
func (m *mockKernel) Retract(predicate string) error { return nil }
func (m *mockKernel) RetractFact(fact types.Fact) error { return nil }
func (m *mockKernel) UpdateSystemFacts() error { return nil }
func (m *mockKernel) AppendPolicy(policy string) error { return nil }
func (m *mockKernel) Reset() {}
"""

# Replace existing mockKernel
content = re.sub(r'type mockKernel struct \{[\s\S]*?\}[\s\S]*?func \(m \*mockKernel\) Retract\(query string\) error \{ return nil \}', mock_kernel_code, content)

with open("tests/e2e/session_clean_loop_integration_test.go", "w") as f:
    f.write(content)

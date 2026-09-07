package core

import "fmt"

// IntegrationPath pins a runtime producer/consumer representation and the
// independently run behavioral witness required to close that path.
type IntegrationPath struct {
	Name        string
	Input       string
	InputArity  int
	Output      string
	OutputArity int
	Witness     string
}

type IntegrationWitness struct {
	Name     string
	Snapshot string
	Passed   bool
}

// CheckIntegrationPath checks loaded declarations, a connected rule path,
// viable production shard placement, and a witness for the current revision.
// Structural reachability and bounded behavioral evidence remain distinct.
func (m *DerivationMap) CheckIntegrationPath(p IntegrationPath, snapshot string, witnesses []IntegrationWitness) error {
	if m == nil || p.Name == "" || p.Input == "" || p.Output == "" || p.Witness == "" || snapshot == "" {
		return fmt.Errorf("incomplete integration contract")
	}
	for name, arity := range map[string]int{p.Input: p.InputArity, p.Output: p.OutputArity} {
		if got, ok := m.Arities[name]; !ok || got != arity {
			return fmt.Errorf("%s: loaded representation missing or mismatched for %s/%d", p.Name, name, arity)
		}
	}
	seen := map[string]bool{}
	var reaches func(string) bool
	reaches = func(head string) bool {
		if head == p.Input {
			return true
		}
		if seen[head] {
			return false
		}
		seen[head] = true
		for _, r := range m.Rules {
			if r.Head != head || r.Fires.IsEmpty() {
				continue
			}
			for _, term := range append(append([]string(nil), r.Pos...), r.Neg...) {
				if reaches(term) {
					return true
				}
			}
		}
		return false
	}
	if !reaches(p.Output) || m.Presence[p.Output].IsEmpty() {
		return fmt.Errorf("%s: producer cannot reach consumer in the production layout", p.Name)
	}
	for _, w := range witnesses {
		if w.Name == p.Witness && w.Passed && w.Snapshot == snapshot {
			return nil
		}
	}
	return fmt.Errorf("%s: current behavioral witness %q is missing", p.Name, p.Witness)
}

//go:build integration

package e2e_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The fixture's existing explicit ceiling, shared by the factory and scripted
// kernel. Registered names never expand it. This is not production policy.
func mechanicsFixtureTools() []string {
	return []string{
		"read_file", "write_file", "shell_exec", "mock_tool", "blocking_tool", "huge_tool",
		"restricted_tool", "race_tool", "heavy_tool", "leaky_tool", "timing_tool",
	}
}

// Called with the mock's read lock held. Match the actual live turn before
// returning typed rows; queryResults remains the explicit override seam.
func (m *mockKernel) mechanicsCatalogRows(query string) ([]types.Fact, bool) {
	if !strings.HasPrefix(query, "turn_catalog(") || !strings.HasSuffix(query, ", Tool)") {
		return nil, false
	}
	turn := strings.TrimSuffix(strings.TrimPrefix(query, "turn_catalog("), ", Tool)")
	if !strings.HasPrefix(turn, "/turn_") {
		return nil, true
	}
	for _, fact := range m.facts {
		if fact.Predicate != "turn_verb" || len(fact.Args) != 2 || types.ExtractString(fact.Args[0]) != turn {
			continue
		}
		names := mechanicsFixtureTools()
		rows := make([]types.Fact, 0, len(names))
		for _, name := range names {
			rows = append(rows, types.Fact{Predicate: "turn_catalog", Args: []any{types.MangleAtom(turn), types.MangleAtom("/" + name)}})
		}
		return rows, true
	}
	return nil, true
}

// RealKernel.RetractFact matches predicate and first argument, not every row
// of the predicate. A sibling turn's catalog must survive this turn's cleanup.
func (m *mockKernel) retractMechanicsFact(fact types.Fact) error {
	if len(fact.Args) == 0 {
		return fmt.Errorf("fact must have at least one argument for matching")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.facts[:0]
	for _, current := range m.facts {
		if current.Predicate == fact.Predicate && len(current.Args) > 0 && reflect.DeepEqual(current.Args[0], fact.Args[0]) {
			continue
		}
		kept = append(kept, current)
	}
	m.facts = kept
	return nil
}

// These fixtures are nonparallel owners of the process registry binding.
// Every repetition gets its own callbacks; a later owner is never overwritten.
func ownMechanicsFixtureRegistry(t *testing.T) {
	t.Helper()
	registry := tools.NewRegistry()
	restore := tools.SwapGlobal(registry)
	t.Cleanup(func() {
		if tools.Global() == registry {
			restore()
		}
	})
}

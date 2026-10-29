package prompt

import (
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// Remediation for 2026-02-13_05-05-EST_dependency_resolver_boundary_analysis.md
// These tests verify the robustness of the DependencyResolver against boundary conditions.
// ============================================================================

// TestResolveGap_NilSafety verifies that the resolver safely handles nil pointers
// without panicking (Vector A1 & A2).
func TestResolveGap_NilSafety(t *testing.T) {
	resolver := NewDependencyResolver()

	// Vector A1: Nil ScoredAtom
	// Vector A2: ScoredAtom with nil Atom
	atoms := []*ScoredAtom{
		nil,
		{Atom: nil},
		{Atom: &PromptAtom{ID: ""}}, // Missing ID
		{Atom: &PromptAtom{ID: "valid_atom"}, Combined: 0.9},
	}

	result, err := resolver.Resolve(atoms)
	if err != nil {
		t.Fatalf("Resolve failed with error on nil inputs: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 valid atom, got %d", len(result))
	}

	if result[0].Atom.ID != "valid_atom" {
		t.Errorf("Expected valid_atom, got %s", result[0].Atom.ID)
	}
}

// TestSortByCategoryGap_Determinism verifies that sorting unknown categories
// is deterministic (Vector D1).
func TestSortByCategoryGap_Determinism(t *testing.T) {
	resolver := NewDependencyResolver()

	// Create atoms with random custom categories
	ordered := make([]*OrderedAtom, 100)
	for i := range 100 {
		ordered[i] = &OrderedAtom{
			Atom: &PromptAtom{
				ID:       fmt.Sprintf("atom_%d", i),
				Category: AtomCategory(fmt.Sprintf("CustomCat_%d", i%10)),
			},
			Score: float64(i),
		}
	}

	// Sort multiple times and verify the output is identical
	var firstResult string
	for i := range 10 {
		sorted := resolver.SortByCategory(ordered)

		var sb strings.Builder
		for _, sa := range sorted {
			sb.WriteString(sa.Atom.ID)
			sb.WriteString(",")
		}

		currentResult := sb.String()
		if i == 0 {
			firstResult = currentResult
		} else if currentResult != firstResult {
			t.Fatalf("SortByCategory is non-deterministic!\nRun 0: %s\nRun %d: %s", firstResult, i, currentResult)
		}
	}
}

// TestDetectCyclesGap_DeepChain verifies that deep dependency chains don't cause stack overflow (Vector C1).
func TestDetectCyclesGap_DeepChain(t *testing.T) {
	resolver := NewDependencyResolver()

	// Create a chain of 1500 atoms
	// DetectCycles should safely abort the path at depth > 1000 without stack overflow
	chainSize := 1500
	atoms := make([]*PromptAtom, chainSize)
	for i := range chainSize {
		dependsOn := []string{}
		if i < chainSize-1 {
			dependsOn = []string{fmt.Sprintf("atom_%d", i+1)}
		}

		atoms[i] = &PromptAtom{
			ID:        fmt.Sprintf("atom_%d", i),
			DependsOn: dependsOn,
		}
	}

	// This should not panic
	cycle := resolver.DetectCycles(atoms)

	// Because there's no actual cycle, but it exceeds max recursion, it returns false safely
	if cycle != nil {
		t.Errorf("Expected no cycle for linear chain, got %v", cycle)
	}
}

// TestResolveGap_ErrorFormat verifies the formatting of cycle errors (Vector R4).
func TestResolveGap_ErrorFormat(t *testing.T) {
	err := DependencyError{
		AtomID:   "A",
		CycleIDs: []string{"A", "B", "C", "A"},
		Type:     DependencyErrorCycle,
	}

	msg := err.Error()
	if !strings.Contains(msg, "dependency cycle:") {
		t.Errorf("Error message missing 'dependency cycle:': %s", msg)
	}
	if !strings.Contains(msg, "[A B C A]") {
		t.Errorf("Error message missing cycle path: %s", msg)
	}
}

// TestValidateGap_EmptyDepends verifies validation handles empty dependency strings gracefully (Vector A3).
func TestValidateGap_EmptyDepends(t *testing.T) {
	resolver := NewDependencyResolver()

	atoms := []*PromptAtom{
		{
			ID:        "Atom1",
			DependsOn: []string{""}, // Empty dependency
		},
	}

	errors := resolver.ValidateDependencies(atoms)

	if len(errors) != 1 {
		t.Fatalf("Expected 1 validation error, got %d", len(errors))
	}

	if errors[0].MissingDepID != "" {
		t.Errorf("Expected MissingDepID to be empty string, got %s", errors[0].MissingDepID)
	}
}

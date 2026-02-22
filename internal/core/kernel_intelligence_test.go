package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntelligenceWiring(t *testing.T) {
	// Initialize kernel
	k, err := NewRealKernel()
	require.NoError(t, err)

	// Define some facts that should trigger high priority context
	// Path: "critical_file.go" has a critical safety warning
	// Path: "churn_file.go" has high churn > 10

	facts := []Fact{
		{
			Predicate: "intelligence_safety_warning",
			Args:      []interface{}{"camp1", "critical_file.go", "delete", "no_delete", "critical"},
		},
		{
			Predicate: "intelligence_churn_hotspot",
			Args:      []interface{}{"churn_file.go", 15, "high churn"},
		},
	}

	err = k.AssertBatch(facts)
	require.NoError(t, err)

	// Query for context_priority
	// Expect: critical_file.go -> 100 (from /critical)
	// Expect: churn_file.go -> 80 (from /high via intelligence_high_priority_file)

	cpFacts, err := k.Query("context_priority")
	require.NoError(t, err)

	foundCritical := false
	foundChurn := false

	for _, f := range cpFacts {
		if len(f.Args) < 2 {
			continue
		}
		path, ok := f.Args[0].(string)
		if !ok {
			continue
		}

		// Priority might be int, int64 or float64 depending on JSON/Mangle parsing
		var priority int64
		switch v := f.Args[1].(type) {
		case int:
			priority = int64(v)
		case int64:
			priority = v
		case float64:
			priority = int64(v)
		}

		if path == "critical_file.go" {
			if priority == 100 {
				foundCritical = true
			}
		}
		if path == "churn_file.go" {
			// Churn > 10 triggers intelligence_high_priority_file -> /high -> 80
			if priority == 80 {
				foundChurn = true
			}
		}
	}

	if !foundCritical {
		t.Errorf("Expected context_priority('critical_file.go', 100) not found")
	}
	if !foundChurn {
		t.Errorf("Expected context_priority('churn_file.go', 80) not found")
	}
}

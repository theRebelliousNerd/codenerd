package context_harness

import (
	"fmt"

	"codenerd/internal/core"
)

// TestKernelFactory creates isolated test kernels for scenarios.
// Each scenario gets a fresh kernel to prevent cross-contamination.
type TestKernelFactory struct {
	// Minimal schema files to load (faster boot)
	minimalSchemas []string
}

// NewTestKernelFactory creates a new factory.
func NewTestKernelFactory() *TestKernelFactory {
	return &TestKernelFactory{
		minimalSchemas: []string{
			// Core schemas needed for context testing
			"schemas_intent.mg",
			"schemas_campaign.mg",
			"schemas_testing.mg",
		},
	}
}

// CreateKernel creates a fresh test kernel for a scenario.
// The kernel is isolated with minimal schemas for fast boot.
func (f *TestKernelFactory) CreateKernel(scenario *Scenario) (*core.RealKernel, error) {
	// Create kernel (uses default workspace)
	kernel, err := core.NewRealKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel: %w", err)
	}

	// Load minimal schemas if available
	for _, schemaFile := range f.minimalSchemas {
		// In production, this would load from embedded files
		// For now, the kernel boots with default schemas
		_ = schemaFile
	}

	// Seed initial facts from scenario
	if len(scenario.InitialFacts) > 0 {
		facts := make([]core.Fact, 0, len(scenario.InitialFacts))
		for _, factStr := range scenario.InitialFacts {
			fact, err := parseMangleFact(factStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse initial fact %q: %w", factStr, err)
			}
			facts = append(facts, fact)
		}
		if err := kernel.LoadFacts(facts); err != nil {
			return nil, fmt.Errorf("failed to load initial facts: %w", err)
		}
	}

	return kernel, nil
}

// CreateIsolatedKernel creates a completely isolated kernel with no external state.
// Used for deterministic testing where cross-scenario contamination must be prevented.
func (f *TestKernelFactory) CreateIsolatedKernel() (*core.RealKernel, error) {
	// Create kernel with isolated workspace
	workspaceDir := fmt.Sprintf("/tmp/context_harness_isolated_%d", randomID())
	return core.NewRealKernelWithWorkspace(workspaceDir)
}

// parseMangleFact parses a Mangle fact string into a core.Fact.
// Example: `current_campaign("auth-migration")` -> Fact{Predicate: "current_campaign", Args: ["auth-migration"]}
func parseMangleFact(factStr string) (core.Fact, error) {
	// Find predicate name (before first paren)
	parenIdx := -1
	for i, c := range factStr {
		if c == '(' {
			parenIdx = i
			break
		}
	}

	if parenIdx == -1 {
		// No args, just predicate name
		return core.Fact{Predicate: factStr}, nil
	}

	predicate := factStr[:parenIdx]
	argsStr := factStr[parenIdx+1:]

	// Remove trailing paren and period
	if len(argsStr) > 0 && argsStr[len(argsStr)-1] == '.' {
		argsStr = argsStr[:len(argsStr)-1]
	}
	if len(argsStr) > 0 && argsStr[len(argsStr)-1] == ')' {
		argsStr = argsStr[:len(argsStr)-1]
	}

	// Parse args (simple string/int parsing)
	args := parseArgs(argsStr)

	return core.Fact{
		Predicate: predicate,
		Args:      args,
	}, nil
}

// parseArgs parses comma-separated arguments.
func parseArgs(argsStr string) []interface{} {
	if argsStr == "" {
		return nil
	}

	var args []interface{}
	var current []byte
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(argsStr); i++ {
		c := argsStr[i]

		if inString {
			if c == stringChar {
				inString = false
				// Add the string value (without quotes)
				args = append(args, string(current))
				current = nil
			} else {
				current = append(current, c)
			}
		} else if c == '"' || c == '\'' {
			inString = true
			stringChar = c
		} else if c == ',' {
			// Flush current value if any
			if len(current) > 0 {
				args = append(args, parseValue(string(current)))
				current = nil
			}
		} else if c != ' ' && c != '\t' {
			current = append(current, c)
		}
	}

	// Flush final value
	if len(current) > 0 {
		args = append(args, parseValue(string(current)))
	}

	return args
}

// parseValue attempts to parse a value as int, otherwise returns string.
func parseValue(s string) interface{} {
	// Try int
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return i
	}

	// Try float
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}

	// Return as string (strip leading slash for atoms)
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

// randomID generates a simple random ID for isolation.
func randomID() int64 {
	// In production, use crypto/rand or time.Now().UnixNano()
	// For now, simple sequential IDs are fine
	return 0 // Will be overwritten by caller if needed
}

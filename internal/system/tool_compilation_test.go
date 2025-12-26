package system_test

import (
	"codenerd/internal/core"
	"codenerd/internal/tactile"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestVirtualStore_CompilationDelegation(t *testing.T) {
	// Setup
	executor := tactile.NewDirectExecutor()
	vs := core.NewVirtualStore(executor)

	// Create a real kernel for fact assertions
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	// Allow compilation logic to proceed by setting up a dummy tool file if needed
	// But handleOuroborosCompile strictly just checks target name and delegations.
	// It constructs source path but doesn't read it before delegating in the NEW logic.
	// The new logic is:
	// 1. Delegate to ToolGenerator shard.
	// 2. Return.
	// So we don't need a real file on disk for THIS test.

	// Action: Compile
	req := core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"test_compile",      // ID
			"ouroboros_compile", // Type (ActionOuroborosCompile)
			"my_test_tool",      // Target
		},
	}

	// Execute
	output, err := vs.RouteAction(context.Background(), req)
	if err != nil {
		t.Fatalf("RouteAction failed: %v", err)
	}

	// Verify: Check if 'delegation_result' exists in kernel
	allFacts := kernel.GetAllFacts()
	found := false
	for _, f := range allFacts {
		// Use string matching to be safe with Mangle Name constants vs strings
		if f.Predicate == "delegation_result" {
			if len(f.Args) >= 1 {
				arg0 := fmt.Sprintf("%v", f.Args[0])
				if arg0 == "/tool_generator" || arg0 == "tool_generator" || strings.Contains(arg0, "tool_generator") {
					found = true
					break
				}
			}
		}
	}

	if !found {
		// Also check for delegation_failed
		for _, f := range allFacts {
			if f.Predicate == "delegation_failed" {
				if len(f.Args) >= 1 {
					arg0 := fmt.Sprintf("%v", f.Args[0])
					if arg0 == "/tool_generator" || arg0 == "tool_generator" || strings.Contains(arg0, "tool_generator") {
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		allFactsStr := ""
		for _, f := range allFacts {
			// Only show relevant-looking facts
			if strings.Contains(f.Predicate, "delegat") || strings.Contains(f.Predicate, "execution") {
				allFactsStr += fmt.Sprintf("\n - %s", f.String())
			}
		}
		t.Errorf("Expected delegation fact for /tool_generator not found. Core facts: %s", allFactsStr)
	}

	// Double Verify: Check output
	if output != "BaseShardAgent execution" {
		t.Errorf("Expected output 'BaseShardAgent execution', got: '%s'", output)
	}
}

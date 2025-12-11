package core

import (
	"testing"
	"time"
)

func TestToolRegistry_RegisterTool(t *testing.T) {
	registry := NewToolRegistry(".")
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	registry.SetKernel(kernel)

	// Register a tool
	err = registry.RegisterTool("test_tool", "test_command", "/coder")
	if err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}

	// Verify tool is registered
	tool, exists := registry.GetTool("test_tool")
	if !exists {
		t.Fatal("Tool not found after registration")
	}
	if tool.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", tool.Name)
	}
	if tool.ShardAffinity != "/coder" {
		t.Errorf("Expected affinity '/coder', got '%s'", tool.ShardAffinity)
	}

	// Verify facts were injected into kernel
	if err := kernel.Evaluate(); err != nil {
		t.Fatalf("Kernel evaluation failed: %v", err)
	}

	// Query for registered_tool fact
	facts, err := kernel.Query("registered_tool")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("No registered_tool facts found in kernel")
	}

	// Query for tool_available
	facts, err = kernel.Query("tool_available")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("No tool_available facts found in kernel")
	}
}

func TestToolRegistry_GetToolsForShard(t *testing.T) {
	registry := NewToolRegistry(".")

	// Register tools with different affinities
	_ = registry.RegisterTool("coder_tool", "coder_cmd", "/coder")
	_ = registry.RegisterTool("tester_tool", "tester_cmd", "/tester")
	_ = registry.RegisterTool("universal_tool", "universal_cmd", "/all")

	// Get tools for coder shard
	coderTools := registry.GetToolsForShard("/coder")
	if len(coderTools) != 2 { // coder_tool + universal_tool
		t.Errorf("Expected 2 tools for /coder, got %d", len(coderTools))
	}

	// Get tools for tester shard
	testerTools := registry.GetToolsForShard("/tester")
	if len(testerTools) != 2 { // tester_tool + universal_tool
		t.Errorf("Expected 2 tools for /tester, got %d", len(testerTools))
	}

	// Get tools for reviewer shard
	reviewerTools := registry.GetToolsForShard("/reviewer")
	if len(reviewerTools) != 1 { // only universal_tool
		t.Errorf("Expected 1 tool for /reviewer, got %d", len(reviewerTools))
	}
}

func TestToolRegistry_UnregisterTool(t *testing.T) {
	registry := NewToolRegistry(".")
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	registry.SetKernel(kernel)

	// Register and then unregister
	_ = registry.RegisterTool("temp_tool", "temp_cmd", "/all")
	_ = kernel.Evaluate()

	err = registry.UnregisterTool("temp_tool")
	if err != nil {
		t.Fatalf("UnregisterTool failed: %v", err)
	}

	// Verify tool is gone
	_, exists := registry.GetTool("temp_tool")
	if exists {
		t.Fatal("Tool still exists after unregistration")
	}
}

func TestToolRegistry_RegisterToolWithInfo(t *testing.T) {
	registry := NewToolRegistry(".")
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	registry.SetKernel(kernel)

	tool := &Tool{
		Name:          "full_tool",
		Command:       "full_cmd",
		ShardAffinity: "/researcher",
		Description:   "A full tool with metadata",
		Capabilities:  []string{"search", "fetch"},
		Hash:          "abc123",
		RegisteredAt:  time.Now(),
		ExecuteCount:  42,
	}

	err = registry.RegisterToolWithInfo(tool)
	if err != nil {
		t.Fatalf("RegisterToolWithInfo failed: %v", err)
	}

	// Verify all metadata preserved
	retrieved, exists := registry.GetTool("full_tool")
	if !exists {
		t.Fatal("Tool not found")
	}
	if retrieved.Description != "A full tool with metadata" {
		t.Errorf("Description mismatch")
	}
	if len(retrieved.Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(retrieved.Capabilities))
	}
	if retrieved.Hash != "abc123" {
		t.Errorf("Hash mismatch")
	}
	if retrieved.ExecuteCount != 42 {
		t.Errorf("ExecuteCount mismatch")
	}

	// Verify capability facts were injected
	_ = kernel.Evaluate()
	facts, err := kernel.Query("tool_capability")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) != 2 {
		t.Errorf("Expected 2 tool_capability facts, got %d", len(facts))
	}
}

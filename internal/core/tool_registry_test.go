package core

import (
	"context"
	"strings"
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

	// F-TOOLREG-1: tool_registered second arg must be epoch seconds (/number per schemas_tools.mg:14), not a string.
	facts, err = kernel.Query("tool_registered")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("No tool_registered facts found in kernel")
	}
	var found bool
	expected := tool.RegisteredAt.Unix()
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		name, ok := f.Args[0].(string)
		if !ok || name != "test_tool" {
			continue
		}
		found = true
		if s, ok := f.Args[1].(string); ok {
			t.Fatalf("tool_registered second arg is string %q, want number %d", s, expected)
		}
		var got int64
		switch v := f.Args[1].(type) {
		case int64:
			got = v
		case int:
			got = int64(v)
		case int32:
			got = int64(v)
		case uint64:
			got = int64(v)
		case float64:
			got = int64(v)
		default:
			t.Fatalf("tool_registered second arg has unexpected type %T, want number %d", f.Args[1], expected)
		}
		if got != expected {
			t.Errorf("tool_registered timestamp = %d, want %d", got, expected)
		}
	}
	if !found {
		t.Fatal("No tool_registered fact found for test_tool")
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

	// F-TOOLREG-1: tool_registered second arg must be epoch seconds (/number per schemas_tools.mg:14), not a string.
	facts, err = kernel.Query("tool_registered")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("No tool_registered facts found in kernel")
	}
	var found bool
	expected := tool.RegisteredAt.Unix()
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		name, ok := f.Args[0].(string)
		if !ok || name != "full_tool" {
			continue
		}
		found = true
		if s, ok := f.Args[1].(string); ok {
			t.Fatalf("tool_registered second arg is string %q, want number %d", s, expected)
		}
		var got int64
		switch v := f.Args[1].(type) {
		case int64:
			got = v
		case int:
			got = int64(v)
		case int32:
			got = int64(v)
		case uint64:
			got = int64(v)
		case float64:
			got = int64(v)
		default:
			t.Fatalf("tool_registered second arg has unexpected type %T, want number %d", f.Args[1], expected)
		}
		if got != expected {
			t.Errorf("tool_registered timestamp = %d, want %d", got, expected)
		}
	}
	if !found {
		t.Fatal("No tool_registered fact found for full_tool")
	}
}

func TestToolRegistry_SecureValidateCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"Valid command", "ls", false},
		{"Valid absolute command", "/usr/bin/ls", false},
		{"Path traversal", "../../bin/ls", true},
		{"Forbidden shell bash", "bash", true},
		{"Forbidden shell sh absolute", "/bin/sh", true},
		{"Forbidden shell cmd.exe", "cmd.exe", true},
		{"Forbidden shell powershell", "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", true},
		{"Valid tool containing shell name", "bashing_tool", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := secureValidateCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("secureValidateCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolRegistry_SecureValidateArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"Valid args", []string{"-l", "-a"}, false},
		{"Null byte in first arg", []string{"-l\x00", "-a"}, true},
		{"Null byte in second arg", []string{"-l", "malicious\x00arg"}, true},
		{"Empty args", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := secureValidateArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("secureValidateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolRegistry_ExecuteRegisteredTool_Validation(t *testing.T) {
	registry := NewToolRegistry(".")

	// Register a malicious tool (using a shell directly)
	tool := &Tool{
		Name:          "malicious_tool",
		Command:       "/bin/bash",
		ShardAffinity: "/all",
	}
	registry.tools["malicious_tool"] = tool

	_, err := registry.ExecuteRegisteredTool(context.Background(), "malicious_tool", []string{"-c", "echo vulnerable"})
	if err == nil {
		t.Fatal("Expected error when executing forbidden shell, got nil")
	}
	if !strings.Contains(err.Error(), "security violation") {
		t.Errorf("Expected security violation error, got: %v", err)
	}

	// Register a tool with null bytes in arguments
	tool2 := &Tool{
		Name:          "echo_tool",
		Command:       "echo",
		ShardAffinity: "/all",
	}
	registry.tools["echo_tool"] = tool2

	_, err = registry.ExecuteRegisteredTool(context.Background(), "echo_tool", []string{"hello\x00world"})
	if err == nil {
		t.Fatal("Expected error when executing with null byte arg, got nil")
	}
	if !strings.Contains(err.Error(), "security violation") {
		t.Errorf("Expected security violation error, got: %v", err)
	}
}

func TestToolRegistry_ProductionToolsCarryVocabularyCapabilities(t *testing.T) {
	registry := NewToolRegistry(".")
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	registry.SetKernel(kernel)

	// Production-like tools through the plain registration path, which
	// previously set no Capabilities at all. Names/commands carry distinct
	// effects so the single mapping must yield distinct vocabulary caps.
	plainTools := []struct {
		name     string
		command  string
		affinity string
	}{
		{"prod_generate", "prod_generate_cmd", "/all"},
		{"prod_debug_test", "prod_debug_test_cmd", "/all"},
		{"prod_inspect", "prod_inspect_cmd", "/all"},
	}
	for _, pt := range plainTools {
		if err := registry.RegisterTool(pt.name, pt.command, pt.affinity); err != nil {
			t.Fatalf("RegisterTool(%s) failed: %v", pt.name, err)
		}
	}

	// Production-like tool through the info path with a legacy raw category
	// string ("build") instead of vocabulary caps. The single mapping must
	// translate it into the routing vocabulary.
	legacyTool := &Tool{
		Name:          "prod_build_tool",
		Command:       "prod_build_cmd",
		ShardAffinity: "/all",
		Description:   "Builds the project binary",
		Capabilities:  []string{"build"},
		RegisteredAt:  time.Now(),
	}
	if err := registry.RegisterToolWithInfo(legacyTool); err != nil {
		t.Fatalf("RegisterToolWithInfo failed: %v", err)
	}

	expectedTools := []string{"prod_generate", "prod_debug_test", "prod_inspect", "prod_build_tool"}

	if err := kernel.Evaluate(); err != nil {
		t.Fatalf("Kernel evaluation failed: %v", err)
	}

	allowed := map[string]bool{
		"/generation": true, "/debugging": true, "/transformation": true,
		"/inspection": true, "/validation": true, "/execution": true,
		"/analysis": true, "/knowledge": true,
	}

	capFacts, err := kernel.Query("tool_capability")
	if err != nil {
		t.Fatalf("Query tool_capability failed: %v", err)
	}
	if len(capFacts) == 0 {
		t.Fatal("No tool_capability facts found for registered production tools")
	}
	capsByTool := map[string]map[string]bool{}
	for _, f := range capFacts {
		if len(f.Args) < 2 {
			continue
		}
		name, ok := f.Args[0].(string)
		if !ok {
			t.Fatalf("tool_capability first arg has unexpected type %T, want string", f.Args[0])
		}
		cap, ok := f.Args[1].(string)
		if !ok {
			t.Fatalf("tool_capability second arg has unexpected type %T, want string", f.Args[1])
		}
		if !allowed[cap] {
			t.Errorf("tool_capability(%q, %q) not in routing vocabulary", name, cap)
		}
		if capsByTool[name] == nil {
			capsByTool[name] = map[string]bool{}
		}
		capsByTool[name][cap] = true
	}
	for _, name := range expectedTools {
		if len(capsByTool[name]) == 0 {
			t.Errorf("registered tool %q carries no tool_capability fact in routing vocabulary", name)
		}
	}

	relFacts, err := kernel.Query("relevant_tool")
	if err != nil {
		t.Fatalf("Query relevant_tool failed: %v", err)
	}
	byShard := map[string]map[string]bool{
		"/coder": {}, "/tester": {}, "/reviewer": {},
	}
	for _, f := range relFacts {
		if len(f.Args) < 2 {
			continue
		}
		shard, ok := f.Args[0].(string)
		if !ok {
			continue
		}
		tool, ok := f.Args[1].(string)
		if !ok {
			continue
		}
		if _, want := byShard[shard]; !want {
			continue
		}
		byShard[shard][tool] = true
	}
	for shard, set := range byShard {
		if len(set) == 0 {
			t.Errorf("relevant_tool(%s, _) derived nothing on a real kernel with the shipped corpus", shard)
		}
	}
	equalSets := func(a, b map[string]bool) bool {
		if len(a) != len(b) {
			return false
		}
		for k := range a {
			if !b[k] {
				return false
			}
		}
		return true
	}
	if equalSets(byShard["/coder"], byShard["/tester"]) && equalSets(byShard["/tester"], byShard["/reviewer"]) {
		t.Errorf("relevant_tool sets for /coder, /tester, /reviewer are identical; routing never differs (coder=%v tester=%v reviewer=%v)",
			byShard["/coder"], byShard["/tester"], byShard["/reviewer"])
	}
}

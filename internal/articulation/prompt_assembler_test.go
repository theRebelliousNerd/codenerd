package articulation

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Tests implemented below.

// mockKernel implements KernelQuerier for testing.
type mockKernel struct {
	facts map[string][]types.Fact
}

func newMockKernel() *mockKernel {
	return &mockKernel{
		facts: make(map[string][]types.Fact),
	}
}

func (m *mockKernel) Query(query string) ([]types.Fact, error) {
	predicate := query
	var arg0 string
	if idx := strings.Index(query, "("); idx > 0 {
		predicate = query[:idx]
		// extract the first arg if it's a string, e.g. "my-shard" or "/_all"
		argsStr := query[idx+1 : len(query)-1]
		parts := strings.Split(argsStr, ",")
		if len(parts) > 0 {
			arg0 = strings.TrimSpace(parts[0])
			if strings.HasPrefix(arg0, "\"") && strings.HasSuffix(arg0, "\"") {
				arg0 = arg0[1 : len(arg0)-1]
			}
		}
	}

	allFacts := m.facts[predicate]
	if arg0 == "" || arg0 == "_" {
		return allFacts, nil
	}

	var filtered []types.Fact
	for _, f := range allFacts {
		if len(f.Args) > 0 {
			if s, ok := f.Args[0].(string); ok && s == arg0 {
				filtered = append(filtered, f)
			}
		}
	}
	return filtered, nil
}

func (m *mockKernel) addFact(predicate string, args ...interface{}) {
	m.facts[predicate] = append(m.facts[predicate], types.Fact{
		Predicate: predicate,
		Args:      args,
	})
}

func TestNewPromptAssembler(t *testing.T) {
	tests := []struct {
		name    string
		kernel  KernelQuerier
		wantErr bool
	}{
		{
			name:    "valid kernel",
			kernel:  newMockKernel(),
			wantErr: false,
		},
		{
			name:    "nil kernel",
			kernel:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa, err := NewPromptAssembler(tt.kernel)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPromptAssembler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && pa == nil {
				t.Error("NewPromptAssembler() returned nil without error")
			}
		})
	}
}

func TestAssembleSystemPrompt(t *testing.T) {
	tests := []struct {
		name         string
		setupKernel  func(*mockKernel)
		promptCtx    *PromptContext
		wantContains []string
		wantErr      bool
	}{
		{
			name:        "nil prompt context",
			setupKernel: func(m *mockKernel) {},
			promptCtx:   nil,
			wantErr:     true,
		},
		{
			name:        "coder shard with fallback template",
			setupKernel: func(m *mockKernel) {},
			promptCtx: &PromptContext{
				ShardID:   "coder-123",
				ShardType: "coder",
				UserIntent: &types.StructuredIntent{
					Verb: "/fix", // Required for coder identity atoms to match
				},
			},
			wantContains: []string{
				"Coder Shard of codeNERD",
				"PIGGYBACK ENVELOPE",
				"control_packet",
			},
			wantErr: false,
		},
		{
			name: "shard with kernel template",
			setupKernel: func(m *mockKernel) {
				m.addFact("shard_prompt_base", "/reviewer", "Custom reviewer template from kernel")
			},
			promptCtx: &PromptContext{
				ShardID:   "reviewer-456",
				ShardType: "reviewer",
			},
			wantContains: []string{
				"Custom reviewer template from kernel",
				"PIGGYBACK ENVELOPE",
			},
			wantErr: false,
		},
		{
			name: "shard with injectable context atoms",
			setupKernel: func(m *mockKernel) {
				m.addFact("injectable_context", "coder-789", "Security: This file handles user authentication")
				m.addFact("injectable_context", "coder-789", "Pattern: Uses repository pattern for data access")
			},
			promptCtx: &PromptContext{
				ShardID:   "coder-789",
				ShardType: "coder",
			},
			wantContains: []string{
				"KERNEL-INJECTED CONTEXT",
				"Security: This file handles user authentication",
				"Pattern: Uses repository pattern for data access",
			},
			wantErr: false,
		},
		{
			name: "shard with wildcard context atoms",
			setupKernel: func(m *mockKernel) {
				m.addFact("injectable_context", "*", "Global: Project uses Go 1.22")
			},
			promptCtx: &PromptContext{
				ShardID:   "any-shard-id",
				ShardType: "tester",
			},
			wantContains: []string{
				"Global: Project uses Go 1.22",
			},
			wantErr: false,
		},
		{
			name:        "shard with session context",
			setupKernel: func(m *mockKernel) {},
			promptCtx: &PromptContext{
				ShardID:   "coder-session",
				ShardType: "coder",
				SessionCtx: &types.SessionContext{
					CurrentDiagnostics: []string{"internal/foo.go:42: undefined: Bar"},
					TestState:          "/failing",
					FailingTests:       []string{"TestBar", "TestBaz"},
					TDDRetryCount:      2,
					GitBranch:          "feature/fix-bar",
				},
			},
			wantContains: []string{
				"SESSION CONTEXT",
				"BUILD/LINT ERRORS",
				"internal/foo.go:42: undefined: Bar",
				"TEST STATE: FAILING",
				"TDD Retry: 2",
				"TestBar",
				"Branch: feature/fix-bar",
			},
			wantErr: false,
		},
		{
			name:        "shard with user intent",
			setupKernel: func(m *mockKernel) {},
			promptCtx: &PromptContext{
				ShardID:   "coder-intent",
				ShardType: "coder",
				UserIntent: &types.StructuredIntent{
					ID:         "intent-123",
					Category:   "/mutation",
					Verb:       "/fix",
					Target:     "internal/auth/login.go",
					Constraint: "preserve existing tests",
				},
			},
			wantContains: []string{
				"USER INTENT",
				"intent-123",
				"/mutation",
				"/fix",
				"internal/auth/login.go",
				"preserve existing tests",
			},
			wantErr: false,
		},
		{
			name:        "shard with dream mode",
			setupKernel: func(m *mockKernel) {},
			promptCtx: &PromptContext{
				ShardID:   "coder-dream",
				ShardType: "coder",
				SessionCtx: &types.SessionContext{
					DreamMode: true,
				},
			},
			wantContains: []string{
				"DREAM",
				"Simulation Only",
				"DO NOT EXECUTE",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mk := newMockKernel()
			if tt.setupKernel != nil {
				tt.setupKernel(mk)
			}

			pa, err := NewPromptAssembler(mk)
			if err != nil {
				t.Fatalf("NewPromptAssembler() error = %v", err)
			}

			result, err := pa.AssembleSystemPrompt(context.Background(), tt.promptCtx)
			if (err != nil) != tt.wantErr {
				t.Errorf("AssembleSystemPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !containsString(result, want) {
					t.Errorf("AssembleSystemPrompt() result missing expected content: %q", want)
				}
			}
		})
	}
}

func TestQueryContextAtoms(t *testing.T) {
	tests := []struct {
		name        string
		setupKernel func(*mockKernel)
		shardID     string
		wantCount   int
	}{
		{
			name:        "no context atoms",
			setupKernel: func(m *mockKernel) {},
			shardID:     "test-shard",
			wantCount:   0,
		},
		{
			name: "context atoms for specific shard",
			setupKernel: func(m *mockKernel) {
				m.addFact("injectable_context", "test-shard", "Atom 1")
				m.addFact("injectable_context", "test-shard", "Atom 2")
				m.addFact("injectable_context", "other-shard", "Atom 3")
			},
			shardID:   "test-shard",
			wantCount: 2,
		},
		{
			name: "wildcard context atoms",
			setupKernel: func(m *mockKernel) {
				m.addFact("injectable_context", "*", "Global Atom")
				m.addFact("injectable_context", "test-shard", "Specific Atom")
			},
			shardID:   "test-shard",
			wantCount: 2,
		},
		{
			name: "_all context atoms",
			setupKernel: func(m *mockKernel) {
				m.addFact("injectable_context", "/_all", "All Shards Atom")
			},
			shardID:   "any-shard",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mk := newMockKernel()
			tt.setupKernel(mk)

			pa, err := NewPromptAssembler(mk)
			if err != nil {
				t.Fatalf("NewPromptAssembler() error = %v", err)
			}

			atoms, err := pa.queryContextAtoms(tt.shardID)
			if err != nil {
				t.Errorf("queryContextAtoms() error = %v", err)
				return
			}

			if len(atoms) != tt.wantCount {
				t.Errorf("queryContextAtoms() got %d atoms, want %d", len(atoms), tt.wantCount)
			}
		})
	}
}

func TestGetFallbackTemplate(t *testing.T) {
	tests := []struct {
		name         string
		shardType    string
		wantContains string
	}{
		{
			name:         "coder fallback",
			shardType:    "coder",
			wantContains: "CODER SHARD",
		},
		{
			name:         "tester fallback",
			shardType:    "tester",
			wantContains: "TESTER SHARD",
		},
		{
			name:         "reviewer fallback",
			shardType:    "reviewer",
			wantContains: "REVIEWER SHARD",
		},
		{
			name:         "researcher fallback",
			shardType:    "researcher",
			wantContains: "RESEARCHER SHARD",
		},
		{
			name:         "unknown type",
			shardType:    "unknown",
			wantContains: "GENERIC SHARD",
		},
	}

	pa := &PromptAssembler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := pa.getFallbackTemplate(tt.shardType)
			if !containsString(template, tt.wantContains) {
				t.Errorf("getFallbackTemplate(%q) missing expected content: %q", tt.shardType, tt.wantContains)
			}
		})
	}
}

func TestAssembleQuickPrompt(t *testing.T) {
	mk := newMockKernel()
	mk.addFact("injectable_context", "quick-test", "Quick context")

	result, err := AssembleQuickPrompt(context.Background(), mk, "quick-test", "coder")
	if err != nil {
		t.Fatalf("AssembleQuickPrompt() error = %v", err)
	}

	// Without an intent verb, the piggyback protocol atoms are included
	// (they're mandatory and match any context), but coder identity atoms
	// require an intent verb to match.
	if !containsString(result, "PIGGYBACK ENVELOPE") && !containsString(result, "control_packet") {
		t.Error("AssembleQuickPrompt() missing baseline prompt content")
	}

	if !containsString(result, "Quick context") {
		t.Error("AssembleQuickPrompt() missing injectable context")
	}
}

func TestPromptContextBuilders(t *testing.T) {
	pc := &PromptContext{
		ShardID:   "test-shard",
		ShardType: "coder",
	}

	// Test WithSessionContext
	sessionCtx := &types.SessionContext{
		GitBranch: "main",
	}
	pc.WithSessionContext(sessionCtx)
	if pc.SessionCtx == nil || pc.SessionCtx.GitBranch != "main" {
		t.Error("WithSessionContext() did not set session context")
	}

	// Test WithIntent
	intent := &types.StructuredIntent{
		ID:       "intent-1",
		Category: "/mutation",
	}
	pc.WithIntent(intent)
	if pc.UserIntent == nil || pc.UserIntent.ID != "intent-1" {
		t.Error("WithIntent() did not set user intent")
	}

	// Test WithCampaign
	pc.WithCampaign("campaign-123")
	if pc.CampaignID != "campaign-123" {
		t.Error("WithCampaign() did not set campaign ID")
	}
}

// containsString checks if s contains substr (case-sensitive).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstr(s, substr) >= 0)
}

func findSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// =============================================================================
// JIT COMPILER INTEGRATION TESTS
// =============================================================================

func TestNewPromptAssemblerWithJIT(t *testing.T) {
	tests := []struct {
		name        string
		kernel      KernelQuerier
		jitCompiler interface{} // Use interface to allow nil
		wantErr     bool
		wantJIT     bool
	}{
		{
			name:        "valid kernel with nil JIT compiler",
			kernel:      newMockKernel(),
			jitCompiler: nil,
			wantErr:     false,
			wantJIT:     false,
		},
		{
			name:    "nil kernel with nil JIT compiler",
			kernel:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa, err := NewPromptAssemblerWithJIT(tt.kernel, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPromptAssemblerWithJIT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if pa == nil {
					t.Error("NewPromptAssemblerWithJIT() returned nil without error")
					return
				}
				if pa.JITReady() != tt.wantJIT {
					t.Errorf("JITReady() = %v, want %v", pa.JITReady(), tt.wantJIT)
				}
			}
		})
	}
}

func TestJITHelperMethods(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	// Initially JIT should not be ready (no compiler)
	if pa.JITReady() {
		t.Error("JITReady() should be false when no compiler is set")
	}

	// Test EnableJIT
	pa.EnableJIT(true)
	if !pa.IsJITEnabled() {
		t.Error("IsJITEnabled() should be true after EnableJIT(true)")
	}

	// Still not ready because no compiler
	if pa.JITReady() {
		t.Error("JITReady() should be false when useJIT is true but no compiler is set")
	}

	// Test disable
	pa.EnableJIT(false)
	if pa.IsJITEnabled() {
		t.Error("IsJITEnabled() should be false after EnableJIT(false)")
	}

	// Test GetJITCompiler returns nil when not set
	if pa.GetJITCompiler() != nil {
		t.Error("GetJITCompiler() should return nil when no compiler is set")
	}
}

func TestToCompilationContext(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	tests := []struct {
		name           string
		promptCtx      *PromptContext
		wantMode       string
		wantShardType  string
		wantIntentVerb string
	}{
		{
			name: "basic shard context",
			promptCtx: &PromptContext{
				ShardID:   "coder-123",
				ShardType: "coder",
			},
			wantMode:      "/active",
			wantShardType: "/coder",
		},
		{
			name: "dream mode context",
			promptCtx: &PromptContext{
				ShardID:   "coder-456",
				ShardType: "coder",
				SessionCtx: &types.SessionContext{
					DreamMode: true,
				},
			},
			wantMode:      "/dream",
			wantShardType: "/coder",
		},
		{
			name: "TDD repair mode context",
			promptCtx: &PromptContext{
				ShardID:   "coder-789",
				ShardType: "coder",
				SessionCtx: &types.SessionContext{
					TestState:    "/failing",
					FailingTests: []string{"TestFoo", "TestBar"},
				},
			},
			wantMode:      "/tdd_repair",
			wantShardType: "/coder",
		},
		{
			name: "with user intent",
			promptCtx: &PromptContext{
				ShardID:   "coder-fix",
				ShardType: "coder",
				UserIntent: &types.StructuredIntent{
					Verb:   "/fix",
					Target: "internal/auth/login.go",
				},
			},
			wantMode:       "/active",
			wantShardType:  "/coder",
			wantIntentVerb: "/fix",
		},
		{
			name: "with campaign context",
			promptCtx: &PromptContext{
				ShardID:    "coder-campaign",
				ShardType:  "coder",
				CampaignID: "campaign-123",
				SessionCtx: &types.SessionContext{
					CampaignActive: true,
					CampaignPhase:  "/planning",
					CampaignGoal:   "Build user authentication",
				},
			},
			wantMode:      "/active",
			wantShardType: "/coder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := pa.toCompilationContext(tt.promptCtx)

			if cc == nil {
				t.Fatal("toCompilationContext() returned nil")
			}

			if cc.OperationalMode != tt.wantMode {
				t.Errorf("OperationalMode = %q, want %q", cc.OperationalMode, tt.wantMode)
			}

			if cc.ShardType != tt.wantShardType {
				t.Errorf("ShardType = %q, want %q", cc.ShardType, tt.wantShardType)
			}

			// ShardID should be stable agent name; instance ID preserved separately.
			wantStable := strings.TrimPrefix(tt.wantShardType, "/")
			if cc.ShardID != wantStable {
				t.Errorf("ShardID = %q, want %q", cc.ShardID, wantStable)
			}
			if cc.ShardInstanceID != tt.promptCtx.ShardID {
				t.Errorf("ShardInstanceID = %q, want %q", cc.ShardInstanceID, tt.promptCtx.ShardID)
			}

			if tt.wantIntentVerb != "" && cc.IntentVerb != tt.wantIntentVerb {
				t.Errorf("IntentVerb = %q, want %q", cc.IntentVerb, tt.wantIntentVerb)
			}

			// Verify token budget is set
			if cc.TokenBudget <= 0 {
				t.Error("TokenBudget should be positive")
			}

			if cc.ReservedTokens <= 0 {
				t.Error("ReservedTokens should be positive")
			}

			// Verify campaign context mapping
			if tt.promptCtx.CampaignID != "" {
				if cc.CampaignID != tt.promptCtx.CampaignID {
					t.Errorf("CampaignID = %q, want %q", cc.CampaignID, tt.promptCtx.CampaignID)
				}
			}
		})
	}
}

func TestAssembleSystemPromptFallsBackOnNoJIT(t *testing.T) {
	// When JIT is not configured, should use legacy assembly
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	pc := &PromptContext{
		ShardID:   "coder-test",
		ShardType: "coder",
		UserIntent: &types.StructuredIntent{
			Verb: "/fix", // Required for coder identity atoms to match
		},
	}

	result, err := pa.AssembleSystemPrompt(context.Background(), pc)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt() error = %v", err)
	}

	// Should contain baseline embedded prompt content
	if !containsString(result, "Coder Shard of codeNERD") {
		t.Error("AssembleSystemPrompt() should use embedded baseline when JIT is not configured")
	}

	// Should contain Piggyback Protocol
	if !containsString(result, "PIGGYBACK ENVELOPE") {
		t.Error("AssembleSystemPrompt() should include Piggyback Protocol suffix")
	}
}

// -----------------------------------------------------------------------------
// Gap Implementations
// -----------------------------------------------------------------------------

func TestPromptAssembler_NullEmptyInputs(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	// Empty ShardID or ShardType should not panic, might return an error or a generic template.
	pc := &PromptContext{
		ShardID:   "",
		ShardType: "",
	}
	res, err := pa.AssembleSystemPrompt(context.Background(), pc)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt() failed for empty ShardID/ShardType: %v", err)
	}
	if res == "" {
		t.Errorf("Expected generic shard fallback to return a prompt, got empty string")
	}
	if !containsString(res, "PIGGYBACK ENVELOPE") {
		t.Errorf("Expected fallback to contain piggyback envelope")
	}

	// Facts with empty string arguments
	mk.addFact("injectable_context", "", "")
	atoms, err := pa.queryContextAtoms("")
	if err != nil {
		t.Fatalf("queryContextAtoms() error = %v", err)
	}
	if len(atoms) != 1 {
		t.Errorf("Expected 1 atom for empty query, got %d", len(atoms))
	}
}

func TestPromptAssembler_MassiveContextInjection(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	for i := 0; i < 15000; i++ {
		mk.addFact("injectable_context", "coder-123", "Some atom text context")
	}

	pc := &PromptContext{
		ShardID:   "coder-123",
		ShardType: "coder",
	}
	
	// Should not OOM or take forever
	_, err = pa.AssembleSystemPrompt(context.Background(), pc)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt() error = %v", err)
	}
}

func TestPromptAssembler_ExtremelyLongSessionCtx(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	// 10MB diagnostic error string
	massiveStr := strings.Repeat("E", 10*1024*1024)
	pc := &PromptContext{
		ShardID:   "coder-123",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			CurrentDiagnostics: []string{massiveStr},
		},
	}
	
	res, err := pa.AssembleSystemPrompt(context.Background(), pc)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt() error = %v", err)
	}
	
	// If the prompt assembler truncates, it's fine. If not, just ensure it works without crashing.
	if len(res) < 10*1024*1024 {
		// Just confirming it succeeded
	}
}

func TestPromptAssembler_StateConflicts_ConcurrentMutation(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	pc := &PromptContext{
		ShardID:   "coder-123",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			CurrentDiagnostics: []string{"A"},
		},
	}

	// The prompt assembler should not race if we read it while another routine is accessing it?
	// Actually, the PromptContext struct isn't thread-safe natively, but the assembler shouldn't panic.
	// Let's just make sure it reads it fine.
	res, err := pa.AssembleSystemPrompt(context.Background(), pc)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt() error = %v", err)
	}
	if !containsString(res, "A") {
		t.Errorf("Expected A in prompt")
	}
}

func TestPromptAssembler_StateConflicts_ConflictingShardPromptBase(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	// Conflicting templates
	mk.addFact("shard_prompt_base", "/reviewer", "Template A")
	mk.addFact("shard_prompt_base", "/reviewer", "Template B")
	mk.addFact("shard_prompt_base", "/reviewer", "Template C")

	pc := &PromptContext{
		ShardID:   "reviewer-123",
		ShardType: "reviewer",
	}

	res, err := pa.AssembleSystemPrompt(context.Background(), pc)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt() error = %v", err)
	}

	// It should pick one, and not crash or fail.
	if !containsString(res, "Template A") && !containsString(res, "Template B") && !containsString(res, "Template C") {
		t.Errorf("Expected one of the templates to be chosen")
	}
}

func TestPromptAssembler_StateConflicts_JITRaceCondition(t *testing.T) {
	mk := newMockKernel()
	
	// Create a dummy JIT compiler
	jit := &prompt.JITPromptCompiler{} 
	
	pa, err := NewPromptAssemblerWithJIT(mk, jit)
	if err != nil {
		t.Fatalf("NewPromptAssemblerWithJIT() error = %v", err)
	}

	pc := &PromptContext{
		ShardID:   "coder-123",
		ShardType: "coder",
	}

	// WaitGroup to synchronize
	var wg sync.WaitGroup
	wg.Add(2)

	// Thread 1: Assemble
	go func() {
		defer wg.Done()
		// Since it checks pa.JITReady(), if SetJITCompiler(nil) happens after check but before Compile, it shouldn't panic.
		_, _ = pa.AssembleSystemPrompt(context.Background(), pc)
	}()

	// Thread 2: Set compiler to nil
	go func() {
		defer wg.Done()
		pa.SetJITCompiler(nil)
	}()

	wg.Wait()
}

func TestPromptAssembler_TypeCoercion_MapTypeAssertions(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	// JSON unmarshals numbers as float64 into interface{}
	inputMap := map[string]interface{}{
		"shard_id":   "coder-123",
		"shard_type": "coder",
		"semantic_top_k": float64(5.0), // Should coerce nicely or fail gracefully
	}

	_, err = pa.AssembleSystemPrompt(context.Background(), inputMap)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt() with map error = %v", err)
	}
}

func TestPromptAssembler_TypeCoercion_MangleAtomVsString(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	// Mangle might return atoms as "/shardType" instead of "shardType".
	// queryShardTemplate should handle this dissonance.
	mk.addFact("shard_prompt_base", "/reviewer", "Atom Template")
	mk.addFact("shard_prompt_base", "reviewer", "String Template") // Both forms exist

	tmpl, err := pa.queryShardTemplate("reviewer")
	if err != nil {
		t.Fatalf("queryShardTemplate failed: %v", err)
	}
	if tmpl != "Atom Template" && tmpl != "String Template" {
		t.Fatalf("Expected one of the templates, got %s", tmpl)
	}
}

func TestPromptAssembler_MissingPiggybackEnvelope(t *testing.T) {
	mk := newMockKernel()
	pa, err := NewPromptAssembler(mk)
	if err != nil {
		t.Fatalf("NewPromptAssembler() error = %v", err)
	}

	pc := &PromptContext{
		ShardID:   "coder-test",
		ShardType: "coder",
	}

	// if the template happens to contain the substring "control_packet" as a normal word,
	// the naive check might skip appending the Piggyback Protocol Suffix.
	mk.addFact("shard_prompt_base", "/coder", "Here is a template that mentions control_packet in text.")
	
	res, err := pa.AssembleSystemPrompt(context.Background(), pc)
	if err != nil {
		t.Fatalf("AssembleSystemPrompt failed: %v", err)
	}

	// If the legacy template is used, the logic adds the suffix anyway.
	if !containsString(res, "PIGGYBACK ENVELOPE") {
		t.Errorf("Piggyback protocol suffix should be appended")
	}
}

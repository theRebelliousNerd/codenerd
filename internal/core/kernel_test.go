package core

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain wires goleak.VerifyTestMain so leak detection runs unconditionally
// at end of suite (the previous goleak.Find call skipped leak detection on
// test failure, which masked real leaks).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// go.opencensus.io spawns a process-lifetime stats worker goroutine
		// that has no clean shutdown hook — out of our control.
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		// database/sql's connection opener is started by sql.Open and lives
		// until the *DB is GC'd; tests don't always close their DB handles.
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		// fsnotify watchers spawn multiple internal goroutines (readEvents,
		// recv, etc.) that we cannot enumerate by TopFunction reliably.
		goleak.IgnoreAnyFunction("github.com/fsnotify/fsnotify.*"),
	)
}

func TestNewRealKernel(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	if kernel == nil {
		t.Fatal("NewRealKernel() returned nil")
	}

	// Should load schemas and policy from mangle directory
	if kernel.GetSchemas() == "" {
		t.Log("Warning: No schemas loaded (expected if mangle files not in path)")
	}
}

func TestKernelLoadPolicyFileIsIdempotent(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	modules := []string{
		// "coder.mg", // REMOVED: Split into multiple files, not loadable as a single file anymore.
		"tester.mg",
		"reviewer.mg",
	}

	for _, module := range modules {
		t.Run(module, func(t *testing.T) {
			if err := kernel.LoadPolicyFile(module); err != nil {
				t.Fatalf("LoadPolicyFile(%q) error = %v", module, err)
			}
			if err := kernel.LoadPolicyFile(module); err != nil {
				t.Fatalf("LoadPolicyFile(%q) second call error = %v", module, err)
			}
			if err := kernel.Evaluate(); err != nil {
				t.Fatalf("Evaluate() after LoadPolicyFile(%q) error = %v", module, err)
			}
		})
	}
}

func TestKernelLoadFacts(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	baseCount := kernel.FactCount()

	facts := []Fact{
		{Predicate: "test_state", Args: []any{"/passing"}},
	}

	err = kernel.LoadFacts(facts)
	if err != nil {
		t.Fatalf("LoadFacts() error = %v", err)
	}

	if kernel.FactCount() != baseCount+len(facts) {
		t.Errorf("FactCount() = %d, want %d", kernel.FactCount(), baseCount+len(facts))
	}

	results, err := kernel.Query("test_state")
	if err != nil {
		t.Fatalf("Query(test_state) error = %v", err)
	}
	if len(results) == 0 {
		t.Error("Query(test_state) returned no results after LoadFacts()")
	}
}

func TestKernelAssert(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	fact := Fact{
		Predicate: "retry_count",
		Args:      []any{int64(0)},
	}

	err = kernel.Assert(fact)
	if err != nil {
		t.Fatalf("Assert() error = %v", err)
	}
}

func TestKernelQuery(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// Load a fact
	facts := []Fact{
		{Predicate: "test_state", Args: []any{"/failing"}},
		{Predicate: "retry_count", Args: []any{int64(1)}},
	}

	err = kernel.LoadFacts(facts)
	if err != nil {
		t.Fatalf("LoadFacts() error = %v", err)
	}

	// Query
	results, err := kernel.Query("test_state")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Query() returned no results, expected at least 1")
	}
}

func TestKernelQueryAll(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	facts := []Fact{
		{Predicate: "test_state", Args: []any{"/passing"}},
		{Predicate: "retry_count", Args: []any{int64(2)}},
	}

	err = kernel.LoadFacts(facts)
	if err != nil {
		t.Fatalf("LoadFacts() error = %v", err)
	}

	results, err := kernel.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("QueryAll() returned empty map")
	}
}

func TestKernelRetract(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// Load facts
	facts := []Fact{
		{Predicate: "test_state", Args: []any{"/failing"}},
		{Predicate: "retry_count", Args: []any{int64(1)}},
	}

	err = kernel.LoadFacts(facts)
	if err != nil {
		t.Fatalf("LoadFacts() error = %v", err)
	}

	// Retract test_state
	err = kernel.Retract("test_state")
	if err != nil {
		t.Fatalf("Retract() error = %v", err)
	}

	testStateFacts, err := kernel.Query("test_state")
	if err != nil {
		t.Fatalf("Query(test_state) error = %v", err)
	}
	if len(testStateFacts) != 0 {
		t.Errorf("expected test_state facts to be retracted, found %d", len(testStateFacts))
	}

	retryFacts, err := kernel.Query("retry_count")
	if err != nil {
		t.Fatalf("Query(retry_count) error = %v", err)
	}
	if len(retryFacts) == 0 {
		t.Error("expected retry_count fact to remain after Retract()")
	}
}

func TestKernelRetractExactFact(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	f1 := Fact{Predicate: "pending_action", Args: []any{"id1", "/write_file", "a.go", map[string]any{}, int64(1)}}
	f2 := Fact{Predicate: "pending_action", Args: []any{"id2", "/write_file", "b.go", map[string]any{}, int64(2)}}

	if err := kernel.LoadFacts([]Fact{f1, f2}); err != nil {
		t.Fatalf("LoadFacts() error = %v", err)
	}

	if err := kernel.RetractExactFact(f1); err != nil {
		t.Fatalf("RetractExactFact() error = %v", err)
	}

	remaining, err := kernel.Query("pending_action")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining pending_action facts = %d, want 1", len(remaining))
	}
	if len(remaining[0].Args) < 3 || remaining[0].Args[2] != "b.go" {
		t.Fatalf("remaining fact = %v, want target b.go", remaining[0])
	}
}

func TestKernelClear(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	facts := []Fact{
		{Predicate: "test_state", Args: []any{"/passing"}},
	}

	_ = kernel.LoadFacts(facts)
	kernel.Clear()

	if kernel.FactCount() != 0 {
		t.Errorf("FactCount() = %d after Clear(), want 0", kernel.FactCount())
	}

	if kernel.IsInitialized() {
		t.Error("IsInitialized() should be false after Clear()")
	}
}

func TestFactString(t *testing.T) {
	tests := []struct {
		name string
		fact Fact
		want string
	}{
		{
			name: "name constant",
			fact: Fact{Predicate: "status", Args: []any{"/active"}},
			want: `status(/active).`,
		},
		{
			name: "string arg",
			fact: Fact{Predicate: "file", Args: []any{"main.go"}},
			want: `file("main.go").`,
		},
		{
			name: "int arg",
			fact: Fact{Predicate: "count", Args: []any{42}},
			want: `count(42).`,
		},
		{
			name: "bool true",
			fact: Fact{Predicate: "flag", Args: []any{true}},
			want: `flag(/true).`,
		},
		{
			name: "bool false",
			fact: Fact{Predicate: "flag", Args: []any{false}},
			want: `flag(/false).`,
		},
		{
			name: "mixed args",
			fact: Fact{Predicate: "record", Args: []any{"test.go", int64(100), "/go", true}},
			want: `record("test.go", 100, /go, /true).`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fact.String()
			if got != tt.want {
				t.Errorf("Fact.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFactToAtom(t *testing.T) {
	tests := []struct {
		name    string
		fact    Fact
		wantErr bool
	}{
		{
			name:    "simple string",
			fact:    Fact{Predicate: "test", Args: []any{"hello"}},
			wantErr: false,
		},
		{
			name:    "name constant",
			fact:    Fact{Predicate: "status", Args: []any{"/active"}},
			wantErr: false,
		},
		{
			name:    "string starting with // is not a name constant",
			fact:    Fact{Predicate: "file_content", Args: []any{"// Package main\npackage main\n"}},
			wantErr: false,
		},
		{
			name:    "unix path starting with / is not a name constant",
			fact:    Fact{Predicate: "path", Args: []any{"/usr/local/bin"}},
			wantErr: false,
		},
		{
			name:    "int arg",
			fact:    Fact{Predicate: "num", Args: []any{int64(42)}},
			wantErr: false,
		},
		{
			name:    "float arg",
			fact:    Fact{Predicate: "score", Args: []any{0.95}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fact.ToAtom()
			if (err != nil) != tt.wantErr {
				t.Errorf("ToAtom() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestKernelPermissionDerivation verifies that the embedded policy.mg
// correctly derives permitted/3 facts from safe_action/1 facts.
// This is critical for the VirtualStore constitutional permission checks.
func TestKernelPermissionDerivation(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// Fix 15.3: permitted now requires pending_action context.
	// We must assert pending_action facts for the safe actions we want to check.
	expectedPermitted := []string{
		"/read_file",
		"/fs_read",
		"/write_file",
		"/fs_write",
		"/search_files",
		"/review",
		"/run_tests",
		"/vector_search",
	}

	var pendingActions []Fact
	for _, action := range expectedPermitted {
		pendingActions = append(pendingActions, Fact{
			Predicate: "pending_action",
			Args:      []any{"id_" + action, action, "target.txt", map[string]any{}, int64(1234567890)},
		})
	}
	if err := kernel.LoadFacts(pendingActions); err != nil {
		t.Fatalf("LoadFacts(pending_action) error = %v", err)
	}

	// Query the derived permitted predicate
	permittedFacts, err := kernel.Query("permitted")
	if err != nil {
		t.Fatalf("Query('permitted') error = %v", err)
	}

	// Build a set of permitted actions for easy lookup
	permittedActions := make(map[string]bool)
	for _, fact := range permittedFacts {
		if len(fact.Args) > 0 {
			if arg, ok := fact.Args[0].(string); ok {
				permittedActions[arg] = true
			}
		}
	}

	// Verify core safe actions are derived as permitted
	for _, action := range expectedPermitted {
		if !permittedActions[action] {
			t.Errorf("Expected %s to be permitted (derived from safe_action), but it was not found in permitted facts", action)
		}
	}

	// Also verify that dangerous actions are NOT in the permitted set (without approval)
	// These should only be permitted with admin_override + signed_approval
	dangerousActions := []string{
		"/delete_system_files",
		"/format_disk",
	}

	// Assert pending actions for dangerous actions too, to verify they are still blocked
	var dangerousPending []Fact
	for _, action := range dangerousActions {
		dangerousPending = append(dangerousPending, Fact{
			Predicate: "pending_action",
			Args:      []any{"id_" + action, action, "critical.sys", map[string]any{}, int64(1234567890)},
		})
	}
	_ = kernel.LoadFacts(dangerousPending)

	// Re-query permitted
	permittedFacts, _ = kernel.Query("permitted")
	permittedActions = make(map[string]bool)
	for _, fact := range permittedFacts {
		if len(fact.Args) > 0 {
			if arg, ok := fact.Args[0].(string); ok {
				permittedActions[arg] = true
			}
		}
	}

	for _, action := range dangerousActions {
		if permittedActions[action] {
			t.Errorf("Expected %s to NOT be permitted without approval, but it was found in permitted facts", action)
		}
	}

	t.Logf("Found %d permitted actions from derived facts", len(permittedActions))
}

// TestKernelLoadCoderPolicyDoesNotTypeConflict ensures that loading coder.mg
// does not introduce a numeric comparison over non-numeric context_priority values.
//
// Regression: policy.mg historically derived context_priority(FactID, /high) which
// collided with coder.mg's numeric threshold checks (P >= 50), causing
// "value /high is not a number" during evaluation once coder.mg was loaded.
//
// LOGOS UPDATE: We now load the split coder files.
func TestKernelLoadCoderPolicyDoesNotTypeConflict(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// Ensure file_in_project(File) can be derived so coder.mg's file-prioritization
	// joins against a real project file.
	projectFile := "internal/core/kernel.go"
	if err := kernel.LoadFacts([]Fact{
		{
			Predicate: "file_topology",
			Args: []any{
				projectFile,
				"deadbeef",
				MangleAtom("/go"),
				int64(0),
				MangleAtom("/false"),
			},
		},
		// Trigger context_priority derivation via policy.mg activation rules.
		{
			Predicate: "activation",
			Args:      []any{projectFile, int64(80)},
		},
	}); err != nil {
		t.Fatalf("LoadFacts(seed) error = %v", err)
	}

	// Loading coder.mg (now coder_workflow.mg) should not crash evaluation due to context_priority typing.
	// Since we split the file and load it by default now via defaults/policy/ iteration,
	// this test confirms that the conflict is resolved in the base system.
	if err := kernel.Evaluate(); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
}

func TestKernelAccessors(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// 1. GetBaseFacts
	facts := []Fact{
		{Predicate: "test_state", Args: []any{"/passing"}},
	}
	if err := kernel.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts() error = %v", err)
	}
	baseFacts := kernel.GetBaseFacts()
	found := false
	for _, f := range baseFacts {
		if f.Predicate == "test_state" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find test_state in base facts")
	}

	// 2. GetProgramInfo
	// Under test setup it should be non-nil if it compiled successfully, or nil if not loaded.
	// But it shouldn't panic.
	_ = kernel.GetProgramInfo()
}

package core

import (
	"testing"
)

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

func TestKernelLoadFacts(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	facts := []Fact{
		{Predicate: "test_state", Args: []interface{}{"/passing"}},
	}

	err = kernel.LoadFacts(facts)
	if err != nil {
		t.Fatalf("LoadFacts() error = %v", err)
	}

	if kernel.FactCount() != 1 {
		t.Errorf("FactCount() = %d, want 1", kernel.FactCount())
	}
}

func TestKernelAssert(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	fact := Fact{
		Predicate: "retry_count",
		Args:      []interface{}{int64(0)},
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
		{Predicate: "test_state", Args: []interface{}{"/failing"}},
		{Predicate: "retry_count", Args: []interface{}{int64(1)}},
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
		{Predicate: "test_state", Args: []interface{}{"/passing"}},
		{Predicate: "retry_count", Args: []interface{}{int64(2)}},
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
		{Predicate: "test_state", Args: []interface{}{"/failing"}},
		{Predicate: "retry_count", Args: []interface{}{int64(1)}},
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

	// Check that only retry_count remains
	if kernel.FactCount() != 1 {
		t.Errorf("FactCount() = %d after Retract(), want 1", kernel.FactCount())
	}
}

func TestKernelRetractExactFact(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	f1 := Fact{Predicate: "pending_action", Args: []interface{}{"id1", "/write_file", "a.go", map[string]interface{}{}, int64(1)}}
	f2 := Fact{Predicate: "pending_action", Args: []interface{}{"id2", "/write_file", "b.go", map[string]interface{}{}, int64(2)}}

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
		{Predicate: "test_state", Args: []interface{}{"/passing"}},
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
			fact: Fact{Predicate: "status", Args: []interface{}{"/active"}},
			want: `status(/active).`,
		},
		{
			name: "string arg",
			fact: Fact{Predicate: "file", Args: []interface{}{"main.go"}},
			want: `file("main.go").`,
		},
		{
			name: "int arg",
			fact: Fact{Predicate: "count", Args: []interface{}{42}},
			want: `count(42).`,
		},
		{
			name: "bool true",
			fact: Fact{Predicate: "flag", Args: []interface{}{true}},
			want: `flag(/true).`,
		},
		{
			name: "bool false",
			fact: Fact{Predicate: "flag", Args: []interface{}{false}},
			want: `flag(/false).`,
		},
		{
			name: "mixed args",
			fact: Fact{Predicate: "record", Args: []interface{}{"test.go", int64(100), "/go", true}},
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
			fact:    Fact{Predicate: "test", Args: []interface{}{"hello"}},
			wantErr: false,
		},
		{
			name:    "name constant",
			fact:    Fact{Predicate: "status", Args: []interface{}{"/active"}},
			wantErr: false,
		},
		{
			name:    "int arg",
			fact:    Fact{Predicate: "num", Args: []interface{}{int64(42)}},
			wantErr: false,
		},
		{
			name:    "float arg",
			fact:    Fact{Predicate: "score", Args: []interface{}{0.95}},
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
// correctly derives permitted/1 facts from safe_action/1 facts.
// This is critical for the VirtualStore constitutional permission checks.
func TestKernelPermissionDerivation(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
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
	// These match the safe_action facts in policy.mg
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

	for _, action := range dangerousActions {
		if permittedActions[action] {
			t.Errorf("Expected %s to NOT be permitted without approval, but it was found in permitted facts", action)
		}
	}

	t.Logf("Found %d permitted actions from derived facts", len(permittedActions))
}

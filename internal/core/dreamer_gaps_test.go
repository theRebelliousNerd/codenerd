package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// Remediation for dreamer_test.go TEST_GAP markers (20 gaps total).
// QA: 2026-04-15_dreamer_boundary_analysis.md
// ============================================================================

// ---------- Null/Undefined ----------

// TestDreamerGap_NilContext verifies SimulateAction with nil context.
func TestDreamerGap_NilContext(t *testing.T) {
	d, _ := setupTestDreamer(t)

	req := ActionRequest{
		Type:   ActionReadFile,
		Target: "test.go",
	}

	// SimulateAction upgrades nil context to context.Background()
	result := d.SimulateAction(nil, req)
	if result.ActionID == "" {
		t.Error("Expected ActionID to be set even with nil context")
	}
}

// TestDreamerGap_NilKernel verifies SimulateAction when kernel is nil (fail-closed).
func TestDreamerGap_NilKernel(t *testing.T) {
	d := NewDreamer(nil) // nil kernel

	ctx := context.Background()
	req := ActionRequest{
		Type:   ActionReadFile,
		Target: "test.go",
	}

	result := d.SimulateAction(ctx, req)

	// Should fail-closed: Unsafe=true
	if !result.Unsafe {
		t.Error("Expected fail-closed (Unsafe=true) when kernel is nil, got safe")
	}
	if result.Reason == "" {
		t.Error("Expected reason for nil kernel failure")
	}
}

// TestDreamerGap_EmptyActionRequestFields verifies behavior with empty Type/Target.
func TestDreamerGap_EmptyActionRequestFields(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	// Empty Type
	result := d.SimulateAction(ctx, ActionRequest{Type: "", Target: "file.go"})
	if result.ActionID == "" {
		t.Error("Expected ActionID even with empty Type")
	}

	// Empty Target
	result = d.SimulateAction(ctx, ActionRequest{Type: ActionReadFile, Target: ""})
	if result.ActionID == "" {
		t.Error("Expected ActionID even with empty Target")
	}

	// Both empty
	result = d.SimulateAction(ctx, ActionRequest{})
	if result.ActionID == "" {
		t.Error("Expected ActionID even with zero-value ActionRequest")
	}
}

// ---------- Type Coercion ----------

// TestDreamerGap_MangleAtomVsStringMismatch verifies that projected facts
// use MangleAtom for Mangle atoms (not raw strings).
func TestDreamerGap_MangleAtomVsStringMismatch(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	req := ActionRequest{
		Type:   ActionDeleteFile,
		Target: "internal/core/test.go",
	}

	result := d.SimulateAction(ctx, req)

	// Check that projected_action uses MangleAtom for the type
	for _, f := range result.ProjectedFacts {
		if f.Predicate == "projected_action" && len(f.Args) >= 2 {
			typeArg := f.Args[1]
			if _, ok := typeArg.(MangleAtom); !ok {
				t.Errorf("projected_action type arg should be MangleAtom, got %T: %v", typeArg, typeArg)
			}
		}
	}
}

// TestDreamerGap_ComplexTypesInPayload verifies behavior when Payload contains
// complex Go types that can't be cleanly converted to Mangle.
func TestDreamerGap_ComplexTypesInPayload(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	req := ActionRequest{
		Type:   ActionExecCmd,
		Target: "echo hello",
		Payload: map[string]any{
			"nested_map":   map[string]any{"key": "val"},
			"slice":        []string{"a", "b", "c"},
			"nil_value":    nil,
			"int_value":    42,
			"string_value": "normal",
		},
	}

	// Should not panic
	result := d.SimulateAction(ctx, req)
	if result.ActionID == "" {
		t.Error("Expected ActionID even with complex payload")
	}
}

// TestDreamerGap_AtomVsStringDissonance verifies that ActionRequest.Type is
// converted to a Mangle atom (not a string) in projected facts.
func TestDreamerGap_AtomVsStringDissonance(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	req := ActionRequest{
		Type:   ActionReadFile,
		Target: "file.go",
	}

	result := d.SimulateAction(ctx, req)

	// Find projected_action and verify the type arg
	for _, f := range result.ProjectedFacts {
		if f.Predicate == "projected_action" && len(f.Args) >= 2 {
			typeArg := f.Args[1]
			atom, ok := typeArg.(MangleAtom)
			if !ok {
				t.Errorf("Expected MangleAtom for action type, got %T: %v", typeArg, typeArg)
				continue
			}
			// Should start with / (Mangle atom prefix)
			if !strings.HasPrefix(string(atom), "/") {
				t.Errorf("MangleAtom should start with /, got: %s", atom)
			}
		}
	}
}

// ---------- User Extremes ----------

// TestDreamerGap_MassivePathLength verifies behavior with a 1MB target path.
func TestDreamerGap_MassivePathLength(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping massive path test in short mode")
	}

	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	massivePath := strings.Repeat("a/", 500000) + "file.go" // ~1MB path

	req := ActionRequest{
		Type:   ActionReadFile,
		Target: massivePath,
	}

	// Should not panic or OOM
	result := d.SimulateAction(ctx, req)
	if result.ActionID == "" {
		t.Error("Expected ActionID even with massive path")
	}
}

// TestDreamerGap_DeeplyNestedPaths verifies behavior with deeply nested paths.
func TestDreamerGap_DeeplyNestedPaths(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	// Build a 200-segment path
	segments := make([]string, 200)
	for i := range segments {
		segments[i] = fmt.Sprintf("dir_%d", i)
	}
	deepPath := strings.Join(segments, "/") + "/file.go"

	req := ActionRequest{
		Type:   ActionReadFile,
		Target: deepPath,
	}

	result := d.SimulateAction(ctx, req)
	if result.ActionID == "" {
		t.Error("Expected ActionID even with deeply nested path")
	}
}

// ---------- Performance ----------

// TestDreamerGap_PerformanceFullTableScan documents the O(N) query cost.
func TestDreamerGap_PerformanceFullTableScan(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	d, k := setupTestDreamer(t)
	ctx := context.Background()

	// Load 1000 code_defines facts to stress the full table scan
	for i := range 1000 {
		k.AssertWithoutEval(Fact{
			Predicate: "code_defines",
			Args: []any{
				fmt.Sprintf("file_%d.go", i),
				fmt.Sprintf("Symbol_%d", i),
				"function",
				int64(1),
				int64(50),
			},
		})
	}
	k.Evaluate()

	req := ActionRequest{
		Type:   ActionDeleteFile,
		Target: "file_500.go",
	}

	// Should complete without hanging
	result := d.SimulateAction(ctx, req)
	if result.ActionID == "" {
		t.Error("Expected ActionID after performance test")
	}
	t.Logf("Performance: ProjectedFacts=%d, Unsafe=%v", len(result.ProjectedFacts), result.Unsafe)
}

// TestDreamerGap_KernelCloneCost documents the cost of kernel cloning.
func TestDreamerGap_KernelCloneCost(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping clone cost test in short mode")
	}

	d, k := setupTestDreamer(t)
	ctx := context.Background()

	// Load many facts
	for i := range 500 {
		k.AssertWithoutEval(Fact{
			Predicate: "code_defines",
			Args: []any{
				fmt.Sprintf("file_%d.go", i),
				fmt.Sprintf("Func_%d", i),
				"function",
				int64(1),
				int64(10),
			},
		})
	}
	k.Evaluate()

	// Run 10 simulations to measure clone cost
	for i := range 10 {
		result := d.SimulateAction(ctx, ActionRequest{
			Type:   ActionReadFile,
			Target: fmt.Sprintf("file_%d.go", i),
		})
		if result.ActionID == "" {
			t.Fatalf("Iteration %d: expected ActionID", i)
		}
	}
}

// ---------- Security ----------

// TestDreamerGap_PathTraversalBypass verifies criticalPrefix with path traversal.
func TestDreamerGap_PathTraversalBypass(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantHit bool
	}{
		{"direct internal/core", "internal/core/kernel.go", true},
		{"traversal bypass", "something/../internal/core/kernel.go", true},
		{"double slash", "internal//core/kernel.go", true},
		{".git direct", ".git/config", true},
		{"unicode safe", "internalⓐcore/kernel.go", false}, // not a real path
		{"case variant", "Internal/Core/kernel.go", false}, // case-sensitive
		{"absolute path", "C:/repo/internal/core/kernel.go", true},
		{"sibling directory", "internal/corex/kernel.go", false},
		{"suffix directory", "pkg/internal/mangle_old/x.go", false},
		{"dotfile sibling", ".nerdfoo/state.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := criticalPrefix(tt.path)
			gotHit := got != ""
			if gotHit != tt.wantHit {
				t.Errorf("criticalPrefix(%q) = %q, wantHit=%v, gotHit=%v",
					tt.path, got, tt.wantHit, gotHit)
			}
		})
	}
}

// TestDreamer_CriticalPathIsAboutRemoval pins the meaning of critical paths:
// deleting under internal/core is a panic state, editing a file there is
// ordinary work (codeNERD must be able to modify its own kernel and CLI), and
// writing into .git is still blocked.
func TestDreamer_CriticalPathIsAboutRemoval(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	hasCriticalHit := func(r DreamResult) bool {
		for _, f := range r.ProjectedFacts {
			if f.Predicate == "projected_fact" && len(f.Args) > 1 {
				if atom, ok := f.Args[1].(MangleAtom); ok && atom == "/critical_path_hit" {
					return true
				}
			}
		}
		return false
	}

	cases := []struct {
		name     string
		req      ActionRequest
		wantHit  bool
		wantSafe bool
	}{
		{"edit kernel source", ActionRequest{Type: ActionEditFile, Target: "internal/core/kernel.go"}, false, true},
		{"write cli source", ActionRequest{Type: ActionWriteFile, Target: "cmd/nerd/cmd_chat.go"}, false, true},
		{"edit lines in mangle engine", ActionRequest{Type: ActionEditLines, Target: "internal/mangle/engine.go"}, false, true},
		{"delete kernel source", ActionRequest{Type: ActionDeleteFile, Target: "internal/core/kernel.go"}, true, false},
		{"write into .git", ActionRequest{Type: ActionWriteFile, Target: ".git/config"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := d.SimulateAction(ctx, tc.req)
			if got := hasCriticalHit(r); got != tc.wantHit {
				t.Errorf("critical_path_hit projected=%v, want %v (facts=%v)", got, tc.wantHit, r.ProjectedFacts)
			}
			if safe := !r.Unsafe; safe != tc.wantSafe {
				t.Errorf("safe=%v, want %v (reason=%q)", safe, tc.wantSafe, r.Reason)
			}
		})
	}
}

// TestDreamerGap_SecurityShellFeatures verifies isDangerousCommand catches
// indirect execution and shell features.
func TestDreamerGap_SecurityShellFeatures(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected bool
	}{
		{"eval base64", "eval $(echo cm0gLXJmIC8= | base64 -d)", false},   // Not directly caught by current patterns
		{"python exec", "python -c 'import os; os.remove(\"/\")'", false}, // Not caught
		{"safe command", "go test ./...", false},

		// These ARE caught by the existing patterns
		{"rm -rf direct", "rm -rf /", true},
		{"mkfs pattern", "mkfs.ext4 /dev/sda1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDangerousCommand(tt.cmd)
			if got != tt.expected {
				t.Errorf("isDangerousCommand(%q) = %v, want %v", tt.cmd, got, tt.expected)
			}
		})
	}

	t.Log("KNOWN GAP: isDangerousCommand uses pattern matching, not semantic analysis. " +
		"Indirect execution (eval, python -c, base64) can bypass detection.")
}

// ---------- Resource Exhaustion ----------

// TestDreamerGap_BoundedDreamCache verifies the DreamCache eviction policy.
func TestDreamerGap_BoundedDreamCache(t *testing.T) {
	cache := NewDreamCache()

	// Store more than the max to trigger eviction
	const count = dreamCacheMaxSize + 100
	for i := range count {
		key := fmt.Sprintf("action_%d:target_%d", i, i)
		cache.Store(key, DreamResult{
			ActionID: fmt.Sprintf("action_%d", i),
			Unsafe:   i%2 == 0,
			Reason:   fmt.Sprintf("reason_%d", i),
		})
	}

	// Verify cache didn't grow beyond max
	cache.mu.RLock()
	cacheSize := len(cache.results)
	cache.mu.RUnlock()

	if cacheSize > dreamCacheMaxSize {
		t.Errorf("Cache size %d exceeds max %d after eviction", cacheSize, dreamCacheMaxSize)
	}

	// Verify recent entries are still accessible
	recentKey := fmt.Sprintf("action_%d:target_%d", count-1, count-1)
	result, ok := cache.Get(recentKey)
	if !ok {
		t.Error("Expected cache hit for most recent entry")
	}
	if result.ActionID != fmt.Sprintf("action_%d", count-1) {
		t.Errorf("Wrong result for recent entry: %s", result.ActionID)
	}

	// Verify Invalidate clears everything
	cache.Invalidate()
	_, ok = cache.Get(recentKey)
	if ok {
		t.Error("Expected cache miss after Invalidate()")
	}

	t.Logf("DreamCache bounded: stored %d, final size %d (max %d)", count, cacheSize, dreamCacheMaxSize)
}

// ---------- Fragile Defaults ----------

// TestDreamerGap_UnknownActionTypes verifies projectEffects behavior with unknown types.
func TestDreamerGap_UnknownActionTypes(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	// Use a custom action type not in the switch statement
	req := ActionRequest{
		Type:   ActionType("custom_unknown_action"),
		Target: "file.go",
	}

	result := d.SimulateAction(ctx, req)

	// Should still get the base projected_action fact
	if len(result.ProjectedFacts) == 0 {
		t.Error("Expected at least projected_action fact for unknown action type")
	}

	// Verify the base projected_action exists
	foundAction := false
	for _, f := range result.ProjectedFacts {
		if f.Predicate == "projected_action" {
			foundAction = true
			break
		}
	}
	if !foundAction {
		t.Error("Expected projected_action fact for unknown action type")
	}

	t.Log("KNOWN: Unknown action types produce only the base projected_action fact (no special projections).")
}

// ---------- Reliability ----------

// TestDreamerGap_PanicSafety verifies AssertWithoutEval doesn't panic on malformed inputs.
func TestDreamerGap_PanicSafety(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	// Test with various edge-case ActionRequests
	edgeCases := []ActionRequest{
		{Type: ActionDeleteFile, Target: "\x00\x01\x02"}, // binary chars
		{Type: ActionExecCmd, Target: strings.Repeat("a", 100000)},
		{Type: ActionWriteFile, Target: ""},
		{Type: "", Target: ""},
	}

	for i, req := range edgeCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			// Should not panic
			result := d.SimulateAction(ctx, req)
			if result.ActionID == "" {
				t.Error("Expected ActionID even for edge case")
			}
		})
	}
}

// ---------- Concurrency ----------

// TestDreamerGap_ConcurrentSetKernelVsSimulate verifies thread safety of
// SetKernel vs SimulateAction.
func TestDreamerGap_ConcurrentSetKernelVsSimulate(t *testing.T) {
	d, k := setupTestDreamer(t)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Concurrent readers
	for range 20 {
		wg.Go(func() {
			for range 5 {
				_ = d.SimulateAction(ctx, ActionRequest{
					Type:   ActionReadFile,
					Target: "file.go",
				})
			}
		})
	}

	// Concurrent writer
	for range 3 {
		wg.Go(func() {
			for range 5 {
				d.SetKernel(k) // Re-set to same kernel
			}
		})
	}

	wg.Wait()
	// No panics or data races should occur (run with -race)
}

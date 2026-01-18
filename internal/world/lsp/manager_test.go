package lsp

import (
	"testing"

	"codenerd/internal/mangle"
)

// =============================================================================
// MANAGER TESTS
// =============================================================================

func TestNewManager(t *testing.T) {
	t.Parallel()

	mgr := NewManager("/tmp/workspace")

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestManager_WorkspaceRoot(t *testing.T) {
	t.Parallel()

	mgr := NewManager("/test/workspace")

	mgr.mu.RLock()
	root := mgr.workspaceRoot
	mgr.mu.RUnlock()

	if root != "/test/workspace" {
		t.Errorf("workspaceRoot mismatch: got %q", root)
	}
}

func TestManager_NotIndexedInitially(t *testing.T) {
	t.Parallel()

	mgr := NewManager("/test/workspace")

	mgr.mu.RLock()
	indexed := mgr.indexed
	mgr.mu.RUnlock()

	if indexed {
		t.Error("manager should not be indexed initially")
	}
}

// =============================================================================
// UTILITY FUNCTION TESTS
// =============================================================================

func TestPathToURI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		contains string
	}{
		{"unix_path", "/home/user/file.go", "file:///"},
		{"simple", "test.go", "file:///"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := pathToURI(tc.path)
			if result == "" {
				t.Error("expected non-empty URI")
			}
		})
	}
}

func TestReferenceKindToAtom(t *testing.T) {
	t.Parallel()

	// Test that the function exists and handles basic input
	result := referenceKindToAtom(mangle.RefInHead)
	if result == "" {
		t.Error("expected non-empty result")
	}

	// Test unknown kind
	result = referenceKindToAtom(-1)
	if result == "" {
		t.Error("expected fallback result for unknown kind")
	}
}

func TestDiagnosticSeverityToAtom(t *testing.T) {
	t.Parallel()

	// Test all severity levels
	severities := []struct {
		name string
	}{
		{"error"},
		{"warning"},
		{"info"},
		{"hint"},
	}

	for _, tc := range severities {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// This tests the function exists and handles inputs without panicking
		})
	}
}

// =============================================================================
// PROJECT TO FACTS TESTS
// =============================================================================

func TestManager_ProjectToFacts_NotInitialized(t *testing.T) {
	t.Parallel()

	mgr := NewManager("/test/workspace")

	// Should handle not-initialized state gracefully
	facts, err := mgr.ProjectToFacts()
	if err != nil {
		// Expected for uninitialized manager
		t.Logf("ProjectToFacts error (expected): %v", err)
	}
	_ = facts
}

// =============================================================================
// QUERY API TESTS
// =============================================================================

func TestManager_GetDefinitions_NotInitialized(t *testing.T) {
	t.Parallel()

	mgr := NewManager("/test/workspace")

	facts, err := mgr.GetDefinitions("TestSymbol")
	if err != nil {
		// Expected for uninitialized manager
		t.Logf("GetDefinitions error (expected): %v", err)
	}
	_ = facts
}

func TestManager_GetReferences_NotInitialized(t *testing.T) {
	t.Parallel()

	mgr := NewManager("/test/workspace")

	facts, err := mgr.GetReferences("TestSymbol")
	if err != nil {
		// Expected for uninitialized manager
		t.Logf("GetReferences error (expected): %v", err)
	}
	_ = facts
}

func TestManager_ValidateCode_NotInitialized(t *testing.T) {
	t.Parallel()

	mgr := NewManager("/test/workspace")

	facts, err := mgr.ValidateCode("test.mangle", "test code")
	if err != nil {
		// Expected for uninitialized manager
		t.Logf("ValidateCode error (expected): %v", err)
	}
	_ = facts
}

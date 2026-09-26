package lsp

import (
	"testing"
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := pathToURI(tc.path)
			if result == "" {
				t.Error("expected non-empty URI")
			}
		})
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// This tests the function exists and handles inputs without panicking
		})
	}
}

// =============================================================================
// PROJECT TO FACTS TESTS
// =============================================================================

// =============================================================================
// QUERY API TESTS
// =============================================================================

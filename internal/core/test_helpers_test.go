package core

import (
	"testing"
)

// setupMockKernel initializes a RealKernel for testing.
// It is shared across test files in the core package.
func setupMockKernel(t *testing.T) *RealKernel {
	t.Helper()
	k, err := NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	return k
}

package core

import (
	"testing"
)

// setupMockKernel initializes a RealKernel for testing.
// It is shared across test files in the core package.
func setupMockKernel(t *testing.T) *RealKernel {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	return k
}

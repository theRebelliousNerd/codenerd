package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrioritizedCallers(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "holographic_perf_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy Go file
	fileContent := `package main

func HighPriority() {
	println("High")
}

func MediumPriority() {
	println("Medium")
}

func LowPriority() {
	println("Low")
}
`
	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	h := NewHolographicProvider(nil, tempDir)

	// Case 1: Sorting and Fetching
	callers := []PrioritizedCaller{
		{Name: "LowPriority", File: filePath, Priority: 20, Depth: 1},
		{Name: "HighPriority", File: filePath, Priority: 80, Depth: 1},
		{Name: "MediumPriority", File: filePath, Priority: 50, Depth: 1},
	}

	ctx := context.Background()
	resolved, err := h.ResolvePrioritizedCallers(ctx, callers)
	if err != nil {
		t.Fatalf("ResolvePrioritizedCallers failed: %v", err)
	}

	if len(resolved) != 3 {
		t.Errorf("Expected 3 callers, got %d", len(resolved))
	}

	// Verify order: High, Medium, Low
	if resolved[0].Name != "HighPriority" {
		t.Errorf("Expected first caller to be HighPriority, got %s", resolved[0].Name)
	}
	if resolved[1].Name != "MediumPriority" {
		t.Errorf("Expected second caller to be MediumPriority, got %s", resolved[1].Name)
	}
	if resolved[2].Name != "LowPriority" {
		t.Errorf("Expected third caller to be LowPriority, got %s", resolved[2].Name)
	}

	// Verify bodies were fetched
	for _, c := range resolved {
		if c.Body == "" {
			t.Errorf("Expected body for %s, got empty", c.Name)
		}
	}

	// Case 2: Limiting
	// Create more callers than maxPrioritizedCallers (10)
	manyCallers := make([]PrioritizedCaller, 15)
	for i := 0; i < 15; i++ {
		manyCallers[i] = PrioritizedCaller{
			Name:     "HighPriority", // Reuse same function to avoid creating many functions
			File:     filePath,
			Priority: 10 + i, // Varied priority
			Depth:    1,
		}
	}

	resolvedMany, err := h.ResolvePrioritizedCallers(ctx, manyCallers)
	if err != nil {
		t.Fatalf("ResolvePrioritizedCallers (many) failed: %v", err)
	}

	if len(resolvedMany) != 10 {
		t.Errorf("Expected 10 callers (limited), got %d", len(resolvedMany))
	}

	// Verify the top priority ones are kept
	// Top priority is 10+14=24. Lowest in top 10 should be 10+5=15.
	// Since we sort descending, the first one should be Priority 24.
	if resolvedMany[0].Priority != 24 {
		t.Errorf("Expected highest priority 24, got %d", resolvedMany[0].Priority)
	}
}

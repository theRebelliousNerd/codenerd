package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/store"
)

func TestEnsureDeepFacts_NoGoFiles(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	txtFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(txtFile, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("Failed to write txt file: %v", err)
	}

	result, err := EnsureDeepFacts(ctx, []string{txtFile}, nil, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.FilesParsed != 0 {
		t.Errorf("Expected 0 files parsed, got %d", result.FilesParsed)
	}
	if len(result.NewFacts) != 0 {
		t.Errorf("Expected 0 new facts, got %d", len(result.NewFacts))
	}
}

func TestEnsureDeepFacts_GoFileWithoutDB(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "test.go")
	content := []byte(`package main

func main() {
}
`)
	err := os.WriteFile(goFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write go file: %v", err)
	}

	result, err := EnsureDeepFacts(ctx, []string{goFile}, nil, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.FilesParsed != 1 {
		t.Errorf("Expected 1 file parsed, got %d", result.FilesParsed)
	}
	if len(result.NewFacts) == 0 {
		t.Errorf("Expected >0 new facts for valid go file, got 0")
	}
	if len(result.RetractFacts) != 0 {
		t.Errorf("Expected 0 retract facts without DB, got %d", len(result.RetractFacts))
	}
}

func TestEnsureDeepFacts_MissingFile(t *testing.T) {
	ctx := context.Background()

	missingFile := "does_not_exist.go"

	result, err := EnsureDeepFacts(ctx, []string{missingFile}, nil, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.FilesParsed != 0 {
		t.Errorf("Expected 0 files parsed, got %d", result.FilesParsed)
	}
}

func TestEnsureDeepFacts_DefaultWorkers(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "test.go")
	content := []byte(`package main

func main() {}
`)
	err := os.WriteFile(goFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write go file: %v", err)
	}

	// Pass 0 or negative for workers to test fallback
	result, err := EnsureDeepFacts(ctx, []string{goFile}, nil, -1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.FilesParsed != 1 {
		t.Errorf("Expected 1 file parsed, got %d", result.FilesParsed)
	}
}

func TestEnsureDeepFacts_GoFileWithDB_Caching(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	// We might need to call db.Close() but let's assume NewLocalStore handles setup correctly for testing.

	goFile := filepath.Join(tmpDir, "test.go")
	content := []byte(`package main
func first() {}
`)
	err = os.WriteFile(goFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write go file: %v", err)
	}

	// First pass
	res1, err := EnsureDeepFacts(ctx, []string{goFile}, db, 1)
	if err != nil {
		t.Fatalf("Pass 1 error: %v", err)
	}
	if res1.FilesParsed != 1 {
		t.Errorf("Expected 1 parsed in pass 1, got %d", res1.FilesParsed)
	}
	if len(res1.RetractFacts) != 0 {
		t.Errorf("Expected 0 retracted in pass 1, got %d", len(res1.RetractFacts))
	}

	factsCount1 := len(res1.NewFacts)

	// Second pass: file unchanged. It should hit the cache.
	res2, err := EnsureDeepFacts(ctx, []string{goFile}, db, 1)
	if err != nil {
		t.Fatalf("Pass 2 error: %v", err)
	}
	if res2.FilesParsed != 0 { // From cache, not parsed!
		t.Errorf("Expected 0 parsed in pass 2 (cached), got %d", res2.FilesParsed)
	}
	if len(res2.NewFacts) != factsCount1 {
		t.Errorf("Expected %d cached facts, got %d", factsCount1, len(res2.NewFacts))
	}
	if len(res2.RetractFacts) != factsCount1 {
		t.Errorf("Expected %d retracted facts for reuse, got %d", factsCount1, len(res2.RetractFacts))
	}

	// Third pass: change file to invalidate fingerprint
	// Ensure enough time has passed so ModTime changes OR size changes. Let's change size and modtime.
	time.Sleep(10 * time.Millisecond) // Just in case, to get different modtime

	content2 := []byte(`package main
func first() {}
func second() {}
`)
	err = os.WriteFile(goFile, content2, 0644)
	if err != nil {
		t.Fatalf("Failed to rewrite go file: %v", err)
	}

	res3, err := EnsureDeepFacts(ctx, []string{goFile}, db, 1)
	if err != nil {
		t.Fatalf("Pass 3 error: %v", err)
	}
	if res3.FilesParsed != 1 {
		t.Errorf("Expected 1 parsed in pass 3 (cache miss), got %d", res3.FilesParsed)
	}
	// It should retract the old facts since they don't match the fingerprint
	if len(res3.RetractFacts) != factsCount1 {
		t.Errorf("Expected %d retracted facts on miss, got %d", factsCount1, len(res3.RetractFacts))
	}
	if len(res3.NewFacts) <= factsCount1 {
		t.Errorf("Expected more new facts after adding function, got %d vs %d", len(res3.NewFacts), factsCount1)
	}
}

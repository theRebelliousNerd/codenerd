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
	t.Cleanup(func() { _ = db.Close() })

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

func TestEnsureDeepFacts(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup LocalStore
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create LocalStore: %v", err)
	}
	defer db.Close()

	// 2. Setup Test File
	goFile := filepath.Join(tmpDir, "test.go")
	content := []byte(`package main
func test() {}
`)
	if err := os.WriteFile(goFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	nonGoFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(nonGoFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 3. Test New Parse
	t.Run("NewParse", func(t *testing.T) {
		res, err := EnsureDeepFacts(context.Background(), []string{goFile, nonGoFile}, db, 1)
		if err != nil {
			t.Fatalf("EnsureDeepFacts failed: %v", err)
		}
		if res.FilesParsed != 1 {
			t.Errorf("Expected 1 file parsed, got %d", res.FilesParsed)
		}
		if len(res.NewFacts) == 0 {
			t.Error("Expected new facts to be parsed, got 0")
		}
		if len(res.RetractFacts) != 0 {
			t.Errorf("Expected 0 retracted facts for new file, got %d", len(res.RetractFacts))
		}
	})

	// 4. Test Cached Parse (Reuse)
	t.Run("CachedParse", func(t *testing.T) {
		res, err := EnsureDeepFacts(context.Background(), []string{goFile}, db, 1)
		if err != nil {
			t.Fatalf("EnsureDeepFacts failed: %v", err)
		}
		if res.FilesParsed != 0 {
			t.Errorf("Expected 0 files parsed (should use cache), got %d", res.FilesParsed)
		}
		if len(res.NewFacts) == 0 {
			t.Error("Expected facts from cache, got 0")
		}
		// In the current implementation, it ALWAYS retracts existing deep facts
		// to avoid duplicates, even if it then reuses them!
		if len(res.RetractFacts) == 0 {
			t.Errorf("Expected >0 retracted facts for cache reuse, got %d", len(res.RetractFacts))
		}
	})

	// 5. Test Modified File (Retract + Parse)
	t.Run("ModifiedParse", func(t *testing.T) {
		// modify file content to change fingerprint
		content2 := []byte(`package main
func test2() {}
`)
		if err := os.WriteFile(goFile, content2, 0644); err != nil {
			t.Fatalf("Failed to modify test file: %v", err)
		}

		// Ensure mtime has updated
		info, _ := os.Stat(goFile)
		_ = info

		res, err := EnsureDeepFacts(context.Background(), []string{goFile}, db, 1)
		if err != nil {
			t.Fatalf("EnsureDeepFacts failed: %v", err)
		}
		if res.FilesParsed != 1 {
			t.Errorf("Expected 1 files parsed (cache invalidated), got %d", res.FilesParsed)
		}
		if len(res.NewFacts) == 0 {
			t.Error("Expected new facts, got 0")
		}
		if len(res.RetractFacts) == 0 {
			t.Errorf("Expected >0 retracted facts for modified file, got %d", len(res.RetractFacts))
		}
	})

	// 6. Test File Not Found
	t.Run("FileNotFound", func(t *testing.T) {
		res, err := EnsureDeepFacts(context.Background(), []string{"does_not_exist.go"}, db, 1)
		if err != nil {
			t.Fatalf("EnsureDeepFacts failed: %v", err)
		}
		if res.FilesParsed != 0 {
			t.Errorf("Expected 0 files parsed, got %d", res.FilesParsed)
		}
	})

	// 7. Test Nil DB
	t.Run("NilDB", func(t *testing.T) {
		res, err := EnsureDeepFacts(context.Background(), []string{goFile}, nil, 1)
		if err != nil {
			t.Fatalf("EnsureDeepFacts failed: %v", err)
		}
		// with nil db, it should re-parse
		if res.FilesParsed != 1 {
			t.Errorf("Expected 1 file parsed with nil db, got %d", res.FilesParsed)
		}
	})

	// 8. Test Invalid Go file
	t.Run("InvalidGoFile", func(t *testing.T) {
		invalidGoFile := filepath.Join(tmpDir, "invalid.go")
		if err := os.WriteFile(invalidGoFile, []byte("invalid go syntax {}{}"), 0644); err != nil {
			t.Fatalf("Failed to create invalid test file: %v", err)
		}
		res, err := EnsureDeepFacts(context.Background(), []string{invalidGoFile}, db, 1)
		if err != nil {
			t.Fatalf("EnsureDeepFacts failed: %v", err)
		}
		if res.FilesParsed != 0 {
			t.Errorf("Expected 0 files parsed for invalid go file, got %d", res.FilesParsed)
		}
	})
}

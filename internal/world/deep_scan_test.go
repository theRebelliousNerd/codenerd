package world

import (
	"codenerd/internal/store"
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

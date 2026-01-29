//go:build integration
package retrieval_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/retrieval"
	"github.com/stretchr/testify/require"
)

func TestSparseRetriever_Integration(t *testing.T) {
	// 1. Setup temporary directory
	rootDir := t.TempDir()

	// 2. Create test files
	files := map[string]string{
		"target.go":              "package main\n\nfunc main() { panic(\"TargetError\") }",
		"ignored.go":             "package main\n\nfunc main() { fmt.Println(\"Nothing\") }",
		"node_modules/lib.js":    "console.log(\"TargetError\")", // Should be ignored
		"subdir/nested.py":       "raise TargetError('oops')",
		"subdir/deep/ignored.txt": "TargetError", // Not a code file, might be picked up if not careful, but SparseRetriever extracts keywords.
	}

	for path, content := range files {
		fullPath := filepath.Join(rootDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// 3. Initialize Retriever
	cfg := retrieval.DefaultSparseRetrieverConfig(rootDir)
	// Ensure we're using the temp dir as root
	cfg.WorkDir = rootDir
	// Default config excludes node_modules

	retriever := retrieval.NewSparseRetriever(cfg)

	// 4. Run Search
	ctx := context.Background()
	// The query mentions "TargetError", which should be extracted as a primary keyword.
	query := "panic: TargetError in production system"

	candidates, err := retriever.FindRelevantFiles(ctx, query, 10)
	require.NoError(t, err)

	// 5. Verification
	require.NotEmpty(t, candidates, "Should find at least one file")

	foundFiles := make(map[string]bool)
	for _, c := range candidates {
		// Normalize path to be relative to root for easier checking
		rel, _ := filepath.Rel(rootDir, c.FilePath)
		foundFiles[rel] = true
	}

	// target.go should be found (contains "TargetError")
	require.True(t, foundFiles["target.go"], "target.go should be found")

	// subdir/nested.py should be found (contains "TargetError")
	require.True(t, foundFiles["subdir/nested.py"], "subdir/nested.py should be found")

	// ignored.go should NOT be found (does not contain keyword)
	require.False(t, foundFiles["ignored.go"], "ignored.go should NOT be found")

	// node_modules/lib.js should NOT be found (excluded dir)
	require.False(t, foundFiles["node_modules/lib.js"], "node_modules/lib.js should be excluded")
}

func TestSparseRetriever_ContextCancellation(t *testing.T) {
	rootDir := t.TempDir()
	// Create a large number of files to ensure search takes some time
	for i := 0; i < 100; i++ {
		fname := filepath.Join(rootDir, fmt.Sprintf("file_%d.go", i))
		_ = os.WriteFile(fname, []byte("package main"), 0644)
	}

	cfg := retrieval.DefaultSparseRetrieverConfig(rootDir)
	retriever := retrieval.NewSparseRetriever(cfg)

	// Create a context that cancels immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// It might return an error or just empty results depending on where the check happens
	// But it must not hang.
	done := make(chan struct{})
	go func() {
		_, _ = retriever.FindRelevantFiles(ctx, "panic: Something", 10)
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("FindRelevantFiles hung on context cancellation")
	}
}

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
	"github.com/stretchr/testify/suite"
)

type SparseRetrieverSuite struct {
	suite.Suite
	tempDir string
}

func (s *SparseRetrieverSuite) SetupTest() {
	s.tempDir = s.T().TempDir()

	// Create file1.txt
	err := os.WriteFile(filepath.Join(s.tempDir, "file1.txt"), []byte("foo bar baz"), 0644)
	s.Require().NoError(err)

	// Create subdir/file2.go
	err = os.Mkdir(filepath.Join(s.tempDir, "subdir"), 0755)
	s.Require().NoError(err)
	file2Content := "package subdir\n\nfunc MyFunc() {\n\t// TODO: fix this\n}"
	err = os.WriteFile(filepath.Join(s.tempDir, "subdir", "file2.go"), []byte(file2Content), 0644)
	s.Require().NoError(err)

	// Create ignored/file3.py
	err = os.Mkdir(filepath.Join(s.tempDir, "ignored"), 0755)
	s.Require().NoError(err)
	err = os.WriteFile(filepath.Join(s.tempDir, "ignored", "file3.py"), []byte("ignored"), 0644)
	s.Require().NoError(err)
}

func (s *SparseRetrieverSuite) TestSearchKeywords_RealRg() {
	cfg := retrieval.DefaultSparseRetrieverConfig(s.tempDir)
	r := retrieval.NewSparseRetriever(cfg)

	kw := &retrieval.IssueKeywords{
		Primary: []string{"MyFunc"},
		Weights: map[string]float64{"MyFunc": 1.0},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hits, err := r.SearchKeywords(ctx, kw)
	s.Require().NoError(err)
	s.Require().Len(hits, 1)

	// Avoid asserting exact path formatting: ripgrep may return absolute or relative paths.
	s.Require().Contains(hits[0].FilePath, "file2.go")
	s.Require().Equal("MyFunc", hits[0].Keyword)
}

func (s *SparseRetrieverSuite) TestSearchKeywords_NoMatch() {
	cfg := retrieval.DefaultSparseRetrieverConfig(s.tempDir)
	r := retrieval.NewSparseRetriever(cfg)

	kw := &retrieval.IssueKeywords{
		Primary: []string{"nonexistent"},
		Weights: map[string]float64{"nonexistent": 1.0},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hits, err := r.SearchKeywords(ctx, kw)
	s.Require().NoError(err)
	s.Require().Empty(hits)
}

func (s *SparseRetrieverSuite) TestFindRelevantFiles() {
	cfg := retrieval.DefaultSparseRetrieverConfig(s.tempDir)
	r := retrieval.NewSparseRetriever(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// "Fix bug in MyFunc" should extract MyFunc as a keyword.
	candidates, err := r.FindRelevantFiles(ctx, "Fix bug in MyFunc", 10)
	s.Require().NoError(err)

	s.Require().NotEmpty(candidates)
	s.Require().Contains(candidates[0].FilePath, "file2.go")
}

func (s *SparseRetrieverSuite) TestSearchKeywords_Exclusions() {
	cfg := retrieval.DefaultSparseRetrieverConfig(s.tempDir)
	// Add "ignored" to exclusions.
	cfg.ExcludePatterns = append(cfg.ExcludePatterns, "ignored")
	r := retrieval.NewSparseRetriever(cfg)

	kw := &retrieval.IssueKeywords{
		Primary: []string{"ignored"},
		Weights: map[string]float64{"ignored": 1.0},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hits, err := r.SearchKeywords(ctx, kw)
	s.Require().NoError(err)
	s.Require().Empty(hits, "Should not find file in excluded directory")
}

func TestSparseRetrieverSuite(t *testing.T) {
	suite.Run(t, new(SparseRetrieverSuite))
}

func TestSparseRetriever_Integration(t *testing.T) {
	// 1. Setup temporary directory
	rootDir := t.TempDir()

	// 2. Create test files
	files := map[string]string{
		"target.go":               "package main\n\nfunc main() { panic(\"TargetError\") }",
		"ignored.go":              "package main\n\nfunc main() { fmt.Println(\"Nothing\") }",
		"node_modules/lib.js":     "console.log(\"TargetError\")", // Should be ignored
		"subdir/nested.py":        "raise TargetError('oops')",
		"subdir/deep/ignored.txt": "TargetError", // Not a code file; ok if it shows up, we just care about key positives/negatives.
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
		// Normalize path to be relative to root for easier checking.
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
	// Create a large number of files to ensure search takes some time.
	for i := 0; i < 100; i++ {
		fname := filepath.Join(rootDir, fmt.Sprintf("file_%d.go", i))
		_ = os.WriteFile(fname, []byte("package main"), 0644)
	}

	cfg := retrieval.DefaultSparseRetrieverConfig(rootDir)
	retriever := retrieval.NewSparseRetriever(cfg)

	// Create a context that cancels immediately.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// It might return an error or just empty results depending on where the check happens,
	// but it must not hang.
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

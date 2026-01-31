//go:build integration
package retrieval_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/retrieval"
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

	// ripgrep output paths are relative to workDir usually, but let's check suffix
	// or normalized path. Based on sparse.go implementation, it passes workDir as last arg to rg.
	// rg usually prints paths relative to the search root.
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

	// "Fix bug in MyFunc" should extract MyFunc as a keyword
	candidates, err := r.FindRelevantFiles(ctx, "Fix bug in MyFunc", 10)
	s.Require().NoError(err)

	s.Require().NotEmpty(candidates)
	s.Require().Contains(candidates[0].FilePath, "file2.go")
}

func (s *SparseRetrieverSuite) TestSearchKeywords_Exclusions() {
	cfg := retrieval.DefaultSparseRetrieverConfig(s.tempDir)
	// Add "ignored" to exclusions
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

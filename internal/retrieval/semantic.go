package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// =============================================================================
// TIER 4: SEMANTIC EXPANSION
// =============================================================================
//
// Tier 4 was documented as "requires embedding service" and implemented as a
// definition-name scan, which finds files that spell a symbol and nothing that
// is merely about the same thing. This is the injection point for a real vector
// pass. It is optional on purpose: a session with no embedding backend running
// must still build a context, so an absent, failing or empty searcher falls
// through to the definition scan rather than emptying the tier.

// SemanticMatch is one vector-similarity hit.
type SemanticMatch struct {
	FilePath string
	// Score is a 0..1 similarity.
	Score float64
}

// SemanticSearcher ranks workspace files by semantic similarity to a query.
type SemanticSearcher interface {
	SimilarFiles(ctx context.Context, query string, limit int) ([]SemanticMatch, error)
}

// Embedder is the slice of an embedding engine this package needs.
// embedding.EmbeddingEngine satisfies it; the narrow form keeps the dependency
// fakeable in tests that must not reach a live endpoint.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Semantic corpus bounds. A full-repository embedding pass on the caller's turn
// is not affordable, so the corpus is a bounded sample of source heads and the
// vectors are cached for the searcher's lifetime.
const (
	// semanticCorpusLimit caps how many files are embedded.
	semanticCorpusLimit = 256

	// semanticHeadBytes is how much of each file is embedded. Package docs,
	// imports and the first declarations carry most of a file's topic.
	semanticHeadBytes = 4 << 10
)

// EmbeddingSemanticSearcher implements SemanticSearcher over an embedding engine
// and the workspace source tree.
type EmbeddingSemanticSearcher struct {
	engine  Embedder
	workDir string

	corpusLimit int
	headBytes   int

	mu      sync.Mutex
	vectors map[string][]float32
}

// NewEmbeddingSemanticSearcher builds a Tier 4 searcher over the given engine.
// Returns nil when no engine is supplied, so callers can pass the result
// straight into TieredContextConfig.Semantic and get the heuristic fallback.
func NewEmbeddingSemanticSearcher(engine Embedder, workDir string) *EmbeddingSemanticSearcher {
	if engine == nil {
		return nil
	}
	if workDir == "" {
		workDir = "."
	}
	return &EmbeddingSemanticSearcher{
		engine:      engine,
		workDir:     workDir,
		corpusLimit: semanticCorpusLimit,
		headBytes:   semanticHeadBytes,
		vectors:     make(map[string][]float32),
	}
}

// SimilarFiles embeds the query and cosine-ranks the workspace corpus against it.
func (s *EmbeddingSemanticSearcher) SimilarFiles(ctx context.Context, query string, limit int) ([]SemanticMatch, error) {
	if s == nil || s.engine == nil {
		return nil, nil
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	queryVec, err := s.engine.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	paths, err := s.corpus(ctx)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	vectors, err := s.vectorsFor(ctx, paths)
	if err != nil {
		return nil, err
	}

	matches := make([]SemanticMatch, 0, len(vectors))
	for path, vec := range vectors {
		sim, cerr := embedding.CosineSimilarity(queryVec, vec)
		if cerr != nil {
			continue
		}
		matches = append(matches, SemanticMatch{FilePath: path, Score: sim})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].FilePath < matches[j].FilePath
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

// corpus lists the source files eligible for embedding, deterministically
// ordered so the bounded sample is stable between calls.
func (s *EmbeddingSemanticSearcher) corpus(ctx context.Context) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(s.workDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "__pycache__", ".venv", "venv", "vendor", "dist", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if !isSemanticSourceFile(path) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)
	if len(paths) > s.corpusLimit {
		paths = paths[:s.corpusLimit]
	}
	return paths, ctx.Err()
}

// vectorsFor returns embeddings for the given paths, embedding only the ones
// not already cached and batching that remainder in a single engine call.
func (s *EmbeddingSemanticSearcher) vectorsFor(ctx context.Context, paths []string) (map[string][]float32, error) {
	out := make(map[string][]float32, len(paths))
	var missing []string
	var texts []string

	s.mu.Lock()
	for _, p := range paths {
		if v, ok := s.vectors[p]; ok {
			out[p] = v
			continue
		}
		missing = append(missing, p)
	}
	s.mu.Unlock()

	// missing and texts must stay index-aligned: EmbedBatch answers positionally,
	// so an unreadable file that is skipped here has to be dropped from both.
	var readable []string
	for _, p := range missing {
		head, err := readHead(p, s.headBytes)
		if err != nil || strings.TrimSpace(head) == "" {
			continue
		}
		readable = append(readable, p)
		texts = append(texts, head)
	}
	missing = readable

	if len(texts) == 0 {
		return out, nil
	}

	vecs, err := s.engine.EmbedBatch(ctx, texts)
	if err != nil {
		return out, fmt.Errorf("embedding %d workspace files: %w", len(texts), err)
	}
	if len(vecs) != len(missing) {
		return out, fmt.Errorf("embedding backend returned %d vectors for %d files", len(vecs), len(missing))
	}

	s.mu.Lock()
	for i, p := range missing {
		s.vectors[p] = vecs[i]
		out[p] = vecs[i]
	}
	s.mu.Unlock()

	return out, nil
}

func readHead(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, n)
	read, err := f.Read(buf)
	if read <= 0 {
		if err != nil {
			return "", err
		}
		return "", nil
	}
	buf = buf[:read]
	if isBinaryContent(buf) {
		return "", nil
	}
	return string(buf), nil
}

var semanticSourceExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".rs": true, ".java": true, ".kt": true, ".rb": true, ".c": true, ".h": true,
	".cpp": true, ".hpp": true, ".cs": true, ".swift": true, ".php": true,
	".vue": true, ".svelte": true, ".scala": true, ".mg": true,
}

func isSemanticSourceFile(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return false
	}
	return semanticSourceExts[strings.ToLower(filepath.Ext(path))]
}

// semanticMatchFiles converts vector hits into Tier 4 context files. Split out
// so the fallback path and the embedding path produce identical shapes.
func semanticMatchFiles(matches []SemanticMatch, max int, addedFiles map[string]bool) []ContextFile {
	var files []ContextFile
	for _, m := range matches {
		if len(files) >= max {
			break
		}
		if m.FilePath == "" || addedFiles[m.FilePath] {
			continue
		}
		if _, err := os.Stat(m.FilePath); err != nil {
			continue
		}
		addedFiles[m.FilePath] = true

		score := m.Score
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		files = append(files, ContextFile{
			FilePath:        m.FilePath,
			Tier:            4,
			RelevanceScore:  score,
			SelectionReason: fmt.Sprintf("Semantically similar to the issue (%.2f)", score),
		})
	}
	if len(files) > 0 {
		logging.Context("TieredContextBuilder: Tier 4 - %d files from embedding similarity", len(files))
	}
	return files
}

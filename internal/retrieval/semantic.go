package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// SemanticResult is one similarity search: its matches and what it covered.
type SemanticResult struct {
	Matches []SemanticMatch
	// Eligible is how many files the corpus rule admits (source files, Go
	// tests excluded); Covered is how many were embedded and ranked. Covered
	// below Eligible is a bounded corpus (or an incomplete embedding) -- not
	// a search of the whole repository.
	Eligible int
	Covered  int
	// Incomplete is the error that left part of the selected corpus
	// unembedded ("" when none did).
	Incomplete string
}

// SemanticSearcher ranks workspace files by semantic similarity to a query.
type SemanticSearcher interface {
	SimilarFiles(ctx context.Context, query string, limit int) (SemanticResult, error)
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

	// semanticEmbedChunkSize bounds one EmbedBatch call. Chunks are cached as
	// they complete: a caller budget expiring mid-corpus must keep the vectors
	// already paid for instead of restarting from zero next turn.
	semanticEmbedChunkSize = 32
)

// EmbeddingSemanticSearcher implements SemanticSearcher over an embedding engine
// and the workspace source tree.
type EmbeddingSemanticSearcher struct {
	engine  Embedder
	workDir string

	corpusLimit int
	headBytes   int

	// mu guards vectors and inflight; it is never held across IO or an
	// embedding call.
	mu sync.Mutex
	// vectors is keyed by path and holds the digest of the exact head bytes
	// the vector represents: a hit needs the same digest, so an edited file
	// is embedded again and never ranked by its old text. The searcher is
	// bound to one engine, workspace and head size for its lifetime, which
	// completes the key.
	vectors map[string]cachedVector
	// inflight are the (path, digest) pairs some caller is embedding; a
	// concurrent caller waits for that result instead of paying for it twice.
	inflight map[vectorKey]chan struct{}
}

type cachedVector struct {
	digest string
	vec    []float32
}

type vectorKey struct{ path, digest string }

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
		vectors:     make(map[string]cachedVector),
		inflight:    make(map[vectorKey]chan struct{}),
	}
}

// Warm embeds the bounded corpus ahead of any query, off the turn's path. The
// vectors land in the cache SimilarFiles ranks from, so a turn only ranks;
// before this, every turn spent its retrieval budget embedding and the pass
// expired with a fraction of the corpus covered. Safe to run beside queries:
// a concurrent caller waits for a vector in flight instead of paying twice.
func (s *EmbeddingSemanticSearcher) Warm(ctx context.Context) error {
	if s == nil || s.engine == nil {
		return nil
	}
	paths, _, err := s.corpus(ctx)
	if err != nil || len(paths) == 0 {
		return err
	}
	_, err = s.vectorsFor(ctx, paths)
	return err
}

// SimilarFiles embeds the query and cosine-ranks the workspace corpus against
// it, reporting how much of the eligible corpus the ranking covered.
func (s *EmbeddingSemanticSearcher) SimilarFiles(ctx context.Context, query string, limit int) (SemanticResult, error) {
	var result SemanticResult
	if s == nil || s.engine == nil {
		return result, nil
	}
	if strings.TrimSpace(query) == "" {
		return result, nil
	}
	if limit <= 0 {
		limit = 10
	}

	queryVec, err := s.engine.Embed(ctx, query)
	if err != nil {
		return result, fmt.Errorf("embedding query: %w", err)
	}
	if len(queryVec) == 0 {
		return result, fmt.Errorf("embedding query: backend returned an empty vector")
	}

	paths, eligible, err := s.corpus(ctx)
	result.Eligible = eligible
	if err != nil {
		return result, err
	}
	s.forget(paths)
	if len(paths) == 0 {
		return result, nil
	}

	vectors, err := s.vectorsFor(ctx, paths)
	if err != nil {
		if len(vectors) == 0 {
			return result, err
		}
		// A budget can expire with most of the corpus embedded. Ranking the
		// vectors we hold is strictly better than pretending Tier 4 found
		// nothing; the remainder stays uncached for the next call.
		result.Incomplete = err.Error()
		logging.Context("TieredContextBuilder: semantic corpus partially embedded (%d/%d files), ranking what is available: %v", len(vectors), len(paths), err)
	}

	matches := make([]SemanticMatch, 0, len(vectors))
	for path, vec := range vectors {
		// A vector of another dimension is another model's: it cannot be
		// compared with this query, and the file counts as not covered.
		if len(vec) != len(queryVec) {
			continue
		}
		sim, cerr := embedding.CosineSimilarity(queryVec, vec)
		if cerr != nil {
			continue
		}
		matches = append(matches, SemanticMatch{FilePath: path, Score: sim})
	}
	result.Covered = len(matches)
	if result.Covered < len(vectors) && result.Incomplete == "" {
		result.Incomplete = fmt.Sprintf("%d vectors did not match the query's dimension", len(vectors)-result.Covered)
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
	result.Matches = matches
	return result, nil
}

// forget drops the vectors of files no longer in the corpus, so the cache is
// bounded by the corpus, not by every revision of every file ever embedded.
func (s *EmbeddingSemanticSearcher) forget(paths []string) {
	keep := make(map[string]bool, len(paths))
	for _, p := range paths {
		keep[p] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for p := range s.vectors {
		if !keep[p] {
			delete(s.vectors, p)
		}
	}
}

// corpus lists the source files eligible for embedding, deterministically
// ordered so the bounded sample is stable between calls (spread across
// directories, see boundedCorpus), and how many were eligible before the
// corpus limit cut the list.
func (s *EmbeddingSemanticSearcher) corpus(ctx context.Context) ([]string, int, error) {
	mem, err := membershipFor(s.workDir)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %w", errMembership, err)
	}
	var paths []string
	err = filepath.WalkDir(s.workDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		member, admErr := mem.Admit(path, d.IsDir())
		if admErr != nil {
			return admErr
		}
		if !member || d.IsDir() {
			return nil
		}
		if !isSemanticSourceFile(path) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	sort.Strings(paths)
	eligible := len(paths)
	paths = boundedCorpus(paths, s.corpusLimit)
	return paths, eligible, ctx.Err()
}

// vectorsFor returns embeddings for the given paths at their current heads,
// embedding only the ones not cached at that exact content, in chunks that
// are cached as soon as they complete. A head another caller is already
// embedding is waited for, not embedded twice. On error it returns every
// vector it holds beside the error, so the caller can rank a partial corpus
// rather than none.
func (s *EmbeddingSemanticSearcher) vectorsFor(ctx context.Context, paths []string) (map[string][]float32, error) {
	out := make(map[string][]float32, len(paths))

	// Read every head first, outside the lock: the digest is the cache key.
	type head struct{ path, text, digest string }
	heads := make([]head, 0, len(paths))
	for _, p := range paths {
		text, err := readHead(p, s.headBytes)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		sum := sha256.Sum256([]byte(text))
		heads = append(heads, head{path: p, text: text, digest: hex.EncodeToString(sum[:])})
	}

	var owned []head
	var waits []head
	s.mu.Lock()
	for _, h := range heads {
		if v, ok := s.vectors[h.path]; ok && v.digest == h.digest {
			out[h.path] = v.vec
			continue
		}
		key := vectorKey{h.path, h.digest}
		if _, busy := s.inflight[key]; busy {
			waits = append(waits, h)
			continue
		}
		s.inflight[key] = make(chan struct{})
		owned = append(owned, h)
	}
	s.mu.Unlock()

	// release ends this caller's claim on hs, whether it stored a vector for
	// them or not, and wakes whoever waits on them.
	release := func(hs []head) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, h := range hs {
			key := vectorKey{h.path, h.digest}
			if ch, ok := s.inflight[key]; ok {
				close(ch)
				delete(s.inflight, key)
			}
		}
	}

	var embedErr error
	for start := 0; start < len(owned); start += semanticEmbedChunkSize {
		end := min(start+semanticEmbedChunkSize, len(owned))
		texts := make([]string, 0, end-start)
		for _, h := range owned[start:end] {
			texts = append(texts, h.text)
		}
		vecs, err := s.engine.EmbedBatch(ctx, texts)
		if err == nil && len(vecs) != len(texts) {
			err = fmt.Errorf("embedding backend returned %d vectors for %d files", len(vecs), len(texts))
		}
		if err != nil {
			embedErr = fmt.Errorf("embedding workspace files %d-%d of %d: %w", start, end-1, len(owned), err)
			release(owned[start:])
			break
		}
		s.mu.Lock()
		for i, h := range owned[start:end] {
			if len(vecs[i]) == 0 {
				continue // an empty vector represents nothing; do not cache it
			}
			s.vectors[h.path] = cachedVector{digest: h.digest, vec: vecs[i]}
			out[h.path] = vecs[i]
		}
		s.mu.Unlock()
		release(owned[start:end])
	}

	for _, h := range waits {
		s.mu.Lock()
		ch := s.inflight[vectorKey{h.path, h.digest}]
		s.mu.Unlock()
		if ch != nil {
			select {
			case <-ch:
			case <-ctx.Done():
				if embedErr == nil {
					embedErr = ctx.Err()
				}
				return out, embedErr
			}
		}
		s.mu.Lock()
		if v, ok := s.vectors[h.path]; ok && v.digest == h.digest {
			out[h.path] = v.vec
		} else if embedErr == nil {
			embedErr = fmt.Errorf("embedding %s failed in a concurrent pass", h.path)
		}
		s.mu.Unlock()
	}
	return out, embedErr
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

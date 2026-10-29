package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
// Tier 4 ranks workspace files by vector similarity to the issue. It is
// optional on purpose: a session with no embedding backend running must still
// build a context, so an absent, failing or empty searcher falls through to
// the definition scan rather than emptying the tier.
//
// Embedding the corpus is not the turn's job. One searcher serves the process
// (built at boot, internal/system), warms the whole corpus in the background,
// and keeps every vector in a FileVectorStore keyed by content digest and
// embedding space, so a new process starts warm and an edit re-embeds only the
// files it changed. A turn embeds its query, embeds any changed file nobody
// else is already embedding, and ranks.

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
	// tests excluded) that have text to embed; Covered is how many were
	// ranked. Covered below Eligible means part of the corpus had no vector
	// yet: Pending of them are being embedded by the background warm-up, and
	// Incomplete says what stopped the rest.
	Eligible int
	Covered  int
	Pending  int
	// Incomplete is the error or mismatch that left part of the corpus
	// unranked ("" when none did).
	Incomplete string
}

// SemanticSearcher ranks workspace files by semantic similarity to a query.
type SemanticSearcher interface {
	SimilarFiles(ctx context.Context, query string, limit int) (SemanticResult, error)
}

// Embedder is the slice of an embedding engine this package needs.
// embedding.EmbeddingEngine satisfies it; the narrow form keeps the dependency
// fakeable in tests that must not reach a live endpoint. Name is the embedding
// space a vector belongs to (internal/store/vector_identity.go): vectors from
// different spaces are never compared or shared.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Name() string
}

const (
	// semanticHeadBytes is how much of each file is embedded. Package docs,
	// imports and the first declarations carry most of a file's topic.
	semanticHeadBytes = 4 << 10

	// semanticEmbedChunkSize bounds one EmbedBatch call. Chunks are kept as
	// they complete: a budget expiring mid-corpus keeps the vectors already
	// paid for instead of restarting from zero.
	semanticEmbedChunkSize = 32
)

// EmbeddingSemanticSearcher implements SemanticSearcher over an embedding
// engine, the workspace source tree and an optional persistent vector store.
type EmbeddingSemanticSearcher struct {
	engine    Embedder
	workDir   string
	headBytes int
	// store persists vectors across processes; nil keeps them in memory
	// only. The searcher owns it and closes it in Close.
	store *FileVectorStore

	// mu guards vectors and inflight; it is never held across IO or an
	// embedding call.
	mu sync.Mutex
	// vectors is keyed by path and holds the digest and space of the exact
	// head bytes the vector represents: a hit needs both, so an edited file
	// or a changed model is embedded again and never ranked by stale text.
	vectors map[string]cachedVector
	// inflight are the heads some caller is embedding. Warm waits for them;
	// a turn does not (it counts them as not yet covered).
	inflight map[vectorKey]chan struct{}
}

type cachedVector struct {
	digest string
	space  string
	vec    []float32
}

type vectorKey struct{ path, digest, space string }

// NewEmbeddingSemanticSearcher builds a Tier 4 searcher over the given engine.
// store may be nil (vectors live for the searcher's lifetime only). Returns
// nil when no engine is supplied, so callers get the heuristic fallback.
func NewEmbeddingSemanticSearcher(engine Embedder, workDir string, store *FileVectorStore) *EmbeddingSemanticSearcher {
	if engine == nil {
		return nil
	}
	if workDir == "" {
		workDir = "."
	}
	return &EmbeddingSemanticSearcher{
		engine:    engine,
		workDir:   workDir,
		headBytes: semanticHeadBytes,
		store:     store,
		vectors:   make(map[string]cachedVector),
		inflight:  make(map[vectorKey]chan struct{}),
	}
}

// Close releases the persistent store, if any.
func (s *EmbeddingSemanticSearcher) Close() error {
	if s == nil {
		return nil
	}
	return s.store.Close()
}

// Warm embeds every corpus file that has no vector at its current content,
// off the turn's path: the persistent store answers what earlier processes
// embedded, and only new or edited files reach the engine. After a complete
// pass the store is trimmed to the files as they are now. Safe to run beside
// queries and to re-run: it waits for heads another caller is embedding
// instead of paying for them twice.
func (s *EmbeddingSemanticSearcher) Warm(ctx context.Context) (WarmReport, error) {
	var report WarmReport
	if s == nil || s.engine == nil {
		return report, nil
	}
	paths, err := s.corpus(ctx)
	if err != nil {
		return report, err
	}
	s.forget(paths)
	cv, err := s.vectorsFor(ctx, paths, true)
	report.Eligible = cv.heads
	report.Covered = len(cv.vecs)
	report.Embedded = cv.embedded
	report.Stored = cv.stored
	if err != nil {
		return report, err
	}
	space := strings.TrimSpace(s.engine.Name())
	if s.store != nil && space != "" {
		removed, rerr := s.store.Retain(ctx, space, cv.digests)
		report.Pruned = removed
		if rerr != nil {
			return report, rerr
		}
	}
	return report, nil
}

// WarmReport is what one Warm pass did.
type WarmReport struct {
	// Eligible files in the corpus have text to embed; Covered of them hold
	// a vector now.
	Eligible, Covered int
	// Embedded files reached the engine; Stored came from the persistent
	// store; Pruned rows of files that no longer exist were removed.
	Embedded, Stored, Pruned int
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

	paths, err := s.corpus(ctx)
	result.Eligible = len(paths)
	if err != nil {
		return result, err
	}
	s.forget(paths)
	if len(paths) == 0 {
		return result, nil
	}

	cv, err := s.vectorsFor(ctx, paths, false)
	var reasons []string
	if err != nil {
		if len(cv.vecs) == 0 {
			return result, err
		}
		// A budget can expire with most of the corpus embedded. Ranking the
		// vectors we hold is strictly better than pretending Tier 4 found
		// nothing; the remainder is embedded by the next caller.
		reasons = append(reasons, err.Error())
	}
	result.Eligible = cv.heads
	result.Pending = cv.pending

	matches := make([]SemanticMatch, 0, len(cv.vecs))
	for path, vec := range cv.vecs {
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
	if mismatched := len(cv.vecs) - result.Covered; mismatched > 0 {
		reasons = append(reasons, fmt.Sprintf("%d vectors did not match the query's dimension", mismatched))
	}
	result.Incomplete = strings.Join(reasons, "; ")
	if result.Covered < result.Eligible {
		logging.Context("TieredContextBuilder: semantic tier ranked %d/%d files (%d still embedding in the background) %s", result.Covered, result.Eligible, result.Pending, result.Incomplete)
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

// forget drops the in-memory vectors of files no longer in the corpus, so the
// cache is bounded by the corpus, not by every file ever embedded.
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

// corpus lists every source file eligible for embedding, sorted. There is no
// cap: with vectors persisted and embedded off the turn, the whole corpus is
// affordable, and a sample left files no query could ever reach.
func (s *EmbeddingSemanticSearcher) corpus(ctx context.Context) ([]string, error) {
	mem, err := membershipFor(s.workDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errMembership, err)
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
		return nil, err
	}
	sort.Strings(paths)
	return paths, ctx.Err()
}

// corpusVectors is what vectorsFor found for a set of paths.
type corpusVectors struct {
	// vecs holds a vector per path at its current content.
	vecs map[string][]float32
	// digests is every current head digest read, embedded or not: the set a
	// complete pass keeps in the store.
	digests map[string]bool
	// heads counts the paths with text to embed (a blank or binary file has
	// nothing to represent and is not part of the corpus).
	heads int
	// pending counts heads another caller was embedding that this one did
	// not wait for.
	pending int
	// embedded heads reached the engine; stored ones came from the store.
	embedded, stored int
}

// vectorsFor returns embeddings for the given paths at their current heads.
// Memory answers first, then the persistent store; only the rest reach the
// engine, in chunks kept (in memory and in the store) as each completes. A
// head another caller is embedding is waited for when wait is set and
// counted as pending otherwise. On error it returns every vector it holds
// beside the error, so the caller can rank a partial corpus rather than none.
func (s *EmbeddingSemanticSearcher) vectorsFor(ctx context.Context, paths []string, wait bool) (corpusVectors, error) {
	cv := corpusVectors{vecs: make(map[string][]float32, len(paths)), digests: make(map[string]bool, len(paths))}
	space := strings.TrimSpace(s.engine.Name())

	// Read every head first, outside the lock: the digest is the cache key.
	type head struct{ path, text, digest string }
	heads := make([]head, 0, len(paths))
	for _, p := range paths {
		text, err := readHead(p, s.headBytes)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		sum := sha256.Sum256([]byte(text))
		h := head{path: p, text: text, digest: hex.EncodeToString(sum[:])}
		heads = append(heads, h)
		cv.digests[h.digest] = true
	}
	cv.heads = len(heads)

	var owned, waits []head
	s.mu.Lock()
	for _, h := range heads {
		if v, ok := s.vectors[h.path]; ok && v.digest == h.digest && v.space == space {
			cv.vecs[h.path] = v.vec
			continue
		}
		key := vectorKey{h.path, h.digest, space}
		if _, busy := s.inflight[key]; busy {
			if wait {
				waits = append(waits, h)
			} else {
				cv.pending++
			}
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
			key := vectorKey{h.path, h.digest, space}
			if ch, ok := s.inflight[key]; ok {
				close(ch)
				delete(s.inflight, key)
			}
		}
	}
	keep := func(h head, vec []float32) {
		s.vectors[h.path] = cachedVector{digest: h.digest, space: space, vec: vec}
		cv.vecs[h.path] = vec
	}

	// The persistent store answers what any earlier process embedded.
	if s.store != nil && space != "" && len(owned) > 0 {
		digests := make([]string, 0, len(owned))
		for _, h := range owned {
			digests = append(digests, h.digest)
		}
		stored, err := s.store.Load(ctx, space, digests)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("semantic tier: file vector store unreadable, embedding instead: %v", err)
		}
		var hits, misses []head
		s.mu.Lock()
		for _, h := range owned {
			if vec, ok := stored[h.digest]; ok && len(vec) > 0 {
				keep(h, vec)
				hits = append(hits, h)
			} else {
				misses = append(misses, h)
			}
		}
		s.mu.Unlock()
		cv.stored = len(hits)
		release(hits)
		owned = misses
	}

	var embedErr error
	for start := 0; start < len(owned); start += semanticEmbedChunkSize {
		end := min(start+semanticEmbedChunkSize, len(owned))
		chunk := owned[start:end]
		texts := make([]string, 0, len(chunk))
		for _, h := range chunk {
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
		fresh := make(map[string][]float32, len(chunk))
		s.mu.Lock()
		for i, h := range chunk {
			if len(vecs[i]) == 0 {
				continue // an empty vector represents nothing; do not keep it
			}
			keep(h, vecs[i])
			fresh[h.digest] = vecs[i]
		}
		s.mu.Unlock()
		cv.embedded += len(fresh)
		if s.store != nil && space != "" {
			if err := s.store.Save(ctx, space, fresh); err != nil {
				logging.Get(logging.CategoryContext).Warn("semantic tier: %d vectors not persisted, the next process embeds them again: %v", len(fresh), err)
			}
		}
		release(chunk)
	}

	for _, h := range waits {
		s.mu.Lock()
		ch := s.inflight[vectorKey{h.path, h.digest, space}]
		s.mu.Unlock()
		if ch != nil {
			select {
			case <-ch:
			case <-ctx.Done():
				return cv, errors.Join(embedErr, ctx.Err())
			}
		}
		s.mu.Lock()
		if v, ok := s.vectors[h.path]; ok && v.digest == h.digest && v.space == space {
			cv.vecs[h.path] = v.vec
		} else if embedErr == nil {
			embedErr = fmt.Errorf("embedding %s failed in a concurrent pass", h.path)
		}
		s.mu.Unlock()
	}
	return cv, embedErr
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
